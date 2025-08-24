// Package cache provides caching implementations for the Brain2 backend.
// This file implements an efficient in-memory cache with LRU eviction.
package cache

import (
	"context"
	"container/list"
	"sync"
	"time"
	
	"go.uber.org/zap"
)

// MemoryCache provides an in-memory cache with LRU eviction and TTL support.
// This implementation is thread-safe and suitable for single-instance deployments.
//
// Key Features:
//   - LRU (Least Recently Used) eviction policy
//   - Per-item TTL support
//   - Pattern-based cache invalidation
//   - Memory usage limits
//   - Hit rate statistics
//   - Thread-safe operations
type MemoryCache struct {
	mu          sync.RWMutex
	items       map[string]*cacheItem
	lruList     *list.List
	maxItems    int
	maxMemory   int64
	currentSize int64
	
	// Statistics
	hits       int64
	misses     int64
	evictions  int64
	
	logger     *zap.Logger
}

// cacheItem represents a single cached entry
type cacheItem struct {
	key        string
	value      []byte
	size       int64
	expiry     time.Time
	lruElement *list.Element
}

// NewMemoryCache creates a new in-memory cache with the specified configuration
func NewMemoryCache(maxItems int, maxMemory int64, logger *zap.Logger) *MemoryCache {
	if logger == nil {
		logger = zap.NewNop()
	}
	
	return &MemoryCache{
		items:     make(map[string]*cacheItem),
		lruList:   list.New(),
		maxItems:  maxItems,
		maxMemory: maxMemory,
		logger:    logger,
	}
}

// Get retrieves a value from the cache
func (c *MemoryCache) Get(ctx context.Context, key string) ([]byte, bool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	item, exists := c.items[key]
	if !exists {
		c.misses++
		return nil, false, nil
	}
	
	// Check if item has expired
	if time.Now().After(item.expiry) {
		c.removeItem(item)
		c.misses++
		return nil, false, nil
	}
	
	// Move to front of LRU list
	c.lruList.MoveToFront(item.lruElement)
	c.hits++
	
	// Return a copy to prevent external modifications
	value := make([]byte, len(item.value))
	copy(value, item.value)
	
	return value, true, nil
}

// Set stores a value in the cache with the specified TTL
func (c *MemoryCache) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	// Calculate item size
	itemSize := int64(len(key) + len(value))
	
	// Check if item is too large for cache
	if itemSize > c.maxMemory {
		c.logger.Warn("Item too large for cache",
			zap.String("key", key),
			zap.Int64("size", itemSize),
			zap.Int64("max_memory", c.maxMemory),
		)
		return nil // Silently skip caching
	}
	
	// Remove existing item if present
	if existingItem, exists := c.items[key]; exists {
		c.removeItem(existingItem)
	}
	
	// Evict items if necessary to make room
	for (c.currentSize+itemSize > c.maxMemory || len(c.items) >= c.maxItems) && c.lruList.Len() > 0 {
		oldest := c.lruList.Back()
		if oldest != nil {
			oldItem := oldest.Value.(*cacheItem)
			c.removeItem(oldItem)
			c.evictions++
		}
	}
	
	// Create new cache item
	item := &cacheItem{
		key:    key,
		value:  make([]byte, len(value)),
		size:   itemSize,
		expiry: time.Now().Add(ttl),
	}
	copy(item.value, value)
	
	// Add to LRU list
	element := c.lruList.PushFront(item)
	item.lruElement = element
	
	// Store in map
	c.items[key] = item
	c.currentSize += itemSize
	
	return nil
}

// Delete removes a value from the cache
func (c *MemoryCache) Delete(ctx context.Context, key string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	if item, exists := c.items[key]; exists {
		c.removeItem(item)
	}
	
	return nil
}

// Clear removes all items matching the pattern from the cache
func (c *MemoryCache) Clear(ctx context.Context, pattern string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	// Simple pattern matching (supports * wildcard)
	toDelete := make([]*cacheItem, 0)
	
	for key, item := range c.items {
		if matchPattern(key, pattern) {
			toDelete = append(toDelete, item)
		}
	}
	
	for _, item := range toDelete {
		c.removeItem(item)
	}
	
	c.logger.Info("Cleared cache entries",
		zap.String("pattern", pattern),
		zap.Int("count", len(toDelete)),
	)
	
	return nil
}

// removeItem removes an item from the cache (must be called with lock held)
func (c *MemoryCache) removeItem(item *cacheItem) {
	if item.lruElement != nil {
		c.lruList.Remove(item.lruElement)
	}
	delete(c.items, item.key)
	c.currentSize -= item.size
}

// GetStats returns cache statistics
func (c *MemoryCache) GetStats() CacheStats {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	hitRate := float64(0)
	if total := c.hits + c.misses; total > 0 {
		hitRate = float64(c.hits) / float64(total)
	}
	
	return CacheStats{
		Hits:      c.hits,
		Misses:    c.misses,
		Evictions: c.evictions,
		Items:     len(c.items),
		Size:      c.currentSize,
		HitRate:   hitRate,
	}
}

// CacheStats holds cache statistics
type CacheStats struct {
	Hits      int64
	Misses    int64
	Evictions int64
	Items     int
	Size      int64
	HitRate   float64
}

// matchPattern implements simple wildcard pattern matching
func matchPattern(str, pattern string) bool {
	if pattern == "*" {
		return true
	}
	
	// Simple prefix/suffix matching
	if len(pattern) > 0 && pattern[0] == '*' {
		suffix := pattern[1:]
		return len(str) >= len(suffix) && str[len(str)-len(suffix):] == suffix
	}
	
	if len(pattern) > 0 && pattern[len(pattern)-1] == '*' {
		prefix := pattern[:len(pattern)-1]
		return len(str) >= len(prefix) && str[:len(prefix)] == prefix
	}
	
	return str == pattern
}

// StartCleanup starts a background goroutine to clean up expired items
func (c *MemoryCache) StartCleanup(interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		
		for range ticker.C {
			c.cleanupExpired()
		}
	}()
}

// cleanupExpired removes expired items from the cache
func (c *MemoryCache) cleanupExpired() {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	now := time.Now()
	toRemove := make([]*cacheItem, 0)
	
	for _, item := range c.items {
		if now.After(item.expiry) {
			toRemove = append(toRemove, item)
		}
	}
	
	for _, item := range toRemove {
		c.removeItem(item)
	}
	
	if len(toRemove) > 0 {
		c.logger.Debug("Cleaned up expired cache items",
			zap.Int("count", len(toRemove)),
		)
	}
}