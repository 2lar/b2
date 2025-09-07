package auth

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// RateLimiter provides rate limiting functionality
type RateLimiter interface {
	Allow(ctx context.Context, key string) (bool, error)
	Reset(ctx context.Context, key string) error
}

// TokenBucketLimiter implements token bucket rate limiting
type TokenBucketLimiter struct {
	mu         sync.RWMutex
	buckets    map[string]*bucket
	maxTokens  int
	refillRate time.Duration
	cleanupInt time.Duration
}

type bucket struct {
	tokens    int
	lastRefill time.Time
	mu        sync.Mutex
}

// NewTokenBucketLimiter creates a new token bucket rate limiter
func NewTokenBucketLimiter(maxTokens int, refillRate time.Duration) *TokenBucketLimiter {
	limiter := &TokenBucketLimiter{
		buckets:    make(map[string]*bucket),
		maxTokens:  maxTokens,
		refillRate: refillRate,
		cleanupInt: 5 * time.Minute,
	}

	// Start cleanup goroutine
	go limiter.cleanup()

	return limiter
}

// Allow checks if a request is allowed
func (l *TokenBucketLimiter) Allow(ctx context.Context, key string) (bool, error) {
	l.mu.Lock()
	b, exists := l.buckets[key]
	if !exists {
		b = &bucket{
			tokens:     l.maxTokens,
			lastRefill: time.Now(),
		}
		l.buckets[key] = b
	}
	l.mu.Unlock()

	b.mu.Lock()
	defer b.mu.Unlock()

	// Refill tokens based on time elapsed
	now := time.Now()
	elapsed := now.Sub(b.lastRefill)
	tokensToAdd := int(elapsed / l.refillRate)
	
	if tokensToAdd > 0 {
		b.tokens = min(b.tokens+tokensToAdd, l.maxTokens)
		b.lastRefill = now
	}

	// Check if request is allowed
	if b.tokens > 0 {
		b.tokens--
		return true, nil
	}

	return false, nil
}

// Reset resets the rate limit for a key
func (l *TokenBucketLimiter) Reset(ctx context.Context, key string) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	
	delete(l.buckets, key)
	return nil
}

// cleanup removes old buckets periodically
func (l *TokenBucketLimiter) cleanup() {
	ticker := time.NewTicker(l.cleanupInt)
	defer ticker.Stop()

	for range ticker.C {
		l.mu.Lock()
		now := time.Now()
		for key, b := range l.buckets {
			b.mu.Lock()
			if now.Sub(b.lastRefill) > 1*time.Hour {
				delete(l.buckets, key)
			}
			b.mu.Unlock()
		}
		l.mu.Unlock()
	}
}

// SlidingWindowLimiter implements sliding window rate limiting
type SlidingWindowLimiter struct {
	mu         sync.RWMutex
	windows    map[string]*window
	limit      int
	windowSize time.Duration
}

type window struct {
	requests []time.Time
	mu       sync.Mutex
}

// NewSlidingWindowLimiter creates a new sliding window rate limiter
func NewSlidingWindowLimiter(limit int, windowSize time.Duration) *SlidingWindowLimiter {
	return &SlidingWindowLimiter{
		windows:    make(map[string]*window),
		limit:      limit,
		windowSize: windowSize,
	}
}

// Allow checks if a request is allowed
func (l *SlidingWindowLimiter) Allow(ctx context.Context, key string) (bool, error) {
	l.mu.Lock()
	w, exists := l.windows[key]
	if !exists {
		w = &window{
			requests: make([]time.Time, 0),
		}
		l.windows[key] = w
	}
	l.mu.Unlock()

	w.mu.Lock()
	defer w.mu.Unlock()

	now := time.Now()
	windowStart := now.Add(-l.windowSize)

	// Remove old requests outside the window
	validRequests := make([]time.Time, 0)
	for _, reqTime := range w.requests {
		if reqTime.After(windowStart) {
			validRequests = append(validRequests, reqTime)
		}
	}
	w.requests = validRequests

	// Check if limit is exceeded
	if len(w.requests) >= l.limit {
		return false, nil
	}

	// Add current request
	w.requests = append(w.requests, now)
	return true, nil
}

// Reset resets the rate limit for a key
func (l *SlidingWindowLimiter) Reset(ctx context.Context, key string) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	
	delete(l.windows, key)
	return nil
}

// IPRateLimiter wraps a rate limiter for IP-based limiting
type IPRateLimiter struct {
	limiter RateLimiter
}

// NewIPRateLimiter creates a new IP-based rate limiter
func NewIPRateLimiter(requestsPerMinute int) *IPRateLimiter {
	return &IPRateLimiter{
		limiter: NewSlidingWindowLimiter(requestsPerMinute, time.Minute),
	}
}

// Allow checks if a request from an IP is allowed
func (l *IPRateLimiter) Allow(ctx context.Context, ip string) (bool, error) {
	return l.limiter.Allow(ctx, fmt.Sprintf("ip:%s", ip))
}

// UserRateLimiter wraps a rate limiter for user-based limiting
type UserRateLimiter struct {
	limiter RateLimiter
}

// NewUserRateLimiter creates a new user-based rate limiter
func NewUserRateLimiter(requestsPerMinute int) *UserRateLimiter {
	return &UserRateLimiter{
		limiter: NewSlidingWindowLimiter(requestsPerMinute, time.Minute),
	}
}

// Allow checks if a request from a user is allowed
func (l *UserRateLimiter) Allow(ctx context.Context, userID string) (bool, error) {
	return l.limiter.Allow(ctx, fmt.Sprintf("user:%s", userID))
}

// CompositeRateLimiter combines multiple rate limiters
type CompositeRateLimiter struct {
	limiters []RateLimiter
}

// NewCompositeRateLimiter creates a new composite rate limiter
func NewCompositeRateLimiter(limiters ...RateLimiter) *CompositeRateLimiter {
	return &CompositeRateLimiter{
		limiters: limiters,
	}
}

// Allow checks if a request is allowed by all limiters
func (l *CompositeRateLimiter) Allow(ctx context.Context, key string) (bool, error) {
	for _, limiter := range l.limiters {
		allowed, err := limiter.Allow(ctx, key)
		if err != nil {
			return false, err
		}
		if !allowed {
			return false, nil
		}
	}
	return true, nil
}

// Reset resets all limiters for a key
func (l *CompositeRateLimiter) Reset(ctx context.Context, key string) error {
	for _, limiter := range l.limiters {
		if err := limiter.Reset(ctx, key); err != nil {
			return err
		}
	}
	return nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}