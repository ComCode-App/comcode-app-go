package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/backgrounds/model"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/backgrounds/thumbnail"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/authctx"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/entitlement"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/storage"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/testutil/pgtest"
	"github.com/google/uuid"
)

func TestMain(m *testing.M) { pgtest.Main(m) }

const (
	staticDoc   = `{"kind":"gradient","params":{"type":"linear","angle":90,"colors":["#ff0000","#0000ff"]}}`
	animatedDoc = `{"kind":"shader","shaderType":"mesh-gradient","params":{"colors":["#ff0000","#00ff00"],"speed":1}}`
)

func ownerOf(u pgtest.User) authctx.Principal {
	return authctx.Principal{
		UserInternal: u.ID, UserID: u.UUID,
		Account: authctx.Account{ID: u.AccountID, Type: authctx.AccountTypePersonal, Role: authctx.AccountRoleOwner},
	}
}

func newService(d *pgtest.DB) (*Service, *storage.Memory) {
	store := storage.NewMemory()
	svc := New(d.Queries)
	svc.SetFeatures(entitlement.New(d.Queries))
	svc.SetThumbnails(store)
	return svc, store
}

// TEC-947: the free plan (design.premium = false since migration 000068) cannot save an animated
// design; a paid plan can.
func TestPremiumDesignNeedsPaidPlan(t *testing.T) {
	d := pgtest.Open(t)
	ctx := context.Background()
	svc, _ := newService(d)
	user := d.CreateUser(t)
	p := ownerOf(user)
	ctx = authctx.WithPrincipal(ctx, p)

	saved, err := svc.Create(ctx, p, model.UpsertBackgroundInput{Name: "Static", Document: json.RawMessage(staticDoc)})
	if err != nil || saved.Premium {
		t.Fatalf("free plan, static design: %+v %v", saved, err)
	}
	_, err = svc.Create(ctx, p, model.UpsertBackgroundInput{Name: "Animated", Document: json.RawMessage(animatedDoc)})
	var fe *entitlement.FeatureError
	if !errors.Is(err, entitlement.ErrPlanLimitExceeded) || !errors.As(err, &fe) || fe.Feature != entitlement.FeatureDesignPremium {
		t.Fatalf("free plan, animated design: want design.premium refusal, got %v", err)
	}
	// Turning a saved static design into an animated one is refused too.
	if _, err := svc.Update(ctx, p, saved.UUID, model.UpsertBackgroundInput{Name: "x", Document: json.RawMessage(animatedDoc)}); !errors.Is(err, entitlement.ErrPlanLimitExceeded) {
		t.Fatalf("free plan, update to animated: %v", err)
	}

	if _, err := d.Pool.Exec(ctx, `
		INSERT INTO subscriptions (account_id, plan_id, status, source, starts_at, next_billing_at)
		SELECT $1, id, 'active', 'manual', now(), now() + interval '30 days' FROM plans WHERE slug = 'pro-monthly'`,
		user.AccountID); err != nil {
		t.Fatal(err)
	}
	animated, err := svc.Create(ctx, p, model.UpsertBackgroundInput{Name: "Animated", Document: json.RawMessage(animatedDoc)})
	if err != nil || !animated.Premium {
		t.Fatalf("paid plan, animated design: %+v %v", animated, err)
	}
	// Selection and the public page see the flag.
	usable, premium, err := svc.Selectable(ctx, user.AccountID, animated.UUID)
	if err != nil || !usable || !premium {
		t.Fatalf("Selectable: %v %v %v", usable, premium, err)
	}
}

// TEC-599: every saved design gets a static thumbnail; an edit replaces it and a delete removes it.
func TestThumbnailLifecycle(t *testing.T) {
	d := pgtest.Open(t)
	ctx := context.Background()
	svc, store := newService(d)
	user := d.CreateUser(t)
	p := ownerOf(user)

	saved, err := svc.Create(ctx, p, model.UpsertBackgroundInput{Name: "Static", Document: json.RawMessage(staticDoc)})
	if err != nil {
		t.Fatal(err)
	}
	key := thumbKey(t, d, saved.UUID)
	if saved.ThumbnailURL == nil || *saved.ThumbnailURL != store.PublicURL(key) {
		t.Fatalf("thumbnail url %v, key %q", saved.ThumbnailURL, key)
	}
	if ct, _, ok := store.GetMeta(key); !ok || ct != thumbnail.ContentType {
		t.Fatalf("thumbnail object: ok=%v content type %q", ok, ct)
	}
	pub, err := svc.PublicBackground(ctx, user.AccountID, saved.UUID)
	if err != nil || pub == nil || pub.ThumbnailURL != *saved.ThumbnailURL || pub.Premium {
		t.Fatalf("public background: %+v %v", pub, err)
	}

	other := strings.Replace(staticDoc, "#0000ff", "#00ff00", 1)
	if _, err := svc.Update(ctx, p, saved.UUID, model.UpsertBackgroundInput{Name: "Static", Document: json.RawMessage(other)}); err != nil {
		t.Fatal(err)
	}
	newKey := thumbKey(t, d, saved.UUID)
	if newKey == key {
		t.Fatal("an edit must render a new thumbnail")
	}
	if ok, _ := store.Exists(ctx, key); ok {
		t.Fatal("the previous thumbnail must be deleted")
	}

	if err := svc.Delete(ctx, p, saved.UUID); err != nil {
		t.Fatal(err)
	}
	if ok, _ := store.Exists(ctx, newKey); ok {
		t.Fatal("deleting the design must delete its thumbnail")
	}
}

// Designs saved before thumbnails existed get one from the startup backfill.
func TestBackfillThumbnails(t *testing.T) {
	d := pgtest.Open(t)
	ctx := context.Background()
	user := d.CreateUser(t)
	var id uuid.UUID
	if err := d.Pool.QueryRow(ctx, `
		INSERT INTO backgrounds (account_id, name, kind, document) VALUES ($1, 'old', 'gradient', $2)
		RETURNING uuid`, user.AccountID, staticDoc).Scan(&id); err != nil {
		t.Fatal(err)
	}
	svc, store := newService(d)
	svc.BackfillThumbnails(ctx)
	key := thumbKey(t, d, id)
	if ok, _ := store.Exists(ctx, key); !ok {
		t.Fatalf("backfill did not store %q", key)
	}
}

func thumbKey(t *testing.T, d *pgtest.DB, id uuid.UUID) string {
	t.Helper()
	key, err := d.Queries.GetBackgroundThumbnailKey(context.Background(), id)
	if err != nil || !key.Valid || key.String == "" {
		t.Fatalf("thumbnail key of %s: %+v %v", id, key, err)
	}
	return key.String
}
