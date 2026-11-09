package usecase

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/auth/model"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/auth/repository"
	notifmodel "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/notifications/model"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/jwt"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/password"
	pqtotp "github.com/pquerna/otp/totp"
)

// ---- fakes (TEC-1121) --------------------------------------------------------

type codeOTP struct {
	id        int64
	userID    *int64
	email     string
	typ       string
	hash      string
	attempts  int32
	max       int32
	expiresAt time.Time
	consumed  bool
}

// codeRepo adds working OTP and TOTP bookkeeping to memRepo.
type codeRepo struct {
	*memRepo
	otps []*codeOTP
	totp map[int64]model.UserTOTP
}

func newCodeRepo() *codeRepo {
	return &codeRepo{memRepo: newMemRepo(), totp: map[int64]model.UserTOTP{}}
}

func (r *codeRepo) CreateOTP(_ context.Context, userID *int64, email, codeHash, otpType string, expiresAt time.Time) error {
	r.otps = append(r.otps, &codeOTP{
		id: int64(len(r.otps) + 1), userID: userID, email: email, typ: otpType, hash: codeHash, max: 5, expiresAt: expiresAt,
	})
	return nil
}

func (r *codeRepo) InvalidateOTPs(_ context.Context, email, otpType string) error {
	for _, o := range r.otps {
		if o.email == email && o.typ == otpType {
			o.consumed = true
		}
	}
	return nil
}

func (r *codeRepo) GetActiveOTP(_ context.Context, email, otpType string) (int64, string, int32, int32, error) {
	for i := len(r.otps) - 1; i >= 0; i-- {
		o := r.otps[i]
		if o.email == email && o.typ == otpType && !o.consumed && time.Now().Before(o.expiresAt) {
			return o.id, o.hash, o.attempts, o.max, nil
		}
	}
	return 0, "", 0, 0, repository.ErrNotFound
}

func (r *codeRepo) IncrementOTPAttempts(_ context.Context, id int64) (int32, int32, error) {
	o := r.otps[id-1]
	o.attempts++
	return o.attempts, o.max, nil
}

func (r *codeRepo) ConsumeOTP(_ context.Context, id int64) error {
	o := r.otps[id-1]
	if o.consumed {
		return repository.ErrNotFound
	}
	o.consumed = true
	return nil
}

func (r *codeRepo) SetEmailVerified(_ context.Context, userID int64) (model.User, error) {
	u := r.users[userID]
	u.EmailVerified = true
	r.users[u.ID], r.byEmail[u.Email], r.byUUID[u.UUID] = u, u, u
	return u, nil
}

func (r *codeRepo) GetUserTOTP(_ context.Context, userID int64) (model.UserTOTP, error) {
	row, ok := r.totp[userID]
	if !ok {
		return model.UserTOTP{}, repository.ErrNotFound
	}
	return row, nil
}

type codeNotifier struct{ sent []notifmodel.EnqueueInput }

func (n *codeNotifier) Enqueue(_ context.Context, in notifmodel.EnqueueInput) ([]notifmodel.Notification, error) {
	n.sent = append(n.sent, in)
	return nil, nil
}

func (n *codeNotifier) lastCode(t *testing.T) string {
	t.Helper()
	for i := len(n.sent) - 1; i >= 0; i-- {
		if n.sent[i].TemplateCode == loginCodeTemplate {
			return n.sent[i].TemplateVars["code"]
		}
	}
	t.Fatal("no login code email sent")
	return ""
}

func (n *codeNotifier) count(template string) int {
	c := 0
	for _, s := range n.sent {
		if s.TemplateCode == template {
			c++
		}
	}
	return c
}

type policyGate struct{ required bool }

func (g policyGate) PasswordLoginTOTPRequired(context.Context) (bool, error) { return g.required, nil }

type codeEnv struct {
	repo  *codeRepo
	uc    *AuthUseCase
	notif *codeNotifier
}

func newCodeEnv(t *testing.T, registration bool) *codeEnv {
	t.Helper()
	repo := newCodeRepo()
	tokens, err := jwt.NewManager("test-secret-key-32-bytes-minimum!", time.Minute, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	uc := New(repo, tokens)
	notif := &codeNotifier{}
	uc.SetNotifier(notif)
	uc.SetSecretBox(testOAuthBox(t), "Test")
	uc.SetAuthSettings(testAuthGate{passwordLogin: true, registration: registration})
	return &codeEnv{repo: repo, uc: uc, notif: notif}
}

func (e *codeEnv) user(t *testing.T, email, status string) model.User {
	t.Helper()
	hash, _ := password.Hash("Secret123!")
	u, err := e.repo.CreateUser(context.Background(), model.User{
		Email: email, PasswordHash: hash, Name: "Ada", Surname: "L", Status: status, Locale: "en",
	}, false)
	if err != nil {
		t.Fatal(err)
	}
	return u
}

func (e *codeEnv) enableTOTP(t *testing.T, userID int64) string {
	t.Helper()
	key, err := pqtotp.Generate(pqtotp.GenerateOpts{Issuer: "Test", AccountName: "x"})
	if err != nil {
		t.Fatal(err)
	}
	enc, _ := e.uc.box.Encrypt(key.Secret())
	e.repo.totp[userID] = model.UserTOTP{UserID: userID, SecretEnc: enc, Enabled: true}
	return key.Secret()
}

func otherCode(code string) string {
	if code == "000000" {
		return "111111"
	}
	return "000000"
}

// ---- tests -------------------------------------------------------------------

func TestEmailCodeRequestRejectsMalformedEmail(t *testing.T) {
	e := newCodeEnv(t, true)
	for _, email := range []string{"", "  ", "not-an-email"} {
		if err := e.uc.RequestLoginCode(context.Background(), email); !errors.Is(err, ErrInvalidRequest) {
			t.Fatalf("%q: got %v", email, err)
		}
	}
}

// Registration closed: an unknown email gets the same (nil) answer, no code, no account.
func TestEmailCodeUnknownEmailRegistrationClosed(t *testing.T) {
	e := newCodeEnv(t, false)
	e.user(t, "known@x.io", "active")
	ctx := context.Background()
	knownErr := e.uc.RequestLoginCode(ctx, "known@x.io")
	unknownErr := e.uc.RequestLoginCode(ctx, "nobody@x.io")
	if knownErr != nil || unknownErr != nil {
		t.Fatalf("both must be accepted: known=%v unknown=%v", knownErr, unknownErr)
	}
	if len(e.repo.otps) != 1 || e.notif.count(loginCodeTemplate) != 1 {
		t.Fatalf("only the known email gets a code: otps=%d mails=%d", len(e.repo.otps), len(e.notif.sent))
	}
	if _, err := e.uc.VerifyLoginCode(ctx, "nobody@x.io", "123456", "", model.SessionMeta{}); !errors.Is(err, ErrInvalidEmailCode) {
		t.Fatalf("unknown email verify: %v", err)
	}
	if _, ok := e.repo.byEmail["nobody@x.io"]; ok {
		t.Fatal("no account may be created while registration is closed")
	}
}

// Wrong code: known, unknown and disabled emails all get the identical error.
func TestEmailCodeWrongCodeDoesNotRevealAccount(t *testing.T) {
	e := newCodeEnv(t, false)
	e.user(t, "known@x.io", "active")
	e.user(t, "disabled@x.io", "disabled")
	ctx := context.Background()
	_ = e.uc.RequestLoginCode(ctx, "known@x.io")
	_ = e.uc.RequestLoginCode(ctx, "disabled@x.io")
	for _, email := range []string{"known@x.io", "disabled@x.io", "nobody@x.io"} {
		_, err := e.uc.VerifyLoginCode(ctx, email, "999999x", "", model.SessionMeta{})
		if err != ErrInvalidEmailCode { //nolint:errorlint // identity is the point
			t.Fatalf("%s: expected the identical ErrInvalidEmailCode, got %v", email, err)
		}
	}
	// A disabled account is reported only after a valid code.
	code := e.notif.lastCode(t)
	if _, err := e.uc.VerifyLoginCode(ctx, "disabled@x.io", code, "", model.SessionMeta{}); !errors.Is(err, ErrUserDisabled) {
		t.Fatalf("disabled with valid code: %v", err)
	}
}

func TestEmailCodeLoginSingleUseHashedSealed(t *testing.T) {
	e := newCodeEnv(t, false)
	e.user(t, "ada@x.io", "active")
	ctx := context.Background()
	if err := e.uc.RequestLoginCode(ctx, " ADA@x.io "); err != nil {
		t.Fatal(err)
	}
	code := e.notif.lastCode(t)
	if len(code) != 6 {
		t.Fatalf("code %q", code)
	}
	row := e.repo.otps[0]
	if row.hash == code || row.hash != hashOTP(code) || row.typ != otpTypeLoginCode || row.userID == nil {
		t.Fatalf("code must be stored hashed as login_code for the user: %+v", row)
	}
	if ttl := time.Until(row.expiresAt); ttl > 10*time.Minute || ttl < 9*time.Minute {
		t.Fatalf("ttl = %v", ttl)
	}
	sent := e.notif.sent[0]
	if !sent.SecurityEmail || sent.Language != "en" || sent.Recipient == nil || *sent.Recipient != "ada@x.io" {
		t.Fatalf("code must go out as a sealed security email: %+v", sent)
	}
	tok, err := e.uc.VerifyLoginCode(ctx, "ada@x.io", code, "", model.SessionMeta{})
	if err != nil || tok.AccessToken == "" || tok.RefreshToken == "" {
		t.Fatalf("verify: %v %+v", err, tok)
	}
	if !e.repo.byEmail["ada@x.io"].EmailVerified {
		t.Fatal("a valid code proves the mailbox")
	}
	if _, err := e.uc.VerifyLoginCode(ctx, "ada@x.io", code, "", model.SessionMeta{}); !errors.Is(err, ErrInvalidEmailCode) {
		t.Fatalf("reuse: %v", err)
	}
}

func TestEmailCodeExpiredAndReplaced(t *testing.T) {
	e := newCodeEnv(t, false)
	e.user(t, "ada@x.io", "active")
	ctx := context.Background()
	_ = e.uc.RequestLoginCode(ctx, "ada@x.io")
	first := e.notif.lastCode(t)
	_ = e.uc.RequestLoginCode(ctx, "ada@x.io")
	second := e.notif.lastCode(t)
	if first != second {
		if _, err := e.uc.VerifyLoginCode(ctx, "ada@x.io", first, "", model.SessionMeta{}); !errors.Is(err, ErrInvalidEmailCode) {
			t.Fatalf("a replaced code must be dead: %v", err)
		}
	}
	e.repo.otps[len(e.repo.otps)-1].expiresAt = time.Now().Add(-time.Second)
	if _, err := e.uc.VerifyLoginCode(ctx, "ada@x.io", second, "", model.SessionMeta{}); !errors.Is(err, ErrInvalidEmailCode) {
		t.Fatalf("expired code: %v", err)
	}
}

func TestEmailCodeAttemptLimit(t *testing.T) {
	e := newCodeEnv(t, false)
	e.user(t, "ada@x.io", "active")
	ctx := context.Background()
	_ = e.uc.RequestLoginCode(ctx, "ada@x.io")
	code := e.notif.lastCode(t)
	for i := 0; i < 5; i++ {
		if _, err := e.uc.VerifyLoginCode(ctx, "ada@x.io", otherCode(code), "", model.SessionMeta{}); !errors.Is(err, ErrInvalidEmailCode) {
			t.Fatalf("attempt %d: %v", i, err)
		}
	}
	if _, err := e.uc.VerifyLoginCode(ctx, "ada@x.io", code, "", model.SessionMeta{}); !errors.Is(err, ErrInvalidEmailCode) {
		t.Fatalf("code must be dead after 5 failures, got %v", err)
	}
}

func TestEmailCodeRequiresTOTPWhenEnabled(t *testing.T) {
	e := newCodeEnv(t, false)
	u := e.user(t, "ada@x.io", "active")
	secret := e.enableTOTP(t, u.ID)
	ctx := context.Background()
	_ = e.uc.RequestLoginCode(ctx, "ada@x.io")
	code := e.notif.lastCode(t)

	if _, err := e.uc.VerifyLoginCode(ctx, "ada@x.io", code, "", model.SessionMeta{}); !errors.Is(err, ErrMFARequired) {
		t.Fatalf("expected MFA required, got %v", err)
	}
	if _, err := e.uc.VerifyLoginCode(ctx, "ada@x.io", code, "000000x", model.SessionMeta{}); !errors.Is(err, ErrInvalidMFACode) {
		t.Fatalf("expected invalid mfa, got %v", err)
	}
	if e.repo.otps[0].attempts != 1 {
		t.Fatalf("a wrong authenticator code must burn an email code attempt, attempts=%d", e.repo.otps[0].attempts)
	}
	totpCode, _ := pqtotp.GenerateCode(secret, time.Now())
	tok, err := e.uc.VerifyLoginCode(ctx, "ada@x.io", code, totpCode, model.SessionMeta{})
	if err != nil || tok.AccessToken == "" {
		t.Fatalf("email code must survive the MFA round-trip: %v", err)
	}
}

// The admin "2FA required" policy cannot be bypassed with an email code.
func TestEmailCodeHonoursTOTPPolicy(t *testing.T) {
	e := newCodeEnv(t, true)
	e.user(t, "ada@x.io", "active")
	e.uc.SetAccessPolicy(policyGate{required: true})
	ctx := context.Background()
	_ = e.uc.RequestLoginCode(ctx, "ada@x.io")
	if _, err := e.uc.VerifyLoginCode(ctx, "ada@x.io", e.notif.lastCode(t), "", model.SessionMeta{}); !errors.Is(err, ErrMFANotEnrolled) {
		t.Fatalf("known user: %v", err)
	}
	_ = e.uc.RequestLoginCode(ctx, "new@x.io")
	if _, err := e.uc.VerifyLoginCode(ctx, "new@x.io", e.notif.lastCode(t), "", model.SessionMeta{}); !errors.Is(err, ErrMFANotEnrolled) {
		t.Fatalf("sign-up: %v", err)
	}
	if _, ok := e.repo.byEmail["new@x.io"]; ok {
		t.Fatal("no account may be created that the policy would lock out")
	}
}

func TestEmailCodeSignUpCreatesAccount(t *testing.T) {
	e := newCodeEnv(t, true)
	roleID := int64(2)
	e.uc.SetAuthSettings(testAuthGate{registration: true, defaultRoleID: &roleID})
	ctx := context.Background()
	if err := e.uc.RequestLoginCode(ctx, " New.Person@X.io "); err != nil {
		t.Fatal(err)
	}
	if len(e.repo.otps) != 1 || e.repo.otps[0].userID != nil {
		t.Fatalf("sign-up code must be issued without a user: %+v", e.repo.otps)
	}
	sent := e.notif.sent[len(e.notif.sent)-1]
	if sent.UserID != nil || sent.Recipient == nil || *sent.Recipient != "new.person@x.io" || !sent.SecurityEmail {
		t.Fatalf("sign-up mail: %+v", sent)
	}
	code := e.notif.lastCode(t)
	if _, err := e.uc.VerifyLoginCode(ctx, "new.person@x.io", otherCode(code), "", model.SessionMeta{}); !errors.Is(err, ErrInvalidEmailCode) {
		t.Fatalf("wrong code: %v", err)
	}
	if _, ok := e.repo.byEmail["new.person@x.io"]; ok {
		t.Fatal("a wrong code must not create an account")
	}
	tok, err := e.uc.VerifyLoginCode(ctx, "new.person@x.io", code, "", model.SessionMeta{})
	if err != nil || tok.AccessToken == "" || tok.RefreshToken == "" {
		t.Fatalf("verify: %v", err)
	}
	u, ok := e.repo.byEmail["new.person@x.io"]
	if !ok {
		t.Fatal("account not created")
	}
	if !u.EmailVerified || u.Name != "new.person" || u.Status != "active" || e.repo.users[u.ID].PasswordSet {
		t.Fatalf("user = %+v", u)
	}
	if roles := e.repo.userRoles[u.ID]; len(roles) != 1 {
		t.Fatalf("default role must be assigned like registration: %v", roles)
	}
	if e.notif.count("auth.welcome") != 1 || e.notif.count("auth.email_verification") != 0 {
		t.Fatalf("welcome=%d verification=%d", e.notif.count("auth.welcome"), e.notif.count("auth.email_verification"))
	}
	if _, err := e.uc.VerifyLoginCode(ctx, "new.person@x.io", code, "", model.SessionMeta{}); !errors.Is(err, ErrInvalidEmailCode) {
		t.Fatalf("code reuse: %v", err)
	}
}

// Registration closed between request and verify: no account.
func TestEmailCodeSignUpRegistrationTurnedOff(t *testing.T) {
	e := newCodeEnv(t, true)
	ctx := context.Background()
	_ = e.uc.RequestLoginCode(ctx, "late@x.io")
	code := e.notif.lastCode(t)
	e.uc.SetAuthSettings(testAuthGate{registration: false})
	if _, err := e.uc.VerifyLoginCode(ctx, "late@x.io", code, "", model.SessionMeta{}); !errors.Is(err, ErrInvalidEmailCode) {
		t.Fatalf("got %v", err)
	}
	if _, ok := e.repo.byEmail["late@x.io"]; ok {
		t.Fatal("no account may be created after registration was turned off")
	}
}

type failingCodeNotifier struct{}

func (failingCodeNotifier) Enqueue(context.Context, notifmodel.EnqueueInput) ([]notifmodel.Notification, error) {
	return nil, errors.New("enqueue failed")
}

// A failing email enqueue for a known address answers like an unknown one (TEC-1116 pattern).
func TestEmailCodeRequestFailureIsSwallowed(t *testing.T) {
	e := newCodeEnv(t, false)
	e.user(t, "ada@x.io", "active")
	e.uc.SetNotifier(failingCodeNotifier{})
	e.uc.SetLogger(quietLogger())
	if err := e.uc.RequestLoginCode(context.Background(), "ada@x.io"); err != nil {
		t.Fatalf("got %v", err)
	}
}

func quietLogger() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }
