package realtime

import (
	"context"
	"fmt"
	"time"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/analytics/domain"
	"github.com/redis/go-redis/v9"
)

// CounterManager updates real-time fast-increment counters in Redis for instant dashboard availability.
type CounterManager struct {
	rdb *redis.Client
	ttl time.Duration
}

// NewCounterManager creates a real-time counter manager.
func NewCounterManager(rdb *redis.Client) *CounterManager {
	return &CounterManager{
		rdb: rdb,
		ttl: 48 * time.Hour, // Retain today's counters for 48 hours
	}
}

// Increment increments today's real-time counter keys for an event.
func (c *CounterManager) Increment(ctx context.Context, event domain.Event) error {
	if c.rdb == nil {
		return nil
	}

	dateStr := event.OccurredAt.UTC().Format("2006-01-02")
	pipe := c.rdb.Pipeline()

	// 1. Total event count for subject & type today
	subjectEventKey := fmt.Sprintf("analytics:%s:%s:%s:%s:total", event.Subject.Type, event.Subject.ID, dateStr, event.Type)
	pipe.Incr(ctx, subjectEventKey)
	pipe.Expire(ctx, subjectEventKey, c.ttl)

	// 2. Global total event count for subject today
	subjectTotalKey := fmt.Sprintf("analytics:%s:%s:%s:total", event.Subject.Type, event.Subject.ID, dateStr)
	pipe.Incr(ctx, subjectTotalKey)
	pipe.Expire(ctx, subjectTotalKey, c.ttl)

	// 3. Unique sessions via HyperLogLog
	if event.SessionID != "" {
		hllKey := fmt.Sprintf("analytics:%s:%s:%s:hll_sessions", event.Subject.Type, event.Subject.ID, dateStr)
		pipe.PFAdd(ctx, hllKey, event.SessionID)
		pipe.Expire(ctx, hllKey, c.ttl)
	}

	// 4. Country breakdown hash
	if event.Context.CountryCode != "" {
		countryKey := fmt.Sprintf("analytics:%s:%s:%s:country", event.Subject.Type, event.Subject.ID, dateStr)
		pipe.HIncrBy(ctx, countryKey, event.Context.CountryCode, 1)
		pipe.Expire(ctx, countryKey, c.ttl)
	}

	// 5. Device breakdown hash
	if event.Context.DeviceType != "" {
		deviceKey := fmt.Sprintf("analytics:%s:%s:%s:device", event.Subject.Type, event.Subject.ID, dateStr)
		pipe.HIncrBy(ctx, deviceKey, event.Context.DeviceType, 1)
		pipe.Expire(ctx, deviceKey, c.ttl)
	}

	// 6. Browser breakdown hash
	if event.Context.Browser != "" {
		browserKey := fmt.Sprintf("analytics:%s:%s:%s:browser", event.Subject.Type, event.Subject.ID, dateStr)
		pipe.HIncrBy(ctx, browserKey, event.Context.Browser, 1)
		pipe.Expire(ctx, browserKey, c.ttl)
	}

	// 7. OS breakdown hash
	if event.Context.OS != "" {
		osKey := fmt.Sprintf("analytics:%s:%s:%s:os", event.Subject.Type, event.Subject.ID, dateStr)
		pipe.HIncrBy(ctx, osKey, event.Context.OS, 1)
		pipe.Expire(ctx, osKey, c.ttl)
	}

	_, err := pipe.Exec(ctx)
	return err
}

// GetTodayTotal returns today's total event count from Redis.
func (c *CounterManager) GetTodayTotal(ctx context.Context, subjectType, subjectID string, eventType *string) (int64, error) {
	if c.rdb == nil {
		return 0, nil
	}
	dateStr := time.Now().UTC().Format("2006-01-02")
	var key string
	if eventType != nil && *eventType != "" {
		key = fmt.Sprintf("analytics:%s:%s:%s:%s:total", subjectType, subjectID, dateStr, *eventType)
	} else {
		key = fmt.Sprintf("analytics:%s:%s:%s:total", subjectType, subjectID, dateStr)
	}

	val, err := c.rdb.Get(ctx, key).Int64()
	if err != nil {
		if err == redis.Nil {
			return 0, nil
		}
		return 0, err
	}
	return val, nil
}

// GetTodayUniqueSessions returns today's estimated unique sessions via HyperLogLog.
func (c *CounterManager) GetTodayUniqueSessions(ctx context.Context, subjectType, subjectID string) (int64, error) {
	if c.rdb == nil {
		return 0, nil
	}
	dateStr := time.Now().UTC().Format("2006-01-02")
	hllKey := fmt.Sprintf("analytics:%s:%s:%s:hll_sessions", subjectType, subjectID, dateStr)
	return c.rdb.PFCount(ctx, hllKey).Result()
}
