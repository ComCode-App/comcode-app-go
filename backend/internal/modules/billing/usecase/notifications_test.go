package usecase_test

import (
	"context"
	"io"
	"log/slog"
	"testing"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/notifications"
	notifusecase "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/notifications/usecase"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/events"
	"github.com/google/uuid"
	"github.com/hibiken/asynq"
)

type notificationQueue struct{ count int }

func (q *notificationQueue) Enqueue(_ *asynq.Task, _ ...asynq.Option) (*asynq.TaskInfo, error) {
	q.count++
	return &asynq.TaskInfo{}, nil
}

func TestBillingNotificationsLocalizedAndDeduplicated(t *testing.T) {
	for _, locale := range []string{"tr", "en"} {
		t.Run(locale, func(t *testing.T) {
			f := setup(t)
			f.exec(t, "UPDATE users SET locale=$2 WHERE id=$1", f.payer.UserID, locale)
			queue := &notificationQueue{}
			log := slog.New(slog.NewTextHandler(io.Discard, nil))
			// No delivery providers; enqueue only. This test cannot send any email.
			svc := notifusecase.New(f.d.Queries, queue, nil, log)
			bus := events.NewBus(log)
			notifications.RegisterBillingHandlers(bus, svc, f.d.Queries)
			event := events.Event{Name: "subscription.past_due", EventID: uuid.New(), Payload: map[string]any{
				"account_id": f.account, "subscription_uuid": f.sub(t).Uuid.String(),
			}}
			for range 2 {
				if err := bus.Publish(context.Background(), event); err != nil {
					t.Fatal(err)
				}
			}
			var count int
			var title string
			err := f.d.Pool.QueryRow(f.ctx, `SELECT count(*),min(title) FROM notifications WHERE user_id=$1 AND source_event LIKE 'billing:%'`, f.payer.UserID).Scan(&count, &title)
			if err != nil || count != 2 {
				t.Fatalf("notification duplicates %d %v", count, err)
			}
			expected := "Subscription payment overdue"
			if locale == "tr" {
				expected = "Abonelik ödemesi gecikti"
			}
			if title != expected {
				t.Fatalf("translation %q expected %q", title, expected)
			}
			f.exec(t, `INSERT INTO notification_preferences(user_id,email_enabled,inapp_enabled,realtime_enabled) VALUES($1,false,true,true)
   ON CONFLICT(user_id) DO UPDATE SET email_enabled=false`, f.payer.UserID)
			event.EventID = uuid.New()
			event.Name = "subscription.expired"
			if err := bus.Publish(f.ctx, event); err != nil {
				t.Fatal(err)
			}
			err = f.d.Pool.QueryRow(f.ctx, `SELECT count(*) FROM notifications WHERE user_id=$1 AND channel='email'`, f.payer.UserID).Scan(&count)
			if err != nil || count != 1 {
				t.Fatalf("email preference ignored %d %v", count, err)
			}
		})
	}
}
