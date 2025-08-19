// Package di provides dependency injection using Google Wire.
// This file demonstrates best practices for organizing providers into logical groups
// following the Clean Architecture pattern.
package di

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"brain2-backend/infrastructure/dynamodb"
	infraDynamodb "brain2-backend/internal/infrastructure/persistence/dynamodb"
	"brain2-backend/internal/application/queries"
	"brain2-backend/internal/application/services"
	"brain2-backend/internal/config"
	"brain2-backend/internal/domain/events"
	eventHandlers "brain2-backend/internal/domain/events/handlers"
	"brain2-backend/internal/domain/shared"
	domainServices "brain2-backend/internal/domain/services"
	"brain2-backend/internal/features"
	"brain2-backend/internal/handlers"
	"brain2-backend/internal/infrastructure/observability"
	"brain2-backend/internal/infrastructure/persistence"
	"brain2-backend/internal/infrastructure/persistence/cache"
	"brain2-backend/internal/repository"

	aws "github.com/aws/aws-sdk-go-v2/aws"
	awsConfig "github.com/aws/aws-sdk-go-v2/config"
	awsDynamodb "github.com/aws/aws-sdk-go-v2/service/dynamodb"
	awsEventbridge "github.com/aws/aws-sdk-go-v2/service/eventbridge"
	"github.com/go-chi/chi/v5"
	"github.com/google/wire"
	"go.uber.org/zap"
)

// ============================================================================
// PROVIDER SETS - Organized by Clean Architecture Layers
// ============================================================================

// SuperSet combines all provider sets for the complete application.
// This demonstrates how to compose multiple provider sets in Wire.
var SuperSet = wire.NewSet(
	ConfigProviders,
	InfrastructureProviders,
	DomainProviders,
	ApplicationProviders,
	InterfaceProviders,
	wire.Bind(new(http.Handler), new(*chi.Mux)), // Bind router as http.Handler
)

// ConfigProviders provides configuration-related dependencies.
// These are the foundation that other layers depend upon.
var ConfigProviders = wire.NewSet(
	provideConfig,
	provideLogger,
	provideEnvironment,
)

// InfrastructureProviders provides all infrastructure components.
// This layer implements interfaces defined by inner layers (Dependency Inversion).
var InfrastructureProviders = wire.NewSet(
	// AWS Clients
	provideAWSConfig,
	provideDynamoDBClient,
	provideEventBridgeClient,
	
	// Repository Implementations
	provideNodeRepository,
	provideEdgeRepository,
	provideCategoryRepository,
	
	// Cross-cutting Concerns
	provideCache,
	provideMetricsCollector,
	
	// Repository Bindings are handled by concrete implementations above
	// No need for wire.Bind when we're providing concrete types directly
)

// DomainProviders provides domain services and business logic components.
// This layer has no external dependencies (Pure Domain).
var DomainProviders = wire.NewSet(
	provideFeatureService,
	provideConnectionAnalyzer,
	provideEventBus,
	provideUnitOfWork,
)

// ApplicationProviders provides application services (use cases).
// This layer orchestrates domain logic and infrastructure.
var ApplicationProviders = wire.NewSet(
	// Application Services (Command Side)
	provideNodeService,
	provideCategoryAppService,
	
	// Query Services (Query Side - CQRS)
	provideNodeQueryService,
	provideCategoryQueryService,
	
)

// InterfaceProviders provides interface layer components (handlers, middleware).
// This is the outermost layer that adapts external requests to application services.
var InterfaceProviders = wire.NewSet(
	provideRouter,
)

// ============================================================================
// CONFIGURATION PROVIDERS
// ============================================================================

// provideConfig loads and validates the application configuration.
// This demonstrates configuration management best practices.
func provideConfig() (*config.Config, error) {
	cfg := config.LoadConfig()
	
	// Validate configuration based on environment
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}
	
	return &cfg, nil
}

// provideLogger creates a structured logger appropriate for the environment.
// Production uses JSON format, development uses console format.
func provideLogger(cfg *config.Config) (*zap.Logger, error) {
	var logger *zap.Logger
	var err error
	
	switch cfg.Environment {
	case config.Production:
		logger, err = zap.NewProduction()
	case config.Development:
		logger, err = zap.NewDevelopment()
	default:
		// Default to development logger
		logger, err = zap.NewDevelopment()
	}
	
	if err != nil {
		return nil, fmt.Errorf("failed to create logger: %w", err)
	}
	
	return logger, nil
}

// provideEnvironment extracts environment configuration.
// This helps other providers make environment-specific decisions.
func provideEnvironment(cfg *config.Config) config.Environment {
	return cfg.Environment
}

// ============================================================================
// INFRASTRUCTURE PROVIDERS - AWS Clients
// ============================================================================

// provideAWSConfig creates the AWS configuration with appropriate settings.
func provideAWSConfig(ctx context.Context, cfg *config.Config) (aws.Config, error) {
	// Use context with timeout for AWS config loading
	loadCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	
	awsCfg, err := awsConfig.LoadDefaultConfig(loadCtx,
		awsConfig.WithRegion(cfg.AWS.Region),
	)
	if err != nil {
		return awsCfg, fmt.Errorf("failed to load AWS config: %w", err)
	}
	
	return awsCfg, nil
}

// provideDynamoDBClient creates a DynamoDB client with optimized settings.
func provideDynamoDBClient(awsCfg aws.Config, cfg *config.Config) *awsDynamodb.Client {
	return awsDynamodb.NewFromConfig(awsCfg, func(o *awsDynamodb.Options) {
		// Configure timeouts based on environment
		timeout := 15 * time.Second
		if cfg.Environment == config.Development {
			timeout = 30 * time.Second // More lenient in development
		}
		
		o.HTTPClient = &http.Client{
			Timeout: timeout,
		}
	})
}

// provideEventBridgeClient creates an EventBridge client for event publishing.
func provideEventBridgeClient(awsCfg aws.Config) *awsEventbridge.Client {
	return awsEventbridge.NewFromConfig(awsCfg, func(o *awsEventbridge.Options) {
		o.HTTPClient = &http.Client{
			Timeout: 10 * time.Second,
		}
	})
}

// ============================================================================
// INFRASTRUCTURE PROVIDERS - Repositories with Decorators
// ============================================================================

// provideNodeRepository creates a fully decorated NodeRepository.
// This demonstrates the Decorator pattern for cross-cutting concerns.
func provideNodeRepository(
	client *awsDynamodb.Client,
	cfg *config.Config,
	logger *zap.Logger,
	cache cache.Cache,
	metrics *observability.Collector,
) repository.NodeRepository {
	// Base repository implementation
	base := dynamodb.NewNodeRepository(client, cfg.Database.TableName, cfg.Database.IndexName, logger)
	
	// Create decorator chain and apply persistence
	decoratorChain := persistence.NewDecoratorChain(cfg, logger, cache, metrics)
	decorated := decoratorChain.DecorateNodeRepository(base)
	
	return decorated
}

// provideEdgeRepository creates an EdgeRepository with appropriate persistence.
func provideEdgeRepository(
	client *awsDynamodb.Client,
	cfg *config.Config,
	logger *zap.Logger,
	cache cache.Cache,
	metrics *observability.Collector,
) repository.EdgeRepository {
	// Base repository implementation
	base := dynamodb.NewEdgeRepository(client, cfg.Database.TableName, cfg.Database.IndexName, logger)
	
	// Create decorator chain and apply persistence
	decoratorChain := persistence.NewDecoratorChain(cfg, logger, cache, metrics)
	decorated := decoratorChain.DecorateEdgeRepository(base)
	
	return decorated
}

// provideCategoryRepository creates a decorated CategoryRepository.
func provideCategoryRepository(
	client *awsDynamodb.Client,
	cfg *config.Config,
	logger *zap.Logger,
	cache cache.Cache,
	metrics *observability.Collector,
) repository.CategoryRepository {
	// Base repository
	base := dynamodb.NewCategoryRepository(client, cfg.Database.TableName, cfg.Database.IndexName, logger)
	
	// Create decorator chain and apply persistence
	decoratorChain := persistence.NewDecoratorChain(cfg, logger, cache, metrics)
	decorated := decoratorChain.DecorateCategoryRepository(base)
	
	return decorated
}


// ============================================================================
// INFRASTRUCTURE PROVIDERS - Cross-cutting Concerns
// ============================================================================

// provideCache creates a cache implementation based on configuration.
func provideCache(cfg *config.Config, logger *zap.Logger) cache.Cache {
	// Create cache based on configuration
	if !cfg.Features.EnableCaching {
		return NewNoOpCache()
	}
	
	// Use in-memory cache for now
	// In production, this could be Redis, Memcached, etc.
	return NewInMemoryCache(1000, 5*time.Minute)
}

// provideMetricsCollector creates a metrics collector based on configuration.
func provideMetricsCollector(cfg *config.Config, logger *zap.Logger) *observability.Collector {
	// Create metrics collector based on configuration
	if !cfg.Features.EnableMetrics {
		return observability.NewCollector("noop") // Use observability.NewCollector instead
	}
	
	// Use observability collector for now
	// In production, this could be Prometheus, CloudWatch, StatsD, etc.
	return observability.NewCollector("brain2")
}

// ============================================================================
// DOMAIN PROVIDERS
// ============================================================================

// provideFeatureService creates the enhanced feature flag service.
func provideFeatureService(cfg *config.Config) *features.FeatureService {
	return features.NewFeatureService(&cfg.Features)
}

// provideConnectionAnalyzer creates the domain service for connection analysis.
func provideConnectionAnalyzer(cfg *config.Config) *domainServices.ConnectionAnalyzer {
	return domainServices.NewConnectionAnalyzer(
		cfg.Domain.SimilarityThreshold,
		cfg.Domain.MaxConnectionsPerNode,
		cfg.Domain.RecencyWeight,
	)
}

// provideEventBus creates the event bus for domain events.
func provideEventBus(cfg *config.Config, logger *zap.Logger) shared.EventBus {
	if cfg.Features.EnableEventBus {
		// Use our new EventBus implementation with Observer pattern
		eventBus := events.NewEventBus(logger)
		
		// Register event handlers
		// Example: Register category created handler
		categoryHandler := eventHandlers.NewCategoryCreatedHandler(logger)
		eventBus.Subscribe("CategoryCreated", categoryHandler)
		
		// In production, you might also forward events to external systems
		// like EventBridge, Kafka, etc.
		
		return eventBus
	}
	
	// Fall back to mock event bus when feature is disabled
	return shared.NewMockEventBus()
}

// provideUnitOfWork creates the Unit of Work for transactional consistency.
func provideUnitOfWork(
	client *awsDynamodb.Client,
	cfg *config.Config,
	eventBus shared.EventBus,
	logger *zap.Logger,
) repository.UnitOfWork {
	// Create EventStore for domain event persistence
	eventStore := infraDynamodb.NewDynamoDBEventStore(
		client,
		cfg.Database.TableName,
	)
	
	return infraDynamodb.NewDynamoDBUnitOfWork(
		client,
		cfg.Database.TableName,
		cfg.Database.IndexName,
		eventBus,
		eventStore,
		logger,
	)
}

// ============================================================================
// APPLICATION PROVIDERS - Services
// ============================================================================

// provideNodeService creates the application service for node operations.
func provideNodeService(
	nodeRepo repository.NodeRepository,
	edgeRepo repository.EdgeRepository,
	uowFactory repository.UnitOfWorkFactory,
	eventBus shared.EventBus,
	analyzer *domainServices.ConnectionAnalyzer,
	idempotencyStore repository.IdempotencyStore,
) *services.NodeService {
	// Use repositories directly
	return services.NewNodeService(
		nodeRepo,
		edgeRepo,
		uowFactory,
		eventBus,
		analyzer,
		idempotencyStore,
	)
}

// provideCategoryAppService creates the application service for categories.
func provideCategoryAppService(
	categoryRepo repository.CategoryRepository,
	uow repository.UnitOfWork,
	eventBus shared.EventBus,
) *services.CategoryService {
	// Would implement CategoryService similar to NodeService
	return nil // Placeholder
}

// provideNodeQueryService creates the query service for nodes (CQRS).
func provideNodeQueryService(
	nodeRepo repository.NodeRepository,
	edgeRepo repository.EdgeRepository,
	graphRepo repository.GraphRepository,
	cache cache.Cache,
) *queries.NodeQueryService {
	// Use repositories directly as they implement the reader interfaces
	nodeReader := nodeRepo.(repository.NodeReader)
	edgeReader := edgeRepo.(repository.EdgeReader)
	
	// Convert cache to queries.Cache interface
	var queryCache queries.Cache
	if cache != nil {
		queryCache = &queryCacheAdapter{inner: cache}
	}
	
	return queries.NewNodeQueryService(
		nodeReader,
		edgeReader,
		graphRepo,
		queryCache,
	)
}

// provideCategoryQueryService creates the query service for categories.
func provideCategoryQueryService(
	categoryRepo repository.CategoryRepository,
	nodeRepo repository.NodeRepository,
	cache cache.Cache,
	logger *zap.Logger,
) *queries.CategoryQueryService {
	// Use repositories directly as they implement the reader interfaces
	categoryReader := categoryRepo.(repository.CategoryReader)
	nodeReader := nodeRepo.(repository.NodeReader)
	
	// Convert cache to queries.Cache interface
	var queryCache queries.Cache
	if cache != nil {
		queryCache = &queryCacheAdapter{inner: cache}
	}
	
	return queries.NewCategoryQueryService(
		categoryReader,
		nodeReader,
		logger,
		queryCache,
	)
}

// ============================================================================
// APPLICATION PROVIDERS - Legacy Services
// ============================================================================




// ============================================================================
// INTERFACE PROVIDERS - Handlers
// ============================================================================




// provideRouter creates and configures the HTTP router.
func provideRouter(
	memoryHandler *handlers.MemoryHandler,
	categoryHandler *handlers.CategoryHandler,
	cfg *config.Config,
) *chi.Mux {
	router := chi.NewRouter()
	
	// Memory handler routes are registered in the router factory
	// Memory and category handlers don't have Routes() methods yet
	// router.Mount("/api/memory", memoryHandler.Routes())
	// router.Mount("/api/categories", categoryHandler.Routes())
	
	return router
}

// ============================================================================
// HELPER TYPES AND ADAPTERS
// ============================================================================

// ColdStartInfoProvider interface for cold start tracking.
type ColdStartInfoProvider interface {
	GetTimeSinceColdStart() time.Duration
	IsPostColdStartRequest() bool
}

// HealthChecker interface for health checks.
type HealthChecker interface {
	Health(ctx context.Context) map[string]string
}

// queryCacheAdapter adapts cache.Cache to queries.Cache.
type queryCacheAdapter struct {
	inner cache.Cache
}

func (a *queryCacheAdapter) Get(ctx context.Context, key string) (interface{}, bool) {
	data, found, _ := a.inner.Get(ctx, key)
	if !found {
		return nil, false
	}
	return data, true
}

func (a *queryCacheAdapter) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) {
	// Would need to serialize value to []byte
	a.inner.Set(ctx, key, nil, ttl)
}

func (a *queryCacheAdapter) Delete(ctx context.Context, key string) {
	a.inner.Delete(ctx, key)
}