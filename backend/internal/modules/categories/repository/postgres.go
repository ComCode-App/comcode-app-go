package repository

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/database/db"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/categories/model"
)

type PostgresRepository struct {
	q *db.Queries
}

func NewPostgres(q *db.Queries) *PostgresRepository {
	return &PostgresRepository{q: q}
}

func (r *PostgresRepository) List(ctx context.Context, q *string, isActive *bool, limit, offset int32) ([]model.CategorySummary, error) {
	rows, err := r.q.ListProductCategories(ctx, db.ListProductCategoriesParams{
		Q:           textToPgtype(q),
		IsActive:    boolToPgtype(isActive),
		LimitCount:  limit,
		OffsetCount: offset,
	})
	if err != nil {
		return nil, err
	}

	result := make([]model.CategorySummary, 0, len(rows))
	for _, row := range rows {
		result = append(result, model.CategorySummary{
			UUID:         row.Uuid.String(),
			Name:         row.Name,
			Slug:         row.Slug,
			Description:  row.Description,
			Icon:         row.Icon,
			ImageURL:     row.ImageUrl,
			DisplayOrder: int(row.DisplayOrder),
			IsActive:     row.IsActive,
		})
	}
	return result, nil
}

func (r *PostgresRepository) Count(ctx context.Context, q *string, isActive *bool) (int64, error) {
	return r.q.CountProductCategories(ctx, db.CountProductCategoriesParams{
		Q:        textToPgtype(q),
		IsActive: boolToPgtype(isActive),
	})
}

func (r *PostgresRepository) ListActive(ctx context.Context) ([]model.CategorySummary, error) {
	rows, err := r.q.ListActiveProductCategories(ctx)
	if err != nil {
		return nil, err
	}

	result := make([]model.CategorySummary, 0, len(rows))
	for _, row := range rows {
		result = append(result, model.CategorySummary{
			UUID:         row.Uuid.String(),
			Name:         row.Name,
			Slug:         row.Slug,
			Description:  row.Description,
			Icon:         row.Icon,
			ImageURL:     row.ImageUrl,
			DisplayOrder: int(row.DisplayOrder),
			IsActive:     row.IsActive,
		})
	}
	return result, nil
}

func (r *PostgresRepository) GetByUUID(ctx context.Context, id uuid.UUID) (*model.CategoryDetail, error) {
	row, err := r.q.GetProductCategoryByUUID(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return toCategoryDetail(row), nil
}

func (r *PostgresRepository) GetBySlug(ctx context.Context, slug string) (*model.CategoryDetail, error) {
	row, err := r.q.GetProductCategoryBySlug(ctx, slug)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return toCategoryDetail(row), nil
}

func (r *PostgresRepository) Create(ctx context.Context, input model.UpsertCategoryInput, slug string) (*model.CategoryDetail, error) {
	row, err := r.q.CreateProductCategory(ctx, db.CreateProductCategoryParams{
		Name:         input.Name,
		Slug:         slug,
		Description:  input.Description,
		Icon:         input.Icon,
		ImageUrl:     input.ImageURL,
		DisplayOrder: int32(input.DisplayOrder),
		IsActive:     input.IsActive,
	})
	if err != nil {
		return nil, err
	}
	return toCategoryDetail(row), nil
}

func (r *PostgresRepository) Update(ctx context.Context, id uuid.UUID, input model.UpsertCategoryInput, slug string) (*model.CategoryDetail, error) {
	row, err := r.q.UpdateProductCategory(ctx, db.UpdateProductCategoryParams{
		Uuid:         id,
		Name:         pgtype.Text{String: input.Name, Valid: input.Name != ""},
		Slug:         pgtype.Text{String: slug, Valid: slug != ""},
		Description:  pgtype.Text{String: input.Description, Valid: true},
		Icon:         pgtype.Text{String: input.Icon, Valid: true},
		ImageUrl:     pgtype.Text{String: input.ImageURL, Valid: true},
		DisplayOrder: pgtype.Int4{Int32: int32(input.DisplayOrder), Valid: true},
		IsActive:     pgtype.Bool{Bool: input.IsActive, Valid: true},
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return toCategoryDetail(row), nil
}

func (r *PostgresRepository) Delete(ctx context.Context, id uuid.UUID) error {
	return r.q.DeleteProductCategory(ctx, id)
}

func (r *PostgresRepository) CheckSlugExists(ctx context.Context, slug string, excludeUUID *uuid.UUID) (bool, error) {
	var exc pgtype.UUID
	if excludeUUID != nil {
		exc = pgtype.UUID{Bytes: *excludeUUID, Valid: true}
	}
	return r.q.CheckProductCategorySlugExists(ctx, db.CheckProductCategorySlugExistsParams{
		Slug:        slug,
		ExcludeUuid: exc,
	})
}

func toCategoryDetail(row db.ProductCategory) *model.CategoryDetail {
	return &model.CategoryDetail{
		ID:           row.ID,
		UUID:         row.Uuid.String(),
		Name:         row.Name,
		Slug:         row.Slug,
		Description:  row.Description,
		Icon:         row.Icon,
		ImageURL:     row.ImageUrl,
		DisplayOrder: int(row.DisplayOrder),
		IsActive:     row.IsActive,
		CreatedAt:    row.CreatedAt.Time,
		UpdatedAt:    row.UpdatedAt.Time,
	}
}

func textToPgtype(s *string) pgtype.Text {
	if s == nil || *s == "" {
		return pgtype.Text{Valid: false}
	}
	return pgtype.Text{String: *s, Valid: true}
}

func boolToPgtype(b *bool) pgtype.Bool {
	if b == nil {
		return pgtype.Bool{Valid: false}
	}
	return pgtype.Bool{Bool: *b, Valid: true}
}
