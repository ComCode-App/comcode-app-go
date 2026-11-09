package billing_test

// TEC-943: card order checkout through the payment module. Real PostgreSQL + fake PayTR HTTP,
// enabled by TEST_DATABASE_URL; never contacts a provider.
import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/middleware"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/billing"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/billing/handler"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/billing/repository"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/billing/usecase"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/orders"
	ordershandler "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/orders/handler"
	ordersmodel "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/orders/model"
	ordersrepo "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/orders/repository"
	ordersusecase "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/orders/usecase"
	productsrepo "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/products/repository"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/activity"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/authctx"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/crypto"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/jwt"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/payment"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/rbac"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/testutil/pgtest"
	"github.com/google/uuid"
)

// One item at 100.00 + 20% KDV = 120.00 TRY.
const orderAmount = "12000"

type mutableIdentity struct {
	mu sync.Mutex
	p  authctx.Principal
}

func (i *mutableIdentity) LoadPrincipal(_ *http.Request, _ jwt.Claims) (authctx.Principal, error) {
	i.mu.Lock()
	defer i.mu.Unlock()
	return i.p, nil
}

func (i *mutableIdentity) set(p authctx.Principal) {
	i.mu.Lock()
	defer i.mu.Unlock()
	i.p = p
}

type orderPayFixture struct {
	d       *pgtest.DB
	user    pgtest.User
	order   *ordersmodel.OrderDetail
	mux     http.Handler
	who     *mutableIdentity
	bearer  string
	refunds *fakeRefunds // PayTR refund API (TEC-935)
	stepUp  *fakeStepUp
}

func newOrderPayFixture(t *testing.T) *orderPayFixture {
	t.Helper()
	ctx := context.Background()
	d := pgtest.Open(t)
	user := d.CreateUser(t)
	product := d.CreateProduct(t, 100)
	ordersSvc := ordersusecase.NewService(ordersrepo.NewPostgres(d.Pool, d.Queries), productsrepo.NewPostgres(d.Queries), nil, nil, "https://comcode.test")
	owner := authctx.Account{ID: user.AccountID, Role: authctx.AccountRoleOwner}
	ord, err := ordersSvc.CreateForAccount(ctx, owner, user.ID, ordersmodel.CreateOrderInput{
		Items: []ordersmodel.CreateOrderItemInput{{ProductUUID: product.String(), Quantity: 1, RecipientName: "Ada"}},
	})
	if err != nil {
		t.Fatal(err)
	}

	refunds := &fakeRefunds{}
	fake := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Error(err)
			w.WriteHeader(400)
			return
		}
		if r.URL.Path == "/odeme/iade" {
			refunds.serve(t, w, r.PostForm)
			return
		}
		f := r.PostForm
		expected := sign(f.Get("merchant_id") + f.Get("user_ip") + f.Get("merchant_oid") + f.Get("email") +
			f.Get("payment_amount") + f.Get("user_basket") + f.Get("no_installment") + f.Get("max_installment") +
			f.Get("currency") + f.Get("test_mode") + merchantSalt)
		if f.Get("paytr_token") != expected {
			t.Error("incorrect iFrame token signature")
		}
		if !strings.HasPrefix(f.Get("merchant_oid"), "ord") || f.Get("payment_amount") != orderAmount || f.Get("currency") != "TRY" {
			t.Errorf("order token request: oid=%s amount=%s currency=%s", f.Get("merchant_oid"), f.Get("payment_amount"), f.Get("currency"))
		}
		if f.Get("store_card") == "1" {
			t.Error("order payments must not store the card")
		}
		basket, _ := base64.StdEncoding.DecodeString(f.Get("user_basket"))
		if want := fmt.Sprintf(`[["ComCode %s","120.00",1]]`, ord.OrderNumber); string(basket) != want {
			t.Errorf("basket=%s want %s", basket, want)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"success","token":"fixture-order-token"}`))
	}))
	t.Cleanup(fake.Close)
	box, err := crypto.NewSecretBox(strings.Repeat("k", 32))
	if err != nil {
		t.Fatal(err)
	}
	key, _ := box.Encrypt(merchantKey)
	salt, _ := box.Encrypt(merchantSalt)
	if _, err := d.Pool.Exec(ctx, `UPDATE payment_settings SET active_provider='paytr',test_mode=true WHERE id=1`); err != nil {
		t.Fatal(err)
	}
	if _, err := d.Pool.Exec(ctx, `UPDATE payment_provider_settings SET enabled=true,merchant_id=$1,merchant_key_enc=$2,merchant_salt_enc=$3,base_url=$4 WHERE provider='paytr'`, merchantID, key, salt, fake.URL); err != nil {
		t.Fatal(err)
	}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	svc := usecase.New(d.Queries, payment.NewRegistry(payment.NewDBSettingsReader(d.Queries, box)), "https://comcode.test", "https://comcode.test/api", log, repository.NewPostgres(d.Pool))
	svc.SetOrderPayments(ordersSvc)
	svc.SetActivity(activity.NewRecorder(d.Queries, log))
	ordersSvc.SetPaymentRefunder(svc)
	tokens, err := jwt.NewManager("fixture-jwt-secret", time.Hour, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	bearer, _, err := tokens.IssueAccess(jwt.AccessInput{UserID: user.UUID})
	if err != nil {
		t.Fatal(err)
	}
	who := &mutableIdentity{}
	f := &orderPayFixture{d: d, user: user, order: ord, who: who, bearer: bearer, refunds: refunds, stepUp: &fakeStepUp{}}
	f.stepUp.valid.Store(true)
	f.as(user, authctx.AccountRoleOwner)
	mux := http.NewServeMux()
	billingHandler := handler.New(svc, "https://comcode.test", log)
	billing.RegisterRoutes(mux, billingHandler, tokens, who)
	billing.RegisterRefunds(mux, billingHandler, middleware.Authenticate(tokens, who), f.stepUp.middleware)
	// Registered together to prove the new /v1/orders patterns do not collide with the orders module.
	ordersHandler := ordershandler.New(ordersSvc)
	ordersHandler.SetStepUp(f.stepUp)
	orders.RegisterRoutes(mux, ordersHandler, tokens, who)
	f.mux = mux
	return f
}

func (f *orderPayFixture) as(u pgtest.User, role string) {
	f.who.set(authctx.Principal{
		UserID: u.UUID, UserInternal: u.ID, Email: "payer@example.test",
		Permissions: []string{rbac.PermAuthSession, rbac.PermAppOrdersRead, rbac.PermAppOrdersWrite},
		Account:     authctx.Account{ID: u.AccountID, Role: role},
	})
}

func (f *orderPayFixture) do(t *testing.T, method, path string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(`{}`))
	req.Header.Set("Authorization", "Bearer "+f.bearer)
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = "192.0.2.9:1234"
	rec := httptest.NewRecorder()
	f.mux.ServeHTTP(rec, req)
	return rec
}

// startCheckout starts an order checkout and returns its session and merchant oid.
func (f *orderPayFixture) startCheckout(t *testing.T) (usecase.OrderCheckoutView, string) {
	t.Helper()
	rec := f.do(t, http.MethodPost, "/v1/orders/"+f.order.UUID+"/checkout")
	if rec.Code != http.StatusCreated {
		t.Fatalf("start order checkout: %d %s", rec.Code, rec.Body)
	}
	var body struct {
		Data usecase.OrderCheckoutView `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	v := body.Data
	if v.Status != "pending" || v.IframeToken == nil || *v.IframeToken != "fixture-order-token" || v.AmountMinor != 12000 ||
		v.OrderNumber != f.order.OrderNumber || v.OrderUUID.String() != f.order.UUID {
		t.Fatalf("checkout response: %s", rec.Body)
	}
	var oid string
	if err := f.d.Pool.QueryRow(context.Background(), `SELECT merchant_oid FROM order_payment_sessions WHERE uuid=$1`, v.UUID).Scan(&oid); err != nil {
		t.Fatal(err)
	}
	return v, oid
}

func orderPayload(oid, status string) url.Values {
	total := orderAmount
	if status == "failed" {
		total = "0"
	}
	p := url.Values{"merchant_oid": {oid}, "status": {status}, "total_amount": {total}, "currency": {"TL"},
		"payment_amount": {orderAmount}, "failed_reason_code": {"2"}, "failed_reason_msg": {"Fixture decline"}}
	resign(p)
	return p
}

func (f *orderPayFixture) callback(p url.Values) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/v1/public/payment/paytr/callback", strings.NewReader(p.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	f.mux.ServeHTTP(rec, req)
	return rec
}

type orderState struct {
	status, paymentStatus, provider string
	history                         []string
	sessions                        map[string]int
}

func (f *orderPayFixture) state(t *testing.T) orderState {
	t.Helper()
	ctx := context.Background()
	var s orderState
	if err := f.d.Pool.QueryRow(ctx, `SELECT status, payment_status, payment_provider FROM orders WHERE id=$1`, f.order.ID).
		Scan(&s.status, &s.paymentStatus, &s.provider); err != nil {
		t.Fatal(err)
	}
	rows, err := f.d.Pool.Query(ctx, `SELECT COALESCE(from_status,'') || '>' || to_status || CASE WHEN changed_by_user_id IS NULL THEN ' (system)' ELSE '' END
		FROM order_status_history WHERE order_id=$1 ORDER BY id`, f.order.ID)
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var h string
		if err := rows.Scan(&h); err != nil {
			t.Fatal(err)
		}
		s.history = append(s.history, h)
	}
	rows.Close()
	s.sessions = map[string]int{}
	rows, err = f.d.Pool.Query(ctx, `SELECT status::text, count(*) FROM order_payment_sessions WHERE order_id=$1 GROUP BY status`, f.order.ID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var st string
		var n int
		if err := rows.Scan(&st, &n); err != nil {
			t.Fatal(err)
		}
		s.sessions[st] = n
	}
	return s
}

// waitProvisioned waits for the background provisioning that a paid order starts.
func (f *orderPayFixture) waitProvisioned(t *testing.T) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for {
		var n int
		if err := f.d.Pool.QueryRow(context.Background(), `SELECT count(*) FROM order_items WHERE order_id=$1 AND provisioning_status='provisioned'`, f.order.ID).Scan(&n); err != nil {
			t.Fatal(err)
		}
		if n == 1 {
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("paid order was not provisioned")
		}
		time.Sleep(50 * time.Millisecond)
	}
}

func TestOrderCheckoutPaidStartsProvisioningIdempotently(t *testing.T) {
	f := newOrderPayFixture(t)
	session, oid := f.startCheckout(t)
	p := orderPayload(oid, "success")
	p.Set("payment_id", "fixture-payment")
	resign(p)

	var wg sync.WaitGroup
	replies := make(chan *httptest.ResponseRecorder, 6)
	for range 6 {
		wg.Add(1)
		go func() { defer wg.Done(); replies <- f.callback(p) }()
	}
	wg.Wait()
	close(replies)
	for reply := range replies {
		checkReply(t, reply, 200, "OK")
	}
	f.waitProvisioned(t)

	s := f.state(t)
	if s.status != "paid" || s.paymentStatus != "paid" || s.provider != "paytr" {
		t.Fatalf("order after payment: %+v", s)
	}
	if want := []string{">pending", "pending>paid (system)"}; strings.Join(s.history, ",") != strings.Join(want, ",") {
		t.Fatalf("history=%v want %v", s.history, want)
	}
	if s.sessions["completed"] != 1 || len(s.sessions) != 1 {
		t.Fatalf("sessions=%v", s.sessions)
	}

	// Replays and late failures after completion change nothing.
	checkReply(t, f.callback(p), 200, "OK")
	checkReply(t, f.callback(orderPayload(oid, "failed")), 200, "OK")
	if again := f.state(t); len(again.history) != 2 || again.sessions["completed"] != 1 || again.status != "paid" {
		t.Fatalf("duplicate callback changed state: %+v", again)
	}

	rec := f.do(t, http.MethodGet, "/v1/orders/checkout/"+session.UUID.String())
	var body struct {
		Data usecase.OrderCheckoutView `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil || rec.Code != 200 {
		t.Fatalf("get checkout: %d %s", rec.Code, rec.Body)
	}
	if body.Data.Status != "completed" || body.Data.OrderPaymentStatus != "paid" || body.Data.IframeToken != nil {
		t.Fatalf("completed checkout view: %s", rec.Body)
	}
	if rec := f.do(t, http.MethodPost, "/v1/orders/"+f.order.UUID+"/checkout"); rec.Code != http.StatusConflict {
		t.Fatalf("paid order checkout: %d %s", rec.Code, rec.Body)
	}
}

func TestOrderCheckoutFailureKeepsOrderPayable(t *testing.T) {
	f := newOrderPayFixture(t)
	first, oid := f.startCheckout(t)
	checkReply(t, f.callback(orderPayload(oid, "failed")), 200, "OK")

	s := f.state(t)
	if s.status != "pending" || s.paymentStatus != "unpaid" || len(s.history) != 1 || s.sessions["failed"] != 1 {
		t.Fatalf("order after failed payment: %+v", s)
	}
	var reason string
	if err := f.d.Pool.QueryRow(context.Background(), `SELECT metadata->>'failed_reason_msg' FROM order_payment_sessions WHERE uuid=$1`, first.UUID).Scan(&reason); err != nil || reason != "Fixture decline" {
		t.Fatalf("failure reason=%q %v", reason, err)
	}
	// A success notification for the failed attempt cannot revive it.
	checkReply(t, f.callback(orderPayload(oid, "success")), 200, "OK")
	if s := f.state(t); s.paymentStatus != "unpaid" || s.sessions["failed"] != 1 {
		t.Fatalf("failed session revived: %+v", s)
	}

	// The order can be paid again with a new session.
	_, retry := f.startCheckout(t)
	checkReply(t, f.callback(orderPayload(retry, "success")), 200, "OK")
	f.waitProvisioned(t)
	if s := f.state(t); s.status != "paid" || s.paymentStatus != "paid" || s.sessions["failed"] != 1 || s.sessions["completed"] != 1 {
		t.Fatalf("order after retry: %+v", s)
	}
}

func TestOrderCheckoutCallbackVerification(t *testing.T) {
	for _, tc := range []struct {
		name, key, value string
		signed           bool
	}{
		{"bad hash", "hash", "tampered", false},
		{"forged status", "status", "success", false},
		{"underpayment", "total_amount", "11999", true},
		{"order amount mismatch", "payment_amount", "12001", true},
		{"currency mismatch", "currency", "USD", true},
		{"unknown order session", "merchant_oid", "ordunknown", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newOrderPayFixture(t)
			_, oid := f.startCheckout(t)
			p := orderPayload(oid, "failed")
			if tc.key != "status" {
				p = orderPayload(oid, "success")
			}
			p.Set(tc.key, tc.value)
			if tc.signed {
				resign(p)
			}
			checkReply(t, f.callback(p), 400, "FAIL")
			if s := f.state(t); s.status != "pending" || s.paymentStatus != "unpaid" || s.sessions["pending"] != 1 || len(s.history) != 1 {
				t.Fatalf("rejected callback changed state: %+v", s)
			}
		})
	}
	t.Run("provider mismatch", func(t *testing.T) {
		f := newOrderPayFixture(t)
		session, oid := f.startCheckout(t)
		if _, err := f.d.Pool.Exec(context.Background(), `UPDATE order_payment_sessions SET provider='iyzico' WHERE uuid=$1`, session.UUID); err != nil {
			t.Fatal(err)
		}
		checkReply(t, f.callback(orderPayload(oid, "success")), 400, "FAIL")
		if s := f.state(t); s.paymentStatus != "unpaid" {
			t.Fatalf("provider mismatch paid the order: %+v", s)
		}
	})
}

func TestOrderCheckoutClosedOrAlreadyPaidOrder(t *testing.T) {
	t.Run("cancelled before the money arrived", func(t *testing.T) {
		f := newOrderPayFixture(t)
		session, oid := f.startCheckout(t)
		if _, err := f.d.Pool.Exec(context.Background(), `UPDATE orders SET status='cancelled' WHERE id=$1`, f.order.ID); err != nil {
			t.Fatal(err)
		}
		checkReply(t, f.callback(orderPayload(oid, "success")), 200, "OK")
		var result string
		if err := f.d.Pool.QueryRow(context.Background(), `SELECT metadata->>'order_result' FROM order_payment_sessions WHERE uuid=$1`, session.UUID).Scan(&result); err != nil || result != "order_closed" {
			t.Fatalf("order_result=%q %v", result, err)
		}
		if s := f.state(t); s.status != "cancelled" || s.paymentStatus != "unpaid" || s.sessions["completed"] != 1 {
			t.Fatalf("cancelled order after payment: %+v", s)
		}
	})
	t.Run("two sessions paid", func(t *testing.T) {
		f := newOrderPayFixture(t)
		_, first := f.startCheckout(t)
		second, oid := f.startCheckout(t)
		checkReply(t, f.callback(orderPayload(first, "success")), 200, "OK")
		f.waitProvisioned(t)
		checkReply(t, f.callback(orderPayload(oid, "success")), 200, "OK")
		var result string
		if err := f.d.Pool.QueryRow(context.Background(), `SELECT metadata->>'order_result' FROM order_payment_sessions WHERE uuid=$1`, second.UUID).Scan(&result); err != nil || result != "already_paid" {
			t.Fatalf("order_result=%q %v", result, err)
		}
		if s := f.state(t); len(s.history) != 2 || s.sessions["completed"] != 2 {
			t.Fatalf("second payment changed the order: %+v", s)
		}
	})
}

func TestOrderCheckoutAccess(t *testing.T) {
	f := newOrderPayFixture(t)
	f.as(f.user, authctx.AccountRoleMember)
	if rec := f.do(t, http.MethodPost, "/v1/orders/"+f.order.UUID+"/checkout"); rec.Code != http.StatusForbidden {
		t.Fatalf("member checkout: %d %s", rec.Code, rec.Body)
	}
	f.as(f.user, authctx.AccountRoleOwner)
	session, _ := f.startCheckout(t)
	other := f.d.CreateUser(t)
	f.as(other, authctx.AccountRoleOwner)
	if rec := f.do(t, http.MethodPost, "/v1/orders/"+f.order.UUID+"/checkout"); rec.Code != http.StatusNotFound {
		t.Fatalf("other account checkout: %d %s", rec.Code, rec.Body)
	}
	if rec := f.do(t, http.MethodGet, "/v1/orders/checkout/"+session.UUID.String()); rec.Code != http.StatusNotFound {
		t.Fatalf("other account session read: %d %s", rec.Code, rec.Body)
	}
	if rec := f.do(t, http.MethodPost, "/v1/orders/"+uuid.NewString()+"/checkout"); rec.Code != http.StatusNotFound {
		t.Fatalf("unknown order checkout: %d %s", rec.Code, rec.Body)
	}
}
