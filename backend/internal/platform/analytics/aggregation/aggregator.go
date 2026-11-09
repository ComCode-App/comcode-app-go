package aggregation

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/database/db"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/analytics/domain"
	"github.com/jackc/pgx/v5/pgtype"
)

// Aggregator handles idempotent hourly and daily rollups.
type Aggregator struct {
	q   *db.Queries
	log *slog.Logger
}

// NewAggregator creates an Aggregator.
func NewAggregator(q *db.Queries, log *slog.Logger) *Aggregator {
	if log == nil {
		log = slog.Default()
	}
	return &Aggregator{
		q:   q,
		log: log,
	}
}

// AggregateEvent processes a single raw event into hourly and daily summary records idempotently.
func (a *Aggregator) AggregateEvent(ctx context.Context, event domain.Event) error {
	if a.q == nil {
		return nil
	}

	if event.ID <= 0 {
		return fmt.Errorf("cannot aggregate event with invalid ID: %d", event.ID)
	}

	occurredAt := event.OccurredAt
	if occurredAt.IsZero() {
		occurredAt = time.Now().UTC()
	}

	// 1. Idempotency Check: skip if already aggregated
	isProcessed, err := a.q.IsEventProcessed(ctx, db.IsEventProcessedParams{
		EventID:    event.ID,
		OccurredAt: pgtype.Timestamptz{Time: occurredAt, Valid: true},
	})
	if err == nil && isProcessed {
		a.log.Debug("event_already_aggregated_skipping", "event_id", event.ID)
		return nil
	}

	// Prepare single-event breakdown maps
	countryMap := map[string]int{}
	if event.Context.CountryCode != "" {
		countryMap[event.Context.CountryCode] = 1
	}
	countryJSON, _ := json.Marshal(countryMap)

	cityMap := map[string]int{}
	if event.Context.City != "" {
		cityMap[event.Context.City] = 1
	}
	cityJSON, _ := json.Marshal(cityMap)

	deviceMap := map[string]int{}
	if event.Context.DeviceType != "" {
		deviceMap[event.Context.DeviceType] = 1
	}
	deviceJSON, _ := json.Marshal(deviceMap)

	browserMap := map[string]int{}
	if event.Context.Browser != "" {
		browserMap[event.Context.Browser] = 1
	}
	browserJSON, _ := json.Marshal(browserMap)

	osMap := map[string]int{}
	if event.Context.OS != "" {
		osMap[event.Context.OS] = 1
	}
	osJSON, _ := json.Marshal(osMap)

	refererMap := map[string]int{}
	if event.Context.Referer != "" {
		refererMap[event.Context.Referer] = 1
	}
	refererJSON, _ := json.Marshal(refererMap)

	// unique_sessions counts events that carried a session, not distinct sessions (an upsert cannot
	// know); dashboards count distinct visitors from raw events (CountDistinctVisitors).
	uniqueSessionInc := int64(0)
	if event.SessionID != "" {
		uniqueSessionInc = 1
	}

	source := domain.EntrySource(event)

	// 2. Hourly Aggregate Upsert
	bucketHour := occurredAt.Truncate(time.Hour)
	_, err = a.q.UpsertHourlyAggregate(ctx, db.UpsertHourlyAggregateParams{
		BucketHour:       pgtype.Timestamptz{Time: bucketHour, Valid: true},
		SubjectType:      event.Subject.Type,
		SubjectID:        event.Subject.ID,
		EventType:        event.Type,
		TotalCount:       1,
		UniqueSessions:   uniqueSessionInc,
		CountryBreakdown: countryJSON,
		CityBreakdown:    cityJSON,
		DeviceBreakdown:  deviceJSON,
		BrowserBreakdown: browserJSON,
		OsBreakdown:      osJSON,
		Source:           source,
	})
	if err != nil {
		return fmt.Errorf("upsert hourly aggregate: %w", err)
	}

	// Distinct visitors per hour for the platform KPI (TEC-953). Idempotent, and not fatal: failing
	// here must not make a retry count the hourly rollup above twice.
	if key := visitorKey(event); key != "" {
		if err := a.q.MarkVisitorHour(ctx, db.MarkVisitorHourParams{
			BucketHour: pgtype.Timestamptz{Time: bucketHour, Valid: true},
			VisitorKey: key,
		}); err != nil {
			a.log.Warn("mark_visitor_hour_failed", "event_id", event.ID, "error", err)
		}
	}

	// 3. Daily Aggregate Upsert
	bucketDate := pgtype.Date{
		Time:  time.Date(occurredAt.Year(), occurredAt.Month(), occurredAt.Day(), 0, 0, 0, 0, time.UTC),
		Valid: true,
	}
	_, err = a.q.UpsertDailyAggregate(ctx, db.UpsertDailyAggregateParams{
		BucketDate:       bucketDate,
		SubjectType:      event.Subject.Type,
		SubjectID:        event.Subject.ID,
		EventType:        event.Type,
		TotalCount:       1,
		UniqueSessions:   uniqueSessionInc,
		CountryBreakdown: countryJSON,
		CityBreakdown:    cityJSON,
		DeviceBreakdown:  deviceJSON,
		BrowserBreakdown: browserJSON,
		OsBreakdown:      osJSON,
		RefererBreakdown: refererJSON,
		Source:           source,
	})
	if err != nil {
		return fmt.Errorf("upsert daily aggregate: %w", err)
	}

	// 4. Mark processed in idempotency log
	_ = a.q.MarkEventProcessed(ctx, db.MarkEventProcessedParams{
		EventID:    event.ID,
		OccurredAt: pgtype.Timestamptz{Time: occurredAt, Valid: true},
	})

	return nil
}

// visitorKey identifies a visitor the way the raw distinct counts do (session id, else the rotating
// IP hash), hashed so the rollup does not keep session ids. Empty when the event carries neither.
func visitorKey(event domain.Event) string {
	id := "s:" + event.SessionID
	if event.SessionID == "" {
		if event.Context.IPHash == "" {
			return ""
		}
		id = "i:" + event.Context.IPHash
	}
	sum := sha256.Sum256([]byte(id))
	return hex.EncodeToString(sum[:16])
}

// AggregateBatch aggregates a batch of events sequentially.
func (a *Aggregator) AggregateBatch(ctx context.Context, events []domain.Event) error {
	for _, ev := range events {
		if err := a.AggregateEvent(ctx, ev); err != nil {
			a.log.Error("aggregate_event_failed", "event_id", ev.ID, "error", err)
		}
	}
	return nil
}
