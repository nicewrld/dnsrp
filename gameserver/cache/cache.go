// caching layer for frequently accessed data
// keeps things zippy when we've got lots of players

package cache

import (
	"sync"
	"time"
)

type Cache struct {
	data     map[string]cacheEntry
	mu       sync.RWMutex
	ttl      time.Duration
	onChange func(key string)
}

type cacheEntry struct {
	value      interface{}
	expiration time.Time
}

func NewCache(ttl time.Duration, onChange func(key string)) *Cache {
	c := &Cache{
		data:     make(map[string]cacheEntry),
		ttl:      ttl,
		onChange: onChange,
	}
	go c.cleanup()
	return c
}

func (c *Cache) Set(key string, value interface{}) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.data[key] = cacheEntry{
		value:      value,
		expiration: time.Now().Add(c.ttl),
	}
	if c.onChange != nil {
		c.onChange(key)
	}
}

func (c *Cache) Get(key string) (interface{}, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	entry, exists := c.data[key]
	if !exists || time.Now().After(entry.expiration) {
		return nil, false
	}
	return entry.value, true
}

func (c *Cache) Delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.data, key)
	if c.onChange != nil {
		c.onChange(key)
	}
}

func (c *Cache) cleanup() {
	ticker := time.NewTicker(c.ttl / 2)
	for range ticker.C {
		c.mu.Lock()
		now := time.Now()
		for key, entry := range c.data {
			if now.After(entry.expiration) {
				delete(c.data, key)
				if c.onChange != nil {
					c.onChange(key)
				}
			}
		}
		c.mu.Unlock()
	}
}
