package billing_test

// TEC-590: payment history and PDF receipts. Real PostgreSQL (TEST_DATABASE_URL); the order is
// paid through the fake PayTR of the order fixture, subscription payments are written directly.
import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/database/db"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/billing/usecase"
	ordersmodel "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/orders/model"
	ordersrepo "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/orders/repository"
	ordersusecase "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/orders/usecase"
	productsrepo "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/products/repository"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/authctx"
	"github.com/jackc/pgx/v5/pgtype"
)

func TestPaymentHistoryAndReceipts(t *testing.T) {
	f := newOrderPayFixture(t)
	ctx := context.Background()
	now := time.Now().UTC()

	// 1. An order marked paid by hand (kind=order_manual).
	ordersSvc := ordersusecase.NewService(ordersrepo.NewPostgres(f.d.Pool, f.d.Queries), productsrepo.NewPostgres(f.d.Queries), nil, nil, "https://comcode.test")
	owner := authctx.Account{ID: f.user.AccountID, Role: authctx.AccountRoleOwner}
	product := f.d.CreateProduct(t, 50)
	manual, err := ordersSvc.CreateForAccount(ctx, owner, f.user.ID, ordersmodel.CreateOrderInput{
		Items: []ordersmodel.CreateOrderItemInput{{ProductUUID: product.String(), Quantity: 1, RecipientName: "Ada"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.d.Pool.Exec(ctx, `UPDATE orders SET payment_status='paid', payment_provider='Havale', payment_reference='DEKONT-1', paid_at=$2 WHERE id=$1`, manual.ID, now.Add(-time.Hour)); err != nil {
		t.Fatal(err)
	}

	// 2. A subscription started from a hosted checkout (kind=subscription_checkout) and one
	//    succeeded renewal (kind=subscription_renewal); a failed attempt must not appear.
	var planID int64
	if err := f.d.Pool.QueryRow(ctx, `SELECT id FROM plans WHERE slug='pro-monthly'`).Scan(&planID); err != nil {
		t.Fatal(err)
	}
	cs, err := f.d.Queries.CreateCheckoutSession(ctx, db.CreateCheckoutSessionParams{
		AccountID: f.user.AccountID, UserID: f.user.ID, PlanID: planID, Provider: db.PaymentProviderPaytr,
		Status: db.CheckoutSessionStatusCompleted, MerchantOid: "chk" + strings.ReplaceAll(f.user.UUID.String(), "-", ""),
		AmountMinor: 99900, Currency: "TRY", ExpiresAt: pgtype.Timestamptz{Time: now.Add(time.Hour), Valid: true}, Metadata: []byte(`{}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.d.Pool.Exec(ctx, `UPDATE payment_checkout_sessions SET completed_at=$2, provider_ref='PAYTR-1' WHERE id=$1`, cs.ID, now.Add(-48*time.Hour)); err != nil {
		t.Fatal(err)
	}
	sub, err := f.d.Queries.CreatePaymentSubscription(ctx, db.CreatePaymentSubscriptionParams{
		PlanID: planID, Status: db.SubscriptionStatusActive, AccountID: f.user.AccountID,
		StartsAt:          pgtype.Timestamptz{Time: now.Add(-48 * time.Hour), Valid: true},
		Provider:          db.NullPaymentProvider{PaymentProvider: db.PaymentProviderPaytr, Valid: true},
		CheckoutSessionID: pgtype.Int8{Int64: cs.ID, Valid: true},
		NextBillingAt:     pgtype.Timestamptz{Time: now.Add(24 * time.Hour), Valid: true},
	})
	if err != nil {
		t.Fatal(err)
	}
	for i, status := range []string{"succeeded", "failed"} {
		attempt, err := f.d.Queries.CreateSubscriptionPaymentAttempt(ctx, db.CreateSubscriptionPaymentAttemptParams{
			AccountID: f.user.AccountID, SubscriptionID: sub.ID, PeriodAt: pgtype.Timestamptz{Time: now.Add(-24 * time.Hour), Valid: true},
			Attempt: int32(i + 1), Kind: "renewal", PlanID: planID, AmountMinor: 99900, Currency: "TRY",
		})
		if err != nil {
			t.Fatal(err)
		}
		if err := f.d.Queries.FinishSubscriptionPaymentAttempt(ctx, db.FinishSubscriptionPaymentAttemptParams{
			ID: attempt.ID, Status: status, ProviderRef: pgtype.Text{String: "REN-" + status, Valid: true},
		}); err != nil {
			t.Fatal(err)
		}
	}

	// 4. Last, so it is the newest payment: the fixture order paid online (kind=order).
	_, oid := f.startCheckout(t)
	p := orderPayload(oid, "success")
	p.Set("payment_id", "fixture-payment")
	resign(p)
	checkReply(t, f.callback(p), 200, "OK")
	f.waitProvisioned(t)

	rec := f.do(t, http.MethodGet, "/v1/billing/payments?limit=10")
	if rec.Code != http.StatusOK {
		t.Fatalf("list payments: %d %s", rec.Code, rec.Body)
	}
	var body struct {
		Data usecase.PaymentHistory `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Data.Total != 4 || len(body.Data.Items) != 4 {
		t.Fatalf("history: total=%d items=%d %s", body.Data.Total, len(body.Data.Items), rec.Body)
	}
	kinds := map[string]usecase.AccountPayment{}
	for _, item := range body.Data.Items {
		kinds[item.Kind] = item
	}
	if got := kinds[usecase.PaymentKindOrder]; got.OrderNumber != f.order.OrderNumber || got.AmountMinor != 12000 || got.Provider != "paytr" || got.ProviderRef != "fixture-payment" {
		t.Errorf("online order payment: %+v", got)
	}
	if got := kinds[usecase.PaymentKindOrderManual]; got.OrderNumber != manual.OrderNumber || got.AmountMinor != 6000 || got.Provider != "Havale" || got.ProviderRef != "DEKONT-1" {
		t.Errorf("manual order payment: %+v", got)
	}
	if got := kinds[usecase.PaymentKindSubscriptionCheckout]; got.PlanName == "" || got.AmountMinor != 99900 || got.ProviderRef != "PAYTR-1" || got.BillingInterval != "month" {
		t.Errorf("subscription checkout: %+v", got)
	}
	if got := kinds[usecase.PaymentKindSubscriptionRenewal]; got.ProviderRef != "REN-succeeded" || got.Provider != "paytr" {
		t.Errorf("renewal: %+v", got)
	}
	// Newest first: the online order payment happened just now.
	if body.Data.Items[0].Kind != usecase.PaymentKindOrder || !strings.HasPrefix(body.Data.Items[0].ReceiptNumber, "RCP-") {
		t.Errorf("ordering: %+v", body.Data.Items[0])
	}
	if rec := f.do(t, http.MethodGet, "/v1/billing/payments?limit=1&offset=3"); rec.Code != 200 || !strings.Contains(rec.Body.String(), `"total":4`) {
		t.Errorf("paging: %d %s", rec.Code, rec.Body)
	}
	if rec := f.do(t, http.MethodGet, "/v1/billing/payments?page=2"); rec.Code != http.StatusBadRequest {
		t.Errorf("forbidden query param: %d", rec.Code)
	}

	// Receipt of each kind renders as PDF.
	for kind, item := range kinds {
		rec := f.do(t, http.MethodGet, "/v1/billing/payments/"+item.UUID.String()+"/receipt?locale=tr")
		if rec.Code != http.StatusOK || rec.Header().Get("Content-Type") != "application/pdf" ||
			!strings.HasPrefix(rec.Body.String(), "%PDF-") || !strings.Contains(rec.Header().Get("Content-Disposition"), item.ReceiptNumber+".pdf") {
			t.Errorf("%s receipt: %d %s %q", kind, rec.Code, rec.Header().Get("Content-Type"), rec.Body.String()[:min(80, rec.Body.Len())])
		}
	}

	// Another account sees nothing and cannot download these receipts.
	other := f.d.CreateUser(t)
	f.as(other, authctx.AccountRoleOwner)
	if rec := f.do(t, http.MethodGet, "/v1/billing/payments"); rec.Code != 200 || !strings.Contains(rec.Body.String(), `"total":0`) {
		t.Errorf("other account history: %d %s", rec.Code, rec.Body)
	}
	if rec := f.do(t, http.MethodGet, "/v1/billing/payments/"+kinds[usecase.PaymentKindOrder].UUID.String()+"/receipt"); rec.Code != http.StatusNotFound {
		t.Errorf("other account receipt: %d %s", rec.Code, rec.Body)
	}
}
