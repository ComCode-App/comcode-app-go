package repository

import (
	"context"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/cards/model"
	"github.com/google/uuid"
)

// Repository methods that take accountID are scoped to that account.
type Repository interface {
	ListAccountCards(ctx context.Context, accountID int64, q *string, isActive *bool, limit, offset int32) ([]model.CardSummary, int64, error)
	GetAccountCardByUUID(ctx context.Context, id uuid.UUID, accountID int64) (*model.CardDetail, error)
	GetCardBySmartCode(ctx context.Context, smartCode string) (*model.CardDetail, error)
	GetCardByActivationCode(ctx context.Context, code string) (*model.CardDetail, error)
	CreateCard(ctx context.Context, accountID, creatorUserID int64, smartCode string, input model.UpsertCardInput, profileID *int64) (*model.CardDetail, error)
	UpdateCard(ctx context.Context, id uuid.UUID, accountID int64, input model.UpsertCardInput, profileID *int64, setProfileID bool) (*model.CardDetail, error)
	ToggleCardLock(ctx context.Context, id uuid.UUID, accountID int64) (bool, error)
	ActivateCardByCode(ctx context.Context, code string, accountID, userID int64, name *string, profileID *int64) (*model.CardDetail, error)
	DeleteCard(ctx context.Context, id uuid.UUID, accountID int64) error
	CheckSmartCodeExists(ctx context.Context, code string, excludeUUID *uuid.UUID) (bool, error)
	GetProfileIDByUUID(ctx context.Context, profileUUID uuid.UUID, accountID int64) (*int64, error)
}
