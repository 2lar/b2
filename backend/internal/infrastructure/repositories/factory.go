// Package repositories demonstrates the Factory pattern for creating and configuring
// repository instances with decorators and cross-cutting concerns.
//
// The Factory pattern encapsulates the complex object creation logic,
// providing a clean interface for creating fully configured repository instances
// with all necessary decorators applied based on configuration.
//
// Educational Goals:
//   - Show how to manage complex object creation
//   - Demonstrate configuration-driven architecture
//   - Illustrate decorator composition patterns
//   - Provide centralized repository configuration
//   - Enable dependency injection with proper lifecycle management
package repositories

import (
	"fmt"
	"time"

	"brain2-backend/internal/infrastructure/decorators"
	"brain2-backend/internal/repository"
)

// RepositoryConfig defines configuration for repository creation and decoration
type RepositoryConfig struct {
	// Base configuration
	DatabaseType string `json:"database_type"` // "dynamodb", "postgres", "memory"
	
	// Caching configuration
	EnableCaching    bool          `json:"enable_caching"`
	CacheTTL         time.Duration `json:"cache_ttl"`
	CacheKeyPrefix   string        `json:"cache_key_prefix"`
	
	// Logging configuration
	EnableLogging    bool                        `json:"enable_logging"`
	LogLevel         decorators.LogLevel         `json:"log_level"`
	LogParams        bool                        `json:"log_params"`
	LogResults       bool                        `json:"log_results"`
	
	// Metrics configuration
	EnableMetrics    bool              `json:"enable_metrics"`
	MetricsTags      map[string]string `json:"metrics_tags"`
	
	// Performance configuration
	SlowQueryThreshold time.Duration `json:"slow_query_threshold"`
	MaxConnections     int           `json:"max_connections"`
	
	// Environment configuration
	Environment string `json:"environment"` // "development", "staging", "production"
}

// DefaultRepositoryConfig returns default configuration for development
func DefaultRepositoryConfig() *RepositoryConfig {
	return &RepositoryConfig{
		DatabaseType:       "dynamodb",
		EnableCaching:      true,
		CacheTTL:           10 * time.Minute,
		CacheKeyPrefix:     "brain2",
		EnableLogging:      true,
		LogLevel:           decorators.LogLevelInfo,
		LogParams:          true,
		LogResults:         false, // Don't log results by default for privacy
		EnableMetrics:      true,
		MetricsTags:        map[string]string{"service": "brain2"},
		SlowQueryThreshold: 1 * time.Second,
		MaxConnections:     10,
		Environment:        "development",
	}
}

// ProductionRepositoryConfig returns configuration optimized for production
func ProductionRepositoryConfig() *RepositoryConfig {
	return &RepositoryConfig{
		DatabaseType:       "dynamodb",
		EnableCaching:      true,
		CacheTTL:           15 * time.Minute,
		CacheKeyPrefix:     "brain2-prod",
		EnableLogging:      true,
		LogLevel:           decorators.LogLevelWarn, // Less verbose in production
		LogParams:          false,                   // Don't log params in production for privacy
		LogResults:         false,                   // Never log results in production
		EnableMetrics:      true,
		MetricsTags: map[string]string{
			"service":     "brain2",
			"environment": "production",
		},
		SlowQueryThreshold: 500 * time.Millisecond,
		MaxConnections:     50,
		Environment:        "production",
	}
}

// RepositoryFactory creates and configures repository instances with decorators.
// This factory demonstrates the Abstract Factory pattern, providing a unified
// interface for creating different types of repositories with consistent decoration.
type RepositoryFactory struct {
	config *RepositoryConfig
	
	// Dependencies for creating repositories
	cache   decorators.Cache
	logger  decorators.Logger
	metrics decorators.MetricsCollector
	
	// Base repository implementations
	baseNodeRepo         repository.NodeRepository
	baseEdgeRepo         repository.EdgeRepository
	baseCategoryRepo     repository.CategoryRepository
	baseNodeCategoryRepo repository.NodeCategoryMapper
	baseKeywordRepo      repository.KeywordSearcher
	baseGraphRepo        repository.GraphReader
}

// NewRepositoryFactory creates a new repository factory with the given configuration
// and dependencies. This constructor demonstrates dependency injection at the factory level.
func NewRepositoryFactory(
	config *RepositoryConfig,
	cache decorators.Cache,
	logger decorators.Logger,
	metrics decorators.MetricsCollector,
	baseRepos BaseRepositories,
) *RepositoryFactory {
	if config == nil {
		config = DefaultRepositoryConfig()
	}
	
	return &RepositoryFactory{
		config:               config,
		cache:               cache,
		logger:              logger,
		metrics:             metrics,
		baseNodeRepo:         baseRepos.NodeRepo,
		baseEdgeRepo:         baseRepos.EdgeRepo,
		baseCategoryRepo:     baseRepos.CategoryRepo,
		baseNodeCategoryRepo: baseRepos.NodeCategoryRepo,
		baseKeywordRepo:      baseRepos.KeywordRepo,
		baseGraphRepo:        baseRepos.GraphRepo,
	}
}

// BaseRepositories holds the base repository implementations
type BaseRepositories struct {
	NodeRepo         repository.NodeRepository
	EdgeRepo         repository.EdgeRepository
	CategoryRepo     repository.CategoryRepository
	NodeCategoryRepo repository.NodeCategoryMapper
	KeywordRepo      repository.KeywordSearcher
	GraphRepo        repository.GraphReader
}

// CreateNodeRepository creates a fully decorated NodeRepository based on configuration.
// This method demonstrates the Builder pattern for composing decorators.
func (f *RepositoryFactory) CreateNodeRepository() repository.NodeRepository {
	// Start with the base repository
	repo := f.baseNodeRepo
	
	// Apply decorators based on configuration
	// Order matters: innermost decorator is applied first
	
	// 1. Metrics (innermost - closest to actual data operations)
	if f.config.EnableMetrics && f.metrics != nil {
		metricsTags := f.createMetricsTags("node_repository")
		repo = decorators.NewMetricsNodeRepository(repo, f.metrics, metricsTags)
	}
	
	// 2. Logging (middle layer - logs what metrics measure)
	if f.config.EnableLogging && f.logger != nil {
		repo = decorators.NewLoggingNodeRepository(
			repo,
			f.logger,
			f.config.LogLevel,
			f.config.LogParams,
			f.config.LogResults,
		)
	}
	
	// 3. Caching (outermost - first to intercept requests)
	if f.config.EnableCaching && f.cache != nil {
		repo = decorators.NewCachingNodeRepository(
			repo,
			f.cache,
			f.config.CacheTTL,
			f.config.CacheKeyPrefix,
		)
	}
	
	return repo
}

// CreateEdgeRepository creates a fully decorated EdgeRepository
func (f *RepositoryFactory) CreateEdgeRepository() repository.EdgeRepository {
	repo := f.baseEdgeRepo
	
	// Apply the same decoration pattern
	if f.config.EnableMetrics && f.metrics != nil {
		// Note: We'd need to create MetricsEdgeRepository similar to MetricsNodeRepository
		// For brevity, assuming it exists
		// metricsTags := f.createMetricsTags("edge_repository")
		// repo = decorators.NewMetricsEdgeRepository(repo, f.metrics, metricsTags)
	}
	
	if f.config.EnableLogging && f.logger != nil {
		// Similarly, we'd need LoggingEdgeRepository
		// repo = decorators.NewLoggingEdgeRepository(repo, f.logger, ...)
	}
	
	if f.config.EnableCaching && f.cache != nil {
		repo = decorators.NewCachingEdgeRepository(
			repo,
			f.cache,
			f.config.CacheTTL,
			f.config.CacheKeyPrefix,
		)
	}
	
	return repo
}

// CreateCategoryRepository creates a fully decorated CategoryRepository
func (f *RepositoryFactory) CreateCategoryRepository() repository.CategoryRepository {
	// For now, return the base repository
	// In a full implementation, we'd apply decorators here too
	return f.baseCategoryRepo
}

// CreateNodeCategoryMapper creates a decorated NodeCategoryMapper
func (f *RepositoryFactory) CreateNodeCategoryMapper() repository.NodeCategoryMapper {
	return f.baseNodeCategoryRepo
}

// CreateKeywordSearcher creates a decorated KeywordSearcher
func (f *RepositoryFactory) CreateKeywordSearcher() repository.KeywordSearcher {
	return f.baseKeywordRepo
}

// CreateGraphReader creates a decorated GraphReader
func (f *RepositoryFactory) CreateGraphReader() repository.GraphReader {
	return f.baseGraphRepo
}

// CreateUnitOfWork creates a Unit of Work with all decorated repositories
func (f *RepositoryFactory) CreateUnitOfWork(
	txFactory repository.TransactionFactory,
	eventPublisher repository.EventPublisher,
) repository.UnitOfWork {
	return repository.NewUnitOfWork(
		txFactory,
		f.CreateNodeRepository(),
		f.CreateEdgeRepository(),
		f.CreateCategoryRepository(),
		f.CreateNodeCategoryMapper(),
		f.CreateKeywordSearcher(),
		f.CreateGraphReader(),
		eventPublisher,
	)
}

// Helper methods

// createMetricsTags creates standardized tags for metrics based on configuration
func (f *RepositoryFactory) createMetricsTags(component string) map[string]string {
	tags := make(map[string]string)
	
	// Copy base tags from configuration
	for k, v := range f.config.MetricsTags {
		tags[k] = v
	}
	
	// Add component-specific tags
	tags["component"] = component
	tags["environment"] = f.config.Environment
	
	// Add performance-related tags
	if f.config.SlowQueryThreshold > 0 {
		tags["slow_query_threshold_ms"] = fmt.Sprintf("%d", f.config.SlowQueryThreshold.Milliseconds())
	}
	
	return tags
}

// UpdateConfig updates the factory configuration
// This allows dynamic reconfiguration without recreating the factory
func (f *RepositoryFactory) UpdateConfig(newConfig *RepositoryConfig) {
	f.config = newConfig
}

// GetConfig returns the current configuration
func (f *RepositoryFactory) GetConfig() *RepositoryConfig {
	return f.config
}

// RepositoryBundle holds all repository types for convenient access
type RepositoryBundle struct {
	Nodes         repository.NodeRepository
	Edges         repository.EdgeRepository
	Categories    repository.CategoryRepository
	NodeCategories repository.NodeCategoryMapper
	Keywords      repository.KeywordSearcher
	Graph         repository.GraphReader
	UnitOfWork    repository.UnitOfWork
}

// CreateRepositoryBundle creates a complete set of repositories
// This is a convenience method for applications that need all repository types
func (f *RepositoryFactory) CreateRepositoryBundle(
	txFactory repository.TransactionFactory,
	eventPublisher repository.EventPublisher,
) *RepositoryBundle {
	return &RepositoryBundle{
		Nodes:         f.CreateNodeRepository(),
		Edges:         f.CreateEdgeRepository(),
		Categories:    f.CreateCategoryRepository(),
		NodeCategories: f.CreateNodeCategoryMapper(),
		Keywords:      f.CreateKeywordSearcher(),
		Graph:         f.CreateGraphReader(),
		UnitOfWork:    f.CreateUnitOfWork(txFactory, eventPublisher),
	}
}

// Environment-specific factory creation methods

// CreateDevelopmentFactory creates a factory configured for development
func CreateDevelopmentFactory(
	cache decorators.Cache,
	logger decorators.Logger,
	metrics decorators.MetricsCollector,
	baseRepos BaseRepositories,
) *RepositoryFactory {
	config := DefaultRepositoryConfig()
	config.Environment = "development"
	config.LogLevel = decorators.LogLevelDebug // More verbose for development
	config.LogParams = true                    // Log parameters for debugging
	config.SlowQueryThreshold = 2 * time.Second // More lenient threshold
	
	return NewRepositoryFactory(config, cache, logger, metrics, baseRepos)
}

// CreateProductionFactory creates a factory configured for production
func CreateProductionFactory(
	cache decorators.Cache,
	logger decorators.Logger,
	metrics decorators.MetricsCollector,
	baseRepos BaseRepositories,
) *RepositoryFactory {
	config := ProductionRepositoryConfig()
	
	return NewRepositoryFactory(config, cache, logger, metrics, baseRepos)
}

// CreateTestingFactory creates a factory configured for testing
func CreateTestingFactory(baseRepos BaseRepositories) *RepositoryFactory {
	config := &RepositoryConfig{
		DatabaseType:       "memory",
		EnableCaching:      false, // Disable caching for predictable tests
		EnableLogging:      false, // Disable logging to reduce test noise
		EnableMetrics:      false, // Disable metrics for faster tests
		Environment:        "test",
		SlowQueryThreshold: 10 * time.Second, // Very lenient for tests
	}
	
	return NewRepositoryFactory(config, nil, nil, nil, baseRepos)
}

// Example usage:
//
// // Create base repositories (these would come from your infrastructure layer)
// baseRepos := BaseRepositories{
//     NodeRepo:         dynamodb.NewNodeRepository(dynamoClient),
//     EdgeRepo:         dynamodb.NewEdgeRepository(dynamoClient),
//     CategoryRepo:     dynamodb.NewCategoryRepository(dynamoClient),
//     NodeCategoryRepo: dynamodb.NewNodeCategoryMapper(dynamoClient),
//     KeywordRepo:      dynamodb.NewKeywordSearcher(dynamoClient),
//     GraphRepo:        dynamodb.NewGraphReader(dynamoClient),
// }
//
// // Create factory based on environment
// var factory *RepositoryFactory
// switch os.Getenv("ENVIRONMENT") {
// case "production":
//     factory = CreateProductionFactory(cache, logger, metrics, baseRepos)
// case "development":
//     factory = CreateDevelopmentFactory(cache, logger, metrics, baseRepos)
// case "test":
//     factory = CreateTestingFactory(baseRepos)
// default:
//     factory = NewRepositoryFactory(DefaultRepositoryConfig(), cache, logger, metrics, baseRepos)
// }
//
// // Create all repositories with decorators applied
// bundle := factory.CreateRepositoryBundle(txFactory, eventPublisher)
//
// // Use the repositories - they're fully decorated based on configuration
// nodes, err := bundle.Nodes.FindByUser(ctx, userID, repository.WithLimit(10))
//
// This demonstrates how the Factory pattern can provide a clean, configuration-driven
// approach to creating complex objects with multiple decorators applied consistently!