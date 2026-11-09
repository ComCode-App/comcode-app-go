package repository_test

// Postgres integration tests (TEC-536). Skipped unless TEST_DATABASE_URL is set; see
// internal/testutil/pgtest.

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/routes/model"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/routes/repository"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/testutil/pgtest"
)

func TestMain(m *testing.M) { pgtest.Main(m) }

// raised reports whether err is a RAISE EXCEPTION from a trigger (SQLSTATE P0001).
func raised(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "P0001"
}

func TestIntegrationProtectImmutableRoutesTrigger(t *testing.T) {
	d := pgtest.Open(t)
	ctx := context.Background()
	repo := repository.NewPostgres(d.Queries)
	u := d.CreateUser(t)
	card := d.CreateCard(t, u)
	route, err := repo.CreateRoute(ctx, model.CreateReservedRouteInput{
		Path: "c/" + card.SmartCode, RouteType: model.RouteTypeCardRoute, Status: model.RouteStatusReserved,
		IsImmutable: true, TargetType: model.TargetTypeProfile, CardID: &card.ID, AccountID: &u.AccountID,
		LegacyMetadata: map[string]any{"source": "test"},
	})
	if err != nil {
		t.Fatal(err)
	}

	for name, stmt := range map[string]string{
		"path":            `UPDATE routes SET path = path || '-x' WHERE id = $1`,
		"route_type":      `UPDATE routes SET route_type = 'custom_smart_link' WHERE id = $1`,
		"legacy_metadata": `UPDATE routes SET legacy_metadata = '{"source":"other"}' WHERE id = $1`,
	} {
		if _, err := d.Pool.Exec(ctx, stmt, route.ID); !raised(err) {
			t.Errorf("changing immutable %s: err = %v, want trigger exception", name, err)
		}
	}
	// Lifecycle and targeting stay editable on immutable routes.
	if _, err := d.Pool.Exec(ctx, `UPDATE routes SET status = 'retired', direct_url = 'https://x.test' WHERE id = $1`, route.ID); err != nil {
		t.Fatalf("status change on immutable route: %v", err)
	}
	// Seeded system paths are immutable too.
	if _, err := d.Pool.Exec(ctx, `UPDATE routes SET path = 'platform2' WHERE path = 'platform' AND route_type = 'system_reserved'`); !raised(err) {
		t.Fatalf("renaming a system route: %v, want trigger exception", err)
	}
	// Mutable routes can be renamed.
	link, err := repo.CreateRoute(ctx, model.CreateReservedRouteInput{
		Path: "s/m-" + pgtest.Unique(), RouteType: model.RouteTypeCustomSmartLink, Status: model.RouteStatusActive,
		TargetType: model.TargetTypeDirectURL, DirectURL: "https://x.test", AccountID: &u.AccountID,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := d.Pool.Exec(ctx, `UPDATE routes SET path = path || '-x' WHERE id = $1`, link.ID); err != nil {
		t.Fatalf("renaming a mutable route: %v", err)
	}
}

func TestIntegrationRoutesRepositoryScopesAndLookups(t *testing.T) {
	d := pgtest.Open(t)
	ctx := context.Background()
	repo := repository.NewPostgres(d.Queries)
	a, b := d.CreateUser(t), d.CreateUser(t)
	path := "s/Mixed-" + pgtest.Unique()

	link, err := repo.CreateRoute(ctx, model.CreateReservedRouteInput{
		Path: path, RouteType: model.RouteTypeCustomSmartLink, Status: model.RouteStatusActive,
		TargetType: model.TargetTypeDirectURL, DirectURL: "https://a.test", AccountID: &a.AccountID, OwnerUserID: &a.ID,
	})
	if err != nil {
		t.Fatal(err)
	}

	// Path uniqueness is case-insensitive and global across accounts.
	if exists, err := repo.CheckPathExists(ctx, " "+path+" ", nil); err != nil || !exists {
		t.Fatalf("CheckPathExists = %v, %v", exists, err)
	}
	if exists, _ := repo.CheckPathExists(ctx, path, &link.UUID); exists {
		t.Fatal("CheckPathExists must ignore the excluded route")
	}
	if _, err := repo.CreateRoute(ctx, model.CreateReservedRouteInput{
		Path: path, RouteType: model.RouteTypeCustomSmartLink, Status: model.RouteStatusActive,
		TargetType: model.TargetTypeDirectURL, AccountID: &b.AccountID,
	}); err == nil {
		t.Fatal("duplicate path in another account must violate the unique index")
	}

	got, err := repo.GetRouteByPath(ctx, path)
	if err != nil || got == nil || got.UUID != link.UUID || got.AccountID == nil || *got.AccountID != a.AccountID {
		t.Fatalf("GetRouteByPath = %+v, %v", got, err)
	}
	if got, err := repo.GetRouteByPath(ctx, "s/nope-"+pgtest.Unique()); err != nil || got != nil {
		t.Fatalf("missing path = %+v, %v", got, err)
	}

	list, total, err := repo.ListRoutes(ctx, a.AccountID, nil, nil, nil, 20, 0)
	if err != nil || total != 1 || len(list) != 1 {
		t.Fatalf("account A routes = %d (total %d), %v", len(list), total, err)
	}
	if list, total, _ := repo.ListRoutes(ctx, b.AccountID, nil, nil, nil, 20, 0); total != 0 || len(list) != 0 {
		t.Fatalf("account B sees %d routes of A", total)
	}

	// Delete is account scoped: B's delete is a no-op. A's soft-deletes and retires the link, whose
	// path stays held for 30 days so nobody takes over a link that is still printed or shared.
	if err := repo.DeleteRoute(ctx, link.UUID, b.AccountID); err != nil {
		t.Fatal(err)
	}
	if got, _ := repo.GetRouteByUUID(ctx, link.UUID); got == nil {
		t.Fatal("another account deleted the route")
	}
	if err := repo.DeleteRoute(ctx, link.UUID, a.AccountID); err != nil {
		t.Fatal(err)
	}
	if got, _ := repo.GetRouteByPath(ctx, path); got != nil {
		t.Fatal("deleted route still resolves")
	}
	if exists, _ := repo.CheckPathExists(ctx, path, nil); !exists {
		t.Fatal("a just-deleted smart link must keep holding its path")
	}
	if _, err := d.Pool.Exec(ctx, `UPDATE routes SET deleted_at = NOW() - INTERVAL '31 days' WHERE uuid = $1`, link.UUID); err != nil {
		t.Fatal(err)
	}
	if exists, _ := repo.CheckPathExists(ctx, path, nil); exists {
		t.Fatal("a smart link deleted over 30 days ago must free its path")
	}

	card := d.CreateCard(t, a)
	cardRoute, err := repo.CreateRoute(ctx, model.CreateReservedRouteInput{
		Path: "c/" + card.SmartCode, RouteType: model.RouteTypeCardRoute, Status: model.RouteStatusReserved,
		IsImmutable: true, TargetType: model.TargetTypeProfile, CardID: &card.ID, AccountID: &a.AccountID,
	})
	if err != nil {
		t.Fatal(err)
	}
	if got, err := repo.GetRouteByCardID(ctx, card.ID); err != nil || got == nil || got.UUID != cardRoute.UUID {
		t.Fatalf("GetRouteByCardID = %+v, %v", got, err)
	}
}
