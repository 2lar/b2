package config

import (
	"fmt"
	"os"
	"strconv"
)

// EdgeCreationConfig holds configuration for edge creation behavior
type EdgeCreationConfig struct {
	// SyncEdgeLimit is the maximum number of edges to create synchronously
	SyncEdgeLimit int
	// SimilarityThreshold is the minimum similarity score for edge creation
	SimilarityThreshold float64
	// MaxEdgesPerNode is the maximum total edges allowed per node
	MaxEdgesPerNode int
	// AsyncEnabled determines if async edge creation is enabled
	AsyncEnabled bool
}

// Config holds all application configuration
type Config struct {
	// Server configuration
	ServerAddress string
	Environment   string

	// AWS configuration
	AWSRegion     string
	DynamoDBTable string
	IndexName     string // GSI1 - for user-level queries
	GSI2IndexName string // GSI2 - for direct NodeID lookups
	EventBusName  string

	// Lambda configuration
	IsLambda           bool
	LambdaFunctionName string
	ColdStartTimeout   int // milliseconds

	// WebSocket configuration
	WebSocketEndpoint string
	ConnectionsTable  string

	// Logging
	LogLevel string

	// Authentication
	JWTSecret string
	JWTIssuer string

	// Feature flags
	EnableMetrics bool
	EnableTracing bool
	EnableCORS    bool

	// Edge creation configuration
	EdgeCreation EdgeCreationConfig
}

// LoadConfig loads configuration from environment variables
func LoadConfig() (*Config, error) {
	cfg := &Config{
		ServerAddress: getEnv("SERVER_ADDRESS", ":8080"),
		Environment:   getEnv("ENVIRONMENT", "development"),
		AWSRegion:     getEnv("AWS_REGION", "us-west-2"),
		DynamoDBTable: getEnv("TABLE_NAME", getEnv("DYNAMODB_TABLE", "brain2")),
		IndexName:     getEnv("INDEX_NAME", "KeywordIndex"),   // GSI1
		GSI2IndexName: getEnv("GSI2_INDEX_NAME", "EdgeIndex"), // GSI2 - Used for both node and edge lookups
		EventBusName:  getEnv("EVENT_BUS_NAME", "brain2-events"),

		// Lambda configuration
		IsLambda:           getEnvBool("IS_LAMBDA", false),
		LambdaFunctionName: getEnv("AWS_LAMBDA_FUNCTION_NAME", ""),
		ColdStartTimeout:   getEnvInt("COLD_START_TIMEOUT", 3000),

		// WebSocket configuration
		WebSocketEndpoint: getEnv("WEBSOCKET_ENDPOINT", ""),
		ConnectionsTable:  getEnv("CONNECTIONS_TABLE", "brain2-connections"),

		// Authentication
		JWTSecret: getEnv("JWT_SECRET", ""),
		JWTIssuer: getEnv("JWT_ISSUER", "brain2-backend2"),

		// Logging and features
		LogLevel:      getEnv("LOG_LEVEL", "info"),
		EnableMetrics: getEnvBool("ENABLE_METRICS", false),
		EnableTracing: getEnvBool("ENABLE_TRACING", false),
		EnableCORS:    getEnvBool("ENABLE_CORS", true),

		// Edge creation configuration
		EdgeCreation: EdgeCreationConfig{
			SyncEdgeLimit:       getEnvInt("EDGE_SYNC_LIMIT", 20),
			SimilarityThreshold: getEnvFloat("EDGE_SIMILARITY_THRESHOLD", 0.3),
			MaxEdgesPerNode:     getEnvInt("EDGE_MAX_PER_NODE", 100),
			AsyncEnabled:        getEnvBool("EDGE_ASYNC_ENABLED", true),
		},
	}

	// Validate required configuration
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

// Load is an alias for LoadConfig for backwards compatibility
func Load() (*Config, error) {
	return LoadConfig()
}

// Validate checks if all required configuration is present
func (c *Config) Validate() error {
	if c.Environment == "production" {
		if c.JWTSecret == "" {
			return fmt.Errorf("JWT_SECRET is required in production")
		}
		if c.DynamoDBTable == "" {
			return fmt.Errorf("DYNAMODB_TABLE is required")
		}
		if c.EventBusName == "" {
			return fmt.Errorf("EVENT_BUS_NAME is required")
		}
	}

	return nil
}

// IsDevelopment checks if running in development mode
func (c *Config) IsDevelopment() bool {
	return c.Environment == "development"
}

// IsProduction checks if running in production mode
func (c *Config) IsProduction() bool {
	return c.Environment == "production"
}

// getEnv gets an environment variable with a default value
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// getEnvBool gets a boolean environment variable with a default value
func getEnvBool(key string, defaultValue bool) bool {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value == "true" || value == "1" || value == "yes"
}

// getEnvInt gets an integer environment variable with a default value
func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intVal, err := strconv.Atoi(value); err == nil {
			return intVal
		}
	}
	return defaultValue
}

// getEnvFloat gets a float environment variable with a default value
func getEnvFloat(key string, defaultValue float64) float64 {
	if value := os.Getenv(key); value != "" {
		if floatVal, err := strconv.ParseFloat(value, 64); err == nil {
			return floatVal
		}
	}
	return defaultValue
}
