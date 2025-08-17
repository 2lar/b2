package queries

import (
	"context"
	"time"
)

// Cache interface for query service caching
type Cache interface {
	Get(ctx context.Context, key string) (interface{}, bool)
	Set(ctx context.Context, key string, value interface{}, duration time.Duration)
	Delete(ctx context.Context, key string)
}