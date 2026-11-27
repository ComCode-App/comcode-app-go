package usecase_test

// Postgres integration tests (TEC-536). Skipped unless TEST_DATABASE_URL is set; see
// internal/testutil/pgtest.

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/routes/model"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/routes/repository"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/routes/usecase"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/authctx"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/routecache"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/testutil/pgtest"
)

func TestMain(m *testing.M) { pgtest.Main(m) }

func newRouteService(t *testing.T) (*pgtest.DB, *repository.PostgresRepository, *usecase.Service) {
	t.Helper()
	d := pgtest.Open(t)
	repo := repository.NewPostgres(d.Queries)
	return d, repo, usecase.NewService(repo, "https://comcode.test/", nil)
}

func createSmartLink(t *testing.T, repo *repository.PostgresRepository, u pgtest.User, path, url string) *model.RouteDetail {
	t.Helper()
	r, err := repo.CreateRoute(context.Background(), model.CreateReservedRouteInput{
		Path: path, RouteType: model.RouteTypeCustomSmartLink, Status: model.RouteStatusActive,
		TargetType: model.TargetTypeDirectURL, DirectURL: url, AccountID: &u.AccountID, OwnerUserID: &u.ID,
	})
	if err != nil {
		t.Fatalf("create smart link: %v", err)
	}
	return r
}

func TestIntegrationResolveSmartLinkAndCardRoutes(t *testing.T) {
	d, repo, svc := newRouteService(t)
	ctx := context.Background()
	u := d.CreateUser(t)
	slug := "link-" + pgtest.Unique()
	createSmartLink(t, repo, u, "s/"+slug, "https://example.com/x")

	// Case, slashes and whitespace are normalized; a bare slug falls back to s/{slug}.
	for _, raw := range []string{"s/" + slug, " /S/" + slug + "/ ", slug} {
		res, err := svc.Resolve(ctx, raw)
		if err != nil {
			t.Fatalf("resolve %q: %v", raw, err)
		}
		if res.TargetURL != "https://example.com/x" || !res.IsActive || res.IsLocked {
			t.Fatalf("resolve %q = %+v", raw, res)
		}
	}
	if _, err := svc.Resolve(ctx, "s/missing-"+pgtest.Unique()); !errors.Is(err, usecase.ErrRouteNotFound) {
		t.Fatalf("missing path: %v, want ErrRouteNotFound", err)
	}

	// A reserved card route resolves (bare smart code falls back to c/{code}) but is not active
	// until the card is activated; a retired one never sends the visitor anywhere.
	card := d.CreateCard(t, u)
	route, err := svc.CreateReservedCardRoute(ctx, card.ID, card.SmartCode, &u.ID, &u.AccountID)
	if err != nil {
		t.Fatalf("reserve card route: %v", err)
	}
	if !route.IsImmutable || route.Status != model.RouteStatusReserved || route.Path != "c/"+card.SmartCode {
		t.Fatalf("card route = %+v", route)
	}
	if _, err := svc.CreateReservedCardRoute(ctx, card.ID, card.SmartCode, &u.ID, &u.AccountID); !errors.Is(err, usecase.ErrRoutePathConflict) {
		t.Fatalf("duplicate card route: %v, want ErrRoutePathConflict", err)
	}
	res, err := svc.Resolve(ctx, card.SmartCode)
	if err != nil {
		t.Fatalf("resolve card: %v", err)
	}
	if res.CardUUID == nil || *res.CardUUID != card.UUID || res.IsActive {
		t.Fatalf("reserved card resolution = %+v", res)
	}
	if _, err := d.Pool.Exec(ctx, `UPDATE routes SET status = 'active' WHERE id = $1`, route.ID); err != nil {
		t.Fatal(err)
	}
	if res, err = svc.Resolve(ctx, "c/"+card.SmartCode); err != nil || !res.IsActive {
		t.Fatalf("active card resolution = %+v, %v", res, err)
	}
	if _, err := d.Pool.Exec(ctx, `UPDATE nfc_cards SET is_locked = true WHERE id = $1`, card.ID); err != nil {
		t.Fatal(err)
	}
	if res, err = svc.Resolve(ctx, "c/"+card.SmartCode); err != nil || !res.IsLocked {
		t.Fatalf("locked card resolution = %+v, %v", res, err)
	}
	if _, err := d.Pool.Exec(ctx, `UPDATE routes SET status = 'retired' WHERE id = $1`, route.ID); err != nil {
		t.Fatal(err)
	}
	if res, err = svc.Resolve(ctx, "c/"+card.SmartCode); err != nil || res.TargetURL != "" || res.Status != model.RouteStatusRetired {
		t.Fatalf("retired card resolution = %+v, %v", res, err)
	}
}

func TestIntegrationResolveThroughCache(t *testing.T) {
	d, repo, svc := newRouteService(t)
	ctx := context.Background()
	mr := miniredis.RunT(t)
	svc.SetCache(routecache.New(redis.NewClient(&redis.Options{Addr: mr.Addr()}), "test", time.Minute))
	u := d.CreateUser(t)
	slug := "cached-" + pgtest.Unique()
	link := createSmartLink(t, repo, u, "s/"+slug, "https://example.com/a")

	if res, err := svc.Resolve(ctx, "s/"+slug); err != nil || res.TargetURL != "https://example.com/a" {
		t.Fatalf("first resolve = %+v, %v", res, err)
	}
	// A change behind the service's back is not seen while the entry is cached...
	if _, err := d.Pool.Exec(ctx, `UPDATE routes SET direct_url = 'https://example.com/b' WHERE id = $1`, link.ID); err != nil {
		t.Fatal(err)
	}
	if res, _ := svc.Resolve(ctx, "s/"+slug); res.TargetURL != "https://example.com/a" {
		t.Fatalf("cached resolve = %q, want the cached target", res.TargetURL)
	}
	// ...but an update through the service invalidates it.
	account := authctx.Account{ID: u.AccountID, Role: authctx.AccountRoleOwner}
	target := "https://example.com/c"
	if _, err := svc.Update(ctx, link.UUID, account, model.UpdateRouteInput{DirectURL: &target}); err != nil {
		t.Fatalf("update: %v", err)
	}
	if res, _ := svc.Resolve(ctx, "s/"+slug); res.TargetURL != target {
		t.Fatalf("resolve after update = %q, want %q", res.TargetURL, target)
	}
}

func TestIntegrationSmartLinkRenameWritesRouteHistory(t *testing.T) {
	d, repo, svc := newRouteService(t)
	ctx := context.Background()
	u := d.CreateUser(t)
	oldSlug, newSlug := "old-"+pgtest.Unique(), "new-"+pgtest.Unique()
	link := createSmartLink(t, repo, u, "s/"+oldSlug, "https://example.com")
	taken := createSmartLink(t, repo, u, "s/taken-"+pgtest.Unique(), "https://example.com")
	account := authctx.Account{ID: u.AccountID, Role: authctx.AccountRoleOwner}
	ctx = authctx.WithPrincipal(ctx, authctx.Principal{UserInternal: u.ID, Account: account})

	historyRows := func() int {
		var n int
		if err := d.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM route_history WHERE route_id = $1`, link.ID).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}

	// A rename onto a taken path fails without writing history.
	if _, err := svc.Update(ctx, link.UUID, account, model.UpdateRouteInput{Path: &taken.Path}); !errors.Is(err, usecase.ErrRoutePathConflict) {
		t.Fatalf("rename onto taken path: %v, want ErrRoutePathConflict", err)
	}
	// Other accounts cannot see the link at all.
	other := d.CreateUser(t)
	otherAccount := authctx.Account{ID: other.AccountID, Role: authctx.AccountRoleOwner}
	if _, err := svc.Update(ctx, link.UUID, otherAccount, model.UpdateRouteInput{Path: &newSlug}); !errors.Is(err, usecase.ErrRouteNotFound) {
		t.Fatalf("cross-account rename: %v, want ErrRouteNotFound", err)
	}
	if n := historyRows(); n != 0 {
		t.Fatalf("history rows after failed renames = %d", n)
	}

	updated, err := svc.Update(ctx, link.UUID, account, model.UpdateRouteInput{Path: &newSlug})
	if err != nil {
		t.Fatalf("rename: %v", err)
	}
	if updated.Path != "s/"+newSlug {
		t.Fatalf("renamed path = %q", updated.Path)
	}

	var oldPath, newPath, reason, routeType string
	var accountID, changedBy int64
	err = d.Pool.QueryRow(ctx, `SELECT old_path, new_path, reason, route_type, account_id, changed_by_user_id
		FROM route_history WHERE route_id = $1`, link.ID).Scan(&oldPath, &newPath, &reason, &routeType, &accountID, &changedBy)
	if err != nil {
		t.Fatalf("route_history: %v", err)
	}
	if oldPath != "s/"+oldSlug || newPath != "s/"+newSlug || reason != model.PathChangeSmartLinkRename ||
		routeType != model.RouteTypeCustomSmartLink || accountID != u.AccountID || changedBy != u.ID {
		t.Fatalf("history = %s -> %s (%s, %s, account %d, by %d)", oldPath, newPath, reason, routeType, accountID, changedBy)
	}

	// Saving the same path again is not a rename.
	same := "s/" + newSlug
	if _, err := svc.Update(ctx, link.UUID, account, model.UpdateRouteInput{Path: &same}); err != nil {
		t.Fatal(err)
	}
	if n := historyRows(); n != 1 {
		t.Fatalf("history rows = %d, want 1", n)
	}
	if _, err := svc.Resolve(ctx, "s/"+oldSlug); !errors.Is(err, usecase.ErrRouteNotFound) {
		t.Fatalf("old path still resolves: %v", err)
	}
	// The old path is held after the rename: another link cannot take it over.
	if _, err := svc.Update(ctx, taken.UUID, account, model.UpdateRouteInput{Path: &oldSlug}); !errors.Is(err, usecase.ErrRoutePathConflict) {
		t.Fatalf("taking over a renamed link's old path: %v, want ErrRoutePathConflict", err)
	}
}

// TestIntegrationBusinessPageTarget: a company card or smart link can send visitors to the
// company's business page (TEC-1327). The target follows the page: no target while the page is a
// draft, the current username once it is published, read fresh even with the route cache on.
func TestIntegrationBusinessPageTarget(t *testing.T) {
	d, _, svc := newRouteService(t)
	ctx := context.Background()
	mr := miniredis.RunT(t)
	svc.SetCache(routecache.New(redis.NewClient(&redis.Options{Addr: mr.Addr()}), "test", time.Minute))
	svc.SetQuotas(&stubQuotas{})
	u := d.CreateUser(t)
	var companyID int64
	if err := d.Pool.QueryRow(ctx, `INSERT INTO accounts (type, name, slug) VALUES ('company', 'Biz Co', $1) RETURNING id`,
		"bizco-"+pgtest.Unique()).Scan(&companyID); err != nil {
		t.Fatal(err)
	}
	if _, err := d.Pool.Exec(ctx, `INSERT INTO account_members (account_id, user_id, role) VALUES ($1, $2, 'owner')`, companyID, u.ID); err != nil {
		t.Fatal(err)
	}
	company := authctx.Account{ID: companyID, Type: authctx.AccountTypeCompany, Role: authctx.AccountRoleOwner}
	personal := authctx.Account{ID: u.AccountID, Type: authctx.AccountTypePersonal, Role: authctx.AccountRoleOwner}

	code := "cc-" + pgtest.Unique()
	var cardID int64
	if err := d.Pool.QueryRow(ctx, `INSERT INTO nfc_cards (user_id, account_id, name, smart_code, target_type)
		VALUES ($1, $2, 'Biz card', $3, 'business_page') RETURNING id`, u.ID, companyID, code).Scan(&cardID); err != nil {
		t.Fatal(err)
	}
	route, err := svc.CreateReservedCardRoute(ctx, cardID, code, &u.ID, &companyID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := d.Pool.Exec(ctx, `UPDATE routes SET status = 'active' WHERE id = $1`, route.ID); err != nil {
		t.Fatal(err)
	}

	// Draft page: the business page route is locked, so the card has nowhere to go yet.
	username := "biz-" + pgtest.Unique()
	var pageRouteID int64
	if err := d.Pool.QueryRow(ctx, `INSERT INTO routes (path, route_type, status, target_type, account_id)
		VALUES ($1, 'business_page', 'locked', 'business_page', $2) RETURNING id`, username, companyID).Scan(&pageRouteID); err != nil {
		t.Fatal(err)
	}
	res, err := svc.Resolve(ctx, "c/"+code)
	if err != nil || res.TargetType != model.TargetTypeBusinessPage || res.TargetURL != "" {
		t.Fatalf("draft page resolution = %+v, %v", res, err)
	}
	// Published: the card opens the page; the change is seen at once, not after the cache TTL.
	if _, err := d.Pool.Exec(ctx, `UPDATE routes SET status = 'active' WHERE id = $1`, pageRouteID); err != nil {
		t.Fatal(err)
	}
	if res, err = svc.Resolve(ctx, "c/"+code); err != nil || res.TargetURL != "https://comcode.test/"+username {
		t.Fatalf("published page resolution = %+v, %v", res, err)
	}

	// Smart links: company accounts only, no profile or URL needed.
	slug := "biz-" + pgtest.Unique()
	if _, err := svc.CreateCustomSmartLink(ctx, personal, u.ID, model.CreateSmartLinkInput{Slug: slug, TargetType: model.TargetTypeBusinessPage}); !errors.Is(err, usecase.ErrInvalidTarget) {
		t.Fatalf("personal business page link: %v, want ErrInvalidTarget", err)
	}
	link, err := svc.CreateCustomSmartLink(ctx, company, u.ID, model.CreateSmartLinkInput{Slug: slug, TargetType: model.TargetTypeBusinessPage})
	if err != nil {
		t.Fatalf("company business page link: %v", err)
	}
	if res, err = svc.Resolve(ctx, "s/"+slug); err != nil || res.TargetURL != "https://comcode.test/"+username {
		t.Fatalf("smart link resolution = %+v, %v", res, err)
	}
	own, err := svc.CreateCustomSmartLink(ctx, personal, u.ID, model.CreateSmartLinkInput{Slug: "own-" + pgtest.Unique(), TargetType: model.TargetTypeDirectURL, DirectURL: "https://example.com"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Update(ctx, link.UUID, personal, model.UpdateRouteInput{}); !errors.Is(err, usecase.ErrRouteNotFound) {
		t.Fatalf("personal update of the company link: %v, want ErrRouteNotFound", err)
	}
	target := model.TargetTypeBusinessPage
	if _, err := svc.Update(ctx, own.UUID, personal, model.UpdateRouteInput{TargetType: &target}); !errors.Is(err, usecase.ErrInvalidTarget) {
		t.Fatalf("personal update to business page: %v, want ErrInvalidTarget", err)
	}
}
