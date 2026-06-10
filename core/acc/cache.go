package acc

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	// claimStatusCacheTTL is the Redis TTL for ACC claim status responses.
	// Claim status changes infrequently once lodged; 1h is a reasonable balance.
	claimStatusCacheTTL = 1 * time.Hour

	// poCacheTTL is the Redis TTL for purchase order status responses.
	poCacheTTL = 1 * time.Hour

	claimStatusCacheKeyPrefix = "acc:claim:status:"
	poCacheKeyPrefix          = "acc:po:status:"
)

// UseCache configures the ACC client to cache claim and PO status responses
// in Redis. rdb must not be nil. Call immediately after New().
func (c *Client) UseCache(rdb *redis.Client) {
	c.rdb = rdb
}

// cacheGetClaim returns a cached Claim by claim number.
// Returns (nil, false) if not in cache.
func (c *Client) cacheGetClaim(ctx context.Context, claimNumber string) (*Claim, bool) {
	if c.rdb == nil {
		return nil, false
	}
	key := claimStatusCacheKeyPrefix + claimNumber
	data, err := c.rdb.Get(ctx, key).Bytes()
	if err != nil {
		return nil, false
	}
	var claim Claim
	if json.Unmarshal(data, &claim) != nil {
		return nil, false
	}
	return &claim, true
}

// cacheSetClaim writes a Claim to Redis.
func (c *Client) cacheSetClaim(ctx context.Context, claimNumber string, claim *Claim) {
	if c.rdb == nil {
		return
	}
	data, err := json.Marshal(claim)
	if err != nil {
		return
	}
	key := fmt.Sprintf("%s%s", claimStatusCacheKeyPrefix, claimNumber)
	_ = c.rdb.Set(ctx, key, data, claimStatusCacheTTL).Err()
}

// cacheGetPO returns a cached PurchaseOrder by PO number.
func (c *Client) cacheGetPO(ctx context.Context, poNumber string) (*PurchaseOrder, bool) {
	if c.rdb == nil {
		return nil, false
	}
	key := poCacheKeyPrefix + poNumber
	data, err := c.rdb.Get(ctx, key).Bytes()
	if err != nil {
		return nil, false
	}
	var po PurchaseOrder
	if json.Unmarshal(data, &po) != nil {
		return nil, false
	}
	return &po, true
}

// cacheSetPO writes a PurchaseOrder to Redis. Terminal-state POs are cached
// indefinitely; pending/approved POs use the standard 1h TTL.
func (c *Client) cacheSetPO(ctx context.Context, po *PurchaseOrder) {
	if c.rdb == nil || po.PONumber == "" {
		return
	}
	data, err := json.Marshal(po)
	if err != nil {
		return
	}
	ttl := poCacheTTL
	if po.Status == POExhausted || po.Status == PODeclined || po.Status == POExpired {
		ttl = 0 // keep indefinitely for terminal states
	}
	key := fmt.Sprintf("%s%s", poCacheKeyPrefix, po.PONumber)
	_ = c.rdb.Set(ctx, key, data, ttl).Err()
}
