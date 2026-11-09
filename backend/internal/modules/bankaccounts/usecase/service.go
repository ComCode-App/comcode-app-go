package usecase

import (
	"context"
	"errors"
	"strings"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/bankaccounts/model"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/bankaccounts/repository"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/authctx"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/iban"
	"github.com/google/uuid"
)

var (
	ErrNotFound              = errors.New("bank account not found")
	ErrAccountHolderRequired = errors.New("account holder is required")
	ErrInvalidIBAN           = errors.New("invalid IBAN format or checksum")
	ErrForbidden             = errors.New("account role cannot modify bank accounts")
)

// Bank accounts belong to the active account. Reads are open to every member; writes need an
// owner or admin role (docs/TENANCY_AND_ACCESS.md).

type Service struct {
	repo repository.Repository
}

func NewService(repo repository.Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) List(ctx context.Context, account authctx.Account) ([]model.UserBankAccountSummary, error) {
	return s.repo.ListByAccountID(ctx, account.ID)
}

func (s *Service) Get(ctx context.Context, id uuid.UUID, account authctx.Account) (*model.UserBankAccountSummary, error) {
	acc, err := s.repo.GetByUUID(ctx, id, account.ID)
	if err != nil {
		return nil, err
	}
	if acc == nil {
		return nil, ErrNotFound
	}
	return acc, nil
}

func (s *Service) Create(ctx context.Context, account authctx.Account, creatorUserID int64, input model.CreateUserBankAccountInput) (*model.UserBankAccountSummary, error) {
	if !account.CanWrite() {
		return nil, ErrForbidden
	}
	if strings.TrimSpace(input.AccountHolder) == "" {
		return nil, ErrAccountHolderRequired
	}

	normalizedIBAN := iban.Normalize(input.IBAN)
	if normalizedIBAN == "" {
		return nil, iban.ErrEmptyIBAN
	}

	// Validate IBAN checksum if it looks like an IBAN (>= 15 chars)
	if len(normalizedIBAN) >= 15 {
		if err := iban.Validate(normalizedIBAN); err != nil {
			return nil, ErrInvalidIBAN
		}
	}

	var bankID *int64
	if input.BankUUID != nil && *input.BankUUID != "" {
		if bUUID, err := uuid.Parse(*input.BankUUID); err == nil {
			bankID, _ = s.repo.FindBankIDByUUID(ctx, bUUID)
		}
	}

	// Auto-detect bank from IBAN if not explicitly provided
	if bankID == nil {
		bankCode := iban.ExtractBankCode(normalizedIBAN)
		if bankCode != "" {
			bankID, _ = s.repo.FindBankIDByCode(ctx, bankCode)
		}
	}

	// Auto extract account number if missing
	if input.AccountNumber == "" {
		input.AccountNumber = iban.ExtractAccountNumber(normalizedIBAN)
	}

	acc, err := s.repo.Create(ctx, account.ID, creatorUserID, bankID, input, normalizedIBAN)
	if err != nil {
		return nil, err
	}

	if input.IsPrimary {
		accUUID, _ := uuid.Parse(acc.UUID)
		_ = s.repo.ClearPrimary(ctx, account.ID, accUUID)
	}

	return acc, nil
}

func (s *Service) Update(ctx context.Context, id uuid.UUID, account authctx.Account, input model.UpdateUserBankAccountInput) (*model.UserBankAccountSummary, error) {
	if !account.CanWrite() {
		return nil, ErrForbidden
	}
	var normalizedIBAN *string
	var bankID *int64

	if input.IBAN != "" {
		norm := iban.Normalize(input.IBAN)
		if len(norm) >= 15 {
			if err := iban.Validate(norm); err != nil {
				return nil, ErrInvalidIBAN
			}
		}
		normalizedIBAN = &norm

		// Auto detect bank code if bank UUID not explicitly provided
		if input.BankUUID == nil || *input.BankUUID == "" {
			bankCode := iban.ExtractBankCode(norm)
			if bankCode != "" {
				bankID, _ = s.repo.FindBankIDByCode(ctx, bankCode)
			}
		}
	}

	if input.BankUUID != nil && *input.BankUUID != "" {
		if bUUID, err := uuid.Parse(*input.BankUUID); err == nil {
			bankID, _ = s.repo.FindBankIDByUUID(ctx, bUUID)
		}
	}

	acc, err := s.repo.Update(ctx, id, account.ID, bankID, input, normalizedIBAN)
	if err != nil {
		return nil, err
	}
	if acc == nil {
		return nil, ErrNotFound
	}

	if input.IsPrimary != nil && *input.IsPrimary {
		_ = s.repo.ClearPrimary(ctx, account.ID, id)
	}

	return acc, nil
}

func (s *Service) Delete(ctx context.Context, id uuid.UUID, account authctx.Account) error {
	if !account.CanWrite() {
		return ErrForbidden
	}
	return s.repo.Delete(ctx, id, account.ID)
}

func (s *Service) ToggleActive(ctx context.Context, id uuid.UUID, account authctx.Account) (*model.UserBankAccountSummary, error) {
	if !account.CanWrite() {
		return nil, ErrForbidden
	}
	acc, err := s.repo.ToggleActive(ctx, id, account.ID)
	if err != nil {
		return nil, err
	}
	if acc == nil {
		return nil, ErrNotFound
	}
	return acc, nil
}

func (s *Service) Reorder(ctx context.Context, account authctx.Account, uuids []string) error {
	if !account.CanWrite() {
		return ErrForbidden
	}
	for i, raw := range uuids {
		if id, err := uuid.Parse(raw); err == nil {
			_ = s.repo.UpdateOrder(ctx, id, account.ID, int32(i))
		}
	}
	return nil
}
