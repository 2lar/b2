// Package middleware provides rate limiting middleware for HTTP handlers.
package middleware

import (
	"fmt"
	"net/http"
	"sync"
	"time"
	
	"brain2-backend/internal/config"
	sharedContext "brain2-backend/internal/context"
	"brain2-backend/pkg/api"
	"go.uber.org/zap"
)

// RateLimiter provides rate limiting functionality.
type RateLimiter struct {
	config    *config.RateLimit
	logger    *zap.Logger
	visitors  map[string]*visitor
	mu        sync.RWMutex
	cleanupTicker *time.Ticker
}

// visitor tracks request counts for rate limiting.
type visitor struct {
	limiter  *tokenBucket
	lastSeen time.Time
}

// tokenBucket implements a simple token bucket algorithm.
type tokenBucket struct {
	tokens    float64
	capacity  float64
	refillRate float64
	lastRefill time.Time
	mu        sync.Mutex
}

// NewRateLimiter creates a new rate limiter.
func NewRateLimiter(cfg *config.RateLimit, logger *zap.Logger) *RateLimiter {
	rl := &RateLimiter{
		config:   cfg,
		logger:   logger,
		visitors: make(map[string]*visitor),
	}
	
	// Start cleanup goroutine
	if cfg.Enabled {
		rl.startCleanup()
	}
	
	return rl
}

// Middleware returns the rate limiting middleware function.
func (rl *RateLimiter) Middleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !rl.config.Enabled {
			next(w, r)
			return
		}
		
		// Get identifier for rate limiting
		identifier := rl.getIdentifier(r)
		if identifier == "" {
			// No identifier available, allow request
			next(w, r)
			return
		}
		
		// Check rate limit
		if !rl.allow(identifier) {
			rl.logger.Warn("Rate limit exceeded",
				zap.String("identifier", identifier),
				zap.String("path", r.URL.Path),
			)
			
			// Set rate limit headers
			w.Header().Set("X-RateLimit-Limit", fmt.Sprintf("%d", rl.config.RequestsPerMinute))
			w.Header().Set("X-RateLimit-Remaining", "0")
			w.Header().Set("X-RateLimit-Reset", fmt.Sprintf("%d", time.Now().Add(time.Minute).Unix()))
			w.Header().Set("Retry-After", "60")
			
			api.Error(w, http.StatusTooManyRequests, "Rate limit exceeded. Please try again later.")
			return
		}
		
		// Set rate limit headers for successful requests
		remaining := rl.getRemaining(identifier)
		w.Header().Set("X-RateLimit-Limit", fmt.Sprintf("%d", rl.config.RequestsPerMinute))
		w.Header().Set("X-RateLimit-Remaining", fmt.Sprintf("%d", remaining))
		w.Header().Set("X-RateLimit-Reset", fmt.Sprintf("%d", time.Now().Add(time.Minute).Unix()))
		
		next(w, r)
	}
}

// getIdentifier determines the rate limiting key based on configuration.
func (rl *RateLimiter) getIdentifier(r *http.Request) string {
	var identifier string
	
	// Priority: API Key > User ID > IP Address
	if rl.config.ByAPIKey {
		apiKey := r.Header.Get("X-API-Key")
		if apiKey != "" {
			return "api:" + apiKey
		}
	}
	
	if rl.config.ByUser {
		userID, ok := sharedContext.GetUserIDFromContext(r.Context())
		if ok && userID != "" {
			return "user:" + userID
		}
	}
	
	if rl.config.ByIP {
		// Get real IP address (considering proxy headers)
		ip := r.RemoteAddr
		if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
			ip = forwarded
		} else if realIP := r.Header.Get("X-Real-IP"); realIP != "" {
			ip = realIP
		}
		return "ip:" + ip
	}
	
	return identifier
}

// allow checks if a request is allowed under the rate limit.
func (rl *RateLimiter) allow(identifier string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	
	v, exists := rl.visitors[identifier]
	if !exists {
		// Create new visitor with token bucket
		tokens := float64(rl.config.Burst)
		if tokens <= 0 {
			tokens = float64(rl.config.RequestsPerMinute) / 60.0
		}
		
		v = &visitor{
			limiter: &tokenBucket{
				tokens:     tokens,
				capacity:   tokens,
				refillRate: float64(rl.config.RequestsPerMinute) / 60.0,
				lastRefill: time.Now(),
			},
			lastSeen: time.Now(),
		}
		rl.visitors[identifier] = v
	}
	
	v.lastSeen = time.Now()
	return v.limiter.allow()
}

// getRemaining returns the number of remaining requests for an identifier.
func (rl *RateLimiter) getRemaining(identifier string) int {
	rl.mu.RLock()
	defer rl.mu.RUnlock()
	
	v, exists := rl.visitors[identifier]
	if !exists {
		return rl.config.RequestsPerMinute
	}
	
	return int(v.limiter.getTokens())
}

// allow checks if the token bucket allows a request.
func (tb *tokenBucket) allow() bool {
	tb.mu.Lock()
	defer tb.mu.Unlock()
	
	// Refill tokens based on time elapsed
	now := time.Now()
	elapsed := now.Sub(tb.lastRefill).Seconds()
	tb.tokens += elapsed * tb.refillRate
	if tb.tokens > tb.capacity {
		tb.tokens = tb.capacity
	}
	tb.lastRefill = now
	
	// Check if we have tokens available
	if tb.tokens >= 1 {
		tb.tokens--
		return true
	}
	
	return false
}

// getTokens returns the current number of tokens.
func (tb *tokenBucket) getTokens() float64 {
	tb.mu.Lock()
	defer tb.mu.Unlock()
	
	// Refill tokens based on time elapsed
	now := time.Now()
	elapsed := now.Sub(tb.lastRefill).Seconds()
	tokens := tb.tokens + (elapsed * tb.refillRate)
	if tokens > tb.capacity {
		tokens = tb.capacity
	}
	
	return tokens
}

// startCleanup starts a goroutine to clean up old visitors.
func (rl *RateLimiter) startCleanup() {
	rl.cleanupTicker = time.NewTicker(rl.config.CleanupInterval)
	
	go func() {
		for range rl.cleanupTicker.C {
			rl.cleanup()
		}
	}()
}

// cleanup removes old visitors from memory.
func (rl *RateLimiter) cleanup() {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	
	cutoff := time.Now().Add(-rl.config.CleanupInterval)
	
	for identifier, v := range rl.visitors {
		if v.lastSeen.Before(cutoff) {
			delete(rl.visitors, identifier)
			rl.logger.Debug("Cleaned up rate limit visitor",
				zap.String("identifier", identifier),
			)
		}
	}
}

// Stop stops the rate limiter and cleans up resources.
func (rl *RateLimiter) Stop() {
	if rl.cleanupTicker != nil {
		rl.cleanupTicker.Stop()
	}
}