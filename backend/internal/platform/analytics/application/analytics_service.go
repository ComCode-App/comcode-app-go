package application

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/database/db"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/analytics/domain"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/analytics/privacy"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/redis/go-redis/v9"
)

// AnalyticsSettings holds runtime configuration for the analytics subsystem.
type AnalyticsSettings struct {
	Enabled                bool   `json:"enabled"`
	EventDeliveryMode      string `json:"event_delivery_mode"`
	RawEventRetentionDays  int    `json:"raw_event_retention_days"`
	AggregateRetentionDays int    `json:"aggregate_retention_days"`
	QueueName              string `json:"queue_name"`
	RealtimeEnabled        bool   `json:"realtime_enabled"`
	BatchSize              int    `json:"batch_size"`
	BatchFlushIntervalMs   int    `json:"batch_flush_interval_ms"`
	IPHashSaltRotationDays int    `json:"ip_hash_salt_rotation_days"`
	AnonymizeIP            bool   `json:"anonymize_ip"`
	// Read-only (TEC-936): a changed rotation period applies from the UTC day after
	// IPHashSaltRotationChangedOn (yyyy-MM-dd); until then IPHashSaltRotationPreviousDays is used.
	IPHashSaltRotationPreviousDays int    `json:"ip_hash_salt_rotation_previous_days,omitempty"`
	IPHashSaltRotationChangedOn    string `json:"ip_hash_salt_rotation_changed_on,omitempty"`
}

// ErrInvalidSettings is returned for analytics settings outside their allowed range.
var ErrInvalidSettings = errors.New("ip_hash_salt_rotation_days must be between 1 and 30")

// AnalyticsService coordinates metric queries and configuration.
type AnalyticsService struct {
	repo domain.AnalyticsRepository
	q    *db.Queries
	rdb  *redis.Client
	log  *slog.Logger

	mu         sync.RWMutex
	settings   AnalyticsSettings
	settingsAt time.Time
}

// NewAnalyticsService creates an AnalyticsService.
func NewAnalyticsService(repo domain.AnalyticsRepository, q *db.Queries, rdb *redis.Client, log *slog.Logger) *AnalyticsService {
	if log == nil {
		log = slog.Default()
	}
	svc := &AnalyticsService{
		repo: repo,
		q:    q,
		rdb:  rdb,
		log:  log,
	}
	_ = svc.loadSettings(context.Background())
	return svc
}

// GetSummary retrieves high-level metric summaries.
func (s *AnalyticsService) GetSummary(ctx context.Context, filter domain.AnalyticsFilter) (*domain.Summary, error) {
	if err := validateDateRange(&filter); err != nil {
		return nil, err
	}
	return s.repo.GetSummary(ctx, filter)
}

// GetTimeSeries retrieves timeline data points.
func (s *AnalyticsService) GetTimeSeries(ctx context.Context, filter domain.AnalyticsFilter) (*domain.TimeSeriesResult, error) {
	if err := validateDateRange(&filter); err != nil {
		return nil, err
	}
	return s.repo.GetTimeSeries(ctx, filter)
}

// GetBreakdown retrieves distribution by dimension.
func (s *AnalyticsService) GetBreakdown(ctx context.Context, filter domain.AnalyticsFilter, dimension string) (*domain.BreakdownResult, error) {
	if err := validateDateRange(&filter); err != nil {
		return nil, err
	}
	return s.repo.GetBreakdown(ctx, filter, dimension)
}

// GetRawEvents retrieves bounded paginated events.
func (s *AnalyticsService) GetRawEvents(ctx context.Context, filter domain.RawEventFilter) (*domain.RawEventListResult, error) {
	return s.repo.GetRawEvents(ctx, filter)
}

// GetSettings retrieves current runtime configuration with cache.
func (s *AnalyticsService) GetSettings(ctx context.Context) (AnalyticsSettings, error) {
	s.mu.RLock()
	if time.Since(s.settingsAt) < 1*time.Minute {
		defer s.mu.RUnlock()
		return s.settings, nil
	}
	s.mu.RUnlock()

	_ = s.loadSettings(ctx)
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.settings, nil
}

// UpdateSettings updates analytics configuration and invalidates cache.
func (s *AnalyticsService) UpdateSettings(ctx context.Context, newSettings AnalyticsSettings) (*AnalyticsSettings, error) {
	if s.q == nil {
		return nil, errors.New("queries not initialized")
	}
	if d := newSettings.IPHashSaltRotationDays; d != 0 && (d < privacy.MinRotationDays || d > privacy.MaxRotationDays) {
		return nil, ErrInvalidSettings
	}

	now := time.Now().UTC()
	row, err := s.q.UpdateAnalyticsSettings(ctx, db.UpdateAnalyticsSettingsParams{
		Enabled:                pgtype.Bool{Bool: newSettings.Enabled, Valid: true},
		EventDeliveryMode:      pgtype.Text{String: newSettings.EventDeliveryMode, Valid: newSettings.EventDeliveryMode != ""},
		RawEventRetentionDays:  pgtype.Int4{Int32: int32(newSettings.RawEventRetentionDays), Valid: newSettings.RawEventRetentionDays > 0},
		AggregateRetentionDays: pgtype.Int4{Int32: int32(newSettings.AggregateRetentionDays), Valid: newSettings.AggregateRetentionDays > 0},
		QueueName:              pgtype.Text{String: newSettings.QueueName, Valid: newSettings.QueueName != ""},
		RealtimeEnabled:        pgtype.Bool{Bool: newSettings.RealtimeEnabled, Valid: true},
		BatchSize:              pgtype.Int4{Int32: int32(newSettings.BatchSize), Valid: newSettings.BatchSize > 0},
		BatchFlushIntervalMs:   pgtype.Int4{Int32: int32(newSettings.BatchFlushIntervalMs), Valid: newSettings.BatchFlushIntervalMs > 0},
		IpHashSaltRotationDays: pgtype.Int4{Int32: int32(newSettings.IPHashSaltRotationDays), Valid: newSettings.IPHashSaltRotationDays > 0},
		Today:                  pgtype.Date{Time: time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC), Valid: true},
		AnonymizeIp:            pgtype.Bool{Bool: newSettings.AnonymizeIP, Valid: true},
	})
	if err != nil {
		return nil, fmt.Errorf("update analytics settings: %w", err)
	}

	st := s.mapRowToSettings(row)
	s.mu.Lock()
	s.settings = st
	s.settingsAt = time.Now()
	s.mu.Unlock()

	// Invalidate redis cache if configured
	if s.rdb != nil {
		_ = s.rdb.Del(ctx, "analytics:settings").Err()
	}

	return &st, nil
}

func (s *AnalyticsService) loadSettings(ctx context.Context) error {
	if s.q == nil {
		return nil
	}

	// Try Redis cache first
	if s.rdb != nil {
		if val, err := s.rdb.Get(ctx, "analytics:settings").Bytes(); err == nil {
			var cached AnalyticsSettings
			if json.Unmarshal(val, &cached) == nil {
				s.mu.Lock()
				s.settings = cached
				s.settingsAt = time.Now()
				s.mu.Unlock()
				return nil
			}
		}
	}

	row, err := s.q.GetAnalyticsSettings(ctx)
	if err != nil {
		return err
	}

	st := s.mapRowToSettings(row)
	s.mu.Lock()
	s.settings = st
	s.settingsAt = time.Now()
	s.mu.Unlock()

	if s.rdb != nil {
		if data, err := json.Marshal(st); err == nil {
			_ = s.rdb.Set(ctx, "analytics:settings", data, 5*time.Minute).Err()
		}
	}

	return nil
}

func (s *AnalyticsService) mapRowToSettings(r db.AnalyticsSetting) AnalyticsSettings {
	st := AnalyticsSettings{
		Enabled:                r.Enabled,
		EventDeliveryMode:      r.EventDeliveryMode,
		RawEventRetentionDays:  int(r.RawEventRetentionDays),
		AggregateRetentionDays: int(r.AggregateRetentionDays),
		QueueName:              r.QueueName,
		RealtimeEnabled:        r.RealtimeEnabled,
		BatchSize:              int(r.BatchSize),
		BatchFlushIntervalMs:   int(r.BatchFlushIntervalMs),
		IPHashSaltRotationDays: int(r.IpHashSaltRotationDays),
		AnonymizeIP:            r.AnonymizeIp,
	}
	if r.IpHashSaltRotationPreviousDays.Valid && r.IpHashSaltRotationChangedOn.Valid {
		st.IPHashSaltRotationPreviousDays = int(r.IpHashSaltRotationPreviousDays.Int32)
		st.IPHashSaltRotationChangedOn = r.IpHashSaltRotationChangedOn.Time.Format(time.DateOnly)
	}
	return st
}

// SaltRotationSchedule is the IP hash salt rotation schedule from the cached settings (TEC-936);
// the collector's hasher calls it for every event. Unreadable settings fall back to 7 days.
func (s *AnalyticsService) SaltRotationSchedule() privacy.RotationSchedule {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	st, _ := s.GetSettings(ctx)
	out := privacy.RotationSchedule{Days: st.IPHashSaltRotationDays, PreviousDays: st.IPHashSaltRotationPreviousDays}
	if st.IPHashSaltRotationChangedOn != "" {
		if d, err := time.Parse(time.DateOnly, st.IPHashSaltRotationChangedOn); err == nil {
			out.ChangedOn = d
		}
	}
	return out
}

func validateDateRange(filter *domain.AnalyticsFilter) error {
	if filter.DateFrom.IsZero() {
		filter.DateFrom = time.Now().UTC().AddDate(0, 0, -30)
	}
	if filter.DateTo.IsZero() {
		filter.DateTo = time.Now().UTC()
	}
	if filter.DateFrom.After(filter.DateTo) {
		return errors.New("date_from cannot be after date_to")
	}
	// Max date range: 365 days
	if filter.DateTo.Sub(filter.DateFrom) > 366*24*time.Hour {
		return errors.New("date range cannot exceed 365 days")
	}
	return nil
}
