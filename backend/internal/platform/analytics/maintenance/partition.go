package maintenance

import (
	"context"
	"fmt"
	"log/slog"
	"regexp"
	"strconv"
	"time"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/database/db"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PartitionManager creates upcoming monthly partitions and enforces data retention.
type PartitionManager struct {
	pool *pgxpool.Pool
	q    *db.Queries
	log  *slog.Logger
}

// NewPartitionManager creates a PartitionManager.
func NewPartitionManager(pool *pgxpool.Pool, q *db.Queries, log *slog.Logger) *PartitionManager {
	if log == nil {
		log = slog.Default()
	}
	return &PartitionManager{
		pool: pool,
		q:    q,
		log:  log,
	}
}

// EnsurePartitions pre-creates partitions for the current month and the next N months.
func (m *PartitionManager) EnsurePartitions(ctx context.Context, monthsAhead int) error {
	if m.pool == nil {
		return nil
	}
	if monthsAhead <= 0 {
		monthsAhead = 3
	}

	now := time.Now().UTC()
	current := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)

	for i := 0; i <= monthsAhead; i++ {
		start := current.AddDate(0, i, 0)
		end := start.AddDate(0, 1, 0)

		tableName := fmt.Sprintf("events_%04d_%02d", start.Year(), start.Month())
		query := fmt.Sprintf(
			"CREATE TABLE IF NOT EXISTS %s PARTITION OF events FOR VALUES FROM ('%s') TO ('%s');",
			tableName,
			start.Format("2006-01-02 15:04:05+00"),
			end.Format("2006-01-02 15:04:05+00"),
		)

		if _, err := m.pool.Exec(ctx, query); err != nil {
			m.log.Error("ensure_partition_failed", "table", tableName, "error", err)
			return fmt.Errorf("create partition %s: %w", tableName, err)
		}
		m.log.Debug("partition_verified", "table", tableName)
	}

	return nil
}

// RunRetentionSweep applies retention rules based on analytics_settings.
func (m *PartitionManager) RunRetentionSweep(ctx context.Context) error {
	if m.pool == nil || m.q == nil {
		return nil
	}

	settings, err := m.q.GetAnalyticsSettings(ctx)
	if err != nil {
		return fmt.Errorf("get analytics settings for retention: %w", err)
	}

	rawDays := int(settings.RawEventRetentionDays)
	if rawDays > 0 {
		cutoff := time.Now().UTC().AddDate(0, 0, -rawDays)
		m.log.Info("running_raw_events_retention_sweep", "cutoff", cutoff)
		// Whole months past the cutoff are dropped; the rest is deleted row by row (this covers the
		// month the cutoff falls in and events_default). Before TEC-566 only events_default was pruned.
		if err := m.dropPartitionsBefore(ctx, cutoff); err != nil {
			m.log.Error("drop_expired_partitions_failed", "error", err)
		}
		if _, err := m.pool.Exec(ctx, "DELETE FROM events WHERE occurred_at < $1", cutoff); err != nil {
			m.log.Error("raw_events_retention_failed", "error", err)
		}
	}
	m.warnDefaultPartition(ctx)

	aggDays := int(settings.AggregateRetentionDays)
	if aggDays > 0 {
		aggCutoff := time.Now().UTC().AddDate(0, 0, -aggDays)
		m.log.Info("running_aggregates_retention_sweep", "cutoff", aggCutoff)
		_, _ = m.pool.Exec(ctx, "DELETE FROM event_aggregates_hourly WHERE bucket_hour < $1", aggCutoff)
		_, _ = m.pool.Exec(ctx, "DELETE FROM event_aggregates_daily WHERE bucket_date < $1", aggCutoff.Format("2006-01-02"))
	}

	// The visitor rollup only feeds the 30-day platform KPI (TEC-953).
	visitorCutoff := time.Now().UTC().AddDate(0, 0, -35)
	_, _ = m.pool.Exec(ctx, "DELETE FROM analytics_visitors_hourly WHERE bucket_hour < $1", visitorCutoff)

	// Clean up event processing idempotency logs older than 30 days
	idempCutoff := time.Now().UTC().AddDate(0, 0, -30)
	_, _ = m.pool.Exec(ctx, "DELETE FROM event_processing_log WHERE occurred_at < $1", idempCutoff)

	return nil
}

var partitionName = regexp.MustCompile(`^events_(\d{4})_(\d{2})$`)

// dropPartitionsBefore drops monthly partitions that end on or before cutoff.
func (m *PartitionManager) dropPartitionsBefore(ctx context.Context, cutoff time.Time) error {
	rows, err := m.pool.Query(ctx, `
		SELECT c.relname FROM pg_inherits i
		JOIN pg_class c ON c.oid = i.inhrelid
		JOIN pg_class p ON p.oid = i.inhparent
		WHERE p.relname = 'events'`)
	if err != nil {
		return err
	}
	var names []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			rows.Close()
			return err
		}
		names = append(names, name)
	}
	rows.Close()
	for _, name := range expiredPartitions(names, cutoff) {
		if _, err := m.pool.Exec(ctx, "DROP TABLE IF EXISTS "+name); err != nil {
			return fmt.Errorf("drop %s: %w", name, err)
		}
		m.log.Info("expired_partition_dropped", "table", name)
	}
	return nil
}

// expiredPartitions picks the events_YYYY_MM tables whose month ends on or before cutoff.
func expiredPartitions(names []string, cutoff time.Time) []string {
	var out []string
	for _, name := range names {
		mm := partitionName.FindStringSubmatch(name)
		if mm == nil {
			continue
		}
		year, _ := strconv.Atoi(mm[1])
		month, _ := strconv.Atoi(mm[2])
		end := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC).AddDate(0, 1, 0)
		if !end.After(cutoff) {
			out = append(out, name)
		}
	}
	return out
}

// warnDefaultPartition logs when events land in events_default, which means a monthly partition
// was missing when they arrived (TEC-566).
func (m *PartitionManager) warnDefaultPartition(ctx context.Context) {
	var n int64
	if err := m.pool.QueryRow(ctx, "SELECT COUNT(*) FROM events_default").Scan(&n); err != nil {
		m.log.Error("events_default_count_failed", "error", err)
		return
	}
	if n > 0 {
		m.log.Warn("events_default_has_rows", "rows", n, "hint", "a monthly events partition was missing; check the analytics maintenance schedule")
	}
}
