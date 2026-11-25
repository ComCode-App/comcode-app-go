package adapters

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/database/db"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/bulkengine"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/jobscope"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/rbac"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

const (
	ResourceAppCards = "app.cards"

	ActionCardsLock   = "lock"
	ActionCardsUnlock = "unlock"
)

// maxCardBulkTargets bounds a card bulk job.
const maxCardBulkTargets = 5000

// errCardsBulkForbidden is the failure of a job whose actor is no longer an owner or admin of
// the job's account (or that runs without a job scope at all).
var errCardsBulkForbidden = errors.New("forbidden")

// CardCache drops resolver cache entries of a card (routecache.Cache).
type CardCache interface {
	InvalidateCard(ctx context.Context, cardUUID uuid.UUID)
}

// CardsAdapter locks and unlocks cards of one account in bulk (TEC-596). Cards are app data:
// every call reads the job's account and actor from jobscope and fails closed without them;
// the actor must still be an owner or admin of that account (the same role the single
// PATCH /v1/cards/{uuid}/toggle-lock needs). Each change drops the card's resolver cache
// entries, like the single toggle. Both actions are reversible: an item remembers the lock it
// had before.
type CardsAdapter struct {
	q     *db.Queries
	cache CardCache
}

func NewCards(q *db.Queries, cache CardCache) *CardsAdapter { return &CardsAdapter{q: q, cache: cache} }

var _ bulkengine.AccountScoped = (*CardsAdapter)(nil)

func (a *CardsAdapter) Resource() string { return ResourceAppCards }

func (a *CardsAdapter) BulkActions() []bulkengine.BulkActionDef {
	return []bulkengine.BulkActionDef{
		{
			ID: ActionCardsLock, LabelKey: "bulk.actions.cards.lock", Permission: rbac.PermAppCardsBulkLock,
			Reversible: true, ConfirmKey: "bulk.confirm.cards.lock",
		},
		{
			ID: ActionCardsUnlock, LabelKey: "bulk.actions.cards.unlock", Permission: rbac.PermAppCardsBulkUnlock,
			Reversible: true, ConfirmKey: "bulk.confirm.cards.unlock",
		},
	}
}

// writer returns the job's account after checking that the actor may still write in it.
func (a *CardsAdapter) writer(ctx context.Context) (int64, error) {
	scope, ok := jobscope.From(ctx)
	if !ok {
		return 0, errCardsBulkForbidden
	}
	m, err := a.q.GetAccountMembershipByID(ctx, db.GetAccountMembershipByIDParams{AccountID: scope.AccountID, UserID: scope.ActorID})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, errCardsBulkForbidden
		}
		return 0, err
	}
	if m.Role != db.AccountRoleOwner && m.Role != db.AccountRoleAdmin {
		return 0, errCardsBulkForbidden
	}
	return scope.AccountID, nil
}

// ValidateParams accepts no parameters: lock and unlock need none.
func (a *CardsAdapter) ValidateParams(ctx context.Context, action string, params json.RawMessage) error {
	if _, err := a.writer(ctx); err != nil {
		return err
	}
	if action != ActionCardsLock && action != ActionCardsUnlock {
		return fmt.Errorf("unknown action %q", action)
	}
	if p := strings.TrimSpace(string(params)); p != "" && p != "null" && p != "{}" {
		return fmt.Errorf("action %q takes no params", action)
	}
	return nil
}

// ResolveTargets returns the cards of the job's account: "ids" as given (each is checked again
// when applied), "query" with the card list filters q and is_active.
func (a *CardsAdapter) ResolveTargets(ctx context.Context, _ string, target bulkengine.BulkTarget) ([]string, error) {
	accountID, err := a.writer(ctx)
	if err != nil {
		return nil, err
	}
	switch target.Scope {
	case "ids":
		seen := map[uuid.UUID]bool{}
		out := make([]string, 0, len(target.IDs))
		for _, raw := range target.IDs {
			id, err := uuid.Parse(strings.TrimSpace(raw))
			if err != nil {
				return nil, fmt.Errorf("invalid card uuid %q", raw)
			}
			if !seen[id] {
				seen[id] = true
				out = append(out, id.String())
			}
		}
		if len(out) == 0 {
			return nil, fmt.Errorf("no cards selected")
		}
		if len(out) > maxCardBulkTargets {
			return nil, fmt.Errorf("too many cards")
		}
		return out, nil
	case "query":
		params := db.ListAccountCardUUIDsForBulkParams{
			AccountID: accountID, Q: textArg(target.Query["q"]), LimitCount: maxCardBulkTargets,
		}
		switch strings.TrimSpace(target.Query["is_active"]) {
		case "":
		case "true", "1":
			params.IsActive = pgtype.Bool{Bool: true, Valid: true}
		case "false", "0":
			params.IsActive = pgtype.Bool{Bool: false, Valid: true}
		default:
			return nil, fmt.Errorf("invalid is_active filter")
		}
		ids, err := a.q.ListAccountCardUUIDsForBulk(ctx, params)
		if err != nil {
			return nil, err
		}
		out := make([]string, len(ids))
		for i, id := range ids {
			out[i] = id.String()
		}
		return out, nil
	default:
		return nil, fmt.Errorf("unknown target scope %q", target.Scope)
	}
}

func (a *CardsAdapter) setLock(ctx context.Context, accountID int64, id uuid.UUID, locked bool) (bool, error) {
	was, err := a.q.SetCardLock(ctx, db.SetCardLockParams{Uuid: id, AccountID: accountID, IsLocked: locked})
	if err != nil {
		return false, err
	}
	if a.cache != nil {
		a.cache.InvalidateCard(ctx, id)
	}
	return was, nil
}

func (a *CardsAdapter) ApplyItem(ctx context.Context, action, entityUUID string) (bulkengine.BulkItemResult, error) {
	fail := func(msg string) (bulkengine.BulkItemResult, error) {
		return bulkengine.BulkItemResult{EntityUUID: entityUUID, EntityType: "card", OK: false, Error: msg}, nil
	}
	accountID, err := a.writer(ctx)
	if err != nil {
		return fail(err.Error())
	}
	id, err := uuid.Parse(entityUUID)
	if err != nil {
		return fail("not_found")
	}
	var locked bool
	switch action {
	case ActionCardsLock:
		locked = true
	case ActionCardsUnlock:
		locked = false
	default:
		return fail("unknown action")
	}
	was, err := a.setLock(ctx, accountID, id, locked)
	if errors.Is(err, pgx.ErrNoRows) {
		// Not a live card of this account: other accounts' cards are never touched.
		return fail("not_found")
	}
	if err != nil {
		return bulkengine.BulkItemResult{}, err
	}
	return bulkengine.BulkItemResult{
		EntityUUID: entityUUID, EntityType: "card", OK: true, Op: "update",
		Previous: map[string]any{"is_locked": was},
	}, nil
}

// RevertItem puts back the lock the card had before the job.
func (a *CardsAdapter) RevertItem(ctx context.Context, _ string, entityUUID string, previous map[string]any) error {
	accountID, err := a.writer(ctx)
	if err != nil {
		return err
	}
	id, err := uuid.Parse(entityUUID)
	if err != nil {
		return err
	}
	was, ok := previous["is_locked"].(bool)
	if !ok {
		return fmt.Errorf("card %s: no previous lock state", entityUUID)
	}
	if _, err := a.setLock(ctx, accountID, id, was); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("card %s: not found", entityUUID)
		}
		return err
	}
	return nil
}
