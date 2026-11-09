package repository

import (
	"context"
	"errors"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/profiles/model"
	"github.com/google/uuid"
)

var (
	ErrNotFound = errors.New("profile not found")
)

// Repository defines the profile data store operations. Every method is scoped to an
// account: rows outside accountID behave as missing (ErrNotFound).
type Repository interface {
	ListByAccountID(ctx context.Context, accountID int64, limit, offset int32, q, status string) ([]model.ProfileSummary, int64, error)
	GetByUUID(ctx context.Context, accountID int64, profileUUID uuid.UUID) (*model.ProfileDetail, error)
	GetPrimaryByAccountID(ctx context.Context, accountID int64) (*model.ProfileDetail, error)
	Create(ctx context.Context, accountID, creatorUserID int64, creatorUUID uuid.UUID, in model.UpsertProfileInput, completionRate int) (*model.ProfileDetail, error)
	Update(ctx context.Context, accountID int64, profileUUID uuid.UUID, userUUID uuid.UUID, in model.UpsertProfileInput, completionRate int) (*model.ProfileDetail, error)
	UpdateAvatar(ctx context.Context, accountID int64, profileUUID uuid.UUID, avatarURL string) (*model.ProfileDetail, error)
	UpdateCover(ctx context.Context, accountID int64, profileUUID uuid.UUID, coverURL string) (*model.ProfileDetail, error)
	Delete(ctx context.Context, accountID int64, profileUUID uuid.UUID) error
	SetPrimary(ctx context.Context, accountID int64, profileUUID uuid.UUID) error
}
