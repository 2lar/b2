package queries

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// CacheHelper provides common cache operations for query services
type CacheHelper struct {
	cache Cache
}

// NewCacheHelper creates a new cache helper
func NewCacheHelper(cache Cache) *CacheHelper {
	return &CacheHelper{cache: cache}
}

// GetCached attempts to retrieve a cached result and unmarshal it
func (h *CacheHelper) GetCached(ctx context.Context, key string, result interface{}) (bool, error) {
	if h.cache == nil {
		return false, nil
	}
	
	cachedData, found, err := h.cache.Get(ctx, key)
	if err != nil || !found {
		return false, err
	}
	
	if err := json.Unmarshal(cachedData, result); err != nil {
		return false, err
	}
	
	return true, nil
}

// SetCached marshals and caches a result
func (h *CacheHelper) SetCached(ctx context.Context, key string, result interface{}, ttl time.Duration) error {
	if h.cache == nil {
		return nil
	}
	
	data, err := json.Marshal(result)
	if err != nil {
		return err
	}
	
	return h.cache.Set(ctx, key, data, ttl)
}

// GenerateCacheKey creates a consistent cache key with prefix and parameters
func GenerateCacheKey(prefix string, params ...interface{}) string {
	switch len(params) {
	case 1:
		return prefix + ":" + toString(params[0])
	case 2:
		return prefix + ":" + toString(params[0]) + ":" + toString(params[1])
	case 3:
		return prefix + ":" + toString(params[0]) + ":" + toString(params[1]) + ":" + toString(params[2])
	case 4:
		return prefix + ":" + toString(params[0]) + ":" + toString(params[1]) + ":" + toString(params[2]) + ":" + toString(params[3])
	default:
		// For more complex keys, fall back to fmt.Sprintf pattern
		result := prefix
		for _, param := range params {
			result += ":" + toString(param)
		}
		return result
	}
}

// toString converts interface{} to string representation for cache keys
func toString(v interface{}) string {
	switch val := v.(type) {
	case string:
		return val
	case int:
		return fmt.Sprintf("%d", val)
	case int64:
		return fmt.Sprintf("%d", val)
	case bool:
		if val {
			return "true"
		}
		return "false"
	case float64:
		return fmt.Sprintf("%.2f", val)
	default:
		// Fallback to sprintf for complex types
		return fmt.Sprintf("%v", val)
	}
}