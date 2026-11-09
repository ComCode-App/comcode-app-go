package usecase_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/routes/handler"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/authctx"
	"github.com/google/uuid"
)

type stubPreviewLimiter struct {
	allow bool
	users []string
}

func (s *stubPreviewLimiter) AllowQRPreview(_ context.Context, userID string) (bool, time.Duration) {
	s.users = append(s.users, userID)
	if s.allow {
		return true, 0
	}
	return false, 90 * time.Second
}

func TestPreviewQRHandler(t *testing.T) {
	f := newPreviewFixture(t)
	limiter := &stubPreviewLimiter{allow: true}
	h := handler.New(f.svc)
	h.SetQRPreviewLimiter(limiter)
	mux := http.NewServeMux()
	mux.HandleFunc("POST /v1/routes/{uuid}/qr/preview", h.PreviewQR)
	user := uuid.New()
	do := func(route uuid.UUID, account authctx.Account, query, body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/v1/routes/"+route.String()+"/qr/preview"+query, strings.NewReader(body))
		req = req.WithContext(authctx.WithPrincipal(req.Context(), authctx.Principal{UserID: user, Account: account}))
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		return rec
	}
	style := `{"foreground":"#1f2937","background":"#ffffff","module":"rounded","eye":"circle","quiet_zone":4,"logo_profile_uuid":null,"logo_field":""}`

	rec := do(f.route, f.account, "?format=svg&size=512", style)
	if rec.Code != http.StatusOK || rec.Header().Get("Content-Type") != "image/svg+xml" {
		t.Fatalf("svg: %d %s", rec.Code, rec.Body.String())
	}
	if rec.Header().Get("Cache-Control") != "private, no-store" || !strings.Contains(rec.Header().Get("Content-Security-Policy"), "sandbox") {
		t.Fatalf("headers: %v", rec.Header())
	}
	if rec.Header().Get("Content-Disposition") != "" {
		t.Fatal("preview must be inline, not an attachment")
	}
	if !strings.Contains(rec.Body.String(), `fill="#1f2937"`) {
		t.Fatal("preview ignored the unsaved style")
	}
	if rec := do(f.route, f.account, "", style); rec.Code != http.StatusOK || rec.Header().Get("Content-Type") != "image/png" {
		t.Fatalf("default png: %d", rec.Code)
	}
	if len(limiter.users) != 2 || limiter.users[0] != user.String() {
		t.Fatalf("limiter keyed by user: %v", limiter.users)
	}

	bad := map[string]*httptest.ResponseRecorder{
		"unknown field": do(f.route, f.account, "", `{"foreground":"#000000","url":"https://evil.test/logo.png"}`),
		"trailing data": do(f.route, f.account, "", style+style),
		"body too big":  do(f.route, f.account, "", `{"foreground":"`+strings.Repeat("a", 5000)+`"}`),
		"size cap":      do(f.route, f.account, "?size=2048", style),
		"size nan":      do(f.route, f.account, "?size=big", style),
		"pdf":           do(f.route, f.account, "?format=pdf", style),
	}
	for name, rec := range bad {
		if rec.Code != http.StatusBadRequest {
			t.Errorf("%s: %d", name, rec.Code)
		}
	}
	if rec := do(f.route, authctx.Account{ID: 99, Role: authctx.AccountRoleOwner}, "", style); rec.Code != http.StatusNotFound {
		t.Fatalf("foreign route: %d", rec.Code)
	}

	limiter.allow = false
	rec = do(f.route, f.account, "", style)
	if rec.Code != http.StatusTooManyRequests || rec.Header().Get("Retry-After") != "91" {
		t.Fatalf("rate limit: %d %q", rec.Code, rec.Header().Get("Retry-After"))
	}
}
