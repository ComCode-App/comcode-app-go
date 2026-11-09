package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"time"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/database/db"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/authctx"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/payment"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/rbac"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// RenewalPolicy offsets retries from the first failure, rather than from each retry.
type RenewalPolicy struct {
	RetryDays []int
	GraceDays int
}

func DefaultRenewalPolicy() RenewalPolicy {
	return RenewalPolicy{RetryDays: []int{1, 3, 7}, GraceDays: 14}
}

func (p RenewalPolicy) Validate() error {
	if p.GraceDays < 1 || p.GraceDays > 365 {
		return fmt.Errorf("billing grace days must be between 1 and 365")
	}
	previous := 0
	for _, day := range p.RetryDays {
		if day <= previous || day >= p.GraceDays {
			return fmt.Errorf("billing retry days must increase and precede grace expiry")
		}
		previous = day
	}
	return nil
}

// SetRecurringCharger allows a gateway adapter to implement explicit recurring support.
func (s *Service) SetRecurringCharger(charger payment.RecurringCharger) { s.recurring = charger }

func billingWriter(ctx context.Context, payer Payer) error {
	p, ok := authctx.PrincipalFrom(ctx)
	if !ok {
		return ErrForbidden
	}
	authorized := p.HasPermission(rbac.PermAppBillingWrite) && payer.Account.CanWrite()
	scoped := payer.Account.ID > 0 && p.Account.ID == payer.Account.ID
	if !authorized || !scoped {
		return ErrForbidden
	}
	return nil
}

func saveSubscription(ctx context.Context, q *db.Queries, sub db.Subscription) error {
	return q.SaveRenewalSubscription(ctx, db.SaveRenewalSubscriptionParams{
		ID: sub.ID, Status: sub.Status, PlanID: sub.PlanID, NextBillingAt: sub.NextBillingAt,
		EndsAt: sub.EndsAt, CanceledAt: sub.CanceledAt, PeriodStartedAt: sub.PeriodStartedAt,
		CancelAtPeriodEnd: sub.CancelAtPeriodEnd, PendingPlanID: sub.PendingPlanID,
		GraceUntil: sub.GraceUntil, RetryAt: sub.RetryAt,
		RenewalAttempts: sub.RenewalAttempts, DunningStartedAt: sub.DunningStartedAt,
	})
}

func billingEvent(
	ctx context.Context,
	q *db.Queries,
	sub db.Subscription,
	name string,
) error {
	body, err := json.Marshal(map[string]any{
		"event_id": uuid.New(), "entity_type": "subscription", "entity_id": sub.ID,
		"entity_uuid": sub.Uuid, "occurred_at": time.Now().UTC(),
		"data": map[string]any{"account_id": sub.AccountID, "subscription_uuid": sub.Uuid.String()},
	})
	if err != nil {
		return err
	}
	_, err = q.InsertOutboxEvent(ctx, db.InsertOutboxEventParams{EventName: name, Payload: body})
	return err
}

func (s *Service) ScheduleCancellation(ctx context.Context, payer Payer, cancel bool) error {
	if err := billingWriter(ctx, payer); err != nil {
		return err
	}
	return s.transactions.WithTx(ctx, func(q *db.Queries) error {
		sub, err := q.LockAccountPaymentSubscription(ctx, payer.Account.ID)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		if !sub.NextBillingAt.Valid {
			return fmt.Errorf("%w: no renewable period", ErrInvalidRequest)
		}
		if !cancel && !sub.NextBillingAt.Time.After(time.Now()) {
			return fmt.Errorf("%w: period already ended", ErrInvalidRequest)
		}
		unresolved, err := q.HasUnresolvedSubscriptionCharge(ctx, sub.ID)
		if err != nil {
			return err
		}
		if unresolved {
			return fmt.Errorf("%w: payment requires reconciliation", ErrInvalidRequest)
		}
		sub.CancelAtPeriodEnd = cancel
		if cancel {
			sub.PendingPlanID = pgtype.Int8{}
		}
		return saveSubscription(ctx, q, sub)
	})
}

// ProcessRenewals is a periodic queue job. Only payment subscriptions are discovered.
func (s *Service) ProcessRenewals(ctx context.Context, now time.Time, policy RenewalPolicy) error {
	if err := policy.Validate(); err != nil {
		return err
	}
	ids, err := s.q.ListDuePaymentSubscriptions(ctx, stamp(now))
	if err != nil {
		return err
	}
	errs := []error{}
	for _, id := range ids {
		if err := s.renewOne(ctx, id, now, policy); err != nil {
			errs = append(errs, fmt.Errorf("subscription %d: %w", id, err))
		}
	}
	return errors.Join(errs...)
}

func (s *Service) renewOne(ctx context.Context, id int64, now time.Time, policy RenewalPolicy) error {
	var claim *db.SubscriptionPaymentAttempt
	var input payment.SavedCardChargeInput
	var provider string
	err := s.transactions.WithTx(ctx, func(q *db.Queries) error {
		sub, err := q.LockRenewalSubscription(ctx, id)
		if err != nil {
			return err
		}
		eligible := false
		switch sub.Status {
		case db.SubscriptionStatusActive, db.SubscriptionStatusTrialing, db.SubscriptionStatusPastDue:
			eligible = true
		}
		if !eligible || !sub.NextBillingAt.Valid || sub.NextBillingAt.Time.After(now) {
			return nil
		}
		if sub.CancelAtPeriodEnd {
			return endSubscription(ctx, q, sub, now, "subscription.canceled")
		}
		if sub.GraceUntil.Valid && !sub.GraceUntil.Time.After(now) {
			return endSubscription(ctx, q, sub, now, "subscription.expired")
		}
		if sub.RetryAt.Valid && sub.RetryAt.Time.After(now) {
			return nil
		}
		unresolved, err := q.HasUnresolvedSubscriptionCharge(ctx, id)
		if err != nil {
			return err
		}
		if unresolved {
			// A pending upgrade may outlive its original period. Do not charge around it;
			// bound entitlement retention even when reconciliation has not happened.
			if !sub.GraceUntil.Valid {
				sub.GraceUntil = stamp(now.AddDate(0, 0, policy.GraceDays))
				sub.DunningStartedAt = stamp(now)
				sub.RetryAt = sub.GraceUntil
				sub.Status = db.SubscriptionStatusPastDue
				if err := billingEvent(ctx, q, sub, "subscription.past_due"); err != nil {
					return err
				}
				return saveSubscription(ctx, q, sub)
			}
			return nil
		}
		planID := sub.PlanID
		if sub.PendingPlanID.Valid {
			planID = sub.PendingPlanID.Int64
		}
		plan, err := q.GetPlanByID(ctx, planID)
		if err != nil {
			return err
		}
		if plan.BillingInterval == db.BillingIntervalFree {
			return endSubscription(ctx, q, sub, now, "subscription.canceled")
		}
		if !sub.GraceUntil.Valid {
			sub.GraceUntil = stamp(now.AddDate(0, 0, policy.GraceDays))
			sub.DunningStartedAt = stamp(now)
		}
		sub.RetryAt = sub.GraceUntil
		sub.RenewalAttempts++
		attempt, err := q.CreateSubscriptionPaymentAttempt(ctx, db.CreateSubscriptionPaymentAttemptParams{
			AccountID: sub.AccountID, SubscriptionID: id, PeriodAt: sub.NextBillingAt, Attempt: sub.RenewalAttempts,
			Kind: "renewal", PlanID: plan.ID, AmountMinor: plan.PriceMinor, Currency: plan.Currency,
		})
		if err != nil {
			return err
		}
		claim = &attempt
		card, cardErr := q.RenewalCard(ctx, db.RenewalCardParams{
			AccountID: sub.AccountID, Provider: sub.Provider.PaymentProvider, SavedCardID: sub.SavedCardID,
		})
		if cardErr != nil && !errors.Is(cardErr, pgx.ErrNoRows) {
			return cardErr
		}
		provider = string(sub.Provider.PaymentProvider)
		input = payment.SavedCardChargeInput{
			MerchantOID: "ren" + attempt.Uuid.String(), AmountMinor: plan.PriceMinor, Currency: plan.Currency,
			UserEmail: card.Email, UserName: card.Name, UToken: card.ProviderCustomerRef, CToken: card.ProviderCardRef,
			PlanName: plan.Name, ClientIP: "127.0.0.1",
		}
		return saveSubscription(ctx, q, sub)
	})
	if err != nil || claim == nil {
		return err
	}
	result := payment.CallbackResult{}
	var chargeErr error
	if input.CToken == "" || input.UToken == "" {
		chargeErr = payment.ErrRecurringUnsupported
	} else {
		result, chargeErr = s.recurring.ChargeRecurring(ctx, provider, input)
	}
	// The claim is already durable. Even a crash/timeout here cannot cause a second charge.
	return s.settleRenewal(ctx, *claim, result, chargeErr, now, policy)
}

func (s *Service) settleRenewal(
	ctx context.Context, attempt db.SubscriptionPaymentAttempt, result payment.CallbackResult,
	chargeErr error, now time.Time, policy RenewalPolicy,
) error {
	return s.transactions.WithTx(ctx, func(q *db.Queries) error {
		sub, err := q.LockRenewalSubscription(ctx, attempt.SubscriptionID)
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
			ID: attempt.ID, Status: status, ProviderRef: textOpt(result.ProviderRef), FailureCode: textOpt(code),
		}); err != nil {
			return err
		}
		if status == "succeeded" {
			plan, err := q.GetPlanByID(ctx, attempt.PlanID)
			if err != nil {
				return err
			}
			sub.Status = db.SubscriptionStatusActive
			sub.PlanID = plan.ID
			sub.PendingPlanID = pgtype.Int8{}
			sub.PeriodStartedAt = stamp(now)
			sub.NextBillingAt = stamp(advancePeriod(now, plan.BillingInterval))
			sub.EndsAt = pgtype.Timestamptz{}
			sub.GraceUntil = pgtype.Timestamptz{}
			sub.DunningStartedAt = pgtype.Timestamptz{}
			sub.RetryAt = pgtype.Timestamptz{}
			sub.RenewalAttempts = 0
			if err := billingEvent(ctx, q, sub, "payment.succeeded"); err != nil {
				return err
			}
			if err := billingEvent(ctx, q, sub, "subscription.renewed"); err != nil {
				return err
			}
		} else {
			first := sub.Status != db.SubscriptionStatusPastDue
			sub.Status = db.SubscriptionStatusPastDue
			sub.RetryAt = sub.GraceUntil
			n := int(sub.RenewalAttempts) - 1
			if n < len(policy.RetryDays) && status != "unknown" {
				firstFailure := sub.DunningStartedAt.Time
				sub.RetryAt = stamp(firstFailure.AddDate(0, 0, policy.RetryDays[n]))
				if !sub.RetryAt.Time.After(now) {
					sub.RetryAt = stamp(now.Add(time.Minute))
				}
			}
			if err := billingEvent(ctx, q, sub, "payment.failed"); err != nil {
				return err
			}
			if first {
				if err := billingEvent(ctx, q, sub, "subscription.past_due"); err != nil {
					return err
				}
			}
		}
		return saveSubscription(ctx, q, sub)
	})
}

func endSubscription(
	ctx context.Context,
	q *db.Queries,
	sub db.Subscription,
	now time.Time,
	event string,
) error {
	sub.Status = db.SubscriptionStatusCanceled
	sub.EndsAt = sub.NextBillingAt
	if event == "subscription.expired" && sub.GraceUntil.Valid {
		sub.EndsAt = sub.GraceUntil
	}
	sub.CanceledAt = stamp(now)
	sub.RetryAt = pgtype.Timestamptz{}
	sub.PendingPlanID = pgtype.Int8{}
	if err := saveSubscription(ctx, q, sub); err != nil {
		return err
	}
	return billingEvent(ctx, q, sub, event)
}

func stamp(t time.Time) pgtype.Timestamptz { return pgtype.Timestamptz{Time: t.UTC(), Valid: true} }
func advancePeriod(t time.Time, interval db.BillingInterval) time.Time {
	months := 1
	if interval == db.BillingIntervalYear {
		months = 12
	}
	// Clamp month-end dates (Jan 31 -> Feb 28/29), preserving time of day.
	first := time.Date(t.Year(), t.Month(), 1, t.Hour(), t.Minute(), t.Second(), t.Nanosecond(), time.UTC)
	target := first.AddDate(0, months, 0)
	last := target.AddDate(0, 1, -1).Day()
	day := t.Day()
	if day > last {
		day = last
	}
	return target.AddDate(0, 0, day-1)
}

// proratedDifference rounds up to the nearest minor unit using exact integer arithmetic.
func proratedDifference(difference int64, remaining, total time.Duration) int64 {
	if remaining <= 0 || total <= 0 {
		return 0
	}
	if remaining > total {
		remaining = total
	}
	n := new(big.Int).Mul(big.NewInt(difference), big.NewInt(int64(remaining)))
	n.Add(n, big.NewInt(int64(total)-1))
	n.Div(n, big.NewInt(int64(total)))
	return n.Int64()
}
