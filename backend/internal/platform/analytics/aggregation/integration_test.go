package aggregation_test

// Postgres integration tests (TEC-536). Skipped unless TEST_DATABASE_URL is set; see
// internal/testutil/pgtest.

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"
	"time"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/analytics/aggregation"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/analytics/domain"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/testutil/pgtest"
)

func TestMain(m *testing.M) { pgtest.Main(m) }

func TestIntegrationAggregateRollupIsIdempotentAndSumsBreakdowns(t *testing.T) {
	d := pgtest.Open(t)
	ctx := context.Background()
	agg := aggregation.NewAggregator(d.Queries, nil)
	subject := domain.EntityReference{Type: "route", ID: "r-" + pgtest.Unique()}
	at := time.Date(2026, 3, 4, 10, 15, 0, 0, time.UTC)

	ev := func(id int64, offset time.Duration, country, session string) domain.Event {
		return domain.Event{
			ID: id, Type: "route.scan", Subject: subject, SessionID: session, OccurredAt: at.Add(offset),
			Context: domain.EventContext{CountryCode: country, DeviceType: "mobile", Referer: "qr"},
		}
	}
	base := time.Now().UnixNano() % 1_000_000_000
	events := []domain.Event{
		ev(base+1, 0, "TR", "s1"),
		ev(base+2, 10*time.Minute, "TR", ""),
		ev(base+3, 2*time.Hour, "DE", "s2"),
	}
	if err := agg.AggregateBatch(ctx, events); err != nil {
		t.Fatal(err)
	}
	// Redelivered events (queue retries) must not be counted twice.
	if err := agg.AggregateEvent(ctx, events[0]); err != nil {
		t.Fatal(err)
	}
	if err := agg.AggregateBatch(ctx, events); err != nil {
		t.Fatal(err)
	}

	var total, sessions int64
	var countryRaw, deviceRaw, refererRaw []byte
	err := d.Pool.QueryRow(ctx, `SELECT total_count, unique_sessions, country_breakdown, device_breakdown, referer_breakdown
		FROM event_aggregates_daily WHERE subject_type = $1 AND subject_id = $2 AND event_type = 'route.scan' AND bucket_date = '2026-03-04'`,
		subject.Type, subject.ID).Scan(&total, &sessions, &countryRaw, &deviceRaw, &refererRaw)
	if err != nil {
		t.Fatalf("daily aggregate: %v", err)
	}
	if total != 3 || sessions != 2 {
		t.Fatalf("daily total=%d sessions=%d, want 3 and 2", total, sessions)
	}
	assertCounts(t, "country", countryRaw, map[string]int64{"TR": 2, "DE": 1})
	assertCounts(t, "device", deviceRaw, map[string]int64{"mobile": 3})
	assertCounts(t, "referer", refererRaw, map[string]int64{"qr": 3})

	rows, err := d.Pool.Query(ctx, `SELECT bucket_hour, total_count FROM event_aggregates_hourly
		WHERE subject_type = $1 AND subject_id = $2 ORDER BY bucket_hour`, subject.Type, subject.ID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	got := map[int]int64{}
	for rows.Next() {
		var hour time.Time
		var n int64
		if err := rows.Scan(&hour, &n); err != nil {
			t.Fatal(err)
		}
		got[hour.UTC().Hour()] = n
	}
	if want := map[int]int64{10: 2, 12: 1}; !reflect.DeepEqual(got, want) {
		t.Fatalf("hourly buckets = %v, want %v", got, want)
	}

	if err := agg.AggregateEvent(ctx, domain.Event{Type: "route.scan", Subject: subject}); err == nil {
		t.Fatal("an event without an id must be rejected")
	}
}

func assertCounts(t *testing.T, name string, raw []byte, want map[string]int64) {
	t.Helper()
	got := map[string]int64{}
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("%s breakdown %s: %v", name, raw, err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("%s breakdown = %v, want %v", name, got, want)
	}
}
