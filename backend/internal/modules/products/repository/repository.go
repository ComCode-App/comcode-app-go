package repository

import (
	"context"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/products/model"
	"github.com/google/uuid"
)

type Repository interface {
	List(ctx context.Context, q *string, categoryID *int64, status *string, productType *string, limit, offset int32) ([]model.ProductSummary, error)
	Count(ctx context.Context, q *string, categoryID *int64, status *string, productType *string) (int64, error)
	GetByUUID(ctx context.Context, id uuid.UUID) (*model.ProductDetail, error)
	GetBySlug(ctx context.Context, slug string) (*model.ProductDetail, error)
	Create(ctx context.Context, input model.UpsertProductInput, categoryID *int64, slug string) (*model.ProductDetail, error)
	Update(ctx context.Context, id uuid.UUID, input model.UpsertProductInput, categoryID *int64, slug string) (*model.ProductDetail, error)
	Update3DViewbox(ctx context.Context, id uuid.UUID, viewbox model.Model3DViewbox, fileURL, usdzURL string) (*model.ProductDetail, error)
	Delete(ctx context.Context, id uuid.UUID) error
	CheckSlugExists(ctx context.Context, slug string, excludeUUID *uuid.UUID) (bool, error)

	// Variants
	ListVariants(ctx context.Context, productID int64) ([]model.ProductVariantSummary, error)
	GetVariantByUUID(ctx context.Context, id uuid.UUID) (*model.ProductVariantSummary, error)
	CreateVariant(ctx context.Context, productID int64, input model.UpsertVariantInput) (*model.ProductVariantSummary, error)
	UpdateVariant(ctx context.Context, id uuid.UUID, input model.UpsertVariantInput) (*model.ProductVariantSummary, error)
	DeleteVariant(ctx context.Context, id uuid.UUID) error
}
