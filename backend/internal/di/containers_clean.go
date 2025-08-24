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
	"brain2-backend/internal/domain/shared"
	domainServices "brain2-backend/internal/domain/services"
	v1handlers "brain2-backend/internal/interfaces/http/v1/handlers"
	"brain2-backend/internal/infrastructure/observability"
	"brain2-backend/internal/infrastructure/persistence/cache"
	"brain2-backend/internal/repository"

	awsDynamodb "github.com/aws/aws-sdk-go-v2/service/dynamodb"
	awsEventbridge "github.com/aws/aws-sdk-go-v2/service/eventbridge"
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
	Cache            cache.Cache
	MetricsCollector *observability.Collector
	TracerProvider   *observability.TracerProvider
	Store            interface{} // Persistence store
	
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
	EdgeCommandService     interface{} // TODO: Add edge service
	
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
	EdgeHandler     interface{} // TODO: Add edge handler
	HealthHandler   http.HandlerFunc
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
func NewApplicationContainer() (*ApplicationContainer, error) {
	// Load configuration
	cfg := config.LoadConfig()
	
	// Create containers in dependency order
	infra, err := NewInfrastructureContainer(&cfg)
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
// CONTAINER INITIALIZATION HELPERS
// ============================================================================

// initialize sets up the infrastructure container.
func (c *InfrastructureContainer) initialize() error {
	// Initialize logger
	var err error
	c.Logger, err = zap.NewProduction()
	if err != nil {
		c.Logger, _ = zap.NewDevelopment()
	}
	c.shutdownFuncs = append(c.shutdownFuncs, func() error {
		return c.Logger.Sync()
	})
	
	// Initialize cache
	// TODO: Implement proper cache
	c.Cache = nil
	
	// Initialize metrics
	// TODO: Fix metrics initialization
	c.MetricsCollector = nil
	
	// AWS clients initialized separately
	
	return nil
}

// initialize sets up the repository container.
func (c *RepositoryContainer) initialize(infra *InfrastructureContainer) error {
	// TODO: Initialize repositories properly
	// For now, repositories will be wired in factories.go
	
	return nil
}

// initialize sets up the service container.
func (c *ServiceContainer) initialize(repos *RepositoryContainer, infra *InfrastructureContainer) error {
	// Initialize domain services
	c.ConnectionAnalyzer = domainServices.NewConnectionAnalyzer(0.3, 5, 0.2)
	// TODO: Initialize event bus properly
	c.EventBus = nil
	
	// TODO: Initialize services properly
	// For now, leave services nil - they'll be wired in factories.go
	return nil
}

// initialize sets up the handler container.
func (c *HandlerContainer) initialize(services *ServiceContainer, infra *InfrastructureContainer) error {
	// TODO: Initialize handlers properly
	// For now, handlers will be wired in factories.go
	return nil
}

// Helper methods implementation details would go here...
// (initializeAWSClients, getFactoryConfig, initializeRepositories, etc.)