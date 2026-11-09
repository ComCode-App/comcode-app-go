package usecase

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/auth/model"
	pqtotp "github.com/pquerna/otp/totp"
)

// App Store / Play review accounts (TEC-1182).

const reviewFixedCode = "246810"

func newReviewEnv(t *testing.T, registration bool) (*codeEnv, *bytes.Buffer) {
	t.Helper()
	e := newCodeEnv(t, registration)
	var logs bytes.Buffer
	e.uc.SetLogger(slog.New(slog.NewTextHandler(&logs, &slog.HandlerOptions{Level: slog.LevelInfo})))
	e.uc.SetReviewAccounts(map[string]string{
		" Review@X.io ": reviewFixedCode,
		"ghost@x.io":    "913572", // configured, but no account exists
		"off@x.io":      "802468", // configured, account disabled
	})
	return e, &logs
}

func TestReviewAccountRequestSendsNoEmail(t *testing.T) {
	e, _ := newReviewEnv(t, false)
	e.user(t, "review@x.io", "active")
	ctx := context.Background()
	if err := e.uc.RequestLoginCode(ctx, "REVIEW@x.io"); err != nil {
		t.Fatalf("must answer like any email: %v", err)
	}
	if len(e.notif.sent) != 0 || len(e.repo.otps) != 0 {
		t.Fatalf("review account must get no email and no stored code: mails=%d otps=%d", len(e.notif.sent), len(e.repo.otps))
	}
}

func TestReviewCodeSignsInOnlyItsOwnExistingEmail(t *testing.T) {
	e, logs := newReviewEnv(t, false)
	review := e.user(t, "review@x.io", "active")
	e.user(t, "other@x.io", "active")
	ctx := context.Background()

	tok, err := e.uc.VerifyLoginCode(ctx, " Review@X.io ", reviewFixedCode, "", model.SessionMeta{})
	if err != nil || tok.AccessToken == "" || tok.RefreshToken == "" {
		t.Fatalf("review code: %v", err)
	}
	// Reusable for the whole review (it is not a one-time code).
	if _, err := e.uc.VerifyLoginCode(ctx, "review@x.io", reviewFixedCode, "", model.SessionMeta{}); err != nil {
		t.Fatalf("second review sign-in: %v", err)
	}
	if e.repo.byEmail["review@x.io"].EmailVerified {
		t.Fatal("a fixed code does not prove the mailbox")
	}
	// Another account's code, another email, a wrong code: all refused.
	for _, tc := range []struct{ email, code string }{
		{"other@x.io", reviewFixedCode},
		{"review@x.io", "913572"},
		{"review@x.io", "246811"},
	} {
		if _, err := e.uc.VerifyLoginCode(ctx, tc.email, tc.code, "", model.SessionMeta{}); err != ErrInvalidEmailCode { //nolint:errorlint // identity is the point
			t.Fatalf("%s/%s: expected ErrInvalidEmailCode, got %v", tc.email, tc.code, err)
		}
	}

	out := logs.String()
	if !strings.Contains(out, "auth_review_code_used") || !strings.Contains(out, "user_id="+strconv.FormatInt(review.ID, 10)) {
		t.Fatalf("missing audit line: %s", out)
	}
	if strings.Contains(out, reviewFixedCode) || strings.Contains(strings.ToLower(out), "review@x.io") {
		t.Fatalf("audit log leaks the code or the email: %s", out)
	}
}

// Normal emailed codes keep working for a review account.
func TestReviewAccountNormalCodeStillWorks(t *testing.T) {
	e, _ := newReviewEnv(t, false)
	e.user(t, "review@x.io", "active")
	e.uc.SetReviewAccounts(nil)
	ctx := context.Background()
	_ = e.uc.RequestLoginCode(ctx, "review@x.io")
	code := e.notif.lastCode(t)
	e.uc.SetReviewAccounts(map[string]string{"review@x.io": reviewFixedCode})
	if _, err := e.uc.VerifyLoginCode(ctx, "review@x.io", code, "", model.SessionMeta{}); err != nil {
		t.Fatalf("emailed code: %v", err)
	}
}

func TestReviewCodeNeverCreatesAccount(t *testing.T) {
	e, _ := newReviewEnv(t, true) // even with self-registration open
	ctx := context.Background()
	_ = e.uc.RequestLoginCode(ctx, "ghost@x.io") // a normal sign-up code is mailed
	if _, err := e.uc.VerifyLoginCode(ctx, "ghost@x.io", "913572", "", model.SessionMeta{}); !errors.Is(err, ErrInvalidEmailCode) {
		t.Fatalf("expected ErrInvalidEmailCode, got %v", err)
	}
	if _, ok := e.repo.byEmail["ghost@x.io"]; ok {
		t.Fatal("a review code must never create an account")
	}
}

func TestReviewCodeRefusedForDisabledAccount(t *testing.T) {
	e, _ := newReviewEnv(t, false)
	e.user(t, "off@x.io", "disabled")
	ctx := context.Background()
	if _, err := e.uc.VerifyLoginCode(ctx, "off@x.io", "802468", "", model.SessionMeta{}); !errors.Is(err, ErrInvalidEmailCode) {
		t.Fatalf("disabled review account: %v", err)
	}
	// Not an active account: a normal code is mailed, like for any other address.
	_ = e.uc.RequestLoginCode(ctx, "off@x.io")
	if e.notif.count(loginCodeTemplate) != 1 {
		t.Fatal("a disabled account is not treated as a review account")
	}
}

func TestReviewAccountsDisabledWhenUnset(t *testing.T) {
	for _, accounts := range []map[string]string{nil, {}, {"": reviewFixedCode, "review@x.io": " "}} {
		e := newCodeEnv(t, false)
		e.user(t, "review@x.io", "active")
		e.uc.SetReviewAccounts(accounts)
		ctx := context.Background()
		if _, err := e.uc.VerifyLoginCode(ctx, "review@x.io", reviewFixedCode, "", model.SessionMeta{}); !errors.Is(err, ErrInvalidEmailCode) {
			t.Fatalf("%v: fixed code must not work, got %v", accounts, err)
		}
		_ = e.uc.RequestLoginCode(ctx, "review@x.io")
		if e.notif.count(loginCodeTemplate) != 1 {
			t.Fatalf("%v: a normal code must be mailed", accounts)
		}
	}
}

// MFA rules are unchanged: a review account with an authenticator still needs it, and the
// admin "2FA required" policy still applies.
func TestReviewCodeKeepsMFARules(t *testing.T) {
	e, _ := newReviewEnv(t, false)
	u := e.user(t, "review@x.io", "active")
	secret := e.enableTOTP(t, u.ID)
	ctx := context.Background()
	if _, err := e.uc.VerifyLoginCode(ctx, "review@x.io", reviewFixedCode, "", model.SessionMeta{}); !errors.Is(err, ErrMFARequired) {
		t.Fatalf("expected MFA required, got %v", err)
	}
	if _, err := e.uc.VerifyLoginCode(ctx, "review@x.io", reviewFixedCode, "000000x", model.SessionMeta{}); !errors.Is(err, ErrInvalidMFACode) {
		t.Fatalf("expected invalid mfa, got %v", err)
	}
	totpCode, _ := pqtotp.GenerateCode(secret, time.Now())
	if _, err := e.uc.VerifyLoginCode(ctx, "review@x.io", reviewFixedCode, totpCode, model.SessionMeta{}); err != nil {
		t.Fatalf("with authenticator: %v", err)
	}

	p, _ := newReviewEnv(t, false)
	p.user(t, "review@x.io", "active")
	p.uc.SetAccessPolicy(policyGate{required: true})
	if _, err := p.uc.VerifyLoginCode(ctx, "review@x.io", reviewFixedCode, "", model.SessionMeta{}); !errors.Is(err, ErrMFANotEnrolled) {
		t.Fatalf("policy: expected ErrMFANotEnrolled, got %v", err)
	}
}
