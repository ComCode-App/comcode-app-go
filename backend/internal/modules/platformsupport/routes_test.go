package platformsupport_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/middleware"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/platformsupport"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/platformsupport/handler"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/platformsupport/model"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/platformsupport/usecase"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/authctx"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/rbac"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/stepup"
)

var (
	cardUUID    = uuid.New()
	profileUUID = uuid.New()
	routeUUID   = uuid.New()
)

// fakeRepo answers every call with an activated card and a quarantined path.
type fakeRepo struct{ writes []string }

func (f *fakeRepo) card() *model.Card {
	return &model.Card{
		ID: 1, UUID: cardUUID, SmartCode: "abc123", ActivationStatus: model.ActivationActive,
		Route: &model.RouteRef{UUID: routeUUID, Path: "c/abc123", Status: "active"},
	}
}
func (f *fakeRepo) SearchCards(context.Context, string, int32) ([]model.Card, error) {
	return []model.Card{*f.card()}, nil
}
func (f *fakeRepo) GetCard(context.Context, uuid.UUID) (*model.Card, error) { return f.card(), nil }
func (f *fakeRepo) RetireCard(context.Context, int64) ([]string, error) {
	f.writes = append(f.writes, "retire")
	return []string{"c/abc123"}, nil
}
func (f *fakeRepo) GetReassignTarget(context.Context, uuid.UUID) (*model.ReassignTarget, error) {
	return &model.ReassignTarget{ProfileID: 2, ProfileUUID: profileUUID, AccountID: 3, OwnerUserID: 4}, nil
}
func (f *fakeRepo) ReassignCard(context.Context, int64, model.ReassignTarget) error {
	f.writes = append(f.writes, "reassign")
	return nil
}
func (f *fakeRepo) SearchRoutes(context.Context, string, int32) ([]model.Route, error) {
	return []model.Route{{UUID: routeUUID, Path: "s/x", Status: model.RouteStatusQuarantine, History: []model.HistoryEntry{}}}, nil
}
func (f *fakeRepo) GetRoute(context.Context, uuid.UUID) (*model.Route, error) {
	return &model.Route{UUID: routeUUID, Path: "s/x", History: []model.HistoryEntry{}}, nil
}
func (f *fakeRepo) ReleasePath(context.Context, string) ([]uuid.UUID, error) {
	f.writes = append(f.writes, "release")
	return []uuid.UUID{routeUUID}, nil
}

type fakeCache struct{ paths, cards []string }

func (c *fakeCache) InvalidatePaths(_ context.Context, paths ...string) {
	c.paths = append(c.paths, paths...)
}
func (c *fakeCache) InvalidateCard(_ context.Context, id uuid.UUID) {
	c.cards = append(c.cards, id.String())
}

type fixture struct {
	mux   *http.ServeMux
	repo  *fakeRepo
	cache *fakeCache
	store *stepup.Store
}

func newFixture(t *testing.T) fixture {
	t.Helper()
	mr := miniredis.RunT(t)
	store := stepup.NewStore(redis.NewClient(&redis.Options{Addr: mr.Addr()}), "test")
	repo, cache := &fakeRepo{}, &fakeCache{}
	mux := http.NewServeMux()
	// Test authentication: the X-Perms header lists the caller's permissions.
	authn := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			p := authctx.Principal{UserID: staffUUID, UserInternal: 9}
			if v := r.Header.Get("X-Perms"); v != "" {
				p.Permissions = strings.Split(v, ",")
			}
			next.ServeHTTP(w, r.WithContext(authctx.WithPrincipal(r.Context(), p)))
		})
	}
	svc := usecase.New(repo, cache, nil)
	platformsupport.Register(mux, handler.New(svc), authn, middleware.RequireStepUp(stepup.NewService(nil, store, nil, nil)))
	return fixture{mux: mux, repo: repo, cache: cache, store: store}
}

var staffUUID = uuid.New()

func (f fixture) do(method, path, perms, body string) (int, string) {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if perms != "" {
		req.Header.Set("X-Perms", perms)
	}
	rec := httptest.NewRecorder()
	f.mux.ServeHTTP(rec, req)
	var env struct {
		Error *struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &env)
	code := ""
	if env.Error != nil {
		code = env.Error.Code
	}
	return rec.Code, code
}

type routeCase struct {
	method, path, body string
	read, write        string // permission that grants it; write routes also need step-up
}

var cases = []routeCase{
	{method: "GET", path: "/v1/platform/cards?q=abc", read: rbac.PermPlatformCardsRead},
	{method: "GET", path: "/v1/platform/cards/" + cardUUID.String(), read: rbac.PermPlatformCardsRead},
	{method: "POST", path: "/v1/platform/cards/" + cardUUID.String() + "/retire", write: rbac.PermPlatformCardsWrite},
	{method: "POST", path: "/v1/platform/cards/" + cardUUID.String() + "/reassign", body: `{"profile_uuid":"` + profileUUID.String() + `"}`, write: rbac.PermPlatformCardsWrite},
	{method: "GET", path: "/v1/platform/routes?q=s/x", read: rbac.PermPlatformRoutesRead},
	{method: "GET", path: "/v1/platform/routes/" + routeUUID.String(), read: rbac.PermPlatformRoutesRead},
	{method: "POST", path: "/v1/platform/routes/quarantine/release", body: `{"path":"s/x"}`, write: rbac.PermPlatformRoutesWrite},
}

func TestPermissionGates(t *testing.T) {
	f := newFixture(t)
	if _, err := f.store.SetGrant(context.Background(), staffUUID, "password", time.Hour); err != nil {
		t.Fatal(err)
	}
	for _, c := range cases {
		need := c.read + c.write
		// No permission, the app-side permission, or the read permission for a write: 403.
		wrong := []string{"", strings.Replace(need, "platform.", "app.", 1)}
		if c.write != "" {
			wrong = append(wrong, strings.Replace(c.write, ".write", ".read", 1))
		}
		for _, perms := range wrong {
			if status, code := f.do(c.method, c.path, perms, c.body); status != http.StatusForbidden || code == "STEP_UP_REQUIRED" {
				t.Errorf("%s %s with %q: %d %s, want 403 permission denied", c.method, c.path, perms, status, code)
			}
		}
		if status, code := f.do(c.method, c.path, need, c.body); status != http.StatusOK {
			t.Errorf("%s %s with %q: %d %s, want 200", c.method, c.path, need, status, code)
		}
	}
	if got := strings.Join(f.repo.writes, ","); got != "retire,reassign,release" {
		t.Fatalf("writes = %s", got)
	}
}

func TestWritesRequireStepUp(t *testing.T) {
	f := newFixture(t)
	for _, c := range cases {
		if c.write == "" {
			// Reads never ask for step-up.
			if status, _ := f.do(c.method, c.path, c.read, c.body); status != http.StatusOK {
				t.Errorf("%s %s without step-up: %d, want 200", c.method, c.path, status)
			}
			continue
		}
		if status, code := f.do(c.method, c.path, c.write, c.body); status != http.StatusForbidden || code != "STEP_UP_REQUIRED" {
			t.Errorf("%s %s without step-up: %d %s, want 403 STEP_UP_REQUIRED", c.method, c.path, status, code)
		}
	}
	if len(f.repo.writes) != 0 {
		t.Fatalf("writes ran without step-up: %v", f.repo.writes)
	}
	if len(f.cache.paths)+len(f.cache.cards) != 0 {
		t.Fatal("cache touched without step-up")
	}
}

func TestActionsInvalidateRouteCache(t *testing.T) {
	f := newFixture(t)
	if _, err := f.store.SetGrant(context.Background(), staffUUID, "password", time.Hour); err != nil {
		t.Fatal(err)
	}
	all := rbac.PermPlatformCardsWrite + "," + rbac.PermPlatformRoutesWrite
	for _, c := range cases {
		if c.write != "" {
			if status, code := f.do(c.method, c.path, all, c.body); status != http.StatusOK {
				t.Fatalf("%s %s: %d %s", c.method, c.path, status, code)
			}
		}
	}
	if got := strings.Join(f.cache.paths, ","); got != "c/abc123,c/abc123,s/x" {
		t.Fatalf("invalidated paths = %s", got)
	}
	if len(f.cache.cards) != 2 || f.cache.cards[0] != cardUUID.String() {
		t.Fatalf("invalidated cards = %v", f.cache.cards)
	}
}
