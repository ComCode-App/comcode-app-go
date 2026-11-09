package usecase_test

import (
	"context"
	"errors"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/entitlement"
	"testing"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/routes/model"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/routes/usecase"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/authctx"
	"github.com/google/uuid"
)

// ownerAccount is the personal account of user id (account id == user id in these tests).
func ownerAccount(id int64) authctx.Account {
	return authctx.Account{ID: id, Type: authctx.AccountTypePersonal, Role: authctx.AccountRoleOwner}
}

type mockRouteRepo struct {
	routes       map[string]*model.RouteDetail
	routesByUUID map[uuid.UUID]*model.RouteDetail
	scanCounts   map[int64]int64
	profiles     map[uuid.UUID]int64
	history      []model.PathChange
}

func newMockRepo() *mockRouteRepo {
	return &mockRouteRepo{
		routes:       make(map[string]*model.RouteDetail),
		routesByUUID: make(map[uuid.UUID]*model.RouteDetail),
		scanCounts:   make(map[int64]int64),
		profiles:     make(map[uuid.UUID]int64),
	}
}

func (m *mockRouteRepo) RecordPathChange(ctx context.Context, c model.PathChange) error {
	m.history = append(m.history, c)
	return nil
}

func (m *mockRouteRepo) GetRouteByPath(ctx context.Context, path string) (*model.RouteDetail, error) {
	r, ok := m.routes[path]
	if !ok {
		return nil, nil
	}
	return r, nil
}

func (m *mockRouteRepo) GetRouteByUUID(ctx context.Context, id uuid.UUID) (*model.RouteDetail, error) {
	r, ok := m.routesByUUID[id]
	if !ok {
		return nil, nil
	}
	return r, nil
}

func (m *mockRouteRepo) GetRouteByCardID(ctx context.Context, cardID int64) (*model.RouteDetail, error) {
	for _, r := range m.routes {
		if r.CardID != nil && *r.CardID == cardID {
			return r, nil
		}
	}
	return nil, nil
}

func (m *mockRouteRepo) ListRoutes(ctx context.Context, accountID int64, routeType, status, q *string, limit, offset int32) ([]model.RouteSummary, int64, error) {
	var list []model.RouteSummary
	for _, r := range m.routes {
		if r.AccountID == nil || *r.AccountID != accountID {
			continue
		}
		list = append(list, model.RouteSummary{
			UUID:        r.UUID,
			Path:        r.Path,
			RouteType:   r.RouteType,
			Status:      r.Status,
			IsImmutable: r.IsImmutable,
			TargetType:  r.TargetType,
		})
	}
	return list, int64(len(list)), nil
}

func (m *mockRouteRepo) CreateRoute(ctx context.Context, input model.CreateReservedRouteInput) (*model.RouteDetail, error) {
	u := uuid.New()
	r := &model.RouteDetail{
		ID:                 int64(len(m.routes) + 1),
		UUID:               u,
		Path:               input.Path,
		RouteType:          input.RouteType,
		Status:             input.Status,
		IsImmutable:        input.IsImmutable,
		TargetType:         input.TargetType,
		ProfileID:          input.ProfileID,
		CardID:             input.CardID,
		DirectURL:          input.DirectURL,
		AccountID:          input.AccountID,
		OwnerUserID:        input.OwnerUserID,
		PresentationConfig: input.PresentationConfig,
		LegacyMetadata:     input.LegacyMetadata,
	}
	// Emulates the DB fill trigger: in tests a user's personal account id equals the user id.
	if r.AccountID == nil && r.OwnerUserID != nil {
		acc := *r.OwnerUserID
		r.AccountID = &acc
	}
	m.routes[input.Path] = r
	m.routesByUUID[u] = r
	return r, nil
}

func (m *mockRouteRepo) UpdateRoute(ctx context.Context, id uuid.UUID, accountID int64, input model.UpdateRouteInput, profileID, cardID *int64, setProfileID, setCardID bool) (*model.RouteDetail, error) {
	r, ok := m.routesByUUID[id]
	if !ok {
		return nil, errors.New("not found")
	}
	if input.Status != nil {
		r.Status = *input.Status
	}
	if input.Path != nil && !r.IsImmutable && *input.Path != r.Path {
		delete(m.routes, r.Path)
		r.Path = *input.Path
		m.routes[r.Path] = r
	}
	if input.TargetType != nil {
		r.TargetType = *input.TargetType
	}
	if input.DirectURL != nil {
		r.DirectURL = *input.DirectURL
	}
	if setProfileID {
		r.ProfileID = profileID
	}
	if input.PresentationConfig != nil {
		r.PresentationConfig = input.PresentationConfig
	}
	return r, nil
}

func (m *mockRouteRepo) DeleteRoute(ctx context.Context, id uuid.UUID, accountID int64) error {
	r, ok := m.routesByUUID[id]
	if !ok {
		return nil
	}
	delete(m.routes, r.Path)
	delete(m.routesByUUID, id)
	return nil
}

func (m *mockRouteRepo) CheckPathExists(ctx context.Context, path string, excludeUUID *uuid.UUID) (bool, error) {
	r, ok := m.routes[path]
	if !ok {
		return false, nil
	}
	if excludeUUID != nil && r.UUID == *excludeUUID {
		return false, nil
	}
	return true, nil
}

func (m *mockRouteRepo) GetProfileIDByUUID(ctx context.Context, profileUUID uuid.UUID, accountID int64) (*int64, error) {
	id, ok := m.profiles[profileUUID]
	if !ok {
		return nil, nil
	}
	return &id, nil
}

func TestReservedPathDetection(t *testing.T) {
	repo := newMockRepo()
	svc := usecase.NewService(repo, "https://app.com", nil)
	svc.SetQuotas(&stubQuotas{})

	tests := []struct {
		path     string
		reserved bool
	}{
		{"platform", true},
		{"/platform/users", true},
		{"login", true},
		{"api/v1/cards", true},
		{"c", true},
		{"s", true},
		{"furkanmeclis", false},
		{"s/my-company", false},
		{"c/cc-123456", false},
	}

	for _, tt := range tests {
		if got := svc.IsReservedPath(tt.path); got != tt.reserved {
			t.Errorf("IsReservedPath(%q) = %v; want %v", tt.path, got, tt.reserved)
		}
	}
}

func TestCreateCustomSmartLink(t *testing.T) {
	repo := newMockRepo()
	svc := usecase.NewService(repo, "https://app.com", nil)
	svc.SetQuotas(&stubQuotas{})
	ctx := context.Background()

	// Reserved keyword rejection
	_, err := svc.CreateCustomSmartLink(ctx, ownerAccount(1), 1, model.CreateSmartLinkInput{
		Slug: "platform",
	})
	if !errors.Is(err, usecase.ErrRoutePathReserved) {
		t.Fatalf("expected ErrRoutePathReserved, got %v", err)
	}

	// Valid creation
	created, err := svc.CreateCustomSmartLink(ctx, ownerAccount(1), 1, model.CreateSmartLinkInput{
		Slug:       "tech-event",
		TargetType: model.TargetTypeDirectURL,
		DirectURL:  "https://technowide.co",
	})
	if err != nil {
		t.Fatalf("failed to create smart link: %v", err)
	}
	if created.Path != "s/tech-event" {
		t.Errorf("expected path 's/tech-event', got %q", created.Path)
	}
	if created.RouteType != model.RouteTypeCustomSmartLink {
		t.Errorf("expected route type %s, got %s", model.RouteTypeCustomSmartLink, created.RouteType)
	}

	// Collision rejection
	_, err = svc.CreateCustomSmartLink(ctx, ownerAccount(1), 1, model.CreateSmartLinkInput{
		Slug: "tech-event",
	})
	if !errors.Is(err, usecase.ErrRoutePathConflict) {
		t.Fatalf("expected ErrRoutePathConflict on duplicate slug, got %v", err)
	}
}

func TestCardRouteReservationAndResolution(t *testing.T) {
	repo := newMockRepo()
	svc := usecase.NewService(repo, "https://app.com", nil)
	svc.SetQuotas(&stubQuotas{})
	ctx := context.Background()

	// Create reserved physical card route
	cardID := int64(42)
	userID := int64(1)
	route, err := svc.CreateReservedCardRoute(ctx, cardID, "cc-998811", &userID, nil)
	if err != nil {
		t.Fatalf("failed to reserve card route: %v", err)
	}
	if !route.IsImmutable {
		t.Errorf("card route must be immutable")
	}
	if route.Status != model.RouteStatusReserved {
		t.Errorf("card route status must be reserved, got %s", route.Status)
	}

	// Resolve route
	res, err := svc.Resolve(ctx, "c/cc-998811")
	if err != nil {
		t.Fatalf("failed to resolve route: %v", err)
	}
	if res.Path != "c/cc-998811" {
		t.Errorf("expected path 'c/cc-998811', got %q", res.Path)
	}
	if res.Status != model.RouteStatusReserved {
		t.Errorf("expected status 'reserved', got %q", res.Status)
	}
}

func TestLegacyRouteRegistration(t *testing.T) {
	repo := newMockRepo()
	svc := usecase.NewService(repo, "https://app.com", nil)
	svc.SetQuotas(&stubQuotas{})
	ctx := context.Background()

	claimedBy := int64(5)
	legacyInput := model.CreateReservedRouteInput{
		AccountID:  &claimedBy,
		Path:       "a8f9c2d1",
		TargetType: model.TargetTypeDirectURL,
		DirectURL:  "https://old.com/profile/84",
		LegacyMetadata: map[string]any{
			"legacy_system": "v1_legacy",
			"original_hash": "a8f9c2d1",
		},
	}

	route, err := svc.RegisterLegacyRoute(ctx, legacyInput)
	if err != nil {
		t.Fatalf("failed to register legacy route: %v", err)
	}
	if !route.IsImmutable {
		t.Errorf("legacy route must be immutable")
	}
	if route.RouteType != model.RouteTypeLegacyRoute {
		t.Errorf("expected route type legacy_route, got %s", route.RouteType)
	}

	// Even the owning account cannot delete an immutable (claimed) legacy route.
	err = svc.Delete(ctx, route.UUID, ownerAccount(claimedBy))
	if !errors.Is(err, usecase.ErrRouteImmutable) {
		t.Fatalf("expected ErrRouteImmutable when deleting immutable route, got %v", err)
	}
}

func TestRouteOwnershipIsolation(t *testing.T) {
	repo := newMockRepo()
	svc := usecase.NewService(repo, "https://app.com", nil)
	svc.SetQuotas(&stubQuotas{})
	ctx := context.Background()

	owner, other := int64(1), int64(2)
	link, err := svc.CreateCustomSmartLink(ctx, ownerAccount(owner), owner, model.CreateSmartLinkInput{
		Slug:       "owned-link",
		TargetType: model.TargetTypeDirectURL,
		DirectURL:  "https://example.com",
	})
	if err != nil {
		t.Fatalf("failed to create smart link: %v", err)
	}

	if _, err := svc.Get(ctx, link.UUID, ownerAccount(other)); !errors.Is(err, usecase.ErrRouteNotFound) {
		t.Errorf("Get by non-owner: expected ErrRouteNotFound, got %v", err)
	}
	newURL := "https://evil.example"
	if _, err := svc.Update(ctx, link.UUID, ownerAccount(other), model.UpdateRouteInput{DirectURL: &newURL}); !errors.Is(err, usecase.ErrRouteNotFound) {
		t.Errorf("Update by non-owner: expected ErrRouteNotFound, got %v", err)
	}
	if _, err := svc.ToggleLock(ctx, link.UUID, ownerAccount(other)); !errors.Is(err, usecase.ErrRouteNotFound) {
		t.Errorf("ToggleLock by non-owner: expected ErrRouteNotFound, got %v", err)
	}
	if err := svc.Delete(ctx, link.UUID, ownerAccount(other)); !errors.Is(err, usecase.ErrRouteNotFound) {
		t.Errorf("Delete by non-owner: expected ErrRouteNotFound, got %v", err)
	}

	if _, err := svc.Get(ctx, link.UUID, ownerAccount(owner)); err != nil {
		t.Errorf("Get by owner: unexpected error %v", err)
	}
	if _, err := svc.Get(ctx, link.UUID, ownerAccount(owner)); err != nil {
		t.Errorf("Get by platform admin: unexpected error %v", err)
	}
}

func TestOwnerCannotChangeRouteLifecycleOrCardPath(t *testing.T) {
	repo := newMockRepo()
	svc := usecase.NewService(repo, "https://app.com", nil)
	svc.SetQuotas(&stubQuotas{})
	ctx := context.Background()

	owner := int64(1)
	card, err := svc.CreateReservedCardRoute(ctx, 42, "cc-123456", &owner, nil)
	if err != nil {
		t.Fatalf("failed to reserve card route: %v", err)
	}

	active := model.RouteStatusActive
	if _, err := svc.Update(ctx, card.UUID, ownerAccount(owner), model.UpdateRouteInput{Status: &active}); !errors.Is(err, usecase.ErrRouteFieldForbidden) {
		t.Errorf("owner status change: expected ErrRouteFieldForbidden, got %v", err)
	}
	newPath := "c/cc-999999"
	if _, err := svc.Update(ctx, card.UUID, ownerAccount(owner), model.UpdateRouteInput{Path: &newPath}); !errors.Is(err, usecase.ErrRouteFieldForbidden) {
		t.Errorf("owner card path change: expected ErrRouteFieldForbidden, got %v", err)
	}

	// A reserved card must be activated, not unlocked.
	if _, err := svc.ToggleLock(ctx, card.UUID, ownerAccount(owner)); !errors.Is(err, usecase.ErrRouteNotLockable) {
		t.Errorf("toggle reserved route: expected ErrRouteNotLockable, got %v", err)
	}
	if got := repo.routesByUUID[card.UUID].Status; got != model.RouteStatusReserved {
		t.Errorf("reserved route status changed to %q", got)
	}
}

func TestToggleLockActiveAndLocked(t *testing.T) {
	repo := newMockRepo()
	svc := usecase.NewService(repo, "https://app.com", nil)
	svc.SetQuotas(&stubQuotas{})
	ctx := context.Background()

	owner := int64(1)
	link, err := svc.CreateCustomSmartLink(ctx, ownerAccount(owner), owner, model.CreateSmartLinkInput{
		Slug:       "lockable",
		TargetType: model.TargetTypeDirectURL,
		DirectURL:  "https://example.com",
	})
	if err != nil {
		t.Fatalf("failed to create smart link: %v", err)
	}

	locked, err := svc.ToggleLock(ctx, link.UUID, ownerAccount(owner))
	if err != nil || locked.Status != model.RouteStatusLocked {
		t.Fatalf("expected locked, got %v / %v", locked, err)
	}
	unlocked, err := svc.ToggleLock(ctx, link.UUID, ownerAccount(owner))
	if err != nil || unlocked.Status != model.RouteStatusActive {
		t.Fatalf("expected active, got %v / %v", unlocked, err)
	}
}

func TestRouteWritesNeedAdminRole(t *testing.T) {
	repo := newMockRepo()
	svc := usecase.NewService(repo, "https://app.com", nil)
	svc.SetQuotas(&stubQuotas{})
	ctx := context.Background()

	link, err := svc.CreateCustomSmartLink(ctx, ownerAccount(1), 1, model.CreateSmartLinkInput{
		Slug: "team-link", TargetType: model.TargetTypeDirectURL, DirectURL: "https://example.com",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	member := authctx.Account{ID: 1, Type: authctx.AccountTypeCompany, Role: authctx.AccountRoleMember}

	if _, err := svc.Get(ctx, link.UUID, member); err != nil {
		t.Errorf("member read: %v", err)
	}
	if _, err := svc.CreateCustomSmartLink(ctx, member, 2, model.CreateSmartLinkInput{Slug: "member-link"}); !errors.Is(err, usecase.ErrForbidden) {
		t.Errorf("member create: expected ErrForbidden, got %v", err)
	}
	url := "https://evil.example"
	if _, err := svc.Update(ctx, link.UUID, member, model.UpdateRouteInput{DirectURL: &url}); !errors.Is(err, usecase.ErrForbidden) {
		t.Errorf("member update: expected ErrForbidden, got %v", err)
	}
	if _, err := svc.ToggleLock(ctx, link.UUID, member); !errors.Is(err, usecase.ErrForbidden) {
		t.Errorf("member toggle: expected ErrForbidden, got %v", err)
	}
	if err := svc.Delete(ctx, link.UUID, member); !errors.Is(err, usecase.ErrForbidden) {
		t.Errorf("member delete: expected ErrForbidden, got %v", err)
	}
}

func TestSmartLinkCreateIsBlockedByQuota(t *testing.T) {
	repo := newMockRepo()
	svc := usecase.NewService(repo, "https://app.com", nil)
	svc.SetQuotas(&stubQuotas{deny: map[string]bool{entitlement.FeatureSmartLinksMax: true}})
	ctx := context.Background()
	in := model.CreateSmartLinkInput{Slug: "quota-link", TargetType: model.TargetTypeDirectURL, DirectURL: "https://example.com"}

	if _, err := svc.CreateCustomSmartLink(ctx, ownerAccount(1), 1, in); !errors.Is(err, entitlement.ErrPlanLimitExceeded) {
		t.Fatalf("expected ErrPlanLimitExceeded, got %v", err)
	}
	// Nothing was stored: the same slug is still free once the quota allows it.
	svc.SetQuotas(&stubQuotas{})
	if _, err := svc.CreateCustomSmartLink(ctx, ownerAccount(1), 1, in); err != nil {
		t.Fatalf("create after quota freed: %v", err)
	}
}

func TestCanonicalProfileRoutesAreNotEditableHere(t *testing.T) {
	repo := newMockRepo()
	svc := usecase.NewService(repo, "https://app.com", nil)
	svc.SetQuotas(&stubQuotas{})
	ctx := context.Background()
	route, err := repo.CreateRoute(ctx, model.CreateReservedRouteInput{
		Path: "ada", RouteType: model.RouteTypeCanonicalProfile, Status: model.RouteStatusRetired,
		TargetType: model.TargetTypeProfile, AccountID: func() *int64 { v := int64(1); return &v }(),
	})
	if err != nil {
		t.Fatal(err)
	}
	owner := ownerAccount(1)
	if err := svc.Delete(ctx, route.UUID, owner); !errors.Is(err, usecase.ErrRouteFieldForbidden) {
		t.Errorf("delete: %v", err)
	}
	url := "https://evil.example"
	if _, err := svc.Update(ctx, route.UUID, owner, model.UpdateRouteInput{DirectURL: &url}); !errors.Is(err, usecase.ErrRouteFieldForbidden) {
		t.Errorf("update: %v", err)
	}
	if _, err := svc.ToggleLock(ctx, route.UUID, owner); !errors.Is(err, usecase.ErrRouteFieldForbidden) {
		t.Errorf("toggle: %v", err)
	}
}

func TestSmartLinkRenameIsValidated(t *testing.T) {
	repo := newMockRepo()
	svc := usecase.NewService(repo, "https://app.com", nil)
	svc.SetQuotas(&stubQuotas{})
	ctx := context.Background()
	owner := ownerAccount(1)
	in := func(slug string) model.CreateSmartLinkInput {
		return model.CreateSmartLinkInput{Slug: slug, TargetType: model.TargetTypeDirectURL, DirectURL: "https://example.com"}
	}
	a, err := svc.CreateCustomSmartLink(ctx, owner, 1, in("promo"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.CreateCustomSmartLink(ctx, owner, 1, in("taken")); err != nil {
		t.Fatal(err)
	}
	cases := map[string]error{
		"s/taken": usecase.ErrRoutePathConflict,
		"login":   usecase.ErrRoutePathReserved,
		"s/-bad-": usecase.ErrInvalidRoutePath,
	}
	for path, want := range cases {
		p := path
		if _, err := svc.Update(ctx, a.UUID, owner, model.UpdateRouteInput{Path: &p}); !errors.Is(err, want) {
			t.Errorf("%q: got %v, want %v", path, err, want)
		}
	}
	// A bare name is kept under s/ instead of taking a first-level path.
	bare := "summer"
	updated, err := svc.Update(ctx, a.UUID, owner, model.UpdateRouteInput{Path: &bare})
	if err != nil || updated.Path != "s/summer" {
		t.Fatalf("rename: %+v %v", updated, err)
	}
	// The rename is recorded in route_history (TEC-558); failed renames and no-op updates are not.
	if len(repo.history) != 1 {
		t.Fatalf("history = %+v", repo.history)
	}
	h := repo.history[0]
	if h.OldPath != "s/promo" || h.NewPath != "s/summer" || h.Reason != model.PathChangeSmartLinkRename {
		t.Fatalf("history row = %+v", h)
	}
	same := "S/Summer"
	if _, err := svc.Update(ctx, a.UUID, owner, model.UpdateRouteInput{Path: &same}); err != nil || len(repo.history) != 1 {
		t.Fatalf("same path must not add history: %v %d", err, len(repo.history))
	}
}
