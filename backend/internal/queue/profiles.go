package queue

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hibiken/asynq"
)

// TaskProfileMediaCleanup deletes the storage objects of a deleted profile (TEC-1176).
const TaskProfileMediaCleanup = "app:profiles:media_cleanup"

// ProfileMediaCleanupPayload names a deleted profile and the object keys its row pointed at,
// collected before the delete. The handler also lists the profile's own prefix.
type ProfileMediaCleanupPayload struct {
	ProfileUUID string   `json:"profile_uuid"`
	Keys        []string `json:"keys,omitempty"`
}

// NewProfileMediaCleanupTask builds a profile media cleanup task.
func NewProfileMediaCleanupTask(p ProfileMediaCleanupPayload) (*asynq.Task, error) {
	body, err := json.Marshal(p)
	if err != nil {
		return nil, fmt.Errorf("queue: marshal profile media cleanup: %w", err)
	}
	return asynq.NewTask(TaskProfileMediaCleanup, body), nil
}

// ParseProfileMediaCleanupPayload decodes a profile media cleanup payload.
func ParseProfileMediaCleanupPayload(data []byte) (ProfileMediaCleanupPayload, error) {
	var payload ProfileMediaCleanupPayload
	if err := json.Unmarshal(data, &payload); err != nil {
		return ProfileMediaCleanupPayload{}, fmt.Errorf("queue: unmarshal profile media cleanup: %w", err)
	}
	return payload, nil
}

// ProcessProfileMediaCleanupFunc deletes the media of one deleted profile.
type ProcessProfileMediaCleanupFunc func(ctx context.Context, payload ProfileMediaCleanupPayload) error

// WithProfileMediaCleanup registers the deleted-profile media cleanup processor (TEC-1176).
func (w *Worker) WithProfileMediaCleanup(fn ProcessProfileMediaCleanupFunc) *Worker {
	w.mux.HandleFunc(TaskProfileMediaCleanup, func(ctx context.Context, task *asynq.Task) error {
		payload, err := ParseProfileMediaCleanupPayload(task.Payload())
		if err != nil {
			return fmt.Errorf("%v: %w", err, asynq.SkipRetry)
		}
		if fn == nil {
			return fmt.Errorf("profile media cleanup processor is missing")
		}
		return fn(ctx, payload)
	})
	return w
}
