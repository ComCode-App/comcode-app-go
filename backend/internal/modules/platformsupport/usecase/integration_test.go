package usecase_test

// Postgres integration tests (TEC-950). Skipped unless TEST_DATABASE_URL is set; see
// internal/testutil/pgtest. The route cache runs on miniredis.

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/database/db"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/platformsupport/model"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/platformsupport/repository"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/platformsupport/usecase"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/routecache"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/testutil/pgtest"
	"github.com/jackc/pgx/v5/pgtype"
)

func TestMain(m *testing.M) { pgtest.Main(m) }

type recorded struct {
	action   string
	resource string
	id       *uuid.UUID
	payload  map[string]any
}

type fakeRecorder struct{ events []recorded }

func (f *fakeRecorder) Record(_ context.Context, _ *int64, action, resource string, id *uuid.UUID, payload map[string]any, _ *http.Request) {
	f.events = append(f.events, recorded{action, resource, id, payload})
}

type env struct {
	d     *pgtest.DB
	svc   *usecase.Service
	cache *routecache.Cache
	rec   *fakeRecorder
	actor usecase.Actor
}

func setup(t *testing.T) env {
	t.Helper()
	d := pgtest.Open(t)
	mr := miniredis.RunT(t)
	cache := routecache.New(redis.NewClient(&redis.Options{Addr: mr.Addr()}), "test", time.Hour)
	rec := &fakeRecorder{}
	staff := d.CreateUser(t)
	return env{
		d: d, cache: cache, rec: rec, actor: usecase.Actor{UserID: staff.ID},
		svc: usecase.New(repository.NewPostgres(d.Pool, d.Queries), cache, rec),
	}
}

// activatedCard seeds an activated card with an active c/{code} route pointing at a profile.
func (e env) activatedCard(t *testing.T, u pgtest.User) (pgtest.Card, int64, string) {
	t.Helper()
	ctx := context.Background()
	card := e.d.CreateCard(t, u)
	profileID := e.profile(t, u)
	if _, err := e.d.Pool.Exec(ctx, `UPDATE nfc_cards SET profile_id = $1, card_uid = $2 WHERE id = $3`,
		profileID, "SER-"+pgtest.Unique(), card.ID); err != nil {
		t.Fatal(err)
	}
	path := "c/" + card.SmartCode
	if _, err := e.d.Pool.Exec(ctx, `
		INSERT INTO routes (path, route_type, status, is_immutable, target_type, card_id, profile_id, account_id, owner_user_id)
		VALUES ($1, 'card_route', 'active', true, 'profile', $2, $3, $4, $5)`,
		path, card.ID, profileID, u.AccountID, u.ID); err != nil {
		t.Fatal(err)
	}
	return card, profileID, path
}

func (e env) profile(t *testing.T, u pgtest.User) int64 {
	t.Helper()
	var id int64
	if err := e.d.Pool.QueryRow(context.Background(),
		`INSERT INTO profiles (user_id, account_id) VALUES ($1, $2) RETURNING id`, u.ID, u.AccountID).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}

func (e env) cachePath(t *testing.T, path string, cardUUID uuid.UUID) {
	t.Helper()
	e.cache.Set(context.Background(), path, []byte(`{}`), &cardUUID, nil)
	if _, ok := e.cache.Get(context.Background(), path); !ok {
		t.Fatal("cache seed failed")
	}
}

func TestIntegrationSearchCards(t *testing.T) {
	e := setup(t)
	ctx := context.Background()
	u := e.d.CreateUser(t)
	card, _, path := e.activatedCard(t, u)

	for _, q := range []string{card.SmartCode, card.SmartCode[:5]} {
		items, err := e.svc.SearchCards(ctx, q)
		if err != nil {
			t.Fatal(err)
		}
		if len(items) == 0 || items[0].UUID != card.UUID {
			t.Fatalf("search %q: got %+v", q, items)
		}
		got := items[0]
		if got.Route == nil || got.Route.Path != path || got.ActivationStatus != model.ActivationActive ||
			got.Profile == nil || got.Account.UUID == uuid.Nil || got.Owner.UUID != u.UUID {
			t.Fatalf("search %q: unexpected card %+v", q, got)
		}
	}
	// LIKE wildcards are literal.
	if items, err := e.svc.SearchCards(ctx, "%%"); err != nil || len(items) != 0 {
		t.Fatalf("wildcard search: %v %d", err, len(items))
	}
	if _, err := e.svc.SearchCards(ctx, "x"); !errors.Is(err, usecase.ErrQueryTooShort) {
		t.Fatalf("short query: %v", err)
	}
}

func TestIntegrationRetireCard(t *testing.T) {
	e := setup(t)
	ctx := context.Background()
	u := e.d.CreateUser(t)
	card, _, path := e.activatedCard(t, u)
	e.cachePath(t, path, card.UUID)

	got, err := e.svc.RetireCard(ctx, e.actor, card.UUID)
	if err != nil {
		t.Fatal(err)
	}
	if got.ActivationStatus != model.ActivationRetired || got.Route.Status != "retired" {
		t.Fatalf("after retire: %+v", got)
	}
	if _, ok := e.cache.Get(ctx, path); ok {
		t.Fatal("retire left the route cache entry")
	}
	if len(e.rec.events) != 1 || e.rec.events[0].action != "card.retired" || *e.rec.events[0].id != card.UUID {
		t.Fatalf("activity: %+v", e.rec.events)
	}
	if _, err := e.svc.RetireCard(ctx, e.actor, card.UUID); !errors.Is(err, usecase.ErrCardRetired) {
		t.Fatalf("second retire: %v", err)
	}
	// A retired card cannot be reassigned.
	if _, err := e.svc.ReassignCard(ctx, e.actor, card.UUID, uuid.New()); !errors.Is(err, usecase.ErrCardRetired) {
		t.Fatalf("reassign retired: %v", err)
	}
}

func TestIntegrationRetirePendingCardClearsActivationCode(t *testing.T) {
	e := setup(t)
	ctx := context.Background()
	u := e.d.CreateUser(t)
	card := e.d.CreateCard(t, u)
	if _, err := e.d.Pool.Exec(ctx, `UPDATE nfc_cards SET activation_code = $1 WHERE id = $2`, "ACT-"+pgtest.Unique(), card.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := e.d.Pool.Exec(ctx, `
		INSERT INTO routes (path, route_type, status, is_immutable, target_type, card_id, account_id)
		VALUES ($1, 'card_route', 'reserved', true, 'profile', $2, $3)`, "c/"+card.SmartCode, card.ID, u.AccountID); err != nil {
		t.Fatal(err)
	}
	if c, _ := e.svc.GetCard(ctx, card.UUID); c.ActivationStatus != model.ActivationPending {
		t.Fatalf("status = %s, want pending", c.ActivationStatus)
	}
	if _, err := e.svc.ReassignCard(ctx, e.actor, card.UUID, uuid.New()); !errors.Is(err, usecase.ErrCardNotActivated) {
		t.Fatalf("reassign pending: %v", err)
	}
	if _, err := e.svc.RetireCard(ctx, e.actor, card.UUID); err != nil {
		t.Fatal(err)
	}
	var code pgtype.Text
	if err := e.d.Pool.QueryRow(ctx, `SELECT activation_code FROM nfc_cards WHERE id = $1`, card.ID).Scan(&code); err != nil {
		t.Fatal(err)
	}
	if code.Valid {
		t.Fatal("retire kept the activation code")
	}
}

func TestIntegrationReassignCard(t *testing.T) {
	e := setup(t)
	ctx := context.Background()
	from := e.d.CreateUser(t)
	to := e.d.CreateUser(t)
	card, _, path := e.activatedCard(t, from)
	targetID := e.profile(t, to)
	var targetUUID uuid.UUID
	if err := e.d.Pool.QueryRow(ctx, `SELECT uuid FROM profiles WHERE id = $1`, targetID).Scan(&targetUUID); err != nil {
		t.Fatal(err)
	}
	e.cachePath(t, path, card.UUID)

	got, err := e.svc.ReassignCard(ctx, e.actor, card.UUID, targetUUID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Profile == nil || got.Profile.UUID != targetUUID || got.Owner.UUID != to.UUID {
		t.Fatalf("after reassign: %+v", got)
	}
	// Card and route moved to the profile's account and owner; the path did not change.
	var cardAcc, routeAcc, routeOwner, routeProfile int64
	var routePath string
	if err := e.d.Pool.QueryRow(ctx, `
		SELECT c.account_id, r.account_id, r.owner_user_id, r.profile_id, r.path
		FROM nfc_cards c JOIN routes r ON r.card_id = c.id WHERE c.id = $1`, card.ID).
		Scan(&cardAcc, &routeAcc, &routeOwner, &routeProfile, &routePath); err != nil {
		t.Fatal(err)
	}
	if cardAcc != to.AccountID || routeAcc != to.AccountID || routeOwner != to.ID || routeProfile != targetID || routePath != path {
		t.Fatalf("rows: card acc %d route acc %d owner %d profile %d path %s", cardAcc, routeAcc, routeOwner, routeProfile, routePath)
	}
	if _, ok := e.cache.Get(ctx, path); ok {
		t.Fatal("reassign left the route cache entry")
	}
	if len(e.rec.events) != 1 || e.rec.events[0].action != "card.reassigned" {
		t.Fatalf("activity: %+v", e.rec.events)
	}
	if _, err := e.svc.ReassignCard(ctx, e.actor, card.UUID, uuid.New()); !errors.Is(err, usecase.ErrProfileNotFound) {
		t.Fatalf("unknown profile: %v", err)
	}
}

func TestIntegrationReleaseQuarantinedPath(t *testing.T) {
	e := setup(t)
	ctx := context.Background()
	u := e.d.CreateUser(t)
	q := e.d.Queries
	taken := func(path string) bool {
		t.Helper()
		ok, err := q.CheckPathExists(ctx, db.CheckPathExistsParams{Path: path})
		if err != nil {
			t.Fatal(err)
		}
		return ok
	}

	// A deleted smart link holds its path for 30 days.
	deleted := "s/del-" + pgtest.Unique()
	var deletedUUID uuid.UUID
	if err := e.d.Pool.QueryRow(ctx, `
		INSERT INTO routes (path, route_type, status, target_type, account_id, owner_user_id, deleted_at)
		VALUES ($1, 'custom_smart_link', 'retired', 'profile', $2, $3, NOW() - INTERVAL '1 day') RETURNING uuid`,
		deleted, u.AccountID, u.ID).Scan(&deletedUUID); err != nil {
		t.Fatal(err)
	}
	// A renamed smart link holds its old path the same way.
	oldPath, newPath := "s/old-"+pgtest.Unique(), "s/new-"+pgtest.Unique()
	var renamedID int64
	var renamedUUID uuid.UUID
	if err := e.d.Pool.QueryRow(ctx, `
		INSERT INTO routes (path, route_type, status, target_type, account_id, owner_user_id)
		VALUES ($1, 'custom_smart_link', 'active', 'profile', $2, $3) RETURNING id, uuid`,
		newPath, u.AccountID, u.ID).Scan(&renamedID, &renamedUUID); err != nil {
		t.Fatal(err)
	}
	if _, err := e.d.Pool.Exec(ctx, `
		INSERT INTO route_history (route_id, route_type, account_id, old_path, new_path, reason)
		VALUES ($1, 'custom_smart_link', $2, $3, $4, 'smart_link_rename')`, renamedID, u.AccountID, oldPath, newPath); err != nil {
		t.Fatal(err)
	}
	if !taken(deleted) || !taken(oldPath) {
		t.Fatal("paths should start quarantined")
	}

	// Search shows the quarantine and the history.
	items, err := e.svc.SearchRoutes(ctx, deleted)
	if err != nil || len(items) == 0 || items[0].UUID != deletedUUID || items[0].Status != model.RouteStatusQuarantine || items[0].QuarantineUntil == nil {
		t.Fatalf("search deleted: %v %+v", err, items)
	}
	items, err = e.svc.SearchRoutes(ctx, "/"+oldPath+"/")
	if err != nil || len(items) == 0 || items[0].UUID != renamedUUID || len(items[0].History) != 1 || !items[0].History[0].Quarantined {
		t.Fatalf("search old path: %v %+v", err, items)
	}

	e.cache.Set(ctx, deleted, []byte(`{}`), nil, nil)
	res, err := e.svc.ReleasePath(ctx, e.actor, "/"+deleted)
	if err != nil {
		t.Fatal(err)
	}
	if res.Path != deleted || len(res.RouteUUIDs) != 1 || res.RouteUUIDs[0] != deletedUUID {
		t.Fatalf("release result: %+v", res)
	}
	if taken(deleted) {
		t.Fatal("released path is still taken")
	}
	if _, ok := e.cache.Get(ctx, deleted); ok {
		t.Fatal("release left the route cache entry")
	}
	if rt, _ := e.svc.GetRoute(ctx, deletedUUID); rt.Status != "retired" || rt.QuarantineReleasedAt == nil {
		t.Fatalf("released route: %+v", rt)
	}

	if _, err := e.svc.ReleasePath(ctx, e.actor, oldPath); err != nil {
		t.Fatal(err)
	}
	if taken(oldPath) {
		t.Fatal("released old path is still taken")
	}
	if len(e.rec.events) != 2 || e.rec.events[0].action != "route.quarantine_released" {
		t.Fatalf("activity: %+v", e.rec.events)
	}

	// Nothing left to release; live routes are never released.
	if _, err := e.svc.ReleasePath(ctx, e.actor, deleted); !errors.Is(err, usecase.ErrPathNotQuarantined) {
		t.Fatalf("second release: %v", err)
	}
	if _, err := e.svc.ReleasePath(ctx, e.actor, newPath); !errors.Is(err, usecase.ErrPathNotQuarantined) {
		t.Fatalf("release live path: %v", err)
	}
	if !taken(newPath) {
		t.Fatal("live path must stay taken")
	}
}
