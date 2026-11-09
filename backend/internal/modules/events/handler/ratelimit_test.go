package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type denyAll struct{ calls int }

func (d *denyAll) AllowEvent(context.Context, string) (bool, time.Duration) {
	d.calls++
	return false, 30 * time.Second
}

func TestIngestIsRateLimited(t *testing.T) {
	limiter := &denyAll{}
	h := &Handler{}
	h.SetRateLimiter(limiter)
	r := httptest.NewRequest(http.MethodPost, "/v1/events", strings.NewReader(`{"type":"card.view"}`))
	r.RemoteAddr = "198.51.100.7:1234"
	w := httptest.NewRecorder()
	h.Ingest(w, r)
	if w.Code != http.StatusTooManyRequests || w.Header().Get("Retry-After") == "" || limiter.calls != 1 {
		t.Fatalf("status %d retry %q calls %d", w.Code, w.Header().Get("Retry-After"), limiter.calls)
	}
}
