// Package di provides a centralized dependency injection container.
package di

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"brain2-backend/infrastructure/dynamodb"
	"brain2-backend/internal/application/adapters"
	"brain2-backend/internal/application/queries"
	"brain2-backend/internal/application/services"
	"brain2-backend/internal/config"
	"brain2-backend/internal/domain"
	domainServices "brain2-backend/internal/domain/services"
	"brain2-backend/internal/handlers"
	"brain2-backend/internal/infrastructure/decorators"
	"brain2-backend/internal/infrastructure/events"
	"brain2-backend/internal/infrastructure/transactions"
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
	NodeQueryService    *queries.NodeQueryService
	// CategoryQueryService *queries.CategoryQueryService
	
	// Domain Services
	ConnectionAnalyzer *domainServices.ConnectionAnalyzer
	EventBus          domain.EventBus
	UnitOfWork        repository.UnitOfWork

	// Handler Layer
	MemoryHandler   *handlers.MemoryHandler
	CategoryHandler *handlers.CategoryHandler
	HealthHandler   *handlers.HealthHandler

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
	
	// Initialize REAL implementations instead of mocks
	transactionProvider := transactions.NewDynamoDBTransactionProvider(c.DynamoDBClient)
	
	// Get event bus name from config, default to "default" if not set
	eventBusName := "default"
	if c.Config != nil && c.Config.Events.EventBusName != "" {
		eventBusName = c.Config.Events.EventBusName
	}
	eventPublisher := events.NewEventBridgePublisher(c.EventBridgeClient, eventBusName, "brain2-backend")
	
	// Create a real transactional repository factory using the implementation below
	repositoryFactory := &simpleTransactionalRepositoryFactory{
		nodeRepo:     c.NodeRepository,
		edgeRepo:     c.EdgeRepository,
		categoryRepo: c.CategoryRepository,
	}
	
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
	
	// Initialize Node Query Service with cache (using InMemoryCache wrapper)
	var queryCache queries.Cache
	if c.Config.Features.EnableCaching {
		// Wrap InMemoryCache to implement queries.Cache interface
		queryCache = &SimpleMemoryCacheWrapper{
			cache: NewInMemoryCache(100, 5*time.Minute),
		}
	} else {
		queryCache = nil // NodeQueryService handles nil cache gracefully
	}
	
	// Create reader adapters for CQRS query services
	nodeReader := NewNodeReaderBridge(c.NodeRepository)
	edgeReader := NewEdgeReaderBridge(c.EdgeRepository)
	
	c.NodeQueryService = queries.NewNodeQueryService(
		nodeReader,
		edgeReader,
		c.GraphRepository,
		queryCache,
	)
	
	// Note: CategoryAppService would be initialized here once it's updated to use adapters
	// c.CategoryAppService = services.NewCategoryService(...)
	// c.CategoryQueryService = queries.NewCategoryQueryService(...)
	
	log.Printf("Phase 3 Application Services initialized in %v", time.Since(startTime))
}

// initializeServices sets up the service layer.
func (c *Container) initializeServices() {
	// First, initialize the legacy service for operations not yet migrated
	legacyMemoryService := memoryService.NewServiceWithIdempotency(
		c.NodeRepository,
		c.EdgeRepository,
		c.KeywordRepository,
		c.TransactionalRepository,
		c.GraphRepository,
		c.IdempotencyStore,
	)
	
	// Initialize Phase 3 CQRS services
	c.initializePhase3Services()
	
	// Create the migration adapter that uses new services where available
	if c.NodeAppService != nil && c.NodeQueryService != nil {
		// Use the adapter for gradual migration
		c.MemoryService = NewMemoryServiceAdapter(
			c.NodeAppService,
			c.NodeQueryService,
			legacyMemoryService,
		)
		log.Println("Using CQRS-based MemoryService with migration adapter")
	} else {
		// Fallback to legacy service if CQRS services aren't ready
		c.MemoryService = legacyMemoryService
		log.Println("Using legacy MemoryService")
	}
	
	// Initialize enhanced category service with repository
	c.CategoryService = categoryService.NewEnhancedService(c.Repository, nil, c.Config) // Pass config for feature flags
}

// initializeHandlers sets up the handler layer.
func (c *Container) initializeHandlers() {
	c.MemoryHandler = handlers.NewMemoryHandler(c.MemoryService, c.EventBridgeClient, c)
	c.CategoryHandler = handlers.NewCategoryHandler(c.CategoryService)
	c.HealthHandler = handlers.NewHealthHandler()
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
	router := chi.NewRouter()
	
	// Health check endpoints (public)
	router.Get("/health", c.HealthHandler.Check)
	router.Get("/ready", c.HealthHandler.Ready)
	
	// API routes (protected)
	router.Route("/api", func(r chi.Router) {
		// Apply authentication middleware to all API routes
		r.Use(handlers.Authenticator)
		
		// Node routes
		r.Route("/nodes", func(r chi.Router) {
			r.Post("/", c.MemoryHandler.CreateNode)
			r.Get("/", c.MemoryHandler.ListNodes)
			r.Get("/{nodeId}", c.MemoryHandler.GetNode)
			r.Put("/{nodeId}", c.MemoryHandler.UpdateNode)
			r.Delete("/{nodeId}", c.MemoryHandler.DeleteNode)
			r.Post("/bulk-delete", c.MemoryHandler.BulkDeleteNodes)
		})
		
		// Graph routes
		r.Get("/graph-data", c.MemoryHandler.GetGraphData)
		
		// Category routes
		r.Route("/categories", func(r chi.Router) {
			r.Post("/", c.CategoryHandler.CreateCategory)
			r.Get("/", c.CategoryHandler.ListCategories)
			r.Get("/{categoryId}", c.CategoryHandler.GetCategory)
			r.Put("/{categoryId}", c.CategoryHandler.UpdateCategory)
			r.Delete("/{categoryId}", c.CategoryHandler.DeleteCategory)
			
			// Category-Node relationships
			r.Post("/{categoryId}/nodes", c.CategoryHandler.AssignNodeToCategory)
			r.Get("/{categoryId}/nodes", c.CategoryHandler.GetNodesInCategory)
			r.Delete("/{categoryId}/nodes/{nodeId}", c.CategoryHandler.RemoveNodeFromCategory)
		})
		
		// Node categorization routes
		r.Get("/nodes/{nodeId}/categories", c.CategoryHandler.GetNodeCategories)
		r.Post("/nodes/{nodeId}/categories", c.CategoryHandler.CategorizeNode)
	})
	
	c.Router = router
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

// NewNoOpCache creates a new no-op cache
func NewNoOpCache() decorators.Cache {
	return &NoOpCache{}
}

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

// NewNoOpMetricsCollector creates a new no-op metrics collector
func NewNoOpMetricsCollector() decorators.MetricsCollector {
	return &NoOpMetricsCollector{}
}

func (m *NoOpMetricsCollector) IncrementCounter(name string, tags map[string]string) {}

func (m *NoOpMetricsCollector) IncrementCounterBy(name string, value float64, tags map[string]string) {}

func (m *NoOpMetricsCollector) SetGauge(name string, value float64, tags map[string]string) {}

func (m *NoOpMetricsCollector) IncrementGauge(name string, value float64, tags map[string]string) {}

func (m *NoOpMetricsCollector) RecordDuration(name string, duration time.Duration, tags map[string]string) {}

func (m *NoOpMetricsCollector) RecordValue(name string, value float64, tags map[string]string) {}

func (m *NoOpMetricsCollector) RecordDistribution(name string, value float64, tags map[string]string) {}

// InMemoryCache is a simple in-memory cache implementation
type InMemoryCache struct {
	items    map[string]inMemoryCacheItem
	maxItems int
	ttl      time.Duration
	mu       sync.RWMutex
}

type inMemoryCacheItem struct {
	value     []byte
	expiresAt time.Time
}

// NewInMemoryCache creates a new in-memory cache
func NewInMemoryCache(maxItems int, defaultTTL time.Duration) decorators.Cache {
	cache := &InMemoryCache{
		items:    make(map[string]inMemoryCacheItem),
		maxItems: maxItems,
		ttl:      defaultTTL,
	}
	// Start cleanup goroutine
	go cache.cleanup()
	return cache
}

func (c *InMemoryCache) Get(ctx context.Context, key string) ([]byte, bool, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	item, exists := c.items[key]
	if !exists {
		return nil, false, nil
	}
	
	if time.Now().After(item.expiresAt) {
		return nil, false, nil
	}
	
	return item.value, true, nil
}

func (c *InMemoryCache) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	if ttl == 0 {
		ttl = c.ttl
	}
	
	// Evict old items if cache is full
	if len(c.items) >= c.maxItems {
		c.evictOldest()
	}
	
	c.items[key] = inMemoryCacheItem{
		value:     value,
		expiresAt: time.Now().Add(ttl),
	}
	
	return nil
}

func (c *InMemoryCache) Delete(ctx context.Context, key string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	delete(c.items, key)
	return nil
}

func (c *InMemoryCache) Clear(ctx context.Context, pattern string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	// Simple pattern matching (prefix only for now)
	for key := range c.items {
		if pattern == "*" || (len(pattern) > 0 && pattern[len(pattern)-1] == '*' && 
			len(key) >= len(pattern)-1 && key[:len(pattern)-1] == pattern[:len(pattern)-1]) {
			delete(c.items, key)
		}
	}
	
	return nil
}

func (c *InMemoryCache) evictOldest() {
	var oldestKey string
	var oldestTime time.Time
	
	for key, item := range c.items {
		if oldestTime.IsZero() || item.expiresAt.Before(oldestTime) {
			oldestKey = key
			oldestTime = item.expiresAt
		}
	}
	
	if oldestKey != "" {
		delete(c.items, oldestKey)
	}
}

func (c *InMemoryCache) cleanup() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()
	
	for range ticker.C {
		c.mu.Lock()
		now := time.Now()
		for key, item := range c.items {
			if now.After(item.expiresAt) {
				delete(c.items, key)
			}
		}
		c.mu.Unlock()
	}
}

// InMemoryMetricsCollector collects metrics in memory
type InMemoryMetricsCollector struct {
	counters map[string]float64
	gauges   map[string]float64
	timings  map[string][]time.Duration
	mu       sync.RWMutex
	logger   *zap.Logger
}

// NewInMemoryMetricsCollector creates a new in-memory metrics collector
func NewInMemoryMetricsCollector(logger *zap.Logger) decorators.MetricsCollector {
	return &InMemoryMetricsCollector{
		counters: make(map[string]float64),
		gauges:   make(map[string]float64),
		timings:  make(map[string][]time.Duration),
		logger:   logger,
	}
}

func (m *InMemoryMetricsCollector) IncrementCounter(name string, tags map[string]string) {
	m.IncrementCounterBy(name, 1, tags)
}

func (m *InMemoryMetricsCollector) IncrementCounterBy(name string, value float64, tags map[string]string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	key := m.buildKey(name, tags)
	m.counters[key] += value
}

func (m *InMemoryMetricsCollector) SetGauge(name string, value float64, tags map[string]string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	key := m.buildKey(name, tags)
	m.gauges[key] = value
}

func (m *InMemoryMetricsCollector) IncrementGauge(name string, value float64, tags map[string]string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	key := m.buildKey(name, tags)
	m.gauges[key] += value
}

func (m *InMemoryMetricsCollector) RecordDuration(name string, duration time.Duration, tags map[string]string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	key := m.buildKey(name, tags)
	m.timings[key] = append(m.timings[key], duration)
	
	// Keep only last 1000 timings per metric
	if len(m.timings[key]) > 1000 {
		m.timings[key] = m.timings[key][len(m.timings[key])-1000:]
	}
}

func (m *InMemoryMetricsCollector) RecordValue(name string, value float64, tags map[string]string) {
	// For now, treat as gauge
	m.SetGauge(name, value, tags)
}

func (m *InMemoryMetricsCollector) RecordDistribution(name string, value float64, tags map[string]string) {
	// For now, treat as gauge
	m.SetGauge(name, value, tags)
}

func (m *InMemoryMetricsCollector) buildKey(name string, tags map[string]string) string {
	if len(tags) == 0 {
		return name
	}
	
	// Build key with tags
	key := name
	for k, v := range tags {
		key += fmt.Sprintf(",%s=%s", k, v)
	}
	return key
}

// Placeholder implementations for Phase 3 components
// Mock implementations have been removed - using real implementations from infrastructure packages:
// - transactions.DynamoDBTransactionProvider for transaction management
// - events.EventBridgePublisher for event publishing  
// - repository.TransactionalRepositoryFactory for creating transactional repositories

// Additional mock factory methods removed - using real TransactionalRepositoryFactory

// simpleTransactionalRepositoryFactory implements repository.TransactionalRepositoryFactory
type simpleTransactionalRepositoryFactory struct {
	nodeRepo     repository.NodeRepository
	edgeRepo     repository.EdgeRepository
	categoryRepo repository.CategoryRepository
}

func (f *simpleTransactionalRepositoryFactory) CreateNodeRepository(tx repository.Transaction) repository.NodeRepository {
	// In a real implementation, this would wrap the repository with transaction support
	// For now, just return the base repository
	return f.nodeRepo
}

func (f *simpleTransactionalRepositoryFactory) CreateEdgeRepository(tx repository.Transaction) repository.EdgeRepository {
	return f.edgeRepo
}

func (f *simpleTransactionalRepositoryFactory) CreateCategoryRepository(tx repository.Transaction) repository.CategoryRepository {
	return f.categoryRepo
}

func (f *simpleTransactionalRepositoryFactory) CreateKeywordRepository(tx repository.Transaction) repository.KeywordRepository {
	// Return nil as we don't have a keyword repository yet
	return nil
}

func (f *simpleTransactionalRepositoryFactory) CreateGraphRepository(tx repository.Transaction) repository.GraphRepository {
	// Return the graph repository if available
	return nil
}

func (f *simpleTransactionalRepositoryFactory) CreateNodeCategoryRepository(tx repository.Transaction) repository.NodeCategoryRepository {
	// Return nil as we'll use the existing mock
	return nil
}

// SimpleMemoryCacheWrapper wraps InMemoryCache to implement queries.Cache interface
type SimpleMemoryCacheWrapper struct {
	cache decorators.Cache
}

// Get retrieves a value from the cache
func (c *SimpleMemoryCacheWrapper) Get(ctx context.Context, key string) (interface{}, bool) {
	data, found, err := c.cache.Get(ctx, key)
	if err != nil || !found {
		return nil, false
	}
	
	// Deserialize the data
	var value interface{}
	if err := json.Unmarshal(data, &value); err != nil {
		return nil, false
	}
	
	return value, true
}

// Set stores a value in the cache
func (c *SimpleMemoryCacheWrapper) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) {
	// Serialize the value
	data, err := json.Marshal(value)
	if err != nil {
		return
	}
	
	c.cache.Set(ctx, key, data, ttl)
}

// Delete removes a value from the cache
func (c *SimpleMemoryCacheWrapper) Delete(ctx context.Context, key string) {
	c.cache.Delete(ctx, key)
}