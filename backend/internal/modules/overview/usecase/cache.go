package usecase

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// Cache stores serialized payloads by key (the overview, one funnel per period).
// Implementations must never fail a request: a miss or a Redis error both mean "compute it".
type Cache interface {
	Get(ctx context.Context, key string) ([]byte, bool)
	Set(ctx context.Context, key string, payload []byte)
}

// Cache keys. Bump the version when the payload shape changes.
const (
	overviewCacheKey = "v2"
	funnelCacheKey   = "funnel:v1:%d"
)

// Redis calls are bounded so a slow or unreachable Redis costs the request at most this much
// before it falls back to Postgres (TEC-651).
const (
	cacheReadTimeout  = 100 * time.Millisecond
	cacheWriteTimeout = 250 * time.Millisecond
)

// RedisCache keeps payloads under app:<env>:platform:overview:<key> for ttl.
type RedisCache struct {
	rdb    *redis.Client
	prefix string
	ttl    time.Duration
}

// NewRedisCache returns nil when rdb is nil, which disables caching.
func NewRedisCache(rdb *redis.Client, env string, ttl time.Duration) *RedisCache {
	if rdb == nil {
		return nil
	}
	return &RedisCache{rdb: rdb, prefix: fmt.Sprintf("app:%s:platform:overview:", env), ttl: ttl}
}

// Get returns the cached payload; a nil cache always misses.
func (c *RedisCache) Get(ctx context.Context, key string) ([]byte, bool) {
	if c == nil {
		return nil, false
	}
	ctx, cancel := context.WithTimeout(ctx, cacheReadTimeout)
	defer cancel()
	b, err := c.rdb.Get(ctx, c.prefix+key).Bytes()
	if err != nil {
		return nil, false
	}
	return b, true
}

// Set stores the payload; errors are ignored.
func (c *RedisCache) Set(ctx context.Context, key string, payload []byte) {
	if c == nil {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, cacheWriteTimeout)
	defer cancel()
	_ = c.rdb.Set(ctx, c.prefix+key, payload, c.ttl).Err()
}
