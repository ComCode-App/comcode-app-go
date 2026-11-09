package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/auth/model"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/jwt"
	"github.com/google/uuid"
)

// TEC-1105: an OAuth-only user must not be able to remove their last way to sign in.

func newSignInMethodFixture(t *testing.T) (*oauthMemRepo, *AuthUseCase, *OAuthUseCase) {
	t.Helper()
	repo := &oauthMemRepo{memRepo: *newMemRepo()}
	tokens, err := jwt.NewManager("test-secret-key-32-bytes-minimum!", time.Minute, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	uc := New(repo, tokens)
	uc.SetAuthSettings(testAuthGate{passwordRegister: true, passwordLogin: true, passkeyLogin: true, registration: true})
	return repo, uc, NewOAuth(repo, testOAuthBox(t))
}

func TestOAuthOnlyUserCannotUnlinkLastIdentityUntilPasswordReset(t *testing.T) {
	repo, uc, oauth := newSignInMethodFixture(t)
	ctx := context.Background()

	adapterUser, err := uc.RegisterOAuthUser(ctx, "oauth@example.com", "O", "Auth", true)
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	user := repo.byEmail["oauth@example.com"]
	if user.PasswordSet {
		t.Fatal("OAuth sign-up must not count as a password")
	}
	if err := oauth.LinkOAuthAccount(ctx, model.LinkOAuthAccountInput{
		UserID: adapterUser.ID, Provider: "google", ProviderAccountID: "g-1",
	}); err != nil {
		t.Fatalf("link: %v", err)
	}

	list, err := oauth.ListIdentities(ctx, user.UUID)
	if err != nil || list.HasPassword || list.HasPasskey || list.Total != 1 {
		t.Fatalf("list: %+v %v", list, err)
	}
	if err := oauth.UnlinkOAuthAccountForUser(ctx, user.UUID, "google"); !errors.Is(err, ErrLastSignInMethod) {
		t.Fatalf("expected ErrLastSignInMethod, got %v", err)
	}
	if len(repo.accounts) != 1 {
		t.Fatalf("identity must stay linked, got %d", len(repo.accounts))
	}
	if err := oauth.UnlinkOAuthAccountForUser(ctx, user.UUID, "github"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("unlinked provider: expected ErrNotFound, got %v", err)
	}

	// Password reset (the set-password path for OAuth-only users) marks it.
	repo.otps = map[string]string{"oauth@example.com|" + otpTypePasswordReset: hashOTP("123456")}
	if err := uc.ResetPassword(ctx, "oauth@example.com", "123456", "NewPassword1!"); err != nil {
		t.Fatalf("reset: %v", err)
	}
	if !repo.byUUID[user.UUID].PasswordSet {
		t.Fatal("password reset must mark password_set")
	}
	if list, _ := oauth.ListIdentities(ctx, user.UUID); !list.HasPassword {
		t.Fatal("expected has_password after reset")
	}
	if err := oauth.UnlinkOAuthAccountForUser(ctx, user.UUID, "google"); err != nil {
		t.Fatalf("unlink with password: %v", err)
	}
}

func TestUnlinkAllowedWhileAnotherSignInMethodRemains(t *testing.T) {
	repo, _, oauth := newSignInMethodFixture(t)
	ctx := context.Background()
	user, _ := repo.CreateUser(ctx, model.User{Email: "two@example.com", Status: "active"}, true)
	_ = repo.MarkPasswordUnset(ctx, user.ID)
	repo.accounts = []model.OAuthAccountRecord{
		{UserID: user.ID, UserUUID: user.UUID, Provider: "apple", ProviderAccountID: "a"},
		{UserID: user.ID, UserUUID: user.UUID, Provider: "google", ProviderAccountID: "g"},
	}
	if err := oauth.UnlinkOAuthAccountForUser(ctx, user.UUID, "google"); err != nil {
		t.Fatalf("unlink with another identity left: %v", err)
	}
	if err := oauth.UnlinkOAuthAccountForUser(ctx, user.UUID, "apple"); !errors.Is(err, ErrLastSignInMethod) {
		t.Fatalf("expected ErrLastSignInMethod, got %v", err)
	}
	// A passkey is a sign-in method too.
	repo.passkeys = []model.PasskeyRecord{{UUID: uuid.New(), UserID: user.ID}}
	if list, _ := oauth.ListIdentities(ctx, user.UUID); !list.HasPasskey || list.HasPassword {
		t.Fatalf("list: %+v", list)
	}
	if err := oauth.UnlinkOAuthAccountForUser(ctx, user.UUID, "apple"); err != nil {
		t.Fatalf("unlink with passkey left: %v", err)
	}
}

func TestDeleteLastPasskeyWithoutPasswordRefused(t *testing.T) {
	repo, uc, _ := newSignInMethodFixture(t)
	ctx := context.Background()
	user, _ := repo.CreateUser(ctx, model.User{Email: "pk@example.com", Status: "active"}, true)
	_ = repo.MarkPasswordUnset(ctx, user.ID)
	first, second := uuid.New(), uuid.New()
	repo.passkeys = []model.PasskeyRecord{{UUID: first, UserID: user.ID}, {UUID: second, UserID: user.ID}}

	if err := uc.DeletePasskey(ctx, user.UUID, uuid.New()); !errors.Is(err, ErrNotFound) {
		t.Fatalf("unknown passkey: expected ErrNotFound, got %v", err)
	}
	if err := uc.DeletePasskey(ctx, user.UUID, first); err != nil {
		t.Fatalf("delete with another passkey left: %v", err)
	}
	if err := uc.DeletePasskey(ctx, user.UUID, second); !errors.Is(err, ErrLastSignInMethod) {
		t.Fatalf("expected ErrLastSignInMethod, got %v", err)
	}
	repo.accounts = []model.OAuthAccountRecord{{UserID: user.ID, UserUUID: user.UUID, Provider: "google", ProviderAccountID: "g"}}
	if err := uc.DeletePasskey(ctx, user.UUID, second); err != nil {
		t.Fatalf("delete with identity left: %v", err)
	}
}

func TestDeleteLastPasskeyWithPassword(t *testing.T) {
	repo, uc, _ := newSignInMethodFixture(t)
	ctx := context.Background()
	user, _ := repo.CreateUser(ctx, model.User{Email: "pw@example.com", Status: "active"}, true)
	pk := uuid.New()
	repo.passkeys = []model.PasskeyRecord{{UUID: pk, UserID: user.ID}}
	if err := uc.DeletePasskey(ctx, user.UUID, pk); err != nil {
		t.Fatalf("delete with password: %v", err)
	}
}
