package usecase_test

import (
	"context"
	"errors"
	"testing"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/database/db"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/companystructure/model"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/companystructure/repository"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/companystructure/usecase"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/authctx"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/rbac"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/testutil/pgtest"
	"github.com/google/uuid"
)

func TestMain(m *testing.M) { pgtest.Main(m) }
func principal(a db.Account, u pgtest.User, role string) authctx.Principal {
	return authctx.Principal{UserInternal: u.ID, UserID: u.UUID, Account: authctx.Account{ID: a.ID, UUID: a.Uuid, Type: "company", Role: role}, Permissions: []string{rbac.PermAppCompanyStructureRead, rbac.PermAppCompanyStructureWrite}}
}
func createCompany(t *testing.T, d *pgtest.DB, u pgtest.User) db.Account {
	t.Helper()
	ctx := context.Background()
	a, err := d.Queries.CreateCompanyAccount(ctx, db.CreateCompanyAccountParams{Name: "Hierarchy", Slug: "structure-" + pgtest.Unique()})
	if err != nil {
		t.Fatal(err)
	}
	if err = d.Queries.AddAccountMember(ctx, db.AddAccountMemberParams{AccountID: a.ID, UserID: u.ID, Role: db.AccountRoleOwner}); err != nil {
		t.Fatal(err)
	}
	return a
}
func TestIntegrationCompanyStructureRolesAndIsolation(t *testing.T) {
	d := pgtest.Open(t)
	ctx := context.Background()
	u, mu, member := d.CreateUser(t), d.CreateUser(t), d.CreateUser(t)
	a, b := createCompany(t, d, u), createCompany(t, d, u)
	for _, in := range []db.AddAccountMemberParams{{AccountID: a.ID, UserID: mu.ID, Role: db.AccountRoleOfficeManager}, {AccountID: a.ID, UserID: member.ID, Role: db.AccountRoleMember}} {
		if err := d.Queries.AddAccountMember(ctx, in); err != nil {
			t.Fatal(err)
		}
	}
	r := repository.New(d.Pool, d.Queries)
	s := usecase.New(r)
	owner := principal(a, u, "owner")
	mgr := principal(a, mu, "office_manager")
	plain := principal(a, member, "member")
	other := principal(b, u, "owner")
	office := func(p authctx.Principal, name string) model.Office {
		t.Helper()
		v, err := s.SaveOffice(ctx, p, uuid.Nil, model.OfficeInput{Name: name})
		if err != nil {
			t.Fatal(err)
		}
		return v
	}
	x, y, z := office(owner, "X"), office(owner, "Y"), office(owner, "Z")
	foreign := office(other, "Foreign")
	group := func(p authctx.Principal, off *uuid.UUID, name string) model.Group {
		t.Helper()
		v, err := s.SaveGroup(ctx, p, uuid.Nil, model.GroupInput{Name: name, Kind: "team", OfficeUUID: off})
		if err != nil {
			t.Fatal(err)
		}
		return v
	}
	gx, gy, gz, global := group(owner, &x.UUID, "X"), group(owner, &y.UUID, "Y"), group(owner, &z.UUID, "Z"), group(owner, nil, "Global")
	wantErr := func(name string, err, want error) {
		t.Helper()
		if !errors.Is(err, want) {
			t.Fatalf("%s: got %v, want %v", name, err, want)
		}
	}
	_, err := s.SaveGroup(ctx, mgr, uuid.Nil, model.GroupInput{Name: "Denied", Kind: "team", OfficeUUID: &x.UUID})
	wantErr("manager without grants", err, model.ErrForbidden)
	if err = s.SetScopes(ctx, owner, mu.UUID, []uuid.UUID{x.UUID, y.UUID}); err != nil {
		t.Fatal(err)
	}
	group(mgr, &x.UUID, "Scoped")
	_, err = s.SaveGroup(ctx, mgr, gz.UUID, model.GroupInput{Name: "Denied", Kind: "team", OfficeUUID: &x.UUID})
	wantErr("steal foreign scope group", err, model.ErrForbidden)
	_, err = s.SaveGroup(ctx, mgr, gx.UUID, model.GroupInput{Name: "Denied", Kind: "team", OfficeUUID: &z.UUID})
	wantErr("move group out of scope", err, model.ErrForbidden)
	_, err = s.SaveGroup(ctx, mgr, uuid.Nil, model.GroupInput{Name: "Denied", Kind: "team"})
	wantErr("manager global group", err, model.ErrForbidden)
	_, err = s.SaveOffice(ctx, mgr, uuid.Nil, model.OfficeInput{Name: "Denied"})
	wantErr("manager create office", err, model.ErrForbidden)
	wantErr("manager grants self", s.SetScopes(ctx, mgr, mu.UUID, []uuid.UUID{z.UUID}), model.ErrForbidden)
	wantErr("manager sector", s.SetSector(ctx, mgr, nil), model.ErrForbidden)
	_, err = s.SetAssignment(ctx, plain, model.Assignment{UserUUID: member.UUID, OfficeUUID: &x.UUID})
	wantErr("member claims unassigned", err, model.ErrForbidden)
	_, err = s.SetAssignment(ctx, mgr, model.Assignment{UserUUID: member.UUID, OfficeUUID: &z.UUID})
	wantErr("claim unassigned into unmanaged office", err, model.ErrForbidden)
	_, err = s.SetAssignment(ctx, mgr, model.Assignment{UserUUID: u.UUID, OfficeUUID: &x.UUID})
	wantErr("claim unassigned owner", err, model.ErrForbidden)
	if _, err = s.SetAssignment(ctx, mgr, model.Assignment{UserUUID: member.UUID, OfficeUUID: &x.UUID}); err != nil {
		t.Fatalf("claim unassigned member: %v", err)
	}
	if _, err = s.SetAssignment(ctx, owner, model.Assignment{UserUUID: member.UUID, OfficeUUID: &x.UUID, GroupUUID: &gx.UUID}); err != nil {
		t.Fatal(err)
	}
	if _, err = s.SetAssignment(ctx, mgr, model.Assignment{UserUUID: member.UUID, OfficeUUID: &y.UUID, GroupUUID: &gy.UUID}); err != nil {
		t.Fatal(err)
	}
	_, err = s.SetAssignment(ctx, mgr, model.Assignment{UserUUID: member.UUID, OfficeUUID: &z.UUID, GroupUUID: &gz.UUID})
	wantErr("move assignment out of scope", err, model.ErrForbidden)
	_, err = s.SetAssignment(ctx, mgr, model.Assignment{UserUUID: member.UUID})
	wantErr("manager clear office", err, model.ErrForbidden)
	_, err = s.SetAssignment(ctx, mgr, model.Assignment{UserUUID: member.UUID, OfficeUUID: &y.UUID, GroupUUID: &global.UUID})
	wantErr("manager assign global group", err, model.ErrForbidden)
	_, err = s.SetAssignment(ctx, owner, model.Assignment{UserUUID: member.UUID, OfficeUUID: &x.UUID, GroupUUID: &gy.UUID})
	wantErr("mismatched group office", err, model.ErrInvalid)
	if _, err = s.SetAssignment(ctx, mgr, model.Assignment{UserUUID: member.UUID, OfficeUUID: &y.UUID}); err != nil {
		t.Fatal(err)
	}
	if _, err = s.SetAssignment(ctx, owner, model.Assignment{UserUUID: member.UUID, OfficeUUID: &z.UUID}); err != nil {
		t.Fatal(err)
	}
	_, err = s.SetAssignment(ctx, mgr, model.Assignment{UserUUID: member.UUID, OfficeUUID: &x.UUID})
	wantErr("steal assignment outside scope", err, model.ErrForbidden)
	for name, fn := range map[string]func() error{
		"read office":   func() error { _, e := s.Office(ctx, other, x.UUID); return e },
		"update office": func() error { _, e := s.SaveOffice(ctx, other, x.UUID, model.OfficeInput{Name: "Bad"}); return e },
		"delete office": func() error { return s.DeleteOffice(ctx, other, x.UUID) },
		"read group":    func() error { _, e := s.Group(ctx, other, gx.UUID); return e },
		"delete group":  func() error { return s.DeleteGroup(ctx, other, gx.UUID) },
		"foreign office group": func() error {
			_, e := s.SaveGroup(ctx, owner, uuid.Nil, model.GroupInput{Name: "Bad", Kind: "team", OfficeUUID: &foreign.UUID})
			return e
		},
		"foreign assignment": func() error {
			_, e := s.SetAssignment(ctx, owner, model.Assignment{UserUUID: member.UUID, OfficeUUID: &foreign.UUID})
			return e
		},
		"foreign member": func() error { _, e := s.Assignment(ctx, other, member.UUID); return e },
		"foreign scope":  func() error { return s.SetScopes(ctx, owner, mu.UUID, []uuid.UUID{foreign.UUID}) },
	} {
		wantErr(name, fn(), model.ErrNotFound)
	}
	// Bulk assignment list is strictly account-scoped; role visibility is covered by the API test.
	list, n, err := s.Assignments(ctx, owner, 100, 0)
	if err != nil || n != 3 || len(list) != 3 {
		t.Fatalf("assignments for owner: %v %d %v", list, n, err)
	}
	for _, v := range list {
		if v.UserUUID == member.UUID && (v.OfficeUUID == nil || *v.OfficeUUID != z.UUID) {
			t.Fatalf("member assignment: %+v", v)
		}
	}
	list, n, err = s.Assignments(ctx, other, 100, 0)
	if err != nil || n != 1 || len(list) != 1 || list[0].UserUUID != u.UUID || list[0].OfficeUUID != nil {
		t.Fatalf("cross-account assignments: %v %d %v", list, n, err)
	}
	noRead := plain
	noRead.Permissions = nil
	_, _, err = s.Assignments(ctx, noRead, 100, 0)
	wantErr("assignments without read permission", err, model.ErrForbidden)
	personal := owner
	personal.Account.Type = authctx.AccountTypePersonal
	_, _, err = s.Assignments(ctx, personal, 100, 0)
	wantErr("assignments on personal account", err, model.ErrForbidden)
	outsider := principal(a, d.CreateUser(t), "owner")
	_, _, err = s.Assignments(ctx, outsider, 100, 0)
	wantErr("assignments for non-member", err, model.ErrForbidden)
	gs, n, err := s.Groups(ctx, owner, model.GroupFilter{OfficeUUID: &x.UUID}, 100, 0)
	if err != nil || n != 2 || len(gs) != 2 {
		t.Fatalf("groups office filter: %v %d %v", gs, n, err)
	}
	gs, n, err = s.Groups(ctx, owner, model.GroupFilter{CompanyWide: true}, 100, 0)
	if err != nil || n != 1 || gs[0].UUID != global.UUID {
		t.Fatalf("groups company-wide filter: %v %d %v", gs, n, err)
	}
	gs, n, err = s.Groups(ctx, owner, model.GroupFilter{OfficeUUID: &foreign.UUID}, 100, 0)
	if err != nil || n != 0 || len(gs) != 0 {
		t.Fatalf("foreign office filter: %v %d %v", gs, n, err)
	}
	_, _, err = s.Groups(ctx, owner, model.GroupFilter{OfficeUUID: &x.UUID, CompanyWide: true}, 100, 0)
	wantErr("conflicting group filter", err, model.ErrInvalid)
	scopes, err := s.Scopes(ctx, owner, mu.UUID)
	if err != nil || len(scopes) != 2 {
		t.Fatalf("failed scope replace must keep grants: %v %v", scopes, err)
	}
	rows, total, err := s.Offices(ctx, owner, 100, 0)
	if err != nil || total != 3 || len(rows) != 3 {
		t.Fatalf("owner office list: %v %d %v", rows, total, err)
	}
	_, err = s.SaveGroup(ctx, plain, uuid.Nil, model.GroupInput{Name: "Bad", Kind: "team", OfficeUUID: &x.UUID})
	wantErr("member write", err, model.ErrForbidden)
	noPerm := owner
	noPerm.Permissions = []string{rbac.PermAppCompanyStructureRead}
	_, err = s.SaveOffice(ctx, noPerm, uuid.Nil, model.OfficeInput{Name: "Bad"})
	wantErr("missing global permission", err, model.ErrForbidden)
	if err = s.SetScopes(ctx, owner, mu.UUID, []uuid.UUID{}); err != nil {
		t.Fatal(err)
	}
	wantErr("revoked grant", s.DeleteGroup(ctx, mgr, gx.UUID), model.ErrForbidden)
	if err = s.SetScopes(ctx, owner, mu.UUID, []uuid.UUID{x.UUID}); err != nil {
		t.Fatal(err)
	}
	if _, err = d.Queries.UpdateAccountMemberRole(ctx, db.UpdateAccountMemberRoleParams{
		AccountID: a.ID,
		UserID:    mu.ID,
		Role:      db.AccountRoleMember,
	}); err != nil {
		t.Fatal(err)
	}
	wantErr("stale principal role", s.DeleteGroup(ctx, mgr, gx.UUID), model.ErrForbidden)
	scopes, err = s.Scopes(ctx, owner, mu.UUID)
	if err != nil || len(scopes) != 0 {
		t.Fatal("demotion did not clear grants")
	}
	admin := usecase.NewAdmin(r)
	_, _, err = admin.Offices(ctx, owner, b.Uuid, 20, 0)
	wantErr("app cannot read platform", err, model.ErrForbidden)
	platform := owner
	platform.Permissions = []string{rbac.PermPlatformAccountsRead}
	rows, total, err = admin.Offices(ctx, platform, b.Uuid, 20, 0)
	if err != nil || total != 1 || len(rows) != 1 || rows[0].UUID != foreign.UUID {
		t.Fatalf("admin read: %v %v", rows, err)
	}
}
func TestIntegrationSectorCatalog(t *testing.T) {
	d := pgtest.Open(t)
	ctx := context.Background()
	u := d.CreateUser(t)
	a := createCompany(t, d, u)
	p := principal(a, u, "owner")
	r := repository.New(d.Pool, d.Queries)
	s := usecase.New(r)
	c := usecase.NewCatalog(r)
	all, total, err := c.List(ctx, p, "", 100, 0)
	if err != nil || total < 12 || len(all) < 12 {
		t.Fatalf("seed catalog: %d %v", total, err)
	}
	if err = s.SetSector(ctx, p, &all[0].UUID); err != nil {
		t.Fatal(err)
	}
	got, err := s.Sector(ctx, p)
	if err != nil || got == nil || *got != all[0].UUID {
		t.Fatalf("sector: %v %v", got, err)
	}
	platform := p
	platform.Permissions = []string{rbac.PermPlatformSectorsRead, rbac.PermPlatformSectorsWrite}
	if err = c.DeleteAdmin(ctx, platform, all[0].UUID); !errors.Is(err, model.ErrConflict) {
		t.Fatalf("used sector delete: %v", err)
	}
	if err = s.SetSector(ctx, p, nil); err != nil {
		t.Fatal(err)
	}
	id := uuid.New()
	if err = s.SetSector(ctx, p, &id); !errors.Is(err, model.ErrNotFound) {
		t.Fatalf("unknown sector: %v", err)
	}
	input := model.SectorInput{Slug: "test-" + pgtest.Unique(), NameTR: "Test", NameEN: "Test"}
	sector, err := c.SaveAdmin(ctx, platform, uuid.Nil, input)
	if err != nil {
		t.Fatal(err)
	}
	input.NameEN = "Changed"
	sector, err = c.SaveAdmin(ctx, platform, sector.UUID, input)
	if err != nil || sector.NameEN != "Changed" {
		t.Fatalf("update sector: %v", err)
	}
	if _, err = c.SaveAdmin(ctx, p, uuid.Nil, input); !errors.Is(err, model.ErrForbidden) {
		t.Fatalf("catalog write permission: %v", err)
	}
	if err = c.DeleteAdmin(ctx, platform, sector.UUID); err != nil {
		t.Fatal(err)
	}
}
