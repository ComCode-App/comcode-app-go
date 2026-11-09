package usecase

// Postgres integration tests (TEC-536). Skipped unless TEST_DATABASE_URL is set; see
// internal/testutil/pgtest.

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/orders/model"
	ordersRepo "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/orders/repository"
	productsRepo "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/products/repository"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/authctx"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/testutil/pgtest"
)

func TestMain(m *testing.M) { pgtest.Main(m) }

type orderFixture struct {
	d     *pgtest.DB
	svc   *Service
	repo  *ordersRepo.PostgresRepository
	order *model.OrderDetail
}

// newOrderFixture places a two-item order for a fresh user's personal account.
func newOrderFixture(t *testing.T) orderFixture {
	t.Helper()
	d := pgtest.Open(t)
	ctx := context.Background()
	user := d.CreateUser(t)
	product := d.CreateProduct(t, 100)

	repo := ordersRepo.NewPostgres(d.Pool, d.Queries)
	svc := NewService(repo, productsRepo.NewPostgres(d.Queries), nil, nil, "https://comcode.test")
	account := authctx.Account{ID: user.AccountID, Role: authctx.AccountRoleOwner}
	ord, err := svc.CreateForAccount(ctx, account, user.ID, model.CreateOrderInput{
		Items: []model.CreateOrderItemInput{
			{ProductUUID: product.String(), Quantity: 1, RecipientName: "Ada"},
			{ProductUUID: product.String(), Quantity: 1, RecipientName: "Grace"},
		},
	})
	if err != nil {
		t.Fatalf("create order: %v", err)
	}
	if len(ord.Items) != 2 || ord.AccountID != user.AccountID {
		t.Fatalf("order = %d items, account %d; want 2 items for account %d", len(ord.Items), ord.AccountID, user.AccountID)
	}
	if ord.OrderType != "bulk" {
		t.Fatalf("multi-item order type = %q, want bulk", ord.OrderType)
	}
	return orderFixture{d: d, svc: svc, repo: repo, order: ord}
}

func (f orderFixture) count(t *testing.T, query string) int {
	t.Helper()
	var n int
	if err := f.d.Pool.QueryRow(context.Background(), query, f.order.ID).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func cardUUIDs(ord *model.OrderDetail) map[string]string {
	out := map[string]string{}
	for _, it := range ord.Items {
		if it.CardUUID != nil {
			out[it.UUID] = *it.CardUUID
		}
	}
	return out
}

func TestIntegrationProvisionOrderIsIdempotent(t *testing.T) {
	f := newOrderFixture(t)
	ctx := context.Background()
	orderUUID := uuid.MustParse(f.order.UUID)

	first, err := f.svc.ProvisionOrder(ctx, orderUUID)
	if err != nil {
		t.Fatalf("provision: %v", err)
	}
	for _, it := range first.Items {
		if it.ProvisioningStatus != "provisioned" || it.CardUUID == nil {
			t.Fatalf("item %s: status %q card %v", it.UUID, it.ProvisioningStatus, it.CardUUID)
		}
	}
	if first.FulfillmentStatus != "in_production" {
		t.Fatalf("fulfillment = %q, want in_production", first.FulfillmentStatus)
	}
	cards := cardUUIDs(first)

	const cardsQ = `SELECT COUNT(*) FROM nfc_cards WHERE order_id = $1`
	const routesQ = `SELECT COUNT(*) FROM routes r JOIN nfc_cards c ON c.id = r.card_id
		WHERE c.order_id = $1 AND r.route_type = 'card_route' AND r.status = 'reserved' AND r.is_immutable
		AND r.path = 'c/' || c.smart_code AND r.account_id = c.account_id`
	if n := f.count(t, cardsQ); n != 2 {
		t.Fatalf("cards after first provision = %d, want 2", n)
	}
	if n := f.count(t, routesQ); n != 2 {
		t.Fatalf("reserved card routes = %d, want 2", n)
	}

	// Second run (e.g. a repeated "paid" webhook) must not mint more cards or routes.
	second, err := f.svc.ProvisionOrder(ctx, orderUUID)
	if err != nil {
		t.Fatalf("second provision: %v", err)
	}
	if n := f.count(t, cardsQ); n != 2 {
		t.Fatalf("cards after second provision = %d, want 2", n)
	}
	if n := f.count(t, `SELECT COUNT(*) FROM routes r JOIN nfc_cards c ON c.id = r.card_id WHERE c.order_id = $1`); n != 2 {
		t.Fatalf("routes after second provision = %d, want 2", n)
	}
	for item, card := range cardUUIDs(second) {
		if cards[item] != card {
			t.Fatalf("item %s card changed from %s to %s", item, cards[item], card)
		}
	}

	// Single-item retry of an already provisioned item returns it unchanged.
	itemUUID := uuid.MustParse(first.Items[0].UUID)
	item, err := f.svc.ProvisionSingleItem(ctx, orderUUID, itemUUID)
	if err != nil {
		t.Fatalf("single item: %v", err)
	}
	if item.CardUUID == nil || *item.CardUUID != cards[item.UUID] {
		t.Fatalf("single item retry changed card: %v", item.CardUUID)
	}
	if n := f.count(t, cardsQ); n != 2 {
		t.Fatalf("cards after single-item retry = %d, want 2", n)
	}
}

func TestIntegrationCancelRetiresCardRoutes(t *testing.T) {
	f := newOrderFixture(t)
	ctx := context.Background()
	orderUUID := uuid.MustParse(f.order.UUID)
	if _, err := f.svc.ProvisionOrder(ctx, orderUUID); err != nil {
		t.Fatalf("provision: %v", err)
	}

	if _, err := f.svc.UpdateStatus(ctx, orderUUID, model.UpdateOrderStatusInput{Status: "cancelled"}); err != nil {
		t.Fatalf("cancel: %v", err)
	}
	retired := f.count(t, `SELECT COUNT(*) FROM routes r JOIN nfc_cards c ON c.id = r.card_id
		WHERE c.order_id = $1 AND r.status = 'retired' AND r.deleted_at IS NULL`)
	if retired != 2 {
		t.Fatalf("retired routes = %d, want 2", retired)
	}
	if n := f.count(t, `SELECT COUNT(*) FROM nfc_cards WHERE order_id = $1 AND activation_code IS NOT NULL`); n != 0 {
		t.Fatalf("cards with activation codes after cancel = %d, want 0", n)
	}

	// Retirement is one-way and already-retired routes are not returned again.
	again, err := f.repo.RetireCardRoutes(ctx, f.order.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(again) != 0 {
		t.Fatalf("second retire returned %d routes, want 0", len(again))
	}
	if _, err := f.svc.UpdateStatus(ctx, orderUUID, model.UpdateOrderStatusInput{Status: "in_production"}); err != nil {
		t.Fatalf("reopen: %v", err)
	}
	if n := f.count(t, `SELECT COUNT(*) FROM routes r JOIN nfc_cards c ON c.id = r.card_id
		WHERE c.order_id = $1 AND r.status <> 'retired'`); n != 0 {
		t.Fatalf("reopening revived %d routes", n)
	}

	// A cancelled order can no longer be provisioned.
	if _, err := f.svc.UpdateStatus(ctx, orderUUID, model.UpdateOrderStatusInput{Status: "cancelled"}); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.ProvisionOrder(ctx, orderUUID); !errors.Is(err, ErrOrderClosed) {
		t.Fatalf("provision cancelled order: %v, want ErrOrderClosed", err)
	}
}

func TestIntegrationRefundRetiresOnlyThatOrder(t *testing.T) {
	a := newOrderFixture(t)
	b := newOrderFixture(t)
	ctx := context.Background()
	for _, f := range []orderFixture{a, b} {
		if _, err := f.svc.ProvisionOrder(ctx, uuid.MustParse(f.order.UUID)); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := a.svc.UpdatePaymentStatus(ctx, uuid.MustParse(a.order.UUID), model.UpdateOrderPaymentStatusInput{PaymentStatus: "refunded"}); err != nil {
		t.Fatalf("refund: %v", err)
	}
	const retiredQ = `SELECT COUNT(*) FROM routes r JOIN nfc_cards c ON c.id = r.card_id WHERE c.order_id = $1 AND r.status = 'retired'`
	if n := a.count(t, retiredQ); n != 2 {
		t.Fatalf("refunded order retired routes = %d, want 2", n)
	}
	if n := b.count(t, retiredQ); n != 0 {
		t.Fatalf("other order retired routes = %d, want 0", n)
	}
}
