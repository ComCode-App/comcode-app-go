// Package publicpath owns the rules for first-level public paths (comcode.../{path}): the
// reserved words that belong to the app itself and the format of profile usernames.
package publicpath

import (
	"errors"
	"regexp"
	"strings"
)

// reserved are first path segments used by the API, the Next.js app (frontend/src/app) or the
// infrastructure. Profiles, smart links and card routes may not take them.
var reserved = map[string]struct{}{
	// API / infrastructure
	"api": {}, "v1": {}, "v2": {}, "internal": {}, "system": {}, "admin": {},
	"health": {}, "ready": {}, "healthz": {}, "readyz": {}, "assets": {}, "static": {}, "images": {},
	"_next": {}, "favicon.ico": {}, "robots.txt": {}, "sitemap.xml": {}, "manifest.json": {},
	"apple-icon": {}, "icon": {}, "icon.svg": {}, ".well-known": {},
	// Route prefixes
	"c": {}, "s": {}, "u": {}, "r": {}, "p": {}, "share": {}, "vcard": {}, "download": {},
	// App pages
	"platform": {}, "login": {}, "register": {}, "auth": {}, "logout": {},
	"verify-email": {}, "reset-password": {}, "forgot-password": {},
	"checkout": {}, "cart": {}, "products": {}, "plans": {}, "subscriptions": {},
	"leads": {}, "settings": {}, "billing": {}, "orders": {}, "cards": {}, "profiles": {}, "profile": {},
	"account": {}, "accounts": {}, "company": {}, "webhooks": {}, "background-studio": {}, "bank-accounts": {}, "smart-links": {}, "analytics": {},
	"forbidden": {}, "unauthorized": {}, "help": {}, "support": {}, "pricing": {},
	"about": {}, "terms": {}, "privacy": {}, "kvkk": {}, "contact": {}, "blog": {},
	"comcode": {}, "www": {}, "mail": {}, "root": {}, "null": {}, "undefined": {},
}

// IsReserved reports whether segment (case-insensitive) belongs to the app.
func IsReserved(segment string) bool {
	_, ok := reserved[strings.ToLower(strings.TrimSpace(segment))]
	return ok
}

// legacyBound are reserved words that a printed legacy card already owns (TEC-955). They stay
// reserved for usernames, smart links and card routes, but the legacy sync binds that card's
// own route there, so the printed address opens its profile. The app must never serve them.
var legacyBound = map[string]struct{}{"comcode": {}}

// IsLegacyBound reports whether segment is a reserved word owned by a printed legacy card.
func IsLegacyBound(segment string) bool {
	_, ok := legacyBound[strings.ToLower(strings.TrimSpace(segment))]
	return ok
}

// Username length limits.
const (
	MinUsername = 3
	MaxUsername = 40
)

var usernameRe = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9]|[._-][a-z0-9])*$`)

var (
	ErrUsernameFormat   = errors.New("username must be 3-40 characters: lowercase letters, digits and single . _ - between them")
	ErrUsernameReserved = errors.New("username is reserved")
)

// NormalizeUsername lowercases and trims a username.
func NormalizeUsername(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

// ValidateUsername checks new registrations and renames, never existing-row reads.
// Adding a reserved word does not revoke an existing username.
func ValidateUsername(u string) error {
	if len(u) < MinUsername || len(u) > MaxUsername || !usernameRe.MatchString(u) {
		return ErrUsernameFormat
	}
	if IsReserved(u) {
		return ErrUsernameReserved
	}
	return nil
}

var turkish = strings.NewReplacer(
	"ç", "c", "Ç", "c", "ğ", "g", "Ğ", "g", "ı", "i", "I", "i", "İ", "i",
	"ö", "o", "Ö", "o", "ş", "s", "Ş", "s", "ü", "u", "Ü", "u",
	"â", "a", "Â", "a", "î", "i", "Î", "i", "û", "u", "Û", "u",
)

var nonSlug = regexp.MustCompile(`[^a-z0-9]+`)

// SlugifyName turns display name parts into a username base ("Furkan Meçlis" -> "furkan-meclis").
// The result may be empty or too short; callers fall back to another base.
func SlugifyName(parts ...string) string {
	s := strings.ToLower(turkish.Replace(strings.Join(parts, " ")))
	s = strings.Trim(nonSlug.ReplaceAllString(s, "-"), "-")
	if len(s) > MaxUsername-4 { // leave room for a "-123" suffix
		s = strings.Trim(s[:MaxUsername-4], "-")
	}
	return s
}
