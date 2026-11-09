package usecase

// Postgres integration test for order_status_history (TEC-944). Skipped unless
// TEST_DATABASE_URL is set; see internal/testutil/pgtest.

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/orders/model"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/authctx"
)

type historyRow struct {
	from, to string
	by       *int64
}

func (f orderFixture) history(t *testing.T) []historyRow {
	t.Helper()
	rows, err := f.d.Pool.Query(context.Background(), `
		SELECT COALESCE(from_status, ''), to_status, changed_by_user_id
		FROM order_status_history WHERE order_id = $1 AND account_id = $2 ORDER BY id`, f.order.ID, f.order.AccountID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var out []historyRow
	for rows.Next() {
		var r historyRow
		if err := rows.Scan(&r.from, &r.to, &r.by); err != nil {
			t.Fatal(err)
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}

func TestIntegrationOrderStatusHistoryRecordsEveryTransition(t *testing.T) {
	f := newOrderFixture(t)
	staff := f.d.CreateUser(t)
	ctx := authctx.WithPrincipal(context.Background(), authctx.Principal{UserInternal: staff.ID})
	id := uuid.MustParse(f.order.UUID)

	if _, err := f.svc.UpdateStatus(ctx, id, model.UpdateOrderStatusInput{Status: "in_production"}); err != nil {
		t.Fatal(err)
	}
	// Same status again: no transition, no row.
	if _, err := f.svc.UpdateStatus(ctx, id, model.UpdateOrderStatusInput{Status: "in_production"}); err != nil {
		t.Fatal(err)
	}
	// Tracking only: status unchanged, no row.
	if _, err := f.svc.UpdateShipping(ctx, id, model.UpdateOrderShippingInput{ShippingCarrier: "Yurtiçi", TrackingNumber: "T1"}); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.UpdateShipping(ctx, id, model.UpdateOrderShippingInput{ShippingCarrier: "Yurtiçi", TrackingNumber: "T1", MarkAsShipped: true}); err != nil {
		t.Fatal(err)
	}
	// A system transition (no principal) has no actor.
	if _, err := f.svc.UpdateStatus(context.Background(), id, model.UpdateOrderStatusInput{Status: "delivered"}); err != nil {
		t.Fatal(err)
	}

	got := f.history(t)
	placer := f.order.UserID
	want := []historyRow{
		{"", "pending", &placer},
		{"pending", "in_production", &staff.ID},
		{"in_production", "shipped", &staff.ID},
		{"shipped", "delivered", nil},
	}
	if len(got) != len(want) {
		t.Fatalf("history = %+v, want %d rows", got, len(want))
	}
	for i := range want {
		g, w := got[i], want[i]
		if g.from != w.from || g.to != w.to || (g.by == nil) != (w.by == nil) || (g.by != nil && *g.by != *w.by) {
			t.Fatalf("history[%d] = {%s→%s by %v}, want {%s→%s by %v}", i, g.from, g.to, g.by, w.from, w.to, w.by)
		}
	}

	// Unknown orders write nothing and report not found.
	if _, err := f.svc.UpdateStatus(ctx, uuid.New(), model.UpdateOrderStatusInput{Status: "paid"}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("unknown order err = %v, want ErrNotFound", err)
	}
}

// TEC-943: a platform admin marking the payment paid by hand also moves a pending order to paid,
// recorded with the admin as actor; a later paid mark does not add another row.
func TestIntegrationManualPaymentMovesPendingOrderToPaid(t *testing.T) {
	f := newOrderFixture(t)
	staff := f.d.CreateUser(t)
	ctx := authctx.WithPrincipal(context.Background(), authctx.Principal{UserInternal: staff.ID})
	id := uuid.MustParse(f.order.UUID)
	ref := "EFT-1"
	ord, err := f.svc.UpdatePaymentStatus(ctx, id, model.UpdateOrderPaymentStatusInput{PaymentStatus: "paid", PaymentReference: &ref})
	if err != nil {
		t.Fatal(err)
	}
	if ord.Status != "paid" || ord.PaymentStatus != "paid" || ord.PaymentReference != ref {
		t.Fatalf("order = %s/%s/%s", ord.Status, ord.PaymentStatus, ord.PaymentReference)
	}
	if _, err := f.svc.UpdatePaymentStatus(ctx, id, model.UpdateOrderPaymentStatusInput{PaymentStatus: "paid"}); err != nil {
		t.Fatal(err)
	}
	got := f.history(t)
	if len(got) != 2 || got[1].from != "pending" || got[1].to != "paid" || got[1].by == nil || *got[1].by != staff.ID {
		t.Fatalf("history = %+v", got)
	}
	// Provisioning runs in the background after a paid mark; let it finish before the database goes.
	deadline := time.Now().Add(10 * time.Second)
	for f.count(t, `SELECT count(*) FROM order_items WHERE order_id = $1 AND provisioning_status = 'provisioned'`) != 2 {
		if time.Now().After(deadline) {
			t.Fatal("paid order was not provisioned")
		}
		time.Sleep(50 * time.Millisecond)
	}
}
