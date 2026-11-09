// Package usecase implements the platform card and route support actions (TEC-950): search,
// retire a card, reassign a card to another profile and release a quarantined path early.
package usecase

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/platformsupport/model"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/platformsupport/repository"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/activity"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/routecache"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/searchengine"
)

var (
	ErrNotFound           = errors.New("not found")
	ErrQueryTooShort      = errors.New("query too short")
	ErrCardRetired        = errors.New("card is retired")
	ErrCardHasNoRoute     = errors.New("card has no route")
	ErrCardNotActivated   = errors.New("card is not activated")
	ErrProfileNotFound    = errors.New("profile not found")
	ErrProfileHasNoOwner  = errors.New("profile has no owner")
	ErrPathNotQuarantined = errors.New("path is not quarantined")
)

const (
	minQueryLen = 2
	searchLimit = 25
)

// Cache is the slice of routecache the service uses; *routecache.Cache satisfies it.
type Cache interface {
	InvalidatePaths(ctx context.Context, paths ...string)
	InvalidateCard(ctx context.Context, cardUUID uuid.UUID)
}

// Recorder writes the activity log; *activity.Recorder satisfies it.
type Recorder interface {
	Record(ctx context.Context, actorID *int64, action, resource string, resourceUUID *uuid.UUID, payload map[string]any, r *http.Request)
}

var (
	_ Cache    = (*routecache.Cache)(nil)
	_ Recorder = (*activity.Recorder)(nil)
)

type Service struct {
	repo     repository.Repository
	cache    Cache
	activity Recorder
	search   searchengine.Enqueuer
}

func New(repo repository.Repository, cache Cache, rec Recorder) *Service {
	return &Service{repo: repo, cache: cache, activity: rec}
}

// SetSearchIndexer re-indexes a card that moved to another account (TEC-951 card index).
func (s *Service) SetSearchIndexer(e searchengine.Enqueuer) { s.search = e }

// Actor is the platform user performing an action, with the request for the activity log.
type Actor struct {
	UserID  int64
	Request *http.Request
}

func normalizeQuery(q string) (string, error) {
	q = strings.TrimSpace(q)
	if len(q) < minQueryLen {
		return "", ErrQueryTooShort
	}
	return q, nil
}

// NormalizePath matches routes.Service.NormalizePath: trimmed, no outer slashes, lower case.
func NormalizePath(p string) string {
	return strings.ToLower(strings.Trim(strings.TrimSpace(p), "/"))
}

func (s *Service) SearchCards(ctx context.Context, q string) ([]model.Card, error) {
	q, err := normalizeQuery(q)
	if err != nil {
		return nil, err
	}
	return s.repo.SearchCards(ctx, q, searchLimit)
}

func (s *Service) GetCard(ctx context.Context, id uuid.UUID) (*model.Card, error) {
	c, err := s.repo.GetCard(ctx, id)
	if err != nil {
		return nil, err
	}
	if c == nil {
		return nil, ErrNotFound
	}
	return c, nil
}

// RetireCard retires the card's route: the card shows "card deactivated", its path is never
// handed out again and an unused activation code is cleared. Retirement is one-way.
func (s *Service) RetireCard(ctx context.Context, actor Actor, id uuid.UUID) (*model.Card, error) {
	card, err := s.GetCard(ctx, id)
	if err != nil {
		return nil, err
	}
	if card.Route == nil {
		return nil, ErrCardHasNoRoute
	}
	if card.ActivationStatus == model.ActivationRetired {
		return nil, ErrCardRetired
	}
	paths, err := s.repo.RetireCard(ctx, card.ID)
	if err != nil {
		return nil, err
	}
	s.invalidateCard(ctx, card, paths...)
	s.record(ctx, actor, "card.retired", "cards", &card.UUID, map[string]any{
		"smart_code":   card.SmartCode,
		"path":         card.Route.Path,
		"account_uuid": card.Account.UUID,
		"before":       map[string]any{"activation_status": card.ActivationStatus, "route_status": card.Route.Status},
		"after":        map[string]any{"activation_status": model.ActivationRetired, "route_status": "retired"},
	})
	return s.GetCard(ctx, id)
}

// ReassignCard points an activated card at another profile. The card and its route move to the
// profile's account and owner; the printed identity (smart code, UID, path) stays.
func (s *Service) ReassignCard(ctx context.Context, actor Actor, id, profileUUID uuid.UUID) (*model.Card, error) {
	card, err := s.GetCard(ctx, id)
	if err != nil {
		return nil, err
	}
	switch card.ActivationStatus {
	case model.ActivationRetired:
		return nil, ErrCardRetired
	case model.ActivationPending:
		// Activation sets the profile itself and would overwrite a reassignment.
		return nil, ErrCardNotActivated
	}
	target, err := s.repo.GetReassignTarget(ctx, profileUUID)
	if err != nil {
		return nil, err
	}
	if target == nil {
		return nil, ErrProfileNotFound
	}
	if target.OwnerUserID == 0 {
		return nil, ErrProfileHasNoOwner
	}
	if card.Profile != nil && card.Profile.UUID == profileUUID {
		return card, nil
	}
	if err := s.repo.ReassignCard(ctx, card.ID, *target); err != nil {
		return nil, err
	}
	var paths []string
	if card.Route != nil {
		paths = append(paths, card.Route.Path)
	}
	s.invalidateCard(ctx, card, paths...)
	updated, err := s.GetCard(ctx, id)
	if err != nil {
		return nil, err
	}
	if s.search != nil {
		s.search.EnqueueUpsert(ctx, "cards", card.UUID.String())
	}
	before := map[string]any{"account_uuid": card.Account.UUID, "owner_uuid": card.Owner.UUID}
	if card.Profile != nil {
		before["profile_uuid"] = card.Profile.UUID
	}
	s.record(ctx, actor, "card.reassigned", "cards", &card.UUID, map[string]any{
		"smart_code": card.SmartCode,
		"before":     before,
		"after": map[string]any{
			"profile_uuid": profileUUID, "account_uuid": updated.Account.UUID, "owner_uuid": updated.Owner.UUID,
		},
	})
	return updated, nil
}

func (s *Service) record(ctx context.Context, actor Actor, action, resource string, id *uuid.UUID, payload map[string]any) {
	if s.activity != nil {
		s.activity.Record(ctx, &actor.UserID, action, resource, id, payload, actor.Request)
	}
}

func (s *Service) invalidateCard(ctx context.Context, card *model.Card, paths ...string) {
	if s.cache == nil {
		return
	}
	s.cache.InvalidatePaths(ctx, paths...)
	s.cache.InvalidateCard(ctx, card.UUID)
}

func (s *Service) SearchRoutes(ctx context.Context, q string) ([]model.Route, error) {
	q, err := normalizeQuery(NormalizePath(q))
	if err != nil {
		return nil, err
	}
	return s.repo.SearchRoutes(ctx, q, searchLimit)
}

func (s *Service) GetRoute(ctx context.Context, id uuid.UUID) (*model.Route, error) {
	rt, err := s.repo.GetRoute(ctx, id)
	if err != nil {
		return nil, err
	}
	if rt == nil {
		return nil, ErrNotFound
	}
	return rt, nil
}

// ReleasePath ends the 30-day quarantine on a path before it runs out: deleted smart links and
// renamed links stop holding it, so a new smart link can take it right away.
func (s *Service) ReleasePath(ctx context.Context, actor Actor, rawPath string) (*model.ReleaseResult, error) {
	path := NormalizePath(rawPath)
	if len(path) < minQueryLen {
		return nil, ErrQueryTooShort
	}
	ids, err := s.repo.ReleasePath(ctx, path)
	if err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return nil, ErrPathNotQuarantined
	}
	if s.cache != nil {
		s.cache.InvalidatePaths(ctx, path)
	}
	s.record(ctx, actor, "route.quarantine_released", "routes", &ids[0], map[string]any{
		"path":        path,
		"route_uuids": ids,
	})
	return &model.ReleaseResult{Path: path, RouteUUIDs: ids}, nil
}
