package usecase_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/database/db"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/billing/repository"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/billing/usecase"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/authctx"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/entitlement"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/payment"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/rbac"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/testutil/pgtest"
	"github.com/google/uuid"
)

func TestMain(m *testing.M) { pgtest.Main(m) }

type gateway struct {
	calls   atomic.Int32
	success bool
	err     error
	amount  atomic.Int64
}

func (g *gateway) ChargeRecurring(_ context.Context, _ string, in payment.SavedCardChargeInput) (payment.CallbackResult, error) {
	g.calls.Add(1)
	g.amount.Store(in.AmountMinor)
	return payment.CallbackResult{Success: g.success, ProviderRef: "fixture-captured"}, g.err
}

type fixture struct {
	d       *pgtest.DB
	s       *usecase.Service
	g       *gateway
	id      int64
	account int64
	plan    int64
	now     time.Time
	payer   usecase.Payer
	ctx     context.Context
}

func setup(t *testing.T) fixture {
	t.Helper()
	d := pgtest.Open(t)
	user := d.CreateUser(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)
	var plan, card, id int64
	err := d.Pool.QueryRow(ctx, `INSERT INTO plans(slug,name,billing_interval,price_minor,currency,is_active)
 VALUES($1,'Renewal fixture','month',10000,'TRY',true) RETURNING id`, "renewal-"+pgtest.Unique()).Scan(&plan)
	if err != nil {
		t.Fatal(err)
	}
	err = d.Pool.QueryRow(ctx, `INSERT INTO payment_saved_cards(account_id,user_id,provider,provider_card_ref,provider_customer_ref,last4,brand,is_default)
 VALUES($1,$2,'iyzico',$3,'fixture-customer','0000','test',true) RETURNING id`, user.AccountID, user.ID, pgtest.Unique()).Scan(&card)
	if err != nil {
		t.Fatal(err)
	}
	err = d.Pool.QueryRow(ctx, `INSERT INTO subscriptions(account_id,plan_id,status,source,provider,saved_card_id,starts_at,period_started_at,next_billing_at)
 VALUES($1,$2,'active','payment','iyzico',$3,$4,$4,$5) RETURNING id`, user.AccountID, plan, card, now.AddDate(0, -1, 0), now.Add(-time.Hour)).Scan(&id)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, err := d.Pool.Exec(ctx, "UPDATE subscriptions SET status='canceled' WHERE id=$1", id)
		if err != nil {
			t.Error(err)
		}
	})
	g := &gateway{success: true}
	s := usecase.New(d.Queries, nil, "https://comcode.test", "", slog.New(slog.NewTextHandler(io.Discard, nil)), repository.NewPostgres(d.Pool))
	s.SetRecurringCharger(g)
	payer := usecase.Payer{Account: authctx.Account{ID: user.AccountID, Role: authctx.AccountRoleOwner}, UserID: user.ID, Email: "fixture@example.test", Name: "Fixture"}
	ctx = authctx.WithPrincipal(ctx, authctx.Principal{UserInternal: user.ID, Account: payer.Account, Permissions: []string{rbac.PermAppBillingWrite}})
	return fixture{d: d, s: s, g: g, id: id, account: user.AccountID, plan: plan, now: now, payer: payer, ctx: ctx}
}
func (f fixture) run(t *testing.T, at time.Time) {
	t.Helper()
	if err := f.s.ProcessRenewals(f.ctx, at, usecase.DefaultRenewalPolicy()); err != nil {
		t.Fatal(err)
	}
}
func (f fixture) sub(t *testing.T) db.Subscription {
	t.Helper()
	sub, err := f.d.Queries.LockRenewalSubscription(f.ctx, f.id)
	if err != nil {
		t.Fatal(err)
	}
	return sub
}
func (f fixture) exec(t *testing.T, sql string, args ...any) {
	t.Helper()
	if _, err := f.d.Pool.Exec(f.ctx, sql, args...); err != nil {
		t.Fatal(err)
	}
}
func (f fixture) eventCount(t *testing.T, name string) int {
	t.Helper()
	var n int
	err := f.d.Pool.QueryRow(f.ctx, `SELECT count(*) FROM outbox_events WHERE event_name=$1 AND payload->'data'->>'subscription_uuid'=$2`, name, f.sub(t).Uuid.String()).Scan(&n)
	if err != nil {
		t.Fatal(err)
	}
	return n
}
func TestDueSelectionAndSuccess(t *testing.T) {
	f := setup(t)
	manualUser := f.d.CreateUser(t)
	f.exec(t, `INSERT INTO subscriptions(account_id,plan_id,status,source,starts_at,next_billing_at)
 VALUES($1,$2,'active','manual',now(),now()-interval '1 day')`, manualUser.AccountID, f.plan)
	f.exec(t, `UPDATE subscriptions SET next_billing_at=$2 WHERE id=$1`, f.id, f.now.Add(time.Hour))
	f.run(t, f.now)
	if f.g.calls.Load() != 0 {
		t.Fatal("charged future/manual subscription")
	}
	f.run(t, f.now.Add(2*time.Hour))
	sub := f.sub(t)
	if sub.Status != db.SubscriptionStatusActive || !sub.NextBillingAt.Time.After(f.now.AddDate(0, 0, 28)) || sub.RenewalAttempts != 0 {
		t.Fatalf("renewal: %+v", sub)
	}
	f.run(t, f.now.Add(2*time.Hour))
	if f.g.calls.Load() != 1 {
		t.Fatal("duplicate renewal")
	}
	for _, name := range []string{"payment.succeeded", "subscription.renewed"} {
		if f.eventCount(t, name) != 1 {
			t.Fatal("missing", name)
		}
	}
	var count int
	if err := f.d.Pool.QueryRow(f.ctx, `SELECT count(*) FROM subscription_payment_attempts WHERE subscription_id=$1 AND status='succeeded' AND provider_ref='fixture-captured' AND amount_minor=10000`, f.id).Scan(&count); err != nil || count != 1 {
		t.Fatalf("payment record %d %v", count, err)
	}
}
func TestDunningRetryGraceAndExpiry(t *testing.T) {
	f := setup(t)
	f.g.success = false
	f.run(t, f.now)
	sub := f.sub(t)
	if sub.Status != db.SubscriptionStatusPastDue || !sub.RetryAt.Time.Equal(f.now.AddDate(0, 0, 1)) || !sub.GraceUntil.Time.Equal(f.now.AddDate(0, 0, 14)) {
		t.Fatalf("first decline: %+v", sub)
	}
	plan, err := entitlement.New(f.d.Queries).ResolvePlan(f.ctx, f.account)
	if err != nil || plan.PlanID != f.plan {
		t.Fatalf("grace lost entitlements: %+v %v", plan, err)
	}
	f.run(t, f.now.Add(12*time.Hour))
	if f.g.calls.Load() != 1 {
		t.Fatal("early retry")
	}
	for i, day := range []int{1, 3, 7} {
		f.run(t, f.now.AddDate(0, 0, day))
		if f.g.calls.Load() != int32(i+2) {
			t.Fatal("retry missing", day)
		}
		sub = f.sub(t)
		next := 14
		if i < 2 {
			next = []int{3, 7}[i]
		}
		if !sub.RetryAt.Time.Equal(f.now.AddDate(0, 0, next)) {
			t.Fatalf("retry day %d: %v", day, sub.RetryAt)
		}
	}
	f.run(t, f.now.AddDate(0, 0, 13))
	if f.g.calls.Load() != 4 {
		t.Fatal("extra attempt")
	}
	f.run(t, f.now.AddDate(0, 0, 14))
	sub = f.sub(t)
	if sub.Status != db.SubscriptionStatusCanceled || !sub.EndsAt.Valid {
		t.Fatal("not expired")
	}
	plan, err = entitlement.New(f.d.Queries).ResolvePlan(f.ctx, f.account)
	if err != nil || plan.PlanSlug != "free" {
		t.Fatalf("expiry entitlements %+v %v", plan, err)
	}
	if f.eventCount(t, "subscription.past_due") != 1 || f.eventCount(t, "subscription.expired") != 1 || f.eventCount(t, "payment.failed") != 4 {
		t.Fatal("dunning events")
	}
}
func TestRetryRecovery(t *testing.T) {
	f := setup(t)
	f.g.success = false
	f.run(t, f.now)
	f.g.success = true
	f.run(t, f.now.AddDate(0, 0, 1))
	sub := f.sub(t)
	if sub.Status != db.SubscriptionStatusActive || sub.GraceUntil.Valid || sub.RetryAt.Valid {
		t.Fatalf("not recovered %+v", sub)
	}
}
func TestConcurrentRenewalNeverDoubleCharges(t *testing.T) {
	f := setup(t)
	var wg sync.WaitGroup
	errs := make(chan error, 16)
	for range 16 {
		wg.Add(1)
		go func() { defer wg.Done(); errs <- f.s.ProcessRenewals(f.ctx, f.now, usecase.DefaultRenewalPolicy()) }()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Error(err)
		}
	}
	if f.g.calls.Load() != 1 {
		t.Fatalf("charged %d times", f.g.calls.Load())
	}
}
func TestUnknownOutcomeNeverRetried(t *testing.T) {
	f := setup(t)
	f.g.err = errors.New("connection lost after request")
	f.run(t, f.now)
	f.run(t, f.now.AddDate(0, 0, 1))
	f.run(t, f.now.AddDate(0, 0, 7))
	if f.g.calls.Load() != 1 {
		t.Fatal("ambiguous charge retried")
	}
	if err := f.s.ChangePlan(f.ctx, f.payer, uuid.New()); !errors.Is(err, usecase.ErrInvalidRequest) {
		t.Fatal(err)
	}
	f.run(t, f.now.AddDate(0, 0, 14))
	if f.sub(t).Status != db.SubscriptionStatusCanceled {
		t.Fatal("unknown did not expire")
	}
}
func TestCancelAtPeriodEndAndUndo(t *testing.T) {
	f := setup(t)
	f.exec(t, "UPDATE subscriptions SET next_billing_at=$2 WHERE id=$1", f.id, f.now.Add(time.Hour))
	if err := f.s.CancelSubscription(f.ctx, f.payer); err != nil {
		t.Fatal(err)
	}
	if !f.sub(t).CancelAtPeriodEnd {
		t.Fatal("not scheduled")
	}
	f.run(t, f.now)
	if f.sub(t).Status != db.SubscriptionStatusActive {
		t.Fatal("canceled early")
	}
	if err := f.s.ScheduleCancellation(f.ctx, f.payer, false); err != nil {
		t.Fatal(err)
	}
	if f.sub(t).CancelAtPeriodEnd {
		t.Fatal("undo failed")
	}
	if err := f.s.CancelSubscription(f.ctx, f.payer); err != nil {
		t.Fatal(err)
	}
	f.run(t, f.now.Add(2*time.Hour))
	if f.sub(t).Status != db.SubscriptionStatusCanceled || f.g.calls.Load() != 0 {
		t.Fatal("cancel charged or did not end")
	}
	if f.eventCount(t, "subscription.canceled") != 1 {
		t.Fatal("cancel event")
	}
}
func TestAccountIsolationAndAuthorization(t *testing.T) {
	f := setup(t)
	other := f.d.CreateUser(t)
	payer := f.payer
	payer.Account.ID = other.AccountID
	if err := f.s.CancelSubscription(f.ctx, payer); !errors.Is(err, usecase.ErrForbidden) {
		t.Fatal("mismatched principal", err)
	}
	ctx := authctx.WithPrincipal(f.ctx, authctx.Principal{Account: payer.Account, Permissions: []string{rbac.PermAppBillingWrite}})
	if err := f.s.CancelSubscription(ctx, payer); !errors.Is(err, usecase.ErrNotFound) {
		t.Fatal("cross-account", err)
	}
	for _, role := range []string{authctx.AccountRoleMember, authctx.AccountRoleOfficeManager} {
		payer = f.payer
		payer.Account.Role = role
		if err := f.s.CancelSubscription(f.ctx, payer); !errors.Is(err, usecase.ErrForbidden) {
			t.Fatal("role", err)
		}
	}
	ctx = authctx.WithPrincipal(f.ctx, authctx.Principal{Account: f.payer.Account})
	if err := f.s.CancelSubscription(ctx, f.payer); !errors.Is(err, usecase.ErrForbidden) {
		t.Fatal("permission", err)
	}
	payer = f.payer
	payer.Account.Role = authctx.AccountRoleAdmin
	if err := f.s.CancelSubscription(f.ctx, payer); err != nil {
		t.Fatal("admin", err)
	}
	if !f.sub(t).CancelAtPeriodEnd {
		t.Fatal("admin failed")
	}
}
func TestPlanUpgradeProrationAndDowngrade(t *testing.T) {
	f := setup(t)
	f.exec(t, "UPDATE subscriptions SET next_billing_at=$2,period_started_at=$3 WHERE id=$1", f.id, f.now.Add(15*24*time.Hour), f.now.Add(-15*24*time.Hour))
	var target uuid.UUID
	if err := f.d.Pool.QueryRow(f.ctx, `INSERT INTO plans(slug,name,billing_interval,price_minor,currency,is_active)
 VALUES($1,'Higher','month',20000,'TRY',true) RETURNING uuid`, pgtest.Unique()).Scan(&target); err != nil {
		t.Fatal(err)
	}
	if err := f.s.ChangePlan(f.ctx, f.payer, target); err != nil {
		t.Fatal(err)
	}
	if f.g.amount.Load() != 5000 {
		t.Fatalf("proration %d", f.g.amount.Load())
	}
	sub := f.sub(t)
	if sub.PlanID == f.plan {
		t.Fatal("upgrade not immediate")
	}
	var original uuid.UUID
	if err := f.d.Pool.QueryRow(f.ctx, "SELECT uuid FROM plans WHERE id=$1", f.plan).Scan(&original); err != nil {
		t.Fatal(err)
	}
	if err := f.s.ChangePlan(f.ctx, f.payer, original); err != nil {
		t.Fatal(err)
	}
	if f.sub(t).PlanID != sub.PlanID || !f.sub(t).PendingPlanID.Valid {
		t.Fatal("downgrade applied early")
	}
	f.run(t, f.now.Add(16*24*time.Hour))
	if f.sub(t).PlanID != f.plan || f.g.amount.Load() != 10000 {
		t.Fatal("downgrade not billed at boundary")
	}
}
func TestUnsupportedProviderDoesNotSucceed(t *testing.T) {
	f := setup(t)
	f.g.err = payment.ErrRecurringUnsupported
	f.run(t, f.now)
	if f.sub(t).Status != db.SubscriptionStatusPastDue || f.eventCount(t, "payment.succeeded") != 0 {
		t.Fatal("unsupported faked success")
	}
}

type failSettlement struct {
	runner usecase.TransactionRunner
	calls  int
}

func (r *failSettlement) WithTx(ctx context.Context, fn func(*db.Queries) error) error {
	r.calls++
	if r.calls == 2 {
		return errors.New("database unavailable after capture")
	}
	return r.runner.WithTx(ctx, fn)
}
func TestCaptureThenDatabaseFailureNeverRecharges(t *testing.T) {
	f := setup(t)
	runner := &failSettlement{runner: repository.NewPostgres(f.d.Pool)}
	svc := usecase.New(f.d.Queries, nil, "https://comcode.test", "", nil, runner)
	svc.SetRecurringCharger(f.g)
	if err := svc.ProcessRenewals(f.ctx, f.now, usecase.DefaultRenewalPolicy()); err == nil {
		t.Fatal("expected settlement failure")
	}
	if f.g.calls.Load() != 1 {
		t.Fatal("no capture")
	}
	f.run(t, f.now.Add(time.Hour))
	f.run(t, f.now.AddDate(0, 0, 7))
	if f.g.calls.Load() != 1 {
		t.Fatal("captured payment repeated")
	}
	f.run(t, f.now.AddDate(0, 0, 14))
	if f.sub(t).Status != db.SubscriptionStatusCanceled {
		t.Fatal("crashed attempt did not expire")
	}
}
func TestTrialAndGraceDeadlineEntitlements(t *testing.T) {
	f := setup(t)
	f.exec(t, "UPDATE subscriptions SET status='trialing' WHERE id=$1", f.id)
	plan, err := entitlement.New(f.d.Queries).ResolvePlan(f.ctx, f.account)
	if err != nil || plan.PlanID != f.plan {
		t.Fatal("trial not entitled", err)
	}
	f.run(t, f.now)
	if f.g.calls.Load() != 1 || f.sub(t).Status != db.SubscriptionStatusActive {
		t.Fatal("trial not renewed")
	}
	f.exec(t, "UPDATE subscriptions SET status='past_due',grace_until=now()-interval '1 second' WHERE id=$1", f.id)
	plan, err = entitlement.New(f.d.Queries).ResolvePlan(f.ctx, f.account)
	if err != nil || plan.PlanSlug != "free" {
		t.Fatal("grace deadline not enforced before worker", err)
	}
}
