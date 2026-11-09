package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/banks/model"
)

type Repository interface {
	List(ctx context.Context, q *string, countryCode *string, isActive *bool, limit, offset int32) ([]model.BankSummary, error)
	Count(ctx context.Context, q *string, countryCode *string, isActive *bool) (int64, error)
	ListActive(ctx context.Context) ([]model.BankSummary, error)
	GetByUUID(ctx context.Context, id uuid.UUID) (*model.BankDetail, error)
	GetBySlug(ctx context.Context, slug string) (*model.BankDetail, error)
	GetByCode(ctx context.Context, code string) (*model.BankDetail, error)
	Create(ctx context.Context, input model.UpsertBankInput, slug string) (*model.BankDetail, error)
	Update(ctx context.Context, id uuid.UUID, input model.UpsertBankInput, slug string) (*model.BankDetail, error)
	UpdateIcon(ctx context.Context, id uuid.UUID, iconURL string) (*model.BankDetail, error)
	Delete(ctx context.Context, id uuid.UUID) error
	CheckSlugExists(ctx context.Context, slug string, excludeUUID *uuid.UUID) (bool, error)
}
