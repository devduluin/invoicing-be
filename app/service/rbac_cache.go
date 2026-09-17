package service

import (
	"context"
	"encoding/json"
	"time"

	"github.com/redis/go-redis/v9"
)

// RBACCache is the small cache surface the membership service needs. Injected so
// the service is unit-testable without Redis.
type RBACCache interface {
	GetJSON(ctx context.Context, key string, dst any) bool
	SetJSON(ctx context.Context, key string, val any, ttl time.Duration)
	Del(ctx context.Context, keys ...string)
	DelByPattern(ctx context.Context, pattern string)
}

// redisRBACCache adapts *redis.Client. A nil client degrades to a no-op.
type redisRBACCache struct{ c *redis.Client }

// NewRedisRBACCache wraps a redis client (may be nil → no-op cache).
func NewRedisRBACCache(c *redis.Client) RBACCache { return &redisRBACCache{c: c} }

func (r *redisRBACCache) GetJSON(ctx context.Context, key string, dst any) bool {
	if r.c == nil {
		return false
	}
	raw, err := r.c.Get(ctx, key).Bytes()
	if err != nil || len(raw) == 0 {
		return false
	}
	return json.Unmarshal(raw, dst) == nil
}

func (r *redisRBACCache) SetJSON(ctx context.Context, key string, val any, ttl time.Duration) {
	if r.c == nil {
		return
	}
	if raw, err := json.Marshal(val); err == nil {
		r.c.Set(ctx, key, raw, ttl)
	}
}

func (r *redisRBACCache) Del(ctx context.Context, keys ...string) {
	if r.c == nil || len(keys) == 0 {
		return
	}
	r.c.Del(ctx, keys...)
}

func (r *redisRBACCache) DelByPattern(ctx context.Context, pattern string) {
	if r.c == nil {
		return
	}
	iter := r.c.Scan(ctx, 0, pattern, 100).Iterator()
	var batch []string
	for iter.Next(ctx) {
		batch = append(batch, iter.Val())
		if len(batch) >= 100 {
			r.c.Del(ctx, batch...)
			batch = batch[:0]
		}
	}
	if len(batch) > 0 {
		r.c.Del(ctx, batch...)
	}
}
