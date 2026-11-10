package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/auth/model"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/authctx"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/jwt"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/rbac"
)

type sessionFixture struct {
	repo   *memRepo
	tokens *jwt.Manager
	uc     *AuthUseCase
	user   model.User
}

func newSessionFixture(t *testing.T) sessionFixture {
	t.Helper()
	repo := newMemRepo()
	tokens, err := jwt.NewManager("test-secret-key-32-bytes-minimum!", time.Minute, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	uc := New(repo, tokens)
	uc.SetAuthSettings(testAuthGate{passwordRegister: true, passwordLogin: true, passkeyLogin: true, registration: true})
	user, err := uc.Register(context.Background(), model.RegisterInput{
		Email: "member@example.com", Password: "Password1!", Name: "Mem", Surname: "Ber",
	})
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	return sessionFixture{repo: repo, tokens: tokens, uc: uc, user: user}
}

func (f sessionFixture) login(t *testing.T) model.Tokens {
	t.Helper()
	tok, err := f.uc.Login(context.Background(), "member@example.com", "Password1!", "", model.SessionMeta{})
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	return tok
}

func (f sessionFixture) principal(t *testing.T, access string) (authctx.Principal, error) {
	t.Helper()
	claims, err := f.tokens.ParseAccess(access)
	if err != nil {
		t.Fatalf("parse access: %v", err)
	}
	return f.uc.LoadPrincipal(context.Background(), claims)
}

func TestLoginStartsOnPersonalAccount(t *testing.T) {
	f := newSessionFixture(t)
	p, err := f.principal(t, f.login(t).AccessToken)
	if err != nil {
		t.Fatalf("load principal: %v", err)
	}
	personal, _ := f.repo.GetPersonalAccount(context.Background(), f.user.ID)
	if p.Account.ID != personal.ID || !p.Account.IsPersonal() || !p.Account.IsOwner() {
		t.Fatalf("expected personal owner account, got %+v", p.Account)
	}
}

func TestRefreshKeepsSessionFamilyAndRejectsOldToken(t *testing.T) {
	f := newSessionFixture(t)
	first := f.login(t)
	p1, err := f.principal(t, first.AccessToken)
	if err != nil {
		t.Fatal(err)
	}

	second, err := f.uc.Refresh(context.Background(), first.RefreshToken, model.SessionMeta{})
	if err != nil {
		t.Fatalf("refresh: %v", err)
	}
	p2, err := f.principal(t, second.AccessToken)
	if err != nil {
		t.Fatal(err)
	}
	if p1.SessionID != p2.SessionID {
		t.Fatalf("rotation changed session family: %s -> %s", p1.SessionID, p2.SessionID)
	}
	// Access tokens issued before rotation stay valid: the family is still live.
	if _, err := f.principal(t, first.AccessToken); err != nil {
		t.Fatalf("pre-rotation access token rejected: %v", err)
	}
	// Presented again right after rotation: a concurrent refresh, not a replay (TEC-1179).
	if _, err := f.uc.Refresh(context.Background(), first.RefreshToken, model.SessionMeta{}); !errors.Is(err, ErrRefreshRace) {
		t.Fatalf("reused refresh token within the race window: expected ErrRefreshRace, got %v", err)
	}
	if _, err := f.principal(t, second.AccessToken); err != nil {
		t.Fatalf("race must not revoke the family: %v", err)
	}
}

func TestRevokedSessionRejectsAccessToken(t *testing.T) {
	f := newSessionFixture(t)
	tok := f.login(t)
	p, err := f.principal(t, tok.AccessToken)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.uc.RevokeSession(context.Background(), f.user.ID, p.SessionID); err != nil {
		t.Fatalf("revoke: %v", err)
	}
	if _, err := f.principal(t, tok.AccessToken); !errors.Is(err, ErrSessionRevoked) {
		t.Fatalf("expected ErrSessionRevoked, got %v", err)
	}
}

func TestLogoutRevokesWholeFamily(t *testing.T) {
	f := newSessionFixture(t)
	first := f.login(t)
	second, err := f.uc.Refresh(context.Background(), first.RefreshToken, model.SessionMeta{})
	if err != nil {
		t.Fatal(err)
	}
	if err := f.uc.Logout(context.Background(), second.RefreshToken); err != nil {
		t.Fatal(err)
	}
	if _, err := f.principal(t, first.AccessToken); !errors.Is(err, ErrSessionRevoked) {
		t.Fatalf("pre-rotation access token after logout: expected ErrSessionRevoked, got %v", err)
	}
}

func TestRoleRevocationAppliesImmediately(t *testing.T) {
	f := newSessionFixture(t)
	f.repo.userRoles[f.user.ID] = []string{rbac.RoleSuperAdmin}
	tok := f.login(t)
	p, err := f.principal(t, tok.AccessToken)
	if err != nil {
		t.Fatal(err)
	}
	if !p.IsSuperAdmin {
		t.Fatal("expected super admin before revocation")
	}

	f.repo.userRoles[f.user.ID] = nil
	p, err = f.principal(t, tok.AccessToken)
	if err != nil {
		t.Fatal(err)
	}
	if p.IsSuperAdmin || p.HasPermission(rbac.PermPlatformUsersRead) {
		t.Fatalf("revoked role still effective: superAdmin=%v perms=%v", p.IsSuperAdmin, p.Permissions)
	}
}

func TestSwitchAccount(t *testing.T) {
	f := newSessionFixture(t)
	tok := f.login(t)
	p, err := f.principal(t, tok.AccessToken)
	if err != nil {
		t.Fatal(err)
	}

	foreign := f.repo.addCompanyAccount("Foreign", map[int64]string{9999: "owner"})
	if _, err := f.uc.SwitchAccount(context.Background(), p, foreign.UUID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("switch to non-member account: expected ErrNotFound, got %v", err)
	}

	acme := f.repo.addCompanyAccount("Acme", map[int64]string{f.user.ID: "member"})
	res, err := f.uc.SwitchAccount(context.Background(), p, acme.UUID)
	if err != nil {
		t.Fatalf("switch: %v", err)
	}
	switched, err := f.principal(t, res.AccessToken)
	if err != nil {
		t.Fatal(err)
	}
	if switched.Account.ID != acme.ID || switched.Account.Role != authctx.AccountRoleMember || switched.Account.CanWrite() {
		t.Fatalf("unexpected account after switch: %+v", switched.Account)
	}
	if switched.SessionID != p.SessionID {
		t.Fatal("switch must stay in the same session family")
	}

	// Refresh keeps the switched account.
	refreshed, err := f.uc.Refresh(context.Background(), res.RefreshToken, model.SessionMeta{})
	if err != nil {
		t.Fatal(err)
	}
	afterRefresh, err := f.principal(t, refreshed.AccessToken)
	if err != nil {
		t.Fatal(err)
	}
	if afterRefresh.Account.ID != acme.ID {
		t.Fatalf("refresh dropped active account: %+v", afterRefresh.Account)
	}

	// Losing the membership falls back to the personal account instead of failing.
	delete(f.repo.accounts[acme.ID].members, f.user.ID)
	fallback, err := f.principal(t, refreshed.AccessToken)
	if err != nil {
		t.Fatal(err)
	}
	if !fallback.Account.IsPersonal() {
		t.Fatalf("expected personal fallback, got %+v", fallback.Account)
	}
}

func TestSwitchToSuspendedAccountIsRejected(t *testing.T) {
	f := newSessionFixture(t)
	p, err := f.principal(t, f.login(t).AccessToken)
	if err != nil {
		t.Fatal(err)
	}
	acc := f.repo.addCompanyAccount("Dormant", map[int64]string{f.user.ID: "owner"})
	f.repo.accounts[acc.ID].membership.Status = "suspended"
	if _, err := f.uc.SwitchAccount(context.Background(), p, acc.UUID); !errors.Is(err, ErrAccountSuspended) {
		t.Fatalf("expected ErrAccountSuspended, got %v", err)
	}
}

func TestStopImpersonationRevokesImpersonationSession(t *testing.T) {
	f := newSessionFixture(t)
	f.repo.userRoles[f.user.ID] = []string{rbac.RoleSuperAdmin}
	target, err := f.uc.Register(context.Background(), model.RegisterInput{
		Email: "target@example.com", Password: "Password1!", Name: "Tar", Surname: "Get",
	})
	if err != nil {
		t.Fatal(err)
	}
	f.repo.users[target.ID] = func() model.User { u := f.repo.users[target.ID]; u.Status = "active"; return u }()
	f.repo.byUUID[target.UUID] = f.repo.users[target.ID]

	imp, err := f.uc.ImpersonatePlatformUser(context.Background(), f.user, true, false, target.UUID, model.SessionMeta{})
	if err != nil {
		t.Fatalf("impersonate: %v", err)
	}
	impPrincipal, err := f.principal(t, imp.AccessToken)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.uc.StopImpersonation(context.Background(), f.user.UUID, impPrincipal.UserInternal, impPrincipal.SessionID, model.SessionMeta{}); err != nil {
		t.Fatalf("stop: %v", err)
	}
	if _, err := f.principal(t, imp.AccessToken); !errors.Is(err, ErrSessionRevoked) {
		t.Fatalf("impersonation token after stop: expected ErrSessionRevoked, got %v", err)
	}
	if _, err := f.uc.Refresh(context.Background(), imp.RefreshToken, model.SessionMeta{}); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("impersonation refresh after stop: expected ErrInvalidCredentials, got %v", err)
	}

}
