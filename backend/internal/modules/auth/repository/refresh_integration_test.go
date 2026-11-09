package repository_test

// Postgres integration tests for device sessions, the mobile refresh lifetime and refresh
// token reuse detection (TEC-1179). Skipped unless TEST_DATABASE_URL is set.

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/auth/model"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/auth/repository"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/auth/usecase"
	notifmodel "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/notifications/model"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/jwt"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/testutil/pgtest"
	"github.com/google/uuid"
)

type allNotifier struct {
	mu   sync.Mutex
	sent []notifmodel.EnqueueInput
}

func (n *allNotifier) Enqueue(_ context.Context, in notifmodel.EnqueueInput) ([]notifmodel.Notification, error) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.sent = append(n.sent, in)
	return nil, nil
}

func (n *allNotifier) count(template string) int {
	n.mu.Lock()
	defer n.mu.Unlock()
	c := 0
	for _, in := range n.sent {
		if in.TemplateCode == template {
			c++
		}
	}
	return c
}

type refreshFixture struct {
	d     *pgtest.DB
	uc    *usecase.AuthUseCase
	notif *allNotifier
	user  pgtest.User
}

// Web sessions live 1h, mobile sessions 48h in these tests.
func newRefreshFixture(t *testing.T) refreshFixture {
	t.Helper()
	d := pgtest.Open(t)
	tokens, err := jwt.NewManager("test-secret-key-32-bytes-minimum!", time.Minute, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if err := tokens.SetMobileRefreshTTL(48 * time.Hour); err != nil {
		t.Fatal(err)
	}
	uc := usecase.New(repository.NewPostgres(d.Pool, d.Queries), tokens)
	notif := &allNotifier{}
	uc.SetNotifier(notif)
	return refreshFixture{d: d, uc: uc, notif: notif, user: d.CreateUser(t)}
}

func (f refreshFixture) signIn(t *testing.T, meta model.SessionMeta) model.Tokens {
	t.Helper()
	tok, err := f.uc.IssueSessionForUser(context.Background(), f.user.UUID, meta, "oauth")
	if err != nil {
		t.Fatalf("sign in: %v", err)
	}
	return tok
}

func hashOf(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

func (f refreshFixture) storedExpiry(t *testing.T, raw string) time.Duration {
	t.Helper()
	var until float64
	if err := f.d.Pool.QueryRow(context.Background(),
		`SELECT EXTRACT(EPOCH FROM expires_at - NOW()) FROM refresh_tokens WHERE token_hash = $1`, hashOf(raw)).Scan(&until); err != nil {
		t.Fatal(err)
	}
	return time.Duration(until * float64(time.Second))
}

var mobileMeta = model.SessionMeta{ClientType: model.ClientMobile, DeviceName: "iPhone 15", AppVersion: "1.0.3"}

// Mobile sessions get JWT_REFRESH_TTL_MOBILE and slide on refresh; web keeps JWT_REFRESH_TTL,
// even when its refresh request claims to be mobile.
func TestIntegrationMobileAndWebRefreshTTL(t *testing.T) {
	f := newRefreshFixture(t)
	ctx := context.Background()

	mobile := f.signIn(t, mobileMeta)
	if d := f.storedExpiry(t, mobile.RefreshToken); d < 47*time.Hour || d > 48*time.Hour {
		t.Fatalf("mobile login expiry %v, want ~48h", d)
	}
	next, err := f.uc.Refresh(ctx, mobile.RefreshToken, model.SessionMeta{})
	if err != nil {
		t.Fatal(err)
	}
	if d := f.storedExpiry(t, next.RefreshToken); d < 47*time.Hour {
		t.Fatalf("mobile refresh expiry %v, want ~48h", d)
	}
	if d := time.Until(next.RefreshExpiresAt); d < 47*time.Hour {
		t.Fatalf("mobile refresh_expires_at in %v", d)
	}

	web := f.signIn(t, model.SessionMeta{})
	if d := f.storedExpiry(t, web.RefreshToken); d > time.Hour || d < 59*time.Minute {
		t.Fatalf("web login expiry %v, want ~1h", d)
	}
	upgraded, err := f.uc.Refresh(ctx, web.RefreshToken, mobileMeta)
	if err != nil {
		t.Fatal(err)
	}
	if d := f.storedExpiry(t, upgraded.RefreshToken); d > time.Hour {
		t.Fatalf("a refresh must not turn a web session into a mobile one: %v", d)
	}
	var ct string
	if err := f.d.Pool.QueryRow(ctx, `SELECT client_type FROM refresh_tokens WHERE token_hash = $1`, hashOf(upgraded.RefreshToken)).Scan(&ct); err != nil || ct != "web" {
		t.Fatalf("client_type = %q (%v)", ct, err)
	}
}

// Security regression (TEC-1179): a rotated refresh token replayed after the race tolerance
// revokes the whole family and sends the security email. Before the fix the replay only got a
// 401 and the attacker's (or victim's) successor token stayed valid.
func TestIntegrationRefreshReuseRevokesFamily(t *testing.T) {
	f := newRefreshFixture(t)
	ctx := context.Background()

	first := f.signIn(t, mobileMeta)
	second, err := f.uc.Refresh(ctx, first.RefreshToken, model.SessionMeta{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.d.Pool.Exec(ctx,
		`UPDATE refresh_tokens SET rotated_at = NOW() - interval '31 seconds' WHERE token_hash = $1`, hashOf(first.RefreshToken)); err != nil {
		t.Fatal(err)
	}

	if _, err := f.uc.Refresh(ctx, first.RefreshToken, model.SessionMeta{}); !errors.Is(err, usecase.ErrInvalidCredentials) {
		t.Fatalf("replay: expected ErrInvalidCredentials, got %v", err)
	}
	var live int
	if err := f.d.Pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM refresh_tokens
		WHERE family_uuid = (SELECT family_uuid FROM refresh_tokens WHERE token_hash = $1)
		  AND revoked_at IS NULL`, hashOf(first.RefreshToken)).Scan(&live); err != nil {
		t.Fatal(err)
	}
	if live != 0 {
		t.Fatalf("family still has %d live tokens", live)
	}
	if _, err := f.uc.Refresh(ctx, second.RefreshToken, model.SessionMeta{}); !errors.Is(err, usecase.ErrInvalidCredentials) {
		t.Fatalf("successor after reuse: expected ErrInvalidCredentials, got %v", err)
	}
	if n := f.notif.count("auth.refresh_reuse"); n != 1 {
		t.Fatalf("security emails = %d, want 1", n)
	}
	var templates int
	if err := f.d.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM notification_templates WHERE code = 'auth.refresh_reuse' AND active`).Scan(&templates); err != nil || templates != 2 {
		t.Fatalf("auth.refresh_reuse templates = %d (%v), want tr + en", templates, err)
	}
}

// Concurrent refreshes of the same token: exactly one rotates, the others get ErrRefreshRace
// and the family stays live.
func TestIntegrationConcurrentRefreshWithinToleranceKeepsFamily(t *testing.T) {
	f := newRefreshFixture(t)
	ctx := context.Background()
	tok := f.signIn(t, mobileMeta)

	const n = 5
	var wg sync.WaitGroup
	results := make([]model.Tokens, n)
	errs := make([]error, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			results[i], errs[i] = f.uc.Refresh(ctx, tok.RefreshToken, model.SessionMeta{})
		}(i)
	}
	wg.Wait()

	var winner model.Tokens
	ok := 0
	for i, err := range errs {
		switch {
		case err == nil:
			ok++
			winner = results[i]
		case errors.Is(err, usecase.ErrRefreshRace):
		default:
			t.Fatalf("request %d: unexpected %v", i, err)
		}
	}
	if ok != 1 {
		t.Fatalf("%d refreshes succeeded, want exactly 1", ok)
	}
	if _, err := f.uc.Refresh(ctx, winner.RefreshToken, model.SessionMeta{}); err != nil {
		t.Fatalf("winner token must stay usable: %v", err)
	}
	if c := f.notif.count("auth.refresh_reuse"); c != 0 {
		t.Fatalf("race must not send a reuse email: %d", c)
	}
}

// GET /v1/auth/sessions data: client type, device name and app version (latest app version
// after a refresh), web sessions without device labels.
func TestIntegrationSessionsListDeviceFields(t *testing.T) {
	f := newRefreshFixture(t)
	ctx := context.Background()

	mobile := f.signIn(t, mobileMeta)
	if _, err := f.uc.Refresh(ctx, mobile.RefreshToken, model.SessionMeta{ClientType: model.ClientMobile, AppVersion: "1.1.0", DeviceName: "renamed"}); err != nil {
		t.Fatal(err)
	}
	f.signIn(t, model.SessionMeta{UserAgent: "Mozilla/5.0", DeviceName: "ignored"})

	items, err := f.uc.ListSessions(ctx, f.user.ID, uuid.Nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 {
		t.Fatalf("sessions = %d, want 2", len(items))
	}
	var sawMobile, sawWeb bool
	for _, it := range items {
		raw, _ := json.Marshal(it)
		var m map[string]any
		_ = json.Unmarshal(raw, &m)
		switch it.ClientType {
		case model.ClientMobile:
			sawMobile = true
			if m["device_name"] != "iPhone 15" || m["app_version"] != "1.1.0" || m["client_type"] != "mobile" {
				t.Fatalf("mobile session json = %s", raw)
			}
		case model.ClientWeb:
			sawWeb = true
			if _, ok := m["device_name"]; ok || m["client_type"] != "web" {
				t.Fatalf("web session json = %s", raw)
			}
		}
	}
	if !sawMobile || !sawWeb {
		t.Fatalf("items = %+v", items)
	}
}
