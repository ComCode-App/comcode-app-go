package cache

import (
	"context"
	"fmt"
	"time"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/config"
	"github.com/redis/go-redis/v9"
)

// NewRedisClient creates and pings a Redis client.
func NewRedisClient(ctx context.Context, cfg config.RedisConfig) (*redis.Client, error) {
	client := redis.NewClient(clientOptions(cfg))

	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := client.Ping(pingCtx).Err(); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("cache: redis ping: %w", err)
	}

	return client, nil
}

// Ping checks Redis availability.
func Ping(ctx context.Context, client *redis.Client) error {
	if client == nil {
		return fmt.Errorf("cache: client is nil")
	}
	return client.Ping(ctx).Err()
}

// clientOptions keeps Redis failures cheap on the request path. Redis backs
// caches and rate limits, never the source of truth, so a dead Redis must cost
// a few hundred milliseconds per request at most, not go-redis' defaults
// (3 command retries, 5 dial attempts 100ms apart, 3s reads, 5s dials).
func clientOptions(cfg config.RedisConfig) *redis.Options {
	return &redis.Options{
		Addr:            cfg.Addr,
		Password:        cfg.Password,
		DB:              cfg.DB,
		DialTimeout:     time.Second,
		DialerRetries:   1,
		ReadTimeout:     500 * time.Millisecond,
		WriteTimeout:    500 * time.Millisecond,
		PoolTimeout:     time.Second,
		MaxRetries:      1,
		MinRetryBackoff: 10 * time.Millisecond,
		MaxRetryBackoff: 50 * time.Millisecond,
	}
}
