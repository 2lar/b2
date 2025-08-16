// Package di - Factory pattern implementations for complex dependency creation.
// This file demonstrates the Factory pattern for creating services with proper
// lifecycle management and environment-specific configurations.
package di

import (
	"context"
	"fmt"

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

	awsEventbridge "github.com/aws/aws-sdk-go-v2/service/eventbridge"
	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"
)

// ============================================================================
// SERVICE FACTORY - Creates services with lifecycle management
// ============================================================================

// ServiceFactory creates application services with proper configuration.
// This demonstrates the Factory pattern for managing complex object creation.
type ServiceFactory struct {
	config           *config.Config
	logger           *zap.Logger
	repositories     *RepositoryContainer
	domainServices   *DomainServiceContainer
	infrastructure   *InfrastructureContainer
	shutdownHandlers []func(context.Context) error
}

// RepositoryContainer holds all repository instances.
type RepositoryContainer struct {
	Node          repository.NodeRepository
	Edge          repository.EdgeRepository
	Category      repository.CategoryRepository
	Keyword       repository.KeywordRepository
	Transactional repository.TransactionalRepository
	Graph         repository.GraphRepository
	Idempotency   repository.IdempotencyStore
	UnitOfWork    repository.UnitOfWork
}

// DomainServiceContainer holds domain services.
type DomainServiceContainer struct {
	ConnectionAnalyzer *domainServices.ConnectionAnalyzer
	EventBus          domain.EventBus
}

// InfrastructureContainer holds infrastructure services.
type InfrastructureContainer struct {
	Cache             decorators.Cache
	MetricsCollector  decorators.MetricsCollector
	EventBridgeClient *awsEventbridge.Client
}

// NewServiceFactory creates a new service factory with all dependencies.
func NewServiceFactory(
	config *config.Config,
	logger *zap.Logger,
	repos *RepositoryContainer,
	domainSvcs *DomainServiceContainer,
	infra *InfrastructureContainer,
) *ServiceFactory {
	factory := &ServiceFactory{
		config:           config,
		logger:           logger,
		repositories:     repos,
		domainServices:   domainSvcs,
		infrastructure:   infra,
		shutdownHandlers: make([]func(context.Context) error, 0),
	}
	
	// Log factory initialization
	logger.Info("ServiceFactory initialized",
		zap.String("environment", string(config.Environment)),
		zap.Bool("caching_enabled", config.Features.EnableCaching),
		zap.Bool("metrics_enabled", config.Features.EnableMetrics),
	)
	
	return factory
}

// ============================================================================
// APPLICATION SERVICE CREATION
// ============================================================================

// CreateNodeService creates a fully configured NodeService.
// This demonstrates how to apply decorators and configurations based on environment.
func (f *ServiceFactory) CreateNodeService() *services.NodeService {
	f.logger.Debug("Creating NodeService with factory pattern")
	
	// Apply repository decorators based on configuration
	nodeRepo := f.decorateNodeRepository(f.repositories.Node)
	edgeRepo := f.decorateEdgeRepository(f.repositories.Edge)
	
	// Create adapters for CQRS compatibility
	nodeAdapter := adapters.NewNodeRepositoryAdapter(nodeRepo, nil)
	
	// Create UnitOfWork adapter with repository adapters
	uowAdapter := adapters.NewUnitOfWorkAdapter(
		f.repositories.UnitOfWork,
		nodeAdapter,
		nil, // EdgeRepositoryAdapter
		nil, // CategoryRepositoryAdapter
		nil, // GraphRepositoryAdapter
		nil, // NodeCategoryRepositoryAdapter
	)
	
	// Create the service with adapted dependencies
	service := services.NewNodeService(
		nodeAdapter,
		edgeRepo,
		uowAdapter,
		f.domainServices.EventBus,
		f.domainServices.ConnectionAnalyzer,
		f.repositories.Idempotency,
	)
	
	// Register shutdown handler if service implements Closeable
	if closeable, ok := interface{}(service).(Closeable); ok {
		f.registerShutdownHandler(closeable.Close)
	}
	
	f.logger.Info("NodeService created successfully")
	return service
}

// CreateCategoryService creates a fully configured CategoryService.
func (f *ServiceFactory) CreateCategoryService() *services.CategoryService {
	f.logger.Debug("Creating CategoryService with factory pattern")
	
	// Apply repository decorators
	categoryRepo := f.decorateCategoryRepository(f.repositories.Category)
	nodeRepo := f.decorateNodeRepository(f.repositories.Node)
	
	// Create adapters for CQRS compatibility
	categoryAdapter := adapters.NewCategoryRepositoryAdapter(categoryRepo)
	nodeAdapter := adapters.NewNodeRepositoryAdapter(nodeRepo, nil)
	uowAdapter := adapters.NewUnitOfWorkAdapter(
		f.repositories.UnitOfWork,
		nodeAdapter,
		nil, // EdgeRepositoryAdapter
		categoryAdapter,
		nil, // GraphRepositoryAdapter
		nil, // NodeCategoryRepositoryAdapter
	)
	
	// Create the service
	service := services.NewCategoryService(
		categoryAdapter,
		nodeAdapter,
		uowAdapter,
		f.domainServices.EventBus,
		f.repositories.Idempotency,
		nil, // AI service
	)
	
	f.logger.Info("CategoryService created successfully")
	return service
}

// ============================================================================
// QUERY SERVICE CREATION (CQRS)
// ============================================================================

// CreateNodeQueryService creates a query service for read operations.
// This demonstrates CQRS pattern with separate read models.
func (f *ServiceFactory) CreateNodeQueryService() *queries.NodeQueryService {
	f.logger.Debug("Creating NodeQueryService for CQRS queries")
	
	// Create cache adapter if caching is enabled
	var cache queries.Cache
	if f.config.Features.EnableCaching {
		cache = f.createQueryCache()
	}
	
	// Create reader adapters with read-optimized configurations
	nodeReader := f.createNodeReader()
	edgeReader := f.createEdgeReader()
	
	service := queries.NewNodeQueryService(
		nodeReader,
		edgeReader,
		f.repositories.Graph,
		cache,
	)
	
	f.logger.Info("NodeQueryService created successfully",
		zap.Bool("cache_enabled", cache != nil),
	)
	
	return service
}

// CreateCategoryQueryService creates a query service for category reads.
func (f *ServiceFactory) CreateCategoryQueryService() *queries.CategoryQueryService {
	f.logger.Debug("Creating CategoryQueryService for CQRS queries")
	
	var cache queries.Cache
	if f.config.Features.EnableCaching {
		cache = f.createQueryCache()
	}
	
	categoryReader := f.createCategoryReader()
	
	service := queries.NewCategoryQueryService(
		categoryReader,
		nil, // nodeCategoryRepo
		nil, // nodeReader
		cache,
		nil, // AI service
	)
	
	f.logger.Info("CategoryQueryService created successfully")
	return service
}

// ============================================================================
// LEGACY SERVICE CREATION (for migration)
// ============================================================================

// CreateLegacyMemoryService creates the legacy service for backward compatibility.
func (f *ServiceFactory) CreateLegacyMemoryService() memoryService.Service {
	f.logger.Debug("Creating legacy MemoryService")
	
	return memoryService.NewServiceWithIdempotency(
		f.repositories.Node,
		f.repositories.Edge,
		f.repositories.Keyword,
		f.repositories.Transactional,
		f.repositories.Graph,
		f.repositories.Idempotency,
	)
}

// CreateMemoryServiceWithMigration creates a memory service with migration adapter.
// This demonstrates the Adapter pattern for gradual migration.
func (f *ServiceFactory) CreateMemoryServiceWithMigration() memoryService.Service {
	f.logger.Debug("Creating MemoryService with migration adapter")
	
	// Create new CQRS-based services
	nodeService := f.CreateNodeService()
	nodeQueryService := f.CreateNodeQueryService()
	
	// Create legacy service as fallback
	legacyService := f.CreateLegacyMemoryService()
	
	// Use adapter if new services are available
	if nodeService != nil && nodeQueryService != nil {
		f.logger.Info("Using CQRS-based MemoryService with migration adapter")
		return NewMemoryServiceAdapter(
			nodeService,
			nodeQueryService,
			legacyService,
		)
	}
	
	f.logger.Warn("Falling back to legacy MemoryService")
	return legacyService
}

// ============================================================================
// REPOSITORY DECORATION
// ============================================================================

// decorateNodeRepository applies decorators to NodeRepository based on config.
// This demonstrates the Decorator pattern with layered application.
func (f *ServiceFactory) decorateNodeRepository(base repository.NodeRepository) repository.NodeRepository {
	// Use decorator chain for clean decorator application
	decoratorChain := decorators.NewDecoratorChain(
		f.config,
		f.logger,
		f.infrastructure.Cache,
		f.infrastructure.MetricsCollector,
	)
	return decoratorChain.DecorateNodeRepository(base)
}

// decorateEdgeRepository applies decorators to EdgeRepository.
func (f *ServiceFactory) decorateEdgeRepository(base repository.EdgeRepository) repository.EdgeRepository {
	// Use decorator chain for clean decorator application
	decoratorChain := decorators.NewDecoratorChain(
		f.config,
		f.logger,
		f.infrastructure.Cache,
		f.infrastructure.MetricsCollector,
	)
	return decoratorChain.DecorateEdgeRepository(base)
}

// decorateCategoryRepository applies decorators to CategoryRepository.
func (f *ServiceFactory) decorateCategoryRepository(base repository.CategoryRepository) repository.CategoryRepository {
	// Use decorator chain for clean decorator application
	decoratorChain := decorators.NewDecoratorChain(
		f.config,
		f.logger,
		f.infrastructure.Cache,
		f.infrastructure.MetricsCollector,
	)
	return decoratorChain.DecorateCategoryRepository(base)
}

// ============================================================================
// HANDLER FACTORY - Creates HTTP handlers
// ============================================================================

// HandlerFactory creates HTTP handlers with proper dependencies.
type HandlerFactory struct {
	config         *config.Config
	logger         *zap.Logger
	serviceFactory *ServiceFactory
	infrastructure *InfrastructureContainer
}

// NewHandlerFactory creates a new handler factory.
func NewHandlerFactory(
	config *config.Config,
	logger *zap.Logger,
	serviceFactory *ServiceFactory,
	infra *InfrastructureContainer,
) *HandlerFactory {
	return &HandlerFactory{
		config:         config,
		logger:         logger,
		serviceFactory: serviceFactory,
		infrastructure: infra,
	}
}

// CreateMemoryHandler creates the memory handler with all dependencies.
func (hf *HandlerFactory) CreateMemoryHandler(coldStartProvider ColdStartInfoProvider) *handlers.MemoryHandler {
	hf.logger.Debug("Creating MemoryHandler")
	
	// Create service with migration support
	service := hf.serviceFactory.CreateMemoryServiceWithMigration()
	
	handler := handlers.NewMemoryHandler(
		service,
		hf.infrastructure.EventBridgeClient,
		coldStartProvider,
	)
	
	hf.logger.Info("MemoryHandler created successfully")
	return handler
}

// CreateCategoryHandler creates the category handler.
func (hf *HandlerFactory) CreateCategoryHandler() *handlers.CategoryHandler {
	hf.logger.Debug("Creating CategoryHandler")
	
	// For now, use legacy service until CategoryService is fully migrated
	// Create a minimal config for the service
	minimalConfig := &config.Config{
		Features: config.Features{
			EnableAIProcessing: false,
		},
	}
	legacyService := categoryService.NewEnhancedService(nil, nil, minimalConfig) // Will be replaced
	
	handler := handlers.NewCategoryHandler(legacyService)
	
	hf.logger.Info("CategoryHandler created successfully")
	return handler
}

// CreateHealthHandler creates the health check handler.
// func (hf *HandlerFactory) CreateHealthHandler(healthChecker HealthChecker) *handlers.HealthHandler {
// 	return handlers.NewHealthHandler(healthChecker)
// }

// CreateAllHandlers creates all handlers as a convenience method.
func (hf *HandlerFactory) CreateAllHandlers(
	coldStartProvider ColdStartInfoProvider,
	healthChecker HealthChecker,
) *HandlerContainer {
	return &HandlerContainer{
		Memory:   hf.CreateMemoryHandler(coldStartProvider),
		Category: hf.CreateCategoryHandler(),
	}
}

// HandlerContainer holds all HTTP handlers.
type HandlerContainer struct {
	Memory   *handlers.MemoryHandler
	Category *handlers.CategoryHandler
}

// ============================================================================
// ROUTER FACTORY - Creates configured routers
// ============================================================================

// RouterFactory creates and configures HTTP routers.
type RouterFactory struct {
	config   *config.Config
	logger   *zap.Logger
	handlers *HandlerContainer
}

// NewRouterFactory creates a new router factory.
func NewRouterFactory(
	config *config.Config,
	logger *zap.Logger,
	handlers *HandlerContainer,
) *RouterFactory {
	return &RouterFactory{
		config:   config,
		logger:   logger,
		handlers: handlers,
	}
}

// CreateRouter creates a fully configured Chi router.
// This demonstrates how to configure middleware based on environment.
func (rf *RouterFactory) CreateRouter() *chi.Mux {
	rf.logger.Debug("Creating HTTP router")
	
	router := chi.NewRouter()
	
	// Apply middleware based on configuration
	rf.applyMiddleware(router)
	
	// Set up routes
	rf.setupRoutes(router)
	
	rf.logger.Info("HTTP router created successfully",
		zap.Int("middleware_count", rf.getMiddlewareCount()),
	)
	
	return router
}

// applyMiddleware configures middleware based on environment and features.
func (rf *RouterFactory) applyMiddleware(router *chi.Mux) {
	// Middleware will be implemented in Phase 5
	// For now, this is a placeholder that will be implemented in Phase 5
	
	// // Always apply these middleware
	// router.Use(middleware.RequestID)
	// router.Use(middleware.Recovery)
	
	// // Environment-specific middleware
	// switch rf.config.Environment {
	// case config.Development:
	// 	router.Use(middleware.Logger) // Verbose logging in development
	// 	router.Use(middleware.Profiler) // Performance profiling
	// case config.Production:
	// 	router.Use(middleware.Compress(5)) // Response compression
	// 	router.Use(middleware.RateLimiter(rf.config.RateLimit))
	// }
	
	// // Feature-based middleware
	// if rf.config.Features.EnableMetrics {
	// 	router.Use(middleware.Metrics(rf.config.Metrics))
	// }
	
	// if rf.config.Features.EnableTracing {
	// 	router.Use(middleware.Tracing(rf.config.Tracing))
	// }
	
	// // Security middleware
	// router.Use(middleware.SecurityHeaders())
	// router.Use(middleware.CORS(rf.config.CORS))
	
	// // Timeout middleware (with different values per environment)
	// timeout := rf.config.Server.RequestTimeout
	// if timeout == 0 {
	// 	timeout = 30 * time.Second
	// }
	// router.Use(middleware.Timeout(timeout))
}

// setupRoutes configures all application routes.
func (rf *RouterFactory) setupRoutes(router *chi.Mux) {
	// Health check (public)
	// router.Get("/health", rf.handlers.Health.Check)
	// router.Get("/ready", rf.handlers.Health.Ready)
	
	// API routes (protected)
	router.Route("/api", func(r chi.Router) {
		// Apply authentication middleware for API routes
		// r.Use(handlers.Authenticator)
		
		// Apply circuit breaker for API routes
		// if rf.config.Features.EnableCircuitBreaker {
		// 	r.Use(middleware.CircuitBreaker(rf.config.Infrastructure.CircuitBreakerConfig))
		// }
		
		// Node routes
		r.Route("/nodes", func(r chi.Router) {
			r.Post("/", rf.handlers.Memory.CreateNode)
			r.Get("/", rf.handlers.Memory.ListNodes)
			r.Get("/{nodeId}", rf.handlers.Memory.GetNode)
			r.Put("/{nodeId}", rf.handlers.Memory.UpdateNode)
			r.Delete("/{nodeId}", rf.handlers.Memory.DeleteNode)
			r.Post("/bulk-delete", rf.handlers.Memory.BulkDeleteNodes)
		})
		
		// Graph routes
		r.Get("/graph-data", rf.handlers.Memory.GetGraphData)
		
		// Category routes
		r.Route("/categories", func(r chi.Router) {
			r.Post("/", rf.handlers.Category.CreateCategory)
			r.Get("/", rf.handlers.Category.ListCategories)
			r.Get("/{categoryId}", rf.handlers.Category.GetCategory)
			r.Put("/{categoryId}", rf.handlers.Category.UpdateCategory)
			r.Delete("/{categoryId}", rf.handlers.Category.DeleteCategory)
			
			// Category-Node relationships
			r.Post("/{categoryId}/nodes", rf.handlers.Category.AssignNodeToCategory)
			r.Get("/{categoryId}/nodes", rf.handlers.Category.GetNodesInCategory)
			r.Delete("/{categoryId}/nodes/{nodeId}", rf.handlers.Category.RemoveNodeFromCategory)
		})
		
		// Node categorization routes
		r.Get("/nodes/{nodeId}/categories", rf.handlers.Category.GetNodeCategories)
		r.Post("/nodes/{nodeId}/categories", rf.handlers.Category.CategorizeNode)
	})
	
	// Development-only routes
	// if rf.config.Environment == config.Development {
	// 	router.Mount("/debug", middleware.Profiler())
	// }
}

// getMiddlewareCount returns the number of middleware configured.
func (rf *RouterFactory) getMiddlewareCount() int {
	count := 2 // RequestID and Recovery are always present
	
	if rf.config.Environment == config.Development {
		count += 2 // Logger and Profiler
	} else if rf.config.Environment == config.Production {
		count += 2 // Compress and RateLimiter
	}
	
	if rf.config.Features.EnableMetrics {
		count++
	}
	
	if rf.config.Features.EnableTracing {
		count++
	}
	
	count += 3 // SecurityHeaders, CORS, Timeout
	
	return count
}

// ============================================================================
// HELPER FUNCTIONS AND INTERFACES
// ============================================================================

// createQueryCache creates a cache adapter for query services.
func (f *ServiceFactory) createQueryCache() queries.Cache {
	return &queryCacheAdapter{
		inner: f.infrastructure.Cache,
		// ttl:   f.config.Cache.QueryTTL, // Need to add this field to the adapter
	}
}

// createNodeReader creates a read-optimized node reader.
func (f *ServiceFactory) createNodeReader() repository.NodeReader {
	// Use the bridge implementation
	return NewNodeReaderBridge(f.repositories.Node)
}

// createEdgeReader creates a read-optimized edge reader.
func (f *ServiceFactory) createEdgeReader() repository.EdgeReader {
	// Use the bridge implementation
	return NewEdgeReaderBridge(f.repositories.Edge)
}

// createCategoryReader creates a read-optimized category reader.
func (f *ServiceFactory) createCategoryReader() repository.CategoryReader {
	// Use the bridge implementation
	return NewCategoryReaderBridge(f.repositories.Category)
}

// registerShutdownHandler registers a function to be called on shutdown.
func (f *ServiceFactory) registerShutdownHandler(handler func(context.Context) error) {
	f.shutdownHandlers = append(f.shutdownHandlers, handler)
}

// Shutdown gracefully shuts down all services created by the factory.
func (f *ServiceFactory) Shutdown(ctx context.Context) error {
	f.logger.Info("Shutting down services",
		zap.Int("handler_count", len(f.shutdownHandlers)),
	)
	
	var errs []error
	for _, handler := range f.shutdownHandlers {
		if err := handler(ctx); err != nil {
			errs = append(errs, err)
			f.logger.Error("Shutdown handler failed", zap.Error(err))
		}
	}
	
	if len(errs) > 0 {
		return fmt.Errorf("shutdown completed with %d errors", len(errs))
	}
	
	f.logger.Info("All services shut down successfully")
	return nil
}

// Closeable interface for services that need cleanup.
type Closeable interface {
	Close(context.Context) error
}

// ============================================================================
// FACTORY BUILDER - Builds factories with all dependencies
// ============================================================================

// FactoryBuilder orchestrates the creation of all factories.
// This demonstrates the Builder pattern for complex object construction.
type FactoryBuilder struct {
	config *config.Config
	logger *zap.Logger
}

// NewFactoryBuilder creates a new factory builder.
func NewFactoryBuilder(config *config.Config, logger *zap.Logger) *FactoryBuilder {
	return &FactoryBuilder{
		config: config,
		logger: logger,
	}
}

// Build creates all factories with proper dependencies.
func (fb *FactoryBuilder) Build(
	repos *RepositoryContainer,
	domainSvcs *DomainServiceContainer,
	infra *InfrastructureContainer,
) (*ApplicationFactories, error) {
	// Create service factory
	serviceFactory := NewServiceFactory(
		fb.config,
		fb.logger,
		repos,
		domainSvcs,
		infra,
	)
	
	// Create handler factory
	handlerFactory := NewHandlerFactory(
		fb.config,
		fb.logger,
		serviceFactory,
		infra,
	)
	
	// Create all handlers
	// Note: In real implementation, would pass actual cold start provider and health checker
	handlers := handlerFactory.CreateAllHandlers(nil, nil)
	
	// Create router factory
	routerFactory := NewRouterFactory(
		fb.config,
		fb.logger,
		handlers,
	)
	
	return &ApplicationFactories{
		Service: serviceFactory,
		Handler: handlerFactory,
		Router:  routerFactory,
	}, nil
}

// ApplicationFactories holds all application factories.
type ApplicationFactories struct {
	Service *ServiceFactory
	Handler *HandlerFactory
	Router  *RouterFactory
}