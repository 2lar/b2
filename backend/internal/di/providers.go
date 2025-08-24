//go:build !wireinject
// +build !wireinject

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
	"github.com/go-chi/chi/v5/middleware"
	"go.uber.org/zap"
)

// ============================================================================
// PROVIDER SETS - Defined in wire_sets.go for Wire generation
// ============================================================================

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

// provideContext provides a context for AWS SDK operations.
func provideContext() context.Context {
	return context.Background()
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
	factory *repository.RepositoryFactory,
	cache cache.Cache,
	metrics *observability.Collector,
) repository.NodeRepository {
	// Base repository implementation
	base := infraDynamodb.NewNodeRepository(client, cfg.TableName, cfg.IndexName, logger)
	
	// Return base repository directly (factory pattern removed for simplicity)
	return base
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
	base := infraDynamodb.NewEdgeRepositoryCQRS(client, cfg.TableName, cfg.IndexName, logger)
	
	// Return base repository directly
	return base
}

// provideCategoryRepository creates a decorated CategoryRepository.
func provideCategoryRepository(
	client *awsDynamodb.Client,
	cfg *config.Config,
	logger *zap.Logger,
	factory *repository.RepositoryFactory,
	cache cache.Cache,
	metrics *observability.Collector,
) repository.CategoryRepository {
	// Base repository
	base := infraDynamodb.NewCategoryRepositoryCQRS(client, cfg.TableName, cfg.IndexName, logger)
	
	// Return base repository directly
	return base
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

// provideCacheAdapter returns the cache directly since interfaces are unified.
func provideCacheAdapter(cache cache.Cache) queries.Cache {
	return cache
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
	nodeRepo repository.NodeRepository,
	edgeRepo repository.EdgeRepository,
	keywordRepo repository.KeywordRepository,
	transactionalRepo repository.TransactionalRepository,
) repository.UnitOfWork {
	// Return nil for now - will need proper implementation
	// that coordinates between these repositories
	return nil
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
	
	// Use cache directly since interfaces are unified
	var queryCache queries.Cache = cache
	
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
	
	// Use cache directly since interfaces are unified
	var queryCache queries.Cache = cache
	
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
	healthHandler *handlers.HealthHandler,
	cfg *config.Config,
) *chi.Mux {
	router := chi.NewRouter()
	
	// Basic middleware
	router.Use(middleware.RequestID)
	router.Use(middleware.RealIP)
	router.Use(middleware.Logger)
	router.Use(middleware.Recoverer)
	
	// Health check endpoints (public)
	router.Get("/health", healthHandler.Check)
	
	// API routes (protected) - v1
	router.Route("/api/v1", func(r chi.Router) {
		// Apply authentication middleware to all API routes
		r.Use(handlers.Authenticator)
		
		// Node routes
		r.Route("/nodes", func(r chi.Router) {
			r.Post("/", memoryHandler.CreateNode)
			r.Get("/", memoryHandler.ListNodes)
			r.Get("/{nodeId}", memoryHandler.GetNode)
			r.Put("/{nodeId}", memoryHandler.UpdateNode)
			r.Delete("/{nodeId}", memoryHandler.DeleteNode)
			r.Post("/bulk-delete", memoryHandler.BulkDeleteNodes)
		})
		
		// Graph routes
		r.Get("/graph-data", memoryHandler.GetGraphData)
		
		// Category routes
		r.Route("/categories", func(r chi.Router) {
			r.Post("/", categoryHandler.CreateCategory)
			r.Get("/", categoryHandler.ListCategories)
			r.Get("/{categoryId}", categoryHandler.GetCategory)
			r.Put("/{categoryId}", categoryHandler.UpdateCategory)
			r.Delete("/{categoryId}", categoryHandler.DeleteCategory)
			
			// Category-Node relationships
			r.Post("/{categoryId}/nodes", categoryHandler.AssignNodeToCategory)
			r.Get("/{categoryId}/nodes", categoryHandler.GetNodesInCategory)
			r.Delete("/{categoryId}/nodes/{nodeId}", categoryHandler.RemoveNodeFromCategory)
		})
		
		// Node categorization routes
		r.Get("/nodes/{nodeId}/categories", categoryHandler.GetNodeCategories)
		r.Post("/nodes/{nodeId}/categories", categoryHandler.CategorizeNode)
	})
	
	return router
}

// ============================================================================
// MISSING REPOSITORY PROVIDERS
// ============================================================================

// provideKeywordRepository creates the keyword repository.
func provideKeywordRepository(
	dynamoClient *awsDynamodb.Client,
	cfg *config.Config,
) repository.KeywordRepository {
	return infraDynamodb.NewKeywordRepository(
		dynamoClient,
		cfg.TableName,
		cfg.IndexName,
	)
}

// provideTransactionalRepository creates the transactional repository.
func provideTransactionalRepository(
	dynamoClient *awsDynamodb.Client,
	cfg *config.Config,
	logger *zap.Logger,
) repository.TransactionalRepository {
	return dynamodb.NewTransactionalRepository(
		dynamoClient,
		cfg.TableName,
		cfg.IndexName,
		logger,
	)
}

// provideGraphRepository creates the graph repository.
func provideGraphRepository(
	dynamoClient *awsDynamodb.Client,
	cfg *config.Config,
	logger *zap.Logger,
) repository.GraphRepository {
	return infraDynamodb.NewGraphRepository(
		dynamoClient,
		cfg.TableName,
		cfg.IndexName,
		logger,
	)
}

// provideRepository creates the composed repository for backward compatibility.
func provideRepository(
	dynamoClient *awsDynamodb.Client,
	cfg *config.Config,
	logger *zap.Logger,
) repository.Repository {
	return dynamodb.NewRepository(
		dynamoClient,
		cfg.TableName,
		cfg.IndexName,
		logger,
	)
}

// provideIdempotencyStore creates the idempotency store.
func provideIdempotencyStore(
	dynamoClient *awsDynamodb.Client,
	cfg *config.Config,
) repository.IdempotencyStore {
	return dynamodb.NewIdempotencyStore(
		dynamoClient,
		cfg.TableName,
		cfg.Infrastructure.IdempotencyTTL,
	)
}

// provideStore creates the persistence store.
func provideStore(
	dynamoClient *awsDynamodb.Client,
	cfg *config.Config,
	logger *zap.Logger,
) persistence.Store {
	storeConfig := persistence.StoreConfig{
		TableName:      cfg.TableName,
		TimeoutMs:      15000,
		RetryAttempts:  3,
		ConsistentRead: false,
	}
	return persistence.NewDynamoDBStore(dynamoClient, storeConfig, logger)
}

// ============================================================================
// UNIT OF WORK PROVIDERS
// ============================================================================


// provideEventStore creates the event store for event sourcing.
func provideEventStore(
	dynamoClient *awsDynamodb.Client,
	cfg *config.Config,
) repository.EventStore {
	return infraDynamodb.NewDynamoDBEventStore(
		dynamoClient,
		cfg.TableName,
	)
}

// provideUnitOfWorkFactory creates a factory for unit of work instances.
func provideUnitOfWorkFactory(
	dynamoClient *awsDynamodb.Client,
	cfg *config.Config,
	eventBus shared.EventBus,
	eventStore repository.EventStore,
	logger *zap.Logger,
) repository.UnitOfWorkFactory {
	return infraDynamodb.NewDynamoDBUnitOfWorkFactory(
		dynamoClient,
		cfg.TableName,
		cfg.IndexName,
		eventBus,
		eventStore,
		logger,
	)
}

// ============================================================================
// MISSING SERVICE PROVIDERS
// ============================================================================

// provideCleanupService creates the cleanup service.
func provideCleanupService(
	nodeRepo repository.NodeRepository,
	edgeRepo repository.EdgeRepository,
	idempotencyStore repository.IdempotencyStore,
	uowFactory repository.UnitOfWorkFactory,
) *services.CleanupService {
	// Get EdgeWriter from EdgeRepository
	var edgeWriter repository.EdgeWriter
	if writer, ok := edgeRepo.(repository.EdgeWriter); ok {
		edgeWriter = writer
	}
	
	return services.NewCleanupService(
		nodeRepo,
		edgeRepo,
		edgeWriter,
		idempotencyStore,
		uowFactory,
	)
}

// provideGraphQueryService creates the graph query service.
func provideGraphQueryService(
	store persistence.Store,
	logger *zap.Logger,
	cache queries.Cache,
) *queries.GraphQueryService {
	return queries.NewGraphQueryService(
		store,
		logger,
		cache,
	)
}

// ============================================================================
// MISSING HANDLER PROVIDERS
// ============================================================================

// provideMemoryHandler creates the memory handler.
func provideMemoryHandler(
	nodeService *services.NodeService,
	nodeQueryService *queries.NodeQueryService,
	graphQueryService *queries.GraphQueryService,
	eventBridgeClient *awsEventbridge.Client,
	coldStartProvider ColdStartInfoProvider,
) *handlers.MemoryHandler {
	return handlers.NewMemoryHandler(
		nodeService,
		nodeQueryService,
		graphQueryService,
		eventBridgeClient,
		coldStartProvider,
	)
}

// provideCategoryHandler creates the category handler.
func provideCategoryHandler(
	categoryService *services.CategoryService,
	categoryQueryService *queries.CategoryQueryService,
) *handlers.CategoryHandler {
	return handlers.NewCategoryHandler(
		categoryService,
		categoryQueryService,
	)
}

// provideHealthHandler creates the health handler.
func provideHealthHandler() *handlers.HealthHandler {
	return handlers.NewHealthHandler()
}

// ============================================================================
// ADVANCED REPOSITORY COMPONENTS
// ============================================================================

// provideRepositoryFactory creates the repository factory.
func provideRepositoryFactory(cfg *config.Config) *repository.RepositoryFactory {
	// Note: repository.FactoryConfig doesn't have these fields
	// and NewRepositoryFactory doesn't exist
	// Return nil for now as a placeholder
	return nil
}

// provideTracerProvider creates the tracer provider for observability.
func provideTracerProvider(cfg *config.Config) (*observability.TracerProvider, error) {
	if cfg.Tracing.Provider == "none" {
		return nil, nil
	}
	
	// Use InitTracing which is the actual function in the observability package
	endpoint := fmt.Sprintf("localhost:%d", cfg.Tracing.AgentPort)
	tracingConfig := observability.TracingConfig{
		ServiceName:  "brain2-backend",
		Environment:  string(cfg.Environment),
		Endpoint:     endpoint,
		SampleRate:   cfg.Tracing.SampleRate,
		EnableXRay:   false, // Will be auto-detected
		EnableDebug:  cfg.Environment == "development",
	}
	return observability.InitTracing(tracingConfig)
}

// ============================================================================
// CONTAINER PROVIDER
// ============================================================================

// provideContainer creates the fully initialized DI container.
func provideContainer(
	cfg *config.Config,
	logger *zap.Logger,
	dynamoClient *awsDynamodb.Client,
	eventBridgeClient *awsEventbridge.Client,
	nodeRepo repository.NodeRepository,
	edgeRepo repository.EdgeRepository,
	categoryRepo repository.CategoryRepository,
	keywordRepo repository.KeywordRepository,
	transactionalRepo repository.TransactionalRepository,
	graphRepo repository.GraphRepository,
	repository repository.Repository,
	idempotencyStore repository.IdempotencyStore,
	store persistence.Store,
	cache cache.Cache,
	metricsCollector *observability.Collector,
	tracerProvider *observability.TracerProvider,
	nodeService *services.NodeService,
	categoryService *services.CategoryService,
	cleanupService *services.CleanupService,
	nodeQueryService *queries.NodeQueryService,
	categoryQueryService *queries.CategoryQueryService,
	graphQueryService *queries.GraphQueryService,
	connectionAnalyzer *domainServices.ConnectionAnalyzer,
	eventBus shared.EventBus,
	unitOfWork repository.UnitOfWork,
	memoryHandler *handlers.MemoryHandler,
	categoryHandler *handlers.CategoryHandler,
	healthHandler *handlers.HealthHandler,
	router *chi.Mux,
	repositoryFactory *repository.RepositoryFactory,
	coldStartTracker *ColdStartTracker,
) *Container {
	// Use cold start tracker's time
	
	return &Container{
		// Configuration
		Config:    cfg,
		TableName: cfg.TableName,
		IndexName: cfg.IndexName,
		
		// Cold start tracking
		ColdStartTime: coldStartTracker.ColdStartTime,
		IsColdStart:   coldStartTracker.IsColdStart,
		
		// AWS Clients
		DynamoDBClient:    dynamoClient,
		EventBridgeClient: eventBridgeClient,
		
		// Repository Layer
		NodeRepository:          nodeRepo,
		EdgeRepository:          edgeRepo,
		CategoryRepository:      categoryRepo,
		KeywordRepository:       keywordRepo,
		TransactionalRepository: transactionalRepo,
		GraphRepository:         graphRepo,
		IdempotencyStore:        idempotencyStore,
		
		// Repository Pattern Enhancements
		RepositoryFactory: repositoryFactory,
		
		// Cross-cutting concerns
		Logger:           logger,
		Cache:            cache,
		MetricsCollector: metricsCollector,
		TracerProvider:   tracerProvider,
		Store:            store,
		
		// Application Service Layer
		NodeAppService:       nodeService,
		CategoryAppService:   categoryService,
		CleanupService:       cleanupService,
		NodeQueryService:     nodeQueryService,
		CategoryQueryService: categoryQueryService,
		GraphQueryService:    graphQueryService,
		
		// Domain Services
		ConnectionAnalyzer: connectionAnalyzer,
		EventBus:           eventBus,
		UnitOfWork:         unitOfWork,
		
		// Handler Layer
		MemoryHandler:   memoryHandler,
		CategoryHandler: categoryHandler,
		HealthHandler:   healthHandler,
		
		// HTTP Router
		Router: router,
		
		// Middleware components
		middlewareConfig: make(map[string]any),
		
		// Lifecycle management
		shutdownFunctions: make([]func() error, 0),
	}
}

// ============================================================================
// HELPER TYPES AND ADAPTERS
// ============================================================================
// Types are defined in types.go to be shared between Wire and manual container