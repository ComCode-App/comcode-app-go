package repository_test

// Postgres integration tests for native Apple / Google sign-in (TEC-1180): every outcome of
// POST /v1/auth/oauth/{provider}/native and POST /v1/auth/identities/{provider}/native through
// the real handler, use case, repository, oauth_provider_settings and an id_token verifier
// backed by a local JWKS. Skipped unless TEST_DATABASE_URL is set.

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	authhandler "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/auth/handler"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/auth/repository"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/auth/usecase"
	oauthproviderusecase "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/integrations/oauthprovider/usecase"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/authctx"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/crypto"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/jwt"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/oidc"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/ratelimit"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/testutil/pgtest"
	jwtlib "github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/pquerna/otp/totp"
	"github.com/redis/go-redis/v9"
)

const (
	nativeAppleIss    = "https://appleid.apple.com"
	nativeGoogleIss   = "https://accounts.google.com"
	nativeBundleID    = "app.comcode.mobile"
	nativeGoogleAud   = "123-ios.apps.googleusercontent.com"
	nativeNonce       = "nonce-1180"
	nativeTestJWTSalt = "test-secret-key-32-bytes-minimum!"
)

type registrationGate struct {
	usecase.AuthSettingsGate
	open bool
}

func (g *registrationGate) RegistrationEnabled(context.Context) (bool, error) { return g.open, nil }
func (*registrationGate) DefaultRoleID(context.Context) (*int64, error)       { return nil, nil }

type nativeEnv struct {
	t       *testing.T
	d       *pgtest.DB
	repo    *repository.Postgres
	tokens  *jwt.Manager
	uc      *usecase.AuthUseCase
	h       *authhandler.Handler
	gate    *registrationGate
	oauthPS *oauthproviderusecase.Service
	key     *rsa.PrivateKey
	box     *crypto.SecretBox
}

func newNativeEnv(t *testing.T) *nativeEnv {
	t.Helper()
	d := pgtest.Open(t)
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	jwks := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"keys": []map[string]string{{
			"kty": "RSA", "kid": "k1", "use": "sig", "alg": "RS256",
			"n": base64.RawURLEncoding.EncodeToString(key.N.Bytes()),
			"e": base64.RawURLEncoding.EncodeToString(big.NewInt(int64(key.E)).Bytes()),
		}}})
	}))
	t.Cleanup(jwks.Close)
	verifier := oidc.New(map[string]oidc.ProviderConfig{
		oidc.ProviderApple:  {Issuers: []string{nativeAppleIss}, JWKSURL: jwks.URL},
		oidc.ProviderGoogle: {Issuers: []string{nativeGoogleIss, "accounts.google.com"}, JWKSURL: jwks.URL},
	}, nil)

	raw := make([]byte, 32)
	_, _ = rand.Read(raw)
	box, err := crypto.NewSecretBox(base64.StdEncoding.EncodeToString(raw))
	if err != nil {
		t.Fatal(err)
	}
	tokens, err := jwt.NewManager(nativeTestJWTSalt, time.Minute, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	repo := repository.NewPostgres(d.Pool, d.Queries)
	uc := usecase.New(repo, tokens)
	gate := &registrationGate{open: true}
	uc.SetAuthSettings(gate)
	uc.SetSecretBox(box, "ComCode")
	oauthPS := oauthproviderusecase.New(d.Queries, box)
	uc.SetNativeOAuth(verifier, oauthPS)
	h := authhandler.New(uc, usecase.NewOAuth(repo, box), nil, oauthPS, gate, "", nil, nil)
	mr := miniredis.RunT(t)
	h.SetRateLimiter(ratelimit.New(redis.NewClient(&redis.Options{Addr: mr.Addr()}), "test"))

	// Each package has its own database, but reset the provider rows for repeatable runs.
	ctx := context.Background()
	for _, p := range []string{"apple", "google"} {
		if _, err := oauthPS.Patch(ctx, p, oauthproviderusecase.PatchInput{
			LoginEnabled: ptr(false), RegisterEnabled: ptr(false), NativeClientIDs: ptr([]string{}),
		}); err != nil {
			t.Fatal(err)
		}
	}
	return &nativeEnv{t: t, d: d, repo: repo, tokens: tokens, uc: uc, h: h, gate: gate, oauthPS: oauthPS, key: key, box: box}
}

func ptr[T any](v T) *T { return &v }

func (e *nativeEnv) enable(provider string, register bool, audiences ...string) {
	e.t.Helper()
	if _, err := e.oauthPS.Patch(context.Background(), provider, oauthproviderusecase.PatchInput{
		LoginEnabled: ptr(true), RegisterEnabled: ptr(register), NativeClientIDs: ptr(audiences),
	}); err != nil {
		e.t.Fatalf("enable %s: %v", provider, err)
	}
}

type tokenOpts struct {
	iss, aud, sub, email string
	verified             any
	nonce                string
	exp                  time.Time
}

func (e *nativeEnv) appleToken(sub, email string) string {
	return e.sign(tokenOpts{iss: nativeAppleIss, aud: nativeBundleID, sub: sub, email: email, verified: "true"})
}

func (e *nativeEnv) googleToken(sub, email string, verified bool) string {
	return e.sign(tokenOpts{iss: nativeGoogleIss, aud: nativeGoogleAud, sub: sub, email: email, verified: verified})
}

func (e *nativeEnv) sign(o tokenOpts) string {
	e.t.Helper()
	if o.nonce == "" {
		o.nonce = nativeNonce
	}
	if o.exp.IsZero() {
		o.exp = time.Now().Add(10 * time.Minute)
	}
	claims := jwtlib.MapClaims{
		"iss": o.iss, "aud": o.aud, "sub": o.sub, "nonce": o.nonce,
		"iat": time.Now().Unix(), "exp": o.exp.Unix(),
	}
	if o.email != "" {
		claims["email"] = o.email
		claims["email_verified"] = o.verified
	}
	tok := jwtlib.NewWithClaims(jwtlib.SigningMethodRS256, claims)
	tok.Header["kid"] = "k1"
	raw, err := tok.SignedString(e.key)
	if err != nil {
		e.t.Fatal(err)
	}
	return raw
}

type envelope struct {
	Success bool            `json:"success"`
	Data    json.RawMessage `json:"data"`
	Error   struct {
		Code string `json:"code"`
	} `json:"error"`
}

func (e *nativeEnv) do(fn http.HandlerFunc, provider string, body map[string]any, principal *authctx.Principal, mobile bool) (int, envelope) {
	e.t.Helper()
	raw, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(string(raw)))
	req.SetPathValue("provider", provider)
	req.RemoteAddr = "203.0.113.7:4000"
	if mobile {
		req.Header.Set("X-Client", "comcode-mobile")
		req.Header.Set("X-Device-Name", "iPhone 15")
		req.Header.Set("X-App-Version", "1.0.3")
	}
	if principal != nil {
		req = req.WithContext(authctx.WithPrincipal(req.Context(), *principal))
	}
	rec := httptest.NewRecorder()
	fn(rec, req)
	var env envelope
	_ = json.Unmarshal(rec.Body.Bytes(), &env)
	return rec.Code, env
}

func (e *nativeEnv) signIn(provider, idToken string, extra map[string]any) (int, envelope) {
	body := map[string]any{"id_token": idToken, "nonce": nativeNonce}
	for k, v := range extra {
		body[k] = v
	}
	return e.do(e.h.NativeOAuthSignIn, provider, body, nil, true)
}

func (e *nativeEnv) link(provider, idToken string, user pgtest.User) (int, envelope) {
	return e.do(e.h.NativeLinkIdentity, provider, map[string]any{"id_token": idToken, "nonce": nativeNonce},
		&authctx.Principal{UserID: user.UUID, UserInternal: user.ID}, false)
}

func expectCode(t *testing.T, what string, status int, env envelope, wantStatus int, wantCode string) {
	t.Helper()
	if status != wantStatus || env.Error.Code != wantCode {
		t.Fatalf("%s: status %d code %q, want %d %q", what, status, env.Error.Code, wantStatus, wantCode)
	}
}

// signedInOnPersonal checks a token pair: the access token's account is the personal account.
func (e *nativeEnv) signedInOnPersonal(what string, status int, env envelope, userID int64) {
	e.t.Helper()
	if status != http.StatusOK {
		e.t.Fatalf("%s: status %d code %q", what, status, env.Error.Code)
	}
	var tok struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
	}
	if err := json.Unmarshal(env.Data, &tok); err != nil || tok.AccessToken == "" || tok.RefreshToken == "" {
		e.t.Fatalf("%s: token pair = %s (%v)", what, env.Data, err)
	}
	claims, err := e.tokens.ParseAccess(tok.AccessToken)
	if err != nil {
		e.t.Fatal(err)
	}
	personal, err := e.repo.GetPersonalAccount(context.Background(), userID)
	if err != nil {
		e.t.Fatal(err)
	}
	if claims.AccountID != personal.UUID.String() {
		e.t.Fatalf("%s: active account %s, want personal %s", what, claims.AccountID, personal.UUID)
	}
}

func (e *nativeEnv) oauthRows(provider, sub string) int {
	e.t.Helper()
	var n int
	if err := e.d.Pool.QueryRow(context.Background(),
		`SELECT count(*) FROM oauth_accounts WHERE provider = $1 AND provider_account_id = $2`, provider, sub).Scan(&n); err != nil {
		e.t.Fatal(err)
	}
	return n
}

func (e *nativeEnv) usersWithEmail(email string) int {
	e.t.Helper()
	var n int
	if err := e.d.Pool.QueryRow(context.Background(), `SELECT count(*) FROM users WHERE email = $1`, email).Scan(&n); err != nil {
		e.t.Fatal(err)
	}
	return n
}

func (e *nativeEnv) userEmail(u pgtest.User) string {
	e.t.Helper()
	var email string
	if err := e.d.Pool.QueryRow(context.Background(), `SELECT email FROM users WHERE id = $1`, u.ID).Scan(&email); err != nil {
		e.t.Fatal(err)
	}
	return email
}

func TestIntegrationNativeOAuthProviderDisabled(t *testing.T) {
	e := newNativeEnv(t)
	sub := "apple-" + pgtest.Unique()
	// Login on, but no native audience.
	if _, err := e.oauthPS.Patch(context.Background(), "apple", oauthproviderusecase.PatchInput{
		LoginEnabled: ptr(false), NativeClientIDs: ptr([]string{nativeBundleID}),
	}); err != nil {
		t.Fatal(err)
	}
	status, env := e.signIn("apple", e.appleToken(sub, sub+"@privaterelay.appleid.com"), nil)
	expectCode(t, "login switched off", status, env, http.StatusForbidden, "OAUTH_PROVIDER_DISABLED")

	// Google without web credentials cannot be switched on without an audience either.
	if _, err := e.oauthPS.Patch(context.Background(), "google", oauthproviderusecase.PatchInput{LoginEnabled: ptr(true)}); err == nil {
		t.Fatal("google login without any credential must be refused")
	}
	status, env = e.signIn("google", e.googleToken(sub, "x@example.com", true), nil)
	expectCode(t, "no audience", status, env, http.StatusForbidden, "OAUTH_PROVIDER_DISABLED")

	status, env = e.signIn("facebook", "x", nil)
	if status != http.StatusNotFound {
		t.Fatalf("facebook native: %d %q", status, env.Error.Code)
	}
}

// New verified email: a passwordless account is created on the personal account, the identity
// linked, and the session is a mobile session; the next sign-in uses the link.
func TestIntegrationNativeOAuthSignUpThenLinkedSignIn(t *testing.T) {
	e := newNativeEnv(t)
	e.enable("apple", true, nativeBundleID)
	sub := "apple-" + pgtest.Unique()
	email := sub + "@privaterelay.appleid.com"

	status, env := e.signIn("apple", e.appleToken(sub, email), map[string]any{"given_name": "Ada", "family_name": "Lovelace"})
	user, err := e.repo.FindUserByEmail(context.Background(), email)
	if err != nil {
		t.Fatalf("user not created (status %d %q): %v", status, env.Error.Code, err)
	}
	e.signedInOnPersonal("sign-up", status, env, user.ID)
	if !user.EmailVerified || user.PasswordSet || user.Name != "Ada" || user.Surname != "Lovelace" {
		t.Fatalf("user = %+v", user)
	}
	if e.oauthRows("apple", sub) != 1 {
		t.Fatal("identity not linked")
	}
	var clientType, device string
	if err := e.d.Pool.QueryRow(context.Background(),
		`SELECT client_type, COALESCE(device_name, '') FROM refresh_tokens WHERE user_id = $1 ORDER BY id DESC LIMIT 1`, user.ID).Scan(&clientType, &device); err != nil {
		t.Fatal(err)
	}
	if clientType != "mobile" || device != "iPhone 15" {
		t.Fatalf("session = %s / %q, want a mobile session", clientType, device)
	}

	status, env = e.signIn("apple", e.appleToken(sub, email), nil)
	e.signedInOnPersonal("linked sign-in", status, env, user.ID)
	if n := e.usersWithEmail(email); n != 1 {
		t.Fatalf("%d users for %s", n, email)
	}
}

func TestIntegrationNativeOAuthEmailOfExistingAccount(t *testing.T) {
	e := newNativeEnv(t)
	e.enable("google", true, nativeGoogleAud)
	existing := e.d.CreateUser(t)
	email := e.userEmail(existing)
	sub := "google-" + pgtest.Unique()

	status, env := e.signIn("google", e.googleToken(sub, email, true), nil)
	expectCode(t, "verified email of an account", status, env, http.StatusConflict, "OAUTH_ACCOUNT_NOT_LINKED")
	if e.oauthRows("google", sub) != 0 {
		t.Fatal("identity must not be linked automatically")
	}

	// The upper-case variant is the same account.
	status, env = e.signIn("google", e.googleToken(sub, strings.ToUpper(email), true), nil)
	expectCode(t, "case-insensitive email", status, env, http.StatusConflict, "OAUTH_ACCOUNT_NOT_LINKED")
}

// email_verified=false never matches, creates or links an account.
func TestIntegrationNativeOAuthUnverifiedEmail(t *testing.T) {
	e := newNativeEnv(t)
	e.enable("google", true, nativeGoogleAud)
	existing := e.d.CreateUser(t)
	email := e.userEmail(existing)

	sub := "google-" + pgtest.Unique()
	status, env := e.signIn("google", e.googleToken(sub, email, false), nil)
	expectCode(t, "unverified email of an account", status, env, http.StatusForbidden, "OAUTH_EMAIL_UNVERIFIED")
	if e.oauthRows("google", sub) != 0 {
		t.Fatal("unverified token linked")
	}

	fresh := "fresh-" + pgtest.Unique() + "@example.com"
	status, env = e.signIn("google", e.googleToken(sub, fresh, false), nil)
	expectCode(t, "unverified new email", status, env, http.StatusForbidden, "OAUTH_EMAIL_UNVERIFIED")
	if e.usersWithEmail(fresh) != 0 {
		t.Fatal("account created from an unverified email")
	}
	status, env = e.signIn("google", e.googleToken(sub, "", false), nil)
	expectCode(t, "no email", status, env, http.StatusForbidden, "OAUTH_EMAIL_UNVERIFIED")

	// Explicit linking refuses it too.
	status, env = e.link("google", e.googleToken(sub, email, false), existing)
	expectCode(t, "link unverified", status, env, http.StatusForbidden, "OAUTH_EMAIL_UNVERIFIED")
	if e.oauthRows("google", sub) != 0 {
		t.Fatal("unverified token linked by the link endpoint")
	}
}

func TestIntegrationNativeOAuthRegistrationClosed(t *testing.T) {
	e := newNativeEnv(t)
	e.enable("google", true, nativeGoogleAud)
	e.gate.open = false
	email := "closed-" + pgtest.Unique() + "@example.com"
	status, env := e.signIn("google", e.googleToken("g-"+pgtest.Unique(), email, true), nil)
	expectCode(t, "registration closed", status, env, http.StatusForbidden, "REGISTRATION_DISABLED")

	// Registration open, but the provider's register flag off.
	e.gate.open = true
	e.enable("google", false, nativeGoogleAud)
	status, env = e.signIn("google", e.googleToken("g-"+pgtest.Unique(), email, true), nil)
	expectCode(t, "provider register off", status, env, http.StatusForbidden, "REGISTRATION_DISABLED")
	if e.usersWithEmail(email) != 0 {
		t.Fatal("account created while registration is closed")
	}
}

// A linked user with an authenticator gets the MFA round; a disabled user is refused.
func TestIntegrationNativeOAuthMFAAndDisabled(t *testing.T) {
	e := newNativeEnv(t)
	e.enable("google", false, nativeGoogleAud)
	ctx := context.Background()
	user := e.d.CreateUser(t)
	email := e.userEmail(user)
	sub := "google-" + pgtest.Unique()

	status, env := e.link("google", e.googleToken(sub, email, true), user)
	if status != http.StatusOK {
		t.Fatalf("link: %d %q", status, env.Error.Code)
	}
	setup, err := e.uc.BeginTOTPSetup(ctx, user.UUID)
	if err != nil {
		t.Fatal(err)
	}
	code, err := totp.GenerateCode(setup.Secret, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	confirmed, err := e.uc.ConfirmTOTPSetup(ctx, user.UUID, code)
	if err != nil {
		t.Fatal(err)
	}

	tok := e.googleToken(sub, email, true)
	status, env = e.signIn("google", tok, nil)
	expectCode(t, "no totp", status, env, http.StatusForbidden, "MFA_REQUIRED")
	status, env = e.signIn("google", tok, map[string]any{"totp_code": "000000"})
	expectCode(t, "wrong totp", status, env, http.StatusUnauthorized, "INVALID_MFA_CODE")
	status, env = e.signIn("google", tok, map[string]any{"totp_code": confirmed.RecoveryCodes[0]})
	e.signedInOnPersonal("with totp", status, env, user.ID)

	if _, err := e.d.Pool.Exec(ctx, `UPDATE users SET status = 'disabled' WHERE id = $1`, user.ID); err != nil {
		t.Fatal(err)
	}
	status, env = e.signIn("google", tok, map[string]any{"totp_code": confirmed.RecoveryCodes[1]})
	expectCode(t, "disabled user", status, env, http.StatusForbidden, "FORBIDDEN")
}

func TestIntegrationNativeOAuthInvalidToken(t *testing.T) {
	e := newNativeEnv(t)
	e.enable("apple", true, nativeBundleID)
	sub := "apple-" + pgtest.Unique()
	cases := map[string]tokenOpts{
		"wrong aud":   {iss: nativeAppleIss, aud: "com.other.app", sub: sub},
		"wrong iss":   {iss: nativeGoogleIss, aud: nativeBundleID, sub: sub},
		"expired":     {iss: nativeAppleIss, aud: nativeBundleID, sub: sub, exp: time.Now().Add(-time.Hour)},
		"other nonce": {iss: nativeAppleIss, aud: nativeBundleID, sub: sub, nonce: "replayed"},
	}
	for name, o := range cases {
		o.email, o.verified = sub+"@example.com", "true"
		status, env := e.signIn("apple", e.sign(o), nil)
		expectCode(t, name, status, env, http.StatusUnauthorized, "OAUTH_INVALID_TOKEN")
	}
	status, env := e.do(e.h.NativeOAuthSignIn, "apple", map[string]any{"id_token": e.appleToken(sub, "")}, nil, true)
	expectCode(t, "missing nonce", status, env, http.StatusBadRequest, "VALIDATION_ERROR")
	if e.oauthRows("apple", sub) != 0 {
		t.Fatal("invalid token linked")
	}
}

func TestIntegrationNativeOAuthLinkConflicts(t *testing.T) {
	e := newNativeEnv(t)
	e.enable("google", false, nativeGoogleAud)
	alice, bob := e.d.CreateUser(t), e.d.CreateUser(t)
	sub := "google-" + pgtest.Unique()

	status, env := e.link("google", e.googleToken(sub, "alice-google@example.com", true), alice)
	if status != http.StatusOK {
		t.Fatalf("link: %d %q", status, env.Error.Code)
	}
	var list struct {
		Items []struct {
			Provider string `json:"provider"`
		} `json:"items"`
	}
	if err := json.Unmarshal(env.Data, &list); err != nil || len(list.Items) != 1 || list.Items[0].Provider != "google" {
		t.Fatalf("identity list = %s", env.Data)
	}
	// Idempotent for the same user.
	if status, env = e.link("google", e.googleToken(sub, "alice-google@example.com", true), alice); status != http.StatusOK {
		t.Fatalf("relink: %d %q", status, env.Error.Code)
	}
	// The identity belongs to Alice.
	status, env = e.link("google", e.googleToken(sub, "alice-google@example.com", true), bob)
	expectCode(t, "identity of another user", status, env, http.StatusConflict, "CONFLICT")
	// Alice already has another Google identity.
	status, env = e.link("google", e.googleToken("google-"+pgtest.Unique(), "x@example.com", true), alice)
	expectCode(t, "second identity", status, env, http.StatusConflict, "CONFLICT")

	// Impersonation sessions cannot link.
	imp := uuid.New()
	status, env = e.do(e.h.NativeLinkIdentity, "google", map[string]any{"id_token": e.googleToken(sub, "a@example.com", true), "nonce": nativeNonce},
		&authctx.Principal{UserID: bob.UUID, UserInternal: bob.ID, ImpersonatorUserID: &imp}, false)
	expectCode(t, "impersonating", status, env, http.StatusForbidden, "FORBIDDEN")

	// The linked identity signs Alice in (web session without X-Client).
	status, env = e.do(e.h.NativeOAuthSignIn, "google", map[string]any{"id_token": e.googleToken(sub, "alice-google@example.com", true), "nonce": nativeNonce}, nil, false)
	e.signedInOnPersonal("linked by endpoint", status, env, alice.ID)
}

func TestIntegrationNativeOAuthRateLimitPerSubject(t *testing.T) {
	e := newNativeEnv(t)
	e.enable("google", false, nativeGoogleAud)
	e.gate.open = false
	tok := e.googleToken("google-"+pgtest.Unique(), "rl-"+pgtest.Unique()+"@example.com", true)
	for i := range 10 {
		status, env := e.signIn("google", tok, nil)
		expectCode(t, "attempt "+string(rune('0'+i)), status, env, http.StatusForbidden, "REGISTRATION_DISABLED")
	}
	status, env := e.signIn("google", tok, nil)
	expectCode(t, "11th attempt", status, env, http.StatusTooManyRequests, "RATE_LIMITED")
}
