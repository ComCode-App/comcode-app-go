package appleauth

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// fakeApple records the last form posted to /auth/token or /auth/revoke.
type fakeApple struct {
	srv    *httptest.Server
	path   string
	form   url.Values
	status int
	body   string
}

func newFakeApple(t *testing.T) *fakeApple {
	t.Helper()
	f := &fakeApple{status: http.StatusOK, body: `{"access_token":"at-1","refresh_token":"rt-1","id_token":"x","token_type":"Bearer","expires_in":3600}`}
	f.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.Header.Get("Content-Type") != "application/x-www-form-urlencoded" {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		_ = r.ParseForm()
		f.path, f.form = r.URL.Path, r.PostForm
		w.WriteHeader(f.status)
		_, _ = w.Write([]byte(f.body))
	}))
	t.Cleanup(f.srv.Close)
	return f
}

func testSigningKey(t *testing.T) SigningKey {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	return SigningKey{TeamID: "TEAM123456", KeyID: "KEY1234567", Key: key}
}

// checkClientSecret verifies the client_secret JWT Apple receives: ES256 with kid, iss = team id,
// sub = client_id, aud = https://appleid.apple.com (a string), short-lived.
func checkClientSecret(t *testing.T, k SigningKey, secret, clientID string) {
	t.Helper()
	tok, err := jwt.Parse(secret, func(tok *jwt.Token) (any, error) { return &k.Key.PublicKey, nil },
		jwt.WithValidMethods([]string{"ES256"}), jwt.WithIssuer(k.TeamID), jwt.WithSubject(clientID),
		jwt.WithAudience(Audience), jwt.WithIssuedAt(), jwt.WithExpirationRequired())
	if err != nil || !tok.Valid {
		t.Fatalf("client_secret: %v", err)
	}
	if tok.Header["kid"] != k.KeyID || tok.Header["alg"] != "ES256" {
		t.Fatalf("header = %v", tok.Header)
	}
	claims := tok.Claims.(jwt.MapClaims)
	if aud, ok := claims["aud"].(string); !ok || aud != Audience {
		t.Fatalf("aud = %#v, want the string %q", claims["aud"], Audience)
	}
	exp, _ := claims.GetExpirationTime()
	iat, _ := claims.GetIssuedAt()
	if d := exp.Sub(iat.Time); d <= 0 || d > TokenClientSecretTTL {
		t.Fatalf("lifetime %s, want at most %s", d, TokenClientSecretTTL)
	}
}

func TestTokenClientExchangeCode(t *testing.T) {
	f := newFakeApple(t)
	k := testSigningKey(t)
	secret, _, err := k.ClientSecret("app.comcode.mobile", TokenClientSecretTTL, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	c := NewTokenClient(f.srv.URL+"/", nil)
	rt, err := c.ExchangeCode(context.Background(), "app.comcode.mobile", secret, " code-1 ")
	if err != nil || rt != "rt-1" {
		t.Fatalf("exchange = %q, %v", rt, err)
	}
	if f.path != "/auth/token" || f.form.Get("grant_type") != "authorization_code" ||
		f.form.Get("code") != "code-1" || f.form.Get("client_id") != "app.comcode.mobile" {
		t.Fatalf("request %s %v", f.path, f.form)
	}
	checkClientSecret(t, k, f.form.Get("client_secret"), "app.comcode.mobile")
}

func TestTokenClientRevoke(t *testing.T) {
	f := newFakeApple(t)
	f.body = ""
	k := testSigningKey(t)
	secret, _, _ := k.ClientSecret("app.comcode.mobile", TokenClientSecretTTL, time.Now())
	c := NewTokenClient(f.srv.URL, nil)
	if err := c.Revoke(context.Background(), "app.comcode.mobile", secret, "rt-1"); err != nil {
		t.Fatal(err)
	}
	if f.path != "/auth/revoke" || f.form.Get("token") != "rt-1" || f.form.Get("token_type_hint") != "refresh_token" ||
		f.form.Get("client_id") != "app.comcode.mobile" {
		t.Fatalf("request %s %v", f.path, f.form)
	}
	checkClientSecret(t, k, f.form.Get("client_secret"), "app.comcode.mobile")
}

// Errors name the endpoint, status and Apple's error code, never the code, token, secret or
// response text.
func TestTokenClientErrorsCarryNoSecrets(t *testing.T) {
	f := newFakeApple(t)
	c := NewTokenClient(f.srv.URL, nil)
	cases := map[string]struct {
		status int
		body   string
		want   string
	}{
		"invalid_grant": {http.StatusBadRequest, `{"error":"invalid_grant"}`, "status 400 (invalid_grant)"},
		"echoing body":  {http.StatusBadRequest, `{"error":"code-SECRET rt-SECRET"}`, "status 400 (unknown)"},
		"html":          {http.StatusBadGateway, `<html>code-SECRET</html>`, "status 502 (unknown)"},
	}
	for name, tc := range cases {
		f.status, f.body = tc.status, tc.body
		_, err := c.ExchangeCode(context.Background(), "app.comcode.mobile", "secret-SECRET", "code-SECRET")
		if err == nil || !strings.Contains(err.Error(), tc.want) || strings.Contains(err.Error(), "SECRET") {
			t.Fatalf("%s: exchange error = %v", name, err)
		}
		err = c.Revoke(context.Background(), "app.comcode.mobile", "secret-SECRET", "rt-SECRET")
		if err == nil || !strings.Contains(err.Error(), tc.want) || strings.Contains(err.Error(), "SECRET") {
			t.Fatalf("%s: revoke error = %v", name, err)
		}
	}

	f.status, f.body = http.StatusOK, `{"access_token":"at-SECRET"}`
	if _, err := c.ExchangeCode(context.Background(), "app.comcode.mobile", "s", "c"); err != ErrNoRefreshToken {
		t.Fatalf("no refresh token: %v", err)
	}
	f.body = `not json code-SECRET`
	if _, err := c.ExchangeCode(context.Background(), "app.comcode.mobile", "s", "c"); err == nil || strings.Contains(err.Error(), "SECRET") {
		t.Fatalf("not json: %v", err)
	}
	if _, err := c.ExchangeCode(context.Background(), "", "s", "c"); err == nil {
		t.Fatal("empty client id accepted")
	}
	if err := c.Revoke(context.Background(), "app.comcode.mobile", "s", ""); err == nil {
		t.Fatal("empty token accepted")
	}

	// Unreachable host: the error names the endpoint only.
	dead := NewTokenClient("http://127.0.0.1:1", &http.Client{Timeout: time.Second})
	if err := dead.Revoke(context.Background(), "app.comcode.mobile", "secret-SECRET", "rt-SECRET"); err == nil || strings.Contains(err.Error(), "SECRET") {
		t.Fatalf("network error = %v", err)
	}
}

func TestTokenClientDoesNotFollowRedirects(t *testing.T) {
	hit := false
	target := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { hit = true }))
	defer target.Close()
	redirect := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL, http.StatusTemporaryRedirect)
	}))
	defer redirect.Close()
	c := NewTokenClient(redirect.URL, nil)
	if err := c.Revoke(context.Background(), "app.comcode.mobile", "s", "rt"); err == nil || !strings.Contains(err.Error(), "status 307") {
		t.Fatalf("redirect: %v", err)
	}
	if hit {
		t.Fatal("redirect followed")
	}
}

func TestNewTokenClientDefaultBaseURL(t *testing.T) {
	if c := NewTokenClient("  ", nil); c.baseURL != DefaultBaseURL {
		t.Fatalf("base = %q", c.baseURL)
	}
}
