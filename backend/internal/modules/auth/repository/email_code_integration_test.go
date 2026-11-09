package repository_test

// Postgres integration tests for email sign-in codes (TEC-1121). Skipped unless
// TEST_DATABASE_URL is set; see internal/testutil/pgtest.

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/auth/model"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/auth/repository"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/auth/usecase"
	notifmodel "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/notifications/model"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/jwt"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/testutil/pgtest"
)

func TestMain(m *testing.M) { pgtest.Main(m) }

type openRegistration struct{ usecase.AuthSettingsGate }

func (openRegistration) RegistrationEnabled(context.Context) (bool, error) { return true, nil }
func (openRegistration) DefaultRoleID(context.Context) (*int64, error)     { return nil, nil }

type captureNotifier struct{ code string }

func (n *captureNotifier) Enqueue(_ context.Context, in notifmodel.EnqueueInput) ([]notifmodel.Notification, error) {
	if c, ok := in.TemplateVars["code"]; ok && in.TemplateCode == "auth.login_code" {
		n.code = c
	}
	return nil, nil
}

// ConsumeOTP reports a lost race, so a code is consumed exactly once.
func TestIntegrationConsumeOTPIsSingleUse(t *testing.T) {
	d := pgtest.Open(t)
	ctx := context.Background()
	repo := repository.NewPostgres(d.Pool, d.Queries)
	email := "code-" + pgtest.Unique() + "@example.com"
	if err := repo.CreateOTP(ctx, nil, email, "hash", "login_code", time.Now().Add(10*time.Minute)); err != nil {
		t.Fatalf("login_code must pass the type check: %v", err)
	}
	id, _, _, _, err := repo.GetActiveOTP(ctx, email, "login_code")
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.ConsumeOTP(ctx, id); err != nil {
		t.Fatalf("first consume: %v", err)
	}
	if err := repo.ConsumeOTP(ctx, id); !errors.Is(err, repository.ErrNotFound) {
		t.Fatalf("second consume must lose: %v", err)
	}
}

// Sign-up by email code creates a verified, passwordless user with its personal account and
// signs in on it; the stored code is hashed.
func TestIntegrationEmailCodeSignUp(t *testing.T) {
	d := pgtest.Open(t)
	ctx := context.Background()
	repo := repository.NewPostgres(d.Pool, d.Queries)
	tokens, err := jwt.NewManager("test-secret-key-32-bytes-minimum!", time.Minute, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	uc := usecase.New(repo, tokens)
	uc.SetAuthSettings(openRegistration{})
	notif := &captureNotifier{}
	uc.SetNotifier(notif)

	email := "signup-" + pgtest.Unique() + "@example.com"
	if err := uc.RequestLoginCode(ctx, email); err != nil {
		t.Fatal(err)
	}
	if notif.code == "" {
		t.Fatal("no code sent")
	}
	var stored string
	if err := d.Pool.QueryRow(ctx, `SELECT code_hash FROM otp_codes WHERE email = $1 AND type = 'login_code'`, email).Scan(&stored); err != nil {
		t.Fatal(err)
	}
	if stored == notif.code {
		t.Fatal("code must be stored hashed")
	}
	tok, err := uc.VerifyLoginCode(ctx, email, notif.code, "", model.SessionMeta{})
	if err != nil || tok.AccessToken == "" {
		t.Fatalf("verify: %v", err)
	}
	user, err := repo.FindUserByEmail(ctx, email)
	if err != nil {
		t.Fatal(err)
	}
	if !user.EmailVerified || user.PasswordSet || user.Status != "active" {
		t.Fatalf("user = %+v", user)
	}
	acc, err := repo.GetPersonalAccount(ctx, user.ID)
	if err != nil || acc.Type != "personal" || acc.Role != "owner" {
		t.Fatalf("personal account: %+v %v", acc, err)
	}
	if _, err := uc.VerifyLoginCode(ctx, email, notif.code, "", model.SessionMeta{}); !errors.Is(err, usecase.ErrInvalidEmailCode) {
		t.Fatalf("reuse: %v", err)
	}
}
