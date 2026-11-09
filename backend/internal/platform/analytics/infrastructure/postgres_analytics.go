package infrastructure

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/database/db"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/analytics/domain"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/analytics/realtime"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresAnalyticsRepository implements AnalyticsRepository by querying rollups without scanning raw event partitions.
type PostgresAnalyticsRepository struct {
	pool     *pgxpool.Pool
	q        *db.Queries
	counters *realtime.CounterManager
}

// NewPostgresAnalyticsRepository creates a new PostgresAnalyticsRepository.
func NewPostgresAnalyticsRepository(pool *pgxpool.Pool, q *db.Queries, counters *realtime.CounterManager) *PostgresAnalyticsRepository {
	return &PostgresAnalyticsRepository{
		pool:     pool,
		q:        q,
		counters: counters,
	}
}

// GetSummary returns metric totals, top events, and live counters for a subject.
func (r *PostgresAnalyticsRepository) GetSummary(ctx context.Context, filter domain.AnalyticsFilter) (*domain.Summary, error) {
	fromDate := pgtype.Date{
		Time:  time.Date(filter.DateFrom.Year(), filter.DateFrom.Month(), filter.DateFrom.Day(), 0, 0, 0, 0, time.UTC),
		Valid: true,
	}
	toDate := pgtype.Date{
		Time:  time.Date(filter.DateTo.Year(), filter.DateTo.Month(), filter.DateTo.Day(), 0, 0, 0, 0, time.UTC),
		Valid: true,
	}

	var evType pgtype.Text
	if filter.EventType != nil && *filter.EventType != "" {
		evType = pgtype.Text{String: *filter.EventType, Valid: true}
	}

	rows, err := r.q.GetDailyAggregatesSummary(ctx, db.GetDailyAggregatesSummaryParams{
		SubjectType:  filter.SubjectType,
		SubjectID:    filter.SubjectID,
		EventType:    evType,
		BucketDate:   fromDate,
		BucketDate_2: toDate,
	})
	if err != nil {
		return nil, fmt.Errorf("get summary aggregates: %w", err)
	}

	var totalEvents int64
	for _, row := range rows {
		totalEvents += row.TotalEvents
	}
	// Distinct visitors come from raw events; the aggregates' unique_sessions is an event count.
	uniqueSessions, err := r.q.CountDistinctVisitors(ctx, db.CountDistinctVisitorsParams{
		SubjectType: filter.SubjectType,
		SubjectID:   filter.SubjectID,
		FromAt:      pgtype.Timestamptz{Time: fromDate.Time, Valid: true},
		ToBefore:    pgtype.Timestamptz{Time: toDate.Time.AddDate(0, 0, 1), Valid: true},
		EventType:   evType,
	})
	if err != nil {
		return nil, fmt.Errorf("count distinct visitors: %w", err)
	}

	topEvents, _ := r.GetTopEvents(ctx, filter, 5)

	var liveToday, liveSessions int64
	if r.counters != nil {
		liveToday, _ = r.counters.GetTodayTotal(ctx, filter.SubjectType, filter.SubjectID, filter.EventType)
		liveSessions, _ = r.counters.GetTodayUniqueSessions(ctx, filter.SubjectType, filter.SubjectID)
	}

	return &domain.Summary{
		TotalEvents:       totalEvents,
		UniqueSessions:    uniqueSessions,
		TopEvents:         topEvents,
		TodayLiveCount:    liveToday,
		TodayLiveSessions: liveSessions,
	}, nil
}

// GetTimeSeries returns discrete data points over the requested timeframe.
func (r *PostgresAnalyticsRepository) GetTimeSeries(ctx context.Context, filter domain.AnalyticsFilter) (*domain.TimeSeriesResult, error) {
	interval := filter.Interval
	if interval == "" {
		interval = "day"
	}

	var evType pgtype.Text
	if filter.EventType != nil && *filter.EventType != "" {
		evType = pgtype.Text{String: *filter.EventType, Valid: true}
	}

	var totalCount int64
	pointsMap := make(map[time.Time]*domain.TimeSeriesPoint)

	if interval == "hour" {
		rows, err := r.q.GetHourlyAggregates(ctx, db.GetHourlyAggregatesParams{
			SubjectType:  filter.SubjectType,
			SubjectID:    filter.SubjectID,
			EventType:    evType,
			BucketHour:   pgtype.Timestamptz{Time: filter.DateFrom, Valid: true},
			BucketHour_2: pgtype.Timestamptz{Time: filter.DateTo, Valid: true},
		})
		if err != nil {
			return nil, fmt.Errorf("get hourly aggregates: %w", err)
		}
		for _, row := range rows {
			bHour := row.BucketHour.Time.UTC()
			pt, exists := pointsMap[bHour]
			if !exists {
				pt = &domain.TimeSeriesPoint{Bucket: bHour}
				pointsMap[bHour] = pt
			}
			pt.TotalCount += row.TotalCount
			totalCount += row.TotalCount
		}
		visitors, err := r.q.DistinctVisitorsPerHour(ctx, db.DistinctVisitorsPerHourParams{
			SubjectType: filter.SubjectType,
			SubjectID:   filter.SubjectID,
			FromAt:      pgtype.Timestamptz{Time: filter.DateFrom, Valid: true},
			ToAt:        pgtype.Timestamptz{Time: filter.DateTo, Valid: true},
			EventType:   evType,
		})
		if err != nil {
			return nil, fmt.Errorf("distinct visitors per hour: %w", err)
		}
		for _, v := range visitors {
			if pt, ok := pointsMap[v.Hour.Time.UTC()]; ok {
				pt.UniqueSessions = v.Visitors
			}
		}
	} else {
		fromDate := pgtype.Date{
			Time:  time.Date(filter.DateFrom.Year(), filter.DateFrom.Month(), filter.DateFrom.Day(), 0, 0, 0, 0, time.UTC),
			Valid: true,
		}
		toDate := pgtype.Date{
			Time:  time.Date(filter.DateTo.Year(), filter.DateTo.Month(), filter.DateTo.Day(), 0, 0, 0, 0, time.UTC),
			Valid: true,
		}
		rows, err := r.q.GetDailyAggregates(ctx, db.GetDailyAggregatesParams{
			SubjectType:  filter.SubjectType,
			SubjectID:    filter.SubjectID,
			EventType:    evType,
			BucketDate:   fromDate,
			BucketDate_2: toDate,
		})
		if err != nil {
			return nil, fmt.Errorf("get daily aggregates: %w", err)
		}
		for _, row := range rows {
			bDate := row.BucketDate.Time.UTC()
			pt, exists := pointsMap[bDate]
			if !exists {
				pt = &domain.TimeSeriesPoint{Bucket: bDate}
				pointsMap[bDate] = pt
			}
			pt.TotalCount += row.TotalCount
			totalCount += row.TotalCount
		}
		visitors, err := r.q.DistinctVisitorsPerDay(ctx, db.DistinctVisitorsPerDayParams{
			SubjectType: filter.SubjectType,
			SubjectID:   filter.SubjectID,
			FromAt:      pgtype.Timestamptz{Time: fromDate.Time, Valid: true},
			ToBefore:    pgtype.Timestamptz{Time: toDate.Time.AddDate(0, 0, 1), Valid: true},
			EventType:   evType,
		})
		if err != nil {
			return nil, fmt.Errorf("distinct visitors per day: %w", err)
		}
		for _, v := range visitors {
			if pt, ok := pointsMap[v.Day.Time.UTC()]; ok {
				pt.UniqueSessions = v.Visitors
			}
		}
	}

	// Sort points chronologically
	points := make([]domain.TimeSeriesPoint, 0, len(pointsMap))
	for _, pt := range pointsMap {
		points = append(points, *pt)
	}
	sort.Slice(points, func(i, j int) bool {
		return points[i].Bucket.Before(points[j].Bucket)
	})

	return &domain.TimeSeriesResult{
		Interval: interval,
		Points:   points,
		Total:    totalCount,
	}, nil
}

// GetBreakdown extracts aggregated frequency distributions for dimensions (country, city, device, browser, os, referer).
func (r *PostgresAnalyticsRepository) GetBreakdown(ctx context.Context, filter domain.AnalyticsFilter, dimension string) (*domain.BreakdownResult, error) {
	fromDate := pgtype.Date{
		Time:  time.Date(filter.DateFrom.Year(), filter.DateFrom.Month(), filter.DateFrom.Day(), 0, 0, 0, 0, time.UTC),
		Valid: true,
	}
	toDate := pgtype.Date{
		Time:  time.Date(filter.DateTo.Year(), filter.DateTo.Month(), filter.DateTo.Day(), 0, 0, 0, 0, time.UTC),
		Valid: true,
	}

	var evType pgtype.Text
	if filter.EventType != nil && *filter.EventType != "" {
		evType = pgtype.Text{String: *filter.EventType, Valid: true}
	}

	rows, err := r.q.GetDailyAggregates(ctx, db.GetDailyAggregatesParams{
		SubjectType:  filter.SubjectType,
		SubjectID:    filter.SubjectID,
		EventType:    evType,
		BucketDate:   fromDate,
		BucketDate_2: toDate,
	})
	if err != nil {
		return nil, fmt.Errorf("get daily aggregates for breakdown: %w", err)
	}

	counts := make(map[string]int64)
	var grandTotal int64

	for _, row := range rows {
		var rawJSON []byte
		switch dimension {
		case "country":
			rawJSON = row.CountryBreakdown
		case "city":
			rawJSON = row.CityBreakdown
		case "device":
			rawJSON = row.DeviceBreakdown
		case "browser":
			rawJSON = row.BrowserBreakdown
		case "os":
			rawJSON = row.OsBreakdown
		case "referer":
			rawJSON = row.RefererBreakdown
		default:
			rawJSON = row.CountryBreakdown
		}

		if len(rawJSON) > 0 {
			var m map[string]int64
			if err := json.Unmarshal(rawJSON, &m); err == nil {
				for k, v := range m {
					if k != "" {
						counts[k] += v
						grandTotal += v
					}
				}
			}
		}
	}

	items := make([]domain.BreakdownItem, 0, len(counts))
	for k, v := range counts {
		pct := 0.0
		if grandTotal > 0 {
			pct = (float64(v) / float64(grandTotal)) * 100.0
		}
		items = append(items, domain.BreakdownItem{
			Key:        k,
			Count:      v,
			Percentage: pct,
		})
	}

	sort.Slice(items, func(i, j int) bool {
		return items[i].Count > items[j].Count
	})

	return &domain.BreakdownResult{
		Dimension: dimension,
		Items:     items,
		Total:     grandTotal,
	}, nil
}

// GetTopEvents returns highest frequency event types for a subject.
func (r *PostgresAnalyticsRepository) GetTopEvents(ctx context.Context, filter domain.AnalyticsFilter, limit int) ([]domain.EventTypeCount, error) {
	if limit <= 0 {
		limit = 10
	}
	fromDate := pgtype.Date{
		Time:  time.Date(filter.DateFrom.Year(), filter.DateFrom.Month(), filter.DateFrom.Day(), 0, 0, 0, 0, time.UTC),
		Valid: true,
	}
	toDate := pgtype.Date{
		Time:  time.Date(filter.DateTo.Year(), filter.DateTo.Month(), filter.DateTo.Day(), 0, 0, 0, 0, time.UTC),
		Valid: true,
	}

	rows, err := r.q.GetTopEventTypes(ctx, db.GetTopEventTypesParams{
		SubjectType:  filter.SubjectType,
		SubjectID:    filter.SubjectID,
		BucketDate:   fromDate,
		BucketDate_2: toDate,
		LimitCount:   int32(limit),
	})
	if err != nil {
		return nil, fmt.Errorf("get top events: %w", err)
	}

	result := make([]domain.EventTypeCount, 0, len(rows))
	for _, row := range rows {
		result = append(result, domain.EventTypeCount{
			EventType: row.EventType,
			Count:     row.TotalCount,
		})
	}
	return result, nil
}

// GetRawEvents returns bounded, paginated raw events with index filtering.
func (r *PostgresAnalyticsRepository) GetRawEvents(ctx context.Context, filter domain.RawEventFilter) (*domain.RawEventListResult, error) {
	limit := filter.Limit
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	offset := filter.Offset
	if offset < 0 {
		offset = 0
	}

	var subType, subID, evType, countryCode pgtype.Text
	if filter.SubjectType != nil && *filter.SubjectType != "" {
		subType = pgtype.Text{String: *filter.SubjectType, Valid: true}
	}
	if filter.SubjectID != nil && *filter.SubjectID != "" {
		subID = pgtype.Text{String: *filter.SubjectID, Valid: true}
	}
	if filter.EventType != nil && *filter.EventType != "" {
		evType = pgtype.Text{String: *filter.EventType, Valid: true}
	}
	if filter.CountryCode != nil && *filter.CountryCode != "" {
		countryCode = pgtype.Text{String: *filter.CountryCode, Valid: true}
	}

	var dateFrom, dateTo pgtype.Timestamptz
	if filter.DateFrom != nil && !filter.DateFrom.IsZero() {
		dateFrom = pgtype.Timestamptz{Time: *filter.DateFrom, Valid: true}
	}
	if filter.DateTo != nil && !filter.DateTo.IsZero() {
		dateTo = pgtype.Timestamptz{Time: *filter.DateTo, Valid: true}
	}

	// One extra row tells whether a next page exists without counting the table.
	rows, err := r.q.ListEventsPaginated(ctx, db.ListEventsPaginatedParams{
		SubjectType:  subType,
		SubjectID:    subID,
		EventType:    evType,
		CountryCode:  countryCode,
		OccurredFrom: dateFrom,
		OccurredTo:   dateTo,
		LimitCount:   int32(limit + 1),
		OffsetCount:  int32(offset),
	})
	if err != nil {
		return nil, fmt.Errorf("list raw events: %w", err)
	}
	hasMore := len(rows) > limit
	if hasMore {
		rows = rows[:limit]
	}

	total, err := r.q.CountEventsFilteredCapped(ctx, db.CountEventsFilteredCappedParams{
		SubjectType:  subType,
		SubjectID:    subID,
		EventType:    evType,
		CountryCode:  countryCode,
		OccurredFrom: dateFrom,
		OccurredTo:   dateTo,
		CapCount:     domain.RawEventCountCap + 1,
	})
	if err != nil {
		return nil, fmt.Errorf("count raw events: %w", err)
	}
	capped := total > domain.RawEventCountCap
	if capped {
		total = domain.RawEventCountCap
	}

	items := make([]domain.Event, 0, len(rows))
	for _, row := range rows {
		items = append(items, mapDBEventToDomain(row))
	}

	return &domain.RawEventListResult{
		Items:       items,
		Total:       total,
		TotalCapped: capped,
		HasMore:     hasMore,
		Limit:       limit,
		Offset:      offset,
	}, nil
}

// GetPlatformDailyCounts sums the hourly rollups of every subject per local calendar day.
func (r *PostgresAnalyticsRepository) GetPlatformDailyCounts(ctx context.Context, from, to time.Time, loc *time.Location) ([]domain.DayCount, error) {
	if loc == nil {
		loc = time.UTC
	}
	rows, err := r.q.PlatformEventCountsByLocalDay(ctx, db.PlatformEventCountsByLocalDayParams{
		TimeZone: loc.String(),
		FromAt:   pgtype.Timestamptz{Time: from, Valid: true},
		ToBefore: pgtype.Timestamptz{Time: to, Valid: true},
	})
	if err != nil {
		return nil, fmt.Errorf("platform daily event counts: %w", err)
	}
	out := make([]domain.DayCount, 0, len(rows))
	for _, row := range rows {
		d := row.Day.Time
		out = append(out, domain.DayCount{Date: time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, time.UTC), Count: row.Count})
	}
	return out, nil
}

// GetSourceCounts reads the per-source daily rollups of a subject and the subjects leading to it.
func (r *PostgresAnalyticsRepository) GetSourceCounts(ctx context.Context, subjectType, subjectID string, from, to time.Time) ([]domain.SourceRow, error) {
	rows, err := r.q.SubjectSourceCounts(ctx, db.SubjectSourceCountsParams{
		SubjectType: subjectType,
		SubjectID:   subjectID,
		FromDate:    pgtype.Date{Time: utcDate(from), Valid: true},
		ToDate:      pgtype.Date{Time: utcDate(to), Valid: true},
	})
	if err != nil {
		return nil, fmt.Errorf("subject source counts: %w", err)
	}
	out := make([]domain.SourceRow, 0, len(rows))
	for _, row := range rows {
		out = append(out, domain.SourceRow{SubjectType: row.SubjectType, EventType: row.EventType, Source: row.Source, Count: row.Total})
	}
	return out, nil
}

// GetPlatformVisitors counts distinct visitors from the hourly visitor rollup.
func (r *PostgresAnalyticsRepository) GetPlatformVisitors(ctx context.Context, todayFrom, weekFrom, from, to time.Time) (domain.VisitorCounts, error) {
	row, err := r.q.PlatformUniqueVisitors(ctx, db.PlatformUniqueVisitorsParams{
		TodayFrom: pgtype.Timestamptz{Time: todayFrom, Valid: true},
		WeekFrom:  pgtype.Timestamptz{Time: weekFrom, Valid: true},
		FromAt:    pgtype.Timestamptz{Time: from, Valid: true},
		ToBefore:  pgtype.Timestamptz{Time: to, Valid: true},
	})
	if err != nil {
		return domain.VisitorCounts{}, fmt.Errorf("platform unique visitors: %w", err)
	}
	return domain.VisitorCounts{Today: row.Today, Last7Days: row.Last7Days, Last30Days: row.Last30Days}, nil
}

// GetPlatformTopEventTypes ranks event types by volume from the hourly rollups.
func (r *PostgresAnalyticsRepository) GetPlatformTopEventTypes(ctx context.Context, from, to time.Time, limit int) ([]domain.EventTypeCount, error) {
	rows, err := r.q.PlatformTopEventTypes(ctx, db.PlatformTopEventTypesParams{
		FromAt:     pgtype.Timestamptz{Time: from, Valid: true},
		ToBefore:   pgtype.Timestamptz{Time: to, Valid: true},
		LimitCount: int32(limit),
	})
	if err != nil {
		return nil, fmt.Errorf("platform top event types: %w", err)
	}
	out := make([]domain.EventTypeCount, 0, len(rows))
	for _, row := range rows {
		out = append(out, domain.EventTypeCount{EventType: row.EventType, Count: row.Count})
	}
	return out, nil
}

func utcDate(t time.Time) time.Time {
	t = t.UTC()
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}

func mapDBEventToDomain(r db.Event) domain.Event {
	var props map[string]any
	_ = json.Unmarshal(r.Properties, &props)
	if props == nil {
		props = make(map[string]any)
	}

	var ctx domain.EventContext
	_ = json.Unmarshal(r.Context, &ctx)

	var meta domain.EventMetadata
	_ = json.Unmarshal(r.Metadata, &meta)

	var actor *domain.EntityReference
	if r.ActorType.Valid && r.ActorID.Valid {
		actor = &domain.EntityReference{
			Type: r.ActorType.String,
			ID:   r.ActorID.String,
		}
	}

	return domain.Event{
		ID:            r.ID,
		UUID:          r.Uuid.String(),
		Type:          r.EventType,
		SchemaVersion: int(r.SchemaVersion),
		Subject: domain.EntityReference{
			Type: r.SubjectType,
			ID:   r.SubjectID,
		},
		Actor:      actor,
		SessionID:  r.SessionID.String,
		RequestID:  r.RequestID.String,
		OccurredAt: r.OccurredAt.Time,
		CreatedAt:  r.CreatedAt.Time,
		Properties: props,
		Context:    ctx,
		Metadata:   meta,
	}
}
