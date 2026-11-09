package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/database/db"
	ordersmodel "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/orders/model"
	ordersusecase "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/orders/usecase"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/authctx"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/payment"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// ErrOrderNotPayable is returned when an order checkout is started for an order that is not
// pending payment (already paid, cancelled, or nothing to charge).
var ErrOrderNotPayable = errors.New("order is not payable")

// orderMerchantPrefix marks order payments (TEC-943) in merchant_oid / iyzico conversationId.
const orderMerchantPrefix = "ord"

// OrderPayments is the orders module as seen by order checkout: every order status change goes
// through it, so order_status_history and notifications stay in one place.
type OrderPayments interface {
	PaymentTarget(ctx context.Context, account authctx.Account, id uuid.UUID) (*ordersmodel.PaymentTarget, error)
	ApplyProviderPayment(ctx context.Context, q *db.Queries, orderID int64, provider, reference string) (ordersmodel.ProviderPaymentOutcome, error)
	PublishProviderPayment(ctx context.Context, orderUUID uuid.UUID)
	// ApplyProviderRefund and PublishProviderRefund record a succeeded provider refund (TEC-935).
	ApplyProviderRefund(ctx context.Context, q *db.Queries, orderID int64, full bool, actorUserID *int64) (uuid.UUID, error)
	PublishProviderRefund(ctx context.Context, orderUUID uuid.UUID) error
}

// SetOrderPayments enables order checkout and order payment callbacks.
func (s *Service) SetOrderPayments(o OrderPayments) { s.orders = o }

type OrderCheckoutView struct {
	UUID               uuid.UUID `json:"uuid"`
	Status             string    `json:"status"`
	Provider           string    `json:"provider"`
	OrderUUID          uuid.UUID `json:"order_uuid"`
	OrderNumber        string    `json:"order_number"`
	OrderStatus        string    `json:"order_status"`
	OrderPaymentStatus string    `json:"order_payment_status"`
	AmountMinor        int64     `json:"amount_minor"`
	Currency           string    `json:"currency"`
	IframeToken        *string   `json:"iframe_token,omitempty"`
	CheckoutURL        *string   `json:"checkout_url,omitempty"`
	ExpiresAt          string    `json:"expires_at"`
	CompletedAt        *string   `json:"completed_at,omitempty"`
}

// StartOrderCheckout starts a hosted checkout for a pending order of the active account. Each
// call creates a new session, so a failed or abandoned attempt can simply be retried.
func (s *Service) StartOrderCheckout(ctx context.Context, payer Payer, orderUUID uuid.UUID, clientIP string) (OrderCheckoutView, error) {
	if s.orders == nil {
		return OrderCheckoutView{}, payment.ErrNotConfigured
	}
	target, err := s.orders.PaymentTarget(ctx, payer.Account, orderUUID)
	switch {
	case errors.Is(err, ordersusecase.ErrNotFound):
		return OrderCheckoutView{}, ErrNotFound
	case errors.Is(err, ordersusecase.ErrForbidden):
		return OrderCheckoutView{}, ErrForbidden
	case errors.Is(err, ordersusecase.ErrNotPayable):
		return OrderCheckoutView{}, ErrOrderNotPayable
	case err != nil:
		return OrderCheckoutView{}, err
	}
	currency := strings.ToUpper(strings.TrimSpace(target.Currency))
	if currency == "TL" {
		currency = "TRY"
	}
	if currency != "TRY" {
		return OrderCheckoutView{}, fmt.Errorf("%w: only TRY orders can be paid online", ErrInvalidRequest)
	}
	provider, err := s.registry.ActiveProvider(ctx)
	if err != nil {
		s.log.Error("order_checkout_provider_unavailable", "order_uuid", orderUUID, "error", err)
		return OrderCheckoutView{}, err
	}
	testMode, err := s.registry.TestMode(ctx)
	if err != nil {
		return OrderCheckoutView{}, err
	}

	merchantOID := orderMerchantPrefix + strings.ReplaceAll(uuid.NewString(), "-", "")
	session, err := s.q.CreateOrderPaymentSession(ctx, db.CreateOrderPaymentSessionParams{
		AccountID:   payer.Account.ID,
		OrderID:     target.ID,
		UserID:      payer.UserID,
		Provider:    db.PaymentProvider(provider.Slug()),
		MerchantOid: merchantOID,
		AmountMinor: target.AmountMinor,
		Currency:    currency,
		ExpiresAt:   pgtype.Timestamptz{Time: time.Now().UTC().Add(30 * time.Minute), Valid: true},
	})
	if err != nil {
		return OrderCheckoutView{}, err
	}
	resultPath := fmt.Sprintf("%s/orders/checkout/%s", s.frontendURL, session.Uuid)
	callbackURL := fmt.Sprintf("%s/v1/public/payment/%s/callback", s.apiBaseURL, provider.Slug())
	s.log.Info("order_checkout_session_created",
		"session_uuid", session.Uuid,
		"order_uuid", target.UUID,
		"merchant_oid", merchantOID,
		"provider", provider.Slug(),
		"amount_minor", target.AmountMinor,
		"test_mode", testMode,
	)
	init, err := provider.InitCheckout(ctx, payment.CheckoutInput{
		SessionUUID: session.Uuid.String(),
		MerchantOID: merchantOID,
		AmountMinor: target.AmountMinor,
		Currency:    currency,
		PlanName:    "ComCode " + target.OrderNumber,
		UserEmail:   payer.Email,
		UserName:    payer.Name,
		ClientIP:    clientIP,
		SuccessURL:  resultPath + "?result=success",
		FailURL:     resultPath + "?result=fail",
		CallbackURL: callbackURL,
		TestMode:    testMode,
		Physical:    true,
	})
	if err != nil {
		s.log.Error("order_checkout_provider_init_failed", "session_uuid", session.Uuid, "provider", provider.Slug(), "error", err)
		_, _ = s.q.FailOrderPaymentSession(ctx, db.FailOrderPaymentSessionParams{ID: session.ID})
		return OrderCheckoutView{}, err
	}
	token := init.IframeToken
	if token == "" {
		token = init.ProviderToken
	}
	if _, err := s.q.UpdateOrderPaymentSessionInit(ctx, db.UpdateOrderPaymentSessionInitParams{
		Uuid:          session.Uuid,
		ProviderToken: textOpt(token),
		CheckoutUrl:   textOpt(init.CheckoutURL),
	}); err != nil {
		return OrderCheckoutView{}, err
	}
	return s.GetOrderCheckout(ctx, payer, session.Uuid)
}

// GetOrderCheckout reads an order checkout session of the active account. A pending session past
// its expiry reads as expired; a late successful callback still pays the order.
func (s *Service) GetOrderCheckout(ctx context.Context, payer Payer, sessionUUID uuid.UUID) (OrderCheckoutView, error) {
	row, err := s.q.GetOrderPaymentSessionByUUID(ctx, sessionUUID)
	if errors.Is(err, pgx.ErrNoRows) {
		return OrderCheckoutView{}, ErrNotFound
	}
	if err != nil {
		return OrderCheckoutView{}, err
	}
	if row.AccountID != payer.Account.ID {
		return OrderCheckoutView{}, ErrNotFound
	}
	view := OrderCheckoutView{
		UUID: row.Uuid, Status: string(row.Status), Provider: string(row.Provider),
		OrderUUID: row.OrderUuid, OrderNumber: row.OrderNumber, OrderStatus: row.OrderStatus,
		OrderPaymentStatus: row.OrderPaymentStatus, AmountMinor: row.AmountMinor, Currency: row.Currency,
		ExpiresAt: row.ExpiresAt.Time.Format(time.RFC3339),
	}
	if row.Status == db.CheckoutSessionStatusPending {
		if row.ExpiresAt.Time.Before(time.Now().UTC()) {
			view.Status = string(db.CheckoutSessionStatusExpired)
		} else if row.ProviderToken.Valid && row.ProviderToken.String != "" {
			// The iframe token / checkout URL only matter while the session can still be paid.
			if row.Provider == db.PaymentProviderPaytr {
				v := row.ProviderToken.String
				view.IframeToken = &v
			}
			if row.CheckoutUrl.Valid && row.CheckoutUrl.String != "" {
				v := row.CheckoutUrl.String
				view.CheckoutURL = &v
			}
		}
	}
	if row.CompletedAt.Valid {
		v := row.CompletedAt.Time.Format(time.RFC3339)
		view.CompletedAt = &v
	}
	return view, nil
}

func isOrderPayment(merchantOID string) bool {
	return strings.HasPrefix(merchantOID, orderMerchantPrefix)
}

// finalizeOrderPayment applies a verified provider callback to an order checkout session. The
// session and the order change in one transaction under row locks, so duplicate or concurrent
// callbacks are no-ops once the session left pending. A failure only fails the session: the
// order stays pending and can be paid again.
func (s *Service) finalizeOrderPayment(ctx context.Context, result payment.CallbackResult, payload map[string]string, providerSlug string) (uuid.UUID, error) {
	if s.orders == nil {
		return uuid.Nil, payment.ErrNotConfigured
	}
	var sessionUUID, paidOrder uuid.UUID
	err := s.transactions.WithTx(ctx, func(q *db.Queries) error {
		row, err := q.LockOrderPaymentSession(ctx, result.MerchantOID)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		sessionUUID = row.Uuid
		if string(row.Provider) != providerSlug {
			return fmt.Errorf("%w: provider mismatch", ErrInvalidRequest)
		}
		if row.Status != db.CheckoutSessionStatusPending {
			s.log.Info("order_payment_already_final", "session_uuid", row.Uuid, "status", row.Status)
			return nil
		}
		metadata := map[string]any{}
		if err := json.Unmarshal(row.Metadata, &metadata); err != nil {
			return err
		}
		if !result.Success {
			if payload != nil {
				metadata["failed_reason_code"] = payload["failed_reason_code"]
				metadata["failed_reason_msg"] = payload["failed_reason_msg"]
			}
			raw, err := json.Marshal(metadata)
			if err != nil {
				return err
			}
			s.log.Warn("order_payment_failed", "session_uuid", row.Uuid, "merchant_oid", row.MerchantOid)
			_, err = q.FailOrderPaymentSession(ctx, db.FailOrderPaymentSessionParams{ID: row.ID, Metadata: raw})
			return err
		}
		if providerSlug == "paytr" {
			if err := checkPayTRAmounts(payload, row.AmountMinor, row.Currency); err != nil {
				return err
			}
		}
		reference := result.ProviderRef
		if reference == "" {
			reference = row.MerchantOid
		}
		outcome, err := s.orders.ApplyProviderPayment(ctx, q, row.OrderID, providerSlug, reference)
		if err != nil {
			return err
		}
		metadata["order_result"] = string(outcome.Result)
		raw, err := json.Marshal(metadata)
		if err != nil {
			return err
		}
		if _, err := q.CompleteOrderPaymentSession(ctx, db.CompleteOrderPaymentSessionParams{
			ID: row.ID, ProviderRef: textOpt(result.ProviderRef), Metadata: raw,
		}); err != nil {
			return err
		}
		if outcome.Result == ordersmodel.ProviderPaymentApplied {
			paidOrder = outcome.OrderUUID
		} else {
			// Money was captured but the order did not need it; it has to be refunded at the
			// provider (TEC-935).
			s.log.Error("order_payment_needs_refund",
				"session_uuid", row.Uuid,
				"merchant_oid", row.MerchantOid,
				"order_result", outcome.Result,
				"provider", providerSlug,
			)
		}
		return nil
	})
	if err != nil {
		return sessionUUID, err
	}
	if paidOrder != uuid.Nil {
		s.log.Info("order_payment_applied", "session_uuid", sessionUUID, "order_uuid", paidOrder)
		s.orders.PublishProviderPayment(ctx, paidOrder)
	}
	return sessionUUID, nil
}

// checkPayTRAmounts validates the unsigned PayTR success fields against the stored session (see
// docs/BILLING.md): the signed total may exceed the price for installment fees, never undercut it.
func checkPayTRAmounts(payload map[string]string, amountMinor int64, currency string) error {
	total, _ := strconv.ParseInt(payload["total_amount"], 10, 64) // Already validated with the callback hash.
	if total < amountMinor {
		return fmt.Errorf("%w: paytr amount mismatch", ErrInvalidRequest)
	}
	amount, err := strconv.ParseInt(payload["payment_amount"], 10, 64)
	if err != nil || amount != amountMinor {
		return fmt.Errorf("%w: paytr order amount mismatch", ErrInvalidRequest)
	}
	got := payload["currency"]
	if got == "TL" {
		got = "TRY"
	}
	if got != currency {
		return fmt.Errorf("%w: paytr currency mismatch", ErrInvalidRequest)
	}
	return nil
}
