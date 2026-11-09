// Package reqid carries the request correlation id (X-Request-ID) through
// contexts: HTTP requests, slog records, and Asynq tasks enqueued while serving them.
package reqid

import (
	"context"

	"github.com/google/uuid"
)

// Header is the HTTP header (and Asynq task header) holding the correlation id.
const Header = "X-Request-ID"

// MaxLen bounds accepted incoming ids so callers cannot bloat logs.
const MaxLen = 128

type ctxKey struct{}

// With returns ctx carrying id. An empty id leaves ctx unchanged.
func With(ctx context.Context, id string) context.Context {
	if id == "" {
		return ctx
	}
	return context.WithValue(ctx, ctxKey{}, id)
}

// From returns the correlation id in ctx, or "".
func From(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if v, ok := ctx.Value(ctxKey{}).(string); ok {
		return v
	}
	return ""
}

// New generates a fresh correlation id.
func New() string { return uuid.NewString() }

// Valid reports whether an incoming id is safe to reuse: 1..MaxLen chars of
// [A-Za-z0-9._:-]. Anything else is replaced by a generated id.
func Valid(id string) bool {
	if id == "" || len(id) > MaxLen {
		return false
	}
	for i := 0; i < len(id); i++ {
		c := id[i]
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9':
		case c == '.', c == '_', c == ':', c == '-':
		default:
			return false
		}
	}
	return true
}

// Resolve returns incoming when valid, otherwise a generated id.
func Resolve(incoming string) string {
	if Valid(incoming) {
		return incoming
	}
	return New()
}
