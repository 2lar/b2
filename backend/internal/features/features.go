// Package features defines feature flag constants and utilities.
package features

// Feature flag constants for type-safe feature references
const (
	// Core features
	Caching       = "caching"
	AutoConnect   = "auto_connect"
	AIProcessing  = "ai_processing"
	Metrics       = "metrics"
	Tracing       = "tracing"
	EventBus      = "event_bus"
	
	// Infrastructure features
	Retries        = "retries"
	CircuitBreaker = "circuit_breaker"
	RateLimiting   = "rate_limiting"
	Compression    = "compression"
	
	// Debugging features
	DebugEndpoints = "debug_endpoints"
	Profiling      = "profiling"
	Logging        = "logging"
	VerboseLogging = "verbose_logging"
	
	// Experimental features
	GraphQL    = "graphql"
	WebSockets = "websockets"
	BatchAPI   = "batch_api"
)

// FeatureInfo provides metadata about a feature
type FeatureInfo struct {
	Name        string
	Description string
	Category    string
	Stable      bool
}

// GetFeatureInfo returns information about a feature
func GetFeatureInfo(feature string) FeatureInfo {
	features := map[string]FeatureInfo{
		Caching: {
			Name:        "Caching",
			Description: "Enable in-memory and distributed caching",
			Category:    "core",
			Stable:      true,
		},
		AutoConnect: {
			Name:        "Auto Connect",
			Description: "Automatically connect related nodes",
			Category:    "core",
			Stable:      true,
		},
		AIProcessing: {
			Name:        "AI Processing",
			Description: "Enable AI-powered features",
			Category:    "core",
			Stable:      false,
		},
		Metrics: {
			Name:        "Metrics",
			Description: "Enable metrics collection",
			Category:    "core",
			Stable:      true,
		},
		Tracing: {
			Name:        "Distributed Tracing",
			Description: "Enable distributed tracing",
			Category:    "core",
			Stable:      true,
		},
		EventBus: {
			Name:        "Event Bus",
			Description: "Enable domain event publishing",
			Category:    "core",
			Stable:      true,
		},
		Retries: {
			Name:        "Automatic Retries",
			Description: "Enable automatic retry on failures",
			Category:    "infrastructure",
			Stable:      true,
		},
		CircuitBreaker: {
			Name:        "Circuit Breaker",
			Description: "Enable circuit breaker pattern",
			Category:    "infrastructure",
			Stable:      true,
		},
		RateLimiting: {
			Name:        "Rate Limiting",
			Description: "Enable API rate limiting",
			Category:    "infrastructure",
			Stable:      true,
		},
		Compression: {
			Name:        "Response Compression",
			Description: "Enable HTTP response compression",
			Category:    "infrastructure",
			Stable:      true,
		},
		DebugEndpoints: {
			Name:        "Debug Endpoints",
			Description: "Enable debug API endpoints",
			Category:    "debugging",
			Stable:      false,
		},
		Profiling: {
			Name:        "Profiling",
			Description: "Enable performance profiling",
			Category:    "debugging",
			Stable:      false,
		},
		Logging: {
			Name:        "Logging",
			Description: "Enable application logging",
			Category:    "debugging",
			Stable:      true,
		},
		VerboseLogging: {
			Name:        "Verbose Logging",
			Description: "Enable verbose debug logging",
			Category:    "debugging",
			Stable:      false,
		},
		GraphQL: {
			Name:        "GraphQL API",
			Description: "Enable GraphQL API endpoint",
			Category:    "experimental",
			Stable:      false,
		},
		WebSockets: {
			Name:        "WebSockets",
			Description: "Enable WebSocket connections",
			Category:    "experimental",
			Stable:      false,
		},
		BatchAPI: {
			Name:        "Batch API",
			Description: "Enable batch API operations",
			Category:    "experimental",
			Stable:      false,
		},
	}
	
	if info, ok := features[feature]; ok {
		return info
	}
	
	return FeatureInfo{
		Name:        feature,
		Description: "Unknown feature",
		Category:    "unknown",
		Stable:      false,
	}
}