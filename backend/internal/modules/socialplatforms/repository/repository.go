package repository

import (
	"context"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/socialplatforms/model"
	"github.com/google/uuid"
)

type Repository interface {
	List(ctx context.Context, q *string, category *string, isActive *bool, limit, offset int32) ([]model.SocialPlatformSummary, error)
	Count(ctx context.Context, q *string, category *string, isActive *bool) (int64, error)
	ListActive(ctx context.Context) ([]model.SocialPlatformSummary, error)
	GetByUUID(ctx context.Context, id uuid.UUID) (*model.SocialPlatformDetail, error)
	GetBySlug(ctx context.Context, slug string) (*model.SocialPlatformDetail, error)
	Create(ctx context.Context, input model.UpsertSocialPlatformInput, slug string) (*model.SocialPlatformDetail, error)
	Update(ctx context.Context, id uuid.UUID, input model.UpsertSocialPlatformInput, slug string) (*model.SocialPlatformDetail, error)
	UpdateIcon(ctx context.Context, id uuid.UUID, iconURL string) (*model.SocialPlatformDetail, error)
	Delete(ctx context.Context, id uuid.UUID) error
	CheckSlugExists(ctx context.Context, slug string, excludeUUID *uuid.UUID) (bool, error)
}
