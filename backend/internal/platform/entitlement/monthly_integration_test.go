package entitlement

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/testutil/pgtest"
)

type recordedAlerts struct{ got []QuotaAlert }

func (r *recordedAlerts) QuotaAlert(_ context.Context, a QuotaAlert) { r.got = append(r.got, a) }

func setLimit(t *testing.T, d *pgtest.DB, accountID int64, key string, value int32) {
	t.Helper()
	ctx := context.Background()
	// A plan of the test's own, so changing a limit does not touch the shared seeded plans.
	slug := "monthly-" + pgtest.Unique()
	if _, err := d.Pool.Exec(ctx, `
		WITH p AS (
			INSERT INTO plans (slug, name, billing_interval, price_minor, currency, is_active)
			VALUES ($1, $1, 'month', 100, 'TRY', true) RETURNING id
		)
		INSERT INTO plan_limits (plan_id, feature_key, value_int, value_bool)
		SELECT p.id, l.feature_key, l.value_int, l.value_bool
		FROM p, plan_limits l JOIN plans f ON f.id = l.plan_id AND f.slug = 'free'`, slug); err != nil {
		t.Fatal(err)
	}
	if _, err := d.Pool.Exec(ctx, `
		INSERT INTO subscriptions (account_id, plan_id, status, source, starts_at)
		SELECT $1, id, 'active', 'manual', now() FROM plans WHERE slug = $2`, accountID, slug); err != nil {
		t.Fatal(err)
	}
	if _, err := d.Pool.Exec(ctx, `
		UPDATE plan_limits SET value_int = $3
		WHERE feature_key = $2 AND plan_id = (SELECT id FROM plans WHERE slug = $1)`, slug, key, value); err != nil {
		t.Fatal(err)
	}
}

// The migration gives every plan the three monthly features: free 20/20/2, paid 1000/1000/100.
func TestMonthlyFeatureDefaults(t *testing.T) {
	d := pgtest.Open(t)
	ctx := context.Background()
	svc := New(d.Queries)
	user := d.CreateUser(t)
	want := map[string]int32{FeatureLeadsMonthlyMax: 20, FeatureExchangeMonthlyMax: 20, FeatureLeadsExportMonthlyMax: 2}
	for key, limit := range want {
		u, err := svc.MonthlyUsage(ctx, user.AccountID, key)
		if err != nil {
			t.Fatal(err)
		}
		if u.Limit != limit || u.Unlimited || u.Used != 0 {
			t.Errorf("free %s: want %d, got %+v", key, limit, u)
		}
	}
	var missing int
	if err := d.Pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM plans p
		CROSS JOIN (VALUES ('leads.monthly_max'), ('exchange.monthly_max'), ('leads.export.monthly_max')) f(key)
		WHERE NOT EXISTS (SELECT 1 FROM plan_limits l WHERE l.plan_id = p.id AND l.feature_key = f.key)`).Scan(&missing); err != nil {
		t.Fatal(err)
	}
	if missing != 0 {
		t.Fatalf("%d plan/feature pairs without a value", missing)
	}
	var paid int32
	if err := d.Pool.QueryRow(ctx, `
		SELECT l.value_int FROM plan_limits l JOIN plans p ON p.id = l.plan_id
		WHERE p.slug = 'pro-monthly' AND l.feature_key = 'leads.monthly_max'`).Scan(&paid); err != nil {
		t.Fatal(err)
	}
	if paid != 1000 {
		t.Fatalf("pro-monthly leads.monthly_max: want 1000, got %d", paid)
	}
}

// Counters are per calendar month in Europe/Istanbul: a new month starts from zero. Events over
// the quota are counted but not granted an unlocked slot; alerts fire once at 80% and 100%.
func TestRecordMonthlyCountsPerMonthAndAlerts(t *testing.T) {
	d := pgtest.Open(t)
	ctx := context.Background()
	user := d.CreateUser(t)
	setLimit(t, d, user.AccountID, FeatureLeadsMonthlyMax, 5)
	alerts := &recordedAlerts{}
	svc := New(d.Queries)
	svc.SetUsageAlerter(alerts)
	// 31 October 23:30 in Istanbul is still October there, though it is the 31st 20:30 UTC.
	oct := time.Date(2026, 10, 31, 20, 30, 0, 0, time.UTC)
	svc.now = func() time.Time { return oct }

	var granted []bool
	for range 6 {
		u, ok, err := svc.RecordMonthly(ctx, user.AccountID, FeatureLeadsMonthlyMax)
		if err != nil {
			t.Fatal(err)
		}
		granted = append(granted, ok)
		if u.ResetsAt.Format(time.RFC3339) != "2026-11-01T00:00:00+03:00" {
			t.Fatalf("resets at %s", u.ResetsAt.Format(time.RFC3339))
		}
	}
	if want := []bool{true, true, true, true, true, false}; !equalBools(granted, want) {
		t.Fatalf("granted: want %v, got %v", want, granted)
	}
	if len(alerts.got) != 2 || alerts.got[0].Percent != 80 || alerts.got[0].Used != 4 || alerts.got[1].Percent != 100 {
		t.Fatalf("alerts: %+v", alerts.got)
	}

	// 1 November 00:30 Istanbul (31 October 21:30 UTC): a new month.
	svc.now = func() time.Time { return time.Date(2026, 10, 31, 21, 30, 0, 0, time.UTC) }
	u, err := svc.MonthlyUsage(ctx, user.AccountID, FeatureLeadsMonthlyMax)
	if err != nil {
		t.Fatal(err)
	}
	if u.Used != 0 || u.Unlocked != 0 || u.Period.Month() != time.November {
		t.Fatalf("counter not reset: %+v", u)
	}
	if _, ok, err := svc.RecordMonthly(ctx, user.AccountID, FeatureLeadsMonthlyMax); err != nil || !ok {
		t.Fatalf("new month: want granted, got %v %v", ok, err)
	}
}

func TestReserveMonthlyStopsAtLimit(t *testing.T) {
	d := pgtest.Open(t)
	ctx := context.Background()
	user := d.CreateUser(t)
	setLimit(t, d, user.AccountID, FeatureLeadsExportMonthlyMax, 2)
	svc := New(d.Queries)
	for i := range 2 {
		if _, err := svc.ReserveMonthly(ctx, user.AccountID, false, FeatureLeadsExportMonthlyMax); err != nil {
			t.Fatalf("export %d: %v", i+1, err)
		}
	}
	_, err := svc.ReserveMonthly(ctx, user.AccountID, false, FeatureLeadsExportMonthlyMax)
	var qe *QuotaError
	if !errors.As(err, &qe) || qe.Limit != 2 || qe.Used != 2 {
		t.Fatalf("third export: want QuotaError 2/2, got %v", err)
	}
	if _, err := svc.ReserveMonthly(ctx, user.AccountID, true, FeatureLeadsExportMonthlyMax); err != nil {
		t.Fatalf("super admin: %v", err)
	}
	u, _ := svc.MonthlyUsage(ctx, user.AccountID, FeatureLeadsExportMonthlyMax)
	if u.Used != 2 {
		t.Fatalf("used must stay 2, got %d", u.Used)
	}

	ov, err := svc.OverviewForAccount(ctx, user.AccountID)
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range ov.Features {
		if f.Key == FeatureLeadsExportMonthlyMax {
			if !f.Monthly || f.ResetsAt == nil || f.Used == nil || *f.Used != 2 || !f.Reached {
				t.Fatalf("overview row: %+v", f)
			}
			return
		}
	}
	t.Fatal("overview misses the export quota")
}

// A quota of 0 is off: nothing is counted and reserving fails.
func TestZeroMonthlyQuotaIsOff(t *testing.T) {
	d := pgtest.Open(t)
	ctx := context.Background()
	user := d.CreateUser(t)
	setLimit(t, d, user.AccountID, FeatureExchangeMonthlyMax, 0)
	svc := New(d.Queries)
	u, ok, err := svc.RecordMonthly(ctx, user.AccountID, FeatureExchangeMonthlyMax)
	if err != nil || ok || !u.Off() || u.Used != 0 {
		t.Fatalf("want off and uncounted: %+v %v %v", u, ok, err)
	}
	if _, err := svc.ReserveMonthly(ctx, user.AccountID, false, FeatureExchangeMonthlyMax); !errors.Is(err, ErrPlanLimitExceeded) {
		t.Fatalf("want ErrPlanLimitExceeded, got %v", err)
	}
}

func equalBools(a, b []bool) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
