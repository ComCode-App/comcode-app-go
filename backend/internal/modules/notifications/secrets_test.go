package notifications

// TEC-1099: verification and password reset codes never sit readable on notification rows,
// the account owner cannot read them back through the inbox API, and the worker still mails
// the real code (also on a retry). Real PostgreSQL (TEST_DATABASE_URL), real templates.
import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	authrepo "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/auth/repository"
	authusecase "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/auth/usecase"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/notifications/handler"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/notifications/model"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/notifications/providers"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/notifications/usecase"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/authctx"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/crypto"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/jwt"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/mail"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/testutil/pgtest"
	"github.com/google/uuid"
)

var otpPattern = regexp.MustCompile(`\b[0-9]{6}\b`)

// flakyMail fails the first failures sends, then records messages.
type flakyMail struct {
	mu       sync.Mutex
	failures int
	msgs     []mail.Message
}

func (f *flakyMail) Send(_ context.Context, m mail.Message) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.failures > 0 {
		f.failures--
		return errors.New("smtp down")
	}
	f.msgs = append(f.msgs, m)
	return nil
}

type secretsFixture struct {
	d     *pgtest.DB
	svc   *usecase.Service
	auth  *authusecase.AuthUseCase
	mail  *flakyMail
	user  pgtest.User
	email string
}

func newSecretsFixture(t *testing.T, failures int) *secretsFixture {
	t.Helper()
	d := pgtest.Open(t)
	box, err := crypto.NewSecretBox(strings.Repeat("k", 32))
	if err != nil {
		t.Fatal(err)
	}
	f := &secretsFixture{d: d, mail: &flakyMail{failures: failures}, user: d.CreateUnverifiedUser(t)}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	f.svc = usecase.New(d.Queries, nil,
		[]providers.Provider{providers.InappProvider{}, providers.EmailProvider{Mail: f.mail}}, log).
		WithSecretBox(box)
	tokens, err := jwt.NewManager("secrets-fixture-jwt", time.Hour, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	f.auth = authusecase.New(authrepo.NewPostgres(d.Pool, d.Queries), tokens)
	f.auth.SetNotifier(f.svc)
	if err := d.Pool.QueryRow(context.Background(), `SELECT email FROM users WHERE id = $1`, f.user.ID).Scan(&f.email); err != nil {
		t.Fatal(err)
	}
	return f
}

type storedRow struct {
	id     int64
	uuid   uuid.UUID
	status string
	title  string
	body   string
	sealed *string
}

func (f *secretsFixture) row(t *testing.T, template string) storedRow {
	t.Helper()
	var r storedRow
	err := f.d.Pool.QueryRow(context.Background(), `
		SELECT id, uuid, status, title, body, sealed_content FROM notifications
		WHERE user_id = $1 AND template_code = $2 ORDER BY id DESC LIMIT 1`, f.user.ID, template).
		Scan(&r.id, &r.uuid, &r.status, &r.title, &r.body, &r.sealed)
	if err != nil {
		t.Fatalf("notification row: %v", err)
	}
	return r
}

func (f *secretsFixture) inbox(t *testing.T, query string) string {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/v1/notifications"+query, nil)
	req = req.WithContext(authctx.WithPrincipal(req.Context(), authctx.Principal{
		UserID: f.user.UUID, UserInternal: f.user.ID,
	}))
	rec := httptest.NewRecorder()
	handler.New(f.svc).List(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /v1/notifications%s = %d %s", query, rec.Code, rec.Body.String())
	}
	return rec.Body.String()
}

func mailedCode(t *testing.T, m mail.Message) string {
	t.Helper()
	code := otpPattern.FindString(m.Body)
	if code == "" {
		t.Fatalf("mailed body has no code: %q", m.Body)
	}
	return code
}

func TestVerificationCodeNeverReadableFromRowOrInbox(t *testing.T) {
	f := newSecretsFixture(t, 0)
	ctx := context.Background()
	if err := f.auth.RequestEmailVerification(ctx, f.user.UUID, ""); err != nil {
		t.Fatalf("RequestEmailVerification: %v", err)
	}
	if len(f.mail.msgs) != 1 {
		t.Fatalf("mails = %d, want 1", len(f.mail.msgs))
	}
	code := mailedCode(t, f.mail.msgs[0])

	r := f.row(t, "auth.email_verification")
	if strings.Contains(r.title, code) || strings.Contains(r.body, code) {
		t.Fatalf("row leaks the code: %q / %q", r.title, r.body)
	}
	if !strings.Contains(r.body, usecase.SecretMask) {
		t.Fatalf("row body is not masked: %q", r.body)
	}
	if r.sealed != nil {
		t.Fatalf("sealed_content kept after a successful send")
	}

	for _, q := range []string{"", "?channel=email", "?channel=email&q=" + code} {
		body := f.inbox(t, q)
		if strings.Contains(body, code) || strings.Contains(body, r.uuid.String()) {
			t.Fatalf("GET /v1/notifications%s returns the verification row: %s", q, body)
		}
	}
	if _, err := f.svc.GetOwned(ctx, f.user.ID, r.uuid); !errors.Is(err, usecase.ErrNotFound) {
		t.Fatalf("GetOwned(email row) err = %v, want ErrNotFound", err)
	}

	// The mailed code is the one the account must present.
	if err := f.auth.VerifyEmail(ctx, f.email, code); err != nil {
		t.Fatalf("VerifyEmail with the mailed code: %v", err)
	}
}

func TestPasswordResetRetrySendsTheRealCode(t *testing.T) {
	f := newSecretsFixture(t, 1)
	ctx := context.Background()
	// The inline smtp failure is logged, not returned: the reset request must answer like an
	// unknown email (TEC-1116). The failed row below is what gets retried.
	if err := f.auth.ForgotPassword(ctx, f.email); err != nil {
		t.Fatalf("ForgotPassword must look accepted despite the smtp failure, got %v", err)
	}

	r := f.row(t, "auth.password_reset")
	if r.status != model.StatusFailed || r.sealed == nil {
		t.Fatalf("after failure: status %q, sealed %v", r.status, r.sealed != nil)
	}
	if otpPattern.MatchString(r.title) || otpPattern.MatchString(r.body) || otpPattern.MatchString(*r.sealed) {
		t.Fatalf("failed row holds a readable code: %q / %q / %q", r.title, r.body, *r.sealed)
	}

	if err := f.svc.Deliver(ctx, r.id); err != nil {
		t.Fatalf("retry Deliver: %v", err)
	}
	if len(f.mail.msgs) != 1 {
		t.Fatalf("mails = %d, want 1", len(f.mail.msgs))
	}
	code := mailedCode(t, f.mail.msgs[0])
	if strings.Contains(f.mail.msgs[0].Body, usecase.SecretMask) {
		t.Fatalf("retry mailed the masked text: %q", f.mail.msgs[0].Body)
	}
	if after := f.row(t, "auth.password_reset"); after.sealed != nil || strings.Contains(after.body, code) {
		t.Fatalf("after retry: sealed %v, body %q", after.sealed != nil, after.body)
	}
	if err := f.auth.ResetPassword(ctx, f.email, code, "N3w-Passw0rd!x"); err != nil {
		t.Fatalf("ResetPassword with the mailed code: %v", err)
	}
}
