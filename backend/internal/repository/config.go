package repository

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

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

	// Resource management settings
	MaxConnections    int           // Maximum concurrent connections
	ConnectionTimeout time.Duration // Connection acquisition timeout
	OperationTimeout  time.Duration // Default operation timeout

	// Retry settings
	RetryConfig RetryConfig // Retry configuration

	// Rate limiting
	RateLimitPerSecond int // Operations per second limit
	RateLimitBurst     int // Burst capacity for rate limiting

	// Validation settings
	EnableStrictValidation bool // Whether to enable strict input validation
	MaxContentLength       int  // Maximum content length
	MaxKeywordCount        int  // Maximum keywords per node

	// Feature flags
	EnableCaching        bool // Whether to enable caching layer
	EnableMetrics        bool // Whether to enable metrics collection
	EnableIdempotency    bool // Whether to enable idempotency support
	EnableCircuitBreaker bool // Whether to enable circuit breaker

	// Cleanup settings
	EnableAutoCleanup   bool          // Whether to enable automatic cleanup
	CleanupInterval     time.Duration // Cleanup interval
	DataRetentionPeriod time.Duration // Data retention period
}

// ConfigDefaults holds default configuration values
var ConfigDefaults = Config{
	MaxRetries:             3,
	TimeoutMs:              5000,
	BatchSize:              25,
	MaxConnections:         100,
	ConnectionTimeout:      10 * time.Second,
	OperationTimeout:       30 * time.Second,
	RetryConfig:            DefaultRetryConfig(),
	RateLimitPerSecond:     100,
	RateLimitBurst:         10,
	EnableStrictValidation: true,
	MaxContentLength:       10000,
	MaxKeywordCount:        50,
	EnableCaching:          true,
	EnableMetrics:          true,
	EnableIdempotency:      true,
	EnableCircuitBreaker:   true,
	EnableAutoCleanup:      true,
	CleanupInterval:        24 * time.Hour,
	DataRetentionPeriod:    90 * 24 * time.Hour,
}

// Environment variable names for configuration
const (
	EnvTableName            = "REPO_TABLE_NAME"
	EnvIndexName            = "REPO_INDEX_NAME"
	EnvRegion               = "REPO_REGION"
	EnvMaxRetries           = "REPO_MAX_RETRIES"
	EnvTimeoutMs            = "REPO_TIMEOUT_MS"
	EnvBatchSize            = "REPO_BATCH_SIZE"
	EnvMaxConnections       = "REPO_MAX_CONNECTIONS"
	EnvConnectionTimeoutMs  = "REPO_CONNECTION_TIMEOUT_MS"
	EnvOperationTimeoutMs   = "REPO_OPERATION_TIMEOUT_MS"
	EnvRateLimitPerSecond   = "REPO_RATE_LIMIT_PER_SECOND"
	EnvRateLimitBurst       = "REPO_RATE_LIMIT_BURST"
	EnvStrictValidation     = "REPO_STRICT_VALIDATION"
	EnvMaxContentLength     = "REPO_MAX_CONTENT_LENGTH"
	EnvMaxKeywordCount      = "REPO_MAX_KEYWORD_COUNT"
	EnvEnableCaching        = "REPO_ENABLE_CACHING"
	EnvEnableMetrics        = "REPO_ENABLE_METRICS"
	EnvEnableIdempotency    = "REPO_ENABLE_IDEMPOTENCY"
	EnvEnableCircuitBreaker = "REPO_ENABLE_CIRCUIT_BREAKER"
	EnvEnableAutoCleanup    = "REPO_ENABLE_AUTO_CLEANUP"
	EnvCleanupIntervalHours = "REPO_CLEANUP_INTERVAL_HOURS"
	EnvDataRetentionDays    = "REPO_DATA_RETENTION_DAYS"
)

// Validate checks if the configuration has all required fields and valid values.
func (c Config) Validate() error {
	var errors []string

	// Required fields
	if c.TableName == "" {
		errors = append(errors, "TableName is required")
	}
	if c.IndexName == "" {
		errors = append(errors, "IndexName is required")
	}

	// Validate numeric ranges
	if c.MaxRetries < 0 {
		errors = append(errors, "MaxRetries cannot be negative")
	}
	if c.TimeoutMs < 0 {
		errors = append(errors, "TimeoutMs cannot be negative")
	}
	if c.BatchSize < 1 {
		errors = append(errors, "BatchSize must be at least 1")
	}
	if c.MaxConnections < 1 {
		errors = append(errors, "MaxConnections must be at least 1")
	}
	if c.ConnectionTimeout < 0 {
		errors = append(errors, "ConnectionTimeout cannot be negative")
	}
	if c.OperationTimeout < 0 {
		errors = append(errors, "OperationTimeout cannot be negative")
	}
	if c.RateLimitPerSecond < 0 {
		errors = append(errors, "RateLimitPerSecond cannot be negative")
	}
	if c.RateLimitBurst < 0 {
		errors = append(errors, "RateLimitBurst cannot be negative")
	}
	if c.MaxContentLength < 1 {
		errors = append(errors, "MaxContentLength must be at least 1")
	}
	if c.MaxKeywordCount < 1 {
		errors = append(errors, "MaxKeywordCount must be at least 1")
	}
	if c.CleanupInterval < 0 {
		errors = append(errors, "CleanupInterval cannot be negative")
	}
	if c.DataRetentionPeriod < 0 {
		errors = append(errors, "DataRetentionPeriod cannot be negative")
	}

	// Validate logical relationships
	if c.BatchSize > c.MaxConnections {
		errors = append(errors, "BatchSize cannot be greater than MaxConnections")
	}
	if c.RateLimitBurst > c.RateLimitPerSecond {
		errors = append(errors, "RateLimitBurst cannot be greater than RateLimitPerSecond")
	}

	// Validate retry configuration
	if err := c.RetryConfig.Validate(); err != nil {
		errors = append(errors, fmt.Sprintf("RetryConfig validation failed: %v", err))
	}

	if len(errors) > 0 {
		return fmt.Errorf("configuration validation failed: %s", strings.Join(errors, ", "))
	}

	return nil
}

// WithDefaults returns a new Config with default values applied for optional fields.
func (c Config) WithDefaults() Config {
	config := c

	// Apply defaults for optional fields
	if config.MaxRetries == 0 {
		config.MaxRetries = ConfigDefaults.MaxRetries
	}
	if config.TimeoutMs == 0 {
		config.TimeoutMs = ConfigDefaults.TimeoutMs
	}
	if config.BatchSize == 0 {
		config.BatchSize = ConfigDefaults.BatchSize
	}
	if config.MaxConnections == 0 {
		config.MaxConnections = ConfigDefaults.MaxConnections
	}
	if config.ConnectionTimeout == 0 {
		config.ConnectionTimeout = ConfigDefaults.ConnectionTimeout
	}
	if config.OperationTimeout == 0 {
		config.OperationTimeout = ConfigDefaults.OperationTimeout
	}
	if config.RateLimitPerSecond == 0 {
		config.RateLimitPerSecond = ConfigDefaults.RateLimitPerSecond
	}
	if config.RateLimitBurst == 0 {
		config.RateLimitBurst = ConfigDefaults.RateLimitBurst
	}
	if config.MaxContentLength == 0 {
		config.MaxContentLength = ConfigDefaults.MaxContentLength
	}
	if config.MaxKeywordCount == 0 {
		config.MaxKeywordCount = ConfigDefaults.MaxKeywordCount
	}
	if config.CleanupInterval == 0 {
		config.CleanupInterval = ConfigDefaults.CleanupInterval
	}
	if config.DataRetentionPeriod == 0 {
		config.DataRetentionPeriod = ConfigDefaults.DataRetentionPeriod
	}

	// Apply retry config defaults
	if config.RetryConfig.MaxAttempts == 0 {
		config.RetryConfig = DefaultRetryConfig()
	}

	return config
}

// LoadFromEnvironment loads configuration from environment variables
func (c Config) LoadFromEnvironment() Config {
	config := c

	// Load string values
	if val := os.Getenv(EnvTableName); val != "" {
		config.TableName = val
	}
	if val := os.Getenv(EnvIndexName); val != "" {
		config.IndexName = val
	}
	if val := os.Getenv(EnvRegion); val != "" {
		config.Region = val
	}

	// Load integer values
	if val := os.Getenv(EnvMaxRetries); val != "" {
		if parsed, err := strconv.Atoi(val); err == nil {
			config.MaxRetries = parsed
		}
	}
	if val := os.Getenv(EnvTimeoutMs); val != "" {
		if parsed, err := strconv.Atoi(val); err == nil {
			config.TimeoutMs = parsed
		}
	}
	if val := os.Getenv(EnvBatchSize); val != "" {
		if parsed, err := strconv.Atoi(val); err == nil {
			config.BatchSize = parsed
		}
	}
	if val := os.Getenv(EnvMaxConnections); val != "" {
		if parsed, err := strconv.Atoi(val); err == nil {
			config.MaxConnections = parsed
		}
	}
	if val := os.Getenv(EnvConnectionTimeoutMs); val != "" {
		if parsed, err := strconv.Atoi(val); err == nil {
			config.ConnectionTimeout = time.Duration(parsed) * time.Millisecond
		}
	}
	if val := os.Getenv(EnvOperationTimeoutMs); val != "" {
		if parsed, err := strconv.Atoi(val); err == nil {
			config.OperationTimeout = time.Duration(parsed) * time.Millisecond
		}
	}
	if val := os.Getenv(EnvRateLimitPerSecond); val != "" {
		if parsed, err := strconv.Atoi(val); err == nil {
			config.RateLimitPerSecond = parsed
		}
	}
	if val := os.Getenv(EnvRateLimitBurst); val != "" {
		if parsed, err := strconv.Atoi(val); err == nil {
			config.RateLimitBurst = parsed
		}
	}
	if val := os.Getenv(EnvMaxContentLength); val != "" {
		if parsed, err := strconv.Atoi(val); err == nil {
			config.MaxContentLength = parsed
		}
	}
	if val := os.Getenv(EnvMaxKeywordCount); val != "" {
		if parsed, err := strconv.Atoi(val); err == nil {
			config.MaxKeywordCount = parsed
		}
	}
	if val := os.Getenv(EnvCleanupIntervalHours); val != "" {
		if parsed, err := strconv.Atoi(val); err == nil {
			config.CleanupInterval = time.Duration(parsed) * time.Hour
		}
	}
	if val := os.Getenv(EnvDataRetentionDays); val != "" {
		if parsed, err := strconv.Atoi(val); err == nil {
			config.DataRetentionPeriod = time.Duration(parsed) * 24 * time.Hour
		}
	}

	// Load boolean values
	if val := os.Getenv(EnvStrictValidation); val != "" {
		config.EnableStrictValidation = strings.ToLower(val) == "true"
	}
	if val := os.Getenv(EnvEnableCaching); val != "" {
		config.EnableCaching = strings.ToLower(val) == "true"
	}
	if val := os.Getenv(EnvEnableMetrics); val != "" {
		config.EnableMetrics = strings.ToLower(val) == "true"
	}
	if val := os.Getenv(EnvEnableIdempotency); val != "" {
		config.EnableIdempotency = strings.ToLower(val) == "true"
	}
	if val := os.Getenv(EnvEnableCircuitBreaker); val != "" {
		config.EnableCircuitBreaker = strings.ToLower(val) == "true"
	}
	if val := os.Getenv(EnvEnableAutoCleanup); val != "" {
		config.EnableAutoCleanup = strings.ToLower(val) == "true"
	}

	return config
}

// Validate method for RetryConfig
func (rc RetryConfig) Validate() error {
	if rc.MaxAttempts < 1 {
		return fmt.Errorf("MaxAttempts must be at least 1")
	}
	if rc.BaseDelay < 0 {
		return fmt.Errorf("BaseDelay cannot be negative")
	}
	if rc.MaxDelay < 0 {
		return fmt.Errorf("MaxDelay cannot be negative")
	}
	if rc.BackoffFactor < 1.0 {
		return fmt.Errorf("BackoffFactor must be at least 1.0")
	}
	if rc.JitterFactor < 0 || rc.JitterFactor > 1.0 {
		return fmt.Errorf("JitterFactor must be between 0 and 1.0")
	}
	if rc.MaxDelay < rc.BaseDelay {
		return fmt.Errorf("MaxDelay must be greater than or equal to BaseDelay")
	}
	return nil
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

// NewConfigFromEnvironment creates a new configuration from environment variables
func NewConfigFromEnvironment() Config {
	config := ConfigDefaults
	return config.LoadFromEnvironment()
}

// ToMap converts the configuration to a map for debugging/logging
func (c Config) ToMap() map[string]interface{} {
	return map[string]interface{}{
		"table_name":               c.TableName,
		"index_name":               c.IndexName,
		"region":                   c.Region,
		"max_retries":              c.MaxRetries,
		"timeout_ms":               c.TimeoutMs,
		"batch_size":               c.BatchSize,
		"max_connections":          c.MaxConnections,
		"connection_timeout":       c.ConnectionTimeout.String(),
		"operation_timeout":        c.OperationTimeout.String(),
		"rate_limit_per_second":    c.RateLimitPerSecond,
		"rate_limit_burst":         c.RateLimitBurst,
		"enable_strict_validation": c.EnableStrictValidation,
		"max_content_length":       c.MaxContentLength,
		"max_keyword_count":        c.MaxKeywordCount,
		"enable_caching":           c.EnableCaching,
		"enable_metrics":           c.EnableMetrics,
		"enable_idempotency":       c.EnableIdempotency,
		"enable_circuit_breaker":   c.EnableCircuitBreaker,
		"enable_auto_cleanup":      c.EnableAutoCleanup,
		"cleanup_interval":         c.CleanupInterval.String(),
		"data_retention_period":    c.DataRetentionPeriod.String(),
	}
}
