package adapters_test

// Bulk lock/unlock of cards (TEC-596) against Postgres: ids and query targets stay in the job's
// account, the actor must be an owner or admin there, every change drops the card's resolver
// cache, and rollback restores each card's previous lock state.

import (
	"context"
	"slices"
	"sync"
	"testing"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/bulkengine"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/bulkengine/adapters"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/jobscope"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/testutil/pgtest"
	"github.com/google/uuid"
)

type cacheSpy struct {
	mu  sync.Mutex
	ids []uuid.UUID
}

func (c *cacheSpy) InvalidateCard(_ context.Context, id uuid.UUID) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.ids = append(c.ids, id)
}

func (c *cacheSpy) take() []uuid.UUID {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := c.ids
	c.ids = nil
	return out
}

func locked(t *testing.T, d *pgtest.DB, id uuid.UUID) bool {
	t.Helper()
	var v bool
	if err := d.Pool.QueryRow(context.Background(), `SELECT is_locked FROM nfc_cards WHERE uuid = $1`, id).Scan(&v); err != nil {
		t.Fatal(err)
	}
	return v
}

func TestIntegrationBulkCardsLock(t *testing.T) {
	d := pgtest.Open(t)
	spy := &cacheSpy{}
	a := adapters.NewCards(d.Queries, spy)

	owner := d.CreateUser(t)
	other := d.CreateUser(t)
	c1, c2, c3 := d.CreateCard(t, owner), d.CreateCard(t, owner), d.CreateCard(t, owner)
	foreign := d.CreateCard(t, other)
	if _, err := d.Pool.Exec(context.Background(), `UPDATE nfc_cards SET is_locked = true WHERE id = $1`, c2.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := d.Pool.Exec(context.Background(), `UPDATE nfc_cards SET is_active = false, name = 'Pasif kart' WHERE id = $1`, c3.ID); err != nil {
		t.Fatal(err)
	}

	ctx := jobscope.With(context.Background(), jobscope.Scope{AccountID: owner.AccountID, ActorID: owner.ID})

	// --- Fail closed: no scope, or an actor who is not a member of the account.
	noScope := context.Background()
	stranger := jobscope.With(context.Background(), jobscope.Scope{AccountID: owner.AccountID, ActorID: other.ID})
	for name, c := range map[string]context.Context{"no scope": noScope, "stranger": stranger} {
		if err := a.ValidateParams(c, adapters.ActionCardsLock, nil); err == nil {
			t.Fatalf("%s: params accepted", name)
		}
		if _, err := a.ResolveTargets(c, adapters.ActionCardsLock, bulkengine.BulkTarget{Scope: "query"}); err == nil {
			t.Fatalf("%s: targets resolved", name)
		}
		res, err := a.ApplyItem(c, adapters.ActionCardsLock, c1.UUID.String())
		if err != nil || res.OK {
			t.Fatalf("%s: applied %+v %v", name, res, err)
		}
	}
	if locked(t, d, c1.UUID) {
		t.Fatal("card locked without access")
	}
	if err := a.ValidateParams(ctx, "delete", nil); err == nil {
		t.Fatal("unknown action accepted")
	}
	if err := a.ValidateParams(ctx, adapters.ActionCardsLock, []byte(`{"x":1}`)); err == nil {
		t.Fatal("params accepted")
	}
	if err := a.ValidateParams(ctx, adapters.ActionCardsLock, []byte(`{}`)); err != nil {
		t.Fatal(err)
	}

	// --- Query targets: only this account's live cards, with the list filters.
	all, err := a.ResolveTargets(ctx, adapters.ActionCardsLock, bulkengine.BulkTarget{Scope: "query"})
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 3 || slices.Contains(all, foreign.UUID.String()) {
		t.Fatalf("query targets: %v", all)
	}
	inactive, err := a.ResolveTargets(ctx, adapters.ActionCardsLock, bulkengine.BulkTarget{Scope: "query", Query: map[string]string{"is_active": "false"}})
	if err != nil || len(inactive) != 1 || inactive[0] != c3.UUID.String() {
		t.Fatalf("is_active filter: %v %v", inactive, err)
	}
	byName, err := a.ResolveTargets(ctx, adapters.ActionCardsLock, bulkengine.BulkTarget{Scope: "query", Query: map[string]string{"q": "pasif"}})
	if err != nil || len(byName) != 1 || byName[0] != c3.UUID.String() {
		t.Fatalf("q filter: %v %v", byName, err)
	}
	if _, err := a.ResolveTargets(ctx, adapters.ActionCardsLock, bulkengine.BulkTarget{Scope: "ids", IDs: []string{"nope"}}); err == nil {
		t.Fatal("invalid uuid accepted")
	}
	ids, err := a.ResolveTargets(ctx, adapters.ActionCardsLock, bulkengine.BulkTarget{Scope: "ids", IDs: []string{c1.UUID.String(), c1.UUID.String(), foreign.UUID.String()}})
	if err != nil || len(ids) != 2 {
		t.Fatalf("ids dedupe: %v %v", ids, err)
	}

	// --- Apply: another account's card fails per item and stays as it was.
	res, err := a.ApplyItem(ctx, adapters.ActionCardsLock, foreign.UUID.String())
	if err != nil || res.OK || res.Error != "not_found" {
		t.Fatalf("foreign card: %+v %v", res, err)
	}
	if locked(t, d, foreign.UUID) {
		t.Fatal("foreign card locked")
	}
	spy.take()

	r1, err := a.ApplyItem(ctx, adapters.ActionCardsLock, c1.UUID.String())
	if err != nil || !r1.OK || r1.Previous["is_locked"] != false || r1.EntityType != "card" {
		t.Fatalf("lock c1: %+v %v", r1, err)
	}
	r2, err := a.ApplyItem(ctx, adapters.ActionCardsLock, c2.UUID.String())
	if err != nil || !r2.OK || r2.Previous["is_locked"] != true {
		t.Fatalf("lock c2: %+v %v", r2, err)
	}
	if !locked(t, d, c1.UUID) || !locked(t, d, c2.UUID) {
		t.Fatal("cards not locked")
	}
	if got := spy.take(); len(got) != 2 || got[0] != c1.UUID || got[1] != c2.UUID {
		t.Fatalf("cache invalidations: %v", got)
	}

	// --- Rollback: each card gets its own previous state back, cache dropped again.
	if err := a.RevertItem(ctx, "card", r1.EntityUUID, r1.Previous); err != nil {
		t.Fatal(err)
	}
	if err := a.RevertItem(ctx, "card", r2.EntityUUID, r2.Previous); err != nil {
		t.Fatal(err)
	}
	if locked(t, d, c1.UUID) || !locked(t, d, c2.UUID) {
		t.Fatal("rollback did not restore lock states")
	}
	if got := spy.take(); len(got) != 2 {
		t.Fatalf("rollback invalidations: %v", got)
	}
	if err := a.RevertItem(stranger, "card", r1.EntityUUID, map[string]any{"is_locked": true}); err == nil {
		t.Fatal("stranger rolled back")
	}

	// --- Unlock, and a deleted card is not found.
	u, err := a.ApplyItem(ctx, adapters.ActionCardsUnlock, c2.UUID.String())
	if err != nil || !u.OK || locked(t, d, c2.UUID) {
		t.Fatalf("unlock: %+v %v", u, err)
	}
	if _, err := d.Pool.Exec(context.Background(), `UPDATE nfc_cards SET deleted_at = now() WHERE id = $1`, c1.ID); err != nil {
		t.Fatal(err)
	}
	if del, err := a.ApplyItem(ctx, adapters.ActionCardsLock, c1.UUID.String()); err != nil || del.OK {
		t.Fatalf("deleted card: %+v %v", del, err)
	}
}
