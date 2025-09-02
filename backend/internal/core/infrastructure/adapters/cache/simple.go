// Package cache provides cache implementations
package cache

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// SimpleCache provides a basic in-memory cache implementation
type SimpleCache struct {
	items map[string]*cacheItem
	mu    sync.RWMutex
}

type cacheItem struct {
	value     []byte
	expiresAt time.Time
}

// NewSimpleCache creates a new simple cache
func NewSimpleCache() *SimpleCache {
	cache := &SimpleCache{
		items: make(map[string]*cacheItem),
	}
	
	// Start cleanup goroutine
	go cache.cleanup()
	
	return cache
}

// Get retrieves a value from cache
func (c *SimpleCache) Get(ctx context.Context, key string) ([]byte, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	item, exists := c.items[key]
	if !exists {
		return nil, fmt.Errorf("cache miss: key %s not found", key)
	}
	
	if time.Now().After(item.expiresAt) {
		return nil, fmt.Errorf("cache miss: key %s expired", key)
	}
	
	return item.value, nil
}

// Set stores a value in cache
func (c *SimpleCache) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	c.items[key] = &cacheItem{
		value:     value,
		expiresAt: time.Now().Add(ttl),
	}
	
	return nil
}

// Delete removes a value from cache
func (c *SimpleCache) Delete(ctx context.Context, key string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	delete(c.items, key)
	return nil
}

// Exists checks if a key exists
func (c *SimpleCache) Exists(ctx context.Context, key string) (bool, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	item, exists := c.items[key]
	if !exists {
		return false, nil
	}
	
	if time.Now().After(item.expiresAt) {
		return false, nil
	}
	
	return true, nil
}

// Clear clears all cache entries
func (c *SimpleCache) Clear(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	c.items = make(map[string]*cacheItem)
	return nil
}

// GetMulti retrieves multiple values
func (c *SimpleCache) GetMulti(ctx context.Context, keys []string) (map[string][]byte, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	result := make(map[string][]byte)
	now := time.Now()
	
	for _, key := range keys {
		if item, exists := c.items[key]; exists {
			if now.Before(item.expiresAt) {
				result[key] = item.value
			}
		}
	}
	
	return result, nil
}

// SetMulti stores multiple values
func (c *SimpleCache) SetMulti(ctx context.Context, items map[string][]byte, ttl time.Duration) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	expiresAt := time.Now().Add(ttl)
	
	for key, value := range items {
		c.items[key] = &cacheItem{
			value:     value,
			expiresAt: expiresAt,
		}
	}
	
	return nil
}

// cleanup periodically removes expired items
func (c *SimpleCache) cleanup() {
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