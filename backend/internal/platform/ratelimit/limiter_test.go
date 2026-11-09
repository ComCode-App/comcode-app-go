package ratelimit_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/ratelimit"
	"github.com/redis/go-redis/v9"
)

func TestLoginLimit(t *testing.T) {
	t.Parallel()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	lim := ratelimit.New(rdb, "test")
	ctx := context.Background()

	for i := 0; i < 10; i++ {
		ok, _ := lim.AllowLogin(ctx, "1.1.1.1", "a@example.com")
		if !ok {
			t.Fatalf("attempt %d should be allowed", i+1)
		}
	}
	ok, retry := lim.AllowLogin(ctx, "1.1.1.1", "a@example.com")
	if ok {
		t.Fatal("expected lockout")
	}
	if retry <= 0 {
		t.Fatal("expected retry-after")
	}
	ok, _ = lim.AllowLogin(ctx, "1.1.1.1", "b@example.com")
	if !ok {
		t.Fatal("different email should not share the bucket")
	}
}

func TestNilRedisAllows(t *testing.T) {
	t.Parallel()
	lim := ratelimit.New(nil, "test")
	ok, _ := lim.AllowRegister(context.Background(), "10.0.0.1")
	if !ok {
		t.Fatal("nil redis must fail open")
	}
}

func TestQRPreviewLimitIsPerUser(t *testing.T) {
	t.Parallel()
	mr := miniredis.RunT(t)
	lim := ratelimit.New(redis.NewClient(&redis.Options{Addr: mr.Addr()}), "test")
	ctx := context.Background()
	for i := 0; i < 600; i++ {
		if ok, _ := lim.AllowQRPreview(ctx, "user-a"); !ok {
			t.Fatalf("preview %d should be allowed", i+1)
		}
	}
	if ok, retry := lim.AllowQRPreview(ctx, "user-a"); ok || retry <= 0 {
		t.Fatal("expected preview limit with retry-after")
	}
	if ok, _ := lim.AllowQRPreview(ctx, "user-b"); !ok {
		t.Fatal("another user should not share the bucket")
	}
}

func TestSmartLinkPasscodeCountsFailuresOnly(t *testing.T) {
	t.Parallel()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	lim := ratelimit.New(rdb, "test")
	ctx := context.Background()

	// Checking does not count: a visitor who knows the passcode can open the link any number of times.
	for i := 0; i < 20; i++ {
		if ok, _ := lim.SmartLinkPasscodeAllowed(ctx, "1.1.1.1", "s/promo"); !ok {
			t.Fatalf("check %d should not consume the budget", i+1)
		}
	}
	for i := 0; i < 5; i++ {
		lim.RecordSmartLinkPasscodeFailure(ctx, "1.1.1.1", "s/promo")
	}
	ok, retry := lim.SmartLinkPasscodeAllowed(ctx, "1.1.1.1", "s/promo")
	if ok || retry <= 0 {
		t.Fatalf("5 wrong passcodes should lock the visitor out of the link, got ok=%v retry=%v", ok, retry)
	}
	if ok, _ := lim.SmartLinkPasscodeAllowed(ctx, "1.1.1.1", "s/other"); !ok {
		t.Fatal("another link has its own per-visitor budget")
	}
	if ok, _ := lim.SmartLinkPasscodeAllowed(ctx, "2.2.2.2", "s/promo"); !ok {
		t.Fatal("another visitor is not locked out by 5 failures")
	}

	// Many addresses together hit the per-link cap.
	for i := 0; i < 50; i++ {
		lim.RecordSmartLinkPasscodeFailure(ctx, fmt.Sprintf("10.0.0.%d", i), "s/target")
	}
	if ok, _ := lim.SmartLinkPasscodeAllowed(ctx, "3.3.3.3", "s/target"); ok {
		t.Fatal("expected the per-link cap to stop a distributed guess")
	}
}

func TestInviteLimits(t *testing.T) {
	t.Parallel()
	mr := miniredis.RunT(t)
	lim := ratelimit.New(redis.NewClient(&redis.Options{Addr: mr.Addr()}), "test")
	ctx := context.Background()
	for i := 0; i < 60; i++ {
		if ok, _ := lim.AllowInvitePreview(ctx, "1.1.1.1"); !ok {
			t.Fatalf("preview %d should be allowed", i+1)
		}
	}
	if ok, retry := lim.AllowInvitePreview(ctx, "1.1.1.1"); ok || retry <= 0 {
		t.Fatal("expected preview limit per IP")
	}
	if ok, _ := lim.AllowInvitePreview(ctx, "2.2.2.2"); !ok {
		t.Fatal("another IP has its own preview budget")
	}
	for i := 0; i < 10; i++ {
		if ok, _ := lim.AllowInviteAccept(ctx, "user-a", fmt.Sprintf("10.0.0.%d", i)); !ok {
			t.Fatalf("accept %d should be allowed", i+1)
		}
	}
	if ok, _ := lim.AllowInviteAccept(ctx, "user-a", "10.0.1.1"); ok {
		t.Fatal("expected accept limit per user across IPs")
	}
	for i := 0; i < 30; i++ {
		lim.AllowInviteAccept(ctx, fmt.Sprintf("user-%d", i), "9.9.9.9")
	}
	if ok, _ := lim.AllowInviteAccept(ctx, "user-new", "9.9.9.9"); ok {
		t.Fatal("expected accept limit per IP across users")
	}
}
