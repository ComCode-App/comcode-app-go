package middleware

import (
	"context"
	"net/http"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/reqid"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/pkg/response"
)

// RequestIDHeader is the correlation id HTTP header.
const RequestIDHeader = reqid.Header

func init() {
	response.RequestIDFunc = GetRequestID
}

// RequestID ensures every request has a correlation id in context and response headers.
// A valid incoming X-Request-ID (set by the Next.js BFF) is reused; anything missing or
// malformed is replaced with a generated UUID. The id flows into slog records logged with
// a request context, the envelope meta.request_id, and Asynq tasks enqueued downstream.
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := reqid.Resolve(r.Header.Get(RequestIDHeader))
		w.Header().Set(RequestIDHeader, id)
		next.ServeHTTP(w, r.WithContext(reqid.With(r.Context(), id)))
	})
}

// GetRequestID extracts the request id from context.
func GetRequestID(ctx context.Context) string {
	return reqid.From(ctx)
}
