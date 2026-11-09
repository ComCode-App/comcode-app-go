package ratelimit

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	defaultWindow = 15 * time.Minute
	loginLimit    = 10
	registerLimit = 5
	forgotLimit   = 5
	resetLimit    = 10
	// Card activation codes are bearer secrets: cap guesses per user and per client IP,
	// so neither many accounts from one address nor one account from many addresses can brute-force.
	activateUserLimit = 10
	activateIPLimit   = 30
	// Public analytics events (POST /v1/events) per client IP: a visitor page sends a handful;
	// this only stops scripted inflation of profile analytics.
	eventsIPLimit = 600
	// Lead form submissions (POST /v1/public/leads) per client IP: a person leaves a lead or two;
	// this stops scripted flooding of an account's lead inbox.
	leadsIPLimit = 10
	// Server QR previews (POST /v1/routes/{uuid}/qr/preview) per signed-in user: the style editor
	// calls it debounced while settings change, so this is generous; it only stops render loops.
	qrPreviewUserLimit = 600

	// Wrong smart link passcodes (TEC-556): per visitor and link, per visitor, and per link so
	// many addresses cannot brute-force a short PIN together.
	passcodeIPPathFailures = 5
	passcodeIPFailures     = 20
	passcodePathFailures   = 50
	// Invite links (TEC-1104): the token is a bearer secret. A person opens their link a few
	// times; the limits only stop scripted guessing.
	invitePreviewIPLimit  = 60
	inviteAcceptUserLimit = 10
	inviteAcceptIPLimit   = 30
)

// Limiter is a Redis INCR + TTL rate limiter. Redis errors fail open.
type Limiter struct {
	rdb *redis.Client
	env string
}

// New creates a limiter. rdb may be nil (all requests allowed).
func New(rdb *redis.Client, appEnv string) *Limiter {
	env := strings.TrimSpace(appEnv)
	if env == "" {
		env = "dev"
	}
	return &Limiter{rdb: rdb, env: env}
}

// AllowLogin limits password login attempts per IP + email, per IP and per email (TEC-1101).
func (l *Limiter) AllowLogin(ctx context.Context, ip, email string) (bool, time.Duration) {
	email = normEmail(email)
	return l.allowAll(ctx,
		counter{"login", ip + "|" + email, loginLimit},
		counter{"login_ip", ip, loginIPLimit},
		counter{"login_email", email, loginEmailLimit},
	)
}

// AllowRegister limits self-registration per IP.
func (l *Limiter) AllowRegister(ctx context.Context, ip string) (bool, time.Duration) {
	return l.allow(ctx, "register", ip, registerLimit)
}

// AllowForgotPassword limits reset emails per IP + email, per IP and per email (TEC-1101).
func (l *Limiter) AllowForgotPassword(ctx context.Context, ip, email string) (bool, time.Duration) {
	email = normEmail(email)
	return l.allowAll(ctx,
		counter{"forgot", ip + "|" + email, forgotLimit},
		counter{"forgot_ip", ip, forgotIPLimit},
		counter{"forgot_email", email, forgotEmailLimit},
	)
}

// AllowResetPassword limits reset submissions per IP and per email (TEC-1101).
func (l *Limiter) AllowResetPassword(ctx context.Context, ip, email string) (bool, time.Duration) {
	return l.allowAll(ctx,
		counter{"reset", ip, resetLimit},
		counter{"reset_email", normEmail(email), resetEmailLimit},
	)
}

// AllowCardActivation limits card activation attempts per user and per client IP.
func (l *Limiter) AllowCardActivation(ctx context.Context, userID, ip string) (bool, time.Duration) {
	if ok, retry := l.allow(ctx, "activate", userID, activateUserLimit); !ok {
		return false, retry
	}
	return l.allow(ctx, "activate_ip", ip, activateIPLimit)
}

// AllowEvent limits public analytics events per client IP.
func (l *Limiter) AllowEvent(ctx context.Context, ip string) (bool, time.Duration) {
	return l.allow(ctx, "events", ip, eventsIPLimit)
}

// AllowLeadSubmit limits public lead form submissions per client IP.
func (l *Limiter) AllowLeadSubmit(ctx context.Context, ip string) (bool, time.Duration) {
	return l.allow(ctx, "leads", ip, leadsIPLimit)
}

// AllowQRPreview limits server-rendered QR previews per user.
func (l *Limiter) AllowQRPreview(ctx context.Context, userID string) (bool, time.Duration) {
	return l.allow(ctx, "qr_preview", userID, qrPreviewUserLimit)
}

// AllowInvitePreview limits public invite lookups (GET /v1/invites/{token}) per client IP.
func (l *Limiter) AllowInvitePreview(ctx context.Context, ip string) (bool, time.Duration) {
	return l.allow(ctx, "invite_preview", ip, invitePreviewIPLimit)
}

// AllowInviteAccept limits invite accepts per user and per client IP.
func (l *Limiter) AllowInviteAccept(ctx context.Context, userID, ip string) (bool, time.Duration) {
	if ok, retry := l.allow(ctx, "invite_accept", userID, inviteAcceptUserLimit); !ok {
		return false, retry
	}
	return l.allow(ctx, "invite_accept_ip", ip, inviteAcceptIPLimit)
}

// SmartLinkPasscodeAllowed reports whether a visitor may still try a passcode on a smart link.
// It does not count the attempt; RecordSmartLinkPasscodeFailure does, for wrong ones only.
func (l *Limiter) SmartLinkPasscodeAllowed(ctx context.Context, ip, path string) (bool, time.Duration) {
	for _, c := range passcodeCounters(ip, path) {
		if ok, retry := l.below(ctx, c.action, c.subject, c.limit); !ok {
			return false, retry
		}
	}
	return true, 0
}

// RecordSmartLinkPasscodeFailure counts a wrong passcode against the visitor and the link.
func (l *Limiter) RecordSmartLinkPasscodeFailure(ctx context.Context, ip, path string) {
	for _, c := range passcodeCounters(ip, path) {
		l.allow(ctx, c.action, c.subject, c.limit)
	}
}

type counter struct {
	action, subject string
	limit           int
}

func passcodeCounters(ip, path string) []counter {
	out := []counter{{"passcode_link", path, passcodePathFailures}}
	if ip != "" {
		out = append(out, counter{"passcode_ip_link", ip + "|" + path, passcodeIPPathFailures}, counter{"passcode_ip", ip, passcodeIPFailures})
	}
	return out
}

// below reports whether a counter is still under its limit without incrementing it.
func (l *Limiter) below(ctx context.Context, action, subject string, limit int) (bool, time.Duration) {
	if l == nil || l.rdb == nil || strings.TrimSpace(subject) == "" {
		return true, 0
	}
	key := fmt.Sprintf("app:%s:rl:%s:%s", l.env, action, subject)
	n, err := l.rdb.Get(ctx, key).Int64()
	if err != nil || n < int64(limit) {
		return true, 0
	}
	ttl, ttlErr := l.rdb.TTL(ctx, key).Result()
	if ttlErr != nil || ttl < 0 {
		ttl = defaultWindow
	}
	return false, ttl
}

func (l *Limiter) allow(ctx context.Context, action, subject string, limit int) (bool, time.Duration) {
	return l.allowWindow(ctx, action, subject, limit, defaultWindow)
}

func (l *Limiter) allowWindow(ctx context.Context, action, subject string, limit int, window time.Duration) (bool, time.Duration) {
	if l == nil || l.rdb == nil || strings.TrimSpace(subject) == "" || strings.TrimSpace(subject) == "|" {
		return true, 0
	}
	key := fmt.Sprintf("app:%s:rl:%s:%s", l.env, action, subject)
	n, err := l.rdb.Incr(ctx, key).Result()
	if err != nil {
		return true, 0
	}
	if n == 1 {
		_ = l.rdb.Expire(ctx, key, window).Err()
	}
	if n > int64(limit) {
		ttl, ttlErr := l.rdb.TTL(ctx, key).Result()
		if ttlErr != nil || ttl < 0 {
			ttl = window
		}
		return false, ttl
	}
	return true, 0
}
