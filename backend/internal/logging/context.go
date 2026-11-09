package logging

import (
	"context"
	"log/slog"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/reqid"
)

// RequestIDFromContext extracts a correlation id from context.
var RequestIDFromContext = reqid.From

// contextHandler adds request_id from the record's context to every record
// logged via the *Context slog methods (InfoContext, ErrorContext, ...).
type contextHandler struct {
	slog.Handler
}

func (h contextHandler) Handle(ctx context.Context, rec slog.Record) error {
	if id := reqid.From(ctx); id != "" {
		rec = rec.Clone()
		rec.AddAttrs(slog.String("request_id", id))
	}
	return h.Handler.Handle(ctx, rec)
}

func (h contextHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return contextHandler{h.Handler.WithAttrs(attrs)}
}

func (h contextHandler) WithGroup(name string) slog.Handler {
	return contextHandler{h.Handler.WithGroup(name)}
}
