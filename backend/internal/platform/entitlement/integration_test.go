package entitlement

import (
	"context"
	"errors"
	"testing"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/testutil/pgtest"
)

func TestMain(m *testing.M) { pgtest.Main(m) }

// TEC-942: a trialing subscription counts like an active one, so the account gets the trial
// plan's limits instead of the free plan's.
func TestTrialingSubscriptionGetsPlanLimits(t *testing.T) {
	d := pgtest.Open(t)
	ctx := context.Background()
	svc := New(d.Queries)
	user := d.CreateUser(t)

	// The seeded free plan allows 1 profile, pro-monthly 10.
	if err := svc.CheckIntUsage(ctx, user.AccountID, false, FeatureProfilesMax, 1); !errors.Is(err, ErrPlanLimitExceeded) {
		t.Fatalf("free plan: want ErrPlanLimitExceeded at 1 profile, got %v", err)
	}

	if _, err := d.Pool.Exec(ctx, `
		INSERT INTO subscriptions (account_id, plan_id, status, source, starts_at, next_billing_at)
		SELECT $1, id, 'trialing', 'manual', now(), now() + interval '14 days' FROM plans WHERE slug = 'pro-monthly'`,
		user.AccountID); err != nil {
		t.Fatal(err)
	}
	plan, err := svc.ResolvePlan(ctx, user.AccountID)
	if err != nil {
		t.Fatal(err)
	}
	if plan.PlanSlug != "pro-monthly" || plan.Source != "subscription" {
		t.Fatalf("trialing subscription must resolve to its plan, got %+v", plan)
	}
	if err := svc.CheckIntUsage(ctx, user.AccountID, false, FeatureProfilesMax, 9); err != nil {
		t.Fatalf("trial: want the pro profile limit, got %v", err)
	}
	if err := svc.CheckIntUsage(ctx, user.AccountID, false, FeatureProfilesMax, 10); !errors.Is(err, ErrPlanLimitExceeded) {
		t.Fatalf("trial: want ErrPlanLimitExceeded at the pro limit, got %v", err)
	}
}
