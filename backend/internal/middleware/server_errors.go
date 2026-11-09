package middleware

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"runtime/debug"
	"strconv"
	"time"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/errreport"
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

// ServerErrors logs 5xx responses with request metadata (and error when recorded),
// reports them to the optional error tracker, and records HTTP metrics.
// Mount it inside RequestID so the correlation id is already in the context.
func ServerErrors(log *slog.Logger) func(http.Handler) http.Handler {
	if log == nil {
		log = slog.Default()
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			started := time.Now()
			rec := &statusWriter{ResponseWriter: w, status: http.StatusOK}
			next.ServeHTTP(rec, r)
			status := rec.StatusCode()
			metrics.ObserveHTTP(r.Method, status, time.Since(started))
			if status < http.StatusInternalServerError {
				return
			}
			ctx := r.Context()
			attrs := []any{
				"status", status,
				"method", r.Method,
				"path", r.URL.Path,
			}
			tags := map[string]string{
				"http.method": r.Method,
				"http.path":   r.URL.Path,
				"http.status": strconv.Itoa(status),
			}
			if msg, ok, err := rec.ErrorRecorded(); ok {
				attrs = append(attrs, "error", err.Error(), "message", msg)
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
				errreport.CaptureMessage(ctx, fmt.Sprintf("HTTP %d %s %s", status, r.Method, r.URL.Path), tags)
			}
			log.ErrorContext(ctx, "http_server_error", attrs...)
		})
	}
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
				"http.path":   r.URL.Path,
			})
			response.InternalErr(w, r, &PanicError{Value: v, Stack: string(debug.Stack())}, "Internal server error")
		}()
		next.ServeHTTP(w, r)
	})
}
