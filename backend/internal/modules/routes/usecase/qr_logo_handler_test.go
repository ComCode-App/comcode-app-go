package usecase_test

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/routes/handler"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/authctx"
	"github.com/google/uuid"
)

type stubLogoLimiter struct {
	allow bool
	users []string
}

func (s *stubLogoLimiter) AllowQRLogo(_ context.Context, userID string) (bool, time.Duration) {
	s.users = append(s.users, userID)
	if s.allow {
		return true, 0
	}
	return false, 90 * time.Second
}

func TestQRLogoHandler(t *testing.T) {
	f := newQRFixture(t)
	limiter := &stubLogoLimiter{allow: true}
	h := handler.New(f.svc)
	h.SetQRLogoLimiter(limiter)
	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1/routes/{uuid}/qr/logo", h.QRLogo)
	user := uuid.New()
	do := func(route uuid.UUID, account authctx.Account, query string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, "/v1/routes/"+route.String()+"/qr/logo"+query, nil)
		req = req.WithContext(authctx.WithPrincipal(req.Context(), authctx.Principal{UserID: user, Account: account}))
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		return rec
	}
	ok := "?profile_uuid=" + f.logo.String() + "&field=avatar"

	rec := do(f.route, f.account, ok)
	if rec.Code != http.StatusOK || rec.Header().Get("Content-Type") != "image/png" {
		t.Fatalf("logo: %d %s", rec.Code, rec.Body.String())
	}
	if !bytes.Equal(rec.Body.Bytes(), f.logoPNG) {
		t.Fatal("logo body differs from the stored avatar")
	}
	if rec.Header().Get("X-Content-Type-Options") != "nosniff" || rec.Header().Get("Cache-Control") != "private, max-age=300" {
		t.Fatalf("headers: %v", rec.Header())
	}
	if len(limiter.users) != 1 || limiter.users[0] != user.String() {
		t.Fatalf("limiter keyed by user: %v", limiter.users)
	}

	bad := map[string]*httptest.ResponseRecorder{
		"no profile":    do(f.route, f.account, "?field=avatar"),
		"bad profile":   do(f.route, f.account, "?profile_uuid=nope&field=avatar"),
		"unknown field": do(f.route, f.account, "?profile_uuid="+f.logo.String()+"&field=banner"),
		"other profile": do(f.route, f.account, "?profile_uuid="+uuid.NewString()+"&field=avatar"),
	}
	for name, rec := range bad {
		if rec.Code != http.StatusBadRequest {
			t.Errorf("%s: %d", name, rec.Code)
		}
	}
	if rec := do(f.route, authctx.Account{ID: 99, Role: authctx.AccountRoleOwner}, ok); rec.Code != http.StatusNotFound {
		t.Fatalf("foreign route: %d", rec.Code)
	}

	limiter.allow = false
	rec = do(f.route, f.account, ok)
	if rec.Code != http.StatusTooManyRequests || rec.Header().Get("Retry-After") != "91" {
		t.Fatalf("rate limit: %d %q", rec.Code, rec.Header().Get("Retry-After"))
	}
}
