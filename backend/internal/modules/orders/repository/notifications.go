package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/database/db"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/orders/model"
)

func (r *PostgresRepository) ListPendingNotifications(ctx context.Context, accountID, orderID int64) ([]model.NotificationEvent, error) {
	rows, err := r.q.ListPendingOrderNotifications(ctx, db.ListPendingOrderNotificationsParams{
		AccountID: accountID, OrderID: orderID,
	})
	if err != nil {
		return nil, err
	}
	out := make([]model.NotificationEvent, 0, len(rows))
	for _, row := range rows {
		snapshot := map[string]string{}
		if err := json.Unmarshal(row.Snapshot, &snapshot); err != nil {
			return nil, fmt.Errorf("decode order notification snapshot: %w", err)
		}
		out = append(out, model.NotificationEvent{
			ID: row.ID, Status: row.Status, Snapshot: snapshot,
			CustomerID: row.CustomerID, CustomerLocale: row.CustomerLocale,
			CustomerName: strings.TrimSpace(row.CustomerName + " " + row.CustomerSurname),
		})
	}
	return out, nil
}

func (r *PostgresRepository) MarkNotificationDispatched(ctx context.Context, accountID, orderID, eventID int64) error {
	return r.q.MarkOrderNotificationDispatched(ctx, db.MarkOrderNotificationDispatchedParams{
		AccountID: accountID, OrderID: orderID, ID: eventID,
	})
}
