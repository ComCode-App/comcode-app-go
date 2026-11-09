package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/authctx"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/entitlement"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/rbac"
)

type gate struct {
	calls int
	err   error
}

func (g *gate) PrepareExport(context.Context, authctx.Principal) error {
	g.calls++
	return g.err
}

func leadsExport(h *Handler, body string) *httptest.ResponseRecorder {
	p := authctx.Principal{UserInternal: 1, Account: authctx.Account{ID: 2, Role: "owner"},
		Permissions: []string{rbac.PermAppLeadsRead}}
	r := httptest.NewRequest(http.MethodPost, "/v1/leads/export", strings.NewReader(body))
	r = r.WithContext(authctx.WithPrincipal(r.Context(), p))
	w := httptest.NewRecorder()
	h.RequestLeadsExport(w, r)
	return w
}

// TEC-941: a used-up export quota is 403 PLAN_LIMIT_EXCEEDED naming the feature; an invalid
// request is rejected before the quota is touched; without a gate exports are refused.
func TestLeadsExportQuota(t *testing.T) {
	g := &gate{err: &entitlement.QuotaError{Feature: entitlement.FeatureLeadsExportMonthlyMax, Limit: 2, Used: 2}}
	h := New(nil)
	h.SetLeadsGate(g)

	w := leadsExport(h, `{"format":"docx"}`)
	if w.Code != http.StatusBadRequest || g.calls != 0 {
		t.Fatalf("invalid format: %d, gate calls %d", w.Code, g.calls)
	}
	w = leadsExport(h, `{"format":"csv"}`)
	if w.Code != http.StatusForbidden || !strings.Contains(w.Body.String(), "PLAN_LIMIT_EXCEEDED") ||
		!strings.Contains(w.Body.String(), entitlement.FeatureLeadsExportMonthlyMax) {
		t.Fatalf("used-up quota: %d %s", w.Code, w.Body.String())
	}
	if w := leadsExport(New(nil), `{"format":"csv"}`); w.Code != http.StatusForbidden {
		t.Fatalf("no gate: want 403, got %d", w.Code)
	}
}
