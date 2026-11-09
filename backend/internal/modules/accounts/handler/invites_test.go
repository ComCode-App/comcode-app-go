package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/authctx"
	"github.com/google/uuid"
)

// fakeInviteLimiter allows nothing and records what was checked.
type fakeInviteLimiter struct {
	previewIP            string
	acceptUser, acceptIP string
}

func (f *fakeInviteLimiter) AllowInvitePreview(_ context.Context, ip string) (bool, time.Duration) {
	f.previewIP = ip
	return false, time.Minute
}

func (f *fakeInviteLimiter) AllowInviteAccept(_ context.Context, userID, ip string) (bool, time.Duration) {
	f.acceptUser, f.acceptIP = userID, ip
	return false, time.Minute
}

func acceptRequest(p authctx.Principal) *http.Request {
	r := httptest.NewRequest(http.MethodPost, "/v1/invites/tok/accept", nil)
	r.SetPathValue("token", "tok")
	r.RemoteAddr = "203.0.113.7:1234"
	return r.WithContext(authctx.WithPrincipal(r.Context(), p))
}

// TestAcceptRefusedWhileImpersonating: an operator impersonating a user must not join a company
// on the user's behalf (TEC-1104). The use case is never reached.
func TestAcceptRefusedWhileImpersonating(t *testing.T) {
	h := NewInvites(nil, nil)
	operator := uuid.New()
	w := httptest.NewRecorder()
	h.Accept(w, acceptRequest(authctx.Principal{UserID: uuid.New(), UserInternal: 7, ImpersonatorUserID: &operator}))
	if w.Code != http.StatusForbidden {
		t.Fatalf("status %d, body %s", w.Code, w.Body)
	}
	if got := w.Header().Get("Referrer-Policy"); got != "no-referrer" {
		t.Fatalf("Referrer-Policy %q", got)
	}
}

// TestInviteRoutesAreRateLimited: preview is limited per client IP, accept per user and IP, and
// a limited response still carries no-referrer (TEC-1104).
func TestInviteRoutesAreRateLimited(t *testing.T) {
	lim := &fakeInviteLimiter{}
	h := NewInvites(nil, nil)
	h.SetRateLimiter(lim)

	r := httptest.NewRequest(http.MethodGet, "/v1/invites/tok", nil)
	r.SetPathValue("token", "tok")
	r.RemoteAddr = "203.0.113.7:1234"
	w := httptest.NewRecorder()
	h.Preview(w, r)
	if w.Code != http.StatusTooManyRequests || w.Header().Get("Retry-After") == "" {
		t.Fatalf("preview: status %d headers %v", w.Code, w.Header())
	}
	if w.Header().Get("Referrer-Policy") != "no-referrer" {
		t.Fatal("preview without no-referrer")
	}
	if lim.previewIP != "203.0.113.7" {
		t.Fatalf("preview limited by %q", lim.previewIP)
	}

	user := uuid.New()
	w = httptest.NewRecorder()
	h.Accept(w, acceptRequest(authctx.Principal{UserID: user, UserInternal: 7}))
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("accept: status %d", w.Code)
	}
	if lim.acceptUser != user.String() || lim.acceptIP != "203.0.113.7" {
		t.Fatalf("accept limited by %q / %q", lim.acceptUser, lim.acceptIP)
	}
}
