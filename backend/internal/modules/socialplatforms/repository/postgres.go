package repository

import (
	"context"
	"errors"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/database/db"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/socialplatforms/model"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresRepository struct {
	pool    *pgxpool.Pool
	queries *db.Queries
}

func NewPostgres(pool *pgxpool.Pool, queries *db.Queries) *PostgresRepository {
	return &PostgresRepository{
		pool:    pool,
		queries: queries,
	}
}

func (r *PostgresRepository) List(ctx context.Context, q *string, category *string, isActive *bool, limit, offset int32) ([]model.SocialPlatformSummary, error) {
	rows, err := r.queries.ListSocialPlatforms(ctx, db.ListSocialPlatformsParams{
		Q:           textToPg(q),
		Category:    textToPg(category),
		IsActive:    boolToPg(isActive),
		LimitCount:  limit,
		OffsetCount: offset,
	})
	if err != nil {
		return nil, err
	}
	out := make([]model.SocialPlatformSummary, len(rows))
	for i, row := range rows {
		out[i] = mapRowToSummary(row)
	}
	return out, nil
}

func (r *PostgresRepository) Count(ctx context.Context, q *string, category *string, isActive *bool) (int64, error) {
	return r.queries.CountSocialPlatforms(ctx, db.CountSocialPlatformsParams{
		Q:        textToPg(q),
		Category: textToPg(category),
		IsActive: boolToPg(isActive),
	})
}

func (r *PostgresRepository) ListActive(ctx context.Context) ([]model.SocialPlatformSummary, error) {
	rows, err := r.queries.ListActiveSocialPlatforms(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]model.SocialPlatformSummary, len(rows))
	for i, row := range rows {
		out[i] = mapRowToSummary(row)
	}
	return out, nil
}

func (r *PostgresRepository) GetByUUID(ctx context.Context, id uuid.UUID) (*model.SocialPlatformDetail, error) {
	row, err := r.queries.GetSocialPlatformByUUID(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	detail := mapRowToDetail(row)
	return &detail, nil
}

func (r *PostgresRepository) GetBySlug(ctx context.Context, slug string) (*model.SocialPlatformDetail, error) {
	row, err := r.queries.GetSocialPlatformBySlug(ctx, slug)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	detail := mapRowToDetail(row)
	return &detail, nil
}

func (r *PostgresRepository) Create(ctx context.Context, input model.UpsertSocialPlatformInput, slug string) (*model.SocialPlatformDetail, error) {
	category := input.Category
	if category == "" {
		category = "social"
	}
	row, err := r.queries.CreateSocialPlatform(ctx, db.CreateSocialPlatformParams{
		Name:         input.Name,
		Slug:         slug,
		Category:     category,
		UrlPattern:   input.URLPattern,
		IconUrl:      input.IconURL,
		Placeholder:  input.Placeholder,
		Description:  input.Description,
		DisplayOrder: int32(input.DisplayOrder),
		IsActive:     input.IsActive,
	})
	if err != nil {
		return nil, err
	}
	detail := mapRowToDetail(row)
	return &detail, nil
}

func (r *PostgresRepository) Update(ctx context.Context, id uuid.UUID, input model.UpsertSocialPlatformInput, slug string) (*model.SocialPlatformDetail, error) {
	dispOrder := int32(input.DisplayOrder)
	row, err := r.queries.UpdateSocialPlatform(ctx, db.UpdateSocialPlatformParams{
		Uuid:         id,
		Name:         textToPg(&input.Name),
		Slug:         textToPg(&slug),
		Category:     textToPg(&input.Category),
		UrlPattern:   textToPg(&input.URLPattern),
		IconUrl:      textToPg(&input.IconURL),
		Placeholder:  textToPg(&input.Placeholder),
		Description:  textToPg(&input.Description),
		DisplayOrder: int4ToPg(&dispOrder),
		IsActive:     boolToPg(&input.IsActive),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	detail := mapRowToDetail(row)
	return &detail, nil
}

func (r *PostgresRepository) UpdateIcon(ctx context.Context, id uuid.UUID, iconURL string) (*model.SocialPlatformDetail, error) {
	row, err := r.queries.UpdateSocialPlatformIcon(ctx, db.UpdateSocialPlatformIconParams{
		Uuid:    id,
		IconUrl: iconURL,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	detail := mapRowToDetail(row)
	return &detail, nil
}

func (r *PostgresRepository) Delete(ctx context.Context, id uuid.UUID) error {
	return r.queries.DeleteSocialPlatform(ctx, id)
}

func (r *PostgresRepository) CheckSlugExists(ctx context.Context, slug string, excludeUUID *uuid.UUID) (bool, error) {
	var exc pgtype.UUID
	if excludeUUID != nil {
		exc = pgtype.UUID{Bytes: *excludeUUID, Valid: true}
	}
	return r.queries.CheckSocialPlatformSlugExists(ctx, db.CheckSocialPlatformSlugExistsParams{
		Slug:        slug,
		ExcludeUuid: exc,
	})
}

func textToPg(s *string) pgtype.Text {
	if s == nil || *s == "" {
		return pgtype.Text{Valid: false}
	}
	return pgtype.Text{String: *s, Valid: true}
}

func boolToPg(b *bool) pgtype.Bool {
	if b == nil {
		return pgtype.Bool{Valid: false}
	}
	return pgtype.Bool{Bool: *b, Valid: true}
}

func int4ToPg(i *int32) pgtype.Int4 {
	if i == nil {
		return pgtype.Int4{Valid: false}
	}
	return pgtype.Int4{Int32: *i, Valid: true}
}

func mapRowToSummary(row db.SocialPlatform) model.SocialPlatformSummary {
	return model.SocialPlatformSummary{
		UUID:         row.Uuid.String(),
		Name:         row.Name,
		Slug:         row.Slug,
		Category:     row.Category,
		URLPattern:   row.UrlPattern,
		IconURL:      row.IconUrl,
		Placeholder:  row.Placeholder,
		Description:  row.Description,
		DisplayOrder: int(row.DisplayOrder),
		IsActive:     row.IsActive,
	}
}

func mapRowToDetail(row db.SocialPlatform) model.SocialPlatformDetail {
	return model.SocialPlatformDetail{
		ID:           row.ID,
		UUID:         row.Uuid.String(),
		Name:         row.Name,
		Slug:         row.Slug,
		Category:     row.Category,
		URLPattern:   row.UrlPattern,
		IconURL:      row.IconUrl,
		Placeholder:  row.Placeholder,
		Description:  row.Description,
		DisplayOrder: int(row.DisplayOrder),
		IsActive:     row.IsActive,
		CreatedAt:    row.CreatedAt.Time,
		UpdatedAt:    row.UpdatedAt.Time,
	}
}
