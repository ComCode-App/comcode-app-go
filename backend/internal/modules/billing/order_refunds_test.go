package billing_test

// TEC-935: refunds of online order payments through the provider's refund API. Real PostgreSQL +
// the fake PayTR server of orders_test.go (/odeme/iade); never contacts a provider.
import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/google/uuid"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/billing/usecase"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/authctx"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/rbac"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/pkg/response"
)

// fakeRefunds answers PayTR refund requests; fail makes it decline them.
type fakeRefunds struct {
	mu    sync.Mutex
	calls []url.Values
	fail  string // "": success, "rejected": status=error, "down": HTTP 503
}

func (f *fakeRefunds) serve(t *testing.T, w http.ResponseWriter, form url.Values) {
	f.mu.Lock()
	f.calls = append(f.calls, form)
	fail := f.fail
	f.mu.Unlock()
	want := sign(form.Get("merchant_id") + form.Get("merchant_oid") + form.Get("return_amount") + merchantSalt)
	if form.Get("paytr_token") != want {
		t.Error("incorrect refund token signature")
	}
	if !strings.HasPrefix(form.Get("reference_no"), "rfd") || form.Get("merchant_id") != merchantID {
		t.Errorf("refund request: %v", form)
	}
	w.Header().Set("Content-Type", "application/json")
	switch fail {
	case "rejected":
		_, _ = w.Write([]byte(`{"status":"error","err_no":"005","err_msg":"Fixture refund declined"}`))
	case "down":
		w.WriteHeader(http.StatusServiceUnavailable)
	default:
		_, _ = w.Write([]byte(`{"status":"success","is_test":1,"merchant_oid":"` + form.Get("merchant_oid") +
			`","return_amount":"` + form.Get("return_amount") + `","reference_no":"` + form.Get("reference_no") + `"}`))
	}
}

func (f *fakeRefunds) set(fail string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.fail = fail
}

func (f *fakeRefunds) requests() []url.Values {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]url.Values(nil), f.calls...)
}

// fakeStepUp stands in for the step-up grant check of both the refund routes and the orders module.
type fakeStepUp struct{ valid atomic.Bool }

func (s *fakeStepUp) HasValidGrant(context.Context, uuid.UUID) (bool, error) {
	return s.valid.Load(), nil
}

func (s *fakeStepUp) middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !s.valid.Load() {
			response.StepUpRequired(w, r)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// asPlatformAdmin signs in as a platform admin with order permissions.
func (f *orderPayFixture) asPlatformAdmin() {
	f.who.set(authctx.Principal{
		UserID: f.user.UUID, UserInternal: f.user.ID, Email: "admin@example.test",
		Permissions: []string{rbac.PermAuthSession, rbac.PermPlatformOrdersRead, rbac.PermPlatformOrdersWrite},
		Account:     authctx.Account{ID: f.user.AccountID, Role: authctx.AccountRoleOwner},
	})
}

func (f *orderPayFixture) send(t *testing.T, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+f.bearer)
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = "192.0.2.9:1234"
	rec := httptest.NewRecorder()
	f.mux.ServeHTTP(rec, req)
	return rec
}

// payOnline pays the order through a PayTR checkout and waits for its card to be provisioned.
func (f *orderPayFixture) payOnline(t *testing.T) string {
	t.Helper()
	session, oid := f.startCheckout(t)
	p := orderPayload(oid, "success")
	p.Set("payment_id", "fixture-payment")
	resign(p)
	checkReply(t, f.callback(p), 200, "OK")
	f.waitProvisioned(t)
	f.asPlatformAdmin()
	return session.UUID.String()
}

func (f *orderPayFixture) refund(t *testing.T, body string) (*httptest.ResponseRecorder, usecase.OrderRefundView) {
	t.Helper()
	rec := f.send(t, http.MethodPost, "/v1/platform/orders/"+f.order.UUID+"/refunds", body)
	var out struct {
		Data usecase.OrderRefundView `json:"data"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	return rec, out.Data
}

func (f *orderPayFixture) listRefunds(t *testing.T) usecase.OrderRefundsView {
	t.Helper()
	rec := f.send(t, http.MethodGet, "/v1/platform/orders/"+f.order.UUID+"/refunds", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("list refunds: %d %s", rec.Code, rec.Body)
	}
	var out struct {
		Data usecase.OrderRefundsView `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	return out.Data
}

// routeStatuses returns the statuses of the order's card routes.
func (f *orderPayFixture) routeStatuses(t *testing.T) []string {
	t.Helper()
	rows, err := f.d.Pool.Query(context.Background(),
		`SELECT r.status FROM routes r JOIN nfc_cards c ON c.id = r.card_id WHERE c.order_id = $1 AND r.deleted_at IS NULL`, f.order.ID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			t.Fatal(err)
		}
		out = append(out, s)
	}
	if len(out) == 0 {
		t.Fatal("order has no card routes")
	}
	return out
}

func (f *orderPayFixture) activity(t *testing.T, action string) int {
	t.Helper()
	var n int
	if err := f.d.Pool.QueryRow(context.Background(),
		`SELECT count(*) FROM activity_events WHERE action = $1 AND resource_uuid = $2`, action, f.order.UUID).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func assertRoutes(t *testing.T, got []string, want string) {
	t.Helper()
	for _, s := range got {
		if s != want {
			t.Fatalf("card routes %v, want all %s", got, want)
		}
	}
}

// assertLiveRoutes checks no card route is retired (provisioned routes are reserved until the
// card is activated).
func assertLiveRoutes(t *testing.T, got []string) {
	t.Helper()
	for _, s := range got {
		if s == "retired" {
			t.Fatalf("card routes %v were retired", got)
		}
	}
}

func TestOrderRefundFullRetiresCardsIdempotently(t *testing.T) {
	f := newOrderPayFixture(t)
	payment := f.payOnline(t)
	assertLiveRoutes(t, f.routeStatuses(t))

	// Concurrent requests with one key refund once.
	var wg sync.WaitGroup
	results := make(chan *httptest.ResponseRecorder, 4)
	for range 4 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			rec, _ := f.refund(t, `{"idempotency_key":"full-1","reason":"customer request"}`)
			results <- rec
		}()
	}
	wg.Wait()
	close(results)
	ids := map[string]bool{}
	for rec := range results {
		if rec.Code != http.StatusCreated && rec.Code != http.StatusOK {
			t.Fatalf("refund: %d %s", rec.Code, rec.Body)
		}
		var out struct {
			Data usecase.OrderRefundView `json:"data"`
		}
		_ = json.Unmarshal(rec.Body.Bytes(), &out)
		ids[out.Data.UUID.String()] = true
	}
	if len(ids) != 1 {
		t.Fatalf("one key created %d refunds", len(ids))
	}
	calls := f.refunds.requests()
	if len(calls) != 1 {
		t.Fatalf("provider refund calls=%d want 1", len(calls))
	}
	var oid string
	if err := f.d.Pool.QueryRow(context.Background(), `SELECT merchant_oid FROM order_payment_sessions WHERE uuid=$1`, payment).Scan(&oid); err != nil {
		t.Fatal(err)
	}
	if calls[0].Get("merchant_oid") != oid || calls[0].Get("return_amount") != "120.00" {
		t.Fatalf("refund request: %v", calls[0])
	}

	s := f.state(t)
	if s.status != "refunded" || s.paymentStatus != "refunded" {
		t.Fatalf("order after full refund: %+v", s)
	}
	// The refund goes through the orders module: history row with the admin as actor.
	if want := ">pending,pending>paid (system),paid>refunded"; strings.Join(s.history, ",") != want {
		t.Fatalf("history=%v want %s", s.history, want)
	}
	assertRoutes(t, f.routeStatuses(t), "retired")
	if f.activity(t, "order.refund_succeeded") != 1 {
		t.Fatal("refund not in the activity log")
	}

	// A replay returns the same refund; a new request finds nothing left.
	rec, again := f.refund(t, `{"idempotency_key":"full-1"}`)
	if rec.Code != http.StatusOK || !ids[again.UUID.String()] || again.Status != "succeeded" || again.AmountMinor != 12000 {
		t.Fatalf("replay: %d %s", rec.Code, rec.Body)
	}
	if rec, _ := f.refund(t, `{"idempotency_key":"full-2"}`); rec.Code != http.StatusConflict || !strings.Contains(rec.Body.String(), "NOTHING_TO_REFUND") {
		t.Fatalf("second refund: %d %s", rec.Code, rec.Body)
	}
	if len(f.refunds.requests()) != 1 {
		t.Fatal("replays called the provider again")
	}
	list := f.listRefunds(t)
	if len(list.Payments) != 1 || list.Payments[0].RefundedMinor != 12000 || list.Payments[0].RefundableMinor != 0 ||
		!list.Payments[0].PaidOrder || len(list.Refunds) != 1 || list.Refunds[0].ProviderRef == nil {
		t.Fatalf("refund list: %+v", list)
	}
}

func TestOrderRefundPartialThenRemaining(t *testing.T) {
	f := newOrderPayFixture(t)
	f.payOnline(t)

	rec, first := f.refund(t, `{"idempotency_key":"part-1","amount_minor":5000}`)
	if rec.Code != http.StatusCreated || first.Status != "succeeded" || first.AmountMinor != 5000 {
		t.Fatalf("partial refund: %d %s", rec.Code, rec.Body)
	}
	if got := f.refunds.requests()[0].Get("return_amount"); got != "50.00" {
		t.Fatalf("return_amount=%s", got)
	}
	s := f.state(t)
	if s.status != "paid" || s.paymentStatus != "partially_refunded" || len(s.history) != 2 {
		t.Fatalf("order after partial refund: %+v", s)
	}
	assertLiveRoutes(t, f.routeStatuses(t))

	if rec, _ := f.refund(t, `{"idempotency_key":"part-2","amount_minor":7001}`); rec.Code != http.StatusBadRequest {
		t.Fatalf("over-refund: %d %s", rec.Code, rec.Body)
	}
	rec, rest := f.refund(t, `{"idempotency_key":"part-3"}`)
	if rec.Code != http.StatusCreated || rest.AmountMinor != 7000 {
		t.Fatalf("remaining refund: %d %s", rec.Code, rec.Body)
	}
	if s := f.state(t); s.status != "refunded" || s.paymentStatus != "refunded" {
		t.Fatalf("order after remaining refund: %+v", s)
	}
	assertRoutes(t, f.routeStatuses(t), "retired")
}

func TestOrderRefundProviderFailureKeepsOrderAndRetries(t *testing.T) {
	f := newOrderPayFixture(t)
	f.payOnline(t)

	f.refunds.set("rejected")
	rec, _ := f.refund(t, `{"idempotency_key":"fail-1"}`)
	if rec.Code != http.StatusBadGateway || !strings.Contains(rec.Body.String(), "REFUND_FAILED") || !strings.Contains(rec.Body.String(), "Fixture refund declined") {
		t.Fatalf("declined refund: %d %s", rec.Code, rec.Body)
	}
	if s := f.state(t); s.status != "paid" || s.paymentStatus != "paid" || len(s.history) != 2 {
		t.Fatalf("declined refund changed the order: %+v", s)
	}
	assertLiveRoutes(t, f.routeStatuses(t))
	list := f.listRefunds(t)
	if len(list.Refunds) != 1 || list.Refunds[0].Status != "failed" || list.Refunds[0].ErrorCode == nil ||
		*list.Refunds[0].ErrorCode != usecase.RefundErrorRejected || list.Payments[0].RefundableMinor != 12000 {
		t.Fatalf("failed refund: %+v", list)
	}
	failed := list.Refunds[0]
	if f.activity(t, "order.refund_failed") != 1 {
		t.Fatal("failed refund not in the activity log")
	}
	// The same key returns the failed refund without calling the provider again.
	if rec, again := f.refund(t, `{"idempotency_key":"fail-1"}`); rec.Code != http.StatusOK || again.UUID != failed.UUID || len(f.refunds.requests()) != 1 {
		t.Fatalf("replay of failed refund: %d %s", rec.Code, rec.Body)
	}

	// An unanswered call is recorded as an unknown outcome.
	f.refunds.set("down")
	retryPath := "/v1/platform/orders/" + f.order.UUID + "/refunds/" + failed.UUID.String() + "/retry"
	if rec := f.send(t, http.MethodPost, retryPath, ""); rec.Code != http.StatusBadGateway {
		t.Fatalf("retry against a down provider: %d %s", rec.Code, rec.Body)
	}
	if list := f.listRefunds(t); *list.Refunds[0].ErrorCode != usecase.RefundErrorUnknown || list.Refunds[0].Attempts != 2 {
		t.Fatalf("unknown outcome: %+v", list.Refunds[0])
	}

	f.refunds.set("")
	rec = f.send(t, http.MethodPost, retryPath, "")
	var out struct {
		Data usecase.OrderRefundView `json:"data"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	if rec.Code != http.StatusOK || out.Data.Status != "succeeded" || out.Data.Attempts != 3 || out.Data.ErrorCode != nil {
		t.Fatalf("retry: %d %s", rec.Code, rec.Body)
	}
	calls := f.refunds.requests()
	if len(calls) != 3 || calls[0].Get("reference_no") != calls[2].Get("reference_no") {
		t.Fatalf("retries must reuse the reference: %v", calls)
	}
	if s := f.state(t); s.status != "refunded" || s.paymentStatus != "refunded" {
		t.Fatalf("order after retry: %+v", s)
	}
	assertRoutes(t, f.routeStatuses(t), "retired")
	// Retrying a succeeded refund is a no-op.
	if rec := f.send(t, http.MethodPost, retryPath, ""); rec.Code != http.StatusOK || len(f.refunds.requests()) != 3 {
		t.Fatalf("retry of succeeded refund: %d %s", rec.Code, rec.Body)
	}
}

func TestOrderStatusRefundedRefundsAtProvider(t *testing.T) {
	f := newOrderPayFixture(t)
	f.payOnline(t)
	path := "/v1/platform/orders/" + f.order.UUID + "/status"

	f.stepUp.valid.Store(false)
	if rec := f.send(t, http.MethodPut, path, `{"status":"refunded"}`); rec.Code != http.StatusForbidden || !strings.Contains(rec.Body.String(), "STEP_UP_REQUIRED") {
		t.Fatalf("refund without step-up: %d %s", rec.Code, rec.Body)
	}
	if rec, _ := f.refund(t, `{"idempotency_key":"no-step-up"}`); rec.Code != http.StatusForbidden {
		t.Fatalf("refund route without step-up: %d %s", rec.Code, rec.Body)
	}
	f.stepUp.valid.Store(true)

	f.refunds.set("rejected")
	if rec := f.send(t, http.MethodPut, path, `{"status":"refunded"}`); rec.Code != http.StatusBadGateway {
		t.Fatalf("status change with declined refund: %d %s", rec.Code, rec.Body)
	}
	if s := f.state(t); s.status != "paid" || s.paymentStatus != "paid" {
		t.Fatalf("declined refund moved the order: %+v", s)
	}
	assertLiveRoutes(t, f.routeStatuses(t))

	f.refunds.set("")
	if rec := f.send(t, http.MethodPut, "/v1/platform/orders/"+f.order.UUID+"/payment", `{"payment_status":"refunded"}`); rec.Code != http.StatusOK {
		t.Fatalf("payment status change to refunded: %d %s", rec.Code, rec.Body)
	}
	s := f.state(t)
	if s.status != "refunded" || s.paymentStatus != "refunded" || s.history[len(s.history)-1] != "paid>refunded" {
		t.Fatalf("order after status change: %+v", s)
	}
	assertRoutes(t, f.routeStatuses(t), "retired")
	if calls := f.refunds.requests(); len(calls) != 2 || calls[1].Get("return_amount") != "120.00" {
		t.Fatalf("provider calls: %v", calls)
	}
	// Nothing is left, so a repeated status change does not refund again.
	if rec := f.send(t, http.MethodPut, path, `{"status":"refunded"}`); rec.Code != http.StatusOK || len(f.refunds.requests()) != 2 {
		t.Fatalf("repeated status change: %d %s", rec.Code, rec.Body)
	}
}

func TestOrderRefundWithoutOnlinePayment(t *testing.T) {
	f := newOrderPayFixture(t)
	f.asPlatformAdmin()
	if rec, _ := f.refund(t, `{"idempotency_key":"none"}`); rec.Code != http.StatusConflict || !strings.Contains(rec.Body.String(), "NOTHING_TO_REFUND") {
		t.Fatalf("refund of unpaid order: %d %s", rec.Code, rec.Body)
	}
	if rec, _ := f.refund(t, `{}`); rec.Code != http.StatusBadRequest {
		t.Fatalf("refund without idempotency key: %d %s", rec.Code, rec.Body)
	}
	// Orders paid outside the payment module keep the manual status change.
	if rec := f.send(t, http.MethodPut, "/v1/platform/orders/"+f.order.UUID+"/status", `{"status":"refunded"}`); rec.Code != http.StatusOK {
		t.Fatalf("manual refund: %d %s", rec.Code, rec.Body)
	}
	if len(f.refunds.requests()) != 0 {
		t.Fatal("provider called for an order without online payment")
	}
	// The customer cannot use the platform routes.
	f.as(f.user, authctx.AccountRoleOwner)
	if rec, _ := f.refund(t, `{"idempotency_key":"customer"}`); rec.Code != http.StatusForbidden {
		t.Fatalf("customer refund: %d %s", rec.Code, rec.Body)
	}
}
