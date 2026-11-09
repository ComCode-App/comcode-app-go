package domain

import (
	"context"
	"time"
)

// AnalyticsFilter defines criteria for querying metrics.
type AnalyticsFilter struct {
	SubjectType string    `json:"subject_type"`
	SubjectID   string    `json:"subject_id"`
	EventType   *string   `json:"event_type,omitempty"`
	DateFrom    time.Time `json:"date_from"`
	DateTo      time.Time `json:"date_to"`
	Interval    string    `json:"interval,omitempty"` // "hour" or "day"
	Country     *string   `json:"country,omitempty"`
	Device      *string   `json:"device,omitempty"`
}

// EventTypeCount pairs an event type slug with its aggregate frequency.
type EventTypeCount struct {
	EventType string `json:"event_type"`
	Count     int64  `json:"count"`
}

// Summary provides high-level metrics for a subject.
type Summary struct {
	TotalEvents       int64            `json:"total_events"`
	UniqueSessions    int64            `json:"unique_sessions"`
	TopEvents         []EventTypeCount `json:"top_events"`
	TodayLiveCount    int64            `json:"today_live_count"`
	TodayLiveSessions int64            `json:"today_live_sessions"`
}

// TimeSeriesPoint represents aggregated counts for a discrete time bucket.
type TimeSeriesPoint struct {
	Bucket         time.Time `json:"bucket"`
	TotalCount     int64     `json:"total_count"`
	UniqueSessions int64     `json:"unique_sessions"`
}

// TimeSeriesResult wraps time series points.
type TimeSeriesResult struct {
	Interval string            `json:"interval"`
	Points   []TimeSeriesPoint `json:"points"`
	Total    int64             `json:"total"`
}

// BreakdownItem represents count and percentage for a single dimension value.
type BreakdownItem struct {
	Key        string  `json:"key"`
	Count      int64   `json:"count"`
	Percentage float64 `json:"percentage"`
}

// BreakdownResult holds breakdown distribution for a specific dimension.
type BreakdownResult struct {
	Dimension string          `json:"dimension"`
	Items     []BreakdownItem `json:"items"`
	Total     int64           `json:"total"`
}

// RawEventFilter specifies bounded parameters for raw event drilldown.
type RawEventFilter struct {
	SubjectType *string
	SubjectID   *string
	EventType   *string
	CountryCode *string
	DateFrom    *time.Time
	DateTo      *time.Time
	Limit       int
	Offset      int
}

// RawEventCountCap bounds the raw events list count: a request counts at most this many matching
// rows instead of the whole (partitioned, unbounded) events table (TEC-859).
const RawEventCountCap = 10000

// RawEventListResult returns paginated raw events.
type RawEventListResult struct {
	Items []Event `json:"items"`
	// Total is the number of matching events, capped at RawEventCountCap.
	Total int64 `json:"total"`
	// TotalCapped is true when more than RawEventCountCap events match; Total is then a lower bound.
	TotalCapped bool `json:"total_capped"`
	// HasMore is true when another page exists after this one (fetched with limit + 1).
	HasMore bool `json:"has_more"`
	Limit   int  `json:"limit"`
	Offset  int  `json:"offset"`
}

// DayCount is the event volume of one calendar day in a given time zone.
type DayCount struct {
	// Date is the local calendar day at midnight UTC (only Y-M-D is meaningful).
	Date  time.Time
	Count int64
}

// SourceRow is the rolled-up count of one event type and entry source over a subject and the
// subjects that lead to it (TEC-953).
type SourceRow struct {
	SubjectType string
	EventType   string
	Source      string
	Count       int64
}

// VisitorCounts are distinct visitors over the platform overview windows (TEC-953).
type VisitorCounts struct {
	Today      int64 `json:"today"`
	Last7Days  int64 `json:"last_7_days"`
	Last30Days int64 `json:"last_30_days"`
}

// AnalyticsRepository provides storage-agnostic analytical query capabilities.
// This abstraction allows swapping PostgreSQL aggregate tables with ClickHouse or other OLAP stores seamlessly.
type AnalyticsRepository interface {
	GetSummary(ctx context.Context, filter AnalyticsFilter) (*Summary, error)
	GetTimeSeries(ctx context.Context, filter AnalyticsFilter) (*TimeSeriesResult, error)
	GetBreakdown(ctx context.Context, filter AnalyticsFilter, dimension string) (*BreakdownResult, error)
	GetTopEvents(ctx context.Context, filter AnalyticsFilter, limit int) ([]EventTypeCount, error)
	GetRawEvents(ctx context.Context, filter RawEventFilter) (*RawEventListResult, error)
	// GetPlatformDailyCounts sums every subject's hourly rollups per local day of loc for
	// [from, to). Platform-wide and unscoped: callers gate it behind platform.analytics.read.
	GetPlatformDailyCounts(ctx context.Context, from, to time.Time, loc *time.Location) ([]DayCount, error)
	// GetSourceCounts sums the daily rollups of a subject and its related subjects (a profile's
	// routes and cards, a route's card, a card's routes; same account only) per event type and
	// source over the UTC days [from, to].
	GetSourceCounts(ctx context.Context, subjectType, subjectID string, from, to time.Time) ([]SourceRow, error)
	// GetPlatformVisitors counts distinct visitors since todayFrom, weekFrom and from, until to.
	// Platform-wide and unscoped like GetPlatformDailyCounts.
	GetPlatformVisitors(ctx context.Context, todayFrom, weekFrom, from, to time.Time) (VisitorCounts, error)
	// GetPlatformTopEventTypes returns the most frequent event types in [from, to), platform-wide.
	GetPlatformTopEventTypes(ctx context.Context, from, to time.Time, limit int) ([]EventTypeCount, error)
}
