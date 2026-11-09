// Package routecache caches public route resolutions in Redis (TEC-557).
//
// Keys (prefix app:<env>:):
//
//	route:path:{normalized_path}   resolved route JSON, TTL
//	route:card:{card_uuid}         set of path keys that embed this card
//	route:profile:{profile_uuid}   set of path keys that embed this profile
//
// Only hits are cached, so creating a route never needs an invalidation. Writes that change a
// route, card or profile call InvalidatePaths / InvalidateCard / InvalidateProfile. A nil *Cache
// (no Redis) is valid and does nothing, and Redis errors never fail a request.
package routecache

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

// DefaultTTL bounds how long an entry lives when an invalidation is missed.
const DefaultTTL = time.Hour

// Redis calls are bounded so a slow or unreachable Redis costs a resolve at most this much
// before it falls back to Postgres.
const (
	readTimeout  = 100 * time.Millisecond
	writeTimeout = 250 * time.Millisecond
)

type Cache struct {
	rdb *redis.Client
	env string
	ttl time.Duration
}

// New returns nil when rdb is nil, which disables caching.
func New(rdb *redis.Client, env string, ttl time.Duration) *Cache {
	if rdb == nil {
		return nil
	}
	if ttl <= 0 {
		ttl = DefaultTTL
	}
	return &Cache{rdb: rdb, env: env, ttl: ttl}
}

func (c *Cache) pathKey(path string) string {
	return fmt.Sprintf("app:%s:route:path:%s", c.env, strings.ToLower(strings.TrimSpace(path)))
}

func (c *Cache) cardKey(id uuid.UUID) string {
	return fmt.Sprintf("app:%s:route:card:%s", c.env, id)
}

func (c *Cache) profileKey(id uuid.UUID) string {
	return fmt.Sprintf("app:%s:route:profile:%s", c.env, id)
}

// Get returns the cached payload for a normalized path.
func (c *Cache) Get(ctx context.Context, path string) ([]byte, bool) {
	if c == nil {
		return nil, false
	}
	ctx, cancel := context.WithTimeout(ctx, readTimeout)
	defer cancel()
	b, err := c.rdb.Get(ctx, c.pathKey(path)).Bytes()
	if err != nil {
		return nil, false
	}
	return b, true
}

// Set stores a payload and tags it with the card and profile it embeds.
func (c *Cache) Set(ctx context.Context, path string, payload []byte, cardUUID, profileUUID *uuid.UUID) {
	if c == nil {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, writeTimeout)
	defer cancel()
	key := c.pathKey(path)
	pipe := c.rdb.TxPipeline()
	pipe.Set(ctx, key, payload, c.ttl)
	for _, tag := range c.tags(cardUUID, profileUUID) {
		pipe.SAdd(ctx, tag, key)
		pipe.Expire(ctx, tag, c.ttl)
	}
	_, _ = pipe.Exec(ctx)
}

func (c *Cache) tags(cardUUID, profileUUID *uuid.UUID) []string {
	var out []string
	if cardUUID != nil {
		out = append(out, c.cardKey(*cardUUID))
	}
	if profileUUID != nil {
		out = append(out, c.profileKey(*profileUUID))
	}
	return out
}

// InvalidatePaths drops the entries of the given normalized paths.
func (c *Cache) InvalidatePaths(ctx context.Context, paths ...string) {
	if c == nil || len(paths) == 0 {
		return
	}
	keys := make([]string, 0, len(paths))
	for _, p := range paths {
		if strings.TrimSpace(p) != "" {
			keys = append(keys, c.pathKey(p))
		}
	}
	if len(keys) > 0 {
		ctx, cancel := context.WithTimeout(ctx, writeTimeout)
		defer cancel()
		_ = c.rdb.Del(ctx, keys...).Err()
	}
}

// InvalidateCard drops every entry that embeds the card (lock, active, passcode, profile...).
func (c *Cache) InvalidateCard(ctx context.Context, cardUUID uuid.UUID) {
	if c == nil {
		return
	}
	c.invalidateTag(ctx, c.cardKey(cardUUID))
}

// InvalidateProfile drops every entry that embeds the profile (username, avatar, locale...).
func (c *Cache) InvalidateProfile(ctx context.Context, profileUUID uuid.UUID) {
	if c == nil {
		return
	}
	c.invalidateTag(ctx, c.profileKey(profileUUID))
}

func (c *Cache) invalidateTag(ctx context.Context, tag string) {
	ctx, cancel := context.WithTimeout(ctx, writeTimeout)
	defer cancel()
	keys, err := c.rdb.SMembers(ctx, tag).Result()
	if err != nil {
		return
	}
	_ = c.rdb.Del(ctx, append(keys, tag)...).Err()
}
