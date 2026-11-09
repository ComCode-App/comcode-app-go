package handler

import (
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
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/authctx"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/jwt"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/password"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/ratelimit"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

// loginRepo implements only what password login touches before it fails.
type loginRepo struct {
	usecase.Repository
	users map[string]model.User
}

func (r loginRepo) FindUserByEmail(_ context.Context, email string) (model.User, error) {
	u, ok := r.users[email]
	if !ok {
		return model.User{}, repository.ErrNotFound
	}
	return u, nil
}

type errorEnvelope struct {
	Success bool `json:"success"`
	Error   struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func postLogin(t *testing.T, h *Handler, email, pw string) (int, errorEnvelope) {
	t.Helper()
	body, _ := json.Marshal(map[string]string{"email": email, "password": pw})
	rec := httptest.NewRecorder()
	h.Login(rec, httptest.NewRequest(http.MethodPost, "/v1/auth/login", strings.NewReader(string(body))))
	var env errorEnvelope
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("decode: %v (%s)", err, rec.Body.String())
	}
	return rec.Code, env
}

// TEC-1101: unknown email, wrong password and a disabled account with a wrong password all get
// the identical HTTP response (status, code, message).
func TestLoginResponsesDoNotRevealAccounts(t *testing.T) {
	hash, _ := password.Hash("Secret123")
	repo := loginRepo{users: map[string]model.User{
		"known@x.io":    {ID: 1, Email: "known@x.io", PasswordHash: hash, Status: "active"},
		"disabled@x.io": {ID: 2, Email: "disabled@x.io", PasswordHash: hash, Status: "disabled"},
	}}
	tokens, _ := jwt.NewManager("test-secret-key-32-bytes-minimum!", time.Minute, time.Hour)
	h := &Handler{uc: usecase.New(repo, tokens)}

	wantStatus, want := postLogin(t, h, "nobody@x.io", "Wrong1234")
	if wantStatus != http.StatusUnauthorized || want.Error.Code != "INVALID_CREDENTIALS" {
		t.Fatalf("unknown email: got %d %+v", wantStatus, want)
	}
	for _, email := range []string{"known@x.io", "disabled@x.io"} {
		status, got := postLogin(t, h, email, "Wrong1234")
		if status != wantStatus || got != want {
			t.Fatalf("%s: response %d %+v differs from unknown email %d %+v", email, status, got, wantStatus, want)
		}
	}
	if status, got := postLogin(t, h, "disabled@x.io", "Secret123"); status != http.StatusForbidden {
		t.Fatalf("disabled account with the correct password: got %d %+v", status, got)
	}
}

// TEC-1101: every auth form endpoint answers 429 RATE_LIMITED with Retry-After once its budget is spent.
func TestAuthEndpointsRateLimited(t *testing.T) {
	mr := miniredis.RunT(t)
	lim := ratelimit.New(redis.NewClient(&redis.Options{Addr: mr.Addr()}), "test")
	h := &Handler{}
	h.SetRateLimiter(lim)
	userID := uuid.New()
	const ip = "203.0.113.7"

	cases := []struct {
		name    string
		path    string
		body    string
		handler http.HandlerFunc
		exhaust func(ctx context.Context)
		auth    bool
	}{
		{"login", "/v1/auth/login", `{"email":"a@x.io","password":"x"}`, h.Login, func(ctx context.Context) {
			for i := 0; i < 10; i++ {
				lim.AllowLogin(ctx, ip, "a@x.io")
			}
		}, false},
		{"forgot", "/v1/auth/password/forgot", `{"email":"a@x.io"}`, h.ForgotPassword, func(ctx context.Context) {
			for i := 0; i < 5; i++ {
				lim.AllowForgotPassword(ctx, ip, "a@x.io")
			}
		}, false},
		{"reset", "/v1/auth/password/reset", `{"email":"a@x.io","code":"1","password":"x"}`, h.ResetPassword, func(ctx context.Context) {
			for i := 0; i < 10; i++ {
				lim.AllowResetPassword(ctx, ip, "a@x.io")
			}
		}, false},
		{"verify submit", "/v1/auth/email/verify", `{"email":"a@x.io","code":"1"}`, h.VerifyEmail, func(ctx context.Context) {
			for i := 0; i < 10; i++ {
				lim.AllowEmailVerificationSubmit(ctx, ip, "a@x.io")
			}
		}, false},
		{"verify request", "/v1/auth/email/verify-request", `{}`, h.VerifyEmailRequest, func(ctx context.Context) {
			for i := 0; i < 5; i++ {
				lim.AllowEmailVerificationRequest(ctx, userID.String(), ip)
			}
		}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tc.exhaust(context.Background())
			r := httptest.NewRequest(http.MethodPost, tc.path, strings.NewReader(tc.body))
			r.RemoteAddr = ip + ":1234"
			if tc.auth {
				r = r.WithContext(authctx.WithPrincipal(r.Context(), authctx.Principal{UserID: userID}))
			}
			rec := httptest.NewRecorder()
			tc.handler(rec, r)
			if rec.Code != http.StatusTooManyRequests {
				t.Fatalf("expected 429, got %d: %s", rec.Code, rec.Body.String())
			}
			if rec.Header().Get("Retry-After") == "" {
				t.Fatal("expected Retry-After")
			}
			var env errorEnvelope
			_ = json.Unmarshal(rec.Body.Bytes(), &env)
			if env.Error.Code != "RATE_LIMITED" {
				t.Fatalf("expected RATE_LIMITED, got %+v", env)
			}
		})
	}
}
