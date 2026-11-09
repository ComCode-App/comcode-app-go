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
	"github.com/google/uuid"
)

// Super admin privilege boundary (TEC-1098): platform staff holding platform.users.write but not
// super_admin can neither grant/remove the super_admin role nor change a super admin account.

type saGuardFixture struct {
	repo   *memRepo
	uc     *AuthUseCase
	sa     model.User // target super admin (two exist, so last-super-admin checks stay out of the way)
	other  model.User // second super admin
	plain  model.User // ordinary user
	staff  model.User // platform staff without super_admin
	staffC context.Context
	saC    context.Context
}

func newSAGuardFixture(t *testing.T) saGuardFixture {
	t.Helper()
	repo := newMemRepo()
	tokens, err := jwt.NewManager("test-secret-key-32-bytes-minimum!", time.Minute, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	mk := func(email string, roles ...string) model.User {
		u, err := repo.CreateUser(context.Background(), model.User{
			Email: email, PasswordHash: "x", Name: "N", Surname: "S", Status: "active",
		}, true)
		if err != nil {
			t.Fatal(err)
		}
		repo.userRoles[u.ID] = roles
		return u
	}
	f := saGuardFixture{repo: repo, uc: New(repo, tokens)}
	f.sa = mk("sa@example.com", rbac.RoleSuperAdmin)
	f.other = mk("sa2@example.com", rbac.RoleSuperAdmin)
	f.plain = mk("plain@example.com", rbac.RoleSubscriber)
	f.staff = mk("staff@example.com", rbac.RoleSubscriber)
	f.staffC = authctx.WithPrincipal(context.Background(), authctx.Principal{
		UserID: f.staff.UUID, UserInternal: f.staff.ID,
		Permissions: []string{rbac.PermPlatformUsersRead, rbac.PermPlatformUsersWrite},
	})
	f.saC = authctx.WithPrincipal(context.Background(), authctx.Principal{
		UserID: f.other.UUID, UserInternal: f.other.ID, IsSuperAdmin: true,
	})
	return f
}

func forbidden(t *testing.T, what string, err error) {
	t.Helper()
	if !errors.Is(err, ErrSuperAdminRequired) || !errors.Is(err, ErrForbidden) {
		t.Fatalf("%s: expected ErrSuperAdminRequired (403), got %v", what, err)
	}
}

func TestNonSuperAdminCannotChangeSuperAdmin(t *testing.T) {
	f := newSAGuardFixture(t)
	name, disabled := "Mallory", "disabled"
	onlySubscriber := []uuid.UUID{subscriberRoleUUID}

	_, err := f.uc.PatchPlatformUser(f.staffC, f.sa.UUID, model.PatchPlatformUserInput{Name: &name})
	forbidden(t, "patch super admin profile", err)
	_, err = f.uc.PatchPlatformUser(f.staffC, f.sa.UUID, model.PatchPlatformUserInput{Status: &disabled})
	forbidden(t, "disable super admin", err)
	_, err = f.uc.PatchPlatformUser(f.staffC, f.sa.UUID, model.PatchPlatformUserInput{RoleUUIDs: &onlySubscriber})
	forbidden(t, "remove super_admin role", err)
	forbidden(t, "set super admin password", f.uc.SetPlatformUserPassword(f.staffC, f.sa.UUID, "Another123!"))
	_, err = f.uc.ImpersonatePlatformUser(f.staffC, f.staff, false, false, f.sa.UUID, model.SessionMeta{})
	forbidden(t, "impersonate super admin", err)

	// Nothing changed.
	if got := f.repo.users[f.sa.ID]; got.Name != "N" || got.Status != "active" || got.PasswordHash != "x" {
		t.Fatalf("super admin was modified: %+v", got)
	}
	if ok, _ := f.repo.UserHasRoleSlug(context.Background(), f.sa.ID, rbac.RoleSuperAdmin); !ok {
		t.Fatal("super_admin role was removed")
	}
}

func TestNonSuperAdminCannotGrantSuperAdmin(t *testing.T) {
	f := newSAGuardFixture(t)
	grant := []uuid.UUID{subscriberRoleUUID, superAdminRoleUUID}

	_, err := f.uc.PatchPlatformUser(f.staffC, f.plain.UUID, model.PatchPlatformUserInput{RoleUUIDs: &grant})
	forbidden(t, "grant super_admin to another user", err)
	_, err = f.uc.PatchPlatformUser(f.staffC, f.staff.UUID, model.PatchPlatformUserInput{RoleUUIDs: &grant})
	forbidden(t, "grant super_admin to self", err)
	_, err = f.uc.CreatePlatformUser(f.staffC, model.CreatePlatformUserInput{
		Email: "new@example.com", Password: "Password1!", Name: "N", Surname: "S", RoleUUIDs: grant,
	})
	forbidden(t, "create user with super_admin", err)
	if _, err := f.repo.FindUserByEmail(context.Background(), "new@example.com"); err == nil {
		t.Fatal("user was created despite the forbidden role")
	}
	for _, u := range []model.User{f.plain, f.staff} {
		if ok, _ := f.repo.UserHasRoleSlug(context.Background(), u.ID, rbac.RoleSuperAdmin); ok {
			t.Fatalf("%s got super_admin", u.Email)
		}
	}

	// Ordinary user management stays open to staff.
	name, only := "Plain", []uuid.UUID{subscriberRoleUUID}
	if _, err := f.uc.PatchPlatformUser(f.staffC, f.plain.UUID, model.PatchPlatformUserInput{Name: &name, RoleUUIDs: &only}); err != nil {
		t.Fatalf("staff patch of ordinary user: %v", err)
	}
	if err := f.uc.SetPlatformUserPassword(f.staffC, f.plain.UUID, "Another123!"); err != nil {
		t.Fatalf("staff password reset of ordinary user: %v", err)
	}
}

func TestSuperAdminCanManageSuperAdmins(t *testing.T) {
	f := newSAGuardFixture(t)
	name, disabled := "Renamed", "disabled"
	grant := []uuid.UUID{superAdminRoleUUID}
	onlySubscriber := []uuid.UUID{subscriberRoleUUID}

	if _, err := f.uc.PatchPlatformUser(f.saC, f.sa.UUID, model.PatchPlatformUserInput{Name: &name}); err != nil {
		t.Fatalf("patch super admin: %v", err)
	}
	if err := f.uc.SetPlatformUserPassword(f.saC, f.sa.UUID, "Another123!"); err != nil {
		t.Fatalf("set super admin password: %v", err)
	}
	out, err := f.uc.PatchPlatformUser(f.saC, f.plain.UUID, model.PatchPlatformUserInput{RoleUUIDs: &grant})
	if err != nil || !out.IsSuperAdmin {
		t.Fatalf("grant super_admin: %+v %v", out, err)
	}
	if _, err := f.uc.PatchPlatformUser(f.saC, f.plain.UUID, model.PatchPlatformUserInput{RoleUUIDs: &onlySubscriber}); err != nil {
		t.Fatalf("remove super_admin: %v", err)
	}
	if _, err := f.uc.PatchPlatformUser(f.saC, f.sa.UUID, model.PatchPlatformUserInput{Status: &disabled}); err != nil {
		t.Fatalf("disable super admin: %v", err)
	}
	created, err := f.uc.CreatePlatformUser(f.saC, model.CreatePlatformUserInput{
		Email: "new-sa@example.com", Password: "Password1!", Name: "N", Surname: "S", RoleUUIDs: grant,
	})
	if err != nil || !created.IsSuperAdmin {
		t.Fatalf("create super admin: %+v %v", created, err)
	}
}
