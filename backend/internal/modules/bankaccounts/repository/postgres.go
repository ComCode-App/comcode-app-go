package repository

import (
	"context"
	"errors"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/database/db"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/bankaccounts/model"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/iban"
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

func (r *PostgresRepository) ListByAccountID(ctx context.Context, accountID int64) ([]model.UserBankAccountSummary, error) {
	rows, err := r.queries.ListUserBankAccounts(ctx, accountID)
	if err != nil {
		return nil, err
	}
	out := make([]model.UserBankAccountSummary, len(rows))
	for i, row := range rows {
		out[i] = mapListRowToSummary(row)
	}
	return out, nil
}

func (r *PostgresRepository) GetByUUID(ctx context.Context, id uuid.UUID, accountID int64) (*model.UserBankAccountSummary, error) {
	row, err := r.queries.GetUserBankAccountByUUID(ctx, db.GetUserBankAccountByUUIDParams{
		Uuid:      id,
		AccountID: accountID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	summary := mapGetRowToSummary(row)
	return &summary, nil
}

func (r *PostgresRepository) Create(ctx context.Context, accountID, creatorUserID int64, bankID *int64, input model.CreateUserBankAccountInput, normalizedIBAN string) (*model.UserBankAccountSummary, error) {
	var bankIDPg pgtype.Int8
	if bankID != nil {
		bankIDPg = pgtype.Int8{Int64: *bankID, Valid: true}
	}

	currency := input.Currency
	if currency == "" {
		currency = "TRY"
	}

	isActive := true
	if input.IsActive != nil {
		isActive = *input.IsActive
	}

	created, err := r.queries.CreateUserBankAccount(ctx, db.CreateUserBankAccountParams{
		AccountID:     accountID,
		UserID:        pgtype.Int8{Int64: creatorUserID, Valid: true},
		BankID:        bankIDPg,
		AccountHolder: input.AccountHolder,
		Iban:          normalizedIBAN,
		AccountNumber: input.AccountNumber,
		BranchCode:    input.BranchCode,
		Currency:      currency,
		Title:         input.Title,
		IsPrimary:     input.IsPrimary,
		IsActive:      isActive,
		DisplayOrder:  0,
	})
	if err != nil {
		return nil, err
	}

	return r.GetByUUID(ctx, created.Uuid, accountID)
}

func (r *PostgresRepository) Update(ctx context.Context, id uuid.UUID, accountID int64, bankID *int64, input model.UpdateUserBankAccountInput, normalizedIBAN *string) (*model.UserBankAccountSummary, error) {
	var bankIDPg pgtype.Int8
	if bankID != nil {
		bankIDPg = pgtype.Int8{Int64: *bankID, Valid: true}
	}

	var dispOrder *int32
	if input.DisplayOrder != nil {
		d := int32(*input.DisplayOrder)
		dispOrder = &d
	}

	var holder *string
	if input.AccountHolder != "" {
		holder = &input.AccountHolder
	}

	var accNum *string
	if input.AccountNumber != "" {
		accNum = &input.AccountNumber
	}

	var branch *string
	if input.BranchCode != "" {
		branch = &input.BranchCode
	}

	var curr *string
	if input.Currency != "" {
		curr = &input.Currency
	}

	var title *string
	if input.Title != "" {
		title = &input.Title
	}

	_, err := r.queries.UpdateUserBankAccount(ctx, db.UpdateUserBankAccountParams{
		Uuid:          id,
		AccountID:     accountID,
		BankID:        bankIDPg,
		AccountHolder: textToPg(holder),
		Iban:          textToPg(normalizedIBAN),
		AccountNumber: textToPg(accNum),
		BranchCode:    textToPg(branch),
		Currency:      textToPg(curr),
		Title:         textToPg(title),
		IsPrimary:     boolToPg(input.IsPrimary),
		IsActive:      boolToPg(input.IsActive),
		DisplayOrder:  int4ToPg(dispOrder),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	return r.GetByUUID(ctx, id, accountID)
}

func (r *PostgresRepository) Delete(ctx context.Context, id uuid.UUID, accountID int64) error {
	return r.queries.DeleteUserBankAccount(ctx, db.DeleteUserBankAccountParams{
		Uuid:      id,
		AccountID: accountID,
	})
}

func (r *PostgresRepository) ClearPrimary(ctx context.Context, accountID int64, exceptUUID uuid.UUID) error {
	return r.queries.ClearPrimaryUserBankAccounts(ctx, db.ClearPrimaryUserBankAccountsParams{
		AccountID: accountID,
		Uuid:      exceptUUID,
	})
}

func (r *PostgresRepository) ToggleActive(ctx context.Context, id uuid.UUID, accountID int64) (*model.UserBankAccountSummary, error) {
	_, err := r.queries.ToggleActiveUserBankAccount(ctx, db.ToggleActiveUserBankAccountParams{
		Uuid:      id,
		AccountID: accountID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return r.GetByUUID(ctx, id, accountID)
}

func (r *PostgresRepository) UpdateOrder(ctx context.Context, id uuid.UUID, accountID int64, order int32) error {
	return r.queries.UpdateUserBankAccountOrder(ctx, db.UpdateUserBankAccountOrderParams{
		Uuid:         id,
		AccountID:    accountID,
		DisplayOrder: order,
	})
}

func (r *PostgresRepository) FindBankIDByUUID(ctx context.Context, bankUUID uuid.UUID) (*int64, error) {
	b, err := r.queries.GetBankByUUID(ctx, bankUUID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &b.ID, nil
}

func (r *PostgresRepository) FindBankIDByCode(ctx context.Context, code string) (*int64, error) {
	b, err := r.queries.GetBankByCode(ctx, code)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &b.ID, nil
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

func mapListRowToSummary(row db.ListUserBankAccountsRow) model.UserBankAccountSummary {
	var bankID *int64
	if row.BankID.Valid {
		bankID = &row.BankID.Int64
	}

	var bankName, bankSlug, bankCode, bankIconURL, bankBrandColor, bankBicSwift string
	if row.BankName.Valid {
		bankName = row.BankName.String
	}
	if row.BankSlug.Valid {
		bankSlug = row.BankSlug.String
	}
	if row.BankCode.Valid {
		bankCode = row.BankCode.String
	}
	if row.BankIconUrl.Valid {
		bankIconURL = row.BankIconUrl.String
	}
	if row.BankBrandColor.Valid {
		bankBrandColor = row.BankBrandColor.String
	}
	if row.BankBicSwift.Valid {
		bankBicSwift = row.BankBicSwift.String
	}

	return model.UserBankAccountSummary{
		UUID:           row.Uuid.String(),
		BankID:         bankID,
		BankName:       bankName,
		BankSlug:       bankSlug,
		BankCode:       bankCode,
		BankIconURL:    bankIconURL,
		BankBrandColor: bankBrandColor,
		BankBicSwift:   bankBicSwift,
		AccountHolder:  row.AccountHolder,
		IBAN:           row.Iban,
		FormattedIBAN:  iban.Format(row.Iban),
		AccountNumber:  row.AccountNumber,
		BranchCode:     row.BranchCode,
		Currency:       row.Currency,
		Title:          row.Title,
		IsPrimary:      row.IsPrimary,
		IsActive:       row.IsActive,
		DisplayOrder:   int(row.DisplayOrder),
		CreatedAt:      row.CreatedAt.Time,
		UpdatedAt:      row.UpdatedAt.Time,
	}
}

func mapGetRowToSummary(row db.GetUserBankAccountByUUIDRow) model.UserBankAccountSummary {
	var bankID *int64
	if row.BankID.Valid {
		bankID = &row.BankID.Int64
	}

	var bankName, bankSlug, bankCode, bankIconURL, bankBrandColor, bankBicSwift string
	if row.BankName.Valid {
		bankName = row.BankName.String
	}
	if row.BankSlug.Valid {
		bankSlug = row.BankSlug.String
	}
	if row.BankCode.Valid {
		bankCode = row.BankCode.String
	}
	if row.BankIconUrl.Valid {
		bankIconURL = row.BankIconUrl.String
	}
	if row.BankBrandColor.Valid {
		bankBrandColor = row.BankBrandColor.String
	}
	if row.BankBicSwift.Valid {
		bankBicSwift = row.BankBicSwift.String
	}

	return model.UserBankAccountSummary{
		UUID:           row.Uuid.String(),
		BankID:         bankID,
		BankName:       bankName,
		BankSlug:       bankSlug,
		BankCode:       bankCode,
		BankIconURL:    bankIconURL,
		BankBrandColor: bankBrandColor,
		BankBicSwift:   bankBicSwift,
		AccountHolder:  row.AccountHolder,
		IBAN:           row.Iban,
		FormattedIBAN:  iban.Format(row.Iban),
		AccountNumber:  row.AccountNumber,
		BranchCode:     row.BranchCode,
		Currency:       row.Currency,
		Title:          row.Title,
		IsPrimary:      row.IsPrimary,
		IsActive:       row.IsActive,
		DisplayOrder:   int(row.DisplayOrder),
		CreatedAt:      row.CreatedAt.Time,
		UpdatedAt:      row.UpdatedAt.Time,
	}
}
