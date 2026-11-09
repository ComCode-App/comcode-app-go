//go:build integration

package usecase

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/database/db"
	notifUsecase "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/notifications/usecase"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/orders/model"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/reqid"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/queue"
	"github.com/google/uuid"
	"github.com/hibiken/asynq"
)

type notificationQueue struct {
	mu             sync.Mutex
	calls          int
	failAt         int
	observe        func(*asynq.Task) error
	allowAnonymous bool
}

func (q *notificationQueue) Enqueue(task *asynq.Task, opts ...asynq.Option) (*asynq.TaskInfo, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.calls++
	payload, err := queue.ParseNotificationDeliverPayload(task.Payload())
	if err != nil {
		return nil, err
	}
	taskID := ""
	for _, opt := range opts {
		if opt.Type() == asynq.TaskIDOpt {
			taskID, _ = opt.Value().(string)
		}
	}
	if taskID != fmt.Sprintf("notification:%d", payload.NotificationID) && !(q.allowAnonymous && taskID == "") {
		return nil, fmt.Errorf("missing stable notification task id: %s", taskID)
	}
	if q.observe != nil {
		if err := q.observe(task); err != nil {
			return nil, err
		}
	}
	if q.failAt == q.calls {
		return nil, errors.New("queue unavailable")
	}
	return &asynq.TaskInfo{}, nil
}

func enableOrderNotifications(f orderFixture, q *notificationQueue) {
	f.svc.notifSvc = notifUsecase.New(f.d.Queries, q, nil, nil)
}

func milestoneCount(t *testing.T, f orderFixture, status string) int {
	t.Helper()
	var n int
	key := "orders.status:" + f.order.UUID + ":" + status
	if err := f.d.Pool.QueryRow(context.Background(), `SELECT count(*) FROM notifications WHERE source_event=$1`, key).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestIntegrationOrderNotificationsAfterCommitAndRollback(t *testing.T) {
	f := newOrderFixture(t)
	ctx := reqid.With(context.Background(), "tec584-commit")
	q := &notificationQueue{}
	enableOrderNotifications(f, q)
	f.svc.notifyOrderTransitions(ctx, f.order) // initial pending milestone
	q.observe = func(task *asynq.Task) error {
		if queue.TaskRequestID(task) != "tec584-commit" {
			return errors.New("missing request correlation on delivery task")
		}
		payload, err := queue.ParseNotificationDeliverPayload(task.Payload())
		if err != nil {
			return err
		}
		var status string
		// A separate pool connection must see the committed status before any delivery is scheduled.
		if err := f.d.Pool.QueryRow(ctx, `SELECT status FROM orders WHERE id=$1`, f.order.ID).Scan(&status); err != nil {
			return err
		}
		if status != "design_pending" {
			return fmt.Errorf("delivery saw uncommitted status %q", status)
		}
		var exists bool
		if err := f.d.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM notifications WHERE id=$1)`, payload.NotificationID).Scan(&exists); err != nil {
			return err
		}
		if !exists {
			return errors.New("notification not committed before scheduling")
		}
		return nil
	}
	if _, err := f.svc.UpdateStatus(ctx, uuid.MustParse(f.order.UUID), model.UpdateOrderStatusInput{Status: "design_pending"}); err != nil {
		t.Fatal(err)
	}
	if n := milestoneCount(t, f, "design_pending"); n != 2 {
		t.Fatalf("committed milestone rows=%d", n)
	}
	pending, err := f.repo.ListPendingNotifications(ctx, f.order.AccountID, f.order.ID)
	if err != nil || len(pending) != 0 {
		t.Fatalf("pending=%v err=%v", pending, err)
	}

	tx, err := f.d.Pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = tx.Rollback(context.Background()) })
	if _, err := db.New(tx).UpdateOrderStatus(ctx, db.UpdateOrderStatusParams{
		Uuid: uuid.MustParse(f.order.UUID), Status: "design_approved",
	}); err != nil {
		t.Fatal(err)
	}
	// Dispatcher on the pool cannot see the transaction's event.
	f.svc.notifyOrderTransitions(ctx, f.order)
	if n := milestoneCount(t, f, "design_approved"); n != 0 {
		t.Fatalf("uncommitted notification rows=%d", n)
	}
	if err := tx.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	f.svc.notifyOrderTransitions(ctx, f.order)
	if n := milestoneCount(t, f, "design_approved"); n != 0 {
		t.Fatalf("rolled-back notification rows=%d", n)
	}
	if n := f.count(t, `SELECT count(*) FROM order_notification_events WHERE order_id=$1 AND status='design_approved'`); n != 0 {
		t.Fatalf("rolled-back event rows=%d", n)
	}
}

func TestIntegrationOrderNotificationIdempotencyAndPartialEnqueue(t *testing.T) {
	f := newOrderFixture(t)
	ctx := context.Background()
	// Fail scheduling the second channel, after its row has been inserted.
	q := &notificationQueue{failAt: 2}
	enableOrderNotifications(f, q)
	f.svc.notifyOrderTransitions(ctx, f.order)
	events, err := f.repo.ListPendingNotifications(ctx, f.order.AccountID, f.order.ID)
	if err != nil || len(events) != 1 {
		t.Fatalf("partial enqueue pending=%v err=%v", events, err)
	}
	if n := milestoneCount(t, f, "pending"); n != 2 {
		t.Fatalf("partial rows=%d", n)
	}
	// Concurrent retries must reuse the two original rows.
	var wg sync.WaitGroup
	for range 4 {
		wg.Add(1)
		go func() { defer wg.Done(); f.svc.notifyOrderTransitions(ctx, f.order) }()
	}
	wg.Wait()
	if n := milestoneCount(t, f, "pending"); n != 2 {
		t.Fatalf("retry rows=%d", n)
	}
	// A redelivered event after successful delivery must not schedule either channel again.
	if _, err := f.d.Pool.Exec(ctx, `UPDATE notifications SET status='sent' WHERE source_event=$1`,
		"orders.status:"+f.order.UUID+":pending"); err != nil {
		t.Fatal(err)
	}
	q.mu.Lock()
	callsBefore := q.calls
	q.mu.Unlock()
	if _, err := f.svc.notifSvc.Enqueue(ctx, orderNotificationInput(events[0], f.svc.frontendURL)); err != nil {
		t.Fatal(err)
	}
	q.mu.Lock()
	callsAfter := q.calls
	q.mu.Unlock()
	if callsAfter != callsBefore {
		t.Fatalf("terminal notifications scheduled again: before=%d after=%d", callsBefore, callsAfter)
	}
	events, err = f.repo.ListPendingNotifications(ctx, f.order.AccountID, f.order.ID)
	if err != nil || len(events) != 0 {
		t.Fatalf("retry pending=%v err=%v", events, err)
	}
	id := uuid.MustParse(f.order.UUID)
	for range 2 {
		if _, err := f.svc.UpdateStatus(ctx, id, model.UpdateOrderStatusInput{Status: "delivered"}); err != nil {
			t.Fatal(err)
		}
	}
	if n := milestoneCount(t, f, "delivered"); n != 2 {
		t.Fatalf("duplicate transition rows=%d", n)
	}
	// Returning to a previously visited status still sends it at most once.
	if _, err := f.svc.UpdateStatus(ctx, id, model.UpdateOrderStatusInput{Status: "pending"}); err != nil {
		t.Fatal(err)
	}
	if n := milestoneCount(t, f, "pending"); n != 2 {
		t.Fatalf("revisited status rows=%d", n)
	}
}

func TestIntegrationOrderNotificationLocaleAndCustomer(t *testing.T) {
	for _, locale := range []string{"tr", "en"} {
		t.Run(locale, func(t *testing.T) {
			f := newOrderFixture(t)
			ctx := context.Background()
			admin := f.d.CreateUser(t)
			// Simulate a platform-created order: recorded placer belongs to a different account.
			if _, err := f.d.Pool.Exec(ctx, `UPDATE orders SET user_id=$1 WHERE id=$2`, admin.ID, f.order.ID); err != nil {
				t.Fatal(err)
			}
			if _, err := f.d.Pool.Exec(ctx, `UPDATE users SET locale=$1 WHERE id=$2`, locale, f.order.UserID); err != nil {
				t.Fatal(err)
			}
			enableOrderNotifications(f, &notificationQueue{})
			f.svc.notifyOrderTransitions(ctx, f.order)
			rows, err := f.d.Pool.Query(ctx, `SELECT user_id, title, body, action_url, recipient, channel FROM notifications WHERE source_event=$1 ORDER BY channel`,
				"orders.status:"+f.order.UUID+":pending")
			if err != nil {
				t.Fatal(err)
			}
			defer rows.Close()
			count := 0
			for rows.Next() {
				var userID int64
				var title, body, actionURL, channel string
				var recipient *string
				if err := rows.Scan(&userID, &title, &body, &actionURL, &recipient, &channel); err != nil {
					t.Fatal(err)
				}
				if userID != f.order.UserID || actionURL != "https://comcode.test/orders" {
					t.Fatalf("wrong customer/action: %d %s", userID, actionURL)
				}
				want := "Sipariş alındı"
				if locale == "en" {
					want = "Order received"
				}
				if !strings.Contains(title, want) || !strings.Contains(body, f.order.OrderNumber) || strings.Contains(body, "{{") {
					t.Fatalf("unrendered/wrong locale: title=%q body=%q", title, body)
				}
				if channel == "email" && (recipient == nil || *recipient != f.order.CustomerEmail) {
					t.Fatalf("email recipient=%v", recipient)
				}
				count++
			}
			if err := rows.Err(); err != nil {
				t.Fatal(err)
			}
			if count != 2 {
				t.Fatalf("notifications=%d", count)
			}
			// Explicit wrong-account scopes must not expose the customer's events.
			pending, err := f.repo.ListPendingNotifications(ctx, admin.AccountID, f.order.ID)
			if err != nil || len(pending) != 0 {
				t.Fatalf("cross-account events=%v err=%v", pending, err)
			}
		})
	}
}

func TestIntegrationOrderNotificationShippingAndDesignPaths(t *testing.T) {
	f := newOrderFixture(t)
	ctx := context.Background()
	enableOrderNotifications(f, &notificationQueue{allowAnonymous: true})
	f.svc.notifyOrderTransitions(ctx, f.order)
	id := uuid.MustParse(f.order.UUID)
	req, err := f.svc.CreateAssetRequest(ctx, id, model.CreateAssetRequestInput{RequestType: "logo", Title: "Logo"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if n := milestoneCount(t, f, "design_pending"); n != 2 {
		t.Fatalf("design pending rows=%d", n)
	}
	if _, err := f.svc.ReviewAsset(ctx, id, uuid.MustParse(req.UUID), model.ReviewAssetInput{Status: "approved"}); err != nil {
		t.Fatal(err)
	}
	if n := milestoneCount(t, f, "design_approved"); n != 2 {
		t.Fatalf("design approved rows=%d", n)
	}
	shipping := model.UpdateOrderShippingInput{ShippingCarrier: "Carrier", TrackingNumber: "TRACK-584", TrackingURL: "https://carrier.test/584"}
	if _, err := f.svc.UpdateShipping(ctx, id, shipping); err != nil {
		t.Fatal(err)
	}
	if n := milestoneCount(t, f, "shipped"); n != 0 {
		t.Fatalf("tracking edit sent %d shipping notifications", n)
	}
	shipping.MarkAsShipped = true
	if _, err := f.svc.UpdateShipping(ctx, id, shipping); err != nil {
		t.Fatal(err)
	}
	if n := milestoneCount(t, f, "shipped"); n != 2 {
		t.Fatalf("shipping rows=%d", n)
	}
	var body string
	if err := f.d.Pool.QueryRow(ctx, `SELECT body FROM notifications WHERE source_event=$1 AND channel='email'`,
		"orders.status:"+f.order.UUID+":shipped").Scan(&body); err != nil {
		t.Fatal(err)
	}
	for _, value := range []string{shipping.ShippingCarrier, shipping.TrackingNumber, shipping.TrackingURL} {
		if !strings.Contains(body, value) {
			t.Fatalf("missing %s in %s", value, body)
		}
	}
	// Refund has no automatic provisioning goroutine and shares the main status dedupe key.
	if _, err := f.svc.UpdatePaymentStatus(ctx, id, model.UpdateOrderPaymentStatusInput{PaymentStatus: "refunded"}); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.UpdateStatus(ctx, id, model.UpdateOrderStatusInput{Status: "refunded"}); err != nil {
		t.Fatal(err)
	}
	if n := milestoneCount(t, f, "refunded"); n != 2 {
		t.Fatalf("refund rows=%d", n)
	}
}
