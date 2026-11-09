package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/database/db"
	ordersmodel "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/orders/model"
	ordersusecase "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/orders/usecase"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/authctx"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/payment"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
)

// TEC-935: refunds of online card order payments through the provider's refund API. A refund row
// is committed as pending before the provider is called, and the order only changes after the
// provider confirmed the refund, so a failed call leaves the order untouched and the refund
// failed (retryable) instead of half-refunded.

// ErrNothingToRefund means the order has no online payment with a refundable balance.
var ErrNothingToRefund = errors.New("order has no online payment to refund")

// errReplay ends a refund transaction whose idempotency key already has a refund.
var errReplay = errors.New("idempotent replay")

// refundReferencePrefix starts the reference sent to the provider (PayTR reference_no is
// alphanumeric, at most 64 characters).
const refundReferencePrefix = "rfd"

// Refund error codes stored on a failed refund.
const (
	// RefundErrorRejected: the provider answered and refunded nothing; retrying is safe.
	RefundErrorRejected = "provider_rejected"
	// RefundErrorUnknown: the call failed without an answer; check the provider panel first.
	RefundErrorUnknown = "provider_error"
	// RefundErrorNotConfigured: the payment's provider has no usable credentials.
	RefundErrorNotConfigured = "provider_not_configured"
)

// activityRecorder writes the platform activity log (platform/activity.Recorder).
type activityRecorder interface {
	Record(ctx context.Context, actorID *int64, action, resource string, resourceUUID *uuid.UUID, payload map[string]any, r *http.Request)
}

// SetActivity records refunds in the activity log.
func (s *Service) SetActivity(a activityRecorder) { s.activity = a }

// RefundOrderInput asks for a refund of an order payment. Without PaymentUUID the payment that
// paid the order is refunded; without AmountMinor everything still refundable is.
type RefundOrderInput struct {
	PaymentUUID    *uuid.UUID `json:"payment_uuid,omitempty"`
	AmountMinor    *int64     `json:"amount_minor,omitempty"`
	Reason         string     `json:"reason"`
	IdempotencyKey string     `json:"idempotency_key"`
}

type OrderRefundView struct {
	UUID         uuid.UUID `json:"uuid"`
	PaymentUUID  uuid.UUID `json:"payment_uuid"`
	Provider     string    `json:"provider"`
	Status       string    `json:"status"`
	AmountMinor  int64     `json:"amount_minor"`
	Currency     string    `json:"currency"`
	Reason       string    `json:"reason"`
	ProviderRef  *string   `json:"provider_ref,omitempty"`
	ErrorCode    *string   `json:"error_code,omitempty"`
	ErrorMessage *string   `json:"error_message,omitempty"`
	Attempts     int32     `json:"attempts"`
	CreatedAt    string    `json:"created_at"`
	CompletedAt  *string   `json:"completed_at,omitempty"`
}

// OrderRefundablePayment is a completed online payment of an order and what is left of it.
type OrderRefundablePayment struct {
	UUID            uuid.UUID `json:"uuid"`
	Provider        string    `json:"provider"`
	ProviderRef     *string   `json:"provider_ref,omitempty"`
	AmountMinor     int64     `json:"amount_minor"`
	RefundedMinor   int64     `json:"refunded_minor"`
	PendingMinor    int64     `json:"pending_minor"`
	RefundableMinor int64     `json:"refundable_minor"`
	Currency        string    `json:"currency"`
	CompletedAt     *string   `json:"completed_at,omitempty"`
	// PaidOrder is true for the payment that paid the order. Other completed payments arrived
	// after the order was paid or closed (docs/BILLING.md) and only need their money returned.
	PaidOrder bool `json:"paid_order"`
}

type OrderRefundsView struct {
	Payments []OrderRefundablePayment `json:"payments"`
	Refunds  []OrderRefundView        `json:"refunds"`
}

// ListOrderRefunds returns an order's online payments and their refunds (platform).
func (s *Service) ListOrderRefunds(ctx context.Context, orderUUID uuid.UUID) (OrderRefundsView, error) {
	order, err := s.refundOrder(ctx, s.q, orderUUID)
	if err != nil {
		return OrderRefundsView{}, err
	}
	payments, err := s.q.ListOrderPaymentsForRefund(ctx, order.ID)
	if err != nil {
		return OrderRefundsView{}, err
	}
	refunds, err := s.q.ListOrderPaymentRefunds(ctx, order.ID)
	if err != nil {
		return OrderRefundsView{}, err
	}
	out := OrderRefundsView{Payments: make([]OrderRefundablePayment, 0, len(payments)), Refunds: make([]OrderRefundView, 0, len(refunds))}
	for _, p := range payments {
		v := OrderRefundablePayment{
			UUID: p.Uuid, Provider: string(p.Provider), ProviderRef: textPtr(p.ProviderRef),
			AmountMinor: p.AmountMinor, RefundedMinor: p.RefundedMinor, PendingMinor: p.PendingMinor,
			RefundableMinor: max(p.AmountMinor-p.RefundedMinor-p.PendingMinor, 0), Currency: p.Currency,
			CompletedAt: timePtr(p.CompletedAt), PaidOrder: p.OrderResult == string(ordersmodel.ProviderPaymentApplied),
		}
		out.Payments = append(out.Payments, v)
	}
	for _, r := range refunds {
		out.Refunds = append(out.Refunds, refundView(r.OrderPaymentRefund, r.SessionUuid))
	}
	return out, nil
}

// RefundOrder refunds an online payment of the order at its provider (platform action). Repeating
// a request with the same idempotency key returns the first refund without calling the provider
// again; created is false then. A provider failure returns the failed refund together with an
// error wrapping ordersusecase.ErrRefundFailed: the order is unchanged and the refund can be
// retried with RetryOrderRefund.
func (s *Service) RefundOrder(ctx context.Context, orderUUID uuid.UUID, in RefundOrderInput, clientIP string) (view OrderRefundView, created bool, err error) {
	if s.orders == nil {
		return OrderRefundView{}, false, payment.ErrNotConfigured
	}
	key := strings.TrimSpace(in.IdempotencyKey)
	if key == "" || len(key) > 128 {
		return OrderRefundView{}, false, fmt.Errorf("%w: idempotency_key is required (max 128 characters)", ErrInvalidRequest)
	}
	reason := strings.TrimSpace(in.Reason)
	if len(reason) > 500 {
		return OrderRefundView{}, false, fmt.Errorf("%w: reason is too long", ErrInvalidRequest)
	}
	order, err := s.refundOrder(ctx, s.q, orderUUID)
	if err != nil {
		return OrderRefundView{}, false, err
	}
	if existing, ok, err := s.refundByKey(ctx, order.ID, key); err != nil || ok {
		return existing, false, err
	}
	target, err := s.refundSession(ctx, order.ID, in.PaymentUUID)
	if err != nil {
		return OrderRefundView{}, false, err
	}

	var session db.OrderPaymentSession
	var refund db.OrderPaymentRefund
	err = s.transactions.WithTx(ctx, func(q *db.Queries) error {
		locked, err := q.LockOrderPaymentSessionByID(ctx, target.ID)
		if err != nil {
			return err
		}
		session = locked
		// A concurrent request with the same key may have committed while this one waited for
		// the lock.
		if _, err := q.GetOrderPaymentRefundByIdempotencyKey(ctx, key); err == nil {
			return errReplay
		} else if !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		refundable, err := refundableMinor(ctx, q, locked)
		if err != nil {
			return err
		}
		amount := refundable
		if in.AmountMinor != nil {
			amount = *in.AmountMinor
		}
		if amount <= 0 || amount > refundable {
			return fmt.Errorf("%w: amount_minor must be between 1 and %d", ErrInvalidRequest, refundable)
		}
		refund, err = q.CreateOrderPaymentRefund(ctx, db.CreateOrderPaymentRefundParams{
			AccountID:         locked.AccountID,
			OrderID:           locked.OrderID,
			SessionID:         locked.ID,
			Provider:          locked.Provider,
			AmountMinor:       amount,
			Currency:          locked.Currency,
			ReferenceNo:       refundReferencePrefix + strings.ReplaceAll(uuid.NewString(), "-", ""),
			IdempotencyKey:    key,
			Reason:            reason,
			RequestedByUserID: actorInt8(ctx),
		})
		return err
	})
	if errors.Is(err, errReplay) {
		existing, _, err := s.refundByKey(ctx, order.ID, key)
		return existing, false, err
	}
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			// A concurrent request with the same key won the insert; return its refund.
			if pgErr.ConstraintName == "uq_order_payment_refunds_idempotency_key" {
				existing, ok, lookupErr := s.refundByKey(ctx, order.ID, key)
				if lookupErr == nil && ok {
					return existing, false, nil
				}
			}
			return OrderRefundView{}, false, ordersusecase.ErrRefundInProgress
		}
		return OrderRefundView{}, false, err
	}
	s.log.Info("order_refund_created", "refund_uuid", refund.Uuid, "order_uuid", order.Uuid, "session_uuid", session.Uuid, "amount_minor", refund.AmountMinor)
	view, err = s.executeRefund(ctx, order, session, refund, clientIP)
	return view, true, err
}

// RetryOrderRefund sends a failed refund to the provider again, with the same reference. A
// refund that already succeeded is returned unchanged.
func (s *Service) RetryOrderRefund(ctx context.Context, orderUUID, refundUUID uuid.UUID, clientIP string) (OrderRefundView, error) {
	if s.orders == nil {
		return OrderRefundView{}, payment.ErrNotConfigured
	}
	order, err := s.refundOrder(ctx, s.q, orderUUID)
	if err != nil {
		return OrderRefundView{}, err
	}
	current, err := s.q.GetOrderPaymentRefundByUUID(ctx, refundUUID)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && current.OrderID != order.ID) {
		return OrderRefundView{}, ErrNotFound
	}
	if err != nil {
		return OrderRefundView{}, err
	}
	var session db.OrderPaymentSession
	var refund db.OrderPaymentRefund
	err = s.transactions.WithTx(ctx, func(q *db.Queries) error {
		session, err = q.LockOrderPaymentSessionByID(ctx, current.SessionID)
		if err != nil {
			return err
		}
		refund, err = q.LockOrderPaymentRefund(ctx, current.ID)
		if err != nil {
			return err
		}
		switch refund.Status {
		case db.OrderRefundStatusSucceeded:
			return nil
		case db.OrderRefundStatusPending:
			return ordersusecase.ErrRefundInProgress
		}
		refundable, err := refundableMinor(ctx, q, session)
		if err != nil {
			return err
		}
		if refund.AmountMinor > refundable {
			return fmt.Errorf("%w: only %d is still refundable", ErrInvalidRequest, refundable)
		}
		refund, err = q.RetryOrderPaymentRefund(ctx, db.RetryOrderPaymentRefundParams{ID: refund.ID, RequestedByUserID: actorInt8(ctx)})
		return err
	})
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return OrderRefundView{}, ordersusecase.ErrRefundInProgress
		}
		return OrderRefundView{}, err
	}
	if refund.Status == db.OrderRefundStatusSucceeded {
		return refundView(refund, session.Uuid), nil
	}
	s.log.Info("order_refund_retry", "refund_uuid", refund.Uuid, "order_uuid", order.Uuid, "attempt", refund.Attempts)
	return s.executeRefund(ctx, order, session, refund, clientIP)
}

// RefundOrderRemaining refunds what is left of the payment that paid the order; the orders module
// calls it before moving an order to refunded (ordersusecase.PaymentRefunder). It returns false
// when there is nothing to refund online, e.g. an order paid by bank transfer.
func (s *Service) RefundOrderRemaining(ctx context.Context, orderUUID uuid.UUID) (bool, error) {
	if s.orders == nil {
		return false, nil
	}
	order, err := s.refundOrder(ctx, s.q, orderUUID)
	if errors.Is(err, ErrNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	payments, err := s.q.ListOrderPaymentsForRefund(ctx, order.ID)
	if err != nil {
		return false, err
	}
	for _, p := range payments {
		if p.OrderResult != string(ordersmodel.ProviderPaymentApplied) {
			continue
		}
		if p.PendingMinor > 0 {
			return false, ordersusecase.ErrRefundInProgress
		}
		if p.AmountMinor-p.RefundedMinor <= 0 {
			return false, nil
		}
		id := p.Uuid
		_, _, err := s.RefundOrder(ctx, orderUUID, RefundOrderInput{
			PaymentUUID:    &id,
			Reason:         "order moved to refunded",
			IdempotencyKey: "status-" + uuid.NewString(),
		}, "")
		return err == nil, err
	}
	return false, nil
}

// executeRefund calls the provider for a pending refund and records the result. On success the
// order is updated through the orders module in the same transaction as the refund row.
func (s *Service) executeRefund(ctx context.Context, order db.GetOrderPaymentAmountRow, session db.OrderPaymentSession, refund db.OrderPaymentRefund, clientIP string) (OrderRefundView, error) {
	result, callErr := s.callRefundProvider(ctx, session, refund, clientIP)

	var final db.OrderPaymentRefund
	var refundedOrder uuid.UUID
	var full bool
	err := s.transactions.WithTx(ctx, func(q *db.Queries) error {
		locked, err := q.LockOrderPaymentRefund(ctx, refund.ID)
		if err != nil {
			return err
		}
		if locked.Status != db.OrderRefundStatusPending {
			final = locked
			return nil
		}
		if callErr != nil {
			code := RefundErrorUnknown
			switch {
			case errors.Is(callErr, payment.ErrRefundRejected):
				code = RefundErrorRejected
			case errors.Is(callErr, payment.ErrNotConfigured):
				code = RefundErrorNotConfigured
			}
			final, err = q.MarkOrderPaymentRefundFailed(ctx, db.MarkOrderPaymentRefundFailedParams{
				ID: refund.ID, ErrorCode: pgtype.Text{String: code, Valid: true},
				ErrorMessage: pgtype.Text{String: truncate(callErr.Error(), 500), Valid: true},
			})
			return err
		}
		final, err = q.MarkOrderPaymentRefundSucceeded(ctx, db.MarkOrderPaymentRefundSucceededParams{
			ID: refund.ID, ProviderRef: textOpt(result.ProviderRef),
		})
		if err != nil {
			return err
		}
		if !isPaidOrderSession(session) {
			return nil
		}
		sums, err := q.SumOrderPaymentRefunds(ctx, session.ID)
		if err != nil {
			return err
		}
		full = sums.RefundedMinor >= session.AmountMinor
		refundedOrder, err = s.orders.ApplyProviderRefund(ctx, q, session.OrderID, full, actorID(ctx))
		return err
	})
	if err != nil {
		// The provider may have refunded, but the result could not be stored: the refund stays
		// pending and blocks further refunds of this payment until it is checked by hand.
		s.log.Error("order_refund_finalize_failed", "refund_uuid", refund.Uuid, "provider_error", callErr, "error", err)
		return OrderRefundView{}, err
	}
	view := refundView(final, session.Uuid)
	s.recordRefund(ctx, order, final, session, full)
	if callErr != nil {
		s.log.Error("order_refund_failed", "refund_uuid", refund.Uuid, "order_uuid", order.Uuid, "provider", session.Provider, "error", callErr)
		return view, fmt.Errorf("%w: %s", ordersusecase.ErrRefundFailed, callErr.Error())
	}
	s.log.Info("order_refund_succeeded", "refund_uuid", refund.Uuid, "order_uuid", order.Uuid, "provider", session.Provider, "full", full)
	if refundedOrder != uuid.Nil {
		if err := s.orders.PublishProviderRefund(ctx, refundedOrder); err != nil {
			s.log.Error("order_refund_publish_failed", "order_uuid", refundedOrder, "error", err)
		}
	}
	return view, nil
}

func (s *Service) callRefundProvider(ctx context.Context, session db.OrderPaymentSession, refund db.OrderPaymentRefund, clientIP string) (payment.RefundResult, error) {
	// The payment's own provider refunds it, even when another provider is active now.
	provider, err := s.registry.ProviderBySlug(ctx, string(session.Provider))
	if err != nil {
		return payment.RefundResult{}, err
	}
	return provider.Refund(ctx, payment.RefundInput{
		MerchantOID: session.MerchantOid,
		PaymentRef:  session.ProviderRef.String,
		ReferenceNo: refund.ReferenceNo,
		AmountMinor: refund.AmountMinor,
		Currency:    refund.Currency,
		ClientIP:    clientIP,
		Reason:      refund.Reason,
	})
}

func (s *Service) recordRefund(ctx context.Context, order db.GetOrderPaymentAmountRow, refund db.OrderPaymentRefund, session db.OrderPaymentSession, full bool) {
	if s.activity == nil {
		return
	}
	action := "order.refund_succeeded"
	payload := map[string]any{
		"order_number": order.OrderNumber,
		"refund_uuid":  refund.Uuid,
		"payment_uuid": session.Uuid,
		"provider":     refund.Provider,
		"amount_minor": refund.AmountMinor,
		"currency":     refund.Currency,
		"attempt":      refund.Attempts,
		"reason":       refund.Reason,
	}
	if refund.Status == db.OrderRefundStatusFailed {
		action = "order.refund_failed"
		payload["error_code"] = refund.ErrorCode.String
		payload["error_message"] = refund.ErrorMessage.String
	} else {
		payload["provider_ref"] = refund.ProviderRef.String
		payload["full"] = full
	}
	id := order.Uuid
	s.activity.Record(ctx, actorID(ctx), action, "orders", &id, payload, nil)
}

func (s *Service) refundOrder(ctx context.Context, q *db.Queries, orderUUID uuid.UUID) (db.GetOrderPaymentAmountRow, error) {
	order, err := q.GetOrderPaymentAmount(ctx, orderUUID)
	if errors.Is(err, pgx.ErrNoRows) {
		return order, ErrNotFound
	}
	return order, err
}

// refundByKey returns the refund created earlier with this idempotency key.
func (s *Service) refundByKey(ctx context.Context, orderID int64, key string) (OrderRefundView, bool, error) {
	existing, err := s.q.GetOrderPaymentRefundByIdempotencyKey(ctx, key)
	if errors.Is(err, pgx.ErrNoRows) {
		return OrderRefundView{}, false, nil
	}
	if err != nil {
		return OrderRefundView{}, false, err
	}
	if existing.OrderID != orderID {
		return OrderRefundView{}, false, fmt.Errorf("%w: idempotency_key was used for another order", ErrInvalidRequest)
	}
	session, err := s.q.GetOrderPaymentSessionByID(ctx, existing.SessionID)
	if err != nil {
		return OrderRefundView{}, false, err
	}
	return refundView(existing, session.Uuid), true, nil
}

// refundSession picks the payment to refund: the given one, or the one that paid the order.
func (s *Service) refundSession(ctx context.Context, orderID int64, paymentUUID *uuid.UUID) (db.ListOrderPaymentsForRefundRow, error) {
	payments, err := s.q.ListOrderPaymentsForRefund(ctx, orderID)
	if err != nil {
		return db.ListOrderPaymentsForRefundRow{}, err
	}
	for i := len(payments) - 1; i >= 0; i-- {
		p := payments[i]
		if paymentUUID != nil && p.Uuid == *paymentUUID {
			return p, nil
		}
		if paymentUUID == nil && p.OrderResult == string(ordersmodel.ProviderPaymentApplied) {
			return p, nil
		}
	}
	if paymentUUID != nil {
		return db.ListOrderPaymentsForRefundRow{}, ErrNotFound
	}
	return db.ListOrderPaymentsForRefundRow{}, ErrNothingToRefund
}

// refundableMinor is what is left of a locked payment after succeeded refunds. A pending refund
// of the same payment means another request is still talking to the provider.
func refundableMinor(ctx context.Context, q *db.Queries, session db.OrderPaymentSession) (int64, error) {
	sums, err := q.SumOrderPaymentRefunds(ctx, session.ID)
	if err != nil {
		return 0, err
	}
	if sums.PendingMinor > 0 {
		return 0, ordersusecase.ErrRefundInProgress
	}
	refundable := session.AmountMinor - sums.RefundedMinor
	if refundable <= 0 {
		return 0, ErrNothingToRefund
	}
	return refundable, nil
}

// isPaidOrderSession reports whether this payment paid the order (finalizeOrderPayment stores
// the outcome in metadata.order_result).
func isPaidOrderSession(session db.OrderPaymentSession) bool {
	var metadata struct {
		OrderResult string `json:"order_result"`
	}
	_ = json.Unmarshal(session.Metadata, &metadata)
	return metadata.OrderResult == string(ordersmodel.ProviderPaymentApplied)
}

func refundView(r db.OrderPaymentRefund, sessionUUID uuid.UUID) OrderRefundView {
	return OrderRefundView{
		UUID: r.Uuid, PaymentUUID: sessionUUID, Provider: string(r.Provider), Status: string(r.Status),
		AmountMinor: r.AmountMinor, Currency: r.Currency, Reason: r.Reason,
		ProviderRef: textPtr(r.ProviderRef), ErrorCode: textPtr(r.ErrorCode), ErrorMessage: textPtr(r.ErrorMessage),
		Attempts: r.Attempts, CreatedAt: r.CreatedAt.Time.Format(time.RFC3339), CompletedAt: timePtr(r.CompletedAt),
	}
}

func actorID(ctx context.Context) *int64 {
	if p, ok := authctx.PrincipalFrom(ctx); ok && p.UserInternal > 0 {
		id := p.UserInternal
		return &id
	}
	return nil
}

func actorInt8(ctx context.Context) pgtype.Int8 {
	if id := actorID(ctx); id != nil {
		return pgtype.Int8{Int64: *id, Valid: true}
	}
	return pgtype.Int8{}
}

func textPtr(t pgtype.Text) *string {
	if !t.Valid || t.String == "" {
		return nil
	}
	v := t.String
	return &v
}

func timePtr(t pgtype.Timestamptz) *string {
	if !t.Valid {
		return nil
	}
	v := t.Time.Format(time.RFC3339)
	return &v
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
