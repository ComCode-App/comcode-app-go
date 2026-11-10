package oidc

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"math/big"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const (
	testAppleIss = "https://appleid.apple.com"
	testBundleID = "app.comcode.mobile"
)

// testIssuer serves a JWKS with one RSA key and signs tokens with it.
type testIssuer struct {
	mu     sync.Mutex
	key    *rsa.PrivateKey
	kid    string
	status int
	srv    *httptest.Server
	hits   atomic.Int32
}

func newTestIssuer(t *testing.T) *testIssuer {
	t.Helper()
	ti := &testIssuer{key: mustKey(t), kid: "k1", status: http.StatusOK}
	ti.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		ti.hits.Add(1)
		ti.mu.Lock()
		defer ti.mu.Unlock()
		if ti.status != http.StatusOK {
			w.WriteHeader(ti.status)
			return
		}
		w.Header().Set("Cache-Control", "public, max-age=3600")
		_ = json.NewEncoder(w).Encode(map[string]any{"keys": []map[string]string{{
			"kty": "RSA", "kid": ti.kid, "alg": "RS256", "use": "sig",
			"n": base64.RawURLEncoding.EncodeToString(ti.key.N.Bytes()),
			"e": base64.RawURLEncoding.EncodeToString(big.NewInt(int64(ti.key.E)).Bytes()),
		}}})
	}))
	t.Cleanup(ti.srv.Close)
	return ti
}

func mustKey(t *testing.T) *rsa.PrivateKey {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	return key
}

// rotate replaces the served key (provider key rotation).
func (ti *testIssuer) rotate(t *testing.T, kid string) {
	ti.mu.Lock()
	defer ti.mu.Unlock()
	ti.key, ti.kid = mustKey(t), kid
}

func (ti *testIssuer) sign(t *testing.T, claims jwt.MapClaims) string {
	t.Helper()
	ti.mu.Lock()
	key, kid := ti.key, ti.kid
	ti.mu.Unlock()
	return signWith(t, key, kid, claims)
}

func signWith(t *testing.T, key *rsa.PrivateKey, kid string, claims jwt.MapClaims) string {
	t.Helper()
	tok := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	tok.Header["kid"] = kid
	raw, err := tok.SignedString(key)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func baseClaims(now time.Time) jwt.MapClaims {
	return jwt.MapClaims{
		"iss":              testAppleIss,
		"aud":              testBundleID,
		"sub":              "001234.abc",
		"email":            "X@privaterelay.appleid.com",
		"email_verified":   "true",
		"is_private_email": "true",
		"iat":              now.Unix(),
		"exp":              now.Add(10 * time.Minute).Unix(),
		"nonce":            "raw-nonce",
	}
}

func appleVerifier(ti *testIssuer) *Verifier {
	return New(map[string]ProviderConfig{ProviderApple: {Issuers: []string{testAppleIss}, JWKSURL: ti.srv.URL}}, nil)
}

func TestVerifyValidToken(t *testing.T) {
	ti := newTestIssuer(t)
	v := appleVerifier(ti)
	now := time.Now()
	ctx := context.Background()
	aud := []string{"other.audience", testBundleID}

	claims, err := v.Verify(ctx, ProviderApple, ti.sign(t, baseClaims(now)), aud, "raw-nonce")
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if claims.Subject != "001234.abc" || claims.Email != "x@privaterelay.appleid.com" ||
		!claims.EmailVerified || !claims.IsPrivateEmail || claims.Audience != testBundleID {
		t.Fatalf("claims = %+v", claims)
	}

	// The token carries the SHA-256 of the nonce the app kept.
	c := baseClaims(now)
	sum := sha256.Sum256([]byte("raw-nonce"))
	c["nonce"] = hex.EncodeToString(sum[:])
	if _, err := v.Verify(ctx, ProviderApple, ti.sign(t, c), aud, "raw-nonce"); err != nil {
		t.Fatalf("hashed nonce: %v", err)
	}

	// JSON booleans (Google) work like Apple's strings.
	c = baseClaims(now)
	c["email_verified"] = false
	claims, err = v.Verify(ctx, ProviderApple, ti.sign(t, c), aud, "raw-nonce")
	if err != nil || claims.EmailVerified {
		t.Fatalf("email_verified=false: %+v %v", claims, err)
	}

	if hits := ti.hits.Load(); hits != 1 {
		t.Fatalf("jwks fetched %d times, want 1 (cached)", hits)
	}
}

func TestVerifyRejectsBadClaims(t *testing.T) {
	ti := newTestIssuer(t)
	v := appleVerifier(ti)
	now := time.Now()
	ctx := context.Background()
	aud := []string{testBundleID}

	cases := map[string]func(jwt.MapClaims){
		"wrong aud":         func(c jwt.MapClaims) { c["aud"] = "com.other.app" },
		"wrong iss":         func(c jwt.MapClaims) { c["iss"] = "https://evil.example" },
		"expired":           func(c jwt.MapClaims) { c["exp"] = now.Add(-10 * time.Minute).Unix() },
		"not yet valid":     func(c jwt.MapClaims) { c["nbf"] = now.Add(10 * time.Minute).Unix() },
		"nonce mismatch":    func(c jwt.MapClaims) { c["nonce"] = "other" },
		"missing nonce":     func(c jwt.MapClaims) { delete(c, "nonce") },
		"missing sub":       func(c jwt.MapClaims) { delete(c, "sub") },
		"missing exp":       func(c jwt.MapClaims) { delete(c, "exp") },
		"hash of wrong one": func(c jwt.MapClaims) { s := sha256.Sum256([]byte("x")); c["nonce"] = hex.EncodeToString(s[:]) },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			c := baseClaims(now)
			mutate(c)
			if _, err := v.Verify(ctx, ProviderApple, ti.sign(t, c), aud, "raw-nonce"); !errors.Is(err, ErrInvalidToken) {
				t.Fatalf("expected ErrInvalidToken, got %v", err)
			}
		})
	}

	if _, err := v.Verify(ctx, ProviderApple, ti.sign(t, baseClaims(now)), nil, ""); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("no audiences configured must fail: %v", err)
	}
	if _, err := v.Verify(ctx, ProviderApple, "  ", aud, ""); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("empty token: %v", err)
	}
}

func TestVerifyRejectsBadSignature(t *testing.T) {
	ti := newTestIssuer(t)
	v := appleVerifier(ti)
	now := time.Now()
	ctx := context.Background()
	aud := []string{testBundleID}

	// Same kid, signed by another key.
	forged := signWith(t, mustKey(t), ti.kid, baseClaims(now))
	if _, err := v.Verify(ctx, ProviderApple, forged, aud, "raw-nonce"); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("forged: expected ErrInvalidToken, got %v", err)
	}

	// A valid token with a tampered payload.
	good := ti.sign(t, baseClaims(now))
	c := baseClaims(now)
	c["sub"] = "someone-else"
	other := ti.sign(t, c)
	parts, otherParts := splitJWT(good), splitJWT(other)
	tampered := parts[0] + "." + otherParts[1] + "." + parts[2]
	if _, err := v.Verify(ctx, ProviderApple, tampered, aud, "raw-nonce"); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("tampered: expected ErrInvalidToken, got %v", err)
	}

	// HS256 with the public modulus as the secret (algorithm confusion) and alg=none.
	hs := jwt.NewWithClaims(jwt.SigningMethodHS256, baseClaims(now))
	hs.Header["kid"] = ti.kid
	hsRaw, _ := hs.SignedString(ti.key.N.Bytes())
	if _, err := v.Verify(ctx, ProviderApple, hsRaw, aud, "raw-nonce"); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("HS256: expected ErrInvalidToken, got %v", err)
	}
	none := jwt.NewWithClaims(jwt.SigningMethodNone, baseClaims(now))
	noneRaw, _ := none.SignedString(jwt.UnsafeAllowNoneSignatureType)
	if _, err := v.Verify(ctx, ProviderApple, noneRaw, aud, "raw-nonce"); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("alg none: expected ErrInvalidToken, got %v", err)
	}
}

func splitJWT(raw string) []string {
	out := make([]string, 0, 3)
	start := 0
	for i := 0; i < len(raw); i++ {
		if raw[i] == '.' {
			out = append(out, raw[start:i])
			start = i + 1
		}
	}
	return append(out, raw[start:])
}

// An unknown kid re-fetches the JWKS at most once per RefetchInterval.
func TestVerifyUnknownKidRefetchIsRateLimited(t *testing.T) {
	ti := newTestIssuer(t)
	v := appleVerifier(ti)
	now := time.Now()
	v.SetClock(func() time.Time { return now })
	ctx := context.Background()
	aud := []string{testBundleID}

	if _, err := v.Verify(ctx, ProviderApple, ti.sign(t, baseClaims(now)), aud, ""); err != nil {
		t.Fatalf("verify: %v", err)
	}
	// Forged kids never hit the provider again within the interval.
	for i := range 5 {
		bogus := signWith(t, mustKey(t), "bogus-"+string(rune('a'+i)), baseClaims(now))
		if _, err := v.Verify(ctx, ProviderApple, bogus, aud, ""); !errors.Is(err, ErrInvalidToken) {
			t.Fatalf("bogus kid: %v", err)
		}
	}
	if hits := ti.hits.Load(); hits != 1 {
		t.Fatalf("jwks fetched %d times, want 1 (no re-fetch within the interval)", hits)
	}

	// The provider rotates its key: still refused inside the interval, accepted after it.
	ti.rotate(t, "k2")
	rotated := ti.sign(t, baseClaims(now))
	if _, err := v.Verify(ctx, ProviderApple, rotated, aud, ""); err == nil {
		t.Fatal("expected unknown kid failure within the re-fetch interval")
	}
	if hits := ti.hits.Load(); hits != 1 {
		t.Fatalf("jwks re-fetched within the interval: %d", hits)
	}
	later := now.Add(RefetchInterval + time.Second)
	v.SetClock(func() time.Time { return later })
	if _, err := v.Verify(ctx, ProviderApple, rotated, aud, ""); err != nil {
		t.Fatalf("after the interval the rotated key should verify: %v", err)
	}
	if hits := ti.hits.Load(); hits != 2 {
		t.Fatalf("jwks fetched %d times, want 2", hits)
	}
	// Another bogus kid right after that re-fetch stays local.
	bogus := signWith(t, mustKey(t), "bogus-z", baseClaims(now))
	if _, err := v.Verify(ctx, ProviderApple, bogus, aud, ""); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("bogus kid: %v", err)
	}
	if hits := ti.hits.Load(); hits != 2 {
		t.Fatalf("jwks fetched %d times after a bogus kid, want 2", hits)
	}
}

// A failing JWKS endpoint is not retried on every request either.
func TestVerifyFailedFetchIsRateLimited(t *testing.T) {
	ti := newTestIssuer(t)
	ti.status = http.StatusServiceUnavailable
	v := appleVerifier(ti)
	now := time.Now()
	v.SetClock(func() time.Time { return now })
	for range 3 {
		if _, err := v.Verify(context.Background(), ProviderApple, ti.sign(t, baseClaims(now)), []string{testBundleID}, ""); !errors.Is(err, ErrInvalidToken) {
			t.Fatalf("expected failure: %v", err)
		}
	}
	if hits := ti.hits.Load(); hits != 1 {
		t.Fatalf("failing jwks fetched %d times, want 1", hits)
	}
}

func TestVerifyUnknownProvider(t *testing.T) {
	v := New(DefaultProviders(), nil)
	if _, err := v.Verify(context.Background(), "github", "x", []string{"a"}, ""); !errors.Is(err, ErrUnknownProvider) {
		t.Fatalf("got %v", err)
	}
}

func TestCacheTTL(t *testing.T) {
	cases := map[string]time.Duration{
		"":                       defaultJWKSTTL,
		"public, max-age=600":    10 * time.Minute,
		"max-age=0":              defaultJWKSTTL,
		"max-age=999999, public": maxJWKSTTL,
		"no-cache":               defaultJWKSTTL,
	}
	for header, want := range cases {
		if got := cacheTTL(header); got != want {
			t.Fatalf("cacheTTL(%q) = %v, want %v", header, got, want)
		}
	}
}
