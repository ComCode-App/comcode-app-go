package application

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/analytics/aggregation"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/analytics/domain"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/analytics/maintenance"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/analytics/realtime"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/analytics/storage"
)

// WorkerProcessor coordinates async ingestion, storage, counter increments, and aggregation.
type WorkerProcessor struct {
	store      storage.EventStore
	counters   *realtime.CounterManager
	aggregator *aggregation.Aggregator
	maint      *maintenance.PartitionManager
	log        *slog.Logger
}

// NewWorkerProcessor creates a WorkerProcessor.
func NewWorkerProcessor(
	store storage.EventStore,
	counters *realtime.CounterManager,
	aggregator *aggregation.Aggregator,
	maint *maintenance.PartitionManager,
	log *slog.Logger,
) *WorkerProcessor {
	if log == nil {
		log = slog.Default()
	}
	return &WorkerProcessor{
		store:      store,
		counters:   counters,
		aggregator: aggregator,
		maint:      maint,
		log:        log,
	}
}

// ProcessEvent deserializes an incoming queue payload, persists the raw event, updates real-time counters, and executes idempotent aggregation.
func (p *WorkerProcessor) ProcessEvent(ctx context.Context, payload []byte) error {
	var event domain.Event
	if err := json.Unmarshal(payload, &event); err != nil {
		p.log.Error("analytics_event_unmarshal_failed", "error", err)
		return fmt.Errorf("unmarshal event payload: %w", err)
	}

	// 1. Raw Event Storage (Immutable)
	var storedEvent *domain.Event
	if p.store != nil {
		var err error
		storedEvent, err = p.store.Store(ctx, event)
		if err != nil {
			p.log.Error("store_raw_event_failed", "uuid", event.UUID, "type", event.Type, "error", err)
			return fmt.Errorf("store raw event: %w", err)
		}
	} else {
		storedEvent = &event
	}

	// 2. Realtime Redis Counters (Fire-and-forget / non-blocking)
	if p.counters != nil {
		if err := p.counters.Increment(ctx, *storedEvent); err != nil {
			p.log.Warn("realtime_counter_increment_failed", "uuid", event.UUID, "error", err)
		}
	}

	// 3. Idempotent Rollup Aggregation (Hourly & Daily)
	if p.aggregator != nil {
		if err := p.aggregator.AggregateEvent(ctx, *storedEvent); err != nil {
			p.log.Error("aggregate_event_failed", "event_id", storedEvent.ID, "error", err)
			return fmt.Errorf("aggregate event: %w", err)
		}
	}

	p.log.Debug("analytics_event_processed_successfully",
		"id", storedEvent.ID,
		"uuid", storedEvent.UUID,
		"type", storedEvent.Type,
		"subject", storedEvent.Subject,
	)

	return nil
}

// ProcessMaintenance runs scheduled partition management and retention sweeps.
func (p *WorkerProcessor) ProcessMaintenance(ctx context.Context) error {
	if p.maint == nil {
		return nil
	}
	if err := p.maint.EnsurePartitions(ctx, 3); err != nil {
		p.log.Error("ensure_partitions_failed", "error", err)
	}
	return p.maint.RunRetentionSweep(ctx)
}
