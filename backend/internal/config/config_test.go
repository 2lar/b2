package config_test

import (
	"os"
	"testing"
	"time"

	"brain2-backend/internal/config"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestLoadConfig tests basic configuration loading from environment variables.
func TestLoadConfig(t *testing.T) {
	// Set test environment variables
	os.Setenv("ENVIRONMENT", "development")
	os.Setenv("SERVER_PORT", "9090")
	os.Setenv("TABLE_NAME", "test-table")
	defer func() {
		os.Unsetenv("ENVIRONMENT")
		os.Unsetenv("SERVER_PORT") 
		os.Unsetenv("TABLE_NAME")
	}()

	cfg := config.LoadConfig()

	assert.Equal(t, config.Development, cfg.Environment)
	assert.Equal(t, 9090, cfg.Server.Port)
	assert.Equal(t, "test-table", cfg.Database.TableName)
}

// TestConfigValidation tests configuration validation.
func TestConfigValidation(t *testing.T) {
	tests := []struct {
		name    string
		config  *config.Config
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid development config",
			config: &config.Config{
				Environment: config.Development,
				Server: config.Server{
					Port:            8080,
					Host:            "localhost",
					ReadTimeout:     30 * time.Second,
					WriteTimeout:    30 * time.Second,
					IdleTimeout:     60 * time.Second,
					ShutdownTimeout: 10 * time.Second,
					MaxRequestSize:  10485760,
					RequestTimeout:  30 * time.Second,
				},
				Database: config.Database{
					TableName:      "brain2-dev",
					IndexName:      "KeywordIndex",
					Region:         "us-east-1",
					MaxRetries:     3,
					RetryBaseDelay: 100 * time.Millisecond,
					ConnectionPool: 10,
					Timeout:        10 * time.Second,
					ReadCapacity:   5,
					WriteCapacity:  5,
				},
				AWS: config.AWS{
					Region: "us-east-1",
				},
				Domain: config.Domain{
					SimilarityThreshold:   0.3,
					MaxConnectionsPerNode: 10,
					MaxContentLength:      20000,
					DocumentThreshold:     800,
					DocumentAutoOpen:      1200,
					MinKeywordLength:      3,
					RecencyWeight:         0.2,
					DiversityThreshold:    0.5,
					MaxTagsPerNode:        10,
					MaxNodesPerUser:       10000,
				},
				Infrastructure: config.Infrastructure{
					RetryConfig: config.RetryConfig{
						MaxRetries:    3,
						InitialDelay:  100 * time.Millisecond,
						MaxDelay:      5 * time.Second,
						BackoffFactor: 2.0,
						JitterFactor:  0.1,
					},
					CircuitBreakerConfig: config.CircuitBreakerConfig{
						FailureThreshold: 0.5,
						SuccessThreshold: 0.8,
						MinimumRequests:  10,
						WindowSize:       10 * time.Second,
						OpenDuration:     30 * time.Second,
						HalfOpenRequests: 3,
					},
					IdempotencyTTL:        24 * time.Hour,
					HealthCheckInterval:   30 * time.Second,
					GracefulShutdownDelay: 5 * time.Second,
				},
				Cache: config.Cache{
					Provider: "memory",
					MaxItems: 1000,
					TTL:      5 * time.Minute,
					QueryTTL: 1 * time.Minute,
					Redis: config.RedisConfig{
						PoolSize: 10,
					},
				},
				Metrics: config.Metrics{
					Provider: "prometheus",
					Prometheus: config.PrometheusConfig{
						Port: 9090,
					},
				},
				RateLimit: config.RateLimit{
					RequestsPerMinute: 100,
					Burst:             10,
				},
				Tracing: config.Tracing{
					Provider:  "jaeger",
					AgentPort: 6831,
				},
				Events: config.Events{
					Provider:  "eventbridge",
					BatchSize: 10,
				},
				Logging: config.Logging{
					Level:  "debug",
					Format: "json",
					Output: "stdout",
				},
				Security: config.Security{
					JWTSecret:       "development-secret-key-minimum-32-characters",
					EnableAuth:      false,
					AllowedOrigins:  []string{"*"},
					CSRFTokenLength: 32,
				},
			},
			wantErr: false,
		},
		{
			name: "invalid production config - missing metrics",
			config: &config.Config{
				Environment: config.Production,
				Server: config.Server{
					Port:            443,
					Host:            "0.0.0.0",
					ReadTimeout:     30 * time.Second,
					WriteTimeout:    30 * time.Second,
					IdleTimeout:     60 * time.Second,
					ShutdownTimeout: 10 * time.Second,
					MaxRequestSize:  10485760,
					RequestTimeout:  30 * time.Second,
				},
				Database: config.Database{
					TableName:      "brain2-prod",
					IndexName:      "KeywordIndex",
					Region:         "us-east-1",
					MaxRetries:     3,
					RetryBaseDelay: 100 * time.Millisecond,
					ConnectionPool: 10,
					Timeout:        10 * time.Second,
					ReadCapacity:   5,
					WriteCapacity:  5,
				},
				AWS: config.AWS{
					Region: "us-east-1",
				},
				Domain: config.Domain{
					SimilarityThreshold:   0.3,
					MaxConnectionsPerNode: 10,
					MaxContentLength:      20000,
					DocumentThreshold:     800,
					DocumentAutoOpen:      1200,
					MinKeywordLength:      3,
					RecencyWeight:         0.2,
					DiversityThreshold:    0.5,
					MaxTagsPerNode:        10,
					MaxNodesPerUser:       10000,
				},
				Infrastructure: config.Infrastructure{
					RetryConfig: config.RetryConfig{
						MaxRetries:    3,
						InitialDelay:  100 * time.Millisecond,
						MaxDelay:      5 * time.Second,
						BackoffFactor: 2.0,
						JitterFactor:  0.1,
					},
					CircuitBreakerConfig: config.CircuitBreakerConfig{
						FailureThreshold: 0.5,
						SuccessThreshold: 0.8,
						MinimumRequests:  10,
						WindowSize:       10 * time.Second,
						OpenDuration:     30 * time.Second,
						HalfOpenRequests: 3,
					},
					IdempotencyTTL:        24 * time.Hour,
					HealthCheckInterval:   30 * time.Second,
					GracefulShutdownDelay: 5 * time.Second,
				},
				Cache: config.Cache{
					Provider: "memory",
					MaxItems: 1000,
					TTL:      5 * time.Minute,
					QueryTTL: 1 * time.Minute,
					Redis: config.RedisConfig{
						PoolSize: 10,
					},
				},
				Metrics: config.Metrics{
					Provider: "prometheus",
					Prometheus: config.PrometheusConfig{
						Port: 9090,
					},
				},
				RateLimit: config.RateLimit{
					RequestsPerMinute: 100,
					Burst:             10,
				},
				Tracing: config.Tracing{
					Provider:  "jaeger",
					AgentPort: 6831,
				},
				Events: config.Events{
					Provider:  "eventbridge",
					BatchSize: 10,
				},
				Features: config.Features{
					EnableMetrics: false, // This will cause validation to fail
				},
				Logging: config.Logging{
					Level:  "info",
					Format: "json",
					Output: "stdout",
				},
				Security: config.Security{
					JWTSecret:       "production-secret-key-minimum-32-characters",
					EnableAuth:      true,
					SecureHeaders:   true,
					AllowedOrigins:  []string{"https://example.com"},
					CSRFTokenLength: 32,
				},
			},
			wantErr: true,
			errMsg:  "metrics must be enabled in production",
		},
		{
			name: "invalid business rules - cache TTL",
			config: &config.Config{
				Environment: config.Development,
				Server: config.Server{
					Port:            8080,
					Host:            "localhost",
					ReadTimeout:     30 * time.Second,
					WriteTimeout:    30 * time.Second,
					IdleTimeout:     60 * time.Second,
					ShutdownTimeout: 10 * time.Second,
					MaxRequestSize:  10485760,
					RequestTimeout:  30 * time.Second,
				},
				Database: config.Database{
					TableName:      "brain2-dev",
					IndexName:      "KeywordIndex",
					Region:         "us-east-1",
					MaxRetries:     3,
					RetryBaseDelay: 100 * time.Millisecond,
					ConnectionPool: 10,
					Timeout:        10 * time.Second,
					ReadCapacity:   5,
					WriteCapacity:  5,
				},
				AWS: config.AWS{
					Region: "us-east-1",
				},
				Domain: config.Domain{
					SimilarityThreshold:   0.3,
					MaxConnectionsPerNode: 10,
					MaxContentLength:      20000,
					DocumentThreshold:     800,
					DocumentAutoOpen:      1200,
					MinKeywordLength:      3,
					RecencyWeight:         0.2,
					DiversityThreshold:    0.5,
					MaxTagsPerNode:        10,
					MaxNodesPerUser:       10000,
				},
				Infrastructure: config.Infrastructure{
					RetryConfig: config.RetryConfig{
						MaxRetries:    3,
						InitialDelay:  100 * time.Millisecond,
						MaxDelay:      5 * time.Second,
						BackoffFactor: 2.0,
						JitterFactor:  0.1,
					},
					CircuitBreakerConfig: config.CircuitBreakerConfig{
						FailureThreshold: 0.5,
						SuccessThreshold: 0.8,
						MinimumRequests:  10,
						WindowSize:       10 * time.Second,
						OpenDuration:     30 * time.Second,
						HalfOpenRequests: 3,
					},
					IdempotencyTTL:        24 * time.Hour,
					HealthCheckInterval:   30 * time.Second,
					GracefulShutdownDelay: 5 * time.Second,
				},
				Cache: config.Cache{
					Provider: "memory",
					MaxItems: 1000,
					TTL:      1 * time.Minute,
					QueryTTL: 5 * time.Minute, // QueryTTL > TTL should fail
					Redis: config.RedisConfig{
						PoolSize: 10,
					},
				},
				Metrics: config.Metrics{
					Provider: "prometheus",
					Prometheus: config.PrometheusConfig{
						Port: 9090,
					},
				},
				RateLimit: config.RateLimit{
					RequestsPerMinute: 100,
					Burst:             10,
				},
				Tracing: config.Tracing{
					Provider:  "jaeger",
					AgentPort: 6831,
				},
				Events: config.Events{
					Provider:  "eventbridge",
					BatchSize: 10,
				},
				Logging: config.Logging{
					Level:  "debug",
					Format: "json",
					Output: "stdout",
				},
				Security: config.Security{
					JWTSecret:       "development-secret-key-minimum-32-characters",
					EnableAuth:      false,
					AllowedOrigins:  []string{"*"},
					CSRFTokenLength: 32,
				},
			},
			wantErr: true,
			errMsg:  "cache query TTL cannot be greater than general TTL",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// TestEnvironmentDefaults tests that environment-specific defaults are applied.
func TestEnvironmentDefaults(t *testing.T) {
	tests := []struct {
		env      config.Environment
		expected func(t *testing.T, cfg config.Config)
	}{
		{
			env: config.Development,
			expected: func(t *testing.T, cfg config.Config) {
				assert.Equal(t, "debug", cfg.Logging.Level)
				assert.True(t, cfg.Features.EnableDebugEndpoints)
				assert.True(t, cfg.Features.VerboseLogging)
			},
		},
		{
			env: config.Production,
			expected: func(t *testing.T, cfg config.Config) {
				assert.Equal(t, "info", cfg.Logging.Level)
				assert.True(t, cfg.Features.EnableMetrics)
				assert.True(t, cfg.Features.EnableCircuitBreaker)
				assert.True(t, cfg.Features.EnableRetries)
				assert.True(t, cfg.Security.SecureHeaders)
			},
		},
		{
			env: config.Staging,
			expected: func(t *testing.T, cfg config.Config) {
				assert.True(t, cfg.Features.EnableMetrics)
				assert.Equal(t, "info", cfg.Logging.Level)
			},
		},
	}

	for _, tt := range tests {
		t.Run(string(tt.env), func(t *testing.T) {
			os.Setenv("ENVIRONMENT", string(tt.env))
			defer os.Unsetenv("ENVIRONMENT")
			
			cfg := config.LoadConfig()
			assert.Equal(t, tt.env, cfg.Environment)
			tt.expected(t, cfg)
		})
	}
}