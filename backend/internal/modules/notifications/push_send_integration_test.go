package notifications

// TEC-1187 (mobile B10): Expo push sending, receipts and cleanup against real PostgreSQL
// (TEST_DATABASE_URL) and a fake Expo server. Only live devices (not disabled, session family
// still has a live refresh token) of users with push_enabled get the push; security emails and
// sealed rows never do.

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	authrepo "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/auth/repository"
	authusecase "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/auth/usecase"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/notifications/handler"
	notifmodel "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/notifications/model"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/notifications/usecase"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/crypto"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/jwt"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/testutil/pgtest"
	"github.com/google/uuid"
)

type expoMsg struct {
	To    string         `json:"to"`
	Title string         `json:"title"`
	Body  string         `json:"body"`
	Data  map[string]any `json:"data"`
}

// fakeExpoServer records pushes. Tokens containing "dead" get a DeviceNotRegistered ticket;
// receipts answers getReceipts.
type fakeExpoServer struct {
	mu       sync.Mutex
	sent     []expoMsg
	calls    int
	receipts map[string]any
}

func (f *fakeExpoServer) start(t *testing.T) string {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		f.calls++
		switch r.URL.Path {
		case "/push/send":
			var msgs []expoMsg
			if err := json.NewDecoder(r.Body).Decode(&msgs); err != nil {
				t.Errorf("decode: %v", err)
			}
			data := make([]map[string]any, 0, len(msgs))
			for _, m := range msgs {
				f.sent = append(f.sent, m)
				if strings.Contains(m.To, "dead") {
					data = append(data, map[string]any{"status": "error", "details": map[string]string{"error": "DeviceNotRegistered"}})
					continue
				}
				data = append(data, map[string]any{"status": "ok", "id": "tk-" + uuid.NewString()})
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"data": data})
		case "/push/getReceipts":
			_ = json.NewEncoder(w).Encode(map[string]any{"data": f.receipts})
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	return srv.URL
}

func (f *fakeExpoServer) sentTo() map[string]expoMsg {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := map[string]expoMsg{}
	for _, m := range f.sent {
		out[m.To] = m
	}
	return out
}

type sendFixture struct {
	*pushFixture
	svc  *usecase.Service
	expo *fakeExpoServer
}

func newSendFixture(t *testing.T) *sendFixture {
	t.Helper()
	d := pgtest.Open(t)
	expo := &fakeExpoServer{}
	base := expo.start(t)
	box, err := crypto.NewSecretBox(strings.Repeat("k", 32))
	if err != nil {
		t.Fatal(err)
	}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	// No queue: Enqueue delivers inline and the push is sent inline.
	svc := usecase.New(d.Queries, nil, nil, log).WithSecretBox(box).
		WithExpo(usecase.ExpoConfig{BaseURL: base, Backoff: time.Millisecond})
	tokens, err := jwt.NewManager("push-send-fixture-jwt-secret!!!!", time.Minute, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	pf := &pushFixture{d: d, h: handler.New(svc), auth: authusecase.New(authrepo.NewPostgres(d.Pool, d.Queries), tokens)}
	return &sendFixture{pushFixture: pf, svc: svc, expo: expo}
}

func (f *sendFixture) notifyLead(t *testing.T, u pgtest.User) uuid.UUID {
	t.Helper()
	action := "/leads?lead=" + uuid.NewString()
	rows, err := f.svc.Enqueue(context.Background(), notifmodel.EnqueueInput{
		UserID: &u.ID, AccountID: &u.AccountID, Channels: []string{notifmodel.ChannelInapp},
		TemplateCode: "leads.received", TemplateVars: map[string]string{"name": "Ayşe Yılmaz"},
		ActionURL: &action, SourceEvent: "leads.received",
	})
	if err != nil || len(rows) != 1 {
		t.Fatalf("enqueue: %v (%d rows)", err, len(rows))
	}
	return rows[0].UUID
}

func (f *sendFixture) count(t *testing.T, sql string, args ...any) int {
	t.Helper()
	var n int
	if err := f.d.Pool.QueryRow(context.Background(), sql, args...).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestIntegrationPushSendOnlyToLiveDevices(t *testing.T) {
	f := newSendFixture(t)
	ctx := context.Background()
	u := f.d.CreateUser(t)
	live, loggedOut, expired, revoked, dead := f.signIn(t, u), f.signIn(t, u), f.signIn(t, u), f.signIn(t, u), f.signIn(t, u)
	liveTok, outTok, expTok, revTok := pushToken(), pushToken(), pushToken(), pushToken()
	deadTok := "ExponentPushToken[dead-" + pgtest.Unique() + "]"
	for s, tok := range map[*pushSession]string{&live: liveTok, &loggedOut: outTok, &expired: expTok, &revoked: revTok, &dead: deadTok} {
		f.mustRegister(t, *s, tok)
	}
	if err := f.auth.Logout(ctx, loggedOut.refresh); err != nil {
		t.Fatal(err)
	}
	if err := f.auth.RevokeSession(ctx, u.ID, revoked.p.SessionID); err != nil {
		t.Fatal(err)
	}
	// An expired family is not marked disabled; the sender must still skip it.
	if _, err := f.d.Pool.Exec(ctx, `UPDATE refresh_tokens SET expires_at = NOW() - interval '1 minute' WHERE family_uuid = $1`,
		expired.p.SessionID); err != nil {
		t.Fatal(err)
	}
	f.expectActive(t, expTok)

	notif := f.notifyLead(t, u)

	sent := f.expo.sentTo()
	if len(sent) != 2 {
		t.Fatalf("pushed to %d devices (%v), want the live and the dead one", len(sent), keys(sent))
	}
	m, ok := sent[liveTok]
	if !ok {
		t.Fatalf("live device not pushed: %v", keys(sent))
	}
	if m.Title != "Yeni iletişim talebi" || strings.Contains(m.Title+m.Body, "Ayşe") {
		t.Fatalf("push text = %+v", m)
	}
	var accountUUID string
	if err := f.d.Pool.QueryRow(ctx, `SELECT uuid::text FROM accounts WHERE id = $1`, u.AccountID).Scan(&accountUUID); err != nil {
		t.Fatal(err)
	}
	if m.Data["notification_uuid"] != notif.String() || m.Data["account_uuid"] != accountUUID ||
		!strings.HasPrefix(m.Data["url"].(string), "/leads?lead=") || len(m.Data) != 3 {
		t.Fatalf("data = %v", m.Data)
	}
	// The ticket is stored; DeviceNotRegistered disabled the dead device.
	if n := f.count(t, `SELECT COUNT(*) FROM push_tickets t JOIN push_devices d ON d.id = t.push_device_id WHERE d.token = $1`, liveTok); n != 1 {
		t.Fatalf("live device tickets = %d", n)
	}
	f.expectDisabled(t, deadTok, "device_not_registered")

	// A retried task does not push a device twice.
	var id int64
	if err := f.d.Pool.QueryRow(ctx, `SELECT id FROM notifications WHERE uuid = $1`, notif).Scan(&id); err != nil {
		t.Fatal(err)
	}
	before := f.expo.calls
	if err := f.svc.SendMobilePush(ctx, id); err != nil {
		t.Fatal(err)
	}
	if f.expo.calls != before {
		t.Fatalf("retry pushed again (%d calls)", f.expo.calls-before)
	}
}

func TestIntegrationPushSendHonorsPushEnabled(t *testing.T) {
	f := newSendFixture(t)
	u := f.d.CreateUser(t)
	f.mustRegister(t, f.signIn(t, u), pushToken()) // first device turns push on
	if _, err := f.d.Pool.Exec(context.Background(), `UPDATE notification_preferences SET push_enabled = FALSE WHERE user_id = $1`, u.ID); err != nil {
		t.Fatal(err)
	}
	f.notifyLead(t, u)
	if f.expo.calls != 0 {
		t.Fatalf("pushed with push_enabled off (%d calls)", f.expo.calls)
	}
}

func TestIntegrationSecurityNotificationsAreNeverPushed(t *testing.T) {
	f := newSendFixture(t)
	ctx := context.Background()
	u := f.d.CreateUser(t)
	f.mustRegister(t, f.signIn(t, u), pushToken())

	// A security email (even asked for on in-app too) and an invitation never reach Expo.
	if _, err := f.svc.Enqueue(ctx, notifmodel.EnqueueInput{
		UserID: &u.ID, Channels: []string{notifmodel.ChannelEmail, notifmodel.ChannelInapp},
		TemplateCode: "auth.password_reset", SecurityEmail: true,
		TemplateVars: map[string]string{"code": "482913", "name": "Test"},
	}); err != nil {
		t.Fatal(err)
	}
	// A sealed in-app row with a pushable code is refused by the sender too.
	var id int64
	if err := f.d.Pool.QueryRow(ctx, `
		INSERT INTO notifications (user_id, channel, status, priority, title, body, payload, template_code, sealed_content)
		VALUES ($1, 'inapp', 'sent', 'normal', 'x', 'x', '{}', 'leads.received', 'v1:sealed') RETURNING id`, u.ID).Scan(&id); err != nil {
		t.Fatal(err)
	}
	if err := f.svc.SendMobilePush(ctx, id); err != nil {
		t.Fatal(err)
	}
	if f.expo.calls != 0 {
		t.Fatalf("security notification pushed: %+v", f.expo.sentTo())
	}
	if n := f.count(t, `SELECT COUNT(*) FROM push_tickets t JOIN push_devices d ON d.id = t.push_device_id WHERE d.user_id = $1`, u.ID); n != 0 {
		t.Fatalf("tickets = %d", n)
	}
}

func TestIntegrationPushReceiptsDisableDeviceNotRegistered(t *testing.T) {
	f := newSendFixture(t)
	ctx := context.Background()
	u := f.d.CreateUser(t)
	a, b := f.signIn(t, u), f.signIn(t, u)
	ta, tb := pushToken(), pushToken()
	f.mustRegister(t, a, ta)
	f.mustRegister(t, b, tb)
	f.notifyLead(t, u)

	tickets := map[string]string{}
	rows, err := f.d.Pool.Query(ctx, `SELECT d.token, t.ticket_id FROM push_tickets t JOIN push_devices d ON d.id = t.push_device_id WHERE d.user_id = $1`, u.ID)
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var tok, id string
		if err := rows.Scan(&tok, &id); err != nil {
			t.Fatal(err)
		}
		tickets[tok] = id
	}
	rows.Close()
	if len(tickets) != 2 {
		t.Fatalf("tickets = %v", tickets)
	}
	// Fresh tickets are not read yet.
	if n, err := f.svc.ProcessPushReceipts(ctx); err != nil || n != 0 {
		t.Fatalf("early receipts: %d %v", n, err)
	}
	if _, err := f.d.Pool.Exec(ctx, `UPDATE push_tickets SET created_at = NOW() - interval '16 minutes'`); err != nil {
		t.Fatal(err)
	}
	f.expo.receipts = map[string]any{
		tickets[ta]: map[string]any{"status": "ok"},
		tickets[tb]: map[string]any{"status": "error", "details": map[string]string{"error": "DeviceNotRegistered"}},
	}
	n, err := f.svc.ProcessPushReceipts(ctx)
	if err != nil || n != 1 {
		t.Fatalf("receipts disabled %d (%v), want 1", n, err)
	}
	f.expectActive(t, ta)
	f.expectDisabled(t, tb, "device_not_registered")
	if c := f.count(t, `SELECT COUNT(*) FROM push_tickets WHERE ticket_id = ANY($1)`, []string{tickets[ta], tickets[tb]}); c != 0 {
		t.Fatalf("processed tickets left: %d", c)
	}
	// Signing in again on the phone re-enables it.
	f.mustRegister(t, f.signIn(t, u), tb)
	f.expectActive(t, tb)
}

func TestIntegrationPushCleanup(t *testing.T) {
	f := newSendFixture(t)
	ctx := context.Background()
	u := f.d.CreateUser(t)
	toks := map[string]string{}
	for _, k := range []string{"fresh", "disabled_old", "disabled_new", "stale"} {
		toks[k] = pushToken()
		f.mustRegister(t, f.signIn(t, u), toks[k])
	}
	exec := func(sql string, args ...any) {
		t.Helper()
		if _, err := f.d.Pool.Exec(ctx, sql, args...); err != nil {
			t.Fatal(err)
		}
	}
	exec(`UPDATE push_devices SET disabled_at = NOW() - interval '31 days', disabled_reason = 'signed_out' WHERE token = $1`, toks["disabled_old"])
	exec(`UPDATE push_devices SET disabled_at = NOW() - interval '29 days', disabled_reason = 'signed_out' WHERE token = $1`, toks["disabled_new"])
	exec(`UPDATE push_devices SET last_seen_at = NOW() - interval '271 days' WHERE token = $1`, toks["stale"])
	var freshID int64
	if err := f.d.Pool.QueryRow(ctx, `SELECT id FROM push_devices WHERE token = $1`, toks["fresh"]).Scan(&freshID); err != nil {
		t.Fatal(err)
	}
	oldTicket, newTicket := "old-"+pgtest.Unique(), "new-"+pgtest.Unique()
	exec(`INSERT INTO push_tickets (ticket_id, push_device_id, created_at) VALUES ($1, $2, NOW() - interval '25 hours'), ($3, $2, NOW() - interval '23 hours')`,
		oldTicket, freshID, newTicket)

	if _, err := f.svc.PrunePushData(ctx); err != nil {
		t.Fatal(err)
	}
	for k, want := range map[string]int{"fresh": 1, "disabled_old": 0, "disabled_new": 1, "stale": 0} {
		if n := f.count(t, `SELECT COUNT(*) FROM push_devices WHERE token = $1`, toks[k]); n != want {
			t.Errorf("%s device rows = %d, want %d", k, n, want)
		}
	}
	for tk, want := range map[string]int{oldTicket: 0, newTicket: 1} {
		if n := f.count(t, `SELECT COUNT(*) FROM push_tickets WHERE ticket_id = $1`, tk); n != want {
			t.Errorf("ticket %s rows = %d, want %d", tk, n, want)
		}
	}
}

func keys(m map[string]expoMsg) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
