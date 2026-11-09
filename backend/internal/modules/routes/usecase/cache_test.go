package usecase_test

import (
	"context"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/routes/model"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/routes/usecase"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/routecache"
)

func TestResolveUsesCacheAndInvalidatesOnLock(t *testing.T) {
	mr := miniredis.RunT(t)
	rc := routecache.New(redis.NewClient(&redis.Options{Addr: mr.Addr()}), "test", 0)
	repo := newMockRepo()
	svc := usecase.NewService(repo, "https://app.com", nil)
	svc.SetCache(rc)
	ctx := context.Background()

	accountID, cardID := int64(7), int64(42)
	cardUUID := uuid.New()
	route := &model.RouteDetail{
		ID: 1, UUID: uuid.New(), Path: "s/promo", RouteType: model.RouteTypeCustomSmartLink,
		Status: model.RouteStatusActive, TargetType: model.TargetTypeDirectURL, DirectURL: "https://x.test",
		AccountID: &accountID, CardID: &cardID, CardUUID: &cardUUID, CardIsActive: true,
	}
	repo.routes[route.Path] = route
	repo.routesByUUID[route.UUID] = route

	first, err := svc.Resolve(ctx, "s/promo")
	if err != nil || !first.IsActive {
		t.Fatalf("first resolve: %+v, %v", first, err)
	}
	if !mr.Exists("app:test:route:path:s/promo") || !mr.Exists("app:test:route:card:"+cardUUID.String()) {
		t.Fatalf("entry or card tag not cached: %v", mr.Keys())
	}

	// A write that bypasses the service is not seen until the entry is invalidated. The cached
	// copy must keep CardID, otherwise is_active would be computed as a card-less route.
	route.DirectURL = "https://changed.test"
	cached, _ := svc.Resolve(ctx, "s/promo")
	if cached.TargetURL != "https://x.test" || !cached.IsActive {
		t.Fatalf("expected cached resolution, got %+v", cached)
	}

	if _, err := svc.ToggleLock(ctx, route.UUID, ownerAccount(accountID)); err != nil {
		t.Fatal(err)
	}
	locked, _ := svc.Resolve(ctx, "s/promo")
	if !locked.IsLocked || locked.TargetURL != "https://changed.test" {
		t.Fatalf("lock must invalidate the cache, got %+v", locked)
	}

	rc.InvalidateCard(ctx, cardUUID)
	if mr.Exists("app:test:route:path:s/promo") {
		t.Fatal("card invalidation must drop tagged entries")
	}
}

func TestResolveWithoutCache(t *testing.T) {
	repo := newMockRepo()
	svc := usecase.NewService(repo, "https://app.com", nil)
	svc.SetCache(routecache.New(nil, "test", 0)) // nil client disables caching
	r := &model.RouteDetail{UUID: uuid.New(), Path: "s/a", Status: model.RouteStatusActive, TargetType: model.TargetTypeDirectURL, DirectURL: "https://a.test"}
	repo.routes[r.Path] = r
	res, err := svc.Resolve(context.Background(), "s/a")
	if err != nil || res.TargetURL != "https://a.test" {
		t.Fatalf("%+v %v", res, err)
	}
}

func TestRetiredRouteHasNoTarget(t *testing.T) {
	repo := newMockRepo()
	svc := usecase.NewService(repo, "https://app.com", nil)
	r := &model.RouteDetail{UUID: uuid.New(), Path: "c/old", Status: model.RouteStatusRetired, TargetType: model.TargetTypeDirectURL, DirectURL: "https://a.test"}
	repo.routes[r.Path] = r
	res, err := svc.Resolve(context.Background(), "c/old")
	if err != nil || res.TargetURL != "" || res.IsActive {
		t.Fatalf("retired route must not redirect: %+v %v", res, err)
	}
}

// Count repository calls, including misses, so an alias probe cannot hide behind a cached root.
type countingRouteRepo struct {
	*mockRouteRepo
	lookups int
}

func (r *countingRouteRepo) GetRouteByPath(ctx context.Context, path string) (*model.RouteDetail, error) {
	r.lookups++
	return r.mockRouteRepo.GetRouteByPath(ctx, path)
}

func TestBarePathCacheIncludesLegacyPrecedence(t *testing.T) {
	for _, alias := range []bool{false, true} {
		name := "ordinary profile"
		if alias {
			name = "legacy collision"
		}
		t.Run(name, func(t *testing.T) {
			mr := miniredis.RunT(t)
			client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
			t.Cleanup(func() { _ = client.Close() })
			cache := routecache.New(client, "test", 0)
			repo := &countingRouteRepo{mockRouteRepo: newMockRepo()}
			svc := usecase.NewService(repo, "https://app.test", nil)
			svc.SetCache(cache)
			root := &model.RouteDetail{UUID: uuid.New(), Path: "alice", RouteType: model.RouteTypeCanonicalProfile, Status: model.RouteStatusActive, TargetType: model.TargetTypeDirectURL, DirectURL: "https://profile.test"}
			repo.routes[root.Path] = root
			expected := root.DirectURL
			if alias {
				repo.routes["r/legacy/alice"] = &model.RouteDetail{UUID: uuid.New(), Path: "r/legacy/alice", RouteType: model.RouteTypeLegacyRoute, Status: model.RouteStatusActive, TargetType: model.TargetTypeDirectURL, DirectURL: "https://legacy.test"}
				expected = "https://legacy.test"
			}
			for i := 0; i < 2; i++ {
				result, err := svc.Resolve(t.Context(), " /ALICE/ ")
				if err != nil || result.TargetURL != expected {
					t.Fatalf("resolve: %+v %v", result, err)
				}
				legacy, err := svc.ResolveLegacy(t.Context(), "alice")
				if err != nil || (alias && (legacy == nil || legacy.TargetURL != expected)) || (!alias && legacy != nil) {
					t.Fatalf("public endpoint resolver: %+v %v", legacy, err)
				}
				want := 2
				if alias {
					want = 1
				}
				if repo.lookups != want {
					t.Fatalf("resolve %d: got %d repository calls, want %d", i, repo.lookups, want)
				}
			}
			// Invalidating the root allows a newly imported alias to take precedence.
			if !alias {
				repo.routes["r/legacy/alice"] = &model.RouteDetail{UUID: uuid.New(), Path: "r/legacy/alice", RouteType: model.RouteTypeLegacyRoute, Status: model.RouteStatusActive, TargetType: model.TargetTypeDirectURL, DirectURL: "https://imported.test"}
				cache.InvalidatePaths(t.Context(), "alice")
				result, err := svc.Resolve(t.Context(), "alice")
				if err != nil || result.TargetURL != "https://imported.test" {
					t.Fatalf("post-import invalidation: %+v %v", result, err)
				}
			}
		})
	}
}

func TestExistingReservedUsernameStillResolves(t *testing.T) {
	repo := newMockRepo()
	// New registrations reject these names; existing rows remain readable.
	for _, path := range []string{"checkout", "products", "plans", "subscriptions", "ready", "leads"} {
		repo.routes[path] = &model.RouteDetail{UUID: uuid.New(), Path: path, RouteType: model.RouteTypeCanonicalProfile, Status: model.RouteStatusActive, TargetType: model.TargetTypeProfile, ProfileUsername: path}
		result, err := usecase.NewService(repo, "https://app.test", nil).Resolve(t.Context(), path)
		if err != nil || result.ProfileUsername != path || result.TargetURL != "https://app.test/"+path {
			t.Fatalf("existing reserved username %s: %+v %v", path, result, err)
		}
	}
}

// TEC-956 option E: the canonical username is the card's own hash, its legacy route lives at
// the alias, and /{hash} must render instead of redirecting to itself.
func TestLegacyAliasForOwnHashUsernameDoesNotRedirect(t *testing.T) {
	repo := newMockRepo()
	svc := usecase.NewService(repo, "https://app.test", nil)
	repo.routes["r/legacy/furkan"] = &model.RouteDetail{UUID: uuid.New(), Path: "r/legacy/furkan", RouteType: model.RouteTypeLegacyRoute, Status: model.RouteStatusActive, TargetType: model.TargetTypeProfile, ProfileUsername: "furkan"}
	if legacy, err := svc.ResolveLegacy(t.Context(), "Furkan"); err != nil || legacy != nil {
		t.Fatalf("own-hash username redirected: %+v %v", legacy, err)
	}
	repo.routes["r/legacy/furkan"].ProfileUsername = "furkan-renamed"
	legacy, err := svc.ResolveLegacy(t.Context(), "furkan")
	if err != nil || legacy == nil || legacy.TargetURL != "https://app.test/furkan-renamed" {
		t.Fatalf("renamed profile must still follow its printed card: %+v %v", legacy, err)
	}
}
