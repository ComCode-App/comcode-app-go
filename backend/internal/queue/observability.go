package queue

import (
	"context"
	"errors"
	"log/slog"
	"maps"
	"time"

	"github.com/hibiken/asynq"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/errreport"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/metrics"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/reqid"
)

// WithRequestID returns task with the correlation id from ctx stored in the Asynq
// task headers (X-Request-ID), so the worker can log it. Without an id in ctx the
// task is returned unchanged. Options given to the task constructor are not kept,
// so pass queue/retry options to Enqueue (as every caller does).
func WithRequestID(ctx context.Context, task *asynq.Task) *asynq.Task {
	id := reqid.From(ctx)
	if task == nil || id == "" {
		return task
	}
	headers := maps.Clone(task.Headers())
	if headers == nil {
		headers = map[string]string{}
	}
	headers[reqid.Header] = id
	return asynq.NewTaskWithHeaders(task.Type(), task.Payload(), headers)
}

// TaskRequestID returns the correlation id carried by a task, or "".
func TaskRequestID(task *asynq.Task) string {
	if task == nil {
		return ""
	}
	id := task.Headers()[reqid.Header]
	if !reqid.Valid(id) {
		return ""
	}
	return id
}

// taskContext restores the task's request id into ctx, so handlers' *Context logs and
// any task they enqueue keep the same correlation id.
func taskContext(ctx context.Context, task *asynq.Task) context.Context {
	return reqid.With(ctx, TaskRequestID(task))
}

// observeMiddleware restores request_id into the handler context and records task
// metrics. Failures are logged once, by errorHandler.
func observeMiddleware(next asynq.Handler) asynq.Handler {
	return asynq.HandlerFunc(func(ctx context.Context, task *asynq.Task) error {
		ctx = taskContext(ctx, task)
		started := time.Now()
		err := next.ProcessTask(ctx, task)
		metrics.ObserveTask(task.Type(), err, time.Since(started))
		return err
	})
}

// errorHandler runs after every failed attempt (Asynq ErrorHandler hook; handler
// panics arrive here as errors too). It logs type, task id, queue, retry count and
// request_id at error level, and reports to the optional error tracker once retries are exhausted,
// so transient failures that later succeed do not page anyone.
func errorHandler(log *slog.Logger) asynq.ErrorHandler {
	return asynq.ErrorHandlerFunc(func(ctx context.Context, task *asynq.Task, err error) {
		ctx = taskContext(ctx, task)
		retried, _ := asynq.GetRetryCount(ctx)
		maxRetry, _ := asynq.GetMaxRetry(ctx)
		final := retried >= maxRetry || errors.Is(err, asynq.SkipRetry)
		attrs := append(taskAttrs(ctx, task, err), "final", final)
		log.ErrorContext(ctx, "queue_task_failed", attrs...)
		if !final {
			return
		}
		id, _ := asynq.GetTaskID(ctx)
		queueName, _ := asynq.GetQueueName(ctx)
		errreport.CaptureError(ctx, err, map[string]string{
			"task.type":  task.Type(),
			"task.id":    id,
			"task.queue": queueName,
		})
	})
}

func taskAttrs(ctx context.Context, task *asynq.Task, err error) []any {
	id, _ := asynq.GetTaskID(ctx)
	queueName, _ := asynq.GetQueueName(ctx)
	retried, _ := asynq.GetRetryCount(ctx)
	maxRetry, _ := asynq.GetMaxRetry(ctx)
	return []any{
		"type", task.Type(),
		"task_id", id,
		"queue", queueName,
		"retry", retried,
		"max_retry", maxRetry,
		"error", err.Error(),
	}
}
