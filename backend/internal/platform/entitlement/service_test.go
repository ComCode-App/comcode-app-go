package entitlement

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/authctx"
)

func intLimit(v int32) *LimitValue {
	return &LimitValue{Key: FeatureCardsMax, ValueType: "int", IntValue: &v}
}

func TestCheckIntLimitIsFailClosed(t *testing.T) {
	cases := []struct {
		name    string
		limit   *LimitValue
		current int32
		allowed bool
	}{
		{"missing from plan", nil, 0, false},
		{"limit without int value", &LimitValue{Key: FeatureCardsMax, ValueType: "int"}, 0, false},
		{"below limit", intLimit(3), 2, true},
		{"at limit", intLimit(3), 3, false},
		{"zero limit", intLimit(0), 0, false},
		{"unlimited", intLimit(Unlimited), 1_000_000, true},
	}
	for _, tc := range cases {
		err := checkIntLimit(tc.limit, int64(tc.current))
		if tc.allowed && err != nil {
			t.Errorf("%s: expected allowed, got %v", tc.name, err)
		}
		if !tc.allowed && !errors.Is(err, ErrPlanLimitExceeded) {
			t.Errorf("%s: expected ErrPlanLimitExceeded, got %v", tc.name, err)
		}
	}
}

func TestAlertPercent(t *testing.T) {
	cases := []struct {
		used  int64
		limit int32
		want  int
	}{
		{16, 20, 80}, {15, 20, 0}, {17, 20, 0}, {20, 20, 100}, {21, 20, 0},
		{1, 1, 100}, {2, 2, 100}, {1, 2, 0}, {4, 5, 80}, {0, 0, 0},
	}
	for _, tc := range cases {
		if got := alertPercent(tc.used, tc.limit); got != tc.want {
			t.Errorf("alertPercent(%d, %d) = %d, want %d", tc.used, tc.limit, got, tc.want)
		}
	}
}

// TEC-947: a bool feature the plan lacks answers 403 PLAN_LIMIT_EXCEEDED naming the feature.
func TestWriteErrorFeature(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/backgrounds", nil)
	if !WriteError(rec, req, fmt.Errorf("save: %w", &FeatureError{Feature: FeatureDesignPremium})) {
		t.Fatal("FeatureError not handled")
	}
	body := rec.Body.String()
	if rec.Code != http.StatusForbidden || !strings.Contains(body, "PLAN_LIMIT_EXCEEDED") || !strings.Contains(body, `"field":"design.premium"`) {
		t.Fatalf("got %d %s", rec.Code, body)
	}
}

type boolFeatures map[string]bool

func (b boolFeatures) CheckBool(_ context.Context, _ int64, isSuperAdmin bool, key string) (bool, error) {
	return isSuperAdmin || b[key], nil
}

func TestRequireFeature(t *testing.T) {
	ctx := context.Background()
	if err := RequireFeature(ctx, nil, 1, FeatureDesignPremium); !errors.Is(err, ErrPlanLimitExceeded) {
		t.Fatalf("nil checker must deny: %v", err)
	}
	if err := RequireFeature(ctx, boolFeatures{}, 1, FeatureDesignPremium); !errors.Is(err, ErrPlanLimitExceeded) {
		t.Fatalf("missing feature must deny: %v", err)
	}
	if err := RequireFeature(ctx, boolFeatures{FeatureDesignPremium: true}, 1, FeatureDesignPremium); err != nil {
		t.Fatalf("enabled feature: %v", err)
	}
	admin := authctx.WithPrincipal(ctx, authctx.Principal{IsSuperAdmin: true})
	if err := RequireFeature(admin, boolFeatures{}, 1, FeatureDesignPremium); err != nil {
		t.Fatalf("super admin: %v", err)
	}
}
