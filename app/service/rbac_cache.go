package service

import (
	"context"
	"encoding/json"
	"path"
	"sync"
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

// redisRBACCache adapts *redis.Client. With no Redis client it falls back to a process-local
// TTL map (same TTLs the callers already pass) instead of a no-op: without it, every permission-
// gated request re-fetched the role's permissions from SSO (10s timeout x 1 retry = ~20s when SSO
// stalls), which is exactly the "every API takes 20s" incident when Redis was down.
type redisRBACCache struct {
	c     *redis.Client
	mu    sync.Mutex
	local map[string]localRBACEntry
}

type localRBACEntry struct {
	raw     []byte
	expires time.Time
}

const localRBACMax = 5000

// NewRedisRBACCache wraps a redis client (may be nil → process-local cache).
func NewRedisRBACCache(c *redis.Client) RBACCache {
	return &redisRBACCache{c: c, local: map[string]localRBACEntry{}}
}

func (r *redisRBACCache) GetJSON(ctx context.Context, key string, dst any) bool {
	if r.c == nil {
		r.mu.Lock()
		e, ok := r.local[key]
		if ok && time.Now().After(e.expires) {
			delete(r.local, key)
			ok = false
		}
		r.mu.Unlock()
		return ok && json.Unmarshal(e.raw, dst) == nil
	}
	raw, err := r.c.Get(ctx, key).Bytes()
	if err != nil || len(raw) == 0 {
		return false
	}
	return json.Unmarshal(raw, dst) == nil
}

func (r *redisRBACCache) SetJSON(ctx context.Context, key string, val any, ttl time.Duration) {
	if r.c == nil {
		raw, err := json.Marshal(val)
		if err != nil {
			return
		}
		r.mu.Lock()
		defer r.mu.Unlock()
		if len(r.local) >= localRBACMax { // bound memory: drop expired, else reset
			now := time.Now()
			for k, e := range r.local {
				if now.After(e.expires) {
					delete(r.local, k)
				}
			}
			if len(r.local) >= localRBACMax {
				r.local = map[string]localRBACEntry{}
			}
		}
		r.local[key] = localRBACEntry{raw: raw, expires: time.Now().Add(ttl)}
		return
	}
	if raw, err := json.Marshal(val); err == nil {
		r.c.Set(ctx, key, raw, ttl)
	}
}

func (r *redisRBACCache) Del(ctx context.Context, keys ...string) {
	if r.c == nil {
		r.mu.Lock()
		for _, k := range keys {
			delete(r.local, k)
		}
		r.mu.Unlock()
		return
	}
	if len(keys) == 0 {
		return
	}
	r.c.Del(ctx, keys...)
}

func (r *redisRBACCache) DelByPattern(ctx context.Context, pattern string) {
	if r.c == nil {
		r.mu.Lock()
		for k := range r.local {
			if ok, _ := path.Match(pattern, k); ok {
				delete(r.local, k)
			}
		}
		r.mu.Unlock()
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
