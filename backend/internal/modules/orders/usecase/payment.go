package usecase

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/database/db"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/orders/model"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/orders/repository"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/authctx"
)

// ErrNotPayable means the order cannot be paid online: it is no longer pending, already paid,
// closed, or has nothing to charge.
var ErrNotPayable = errors.New("order is not payable")

// PaymentTarget returns what an online checkout charges for an order of the active account
// (TEC-943). Only owners/admins of the account may pay; a pending order whose payment is unpaid
// or failed stays payable.
func (s *Service) PaymentTarget(ctx context.Context, account authctx.Account, id uuid.UUID) (*model.PaymentTarget, error) {
	if !account.CanWrite() {
		return nil, ErrForbidden
	}
	target, err := s.repo.PaymentTarget(ctx, id)
	if err != nil {
		return nil, err
	}
	if target == nil || target.AccountID != account.ID {
		return nil, ErrNotFound
	}
	if !IsPayable(target.Status, target.PaymentStatus) || target.AmountMinor <= 0 {
		return nil, ErrNotPayable
	}
	return target, nil
}

// IsPayable reports whether an order with these statuses can start an online payment.
func IsPayable(status, paymentStatus string) bool {
	return status == "pending" && (paymentStatus == "unpaid" || paymentStatus == "failed")
}

// ApplyProviderPayment records a verified provider payment on the order inside the caller's
// transaction q, so the payment session and the order commit together. Call
// PublishProviderPayment after the commit.
func (s *Service) ApplyProviderPayment(ctx context.Context, q *db.Queries, orderID int64, provider, reference string) (model.ProviderPaymentOutcome, error) {
	return repository.NewPostgres(nil, q).ApplyProviderPayment(ctx, orderID, provider, reference)
}

// PublishProviderPayment runs the side effects of a committed provider payment: the paid
// milestone notification, search indexing and card provisioning in the background.
func (s *Service) PublishProviderPayment(ctx context.Context, orderUUID uuid.UUID) {
	ord, err := s.repo.GetByUUID(ctx, orderUUID)
	if err != nil || ord == nil {
		return
	}
	s.notifyOrderTransitions(ctx, ord)
	_, _ = s.indexed(ord, nil)
	go func() {
		_, _ = s.ProvisionOrder(context.Background(), orderUUID)
	}()
}
