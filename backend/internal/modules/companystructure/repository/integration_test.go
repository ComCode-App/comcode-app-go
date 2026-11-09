package repository_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/database/db"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/companystructure/model"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/companystructure/repository"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/authctx"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/testutil/pgtest"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

func TestMain(m *testing.M) { pgtest.Main(m) }
func company(t *testing.T, d *pgtest.DB, u pgtest.User) db.Account {
	t.Helper()
	ctx := context.Background()
	a, err := d.Queries.CreateCompanyAccount(ctx, db.CreateCompanyAccountParams{Name: "Hierarchy", Slug: "hierarchy-" + pgtest.Unique()})
	if err != nil {
		t.Fatal(err)
	}
	if err = d.Queries.AddAccountMember(ctx, db.AddAccountMemberParams{AccountID: a.ID, UserID: u.ID, Role: db.AccountRoleOwner}); err != nil {
		t.Fatal(err)
	}
	return a
}
func TestIntegrationStructureIsolationAndConstraints(t *testing.T) {
	d := pgtest.Open(t)
	ctx := context.Background()
	u := d.CreateUser(t)
	a, b := company(t, d, u), company(t, d, u)
	oa, err := d.Queries.CreateStructureOffice(ctx, db.CreateStructureOfficeParams{AccountID: a.ID, Name: "A"})
	if err != nil {
		t.Fatal(err)
	}
	ob, err := d.Queries.CreateStructureOffice(ctx, db.CreateStructureOfficeParams{AccountID: b.ID, Name: "B"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = d.Queries.GetStructureOffice(ctx, db.GetStructureOfficeParams{AccountID: b.ID, Uuid: oa.Uuid}); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("cross-account lookup: %v", err)
	}
	if n, err := d.Queries.DeleteStructureOffice(ctx, db.DeleteStructureOfficeParams{AccountID: b.ID, Uuid: oa.Uuid}); err != nil || n != 0 {
		t.Fatalf("cross-account delete: %d %v", n, err)
	}
	if _, err = d.Pool.Exec(ctx, `INSERT INTO company_groups(account_id,office_id,name,kind) VALUES($1,$2,'bad','team')`, a.ID, ob.ID); err == nil {
		t.Fatal("cross-account group accepted")
	}
	g, err := d.Queries.CreateStructureGroup(ctx, db.CreateStructureGroupParams{AccountID: a.ID, Name: "Team", Kind: "team", OfficeUuid: pgtype.UUID{Bytes: oa.Uuid, Valid: true}})
	if err != nil {
		t.Fatal(err)
	}
	var member int64
	if err = d.Pool.QueryRow(ctx, `SELECT id FROM account_members WHERE account_id=$1 AND user_id=$2`, a.ID, u.ID).Scan(&member); err != nil {
		t.Fatal(err)
	}
	for name, stmt := range map[string]string{
		"foreign office": `INSERT INTO company_member_assignments(account_id,member_id,office_id) VALUES($1,$2,$3)`,
		"foreign scope":  `INSERT INTO account_member_scopes(account_id,member_id,office_id) VALUES($1,$2,$3)`,
	} {
		if _, err = d.Pool.Exec(ctx, stmt, a.ID, member, ob.ID); err == nil {
			t.Fatalf("%s accepted", name)
		}
	}
	if _, err = d.Pool.Exec(ctx, `INSERT INTO company_member_assignments(account_id,member_id,group_id) VALUES($1,$2,$3)`, a.ID, member, g.ID); err == nil {
		t.Fatal("office-bound group without matching office accepted")
	}
	_, err = d.Queries.SetStructureAssignment(ctx, db.SetStructureAssignmentParams{AccountID: a.ID, UserUuid: u.UUID, OfficeUuid: pgtype.UUID{Bytes: oa.Uuid, Valid: true}, GroupUuid: pgtype.UUID{Bytes: g.Uuid, Valid: true}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = d.Queries.DeleteStructureOffice(ctx, db.DeleteStructureOfficeParams{AccountID: a.ID, Uuid: oa.Uuid}); !errors.Is(repository.MapError(err), model.ErrConflict) {
		t.Fatalf("used office deletion: %v", err)
	}
	if _, err = d.Queries.UpdateStructureGroup(ctx, db.UpdateStructureGroupParams{AccountID: a.ID, Uuid: g.Uuid, Name: "Team", Kind: "team", OfficeUuid: pgtype.UUID{Bytes: ob.Uuid, Valid: true}}); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("foreign office update: %v", err)
	}
	// Moving a group to a different same-account office is rejected.
	oa2, err := d.Queries.CreateStructureOffice(ctx, db.CreateStructureOfficeParams{AccountID: a.ID, Name: "A2"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = d.Pool.Exec(ctx, `UPDATE company_groups SET office_id=$1 WHERE id=$2`, oa2.ID, g.ID); err == nil {
		t.Fatal("group move invalidated assignment")
	}
	if _, err = d.Queries.CreateStructureOffice(ctx, db.CreateStructureOfficeParams{AccountID: u.AccountID, Name: "Personal"}); err == nil {
		t.Fatal("personal structure accepted")
	}
	if _, err = d.Pool.Exec(ctx, `INSERT INTO account_member_scopes(account_id,member_id,office_id) VALUES($1,$2,$3)`, b.ID, member, ob.ID); err == nil {
		t.Fatal("cross-account member scope accepted")
	}
	r := repository.New(d.Pool, d.Queries)
	p := authctx.Principal{UserInternal: u.ID, Account: authctx.Account{ID: a.ID, Type: "company"}}
	sentinel := errors.New("abort")
	var rolledBack uuid.UUID
	err = r.InCompany(ctx, p, func(q repository.Store, _ string) error {
		v, err := q.CreateStructureOffice(ctx, db.CreateStructureOfficeParams{AccountID: a.ID, Name: "rollback"})
		if err != nil {
			return err
		}
		rolledBack = v.Uuid
		return sentinel
	})
	if !errors.Is(err, sentinel) {
		t.Fatal(err)
	}
	if _, err = d.Queries.GetStructureOffice(ctx, db.GetStructureOfficeParams{AccountID: a.ID, Uuid: rolledBack}); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatal("transaction did not roll back")
	}
	p.Account.ID = 0
	if err = r.InCompany(ctx, p, func(repository.Store, string) error { return nil }); !errors.Is(err, model.ErrForbidden) {
		t.Fatal("zero scope accepted")
	}
}

func TestIntegrationHierarchyRollback(t *testing.T) {
	d := pgtest.Open(t)
	ctx := context.Background()
	dir, err := pgtest.MigrationsDir()
	if err != nil {
		t.Fatal(err)
	}
	// Later migrations depend on the hierarchy, so roll back from the newest down to 000046.
	downs, err := filepath.Glob(filepath.Join(dir, "*.down.sql"))
	if err != nil {
		t.Fatal(err)
	}
	sort.Sort(sort.Reverse(sort.StringSlice(downs)))
	tx, err := d.Pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	for _, path := range downs {
		name := filepath.Base(path)
		if name < "000046_" {
			break
		}
		sql, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = tx.Exec(ctx, string(sql)); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
	}
	var table *string
	if err = tx.QueryRow(ctx, "SELECT to_regclass('company_offices')::text").Scan(&table); err != nil || table != nil {
		t.Fatalf("rollback kept offices: %v %v", table, err)
	}
}
func TestIntegrationListAssignmentsAndGroupOfficeFilter(t *testing.T) {
	d := pgtest.Open(t)
	ctx := context.Background()
	u, m := d.CreateUser(t), d.CreateUser(t)
	a, b := company(t, d, u), company(t, d, u)
	if err := d.Queries.AddAccountMember(ctx, db.AddAccountMemberParams{AccountID: a.ID, UserID: m.ID, Role: db.AccountRoleMember}); err != nil {
		t.Fatal(err)
	}
	oa, err := d.Queries.CreateStructureOffice(ctx, db.CreateStructureOfficeParams{AccountID: a.ID, Name: "A"})
	if err != nil {
		t.Fatal(err)
	}
	ob, err := d.Queries.CreateStructureOffice(ctx, db.CreateStructureOfficeParams{AccountID: b.ID, Name: "B"})
	if err != nil {
		t.Fatal(err)
	}
	office := pgtype.UUID{Bytes: oa.Uuid, Valid: true}
	ga, err := d.Queries.CreateStructureGroup(ctx, db.CreateStructureGroupParams{AccountID: a.ID, Name: "Office", Kind: "team", OfficeUuid: office})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = d.Queries.CreateStructureGroup(ctx, db.CreateStructureGroupParams{AccountID: a.ID, Name: "Wide", Kind: "team"}); err != nil {
		t.Fatal(err)
	}
	if _, err = d.Queries.CreateStructureGroup(ctx, db.CreateStructureGroupParams{AccountID: b.ID, Name: "Foreign", Kind: "team", OfficeUuid: pgtype.UUID{Bytes: ob.Uuid, Valid: true}}); err != nil {
		t.Fatal(err)
	}
	if _, err = d.Queries.SetStructureAssignment(ctx, db.SetStructureAssignmentParams{AccountID: a.ID, UserUuid: m.UUID, OfficeUuid: office, GroupUuid: pgtype.UUID{Bytes: ga.Uuid, Valid: true}}); err != nil {
		t.Fatal(err)
	}
	// Account A lists both members (owner unassigned, member placed); B only its owner.
	rows, err := d.Queries.ListStructureAssignments(ctx, db.ListStructureAssignmentsParams{AccountID: a.ID, LimitCount: 100})
	if err != nil || len(rows) != 2 {
		t.Fatalf("list A: %v %v", rows, err)
	}
	if rows[0].UserUuid != u.UUID || rows[0].OfficeUuid.Valid || rows[1].UserUuid != m.UUID || uuid.UUID(rows[1].OfficeUuid.Bytes) != oa.Uuid || uuid.UUID(rows[1].GroupUuid.Bytes) != ga.Uuid {
		t.Fatalf("assignment rows: %+v", rows)
	}
	if n, err := d.Queries.CountStructureAssignments(ctx, db.CountStructureAssignmentsParams{AccountID: a.ID}); err != nil || n != 2 {
		t.Fatalf("count A: %d %v", n, err)
	}
	rows, err = d.Queries.ListStructureAssignments(ctx, db.ListStructureAssignmentsParams{AccountID: b.ID, LimitCount: 100})
	if err != nil || len(rows) != 1 || rows[0].OfficeUuid.Valid {
		t.Fatalf("cross-account assignments leaked: %+v %v", rows, err)
	}
	rows, err = d.Queries.ListStructureAssignments(ctx, db.ListStructureAssignmentsParams{AccountID: a.ID, LimitCount: 1, OffsetCount: 1})
	if err != nil || len(rows) != 1 || rows[0].UserUuid != m.UUID {
		t.Fatalf("paging: %+v %v", rows, err)
	}
	groups := func(acc int64, off pgtype.UUID, wide bool) []string {
		t.Helper()
		gs, err := d.Queries.ListStructureGroups(ctx, db.ListStructureGroupsParams{AccountID: acc, FilterOfficeUuid: off, CompanyWide: wide, LimitCount: 100})
		if err != nil {
			t.Fatal(err)
		}
		n, err := d.Queries.CountStructureGroups(ctx, db.CountStructureGroupsParams{AccountID: acc, FilterOfficeUuid: off, CompanyWide: wide})
		if err != nil || n != int64(len(gs)) {
			t.Fatalf("count mismatch: %d %d %v", n, len(gs), err)
		}
		out := []string{}
		for _, g := range gs {
			out = append(out, g.Name)
		}
		return out
	}
	if got := groups(a.ID, pgtype.UUID{}, false); len(got) != 2 {
		t.Fatalf("unfiltered: %v", got)
	}
	if got := groups(a.ID, office, false); len(got) != 1 || got[0] != "Office" {
		t.Fatalf("office filter: %v", got)
	}
	if got := groups(a.ID, pgtype.UUID{}, true); len(got) != 1 || got[0] != "Wide" {
		t.Fatalf("company-wide filter: %v", got)
	}
	if got := groups(b.ID, office, false); len(got) != 0 {
		t.Fatalf("foreign office filter leaked: %v", got)
	}
	if got := groups(a.ID, pgtype.UUID{Bytes: ob.Uuid, Valid: true}, false); len(got) != 0 {
		t.Fatalf("filter by other account's office leaked: %v", got)
	}
}
