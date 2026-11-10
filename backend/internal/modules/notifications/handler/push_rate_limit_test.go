package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/authctx"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/ratelimit"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

func pushRequest(method, path, body string, userID int64) *http.Request {
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	return r.WithContext(authctx.WithPrincipal(r.Context(), authctx.Principal{UserID: uuid.New(), UserInternal: userID}))
}

const badPushBody = `{"token":"bad","platform":"ios"}`

// TEC-1186: push device registration and removal share 10 requests per minute per user. An
// invalid body is rejected before the service is touched (svc is nil), so the calls under the
// limit answer 400 and the next one 429.
func TestPushDeviceRateLimitPerUser(t *testing.T) {
	mr := miniredis.RunT(t)
	h := New(nil)
	h.SetRateLimiter(ratelimit.New(redis.NewClient(&redis.Options{Addr: mr.Addr()}), "test"))

	for i := 0; i < pushDeviceLimit-1; i++ {
		w := httptest.NewRecorder()
		h.RegisterPushDevice(w, pushRequest(http.MethodPost, "/v1/notifications/push-devices", badPushBody, 7))
		if w.Code != http.StatusBadRequest {
			t.Fatalf("call %d: status %d, want 400", i+1, w.Code)
		}
	}
	// DELETE counts against the same budget (empty token: 400 without the service).
	w := httptest.NewRecorder()
	h.DeletePushDevice(w, pushRequest(http.MethodDelete, "/v1/notifications/push-devices/", "", 7))
	if w.Code != http.StatusBadRequest {
		t.Fatalf("delete under limit: status %d, want 400", w.Code)
	}

	for _, call := range []func(http.ResponseWriter, *http.Request){h.RegisterPushDevice, h.DeletePushDevice} {
		w = httptest.NewRecorder()
		call(w, pushRequest(http.MethodPost, "/v1/notifications/push-devices", badPushBody, 7))
		if w.Code != http.StatusTooManyRequests {
			t.Fatalf("over limit: status %d, want 429", w.Code)
		}
		if w.Header().Get("Retry-After") == "" {
			t.Fatal("over limit: missing Retry-After")
		}
		if !strings.Contains(w.Body.String(), "RATE_LIMITED") {
			t.Fatalf("over limit: body %s, want RATE_LIMITED", w.Body.String())
		}
	}

	// Another user has their own budget.
	w = httptest.NewRecorder()
	h.RegisterPushDevice(w, pushRequest(http.MethodPost, "/v1/notifications/push-devices", badPushBody, 8))
	if w.Code != http.StatusBadRequest {
		t.Fatalf("other user: status %d, want 400", w.Code)
	}

	// The window is one minute.
	mr.FastForward(61 * time.Second)
	w = httptest.NewRecorder()
	h.RegisterPushDevice(w, pushRequest(http.MethodPost, "/v1/notifications/push-devices", badPushBody, 7))
	if w.Code != http.StatusBadRequest {
		t.Fatalf("after the window: status %d, want 400", w.Code)
	}
}

func TestPushDeviceNoLimiterConfigured(t *testing.T) {
	h := New(nil)
	for i := 0; i < pushDeviceLimit*3; i++ {
		w := httptest.NewRecorder()
		h.RegisterPushDevice(w, pushRequest(http.MethodPost, "/v1/notifications/push-devices", badPushBody, 7))
		if w.Code != http.StatusBadRequest {
			t.Fatalf("call %d: status %d, want 400 (no limiter)", i+1, w.Code)
		}
	}
}
