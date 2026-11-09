package ratelimit_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/ratelimit"
	"github.com/redis/go-redis/v9"
)

func newTestLimiter(t *testing.T) *ratelimit.Limiter {
	t.Helper()
	mr := miniredis.RunT(t)
	return ratelimit.New(redis.NewClient(&redis.Options{Addr: mr.Addr()}), "test")
}

func TestLoginLimitPerEmailAcrossIPs(t *testing.T) {
	t.Parallel()
	lim := newTestLimiter(t)
	ctx := context.Background()
	for i := 0; i < 30; i++ {
		if ok, _ := lim.AllowLogin(ctx, fmt.Sprintf("10.0.%d.1", i), "Victim@x.io"); !ok {
			t.Fatalf("attempt %d should be allowed", i+1)
		}
	}
	if ok, retry := lim.AllowLogin(ctx, "10.9.9.9", "victim@x.io "); ok || retry <= 0 {
		t.Fatal("expected per-email cap across IPs")
	}
}

func TestLoginLimitPerIPAcrossEmails(t *testing.T) {
	t.Parallel()
	lim := newTestLimiter(t)
	ctx := context.Background()
	for i := 0; i < 50; i++ {
		if ok, _ := lim.AllowLogin(ctx, "1.1.1.1", fmt.Sprintf("u%d@x.io", i)); !ok {
			t.Fatalf("attempt %d should be allowed", i+1)
		}
	}
	if ok, _ := lim.AllowLogin(ctx, "1.1.1.1", "fresh@x.io"); ok {
		t.Fatal("expected per-IP cap across emails")
	}
}

func TestForgotAndResetPerEmail(t *testing.T) {
	t.Parallel()
	lim := newTestLimiter(t)
	ctx := context.Background()
	for i := 0; i < 5; i++ {
		lim.AllowForgotPassword(ctx, fmt.Sprintf("10.0.0.%d", i), "a@x.io")
	}
	if ok, _ := lim.AllowForgotPassword(ctx, "10.0.1.1", "a@x.io"); ok {
		t.Fatal("expected per-email reset mail cap across IPs")
	}
	for i := 0; i < 10; i++ {
		lim.AllowResetPassword(ctx, fmt.Sprintf("10.0.0.%d", i), "a@x.io")
	}
	if ok, _ := lim.AllowResetPassword(ctx, "10.0.1.1", "a@x.io"); ok {
		t.Fatal("expected per-email reset submit cap across IPs")
	}
}

func TestEmailVerificationLimits(t *testing.T) {
	t.Parallel()
	lim := newTestLimiter(t)
	ctx := context.Background()
	for i := 0; i < 5; i++ {
		if ok, _ := lim.AllowEmailVerificationRequest(ctx, "user-a", fmt.Sprintf("10.0.0.%d", i)); !ok {
			t.Fatalf("request %d should be allowed", i+1)
		}
	}
	if ok, _ := lim.AllowEmailVerificationRequest(ctx, "user-a", "10.0.1.1"); ok {
		t.Fatal("expected per-user verification request cap")
	}
	for i := 0; i < 20; i++ {
		lim.AllowEmailVerificationRequest(ctx, fmt.Sprintf("user-%d", i), "9.9.9.9")
	}
	if ok, _ := lim.AllowEmailVerificationRequest(ctx, "user-new", "9.9.9.9"); ok {
		t.Fatal("expected per-IP verification request cap")
	}
	for i := 0; i < 10; i++ {
		lim.AllowEmailVerificationSubmit(ctx, fmt.Sprintf("10.0.0.%d", i), "a@x.io")
	}
	if ok, _ := lim.AllowEmailVerificationSubmit(ctx, "10.0.1.1", "A@x.io"); ok {
		t.Fatal("expected per-email verification submit cap across IPs")
	}
	for i := 0; i < 10; i++ {
		lim.AllowEmailVerificationSubmit(ctx, "8.8.8.8", fmt.Sprintf("u%d@x.io", i))
	}
	if ok, _ := lim.AllowEmailVerificationSubmit(ctx, "8.8.8.8", "fresh@x.io"); ok {
		t.Fatal("expected per-IP verification submit cap")
	}
}

func TestGenericAllowUsesWindow(t *testing.T) {
	t.Parallel()
	mr := miniredis.RunT(t)
	lim := ratelimit.New(redis.NewClient(&redis.Options{Addr: mr.Addr()}), "test")
	ctx := context.Background()
	for i := 0; i < 2; i++ {
		if ok, _ := lim.Allow(ctx, "custom", "s", 2, time.Minute); !ok {
			t.Fatalf("hit %d should be allowed", i+1)
		}
	}
	ok, retry := lim.Allow(ctx, "custom", "s", 2, time.Minute)
	if ok || retry <= 0 || retry > time.Minute {
		t.Fatalf("expected limit with retry within the window, got ok=%v retry=%v", ok, retry)
	}
	mr.FastForward(time.Minute + time.Second)
	if ok, _ := lim.Allow(ctx, "custom", "s", 2, time.Minute); !ok {
		t.Fatal("window should have reset")
	}
}
