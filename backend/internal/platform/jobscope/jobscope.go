// Package jobscope carries the account scope of an app (non-platform) engine job.
//
// Bulk and import jobs started from app routes act inside the caller's active account. The
// engines store the account on the job row and put a Scope on the context of every adapter call
// (target resolution, apply, revert), including calls made later by the worker. Account-scoped
// adapters must read it with From and fail closed when it is missing.
package jobscope

import (
	"context"
	"encoding/json"
)

type ctxKey struct{}

// Scope is the account an app job acts in, the user acting (the job's actor, or the user rolling
// it back) and the action parameters the job was started with.
type Scope struct {
	AccountID int64
	ActorID   int64
	Params    json.RawMessage
}

// With returns ctx carrying s.
func With(ctx context.Context, s Scope) context.Context {
	return context.WithValue(ctx, ctxKey{}, s)
}

// From returns the scope on ctx. ok is false when there is none or it names no account.
func From(ctx context.Context) (Scope, bool) {
	s, ok := ctx.Value(ctxKey{}).(Scope)
	if !ok || s.AccountID <= 0 || s.ActorID <= 0 {
		return Scope{}, false
	}
	return s, true
}
