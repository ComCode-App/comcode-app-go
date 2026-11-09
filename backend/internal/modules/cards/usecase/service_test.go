package usecase

import (
	"context"
	"errors"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/entitlement"
	"testing"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/cards/model"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/authctx"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type memCard struct {
	detail     model.CardDetail
	accountID  int64
	activation string
}

type memCardRepo struct {
	cards     map[uuid.UUID]*memCard
	activated []int64 // account ids passed to ActivateCardByCode
}

func newMemCardRepo() *memCardRepo { return &memCardRepo{cards: map[uuid.UUID]*memCard{}} }

func (r *memCardRepo) find(id uuid.UUID, accountID int64) *memCard {
	c, ok := r.cards[id]
	if !ok || c.accountID != accountID {
		return nil
	}
	return c
}

func (r *memCardRepo) ListAccountCards(_ context.Context, accountID int64, _ *string, _ *bool, _, _ int32) ([]model.CardSummary, int64, error) {
	var out []model.CardSummary
	for _, c := range r.cards {
		if c.accountID == accountID {
			out = append(out, model.CardSummary{UUID: c.detail.UUID})
		}
	}
	return out, int64(len(out)), nil
}

func (r *memCardRepo) GetAccountCardByUUID(_ context.Context, id uuid.UUID, accountID int64) (*model.CardDetail, error) {
	if c := r.find(id, accountID); c != nil {
		d := c.detail
		return &d, nil
	}
	return nil, nil
}

func (r *memCardRepo) GetCardBySmartCode(context.Context, string) (*model.CardDetail, error) {
	return nil, nil
}

func (r *memCardRepo) GetCardByActivationCode(_ context.Context, code string) (*model.CardDetail, error) {
	for _, c := range r.cards {
		if c.activation == code {
			d := c.detail
			return &d, nil
		}
	}
	return nil, nil
}

func (r *memCardRepo) CreateCard(_ context.Context, accountID, _ int64, smartCode string, input model.UpsertCardInput, _ *int64) (*model.CardDetail, error) {
	c := &memCard{detail: model.CardDetail{UUID: uuid.New(), Name: input.Name, SmartCode: smartCode}, accountID: accountID}
	r.cards[c.detail.UUID] = c
	d := c.detail
	return &d, nil
}

func (r *memCardRepo) UpdateCard(_ context.Context, id uuid.UUID, accountID int64, input model.UpsertCardInput, _ *int64, _ bool) (*model.CardDetail, error) {
	c := r.find(id, accountID)
	if c == nil {
		return nil, nil
	}
	c.detail.Name = input.Name
	d := c.detail
	return &d, nil
}

func (r *memCardRepo) ToggleCardLock(_ context.Context, id uuid.UUID, accountID int64) (bool, error) {
	c := r.find(id, accountID)
	if c == nil {
		return false, pgx.ErrNoRows
	}
	c.detail.IsLocked = !c.detail.IsLocked
	return c.detail.IsLocked, nil
}

func (r *memCardRepo) ActivateCardByCode(_ context.Context, code string, accountID, _ int64, _ *string, _ *int64) (*model.CardDetail, error) {
	for _, c := range r.cards {
		if c.activation == code {
			c.activation = ""
			c.accountID = accountID
			c.detail.IsActive = true
			r.activated = append(r.activated, accountID)
			d := c.detail
			return &d, nil
		}
	}
	return nil, pgx.ErrNoRows
}

func (r *memCardRepo) DeleteCard(_ context.Context, id uuid.UUID, accountID int64) error {
	if r.find(id, accountID) != nil {
		delete(r.cards, id)
	}
	return nil
}

func (r *memCardRepo) CheckSmartCodeExists(context.Context, string, *uuid.UUID) (bool, error) {
	return false, nil
}

func (r *memCardRepo) GetProfileIDByUUID(context.Context, uuid.UUID, int64) (*int64, error) {
	return nil, nil
}

func account(id int64, role string) authctx.Account {
	return authctx.Account{ID: id, Type: authctx.AccountTypeCompany, Role: role}
}

func TestCardsAreIsolatedByAccount(t *testing.T) {
	repo := newMemCardRepo()
	svc := NewService(repo, "https://app.test")
	svc.SetQuotas(&stubQuotas{})
	ctx := context.Background()

	card, err := svc.Create(ctx, account(1, authctx.AccountRoleOwner), 10, model.UpsertCardInput{Name: "Acme card"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	other := account(2, authctx.AccountRoleOwner)
	if _, err := svc.Get(ctx, card.UUID, other); !errors.Is(err, ErrCardNotFound) {
		t.Errorf("get from other account: expected ErrCardNotFound, got %v", err)
	}
	if _, err := svc.Update(ctx, card.UUID, other, model.UpsertCardInput{Name: "stolen"}); !errors.Is(err, ErrCardNotFound) {
		t.Errorf("update from other account: expected ErrCardNotFound, got %v", err)
	}
	if _, err := svc.ToggleLock(ctx, card.UUID, other); !errors.Is(err, ErrCardNotFound) {
		t.Errorf("lock from other account: expected ErrCardNotFound, got %v", err)
	}
	if err := svc.Delete(ctx, card.UUID, other); !errors.Is(err, ErrCardNotFound) {
		t.Errorf("delete from other account: expected ErrCardNotFound, got %v", err)
	}
	if items, _, _ := svc.List(ctx, other, nil, nil, 20, 0); len(items) != 0 {
		t.Errorf("list from other account returned %d cards", len(items))
	}
}

func TestCardWritesAndActivationNeedAdminRole(t *testing.T) {
	repo := newMemCardRepo()
	svc := NewService(repo, "https://app.test")
	svc.SetQuotas(&stubQuotas{})
	ctx := context.Background()

	card, err := svc.Create(ctx, account(1, authctx.AccountRoleAdmin), 10, model.UpsertCardInput{Name: "Team card"})
	if err != nil {
		t.Fatalf("admin create: %v", err)
	}
	member := account(1, authctx.AccountRoleMember)
	if _, err := svc.Get(ctx, card.UUID, member); err != nil {
		t.Errorf("member read: %v", err)
	}
	if _, err := svc.Create(ctx, member, 11, model.UpsertCardInput{Name: "x"}); !errors.Is(err, ErrForbidden) {
		t.Errorf("member create: expected ErrForbidden, got %v", err)
	}
	if _, err := svc.ToggleLock(ctx, card.UUID, member); !errors.Is(err, ErrForbidden) {
		t.Errorf("member lock: expected ErrForbidden, got %v", err)
	}
	if err := svc.Delete(ctx, card.UUID, member); !errors.Is(err, ErrForbidden) {
		t.Errorf("member delete: expected ErrForbidden, got %v", err)
	}

	repo.cards[uuid.New()] = &memCard{detail: model.CardDetail{UUID: uuid.New()}, accountID: 99, activation: "ACT-1"}
	if _, err := svc.Activate(ctx, member, 11, model.ActivateCardInput{ActivationCode: "act-1"}); !errors.Is(err, ErrForbidden) {
		t.Errorf("member activate: expected ErrForbidden, got %v", err)
	}
	if len(repo.activated) != 0 {
		t.Fatal("forbidden activation reached the repository")
	}
}

func TestActivationClaimsCardIntoActiveAccount(t *testing.T) {
	repo := newMemCardRepo()
	svc := NewService(repo, "https://app.test")
	svc.SetQuotas(&stubQuotas{})
	ctx := context.Background()

	repo.cards[uuid.New()] = &memCard{detail: model.CardDetail{UUID: uuid.New()}, accountID: 99, activation: "ACT-2"}
	owner := account(7, authctx.AccountRoleOwner)
	if _, err := svc.Activate(ctx, owner, 70, model.ActivateCardInput{ActivationCode: " act-2 "}); err != nil {
		t.Fatalf("activate: %v", err)
	}
	if len(repo.activated) != 1 || repo.activated[0] != owner.ID {
		t.Fatalf("card not activated into account %d: %v", owner.ID, repo.activated)
	}
	if _, err := svc.Activate(ctx, owner, 70, model.ActivateCardInput{ActivationCode: "ACT-2"}); !errors.Is(err, ErrInvalidActivation) {
		t.Fatalf("second activation: expected ErrInvalidActivation, got %v", err)
	}
}

func TestCardCreateAndActivationAreBlockedByQuota(t *testing.T) {
	repo := newMemCardRepo()
	svc := NewService(repo, "https://app.test")
	svc.SetQuotas(&stubQuotas{deny: map[string]bool{entitlement.FeatureCardsMax: true}})
	ctx := context.Background()
	owner := account(7, authctx.AccountRoleOwner)

	if _, err := svc.Create(ctx, owner, 70, model.UpsertCardInput{Name: "x"}); !errors.Is(err, entitlement.ErrPlanLimitExceeded) {
		t.Fatalf("create: expected ErrPlanLimitExceeded, got %v", err)
	}
	repo.cards[uuid.New()] = &memCard{detail: model.CardDetail{UUID: uuid.New()}, accountID: 99, activation: "ACT-9"}
	if _, err := svc.Activate(ctx, owner, 70, model.ActivateCardInput{ActivationCode: "ACT-9"}); !errors.Is(err, entitlement.ErrPlanLimitExceeded) {
		t.Fatalf("activate: expected ErrPlanLimitExceeded, got %v", err)
	}
	if len(repo.activated) != 0 || len(repo.cards) != 1 {
		t.Fatalf("denied writes reached the repository: activated=%v cards=%d", repo.activated, len(repo.cards))
	}
}
