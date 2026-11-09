package repository_test

// Postgres integration tests (TEC-594). Skipped unless TEST_DATABASE_URL is set; see
// internal/testutil/pgtest. The package database is shared and migrations seed rows, so the
// assertions compare against a baseline taken before the fixtures are inserted.

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/overview/model"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/overview/repository"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/testutil/pgtest"
)

func TestMain(m *testing.M) { pgtest.Main(m) }

type snapshot struct {
	users    model.UserStats
	profiles model.ProfileStats
	cards    repository.CardCounts
	orders   map[string]int64
	subs     model.SubscriptionStats
	mrr      map[string]model.MRR
	atRisk   map[string]model.MRR
	hourly   int64
	byType   map[string]int64
}

func take(t *testing.T, r *repository.Postgres, since, firstHour time.Time) snapshot {
	t.Helper()
	ctx := context.Background()
	var s snapshot
	var err error
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	s.users, err = r.Users(ctx, since)
	must(err)
	s.profiles, err = r.Profiles(ctx)
	must(err)
	s.cards, err = r.Cards(ctx)
	must(err)
	s.orders, err = r.OrderStatusCounts(ctx, since)
	must(err)
	s.subs, err = r.Subscriptions(ctx, since)
	must(err)
	mrr, err := r.MRR(ctx)
	must(err)
	s.mrr, s.atRisk = map[string]model.MRR{}, map[string]model.MRR{}
	for _, m := range mrr.Active {
		s.mrr[m.Currency] = m
	}
	for _, m := range mrr.AtRisk {
		s.atRisk[m.Currency] = m
	}
	hours, err := r.EventsByHour(ctx, firstHour)
	must(err)
	for _, h := range hours {
		s.hourly += h.Count
	}
	types, err := r.EventsByType(ctx, firstHour, 50)
	must(err)
	s.byType = map[string]int64{}
	for _, e := range types {
		s.byType[e.EventType] = e.Count
	}
	return s
}

func TestIntegrationOverviewKPIs(t *testing.T) {
	d := pgtest.Open(t)
	ctx := context.Background()
	repo := repository.NewPostgres(d.Queries)
	exec := func(stmt string, args ...any) {
		t.Helper()
		if _, err := d.Pool.Exec(ctx, stmt, args...); err != nil {
			t.Fatalf("%s: %v", stmt, err)
		}
	}

	now := time.Now().UTC()
	since := now.Add(-30 * 24 * time.Hour)
	firstHour := now.Truncate(time.Hour).Add(-23 * time.Hour)
	before := take(t, repo, since, firstHour)

	// Users: one signed in today, one 40 days ago (both created now).
	active, stale := d.CreateUser(t), d.CreateUser(t)
	exec(`UPDATE users SET last_login_at = NOW() WHERE id = $1`, active.ID)
	exec(`UPDATE users SET last_login_at = NOW() - INTERVAL '40 days' WHERE id = $1`, stale.ID)

	// Profiles: published (live username), unpublished, deleted.
	var published int64
	var publishedUUID uuid.UUID
	if err := d.Pool.QueryRow(ctx, `INSERT INTO profiles (user_id, account_id, primary_locale) VALUES ($1, $2, 'en') RETURNING id, uuid`,
		active.ID, active.AccountID).Scan(&published, &publishedUUID); err != nil {
		t.Fatal(err)
	}
	exec(`INSERT INTO profiles (user_id, account_id) VALUES ($1, $2)`, active.ID, active.AccountID)
	exec(`INSERT INTO profiles (user_id, account_id, deleted_at) VALUES ($1, $2, NOW())`, active.ID, active.AccountID)
	exec(`INSERT INTO profile_translations (profile_id, locale, first_name, last_name, company_name, is_default)
		VALUES ($1, 'tr', 'Varsayılan', 'Ad', '', true), ($1, 'en', 'Ada', 'Lovelace', 'Analytical', false)`, published)
	username := "ov-" + pgtest.Unique()
	exec(`INSERT INTO routes (path, route_type, status, target_type, profile_id, account_id, owner_user_id)
		VALUES ($1, 'canonical_profile', 'active', 'profile', $2, $3, $4)`, username, published, active.AccountID, active.ID)

	// Cards: one active (fixture default), one waiting for activation.
	d.CreateCard(t, active)
	unactivated := d.CreateCard(t, active)
	exec(`UPDATE nfc_cards SET status = 'unactivated', is_active = false WHERE id = $1`, unactivated.ID)

	// Orders: two in the window, one older than it.
	order := func(status string, age string) {
		exec(`INSERT INTO orders (order_number, user_id, account_id, status, created_at)
			VALUES ($1, $2, $3, $4, NOW() - $5::interval)`, "OV-"+pgtest.Unique(), active.ID, active.AccountID, status, age)
	}
	order("paid", "1 day")
	order("in_production", "2 days")
	order("paid", "40 days")

	// Subscriptions on the seeded TRY plans, one account each.
	sub := func(planSlug, status, source string, canceledAt *time.Time) {
		u := d.CreateUser(t)
		exec(`INSERT INTO subscriptions (account_id, plan_id, status, source, canceled_at)
			SELECT $1, id, $3::subscription_status, $4::subscription_source, $5 FROM plans WHERE slug = $2`,
			u.AccountID, planSlug, status, source, canceledAt)
	}
	sub("pro-monthly", "active", "payment", nil)
	sub("pro-yearly", "active", "manual", nil)
	sub("pro-monthly", "trialing", "payment", nil)
	sub("free", "active", "manual", nil) // no recurring revenue
	sub("pro-monthly", "past_due", "payment", nil)
	sub("pro-yearly", "past_due", "manual", nil)
	recent := now.Add(-time.Hour)
	sub("pro-monthly", "canceled", "payment", &recent)

	// Analytics aggregates: views of the published profile today and 40 days ago; hourly volume
	// in the current hour and 30 hours ago.
	pid := publishedUUID.String()
	exec(`INSERT INTO event_aggregates_daily (bucket_date, subject_type, subject_id, event_type, total_count, unique_sessions)
		VALUES (CURRENT_DATE, 'profile', $1, 'card.view', 9000000, 7000000),
		       (CURRENT_DATE - 40, 'profile', $1, 'card.view', 5, 5),
		       (CURRENT_DATE, 'profile', $1, 'contact.phone.click', 3, 3)`, pid)
	exec(`INSERT INTO event_aggregates_hourly (bucket_hour, subject_type, subject_id, event_type, total_count, unique_sessions)
		VALUES (date_trunc('hour', NOW()), 'profile', $1, 'ov.test.event', 11, 1),
		       (date_trunc('hour', NOW()) - INTERVAL '30 hours', 'profile', $1, 'ov.test.event', 100, 1)`, pid)

	after := take(t, repo, since, firstHour)

	if got := after.users.Total - before.users.Total; got < 2 {
		t.Fatalf("users total delta = %d", got)
	}
	// Every CreateUser adds a user; only `active` signed in during the window.
	if got := after.users.ActiveLast30d - before.users.ActiveLast30d; got != 1 {
		t.Fatalf("active users delta = %d, want 1", got)
	}
	if got := after.profiles.Total - before.profiles.Total; got != 2 {
		t.Fatalf("profiles delta = %d, want 2 (deleted excluded)", got)
	}
	if got := after.profiles.Published - before.profiles.Published; got != 1 {
		t.Fatalf("published delta = %d, want 1", got)
	}
	if after.cards.Total-before.cards.Total != 2 || after.cards.Activated-before.cards.Activated != 1 {
		t.Fatalf("cards before %+v after %+v", before.cards, after.cards)
	}
	if after.orders["paid"]-before.orders["paid"] != 1 || after.orders["in_production"]-before.orders["in_production"] != 1 {
		t.Fatalf("orders before %v after %v", before.orders, after.orders)
	}
	if after.subs.Active-before.subs.Active != 3 || after.subs.Trialing-before.subs.Trialing != 1 ||
		after.subs.Canceled-before.subs.Canceled != 1 || after.subs.CanceledLast30d-before.subs.CanceledLast30d != 1 {
		t.Fatalf("subscriptions before %+v after %+v", before.subs, after.subs)
	}
	b, a := before.mrr["TRY"], after.mrr["TRY"]
	// 999.00 monthly + 9,990.00 / 12 yearly = 99900 + 83250 kuruş; the yearly one is manual.
	// The two past_due subscriptions are not MRR.
	if a.AmountMinor-b.AmountMinor != 183150 || a.ManualAmountMinor-b.ManualAmountMinor != 83250 || a.Subscriptions-b.Subscriptions != 2 {
		t.Fatalf("mrr before %+v after %+v", b, a)
	}
	// They are at-risk MRR instead, with the same monthly spread.
	br, ar := before.atRisk["TRY"], after.atRisk["TRY"]
	if ar.AmountMinor-br.AmountMinor != 183150 || ar.ManualAmountMinor-br.ManualAmountMinor != 83250 || ar.Subscriptions-br.Subscriptions != 2 {
		t.Fatalf("at-risk mrr before %+v after %+v", br, ar)
	}
	if after.subs.PastDue-before.subs.PastDue != 2 {
		t.Fatalf("past due before %d after %d", before.subs.PastDue, after.subs.PastDue)
	}
	if after.hourly-before.hourly != 11 || after.byType["ov.test.event"] != 11 {
		t.Fatalf("event volume before %d after %d, by type %v", before.hourly, after.hourly, after.byType)
	}

	top, err := repo.TopProfiles(ctx, time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC).AddDate(0, 0, -29), 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(top) == 0 || top[0].UUID != publishedUUID {
		t.Fatalf("top profiles = %+v", top)
	}
	// Only card.view inside the window; the en translation matches the primary locale.
	want := model.TopProfile{UUID: publishedUUID, Name: "Ada Lovelace", CompanyName: "Analytical", Username: username,
		Views: 9000000, UniqueSessions: 7000000}
	if top[0] != want {
		t.Fatalf("top[0] = %+v, want %+v", top[0], want)
	}
	for _, p := range top {
		if p.Views <= 0 {
			t.Fatalf("profile without views listed: %+v", p)
		}
	}
}

// TestIntegrationConversionFunnel uses a period in the far past so other tests' orders stay out.
func TestIntegrationConversionFunnel(t *testing.T) {
	d := pgtest.Open(t)
	ctx := context.Background()
	repo := repository.NewPostgres(d.Queries)
	u := d.CreateUser(t)

	start := time.Date(2001, 3, 1, 0, 0, 0, 0, time.UTC)
	// order inserts an order created at start+offset and its status history: each step is
	// {to_status, hours after creation}. backfilled marks a single migration-style row.
	order := func(offset time.Duration, current string, backfilled bool, steps ...any) {
		t.Helper()
		created := start.Add(offset)
		var id int64
		if err := d.Pool.QueryRow(ctx, `INSERT INTO orders (order_number, user_id, account_id, status, created_at)
			VALUES ($1, $2, $3, $4, $5) RETURNING id`, "FN-"+pgtest.Unique(), u.ID, u.AccountID, current, created).Scan(&id); err != nil {
			t.Fatal(err)
		}
		from := any(nil)
		if !backfilled {
			if _, err := d.Pool.Exec(ctx, `INSERT INTO order_status_history (account_id, order_id, to_status, changed_at)
				VALUES ($1, $2, 'pending', $3)`, u.AccountID, id, created); err != nil {
				t.Fatal(err)
			}
			from = "pending"
		}
		for i := 0; i < len(steps); i += 2 {
			to, hours := steps[i].(string), steps[i+1].(int)
			if _, err := d.Pool.Exec(ctx, `INSERT INTO order_status_history (account_id, order_id, from_status, to_status, changed_at)
				VALUES ($1, $2, $3, $4, $5)`, u.AccountID, id, from, to, created.Add(time.Duration(hours)*time.Hour)); err != nil {
				t.Fatal(err)
			}
			from = to
		}
	}
	order(time.Hour, "pending", false)
	order(2*time.Hour, "cancelled", false, "cancelled", 1)
	order(3*time.Hour, "in_production", false, "paid", 2, "design_pending", 3, "in_production", 10)
	// Skips "paid" (payment tracked on payment_status) but still reached the paid stage.
	order(4*time.Hour, "delivered", false, "in_production", 4, "shipped", 30, "delivered", 50)
	// Backfilled delivered order: reached every stage, timed for none.
	order(5*time.Hour, "delivered", true, "delivered", 100)
	// Outside the period.
	order(-time.Hour, "paid", false, "paid", 1)
	order(24*time.Hour, "paid", false, "paid", 1)

	got, err := repo.ConversionFunnel(ctx, start, start.Add(24*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if got.Created != 5 || got.Cancelled != 1 {
		t.Fatalf("created %d cancelled %d, want 5 and 1", got.Created, got.Cancelled)
	}
	want := map[string]repository.StageCounts{
		model.StagePaid:         {Reached: 3, Timed: 2, Seconds: (2 + 4) * 3600},
		model.StageInProduction: {Reached: 3, Timed: 2, Seconds: (10 + 4) * 3600},
		model.StageShipped:      {Reached: 2, Timed: 1, Seconds: 30 * 3600},
		model.StageDelivered:    {Reached: 2, Timed: 1, Seconds: 50 * 3600},
	}
	for stage, w := range want {
		if got.Stages[stage] != w {
			t.Fatalf("%s = %+v, want %+v", stage, got.Stages[stage], w)
		}
	}
}
