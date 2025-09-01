// Package di provides focused dependency injection containers following SOLID principles.
// This file replaces the God Container anti-pattern with specialized containers.
package di

import (
	"context"
	"net/http"
	"time"

	"brain2-backend/internal/application/queries"
	"brain2-backend/internal/application/services"
	"brain2-backend/internal/config"
	"brain2-backend/internal/di/cache"
	"brain2-backend/internal/errors"
	"brain2-backend/internal/di/initialization"
	"brain2-backend/internal/domain/shared"
	domainServices "brain2-backend/internal/domain/services"
	v1handlers "brain2-backend/internal/interfaces/http/v1/handlers"
	"brain2-backend/internal/infrastructure/messaging"
	"brain2-backend/internal/infrastructure/observability"
	"brain2-backend/internal/infrastructure/persistence"
	persistenceCache "brain2-backend/internal/infrastructure/persistence/cache"
	"brain2-backend/internal/repository"
	"brain2-backend/internal/domain/category"

	awsDynamodb "github.com/aws/aws-sdk-go-v2/service/dynamodb"
	awsEventbridge "github.com/aws/aws-sdk-go-v2/service/eventbridge"
	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"
)

// ============================================================================
// INFRASTRUCTURE CONTAINER - AWS clients and cross-cutting concerns
// ============================================================================

// InfrastructureContainer manages infrastructure dependencies.
// Single Responsibility: Infrastructure and cross-cutting concerns only.
type InfrastructureContainer struct {
	// Configuration
	Config *config.Config
	
	// AWS Clients
	DynamoDBClient    *awsDynamodb.Client
	EventBridgeClient *awsEventbridge.Client
	HTTPClient        *http.Client
	
	// Cross-cutting concerns
	Logger           *zap.Logger
	Cache            persistenceCache.Cache
	MetricsCollector *observability.Collector
	TracerProvider   *observability.TracerProvider
	Store            persistence.Store
	
	// Lifecycle
	shutdownFuncs []func() error
}

// NewInfrastructureContainer creates a new infrastructure container.
func NewInfrastructureContainer(cfg *config.Config) (*InfrastructureContainer, error) {
	c := &InfrastructureContainer{
		Config:        cfg,
		shutdownFuncs: make([]func() error, 0),
	}
	
	if err := c.initialize(); err != nil {
		return nil, err
	}
	
	return c, nil
}

// Shutdown gracefully shuts down infrastructure components.
func (c *InfrastructureContainer) Shutdown() error {
	for _, fn := range c.shutdownFuncs {
		if err := fn(); err != nil {
			return err
		}
	}
	return nil
}

// ============================================================================
// INTERFACE IMPLEMENTATIONS FOR INFRASTRUCTURE CONTAINER
// ============================================================================

// GetConfig returns the configuration.
func (c *InfrastructureContainer) GetConfig() *config.Config {
	return c.Config
}

// GetDynamoDBClient returns the DynamoDB client.
func (c *InfrastructureContainer) GetDynamoDBClient() *awsDynamodb.Client {
	return c.DynamoDBClient
}

// GetEventBridgeClient returns the EventBridge client.
func (c *InfrastructureContainer) GetEventBridgeClient() *awsEventbridge.Client {
	return c.EventBridgeClient
}

// GetHTTPClient returns the HTTP client.
func (c *InfrastructureContainer) GetHTTPClient() *http.Client {
	return c.HTTPClient
}

// GetLogger returns the logger.
func (c *InfrastructureContainer) GetLogger() *zap.Logger {
	return c.Logger
}

// GetCache returns the cache.
func (c *InfrastructureContainer) GetCache() persistenceCache.Cache {
	return c.Cache
}

// GetMetricsCollector returns the metrics collector.
func (c *InfrastructureContainer) GetMetricsCollector() *observability.Collector {
	return c.MetricsCollector
}

// GetTracerProvider returns the tracer provider.
func (c *InfrastructureContainer) GetTracerProvider() *observability.TracerProvider {
	return c.TracerProvider
}

// GetStore returns the persistence store.
func (c *InfrastructureContainer) GetStore() persistence.Store {
	return c.Store
}

// ============================================================================
// REPOSITORY CONTAINER - Data access layer with CQRS
// ============================================================================

// RepositoryContainer manages all repository dependencies.
// Single Responsibility: Data access and persistence only.
type RepositoryContainer struct {
	// Combined repositories (using single repository pattern)
	Node            repository.NodeRepository
	Edge            repository.EdgeRepository
	Category        category.CategoryRepository
	Graph           shared.GraphRepository
	Keyword         repository.KeywordRepository
	Transactional   repository.TransactionalRepository
	Idempotency     repository.IdempotencyStore
	
	// Note: CQRS Reader/Writer interfaces removed in favor of combined repositories
	// Use Node, Edge, Category repositories directly
	
	// Specialized repositories
	GraphRepository    shared.GraphRepository
	KeywordRepository  repository.KeywordRepository
	IdempotencyStore   repository.IdempotencyStore
	
	// Unit of Work for transactional boundaries
	UnitOfWork        repository.UnitOfWork
	UnitOfWorkFactory repository.UnitOfWorkFactory
	
	// Repository Factory for decorators
	RepositoryFactory *repository.RepositoryFactory
}

// NewRepositoryContainer creates a new repository container.
func NewRepositoryContainer(infra IInfrastructureContainer) (*RepositoryContainer, error) {
	c := &RepositoryContainer{}
	
	if err := c.initialize(infra); err != nil {
		return nil, err
	}
	
	return c, nil
}

// ============================================================================
// INTERFACE IMPLEMENTATIONS FOR REPOSITORY CONTAINER
// ============================================================================

// GetNodeRepository returns the node repository.
func (c *RepositoryContainer) GetNodeRepository() repository.NodeRepository {
	return c.Node
}

// GetEdgeRepository returns the edge repository.
func (c *RepositoryContainer) GetEdgeRepository() repository.EdgeRepository {
	return c.Edge
}

// GetCategoryRepository returns the category repository.
func (c *RepositoryContainer) GetCategoryRepository() category.CategoryRepository {
	return c.Category
}

// GetGraphRepository returns the graph repository.
func (c *RepositoryContainer) GetGraphRepository() shared.GraphRepository {
	return c.Graph
}

// GetKeywordRepository returns the keyword repository.
func (c *RepositoryContainer) GetKeywordRepository() repository.KeywordRepository {
	return c.Keyword
}

// GetTransactionalRepository returns the transactional repository.
func (c *RepositoryContainer) GetTransactionalRepository() repository.TransactionalRepository {
	return c.Transactional
}

// GetIdempotencyStore returns the idempotency store.
func (c *RepositoryContainer) GetIdempotencyStore() repository.IdempotencyStore {
	return c.Idempotency
}

// Note: GetNodeReader/Writer, GetEdgeReader/Writer, GetCategoryReader/Writer removed
// Use GetNodeRepository(), GetEdgeRepository(), GetCategoryRepository() instead

// GetUnitOfWork returns the unit of work.
func (c *RepositoryContainer) GetUnitOfWork() repository.UnitOfWork {
	return c.UnitOfWork
}

// GetUnitOfWorkFactory returns the unit of work factory.
func (c *RepositoryContainer) GetUnitOfWorkFactory() repository.UnitOfWorkFactory {
	return c.UnitOfWorkFactory
}

// GetRepositoryFactory returns the repository factory.
func (c *RepositoryContainer) GetRepositoryFactory() *repository.RepositoryFactory {
	return c.RepositoryFactory
}

// ============================================================================
// SERVICE CONTAINER - Application and domain services
// ============================================================================

// ServiceContainer manages application and domain services.
// Single Responsibility: Business logic orchestration only.
type ServiceContainer struct {
	// Application Services - Command Handlers
	NodeCommandService     *services.NodeService
	CategoryCommandService *services.CategoryService
	
	// Query Services - Read Models
	NodeQueryService     *queries.NodeQueryService
	CategoryQueryService *queries.CategoryQueryService
	GraphQueryService    *queries.GraphQueryService
	
	// Domain Services
	ConnectionAnalyzer *domainServices.ConnectionAnalyzer
	EventBus          shared.EventBus
	
	// Supporting Services
	CleanupService *services.CleanupService
}

// NewServiceContainer creates a new service container.
func NewServiceContainer(repos IRepositoryContainer, infra IInfrastructureContainer) (*ServiceContainer, error) {
	c := &ServiceContainer{}
	
	if err := c.initialize(repos, infra); err != nil {
		return nil, err
	}
	
	return c, nil
}

// ============================================================================
// INTERFACE IMPLEMENTATIONS FOR SERVICE CONTAINER
// ============================================================================

// GetNodeCommandService returns the node command service.
func (c *ServiceContainer) GetNodeCommandService() *services.NodeService {
	return c.NodeCommandService
}

// GetCategoryCommandService returns the category command service.
func (c *ServiceContainer) GetCategoryCommandService() *services.CategoryService {
	return c.CategoryCommandService
}

// GetNodeQueryService returns the node query service.
func (c *ServiceContainer) GetNodeQueryService() *queries.NodeQueryService {
	return c.NodeQueryService
}

// GetCategoryQueryService returns the category query service.
func (c *ServiceContainer) GetCategoryQueryService() *queries.CategoryQueryService {
	return c.CategoryQueryService
}

// GetGraphQueryService returns the graph query service.
func (c *ServiceContainer) GetGraphQueryService() *queries.GraphQueryService {
	return c.GraphQueryService
}

// GetConnectionAnalyzer returns the connection analyzer.
func (c *ServiceContainer) GetConnectionAnalyzer() *domainServices.ConnectionAnalyzer {
	return c.ConnectionAnalyzer
}

// GetEventBus returns the event bus.
func (c *ServiceContainer) GetEventBus() shared.EventBus {
	return c.EventBus
}

// GetCleanupService returns the cleanup service.
func (c *ServiceContainer) GetCleanupService() *services.CleanupService {
	return c.CleanupService
}

// ============================================================================
// HANDLER CONTAINER - HTTP handlers and routing
// ============================================================================

// HandlerContainer manages HTTP handlers and routing.
// Single Responsibility: HTTP request handling only.
type HandlerContainer struct {
	// Handlers (with aliases for backward compatibility)
	NodeHandler     *v1handlers.MemoryHandler
	Memory          *v1handlers.MemoryHandler // Alias
	CategoryHandler *v1handlers.CategoryHandler
	Category        *v1handlers.CategoryHandler // Alias
	HealthHandler   *v1handlers.HealthHandler
	MetricsHandler  http.HandlerFunc
	
	// Router
	Router http.Handler
	
	// Middleware chain
	Middleware []func(http.Handler) http.Handler
}

// NewHandlerContainer creates a new handler container.
func NewHandlerContainer(services IServiceContainer, infra IInfrastructureContainer) (*HandlerContainer, error) {
	c := &HandlerContainer{}
	
	if err := c.initialize(services, infra); err != nil {
		return nil, err
	}
	
	return c, nil
}

// ============================================================================
// INTERFACE IMPLEMENTATIONS FOR HANDLER CONTAINER
// ============================================================================

// GetRouter returns the HTTP router.
func (c *HandlerContainer) GetRouter() http.Handler {
	return c.Router
}

// GetMiddleware returns the middleware chain.
func (c *HandlerContainer) GetMiddleware() []func(http.Handler) http.Handler {
	return c.Middleware
}

// GetNodeHandler returns the node handler.
func (c *HandlerContainer) GetNodeHandler() interface{} {
	return c.NodeHandler
}

// GetCategoryHandler returns the category handler.
func (c *HandlerContainer) GetCategoryHandler() interface{} {
	return c.CategoryHandler
}

// GetHealthHandler returns the health handler.
func (c *HandlerContainer) GetHealthHandler() interface{} {
	return c.HealthHandler
}

// GetMetricsHandler returns the metrics handler.
func (c *HandlerContainer) GetMetricsHandler() http.HandlerFunc {
	return c.MetricsHandler
}

// ============================================================================
// APPLICATION CONTAINER - Root container that orchestrates all others
// ============================================================================

// ApplicationContainer is the root container that manages all sub-containers.
// This replaces the God Container with proper separation of concerns.
type ApplicationContainer struct {
	// Sub-containers with clear responsibilities (using interfaces)
	Infrastructure IInfrastructureContainer
	Repositories   IRepositoryContainer
	Services       IServiceContainer
	Handlers       IHandlerContainer
	
	// Application metadata
	Version       string
	Environment   string
	StartTime     time.Time
	coldStart     bool
}

// NewApplicationContainer creates the root application container.
func NewApplicationContainer(cfg *config.Config) (*ApplicationContainer, error) {
	// Create containers in dependency order
	infra, err := NewInfrastructureContainer(cfg)
	if err != nil {
		return nil, err
	}
	
	repos, err := NewRepositoryContainer(infra)
	if err != nil {
		infra.Shutdown()
		return nil, err
	}
	
	services, err := NewServiceContainer(repos, infra)
	if err != nil {
		infra.Shutdown()
		return nil, err
	}
	
	handlers, err := NewHandlerContainer(services, infra)
	if err != nil {
		infra.Shutdown()
		return nil, err
	}
	
	return &ApplicationContainer{
		Infrastructure: infra,
		Repositories:   repos,
		Services:       services,
		Handlers:       handlers,
		Version:        cfg.Version,
		Environment:    string(cfg.Environment),
		StartTime:      time.Now(),
		coldStart:      true,
	}, nil
}

// Shutdown gracefully shuts down all containers.
func (c *ApplicationContainer) Shutdown(ctx context.Context) error {
	// Shutdown in reverse order of initialization
	// Handlers don't need shutdown
	// Services don't need shutdown
	// Repositories don't need shutdown
	
	// Only infrastructure needs shutdown
	return c.Infrastructure.Shutdown()
}

// GetHTTPHandler returns the main HTTP handler.
func (c *ApplicationContainer) GetHTTPHandler() http.Handler {
	return c.Handlers.GetRouter()
}

// Health returns the application health status.
func (c *ApplicationContainer) Health(ctx context.Context) map[string]interface{} {
	return map[string]interface{}{
		"status":      "healthy",
		"version":     c.Version,
		"environment": c.Environment,
		"uptime":      time.Since(c.StartTime).String(),
		"cold_start":  c.coldStart,
	}
}

// ============================================================================
// BACKWARD COMPATIBILITY METHODS
// ============================================================================
// These methods maintain compatibility with existing code that expects
// the old Container interface. They delegate to the appropriate sub-containers.

// Validate ensures all containers are properly initialized and configured.
func (c *ApplicationContainer) Validate() error {
	if c.Infrastructure == nil {
		return errors.Internal("INFRASTRUCTURE_CONTAINER_NIL", "Infrastructure container is nil").
			WithOperation("ValidateContainers").
			WithResource("application_container").
			Build()
	}
	if c.Repositories == nil {
		return errors.Internal("REPOSITORY_CONTAINER_NIL", "Repository container is nil").
			WithOperation("ValidateContainers").
			WithResource("application_container").
			Build()
	}
	if c.Services == nil {
		return errors.Internal("SERVICE_CONTAINER_NIL", "Service container is nil").
			WithOperation("ValidateContainers").
			WithResource("application_container").
			Build()
	}
	if c.Handlers == nil {
		return errors.Internal("HANDLER_CONTAINER_NIL", "Handler container is nil").
			WithOperation("ValidateContainers").
			WithResource("application_container").
			Build()
	}
	
	// Validate key components are initialized
	if c.Infrastructure.GetLogger() == nil {
		return errors.Internal("LOGGER_NIL", "Logger is not initialized").
			WithOperation("ValidateContainers").
			WithResource("infrastructure_container").
			Build()
	}
	if c.Infrastructure.GetCache() == nil {
		return errors.Internal("CACHE_NIL", "Cache is not initialized").
			WithOperation("ValidateContainers").
			WithResource("infrastructure_container").
			Build()
	}
	if c.Handlers.GetRouter() == nil {
		return errors.Internal("ROUTER_NIL", "Router is not initialized").
			WithOperation("ValidateContainers").
			WithResource("handler_container").
			Build()
	}
	
	return nil
}

// SetColdStartInfo updates cold start tracking information.
func (c *ApplicationContainer) SetColdStartInfo(coldStartTime time.Time, isColdStart bool) {
	c.coldStart = isColdStart
	if !coldStartTime.IsZero() {
		c.StartTime = coldStartTime
	}
}

// ============================================================================
// INTERFACE IMPLEMENTATIONS FOR APPLICATION CONTAINER
// ============================================================================

// GetInfrastructure returns the infrastructure container.
func (c *ApplicationContainer) GetInfrastructure() IInfrastructureContainer {
	return c.Infrastructure
}

// GetRepositories returns the repository container.
func (c *ApplicationContainer) GetRepositories() IRepositoryContainer {
	return c.Repositories
}

// GetServices returns the service container.
func (c *ApplicationContainer) GetServices() IServiceContainer {
	return c.Services
}

// GetHandlers returns the handler container.
func (c *ApplicationContainer) GetHandlers() IHandlerContainer {
	return c.Handlers
}

// GetVersion returns the application version.
func (c *ApplicationContainer) GetVersion() string {
	return c.Version
}

// GetEnvironment returns the application environment.
func (c *ApplicationContainer) GetEnvironment() string {
	return c.Environment
}

// GetStartTime returns the application start time.
func (c *ApplicationContainer) GetStartTime() time.Time {
	return c.StartTime
}

// IsColdStart returns whether this is a cold start.
func (c *ApplicationContainer) IsColdStart() bool {
	return c.coldStart
}

// GetRouter returns the HTTP router from the handler container.
func (c *ApplicationContainer) GetRouter() *chi.Mux {
	if c.Handlers != nil {
		if router, ok := c.Handlers.GetRouter().(*chi.Mux); ok {
			return router
		}
	}
	return nil
}

// ============================================================================
// CONTAINER INITIALIZATION HELPERS
// ============================================================================

// initialize sets up the infrastructure container.
func (c *InfrastructureContainer) initialize() error {
	// Initialize observability components
	if err := c.initializeObservability(); err != nil {
		return errors.Internal("OBSERVABILITY_INIT_FAILED", "Failed to initialize observability").
			WithOperation("initialize").
			WithResource("infrastructure_container").
			WithCause(err).
			Build()
	}
	
	// Initialize AWS clients
	if err := c.initializeAWSClients(); err != nil {
		return errors.Internal("AWS_CLIENTS_INIT_FAILED", "Failed to initialize AWS clients").
			WithOperation("initialize").
			WithResource("infrastructure_container").
			WithCause(err).
			Build()
	}
	
	// Initialize cache
	if err := c.initializeCache(); err != nil {
		return errors.Internal("CACHE_INIT_FAILED", "Failed to initialize cache").
			WithOperation("initialize").
			WithResource("infrastructure_container").
			WithCause(err).
			Build()
	}
	
	// Initialize store
	if err := c.initializeStore(); err != nil {
		return errors.Internal("STORE_INIT_FAILED", "Failed to initialize store").
			WithOperation("initialize").
			WithResource("infrastructure_container").
			WithCause(err).
			Build()
	}
	
	return nil
}

// initialize sets up the repository container.
func (c *RepositoryContainer) initialize(infra IInfrastructureContainer) error {
	// Create repository configuration
	cfg := infra.GetConfig()
	repoConfig := initialization.RepositoryConfig{
		TableName:       cfg.Database.TableName,
		IndexName:       cfg.Database.IndexName,
		DynamoDBClient:  infra.GetDynamoDBClient(),
		Logger:          infra.GetLogger(),
		EnableCaching:   cfg.Features.EnableCaching,
	}
	
	// Initialize repository services
	services, err := initialization.InitializeRepositoryLayer(repoConfig)
	if err != nil {
		return errors.Internal("REPOSITORY_INIT_FAILED", "Failed to initialize repository layer").
			WithOperation("initialize").
			WithResource("repository_container").
			WithCause(err).
			Build()
	}
	
	// Set up combined repositories
	c.Node = services.NodeRepository
	c.Edge = services.EdgeRepository
	c.Category = services.CategoryRepository
	c.Graph = services.GraphRepository
	c.Keyword = services.KeywordRepository
	c.Transactional = services.TransactionalRepository
	c.Idempotency = services.IdempotencyStore
	
	// Set up CQRS readers/writers
	// Note: Reader/Writer interfaces removed - using combined repositories directly
	
	// Set up specialized repositories
	c.GraphRepository = services.GraphRepository
	c.KeywordRepository = services.KeywordRepository
	c.IdempotencyStore = services.IdempotencyStore
	
	// Set up Unit of Work
	c.UnitOfWorkFactory = services.UnitOfWorkFactory
	
	// Initialize repository factory for decorators
	factoryConfig := repository.DefaultFactoryConfig()
	if cfg.Environment == config.Production {
		factoryConfig = repository.ProductionFactoryConfig()
		factoryConfig.EnableCaching = cfg.Features.EnableCaching
		factoryConfig.EnableMetrics = cfg.Features.EnableMetrics
		factoryConfig.EnableRetries = cfg.Features.EnableRetries
		factoryConfig.EnableLogging = cfg.Features.EnableLogging
	} else if cfg.Environment == config.Development {
		factoryConfig = repository.DevelopmentFactoryConfig()
		factoryConfig.EnableCaching = cfg.Features.EnableCaching
		factoryConfig.EnableMetrics = cfg.Features.EnableMetrics
		factoryConfig.EnableLogging = cfg.Features.VerboseLogging
	}
	
	c.RepositoryFactory = repository.NewRepositoryFactory(factoryConfig, infra.GetLogger())
	
	// Apply decorators to repositories using the factory
	if c.RepositoryFactory != nil && (cfg.Features.EnableCaching || cfg.Features.EnableMetrics || cfg.Features.EnableLogging) {
		// Decorate repositories with cross-cutting concerns
		c.Node = c.RepositoryFactory.CreateNodeRepository(
			c.Node,
			infra.GetLogger(),
			infra.GetCache(),
			infra.GetMetricsCollector(),
		)
		c.Edge = c.RepositoryFactory.CreateEdgeRepository(
			c.Edge,
			infra.GetLogger(),
			infra.GetCache(),
			infra.GetMetricsCollector(),
		)
		c.Category = c.RepositoryFactory.CreateCategoryRepository(
			c.Category,
			infra.GetLogger(),
			infra.GetCache(),
			infra.GetMetricsCollector(),
		)
	}
	
	return nil
}

// initialize sets up the service container.
func (c *ServiceContainer) initialize(repos IRepositoryContainer, infra IInfrastructureContainer) error {
	// Initialize domain services
	c.ConnectionAnalyzer = domainServices.NewConnectionAnalyzer(0.3, 5, 0.2)
	
	// Create service configuration
	cfg := infra.GetConfig()
	
	// Initialize event bus with EventBridge
	eventBridgeClient := infra.GetEventBridgeClient()
	if eventBridgeClient != nil && cfg.Events.Provider == "eventbridge" {
		// Use real EventBridge publisher
		eventBusName := cfg.Events.EventBusName
		if eventBusName == "" {
			eventBusName = "default"
		}
		eventPublisher := messaging.NewEventBridgePublisher(
			eventBridgeClient,
			eventBusName,
			"brain2-backend",
		)
		// Wrap in async publisher for better performance
		asyncPublisher := messaging.NewAsyncEventPublisher(eventPublisher, 1000)
		c.EventBus = messaging.NewEventBusAdapter(asyncPublisher)
	} else {
		// Fallback to mock for local development or when disabled
		c.EventBus = shared.NewMockEventBus()
	}
	serviceConfig := initialization.ServiceConfig{
		Config:        cfg,
		EventBus:      c.EventBus,
		Logger:        infra.GetLogger(),
		EnableCaching: cfg.Features.EnableCaching,
	}
	
	// Convert RepositoryContainer to RepositoryServices for initialization
	repoServices := &initialization.RepositoryServices{
		Store:              infra.GetStore(),
		NodeRepository:     repos.GetNodeRepository(),
		EdgeRepository:     repos.GetEdgeRepository(),
		CategoryRepository: repos.GetCategoryRepository(),
		GraphRepository:    repos.GetGraphRepository(),
		ConnectionAnalyzer: c.ConnectionAnalyzer,
		IdempotencyStore:   repos.GetIdempotencyStore(),
		UnitOfWorkFactory:  repos.GetUnitOfWorkFactory(),
	}
	
	// Initialize application services
	appServices := initialization.InitializeApplicationServices(serviceConfig, repoServices)
	
	// Set up command services
	c.NodeCommandService = appServices.NodeAppService
	c.CategoryCommandService = appServices.CategoryAppService
	c.CleanupService = appServices.CleanupService
	
	// Set up query services
	c.NodeQueryService = appServices.NodeQueryService
	c.CategoryQueryService = appServices.CategoryQueryService
	c.GraphQueryService = appServices.GraphQueryService
	
	return nil
}

// initialize sets up the handler container.
func (c *HandlerContainer) initialize(services IServiceContainer, infra IInfrastructureContainer) error {
	// Initialize handlers
	c.initializeHandlers(services, infra)
	
	// Initialize router
	c.initializeRouter(infra)
	
	// Initialize middleware
	c.initializeMiddleware(infra)
	
	return nil
}

// ============================================================================
// INFRASTRUCTURE CONTAINER INITIALIZATION HELPERS
// ============================================================================

// initializeObservability sets up logging, metrics, and tracing.
func (c *InfrastructureContainer) initializeObservability() error {
	observabilityConfig := initialization.ObservabilityConfig{
		Config:  c.Config,
		AppName: "brain2-backend",
		Version: c.Config.Version,
	}
	
	services, err := initialization.InitializeObservability(observabilityConfig)
	if err != nil {
		return err
	}
	
	c.Logger = services.Logger
	c.MetricsCollector = services.MetricsCollector
	// TODO: Set TracerProvider when tracing is fully implemented
	
	// Add logger shutdown to cleanup functions
	c.shutdownFuncs = append(c.shutdownFuncs, func() error {
		return safeLoggerSync(c.Logger)
	})
	
	return nil
}

// initializeAWSClients sets up AWS service clients.
func (c *InfrastructureContainer) initializeAWSClients() error {
	clients, err := initialization.InitializeAWSClients()
	if err != nil {
		return err
	}
	
	c.DynamoDBClient = clients.DynamoDBClient
	c.EventBridgeClient = clients.EventBridgeClient
	
	return nil
}

// initializeCache sets up the caching layer.
func (c *InfrastructureContainer) initializeCache() error {
	// Initialize cache based on configuration
	if c.Config.Features.EnableCaching {
		// Use memory cache implementation with reasonable defaults
		c.Cache = cache.NewInMemoryCache(1000, 30*time.Minute) // 1000 items, 30min TTL
	} else {
		// Use no-op cache
		c.Cache = cache.NewNoOpCache()
	}
	
	return nil
}

// initializeStore sets up the persistence store.
func (c *InfrastructureContainer) initializeStore() error {
	// Create store configuration
	storeConfig := persistence.StoreConfig{
		TableName:      c.Config.Database.TableName,
		IndexNames:     map[string]string{"GSI1": "GSI1", "GSI2": "GSI2"}, // Default GSIs
		TimeoutMs:      5000,
		RetryAttempts:  3,
		ConsistentRead: false, // Use eventual consistency for better performance
	}
	
	// Create DynamoDB store
	c.Store = persistence.NewDynamoDBStore(
		c.DynamoDBClient,
		storeConfig,
		c.Logger,
	)
	
	return nil
}

// ============================================================================
// HANDLER CONTAINER INITIALIZATION HELPERS
// ============================================================================

// initializeHandlers sets up HTTP handlers.
func (c *HandlerContainer) initializeHandlers(services IServiceContainer, infra IInfrastructureContainer) {
	// Create cold start provider (simple implementation)
	coldStartProvider := &simpleColdStartProvider{startTime: time.Now()}
	
	// Memory handler (nodes/edges)
	nodeCmd := services.GetNodeCommandService()
	nodeQuery := services.GetNodeQueryService()
	graphQuery := services.GetGraphQueryService()
	if nodeCmd != nil && nodeQuery != nil && graphQuery != nil {
		c.NodeHandler = v1handlers.NewMemoryHandler(
			nodeCmd,
			nodeQuery,
			graphQuery,
			infra.GetEventBridgeClient(),
			coldStartProvider,
		)
		c.Memory = c.NodeHandler // Alias
	}
	
	// Category handler
	catCmd := services.GetCategoryCommandService()
	catQuery := services.GetCategoryQueryService()
	if catCmd != nil && catQuery != nil {
		c.CategoryHandler = v1handlers.NewCategoryHandler(
			catCmd,
			catQuery,
		)
		c.Category = c.CategoryHandler // Alias
	}
	
	// Health handler
	c.HealthHandler = v1handlers.NewHealthHandler()
	
	// Metrics handler (placeholder)
	c.MetricsHandler = func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"healthy"}`))
	}
}

// initializeRouter sets up the HTTP router with all routes.
func (c *HandlerContainer) initializeRouter(_ IInfrastructureContainer) {
	router := chi.NewRouter()
	
	// Health endpoints
	if c.HealthHandler != nil {
		router.Get("/health", c.HealthHandler.Check)
		router.Get("/ready", c.HealthHandler.Ready)
	}
	
	// Metrics endpoint
	router.Get("/metrics", c.MetricsHandler)
	
	// API routes
	router.Route("/api/v1", func(r chi.Router) {
		// Apply authentication middleware to all API routes
		r.Use(v1handlers.Authenticator)
		
		// Node routes
		if c.Memory != nil {
			r.Route("/nodes", func(r chi.Router) {
				r.Post("/", c.Memory.CreateNode)
				r.Get("/", c.Memory.ListNodes)
				r.Get("/{nodeId}", c.Memory.GetNode)
				r.Put("/{nodeId}", c.Memory.UpdateNode)
				r.Delete("/{nodeId}", c.Memory.DeleteNode)
				r.Post("/bulk-delete", c.Memory.BulkDeleteNodes)
			})
			
			// Graph routes
			r.Get("/graph-data", c.Memory.GetGraphData)
		}
		
		// Category routes
		if c.Category != nil {
			r.Route("/categories", func(r chi.Router) {
				r.Post("/", c.Category.CreateCategory)
				r.Get("/", c.Category.ListCategories)
				r.Get("/{categoryId}", c.Category.GetCategory)
				r.Put("/{categoryId}", c.Category.UpdateCategory)
				r.Delete("/{categoryId}", c.Category.DeleteCategory)
				
				// Category-Node relationships
				r.Post("/{categoryId}/nodes", c.Category.AssignNodeToCategory)
				r.Get("/{categoryId}/nodes", c.Category.GetNodesInCategory)
				r.Delete("/{categoryId}/nodes/{nodeId}", c.Category.RemoveNodeFromCategory)
			})
			
			// Node categorization routes
			r.Get("/nodes/{nodeId}/categories", c.Category.GetNodeCategories)
			r.Post("/nodes/{nodeId}/categories", c.Category.CategorizeNode)
		}
	})
	
	c.Router = router
}

// initializeMiddleware sets up middleware chain.
func (c *HandlerContainer) initializeMiddleware(_ IInfrastructureContainer) {
	// Initialize basic middleware
	// TODO: Add proper middleware based on configuration
	c.Middleware = []func(http.Handler) http.Handler{
		// Add basic middleware here
	}
}

// Helper types for HandlerContainer
type simpleColdStartProvider struct {
	startTime time.Time
}

func (p *simpleColdStartProvider) GetTimeSinceColdStart() time.Duration {
	return time.Since(p.startTime)
}

func (p *simpleColdStartProvider) IsPostColdStartRequest() bool {
	return time.Since(p.startTime) > 100*time.Millisecond
}

type simpleHealthChecker struct {
	config *config.Config
}

func (h *simpleHealthChecker) Health(ctx context.Context) map[string]string {
	return map[string]string{
		"status":      "healthy",
		"environment": string(h.config.Environment),
		"version":     h.config.Version,
	}
}

// ============================================================================
// UTILITY FUNCTIONS
// ============================================================================

// safeLoggerSync safely syncs a zap logger, ignoring known stderr/stdout sync errors.
// This is needed because zap logger sync can fail with "sync /dev/stderr: invalid argument"
// in certain environments (like tests), which is harmless but causes test failures.
func safeLoggerSync(logger *zap.Logger) error {
	if logger == nil {
		return nil
	}
	
	err := logger.Sync()
	if err != nil {
		// Ignore the known stderr/stdout sync errors that are harmless
		// This is a known issue with zap logger in test environments
		errStr := err.Error()
		if errStr == "sync /dev/stderr: invalid argument" || 
		   errStr == "sync /dev/stdout: invalid argument" ||
		   errStr == "sync /dev/stderr: inappropriate ioctl for device" ||
		   errStr == "sync /dev/stdout: inappropriate ioctl for device" {
			return nil // Ignore these harmless sync errors
		}
		return err // Return actual errors
	}
	
	return nil
}