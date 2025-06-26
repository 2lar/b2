package repository

import "fmt"

// Config represents the configuration needed for repository implementations.
type Config struct {
	// Database connection settings
	TableName string // Primary table name for data storage
	IndexName string // Secondary index name for queries
	Region    string // Database region (for cloud databases)

	// Performance settings
	MaxRetries int // Maximum number of retry attempts
	TimeoutMs  int // Query timeout in milliseconds
	BatchSize  int // Maximum batch size for bulk operations

	// Feature flags
	EnableCaching bool // Whether to enable caching layer
	EnableMetrics bool // Whether to enable metrics collection
}

// Validate checks if the configuration has all required fields and valid values.
func (c Config) Validate() error {
	if c.TableName == "" {
		return fmt.Errorf("TableName is required")
	}
	if c.IndexName == "" {
		return fmt.Errorf("IndexName is required")
	}
	if c.MaxRetries < 0 {
		return fmt.Errorf("MaxRetries cannot be negative")
	}
	if c.TimeoutMs < 0 {
		return fmt.Errorf("TimeoutMs cannot be negative")
	}
	if c.BatchSize < 1 {
		return fmt.Errorf("BatchSize must be at least 1")
	}
	return nil
}

// WithDefaults returns a new Config with default values applied for optional fields.
func (c Config) WithDefaults() Config {
	config := c

	// Apply defaults for optional fields
	if config.MaxRetries == 0 {
		config.MaxRetries = 3
	}
	if config.TimeoutMs == 0 {
		config.TimeoutMs = 5000 // 5 seconds
	}
	if config.BatchSize == 0 {
		config.BatchSize = 25
	}

	return config
}

// NewConfig creates a new repository configuration with required fields.
func NewConfig(tableName, indexName string) Config {
	return Config{
		TableName: tableName,
		IndexName: indexName,
	}.WithDefaults()
}

// NewConfigWithRegion creates a new repository configuration with region.
func NewConfigWithRegion(tableName, indexName, region string) Config {
	return Config{
		TableName: tableName,
		IndexName: indexName,
		Region:    region,
	}.WithDefaults()
}
