package adapters_test

// The platform users import never assigns super_admin (TEC-1098), against Postgres (skipped
// unless TEST_DATABASE_URL is set, see internal/testutil/pgtest).

import (
	"context"
	"testing"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/database/db"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/ioengine/adapters"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/rbac"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/testutil/pgtest"
	"github.com/jackc/pgx/v5"
)

func TestMain(m *testing.M) { pgtest.Main(m) }

func TestIntegrationUsersImportRejectsSuperAdmin(t *testing.T) {
	d := pgtest.Open(t)
	ctx := context.Background()
	a := adapters.NewUsers(d.Queries)

	cases := []struct {
		name     string
		row      map[string]any
		defaults map[string]any
	}{
		{"row role", map[string]any{"role_slugs": "subscriber, super_admin"}, nil},
		{"default role", map[string]any{}, map[string]any{"role_slugs": rbac.RoleSuperAdmin}},
	}
	for _, tc := range cases {
		email := "imp-" + pgtest.Unique() + "@example.test"
		tc.row["email"], tc.row["name"], tc.row["surname"] = email, "Imp", "Orter"
		res, err := a.ApplyRow(ctx, tc.row, tc.defaults)
		if err != nil {
			t.Fatal(err)
		}
		if res.OK || res.Error == "" {
			t.Fatalf("%s: row accepted: %+v", tc.name, res)
		}
		if _, err := d.Queries.GetUserByEmail(ctx, email); err != pgx.ErrNoRows {
			t.Fatalf("%s: user was created (err %v)", tc.name, err)
		}
	}

	// Other roles still import.
	email := "imp-" + pgtest.Unique() + "@example.test"
	res, err := a.ApplyRow(ctx, map[string]any{"email": email, "name": "I", "surname": "O", "role_slugs": rbac.RoleSubscriber}, nil)
	if err != nil || !res.OK {
		t.Fatalf("ordinary import: %+v %v", res, err)
	}
	u, err := d.Queries.GetUserByEmail(ctx, email)
	if err != nil {
		t.Fatal(err)
	}

	// An imported user promoted to super admin since is left alone by an import rollback.
	if err := d.Queries.AssignUserRoleBySlug(ctx, db.AssignUserRoleBySlugParams{UserID: u.ID, Slug: rbac.RoleSuperAdmin}); err != nil {
		t.Fatal(err)
	}
	if err := a.RevertRow(ctx, "user", u.Uuid.String(), nil); err == nil {
		t.Fatal("import rollback changed a super admin")
	}
	if ok, _ := d.Queries.UserHasRoleSlug(ctx, db.UserHasRoleSlugParams{UserID: u.ID, Slug: rbac.RoleSuperAdmin}); !ok {
		t.Fatal("super_admin was cleared by import rollback")
	}
}
