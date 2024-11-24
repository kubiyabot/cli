package kubiya

import (
	"sync"
	"time"
)

type cacheEntry struct {
	data       interface{}
	expiration time.Time
}

type Cache struct {
	sync.RWMutex
	data map[string]cacheEntry
	ttl  time.Duration
}

func NewCache(ttl time.Duration) *Cache {
	cache := &Cache{
		data: make(map[string]cacheEntry),
		ttl:  ttl,
	}
	// Start cleanup goroutine
	go cache.cleanup()
	return cache
}

func (c *Cache) Set(key string, value interface{}) {
	c.Lock()
	defer c.Unlock()
	c.data[key] = cacheEntry{
		data:       value,
		expiration: time.Now().Add(c.ttl),
	}
}

func (c *Cache) Get(key string) (interface{}, bool) {
	c.RLock()
	defer c.RUnlock()
	entry, exists := c.data[key]
	if !exists {
		return nil, false
	}
	if time.Now().After(entry.expiration) {
		delete(c.data, key)
		return nil, false
	}
	return entry.data, true
}

func (c *Cache) cleanup() {
	ticker := time.NewTicker(time.Minute)
	for range ticker.C {
		c.Lock()
		for key, entry := range c.data {
			if time.Now().After(entry.expiration) {
				delete(c.data, key)
			}
		}
		c.Unlock()
	}
}
