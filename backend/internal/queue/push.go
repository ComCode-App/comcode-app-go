package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/hibiken/asynq"
)

// Mobile push through the Expo Push Service (TEC-1187).
const (
	// TaskPushSend sends the mobile push of one delivered notification row.
	TaskPushSend = "app:push:send"
	// TaskPushReceipts reads Expo push receipts and disables dead device tokens.
	TaskPushReceipts = "app:push:receipts"

	// PushSendMaxRetry bounds Asynq retries after the client's own retries on 429 / 5xx.
	PushSendMaxRetry = 5

	pushReceiptsCron = "@every 15m"
)

// PushSendPayload names the notification row to push.
type PushSendPayload struct {
	NotificationID int64 `json:"notification_id"`
}

// NewPushSendTask builds a mobile push task.
func NewPushSendTask(notificationID int64) (*asynq.Task, error) {
	body, err := json.Marshal(PushSendPayload{NotificationID: notificationID})
	if err != nil {
		return nil, fmt.Errorf("queue: marshal push send: %w", err)
	}
	return asynq.NewTask(TaskPushSend, body), nil
}

// PushSendOptions are the enqueue options of a mobile push. The task id makes a second enqueue
// for the same row (a reclaimed delivery) a no-op while the first task is pending.
func PushSendOptions(notificationID int64) []asynq.Option {
	return []asynq.Option{
		asynq.Queue(QueueNotifications),
		asynq.MaxRetry(PushSendMaxRetry),
		asynq.Timeout(2 * time.Minute),
		asynq.TaskID(fmt.Sprintf("push:%d", notificationID)),
	}
}

// ParsePushSendPayload decodes a push send payload.
func ParsePushSendPayload(data []byte) (PushSendPayload, error) {
	var p PushSendPayload
	if err := json.Unmarshal(data, &p); err != nil {
		return PushSendPayload{}, fmt.Errorf("queue: unmarshal push send: %w", err)
	}
	return p, nil
}

// NewPushReceiptsTask builds the periodic receipt check.
func NewPushReceiptsTask() *asynq.Task {
	return asynq.NewTask(TaskPushReceipts, []byte("{}"))
}

// SendPushFunc sends the mobile push of a notification id.
type SendPushFunc func(ctx context.Context, notificationID int64) error

// ProcessPushReceiptsFunc checks due receipts and returns how many devices it disabled.
type ProcessPushReceiptsFunc func(ctx context.Context) (int, error)

// WithMobilePush registers the Expo push sender and the receipt processor.
func (w *Worker) WithMobilePush(send SendPushFunc, receipts ProcessPushReceiptsFunc) *Worker {
	w.mux.HandleFunc(TaskPushSend, func(ctx context.Context, task *asynq.Task) error {
		p, err := ParsePushSendPayload(task.Payload())
		if err != nil {
			return fmt.Errorf("%v: %w", err, asynq.SkipRetry)
		}
		if send == nil {
			return nil
		}
		return send(ctx, p.NotificationID)
	})
	w.mux.HandleFunc(TaskPushReceipts, func(ctx context.Context, _ *asynq.Task) error {
		if receipts == nil {
			return nil
		}
		n, err := receipts(ctx)
		if n > 0 {
			w.log.Info("push_receipts_disabled_devices", "count", n)
		}
		return err
	})
	return w
}

// registerPushReceiptsSchedule adds the receipt check every 15 minutes (Expo recommends reading
// receipts about 15 minutes after sending).
func registerPushReceiptsSchedule(scheduler *asynq.Scheduler) error {
	if _, err := scheduler.Register(
		pushReceiptsCron, NewPushReceiptsTask(),
		asynq.Queue(QueueMaintenance), asynq.MaxRetry(0),
		asynq.Timeout(5*time.Minute), asynq.Unique(14*time.Minute),
	); err != nil {
		return fmt.Errorf("queue: register push receipts: %w", err)
	}
	return nil
}
