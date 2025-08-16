// Package di provides a centralized dependency injection container.
package di

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"brain2-backend/infrastructure/dynamodb"
	"brain2-backend/internal/application/adapters"
	// "brain2-backend/internal/application/queries"
	"brain2-backend/internal/application/services"
	"brain2-backend/internal/config"
	"brain2-backend/internal/domain"
	domainServices "brain2-backend/internal/domain/services"
	"brain2-backend/internal/handlers"
	"brain2-backend/internal/infrastructure/decorators"
	"brain2-backend/internal/repository"
	categoryService "brain2-backend/internal/service/category"
	memoryService "brain2-backend/internal/service/memory"

	awsConfig "github.com/aws/aws-sdk-go-v2/config"
	awsDynamodb "github.com/aws/aws-sdk-go-v2/service/dynamodb"
	awsEventbridge "github.com/aws/aws-sdk-go-v2/service/eventbridge"
	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"
)

// Container holds all application dependencies with lifecycle management.
// Enhanced for Phase 2 repository pattern excellence
type Container struct {
	// Configuration
	Config *config.Config
	
	// Cold start tracking
	ColdStartTime *time.Time
	IsColdStart   bool

	// AWS Clients
	DynamoDBClient    *awsDynamodb.Client
	EventBridgeClient *awsEventbridge.Client

	// Repository Layer - Phase 2 Enhanced Architecture
	NodeRepository         repository.NodeRepository
	EdgeRepository         repository.EdgeRepository
	KeywordRepository      repository.KeywordRepository
	TransactionalRepository repository.TransactionalRepository
	CategoryRepository     repository.CategoryRepository
	GraphRepository        repository.GraphRepository
	
	// Composed repository for backward compatibility
	Repository       repository.Repository
	IdempotencyStore repository.IdempotencyStore
	
	// Phase 2 Repository Pattern Enhancements
	RepositoryFactory    *repository.RepositoryFactory
	UnitOfWorkProvider   repository.UnitOfWorkProvider
	QueryExecutor        repository.QueryExecutor
	RepositoryManager    repository.RepositoryManager
	
	// Cross-cutting concerns
	Logger           *zap.Logger
	Cache            decorators.Cache
	MetricsCollector decorators.MetricsCollector

	// Legacy Service Layer (Phase 2 - will be deprecated)
	MemoryService   memoryService.Service
	CategoryService categoryService.Service
	
	// Phase 3: Application Service Layer (CQRS)
	NodeAppService      *services.NodeService
	// CategoryAppService  *services.CategoryService
	// NodeQueryService    *queries.NodeQueryService
	// CategoryQueryService *queries.CategoryQueryService
	
	// Domain Services
	ConnectionAnalyzer *domainServices.ConnectionAnalyzer
	EventBus          domain.EventBus
	UnitOfWork        repository.UnitOfWork

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

// initializeAWSClients sets up AWS service clients with optimized timeouts.
func (c *Container) initializeAWSClients() error {
	log.Println("Initializing AWS clients...")
	startTime := time.Now()

	// Create context with timeout for AWS config loading
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	awsCfg, err := awsConfig.LoadDefaultConfig(ctx)
	if err != nil {
		return fmt.Errorf("failed to load AWS config: %w", err)
	}

	// DynamoDB client with custom timeouts
	c.DynamoDBClient = awsDynamodb.NewFromConfig(awsCfg, func(o *awsDynamodb.Options) {
		// Set reasonable timeouts for DynamoDB operations
		o.HTTPClient = &http.Client{
			Timeout: 15 * time.Second,
		}
	})

	// EventBridge client with custom timeouts
	c.EventBridgeClient = awsEventbridge.NewFromConfig(awsCfg, func(o *awsEventbridge.Options) {
		// Set reasonable timeouts for EventBridge operations
		o.HTTPClient = &http.Client{
			Timeout: 10 * time.Second,
		}
	})

	log.Printf("AWS clients initialized in %v", time.Since(startTime))
	return nil
}

// initializeRepository sets up the repository layer with Phase 2 enhancements.
func (c *Container) initializeRepository() error {
	log.Println("Initializing repository layer with Phase 2 enhancements...")
	startTime := time.Now()

	if c.DynamoDBClient == nil {
		return fmt.Errorf("DynamoDB client not initialized")
	}
	if c.Config == nil {
		return fmt.Errorf("config not loaded")
	}

	// Initialize cross-cutting concerns first
	c.initializeCrossCuttingConcerns()

	// Phase 2: Initialize repository factory with environment-specific configuration
	factoryConfig := c.getRepositoryFactoryConfig()
	c.RepositoryFactory = repository.NewRepositoryFactory(factoryConfig)

	// Initialize base repositories (without decorators)
	baseNodeRepo := dynamodb.NewNodeRepository(c.DynamoDBClient, c.Config.TableName, c.Config.IndexName)
	baseEdgeRepo := dynamodb.NewEdgeRepository(c.DynamoDBClient, c.Config.TableName, c.Config.IndexName)
	baseKeywordRepo := dynamodb.NewKeywordRepository(c.DynamoDBClient, c.Config.TableName, c.Config.IndexName)
	baseTransactionalRepo := dynamodb.NewTransactionalRepository(c.DynamoDBClient, c.Config.TableName, c.Config.IndexName)
	baseCategoryRepo := dynamodb.NewCategoryRepository(c.DynamoDBClient, c.Config.TableName, c.Config.IndexName)
	baseGraphRepo := dynamodb.NewGraphRepository(c.DynamoDBClient, c.Config.TableName, c.Config.IndexName)

	// Phase 2: Apply decorators using the factory
	c.NodeRepository = c.RepositoryFactory.CreateNodeRepository(baseNodeRepo, c.Logger, c.Cache, c.MetricsCollector)
	c.EdgeRepository = c.RepositoryFactory.CreateEdgeRepository(baseEdgeRepo, c.Logger, c.Cache, c.MetricsCollector)
	c.CategoryRepository = c.RepositoryFactory.CreateCategoryRepository(baseCategoryRepo, c.Logger, c.Cache, c.MetricsCollector)
	
	// Repositories that don't need decorators yet (can be enhanced later)
	c.KeywordRepository = baseKeywordRepo
	c.TransactionalRepository = baseTransactionalRepo
	c.GraphRepository = baseGraphRepo

	// Initialize composed repository for backward compatibility
	c.Repository = dynamodb.NewRepository(c.DynamoDBClient, c.Config.TableName, c.Config.IndexName)

	// Initialize idempotency store with 24-hour TTL
	c.IdempotencyStore = dynamodb.NewIdempotencyStore(c.DynamoDBClient, c.Config.TableName, 24*time.Hour)

	// Phase 2: Initialize advanced repository components
	if err := c.initializeAdvancedRepositoryComponents(); err != nil {
		return fmt.Errorf("failed to initialize advanced repository components: %w", err)
	}

	log.Printf("Enhanced repository layer initialized in %v", time.Since(startTime))
	return nil
}

// initializeCrossCuttingConcerns sets up logging, caching, and metrics
func (c *Container) initializeCrossCuttingConcerns() {
	// Initialize logger
	var err error
	c.Logger, err = zap.NewProduction()
	if err != nil {
		// Fallback to development logger
		c.Logger, _ = zap.NewDevelopment()
	}

	// Initialize cache (placeholder - would be Redis/Memcached in production)
	c.Cache = &NoOpCache{} // Placeholder implementation

	// Initialize metrics collector (placeholder - would be Prometheus/StatsD in production)  
	c.MetricsCollector = &NoOpMetricsCollector{} // Placeholder implementation
}

// getRepositoryFactoryConfig returns environment-appropriate factory configuration
func (c *Container) getRepositoryFactoryConfig() repository.FactoryConfig {
	// Determine environment and return appropriate config
	// This would typically be based on environment variables or config
	environment := "development" // Placeholder
	
	switch environment {
	case "production":
		return repository.ProductionFactoryConfig()
	case "testing":
		// Create testing factory config directly 
		return repository.FactoryConfig{
			LoggingConfig:     repository.LoggingConfig{},
			CachingConfig:     repository.CachingConfig{},
			MetricsConfig:     repository.MetricsConfig{},
			RetryConfig:       repository.DefaultRetryConfig(),
			EnableLogging:     false,
			EnableCaching:     false,
			EnableMetrics:     false,
			EnableRetries:     false,
			StrictMode:        true,
			EnableValidation:  true,
		}
	default:
		return repository.DevelopmentFactoryConfig()
	}
}

// initializeAdvancedRepositoryComponents initializes Phase 2 advanced components
func (c *Container) initializeAdvancedRepositoryComponents() error {
	// Initialize Unit of Work provider (placeholder implementation)
	// c.UnitOfWorkProvider = NewUnitOfWorkProvider(...)
	
	// Initialize Query Executor (placeholder implementation)
	// c.QueryExecutor = NewQueryExecutor(...)
	
	// Initialize Repository Manager (placeholder implementation) 
	// c.RepositoryManager = NewRepositoryManager(...)
	
	log.Println("Advanced repository components initialized (placeholder implementations)")
	return nil
}

// initializePhase3Services initializes the Phase 3 CQRS application services
func (c *Container) initializePhase3Services() {
	log.Println("Initializing Phase 3 Application Services with CQRS pattern...")
	startTime := time.Now()

	// Initialize domain services first
	c.ConnectionAnalyzer = domainServices.NewConnectionAnalyzer(0.3, 5, 0.2) // threshold, max connections, recency weight
	c.EventBus = domain.NewMockEventBus() // Use mock for now, can be replaced with real implementation
	
	// Initialize Unit of Work with existing repository
	transactionProvider := &MockTransactionProvider{} // Placeholder
	eventPublisher := &MockEventPublisher{} // Placeholder
	repositoryFactory := &MockTransactionalRepositoryFactory{} // Placeholder
	
	c.UnitOfWork = repository.NewUnitOfWork(transactionProvider, eventPublisher, repositoryFactory)
	
	// Create repository adapters
	nodeAdapter := adapters.NewNodeRepositoryAdapter(c.NodeRepository, c.TransactionalRepository)
	edgeAdapter := adapters.NewEdgeRepositoryAdapter(c.EdgeRepository)
	categoryAdapter := adapters.NewCategoryRepositoryAdapter(c.CategoryRepository)
	graphAdapter := adapters.NewGraphRepositoryAdapter(c.GraphRepository)
	
	// Create NodeCategoryRepository using the factory instead of UnitOfWork
	// For container initialization, we use nil transaction since this is just for DI setup
	nodeCategoryRepo := repositoryFactory.CreateNodeCategoryRepository(nil)
	nodeCategoryAdapter := adapters.NewNodeCategoryRepositoryAdapter(nodeCategoryRepo)
	
	// Create unit of work adapter
	uowAdapter := adapters.NewUnitOfWorkAdapter(c.UnitOfWork, nodeAdapter, edgeAdapter, categoryAdapter, graphAdapter, nodeCategoryAdapter)
	
	// Initialize Application Services (only NodeService for now)
	c.NodeAppService = services.NewNodeService(
		nodeAdapter,
		c.EdgeRepository,
		uowAdapter,
		c.EventBus,
		c.ConnectionAnalyzer,
		c.IdempotencyStore,
	)
	
	// Note: CategoryAppService and Query services would be initialized here once they're updated to use adapters
	// c.CategoryAppService = services.NewCategoryService(...)
	// c.NodeQueryService = queries.NewNodeQueryService(...)
	// c.CategoryQueryService = queries.NewCategoryQueryService(...)
	
	log.Printf("Phase 3 Application Services initialized in %v", time.Since(startTime))
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
	
	// Initialize enhanced category service with repository
	c.CategoryService = categoryService.NewEnhancedService(c.Repository, nil) // LLM service can be nil for now
	
	// Phase 3: Initialize Application Services with CQRS pattern
	c.initializePhase3Services()
}

// initializeHandlers sets up the handler layer.
func (c *Container) initializeHandlers() {
	c.MemoryHandler = handlers.NewMemoryHandler(c.MemoryService, c.EventBridgeClient, c)
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
	// Category service has been fixed, re-enabling router
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

// SetColdStartInfo sets cold start tracking information
func (c *Container) SetColdStartInfo(coldStartTime time.Time, isColdStart bool) {
	c.ColdStartTime = &coldStartTime
	c.IsColdStart = isColdStart
}

// GetTimeSinceColdStart returns the duration since cold start, or zero if not available
func (c *Container) GetTimeSinceColdStart() time.Duration {
	if c.ColdStartTime == nil {
		return 0
	}
	return time.Since(*c.ColdStartTime)
}

// IsPostColdStartRequest returns true if this is a request happening shortly after cold start
func (c *Container) IsPostColdStartRequest() bool {
	if c.ColdStartTime == nil {
		return false
	}
	timeSince := time.Since(*c.ColdStartTime)
	return timeSince < 30*time.Second && !c.IsColdStart
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

// Placeholder implementations for Phase 2 components
// These would be replaced with actual implementations in production

// NoOpCache is a placeholder cache implementation that does nothing
type NoOpCache struct{}

func (c *NoOpCache) Get(ctx context.Context, key string) ([]byte, bool, error) {
	return nil, false, nil
}

func (c *NoOpCache) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	return nil
}

func (c *NoOpCache) Delete(ctx context.Context, key string) error {
	return nil
}

func (c *NoOpCache) Clear(ctx context.Context, pattern string) error {
	return nil
}

// NoOpMetricsCollector is a placeholder metrics collector that does nothing
type NoOpMetricsCollector struct{}

func (m *NoOpMetricsCollector) IncrementCounter(name string, tags map[string]string) {}

func (m *NoOpMetricsCollector) IncrementCounterBy(name string, value float64, tags map[string]string) {}

func (m *NoOpMetricsCollector) SetGauge(name string, value float64, tags map[string]string) {}

func (m *NoOpMetricsCollector) IncrementGauge(name string, value float64, tags map[string]string) {}

func (m *NoOpMetricsCollector) RecordDuration(name string, duration time.Duration, tags map[string]string) {}

func (m *NoOpMetricsCollector) RecordValue(name string, value float64, tags map[string]string) {}

func (m *NoOpMetricsCollector) RecordDistribution(name string, value float64, tags map[string]string) {}

// Placeholder implementations for Phase 3 components
// These would be replaced with actual implementations in production

// MockTransactionProvider is a placeholder transaction provider
type MockTransactionProvider struct{}

func (m *MockTransactionProvider) BeginTransaction(ctx context.Context) (repository.Transaction, error) {
	return &MockTransaction{}, nil
}

// MockTransaction is a placeholder transaction
type MockTransaction struct {
	committed bool
	rolledBack bool
}

func (m *MockTransaction) Commit() error {
	m.committed = true
	return nil
}

func (m *MockTransaction) Rollback() error {
	m.rolledBack = true
	return nil
}

func (m *MockTransaction) IsActive() bool {
	return !m.committed && !m.rolledBack
}

// MockEventPublisher is a placeholder event publisher
type MockEventPublisher struct{}

func (m *MockEventPublisher) Publish(ctx context.Context, events []domain.DomainEvent) error {
	// In a real implementation, this would publish events to a message bus
	return nil
}

// MockTransactionalRepositoryFactory is a placeholder repository factory
type MockTransactionalRepositoryFactory struct{}

func (m *MockTransactionalRepositoryFactory) CreateNodeRepository(tx repository.Transaction) repository.NodeRepository {
	// Return the same repository - in production this would be a transactional wrapper
	return nil // Placeholder
}

func (m *MockTransactionalRepositoryFactory) CreateEdgeRepository(tx repository.Transaction) repository.EdgeRepository {
	return nil // Placeholder
}

func (m *MockTransactionalRepositoryFactory) CreateCategoryRepository(tx repository.Transaction) repository.CategoryRepository {
	return nil // Placeholder
}

func (m *MockTransactionalRepositoryFactory) CreateKeywordRepository(tx repository.Transaction) repository.KeywordRepository {
	return nil // Placeholder
}

func (m *MockTransactionalRepositoryFactory) CreateGraphRepository(tx repository.Transaction) repository.GraphRepository {
	return nil // Placeholder
}

func (m *MockTransactionalRepositoryFactory) CreateNodeCategoryRepository(tx repository.Transaction) repository.NodeCategoryRepository {
	return nil // Placeholder
}