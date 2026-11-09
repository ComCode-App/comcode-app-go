package delivery

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/analytics/domain"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/queue"
	"github.com/hibiken/asynq"
)

// AsynqPublisher publishes analytics events to the Asynq background queue.
type AsynqPublisher struct {
	client *queue.Client
}

// NewAsynqPublisher creates a new Asynq-backed event publisher.
func NewAsynqPublisher(client *queue.Client) *AsynqPublisher {
	return &AsynqPublisher{client: client}
}

// PublishEvent marshals and enqueues an event for background ingestion and aggregation.
func (p *AsynqPublisher) PublishEvent(ctx context.Context, event domain.Event) error {
	if p.client == nil {
		return fmt.Errorf("asynq client is nil")
	}

	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal analytics event: %w", err)
	}

	task, err := queue.NewAnalyticsProcessEventTask(data)
	if err != nil {
		return err
	}

	_, err = p.client.Enqueue(queue.WithRequestID(ctx, task),
		asynq.Queue(queue.QueueAnalytics),
		asynq.MaxRetry(5),
		asynq.Timeout(30),
	)
	return err
}
