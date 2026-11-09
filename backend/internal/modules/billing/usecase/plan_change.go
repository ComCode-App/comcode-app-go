package usecase

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/database/db"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/payment"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// ChangePlan applies same-interval upgrades after charging the prorated difference.
// Downgrades (including free) are scheduled, without refund, at the period boundary.
func (s *Service) ChangePlan(ctx context.Context, payer Payer, planUUID uuid.UUID) error {
	if err := billingWriter(ctx, payer); err != nil {
		return err
	}
	now := time.Now().UTC()
	var claim *db.SubscriptionPaymentAttempt
	var input payment.SavedCardChargeInput
	var provider string
	err := s.transactions.WithTx(ctx, func(q *db.Queries) error {
		sub, err := q.LockAccountPaymentSubscription(ctx, payer.Account.ID)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		if sub.Status == db.SubscriptionStatusPastDue || !sub.NextBillingAt.Valid || !sub.NextBillingAt.Time.After(now) {
			return fmt.Errorf("%w: settle overdue payment before changing plans", ErrInvalidRequest)
		}
		unresolved, err := q.HasUnresolvedSubscriptionCharge(ctx, sub.ID)
		if err != nil {
			return err
		}
		if unresolved {
			return fmt.Errorf("%w: payment requires reconciliation", ErrInvalidRequest)
		}
		target, err := q.GetPlanByUUID(ctx, planUUID)
		if err != nil {
			return ErrNotFound
		}
		current, err := q.GetPlanByID(ctx, sub.PlanID)
		if err != nil {
			return err
		}
		if !target.IsActive || target.Currency != current.Currency {
			return ErrInvalidRequest
		}
		if target.ID == current.ID {
			sub.PendingPlanID = pgtype.Int8{}
			return saveSubscription(ctx, q, sub)
		}
		if target.BillingInterval != db.BillingIntervalFree && target.BillingInterval != current.BillingInterval {
			return fmt.Errorf("%w: interval changes require a new subscription after cancellation", ErrInvalidRequest)
		}
		if target.PriceMinor <= current.PriceMinor {
			sub.PendingPlanID = pgtype.Int8{Int64: target.ID, Valid: true}
			sub.CancelAtPeriodEnd = target.BillingInterval == db.BillingIntervalFree
			return saveSubscription(ctx, q, sub)
		}
		start := sub.StartsAt.Time
		if sub.PeriodStartedAt.Valid {
			start = sub.PeriodStartedAt.Time
		}
		amount := proratedDifference(
			target.PriceMinor-current.PriceMinor,
			sub.NextBillingAt.Time.Sub(now),
			sub.NextBillingAt.Time.Sub(start),
		)
		if amount <= 0 {
			return ErrInvalidRequest
		}
		card, err := q.RenewalCard(ctx, db.RenewalCardParams{
			AccountID: sub.AccountID, Provider: sub.Provider.PaymentProvider, SavedCardID: sub.SavedCardID,
		})
		if err != nil {
			return fmt.Errorf("%w: saved card required", ErrInvalidRequest)
		}
		number, err := q.NextUpgradeAttempt(ctx, db.NextUpgradeAttemptParams{SubscriptionID: sub.ID, PeriodAt: sub.NextBillingAt})
		if err != nil {
			return err
		}
		attempt, err := q.CreateSubscriptionPaymentAttempt(ctx, db.CreateSubscriptionPaymentAttemptParams{
			AccountID: sub.AccountID, SubscriptionID: sub.ID, PeriodAt: sub.NextBillingAt, Attempt: number,
			Kind: "upgrade", PlanID: target.ID, AmountMinor: amount, Currency: target.Currency,
		})
		if err != nil {
			return err
		}
		claim = &attempt
		provider = string(sub.Provider.PaymentProvider)
		input = payment.SavedCardChargeInput{
			MerchantOID: "upg" + attempt.Uuid.String(), AmountMinor: amount, Currency: target.Currency,
			UserEmail: payer.Email, UserName: payer.Name, UToken: card.ProviderCustomerRef, CToken: card.ProviderCardRef,
			PlanName: target.Name, ClientIP: "127.0.0.1",
		}
		return nil
	})
	if err != nil || claim == nil {
		return err
	}
	result, chargeErr := s.recurring.ChargeRecurring(ctx, provider, input)
	err = s.transactions.WithTx(ctx, func(q *db.Queries) error {
		sub, err := q.LockRenewalSubscription(ctx, claim.SubscriptionID)
		if err != nil {
			return err
		}
		status := "failed"
		code := "declined"
		if chargeErr != nil {
			status = "unknown"
			code = "reconciliation_required"
		}
		if errors.Is(chargeErr, payment.ErrRecurringUnsupported) {
			status = "failed"
			code = "recurring_unsupported"
		}
		if chargeErr == nil && result.Success {
			status = "succeeded"
			code = ""
		}
		if err := q.FinishSubscriptionPaymentAttempt(ctx, db.FinishSubscriptionPaymentAttemptParams{
			ID: claim.ID, Status: status, ProviderRef: textOpt(result.ProviderRef), FailureCode: textOpt(code),
		}); err != nil {
			return err
		}
		if status != "succeeded" {
			return billingEvent(ctx, q, sub, "payment.failed")
		}
		sub.PlanID = claim.PlanID
		sub.PendingPlanID = pgtype.Int8{}
		sub.CancelAtPeriodEnd = false
		if err := saveSubscription(ctx, q, sub); err != nil {
			return err
		}
		return billingEvent(ctx, q, sub, "payment.succeeded")
	})
	if err != nil {
		return err
	}
	if chargeErr != nil || !result.Success {
		return fmt.Errorf("%w: upgrade payment failed or requires reconciliation", ErrInvalidRequest)
	}
	return nil
}
