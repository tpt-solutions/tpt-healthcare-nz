package pharmac

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	// scheduleSearchCacheTTL is the Redis TTL for PHARMAC medicine search results.
	// The PHARMAC schedule is updated monthly; 24h TTL balances freshness and API load.
	scheduleSearchCacheTTL = 24 * time.Hour

	// medicineCacheTTL is the Redis TTL for individual medicine lookups.
	medicineCacheTTL = 24 * time.Hour

	// interactionCacheTTL is the TTL for drug interaction check results.
	interactionCacheTTL = 24 * time.Hour

	scheduleCacheKeyPrefix    = "pharmac:schedule:"
	medicineCacheKeyPrefix    = "pharmac:medicine:"
	interactionCacheKeyPrefix = "pharmac:interaction:"
)

// UseCache configures the PHARMAC client to cache schedule lookups and
// interaction checks in Redis. rdb must not be nil. Call immediately after New().
func (c *Client) UseCache(rdb *redis.Client) {
	c.rdb = rdb
}

// cacheGet attempts to retrieve a cached value by key, unmarshalling into dest.
// Returns true when a valid cached value was found.
func (c *Client) cacheGet(ctx context.Context, key string, dest any) bool {
	if c.rdb == nil {
		return false
	}
	data, err := c.rdb.Get(ctx, key).Bytes()
	if err != nil {
		return false
	}
	return json.Unmarshal(data, dest) == nil
}

// cacheSet writes value to Redis with the given TTL. Errors are silently ignored
// to keep the cache non-blocking.
func (c *Client) cacheSet(ctx context.Context, key string, value any, ttl time.Duration) {
	if c.rdb == nil {
		return
	}
	data, err := json.Marshal(value)
	if err != nil {
		return
	}
	_ = c.rdb.Set(ctx, key, data, ttl).Err()
}

// cacheKeyForSearch builds the Redis key for a medicine search query.
func cacheKeyForSearch(query string) string {
	return fmt.Sprintf("%s%s", scheduleCacheKeyPrefix, query)
}

// cacheKeyForMedicine builds the Redis key for a single medicine lookup.
func cacheKeyForMedicine(nzulm string) string {
	return fmt.Sprintf("%s%s", medicineCacheKeyPrefix, nzulm)
}

// cacheKeyForInteraction builds the Redis key for an interaction check.
func cacheKeyForInteraction(nzulm1, nzulm2 string) string {
	// Normalise key order so (A,B) and (B,A) hit the same cache entry.
	if nzulm1 > nzulm2 {
		nzulm1, nzulm2 = nzulm2, nzulm1
	}
	return fmt.Sprintf("%s%s+%s", interactionCacheKeyPrefix, nzulm1, nzulm2)
}
