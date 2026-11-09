package repository

import (
	"context"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/routes/model"
	"github.com/google/uuid"
)

// Repository: List/Update/Delete/GetProfileIDByUUID are scoped to an account; path, uuid and
// card lookups are unscoped (resolver and provisioning) and callers check ownership.
type Repository interface {
	GetRouteByPath(ctx context.Context, path string) (*model.RouteDetail, error)
	GetRouteByUUID(ctx context.Context, id uuid.UUID) (*model.RouteDetail, error)
	GetRouteByCardID(ctx context.Context, cardID int64) (*model.RouteDetail, error)
	ListRoutes(ctx context.Context, accountID int64, routeType, status, q *string, limit, offset int32) ([]model.RouteSummary, int64, error)
	CreateRoute(ctx context.Context, input model.CreateReservedRouteInput) (*model.RouteDetail, error)
	UpdateRoute(ctx context.Context, id uuid.UUID, accountID int64, input model.UpdateRouteInput, profileID, cardID *int64, setProfileID, setCardID bool) (*model.RouteDetail, error)
	DeleteRoute(ctx context.Context, id uuid.UUID, accountID int64) error
	CheckPathExists(ctx context.Context, path string, excludeUUID *uuid.UUID) (bool, error)
	GetProfileIDByUUID(ctx context.Context, profileUUID uuid.UUID, accountID int64) (*int64, error)
	RecordPathChange(ctx context.Context, change model.PathChange) error
}
