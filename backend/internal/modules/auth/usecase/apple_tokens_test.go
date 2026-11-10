package usecase

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/auth/model"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/auth/repository"
	oauthproviderusecase "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/integrations/oauthprovider/usecase"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/appleauth"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/crypto"
	"github.com/golang-jwt/jwt/v5"
)

const (
	appleTestBundle   = "app.comcode.mobile"
	appleTestServices = "app.comcode.web"
	appleTestCode     = "c0de-PLAINTEXT"
	appleTestRefresh  = "rt-PLAINTEXT-123"
)

type appleStoreRow struct {
	sub      string
	enc      *string
	clientID *string
}

type fakeAppleStore struct {
	mu   sync.Mutex
	rows map[int64]*appleStoreRow
}

func (s *fakeAppleStore) GetOAuthAccountToken(_ context.Context, userID int64, provider string) (model.OAuthAccountToken, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	r, ok := s.rows[userID]
	if !ok || provider != model.OAuthProviderApple {
		return model.OAuthAccountToken{}, repository.ErrNotFound
	}
	return model.OAuthAccountToken{ID: userID, UserID: userID, Provider: provider, ProviderAccountID: r.sub, RefreshTokenEnc: r.enc, ClientID: r.clientID}, nil
}

func (s *fakeAppleStore) SetOAuthAccountRefreshToken(_ context.Context, userID int64, provider, sub string, enc, clientID *string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	r, ok := s.rows[userID]
	if !ok || r.sub != sub || provider != model.OAuthProviderApple {
		return repository.ErrNotFound
	}
	r.enc, r.clientID = enc, clientID
	return nil
}

type fakeAppleCreds struct{ creds oauthproviderusecase.AppleTokenCredentials }

func (f fakeAppleCreds) AppleTokenCredentials(context.Context) (oauthproviderusecase.AppleTokenCredentials, error) {
	return f.creds, nil
}

type appleCall struct {
	path string
	form url.Values
}

type appleFixture struct {
	t      *testing.T
	key    appleauth.SigningKey
	box    *crypto.SecretBox
	store  *fakeAppleStore
	tokens *AppleTokens
	logs   *bytes.Buffer
	status int
	calls  []appleCall
	mu     sync.Mutex
}

func newAppleFixture(t *testing.T, withKey bool) *appleFixture {
	t.Helper()
	f := &appleFixture{t: t, status: http.StatusOK, logs: &bytes.Buffer{}, store: &fakeAppleStore{rows: map[int64]*appleStoreRow{}}}
	ec, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	f.key = appleauth.SigningKey{TeamID: "TEAM123456", KeyID: "KEY1234567", Key: ec}
	raw := make([]byte, 32)
	_, _ = rand.Read(raw)
	if f.box, err = crypto.NewSecretBox(base64.StdEncoding.EncodeToString(raw)); err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		f.mu.Lock()
		f.calls = append(f.calls, appleCall{r.URL.Path, r.PostForm})
		status := f.status
		f.mu.Unlock()
		if status != http.StatusOK {
			w.WriteHeader(status)
			_, _ = w.Write([]byte(`{"error":"invalid_grant"}`))
			return
		}
		if r.URL.Path == "/auth/token" {
			_, _ = w.Write([]byte(`{"access_token":"at-PLAINTEXT","refresh_token":"` + appleTestRefresh + `","id_token":"x"}`))
		}
	}))
	t.Cleanup(srv.Close)
	creds := oauthproviderusecase.AppleTokenCredentials{WebClientID: appleTestServices, WebClientSecret: "pasted-web-secret"}
	if withKey {
		creds.Key = &f.key
	}
	log := slog.New(slog.NewTextHandler(f.logs, &slog.HandlerOptions{Level: slog.LevelDebug}))
	f.tokens = NewAppleTokens(f.store, f.box, fakeAppleCreds{creds}, appleauth.NewTokenClient(srv.URL, nil), log)
	return f
}

func (f *appleFixture) lastCall() appleCall {
	f.t.Helper()
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.callCount() == 0 {
		f.t.Fatal("no Apple call")
	}
	return f.calls[f.callCount()-1]
}

func (f *appleFixture) noSecretsLogged() {
	f.t.Helper()
	out := f.logs.String()
	for _, s := range []string{appleTestCode, appleTestRefresh, "at-PLAINTEXT", "PRIVATE KEY", "pasted-web-secret", "eyJ"} {
		if strings.Contains(out, s) {
			f.t.Fatalf("log contains %q:\n%s", s, out)
		}
	}
}

func (f *appleFixture) checkSecret(secret, clientID string) {
	f.t.Helper()
	tok, err := jwt.Parse(secret, func(*jwt.Token) (any, error) { return &f.key.Key.PublicKey, nil },
		jwt.WithValidMethods([]string{"ES256"}), jwt.WithIssuer(f.key.TeamID), jwt.WithSubject(clientID),
		jwt.WithAudience(appleauth.Audience))
	if err != nil || tok.Header["kid"] != f.key.KeyID {
		f.t.Fatalf("client_secret for %s: %v (header %v)", clientID, err, tok.Header)
	}
}

func TestAppleStoreFromCodeStoresEncryptedTokenForBundleID(t *testing.T) {
	f := newAppleFixture(t, true)
	f.store.rows[7] = &appleStoreRow{sub: "apple-sub"}
	f.tokens.StoreFromCode(context.Background(), 7, "apple-sub", appleTestBundle, appleTestCode)

	call := f.lastCall()
	if call.path != "/auth/token" || call.form.Get("client_id") != appleTestBundle || call.form.Get("code") != appleTestCode {
		t.Fatalf("call = %+v", call)
	}
	f.checkSecret(call.form.Get("client_secret"), appleTestBundle)
	row := f.store.rows[7]
	if row.enc == nil || strings.Contains(*row.enc, appleTestRefresh) || row.clientID == nil || *row.clientID != appleTestBundle {
		t.Fatalf("stored = %+v", row)
	}
	if plain, err := f.box.Decrypt(*row.enc); err != nil || plain != appleTestRefresh {
		t.Fatalf("decrypt = %q %v", plain, err)
	}
	f.noSecretsLogged()
}

func TestAppleStoreFromCodeFailureIsLoggedOnly(t *testing.T) {
	f := newAppleFixture(t, true)
	f.store.rows[7] = &appleStoreRow{sub: "apple-sub"}
	f.status = http.StatusBadRequest
	f.tokens.StoreFromCode(context.Background(), 7, "apple-sub", appleTestBundle, appleTestCode)
	if f.store.rows[7].enc != nil {
		t.Fatal("token stored after a failed exchange")
	}
	if !strings.Contains(f.logs.String(), "auth_apple_code_exchange_failed") || !strings.Contains(f.logs.String(), "invalid_grant") {
		t.Fatalf("logs = %s", f.logs)
	}
	f.noSecretsLogged()

	// No code: no call.
	n := f.callCount()
	f.tokens.StoreFromCode(context.Background(), 7, "apple-sub", appleTestBundle, "  ")
	if f.callCount() != n {
		t.Fatal("exchange without a code")
	}
	// No signing key: skipped without calling Apple.
	nokey := newAppleFixture(t, false)
	nokey.store.rows[7] = &appleStoreRow{sub: "apple-sub"}
	nokey.tokens.StoreFromCode(context.Background(), 7, "apple-sub", appleTestBundle, appleTestCode)
	if nokey.callCount() != 0 || !strings.Contains(nokey.logs.String(), "no_signing_key") {
		t.Fatalf("no key: calls %d logs %s", nokey.callCount(), nokey.logs)
	}
	// A nil helper is a no-op.
	var off *AppleTokens
	off.StoreFromCode(context.Background(), 7, "apple-sub", appleTestBundle, appleTestCode)
	if err := off.RevokeForUser(context.Background(), 7); err != nil {
		t.Fatal(err)
	}
}

func (f *appleFixture) seed(userID int64, clientID *string) {
	f.t.Helper()
	enc, err := f.box.Encrypt(appleTestRefresh)
	if err != nil {
		f.t.Fatal(err)
	}
	f.store.rows[userID] = &appleStoreRow{sub: "apple-sub", enc: &enc, clientID: clientID}
}

func TestAppleRevokeForUser(t *testing.T) {
	f := newAppleFixture(t, true)
	bundle := appleTestBundle
	f.seed(7, &bundle)
	if err := f.tokens.RevokeForUser(context.Background(), 7); err != nil {
		t.Fatal(err)
	}
	call := f.lastCall()
	if call.path != "/auth/revoke" || call.form.Get("token") != appleTestRefresh || call.form.Get("client_id") != appleTestBundle {
		t.Fatalf("call = %+v", call)
	}
	f.checkSecret(call.form.Get("client_secret"), appleTestBundle)
	if row := f.store.rows[7]; row.enc != nil || row.clientID == nil || *row.clientID != appleTestBundle {
		t.Fatalf("after revoke: %+v", row)
	}
	// Nothing stored any more / no identity: nothing to do.
	n := f.callCount()
	if err := f.tokens.RevokeForUser(context.Background(), 7); err != nil {
		t.Fatal(err)
	}
	if err := f.tokens.RevokeForUser(context.Background(), 99); err != nil {
		t.Fatal(err)
	}
	if f.callCount() != n {
		t.Fatal("revoke without a stored token")
	}

	// A failure is returned (for the caller to log) and the token is kept.
	f.seed(8, &bundle)
	f.status = http.StatusBadRequest
	if err := f.tokens.RevokeForUser(context.Background(), 8); err == nil || strings.Contains(err.Error(), appleTestRefresh) {
		t.Fatalf("failed revoke = %v", err)
	}
	if f.store.rows[8].enc == nil {
		t.Fatal("token cleared after a failed revoke")
	}
}

// A web (Auth.js) token has no client id: it is revoked with the Services ID, signed by the
// stored key, or with the pasted web client secret when there is no key.
func TestAppleRevokeWebTokenFallsBackToServicesID(t *testing.T) {
	f := newAppleFixture(t, true)
	f.seed(7, nil)
	if err := f.tokens.RevokeForUser(context.Background(), 7); err != nil {
		t.Fatal(err)
	}
	call := f.lastCall()
	if call.form.Get("client_id") != appleTestServices {
		t.Fatalf("client_id = %q", call.form.Get("client_id"))
	}
	f.checkSecret(call.form.Get("client_secret"), appleTestServices)

	nokey := newAppleFixture(t, false)
	nokey.seed(7, nil)
	if err := nokey.tokens.RevokeForUser(context.Background(), 7); err != nil {
		t.Fatal(err)
	}
	if got := nokey.lastCall().form.Get("client_secret"); got != "pasted-web-secret" {
		t.Fatalf("client_secret = %q", got)
	}
	// A native token cannot be revoked without the key.
	bundle := appleTestBundle
	nokey.seed(8, &bundle)
	if err := nokey.tokens.RevokeForUser(context.Background(), 8); !errors.Is(err, ErrAppleTokenNotConfigured) {
		t.Fatalf("native without key = %v", err)
	}
}

func (f *appleFixture) callCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.calls)
}
