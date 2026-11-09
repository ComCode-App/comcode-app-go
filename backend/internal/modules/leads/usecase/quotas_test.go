package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/leads/model"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/leads/repository"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/authctx"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/entitlement"
)

// memQuotas mirrors entitlement's monthly counters in memory: limits per key (missing =
// unlimited), counters per key and month. The SQL behind them is covered in the entitlement
// package's integration tests.
type memQuotas struct {
	now      func() time.Time
	limits   map[string]int32
	used     map[string]int64
	unlocked map[string]int64
}

func newMemQuotas(now func() time.Time, limits map[string]int32) *memQuotas {
	return &memQuotas{now: now, limits: limits, used: map[string]int64{}, unlocked: map[string]int64{}}
}

func (q *memQuotas) slot(key string) string {
	return key + "@" + entitlement.MonthStart(q.now()).Format("2006-01")
}

func (q *memQuotas) MonthlyUsage(_ context.Context, _ int64, key string) (entitlement.MonthlyUsage, error) {
	lim, ok := q.limits[key]
	u := entitlement.MonthlyUsage{Key: key, Limit: lim, Unlimited: !ok || lim < 0}
	u.Used, u.Unlocked = q.used[q.slot(key)], q.unlocked[q.slot(key)]
	return u, nil
}

func (q *memQuotas) RecordMonthly(ctx context.Context, accountID int64, key string) (entitlement.MonthlyUsage, bool, error) {
	u, _ := q.MonthlyUsage(ctx, accountID, key)
	if u.Off() {
		return u, false, nil
	}
	q.used[q.slot(key)]++
	granted := u.Unlimited || u.Unlocked < int64(u.Limit)
	if granted {
		q.unlocked[q.slot(key)]++
	}
	u, _ = q.MonthlyUsage(ctx, accountID, key)
	return u, granted, nil
}

func (q *memQuotas) ReserveMonthly(ctx context.Context, accountID int64, admin bool, key string) (entitlement.MonthlyUsage, error) {
	u, _ := q.MonthlyUsage(ctx, accountID, key)
	if admin {
		return u, nil
	}
	if u.Off() || u.Reached() {
		return u, &entitlement.QuotaError{Feature: key, Limit: u.Limit, Used: u.Used}
	}
	q.used[q.slot(key)]++
	return u, nil
}

func (q *memQuotas) AddMonthlyUnlocked(_ context.Context, _ int64, key string, n int64) error {
	q.unlocked[q.slot(key)] += n
	return nil
}

type clock struct{ t time.Time }

func (c *clock) now() time.Time { return c.t }

func quotaService(limits map[string]int32) (*Service, *memRepo, *memNotifier, *clock) {
	repo := newMemRepo()
	repo.routes["ahmet"] = &repository.CaptureRoute{
		ID: 1, RouteType: "canonical_profile", Status: "active", AccountID: 42, ProfileID: int64p(5),
		PresentationConfig: []byte(`{"lead_capture":{"enabled":true},"contact_exchange":{"enabled":true}}`),
	}
	repo.owners = []int64{100}
	notifier := &memNotifier{}
	c := &clock{t: time.Date(2026, 10, 14, 9, 0, 0, 0, time.UTC)}
	svc := New(repo, notifier, nil).SetQuotas(newMemQuotas(c.now, limits))
	svc.now = c.now
	return svc, repo, notifier, c
}

var quotaOwner = authctx.Principal{UserInternal: 100, Account: authctx.Account{ID: 42, Role: authctx.AccountRoleOwner}}

func submitN(t *testing.T, svc *Service, c *clock, n int) {
	t.Helper()
	for range n {
		c.t = c.t.Add(time.Minute)
		if _, err := svc.Submit(context.Background(), validInput(), model.Visitor{}); err != nil {
			t.Fatal(err)
		}
	}
}

func lockedCount(t *testing.T, svc *Service) int {
	t.Helper()
	items, _, err := svc.List(context.Background(), quotaOwner, model.ListFilter{})
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, l := range items {
		if l.Locked {
			n++
			if l.Name != "A*** Y***" || l.Email != "a***@***" || l.Message != "" {
				t.Fatalf("locked lead not masked in the list: %+v", l)
			}
		} else if l.Name != "Ayşe Yılmaz" || l.Email != "ayse@example.com" {
			t.Fatalf("unlocked lead masked: %+v", l)
		}
	}
	return n
}

// Over the monthly quota a lead is stored locked: the owner sees it masked in the list, the
// detail and the notification.
func TestOverQuotaLeadIsStoredLockedAndMasked(t *testing.T) {
	t.Parallel()
	svc, repo, notifier, c := quotaService(map[string]int32{entitlement.FeatureLeadsMonthlyMax: 2})
	submitN(t, svc, c, 3)
	if len(repo.created) != 3 || repo.created[0].Locked || repo.created[1].Locked || !repo.created[2].Locked {
		t.Fatalf("want the third lead locked: %+v", repo.created)
	}
	if got := lockedCount(t, svc); got != 1 {
		t.Fatalf("want 1 locked lead, got %d", got)
	}
	if notifier.sent[2].TemplateVars["name"] != "A*** Y***" || notifier.sent[2].Payload["locked"] != true {
		t.Fatalf("locked lead's notification shows the name: %+v", notifier.sent[2])
	}
	for id, in := range repo.stored {
		l, err := svc.Get(context.Background(), quotaOwner, id)
		if err != nil {
			t.Fatal(err)
		}
		if in.Locked != l.Locked || (l.Locked && (l.Name != "A*** Y***" || l.Email != "a***@***")) {
			t.Fatalf("detail: %+v", l)
		}
	}
}

// A plan upgrade unlocks the month's locked leads up to the new quota, oldest first; a new month
// unlocks the rest; an unlocked lead never locks again.
func TestLockedLeadsUnlockOnUpgradeAndNewMonth(t *testing.T) {
	t.Parallel()
	svc, _, _, c := quotaService(map[string]int32{entitlement.FeatureLeadsMonthlyMax: 1})
	quotas := svc.quotas.(*memQuotas)
	submitN(t, svc, c, 4)
	if got := lockedCount(t, svc); got != 3 {
		t.Fatalf("want 3 locked, got %d", got)
	}

	quotas.limits[entitlement.FeatureLeadsMonthlyMax] = 3 // upgrade
	if got := lockedCount(t, svc); got != 1 {
		t.Fatalf("upgrade to 3: want 1 locked, got %d", got)
	}
	quotas.limits[entitlement.FeatureLeadsMonthlyMax] = 1 // downgrade
	if got := lockedCount(t, svc); got != 1 {
		t.Fatalf("downgrade must not re-lock: want 1 locked, got %d", got)
	}

	c.t = time.Date(2026, 11, 1, 0, 30, 0, 0, entitlement.MonthStart(c.t).Location()) // new month
	if got := lockedCount(t, svc); got != 0 {
		t.Fatalf("new month: want 0 locked, got %d", got)
	}
	// The new month's counter starts over: one more lead is unlocked, the next locked.
	submitN(t, svc, c, 2)
	if got := lockedCount(t, svc); got != 1 {
		t.Fatalf("new month quota: want 1 locked, got %d", got)
	}
}

// Deleting a visible lead does not free its slot for a locked one.
func TestDeletingDoesNotUnlock(t *testing.T) {
	t.Parallel()
	svc, repo, _, c := quotaService(map[string]int32{entitlement.FeatureLeadsMonthlyMax: 1})
	submitN(t, svc, c, 2)
	for id, in := range repo.stored {
		if !in.Locked {
			if err := svc.Delete(context.Background(), quotaOwner, id); err != nil {
				t.Fatal(err)
			}
			delete(repo.stored, id)
		}
	}
	if got := lockedCount(t, svc); got != 1 {
		t.Fatalf("want the lead still locked, got %d locked", got)
	}
}

// Exchanges have their own quota.
func TestExchangeQuotaIsSeparate(t *testing.T) {
	t.Parallel()
	svc, repo, _, c := quotaService(map[string]int32{
		entitlement.FeatureLeadsMonthlyMax: 1, entitlement.FeatureExchangeMonthlyMax: 1,
	})
	submitN(t, svc, c, 1)
	in := model.ExchangeInput{Path: "ahmet", Name: "Ayşe Yılmaz", Phone: "+90 532 111 22 33",
		Consent: true, ConsentVersion: model.ExchangeConsentVersion}
	for range 2 {
		if _, err := svc.Exchange(context.Background(), in, model.Visitor{}); err != nil {
			t.Fatal(err)
		}
	}
	if repo.created[0].Locked || repo.created[1].Locked || !repo.created[2].Locked {
		t.Fatalf("want only the second exchange locked: %+v", repo.created)
	}
	items, _, _ := svc.List(context.Background(), quotaOwner, model.ListFilter{})
	for _, l := range items {
		if l.Locked && l.Phone != "*** 33" {
			t.Fatalf("locked exchange phone: %q", l.Phone)
		}
	}
}

// A quota of 0 turns the form or the exchange off; without quotas both are off (fail-closed).
func TestZeroQuotaTurnsFeatureOff(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	svc, repo, _, _ := quotaService(map[string]int32{
		entitlement.FeatureLeadsMonthlyMax: 0, entitlement.FeatureExchangeMonthlyMax: 5,
	})
	cfg, err := svc.FormConfig(ctx, "ahmet")
	if err != nil || cfg.Enabled || !cfg.ExchangeEnabled {
		t.Fatalf("want form off, exchange on: %+v %v", cfg, err)
	}
	if _, err := svc.Submit(ctx, validInput(), model.Visitor{}); !errors.Is(err, ErrFormUnavailable) {
		t.Fatalf("want ErrFormUnavailable, got %v", err)
	}
	if len(repo.created) != 0 {
		t.Fatal("lead stored with the form off")
	}

	bare := New(repo, nil, nil)
	if cfg, _ := bare.FormConfig(ctx, "ahmet"); cfg.Enabled || cfg.ExchangeEnabled {
		t.Fatalf("no quotas must turn both off: %+v", cfg)
	}
	if _, err := bare.Submit(ctx, validInput(), model.Visitor{}); !errors.Is(err, ErrFormUnavailable) {
		t.Fatalf("no quotas: want ErrFormUnavailable, got %v", err)
	}
	if err := bare.PrepareExport(ctx, quotaOwner); !errors.Is(err, entitlement.ErrPlanLimitExceeded) {
		t.Fatalf("no quotas: export must be refused, got %v", err)
	}
}

func TestExportQuota(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	svc, _, _, _ := quotaService(map[string]int32{entitlement.FeatureLeadsExportMonthlyMax: 1})
	if err := svc.PrepareExport(ctx, quotaOwner); err != nil {
		t.Fatal(err)
	}
	var qe *entitlement.QuotaError
	if err := svc.PrepareExport(ctx, quotaOwner); !errors.As(err, &qe) || qe.Feature != entitlement.FeatureLeadsExportMonthlyMax {
		t.Fatalf("second export: want QuotaError, got %v", err)
	}
	admin := quotaOwner
	admin.IsSuperAdmin = true
	if err := svc.PrepareExport(ctx, admin); err != nil {
		t.Fatalf("super admin bypasses: %v", err)
	}
}

func TestMask(t *testing.T) {
	t.Parallel()
	cases := []struct{ got, want string }{
		{model.MaskName("Ayşe  Yılmaz"), "A*** Y***"},
		{model.MaskName("Çağrı"), "Ç***"},
		{model.MaskName(""), ""},
		{model.MaskEmail("ayse@example.com"), "a***@***"},
		{model.MaskEmail(""), ""},
		{model.MaskPhone("+90 (532) 111-22-33"), "*** 33"},
		{model.MaskPhone(""), ""},
	}
	for _, tc := range cases {
		if tc.got != tc.want {
			t.Errorf("want %q, got %q", tc.want, tc.got)
		}
	}
}
