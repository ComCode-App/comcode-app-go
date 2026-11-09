package infrastructure_test

// Postgres integration tests (TEC-859). Skipped unless TEST_DATABASE_URL is set; see
// internal/testutil/pgtest. Fixtures use unique subjects or a 2019 window nothing else writes to.

import (
	"context"
	"testing"
	"time"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/analytics/domain"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/analytics/infrastructure"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/testutil/pgtest"
)

func TestMain(m *testing.M) { pgtest.Main(m) }

func TestIntegrationPlatformDailyCountsBucketsLocalDays(t *testing.T) {
	d := pgtest.Open(t)
	ctx := context.Background()
	repo := infrastructure.NewPostgresAnalyticsRepository(d.Pool, d.Queries, nil)

	insert := func(hour, subjectType, subjectID, eventType string, count int) {
		t.Helper()
		if _, err := d.Pool.Exec(ctx, `INSERT INTO event_aggregates_hourly (bucket_hour, subject_type, subject_id, event_type, total_count)
			VALUES ($1::timestamptz, $2, $3, $4, $5)`, hour, subjectType, subjectID, eventType, count); err != nil {
			t.Fatal(err)
		}
	}
	// 2019-05-01 21:00 UTC is 2019-05-02 00:00 in Istanbul (UTC+3).
	insert("2019-05-01 20:00:00+00", "route", "a", "card.view", 2)
	insert("2019-05-01 21:00:00+00", "route", "a", "card.view", 3)
	insert("2019-05-01 21:00:00+00", "profile", "b", "contact.phone.click", 4)
	insert("2019-05-02 10:00:00+00", "card", "c", "card.qr.scan", 5)
	insert("2019-05-03 21:00:00+00", "route", "a", "card.view", 100) // outside [from, to)

	ist, err := time.LoadLocation("Europe/Istanbul")
	if err != nil {
		t.Skip("tzdata unavailable")
	}
	from := time.Date(2019, 5, 1, 0, 0, 0, 0, ist)
	to := time.Date(2019, 5, 3, 0, 0, 0, 0, ist).Add(24 * time.Hour) // 2019-05-04 00:00 local = 05-03 21:00 UTC
	rows, err := repo.GetPlatformDailyCounts(ctx, from, to, ist)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]int64{}
	for _, r := range rows {
		got[r.Date.Format(time.DateOnly)] = r.Count
	}
	want := map[string]int64{"2019-05-01": 2, "2019-05-02": 12}
	if len(got) != len(want) || got["2019-05-01"] != 2 || got["2019-05-02"] != 12 {
		t.Fatalf("istanbul days = %v, want %v", got, want)
	}

	// The same rows in UTC fall on the UTC calendar days.
	rows, err = repo.GetPlatformDailyCounts(ctx, time.Date(2019, 5, 1, 0, 0, 0, 0, time.UTC), time.Date(2019, 5, 3, 0, 0, 0, 0, time.UTC), time.UTC)
	if err != nil {
		t.Fatal(err)
	}
	got = map[string]int64{}
	for _, r := range rows {
		got[r.Date.Format(time.DateOnly)] = r.Count
	}
	if got["2019-05-01"] != 9 || got["2019-05-02"] != 5 || len(got) != 2 {
		t.Fatalf("utc days = %v", got)
	}
}

func TestIntegrationRawEventsBoundedCountAndHasMore(t *testing.T) {
	d := pgtest.Open(t)
	ctx := context.Background()
	repo := infrastructure.NewPostgresAnalyticsRepository(d.Pool, d.Queries, nil)

	seed := func(subjectID string, n int) {
		t.Helper()
		if _, err := d.Pool.Exec(ctx, `INSERT INTO events (event_type, subject_type, subject_id, occurred_at)
			SELECT 'card.view', 'route', $1, '2026-10-01 00:00:00+00'::timestamptz + make_interval(secs => g)
			FROM generate_series(1, $2::int) g`, subjectID, n); err != nil {
			t.Fatal(err)
		}
	}
	list := func(subjectID string, limit, offset int) *domain.RawEventListResult {
		t.Helper()
		st := "route"
		res, err := repo.GetRawEvents(ctx, domain.RawEventFilter{SubjectType: &st, SubjectID: &subjectID, Limit: limit, Offset: offset})
		if err != nil {
			t.Fatal(err)
		}
		return res
	}

	small := "small-" + pgtest.Unique()
	seed(small, 5)
	res := list(small, 2, 0)
	if len(res.Items) != 2 || !res.HasMore || res.Total != 5 || res.TotalCapped {
		t.Fatalf("first page items=%d has_more=%v total=%d capped=%v", len(res.Items), res.HasMore, res.Total, res.TotalCapped)
	}
	if !res.Items[0].OccurredAt.After(res.Items[1].OccurredAt) {
		t.Fatal("want newest first")
	}
	res = list(small, 2, 4)
	if len(res.Items) != 1 || res.HasMore {
		t.Fatalf("last page items=%d has_more=%v", len(res.Items), res.HasMore)
	}
	res = list(small, 5, 0)
	if len(res.Items) != 5 || res.HasMore {
		t.Fatalf("exact page items=%d has_more=%v", len(res.Items), res.HasMore)
	}

	big := "big-" + pgtest.Unique()
	seed(big, domain.RawEventCountCap+3)
	res = list(big, 20, 0)
	if res.Total != domain.RawEventCountCap || !res.TotalCapped || !res.HasMore || len(res.Items) != 20 {
		t.Fatalf("capped total=%d capped=%v has_more=%v items=%d", res.Total, res.TotalCapped, res.HasMore, len(res.Items))
	}
	// Paging past the cap still works through has_more.
	res = list(big, 20, domain.RawEventCountCap)
	if len(res.Items) != 3 || res.HasMore || !res.TotalCapped {
		t.Fatalf("past cap items=%d has_more=%v capped=%v", len(res.Items), res.HasMore, res.TotalCapped)
	}
}
