// Package appleauth parses Sign in with Apple signing keys (.p8) and builds the
// ES256 client_secret JWT Apple expects from a web client (TEC-1123).
//
// The key itself is admin-managed: uploaded from the platform panel and stored
// encrypted in oauth_provider_settings (see the oauthprovider module). Nothing
// in this package logs or returns key material; errors are fixed strings.
package appleauth

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Audience is the fixed `aud` of an Apple client_secret.
const Audience = "https://appleid.apple.com"

// MaxClientSecretTTL is Apple's cap on a client_secret lifetime (6 months).
const MaxClientSecretTTL = 180 * 24 * time.Hour

// MaxKeyBytes bounds an uploaded .p8 (a real one is ~250 bytes).
const MaxKeyBytes = 16 * 1024

var (
	// ErrInvalidKey is returned for anything that is not a PKCS#8 EC P-256 PEM key.
	ErrInvalidKey = errors.New("appleauth: private key must be an Apple .p8 key (PEM, PKCS#8, EC P-256)")

	idPattern = regexp.MustCompile(`^[A-Z0-9]{10}$`)
)

// ValidID reports whether s looks like an Apple Team ID or Key ID (10 uppercase
// letters or digits).
func ValidID(s string) bool { return idPattern.MatchString(s) }

// ParsePrivateKey parses a Sign in with Apple .p8 key: PEM, PKCS#8, EC P-256.
// Literal "\n" sequences (a key pasted as one line) are accepted.
func ParsePrivateKey(pemText string) (*ecdsa.PrivateKey, error) {
	if len(pemText) > MaxKeyBytes {
		return nil, ErrInvalidKey
	}
	pemText = strings.TrimSpace(strings.ReplaceAll(pemText, `\n`, "\n"))
	block, _ := pem.Decode([]byte(pemText))
	if block == nil || block.Type != "PRIVATE KEY" {
		return nil, ErrInvalidKey
	}
	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, ErrInvalidKey
	}
	ec, ok := parsed.(*ecdsa.PrivateKey)
	if !ok || ec.Curve != elliptic.P256() {
		return nil, ErrInvalidKey
	}
	return ec, nil
}

// Fingerprint identifies a key without revealing it: SHA-256 of the public
// key (PKIX DER), base64 without padding, prefixed "SHA256:" (ssh-keygen style).
func Fingerprint(key *ecdsa.PrivateKey) string {
	der, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(der)
	return "SHA256:" + base64.RawStdEncoding.EncodeToString(sum[:])
}

// SigningKey is a parsed key with the ids Apple needs in the JWT.
type SigningKey struct {
	TeamID string
	KeyID  string
	Key    *ecdsa.PrivateKey
}

// ClientSecret builds an ES256 client_secret JWT for clientID (the Services
// ID) valid for ttl from now, and returns it with its expiry.
func (k SigningKey) ClientSecret(clientID string, ttl time.Duration, now time.Time) (string, time.Time, error) {
	if k.Key == nil || k.TeamID == "" || k.KeyID == "" {
		return "", time.Time{}, errors.New("appleauth: signing key is not configured")
	}
	if strings.TrimSpace(clientID) == "" {
		return "", time.Time{}, errors.New("appleauth: client id is empty")
	}
	if ttl <= 0 || ttl > MaxClientSecretTTL {
		return "", time.Time{}, fmt.Errorf("appleauth: client secret ttl %s out of range", ttl)
	}
	now = now.UTC().Truncate(time.Second)
	exp := now.Add(ttl)
	// aud is a plain string as in Apple's docs (jwt.ClaimStrings would encode an array).
	tok := jwt.NewWithClaims(jwt.SigningMethodES256, jwt.MapClaims{
		"iss": k.TeamID,
		"sub": clientID,
		"aud": Audience,
		"iat": now.Unix(),
		"exp": exp.Unix(),
	})
	tok.Header["kid"] = k.KeyID
	signed, err := tok.SignedString(k.Key)
	if err != nil {
		return "", time.Time{}, errors.New("appleauth: could not sign client secret")
	}
	return signed, exp, nil
}
