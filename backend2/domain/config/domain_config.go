package config

import "time"

// DomainConfig holds all configurable business rules and constraints
type DomainConfig struct {
	// Graph constraints
	MaxNodesPerGraph int
	MaxEdgesPerGraph int
	DefaultGraphName string
	
	// Performance limits
	MaxNodesPerQuery int
	MaxEdgesPerQuery int
	MaxSimilarityCalculations int
	SimilarityThreshold float64

	// Node constraints
	MaxConnectionsPerNode int
	MaxTagsPerNode        int
	MaxTitleLength        int
	MaxContentLength      int
	MinTitleLength        int

	// Edge constraints
	MaxEdgeWeight     float64
	MinEdgeWeight     float64
	DefaultEdgeWeight float64

	// Time constraints
	NodeTTL           time.Duration
	EdgeTTL           time.Duration
	SessionTimeout    time.Duration
	ConnectionTimeout time.Duration

	// Validation settings
	AllowEmptyContent     bool
	RequireUniqueNodeTitles bool
	AllowSelfConnections   bool
	AllowDuplicateEdges    bool

	// Feature flags
	EnableAutoTagging     bool
	EnableSemanticSearch  bool
	EnableGraphAnalytics  bool
	EnableRealTimeSync    bool
}

// DefaultDomainConfig returns the default domain configuration
func DefaultDomainConfig() *DomainConfig {
	return &DomainConfig{
		// Graph constraints
		MaxNodesPerGraph: 10000,
		MaxEdgesPerGraph: 50000,
		DefaultGraphName: "Default Graph",
		
		// Performance limits
		MaxNodesPerQuery: 1000,
		MaxEdgesPerQuery: 5000,
		MaxSimilarityCalculations: 100,
		SimilarityThreshold: 0.3,

		// Node constraints
		MaxConnectionsPerNode: 50,
		MaxTagsPerNode:        20,
		MaxTitleLength:        200,
		MaxContentLength:      50000,
		MinTitleLength:        1,

		// Edge constraints
		MaxEdgeWeight:     1.0,
		MinEdgeWeight:     0.0,
		DefaultEdgeWeight: 0.5,

		// Time constraints
		NodeTTL:           0, // No expiration by default
		EdgeTTL:           0, // No expiration by default
		SessionTimeout:    24 * time.Hour,
		ConnectionTimeout: 30 * time.Second,

		// Validation settings
		AllowEmptyContent:       false,
		RequireUniqueNodeTitles: false,
		AllowSelfConnections:    false,
		AllowDuplicateEdges:     false,

		// Feature flags
		EnableAutoTagging:    false,
		EnableSemanticSearch: false,
		EnableGraphAnalytics: false,
		EnableRealTimeSync:   true,
	}
}

// ProductionDomainConfig returns production-specific configuration
func ProductionDomainConfig() *DomainConfig {
	config := DefaultDomainConfig()
	
	// More restrictive limits for production
	config.MaxNodesPerGraph = 5000
	config.MaxEdgesPerGraph = 25000
	config.MaxConnectionsPerNode = 30
	config.MaxContentLength = 20000
	
	// Stricter validation
	config.AllowEmptyContent = false
	config.RequireUniqueNodeTitles = true
	
	return config
}

// DevelopmentDomainConfig returns development-specific configuration
func DevelopmentDomainConfig() *DomainConfig {
	config := DefaultDomainConfig()
	
	// More permissive for development
	config.MaxNodesPerGraph = 100000
	config.MaxEdgesPerGraph = 500000
	config.AllowEmptyContent = true
	config.AllowSelfConnections = true
	config.AllowDuplicateEdges = true
	
	// Enable all features for testing
	config.EnableAutoTagging = true
	config.EnableSemanticSearch = true
	config.EnableGraphAnalytics = true
	
	return config
}

// LoadDomainConfig loads domain configuration based on environment
func LoadDomainConfig(environment string) *DomainConfig {
	switch environment {
	case "production":
		return ProductionDomainConfig()
	case "development":
		return DevelopmentDomainConfig()
	default:
		return DefaultDomainConfig()
	}
}

// Validate checks if the configuration is valid
func (c *DomainConfig) Validate() error {
	// Add validation logic here
	return nil
}