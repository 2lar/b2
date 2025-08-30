// Package di provides focused dependency injection containers following SOLID principles.
// This file replaces the God Container anti-pattern with specialized containers.
package di

import (
	"context"
	"fmt"
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
	"brain2-backend/internal/infrastructure/observability"
	"brain2-backend/internal/infrastructure/persistence"
	persistenceCache "brain2-backend/internal/infrastructure/persistence/cache"
	"brain2-backend/internal/repository"

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
// REPOSITORY CONTAINER - Data access layer with CQRS
// ============================================================================

// RepositoryContainer manages all repository dependencies.
// Single Responsibility: Data access and persistence only.
type RepositoryContainer struct {
	// Combined repositories (for backward compatibility)
	Node       repository.NodeRepository
	Edge       repository.EdgeRepository
	Category   repository.CategoryRepository
	Graph      repository.GraphRepository
	Keyword    repository.KeywordRepository
	Idempotency repository.IdempotencyStore
	
	// CQRS Readers - Read models
	NodeReader     repository.NodeReader
	EdgeReader     repository.EdgeReader
	CategoryReader repository.CategoryReader
	
	// CQRS Writers - Write models
	NodeWriter     repository.NodeWriter
	EdgeWriter     repository.EdgeWriter
	CategoryWriter repository.CategoryWriter
	
	// Specialized repositories
	GraphRepository    repository.GraphRepository
	KeywordRepository  repository.KeywordRepository
	IdempotencyStore   repository.IdempotencyStore
	
	// Unit of Work for transactional boundaries
	UnitOfWork        repository.UnitOfWork
	UnitOfWorkFactory repository.UnitOfWorkFactory
	
	// Repository Factory for decorators
	RepositoryFactory repository.RepositoryFactory
}

// NewRepositoryContainer creates a new repository container.
func NewRepositoryContainer(infra *InfrastructureContainer) (*RepositoryContainer, error) {
	c := &RepositoryContainer{}
	
	if err := c.initialize(infra); err != nil {
		return nil, err
	}
	
	return c, nil
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
func NewServiceContainer(repos *RepositoryContainer, infra *InfrastructureContainer) (*ServiceContainer, error) {
	c := &ServiceContainer{}
	
	if err := c.initialize(repos, infra); err != nil {
		return nil, err
	}
	
	return c, nil
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
func NewHandlerContainer(services *ServiceContainer, infra *InfrastructureContainer) (*HandlerContainer, error) {
	c := &HandlerContainer{}
	
	if err := c.initialize(services, infra); err != nil {
		return nil, err
	}
	
	return c, nil
}

// ============================================================================
// APPLICATION CONTAINER - Root container that orchestrates all others
// ============================================================================

// ApplicationContainer is the root container that manages all sub-containers.
// This replaces the God Container with proper separation of concerns.
type ApplicationContainer struct {
	// Sub-containers with clear responsibilities
	Infrastructure *InfrastructureContainer
	Repositories   *RepositoryContainer
	Services       *ServiceContainer
	Handlers       *HandlerContainer
	
	// Application metadata
	Version       string
	Environment   string
	StartTime     time.Time
	IsColdStart   bool
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
		IsColdStart:    true,
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
	return c.Handlers.Router
}

// Health returns the application health status.
func (c *ApplicationContainer) Health(ctx context.Context) map[string]interface{} {
	return map[string]interface{}{
		"status":      "healthy",
		"version":     c.Version,
		"environment": c.Environment,
		"uptime":      time.Since(c.StartTime).String(),
		"cold_start":  c.IsColdStart,
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
	if c.Infrastructure.Logger == nil {
		return fmt.Errorf("logger is not initialized")
	}
	if c.Infrastructure.Cache == nil {
		return fmt.Errorf("cache is not initialized")
	}
	if c.Handlers.Router == nil {
		return fmt.Errorf("router is not initialized")
	}
	
	return nil
}

// SetColdStartInfo updates cold start tracking information.
func (c *ApplicationContainer) SetColdStartInfo(coldStartTime time.Time, isColdStart bool) {
	c.IsColdStart = isColdStart
	if !coldStartTime.IsZero() {
		c.StartTime = coldStartTime
	}
}

// GetRouter returns the HTTP router from the handler container.
func (c *ApplicationContainer) GetRouter() *chi.Mux {
	if c.Handlers != nil {
		if router, ok := c.Handlers.Router.(*chi.Mux); ok {
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
		return fmt.Errorf("failed to initialize observability: %w", err)
	}
	
	// Initialize AWS clients
	if err := c.initializeAWSClients(); err != nil {
		return fmt.Errorf("failed to initialize AWS clients: %w", err)
	}
	
	// Initialize cache
	if err := c.initializeCache(); err != nil {
		return fmt.Errorf("failed to initialize cache: %w", err)
	}
	
	// Initialize store
	if err := c.initializeStore(); err != nil {
		return fmt.Errorf("failed to initialize store: %w", err)
	}
	
	return nil
}

// initialize sets up the repository container.
func (c *RepositoryContainer) initialize(infra *InfrastructureContainer) error {
	// Create repository configuration
	repoConfig := initialization.RepositoryConfig{
		TableName:       infra.Config.Database.TableName,
		IndexName:       infra.Config.Database.IndexName,
		DynamoDBClient:  infra.DynamoDBClient,
		Logger:          infra.Logger,
		EnableCaching:   infra.Config.Features.EnableCaching,
	}
	
	// Initialize repository services
	services, err := initialization.InitializeRepositoryLayer(repoConfig)
	if err != nil {
		return fmt.Errorf("failed to initialize repository layer: %w", err)
	}
	
	// Set up combined repositories
	c.Node = services.NodeRepository
	c.Edge = services.EdgeRepository
	c.Category = services.CategoryRepository
	c.Graph = services.GraphRepository
	c.Idempotency = services.IdempotencyStore
	
	// Set up CQRS readers/writers
	c.NodeReader = services.SafeGetNodeReader()
	c.EdgeReader = services.SafeGetEdgeReader()
	c.CategoryReader = services.SafeGetCategoryReader()
	c.EdgeWriter = services.SafeGetEdgeWriter()
	c.CategoryWriter = services.SafeGetCategoryWriter()
	
	// Set up specialized repositories
	c.GraphRepository = services.GraphRepository
	c.IdempotencyStore = services.IdempotencyStore
	
	// Set up Unit of Work
	c.UnitOfWorkFactory = services.UnitOfWorkFactory
	
	// Initialize repository factory for decorators
	// TODO: Implement repository factory if needed
	
	return nil
}

// initialize sets up the service container.
func (c *ServiceContainer) initialize(repos *RepositoryContainer, infra *InfrastructureContainer) error {
	// Initialize domain services
	c.ConnectionAnalyzer = domainServices.NewConnectionAnalyzer(0.3, 5, 0.2)
	
	// Initialize event bus (placeholder for now)
	// TODO: Initialize proper event bus with EventBridge
	c.EventBus = nil
	
	// Create service configuration
	serviceConfig := initialization.ServiceConfig{
		Config:        infra.Config,
		EventBus:      c.EventBus,
		Logger:        infra.Logger,
		EnableCaching: infra.Config.Features.EnableCaching,
	}
	
	// Convert RepositoryContainer to RepositoryServices for initialization
	repoServices := &initialization.RepositoryServices{
		NodeRepository:     repos.Node,
		EdgeRepository:     repos.Edge,
		CategoryRepository: repos.Category,
		GraphRepository:    repos.GraphRepository,
		ConnectionAnalyzer: c.ConnectionAnalyzer,
		IdempotencyStore:   repos.IdempotencyStore,
		UnitOfWorkFactory:  repos.UnitOfWorkFactory,
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
func (c *HandlerContainer) initialize(services *ServiceContainer, infra *InfrastructureContainer) error {
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
func (c *HandlerContainer) initializeHandlers(services *ServiceContainer, infra *InfrastructureContainer) {
	// Create cold start provider (simple implementation)
	coldStartProvider := &simpleColdStartProvider{startTime: time.Now()}
	
	// Memory handler (nodes/edges)
	if services.NodeCommandService != nil && services.NodeQueryService != nil && services.GraphQueryService != nil {
		c.NodeHandler = v1handlers.NewMemoryHandler(
			services.NodeCommandService,
			services.NodeQueryService,
			services.GraphQueryService,
			infra.EventBridgeClient,
			coldStartProvider,
		)
		c.Memory = c.NodeHandler // Alias
	}
	
	// Category handler
	if services.CategoryCommandService != nil && services.CategoryQueryService != nil {
		c.CategoryHandler = v1handlers.NewCategoryHandler(
			services.CategoryCommandService,
			services.CategoryQueryService,
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
func (c *HandlerContainer) initializeRouter(_ *InfrastructureContainer) {
	router := chi.NewRouter()
	
	// Health endpoints
	if c.HealthHandler != nil {
		router.Get("/health", c.HealthHandler.Check)
		router.Get("/ready", c.HealthHandler.Ready)
	}
	
	// Metrics endpoint
	router.Get("/metrics", c.MetricsHandler)
	
	// API routes
	router.Route("/api", func(r chi.Router) {
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
func (c *HandlerContainer) initializeMiddleware(_ *InfrastructureContainer) {
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