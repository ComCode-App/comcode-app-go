// Package release exposes the release provenance baked into the binary (TEC-1094) and records
// it in app_releases when the server boots.
//
// CI (.github/workflows/ci.yml) passes APP_VERSION, APP_COMMIT, APP_BUILT_AT and
// APP_RELEASE_JSON_B64 as Docker build args. backend/Dockerfile turns the first three into
//
//	-ldflags "-X <module>/internal/platform/release.Version=v0.N
//	          -X <module>/internal/platform/release.Commit=<full sha>
//	          -X <module>/internal/platform/release.BuiltAt=<RFC 3339>"
//
// and decodes the last one over release.json (version, git_commit, built_at, notes; the notes
// are generated from `git log --no-merges` since the previous v0.* tag) before `go build`, so it
// is embedded below. The committed release.json is a `{}` placeholder: local and dev builds
// still compile and report the version "dev".
package release

import (
	_ "embed"
	"encoding/json"
	"strings"
	"time"
)

// Set with -ldflags -X. Empty in local builds.
var (
	Version string
	Commit  string
	BuiltAt string
)

//go:embed release.json
var releaseJSON []byte

// DevVersion is reported when nothing was baked in.
const DevVersion = "dev"

// Info is the provenance of the running binary.
type Info struct {
	Version string `json:"version"`
	Commit  string `json:"git_commit"`
	BuiltAt string `json:"built_at"`
	Notes   string `json:"notes"`
}

// Current merges the embedded release.json with the ldflags values (ldflags win for version,
// commit and build time; the notes only exist in release.json). It never fails: a missing or
// malformed file yields what ldflags provide, and the version falls back to "dev".
func Current() Info {
	return merge(releaseJSON, Version, Commit, BuiltAt)
}

func merge(raw []byte, version, commit, builtAt string) Info {
	var r Info
	_ = json.Unmarshal(raw, &r) // fail-soft: the `{}` placeholder or garbage -> zero value
	if v := strings.TrimSpace(version); v != "" {
		r.Version = v
	}
	if c := strings.TrimSpace(commit); c != "" {
		r.Commit = c
	}
	if b := strings.TrimSpace(builtAt); b != "" {
		r.BuiltAt = b
	}
	r.Version = strings.TrimSpace(r.Version)
	r.Commit = strings.TrimSpace(r.Commit)
	if r.Version == "" {
		r.Version = DevVersion
	}
	return r
}

// IsDev reports whether no version was baked into this binary.
func (r Info) IsDev() bool { return r.Version == DevVersion }

// ShortCommit is the first 7 characters of the commit ("" when unknown).
func (r Info) ShortCommit() string {
	if len(r.Commit) > 7 {
		return r.Commit[:7]
	}
	return r.Commit
}

// BuiltAtTime parses BuiltAt (RFC 3339). ok is false when it is empty or invalid.
func (r Info) BuiltAtTime() (t time.Time, ok bool) {
	if r.BuiltAt == "" {
		return time.Time{}, false
	}
	t, err := time.Parse(time.RFC3339, r.BuiltAt)
	if err != nil {
		return time.Time{}, false
	}
	return t, true
}

// SentryRelease is the release name for error reports: the configured SENTRY_RELEASE when set,
// else the baked version (v0.N / sha-<short>), else "" (dev builds keep the SDK default).
func SentryRelease(configured string) string {
	if c := strings.TrimSpace(configured); c != "" {
		return c
	}
	if r := Current(); !r.IsDev() {
		return r.Version
	}
	return ""
}
