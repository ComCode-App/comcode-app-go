package appleauth

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func pkcs8PEM(t *testing.T, key any) string {
	t.Helper()
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	return string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}))
}

func TestParsePrivateKey(t *testing.T) {
	p256, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	good := pkcs8PEM(t, p256)
	if _, err := ParsePrivateKey(good); err != nil {
		t.Fatalf("valid p8: %v", err)
	}
	oneLine := strings.ReplaceAll(strings.TrimSpace(good), "\n", `\n`)
	if _, err := ParsePrivateKey(oneLine); err != nil {
		t.Fatalf("single-line p8: %v", err)
	}

	rsaKey, _ := rsa.GenerateKey(rand.Reader, 2048)
	p384, _ := ecdsa.GenerateKey(elliptic.P384(), rand.Reader)
	sec1, _ := x509.MarshalECPrivateKey(p256)
	bad := map[string]string{
		"empty":   "",
		"not pem": "hello",
		"rsa":     pkcs8PEM(t, rsaKey),
		"p384":    pkcs8PEM(t, p384),
		"sec1":    string(pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: sec1})),
		"huge":    good + strings.Repeat("A", MaxKeyBytes),
	}
	for name, text := range bad {
		_, err := ParsePrivateKey(text)
		if err == nil {
			t.Errorf("%s: expected error", name)
			continue
		}
		if strings.Contains(err.Error(), "MIG") || strings.Contains(err.Error(), "BEGIN") {
			t.Errorf("%s: error leaks key material: %v", name, err)
		}
	}
}

func TestFingerprintStableAndPublic(t *testing.T) {
	k, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	other, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	fp := Fingerprint(k)
	if !strings.HasPrefix(fp, "SHA256:") || len(fp) != len("SHA256:")+43 {
		t.Fatalf("fingerprint = %q", fp)
	}
	if Fingerprint(k) != fp || Fingerprint(other) == fp {
		t.Fatal("fingerprint must be stable per key and differ between keys")
	}
}

func TestClientSecret(t *testing.T) {
	key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	sk := SigningKey{TeamID: "TEAM000001", KeyID: "KEY0000001", Key: key}
	now := time.Now()
	secret, exp, err := sk.ClientSecret("com.example.web", 24*time.Hour, now)
	if err != nil {
		t.Fatal(err)
	}
	if got := exp.Sub(now.UTC().Truncate(time.Second)); got != 24*time.Hour {
		t.Fatalf("expiry offset = %s", got)
	}
	claims := jwt.MapClaims{}
	tok, err := jwt.ParseWithClaims(secret, claims, func(*jwt.Token) (any, error) { return &key.PublicKey, nil },
		jwt.WithValidMethods([]string{"ES256"}))
	if err != nil || !tok.Valid {
		t.Fatalf("secret does not verify: %v", err)
	}
	if tok.Header["kid"] != "KEY0000001" || claims["iss"] != "TEAM000001" ||
		claims["sub"] != "com.example.web" || claims["aud"] != Audience {
		t.Fatalf("unexpected header/claims: %v %v", tok.Header, claims)
	}

	if _, _, err := sk.ClientSecret("x", MaxClientSecretTTL+time.Hour, now); err == nil {
		t.Fatal("expected error above Apple's 6-month cap")
	}
	if _, _, err := (SigningKey{}).ClientSecret("x", time.Hour, now); err == nil {
		t.Fatal("expected error without a key")
	}
	if _, _, err := sk.ClientSecret(" ", time.Hour, now); err == nil {
		t.Fatal("expected error without a client id")
	}
}

func TestValidID(t *testing.T) {
	for _, ok := range []string{"ABCDE12345", "0000000000"} {
		if !ValidID(ok) {
			t.Errorf("%q should be valid", ok)
		}
	}
	for _, bad := range []string{"", "abcde12345", "ABC", "ABCDE123456", "ABCDE-1234"} {
		if ValidID(bad) {
			t.Errorf("%q should be invalid", bad)
		}
	}
}
