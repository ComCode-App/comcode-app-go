package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/categories/model"
)

type Repository interface {
	List(ctx context.Context, q *string, isActive *bool, limit, offset int32) ([]model.CategorySummary, error)
	Count(ctx context.Context, q *string, isActive *bool) (int64, error)
	ListActive(ctx context.Context) ([]model.CategorySummary, error)
	GetByUUID(ctx context.Context, id uuid.UUID) (*model.CategoryDetail, error)
	GetBySlug(ctx context.Context, slug string) (*model.CategoryDetail, error)
	Create(ctx context.Context, input model.UpsertCategoryInput, slug string) (*model.CategoryDetail, error)
	Update(ctx context.Context, id uuid.UUID, input model.UpsertCategoryInput, slug string) (*model.CategoryDetail, error)
	Delete(ctx context.Context, id uuid.UUID) error
	CheckSlugExists(ctx context.Context, slug string, excludeUUID *uuid.UUID) (bool, error)
}
