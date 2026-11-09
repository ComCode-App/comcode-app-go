package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/auth/model"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/auth/repository"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/auth/usecase"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/jwt"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/ratelimit"
	"github.com/redis/go-redis/v9"
)

// codeRepo stores one active login code per email (enough for the HTTP contract).
type codeRepo struct {
	loginRepo
	codes map[string]string
}

func (r *codeRepo) InvalidateOTPs(context.Context, string, string) error { return nil }

func (r *codeRepo) CreateOTP(_ context.Context, _ *int64, email, codeHash, _ string, _ time.Time) error {
	r.codes[email] = codeHash
	return nil
}

func (r *codeRepo) GetActiveOTP(_ context.Context, email, _ string) (int64, string, int32, int32, error) {
	if h, ok := r.codes[email]; ok {
		return 1, h, 0, 5, nil
	}
	return 0, "", 0, 0, repository.ErrNotFound
}

func (r *codeRepo) IncrementOTPAttempts(context.Context, int64) (int32, int32, error) {
	return 1, 5, nil
}

func (r *codeRepo) ConsumeOTP(context.Context, int64) error { return nil }

type closedRegistration struct{ usecase.AuthSettingsGate }

func (closedRegistration) RegistrationEnabled(context.Context) (bool, error) { return false, nil }

func postJSON(t *testing.T, fn http.HandlerFunc, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	raw, _ := json.Marshal(body)
	rec := httptest.NewRecorder()
	fn(rec, httptest.NewRequest(http.MethodPost, path, strings.NewReader(string(raw))))
	return rec
}

// TEC-1121: request-code answers byte-for-byte the same for a known, a disabled and an unknown
// email; verify with a wrong code answers the same 400 INVALID_EMAIL_CODE for all of them.
func TestEmailCodeResponsesDoNotRevealAccounts(t *testing.T) {
	repo := &codeRepo{loginRepo: loginRepo{users: map[string]model.User{
		"known@x.io":    {ID: 1, Email: "known@x.io", Status: "active"},
		"disabled@x.io": {ID: 2, Email: "disabled@x.io", Status: "disabled"},
	}}, codes: map[string]string{}}
	tokens, _ := jwt.NewManager("test-secret-key-32-bytes-minimum!", time.Minute, time.Hour)
	uc := usecase.New(repo, tokens)
	uc.SetAuthSettings(closedRegistration{})
	h := &Handler{uc: uc}

	want := postJSON(t, h.RequestEmailCode, "/v1/auth/email-code/request", map[string]string{"email": "nobody@x.io"})
	if want.Code != http.StatusOK || !strings.Contains(want.Body.String(), `"accepted"`) {
		t.Fatalf("unknown email: %d %s", want.Code, want.Body.String())
	}
	for _, email := range []string{"known@x.io", "disabled@x.io"} {
		got := postJSON(t, h.RequestEmailCode, "/v1/auth/email-code/request", map[string]string{"email": email})
		if got.Code != want.Code || !bytes.Equal(got.Body.Bytes(), want.Body.Bytes()) {
			t.Fatalf("%s: %d %s differs from unknown email %d %s", email, got.Code, got.Body.String(), want.Code, want.Body.String())
		}
	}
	if _, ok := repo.codes["nobody@x.io"]; ok {
		t.Fatal("no code for an unknown email while registration is closed")
	}

	wantStatus, wantEnv := verifyCode(t, h, "nobody@x.io", "123456")
	if wantStatus != http.StatusBadRequest || wantEnv.Error.Code != "INVALID_EMAIL_CODE" {
		t.Fatalf("unknown email verify: %d %+v", wantStatus, wantEnv)
	}
	for _, email := range []string{"known@x.io", "disabled@x.io"} {
		status, env := verifyCode(t, h, email, "not-the-code")
		if status != wantStatus || env != wantEnv {
			t.Fatalf("%s: %d %+v differs from unknown email %d %+v", email, status, env, wantStatus, wantEnv)
		}
	}
}

func verifyCode(t *testing.T, h *Handler, email, code string) (int, errorEnvelope) {
	t.Helper()
	rec := postJSON(t, h.VerifyEmailCode, "/v1/auth/email-code/verify", map[string]string{"email": email, "code": code})
	var env errorEnvelope
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("decode: %v (%s)", err, rec.Body.String())
	}
	return rec.Code, env
}

// TEC-1121: both endpoints answer 429 RATE_LIMITED + Retry-After once the per-IP or the
// per-email budget is spent (the per-email one holds across IPs).
func TestEmailCodeEndpointsRateLimited(t *testing.T) {
	const ip = "203.0.113.7"
	cases := []struct {
		name    string
		path    string
		action  string
		subject string
		limit   int
		reqIP   string
		email   string
		verify  bool
	}{
		{"request per ip", "/v1/auth/email-code/request", "email_code_request_ip", ip, emailCodeRequestIPLimit, ip, "fresh@x.io", false},
		{"request per email", "/v1/auth/email-code/request", "email_code_request_email", "a@x.io", emailCodeRequestEmailLimit, "198.51.100.9", "A@x.io", false},
		{"verify per ip", "/v1/auth/email-code/verify", "email_code_verify_ip", ip, emailCodeVerifyIPLimit, ip, "fresh@x.io", true},
		{"verify per email", "/v1/auth/email-code/verify", "email_code_verify_email", "a@x.io", emailCodeVerifyEmailLimit, "198.51.100.9", "a@x.io", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mr := miniredis.RunT(t)
			lim := ratelimit.New(redis.NewClient(&redis.Options{Addr: mr.Addr()}), "test")
			h := &Handler{}
			h.SetRateLimiter(lim)
			for i := 0; i < tc.limit; i++ {
				lim.Allow(context.Background(), tc.action, tc.subject, tc.limit, emailCodeRateWindow)
			}
			body := map[string]string{"email": tc.email}
			if tc.verify {
				body["code"] = "123456"
			}
			raw, _ := json.Marshal(body)
			r := httptest.NewRequest(http.MethodPost, tc.path, strings.NewReader(string(raw)))
			r.RemoteAddr = tc.reqIP + ":1234"
			rec := httptest.NewRecorder()
			if tc.verify {
				h.VerifyEmailCode(rec, r)
			} else {
				h.RequestEmailCode(rec, r)
			}
			if rec.Code != http.StatusTooManyRequests || rec.Header().Get("Retry-After") == "" {
				t.Fatalf("expected 429 with Retry-After, got %d %s", rec.Code, rec.Body.String())
			}
			var env errorEnvelope
			_ = json.Unmarshal(rec.Body.Bytes(), &env)
			if env.Error.Code != "RATE_LIMITED" {
				t.Fatalf("expected RATE_LIMITED, got %+v", env)
			}
		})
	}
}
