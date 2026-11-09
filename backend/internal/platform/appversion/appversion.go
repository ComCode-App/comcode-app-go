// Package appversion holds the mobile client version gate (TEC-1178, docs/design/mobile.md §2.1).
//
// The mobile app reaches the API through the Next.js mobile gateway (/api/mobile/v1/*), which
// forwards its X-Client / X-App-Version / X-Platform headers. The headers are used for display
// and for this gate only, never for a security decision: a request from the mobile client whose
// version is below the configured minimum (platform settings, `mobile.min_version` in
// GET /v1/app/config) is answered 426 APP_UPDATE_REQUIRED so the app can send the user to the
// store. Web requests (the cookie BFF never forwards X-Client) are not affected.
package appversion

import (
	"context"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/pkg/response"
)

// Request headers sent by the mobile app and forwarded by the mobile gateway.
const (
	HeaderClient   = "X-Client"
	HeaderVersion  = "X-App-Version"
	HeaderPlatform = "X-Platform"
)

// ClientMobile is the X-Client value the ComCode mobile app sends.
const ClientMobile = "comcode-mobile"

// ConfigPath stays reachable for outdated apps: it carries the store URLs the update screen opens.
const ConfigPath = "/v1/app/config"

var versionPattern = regexp.MustCompile(`^v?(\d{1,6})(?:\.(\d{1,6}))?(?:\.(\d{1,6}))?(?:[-+][0-9A-Za-z.\-+]{0,64})?$`)

// Version is a parsed major.minor.patch triple. Pre-release / build suffixes are ignored.
type Version [3]int

// Parse reads "1", "1.2", "1.2.3", an optional leading "v" and an optional "-beta.1" / "+42"
// suffix. Missing parts are 0.
func Parse(s string) (Version, bool) {
	m := versionPattern.FindStringSubmatch(strings.TrimSpace(s))
	if m == nil {
		return Version{}, false
	}
	var v Version
	for i := 0; i < 3; i++ {
		if m[i+1] == "" {
			continue
		}
		n, err := strconv.Atoi(m[i+1])
		if err != nil {
			return Version{}, false
		}
		v[i] = n
	}
	return v, true
}

// Valid reports whether s is a version the settings form accepts (strict: digits and dots only).
func Valid(s string) bool {
	return strictPattern.MatchString(s)
}

var strictPattern = regexp.MustCompile(`^\d{1,6}(\.\d{1,6}){0,2}$`)

// Less reports a < b.
func (a Version) Less(b Version) bool {
	for i := 0; i < 3; i++ {
		if a[i] != b[i] {
			return a[i] < b[i]
		}
	}
	return false
}

// IsMobileClient reports whether the request comes from the mobile app ("comcode-mobile"; the
// short "mobile" is accepted too).
func IsMobileClient(r *http.Request) bool {
	switch strings.ToLower(strings.TrimSpace(r.Header.Get(HeaderClient))) {
	case ClientMobile, "mobile":
		return true
	}
	return false
}

// MinVersionFunc returns the configured minimum mobile version ("" = no gate).
type MinVersionFunc func(ctx context.Context) (string, error)

// UpdateRequired reports whether a mobile request must be refused for minVersion. Requests that
// are not from the mobile client, carry no / an unparsable version, or meet the minimum pass.
func UpdateRequired(r *http.Request, minVersion string) bool {
	if minVersion == "" || !IsMobileClient(r) {
		return false
	}
	minV, ok := Parse(minVersion)
	if !ok {
		return false
	}
	got, ok := Parse(r.Header.Get(HeaderVersion))
	if !ok {
		return false
	}
	return got.Less(minV)
}

// Gate refuses /v1 requests from mobile apps older than the minimum version with
// 426 APP_UPDATE_REQUIRED (details: min_version). GET /v1/app/config and non-/v1 paths
// (health checks, docs) always pass. A settings read error fails open: the gate is a product
// nudge, not a security control.
func Gate(minVersion MinVersionFunc) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !IsMobileClient(r) || !strings.HasPrefix(r.URL.Path, "/v1/") || r.URL.Path == ConfigPath {
				next.ServeHTTP(w, r)
				return
			}
			minV, err := minVersion(r.Context())
			if err != nil || !UpdateRequired(r, minV) {
				next.ServeHTTP(w, r)
				return
			}
			response.ErrorWithDetails(w, r, http.StatusUpgradeRequired, response.CodeAppUpdateRequired,
				"app update required", []response.Detail{{Field: "min_version", Message: minV}})
		})
	}
}

// Cached wraps load with a short in-process cache so the gate does not read the database on
// every mobile request. A settings change takes effect within ttl.
func Cached(load MinVersionFunc, ttl time.Duration, now func() time.Time) MinVersionFunc {
	if now == nil {
		now = time.Now
	}
	var (
		mu      sync.Mutex
		value   string
		expires time.Time
	)
	return func(ctx context.Context) (string, error) {
		mu.Lock()
		defer mu.Unlock()
		if t := now(); t.Before(expires) {
			return value, nil
		}
		v, err := load(ctx)
		if err != nil {
			return "", err
		}
		value, expires = v, now().Add(ttl)
		return value, nil
	}
}
