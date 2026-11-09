package ratelimit

import (
	"context"
	"strings"
	"time"
)

// Auth anti-enumeration and abuse limits (TEC-1101). All use the 15-minute window.
// The per-email login cap is higher than the per IP + email one so that a single
// attacker cannot lock a person out as cheaply as they could guess their password.
const (
	loginIPLimit     = 50
	loginEmailLimit  = 30
	forgotIPLimit    = 20
	forgotEmailLimit = 5
	resetEmailLimit  = 10
	// Verification code emails a signed-in user can request, and per client IP.
	verifyRequestUserLimit = 5
	verifyRequestIPLimit   = 20
	// Verification code submissions per client IP and per email (codes are 6 digits).
	verifySubmitIPLimit    = 10
	verifySubmitEmailLimit = 10
)

// Allow is a generic fixed-window limit: at most limit hits per window for
// (action, subject). Callers pick a stable action name and a subject such as a
// user id or client IP. Like the other limits it fails open when Redis is unavailable.
func (l *Limiter) Allow(ctx context.Context, action, subject string, limit int, window time.Duration) (bool, time.Duration) {
	if limit <= 0 || window <= 0 {
		return true, 0
	}
	return l.allowWindow(ctx, action, subject, limit, window)
}

// AllowEmailVerificationRequest limits verification code emails per user and per client IP.
func (l *Limiter) AllowEmailVerificationRequest(ctx context.Context, userID, ip string) (bool, time.Duration) {
	return l.allowAll(ctx,
		counter{"verify_request", userID, verifyRequestUserLimit},
		counter{"verify_request_ip", ip, verifyRequestIPLimit},
	)
}

// AllowEmailVerificationSubmit limits verification code submissions per client IP and per email.
func (l *Limiter) AllowEmailVerificationSubmit(ctx context.Context, ip, email string) (bool, time.Duration) {
	return l.allowAll(ctx,
		counter{"verify_submit_ip", ip, verifySubmitIPLimit},
		counter{"verify_submit_email", normEmail(email), verifySubmitEmailLimit},
	)
}

// allowAll counts the attempt against each counter in order and stops at the first one over its limit.
func (l *Limiter) allowAll(ctx context.Context, counters ...counter) (bool, time.Duration) {
	for _, c := range counters {
		if ok, retry := l.allow(ctx, c.action, c.subject, c.limit); !ok {
			return false, retry
		}
	}
	return true, 0
}

func normEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}
