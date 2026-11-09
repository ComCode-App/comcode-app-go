package routecache

import (
	"context"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

func TestTagsAndInvalidation(t *testing.T) {
	mr := miniredis.RunT(t)
	c := New(redis.NewClient(&redis.Options{Addr: mr.Addr()}), "prod", 0)
	ctx := context.Background()
	card, profile := uuid.New(), uuid.New()

	c.Set(ctx, "c/abc", []byte(`1`), &card, &profile)
	c.Set(ctx, "jane", []byte(`2`), nil, &profile)
	c.Set(ctx, "s/other", []byte(`3`), nil, nil)

	if b, ok := c.Get(ctx, "C/ABC "); !ok || string(b) != "1" {
		t.Fatalf("get is case/space insensitive: %q %v", b, ok)
	}
	if ttl := mr.TTL("app:prod:route:path:c/abc"); ttl != DefaultTTL {
		t.Fatalf("ttl = %v", ttl)
	}

	c.InvalidateProfile(ctx, profile)
	for _, p := range []string{"c/abc", "jane"} {
		if _, ok := c.Get(ctx, p); ok {
			t.Fatalf("%s should be gone after profile invalidation", p)
		}
	}
	if _, ok := c.Get(ctx, "s/other"); !ok {
		t.Fatal("untagged entry must survive")
	}
	c.InvalidatePaths(ctx, "s/other", "")
	if _, ok := c.Get(ctx, "s/other"); ok {
		t.Fatal("path invalidation failed")
	}
}

func TestNilCacheIsNoop(t *testing.T) {
	var c *Cache
	ctx := context.Background()
	c.Set(ctx, "a", []byte("x"), nil, nil)
	if _, ok := c.Get(ctx, "a"); ok {
		t.Fatal("nil cache must miss")
	}
	c.InvalidatePaths(ctx, "a")
	c.InvalidateCard(ctx, uuid.New())
	c.InvalidateProfile(ctx, uuid.New())
}

func TestRedisDownFallsThrough(t *testing.T) {
	mr := miniredis.RunT(t)
	c := New(redis.NewClient(&redis.Options{Addr: mr.Addr()}), "prod", 0)
	mr.Close()
	ctx := context.Background()
	c.Set(ctx, "a", []byte("x"), nil, nil)
	if _, ok := c.Get(ctx, "a"); ok {
		t.Fatal("unreachable redis must miss, not fail")
	}
}
