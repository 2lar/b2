// Package di provides a centralized dependency injection container.
package di

import (
	"context"
	"fmt"
	"log"
	"time"

	"brain2-backend/infrastructure/dynamodb"
	"brain2-backend/internal/config"
	"brain2-backend/internal/handlers"
	"brain2-backend/internal/repository"
	categoryService "brain2-backend/internal/service/category"
	memoryService "brain2-backend/internal/service/memory"

	awsConfig "github.com/aws/aws-sdk-go-v2/config"
	awsDynamodb "github.com/aws/aws-sdk-go-v2/service/dynamodb"
	awsEventbridge "github.com/aws/aws-sdk-go-v2/service/eventbridge"
	"github.com/go-chi/chi/v5"
)

// Container holds all application dependencies with lifecycle management.
type Container struct {
	// Configuration
	Config *config.Config

	// AWS Clients
	DynamoDBClient    *awsDynamodb.Client
	EventBridgeClient *awsEventbridge.Client

	// Repository Layer - Segregated interfaces for better dependency management
	NodeRepository         repository.NodeRepository
	EdgeRepository         repository.EdgeRepository
	KeywordRepository      repository.KeywordRepository
	TransactionalRepository repository.TransactionalRepository
	CategoryRepository     repository.CategoryRepository
	GraphRepository        repository.GraphRepository
	
	// Composed repository for backward compatibility
	Repository       repository.Repository
	IdempotencyStore repository.IdempotencyStore

	// Service Layer  
	MemoryService   memoryService.Service
	CategoryService categoryService.Service

	// Handler Layer
	MemoryHandler   *handlers.MemoryHandler
	CategoryHandler *handlers.CategoryHandler

	// HTTP Router
	Router *chi.Mux

	// Middleware components (for monitoring/observability)
	middlewareConfig map[string]any

	// Lifecycle management
	shutdownFunctions []func() error
}

// NewContainer creates and initializes a new dependency injection container.
func NewContainer() (*Container, error) {
	container := &Container{
		shutdownFunctions: make([]func() error, 0),
		middlewareConfig:  make(map[string]any),
	}

	if err := container.initialize(); err != nil {
		return nil, fmt.Errorf("failed to initialize container: %w", err)
	}

	return container, nil
}

// initialize sets up all dependencies in the correct order.
func (c *Container) initialize() error {
	// 1. Load configuration
	cfg := config.LoadConfig()
	c.Config = &cfg

	// 2. Initialize AWS clients
	if err := c.initializeAWSClients(); err != nil {
		return fmt.Errorf("failed to initialize AWS clients: %w", err)
	}

	// 3. Initialize repository layer
	if err := c.initializeRepository(); err != nil {
		return fmt.Errorf("failed to initialize repository: %w", err)
	}

	// 4. Initialize service layer
	c.initializeServices()

	// 5. Initialize handler layer
	c.initializeHandlers()

	// 6. Initialize middleware configuration
	c.initializeMiddleware()

	// 7. Initialize HTTP router
	c.initializeRouter()

	log.Println("Dependency injection container initialized successfully")
	return nil
}

// initializeAWSClients sets up AWS service clients.
func (c *Container) initializeAWSClients() error {
	awsCfg, err := awsConfig.LoadDefaultConfig(context.TODO())
	if err != nil {
		return fmt.Errorf("failed to load AWS config: %w", err)
	}

	// DynamoDB client
	c.DynamoDBClient = awsDynamodb.NewFromConfig(awsCfg)

	// EventBridge client
	c.EventBridgeClient = awsEventbridge.NewFromConfig(awsCfg)

	return nil
}

// initializeRepository sets up the repository layer.
func (c *Container) initializeRepository() error {
	if c.DynamoDBClient == nil {
		return fmt.Errorf("DynamoDB client not initialized")
	}
	if c.Config == nil {
		return fmt.Errorf("config not loaded")
	}

	// Initialize segregated repositories
	c.NodeRepository = dynamodb.NewNodeRepository(c.DynamoDBClient, c.Config.TableName, c.Config.IndexName)
	c.EdgeRepository = dynamodb.NewEdgeRepository(c.DynamoDBClient, c.Config.TableName, c.Config.IndexName)
	c.KeywordRepository = dynamodb.NewKeywordRepository(c.DynamoDBClient, c.Config.TableName, c.Config.IndexName)
	c.TransactionalRepository = dynamodb.NewTransactionalRepository(c.DynamoDBClient, c.Config.TableName, c.Config.IndexName)
	c.CategoryRepository = dynamodb.NewCategoryRepository(c.DynamoDBClient, c.Config.TableName, c.Config.IndexName)
	c.GraphRepository = dynamodb.NewGraphRepository(c.DynamoDBClient, c.Config.TableName, c.Config.IndexName)

	// Initialize composed repository for backward compatibility
	c.Repository = dynamodb.NewRepository(c.DynamoDBClient, c.Config.TableName, c.Config.IndexName)

	// Initialize idempotency store with 24-hour TTL
	c.IdempotencyStore = dynamodb.NewIdempotencyStore(c.DynamoDBClient, c.Config.TableName, 24*time.Hour)

	return nil
}

// initializeServices sets up the service layer.
func (c *Container) initializeServices() {
	// Initialize memory service with segregated repositories
	c.MemoryService = memoryService.NewServiceWithIdempotency(
		c.NodeRepository,
		c.EdgeRepository,
		c.KeywordRepository,
		c.TransactionalRepository,
		c.GraphRepository,
		c.IdempotencyStore,
	)
	
	// Initialize category service with segregated repositories
	c.CategoryService = categoryService.NewService(c.CategoryRepository, c.NodeRepository)
}

// initializeHandlers sets up the handler layer.
func (c *Container) initializeHandlers() {
	c.MemoryHandler = handlers.NewMemoryHandler(c.MemoryService, c.EventBridgeClient)
	c.CategoryHandler = handlers.NewCategoryHandler(c.CategoryService)
}

// initializeMiddleware sets up middleware configuration.
func (c *Container) initializeMiddleware() {
	// Store middleware configuration for monitoring/observability
	c.middlewareConfig["request_id"] = map[string]any{
		"enabled": true,
		"header_name": "X-Request-ID",
	}
	
	c.middlewareConfig["circuit_breaker"] = map[string]any{
		"enabled": true,
		"api_routes": map[string]any{
			"name": "api-routes",
			"max_requests": 3,
			"interval_seconds": 10,
			"timeout_seconds": 30,
			"failure_threshold": 0.6,
			"min_requests": 3,
		},
	}
	
	c.middlewareConfig["timeout"] = map[string]any{
		"enabled": true,
		"default_timeout_seconds": 30,
	}
	
	c.middlewareConfig["recovery"] = map[string]any{
		"enabled": true,
		"log_stack_trace": true,
	}
	
	log.Println("Middleware configuration initialized")
}

// initializeRouter sets up the HTTP router with all routes.
func (c *Container) initializeRouter() {
	c.Router = setupRouter(c.MemoryHandler, c.CategoryHandler)
}

// Shutdown gracefully shuts down all container components.
func (c *Container) Shutdown(ctx context.Context) error {
	log.Println("Shutting down dependency injection container...")

	var errors []error
	
	// Execute shutdown functions in reverse order
	for i := len(c.shutdownFunctions) - 1; i >= 0; i-- {
		if err := c.shutdownFunctions[i](); err != nil {
			errors = append(errors, err)
			log.Printf("Error during shutdown: %v", err)
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("shutdown completed with %d errors", len(errors))
	}

	log.Println("Container shutdown completed successfully")
	return nil
}

// AddShutdownFunction adds a function to be called during container shutdown.
func (c *Container) AddShutdownFunction(fn func() error) {
	c.shutdownFunctions = append(c.shutdownFunctions, fn)
}

// Validate ensures all critical dependencies are properly initialized.
func (c *Container) Validate() error {
	if c.Config == nil {
		return fmt.Errorf("config not initialized")
	}
	if c.DynamoDBClient == nil {
		return fmt.Errorf("DynamoDB client not initialized")
	}
	if c.EventBridgeClient == nil {
		return fmt.Errorf("EventBridge client not initialized")
	}
	
	// Validate segregated repositories
	if c.NodeRepository == nil {
		return fmt.Errorf("node repository not initialized")
	}
	if c.EdgeRepository == nil {
		return fmt.Errorf("edge repository not initialized")
	}
	if c.KeywordRepository == nil {
		return fmt.Errorf("keyword repository not initialized")
	}
	if c.TransactionalRepository == nil {
		return fmt.Errorf("transactional repository not initialized")
	}
	if c.CategoryRepository == nil {
		return fmt.Errorf("category repository not initialized")
	}
	if c.GraphRepository == nil {
		return fmt.Errorf("graph repository not initialized")
	}
	
	// Validate composed repository (backward compatibility)
	if c.Repository == nil {
		return fmt.Errorf("composed repository not initialized")
	}
	if c.IdempotencyStore == nil {
		return fmt.Errorf("idempotency store not initialized")
	}
	
	if c.MemoryService == nil {
		return fmt.Errorf("memory service not initialized")
	}
	if c.CategoryService == nil {
		return fmt.Errorf("category service not initialized")
	}
	if c.MemoryHandler == nil {
		return fmt.Errorf("memory handler not initialized")
	}
	if c.CategoryHandler == nil {
		return fmt.Errorf("category handler not initialized")
	}
	if c.Router == nil {
		return fmt.Errorf("router not initialized")
	}

	return nil
}

// GetRouter returns the configured HTTP router.
func (c *Container) GetRouter() *chi.Mux {
	return c.Router
}

// GetMiddlewareConfig returns the middleware configuration for monitoring
func (c *Container) GetMiddlewareConfig() map[string]any {
	return c.middlewareConfig
}

// Health returns the health status of all components.
func (c *Container) Health(ctx context.Context) map[string]string {
	health := make(map[string]string)
	
	health["container"] = "healthy"
	health["config"] = "loaded"
	
	// Add health checks for individual components as needed
	if c.DynamoDBClient != nil {
		health["dynamodb"] = "connected"
	} else {
		health["dynamodb"] = "not_connected"
	}
	
	if c.EventBridgeClient != nil {
		health["eventbridge"] = "connected"
	} else {
		health["eventbridge"] = "not_connected"
	}
	
	// Add middleware status
	if len(c.middlewareConfig) > 0 {
		health["middleware"] = "configured"
	} else {
		health["middleware"] = "not_configured"
	}

	return health
}

// InitializeContainer creates and returns a new dependency injection container.
// This is the main entry point for initializing the application.
func InitializeContainer() (*Container, error) {
	return NewContainer()
}