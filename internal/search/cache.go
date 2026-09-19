package search

import (
	"sync"
	"time"
)

type cacheEntry struct {
	value     Response
	expiresAt time.Time
	createdAt time.Time
}

type Cache struct {
	mu         sync.Mutex
	items      map[string]cacheEntry
	maxEntries int
}

func NewCache(maxEntries int) *Cache {
	if maxEntries <= 0 { maxEntries = 256 }
	return &Cache{items: make(map[string]cacheEntry, maxEntries), maxEntries: maxEntries}
}

func (c *Cache) Get(key string) (Response, bool) {
	if c == nil || key == "" { return Response{}, false }
	now := time.Now()
	c.mu.Lock()
	defer c.mu.Unlock()
	entry, ok := c.items[key]
	if !ok { return Response{}, false }
	if now.After(entry.expiresAt) {
		delete(c.items, key)
		return Response{}, false
	}
	return entry.value, true
}

func (c *Cache) Put(key string, value Response, ttl time.Duration) {
	if c == nil || key == "" || ttl <= 0 { return }
	now := time.Now()
	c.mu.Lock()
	defer c.mu.Unlock()
	for k, entry := range c.items {
		if now.After(entry.expiresAt) { delete(c.items, k) }
	}
	if len(c.items) >= c.maxEntries {
		var oldestKey string
		var oldest time.Time
		for k, entry := range c.items {
			if oldestKey == "" || entry.createdAt.Before(oldest) {
				oldestKey, oldest = k, entry.createdAt
			}
		}
		if oldestKey != "" { delete(c.items, oldestKey) }
	}
	value.Cached = false
	c.items[key] = cacheEntry{value: value, createdAt: now, expiresAt: now.Add(ttl)}
}
