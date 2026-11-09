package infrastructure_test

// Postgres integration tests for TEC-953 (entry sources, platform visitors) and TEC-936 (salt
// rotation schedule). Skipped unless TEST_DATABASE_URL is set.

import (
	"context"
	"testing"
	"time"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/database/db"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/analytics/aggregation"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/analytics/application"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/analytics/domain"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/analytics/infrastructure"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/testutil/pgtest"
	"github.com/jackc/pgx/v5/pgtype"
)

func TestIntegrationProfileSourcesIncludeItsCardAndLegacyRoutes(t *testing.T) {
	d := pgtest.Open(t)
	ctx := context.Background()
	repo := infrastructure.NewPostgresAnalyticsRepository(d.Pool, d.Queries, nil)
	agg := aggregation.NewAggregator(d.Queries, nil)

	scalar := func(dst any, sql string, args ...any) {
		t.Helper()
		if err := d.Pool.QueryRow(ctx, sql, args...).Scan(dst); err != nil {
			t.Fatal(err)
		}
	}
	owner := d.CreateUser(t)
	other := d.CreateUser(t)
	var profileID int64
	var profileUUID, cardRoute, legacyRoute, smartLink, foreignRoute string
	scalar(&profileID, `INSERT INTO profiles(account_id) VALUES ($1) RETURNING id`, owner.AccountID)
	scalar(&profileUUID, `SELECT uuid::text FROM profiles WHERE id = $1`, profileID)
	card := d.CreateCard(t, owner)
	if _, err := d.Pool.Exec(ctx, `UPDATE nfc_cards SET profile_id = $1 WHERE id = $2`, profileID, card.ID); err != nil {
		t.Fatal(err)
	}
	scalar(&cardRoute, `INSERT INTO routes(account_id,card_id,path,route_type,status,is_immutable,profile_id)
		VALUES ($1,$2,'c/'||$3,'card_route','active',true,$4) RETURNING uuid::text`, owner.AccountID, card.ID, card.SmartCode, profileID)
	route := func(dst *string, account int64, routeType string) {
		scalar(dst, `INSERT INTO routes(account_id,path,route_type,status,target_type,direct_url,profile_id)
			VALUES ($1,$2,$3,'active','direct_url','https://example.test',$4) RETURNING uuid::text`,
			account, "src-"+pgtest.Unique(), routeType, profileID)
	}
	route(&legacyRoute, owner.AccountID, "legacy_route")
	route(&smartLink, owner.AccountID, "custom_smart_link")
	// Another account's route pointing at this profile must never be counted.
	route(&foreignRoute, other.AccountID, "legacy_route")

	at := time.Date(2019, 6, 10, 9, 0, 0, 0, time.UTC)
	nextID := time.Now().UnixNano() % 1_000_000_000
	events := []domain.Event{}
	add := func(n int, typ, subjectType, subjectID string, props map[string]any) {
		for i := 0; i < n; i++ {
			nextID++
			events = append(events, domain.Event{ID: nextID, Type: typ, OccurredAt: at,
				Subject: domain.EntityReference{Type: subjectType, ID: subjectID}, Properties: props})
		}
	}
	add(5, "card.view", "profile", profileUUID, nil)
	add(4, "route.scan", "route", cardRoute, map[string]any{"route_type": "card_route"})
	add(2, "route.scan", "route", cardRoute, map[string]any{"route_type": "card_route", "entry_source": "qr"})
	add(2, "card.qr.scan", "card", card.UUID.String(), nil)
	add(3, "route.scan", "route", legacyRoute, map[string]any{"route_type": "legacy_route"})
	add(1, "route.scan", "route", smartLink, map[string]any{"route_type": "custom_smart_link"})
	add(9, "route.scan", "route", foreignRoute, map[string]any{"route_type": "legacy_route"})
	if err := agg.AggregateBatch(ctx, events); err != nil {
		t.Fatal(err)
	}

	rows, err := repo.GetSourceCounts(ctx, "profile", profileUUID, at, at)
	if err != nil {
		t.Fatal(err)
	}
	got := application.BuildSubjectSources("profile", rows)
	if got.Views != 5 || got.QRScans != 2 {
		t.Fatalf("profile views=%d qr=%d, want 5 and 2", got.Views, got.QRScans)
	}
	want := map[string]int64{"qr": 2, "nfc": 4, "link": 1, "legacy": 3}
	for _, s := range got.Sources {
		if s.Count != want[s.Source] {
			t.Errorf("profile source %s = %d, want %d", s.Source, s.Count, want[s.Source])
		}
	}

	// The card route on its own: its scans and its card's QR scans, QR-opened scans not as views.
	rows, err = repo.GetSourceCounts(ctx, "route", cardRoute, at, at)
	if err != nil {
		t.Fatal(err)
	}
	got = application.BuildSubjectSources("route", rows)
	if got.Views != 4 || got.QRScans != 2 || got.TotalEntries != 6 {
		t.Fatalf("card route views=%d qr=%d entries=%d, want 4, 2, 6", got.Views, got.QRScans, got.TotalEntries)
	}

	// Outside the date range nothing is counted.
	rows, err = repo.GetSourceCounts(ctx, "profile", profileUUID, at.AddDate(0, 0, 1), at.AddDate(0, 0, 2))
	if err != nil || len(rows) != 0 {
		t.Fatalf("rows outside range = %v, %v", rows, err)
	}
}

func TestIntegrationPlatformVisitorsAndTopEventTypes(t *testing.T) {
	d := pgtest.Open(t)
	ctx := context.Background()
	repo := infrastructure.NewPostgresAnalyticsRepository(d.Pool, d.Queries, nil)
	agg := aggregation.NewAggregator(d.Queries, nil)

	day := time.Date(2019, 7, 20, 0, 0, 0, 0, time.UTC)
	nextID := time.Now().UnixNano()%1_000_000_000 + 2_000_000_000
	ev := func(at time.Time, typ, session, ipHash string) domain.Event {
		nextID++
		return domain.Event{ID: nextID, Type: typ, OccurredAt: at, SessionID: session,
			Subject: domain.EntityReference{Type: "profile", ID: "visitors-" + pgtest.Unique()},
			Context: domain.EventContext{IPHash: ipHash}}
	}
	events := []domain.Event{
		ev(day.Add(10*time.Hour), "card.view", "s1", ""),
		ev(day.Add(11*time.Hour), "vcard.download", "s1", ""), // same visitor, other hour
		ev(day.Add(12*time.Hour), "card.view", "", "ip-a"),
		ev(day.Add(-3*24*time.Hour), "card.view", "s2", ""),  // inside the week only
		ev(day.Add(-20*24*time.Hour), "card.view", "s3", ""), // inside the 30 days only
		ev(day.Add(-20*24*time.Hour), "card.view", "", ""),   // no visitor key
	}
	if err := agg.AggregateBatch(ctx, events); err != nil {
		t.Fatal(err)
	}
	to := day.AddDate(0, 0, 1)
	got, err := repo.GetPlatformVisitors(ctx, day, day.AddDate(0, 0, -6), day.AddDate(0, 0, -29), to)
	if err != nil {
		t.Fatal(err)
	}
	if got != (domain.VisitorCounts{Today: 2, Last7Days: 3, Last30Days: 4}) {
		t.Fatalf("visitors = %+v, want today 2, 7d 3, 30d 4", got)
	}

	top, err := repo.GetPlatformTopEventTypes(ctx, day.AddDate(0, 0, -29), to, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(top) != 2 || top[0].EventType != "card.view" || top[0].Count != 5 || top[1].Count != 1 {
		t.Fatalf("top event types = %+v", top)
	}
}

// TEC-936: a changed rotation period applies from the next UTC day; a second change on the same
// day keeps the period that was in force that morning.
func TestIntegrationSaltRotationScheduleUpdate(t *testing.T) {
	d := pgtest.Open(t)
	ctx := context.Background()
	date := func(s string) pgtype.Date {
		v, _ := time.Parse(time.DateOnly, s)
		return pgtype.Date{Time: v, Valid: true}
	}
	update := func(days int32, today string) db.AnalyticsSetting {
		t.Helper()
		row, err := d.Queries.UpdateAnalyticsSettings(ctx, db.UpdateAnalyticsSettingsParams{
			IpHashSaltRotationDays: pgtype.Int4{Int32: days, Valid: days > 0},
			Today:                  date(today),
		})
		if err != nil {
			t.Fatal(err)
		}
		return row
	}
	if _, err := d.Pool.Exec(ctx, `UPDATE analytics_settings SET ip_hash_salt_rotation_days = 7,
		ip_hash_salt_rotation_previous_days = NULL, ip_hash_salt_rotation_changed_on = NULL`); err != nil {
		t.Fatal(err)
	}

	row := update(0, "2026-10-22") // other settings only
	if row.IpHashSaltRotationDays != 7 || row.IpHashSaltRotationChangedOn.Valid {
		t.Fatalf("untouched period = %+v", row)
	}
	row = update(3, "2026-10-23")
	if row.IpHashSaltRotationDays != 3 || row.IpHashSaltRotationPreviousDays.Int32 != 7 ||
		row.IpHashSaltRotationChangedOn.Time.Format(time.DateOnly) != "2026-10-23" {
		t.Fatalf("first change = %+v", row)
	}
	row = update(1, "2026-10-23")
	if row.IpHashSaltRotationDays != 1 || row.IpHashSaltRotationPreviousDays.Int32 != 7 {
		t.Fatalf("second change on the same day must keep previous 7: %+v", row)
	}
	row = update(1, "2026-10-24") // same value: nothing moves
	if row.IpHashSaltRotationChangedOn.Time.Format(time.DateOnly) != "2026-10-23" {
		t.Fatalf("unchanged period moved the change day: %+v", row)
	}
	row = update(14, "2026-10-25")
	if row.IpHashSaltRotationPreviousDays.Int32 != 1 || row.IpHashSaltRotationChangedOn.Time.Format(time.DateOnly) != "2026-10-25" {
		t.Fatalf("later change = %+v", row)
	}
	if _, err := d.Pool.Exec(ctx, `UPDATE analytics_settings SET ip_hash_salt_rotation_days = 31`); err == nil {
		t.Fatal("periods above 30 days must be rejected by the check constraint")
	}
}
