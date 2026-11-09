package usecase

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/database/db"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/orders/repository"
)

var (
	// ErrRefundFailed means the payment provider did not refund; the order was not changed and
	// the refund can be retried (TEC-935).
	ErrRefundFailed = errors.New("provider refund failed")
	// ErrRefundInProgress means another refund of the same payment has not finished yet.
	ErrRefundInProgress = errors.New("a refund of this payment is in progress")
)

// PaymentRefunder refunds an order's online payment at the payment provider (TEC-935, billing
// module).
type PaymentRefunder interface {
	// RefundOrderRemaining refunds whatever is still refundable of the payment that paid the
	// order. refunded is false when the order has no online payment left to refund.
	RefundOrderRemaining(ctx context.Context, orderUUID uuid.UUID) (refunded bool, err error)
}

// SetPaymentRefunder makes moving an online-paid order to refunded refund the money at the
// provider first.
func (s *Service) SetPaymentRefunder(r PaymentRefunder) { s.refunder = r }

// refundAtProvider runs before a status change to refunded: the provider refund must succeed
// first, so a failed refund leaves the order exactly as it was.
func (s *Service) refundAtProvider(ctx context.Context, id uuid.UUID) error {
	if s.refunder == nil {
		return nil
	}
	_, err := s.refunder.RefundOrderRemaining(ctx, id)
	return err
}

// ApplyProviderRefund records a succeeded provider refund of the order's payment inside the
// caller's transaction q; full means nothing of the payment is left. Call PublishProviderRefund
// after the commit.
func (s *Service) ApplyProviderRefund(ctx context.Context, q *db.Queries, orderID int64, full bool, actorUserID *int64) (uuid.UUID, error) {
	return repository.NewPostgres(nil, q).ApplyProviderRefund(ctx, orderID, full, actorUserID)
}

// PublishProviderRefund runs the side effects of a committed refund: a fully refunded order's
// card routes are retired (TEC-585), milestone notifications are sent and the order is reindexed.
func (s *Service) PublishProviderRefund(ctx context.Context, orderUUID uuid.UUID) error {
	ord, err := s.repo.GetByUUID(ctx, orderUUID)
	if err != nil || ord == nil {
		return err
	}
	s.notifyOrderTransitions(ctx, ord)
	if isClosedOrder(ord) {
		if err := s.retireCardRoutes(ctx, ord.ID); err != nil {
			return err
		}
	}
	_, _ = s.indexed(ord, nil)
	return nil
}
