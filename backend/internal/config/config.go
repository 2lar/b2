// Package config provides comprehensive configuration management for the Brain2 application.
// This demonstrates best practices for configuration including:
//   - Environment-specific settings
//   - Validation with struct tags
//   - Feature flags for gradual rollout
//   - Sensible defaults with overrides
//   - Type safety and documentation
package config

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/go-playground/validator/v10"
)

// ============================================================================
// MAIN CONFIGURATION STRUCTURE
// ============================================================================

// Config represents the complete application configuration.
// This structure demonstrates clean configuration architecture with:
//   - Logical grouping of related settings
//   - Clear type definitions
//   - Comprehensive documentation
//   - Validation support
type Config struct {
	Environment    Environment    `yaml:"environment" json:"environment" validate:"required,oneof=development staging production"`
	Server         Server         `yaml:"server" json:"server" validate:"required,dive"`
	Database       Database       `yaml:"database" json:"database" validate:"required,dive"`
	AWS            AWS            `yaml:"aws" json:"aws" validate:"required,dive"`
	Domain         Domain         `yaml:"domain" json:"domain" validate:"required,dive"`
	Infrastructure Infrastructure `yaml:"infrastructure" json:"infrastructure" validate:"required,dive"`
	Features       Features       `yaml:"features" json:"features"` // Feature flags
	Cache          Cache          `yaml:"cache" json:"cache" validate:"dive"`
	Metrics        Metrics        `yaml:"metrics" json:"metrics" validate:"dive"`
	Logging        Logging        `yaml:"logging" json:"logging" validate:"dive"`
	Security       Security       `yaml:"security" json:"security" validate:"required,dive"`
	RateLimit      RateLimit      `yaml:"rate_limit" json:"rate_limit" validate:"dive"`
	CORS           CORS           `yaml:"cors" json:"cors" validate:"dive"`
	Tracing        Tracing        `yaml:"tracing" json:"tracing" validate:"dive"`
	Events         Events         `yaml:"events" json:"events" validate:"dive"`
	Concurrency    Concurrency    `yaml:"concurrency" json:"concurrency" validate:"dive"`
	
	// Metadata fields
	Version        string         `yaml:"version" json:"version"` // Configuration version
	LoadedFrom     []string       `yaml:"-" json:"-"`             // Sources configuration was loaded from
	
}

// Environment represents the deployment environment.
type Environment string

const (
	Development Environment = "development"
	Staging     Environment = "staging"
	Production  Environment = "production"
)

// ============================================================================
// SERVER CONFIGURATION
// ============================================================================

// Server contains HTTP server configuration.
type Server struct {
	Port            int           `yaml:"port" json:"port" validate:"required,min=1,max=65535"`
	Host            string        `yaml:"host" json:"host" validate:"required,hostname|ip"`
	ReadTimeout     time.Duration `yaml:"read_timeout" json:"read_timeout" validate:"required,min=1s"`
	WriteTimeout    time.Duration `yaml:"write_timeout" json:"write_timeout" validate:"required,min=1s"`
	IdleTimeout     time.Duration `yaml:"idle_timeout" json:"idle_timeout" validate:"required,min=1s"`
	ShutdownTimeout time.Duration `yaml:"shutdown_timeout" json:"shutdown_timeout" validate:"required,min=1s"`
	MaxRequestSize  int64         `yaml:"max_request_size" json:"max_request_size" validate:"required,min=1024"`
	RequestTimeout  time.Duration `yaml:"request_timeout" json:"request_timeout" validate:"required,min=1s"`
	EnableHTTPS     bool          `yaml:"enable_https" json:"enable_https"`
	CertFile        string        `yaml:"cert_file" json:"cert_file" validate:"required_if=EnableHTTPS true,omitempty,file"`
	KeyFile         string        `yaml:"key_file" json:"key_file" validate:"required_if=EnableHTTPS true,omitempty,file"`
}

// ============================================================================
// DATABASE CONFIGURATION
// ============================================================================

// Database contains DynamoDB configuration.
type Database struct {
	TableName       string        `yaml:"table_name" json:"table_name" validate:"required,min=3,max=255"`
	IndexName       string        `yaml:"index_name" json:"index_name" validate:"required,min=3,max=255"`
	Region          string        `yaml:"region" json:"region" validate:"required,aws_region"`
	MaxRetries      int           `yaml:"max_retries" json:"max_retries" validate:"min=0,max=10"`
	RetryBaseDelay  time.Duration `yaml:"retry_base_delay" json:"retry_base_delay" validate:"min=10ms"`
	ConnectionPool  int           `yaml:"connection_pool" json:"connection_pool" validate:"min=1,max=100"`
	Timeout         time.Duration `yaml:"timeout" json:"timeout" validate:"min=1s,max=5m"`
	ReadCapacity    int64         `yaml:"read_capacity" json:"read_capacity" validate:"min=1,max=40000"`
	WriteCapacity   int64         `yaml:"write_capacity" json:"write_capacity" validate:"min=1,max=40000"`
	EnableBackups   bool          `yaml:"enable_backups" json:"enable_backups"`
	EnableStreams   bool          `yaml:"enable_streams" json:"enable_streams"`
}

// ============================================================================
// AWS CONFIGURATION
// ============================================================================

// AWS contains AWS service configuration.
type AWS struct {
	Region          string `yaml:"region" json:"region" validate:"required,aws_region"`
	Profile         string `yaml:"profile" json:"profile" validate:"omitempty,min=1"`
	Endpoint        string `yaml:"endpoint" json:"endpoint" validate:"omitempty,url"` // For local development with LocalStack
	AccessKeyID     string `yaml:"access_key_id" json:"access_key_id" validate:"omitempty,min=16"` // Optional, uses IAM role if not provided
	SecretAccessKey string `yaml:"secret_access_key" json:"secret_access_key" validate:"omitempty,min=16"` // Optional, uses IAM role if not provided
	SessionToken    string `yaml:"session_token" json:"session_token" validate:"omitempty"` // For temporary credentials
}

// ============================================================================
// DOMAIN CONFIGURATION
// ============================================================================

// Domain contains business logic configuration.
type Domain struct {
	SimilarityThreshold   float64 `yaml:"similarity_threshold" json:"similarity_threshold" validate:"min=0,max=1"`
	MaxConnectionsPerNode int     `yaml:"max_connections_per_node" json:"max_connections_per_node" validate:"min=1,max=1000"`
	MaxContentLength      int     `yaml:"max_content_length" json:"max_content_length" validate:"min=100,max=100000"`
	DocumentThreshold     int     `yaml:"document_threshold" json:"document_threshold" validate:"min=100,max=10000"`
	DocumentAutoOpen      int     `yaml:"document_auto_open" json:"document_auto_open" validate:"min=500,max=15000"`
	MinKeywordLength      int     `yaml:"min_keyword_length" json:"min_keyword_length" validate:"min=2,max=50"`
	RecencyWeight         float64 `yaml:"recency_weight" json:"recency_weight" validate:"min=0,max=1"`
	DiversityThreshold    float64 `yaml:"diversity_threshold" json:"diversity_threshold" validate:"min=0,max=1"`
	MaxTagsPerNode        int     `yaml:"max_tags_per_node" json:"max_tags_per_node" validate:"min=1,max=100"`
	MaxNodesPerUser       int     `yaml:"max_nodes_per_user" json:"max_nodes_per_user" validate:"min=1,max=1000000"`
}

// ============================================================================
// INFRASTRUCTURE CONFIGURATION
// ============================================================================

// Infrastructure contains infrastructure-level settings.
type Infrastructure struct {
	RetryConfig           RetryConfig           `yaml:"retry" json:"retry" validate:"dive"`
	CircuitBreakerConfig  CircuitBreakerConfig  `yaml:"circuit_breaker" json:"circuit_breaker" validate:"dive"`
	// IdempotencyTTL controls how long idempotency records are retained in DynamoDB.
	// These records prevent duplicate processing of requests with the same idempotency key.
	// After this duration, records expire via DynamoDB TTL and identical requests will be processed as new.
	// Can be configured via:
	//   - Environment variable: IDEMPOTENCY_TTL (e.g., "24h", "7d", "1h")
	//   - Config file: infrastructure.idempotency_ttl
	// Default: 24h (24 hours)
	// Valid range: 1h to 168h (1 hour to 7 days)
	IdempotencyTTL        time.Duration         `yaml:"idempotency_ttl" json:"idempotency_ttl" validate:"min=1h,max=168h"`
	HealthCheckInterval   time.Duration         `yaml:"health_check_interval" json:"health_check_interval" validate:"min=10s,max=5m"`
	GracefulShutdownDelay time.Duration         `yaml:"graceful_shutdown_delay" json:"graceful_shutdown_delay" validate:"min=0,max=60s"`
}

// RetryConfig contains retry behavior settings.
type RetryConfig struct {
	MaxRetries     int           `yaml:"max_retries" json:"max_retries" validate:"min=0,max=10"`
	InitialDelay   time.Duration `yaml:"initial_delay" json:"initial_delay" validate:"min=10ms,max=10s"`
	MaxDelay       time.Duration `yaml:"max_delay" json:"max_delay" validate:"min=100ms,max=60s"`
	BackoffFactor  float64       `yaml:"backoff_factor" json:"backoff_factor" validate:"min=1,max=10"`
	JitterFactor   float64       `yaml:"jitter_factor" json:"jitter_factor" validate:"min=0,max=1"`
	RetryOnTimeout bool          `yaml:"retry_on_timeout" json:"retry_on_timeout"`
	RetryOn5xx     bool          `yaml:"retry_on_5xx" json:"retry_on_5xx"`
}

// CircuitBreakerConfig contains circuit breaker settings.
type CircuitBreakerConfig struct {
	FailureThreshold float64       `yaml:"failure_threshold" json:"failure_threshold" validate:"min=0,max=1"`
	SuccessThreshold float64       `yaml:"success_threshold" json:"success_threshold" validate:"min=0,max=1"`
	MinimumRequests  int           `yaml:"minimum_requests" json:"minimum_requests" validate:"min=1,max=1000"`
	WindowSize       time.Duration `yaml:"window_size" json:"window_size" validate:"min=1s,max=5m"`
	OpenDuration     time.Duration `yaml:"open_duration" json:"open_duration" validate:"min=1s,max=5m"`
	HalfOpenRequests int           `yaml:"half_open_requests" json:"half_open_requests" validate:"min=1,max=100"`
}

// ============================================================================
// FEATURE FLAGS
// ============================================================================

// Features contains feature flags for gradual rollout and A/B testing.
type Features struct {
	// Core features
	EnableCaching        bool `yaml:"enable_caching" json:"enable_caching"`
	EnableAutoConnect    bool `yaml:"enable_auto_connect" json:"enable_auto_connect"`
	EnableAIProcessing   bool `yaml:"enable_ai_processing" json:"enable_ai_processing"`
	EnableMetrics        bool `yaml:"enable_metrics" json:"enable_metrics"`
	EnableTracing        bool `yaml:"enable_tracing" json:"enable_tracing"`
	EnableEventBus       bool `yaml:"enable_event_bus" json:"enable_event_bus"`
	
	// Infrastructure features
	EnableRetries        bool `yaml:"enable_retries" json:"enable_retries"`
	EnableCircuitBreaker bool `yaml:"enable_circuit_breaker" json:"enable_circuit_breaker"`
	EnableRateLimiting   bool `yaml:"enable_rate_limiting" json:"enable_rate_limiting"`
	EnableCompression    bool `yaml:"enable_compression" json:"enable_compression"`
	
	// Debugging features
	EnableDebugEndpoints bool `yaml:"enable_debug_endpoints" json:"enable_debug_endpoints"`
	EnableProfiling      bool `yaml:"enable_profiling" json:"enable_profiling"`
	EnableLogging        bool `yaml:"enable_logging" json:"enable_logging"`
	VerboseLogging       bool `yaml:"verbose_logging" json:"verbose_logging"`
	
	// Experimental features
	EnableGraphQL        bool `yaml:"enable_graphql" json:"enable_graphql"`
	EnableWebSockets     bool `yaml:"enable_websockets" json:"enable_websockets"`
	EnableBatchAPI       bool `yaml:"enable_batch_api" json:"enable_batch_api"`
}

// ============================================================================
// CACHE CONFIGURATION
// ============================================================================

// Cache contains caching configuration.
type Cache struct {
	Provider  string        `yaml:"provider" json:"provider" validate:"oneof=memory redis memcached"`
	MaxItems  int           `yaml:"max_items" json:"max_items" validate:"min=1,max=1000000"`
	TTL       time.Duration `yaml:"ttl" json:"ttl" validate:"min=1s,max=24h"`
	QueryTTL  time.Duration `yaml:"query_ttl" json:"query_ttl" validate:"min=1s,max=24h"`
	
	// Redis-specific settings
	Redis RedisConfig `yaml:"redis" json:"redis" validate:"dive"`
}

// RedisConfig contains Redis-specific settings.
type RedisConfig struct {
	Host     string `yaml:"host" json:"host" validate:"omitempty,hostname|ip"`
	Port     int    `yaml:"port" json:"port" validate:"omitempty,min=1,max=65535"`
	Password string `yaml:"password" json:"password" validate:"omitempty"`
	DB       int    `yaml:"db" json:"db" validate:"min=0,max=15"`
	PoolSize int    `yaml:"pool_size" json:"pool_size" validate:"min=1,max=1000"`
}

// ============================================================================
// METRICS CONFIGURATION
// ============================================================================

// Metrics contains metrics collection configuration.
type Metrics struct {
	Provider   string           `yaml:"provider" json:"provider" validate:"oneof=prometheus datadog cloudwatch statsd"`
	Interval   time.Duration    `yaml:"interval" json:"interval" validate:"omitempty,min=1s,max=5m"`
	Namespace  string           `yaml:"namespace" json:"namespace" validate:"omitempty,min=1,max=255"`
	Prometheus PrometheusConfig `yaml:"prometheus" json:"prometheus" validate:"dive"`
	Datadog    DatadogConfig    `yaml:"datadog" json:"datadog" validate:"dive"`
}

// PrometheusConfig contains Prometheus-specific settings.
type PrometheusConfig struct {
	Port int    `yaml:"port" json:"port" validate:"min=1,max=65535"`
	Path string `yaml:"path" json:"path" validate:"omitempty,startswith=/"`
}

// DatadogConfig contains Datadog-specific settings.
type DatadogConfig struct {
	APIKey string `yaml:"api_key" json:"api_key" validate:"omitempty,min=32"`
	Host   string `yaml:"host" json:"host" validate:"omitempty,hostname|ip"`
	Port   int    `yaml:"port" json:"port" validate:"omitempty,min=1,max=65535"`
}

// ============================================================================
// LOGGING CONFIGURATION
// ============================================================================

// Logging contains logging configuration.
type Logging struct {
	Level      string `validate:"oneof=debug info warn error fatal"`
	Format     string `validate:"oneof=json console"`
	Output     string `validate:"oneof=stdout stderr file"`
	FilePath   string `validate:"required_if=Output file"`
	MaxSize    int    // MB
	MaxAge     int    // Days
	MaxBackups int
	Compress   bool
}

// ============================================================================
// SECURITY CONFIGURATION
// ============================================================================

// Security contains security-related settings.
type Security struct {
	JWTSecret       string `validate:"required,min=32"`
	JWTExpiry       time.Duration
	APIKeyHeader    string
	EnableAuth      bool
	AllowedOrigins  []string
	TrustedProxies  []string
	SecureHeaders   bool
	EnableCSRF      bool
	CSRFTokenLength int `validate:"min=16,max=256"`
}

// ============================================================================
// RATE LIMITING CONFIGURATION
// ============================================================================

// RateLimit contains rate limiting configuration.
type RateLimit struct {
	Enabled       bool
	RequestsPerMinute int `validate:"min=1"`
	Burst         int `validate:"min=1"`
	CleanupInterval time.Duration
	ByIP          bool
	ByUser        bool
	ByAPIKey      bool
}

// ============================================================================
// CORS CONFIGURATION
// ============================================================================

// CORS contains CORS configuration.
type CORS struct {
	Enabled          bool
	AllowedOrigins   []string
	AllowedMethods   []string
	AllowedHeaders   []string
	ExposedHeaders   []string
	AllowCredentials bool
	MaxAge           int
}

// ============================================================================
// TRACING CONFIGURATION
// ============================================================================

// Tracing contains distributed tracing configuration.
type Tracing struct {
	Enabled      bool    `yaml:"enabled" json:"enabled"`
	Provider     string  `yaml:"provider" json:"provider" validate:"omitempty,oneof=jaeger xray otlp"`
	ServiceName  string  `yaml:"service_name" json:"service_name"`
	AgentHost    string  `yaml:"agent_host" json:"agent_host"`
	AgentPort    int     `yaml:"agent_port" json:"agent_port"`
	Endpoint     string  `yaml:"endpoint" json:"endpoint"` // For OTLP
	SampleRate   float64 `yaml:"sample_rate" json:"sample_rate" validate:"min=0,max=1"`
}

// ============================================================================
// EVENTS CONFIGURATION
// ============================================================================

// Events contains event bus configuration.
type Events struct {
	Provider      string `yaml:"provider" json:"provider" validate:"oneof=eventbridge kafka rabbitmq sns"`
	EventBusName  string `yaml:"event_bus_name" json:"event_bus_name" validate:"omitempty,min=1,max=255"`
	TopicPrefix   string `yaml:"topic_prefix" json:"topic_prefix" validate:"omitempty,min=1,max=255"`
	RetryAttempts int    `yaml:"retry_attempts" json:"retry_attempts" validate:"min=0,max=10"`
	BatchSize     int    `yaml:"batch_size" json:"batch_size" validate:"min=1,max=1000"`
}

// ============================================================================
// CONCURRENCY CONFIGURATION
// ============================================================================

// Concurrency contains environment-aware concurrency settings.
type Concurrency struct {
	// Lambda-specific settings
	Lambda LambdaConcurrency `yaml:"lambda" json:"lambda" validate:"dive"`
	
	// ECS/Fargate-specific settings
	ECS ECSConcurrency `yaml:"ecs" json:"ecs" validate:"dive"`
	
	// Local development settings
	Local LocalConcurrency `yaml:"local" json:"local" validate:"dive"`
	
	// Auto-detection settings
	AutoDetect bool `yaml:"auto_detect" json:"auto_detect"` // Auto-detect environment
	ForceMode  string `yaml:"force_mode" json:"force_mode" validate:"omitempty,oneof=lambda ecs local"` // Force specific mode
}

// LambdaConcurrency contains Lambda-specific concurrency settings.
type LambdaConcurrency struct {
	MaxWorkers       int           `yaml:"max_workers" json:"max_workers" validate:"min=1,max=10"`
	BatchSize        int           `yaml:"batch_size" json:"batch_size" validate:"min=1,max=100"`
	QueueSize        int           `yaml:"queue_size" json:"queue_size" validate:"min=10,max=500"`
	TimeoutBuffer    time.Duration `yaml:"timeout_buffer" json:"timeout_buffer" validate:"min=5s,max=60s"`
	ChunkSize        int           `yaml:"chunk_size" json:"chunk_size" validate:"min=5,max=50"`
	MaxConnections   int           `yaml:"max_connections" json:"max_connections" validate:"min=1,max=5"`
}

// ECSConcurrency contains ECS/Fargate-specific concurrency settings.
type ECSConcurrency struct {
	MaxWorkers       int           `yaml:"max_workers" json:"max_workers" validate:"min=1,max=100"`
	BatchSize        int           `yaml:"batch_size" json:"batch_size" validate:"min=10,max=500"`
	QueueSize        int           `yaml:"queue_size" json:"queue_size" validate:"min=100,max=5000"`
	ConnectionPool   int           `yaml:"connection_pool" json:"connection_pool" validate:"min=5,max=50"`
	MaxConnections   int           `yaml:"max_connections" json:"max_connections" validate:"min=5,max=50"`
	WorkerMultiplier int           `yaml:"worker_multiplier" json:"worker_multiplier" validate:"min=1,max=10"` // Workers per CPU
}

// LocalConcurrency contains local development concurrency settings.
type LocalConcurrency struct {
	MaxWorkers       int           `yaml:"max_workers" json:"max_workers" validate:"min=1,max=50"`
	BatchSize        int           `yaml:"batch_size" json:"batch_size" validate:"min=5,max=200"`
	QueueSize        int           `yaml:"queue_size" json:"queue_size" validate:"min=50,max=2000"`
	ConnectionPool   int           `yaml:"connection_pool" json:"connection_pool" validate:"min=2,max=20"`
	MaxConnections   int           `yaml:"max_connections" json:"max_connections" validate:"min=2,max=20"`
}

// ============================================================================
// CONFIGURATION LOADING
// ============================================================================

// LoadConfig loads configuration from environment variables with validation.
func LoadConfig() Config {
	cfg := Config{
		Environment: getEnvironment(),
		Server:      loadServerConfig(),
		Database:    loadDatabaseConfig(),
		AWS:         loadAWSConfig(),
		Domain:      loadDomainConfig(),
		Infrastructure: loadInfrastructureConfig(),
		Features:    loadFeatures(),
		Cache:       loadCacheConfig(),
		Metrics:     loadMetricsConfig(),
		Logging:     loadLoggingConfig(),
		Security:    loadSecurityConfig(),
		RateLimit:   loadRateLimitConfig(),
		CORS:        loadCORSConfig(),
		Tracing:     loadTracingConfig(),
		Events:      loadEventsConfig(),
		Concurrency: loadConcurrencyConfig(),
	}
	
	
	// Apply environment-specific defaults
	cfg.applyEnvironmentDefaults()
	
	return cfg
}

// Validate validates the configuration using struct tags and custom rules.
func (c *Config) Validate() error {
	// Use struct tag validation
	validate := validator.New()
	
	// Register custom validation for AWS regions
	validate.RegisterValidation("aws_region", validateAWSRegion)
	
	// Validate struct tags
	if err := validate.Struct(c); err != nil {
		// Format validation errors nicely
		if validationErrors, ok := err.(validator.ValidationErrors); ok {
			var errors []string
			for _, e := range validationErrors {
				errors = append(errors, formatValidationError(e))
			}
			return fmt.Errorf("validation failed:\n  - %s", strings.Join(errors, "\n  - "))
		}
		return fmt.Errorf("validation failed: %w", err)
	}
	
	// Custom business logic validation
	if err := c.validateBusinessRules(); err != nil {
		return fmt.Errorf("business rule validation failed: %w", err)
	}
	
	// Environment-specific validation
	if err := c.validateEnvironmentRules(); err != nil {
		return fmt.Errorf("environment validation failed: %w", err)
	}
	
	return nil
}

// validateBusinessRules checks custom business logic constraints.
func (c *Config) validateBusinessRules() error {
	// Ensure retry max delay is greater than initial delay
	if c.Infrastructure.RetryConfig.MaxDelay <= c.Infrastructure.RetryConfig.InitialDelay {
		return fmt.Errorf("retry max delay must be greater than initial delay")
	}
	
	// Ensure cache TTL is reasonable compared to query TTL
	if c.Cache.QueryTTL > c.Cache.TTL {
		return fmt.Errorf("cache query TTL cannot be greater than general TTL")
	}
	
	// Ensure circuit breaker thresholds are sensible
	if c.Infrastructure.CircuitBreakerConfig.SuccessThreshold <= c.Infrastructure.CircuitBreakerConfig.FailureThreshold {
		return fmt.Errorf("circuit breaker success threshold must be greater than failure threshold")
	}
	
	// Validate Redis configuration if Redis is selected
	if c.Cache.Provider == "redis" {
		if c.Cache.Redis.Host == "" {
			return fmt.Errorf("redis host is required when cache provider is redis")
		}
		if c.Cache.Redis.Port == 0 {
			return fmt.Errorf("redis port is required when cache provider is redis")
		}
	}
	
	return nil
}

// validateEnvironmentRules enforces environment-specific constraints.
func (c *Config) validateEnvironmentRules() error {
	switch c.Environment {
	case Production:
		// Production must have certain features enabled
		if !c.Features.EnableMetrics {
			return fmt.Errorf("metrics must be enabled in production")
		}
		if !c.Security.EnableAuth {
			return fmt.Errorf("authentication must be enabled in production")
		}
		if c.Logging.Level == "debug" {
			return fmt.Errorf("debug logging should not be used in production")
		}
		if !c.Security.SecureHeaders {
			return fmt.Errorf("secure headers must be enabled in production")
		}
		if c.Server.Port == 8080 {
			return fmt.Errorf("default port 8080 should not be used in production")
		}
		
	case Staging:
		// Staging should have metrics enabled for testing
		if !c.Features.EnableMetrics {
			return fmt.Errorf("metrics should be enabled in staging for testing")
		}
		
	case Development:
		// Development warnings (not errors)
		if c.Features.EnableDebugEndpoints && c.Security.EnableAuth {
			// This is just a warning, logged elsewhere
		}
	}
	
	return nil
}

// validateAWSRegion is a custom validator for AWS region format.
func validateAWSRegion(fl validator.FieldLevel) bool {
	region := fl.Field().String()
	// Simple AWS region pattern validation
	// Format: us-east-1, eu-west-2, ap-southeast-1, etc.
	if region == "" {
		return false
	}
	
	parts := strings.Split(region, "-")
	if len(parts) < 3 {
		return false
	}
	
	// Check if it matches known region patterns
	validPrefixes := []string{"us", "eu", "ap", "ca", "sa", "me", "af"}
	for _, prefix := range validPrefixes {
		if strings.HasPrefix(region, prefix+"-") {
			return true
		}
	}
	
	return false
}

// formatValidationError formats a validation error for better readability.
func formatValidationError(e validator.FieldError) string {
	field := e.Namespace()
	tag := e.Tag()
	param := e.Param()
	
	switch tag {
	case "required":
		return fmt.Sprintf("%s is required", field)
	case "min":
		return fmt.Sprintf("%s must be at least %s", field, param)
	case "max":
		return fmt.Sprintf("%s must be at most %s", field, param)
	case "oneof":
		return fmt.Sprintf("%s must be one of: %s", field, param)
	case "required_if":
		return fmt.Sprintf("%s is required when %s", field, param)
	case "aws_region":
		return fmt.Sprintf("%s must be a valid AWS region (e.g., us-east-1)", field)
	default:
		return fmt.Sprintf("%s failed %s validation", field, tag)
	}
}

// applyEnvironmentDefaults applies environment-specific defaults.
func (c *Config) applyEnvironmentDefaults() {
	switch c.Environment {
	case Production:
		// Production defaults
		if c.Features.EnableLogging {
			c.Logging.Level = "info"
		}
		c.Features.EnableMetrics = true
		c.Features.EnableCircuitBreaker = true
		c.Features.EnableRetries = true
		c.Security.SecureHeaders = true
		
	case Development:
		// Development defaults
		c.Logging.Level = "debug"
		c.Features.EnableDebugEndpoints = true
		c.Features.VerboseLogging = true
		
	case Staging:
		// Staging defaults
		c.Features.EnableMetrics = true
		c.Logging.Level = "info"
	}
}

// ============================================================================
// HELPER FUNCTIONS FOR LOADING CONFIGURATION
// ============================================================================

func getEnvironment() Environment {
	env := os.Getenv("ENVIRONMENT")
	if env == "" {
		env = os.Getenv("ENV")
	}
	if env == "" {
		env = "development"
	}
	
	switch strings.ToLower(env) {
	case "production", "prod":
		return Production
	case "staging", "stage":
		return Staging
	default:
		return Development
	}
}

func loadServerConfig() Server {
	return Server{
		Port:            getEnvInt("SERVER_PORT", 8080),
		Host:            getEnvString("SERVER_HOST", "0.0.0.0"),
		ReadTimeout:     getEnvDuration("SERVER_READ_TIMEOUT", 30*time.Second),
		WriteTimeout:    getEnvDuration("SERVER_WRITE_TIMEOUT", 30*time.Second),
		IdleTimeout:     getEnvDuration("SERVER_IDLE_TIMEOUT", 60*time.Second),
		ShutdownTimeout: getEnvDuration("SERVER_SHUTDOWN_TIMEOUT", 10*time.Second),
		MaxRequestSize:  getEnvInt64("SERVER_MAX_REQUEST_SIZE", 10*1024*1024), // 10MB
		RequestTimeout:  getEnvDuration("SERVER_REQUEST_TIMEOUT", 30*time.Second),
		EnableHTTPS:     getEnvBool("SERVER_ENABLE_HTTPS", false),
		CertFile:        getEnvString("SERVER_CERT_FILE", ""),
		KeyFile:         getEnvString("SERVER_KEY_FILE", ""),
	}
}

func loadDatabaseConfig() Database {
	return Database{
		TableName:      getEnvString("TABLE_NAME", "brain2-dev"),
		IndexName:      getEnvString("INDEX_NAME", "KeywordIndex"),
		Region:         getEnvString("AWS_REGION", "us-east-1"),
		MaxRetries:     getEnvInt("DB_MAX_RETRIES", 3),
		RetryBaseDelay: getEnvDuration("DB_RETRY_BASE_DELAY", 100*time.Millisecond),
		ConnectionPool: getEnvInt("DB_CONNECTION_POOL", 10),
		Timeout:        getEnvDuration("DB_TIMEOUT", 10*time.Second),
		ReadCapacity:   getEnvInt64("DB_READ_CAPACITY", 5),
		WriteCapacity:  getEnvInt64("DB_WRITE_CAPACITY", 5),
		EnableBackups:  getEnvBool("DB_ENABLE_BACKUPS", false),
		EnableStreams:  getEnvBool("DB_ENABLE_STREAMS", false),
	}
}

func loadAWSConfig() AWS {
	return AWS{
		Region:          getEnvString("AWS_REGION", "us-east-1"),
		Profile:         getEnvString("AWS_PROFILE", ""),
		Endpoint:        getEnvString("AWS_ENDPOINT", ""),
		AccessKeyID:     getEnvString("AWS_ACCESS_KEY_ID", ""),
		SecretAccessKey: getEnvString("AWS_SECRET_ACCESS_KEY", ""),
		SessionToken:    getEnvString("AWS_SESSION_TOKEN", ""),
	}
}

func loadDomainConfig() Domain {
	return Domain{
		SimilarityThreshold:   getEnvFloat("DOMAIN_SIMILARITY_THRESHOLD", 0.3),
		MaxConnectionsPerNode: getEnvInt("DOMAIN_MAX_CONNECTIONS", 10),
		MaxContentLength:      getEnvInt("DOMAIN_MAX_CONTENT_LENGTH", 20000),
		DocumentThreshold:     getEnvInt("DOMAIN_DOCUMENT_THRESHOLD", 800),
		DocumentAutoOpen:      getEnvInt("DOMAIN_DOCUMENT_AUTO_OPEN", 1200),
		MinKeywordLength:      getEnvInt("DOMAIN_MIN_KEYWORD_LENGTH", 3),
		RecencyWeight:         getEnvFloat("DOMAIN_RECENCY_WEIGHT", 0.2),
		DiversityThreshold:    getEnvFloat("DOMAIN_DIVERSITY_THRESHOLD", 0.5),
		MaxTagsPerNode:        getEnvInt("DOMAIN_MAX_TAGS", 10),
		MaxNodesPerUser:       getEnvInt("DOMAIN_MAX_NODES_PER_USER", 10000),
	}
}

func loadInfrastructureConfig() Infrastructure {
	return Infrastructure{
		RetryConfig: RetryConfig{
			MaxRetries:     getEnvInt("RETRY_MAX_RETRIES", 3),
			InitialDelay:   getEnvDuration("RETRY_INITIAL_DELAY", 100*time.Millisecond),
			MaxDelay:       getEnvDuration("RETRY_MAX_DELAY", 5*time.Second),
			BackoffFactor:  getEnvFloat("RETRY_BACKOFF_FACTOR", 2.0),
			JitterFactor:   getEnvFloat("RETRY_JITTER_FACTOR", 0.1),
			RetryOnTimeout: getEnvBool("RETRY_ON_TIMEOUT", true),
			RetryOn5xx:     getEnvBool("RETRY_ON_5XX", true),
		},
		CircuitBreakerConfig: CircuitBreakerConfig{
			FailureThreshold: getEnvFloat("CB_FAILURE_THRESHOLD", 0.5),
			SuccessThreshold: getEnvFloat("CB_SUCCESS_THRESHOLD", 0.8),
			MinimumRequests:  getEnvInt("CB_MINIMUM_REQUESTS", 10),
			WindowSize:       getEnvDuration("CB_WINDOW_SIZE", 10*time.Second),
			OpenDuration:     getEnvDuration("CB_OPEN_DURATION", 30*time.Second),
			HalfOpenRequests: getEnvInt("CB_HALF_OPEN_REQUESTS", 3),
		},
		IdempotencyTTL:        getEnvDuration("IDEMPOTENCY_TTL", 24*time.Hour),
		HealthCheckInterval:   getEnvDuration("HEALTH_CHECK_INTERVAL", 30*time.Second),
		GracefulShutdownDelay: getEnvDuration("GRACEFUL_SHUTDOWN_DELAY", 5*time.Second),
	}
}

func loadFeatures() Features {
	return Features{
		EnableCaching:        getEnvBool("ENABLE_CACHING", false),
		EnableAutoConnect:    getEnvBool("ENABLE_AUTO_CONNECT", true),
		EnableAIProcessing:   getEnvBool("ENABLE_AI_PROCESSING", false),
		EnableMetrics:        getEnvBool("ENABLE_METRICS", false),
		EnableTracing:        getEnvBool("ENABLE_TRACING", false),
		EnableEventBus:       getEnvBool("ENABLE_EVENT_BUS", false),
		EnableRetries:        getEnvBool("ENABLE_RETRIES", true),
		EnableCircuitBreaker: getEnvBool("ENABLE_CIRCUIT_BREAKER", false),
		EnableRateLimiting:   getEnvBool("ENABLE_RATE_LIMITING", false),
		EnableCompression:    getEnvBool("ENABLE_COMPRESSION", false),
		EnableDebugEndpoints: getEnvBool("ENABLE_DEBUG_ENDPOINTS", false),
		EnableProfiling:      getEnvBool("ENABLE_PROFILING", false),
		EnableLogging:        getEnvBool("ENABLE_LOGGING", true),
		VerboseLogging:       getEnvBool("VERBOSE_LOGGING", false),
		EnableGraphQL:        getEnvBool("ENABLE_GRAPHQL", false),
		EnableWebSockets:     getEnvBool("ENABLE_WEBSOCKETS", false),
		EnableBatchAPI:       getEnvBool("ENABLE_BATCH_API", false),
	}
}

func loadCacheConfig() Cache {
	return Cache{
		Provider: getEnvString("CACHE_PROVIDER", "memory"),
		MaxItems: getEnvInt("CACHE_MAX_ITEMS", 1000),
		TTL:      getEnvDuration("CACHE_TTL", 5*time.Minute),
		QueryTTL: getEnvDuration("CACHE_QUERY_TTL", 1*time.Minute),
		Redis: RedisConfig{
			Host:     getEnvString("REDIS_HOST", "localhost"),
			Port:     getEnvInt("REDIS_PORT", 6379),
			Password: getEnvString("REDIS_PASSWORD", ""),
			DB:       getEnvInt("REDIS_DB", 0),
			PoolSize: getEnvInt("REDIS_POOL_SIZE", 10),
		},
	}
}

func loadMetricsConfig() Metrics {
	return Metrics{
		Provider:  getEnvString("METRICS_PROVIDER", "prometheus"),
		Interval:  getEnvDuration("METRICS_INTERVAL", 10*time.Second),
		Namespace: getEnvString("METRICS_NAMESPACE", "brain2"),
		Prometheus: PrometheusConfig{
			Port: getEnvInt("PROMETHEUS_PORT", 9090),
			Path: getEnvString("PROMETHEUS_PATH", "/metrics"),
		},
		Datadog: DatadogConfig{
			APIKey: getEnvString("DATADOG_API_KEY", ""),
			Host:   getEnvString("DATADOG_HOST", "localhost"),
			Port:   getEnvInt("DATADOG_PORT", 8125),
		},
	}
}

func loadLoggingConfig() Logging {
	return Logging{
		Level:      getEnvString("LOG_LEVEL", "info"),
		Format:     getEnvString("LOG_FORMAT", "json"),
		Output:     getEnvString("LOG_OUTPUT", "stdout"),
		FilePath:   getEnvString("LOG_FILE_PATH", "/var/log/brain2.log"),
		MaxSize:    getEnvInt("LOG_MAX_SIZE", 100),
		MaxAge:     getEnvInt("LOG_MAX_AGE", 30),
		MaxBackups: getEnvInt("LOG_MAX_BACKUPS", 10),
		Compress:   getEnvBool("LOG_COMPRESS", true),
	}
}

func loadSecurityConfig() Security {
	return Security{
		JWTSecret:       getEnvString("JWT_SECRET", generateDefaultSecret()),
		JWTExpiry:       getEnvDuration("JWT_EXPIRY", 24*time.Hour),
		APIKeyHeader:    getEnvString("API_KEY_HEADER", "X-API-Key"),
		EnableAuth:      getEnvBool("ENABLE_AUTH", true),
		AllowedOrigins:  getEnvStringSlice("ALLOWED_ORIGINS", []string{"*"}),
		TrustedProxies:  getEnvStringSlice("TRUSTED_PROXIES", []string{}),
		SecureHeaders:   getEnvBool("SECURE_HEADERS", true),
		EnableCSRF:      getEnvBool("ENABLE_CSRF", false),
		CSRFTokenLength: getEnvInt("CSRF_TOKEN_LENGTH", 32),
	}
}

func loadRateLimitConfig() RateLimit {
	return RateLimit{
		Enabled:           getEnvBool("RATE_LIMIT_ENABLED", false),
		RequestsPerMinute: getEnvInt("RATE_LIMIT_RPM", 100),
		Burst:             getEnvInt("RATE_LIMIT_BURST", 10),
		CleanupInterval:   getEnvDuration("RATE_LIMIT_CLEANUP", 1*time.Minute),
		ByIP:              getEnvBool("RATE_LIMIT_BY_IP", true),
		ByUser:            getEnvBool("RATE_LIMIT_BY_USER", false),
		ByAPIKey:          getEnvBool("RATE_LIMIT_BY_API_KEY", false),
	}
}

func loadCORSConfig() CORS {
	return CORS{
		Enabled:          getEnvBool("CORS_ENABLED", true),
		AllowedOrigins:   getEnvStringSlice("CORS_ALLOWED_ORIGINS", []string{"*"}),
		AllowedMethods:   getEnvStringSlice("CORS_ALLOWED_METHODS", []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"}),
		AllowedHeaders:   getEnvStringSlice("CORS_ALLOWED_HEADERS", []string{"*"}),
		ExposedHeaders:   getEnvStringSlice("CORS_EXPOSED_HEADERS", []string{}),
		AllowCredentials: getEnvBool("CORS_ALLOW_CREDENTIALS", false),
		MaxAge:           getEnvInt("CORS_MAX_AGE", 86400),
	}
}

func loadTracingConfig() Tracing {
	return Tracing{
		Enabled:     getEnvBool("TRACING_ENABLED", false),
		Provider:    getEnvString("TRACING_PROVIDER", "jaeger"),
		ServiceName: getEnvString("TRACING_SERVICE_NAME", "brain2"),
		SampleRate:  getEnvFloat("TRACING_SAMPLE_RATE", 0.1),
		Endpoint:    getEnvString("TRACING_ENDPOINT", ""),
		AgentHost:   getEnvString("TRACING_AGENT_HOST", "localhost"),
		AgentPort:   getEnvInt("TRACING_AGENT_PORT", 6831),
	}
}

func loadEventsConfig() Events {
	return Events{
		Provider:      getEnvString("EVENTS_PROVIDER", "eventbridge"),
		EventBusName:  getEnvString("EVENT_BUS_NAME", "B2EventBus"),
		TopicPrefix:   getEnvString("EVENT_TOPIC_PREFIX", "brain2"),
		RetryAttempts: getEnvInt("EVENT_RETRY_ATTEMPTS", 3),
		BatchSize:     getEnvInt("EVENT_BATCH_SIZE", 10),
	}
}

func loadConcurrencyConfig() Concurrency {
	config := Concurrency{
		AutoDetect: getEnvBool("CONCURRENCY_AUTO_DETECT", true),
		ForceMode:  getEnvString("CONCURRENCY_FORCE_MODE", ""),
		
		Lambda: LambdaConcurrency{
			MaxWorkers:     getEnvInt("LAMBDA_MAX_WORKERS", 4),
			BatchSize:      getEnvInt("LAMBDA_BATCH_SIZE", 25),
			QueueSize:      getEnvInt("LAMBDA_QUEUE_SIZE", 100),
			TimeoutBuffer:  getEnvDuration("LAMBDA_TIMEOUT_BUFFER", 30*time.Second),
			ChunkSize:      getEnvInt("LAMBDA_CHUNK_SIZE", 25),
			MaxConnections: getEnvInt("LAMBDA_MAX_CONNECTIONS", 2),
		},
		
		ECS: ECSConcurrency{
			MaxWorkers:       getEnvInt("ECS_MAX_WORKERS", 20),
			BatchSize:        getEnvInt("ECS_BATCH_SIZE", 100),
			QueueSize:        getEnvInt("ECS_QUEUE_SIZE", 1000),
			ConnectionPool:   getEnvInt("ECS_CONNECTION_POOL", 10),
			MaxConnections:   getEnvInt("ECS_MAX_CONNECTIONS", 20),
			WorkerMultiplier: getEnvInt("ECS_WORKER_MULTIPLIER", 4),
		},
		
		Local: LocalConcurrency{
			MaxWorkers:     getEnvInt("LOCAL_MAX_WORKERS", 10),
			BatchSize:      getEnvInt("LOCAL_BATCH_SIZE", 50),
			QueueSize:      getEnvInt("LOCAL_QUEUE_SIZE", 500),
			ConnectionPool: getEnvInt("LOCAL_CONNECTION_POOL", 5),
			MaxConnections: getEnvInt("LOCAL_MAX_CONNECTIONS", 10),
		},
	}
	
	// Validate the configuration
	if err := config.Validate(); err != nil {
		log.Printf("Warning: Concurrency configuration validation failed: %v", err)
	}
	
	return config
}

// Validate validates the concurrency configuration
func (c *Concurrency) Validate() error {
	// Validate force mode if set
	if c.ForceMode != "" && c.ForceMode != "lambda" && c.ForceMode != "ecs" && c.ForceMode != "local" {
		return fmt.Errorf("invalid force_mode: %s (must be lambda, ecs, or local)", c.ForceMode)
	}
	
	// Lambda-specific validation
	if c.Lambda.MaxWorkers > 8 {
		return fmt.Errorf("lambda max_workers should not exceed 8 (got %d)", c.Lambda.MaxWorkers)
	}
	if c.Lambda.BatchSize > 25 {
		return fmt.Errorf("lambda batch_size cannot exceed 25 for DynamoDB (got %d)", c.Lambda.BatchSize)
	}
	if c.Lambda.ChunkSize > 25 {
		return fmt.Errorf("lambda chunk_size cannot exceed 25 for DynamoDB (got %d)", c.Lambda.ChunkSize)
	}
	
	// ECS-specific validation
	if c.ECS.MaxWorkers > 100 {
		return fmt.Errorf("ecs max_workers should not exceed 100 (got %d)", c.ECS.MaxWorkers)
	}
	if c.ECS.WorkerMultiplier > 10 {
		return fmt.Errorf("ecs worker_multiplier should not exceed 10 (got %d)", c.ECS.WorkerMultiplier)
	}
	
	// Local development validation
	if c.Local.MaxWorkers > 50 {
		return fmt.Errorf("local max_workers should not exceed 50 (got %d)", c.Local.MaxWorkers)
	}
	
	return nil
}

// ============================================================================
// UTILITY FUNCTIONS
// ============================================================================

func getEnvString(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intVal, err := strconv.Atoi(value); err == nil {
			return intVal
		}
	}
	return defaultValue
}

func getEnvInt64(key string, defaultValue int64) int64 {
	if value := os.Getenv(key); value != "" {
		if intVal, err := strconv.ParseInt(value, 10, 64); err == nil {
			return intVal
		}
	}
	return defaultValue
}

func getEnvFloat(key string, defaultValue float64) float64 {
	if value := os.Getenv(key); value != "" {
		if floatVal, err := strconv.ParseFloat(value, 64); err == nil {
			return floatVal
		}
	}
	return defaultValue
}

func getEnvBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if boolVal, err := strconv.ParseBool(value); err == nil {
			return boolVal
		}
	}
	return defaultValue
}

func getEnvDuration(key string, defaultValue time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		if duration, err := time.ParseDuration(value); err == nil {
			return duration
		}
	}
	return defaultValue
}

func getEnvStringSlice(key string, defaultValue []string) []string {
	if value := os.Getenv(key); value != "" {
		return strings.Split(value, ",")
	}
	return defaultValue
}

func generateDefaultSecret() string {
	// In production, this should be properly generated and stored securely
	return "default-secret-please-change-in-production-environment"
}
