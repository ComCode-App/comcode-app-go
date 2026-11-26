package repository

import (
	"context"
	"errors"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/database/db"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/orders/model"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// ErrStatusMismatch is returned by ReviewOrder when the order is not in the expected status.
var ErrStatusMismatch = errors.New("order is not in the expected status")

// FindAccountProfileByEmail returns the oldest live profile of the account that lists email as
// a contact e-mail (TEC-1317 recipient matching). found is false when there is none.
func (r *PostgresRepository) FindAccountProfileByEmail(ctx context.Context, accountID int64, email string) (int64, uuid.UUID, bool, error) {
	row, err := r.q.FindAccountProfileByEmail(ctx, db.FindAccountProfileByEmailParams{AccountID: accountID, Email: email})
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, uuid.Nil, false, nil
	}
	if err != nil {
		return 0, uuid.Nil, false, err
	}
	return row.ID, row.Uuid, true, nil
}

// MarkItemProfileCreated flags the item's profile as a draft the order created.
func (r *PostgresRepository) MarkItemProfileCreated(ctx context.Context, orderID int64, itemUUID uuid.UUID) error {
	return r.q.MarkOrderItemProfileCreated(ctx, db.MarkOrderItemProfileCreatedParams{OrderID: orderID, ItemUuid: itemUUID})
}

// ListOrderCreatedProfiles lists the live draft profiles the order created in accountID.
func (r *PostgresRepository) ListOrderCreatedProfiles(ctx context.Context, accountID, orderID int64) ([]uuid.UUID, error) {
	return r.q.ListOrderCreatedProfiles(ctx, db.ListOrderCreatedProfilesParams{OrderID: orderID, AccountID: accountID})
}

// OpenOrderDraftProfiles opens the closed public pages of the draft profiles the order created
// and returns the profiles whose page opened.
func (r *PostgresRepository) OpenOrderDraftProfiles(ctx context.Context, accountID, orderID int64) ([]uuid.UUID, error) {
	return r.q.OpenOrderDraftProfileRoutes(ctx, db.OpenOrderDraftProfileRoutesParams{OrderID: orderID, AccountID: accountID})
}

// ReviewOrder moves an order from status from to status to in one transaction with the status
// history row; a non-empty rejectionReason is stored with it. It returns ErrStatusMismatch when
// the order is in another status and (nil, nil) when it does not exist.
func (r *PostgresRepository) ReviewOrder(ctx context.Context, id uuid.UUID, from, to, rejectionReason string, actorUserID *int64) (*model.OrderDetail, error) {
	found := false
	err := r.withTx(ctx, func(q *db.Queries) error {
		prev, err := q.LockOrderForStatusChange(ctx, id)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		if err != nil {
			return err
		}
		found = true
		if prev.Status != from {
			return ErrStatusMismatch
		}
		row, err := q.UpdateOrderStatus(ctx, db.UpdateOrderStatusParams{Uuid: id, Status: to})
		if err != nil {
			return err
		}
		if rejectionReason != "" {
			if err := q.SetOrderRejectionReason(ctx, db.SetOrderRejectionReasonParams{ID: prev.ID, Reason: rejectionReason}); err != nil {
				return err
			}
		}
		return recordStatusChange(ctx, q, prev.AccountID, prev.ID, prev.Status, row.Status, actorUserID)
	})
	if err != nil || !found {
		return nil, err
	}
	return r.GetByUUID(ctx, id)
}
