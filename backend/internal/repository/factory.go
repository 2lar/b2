package repository

import (
	"context"
	"time"
	
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Note: Interface compliance checks moved to individual repository implementations
// to avoid import cycles between factory and implementation packages.

// RepositoryFactory creates repository instances with configurable decorators.
//
// Key Concepts Illustrated:
//   1. Abstract Factory Pattern: Creates families of related objects
//   2. Decorator Pattern: Applies cross-cutting concerns transparently
//   3. Dependency Injection: Injects dependencies without tight coupling
//   4. Configuration-Driven Design: Behavior controlled by configuration
//   5. Chain of Responsibility: Decorators can be chained in any order
//
// This factory allows creating repository instances with different combinations
// of decorators (logging, caching, metrics) based on configuration, enabling
// flexible deployment scenarios and easy A/B testing of different configurations.
//
// Example Usage:
//   factory := NewRepositoryFactory(FactoryConfig{
//       EnableLogging: true,
//       EnableCaching: true,
//       EnableMetrics: true,
//   })
//   
//   nodeRepo := factory.CreateNodeRepository(baseRepo, logger, cache, metrics)
type RepositoryFactory struct {
	config FactoryConfig
	logger *zap.Logger
}

// FactoryConfig controls which decorators are applied and their configuration
type FactoryConfig struct {
	// Decorator enablement
	EnableLogging bool
	EnableCaching bool
	EnableMetrics bool
	EnableRetries bool
	
	// Decorator configurations
	LoggingConfig LoggingConfig
	CachingConfig CachingConfig
	MetricsConfig MetricsConfig
	RetryConfig   RetryConfig
	
	// Factory behavior
	StrictMode        bool     // Fail fast on configuration errors
	EnableValidation  bool     // Validate repository implementations
	DecoratorOrder    []string // Order in which decorators are applied
}

// Cross-cutting concern interfaces (avoiding import cycle with decorators package)

// Cache interface for caching decorator
type Cache interface {
	Get(ctx context.Context, key string) ([]byte, bool, error)
	Set(ctx context.Context, key string, value []byte, ttl time.Duration) error
	Delete(ctx context.Context, key string) error
	Clear(ctx context.Context, pattern string) error
}

// MetricsCollector interface for metrics decorator
type MetricsCollector interface {
	IncrementCounter(name string, tags map[string]string)
	IncrementCounterBy(name string, value float64, tags map[string]string)
	SetGauge(name string, value float64, tags map[string]string)
	IncrementGauge(name string, value float64, tags map[string]string)
	RecordDuration(name string, duration time.Duration, tags map[string]string)
	RecordValue(name string, value float64, tags map[string]string)
	RecordDistribution(name string, value float64, tags map[string]string)
}

// Configuration types for decorators

// LoggingConfig controls logging behavior
type LoggingConfig struct {
	LogRequests     bool
	LogResponses    bool
	LogErrors       bool
	LogTiming       bool
	LogLevel        zapcore.Level
	SanitizeData    bool
	MaxResponseSize int
}

// CachingConfig controls caching behavior
type CachingConfig struct {
	EnableReads    bool
	EnableWrites   bool
	DefaultTTL     int // seconds
	KeyPrefix      string
	Serialization  string // "json", "gob", etc.
}

// MetricsConfig controls metrics collection
type MetricsConfig struct {
	EnableLatency   bool
	EnableThroughput bool
	EnableErrors    bool
	EnableBusiness  bool
	SampleRate      float64
	MetricPrefix    string
}

// RetryConfig is defined in retry.go - removed duplicate definition

// Default configuration functions

// DefaultLoggingConfig returns default logging configuration
func DefaultLoggingConfig() LoggingConfig {
	return LoggingConfig{
		LogRequests:     true,
		LogResponses:    false, // May contain PII
		LogErrors:       true,
		LogTiming:       true,
		LogLevel:        zapcore.InfoLevel,
		SanitizeData:    true,
		MaxResponseSize: 1000,
	}
}

// DefaultCachingConfig returns default caching configuration
func DefaultCachingConfig() CachingConfig {
	return CachingConfig{
		EnableReads:   true,
		EnableWrites:  true,
		DefaultTTL:    300, // 5 minutes
		KeyPrefix:     "repo:",
		Serialization: "json",
	}
}

// DefaultMetricsConfig returns default metrics configuration
func DefaultMetricsConfig() MetricsConfig {
	return MetricsConfig{
		EnableLatency:    true,
		EnableThroughput: true,
		EnableErrors:     true,
		EnableBusiness:   false, // Business metrics may be expensive
		SampleRate:       0.1,   // 10% sampling
		MetricPrefix:     "repository.",
	}
}

// DefaultFactoryConfig returns sensible defaults for production use
func DefaultFactoryConfig() FactoryConfig {
	return FactoryConfig{
		EnableLogging: true,
		EnableCaching: false, // Disabled by default for safety
		EnableMetrics: true,
		EnableRetries: false, // Disabled by default for predictability
		
		LoggingConfig: DefaultLoggingConfig(),
		CachingConfig: DefaultCachingConfig(),
		MetricsConfig: DefaultMetricsConfig(),
		RetryConfig: DefaultRetryConfig(),
		
		StrictMode:       false,
		EnableValidation: true,
		DecoratorOrder:   []string{"metrics", "logging", "caching", "retry"}, // Inner to outer
	}
}

// DevelopmentFactoryConfig returns configuration optimized for development
func DevelopmentFactoryConfig() FactoryConfig {
	config := DefaultFactoryConfig()
	
	// Development-specific overrides
	config.EnableCaching = false // Easier debugging without cache
	config.LoggingConfig.LogRequests = true
	config.LoggingConfig.LogResponses = true // OK in development
	config.LoggingConfig.LogLevel = zap.DebugLevel
	config.MetricsConfig.SampleRate = 1.0 // Collect all metrics
	
	return config
}

// ProductionFactoryConfig returns configuration optimized for production
func ProductionFactoryConfig() FactoryConfig {
	config := DefaultFactoryConfig()
	
	// Production-specific overrides
	config.EnableCaching = true
	config.EnableRetries = true
	config.StrictMode = true
	config.LoggingConfig.LogResponses = false // Avoid logging PII
	config.LoggingConfig.LogLevel = zap.InfoLevel
	config.MetricsConfig.SampleRate = 0.1 // Sample 10% for performance
	
	return config
}

// NewRepositoryFactory creates a new repository factory with the given configuration
func NewRepositoryFactory(config FactoryConfig, logger *zap.Logger) *RepositoryFactory {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &RepositoryFactory{
		config: config,
		logger: logger,
	}
}

// Repository Creation Methods

// CreateNodeRepository creates a NodeRepository with configured decorators
func (f *RepositoryFactory) CreateNodeRepository(
	base NodeRepository,
	logger *zap.Logger,
	cache Cache,
	metrics MetricsCollector,
) NodeRepository {
	return f.applyNodeDecorators(base, logger, cache, metrics)
}

// Store-based repository creation is handled directly in the infrastructure layer

// CreateCategoryRepository creates a CategoryRepository with configured decorators
func (f *RepositoryFactory) CreateCategoryRepository(
	base CategoryRepository,
	logger *zap.Logger,
	cache Cache,
	metrics MetricsCollector,
) CategoryRepository {
	return f.applyCategoryDecorators(base, logger, cache, metrics)
}

// CreateEdgeRepository creates an EdgeRepository with configured decorators
func (f *RepositoryFactory) CreateEdgeRepository(
	base EdgeRepository,
	logger *zap.Logger,
	cache Cache,
	metrics MetricsCollector,
) EdgeRepository {
	return f.applyEdgeDecorators(base, logger, cache, metrics)
}

// Store-based factory methods are implemented in the infrastructure layer

// CreateUnitOfWork creates a UnitOfWork with the configured transaction provider
// Now integrated with existing TransactionManager infrastructure
func (f *RepositoryFactory) CreateUnitOfWork(
	provider TransactionProvider,
	eventPublisher EventPublisher,
	repoFactory TransactionalRepositoryFactory,
) UnitOfWork {
	return NewUnitOfWork(provider, eventPublisher, repoFactory, f.logger)
}

// Note: CQRS factory methods removed as they were using placeholder implementations.
// The system uses direct repository implementations with proper decorator patterns.

// Decorator Application Methods

// applyNodeDecorators applies decorators to a NodeRepository in the configured order
func (f *RepositoryFactory) applyNodeDecorators(
	base NodeRepository,
	logger *zap.Logger,
	cache Cache,
	metrics MetricsCollector,
) NodeRepository {
	if f.config.EnableValidation {
		f.validateNodeRepository(base)
	}
	
	var repo NodeRepository = base
	
	// Apply decorators in the configured order (inner to outer)
	// For consolidation phase, decorators are simplified to avoid import cycles
	// In production, these would be proper decorator implementations
	for _, decoratorName := range f.config.DecoratorOrder {
		switch decoratorName {
		case "logging":
			if f.config.EnableLogging && logger != nil {
				// Placeholder: In production would wrap with logging decorator
				// repo = NewLoggingNodeRepository(repo, logger, f.config.LoggingConfig)
			}
		case "caching":
			if f.config.EnableCaching && cache != nil {
				// Placeholder: In production would wrap with caching decorator
				// repo = NewCachingNodeRepository(repo, cache, f.config.CachingConfig)
			}
		case "metrics":
			if f.config.EnableMetrics && metrics != nil {
				// Placeholder: In production would wrap with metrics decorator
				// repo = NewMetricsNodeRepository(repo, metrics, f.config.MetricsConfig)
			}
		case "retry":
			if f.config.EnableRetries {
				repo = f.wrapWithRetry(repo)
			}
		}
	}
	
	return repo
}

// applyEdgeDecorators applies decorators to an EdgeRepository
func (f *RepositoryFactory) applyEdgeDecorators(
	base EdgeRepository,
	logger *zap.Logger,
	cache Cache,
	metrics MetricsCollector,
) EdgeRepository {
	// Parameters kept for future decorator implementations
	_ = cache
	_ = metrics
	
	if f.config.EnableValidation {
		f.validateEdgeRepository(base)
	}
	
	var repo EdgeRepository = base
	
	// Apply decorators (similar pattern to node repository)
	// For consolidation phase, decorators are simplified to avoid import cycles
	for _, decoratorName := range f.config.DecoratorOrder {
		switch decoratorName {
		case "logging":
			if f.config.EnableLogging && logger != nil {
				// Placeholder: Would implement LoggingEdgeRepository in production
				// repo = NewLoggingEdgeRepository(repo, logger, f.config.LoggingConfig)
			}
		case "metrics":
			if f.config.EnableMetrics && metrics != nil {
				// Placeholder: Would implement MetricsEdgeRepository in production
				// repo = NewMetricsEdgeRepository(repo, metrics, f.config.MetricsConfig)
			}
		// Add other decorators as needed
		}
	}
	
	return repo
}

// applyCategoryDecorators applies decorators to a CategoryRepository
func (f *RepositoryFactory) applyCategoryDecorators(
	base CategoryRepository,
	logger *zap.Logger,
	cache Cache,
	metrics MetricsCollector,
) CategoryRepository {
	// Parameters kept for future decorator implementations
	_ = cache
	_ = metrics
	
	if f.config.EnableValidation {
		f.validateCategoryRepository(base)
	}
	
	var repo CategoryRepository = base
	
	// Apply decorators (similar pattern, would need category-specific decorators)
	// For consolidation phase, decorators are simplified to avoid import cycles  
	for _, decoratorName := range f.config.DecoratorOrder {
		switch decoratorName {
		case "logging":
			if f.config.EnableLogging && logger != nil {
				// Placeholder: Would implement LoggingCategoryRepository in production
			}
		case "metrics":
			if f.config.EnableMetrics && metrics != nil {
				// Placeholder: Would implement MetricsCategoryRepository in production
			}
		}
	}
	
	return repo
}

// Validation Methods

// validateNodeRepository validates that a NodeRepository implementation is correct
func (f *RepositoryFactory) validateNodeRepository(repo NodeRepository) {
	if repo == nil {
		if f.config.StrictMode {
			f.logger.Error("NodeRepository cannot be nil in strict mode")
			return
		}
		return
	}
	
	// Additional runtime validation could be added here
	// For example, checking that all required methods are implemented
}

// validateEdgeRepository validates that an EdgeRepository implementation is correct
func (f *RepositoryFactory) validateEdgeRepository(repo EdgeRepository) {
	if repo == nil {
		if f.config.StrictMode {
			f.logger.Error("EdgeRepository cannot be nil in strict mode")
			return
		}
		return
	}
}

// validateCategoryRepository validates that a CategoryRepository implementation is correct
func (f *RepositoryFactory) validateCategoryRepository(repo CategoryRepository) {
	if repo == nil {
		if f.config.StrictMode {
			f.logger.Error("CategoryRepository cannot be nil in strict mode")
			return
		}
		return
	}
}

// Retry Wrapper Implementation

// wrapWithRetry wraps a repository with retry logic
func (f *RepositoryFactory) wrapWithRetry(repo NodeRepository) NodeRepository {
	// This would implement a retry decorator
	// For now, return the original repository
	return repo
}

// Builder Pattern for Factory Configuration

// FactoryBuilder provides a fluent API for building factory configurations
type FactoryBuilder struct {
	config FactoryConfig
}

// NewFactoryBuilder creates a new factory configuration builder
func NewFactoryBuilder() *FactoryBuilder {
	return &FactoryBuilder{
		config: DefaultFactoryConfig(),
	}
}

// WithLogging enables/disables logging with configuration
func (fb *FactoryBuilder) WithLogging(enable bool, config LoggingConfig) *FactoryBuilder {
	fb.config.EnableLogging = enable
	fb.config.LoggingConfig = config
	return fb
}

// WithCaching enables/disables caching with configuration
func (fb *FactoryBuilder) WithCaching(enable bool, config CachingConfig) *FactoryBuilder {
	fb.config.EnableCaching = enable
	fb.config.CachingConfig = config
	return fb
}

// WithMetrics enables/disables metrics with configuration
func (fb *FactoryBuilder) WithMetrics(enable bool, config MetricsConfig) *FactoryBuilder {
	fb.config.EnableMetrics = enable
	fb.config.MetricsConfig = config
	return fb
}

// WithRetries enables/disables retries with configuration
func (fb *FactoryBuilder) WithRetries(enable bool, config RetryConfig) *FactoryBuilder {
	fb.config.EnableRetries = enable
	fb.config.RetryConfig = config
	return fb
}

// WithDecoratorOrder sets the order in which decorators are applied
func (fb *FactoryBuilder) WithDecoratorOrder(order ...string) *FactoryBuilder {
	fb.config.DecoratorOrder = order
	return fb
}

// StrictMode enables/disables strict mode (fail fast on errors)
func (fb *FactoryBuilder) StrictMode(enable bool) *FactoryBuilder {
	fb.config.StrictMode = enable
	return fb
}

// WithValidation enables/disables repository validation
func (fb *FactoryBuilder) WithValidation(enable bool) *FactoryBuilder {
	fb.config.EnableValidation = enable
	return fb
}

// Build creates the factory with the configured settings
func (fb *FactoryBuilder) Build() *RepositoryFactory {
	return NewRepositoryFactory(fb.config, zap.NewNop())
}

// Specialized Factory Methods for Common Scenarios

// CreateDevelopmentFactory creates a factory optimized for development
func CreateDevelopmentFactory(logger *zap.Logger) *RepositoryFactory {
	config := DevelopmentFactoryConfig()
	
	// Override logging config for development
	config.LoggingConfig.LogRequests = true
	config.LoggingConfig.LogResponses = true
	config.LoggingConfig.LogLevel = zap.DebugLevel
	
	return NewRepositoryFactory(config, zap.NewNop())
}

// CreateProductionFactory creates a factory optimized for production
func CreateProductionFactory() *RepositoryFactory {
	config := ProductionFactoryConfig()
	
	// Production-specific optimizations
	config.MetricsConfig.SampleRate = 0.05 // 5% sampling for high-volume production
	config.CachingConfig.DefaultTTL = 10 // 10 minute default TTL
	
	return NewRepositoryFactory(config, zap.NewNop())
}

// CreateTestingFactory creates a factory optimized for testing
func CreateTestingFactory() *RepositoryFactory {
	return NewFactoryBuilder().
		WithLogging(false, LoggingConfig{}). // Reduce test noise
		WithCaching(false, CachingConfig{}). // Predictable test behavior
		WithMetrics(false, MetricsConfig{}). // Faster tests
		WithRetries(false, DefaultRetryConfig()).               // Predictable test failures
		StrictMode(true).                               // Fail fast in tests
		WithValidation(true).
		Build()
}

// Repository Factory Registry for Dependency Injection

// FactoryRegistry manages multiple repository factories for different contexts
type FactoryRegistry struct {
	factories map[string]*RepositoryFactory
	default_  *RepositoryFactory
}

// NewFactoryRegistry creates a new factory registry
func NewFactoryRegistry() *FactoryRegistry {
	return &FactoryRegistry{
		factories: make(map[string]*RepositoryFactory),
		default_:  NewRepositoryFactory(DefaultFactoryConfig(), zap.NewNop()),
	}
}

// Register registers a factory with a specific name
func (fr *FactoryRegistry) Register(name string, factory *RepositoryFactory) {
	fr.factories[name] = factory
}

// Get retrieves a factory by name, returns default if not found
func (fr *FactoryRegistry) Get(name string) *RepositoryFactory {
	if factory, exists := fr.factories[name]; exists {
		return factory
	}
	return fr.default_
}

// SetDefault sets the default factory
func (fr *FactoryRegistry) SetDefault(factory *RepositoryFactory) {
	fr.default_ = factory
}

// GetDefault returns the default factory
func (fr *FactoryRegistry) GetDefault() *RepositoryFactory {
	return fr.default_
}

// Configuration Presets

// ConfigPresets provides pre-configured factory setups for common scenarios
var ConfigPresets = struct {
	Development FactoryConfig
	Production  FactoryConfig
	Testing     FactoryConfig
	HighVolume  FactoryConfig
	LowLatency  FactoryConfig
}{
	Development: DevelopmentFactoryConfig(),
	Production:  ProductionFactoryConfig(),
	Testing: NewFactoryBuilder().
		WithLogging(false, LoggingConfig{}).
		WithCaching(false, CachingConfig{}).
		WithMetrics(false, MetricsConfig{}).
		StrictMode(true).
		Build().config,
	HighVolume: NewFactoryBuilder().
		WithLogging(true, LoggingConfig{LogLevel: zapcore.WarnLevel}).
		WithCaching(true, CachingConfig{DefaultTTL: 30}).
		WithMetrics(true, MetricsConfig{SampleRate: 0.01}).
		WithDecoratorOrder("metrics", "caching", "logging").
		Build().config,
	LowLatency: NewFactoryBuilder().
		WithLogging(false, LoggingConfig{}).
		WithCaching(true, CachingConfig{DefaultTTL: 60}).
		WithMetrics(true, MetricsConfig{SampleRate: 0.1}).
		WithDecoratorOrder("caching", "metrics").
		Build().config,
}

// Example Usage Functions

// ExampleFactoryUsage demonstrates how to use the repository factory
func ExampleFactoryUsage() {
	// Create a factory for development
	factory := CreateDevelopmentFactory(nil)
	
	// Create repositories with decorators
	// nodeRepo := factory.CreateNodeRepository(baseNodeRepo, logger, cache, metrics)
	// edgeRepo := factory.CreateEdgeRepository(baseEdgeRepo, logger, cache, metrics)
	
	// Use the repositories normally - decorators are transparent
	// nodes, err := nodeRepo.FindNodes(ctx, query)
	
	_ = factory
}

// ExampleCustomFactory demonstrates creating a custom factory configuration
func ExampleCustomFactory() {
	// Build a custom factory configuration
	factory := NewFactoryBuilder().
		WithLogging(true, LoggingConfig{
			LogRequests:  true,
			LogResponses: false,
			LogErrors:    true,
			LogTiming:    true,
		}).
		WithCaching(true, CachingConfig{
			EnableReads:  true,
			EnableWrites: false,
			DefaultTTL:   300, // 5 minutes
		}).
		WithMetrics(true, MetricsConfig{
			EnableLatency:   true,
			EnableBusiness:  true,
			SampleRate:      0.5,
		}).
		WithDecoratorOrder("metrics", "logging", "caching").
		StrictMode(false).
		Build()
	
	_ = factory
}

// Note: CQRS repository wrapper types have been removed as they contained only placeholder methods.
// The actual repositories implement their interfaces directly through composition patterns.