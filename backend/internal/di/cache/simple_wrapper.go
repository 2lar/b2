package cache

import (
	"context"
	"time"

	"brain2-backend/internal/infrastructure/persistence/cache"
)

type SimpleMemoryCacheWrapper struct {
	cache cache.Cache
}

// Get retrieves a value from the cache
func (c *SimpleMemoryCacheWrapper) Get(ctx context.Context, key string) ([]byte, bool, error) {
	return c.cache.Get(ctx, key)
}

// Set stores a value in the cache
func (c *SimpleMemoryCacheWrapper) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	return c.cache.Set(ctx, key, value, ttl)
}

// Delete removes a value from the cache
func (c *SimpleMemoryCacheWrapper) Delete(ctx context.Context, key string) error {
	return c.cache.Delete(ctx, key)
}

// Clear removes all values matching the pattern from the cache
func (c *SimpleMemoryCacheWrapper) Clear(ctx context.Context, pattern string) error {
	return c.cache.Clear(ctx, pattern)
}