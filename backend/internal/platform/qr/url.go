// Package qr renders account-owned branded route QR codes.
package qr

import (
	"net/url"
	"strings"
)

// IdentityURL is the canonical public identity shared by NFC and QR payloads.
func IdentityURL(base, path string) string {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	for i := range parts {
		parts[i] = url.PathEscape(parts[i])
	}
	return strings.TrimRight(base, "/") + "/" + strings.Join(parts, "/")
}

// PayloadURL adds stable scan attribution to the public identity, never an API path.
func PayloadURL(base, path string) string {
	return IdentityURL(base, path) + "?src=qr"
}

// CardIdentityURL preserves the existing NFC URL and escapes a code as one segment.
func CardIdentityURL(base, code string) string {
	return strings.TrimRight(base, "/") + "/c/" + url.PathEscape(code)
}

// CardPayloadURL is the production print payload, shared by exports and QR packs.
func CardPayloadURL(base, code string) string {
	return CardIdentityURL(base, code) + "?src=qr"
}
