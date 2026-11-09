package geoip

import (
	"context"
	"encoding/json"
	"net"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

type cacheEntry struct {
	loc       *GeoLocation
	expiresAt time.Time
}

// MemoryCache is a high-performance in-memory cache with eviction and TTL.
type MemoryCache struct {
	mu         sync.RWMutex
	items      map[string]cacheEntry
	maxEntries int
	defaultTTL time.Duration
}

// NewMemoryCache creates an in-memory GeoIP cache.
func NewMemoryCache(maxEntries int, defaultTTL time.Duration) *MemoryCache {
	if maxEntries <= 0 {
		maxEntries = 100000
	}
	if defaultTTL <= 0 {
		defaultTTL = 24 * time.Hour
	}
	return &MemoryCache{
		items:      make(map[string]cacheEntry, 1024),
		maxEntries: maxEntries,
		defaultTTL: defaultTTL,
	}
}

// Get looks up a cached IP.
func (c *MemoryCache) Get(ip string) (*GeoLocation, bool) {
	c.mu.RLock()
	entry, found := c.items[ip]
	c.mu.RUnlock()

	if !found {
		return nil, false
	}
	if time.Now().After(entry.expiresAt) {
		c.mu.Lock()
		delete(c.items, ip)
		c.mu.Unlock()
		return nil, false
	}
	return entry.loc, true
}

// Set stores a location for an IP with eviction if max capacity is reached.
func (c *MemoryCache) Set(ip string, loc *GeoLocation, ttl time.Duration) {
	if ttl <= 0 {
		ttl = c.defaultTTL
	}
	c.mu.Lock()
	defer c.mu.Unlock()

	if len(c.items) >= c.maxEntries {
		now := time.Now()
		evicted := 0
		target := c.maxEntries / 10
		if target < 1 {
			target = 1
		}
		for k, v := range c.items {
			if now.After(v.expiresAt) || evicted < target {
				delete(c.items, k)
				evicted++
			}
		}
	}

	c.items[ip] = cacheEntry{
		loc:       loc,
		expiresAt: time.Now().Add(ttl),
	}
}

// Clear flushes all entries.
func (c *MemoryCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.items = make(map[string]cacheEntry, 1024)
}

// Size returns total cached entries.
func (c *MemoryCache) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.items)
}

// RedisCache provides distributed cache for GeoIP lookups.
type RedisCache struct {
	rdb        *redis.Client
	prefix     string
	defaultTTL time.Duration
}

// NewRedisCache creates a Redis-backed GeoIP cache.
func NewRedisCache(rdb *redis.Client, prefix string, defaultTTL time.Duration) *RedisCache {
	if prefix == "" {
		prefix = "geoip:cache:"
	}
	if defaultTTL <= 0 {
		defaultTTL = 24 * time.Hour
	}
	return &RedisCache{
		rdb:        rdb,
		prefix:     prefix,
		defaultTTL: defaultTTL,
	}
}

// Get fetches location from Redis.
func (r *RedisCache) Get(ctx context.Context, ip net.IP) (*GeoLocation, error) {
	if r.rdb == nil || ip == nil {
		return nil, nil
	}
	val, err := r.rdb.Get(ctx, r.prefix+ip.String()).Bytes()
	if err != nil {
		if err == redis.Nil {
			return nil, nil
		}
		return nil, err
	}
	var loc GeoLocation
	if err := json.Unmarshal(val, &loc); err != nil {
		return nil, err
	}
	return &loc, nil
}

// Set stores location in Redis.
func (r *RedisCache) Set(ctx context.Context, ip net.IP, loc *GeoLocation, ttl time.Duration) error {
	if r.rdb == nil || ip == nil || loc == nil {
		return nil
	}
	if ttl <= 0 {
		ttl = r.defaultTTL
	}
	data, err := json.Marshal(loc)
	if err != nil {
		return err
	}
	return r.rdb.Set(ctx, r.prefix+ip.String(), data, ttl).Err()
}

// Clear removes all keys under geoip:cache:*.
func (r *RedisCache) Clear(ctx context.Context) error {
	if r.rdb == nil {
		return nil
	}
	iter := r.rdb.Scan(ctx, 0, r.prefix+"*", 500).Iterator()
	for iter.Next(ctx) {
		_ = r.rdb.Del(ctx, iter.Val()).Err()
	}
	return iter.Err()
}
