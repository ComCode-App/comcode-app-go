package notifications

import (
	"context"
	"fmt"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/database/db"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/notifications/model"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/notifications/usecase"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/events"
)

// RegisterBillingHandlers uses the existing notification preferences, templates and delivery
// system. Unique event/user/channel keys make outbox redelivery safe.
func RegisterBillingHandlers(bus events.Bus, svc *usecase.Service, q *db.Queries) {
	if bus == nil || svc == nil {
		return
	}
	for _, name := range []string{
		"subscription.renewed", "payment.succeeded", "payment.failed",
		"subscription.past_due", "subscription.expired", "subscription.canceled",
	} {
		bus.Subscribe(name, func(ctx context.Context, event events.Event) error {
			accountID, ok := int64FromPayload(event.Payload, "account_id")
			if !ok || accountID <= 0 {
				return nil
			}
			recipients, err := q.ListBillingRecipients(ctx, accountID)
			if err != nil {
				return err
			}
			for _, id := range recipients {
				userID := id
				actionURL := "/billing"
				_, err := svc.Enqueue(ctx, model.EnqueueInput{
					UserID: &userID, AccountID: &accountID, Channels: []string{model.ChannelEmail, model.ChannelInapp},
					TemplateCode: event.Name, TemplateVars: map[string]string{}, ActionURL: &actionURL,
					SourceEvent: fmt.Sprintf("billing:%s:%d", event.EventID, userID),
					Payload:     map[string]any{"account_id": accountID, "subscription_uuid": event.Payload["subscription_uuid"]},
				})
				if err != nil {
					return err
				}
			}
			return nil
		})
	}
}
