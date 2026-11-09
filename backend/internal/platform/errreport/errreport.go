// Package errreport sends API 5xx errors, panics, and worker task failures to a
// Sentry-compatible endpoint (Sentry, GlitchTip). It is optional: with no DSN every
// Capture* call returns immediately.
//
// The DSN can change at runtime (TEC-952): Init applies the SENTRY_DSN bootstrap value,
// and SetDSN swaps the client when the platform "Hata izleme" setting changes, without a
// restart. Captures always use the client that is current when they run.
package errreport

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/getsentry/sentry-go"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/reqid"
)

// Config selects the reporting target. Empty DSN disables reporting.
type Config struct {
	DSN         string
	Environment string
	Release     string
	// ServerName distinguishes processes (api / worker) in the issue view.
	ServerName string
}

// flushTimeout bounds the delivery of buffered events at exit or when a client is replaced.
const flushTimeout = 3 * time.Second

type state struct {
	dsn string
	hub *sentry.Hub
}

var (
	mu      sync.Mutex // serializes SetDSN; reads go through cur
	base    Config
	baseLog *slog.Logger
	cur     atomic.Pointer[state]
)

// Init stores the process options and applies cfg.DSN. It returns a flush func that should
// be deferred by main so buffered events are delivered before exit.
func Init(cfg Config, log *slog.Logger) (flush func(), err error) {
	mu.Lock()
	base = cfg
	baseLog = log
	mu.Unlock()
	if err := SetDSN(cfg.DSN); err != nil {
		return Flush, err
	}
	return Flush, nil
}

// ValidateDSN reports whether dsn is a well-formed Sentry DSN. Empty is valid (disabled).
func ValidateDSN(dsn string) error {
	dsn = strings.TrimSpace(dsn)
	if dsn == "" {
		return nil
	}
	if _, err := sentry.NewDsn(dsn); err != nil {
		return fmt.Errorf("errreport: invalid dsn: %w", err)
	}
	return nil
}

// SetDSN switches reporting to dsn. Empty disables it. Setting the current DSN again is a
// no-op. The replaced client is flushed in the background so its queued events still go out.
func SetDSN(dsn string) error {
	dsn = strings.TrimSpace(dsn)
	mu.Lock()
	defer mu.Unlock()

	prev := cur.Load()
	if prev != nil && prev.dsn == dsn {
		return nil
	}
	if prev == nil && dsn == "" {
		return nil
	}

	var next *state
	if dsn != "" {
		client, err := sentry.NewClient(sentry.ClientOptions{
			Dsn:              dsn,
			Environment:      base.Environment,
			Release:          base.Release,
			ServerName:       base.ServerName,
			AttachStacktrace: true,
		})
		if err != nil {
			return fmt.Errorf("errreport: init: %w", err)
		}
		next = &state{dsn: dsn, hub: sentry.NewHub(client, sentry.NewScope())}
	}
	cur.Store(next)

	if prev != nil {
		go prev.hub.Flush(flushTimeout)
	}
	if baseLog != nil {
		if next != nil {
			baseLog.Info("error_reporting_enabled", "environment", base.Environment, "server_name", base.ServerName)
		} else {
			baseLog.Info("error_reporting_disabled", "server_name", base.ServerName)
		}
	}
	return nil
}

// Flush delivers buffered events of the current client.
func Flush() {
	if s := cur.Load(); s != nil {
		s.hub.Flush(flushTimeout)
	}
}

// Enabled reports whether events are being sent.
func Enabled() bool { return cur.Load() != nil }

func hub() *sentry.Hub {
	s := cur.Load()
	if s == nil {
		return nil
	}
	return s.hub.Clone()
}

// CaptureError reports err with request_id (from ctx) and tags.
func CaptureError(ctx context.Context, err error, tags map[string]string) {
	if err == nil {
		return
	}
	h := hub()
	if h == nil {
		return
	}
	h.WithScope(func(scope *sentry.Scope) {
		applyScope(ctx, scope, tags)
		h.CaptureException(err)
	})
}

// CapturePanic reports a recovered panic value. Call it from the deferred recover so
// the attached stack trace still contains the panicking frames.
func CapturePanic(ctx context.Context, recovered any, tags map[string]string) {
	if recovered == nil {
		return
	}
	h := hub()
	if h == nil {
		return
	}
	h.WithScope(func(scope *sentry.Scope) {
		applyScope(ctx, scope, tags)
		h.RecoverWithContext(ctx, recovered)
	})
}

// CaptureMessage reports a message-level event (e.g. a 500 with no recorded error).
func CaptureMessage(ctx context.Context, msg string, tags map[string]string) {
	if msg == "" {
		return
	}
	h := hub()
	if h == nil {
		return
	}
	h.WithScope(func(scope *sentry.Scope) {
		applyScope(ctx, scope, tags)
		scope.SetLevel(sentry.LevelError)
		h.CaptureMessage(msg)
	})
}

// CaptureTest sends one error event for the admin "send test error" button and waits for
// (best effort) for delivery. It returns the event id, or false when reporting is off.
func CaptureTest(ctx context.Context, msg string, tags map[string]string) (string, bool) {
	h := hub()
	if h == nil {
		return "", false
	}
	var id *sentry.EventID
	h.WithScope(func(scope *sentry.Scope) {
		applyScope(ctx, scope, tags)
		scope.SetLevel(sentry.LevelError)
		id = h.CaptureMessage(msg)
	})
	if id == nil {
		return "", false
	}
	h.Flush(flushTimeout)
	return string(*id), true
}

func applyScope(ctx context.Context, scope *sentry.Scope, tags map[string]string) {
	if id := reqid.From(ctx); id != "" {
		scope.SetTag("request_id", id)
	}
	for k, v := range tags {
		if v != "" {
			scope.SetTag(k, v)
		}
	}
}
