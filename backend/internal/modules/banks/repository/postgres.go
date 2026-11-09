package repository

import (
	"context"
	"errors"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/database/db"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/banks/model"
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

func (r *PostgresRepository) List(ctx context.Context, q *string, countryCode *string, isActive *bool, limit, offset int32) ([]model.BankSummary, error) {
	rows, err := r.queries.ListBanks(ctx, db.ListBanksParams{
		Q:           textToPg(q),
		CountryCode: textToPg(countryCode),
		IsActive:    boolToPg(isActive),
		LimitCount:  limit,
		OffsetCount: offset,
	})
	if err != nil {
		return nil, err
	}
	out := make([]model.BankSummary, len(rows))
	for i, row := range rows {
		out[i] = mapRowToSummary(row)
	}
	return out, nil
}

func (r *PostgresRepository) Count(ctx context.Context, q *string, countryCode *string, isActive *bool) (int64, error) {
	return r.queries.CountBanks(ctx, db.CountBanksParams{
		Q:           textToPg(q),
		CountryCode: textToPg(countryCode),
		IsActive:    boolToPg(isActive),
	})
}

func (r *PostgresRepository) ListActive(ctx context.Context) ([]model.BankSummary, error) {
	rows, err := r.queries.ListActiveBanks(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]model.BankSummary, len(rows))
	for i, row := range rows {
		out[i] = mapRowToSummary(row)
	}
	return out, nil
}

func (r *PostgresRepository) GetByUUID(ctx context.Context, id uuid.UUID) (*model.BankDetail, error) {
	row, err := r.queries.GetBankByUUID(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	detail := mapRowToDetail(row)
	return &detail, nil
}

func (r *PostgresRepository) GetBySlug(ctx context.Context, slug string) (*model.BankDetail, error) {
	row, err := r.queries.GetBankBySlug(ctx, slug)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	detail := mapRowToDetail(row)
	return &detail, nil
}

func (r *PostgresRepository) GetByCode(ctx context.Context, code string) (*model.BankDetail, error) {
	row, err := r.queries.GetBankByCode(ctx, code)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	detail := mapRowToDetail(row)
	return &detail, nil
}

func (r *PostgresRepository) Create(ctx context.Context, input model.UpsertBankInput, slug string) (*model.BankDetail, error) {
	countryCode := input.CountryCode
	if countryCode == "" {
		countryCode = "TR"
	}
	row, err := r.queries.CreateBank(ctx, db.CreateBankParams{
		Name:         input.Name,
		Slug:         slug,
		Code:         input.Code,
		CountryCode:  countryCode,
		BicSwift:     input.BicSwift,
		IconUrl:      input.IconURL,
		BrandColor:   input.BrandColor,
		IbanPattern:  input.IBANPattern,
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

func (r *PostgresRepository) Update(ctx context.Context, id uuid.UUID, input model.UpsertBankInput, slug string) (*model.BankDetail, error) {
	dispOrder := int32(input.DisplayOrder)
	row, err := r.queries.UpdateBank(ctx, db.UpdateBankParams{
		Uuid:         id,
		Name:         textToPg(&input.Name),
		Slug:         textToPg(&slug),
		Code:         textToPg(&input.Code),
		CountryCode:  textToPg(&input.CountryCode),
		BicSwift:     textToPg(&input.BicSwift),
		IconUrl:      textToPg(&input.IconURL),
		BrandColor:   textToPg(&input.BrandColor),
		IbanPattern:  textToPg(&input.IBANPattern),
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

func (r *PostgresRepository) UpdateIcon(ctx context.Context, id uuid.UUID, iconURL string) (*model.BankDetail, error) {
	row, err := r.queries.UpdateBankIcon(ctx, db.UpdateBankIconParams{
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
	return r.queries.DeleteBank(ctx, id)
}

func (r *PostgresRepository) CheckSlugExists(ctx context.Context, slug string, excludeUUID *uuid.UUID) (bool, error) {
	var exc pgtype.UUID
	if excludeUUID != nil {
		exc = pgtype.UUID{Bytes: *excludeUUID, Valid: true}
	}
	return r.queries.CheckBankSlugExists(ctx, db.CheckBankSlugExistsParams{
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

func mapRowToSummary(row db.Bank) model.BankSummary {
	return model.BankSummary{
		UUID:         row.Uuid.String(),
		Name:         row.Name,
		Slug:         row.Slug,
		Code:         row.Code,
		CountryCode:  row.CountryCode,
		BicSwift:     row.BicSwift,
		IconURL:      row.IconUrl,
		BrandColor:   row.BrandColor,
		IBANPattern:  row.IbanPattern,
		Description:  row.Description,
		DisplayOrder: int(row.DisplayOrder),
		IsActive:     row.IsActive,
	}
}

func mapRowToDetail(row db.Bank) model.BankDetail {
	return model.BankDetail{
		ID:           row.ID,
		UUID:         row.Uuid.String(),
		Name:         row.Name,
		Slug:         row.Slug,
		Code:         row.Code,
		CountryCode:  row.CountryCode,
		BicSwift:     row.BicSwift,
		IconURL:      row.IconUrl,
		BrandColor:   row.BrandColor,
		IBANPattern:  row.IbanPattern,
		Description:  row.Description,
		DisplayOrder: int(row.DisplayOrder),
		IsActive:     row.IsActive,
		CreatedAt:    row.CreatedAt.Time,
		UpdatedAt:    row.UpdatedAt.Time,
	}
}
