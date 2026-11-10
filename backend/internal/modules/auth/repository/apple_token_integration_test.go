package repository_test

// Postgres integration tests for the Apple token exchange and revocation (TEC-1181): native
// Apple sign-in / link with an authorization_code against a fake Apple token endpoint
// (httptest), the encrypted refresh token in oauth_accounts, and revocation on unlink.
// Skipped unless TEST_DATABASE_URL is set.

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"database/sql"
	"encoding/pem"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"

	"github.com/alicebob/miniredis/v2"
	authhandler "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/auth/handler"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/auth/usecase"
	oauthproviderusecase "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/integrations/oauthprovider/usecase"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/appleauth"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/authctx"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/ratelimit"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/testutil/pgtest"
	jwtlib "github.com/golang-jwt/jwt/v5"
	"github.com/redis/go-redis/v9"
)

const (
	appleTeamID     = "TEAM118100"
	appleKeyID      = "KEY1181000"
	appleServicesID = "app.comcode.web"
)

type appleCall struct {
	path string
	form url.Values
}

type appleTokenEnv struct {
	*nativeEnv
	p8     *ecdsa.PrivateKey
	tokens *usecase.AppleTokens
	logs   *bytes.Buffer

	mu     sync.Mutex
	calls  []appleCall
	status int // forced status for every Apple call; 0 = normal behaviour
}

func newAppleTokenEnv(t *testing.T) *appleTokenEnv {
	t.Helper()
	e := &appleTokenEnv{nativeEnv: newNativeEnv(t), logs: &bytes.Buffer{}}
	p8, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	e.p8 = p8
	der, err := x509.MarshalPKCS8PrivateKey(p8)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if _, err := e.oauthPS.Patch(ctx, "apple", oauthproviderusecase.PatchInput{
		ClientID: ptr(appleServicesID), TeamID: ptr(appleTeamID), KeyID: ptr(appleKeyID),
		PrivateKey: ptr(string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}))),
	}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = e.oauthPS.Patch(context.Background(), "apple", oauthproviderusecase.PatchInput{
			LoginEnabled: ptr(false), RemovePrivateKey: ptr(true),
		})
	})

	apple := httptest.NewServer(http.HandlerFunc(e.serveApple))
	t.Cleanup(apple.Close)
	log := slog.New(slog.NewTextHandler(&lockedWriter{w: e.logs, mu: &e.mu}, &slog.HandlerOptions{Level: slog.LevelDebug}))
	e.tokens = usecase.NewAppleTokens(e.repo, e.box, e.oauthPS, appleauth.NewTokenClient(apple.URL, nil), log)
	e.uc.SetAppleTokens(e.tokens)
	oauthUC := usecase.NewOAuth(e.repo, e.box)
	oauthUC.SetAppleTokens(e.tokens)
	e.h = authhandler.New(e.uc, oauthUC, nil, e.oauthPS, e.gate, "", nil, nil)
	mr := miniredis.RunT(t)
	e.h.SetRateLimiter(ratelimit.New(redis.NewClient(&redis.Options{Addr: mr.Addr()}), "test"))
	e.enable("apple", true, nativeBundleID)
	return e
}

type lockedWriter struct {
	w  *bytes.Buffer
	mu *sync.Mutex
}

func (l *lockedWriter) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.w.Write(p)
}

// serveApple is the fake appleid.apple.com: it accepts only a client_secret signed with the
// uploaded .p8 for the posted client_id, like Apple.
func (e *appleTokenEnv) serveApple(w http.ResponseWriter, r *http.Request) {
	_ = r.ParseForm()
	e.mu.Lock()
	e.calls = append(e.calls, appleCall{r.URL.Path, r.PostForm})
	forced := e.status
	e.mu.Unlock()
	if forced != 0 {
		w.WriteHeader(forced)
		_, _ = w.Write([]byte(`{"error":"invalid_grant"}`))
		return
	}
	clientID := r.PostForm.Get("client_id")
	if _, err := jwtlib.Parse(r.PostForm.Get("client_secret"), func(*jwtlib.Token) (any, error) { return &e.p8.PublicKey, nil },
		jwtlib.WithValidMethods([]string{"ES256"}), jwtlib.WithIssuer(appleTeamID), jwtlib.WithSubject(clientID),
		jwtlib.WithAudience("https://appleid.apple.com")); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"invalid_client"}`))
		return
	}
	switch r.URL.Path {
	case "/auth/token":
		_, _ = w.Write([]byte(`{"access_token":"apple-at-` + r.PostForm.Get("code") + `","refresh_token":"apple-rt-` +
			r.PostForm.Get("code") + `","id_token":"x","token_type":"Bearer","expires_in":3600}`))
	case "/auth/revoke":
		w.WriteHeader(http.StatusOK)
	default:
		w.WriteHeader(http.StatusNotFound)
	}
}

func (e *appleTokenEnv) setStatus(s int) {
	e.mu.Lock()
	e.status = s
	e.mu.Unlock()
}

func (e *appleTokenEnv) callsTo(path string) []appleCall {
	e.mu.Lock()
	defer e.mu.Unlock()
	var out []appleCall
	for _, c := range e.calls {
		if c.path == path {
			out = append(out, c)
		}
	}
	return out
}

func (e *appleTokenEnv) logText() string {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.logs.String()
}

type storedAppleToken struct {
	refreshEnc, accessEnc, clientID sql.NullString
	rowJSON                         string
}

func (e *appleTokenEnv) stored(sub string) storedAppleToken {
	e.t.Helper()
	var s storedAppleToken
	if err := e.d.Pool.QueryRow(context.Background(), `
		SELECT refresh_token_enc, access_token_enc, client_id, row_to_json(oa)::text
		FROM oauth_accounts oa WHERE provider = 'apple' AND provider_account_id = $1`, sub).
		Scan(&s.refreshEnc, &s.accessEnc, &s.clientID, &s.rowJSON); err != nil {
		e.t.Fatalf("oauth_accounts row for %s: %v", sub, err)
	}
	return s
}

// expectEncrypted: the refresh token is stored only as SecretBox ciphertext, with the bundle
// id as client_id and no access token; the plaintext appears nowhere in the row.
func (e *appleTokenEnv) expectEncrypted(sub, plaintext string) {
	e.t.Helper()
	s := e.stored(sub)
	if !s.refreshEnc.Valid || s.refreshEnc.String == plaintext || strings.Contains(s.rowJSON, plaintext) {
		e.t.Fatalf("refresh token not stored encrypted: %s", s.rowJSON)
	}
	if got, err := e.box.Decrypt(s.refreshEnc.String); err != nil || got != plaintext {
		e.t.Fatalf("decrypt stored token = %q, %v", got, err)
	}
	if !s.clientID.Valid || s.clientID.String != nativeBundleID {
		e.t.Fatalf("client_id = %+v, want the bundle id %s", s.clientID, nativeBundleID)
	}
	if s.accessEnc.Valid || strings.Contains(s.rowJSON, "apple-at-") {
		e.t.Fatalf("access token stored: %s", s.rowJSON)
	}
}

func (e *appleTokenEnv) noSecretsLogged(secrets ...string) {
	e.t.Helper()
	out := e.logText()
	for _, s := range append(secrets, "apple-rt-", "apple-at-", "PRIVATE KEY", "eyJ") {
		if strings.Contains(out, s) {
			e.t.Fatalf("log contains %q:\n%s", s, out)
		}
	}
}

func (e *appleTokenEnv) linkWithCode(user pgtest.User, sub, code string) (int, envelope) {
	return e.do(e.h.NativeLinkIdentity, "apple",
		map[string]any{"id_token": e.appleToken(sub, sub+"@privaterelay.appleid.com"), "nonce": nativeNonce, "authorization_code": code},
		&authctx.Principal{UserID: user.UUID, UserInternal: user.ID}, false)
}

func (e *appleTokenEnv) unlink(user pgtest.User) (int, envelope) {
	return e.do(e.h.UnlinkIdentity, "apple", map[string]any{}, &authctx.Principal{UserID: user.UUID, UserInternal: user.ID}, false)
}

// Sign-up and a later sign-in with an authorization_code: the code is exchanged with the bundle
// id as client_id and the newest refresh token is stored encrypted.
func TestIntegrationAppleTokenExchangeOnNativeSignIn(t *testing.T) {
	e := newAppleTokenEnv(t)
	sub := "apple-" + pgtest.Unique()
	email := sub + "@privaterelay.appleid.com"
	code1 := "code1-" + pgtest.Unique()

	status, env := e.signIn("apple", e.appleToken(sub, email), map[string]any{"authorization_code": code1})
	user, err := e.repo.FindUserByEmail(context.Background(), email)
	if err != nil {
		t.Fatalf("user not created (status %d %q): %v", status, env.Error.Code, err)
	}
	e.signedInOnPersonal("sign-up with code", status, env, user.ID)
	ex := e.callsTo("/auth/token")
	if len(ex) != 1 || ex[0].form.Get("client_id") != nativeBundleID || ex[0].form.Get("code") != code1 ||
		ex[0].form.Get("grant_type") != "authorization_code" {
		t.Fatalf("exchange calls = %+v", ex)
	}
	e.expectEncrypted(sub, "apple-rt-"+code1)

	code2 := "code2-" + pgtest.Unique()
	status, env = e.signIn("apple", e.appleToken(sub, email), map[string]any{"authorization_code": code2})
	e.signedInOnPersonal("sign-in with code", status, env, user.ID)
	e.expectEncrypted(sub, "apple-rt-"+code2)

	// Without a code nothing is exchanged and the stored token stays.
	status, env = e.signIn("apple", e.appleToken(sub, email), nil)
	e.signedInOnPersonal("sign-in without code", status, env, user.ID)
	if n := len(e.callsTo("/auth/token")); n != 2 {
		t.Fatalf("%d exchanges, want 2", n)
	}
	e.expectEncrypted(sub, "apple-rt-"+code2)
	e.noSecretsLogged(code1, code2)
}

// Apple refusing (or failing) the exchange never blocks the sign-in.
func TestIntegrationAppleTokenExchangeFailureDoesNotBlockSignIn(t *testing.T) {
	e := newAppleTokenEnv(t)
	sub := "apple-" + pgtest.Unique()
	email := sub + "@privaterelay.appleid.com"
	code := "code-" + pgtest.Unique()

	for _, s := range []int{http.StatusBadRequest, http.StatusInternalServerError} {
		e.setStatus(s)
		status, env := e.signIn("apple", e.appleToken(sub, email), map[string]any{"authorization_code": code})
		user, err := e.repo.FindUserByEmail(context.Background(), email)
		if err != nil {
			t.Fatalf("status %d: user not created (%d %q): %v", s, status, env.Error.Code, err)
		}
		e.signedInOnPersonal("exchange failed", status, env, user.ID)
		if st := e.stored(sub); st.refreshEnc.Valid || st.clientID.Valid {
			t.Fatalf("token stored after a failed exchange: %s", st.rowJSON)
		}
	}
	if len(e.callsTo("/auth/token")) != 2 || !strings.Contains(e.logText(), "auth_apple_code_exchange_failed") {
		t.Fatalf("exchange not attempted / not logged:\n%s", e.logText())
	}
	e.noSecretsLogged(code)
}

// Linking from the profile with a code stores the token; unlinking revokes it at Apple with the
// stored client id and removes the identity.
func TestIntegrationAppleTokenLinkThenUnlinkRevokes(t *testing.T) {
	e := newAppleTokenEnv(t)
	user := e.d.CreateUser(t)
	sub := "apple-" + pgtest.Unique()
	code := "code-" + pgtest.Unique()

	status, env := e.linkWithCode(user, sub, code)
	if status != http.StatusOK {
		t.Fatalf("link: %d %q", status, env.Error.Code)
	}
	e.expectEncrypted(sub, "apple-rt-"+code)

	status, env = e.unlink(user)
	if status != http.StatusOK {
		t.Fatalf("unlink: %d %q", status, env.Error.Code)
	}
	rv := e.callsTo("/auth/revoke")
	if len(rv) != 1 || rv[0].form.Get("token") != "apple-rt-"+code || rv[0].form.Get("client_id") != nativeBundleID ||
		rv[0].form.Get("token_type_hint") != "refresh_token" {
		t.Fatalf("revoke calls = %+v", rv)
	}
	if e.oauthRows("apple", sub) != 0 {
		t.Fatal("identity not removed")
	}
	if !strings.Contains(e.logText(), "auth_apple_token_revoked") {
		t.Fatalf("revoke not logged:\n%s", e.logText())
	}
	e.noSecretsLogged(code)

	// An Apple identity without a stored token is unlinked without calling Apple.
	sub2 := "apple-" + pgtest.Unique()
	if status, env = e.linkWithCode(user, sub2, ""); status != http.StatusOK {
		t.Fatalf("link without code: %d %q", status, env.Error.Code)
	}
	if status, env = e.unlink(user); status != http.StatusOK {
		t.Fatalf("unlink without token: %d %q", status, env.Error.Code)
	}
	if n := len(e.callsTo("/auth/revoke")); n != 1 {
		t.Fatalf("%d revoke calls, want 1", n)
	}
}

// Apple failing the revocation never blocks the unlink.
func TestIntegrationAppleTokenRevokeFailureDoesNotBlockUnlink(t *testing.T) {
	e := newAppleTokenEnv(t)
	user := e.d.CreateUser(t)
	sub := "apple-" + pgtest.Unique()
	code := "code-" + pgtest.Unique()
	if status, env := e.linkWithCode(user, sub, code); status != http.StatusOK {
		t.Fatalf("link: %d %q", status, env.Error.Code)
	}
	e.setStatus(http.StatusServiceUnavailable)
	status, env := e.unlink(user)
	if status != http.StatusOK {
		t.Fatalf("unlink: %d %q", status, env.Error.Code)
	}
	if e.oauthRows("apple", sub) != 0 {
		t.Fatal("identity not removed after a failed revoke")
	}
	if len(e.callsTo("/auth/revoke")) != 1 || !strings.Contains(e.logText(), "auth_apple_token_revoke_failed") {
		t.Fatalf("revoke not attempted / not logged:\n%s", e.logText())
	}
	e.noSecretsLogged(code)
}

// RevokeForUser (the account deletion hook, TEC-1122) revokes and clears the stored token.
func TestIntegrationAppleTokenRevokeForUser(t *testing.T) {
	e := newAppleTokenEnv(t)
	user := e.d.CreateUser(t)
	sub := "apple-" + pgtest.Unique()
	code := "code-" + pgtest.Unique()
	if status, env := e.linkWithCode(user, sub, code); status != http.StatusOK {
		t.Fatalf("link: %d %q", status, env.Error.Code)
	}
	if err := e.tokens.RevokeForUser(context.Background(), user.ID); err != nil {
		t.Fatal(err)
	}
	if rv := e.callsTo("/auth/revoke"); len(rv) != 1 || rv[0].form.Get("token") != "apple-rt-"+code {
		t.Fatalf("revoke calls = %+v", rv)
	}
	st := e.stored(sub)
	if st.refreshEnc.Valid || st.clientID.String != nativeBundleID {
		t.Fatalf("after RevokeForUser: %s", st.rowJSON)
	}
	// Nothing left to revoke, and a user without an Apple identity: no call, no error.
	other := e.d.CreateUser(t)
	for _, id := range []int64{user.ID, other.ID} {
		if err := e.tokens.RevokeForUser(context.Background(), id); err != nil {
			t.Fatal(err)
		}
	}
	if n := len(e.callsTo("/auth/revoke")); n != 1 {
		t.Fatalf("%d revoke calls, want 1", n)
	}
}
