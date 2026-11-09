package cache

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/config"
	"github.com/redis/go-redis/v9"
)

// A request that touches a dead Redis must fail fast instead of retrying with backoff.
func TestDeadRedisFailsFast(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	_ = ln.Close() // nothing listens here any more: connections are refused

	client := redis.NewClient(clientOptions(config.RedisConfig{Addr: addr}))
	defer func() { _ = client.Close() }()

	start := time.Now()
	for range 3 {
		if err := client.Get(context.Background(), "k").Err(); err == nil {
			t.Fatal("expected an error from a dead Redis")
		}
	}
	if elapsed := time.Since(start); elapsed > 500*time.Millisecond {
		t.Fatalf("3 calls to a dead Redis took %s, want < 500ms", elapsed)
	}
}
