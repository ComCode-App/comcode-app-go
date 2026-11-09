package usecase

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/auth/model"
	notifmodel "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/notifications/model"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/jwt"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/password"
)

func enumerationUseCase(t *testing.T, repo Repository) *AuthUseCase {
	t.Helper()
	tokens, err := jwt.NewManager("test-secret-key-32-bytes-minimum!", time.Minute, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	return New(repo, tokens)
}

// TEC-1101: an unknown email and a wrong password must be indistinguishable, and the unknown
// email must still spend Argon2 work (dummy hash) so timing does not reveal registration.
func TestLoginUnknownEmailMatchesWrongPassword(t *testing.T) {
	repo := newMemRepo()
	hash, _ := password.Hash("Secret123")
	_, _ = repo.CreateUser(context.Background(), model.User{Email: "known@x.io", PasswordHash: hash, Status: "active"}, true)
	uc := enumerationUseCase(t, repo)
	dummyHashOnce, dummyHash = sync.Once{}, "" // observe this test's own dummy verification

	_, wrongErr := uc.Login(context.Background(), "known@x.io", "Wrong1234", "", model.SessionMeta{})
	_, unknownErr := uc.Login(context.Background(), "nobody@x.io", "Wrong1234", "", model.SessionMeta{})
	if wrongErr != ErrInvalidCredentials || unknownErr != ErrInvalidCredentials { //nolint:errorlint // identity is the point
		t.Fatalf("expected the identical ErrInvalidCredentials, got wrong=%v unknown=%v", wrongErr, unknownErr)
	}
	if dummyHash == "" || !strings.HasPrefix(dummyHash, "$argon2id$") {
		t.Fatal("unknown email must run a dummy Argon2 verification")
	}
}

// TEC-1101: a disabled account with a wrong password gets the generic error, not "disabled".
func TestLoginDisabledUserRequiresPassword(t *testing.T) {
	repo := newMemRepo()
	hash, _ := password.Hash("Secret123")
	for _, st := range []string{"disabled", "suspended"} {
		email := st + "@x.io"
		_, _ = repo.CreateUser(context.Background(), model.User{Email: email, PasswordHash: hash, Status: st}, true)
		uc := enumerationUseCase(t, repo)
		if _, err := uc.Login(context.Background(), email, "Wrong1234", "", model.SessionMeta{}); !errors.Is(err, ErrInvalidCredentials) {
			t.Fatalf("%s: wrong password must not reveal account state, got %v", st, err)
		}
		if _, err := uc.Login(context.Background(), email, "Secret123", "", model.SessionMeta{}); !errors.Is(err, ErrUserDisabled) {
			t.Fatalf("%s: expected ErrUserDisabled after the correct password, got %v", st, err)
		}
	}
}

type otpRecordingRepo struct {
	*memRepo
	otpEmails []string
}

func (r *otpRecordingRepo) CreateOTP(_ context.Context, _ *int64, email, _, _ string, _ time.Time) error {
	r.otpEmails = append(r.otpEmails, email)
	return nil
}

type recordingNotifier struct{ recipients []string }

func (n *recordingNotifier) Enqueue(_ context.Context, in notifmodel.EnqueueInput) ([]notifmodel.Notification, error) {
	if in.Recipient != nil {
		n.recipients = append(n.recipients, *in.Recipient)
	}
	return nil, nil
}

// TEC-1101: a signed-in user cannot have verification codes mailed to another address.
func TestEmailVerificationRequestOnlyTargetsCaller(t *testing.T) {
	repo := &otpRecordingRepo{memRepo: newMemRepo()}
	caller, _ := repo.CreateUser(context.Background(), model.User{Email: "me@x.io", Status: "active"}, false)
	_, _ = repo.CreateUser(context.Background(), model.User{Email: "victim@x.io", Status: "active"}, false)
	uc := enumerationUseCase(t, repo)
	notifier := &recordingNotifier{}
	uc.SetNotifier(notifier)

	for _, target := range []string{"victim@x.io", "stranger@x.io"} {
		if err := uc.RequestEmailVerification(context.Background(), caller.UUID, target); err != nil {
			t.Fatalf("request for %s must look accepted, got %v", target, err)
		}
	}
	if len(repo.otpEmails) != 0 || len(notifier.recipients) != 0 {
		t.Fatalf("no code may be issued for another address, got otps=%v mails=%v", repo.otpEmails, notifier.recipients)
	}

	if err := uc.RequestEmailVerification(context.Background(), caller.UUID, " ME@x.io "); err != nil {
		t.Fatal(err)
	}
	if err := uc.RequestEmailVerification(context.Background(), caller.UUID, ""); err != nil {
		t.Fatal(err)
	}
	if len(notifier.recipients) != 2 || notifier.recipients[0] != "me@x.io" || notifier.recipients[1] != "me@x.io" {
		t.Fatalf("expected two codes to the caller's own address, got %v", notifier.recipients)
	}
}

// TEC-1101: password reset request answers the same for known and unknown emails.
func TestForgotPasswordDoesNotRevealAccount(t *testing.T) {
	repo := &otpRecordingRepo{memRepo: newMemRepo()}
	_, _ = repo.CreateUser(context.Background(), model.User{Email: "known@x.io", Status: "active"}, true)
	uc := enumerationUseCase(t, repo)
	uc.SetNotifier(&recordingNotifier{})
	knownErr := uc.ForgotPassword(context.Background(), "known@x.io")
	unknownErr := uc.ForgotPassword(context.Background(), "nobody@x.io")
	if knownErr != nil || unknownErr != nil {
		t.Fatalf("expected accepted for both, got known=%v unknown=%v", knownErr, unknownErr)
	}
	if len(repo.otpEmails) != 1 || repo.otpEmails[0] != "known@x.io" {
		t.Fatalf("expected one code for the known account only, got %v", repo.otpEmails)
	}
}
