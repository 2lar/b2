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
	infraDynamodb "brain2-backend/internal/infrastructure/dynamodb"
	"brain2-backend/internal/application/adapters"
	"brain2-backend/internal/application/queries"
	"brain2-backend/internal/application/services"
	"brain2-backend/internal/config"
	"brain2-backend/internal/domain"
	domainServices "brain2-backend/internal/domain/services"
	"brain2-backend/internal/handlers"
	"brain2-backend/internal/infrastructure/decorators"
	"brain2-backend/internal/repository"
	categoryService "brain2-backend/internal/service/category"
	memoryService "brain2-backend/internal/service/memory"

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
	// provideKeywordRepository, // TODO: Implement when needed
	// provideTransactionalRepository, // TODO: Implement when needed
	// provideGraphRepository, // TODO: Implement when needed
	// provideIdempotencyStore, // TODO: Implement when needed
	
	// Composed Repository (for backward compatibility)
	// provideComposedRepository, // TODO: Implement when needed
	
	// Cross-cutting Concerns
	provideCache,
	provideMetricsCollector,
	
	// Repository Bindings (Interface -> Implementation)
	// TODO: Uncomment when decorators are implemented
	// wire.Bind(new(repository.NodeRepository), new(*decorators.InstrumentedNodeRepository)),
	// wire.Bind(new(repository.EdgeRepository), new(*dynamodb.EdgeRepository)),
	// wire.Bind(new(repository.CategoryRepository), new(*decorators.InstrumentedCategoryRepository)),
	// wire.Bind(new(repository.KeywordRepository), new(*dynamodb.KeywordRepository)),
	// wire.Bind(new(repository.TransactionalRepository), new(*dynamodb.TransactionalRepository)),
	// wire.Bind(new(repository.GraphRepository), new(*dynamodb.GraphRepository)),
	// wire.Bind(new(repository.IdempotencyStore), new(*dynamodb.IdempotencyStore)),
	// wire.Bind(new(repository.Repository), new(*dynamodb.Repository)),
)

// DomainProviders provides domain services and business logic components.
// This layer has no external dependencies (Pure Domain).
var DomainProviders = wire.NewSet(
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
	
	// Legacy Services (for migration)
	// provideLegacyMemoryService, // Commented out to avoid duplicate binding
	provideLegacyCategoryService,
	
	// Service Adapters for gradual migration
	provideMemoryServiceAdapter, // This provides memoryService.Service
)

// InterfaceProviders provides interface adapters (handlers, middleware).
// This is the outermost layer that adapts external requests to application services.
var InterfaceProviders = wire.NewSet(
	provideMemoryHandler,
	provideCategoryHandler,
	// provideHealthHandler, // TODO: Implement when HealthHandler is created
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
	cache decorators.Cache,
	metrics decorators.MetricsCollector,
) repository.NodeRepository { // TODO: Return InstrumentedNodeRepository when implemented
	// Base repository implementation
	base := dynamodb.NewNodeRepository(client, cfg.Database.TableName, cfg.Database.IndexName)
	
	// Apply decorators based on configuration
	var decorated repository.NodeRepository = base
	
	// TODO: Fix decorator configurations when implementing Phase 3
	// The decorators expect different config types than what we have
	
	// // Layer 1: Retry decorator (closest to base)
	// if cfg.Features.EnableRetries {
	// 	decorated = decorators.NewRetryNodeRepository(decorated, cfg.Infrastructure.RetryConfig)
	// }
	
	// // Layer 2: Circuit breaker
	// if cfg.Features.EnableCircuitBreaker {
	// 	decorated = decorators.NewCircuitBreakerNodeRepository(
	// 		decorated,
	// 		cfg.Infrastructure.CircuitBreakerConfig,
	// 	)
	// }
	
	// // Layer 3: Caching
	// if cfg.Features.EnableCaching {
	// 	decorated = decorators.NewCachingNodeRepository(decorated, cache)
	// }
	
	// // Layer 4: Metrics
	// if cfg.Features.EnableMetrics {
	// 	decorated = decorators.NewMetricsNodeRepository(decorated, metrics)
	// }
	
	// // Layer 5: Logging (outermost)
	// if cfg.Features.EnableLogging {
	// 	decorated = decorators.NewLoggingNodeRepository(decorated, logger)
	// }
	
	// TODO: Return as InstrumentedNodeRepository when implemented
	// return &decorators.InstrumentedNodeRepository{
	// 	Inner: decorated,
	// }
	return decorated
}

// provideEdgeRepository creates an EdgeRepository with appropriate decorators.
func provideEdgeRepository(
	client *awsDynamodb.Client,
	cfg *config.Config,
	logger *zap.Logger,
	cache decorators.Cache,
	metrics decorators.MetricsCollector,
) repository.EdgeRepository {
	// Base repository implementation
	base := dynamodb.NewEdgeRepository(client, cfg.Database.TableName, cfg.Database.IndexName)
	
	// Apply decorators based on configuration
	var decorated repository.EdgeRepository = base
	
	// Note: Decorator implementations would be added here when available
	// For now, return the base repository to enable functionality
	
	return decorated
}

// provideCategoryRepository creates a decorated CategoryRepository.
func provideCategoryRepository(
	client *awsDynamodb.Client,
	cfg *config.Config,
	logger *zap.Logger,
	cache decorators.Cache,
	metrics decorators.MetricsCollector,
) repository.CategoryRepository { // TODO: Return InstrumentedCategoryRepository when implemented
	// Base repository
	base := dynamodb.NewCategoryRepository(client, cfg.Database.TableName, cfg.Database.IndexName)
	
	// Apply decorators similar to NodeRepository
	var decorated repository.CategoryRepository = base
	
	// TODO: Fix decorator configurations when implementing Phase 3
	// if cfg.Features.EnableCaching {
	// 	decorated = decorators.NewCachingCategoryRepository(decorated, cache)
	// }
	
	// if cfg.Features.EnableMetrics {
	// 	decorated = decorators.NewMetricsCategoryRepository(decorated, metrics)
	// }
	
	// if cfg.Features.EnableLogging {
	// 	decorated = decorators.NewLoggingCategoryRepository(decorated, logger)
	// }
	
	// TODO: Return as InstrumentedCategoryRepository when implemented
	// return &decorators.InstrumentedCategoryRepository{
	// 	Inner: decorated,
	// }
	return decorated
}

// TODO: Implement these repositories when needed
// // provideKeywordRepository creates a KeywordRepository.
// func provideKeywordRepository(
// 	client *awsDynamodb.Client,
// 	cfg *config.Config,
// ) *dynamodb.KeywordRepository {
// 	return dynamodb.NewKeywordRepository(client, cfg.Database.TableName, cfg.Database.IndexName)
// }

// // provideTransactionalRepository creates a TransactionalRepository.
// func provideTransactionalRepository(
// 	client *awsDynamodb.Client,
// 	cfg *config.Config,
// ) *dynamodb.TransactionalRepository {
// 	return dynamodb.NewTransactionalRepository(client, cfg.Database.TableName, cfg.Database.IndexName)
// }

// // provideGraphRepository creates a GraphRepository.
// func provideGraphRepository(
// 	client *awsDynamodb.Client,
// 	cfg *config.Config,
// ) *dynamodb.GraphRepository {
// 	return dynamodb.NewGraphRepository(client, cfg.Database.TableName, cfg.Database.IndexName)
// }

// // provideIdempotencyStore creates an IdempotencyStore with TTL.
// func provideIdempotencyStore(
// 	client *awsDynamodb.Client,
// 	cfg *config.Config,
// ) *dynamodb.IdempotencyStore {
// 	ttl := 24 * time.Hour // Default TTL
// 	if cfg.Infrastructure.IdempotencyTTL > 0 {
// 		ttl = cfg.Infrastructure.IdempotencyTTL
// 	}
	
// 	return dynamodb.NewIdempotencyStore(client, cfg.Database.TableName, ttl)
// }

// // provideComposedRepository provides backward compatibility.
// func provideComposedRepository(
// 	client *awsDynamodb.Client,
// 	cfg *config.Config,
// ) *dynamodb.Repository {
// 	return dynamodb.NewRepository(client, cfg.Database.TableName, cfg.Database.IndexName)
// }

// ============================================================================
// INFRASTRUCTURE PROVIDERS - Cross-cutting Concerns
// ============================================================================

// provideCache creates a cache implementation based on configuration.
func provideCache(cfg *config.Config, logger *zap.Logger) decorators.Cache {
	// For now, return a no-op cache until proper implementation is available
	// TODO: Implement actual cache when decorators package is complete
	return &NoOpCache{}
}

// provideMetricsCollector creates a metrics collector based on configuration.
func provideMetricsCollector(cfg *config.Config, logger *zap.Logger) decorators.MetricsCollector {
	// For now, return a no-op metrics collector until proper implementation is available
	// TODO: Implement actual metrics when decorators package is complete
	return &NoOpMetricsCollector{}
}

// ============================================================================
// DOMAIN PROVIDERS
// ============================================================================

// provideConnectionAnalyzer creates the domain service for connection analysis.
func provideConnectionAnalyzer(cfg *config.Config) *domainServices.ConnectionAnalyzer {
	return domainServices.NewConnectionAnalyzer(
		cfg.Domain.SimilarityThreshold,
		cfg.Domain.MaxConnectionsPerNode,
		cfg.Domain.RecencyWeight,
	)
}

// provideEventBus creates the event bus for domain events.
func provideEventBus(cfg *config.Config, logger *zap.Logger) domain.EventBus {
	if cfg.Features.EnableEventBus {
		// In production, would use real event bus (e.g., EventBridge, Kafka)
		// For now, use mock event bus
		return domain.NewMockEventBus()
	}
	
	return domain.NewMockEventBus()
}

// provideUnitOfWork creates the Unit of Work for transactional consistency.
func provideUnitOfWork(
	client *awsDynamodb.Client,
	cfg *config.Config,
	eventBus domain.EventBus,
	logger *zap.Logger,
) repository.UnitOfWork {
	return infraDynamodb.NewDynamoDBUnitOfWork(
		client,
		cfg.Database.TableName,
		cfg.Database.IndexName,
		eventBus,
	)
}

// ============================================================================
// APPLICATION PROVIDERS - Services
// ============================================================================

// provideNodeService creates the application service for node operations.
func provideNodeService(
	nodeRepo repository.NodeRepository,
	edgeRepo repository.EdgeRepository,
	uow repository.UnitOfWork,
	eventBus domain.EventBus,
	analyzer *domainServices.ConnectionAnalyzer,
	idempotencyStore repository.IdempotencyStore,
) *services.NodeService {
	// Create repository adapters for the service
	nodeAdapter := adapters.NewNodeRepositoryAdapter(nodeRepo, nil)
	
	// Create UnitOfWork adapter with all required dependencies
	uowAdapter := adapters.NewUnitOfWorkAdapter(
		uow,
		nodeAdapter,
		nil, // EdgeRepositoryAdapter - TODO
		nil, // CategoryRepositoryAdapter - TODO
		nil, // GraphRepositoryAdapter - TODO
		nil, // NodeCategoryRepositoryAdapter - TODO
	)
	
	return services.NewNodeService(
		nodeAdapter,
		edgeRepo,
		uowAdapter,
		eventBus,
		analyzer,
		idempotencyStore,
	)
}

// provideCategoryAppService creates the application service for categories.
func provideCategoryAppService(
	categoryRepo repository.CategoryRepository,
	uow repository.UnitOfWork,
	eventBus domain.EventBus,
) *services.CategoryService {
	// Would implement CategoryService similar to NodeService
	return nil // Placeholder
}

// provideNodeQueryService creates the query service for nodes (CQRS).
func provideNodeQueryService(
	nodeRepo repository.NodeRepository,
	edgeRepo repository.EdgeRepository,
	graphRepo repository.GraphRepository,
	cache decorators.Cache,
) *queries.NodeQueryService {
	// Create reader adapters
	nodeReader := NewNodeReaderBridge(nodeRepo)
	edgeReader := NewEdgeReaderBridge(edgeRepo)
	
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
	cache decorators.Cache,
) *queries.CategoryQueryService {
	// Would implement similar to NodeQueryService
	return nil // Placeholder
}

// ============================================================================
// APPLICATION PROVIDERS - Legacy Services
// ============================================================================

// provideLegacyMemoryService creates the legacy memory service.
func provideLegacyMemoryService(
	nodeRepo repository.NodeRepository,
	edgeRepo repository.EdgeRepository,
	keywordRepo repository.KeywordRepository,
	transactionalRepo repository.TransactionalRepository,
	graphRepo repository.GraphRepository,
	idempotencyStore repository.IdempotencyStore,
) memoryService.Service {
	return memoryService.NewServiceWithIdempotency(
		nodeRepo,
		edgeRepo,
		keywordRepo,
		transactionalRepo,
		graphRepo,
		idempotencyStore,
	)
}

// provideLegacyCategoryService creates the legacy category service.
func provideLegacyCategoryService(
	repo repository.Repository,
	cfg *config.Config,
) categoryService.Service {
	return categoryService.NewEnhancedService(repo, nil, cfg)
}

// provideMemoryServiceAdapter creates an adapter for gradual migration.
func provideMemoryServiceAdapter(
	nodeService *services.NodeService,
	nodeQueryService *queries.NodeQueryService,
	nodeRepo repository.NodeRepository,
	edgeRepo repository.EdgeRepository,
	keywordRepo repository.KeywordRepository,
	transactionalRepo repository.TransactionalRepository,
	graphRepo repository.GraphRepository,
	idempotencyStore repository.IdempotencyStore,
) memoryService.Service {
	// Create legacy service
	legacyService := memoryService.NewServiceWithIdempotency(
		nodeRepo,
		edgeRepo,
		keywordRepo,
		transactionalRepo,
		graphRepo,
		idempotencyStore,
	)
	
	if nodeService != nil && nodeQueryService != nil {
		return NewMemoryServiceAdapter(
			nodeService,
			nodeQueryService,
			legacyService,
		)
	}
	
	return legacyService
}

// ============================================================================
// INTERFACE PROVIDERS - Handlers
// ============================================================================

// provideMemoryHandler creates the HTTP handler for memory operations.
func provideMemoryHandler(
	service memoryService.Service,
	eventBridge *awsEventbridge.Client,
	container ColdStartInfoProvider,
) *handlers.MemoryHandler {
	return handlers.NewMemoryHandler(service, eventBridge, container)
}

// provideCategoryHandler creates the HTTP handler for category operations.
func provideCategoryHandler(
	service categoryService.Service,
) *handlers.CategoryHandler {
	return handlers.NewCategoryHandler(service)
}

// TODO: Implement HealthHandler
// // provideHealthHandler creates the health check handler.
// func provideHealthHandler(
// 	container HealthChecker,
// ) *handlers.HealthHandler {
// 	return handlers.NewHealthHandler(container)
// }

// provideRouter creates and configures the HTTP router.
func provideRouter(
	memoryHandler *handlers.MemoryHandler,
	categoryHandler *handlers.CategoryHandler,
	// healthHandler *handlers.HealthHandler, // TODO: Add when HealthHandler is implemented
	cfg *config.Config,
) *chi.Mux {
	// TODO: Implement setupRouter function
	// return setupRouter(
	// 	memoryHandler,
	// 	categoryHandler,
	// 	nil, // healthHandler - TODO: Add when HealthHandler is implemented
	// 	cfg,
	// )
	return chi.NewRouter() // Placeholder - return basic router for now
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

// queryCacheAdapter adapts decorators.Cache to queries.Cache.
type queryCacheAdapter struct {
	inner decorators.Cache
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