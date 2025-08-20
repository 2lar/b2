// Package features provides enhanced feature flag management.
// This is a simple implementation that extends the basic boolean flags
// with percentage-based rollout capabilities.
package features

import (
	"hash/fnv"
	"sync"
	
	"brain2-backend/internal/config"
)

// FeatureService provides enhanced feature flag management.
// It supports percentage-based rollouts and runtime overrides.
type FeatureService struct {
	config    *config.Features
	overrides map[string]interface{} // For runtime changes
	mu        sync.RWMutex
}

// NewFeatureService creates a new feature service instance
func NewFeatureService(config *config.Features) *FeatureService {
	return &FeatureService{
		config:    config,
		overrides: make(map[string]interface{}),
	}
}

// IsEnabled checks if a feature is enabled for a user.
// It supports percentage-based rollouts for gradual feature deployment.
func (fs *FeatureService) IsEnabled(feature string, userID string) bool {
	fs.mu.RLock()
	defer fs.mu.RUnlock()
	
	// Check for percentage rollout override
	if percentage, ok := fs.overrides[feature+"_percentage"].(float64); ok {
		hash := hashUserID(userID)
		return (hash % 100) < int(percentage*100)
	}
	
	// Check for boolean override
	if enabled, ok := fs.overrides[feature].(bool); ok {
		return enabled
	}
	
	// Fall back to config
	switch feature {
	case "caching":
		return fs.config.EnableCaching
	case "auto_connect":
		return fs.config.EnableAutoConnect
	case "ai_processing":
		return fs.config.EnableAIProcessing
	case "metrics":
		return fs.config.EnableMetrics
	case "tracing":
		return fs.config.EnableTracing
	case "event_bus":
		return fs.config.EnableEventBus
	case "retries":
		return fs.config.EnableRetries
	case "circuit_breaker":
		return fs.config.EnableCircuitBreaker
	case "rate_limiting":
		return fs.config.EnableRateLimiting
	case "compression":
		return fs.config.EnableCompression
	case "debug_endpoints":
		return fs.config.EnableDebugEndpoints
	case "profiling":
		return fs.config.EnableProfiling
	case "logging":
		return fs.config.EnableLogging
	case "verbose_logging":
		return fs.config.VerboseLogging
	case "graphql":
		return fs.config.EnableGraphQL
	case "websockets":
		return fs.config.EnableWebSockets
	case "batch_api":
		return fs.config.EnableBatchAPI
	default:
		return false
	}
}

// SetPercentageRollout enables gradual feature rollout.
// percentage should be between 0.0 and 1.0 (0% to 100%).
func (fs *FeatureService) SetPercentageRollout(feature string, percentage float64) {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	
	// Clamp percentage between 0 and 1
	if percentage < 0 {
		percentage = 0
	}
	if percentage > 1 {
		percentage = 1
	}
	
	fs.overrides[feature+"_percentage"] = percentage
}

// SetOverride sets a boolean override for a feature.
// This is useful for emergency feature toggles.
func (fs *FeatureService) SetOverride(feature string, enabled bool) {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	
	fs.overrides[feature] = enabled
}

// ClearOverride removes any override for a feature.
// The feature will revert to its config value.
func (fs *FeatureService) ClearOverride(feature string) {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	
	delete(fs.overrides, feature)
	delete(fs.overrides, feature+"_percentage")
}

// GetFeatureStatus returns the current status of a feature.
// This is useful for debugging and monitoring.
func (fs *FeatureService) GetFeatureStatus(feature string) map[string]interface{} {
	fs.mu.RLock()
	defer fs.mu.RUnlock()
	
	status := make(map[string]interface{})
	status["feature"] = feature
	
	// Check for overrides
	if percentage, ok := fs.overrides[feature+"_percentage"].(float64); ok {
		status["rollout_percentage"] = percentage * 100
		status["type"] = "percentage_rollout"
	} else if enabled, ok := fs.overrides[feature].(bool); ok {
		status["enabled"] = enabled
		status["type"] = "override"
	} else {
		// Get from config
		status["enabled"] = fs.IsEnabled(feature, "")
		status["type"] = "config"
	}
	
	return status
}

// GetAllFeatures returns the status of all features.
// This is useful for admin dashboards.
func (fs *FeatureService) GetAllFeatures() map[string]interface{} {
	features := []string{
		"caching", "auto_connect", "ai_processing", "metrics", "tracing",
		"event_bus", "retries", "circuit_breaker", "rate_limiting",
		"compression", "debug_endpoints", "profiling", "logging",
		"verbose_logging", "graphql", "websockets", "batch_api",
	}
	
	result := make(map[string]interface{})
	for _, feature := range features {
		result[feature] = fs.GetFeatureStatus(feature)
	}
	
	return result
}

// hashUserID generates a consistent hash for a user ID.
// This ensures the same user always gets the same feature state.
func hashUserID(userID string) int {
	h := fnv.New32a()
	h.Write([]byte(userID))
	return int(h.Sum32())
}

// abs returns the absolute value of an integer
func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}