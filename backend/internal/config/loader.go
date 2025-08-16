// Package config provides advanced configuration loading with multiple sources.
// This file demonstrates best practices for configuration management including:
//   - Multiple configuration sources (files, environment variables)
//   - Configuration hierarchy and overlays
//   - Type-safe configuration loading
//   - Comprehensive error handling
package config

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// ============================================================================
// CONFIGURATION LOADER
// ============================================================================

// Loader handles loading configuration from multiple sources.
// It demonstrates the Strategy pattern for different configuration formats
// and the Chain of Responsibility pattern for layered configuration.
type Loader struct {
	// basePath is the root directory for configuration files
	basePath string
	
	// environment is the current deployment environment
	environment Environment
	
	// sources tracks where configuration was loaded from
	sources []string
	
	// fileLoaders maps file extensions to their loaders
	fileLoaders map[string]FileLoader
}

// FileLoader interface for different configuration file formats.
// This demonstrates the Strategy pattern for handling multiple formats.
type FileLoader interface {
	Load(reader io.Reader, target interface{}) error
	Extension() string
}

// ============================================================================
// LOADER IMPLEMENTATION
// ============================================================================

// NewLoader creates a new configuration loader with sensible defaults.
func NewLoader(basePath string, env Environment) *Loader {
	if basePath == "" {
		basePath = "config"
	}
	
	loader := &Loader{
		basePath:    basePath,
		environment: env,
		sources:     make([]string, 0),
		fileLoaders: make(map[string]FileLoader),
	}
	
	// Register default file loaders
	loader.RegisterLoader(&YAMLLoader{})
	loader.RegisterLoader(&JSONLoader{})
	
	return loader
}

// RegisterLoader registers a new file loader for a specific format.
func (l *Loader) RegisterLoader(loader FileLoader) {
	l.fileLoaders[loader.Extension()] = loader
}

// Load loads configuration using a hierarchy of sources.
// The loading order (from lowest to highest priority):
//   1. Default values (in code)
//   2. Base configuration file (base.yaml)
//   3. Environment-specific file (e.g., production.yaml)
//   4. Local overrides file (local.yaml - for development)
//   5. Environment variables (highest priority)
func (l *Loader) Load() (*Config, error) {
	// Start with default configuration
	cfg := l.defaultConfig()
	l.sources = append(l.sources, "defaults")
	
	// Load base configuration
	if err := l.loadFile("base", cfg); err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("failed to load base config: %w", err)
	}
	
	// Load environment-specific configuration
	envFile := strings.ToLower(string(l.environment))
	if err := l.loadFile(envFile, cfg); err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("failed to load %s config: %w", envFile, err)
	}
	
	// Load local overrides (primarily for development)
	if l.environment == Development {
		if err := l.loadFile("local", cfg); err != nil && !os.IsNotExist(err) {
			// Local file errors are warnings in development
			fmt.Fprintf(os.Stderr, "Warning: failed to load local config: %v\n", err)
		}
	}
	
	// Apply environment variables (highest priority)
	l.loadEnvironmentVariables(cfg)
	l.sources = append(l.sources, "environment")
	
	// Set metadata
	cfg.LoadedFrom = l.sources
	cfg.Version = "2.0.0" // Configuration schema version
	
	// Apply environment-specific defaults
	cfg.applyEnvironmentDefaults()
	
	// Validate the final configuration
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("configuration validation failed: %w", err)
	}
	
	return cfg, nil
}

// loadFile loads configuration from a file with automatic format detection.
func (l *Loader) loadFile(name string, cfg *Config) error {
	// Try each supported extension
	for ext, loader := range l.fileLoaders {
		filename := fmt.Sprintf("%s.%s", name, ext)
		filepath := filepath.Join(l.basePath, filename)
		
		file, err := os.Open(filepath)
		if err != nil {
			if os.IsNotExist(err) {
				continue // Try next extension
			}
			return err
		}
		defer file.Close()
		
		if err := loader.Load(file, cfg); err != nil {
			return fmt.Errorf("failed to parse %s: %w", filepath, err)
		}
		
		l.sources = append(l.sources, filepath)
		return nil
	}
	
	// No file found with any supported extension
	return os.ErrNotExist
}

// loadEnvironmentVariables overlays environment variables on the configuration.
// This provides the highest priority configuration source.
func (l *Loader) loadEnvironmentVariables(cfg *Config) {
	// This uses the existing environment variable loading logic
	// but could be enhanced to use struct tags for automatic mapping
	
	// Server configuration
	if val := os.Getenv("SERVER_PORT"); val != "" {
		if port := parseInt(val); port > 0 {
			cfg.Server.Port = port
		}
	}
	if val := os.Getenv("SERVER_HOST"); val != "" {
		cfg.Server.Host = val
	}
	
	// Database configuration
	if val := os.Getenv("TABLE_NAME"); val != "" {
		cfg.Database.TableName = val
	}
	if val := os.Getenv("INDEX_NAME"); val != "" {
		cfg.Database.IndexName = val
	}
	
	// AWS configuration
	if val := os.Getenv("AWS_REGION"); val != "" {
		cfg.AWS.Region = val
		cfg.Database.Region = val // Keep in sync
	}
	
	// Feature flags
	if val := os.Getenv("ENABLE_METRICS"); val != "" {
		cfg.Features.EnableMetrics = parseBool(val)
	}
	if val := os.Getenv("ENABLE_CACHING"); val != "" {
		cfg.Features.EnableCaching = parseBool(val)
	}
	
	// Security
	if val := os.Getenv("JWT_SECRET"); val != "" {
		cfg.Security.JWTSecret = val
	}
	if val := os.Getenv("ENABLE_AUTH"); val != "" {
		cfg.Security.EnableAuth = parseBool(val)
	}
}

// defaultConfig returns a configuration with sensible defaults.
// This ensures the application can run even without configuration files.
func (l *Loader) defaultConfig() *Config {
	return &Config{
		Environment: l.environment,
		Server: Server{
			Port:            8080,
			Host:            "0.0.0.0",
			ReadTimeout:     30 * time.Second,
			WriteTimeout:    30 * time.Second,
			IdleTimeout:     60 * time.Second,
			ShutdownTimeout: 10 * time.Second,
			MaxRequestSize:  10 * 1024 * 1024, // 10MB
			RequestTimeout:  30 * time.Second,
		},
		Database: Database{
			TableName:      "brain2-" + strings.ToLower(string(l.environment)),
			IndexName:      "KeywordIndex",
			Region:         "us-east-1",
			MaxRetries:     3,
			RetryBaseDelay: 100 * time.Millisecond,
			ConnectionPool: 10,
			Timeout:        10 * time.Second,
			ReadCapacity:   5,
			WriteCapacity:  5,
		},
		Domain: Domain{
			SimilarityThreshold:   0.3,
			MaxConnectionsPerNode: 10,
			MaxContentLength:      10000,
			MinKeywordLength:      3,
			RecencyWeight:         0.2,
			DiversityThreshold:    0.5,
			MaxTagsPerNode:        10,
			MaxNodesPerUser:       10000,
		},
		Infrastructure: Infrastructure{
			RetryConfig: RetryConfig{
				MaxRetries:    3,
				InitialDelay:  100 * time.Millisecond,
				MaxDelay:      5 * time.Second,
				BackoffFactor: 2.0,
				JitterFactor:  0.1,
			},
			CircuitBreakerConfig: CircuitBreakerConfig{
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
		Cache: Cache{
			Provider: "memory",
			MaxItems: 1000,
			TTL:      5 * time.Minute,
			QueryTTL: 1 * time.Minute,
			Redis: RedisConfig{
				Host:     "localhost",
				Port:     6379,
				DB:       0,
				PoolSize: 10,
			},
		},
		Metrics: Metrics{
			Provider: "prometheus",
			Interval: 10 * time.Second,
			Namespace: "brain2",
			Prometheus: PrometheusConfig{
				Port: 9090,
				Path: "/metrics",
			},
		},
		Logging: Logging{
			Level:      "info",
			Format:     "json",
			Output:     "stdout",
			MaxSize:    100,
			MaxAge:     30,
			MaxBackups: 10,
			Compress:   true,
		},
		Security: Security{
			JWTSecret:       generateDefaultSecret(),
			JWTExpiry:       24 * time.Hour,
			APIKeyHeader:    "X-API-Key",
			EnableAuth:      true,
			AllowedOrigins:  []string{"*"},
			SecureHeaders:   true,
			CSRFTokenLength: 32,
		},
		RateLimit: RateLimit{
			RequestsPerMinute: 100,
			Burst:             10,
			CleanupInterval:   1 * time.Minute,
			ByIP:              true,
		},
		CORS: CORS{
			Enabled:        true,
			AllowedOrigins: []string{"*"},
			AllowedMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
			AllowedHeaders: []string{"*"},
			MaxAge:         86400,
		},
		Tracing: Tracing{
			Provider:    "jaeger",
			ServiceName: "brain2-backend",
			SampleRate:  0.1,
			AgentHost:   "localhost",
			AgentPort:   6831,
		},
		Events: Events{
			Provider:      "eventbridge",
			EventBusName:  "default",
			TopicPrefix:   "brain2",
			RetryAttempts: 3,
			BatchSize:     10,
		},
	}
}

// ============================================================================
// FILE LOADERS
// ============================================================================

// YAMLLoader loads configuration from YAML files.
type YAMLLoader struct{}

func (y *YAMLLoader) Load(reader io.Reader, target interface{}) error {
	decoder := yaml.NewDecoder(reader)
	return decoder.Decode(target)
}

func (y *YAMLLoader) Extension() string {
	return "yaml"
}

// JSONLoader loads configuration from JSON files.
type JSONLoader struct{}

func (j *JSONLoader) Load(reader io.Reader, target interface{}) error {
	decoder := json.NewDecoder(reader)
	return decoder.Decode(target)
}

func (j *JSONLoader) Extension() string {
	return "json"
}

// ============================================================================
// HELPER FUNCTIONS
// ============================================================================

func parseInt(s string) int {
	val, _ := strconv.Atoi(s)
	return val
}

func parseBool(s string) bool {
	val, _ := strconv.ParseBool(s)
	return val
}

// LoadWithLoader loads configuration using the advanced loader.
// This is the recommended way to load configuration.
func LoadWithLoader() (*Config, error) {
	env := getEnvironment()
	loader := NewLoader("config", env)
	return loader.Load()
}

// MustLoadWithLoader loads configuration and panics on error.
// Use this only in main() or init() functions.
func MustLoadWithLoader() *Config {
	cfg, err := LoadWithLoader()
	if err != nil {
		panic(fmt.Sprintf("failed to load configuration: %v", err))
	}
	return cfg
}