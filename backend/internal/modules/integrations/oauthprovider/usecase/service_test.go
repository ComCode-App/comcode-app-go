package usecase

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/database/db"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/appleauth"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/crypto"
	"github.com/golang-jwt/jwt/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// memStore mimics oauth_provider_settings.sql (COALESCE / clear semantics).
type memStore struct {
	rows map[string]db.OauthProviderSetting
}

func newMemStore() *memStore {
	return &memStore{rows: map[string]db.OauthProviderSetting{
		"google": {Provider: "google"}, "facebook": {Provider: "facebook"}, "apple": {Provider: "apple"},
	}}
}

func (m *memStore) GetOAuthProviderSettings(_ context.Context, p string) (db.OauthProviderSetting, error) {
	row, ok := m.rows[p]
	if !ok {
		return db.OauthProviderSetting{}, errors.New("not found")
	}
	return row, nil
}

func (m *memStore) ListOAuthProviderSettings(context.Context) ([]db.OauthProviderSetting, error) {
	return nil, nil
}

func (m *memStore) UpdateOAuthProviderSettings(_ context.Context, a db.UpdateOAuthProviderSettingsParams) (db.OauthProviderSetting, error) {
	row := m.rows[a.Provider]
	if a.LoginEnabled.Valid {
		row.LoginEnabled = a.LoginEnabled.Bool
	}
	if a.RegisterEnabled.Valid {
		row.RegisterEnabled = a.RegisterEnabled.Bool
	}
	if a.ClientID.Valid {
		row.ClientID = a.ClientID.String
	}
	if a.ClientSecretEnc.Valid {
		row.ClientSecretEnc = a.ClientSecretEnc
	}
	if a.AppleTeamID.Valid {
		row.AppleTeamID = a.AppleTeamID.String
	}
	if a.AppleKeyID.Valid {
		row.AppleKeyID = a.AppleKeyID.String
	}
	switch {
	case a.ClearApplePrivateKey:
		row.ApplePrivateKeyEnc = pgtype.Text{}
	case a.ApplePrivateKeyEnc.Valid:
		row.ApplePrivateKeyEnc = a.ApplePrivateKeyEnc
	}
	m.rows[a.Provider] = row
	return row, nil
}

func testBox(t *testing.T) *crypto.SecretBox {
	t.Helper()
	raw := make([]byte, 32)
	for i := range raw {
		raw[i] = byte(i + 7)
	}
	box, err := crypto.NewSecretBox(base64.StdEncoding.EncodeToString(raw))
	if err != nil {
		t.Fatal(err)
	}
	return box
}

func testP8(t *testing.T) (string, *ecdsa.PrivateKey) {
	t.Helper()
	k, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	der, err := x509.MarshalPKCS8PrivateKey(k)
	if err != nil {
		t.Fatal(err)
	}
	return string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})), k
}

// pemBody is the base64 payload of a PEM key, the part that must never leak.
func pemBody(pemText string) string {
	lines := strings.Split(strings.TrimSpace(pemText), "\n")
	return strings.Join(lines[1:len(lines)-1], "")
}

func ptr[T any](v T) *T { return &v }

func assertNoKey(t *testing.T, what string, v any, pemText string, enc string) {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	body := pemBody(pemText)
	for _, leak := range []string{body[:24], body[len(body)-24:], "PRIVATE KEY", "private_key\"", enc} {
		if leak != "" && strings.Contains(string(b), leak) {
			t.Fatalf("%s leaks key material (%q): %s", what, leak, b)
		}
	}
}

func TestApplePrivateKeyStoredEncryptedAndNeverReturned(t *testing.T) {
	store := newMemStore()
	box := testBox(t)
	svc := New(store, box)
	ctx := context.Background()
	pemText, key := testP8(t)

	got, err := svc.Patch(ctx, "apple", PatchInput{
		LoginEnabled: ptr(true), ClientID: ptr("com.example.web"),
		TeamID: ptr("team000001"), KeyID: ptr(" key0000001 "), PrivateKey: ptr(pemText),
	})
	if err != nil {
		t.Fatalf("patch: %v", err)
	}

	row := store.rows["apple"]
	if !row.ApplePrivateKeyEnc.Valid || strings.Contains(row.ApplePrivateKeyEnc.String, pemBody(pemText)[:24]) ||
		strings.Contains(row.ApplePrivateKeyEnc.String, "PRIVATE KEY") {
		t.Fatalf("key not stored encrypted: %q", row.ApplePrivateKeyEnc.String)
	}
	if plain, err := box.Decrypt(row.ApplePrivateKeyEnc.String); err != nil || strings.TrimSpace(plain) != strings.TrimSpace(pemText) {
		t.Fatalf("stored key does not decrypt to the upload: %v", err)
	}
	if row.AppleTeamID != "TEAM000001" || row.AppleKeyID != "KEY0000001" {
		t.Fatalf("ids not normalised: %q %q", row.AppleTeamID, row.AppleKeyID)
	}

	assertNoKey(t, "patch response", got, pemText, row.ApplePrivateKeyEnc.String)
	if got.PrivateKeyConfigured == nil || !*got.PrivateKeyConfigured ||
		*got.PrivateKeyFingerprint != appleauth.Fingerprint(key) || *got.ClientSecretSource != AppleSecretFromKey {
		t.Fatalf("unexpected settings: %+v", got)
	}
	read, err := svc.Get(ctx, "apple")
	if err != nil {
		t.Fatal(err)
	}
	assertNoKey(t, "get response", read, pemText, row.ApplePrivateKeyEnc.String)

	// The web client gets a generated client secret signed with the key.
	now := time.Date(2026, 11, 9, 12, 0, 0, 0, time.UTC)
	svc.now = func() time.Time { return now }
	cfg, err := svc.GetAdapterOAuthConfig(ctx, "apple")
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.Enabled || cfg.ClientSecretExpiresAt == nil || !cfg.ClientSecretExpiresAt.Equal(now.Add(AppleWebClientSecretTTL)) {
		t.Fatalf("unexpected adapter config: %+v", cfg)
	}
	assertNoKey(t, "adapter config", cfg, pemText, row.ApplePrivateKeyEnc.String)
	claims := jwt.MapClaims{}
	tok, err := jwt.ParseWithClaims(cfg.ClientSecret, claims, func(*jwt.Token) (any, error) { return &key.PublicKey, nil },
		jwt.WithValidMethods([]string{"ES256"}), jwt.WithTimeFunc(func() time.Time { return now }))
	if err != nil || !tok.Valid {
		t.Fatalf("client secret does not verify: %v", err)
	}
	if claims["sub"] != "com.example.web" || claims["iss"] != "TEAM000001" || tok.Header["kid"] != "KEY0000001" {
		t.Fatalf("claims: %v header: %v", claims, tok.Header)
	}
}

func TestAppleKeyPrecedenceOverPastedSecret(t *testing.T) {
	store := newMemStore()
	svc := New(store, testBox(t))
	ctx := context.Background()

	// Pasted JWT only: used as is.
	if _, err := svc.Patch(ctx, "apple", PatchInput{LoginEnabled: ptr(true), ClientID: ptr("com.example.web"), ClientSecret: ptr("pasted.jwt.value")}); err != nil {
		t.Fatal(err)
	}
	cfg, _ := svc.GetAdapterOAuthConfig(ctx, "apple")
	if cfg.ClientSecret != "pasted.jwt.value" || cfg.ClientSecretExpiresAt != nil {
		t.Fatalf("pasted secret not used: %+v", cfg)
	}
	s, _ := svc.Get(ctx, "apple")
	if *s.ClientSecretSource != AppleSecretFromPasted {
		t.Fatalf("source = %q", *s.ClientSecretSource)
	}

	// Key uploaded: generated secret wins.
	pemText, _ := testP8(t)
	if _, err := svc.Patch(ctx, "apple", PatchInput{TeamID: ptr("TEAM000001"), KeyID: ptr("KEY0000001"), PrivateKey: ptr(pemText)}); err != nil {
		t.Fatal(err)
	}
	cfg, _ = svc.GetAdapterOAuthConfig(ctx, "apple")
	if cfg.ClientSecret == "pasted.jwt.value" || cfg.ClientSecretExpiresAt == nil {
		t.Fatalf("key did not take precedence: %+v", cfg)
	}

	// Key removed: back to the pasted JWT.
	if _, err := svc.Patch(ctx, "apple", PatchInput{RemovePrivateKey: ptr(true)}); err != nil {
		t.Fatal(err)
	}
	if store.rows["apple"].ApplePrivateKeyEnc.Valid {
		t.Fatal("key not cleared")
	}
	cfg, _ = svc.GetAdapterOAuthConfig(ctx, "apple")
	if cfg.ClientSecret != "pasted.jwt.value" {
		t.Fatalf("fallback not used: %+v", cfg)
	}
}

func TestAppleKeyOnlyEnablesSignIn(t *testing.T) {
	svc := New(newMemStore(), testBox(t))
	ctx := context.Background()
	pemText, _ := testP8(t)
	if _, err := svc.Patch(ctx, "apple", PatchInput{LoginEnabled: ptr(true), ClientID: ptr("com.example.web")}); !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("enable without secret or key: %v", err)
	}
	if _, err := svc.Patch(ctx, "apple", PatchInput{
		LoginEnabled: ptr(true), ClientID: ptr("com.example.web"),
		TeamID: ptr("TEAM000001"), KeyID: ptr("KEY0000001"), PrivateKey: ptr(pemText),
	}); err != nil {
		t.Fatalf("enable with key only: %v", err)
	}
	if ok, _ := svc.IsLoginEnabled(ctx, "apple"); !ok {
		t.Fatal("login should be enabled with a key")
	}
}

func TestApplePatchValidation(t *testing.T) {
	pemText, _ := testP8(t)
	p384, _ := ecdsa.GenerateKey(elliptic.P384(), rand.Reader)
	der, _ := x509.MarshalPKCS8PrivateKey(p384)
	wrongCurve := string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}))
	cases := map[string]PatchInput{
		"not a key":      {TeamID: ptr("TEAM000001"), KeyID: ptr("KEY0000001"), PrivateKey: ptr("hello")},
		"wrong curve":    {TeamID: ptr("TEAM000001"), KeyID: ptr("KEY0000001"), PrivateKey: ptr(wrongCurve)},
		"bad team id":    {TeamID: ptr("TEAM"), KeyID: ptr("KEY0000001"), PrivateKey: ptr(pemText)},
		"missing key id": {TeamID: ptr("TEAM000001"), PrivateKey: ptr(pemText)},
		"key and remove": {TeamID: ptr("TEAM000001"), KeyID: ptr("KEY0000001"), PrivateKey: ptr(pemText), RemovePrivateKey: ptr(true)},
	}
	for name, in := range cases {
		store := newMemStore()
		_, err := New(store, testBox(t)).Patch(context.Background(), "apple", in)
		if !errors.Is(err, ErrInvalidRequest) {
			t.Errorf("%s: err = %v", name, err)
			continue
		}
		if strings.Contains(err.Error(), "MIG") || strings.Contains(err.Error(), "BEGIN") {
			t.Errorf("%s: error echoes the key: %v", name, err)
		}
		if store.rows["apple"].ApplePrivateKeyEnc.Valid {
			t.Errorf("%s: key stored despite error", name)
		}
	}

	// Apple fields are refused on other providers.
	if _, err := New(newMemStore(), testBox(t)).Patch(context.Background(), "google", PatchInput{PrivateKey: ptr(pemText)}); !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("google with private_key: %v", err)
	}
	g, _ := New(newMemStore(), testBox(t)).Get(context.Background(), "google")
	if g.TeamID != nil || g.PrivateKeyConfigured != nil {
		t.Fatalf("google settings carry apple fields: %+v", g)
	}
}
