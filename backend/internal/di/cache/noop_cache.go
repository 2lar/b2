package cache

import (
	"context"
	"time"

	"brain2-backend/internal/infrastructure/persistence/cache"
)

// NoOpCache is a simple cache implementation that does nothing
type NoOpCache struct{}

// NewNoOpCache creates a new no-op cache
func NewNoOpCache() cache.Cache {
	return &NoOpCache{}
}

func (c *NoOpCache) Get(ctx context.Context, key string) ([]byte, bool, error) {
	return nil, false, nil
}

func (c *NoOpCache) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	return nil
}

func (c *NoOpCache) Delete(ctx context.Context, key string) error {
	return nil
}

func (c *NoOpCache) Clear(ctx context.Context, pattern string) error {
	return nil
}