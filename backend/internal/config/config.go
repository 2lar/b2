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
	"os"
	"strconv"
	"strings"
	"time"
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
	Environment    Environment    `validate:"required,oneof=development staging production"`
	Server         Server         `validate:"required"`
	Database       Database       `validate:"required"`
	AWS            AWS            `validate:"required"`
	Domain         Domain         `validate:"required"`
	Infrastructure Infrastructure `validate:"required"`
	Features       Features       // Feature flags
	Cache          Cache          // Cache configuration
	Metrics        Metrics        // Metrics configuration
	Logging        Logging        // Logging configuration
	Security       Security       // Security settings
	RateLimit      RateLimit      // Rate limiting configuration
	CORS           CORS           // CORS configuration
	Tracing        Tracing        // Distributed tracing
	Events         Events         // Event configuration
	
	// Legacy fields for backward compatibility
	TableName string // Deprecated: Use Database.TableName
	IndexName string // Deprecated: Use Database.IndexName
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
	Port            int           `validate:"required,min=1,max=65535"`
	Host            string        `validate:"required"`
	ReadTimeout     time.Duration `validate:"required,min=1s"`
	WriteTimeout    time.Duration `validate:"required,min=1s"`
	IdleTimeout     time.Duration `validate:"required,min=1s"`
	ShutdownTimeout time.Duration `validate:"required,min=1s"`
	MaxRequestSize  int64         `validate:"required,min=1024"`
	RequestTimeout  time.Duration `validate:"required,min=1s"`
	EnableHTTPS     bool
	CertFile        string `validate:"required_if=EnableHTTPS true"`
	KeyFile         string `validate:"required_if=EnableHTTPS true"`
}

// ============================================================================
// DATABASE CONFIGURATION
// ============================================================================

// Database contains DynamoDB configuration.
type Database struct {
	TableName       string        `validate:"required"`
	IndexName       string        `validate:"required"`
	Region          string        `validate:"required"`
	MaxRetries      int           `validate:"min=0,max=10"`
	RetryBaseDelay  time.Duration `validate:"min=10ms"`
	ConnectionPool  int           `validate:"min=1,max=100"`
	Timeout         time.Duration `validate:"min=1s"`
	ReadCapacity    int64         `validate:"min=1"`
	WriteCapacity   int64         `validate:"min=1"`
	EnableBackups   bool
	EnableStreams   bool
}

// ============================================================================
// AWS CONFIGURATION
// ============================================================================

// AWS contains AWS service configuration.
type AWS struct {
	Region          string `validate:"required"`
	Profile         string
	Endpoint        string // For local development with LocalStack
	AccessKeyID     string // Optional, uses IAM role if not provided
	SecretAccessKey string // Optional, uses IAM role if not provided
	SessionToken    string // For temporary credentials
}

// ============================================================================
// DOMAIN CONFIGURATION
// ============================================================================

// Domain contains business logic configuration.
type Domain struct {
	SimilarityThreshold   float64 `validate:"min=0,max=1"`
	MaxConnectionsPerNode int     `validate:"min=1,max=1000"`
	MaxContentLength      int     `validate:"min=100,max=100000"`
	MinKeywordLength      int     `validate:"min=2,max=50"`
	RecencyWeight         float64 `validate:"min=0,max=1"`
	DiversityThreshold    float64 `validate:"min=0,max=1"`
	MaxTagsPerNode        int     `validate:"min=1,max=100"`
	MaxNodesPerUser       int     `validate:"min=1"`
}

// ============================================================================
// INFRASTRUCTURE CONFIGURATION
// ============================================================================

// Infrastructure contains infrastructure-level settings.
type Infrastructure struct {
	RetryConfig           RetryConfig
	CircuitBreakerConfig  CircuitBreakerConfig
	IdempotencyTTL        time.Duration `validate:"min=1h"`
	HealthCheckInterval   time.Duration `validate:"min=10s"`
	GracefulShutdownDelay time.Duration `validate:"min=0"`
}

// RetryConfig contains retry behavior settings.
type RetryConfig struct {
	MaxRetries     int           `validate:"min=0,max=10"`
	InitialDelay   time.Duration `validate:"min=10ms"`
	MaxDelay       time.Duration `validate:"min=100ms"`
	BackoffFactor  float64       `validate:"min=1,max=10"`
	JitterFactor   float64       `validate:"min=0,max=1"`
	RetryOnTimeout bool
	RetryOn5xx     bool
}

// CircuitBreakerConfig contains circuit breaker settings.
type CircuitBreakerConfig struct {
	FailureThreshold float64       `validate:"min=0,max=1"`
	SuccessThreshold float64       `validate:"min=0,max=1"`
	MinimumRequests  int           `validate:"min=1"`
	WindowSize       time.Duration `validate:"min=1s"`
	OpenDuration     time.Duration `validate:"min=1s"`
	HalfOpenRequests int           `validate:"min=1"`
}

// ============================================================================
// FEATURE FLAGS
// ============================================================================

// Features contains feature flags for gradual rollout and A/B testing.
type Features struct {
	// Core features
	EnableCaching        bool
	EnableAutoConnect    bool
	EnableAIProcessing   bool
	EnableMetrics        bool
	EnableTracing        bool
	EnableEventBus       bool
	
	// Infrastructure features
	EnableRetries        bool
	EnableCircuitBreaker bool
	EnableRateLimiting   bool
	EnableCompression    bool
	
	// Debugging features
	EnableDebugEndpoints bool
	EnableProfiling      bool
	EnableLogging        bool
	VerboseLogging       bool
	
	// Experimental features
	EnableGraphQL        bool
	EnableWebSockets     bool
	EnableBatchAPI       bool
}

// ============================================================================
// CACHE CONFIGURATION
// ============================================================================

// Cache contains caching configuration.
type Cache struct {
	Provider  string        `validate:"oneof=memory redis memcached"`
	MaxItems  int           `validate:"min=1"`
	TTL       time.Duration `validate:"min=1s"`
	QueryTTL  time.Duration `validate:"min=1s"`
	
	// Redis-specific settings
	Redis RedisConfig
}

// RedisConfig contains Redis-specific settings.
type RedisConfig struct {
	Host     string `validate:"required_if=Provider redis"`
	Port     int    `validate:"required_if=Provider redis,min=1,max=65535"`
	Password string
	DB       int `validate:"min=0,max=15"`
	PoolSize int `validate:"min=1,max=1000"`
}

// ============================================================================
// METRICS CONFIGURATION
// ============================================================================

// Metrics contains metrics collection configuration.
type Metrics struct {
	Provider   string `validate:"oneof=prometheus datadog cloudwatch statsd"`
	Interval   time.Duration
	Namespace  string
	Prometheus PrometheusConfig
	Datadog    DatadogConfig
}

// PrometheusConfig contains Prometheus-specific settings.
type PrometheusConfig struct {
	Port int `validate:"min=1,max=65535"`
	Path string
}

// DatadogConfig contains Datadog-specific settings.
type DatadogConfig struct {
	APIKey string
	Host   string
	Port   int `validate:"min=1,max=65535"`
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
	Enabled      bool
	Provider     string `validate:"oneof=jaeger zipkin xray datadog"`
	ServiceName  string
	SampleRate   float64 `validate:"min=0,max=1"`
	Endpoint     string
	AgentHost    string
	AgentPort    int `validate:"min=1,max=65535"`
}

// ============================================================================
// EVENTS CONFIGURATION
// ============================================================================

// Events contains event bus configuration.
type Events struct {
	Provider     string `validate:"oneof=eventbridge kafka rabbitmq sns"`
	EventBusName string
	TopicPrefix  string
	RetryAttempts int
	BatchSize    int
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
	}
	
	// Set legacy fields for backward compatibility
	cfg.TableName = cfg.Database.TableName
	cfg.IndexName = cfg.Database.IndexName
	
	// Apply environment-specific defaults
	cfg.applyEnvironmentDefaults()
	
	return cfg
}

// Validate validates the configuration.
func (c *Config) Validate() error {
	// Basic validation
	if c.Server.Port < 1 || c.Server.Port > 65535 {
		return fmt.Errorf("invalid server port: %d", c.Server.Port)
	}
	
	if c.Domain.SimilarityThreshold < 0 || c.Domain.SimilarityThreshold > 1 {
		return fmt.Errorf("similarity threshold must be between 0 and 1")
	}
	
	// Environment-specific validation
	if c.Environment == Production {
		if !c.Features.EnableMetrics {
			return fmt.Errorf("metrics must be enabled in production")
		}
		if !c.Security.EnableAuth {
			return fmt.Errorf("authentication must be enabled in production")
		}
		if c.Logging.Level == "debug" {
			return fmt.Errorf("debug logging should not be used in production")
		}
	}
	
	return nil
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
		MaxContentLength:      getEnvInt("DOMAIN_MAX_CONTENT_LENGTH", 10000),
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
		EventBusName:  getEnvString("EVENT_BUS_NAME", "default"),
		TopicPrefix:   getEnvString("EVENT_TOPIC_PREFIX", "brain2"),
		RetryAttempts: getEnvInt("EVENT_RETRY_ATTEMPTS", 3),
		BatchSize:     getEnvInt("EVENT_BATCH_SIZE", 10),
	}
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
