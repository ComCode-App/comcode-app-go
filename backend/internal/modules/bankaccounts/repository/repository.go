package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/bankaccounts/model"
)

// Repository is scoped to an account: rows outside accountID behave as missing.
type Repository interface {
	ListByAccountID(ctx context.Context, accountID int64) ([]model.UserBankAccountSummary, error)
	GetByUUID(ctx context.Context, id uuid.UUID, accountID int64) (*model.UserBankAccountSummary, error)
	Create(ctx context.Context, accountID, creatorUserID int64, bankID *int64, input model.CreateUserBankAccountInput, normalizedIBAN string) (*model.UserBankAccountSummary, error)
	Update(ctx context.Context, id uuid.UUID, accountID int64, bankID *int64, input model.UpdateUserBankAccountInput, normalizedIBAN *string) (*model.UserBankAccountSummary, error)
	Delete(ctx context.Context, id uuid.UUID, accountID int64) error
	ClearPrimary(ctx context.Context, accountID int64, exceptUUID uuid.UUID) error
	ToggleActive(ctx context.Context, id uuid.UUID, accountID int64) (*model.UserBankAccountSummary, error)
	UpdateOrder(ctx context.Context, id uuid.UUID, accountID int64, order int32) error
	FindBankIDByUUID(ctx context.Context, bankUUID uuid.UUID) (*int64, error)
	FindBankIDByCode(ctx context.Context, code string) (*int64, error)
}
