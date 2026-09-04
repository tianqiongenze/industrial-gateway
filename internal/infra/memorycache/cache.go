// Package memorycache implements domain.CachePort with a thread-safe map.
package memorycache

import (
	"sync"
	"time"
)

type item struct {
	value string
	exp   time.Time
}

type Cache struct {
	mu   sync.RWMutex
	data map[string]item
}

func New() *Cache {
	return &Cache{data: map[string]item{}}
}

func (c *Cache) Get(key string) (string, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	it, ok := c.data[key]
	if !ok {
		return "", false
	}
	if !it.exp.IsZero() && time.Now().After(it.exp) {
		return "", false
	}
	return it.value, true
}

func (c *Cache) Set(key, value string, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	var exp time.Time
	if ttl > 0 {
		exp = time.Now().Add(ttl)
	}
	c.data[key] = item{value: value, exp: exp}
}
