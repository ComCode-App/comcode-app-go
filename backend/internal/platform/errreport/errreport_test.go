package errreport

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/reqid"
)

// fakeSentry is a Sentry-compatible ingest endpoint that records every request.
type fakeSentry struct {
	srv    *httptest.Server
	mu     sync.Mutex
	bodies []string
}

func newFakeSentry(t *testing.T) *fakeSentry {
	t.Helper()
	f := &fakeSentry{}
	f.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		f.mu.Lock()
		f.bodies = append(f.bodies, r.URL.Path+"\n"+string(b))
		f.mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(f.srv.Close)
	return f
}

// dsn is a throwaway DSN pointing at the local test server.
func (f *fakeSentry) dsn(project string) string {
	return strings.Replace(f.srv.URL, "http://", "http://publickey@", 1) + "/" + project
}

func (f *fakeSentry) received() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return strings.Join(f.bodies, "\n")
}

// waitFor polls until the server received something. sentry-go's telemetry scheduler can
// pop the event from its buffer before handing it to the transport queue; a Flush landing
// in that gap sees both empty and returns at once (CI flake).
func (f *fakeSentry) waitFor(t *testing.T) string {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for {
		if got := f.received(); got != "" || time.Now().After(deadline) {
			return got
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func resetDSN(t *testing.T) {
	t.Cleanup(func() { _ = SetDSN("") })
}

func TestDisabledWithoutDSN(t *testing.T) {
	flush, err := Init(Config{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	flush()
	if Enabled() {
		t.Fatal("empty DSN must keep reporting disabled")
	}
	// Must be safe no-ops.
	CaptureError(context.Background(), errors.New("x"), nil)
	CapturePanic(context.Background(), "x", nil)
	CaptureMessage(context.Background(), "x", nil)
	if _, ok := CaptureTest(context.Background(), "x", nil); ok {
		t.Fatal("CaptureTest must report false while disabled")
	}
}

func TestCaptureErrorSendsEventWithRequestID(t *testing.T) {
	f := newFakeSentry(t)
	flush, err := Init(Config{DSN: f.dsn("1"), Environment: "test", ServerName: "api"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	resetDSN(t)

	ctx := reqid.With(context.Background(), "rid-sentry-1")
	CaptureError(ctx, errors.New("db exploded"), map[string]string{"http.path": "/v1/x"})
	flush()

	got := f.waitFor(t)
	if got == "" {
		t.Fatal("no event delivered")
	}
	for _, want := range []string{"/api/1/", "db exploded", "rid-sentry-1", "/v1/x"} {
		if !strings.Contains(got, want) {
			t.Errorf("event missing %q\n%s", want, got)
		}
	}
}

func TestSetDSNSwapsClientWithoutRestart(t *testing.T) {
	first, second := newFakeSentry(t), newFakeSentry(t)
	if _, err := Init(Config{DSN: first.dsn("1"), ServerName: "api"}, nil); err != nil {
		t.Fatal(err)
	}
	resetDSN(t)

	if err := SetDSN(second.dsn("2")); err != nil {
		t.Fatal(err)
	}
	id, ok := CaptureTest(context.Background(), "after swap", map[string]string{"source": "test"})
	if !ok || id == "" {
		t.Fatalf("CaptureTest = %q, %v", id, ok)
	}
	got := second.waitFor(t)
	if !strings.Contains(got, "/api/2/") || !strings.Contains(got, "after swap") {
		t.Fatalf("second server got %q", got)
	}
	if first.received() != "" {
		t.Fatalf("old DSN still received events: %q", first.received())
	}

	if err := SetDSN(""); err != nil {
		t.Fatal(err)
	}
	if Enabled() {
		t.Fatal("empty DSN must disable reporting")
	}
}

func TestValidateDSN(t *testing.T) {
	if err := ValidateDSN(""); err != nil {
		t.Fatalf("empty: %v", err)
	}
	if err := ValidateDSN("https://key@errors.example.test/42"); err != nil {
		t.Fatalf("valid: %v", err)
	}
	for _, bad := range []string{"not a dsn", "https://errors.example.test/42", "ftp://key@x/1"} {
		if ValidateDSN(bad) == nil {
			t.Errorf("%q accepted", bad)
		}
	}
	if SetDSN("not a dsn") == nil {
		t.Fatal("SetDSN accepted an invalid DSN")
	}
}
