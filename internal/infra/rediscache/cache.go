// Package rediscache implements a two-level cache:
//   L1: in-process sync.Map, TTL 30s  (fastest, per-pod)
//   L2: Redis (go-redis/v9), TTL 300s (shared across pods)
// Lookup: L1 -> L2 -> (caller loads from DB and calls Set)
// Invalidation: Delete drops L2 first, then L1.
package rediscache

import (
	"context"
	"encoding/json"
	"log"
	"sync"
	"sync/atomic"
	"time"

	"github.com/redis/go-redis/v9"
)

// Metrics are atomic cache counters for observability.
type Metrics struct {
	L1Hits, L1Misses, L2Hits, L2Misses, L2Errs, Sets, Deletes int64
}

var (
	m   Metrics
	mmu sync.RWMutex
)

// Stats returns a snapshot of the counters.
func Stats() Metrics {
	mmu.RLock()
	defer mmu.RUnlock()
	return m
}

func inc(field *int64) { atomic.AddInt64(field, 1) }

type l1Item struct {
	value string
	exp   int64 // unix nanos
}

// Cache is the two-level cache.
type Cache struct {
	l1     sync.Map
	client *redis.Client
	ctx    context.Context
	l1TTL  time.Duration
	l2TTL  time.Duration
}

// New builds the cache and pings Redis.
func New(redisAddr, redisPass string) (*Cache, error) {
	rdb := redis.NewClient(&redis.Options{
		Addr:         redisAddr,
		Password:     redisPass,
		DialTimeout:  3 * time.Second,
		ReadTimeout:  2 * time.Second,
		WriteTimeout: 2 * time.Second,
		PoolSize:     50,
	})
	ctx := context.Background()
	if err := rdb.Ping(ctx).Err(); err != nil {
		return nil, err
	}
	log.Printf("[cache] L2 Redis connected: %s", redisAddr)
	return &Cache{client: rdb, ctx: ctx, l1TTL: 30 * time.Second, l2TTL: 300 * time.Second}, nil
}

// Get implements the two-level lookup. Returns value + whether it came from L1.
func (c *Cache) Get(key string) (string, bool) {
	// L1
	if v, ok := c.l1.Load(key); ok {
		it := v.(l1Item)
		if time.Now().UnixNano() < it.exp {
			inc(&m.L1Hits)
			return it.value, true
		}
		c.l1.Delete(key)
	}
	inc(&m.L1Misses)
	// L2
	val, err := c.client.Get(c.ctx, key).Result()
	if err == redis.Nil {
		inc(&m.L2Misses)
		return "", false
	}
	if err != nil {
		inc(&m.L2Errs)
		return "", false
	}
	inc(&m.L2Hits)
	// backfill L1 with remaining L2 TTL (capped at L1 TTL)
	ttl, err := c.client.TTL(c.ctx, key).Result()
	if err != nil || ttl <= 0 || ttl > c.l1TTL {
		ttl = c.l1TTL
	}
	c.l1.Store(key, l1Item{value: val, exp: time.Now().UnixNano() + ttl.Nanoseconds()})
	return val, true
}

// GetJSON unmarshals into dst.
func (c *Cache) GetJSON(key string, dst interface{}) (bool, error) {
	v, ok := c.Get(key)
	if !ok {
		return false, nil
	}
	return true, json.Unmarshal([]byte(v), dst)
}

// SetJSON marshals, writes L2 then L1.
func (c *Cache) SetJSON(key string, v interface{}) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	if err := c.client.Set(c.ctx, key, string(b), c.l2TTL).Err(); err != nil {
		inc(&m.L2Errs)
		return err
	}
	inc(&m.Sets)
	c.l1.Store(key, l1Item{value: string(b), exp: time.Now().Add(c.l1TTL).UnixNano()})
	return nil
}

// Delete invalidates L2 first (source of truth), then L1.
func (c *Cache) Delete(key string) {
	c.client.Del(c.ctx, key)
	c.l1.Delete(key)
	inc(&m.Deletes)
}

// Name identifies the cache implementation.
func (c *Cache) Name() string { return "two-level (L1 sync.Map 30s + L2 redis 300s)" }

// GoString avoids printing secrets.
func (c *Cache) GoString() string { return "rediscache.Cache" }

// Set implements the legacy domain.CachePort (string -> string, ttl 0 = use L2 TTL).
func (c *Cache) Set(key, value string, ttl time.Duration) {
	t := c.l2TTL
	if ttl > 0 && ttl < c.l2TTL {
		t = ttl
	}
	if err := c.client.Set(c.ctx, key, value, t).Err(); err != nil {
		inc(&m.L2Errs)
	}
	c.l1.Store(key, l1Item{value: value, exp: time.Now().Add(c.l1TTL).UnixNano()})
	inc(&m.Sets)
}
