package queue

import (
	"context"
	"fmt"

	"github.com/hibiken/asynq"
)

const TaskBillingRenewalSweep = "app:billing:renewal_sweep"

// WithBilling registers the payment-only subscription lifecycle sweep.
func (w *Worker) WithBilling(fn func(context.Context) error) *Worker {
	w.mux.HandleFunc(TaskBillingRenewalSweep, func(ctx context.Context, _ *asynq.Task) error {
		if fn == nil {
			return fmt.Errorf("billing processor is missing")
		}
		return fn(ctx)
	})
	return w
}
