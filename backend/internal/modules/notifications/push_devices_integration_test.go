package notifications

// TEC-1186 (mobile B9): push devices are tied to the refresh token family they registered
// from. Logout, session revocation, refresh reuse and revoking all sessions disable them; a
// revoked session cannot re-enable a device; a user cannot delete another user's token.
// Real PostgreSQL (TEST_DATABASE_URL).

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	authmodel "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/auth/model"
	authrepo "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/auth/repository"
	authusecase "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/auth/usecase"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/notifications/handler"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/notifications/usecase"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/authctx"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/jwt"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/testutil/pgtest"
	"github.com/google/uuid"
)

type pushFixture struct {
	d    *pgtest.DB
	h    *handler.Handler
	auth *authusecase.AuthUseCase
}

func newPushFixture(t *testing.T) *pushFixture {
	t.Helper()
	d := pgtest.Open(t)
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	svc := usecase.New(d.Queries, nil, nil, log)
	tokens, err := jwt.NewManager("push-devices-fixture-jwt-secret!", time.Minute, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	return &pushFixture{d: d, h: handler.New(svc), auth: authusecase.New(authrepo.NewPostgres(d.Pool, d.Queries), tokens)}
}

// pushSession is one signed-in phone: its refresh token and the principal its access token gives.
type pushSession struct {
	refresh string
	p       authctx.Principal
}

func (f *pushFixture) signIn(t *testing.T, u pgtest.User) pushSession {
	t.Helper()
	tok, err := f.auth.IssueSessionForUser(context.Background(), u.UUID,
		authmodel.SessionMeta{ClientType: authmodel.ClientMobile, DeviceName: "Pixel 9", AppVersion: "1.0.0"}, "oauth")
	if err != nil {
		t.Fatalf("sign in: %v", err)
	}
	sum := sha256.Sum256([]byte(tok.RefreshToken))
	var family uuid.UUID
	if err := f.d.Pool.QueryRow(context.Background(),
		`SELECT family_uuid FROM refresh_tokens WHERE token_hash = $1`, hex.EncodeToString(sum[:])).Scan(&family); err != nil {
		t.Fatal(err)
	}
	return pushSession{refresh: tok.RefreshToken, p: authctx.Principal{UserID: u.UUID, UserInternal: u.ID, SessionID: family}}
}

func (f *pushFixture) do(t *testing.T, s pushSession, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(authctx.WithPrincipal(req.Context(), s.p))
	rec := httptest.NewRecorder()
	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1/notifications/push-devices", f.h.ListPushDevices)
	mux.HandleFunc("POST /v1/notifications/push-devices", f.h.RegisterPushDevice)
	mux.HandleFunc("DELETE /v1/notifications/push-devices/{token}", f.h.DeletePushDevice)
	mux.ServeHTTP(rec, req)
	return rec
}

func (f *pushFixture) register(t *testing.T, s pushSession, token string) *httptest.ResponseRecorder {
	t.Helper()
	return f.do(t, s, http.MethodPost, "/v1/notifications/push-devices",
		`{"token":"`+token+`","platform":"android","locale":"tr-TR","app_version":"1.0.0","device_name":"Pixel 9"}`)
}

func (f *pushFixture) mustRegister(t *testing.T, s pushSession, token string) {
	t.Helper()
	if rec := f.register(t, s, token); rec.Code != http.StatusOK {
		t.Fatalf("register %s = %d %s", token, rec.Code, rec.Body.String())
	}
}

type deviceRow struct {
	userID   int64
	disabled bool
	reason   *string
}

func (f *pushFixture) device(t *testing.T, token string) deviceRow {
	t.Helper()
	var r deviceRow
	if err := f.d.Pool.QueryRow(context.Background(),
		`SELECT user_id, disabled_at IS NOT NULL, disabled_reason FROM push_devices WHERE token = $1`, token).
		Scan(&r.userID, &r.disabled, &r.reason); err != nil {
		t.Fatalf("device %s: %v", token, err)
	}
	return r
}

func (f *pushFixture) expectDisabled(t *testing.T, token, reason string) {
	t.Helper()
	r := f.device(t, token)
	if !r.disabled || r.reason == nil || *r.reason != reason {
		t.Fatalf("device %s: disabled=%v reason=%v, want disabled with %q", token, r.disabled, r.reason, reason)
	}
}

func (f *pushFixture) expectActive(t *testing.T, token string) {
	t.Helper()
	if r := f.device(t, token); r.disabled {
		t.Fatalf("device %s was disabled (%v)", token, r.reason)
	}
}

func pushToken() string { return "ExponentPushToken[" + pgtest.Unique() + "]" }

func TestIntegrationPushDeviceLogoutDisablesDevice(t *testing.T) {
	f := newPushFixture(t)
	u := f.d.CreateUser(t)
	phone, other := f.signIn(t, u), f.signIn(t, u)
	phoneToken, otherToken := pushToken(), pushToken()
	f.mustRegister(t, phone, phoneToken)
	f.mustRegister(t, other, otherToken)

	if err := f.auth.Logout(context.Background(), phone.refresh); err != nil {
		t.Fatal(err)
	}
	f.expectDisabled(t, phoneToken, "signed_out")
	f.expectActive(t, otherToken)

	// The access token outlives the session: it must not re-enable the device.
	rec := f.register(t, phone, phoneToken)
	if rec.Code != http.StatusConflict || !strings.Contains(rec.Body.String(), "PUSH_SESSION_INACTIVE") {
		t.Fatalf("register after logout = %d %s, want 409 PUSH_SESSION_INACTIVE", rec.Code, rec.Body.String())
	}
	f.expectDisabled(t, phoneToken, "signed_out")

	// Signing in again on the phone re-enables it on the new session.
	again := f.signIn(t, u)
	f.mustRegister(t, again, phoneToken)
	f.expectActive(t, phoneToken)
}

func TestIntegrationPushDeviceSessionRevocationDisablesDevice(t *testing.T) {
	f := newPushFixture(t)
	ctx := context.Background()
	u := f.d.CreateUser(t)
	web, phone, tablet := f.signIn(t, u), f.signIn(t, u), f.signIn(t, u)
	phoneToken, tabletToken := pushToken(), pushToken()
	f.mustRegister(t, phone, phoneToken)
	f.mustRegister(t, tablet, tabletToken)

	// Revoking the phone from the sessions list on the web.
	if err := f.auth.RevokeSession(ctx, u.ID, phone.p.SessionID); err != nil {
		t.Fatal(err)
	}
	f.expectDisabled(t, phoneToken, "session_revoked")
	f.expectActive(t, tabletToken)

	// "Sign out other sessions" from the web disables the tablet too.
	if err := f.auth.RevokeOtherSessions(ctx, u.ID, web.p.SessionID); err != nil {
		t.Fatal(err)
	}
	f.expectDisabled(t, tabletToken, "session_revoked")
}

func TestIntegrationPushDeviceRevokeOthersKeepsCurrent(t *testing.T) {
	f := newPushFixture(t)
	u := f.d.CreateUser(t)
	phone, tablet := f.signIn(t, u), f.signIn(t, u)
	phoneToken, tabletToken := pushToken(), pushToken()
	f.mustRegister(t, phone, phoneToken)
	f.mustRegister(t, tablet, tabletToken)
	if err := f.auth.RevokeOtherSessions(context.Background(), u.ID, phone.p.SessionID); err != nil {
		t.Fatal(err)
	}
	f.expectActive(t, phoneToken)
	f.expectDisabled(t, tabletToken, "session_revoked")
}

func TestIntegrationPushDeviceRefreshReuseDisablesDevice(t *testing.T) {
	f := newPushFixture(t)
	ctx := context.Background()
	u := f.d.CreateUser(t)
	phone := f.signIn(t, u)
	token := pushToken()
	f.mustRegister(t, phone, token)

	if _, err := f.auth.Refresh(ctx, phone.refresh, authmodel.SessionMeta{}); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256([]byte(phone.refresh))
	if _, err := f.d.Pool.Exec(ctx,
		`UPDATE refresh_tokens SET rotated_at = NOW() - interval '31 seconds' WHERE token_hash = $1`, hex.EncodeToString(sum[:])); err != nil {
		t.Fatal(err)
	}
	f.expectActive(t, token)
	if _, err := f.auth.Refresh(ctx, phone.refresh, authmodel.SessionMeta{}); err == nil {
		t.Fatal("replayed refresh token was accepted")
	}
	f.expectDisabled(t, token, "session_revoked")
}

func TestIntegrationPushDeviceRevokeAllDisablesDevices(t *testing.T) {
	f := newPushFixture(t)
	ctx := context.Background()
	u := f.d.CreateUser(t)
	a, b := f.signIn(t, u), f.signIn(t, u)
	ta, tb := pushToken(), pushToken()
	f.mustRegister(t, a, ta)
	f.mustRegister(t, b, tb)
	if err := f.d.Queries.RevokeAllRefreshTokensForUser(ctx, u.ID); err != nil {
		t.Fatal(err)
	}
	f.expectDisabled(t, ta, "all_sessions_revoked")
	f.expectDisabled(t, tb, "all_sessions_revoked")
}

// A user cannot delete (or learn about) another user's token; their own goes away.
func TestIntegrationPushDeviceDeleteOnlyOwnToken(t *testing.T) {
	f := newPushFixture(t)
	owner, intruder := f.d.CreateUser(t), f.d.CreateUser(t)
	ownerPhone, intruderPhone := f.signIn(t, owner), f.signIn(t, intruder)
	token := pushToken()
	f.mustRegister(t, ownerPhone, token)

	path := "/v1/notifications/push-devices/" + url.PathEscape(token)
	if rec := f.do(t, intruderPhone, http.MethodDelete, path, ""); rec.Code != http.StatusNotFound {
		t.Fatalf("delete another user's token = %d %s, want 404", rec.Code, rec.Body.String())
	}
	if r := f.device(t, token); r.userID != owner.ID || r.disabled {
		t.Fatalf("owner's device changed: %+v", r)
	}
	if rec := f.do(t, ownerPhone, http.MethodDelete, path, ""); rec.Code != http.StatusOK {
		t.Fatalf("delete own token = %d %s", rec.Code, rec.Body.String())
	}
	var n int
	if err := f.d.Pool.QueryRow(context.Background(), `SELECT COUNT(*) FROM push_devices WHERE token = $1`, token).Scan(&n); err != nil || n != 0 {
		t.Fatalf("device rows after delete = %d (%v)", n, err)
	}
	if rec := f.do(t, ownerPhone, http.MethodDelete, path, ""); rec.Code != http.StatusNotFound {
		t.Fatalf("delete twice = %d, want 404", rec.Code)
	}
}

// The phone changed hands: registering the same token as another user moves the row.
func TestIntegrationPushDeviceTokenMovesToNewUser(t *testing.T) {
	f := newPushFixture(t)
	first, second := f.d.CreateUser(t), f.d.CreateUser(t)
	token := pushToken()
	f.mustRegister(t, f.signIn(t, first), token)
	secondPhone := f.signIn(t, second)
	f.mustRegister(t, secondPhone, token)
	if r := f.device(t, token); r.userID != second.ID || r.disabled {
		t.Fatalf("device after move: %+v, want user %d", r, second.ID)
	}

	rec := f.do(t, secondPhone, http.MethodGet, "/v1/notifications/push-devices", "")
	var out struct {
		Data struct {
			Items []struct {
				Token   string `json:"token"`
				Current bool   `json:"current"`
			} `json:"items"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil || rec.Code != http.StatusOK {
		t.Fatalf("list = %d %s (%v)", rec.Code, rec.Body.String(), err)
	}
	if len(out.Data.Items) != 1 || out.Data.Items[0].Token != token || !out.Data.Items[0].Current {
		t.Fatalf("list = %s", rec.Body.String())
	}
}

// The first device turns push on; a later "off" survives registering again.
func TestIntegrationPushDeviceFirstRegistrationEnablesPush(t *testing.T) {
	f := newPushFixture(t)
	ctx := context.Background()
	u := f.d.CreateUser(t)
	// A preference row that already exists (push off by default) is switched on too.
	if _, err := f.d.Pool.Exec(ctx, `INSERT INTO notification_preferences (user_id, push_enabled) VALUES ($1, FALSE)`, u.ID); err != nil {
		t.Fatal(err)
	}
	phone := f.signIn(t, u)
	rec := f.register(t, phone, pushToken())
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"push_enabled":true`) {
		t.Fatalf("first register = %d %s, want push_enabled true", rec.Code, rec.Body.String())
	}

	if _, err := f.d.Pool.Exec(ctx, `UPDATE notification_preferences SET push_enabled = FALSE WHERE user_id = $1`, u.ID); err != nil {
		t.Fatal(err)
	}
	rec = f.register(t, phone, pushToken())
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"push_enabled":false`) {
		t.Fatalf("second register = %d %s, want push_enabled false kept", rec.Code, rec.Body.String())
	}
}

func TestIntegrationPushDeviceImpersonationRefused(t *testing.T) {
	f := newPushFixture(t)
	u := f.d.CreateUser(t)
	s := f.signIn(t, u)
	admin := uuid.New()
	s.p.ImpersonatorUserID = &admin
	rec := f.register(t, s, pushToken())
	if rec.Code != http.StatusForbidden || !strings.Contains(rec.Body.String(), "PUSH_IMPERSONATION") {
		t.Fatalf("impersonated register = %d %s, want 403 PUSH_IMPERSONATION", rec.Code, rec.Body.String())
	}
}
