package usecase_test

import (
	"context"
	"errors"
	"testing"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/database/db"
	pmodel "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/profiles/model"
	profilesrepo "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/profiles/repository"
	profilesusecase "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/profiles/usecase"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/profileshares/usecase"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/authctx"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/testutil/pgtest"
	"github.com/google/uuid"
)

func TestMain(m *testing.M) { pgtest.Main(m) }

type allowQuotas struct{}

func (allowQuotas) CheckQuota(context.Context, int64, bool, string) error { return nil }

type env struct {
	t        *testing.T
	d        *pgtest.DB
	ctx      context.Context
	profiles *profilesusecase.Service
	svc      *usecase.Service
}

func newEnv(t *testing.T) *env {
	d := pgtest.Open(t)
	profiles := profilesusecase.NewService(profilesrepo.NewPostgres(d.Pool, d.Queries), nil)
	profiles.SetQuotas(allowQuotas{})
	return &env{t: t, d: d, ctx: context.Background(), profiles: profiles, svc: usecase.NewService(d.Queries, nil)}
}

func (e *env) company(owner pgtest.User) db.Account {
	e.t.Helper()
	a, err := e.d.Queries.CreateCompanyAccount(e.ctx, db.CreateCompanyAccountParams{Name: "Co " + pgtest.Unique(), Slug: "share-" + pgtest.Unique()})
	if err != nil {
		e.t.Fatal(err)
	}
	e.addMember(a, owner, db.AccountRoleOwner)
	return a
}

func (e *env) addMember(a db.Account, u pgtest.User, role db.AccountRole) {
	e.t.Helper()
	if err := e.d.Queries.AddAccountMember(e.ctx, db.AddAccountMemberParams{AccountID: a.ID, UserID: u.ID, Role: role}); err != nil {
		e.t.Fatal(err)
	}
}

func personal(u pgtest.User) authctx.Principal {
	return authctx.Principal{
		UserID: u.UUID, UserInternal: u.ID,
		Account: authctx.Account{ID: u.AccountID, Type: authctx.AccountTypePersonal, Role: authctx.AccountRoleOwner},
	}
}

func inCompany(a db.Account, u pgtest.User, role string) authctx.Principal {
	return authctx.Principal{
		UserID: u.UUID, UserInternal: u.ID,
		Account: authctx.Account{ID: a.ID, UUID: a.Uuid, Type: authctx.AccountTypeCompany, Role: role},
	}
}

func (e *env) profile(u pgtest.User, first string) uuid.UUID {
	e.t.Helper()
	p, err := e.profiles.Create(e.ctx, personal(u), pmodel.UpsertProfileInput{
		PrimaryLocale: "tr",
		Translations:  []pmodel.UpsertTranslationInput{{Locale: "tr", FirstName: first, LastName: "Test", Title: "Engineer", IsDefault: true}},
	})
	if err != nil {
		e.t.Fatal(err)
	}
	return p.UUID
}

func (e *env) shared(a db.Account, owner pgtest.User) []usecase.SharedProfile {
	e.t.Helper()
	items, total, err := e.svc.CompanyProfiles(e.ctx, inCompany(a, owner, "owner"), 50, 0)
	if err != nil {
		e.t.Fatal(err)
	}
	if int(total) != len(items) {
		e.t.Fatalf("total %d, items %d", total, len(items))
	}
	return items
}

func TestShareLifecycle(t *testing.T) {
	e := newEnv(t)
	owner := e.d.CreateUser(t)
	co := e.company(owner)
	other := e.company(e.d.CreateUser(t))

	member := e.d.CreateUser(t)
	e.addMember(co, member, db.AccountRoleMember)
	e.addMember(other, member, db.AccountRoleMember)
	silent := e.d.CreateUser(t)
	e.addMember(co, silent, db.AccountRoleMember)

	pShared := e.profile(member, "Ada")
	_ = e.profile(member, "Kept")
	_ = e.profile(silent, "Silent")

	// Default off: nothing is visible and every target reads unshared.
	if got := e.shared(co, owner); len(got) != 0 {
		t.Fatalf("expected no shared profiles by default, got %d", len(got))
	}
	targets, err := e.svc.Targets(e.ctx, personal(member), pShared)
	if err != nil {
		t.Fatal(err)
	}
	if len(targets) != 2 || targets[0].Shared || targets[1].Shared {
		t.Fatalf("targets = %+v", targets)
	}

	if err := e.svc.SetShared(e.ctx, personal(member), pShared, co.Uuid, true, nil); err != nil {
		t.Fatal(err)
	}
	// Idempotent.
	if err := e.svc.SetShared(e.ctx, personal(member), pShared, co.Uuid, true, nil); err != nil {
		t.Fatal(err)
	}
	got := e.shared(co, owner)
	if len(got) != 1 || got[0].UUID != pShared || got[0].FirstName != "Ada" || got[0].Member.UserUUID != member.UUID {
		t.Fatalf("shared = %+v", got)
	}
	// The other company of the member sees nothing: sharing is per company.
	if got := e.shared(other, e.d.CreateUser(t)); len(got) != 0 {
		t.Fatalf("other company sees %d profiles", len(got))
	}

	// Company members other than owners and admins cannot read the section.
	if _, _, err := e.svc.CompanyProfiles(e.ctx, inCompany(co, silent, "member"), 50, 0); !errors.Is(err, usecase.ErrForbidden) {
		t.Fatalf("member read err = %v", err)
	}
	// Admins can.
	admin := e.d.CreateUser(t)
	e.addMember(co, admin, db.AccountRoleAdmin)
	if items, _, err := e.svc.CompanyProfiles(e.ctx, inCompany(co, admin, "admin"), 50, 0); err != nil || len(items) != 1 {
		t.Fatalf("admin read = %d, %v", len(items), err)
	}

	// Unshare hides it again.
	if err := e.svc.SetShared(e.ctx, personal(member), pShared, co.Uuid, false, nil); err != nil {
		t.Fatal(err)
	}
	if got := e.shared(co, owner); len(got) != 0 {
		t.Fatalf("unshared profile still visible: %d", len(got))
	}

	// Leaving the company removes the share.
	if err := e.svc.SetShared(e.ctx, personal(member), pShared, co.Uuid, true, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := e.d.Queries.RemoveAccountMember(e.ctx, db.RemoveAccountMemberParams{AccountID: co.ID, UserID: member.ID}); err != nil {
		t.Fatal(err)
	}
	if got := e.shared(co, owner); len(got) != 0 {
		t.Fatalf("profile of a removed member still visible: %d", len(got))
	}
	var n int
	if err := e.d.Pool.QueryRow(e.ctx, `SELECT count(*) FROM profile_company_shares WHERE company_account_id = $1`, co.ID).Scan(&n); err != nil || n != 0 {
		t.Fatalf("share rows after removal = %d, %v", n, err)
	}
	// Rejoining does not bring the share back.
	e.addMember(co, member, db.AccountRoleMember)
	if got := e.shared(co, owner); len(got) != 0 {
		t.Fatalf("share came back after rejoin: %d", len(got))
	}
}

func TestShareGuards(t *testing.T) {
	e := newEnv(t)
	owner := e.d.CreateUser(t)
	co := e.company(owner)
	member := e.d.CreateUser(t)
	e.addMember(co, member, db.AccountRoleMember)
	stranger := e.d.CreateUser(t)
	pMember := e.profile(member, "Ada")
	pStranger := e.profile(stranger, "Eve")

	// Not a member of the company.
	if err := e.svc.SetShared(e.ctx, personal(stranger), pStranger, co.Uuid, true, nil); !errors.Is(err, usecase.ErrNotFound) {
		t.Fatalf("stranger share err = %v", err)
	}
	// Someone else's profile.
	if err := e.svc.SetShared(e.ctx, personal(stranger), pMember, co.Uuid, true, nil); !errors.Is(err, usecase.ErrNotFound) {
		t.Fatalf("foreign profile share err = %v", err)
	}
	// Only from the personal account.
	if err := e.svc.SetShared(e.ctx, inCompany(co, member, "member"), pMember, co.Uuid, true, nil); !errors.Is(err, usecase.ErrForbidden) {
		t.Fatalf("company-account share err = %v", err)
	}
	if items, err := e.svc.Targets(e.ctx, inCompany(co, owner, "owner"), pMember); err != nil || len(items) != 0 {
		t.Fatalf("company-account targets = %v, %v", items, err)
	}
	// A deleted profile disappears from the company list.
	if err := e.svc.SetShared(e.ctx, personal(member), pMember, co.Uuid, true, nil); err != nil {
		t.Fatal(err)
	}
	if err := e.profiles.Delete(e.ctx, personal(member), pMember); err != nil {
		t.Fatal(err)
	}
	if got := e.shared(co, owner); len(got) != 0 {
		t.Fatalf("deleted profile still visible: %d", len(got))
	}
}
