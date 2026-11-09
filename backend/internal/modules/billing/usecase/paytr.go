package usecase

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/database/db"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/payment"
)

func (s *Service) finalizePayTRCheckout(ctx context.Context, result payment.CallbackResult, payload map[string]string) error {
	return s.transactions.WithTx(ctx, func(q *db.Queries) error {
		row, err := q.LockCheckoutSession(ctx, result.MerchantOID)
		if err != nil {
			return err
		}
		if row.Provider != db.PaymentProviderPaytr {
			return fmt.Errorf("%w: provider mismatch", ErrInvalidRequest)
		}
		if row.Status != db.CheckoutSessionStatusPending {
			return nil
		}
		// Serializes different checkouts for the same account as well as duplicate notifications.
		if _, err := q.LockBillingAccount(ctx, row.AccountID); err != nil {
			return err
		}
		if !result.Success {
			metadata := map[string]any{}
			if err := json.Unmarshal(row.Metadata, &metadata); err != nil {
				return err
			}
			metadata["failed_reason_code"] = payload["failed_reason_code"]
			metadata["failed_reason_msg"] = payload["failed_reason_msg"]
			raw, err := json.Marshal(metadata)
			if err != nil {
				return err
			}
			_, err = q.FailCheckoutSession(ctx, db.FailCheckoutSessionParams{Uuid: row.Uuid, Metadata: raw})
			return err
		}
		if err := checkPayTRAmounts(payload, row.AmountMinor, row.Currency); err != nil {
			return err
		}
		scoped := *s
		scoped.q = q
		if err := scoped.finalizeCheckout(ctx, result.MerchantOID, true, result.ProviderRef, "paytr"); err != nil {
			return err
		}
		if result.UToken != "" && result.CToken != "" {
			return scoped.persistSavedCard(ctx, row.AccountID, row.UserID, "paytr", result.UToken, result.CToken, result.CardLast4, result.CardBrand, true)
		}
		return nil
	})
}
