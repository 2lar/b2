package di

import (
	"context"
	"sync"
	"time"
)

// InMemoryCache provides a simple in-memory cache implementation
type InMemoryCache struct {
	mu    sync.RWMutex
	items map[string]cacheItem
}

type cacheItem struct {
	value     interface{}
	expiresAt time.Time
}

// NewInMemoryCache creates a new in-memory cache
func NewInMemoryCache() *InMemoryCache {
	cache := &InMemoryCache{
		items: make(map[string]cacheItem),
	}

	// Start cleanup goroutine
	go cache.cleanupExpired()

	return cache
}

// Get retrieves a value from cache
func (c *InMemoryCache) Get(ctx context.Context, key string) (interface{}, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	item, exists := c.items[key]
	if !exists {
		return nil, false
	}

	if time.Now().After(item.expiresAt) {
		return nil, false
	}

	return item.value, true
}

// Set stores a value in cache with TTL in seconds
func (c *InMemoryCache) Set(ctx context.Context, key string, value interface{}, ttl int) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.items[key] = cacheItem{
		value:     value,
		expiresAt: time.Now().Add(time.Duration(ttl) * time.Second),
	}

	return nil
}

// Delete removes a value from cache
func (c *InMemoryCache) Delete(ctx context.Context, key string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	delete(c.items, key)
	return nil
}

// Clear removes all values from cache
func (c *InMemoryCache) Clear(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.items = make(map[string]cacheItem)
	return nil
}

// cleanupExpired periodically removes expired items
func (c *InMemoryCache) cleanupExpired() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		c.mu.Lock()
		now := time.Now()
		for key, item := range c.items {
			if now.After(item.expiresAt) {
				delete(c.items, key)
			}
		}
		c.mu.Unlock()
	}
}
