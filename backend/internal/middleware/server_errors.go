package middleware

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"runtime/debug"
	"strconv"
	"strings"
	"time"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/errreport"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/logredact"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/metrics"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/pkg/response"
)

type statusWriter struct {
	http.ResponseWriter
	status        int
	headerWritten bool
	err           error
	publicMessage string
	recorded      bool
	errorCode     string
	failure       error
	route         string
}

func (w *statusWriter) WriteHeader(status int) {
	if !w.headerWritten {
		w.status = status
		w.headerWritten = true
	}
	w.ResponseWriter.WriteHeader(status)
}

func (w *statusWriter) Write(b []byte) (int, error) {
	if !w.headerWritten {
		w.status = http.StatusOK
		w.headerWritten = true
	}
	return w.ResponseWriter.Write(b)
}

func (w *statusWriter) RecordServerError(err error, publicMessage string) {
	if err == nil {
		return
	}
	w.err = err
	w.publicMessage = publicMessage
	w.recorded = true
}

// RecordErrorCode keeps the envelope error code (first one wins) for the request log.
func (w *statusWriter) RecordErrorCode(code string) {
	if w.errorCode == "" {
		w.errorCode = code
	}
}

// RecordFailure keeps the cause of a non-5xx failure for the request log.
func (w *statusWriter) RecordFailure(err error) {
	if err != nil && w.failure == nil {
		w.failure = err
	}
}

func (w *statusWriter) recordRoute(pattern string) {
	if pattern != "" {
		w.route = pattern
	}
}

func (w *statusWriter) StatusCode() int {
	if w.status == 0 {
		return http.StatusOK
	}
	return w.status
}

func (w *statusWriter) ErrorRecorded() (publicMessage string, ok bool, err error) {
	if !w.recorded {
		return "", false, nil
	}
	return w.publicMessage, true, w.err
}

type routeRecorder interface {
	recordRoute(pattern string)
}

// CaptureRoute wraps the ServeMux so ServerErrors can log the matched route pattern
// ("POST /v1/auth/login") instead of the raw path, which may carry tokens or ids.
// ServeMux sets Request.Pattern on the request it routes; it is read after the handler
// returns (or panics).
func CaptureRoute(mux http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec, ok := w.(routeRecorder); ok {
				rec.recordRoute(r.Pattern)
			}
		}()
		mux.ServeHTTP(w, r)
	})
}

// ServerErrors is the request observer. It records HTTP metrics and:
//   - logs 5xx responses at ERROR and reports them to the optional error tracker;
//   - logs requests the client cancelled at INFO with status 499 (never reported, never
//     counted as 5xx);
//   - writes one INFO "http_request" line for every 4xx and every sign-in / NextAuth
//     adapter request (/v1/auth/*, /v1/internal/auth/*), with the envelope error code and
//     the redacted failure reason.
//
// Logs carry the route pattern, never the raw path, query string or body. Mount it inside
// RequestID so the correlation id is already in the context, and wrap the mux with
// CaptureRoute.
func ServerErrors(log *slog.Logger) func(http.Handler) http.Handler {
	if log == nil {
		log = slog.Default()
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			started := time.Now()
			rec := &statusWriter{ResponseWriter: w, status: http.StatusOK}
			next.ServeHTTP(rec, r)
			observe(log, rec, r, time.Since(started))
		})
	}
}

func routeOf(rec *statusWriter) string {
	if rec.route != "" {
		return rec.route
	}
	return "unmatched"
}

// routePath is the path part of a pattern ("POST /v1/x" -> "/v1/x").
func routePath(route string) string {
	if _, p, ok := strings.Cut(route, " "); ok {
		return p
	}
	return route
}

func isHealthPath(p string) bool {
	return p == "/healthz" || p == "/readyz"
}

// isAuthRoute covers sign-in, session and NextAuth adapter endpoints, which are
// logged on success too.
func isAuthRoute(path string) bool {
	return strings.HasPrefix(path, "/v1/auth/") || strings.HasPrefix(path, "/v1/internal/auth/")
}

func clientGone(r *http.Request) bool {
	return errors.Is(r.Context().Err(), context.Canceled)
}

func observe(log *slog.Logger, rec *statusWriter, r *http.Request, dur time.Duration) {
	ctx := r.Context()
	status := rec.StatusCode()
	route := routeOf(rec)

	// The client went away: not a server fault. Count and log it as 499, never report.
	if status == response.StatusClientClosedRequest || (status >= http.StatusInternalServerError && clientGone(r)) {
		metrics.ObserveHTTP(r.Method, response.StatusClientClosedRequest, dur)
		attrs := []any{
			"status", response.StatusClientClosedRequest,
			"method", r.Method,
			"route", route,
			"duration_ms", dur.Milliseconds(),
		}
		if _, ok, err := rec.ErrorRecorded(); ok {
			attrs = append(attrs, "error", logredact.Error(err))
		} else if rec.failure != nil {
			attrs = append(attrs, "error", logredact.Error(rec.failure))
		}
		log.InfoContext(ctx, "http_request_canceled", attrs...)
		return
	}

	metrics.ObserveHTTP(r.Method, status, dur)

	if status >= http.StatusInternalServerError {
		attrs := []any{
			"status", status,
			"method", r.Method,
			"route", route,
			"duration_ms", dur.Milliseconds(),
		}
		if rec.errorCode != "" {
			attrs = append(attrs, "error_code", rec.errorCode)
		}
		tags := map[string]string{
			"http.method": r.Method,
			"http.route":  route,
			"http.status": strconv.Itoa(status),
		}
		if msg, ok, err := rec.ErrorRecorded(); ok {
			attrs = append(attrs, "error", logredact.Error(err), "message", msg)
			var pe *PanicError
			if errors.As(err, &pe) {
				// Recover already reported it with the panicking stack.
				attrs = append(attrs, "stack", pe.Stack)
			} else {
				errreport.CaptureError(ctx, err, tags)
			}
			log.ErrorContext(ctx, "http_handler_failed", attrs...)
			return
		}
		if status == http.StatusInternalServerError {
			errreport.CaptureMessage(ctx, fmt.Sprintf("HTTP %d %s", status, route), tags)
		}
		log.ErrorContext(ctx, "http_server_error", attrs...)
		return
	}

	if isHealthPath(r.URL.Path) {
		return
	}
	failed := status >= http.StatusBadRequest
	if !failed && !isAuthRoute(routePath(route)) {
		return
	}
	attrs := []any{
		"status", status,
		"method", r.Method,
		"route", route,
		"duration_ms", dur.Milliseconds(),
	}
	if rec.errorCode != "" {
		attrs = append(attrs, "error_code", rec.errorCode)
	}
	if rec.failure != nil {
		attrs = append(attrs, "reason", logredact.Error(rec.failure))
	}
	log.InfoContext(ctx, "http_request", attrs...)
}

// PanicError is recorded by Recover so ServerErrors can log the stack without
// reporting the panic twice.
type PanicError struct {
	Value any
	Stack string
}

func (e *PanicError) Error() string { return fmt.Sprintf("panic: %v", e.Value) }

// Recover turns a handler panic into a 500 envelope, reports it to the error tracker
// with the panicking stack, and records it for ServerErrors to log.
// http.ErrAbortHandler is re-panicked so net/http aborts the response as intended.
func Recover(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			v := recover()
			if v == nil {
				return
			}
			if v == http.ErrAbortHandler { //nolint:errorlint // sentinel comparison as documented by net/http
				panic(v)
			}
			errreport.CapturePanic(r.Context(), v, map[string]string{
				"http.method": r.Method,
				"http.route":  panicRoute(w),
			})
			response.InternalErr(w, r, &PanicError{Value: v, Stack: string(debug.Stack())}, "Internal server error")
		}()
		next.ServeHTTP(w, r)
	})
}

// panicRoute is the route pattern CaptureRoute stored while the panic unwound.
func panicRoute(w http.ResponseWriter) string {
	if rec, ok := w.(*statusWriter); ok {
		return routeOf(rec)
	}
	return "unknown"
}
