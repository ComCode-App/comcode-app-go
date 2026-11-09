package billing_test

// Real PostgreSQL + fake PayTR HTTP, enabled by TEST_DATABASE_URL (repository convention).
import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
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

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/database/db"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/billing"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/billing/handler"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/billing/repository"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/billing/usecase"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/authctx"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/crypto"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/jwt"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/payment"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/rbac"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/testutil/pgtest"
	"github.com/google/uuid"
)

const (
	merchantID   = "fixture-merchant"
	merchantKey  = "fixture-key-not-a-credential"
	merchantSalt = "fixture-salt-not-a-credential"
)

func TestMain(m *testing.M) { pgtest.Main(m) }

type identity struct{ principal authctx.Principal }

func (i identity) LoadPrincipal(_ *http.Request, _ jwt.Claims) (authctx.Principal, error) {
	return i.principal, nil
}

func sign(s string) string {
	mac := hmac.New(sha256.New, []byte(merchantKey))
	mac.Write([]byte(s))
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

type fixture struct {
	d       *pgtest.DB
	user    pgtest.User
	mux     http.Handler
	oid     string
	session uuid.UUID
}

func checkout(t *testing.T, testMode bool, tokenStatus int) fixture {
	t.Helper()
	ctx := context.Background()
	d := pgtest.Open(t)
	user := d.CreateUser(t)
	var plan uuid.UUID
	if err := d.Pool.QueryRow(ctx, `INSERT INTO plans (slug,name,billing_interval,price_minor,currency,is_active)
 VALUES ($1,'Fixture plan','month',12345,'TRY',true) RETURNING uuid`, "paytr-"+pgtest.Unique()).Scan(&plan); err != nil {
		t.Fatal(err)
	}
	fake := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/odeme/api/get-token" {
			t.Errorf("unexpected PayTR request: %s %s", r.Method, r.URL)
			w.WriteHeader(400)
			return
		}
		if err := r.ParseForm(); err != nil {
			t.Error(err)
			w.WriteHeader(400)
			return
		}
		f := r.PostForm
		expected := sign(f.Get("merchant_id") + f.Get("user_ip") + f.Get("merchant_oid") + f.Get("email") +
			f.Get("payment_amount") + f.Get("user_basket") + f.Get("no_installment") + f.Get("max_installment") +
			f.Get("currency") + f.Get("test_mode") + merchantSalt)
		if f.Get("paytr_token") != expected {
			t.Error("incorrect iFrame token signature")
			w.WriteHeader(400)
			return
		}
		mode := "0"
		if testMode {
			mode = "1"
		}
		for key, want := range map[string]string{"merchant_id": merchantID, "payment_amount": "12345", "currency": "TRY",
			"test_mode": mode, "debug_on": mode, "no_installment": "0", "max_installment": "0", "user_ip": "192.0.2.9"} {
			if f.Get(key) != want {
				t.Errorf("%s=%q want %q", key, f.Get(key), want)
			}
		}
		basket, err := base64.StdEncoding.DecodeString(f.Get("user_basket"))
		if err != nil || string(basket) != `[["Fixture plan","123.45",1]]` {
			t.Errorf("basket=%s err=%v", basket, err)
		}
		row, err := d.Queries.GetCheckoutSessionByMerchantOID(ctx, f.Get("merchant_oid"))
		if err != nil || row.Status != db.CheckoutSessionStatusPending || row.AmountMinor != 12345 || row.AccountID != user.AccountID {
			t.Errorf("checkout must be pending before token request: %+v %v", row, err)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(tokenStatus)
		// Even a success-shaped body on HTTP failure must not initialize checkout.
		_, _ = w.Write([]byte(`{"status":"success","token":"fixture-iframe-token"}`))
	}))
	t.Cleanup(fake.Close)
	box, err := crypto.NewSecretBox(strings.Repeat("k", 32))
	if err != nil {
		t.Fatal(err)
	}
	key, err := box.Encrypt(merchantKey)
	if err != nil {
		t.Fatal(err)
	}
	salt, err := box.Encrypt(merchantSalt)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := d.Pool.Exec(ctx, `UPDATE payment_settings SET active_provider='paytr',test_mode=$1 WHERE id=1`, testMode); err != nil {
		t.Fatal(err)
	}
	if _, err := d.Pool.Exec(ctx, `UPDATE payment_provider_settings SET enabled=true,merchant_id=$1,merchant_key_enc=$2,merchant_salt_enc=$3,base_url=$4 WHERE provider='paytr'`, merchantID, key, salt, fake.URL); err != nil {
		t.Fatal(err)
	}
	registry := payment.NewRegistry(payment.NewDBSettingsReader(d.Queries, box))
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	svc := usecase.New(d.Queries, registry, "https://comcode.test", "https://comcode.test/api", log, repository.NewPostgres(d.Pool))
	tokens, err := jwt.NewManager("fixture-jwt-secret", time.Hour, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	bearer, _, err := tokens.IssueAccess(jwt.AccessInput{UserID: user.UUID})
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	billing.RegisterRoutes(mux, handler.New(svc, "https://comcode.test", log), tokens, identity{principal: authctx.Principal{
		UserID: user.UUID, UserInternal: user.ID, Email: "payer@example.test", Permissions: []string{rbac.PermAuthSession, rbac.PermAppBillingWrite},
		Account: authctx.Account{ID: user.AccountID, Role: authctx.AccountRoleOwner},
	}})
	req := httptest.NewRequest(http.MethodPost, "/v1/billing/checkout", strings.NewReader(fmt.Sprintf(`{"plan_uuid":%q}`, plan)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+bearer)
	req.RemoteAddr = "192.0.2.9:1234"
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	f := fixture{d: d, user: user, mux: mux}
	if tokenStatus != http.StatusOK {
		if rec.Code < 400 {
			t.Fatalf("token HTTP failure accepted: %d %s", rec.Code, rec.Body)
		}
		if err := d.Pool.QueryRow(ctx, `SELECT uuid,merchant_oid FROM payment_checkout_sessions WHERE account_id=$1`, user.AccountID).Scan(&f.session, &f.oid); err != nil {
			t.Fatal(err)
		}
		f.assertState(t, "failed", 0)
		return f
	}
	if rec.Code != http.StatusCreated {
		t.Fatalf("checkout: %d %s", rec.Code, rec.Body)
	}
	var body struct {
		Data usecase.StartCheckoutResult `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Data.Session.Status != "pending" || body.Data.Session.IframeToken == nil || *body.Data.Session.IframeToken != "fixture-iframe-token" {
		t.Fatalf("checkout response: %s", rec.Body)
	}
	f.session = body.Data.Session.UUID
	if err := d.Pool.QueryRow(ctx, `SELECT merchant_oid FROM payment_checkout_sessions WHERE uuid=$1`, f.session).Scan(&f.oid); err != nil {
		t.Fatal(err)
	}
	return f
}

func (f fixture) payload(status string) url.Values {
	total := "12345"
	if status == "failed" {
		total = "0"
	}
	p := url.Values{"merchant_oid": {f.oid}, "status": {status}, "total_amount": {total}, "currency": {"TL"},
		"payment_amount": {"12345"}, "failed_reason_code": {"0"}, "failed_reason_msg": {"Fixture decline"}}
	resign(p)
	return p
}
func resign(p url.Values) {
	p.Set("hash", sign(p.Get("merchant_oid")+merchantSalt+p.Get("status")+p.Get("total_amount")))
}
func (f fixture) callback(p url.Values) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/v1/public/payment/paytr/callback", strings.NewReader(p.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	// No Authorization, browser cookies, or CSRF token: this is a public provider notification.
	rec := httptest.NewRecorder()
	f.mux.ServeHTTP(rec, req)
	return rec
}
func checkReply(t *testing.T, rec *httptest.ResponseRecorder, status int, body string) {
	t.Helper()
	if rec.Code != status || rec.Body.String() != body || rec.Header().Get("Content-Type") != "text/plain; charset=utf-8" {
		t.Fatalf("callback: %d %q %v", rec.Code, rec.Body.String(), rec.Header())
	}
}
func (f fixture) assertState(t *testing.T, status string, subscriptions int) {
	t.Helper()
	row, err := f.d.Queries.GetCheckoutSessionByMerchantOID(context.Background(), f.oid)
	if err != nil {
		t.Fatal(err)
	}
	if string(row.Status) != status {
		t.Fatalf("status=%s want %s", row.Status, status)
	}
	var count int
	if err := f.d.Pool.QueryRow(context.Background(), `SELECT count(*) FROM subscriptions WHERE account_id=$1`, f.user.AccountID).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != subscriptions {
		t.Fatalf("subscriptions=%d want %d", count, subscriptions)
	}
}

func TestPayTRCheckoutCallback(t *testing.T) {
	t.Run("success and concurrent duplicates", func(t *testing.T) {
		f := checkout(t, true, 200)
		p := f.payload("success")
		p.Set("utoken", "fixture-customer")
		p.Set("ctoken", "fixture-card")
		p.Set("last_4", "4242")
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
		f.assertState(t, "completed", 1)
		p.Set("ctoken", "changed-duplicate-card")
		checkReply(t, f.callback(p), 200, "OK")
		cards, err := f.d.Queries.ListPaymentSavedCardsByAccount(context.Background(), f.user.AccountID)
		if err != nil || len(cards) != 1 || cards[0].ProviderCardRef != "fixture-card" {
			t.Fatalf("saved-card duplicate effects: %+v %v", cards, err)
		}
		var active int
		if err := f.d.Pool.QueryRow(context.Background(), `SELECT count(*) FROM subscriptions WHERE account_id=$1 AND source='payment' AND provider='paytr' AND status='active' AND checkout_session_id=(SELECT id FROM payment_checkout_sessions WHERE uuid=$2)`, f.user.AccountID, f.session).Scan(&active); err != nil || active != 1 {
			t.Fatalf("paid subscription count=%d err=%v", active, err)
		}
		checkReply(t, f.callback(f.payload("failed")), 200, "OK")
		p.Set("hash", "tampered")
		checkReply(t, f.callback(p), 400, "FAIL")
		f.assertState(t, "completed", 1)
	})
	t.Run("failed and duplicates preserve first reason", func(t *testing.T) {
		f := checkout(t, true, 200)
		p := f.payload("failed")
		p.Del("currency")
		p.Del("payment_amount")
		checkReply(t, f.callback(p), 200, "OK")
		p.Set("failed_reason_msg", "Changed duplicate")
		checkReply(t, f.callback(p), 200, "OK")
		checkReply(t, f.callback(f.payload("success")), 200, "OK")
		f.assertState(t, "failed", 0)
		row, err := f.d.Queries.GetCheckoutSessionByMerchantOID(context.Background(), f.oid)
		if err != nil {
			t.Fatal(err)
		}
		var metadata map[string]string
		if err := json.Unmarshal(row.Metadata, &metadata); err != nil {
			t.Fatal(err)
		}
		if metadata["failed_reason_code"] != "0" || metadata["failed_reason_msg"] != "Fixture decline" {
			t.Fatalf("failure metadata=%s", row.Metadata)
		}
	})
	for _, tc := range []struct {
		name, key, value string
		signed           bool
	}{
		{"bad hash", "hash", "tampered", false},
		{"underpayment", "total_amount", "12344", true},
		{"order amount mismatch", "payment_amount", "12346", true},
		{"currency mismatch", "currency", "USD", true},
		{"missing currency", "currency", "", true},
		{"missing order amount", "payment_amount", "", true},
		{"invalid amount", "total_amount", "12.34", true},
		{"negative amount", "total_amount", "-1", true},
		{"unknown status", "status", "unknown", true},
		{"unknown order", "merchant_oid", "chkunknown", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := checkout(t, true, 200)
			p := f.payload("success")
			p.Set(tc.key, tc.value)
			if tc.signed {
				resign(p)
			}
			checkReply(t, f.callback(p), 400, "FAIL")
			f.assertState(t, "pending", 0)
		})
	}
	t.Run("installment fees and TRY alias in live config", func(t *testing.T) {
		f := checkout(t, false, 200)
		p := f.payload("success")
		p.Set("total_amount", "13000")
		p.Set("currency", "TRY")
		resign(p)
		checkReply(t, f.callback(p), 200, "OK")
		f.assertState(t, "completed", 1)
	})
	t.Run("provider mismatch", func(t *testing.T) {
		f := checkout(t, true, 200)
		if _, err := f.d.Pool.Exec(context.Background(), `UPDATE payment_checkout_sessions SET provider='iyzico' WHERE uuid=$1`, f.session); err != nil {
			t.Fatal(err)
		}
		checkReply(t, f.callback(f.payload("success")), 400, "FAIL")
		f.assertState(t, "pending", 0)
	})
	t.Run("malformed form", func(t *testing.T) {
		f := checkout(t, true, 200)
		req := httptest.NewRequest(http.MethodPost, "/v1/public/payment/paytr/callback", strings.NewReader("hash=%zz"))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rec := httptest.NewRecorder()
		f.mux.ServeHTTP(rec, req)
		checkReply(t, rec, 400, "FAIL")
		f.assertState(t, "pending", 0)
	})
	t.Run("provider HTTP error", func(t *testing.T) { checkout(t, true, 502) })
	t.Run("transaction rollback permits retry", func(t *testing.T) {
		f := checkout(t, true, 200)
		var previous uuid.UUID
		if err := f.d.Pool.QueryRow(context.Background(), `INSERT INTO subscriptions (account_id,plan_id,status,source)
            SELECT account_id,plan_id,'active','manual' FROM payment_checkout_sessions WHERE uuid=$1 RETURNING uuid`, f.session).Scan(&previous); err != nil {
			t.Fatal(err)
		}
		constraint := "tec591_" + pgtest.Unique()
		// Force subscription creation to fail after session completion; neither write may survive.
		sql := fmt.Sprintf(`ALTER TABLE subscriptions ADD CONSTRAINT %s CHECK (account_id <> %d OR source = 'manual')`, constraint, f.user.AccountID)
		if _, err := f.d.Pool.Exec(context.Background(), sql); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			_, err := f.d.Pool.Exec(context.Background(), `ALTER TABLE subscriptions DROP CONSTRAINT IF EXISTS `+constraint)
			if err != nil {
				t.Error(err)
			}
		})
		checkReply(t, f.callback(f.payload("success")), 400, "FAIL")
		f.assertState(t, "pending", 1)
		var previousStatus string
		if err := f.d.Pool.QueryRow(context.Background(), `SELECT status FROM subscriptions WHERE uuid=$1`, previous).Scan(&previousStatus); err != nil || previousStatus != "active" {
			t.Fatalf("previous subscription after rollback=%s %v", previousStatus, err)
		}
		if _, err := f.d.Pool.Exec(context.Background(), `ALTER TABLE subscriptions DROP CONSTRAINT `+constraint); err != nil {
			t.Fatal(err)
		}
		checkReply(t, f.callback(f.payload("success")), 200, "OK")
		f.assertState(t, "completed", 2)
		if err := f.d.Pool.QueryRow(context.Background(), `SELECT status FROM subscriptions WHERE uuid=$1`, previous).Scan(&previousStatus); err != nil || previousStatus != "canceled" {
			t.Fatalf("previous subscription after success=%s %v", previousStatus, err)
		}
	})
}

func TestPeriodEndCancellationHTTPContract(t *testing.T) {
	f := checkout(t, true, http.StatusOK)
	if rec := f.callback(f.payload("success")); rec.Code != http.StatusOK {
		t.Fatalf("callback %d %s", rec.Code, rec.Body)
	}
	tokens, err := jwt.NewManager("fixture-jwt-secret", time.Hour, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	bearer, _, err := tokens.IssueAccess(jwt.AccessInput{UserID: f.user.UUID})
	if err != nil {
		t.Fatal(err)
	}
	for _, step := range []struct {
		method, path string
		want         int
	}{
		{method: "DELETE", path: "/v1/billing/subscription", want: 204},
		{method: "GET", path: "/v1/billing/subscription", want: 200},
		{method: "POST", path: "/v1/billing/subscription/resume", want: 204},
		{method: "PATCH", path: "/v1/billing/subscription/plan", want: 400},
	} {
		req := httptest.NewRequest(step.method, step.path, strings.NewReader(`{}`))
		req.Header.Set("Authorization", "Bearer "+bearer)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		f.mux.ServeHTTP(rec, req)
		if rec.Code != step.want {
			t.Fatalf("%s %s: %d %s", step.method, step.path, rec.Code, rec.Body)
		}
		if step.method == "GET" {
			var body struct {
				Data struct {
					Subscription usecase.UserSubscriptionView `json:"subscription"`
				} `json:"data"`
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			if !body.Data.Subscription.CancelAtPeriodEnd || body.Data.Subscription.Status != "active" || body.Data.Subscription.PeriodEnd == nil {
				t.Fatalf("cancel response %s", rec.Body)
			}
		}
	}
}
