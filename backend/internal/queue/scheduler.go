package queue

import (
	"fmt"
	"log/slog"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/config"
	"github.com/hibiken/asynq"
)

const (
	logPurgeCron             = "@every 5m"
	analyticsMaintenanceCron = "@every 6h"
)

// StartLogPurgeScheduler registers the periodic maintenance sweeps: log retention rules,
// analytics partitions/retention, billing renewals and Expo push receipts.
func StartLogPurgeScheduler(cfg config.Config, log *slog.Logger) (*asynq.Scheduler, error) {
	if log == nil {
		log = slog.Default()
	}
	scheduler := asynq.NewScheduler(RedisOpt(cfg.Redis), nil)
	task, err := NewLogPurgeSweepTask()
	if err != nil {
		return nil, fmt.Errorf("queue: log purge task: %w", err)
	}
	if _, err := scheduler.Register(logPurgeCron, task, asynq.Queue(QueueMaintenance)); err != nil {
		return nil, fmt.Errorf("queue: register log purge schedule: %w", err)
	}
	log.Info("queue_log_purge_scheduler_registered", "cron", logPurgeCron)

	// Analytics maintenance (TEC-566): next months' events partitions, retention, events_default
	// check. It was never scheduled before, so partitions only existed up to the migration's
	// horizon (2027-06).
	analyticsTask, err := NewAnalyticsPartitionSweepTask()
	if err != nil {
		return nil, fmt.Errorf("queue: analytics maintenance task: %w", err)
	}
	if _, err := scheduler.Register(analyticsMaintenanceCron, analyticsTask, asynq.Queue(QueueMaintenance)); err != nil {
		return nil, fmt.Errorf("queue: register analytics maintenance schedule: %w", err)
	}
	log.Info("queue_analytics_maintenance_scheduler_registered", "cron", analyticsMaintenanceCron)
	if _, err := scheduler.Register("@every 5m", asynq.NewTask(TaskBillingRenewalSweep, nil), asynq.Queue(QueueMaintenance)); err != nil {
		return nil, fmt.Errorf("queue: register billing renewal: %w", err)
	}
	// Expo push receipts (TEC-1187).
	if err := registerPushReceiptsSchedule(scheduler); err != nil {
		return nil, err
	}
	log.Info("queue_push_receipts_scheduler_registered", "cron", pushReceiptsCron)
	return scheduler, nil
}
