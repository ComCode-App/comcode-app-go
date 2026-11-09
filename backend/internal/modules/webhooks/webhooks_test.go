package webhooks_test

// TEC-603: endpoint API, fan-out, signed delivery, retries and resend. Real PostgreSQL
// (TEST_DATABASE_URL) and an httptest receiver; deliveries are run in-process (no Asynq).
import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/webhooks"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/webhooks/handler"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/webhooks/model"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/webhooks/usecase"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/authctx"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/crypto"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/jwt"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/rbac"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/testutil/pgtest"
)

func TestMain(m *testing.M) { pgtest.Main(m) }

type identity struct {
	mu sync.Mutex
	p  authctx.Principal
}

func (i *identity) LoadPrincipal(_ *http.Request, _ jwt.Claims) (authctx.Principal, error) {
	i.mu.Lock()
	defer i.mu.Unlock()
	return i.p, nil
}

type received struct {
	body    []byte
	headers http.Header
}

type fixture struct {
	t        *testing.T
	d        *pgtest.DB
	svc      *usecase.Service
	mux      http.Handler
	who      *identity
	bearer   string
	user     pgtest.User
	receiver *httptest.Server
	secret   atomic.Value // string
	failNext atomic.Int32 // number of 500 answers to give
	got      chan received
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	d := pgtest.Open(t)
	user := d.CreateUser(t)
	box, err := crypto.NewSecretBox(strings.Repeat("k", 32))
	if err != nil {
		t.Fatal(err)
	}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	svc := usecase.New(d.Queries, box, nil, log).AllowInsecure(true)
	tokens, err := jwt.NewManager("fixture-jwt-secret", time.Hour, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	bearer, _, err := tokens.IssueAccess(jwt.AccessInput{UserID: user.UUID})
	if err != nil {
		t.Fatal(err)
	}
	f := &fixture{t: t, d: d, svc: svc, who: &identity{}, bearer: bearer, user: user, got: make(chan received, 16)}
	f.as(user, authctx.AccountRoleOwner)
	f.receiver = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		secret, _ := f.secret.Load().(string)
		if err := usecase.Verify(secret, r.Header.Get(usecase.HeaderSignature), body, time.Now(), 5*time.Minute); err != nil {
			t.Errorf("receiver: %v", err)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		f.got <- received{body: body, headers: r.Header.Clone()}
		if f.failNext.Load() > 0 {
			f.failNext.Add(-1)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(f.receiver.Close)
	mux := http.NewServeMux()
	webhooks.RegisterRoutes(mux, handler.New(svc), tokens, f.who)
	f.mux = mux
	return f
}

func (f *fixture) as(u pgtest.User, role string) {
	f.who.mu.Lock()
	defer f.who.mu.Unlock()
	f.who.p = authctx.Principal{
		UserID: u.UUID, UserInternal: u.ID, Email: "owner@example.test",
		Permissions: []string{rbac.PermAuthSession, rbac.PermAppWebhooksRead, rbac.PermAppWebhooksWrite},
		Account:     authctx.Account{ID: u.AccountID, Role: role},
	}
}

func (f *fixture) do(method, path string, body any) *httptest.ResponseRecorder {
	f.t.Helper()
	raw := "{}"
	if body != nil {
		b, _ := json.Marshal(body)
		raw = string(b)
	}
	req := httptest.NewRequest(method, path, strings.NewReader(raw))
	req.Header.Set("Authorization", "Bearer "+f.bearer)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	f.mux.ServeHTTP(rec, req)
	return rec
}

func decodeData[T any](t *testing.T, rec *httptest.ResponseRecorder) T {
	t.Helper()
	var env struct {
		Data T `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("decode %s: %v", rec.Body, err)
	}
	return env.Data
}

// pendingDeliveryIDs returns the ids of deliveries the worker would pick up, oldest first.
func (f *fixture) deliveryIDs(status string) []int64 {
	f.t.Helper()
	rows, err := f.d.Pool.Query(context.Background(),
		`SELECT id FROM webhook_deliveries WHERE account_id=$1 AND status=$2::webhook_delivery_status ORDER BY id`, f.user.AccountID, status)
	if err != nil {
		f.t.Fatal(err)
	}
	defer rows.Close()
	var ids []int64
	for rows.Next() {
		var id int64
		_ = rows.Scan(&id)
		ids = append(ids, id)
	}
	return ids
}

func TestWebhookEndpointsAndDeliveries(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()

	// Registry.
	if rec := f.do(http.MethodGet, "/v1/webhooks/events", nil); rec.Code != 200 || !strings.Contains(rec.Body.String(), `"lead.created"`) {
		t.Fatalf("events: %d %s", rec.Code, rec.Body)
	}

	// Validation: unknown event, bad URL.
	if rec := f.do(http.MethodPost, "/v1/webhooks", map[string]any{"url": f.receiver.URL, "events": []string{"nope"}}); rec.Code != 400 {
		t.Fatalf("unknown event: %d %s", rec.Code, rec.Body)
	}
	if rec := f.do(http.MethodPost, "/v1/webhooks", map[string]any{"url": "ftp://x", "events": []string{"lead.created"}}); rec.Code != 400 {
		t.Fatalf("bad url: %d %s", rec.Code, rec.Body)
	}

	// Create: the secret comes back once.
	rec := f.do(http.MethodPost, "/v1/webhooks", map[string]any{
		"url": f.receiver.URL + "/hook", "description": "CRM", "events": []string{"lead.created", "lead.created", "order.created"},
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", rec.Code, rec.Body)
	}
	created := decodeData[model.EndpointWithSecret](t, rec)
	if !strings.HasPrefix(created.Secret, "whsec_") || len(created.Events) != 2 || created.SecretHint != created.Secret[len(created.Secret)-6:] {
		t.Fatalf("created: %+v", created)
	}
	f.secret.Store(created.Secret)
	if rec := f.do(http.MethodGet, "/v1/webhooks", nil); rec.Code != 200 || strings.Contains(rec.Body.String(), "whsec_") {
		t.Fatalf("list must not expose the secret: %d %s", rec.Code, rec.Body)
	}

	// Dispatch: subscribed event fans out, unsubscribed one does not.
	f.svc.Dispatch(ctx, f.user.AccountID, "card.scanned", map[string]any{"path": "/c/x"})
	f.svc.Dispatch(ctx, f.user.AccountID, "lead.created", map[string]any{"lead_uuid": "L1", "name": "Ada"})
	pending := f.deliveryIDs("pending")
	if len(pending) != 1 {
		t.Fatalf("pending deliveries: %v", pending)
	}

	// Deliver: signed POST, 2xx marks it succeeded.
	if err := f.svc.Deliver(ctx, pending[0]); err != nil {
		t.Fatal(err)
	}
	got := <-f.got
	var env model.Envelope
	if err := json.Unmarshal(got.body, &env); err != nil {
		t.Fatal(err)
	}
	if env.Event != "lead.created" || env.Attempt != 1 || env.Data["name"] != "Ada" ||
		got.headers.Get(usecase.HeaderEvent) != "lead.created" || got.headers.Get(usecase.HeaderDelivery) != env.ID.String() ||
		got.headers.Get("User-Agent") != usecase.UserAgent {
		t.Fatalf("envelope: %+v headers=%v", env, got.headers)
	}
	if ids := f.deliveryIDs("succeeded"); len(ids) != 1 {
		t.Fatalf("succeeded: %v", ids)
	}
	rec = f.do(http.MethodGet, "/v1/webhooks/"+created.UUID.String()+"/deliveries", nil)
	page := decodeData[model.DeliveryPage](t, rec)
	if rec.Code != 200 || page.Total != 1 || page.Items[0].Status != "succeeded" || page.Items[0].LastStatusCode == nil || *page.Items[0].LastStatusCode != 204 {
		t.Fatalf("deliveries: %d %s", rec.Code, rec.Body)
	}

	// Retry: a 500 leaves it failed with a later attempt; after MaxAttempts it is exhausted.
	f.failNext.Store(int32(usecase.MaxAttempts))
	f.svc.Dispatch(ctx, f.user.AccountID, "order.created", map[string]any{"order_number": "ORD-1"})
	id := f.deliveryIDs("pending")[0]
	for i := 1; i <= usecase.MaxAttempts; i++ {
		// The worker runs each attempt when its backoff has elapsed; the test skips the wait.
		if _, err := f.d.Pool.Exec(ctx, `UPDATE webhook_deliveries SET next_attempt_at = NOW() WHERE id=$1`, id); err != nil {
			t.Fatal(err)
		}
		if err := f.svc.Deliver(ctx, id); err != nil {
			t.Fatal(err)
		}
		<-f.got
		var status string
		var attempts int
		var next time.Time
		if err := f.d.Pool.QueryRow(ctx, `SELECT status::text, attempts, next_attempt_at FROM webhook_deliveries WHERE id=$1`, id).Scan(&status, &attempts, &next); err != nil {
			t.Fatal(err)
		}
		switch {
		case i < usecase.MaxAttempts && (status != "failed" || attempts != i || !next.After(time.Now())):
			t.Fatalf("attempt %d: status=%s attempts=%d next=%s", i, status, attempts, next)
		case i == usecase.MaxAttempts && (status != "exhausted" || attempts != i):
			t.Fatalf("final attempt: status=%s attempts=%d", status, attempts)
		}
	}
	var failures int
	if err := f.d.Pool.QueryRow(ctx, `SELECT consecutive_failures FROM webhook_endpoints WHERE uuid=$1`, created.UUID).Scan(&failures); err != nil {
		t.Fatal(err)
	}
	if failures != usecase.MaxAttempts {
		t.Fatalf("consecutive_failures=%d", failures)
	}

	// A delivery that is still scheduled cannot be re-sent; an exhausted one can. The receiver is
	// healthy again, so the resend succeeds.
	f.svc.Dispatch(ctx, f.user.AccountID, "lead.created", map[string]any{"lead_uuid": "L2"})
	queued := f.deliveryIDs("pending")[0]
	var queuedUUID string
	if err := f.d.Pool.QueryRow(ctx, `SELECT uuid::text FROM webhook_deliveries WHERE id=$1`, queued).Scan(&queuedUUID); err != nil {
		t.Fatal(err)
	}
	if rec := f.do(http.MethodPost, "/v1/webhooks/"+created.UUID.String()+"/deliveries/"+queuedUUID+"/resend", nil); rec.Code != http.StatusConflict {
		t.Fatalf("resend of a queued delivery: %d %s", rec.Code, rec.Body)
	}
	// A task that fires before its retry time, or twice, claims nothing and does not POST.
	if _, err := f.d.Pool.Exec(ctx, `UPDATE webhook_deliveries SET next_attempt_at = NOW() + INTERVAL '1 hour' WHERE id=$1`, queued); err != nil {
		t.Fatal(err)
	}
	if err := f.svc.Deliver(ctx, queued); err != nil {
		t.Fatal(err)
	}
	select {
	case got := <-f.got:
		t.Fatalf("early task delivered: %s", got.headers.Get(usecase.HeaderEvent))
	case <-time.After(100 * time.Millisecond):
	}
	if _, err := f.d.Pool.Exec(ctx, `UPDATE webhook_deliveries SET next_attempt_at = NOW() WHERE id=$1`, queued); err != nil {
		t.Fatal(err)
	}
	if err := f.svc.Deliver(ctx, queued); err != nil {
		t.Fatal(err)
	}
	<-f.got
	rec = f.do(http.MethodGet, "/v1/webhooks/"+created.UUID.String()+"/deliveries?status=exhausted", nil)
	exhausted := decodeData[model.DeliveryPage](t, rec)
	if exhausted.Total != 1 {
		t.Fatalf("exhausted page: %s", rec.Body)
	}
	rec = f.do(http.MethodPost, "/v1/webhooks/"+created.UUID.String()+"/deliveries/"+exhausted.Items[0].UUID.String()+"/resend", nil)
	if rec.Code != http.StatusAccepted || !strings.Contains(rec.Body.String(), `"attempts":0`) {
		t.Fatalf("resend: %d %s", rec.Code, rec.Body)
	}
	if err := f.svc.Deliver(ctx, id); err != nil {
		t.Fatal(err)
	}
	<-f.got
	if ids := f.deliveryIDs("succeeded"); len(ids) != 3 {
		t.Fatalf("after resend: %v", ids)
	}
	if err := f.d.Pool.QueryRow(ctx, `SELECT consecutive_failures FROM webhook_endpoints WHERE uuid=$1`, created.UUID).Scan(&failures); err != nil || failures != 0 {
		t.Fatalf("failures reset: %d %v", failures, err)
	}

	// Test ping ignores subscriptions; rotate changes the secret and the receiver still verifies.
	if rec := f.do(http.MethodPost, "/v1/webhooks/"+created.UUID.String()+"/test", nil); rec.Code != http.StatusAccepted {
		t.Fatalf("test: %d %s", rec.Code, rec.Body)
	}
	rec = f.do(http.MethodPost, "/v1/webhooks/"+created.UUID.String()+"/rotate-secret", nil)
	rotated := decodeData[model.EndpointWithSecret](t, rec)
	if rec.Code != 200 || rotated.Secret == created.Secret || !strings.HasPrefix(rotated.Secret, "whsec_") {
		t.Fatalf("rotate: %d %s", rec.Code, rec.Body)
	}
	f.secret.Store(rotated.Secret)
	if err := f.svc.Deliver(ctx, f.deliveryIDs("pending")[0]); err != nil {
		t.Fatal(err)
	}
	ping := <-f.got
	if ping.headers.Get(usecase.HeaderEvent) != "webhook.test" {
		t.Fatalf("ping event: %s", ping.headers.Get(usecase.HeaderEvent))
	}

	// Update: deactivating stops deliveries (an existing pending one is exhausted on pickup).
	rec = f.do(http.MethodPut, "/v1/webhooks/"+created.UUID.String(), map[string]any{
		"url": f.receiver.URL + "/hook2", "events": []string{"card.scanned"}, "is_active": false,
	})
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"is_active":false`) || !strings.Contains(rec.Body.String(), "/hook2") {
		t.Fatalf("update: %d %s", rec.Code, rec.Body)
	}
	f.svc.Dispatch(ctx, f.user.AccountID, "card.scanned", map[string]any{"path": "/c/y"})
	if ids := f.deliveryIDs("pending"); len(ids) != 0 {
		t.Fatalf("inactive endpoint got deliveries: %v", ids)
	}

	// Access: a member cannot write; another account cannot see it; delete hides it.
	f.as(f.user, authctx.AccountRoleMember)
	if rec := f.do(http.MethodDelete, "/v1/webhooks/"+created.UUID.String(), nil); rec.Code != http.StatusForbidden {
		t.Fatalf("member delete: %d", rec.Code)
	}
	other := f.d.CreateUser(t)
	f.as(other, authctx.AccountRoleOwner)
	if rec := f.do(http.MethodGet, "/v1/webhooks/"+created.UUID.String(), nil); rec.Code != http.StatusNotFound {
		t.Fatalf("other account read: %d", rec.Code)
	}
	if rec := f.do(http.MethodGet, "/v1/webhooks/"+created.UUID.String()+"/deliveries", nil); rec.Code != http.StatusNotFound {
		t.Fatalf("other account deliveries: %d", rec.Code)
	}
	f.as(f.user, authctx.AccountRoleOwner)
	if rec := f.do(http.MethodDelete, "/v1/webhooks/"+created.UUID.String(), nil); rec.Code != http.StatusNoContent {
		t.Fatalf("delete: %d %s", rec.Code, rec.Body)
	}
	if rec := f.do(http.MethodGet, "/v1/webhooks", nil); !strings.Contains(rec.Body.String(), `"items":[]`) {
		t.Fatalf("after delete: %s", rec.Body)
	}
}

func TestWebhookEndpointLimit(t *testing.T) {
	f := newFixture(t)
	for i := 0; i < usecase.MaxEndpointsPerAccount; i++ {
		if rec := f.do(http.MethodPost, "/v1/webhooks", map[string]any{"url": f.receiver.URL, "events": []string{"lead.created"}}); rec.Code != 201 {
			t.Fatalf("create %d: %d %s", i, rec.Code, rec.Body)
		}
	}
	if rec := f.do(http.MethodPost, "/v1/webhooks", map[string]any{"url": f.receiver.URL, "events": []string{"lead.created"}}); rec.Code != http.StatusConflict {
		t.Fatalf("limit: %d %s", rec.Code, rec.Body)
	}
}
