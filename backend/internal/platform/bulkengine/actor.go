package bulkengine

import "context"

type actorKey struct{}

// WithActor records the id of the user who started (or rolls back) a bulk job on the context
// of every adapter call, so platform adapters can re-check the actor's rights per item. The
// bulk service sets it for inline runs, queued runs (from the persisted job) and rollbacks.
func WithActor(ctx context.Context, actorID int64) context.Context {
	return context.WithValue(ctx, actorKey{}, actorID)
}

// ActorFrom returns the actor recorded by WithActor.
func ActorFrom(ctx context.Context) (int64, bool) {
	id, ok := ctx.Value(actorKey{}).(int64)
	return id, ok && id > 0
}
