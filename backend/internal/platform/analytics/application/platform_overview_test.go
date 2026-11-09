package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/analytics/domain"
)

// fakeRepo records the platform overview calls; the other methods are unused here.
type fakeRepo struct {
	domain.AnalyticsRepository
	rows     []domain.DayCount
	err      error
	from, to time.Time
	loc      *time.Location

	visitors       domain.VisitorCounts
	visitorWindows [4]time.Time
	top            []domain.EventTypeCount
	topFrom, topTo time.Time
	topLimit       int
}

func (f *fakeRepo) GetPlatformVisitors(_ context.Context, todayFrom, weekFrom, from, to time.Time) (domain.VisitorCounts, error) {
	f.visitorWindows = [4]time.Time{todayFrom, weekFrom, from, to}
	return f.visitors, nil
}

func (f *fakeRepo) GetPlatformTopEventTypes(_ context.Context, from, to time.Time, limit int) ([]domain.EventTypeCount, error) {
	f.topFrom, f.topTo, f.topLimit = from, to, limit
	return f.top, nil
}

func (f *fakeRepo) GetPlatformDailyCounts(_ context.Context, from, to time.Time, loc *time.Location) ([]domain.DayCount, error) {
	f.from, f.to, f.loc = from, to, loc
	return f.rows, f.err
}

func day(s string) time.Time {
	t, _ := time.Parse(time.DateOnly, s)
	return t
}

func TestGetPlatformOverviewWindowsAndSeries(t *testing.T) {
	ist, err := time.LoadLocation("Europe/Istanbul")
	if err != nil {
		t.Skip("tzdata unavailable")
	}
	// 2026-10-21 01:30 in Istanbul is still 2026-10-20 in UTC: the local day must win.
	now := time.Date(2026, 10, 20, 22, 30, 0, 0, time.UTC)
	repo := &fakeRepo{rows: []domain.DayCount{
		{Date: day("2026-10-21"), Count: 5},  // today
		{Date: day("2026-10-15"), Count: 7},  // 7th day back (inside the 7-day window)
		{Date: day("2026-10-14"), Count: 11}, // 8th day back (30-day window only)
		{Date: day("2026-09-22"), Count: 13}, // first day of the 30-day window
	},
		visitors: domain.VisitorCounts{Today: 2, Last7Days: 9, Last30Days: 20},
		top:      []domain.EventTypeCount{{EventType: "card.view", Count: 30}, {EventType: "route.scan", Count: 6}},
	}
	svc := &AnalyticsService{repo: repo}

	got, err := svc.GetPlatformOverview(context.Background(), ist, now)
	if err != nil {
		t.Fatal(err)
	}
	wantFrom := time.Date(2026, 9, 22, 0, 0, 0, 0, ist)
	wantTo := time.Date(2026, 10, 22, 0, 0, 0, 0, ist)
	if !repo.from.Equal(wantFrom) || !repo.to.Equal(wantTo) || repo.loc != ist {
		t.Fatalf("window = [%s, %s) %v, want [%s, %s)", repo.from, repo.to, repo.loc, wantFrom, wantTo)
	}
	if got.Today != 5 || got.Last7Days != 12 || got.Last30Days != 36 {
		t.Fatalf("kpis today=%d 7d=%d 30d=%d", got.Today, got.Last7Days, got.Last30Days)
	}
	if len(got.Daily) != PlatformOverviewDays || got.Daily[0].Date != "2026-09-22" || got.Daily[0].Count != 13 ||
		got.Daily[29].Date != "2026-10-21" || got.Daily[29].Count != 5 || got.Daily[1].Count != 0 {
		t.Fatalf("daily series = %+v", got.Daily)
	}
	if got.TimeZone != "Europe/Istanbul" || got.Days != 30 {
		t.Fatalf("meta tz=%s days=%d", got.TimeZone, got.Days)
	}
	// Unique visitors and top event types use the same local-day windows (TEC-953).
	wantVisitorWindows := [4]time.Time{
		time.Date(2026, 10, 21, 0, 0, 0, 0, ist), time.Date(2026, 10, 15, 0, 0, 0, 0, ist), wantFrom, wantTo,
	}
	for i, w := range wantVisitorWindows {
		if !repo.visitorWindows[i].Equal(w) {
			t.Fatalf("visitor window %d = %s, want %s", i, repo.visitorWindows[i], w)
		}
	}
	if got.UniqueVisitors != repo.visitors {
		t.Fatalf("unique visitors = %+v", got.UniqueVisitors)
	}
	if !repo.topFrom.Equal(wantFrom) || !repo.topTo.Equal(wantTo) || repo.topLimit != 10 || len(got.TopEventTypes) != 2 {
		t.Fatalf("top event types [%s, %s) limit %d -> %+v", repo.topFrom, repo.topTo, repo.topLimit, got.TopEventTypes)
	}
}

func TestGetPlatformOverviewRepoError(t *testing.T) {
	svc := &AnalyticsService{repo: &fakeRepo{err: errors.New("boom")}}
	if _, err := svc.GetPlatformOverview(context.Background(), nil, time.Now()); err == nil {
		t.Fatal("want error")
	}
}

func TestParseTimeZone(t *testing.T) {
	if loc, err := ParseTimeZone(""); err != nil || loc != time.UTC {
		t.Fatalf("empty = %v, %v", loc, err)
	}
	if _, err := ParseTimeZone("UTC"); err != nil {
		t.Fatal(err)
	}
	for _, bad := range []string{"Local", "local", "Mars/Olympus", "../etc/passwd"} {
		if _, err := ParseTimeZone(bad); !errors.Is(err, ErrInvalidTimeZone) {
			t.Fatalf("%q: err = %v", bad, err)
		}
	}
}
