package appversion

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/clientip"
)

func TestParseAndLess(t *testing.T) {
	cases := []struct {
		a, b string
		less bool
	}{
		{"1.0.2", "1.0.3", true},
		{"1.0.3", "1.0.3", false},
		{"1.10.0", "1.9.9", false},
		{"1.9", "1.9.1", true},
		{"2", "1.99.99", false},
		{"v1.2.3", "1.2.4", true},
		{"1.2.3-beta.1", "1.2.3", false},
		{"1.2.3+42", "1.2.4", true},
	}
	for _, tc := range cases {
		a, ok := Parse(tc.a)
		if !ok {
			t.Fatalf("parse %q failed", tc.a)
		}
		b, ok := Parse(tc.b)
		if !ok {
			t.Fatalf("parse %q failed", tc.b)
		}
		if got := a.Less(b); got != tc.less {
			t.Errorf("%s < %s = %v, want %v", tc.a, tc.b, got, tc.less)
		}
	}
	for _, bad := range []string{"", "abc", "1..2", "1.2.3.4", "-1", "1.2.x"} {
		if _, ok := Parse(bad); ok {
			t.Errorf("Parse(%q) accepted", bad)
		}
	}
}

func TestValid(t *testing.T) {
	for _, s := range []string{"1", "1.2", "1.2.3", "10.20.300"} {
		if !Valid(s) {
			t.Errorf("Valid(%q) = false", s)
		}
	}
	for _, s := range []string{"", "v1.2.3", "1.2.3-beta", "1.2.3.4", " 1.2", "1.a"} {
		if Valid(s) {
			t.Errorf("Valid(%q) = true", s)
		}
	}
}

func gated(minVersion string, err error) http.Handler {
	ok := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	return Gate(func(context.Context) (string, error) { return minVersion, err })(ok)
}

func mobileRequest(path, version string) *http.Request {
	r := httptest.NewRequest(http.MethodGet, path, nil)
	r.Header.Set(HeaderClient, ClientMobile)
	r.Header.Set(HeaderPlatform, "ios")
	if version != "" {
		r.Header.Set(HeaderVersion, version)
	}
	return r
}

func TestGateRefusesOutdatedMobileClient(t *testing.T) {
	rec := httptest.NewRecorder()
	gated("1.2.0", nil).ServeHTTP(rec, mobileRequest("/v1/auth/me", "1.1.9"))
	if rec.Code != http.StatusUpgradeRequired {
		t.Fatalf("status = %d, want 426", rec.Code)
	}
	var body struct {
		Success bool `json:"success"`
		Error   struct {
			Code    string `json:"code"`
			Details []struct {
				Field   string `json:"field"`
				Message string `json:"message"`
			} `json:"details"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Success || body.Error.Code != "APP_UPDATE_REQUIRED" {
		t.Fatalf("body = %s", rec.Body.String())
	}
	if len(body.Error.Details) != 1 || body.Error.Details[0].Field != "min_version" || body.Error.Details[0].Message != "1.2.0" {
		t.Fatalf("details = %+v", body.Error.Details)
	}
}

func TestGatePasses(t *testing.T) {
	web := httptest.NewRequest(http.MethodGet, "/v1/auth/me", nil)
	web.Header.Set(HeaderVersion, "0.0.1") // a version without X-Client is ignored
	short := mobileRequest("/v1/auth/me", "1.0.0")
	short.Header.Set(HeaderClient, "Mobile")
	cases := []struct {
		name     string
		min      string
		err      error
		req      *http.Request
		wantCode int
	}{
		{"current mobile app", "1.2.0", nil, mobileRequest("/v1/auth/me", "1.2.0"), http.StatusNoContent},
		{"newer mobile app", "1.2.0", nil, mobileRequest("/v1/auth/me", "1.10.0"), http.StatusNoContent},
		{"no minimum configured", "", nil, mobileRequest("/v1/auth/me", "0.1.0"), http.StatusNoContent},
		{"app config stays reachable", "1.2.0", nil, mobileRequest("/v1/app/config", "1.0.0"), http.StatusNoContent},
		{"health checks are not gated", "1.2.0", nil, mobileRequest("/healthz", "1.0.0"), http.StatusNoContent},
		{"web request", "1.2.0", nil, web, http.StatusNoContent},
		{"missing version header", "1.2.0", nil, mobileRequest("/v1/auth/me", ""), http.StatusNoContent},
		{"unparsable version header", "1.2.0", nil, mobileRequest("/v1/auth/me", "latest"), http.StatusNoContent},
		{"settings read error fails open", "", errors.New("db down"), mobileRequest("/v1/auth/me", "0.1.0"), http.StatusNoContent},
		{"short X-Client value is gated too", "1.2.0", nil, short, http.StatusUpgradeRequired},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			gated(tc.min, tc.err).ServeHTTP(rec, tc.req)
			if rec.Code != tc.wantCode {
				t.Fatalf("status = %d, want %d", rec.Code, tc.wantCode)
			}
		})
	}
}

// Mobile requests reach Go from the Next.js gateway (a trusted proxy) with the edge proxy's
// X-Forwarded-For chain. The app can put anything in its own X-Forwarded-For; only the hop the
// edge appended (the real peer) counts, exactly like a browser request through the cookie BFF.
func TestMobileRequestClientIPThroughTrustedProxies(t *testing.T) {
	trusted, err := clientip.ParseTrusted(clientip.DefaultTrustedProxies)
	if err != nil {
		t.Fatal(err)
	}
	var seen string
	h := clientip.New(trusted).Middleware(Gate(func(context.Context) (string, error) { return "1.0.0", nil })(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			seen = clientip.String(r)
			w.WriteHeader(http.StatusNoContent)
		}),
	))

	cases := []struct {
		name, remote, xff, want string
	}{
		// Gateway container → Go; Traefik appended the phone's address after the app-supplied value.
		{"app-supplied prefix is ignored", "172.18.0.5:41000", "10.1.2.3, 1.1.1.1, 203.0.113.50", "203.0.113.50"},
		{"edge and gateway hops are skipped", "172.18.0.5:41000", "203.0.113.50, 172.18.0.2", "203.0.113.50"},
		// Something reaching Go directly (not the gateway) cannot choose its address.
		{"untrusted peer keeps its own address", "198.51.100.9:5000", "203.0.113.50", "198.51.100.9"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			seen = ""
			r := mobileRequest("/v1/auth/login", "1.0.0")
			r.RemoteAddr = tc.remote
			r.Header.Set("X-Forwarded-For", tc.xff)
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, r)
			if rec.Code != http.StatusNoContent {
				t.Fatalf("status = %d", rec.Code)
			}
			if seen != tc.want {
				t.Fatalf("client ip = %q, want %q", seen, tc.want)
			}
		})
	}
}

func TestCached(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	calls := 0
	value := "1.0.0"
	src := Cached(func(context.Context) (string, error) {
		calls++
		return value, nil
	}, 30*time.Second, func() time.Time { return now })

	for i := 0; i < 3; i++ {
		if v, _ := src(context.Background()); v != "1.0.0" {
			t.Fatalf("v = %q", v)
		}
	}
	if calls != 1 {
		t.Fatalf("loads = %d, want 1", calls)
	}
	value = "2.0.0"
	now = now.Add(31 * time.Second)
	if v, _ := src(context.Background()); v != "2.0.0" || calls != 2 {
		t.Fatalf("after ttl: v = %q, loads = %d", v, calls)
	}
}
