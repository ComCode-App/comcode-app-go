package application

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/analytics/domain"
)

// PlatformOverviewDays is the look-back of the platform analytics overview (daily series length).
const PlatformOverviewDays = 30

// PlatformTopEventTypes is how many event types the overview ranks (TEC-953).
const PlatformTopEventTypes = 10

// ErrInvalidTimeZone is returned when the requested time zone is not an IANA zone name.
var ErrInvalidTimeZone = errors.New("tz must be an IANA time zone name, e.g. Europe/Istanbul")

// PlatformDayCount is one local calendar day of the overview series.
type PlatformDayCount struct {
	// Date is the local calendar day (yyyy-MM-dd).
	Date  string `json:"date"`
	Count int64  `json:"count"`
}

// PlatformAnalyticsOverview is the GET /v1/platform/analytics/overview payload (TEC-859): platform
// wide event volume from the hourly rollups, bucketed into the viewer's local calendar days.
type PlatformAnalyticsOverview struct {
	GeneratedAt time.Time `json:"generated_at"`
	TimeZone    string    `json:"time_zone"`
	Days        int       `json:"days"`
	// Today counts the current local day so far.
	Today int64 `json:"today"`
	// Last7Days and Last30Days are rolling windows of local days ending today (today included).
	Last7Days  int64 `json:"last_7_days"`
	Last30Days int64 `json:"last_30_days"`
	// Daily has Days points, oldest first, zero for days without events.
	Daily []PlatformDayCount `json:"daily"`
	// UniqueVisitors are distinct visitors (session, else rotating IP hash) over the same windows,
	// from the hourly visitor rollup (TEC-953).
	UniqueVisitors domain.VisitorCounts `json:"unique_visitors"`
	// TopEventTypes are the PlatformTopEventTypes most frequent event types of the Days window.
	TopEventTypes []domain.EventTypeCount `json:"top_event_types"`
}

// ParseTimeZone resolves an IANA zone name; empty means UTC. "Local" is rejected because it
// would mean the server's zone, which Postgres would not resolve the same way.
func ParseTimeZone(name string) (*time.Location, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return time.UTC, nil
	}
	if strings.EqualFold(name, "local") || len(name) > 64 {
		return nil, ErrInvalidTimeZone
	}
	loc, err := time.LoadLocation(name)
	if err != nil {
		return nil, ErrInvalidTimeZone
	}
	return loc, nil
}

// GetPlatformOverview builds the platform analytics KPIs from the aggregate tables only (the
// dashboards never scan raw rows, docs/ANALYTICS_ARCHITECTURE.md). now is injected for tests.
func (s *AnalyticsService) GetPlatformOverview(ctx context.Context, loc *time.Location, now time.Time) (*PlatformAnalyticsOverview, error) {
	if loc == nil {
		loc = time.UTC
	}
	local := now.In(loc)
	todayStart := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, loc)
	from := todayStart.AddDate(0, 0, -(PlatformOverviewDays - 1))
	to := todayStart.AddDate(0, 0, 1)

	rows, err := s.repo.GetPlatformDailyCounts(ctx, from, to, loc)
	if err != nil {
		return nil, err
	}
	visitors, err := s.repo.GetPlatformVisitors(ctx, todayStart, todayStart.AddDate(0, 0, -6), from, to)
	if err != nil {
		return nil, err
	}
	top, err := s.repo.GetPlatformTopEventTypes(ctx, from, to, PlatformTopEventTypes)
	if err != nil {
		return nil, err
	}
	byDay := make(map[string]int64, len(rows))
	for _, r := range rows {
		byDay[r.Date.Format(time.DateOnly)] += r.Count
	}

	out := &PlatformAnalyticsOverview{
		GeneratedAt: now.UTC(),
		TimeZone:    loc.String(),
		Days:        PlatformOverviewDays,
		Daily:       make([]PlatformDayCount, 0, PlatformOverviewDays),

		UniqueVisitors: visitors,
		TopEventTypes:  top,
	}
	for i := 0; i < PlatformOverviewDays; i++ {
		day := from.AddDate(0, 0, i).Format(time.DateOnly)
		c := byDay[day]
		out.Daily = append(out.Daily, PlatformDayCount{Date: day, Count: c})
		out.Last30Days += c
		if i >= PlatformOverviewDays-7 {
			out.Last7Days += c
		}
		if i == PlatformOverviewDays-1 {
			out.Today = c
		}
	}
	return out, nil
}
