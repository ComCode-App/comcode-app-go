package adapters_test

// Super admin boundary of the platform users bulk adapter (TEC-1098), against Postgres
// (skipped unless TEST_DATABASE_URL is set, see internal/testutil/pgtest).

import (
	"context"
	"strings"
	"testing"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/database/db"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/bulkengine"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/bulkengine/adapters"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/rbac"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/testutil/pgtest"
)

func TestMain(m *testing.M) { pgtest.Main(m) }

func grant(t *testing.T, d *pgtest.DB, userID int64, slug string) {
	t.Helper()
	if err := d.Queries.AssignUserRoleBySlug(context.Background(), db.AssignUserRoleBySlugParams{UserID: userID, Slug: slug}); err != nil {
		t.Fatal(err)
	}
}

func status(t *testing.T, d *pgtest.DB, u pgtest.User) string {
	t.Helper()
	row, err := d.Queries.GetUserByUUID(context.Background(), u.UUID)
	if err != nil {
		t.Fatal(err)
	}
	return row.Status
}

func TestIntegrationBulkUsersSuperAdminTargets(t *testing.T) {
	d := pgtest.Open(t)
	a := adapters.NewUsers(d.Queries)
	target := d.CreateUser(t)
	grant(t, d, target.ID, rbac.RoleSuperAdmin)
	spare := d.CreateUser(t) // keeps the last-super-admin guard out of the way
	grant(t, d, spare.ID, rbac.RoleSuperAdmin)
	staff := d.CreateUser(t)
	actingSA := d.CreateUser(t)
	grant(t, d, actingSA.ID, rbac.RoleSuperAdmin)
	plain := d.CreateUser(t)

	staffCtx := bulkengine.WithActor(context.Background(), staff.ID)
	saCtx := bulkengine.WithActor(context.Background(), actingSA.ID)

	for name, ctx := range map[string]context.Context{"staff": staffCtx, "no actor": context.Background()} {
		res, err := a.ApplyItem(ctx, "disable", target.UUID.String())
		if err != nil {
			t.Fatal(err)
		}
		if res.OK || !strings.Contains(res.Error, "super admin") {
			t.Fatalf("%s disable of super admin: %+v", name, res)
		}
	}
	if got := status(t, d, target); got != "active" {
		t.Fatalf("super admin status changed to %s", got)
	}

	// Staff still bulk-manages ordinary users.
	if res, err := a.ApplyItem(staffCtx, "disable", plain.UUID.String()); err != nil || !res.OK {
		t.Fatalf("staff disable of ordinary user: %+v %v", res, err)
	}

	// A super admin may disable and re-enable a super admin.
	res, err := a.ApplyItem(saCtx, "disable", target.UUID.String())
	if err != nil || !res.OK {
		t.Fatalf("super admin disable: %+v %v", res, err)
	}
	if got := status(t, d, target); got != "disabled" {
		t.Fatalf("status %s", got)
	}

	// Rolling that job back (re-enabling a super admin, restoring super_admin) needs a super admin.
	if err := a.RevertItem(staffCtx, "disable", target.UUID.String(), res.Previous); err == nil {
		t.Fatal("staff rollback of a super admin change succeeded")
	}
	if got := status(t, d, target); got != "disabled" {
		t.Fatalf("staff rollback changed status to %s", got)
	}
	// Restoring a snapshot that holds super_admin onto a user who lost it is refused too.
	demoted := d.CreateUser(t)
	prev := map[string]any{"status": "active", "role_slugs": []any{rbac.RoleSuperAdmin}}
	if err := a.RevertItem(staffCtx, "disable", demoted.UUID.String(), prev); err == nil {
		t.Fatal("staff rollback restored super_admin")
	}
	if ok, _ := d.Queries.UserHasRoleSlug(context.Background(), db.UserHasRoleSlugParams{UserID: demoted.ID, Slug: rbac.RoleSuperAdmin}); ok {
		t.Fatal("super_admin was restored by staff rollback")
	}
	if err := a.RevertItem(saCtx, "disable", target.UUID.String(), res.Previous); err != nil {
		t.Fatalf("super admin rollback: %v", err)
	}
	if got := status(t, d, target); got != "active" {
		t.Fatalf("status after rollback %s", got)
	}
}
