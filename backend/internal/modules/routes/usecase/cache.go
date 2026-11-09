package usecase

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/routes/model"
)

// cachedRoute is the cache payload. RouteDetail hides its internal ids from JSON, but the
// resolver needs CardID to compute is_active, so they are carried explicitly.
type cachedRoute struct {
	Detail      model.RouteDetail `json:"detail"`
	ID          int64             `json:"id"`
	ProfileID   *int64            `json:"profile_id,omitempty"`
	CardID      *int64            `json:"card_id,omitempty"`
	AccountID   *int64            `json:"account_id,omitempty"`
	OwnerUserID *int64            `json:"owner_user_id,omitempty"`
}

// lookupPath resolves one normalized path through the cache, falling back to Postgres.
func (s *Service) lookupPath(ctx context.Context, path string) (*model.RouteDetail, error) {
	if raw, ok := s.cache.Get(ctx, path); ok {
		var c cachedRoute
		if err := json.Unmarshal(raw, &c); err == nil {
			d := c.Detail
			d.ID, d.ProfileID, d.CardID, d.AccountID, d.OwnerUserID = c.ID, c.ProfileID, c.CardID, c.AccountID, c.OwnerUserID
			return &d, nil
		}
	}
	// Check aliases only on a root cache miss. Alias hits use the same cache;
	// ordinary profile cache hits never query Postgres for an absent alias.
	if !strings.Contains(path, "/") {
		candidate, lookupErr := s.lookupPath(ctx, "r/legacy/"+path)
		if lookupErr != nil {
			return nil, lookupErr
		}
		if candidate != nil && candidate.RouteType == model.RouteTypeLegacyRoute {
			return candidate, nil
		}
	}
	route, err := s.repo.GetRouteByPath(ctx, path)
	if err != nil || route == nil {
		return route, err
	}
	if raw, err := json.Marshal(cachedRoute{
		Detail: *route, ID: route.ID, ProfileID: route.ProfileID, CardID: route.CardID,
		AccountID: route.AccountID, OwnerUserID: route.OwnerUserID,
	}); err == nil {
		s.cache.Set(ctx, path, raw, route.CardUUID, route.ProfileUUID)
	}
	return route, nil
}
