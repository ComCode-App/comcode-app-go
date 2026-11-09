package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/bankaccounts/model"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/authctx"
	"github.com/google/uuid"
)

// scopeRecorder records the account each repository call was scoped to.
type scopeRecorder struct {
	accounts []int64
	creators []int64
}

func (r *scopeRecorder) note(accountID int64) { r.accounts = append(r.accounts, accountID) }

func (r *scopeRecorder) ListByAccountID(_ context.Context, accountID int64) ([]model.UserBankAccountSummary, error) {
	r.note(accountID)
	return nil, nil
}

func (r *scopeRecorder) GetByUUID(_ context.Context, _ uuid.UUID, accountID int64) (*model.UserBankAccountSummary, error) {
	r.note(accountID)
	return nil, nil
}

func (r *scopeRecorder) Create(_ context.Context, accountID, creatorUserID int64, _ *int64, _ model.CreateUserBankAccountInput, _ string) (*model.UserBankAccountSummary, error) {
	r.note(accountID)
	r.creators = append(r.creators, creatorUserID)
	return &model.UserBankAccountSummary{UUID: uuid.NewString()}, nil
}

func (r *scopeRecorder) Update(_ context.Context, _ uuid.UUID, accountID int64, _ *int64, _ model.UpdateUserBankAccountInput, _ *string) (*model.UserBankAccountSummary, error) {
	r.note(accountID)
	return &model.UserBankAccountSummary{}, nil
}

func (r *scopeRecorder) Delete(_ context.Context, _ uuid.UUID, accountID int64) error {
	r.note(accountID)
	return nil
}

func (r *scopeRecorder) ClearPrimary(_ context.Context, accountID int64, _ uuid.UUID) error {
	r.note(accountID)
	return nil
}

func (r *scopeRecorder) ToggleActive(_ context.Context, _ uuid.UUID, accountID int64) (*model.UserBankAccountSummary, error) {
	r.note(accountID)
	return &model.UserBankAccountSummary{}, nil
}

func (r *scopeRecorder) UpdateOrder(_ context.Context, _ uuid.UUID, accountID int64, _ int32) error {
	r.note(accountID)
	return nil
}

func (r *scopeRecorder) FindBankIDByUUID(context.Context, uuid.UUID) (*int64, error) { return nil, nil }
func (r *scopeRecorder) FindBankIDByCode(context.Context, string) (*int64, error)    { return nil, nil }

func TestBankAccountWritesNeedAdminRole(t *testing.T) {
	repo := &scopeRecorder{}
	svc := NewService(repo)
	ctx := context.Background()
	member := authctx.Account{ID: 7, Role: authctx.AccountRoleMember}
	id := uuid.New()

	if _, err := svc.Create(ctx, member, 1, model.CreateUserBankAccountInput{AccountHolder: "Acme", IBAN: "TR330006100519786457841326"}); !errors.Is(err, ErrForbidden) {
		t.Errorf("member create: expected ErrForbidden, got %v", err)
	}
	if _, err := svc.Update(ctx, id, member, model.UpdateUserBankAccountInput{}); !errors.Is(err, ErrForbidden) {
		t.Errorf("member update: expected ErrForbidden, got %v", err)
	}
	if err := svc.Delete(ctx, id, member); !errors.Is(err, ErrForbidden) {
		t.Errorf("member delete: expected ErrForbidden, got %v", err)
	}
	if _, err := svc.ToggleActive(ctx, id, member); !errors.Is(err, ErrForbidden) {
		t.Errorf("member toggle: expected ErrForbidden, got %v", err)
	}
	if err := svc.Reorder(ctx, member, []string{id.String()}); !errors.Is(err, ErrForbidden) {
		t.Errorf("member reorder: expected ErrForbidden, got %v", err)
	}
	if len(repo.accounts) != 0 {
		t.Fatalf("forbidden calls reached the repository: %v", repo.accounts)
	}

	// Members can still read.
	if _, err := svc.List(ctx, member); err != nil {
		t.Errorf("member list: %v", err)
	}
}

func TestBankAccountCallsAreScopedToActiveAccount(t *testing.T) {
	repo := &scopeRecorder{}
	svc := NewService(repo)
	ctx := context.Background()
	admin := authctx.Account{ID: 42, Role: authctx.AccountRoleAdmin}

	if _, err := svc.Create(ctx, admin, 9, model.CreateUserBankAccountInput{AccountHolder: "Acme", IBAN: "TR330006100519786457841326", IsPrimary: true}); err != nil {
		t.Fatalf("create: %v", err)
	}
	for _, got := range repo.accounts {
		if got != admin.ID {
			t.Fatalf("repository call scoped to account %d, want %d", got, admin.ID)
		}
	}
	if len(repo.creators) != 1 || repo.creators[0] != 9 {
		t.Fatalf("creator not recorded: %v", repo.creators)
	}
}
