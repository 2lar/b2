package cache

import (
	"context"
	"sync"
	"time"

	"brain2-backend/internal/infrastructure/persistence/cache"
)

// InMemoryCache is a simple in-memory cache implementation
type InMemoryCache struct {
	items    map[string]inMemoryCacheItem
	maxItems int
	ttl      time.Duration
	mu       sync.RWMutex
}

type inMemoryCacheItem struct {
	value     []byte
	expiresAt time.Time
}

// NewInMemoryCache creates a new in-memory cache
func NewInMemoryCache(maxItems int, defaultTTL time.Duration) cache.Cache {
	cache := &InMemoryCache{
		items:    make(map[string]inMemoryCacheItem),
		maxItems: maxItems,
		ttl:      defaultTTL,
	}
	// Start cleanup goroutine
	go cache.cleanup()
	return cache
}

func (c *InMemoryCache) Get(ctx context.Context, key string) ([]byte, bool, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	item, exists := c.items[key]
	if !exists {
		return nil, false, nil
	}

	if time.Now().After(item.expiresAt) {
		return nil, false, nil
	}

	return item.value, true, nil
}

func (c *InMemoryCache) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if ttl == 0 {
		ttl = c.ttl
	}

	// Evict old items if cache is full
	if len(c.items) >= c.maxItems {
		c.evictOldest()
	}

	c.items[key] = inMemoryCacheItem{
		value:     value,
		expiresAt: time.Now().Add(ttl),
	}

	return nil
}

func (c *InMemoryCache) Delete(ctx context.Context, key string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	delete(c.items, key)
	return nil
}

func (c *InMemoryCache) Clear(ctx context.Context, pattern string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Simple pattern matching (prefix only for now)
	for key := range c.items {
		if pattern == "*" || (len(pattern) > 0 && pattern[len(pattern)-1] == '*' &&
			len(key) >= len(pattern)-1 && key[:len(pattern)-1] == pattern[:len(pattern)-1]) {
			delete(c.items, key)
		}
	}

	return nil
}

func (c *InMemoryCache) evictOldest() {
	var oldestKey string
	var oldestTime time.Time

	for key, item := range c.items {
		if oldestTime.IsZero() || item.expiresAt.Before(oldestTime) {
			oldestKey = key
			oldestTime = item.expiresAt
		}
	}

	if oldestKey != "" {
		delete(c.items, oldestKey)
	}
}

func (c *InMemoryCache) cleanup() {
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