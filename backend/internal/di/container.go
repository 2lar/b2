//go:build !wireinject
// +build !wireinject

// Package di provides a centralized dependency injection container.
package di

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	"brain2-backend/internal/application/queries"
	"brain2-backend/internal/application/services"
	"brain2-backend/internal/config"
	"brain2-backend/internal/domain/category"
	"brain2-backend/internal/domain/edge"
	"brain2-backend/internal/domain/node"
	domainServices "brain2-backend/internal/domain/services"
	"brain2-backend/internal/domain/shared"
	"brain2-backend/internal/interfaces/http/middleware"
	v1handlers "brain2-backend/internal/interfaces/http/v1/handlers"
	"brain2-backend/internal/infrastructure/messaging"
	"brain2-backend/internal/infrastructure/observability"
	"brain2-backend/internal/infrastructure/persistence"
	"brain2-backend/internal/infrastructure/persistence/cache"
	infradynamodb "brain2-backend/internal/infrastructure/persistence/dynamodb"
	"brain2-backend/internal/repository"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsConfig "github.com/aws/aws-sdk-go-v2/config"
	awsDynamodb "github.com/aws/aws-sdk-go-v2/service/dynamodb"
	awsEventbridge "github.com/aws/aws-sdk-go-v2/service/eventbridge"
	"github.com/go-chi/chi/v5"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"
)

// contextKey is a type for context keys to avoid collisions
type contextKey string

const (
	// txContextKey is the context key for transactions
	txContextKey contextKey = "tx"
)

// Container type is defined in types.go to be shared between Wire and manual initialization

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
	c.TableName = cfg.Database.TableName
	c.IndexName = cfg.Database.IndexName

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

	// 6. Initialize observability
	if err := c.initializeObservability(); err != nil {
		return fmt.Errorf("failed to initialize observability: %w", err)
	}

	// 7. Initialize middleware configuration
	c.initializeMiddleware()

	// 8. Initialize HTTP router
	c.initializeRouter()

	// 8. Initialize tracing if enabled
	if err := c.initializeTracing(); err != nil {
		log.Printf("Failed to initialize tracing: %v", err)
		// Don't fail startup, just log the error
	}

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

	// Create shared HTTP client with Keep-Alive explicitly enabled for connection reuse
	httpClient := &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			// Explicitly enable Keep-Alive for connection reuse within the Lambda container
			DisableKeepAlives:   false, // IMPORTANT: Reuse TCP connections on warm starts
			MaxIdleConns:        10,    // Keep some connections ready (useful within single container)
			MaxIdleConnsPerHost: 2,     // Per-host limit (we mainly talk to DynamoDB)
			IdleConnTimeout:     90 * time.Second,
			TLSHandshakeTimeout: 10 * time.Second,
		},
	}

	// DynamoDB client with optimized HTTP client
	c.DynamoDBClient = awsDynamodb.NewFromConfig(awsCfg, func(o *awsDynamodb.Options) {
		o.HTTPClient = httpClient
		o.RetryMaxAttempts = 3
		o.RetryMode = aws.RetryModeAdaptive
	})

	// EventBridge client with optimized HTTP client
	c.EventBridgeClient = awsEventbridge.NewFromConfig(awsCfg, func(o *awsEventbridge.Options) {
		o.HTTPClient = httpClient
		o.RetryMaxAttempts = 3
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

	// Initialize Store implementation now that logger is available
	storeConfig := persistence.StoreConfig{
		TableName:      c.Config.Database.TableName, // Use config table name
		TimeoutMs:      15000,
		RetryAttempts:  3,
		ConsistentRead: false,
	}
	c.Store = persistence.NewDynamoDBStore(c.DynamoDBClient, storeConfig, c.Logger)

	// Phase 2: Initialize repository factory with environment-specific configuration
	factoryConfig := c.getRepositoryFactoryConfig()
	c.RepositoryFactory = repository.NewRepositoryFactory(factoryConfig, c.Logger)

	// Initialize base repositories (without persistence)
	baseNodeRepo := infradynamodb.NewNodeRepository(c.DynamoDBClient, c.Config.Database.TableName, c.Config.Database.IndexName, c.Logger)
	baseEdgeRepo := infradynamodb.NewEdgeRepositoryCQRS(c.DynamoDBClient, c.Config.Database.TableName, c.Config.Database.IndexName, c.Logger)
	baseKeywordRepo := infradynamodb.NewKeywordRepository(c.DynamoDBClient, c.Config.Database.TableName, c.Config.Database.IndexName)
	// Create transactional repository
	baseTransactionalRepo := infradynamodb.NewTransactionalRepository(c.DynamoDBClient, c.Config.Database.TableName, c.Config.Database.IndexName, c.Logger)
	baseCategoryRepo := infradynamodb.NewCategoryRepositoryCQRS(c.DynamoDBClient, c.Config.Database.TableName, c.Config.Database.IndexName, c.Logger)
	baseGraphRepo := infradynamodb.NewGraphRepository(c.DynamoDBClient, c.Config.Database.TableName, c.Config.Database.IndexName, c.Logger)

	// Phase 2: Apply persistence using the factory
	c.NodeRepository = c.RepositoryFactory.CreateNodeRepository(baseNodeRepo, c.Logger, c.Cache, c.MetricsCollector)
	c.EdgeRepository = c.RepositoryFactory.CreateEdgeRepository(baseEdgeRepo, c.Logger, c.Cache, c.MetricsCollector)
	c.CategoryRepository = c.RepositoryFactory.CreateCategoryRepository(baseCategoryRepo, c.Logger, c.Cache, c.MetricsCollector)

	// Repositories that don't need persistence yet (can be enhanced later)
	c.KeywordRepository = baseKeywordRepo
	c.TransactionalRepository = baseTransactionalRepo
	c.GraphRepository = baseGraphRepo

	// Repository field removed - using specific repositories directly

	// Initialize idempotency store with configured TTL
	// TTL can be configured via:
	//   - IDEMPOTENCY_TTL environment variable (e.g., "24h", "7d", "1h")
	//   - infrastructure.idempotency_ttl in config files
	// Default: 24h, Valid range: 1h to 168h (7 days)
	c.IdempotencyStore = infradynamodb.NewIdempotencyStore(c.DynamoDBClient, c.Config.Database.TableName, c.Config.Infrastructure.IdempotencyTTL)

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

	// Initialize cache (in-memory cache for development, would be Redis/Memcached in production)
	c.Cache = &NoOpCache{} // Simple no-op implementation

	// Initialize metrics collector (in-memory for development, would be Prometheus/StatsD in production)
	c.MetricsCollector = NewNoOpMetricsCollector() // Simple no-op implementation
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
			LoggingConfig:    repository.LoggingConfig{},
			CachingConfig:    repository.CachingConfig{},
			MetricsConfig:    repository.MetricsConfig{},
			RetryConfig:      repository.DefaultRetryConfig(),
			EnableLogging:    false,
			EnableCaching:    false,
			EnableMetrics:    false,
			EnableRetries:    false,
			StrictMode:       true,
			EnableValidation: true,
		}
	default:
		return repository.DevelopmentFactoryConfig()
	}
}

// initializeAdvancedRepositoryComponents initializes Phase 2 advanced components
func (c *Container) initializeAdvancedRepositoryComponents() error {
	// These components are not currently used but reserved for future enhancements
	log.Println("Advanced repository components reserved for future use")
	return nil
}

// initializeCQRSServices initializes the CQRS application services
func (c *Container) initializeCQRSServices() {
	log.Println("Initializing CQRS Application Services...")
	startTime := time.Now()

	// Initialize domain services first
	c.ConnectionAnalyzer = domainServices.NewConnectionAnalyzer(0.3, 5, 0.2) // threshold, max connections, recency weight

	// Initialize REAL implementations instead of mocks
	transactionProvider := persistence.NewDynamoDBTransactionProvider(c.DynamoDBClient)

	// Get event bus name from environment variable first, then config, default to "B2EventBus"
	eventBusName := os.Getenv("EVENT_BUS_NAME")
	if eventBusName == "" {
		eventBusName = "B2EventBus" // Match the CDK-created event bus name
		if c.Config != nil && c.Config.Events.EventBusName != "" {
			eventBusName = c.Config.Events.EventBusName
		}
	}

	// Add comprehensive debug logging for event bus configuration
	log.Printf("DEBUG: EventBridge Configuration Details:")
	log.Printf("  - FINAL EventBusName: '%s'", eventBusName)
	log.Printf("  - Source: 'brain2-backend'") 
	log.Printf("  - Environment EVENT_BUS_NAME: '%s'", os.Getenv("EVENT_BUS_NAME"))
	log.Printf("  - Config loaded: %v", c.Config != nil)
	if c.Config != nil {
		log.Printf("  - Config.Events.EventBusName: '%s'", c.Config.Events.EventBusName)
	}
	log.Printf("  - Using environment variable: %v", os.Getenv("EVENT_BUS_NAME") != "")
	
	// Test EventBridge client
	if c.EventBridgeClient == nil {
		log.Printf("ERROR: EventBridge client is nil - EventBridge publishing will fail!")
	} else {
		log.Printf("DEBUG: EventBridge client initialized successfully")
		
		// Add basic connectivity test
		testCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		
		// Try to list event buses to test basic connectivity
		_, err := c.EventBridgeClient.ListEventBuses(testCtx, &awsEventbridge.ListEventBusesInput{
			Limit: aws.Int32(1),
		})
		if err != nil {
			log.Printf("ERROR: EventBridge connectivity test failed: %v", err)
			log.Printf("ERROR: This indicates EventBridge client cannot reach AWS service")
		} else {
			log.Printf("DEBUG: EventBridge connectivity test passed - client can reach AWS EventBridge")
		}
	}

	eventPublisher := messaging.NewEventBridgePublisher(c.EventBridgeClient, eventBusName, "brain2-backend")
	
	if eventPublisher == nil {
		log.Printf("ERROR: EventBridge publisher creation failed!")
	} else {
		log.Printf("DEBUG: EventBridge publisher created successfully")
	}

	// Use the REAL EventBridge publisher through the adapter
	c.EventBus = messaging.NewEventBusAdapter(eventPublisher)
	
	if c.EventBus == nil {
		log.Printf("ERROR: EventBus adapter creation failed!")
	} else {
		log.Printf("DEBUG: EventBus adapter created successfully")
	}

	log.Printf("DEBUG: EventBridge publisher configured successfully - Bus: %s", eventBusName)

	// Create a real transactional repository factory using the implementation below
	repositoryFactory := NewTransactionalRepositoryFactory(
		c.NodeRepository,
		c.EdgeRepository,
		c.CategoryRepository,
	)

	// Create EventStore for domain event persistence
	eventStore := infradynamodb.NewDynamoDBEventStore(
		c.DynamoDBClient,
		c.TableName, // Reuse the same table with different partition keys
	)

	// Create UnitOfWorkFactory instead of singleton UnitOfWork
	// This ensures each request gets its own isolated transaction context
	c.UnitOfWorkFactory = infradynamodb.NewDynamoDBUnitOfWorkFactory(
		c.DynamoDBClient,
		c.TableName,
		c.IndexName,
		c.EventBus,
		eventStore,
		c.Logger,
	)

	// Keep singleton for backward compatibility (will be removed later)
	c.UnitOfWork = repository.NewUnitOfWork(transactionProvider, eventPublisher, repositoryFactory, c.Logger)

	// Initialize Application Services (only NodeService for now)
	// Use repositories directly
	c.NodeAppService = services.NewNodeService(
		c.NodeRepository,
		c.EdgeRepository,
		c.UnitOfWorkFactory, // Use factory instead of singleton
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

	// Use repositories directly - they implement the Reader/Writer interfaces
	c.NodeQueryService = queries.NewNodeQueryService(
		c.NodeRepository.(repository.NodeReader),
		c.EdgeRepository.(repository.EdgeReader),
		c.GraphRepository,
		queryCache,
	)

	// Initialize CategoryQueryService with proper dependencies
	c.CategoryQueryService = queries.NewCategoryQueryService(
		c.CategoryRepository.(repository.CategoryReader),
		c.NodeRepository.(repository.NodeReader),
		c.Logger,
		queryCache,
	)

	// Initialize GraphQueryService with Store implementation
	c.GraphQueryService = queries.NewGraphQueryService(
		c.Store,
		c.Logger,
		queryCache,
	)

	// Initialize CategoryAppService for command handling
	// Cast CategoryRepository to reader and writer interfaces
	var categoryReader repository.CategoryReader
	var categoryWriter repository.CategoryWriter
	if reader, ok := c.CategoryRepository.(repository.CategoryReader); ok {
		categoryReader = reader
	}
	if writer, ok := c.CategoryRepository.(repository.CategoryWriter); ok {
		categoryWriter = writer
	}
	
	c.CategoryAppService = services.NewCategoryService(
		categoryReader,
		categoryWriter,
		c.UnitOfWorkFactory,
		c.EventBus,
		c.IdempotencyStore,
	)

	// Initialize CleanupService for async resource cleanup
	// Get EdgeWriter from the edge repository if it implements the interface
	var edgeWriter repository.EdgeWriter
	if writer, ok := c.EdgeRepository.(repository.EdgeWriter); ok {
		edgeWriter = writer
	}

	c.CleanupService = services.NewCleanupService(
		c.NodeRepository,
		c.EdgeRepository,
		edgeWriter,
		c.IdempotencyStore,
		c.UnitOfWorkFactory,
	)

	log.Printf("CQRS Application Services initialized in %v", time.Since(startTime))
}

// initializeServices sets up the service layer.
func (c *Container) initializeServices() {
	// Initialize CQRS services
	c.initializeCQRSServices()
}

// initializeHandlers sets up the handler layer.
func (c *Container) initializeHandlers() {
	// Initialize handlers with CQRS services
	if c.NodeAppService != nil && c.NodeQueryService != nil && c.GraphQueryService != nil {
		c.MemoryHandler = v1handlers.NewMemoryHandler(
			c.NodeAppService,
			c.NodeQueryService,
			c.GraphQueryService,
			c.EventBridgeClient,
			c,
		)
		log.Println("MemoryHandler initialized with CQRS services")
	} else {
		log.Println("ERROR: CQRS services not available for MemoryHandler")
	}

	if c.CategoryAppService != nil && c.CategoryQueryService != nil {
		c.CategoryHandler = v1handlers.NewCategoryHandler(
			c.CategoryAppService,
			c.CategoryQueryService,
		)
		log.Println("CategoryHandler initialized with CQRS services")
	} else {
		log.Println("ERROR: CQRS services not available for CategoryHandler")
	}

	c.HealthHandler = v1handlers.NewHealthHandler()
}

// initializeMiddleware sets up middleware configuration.
func (c *Container) initializeMiddleware() {
	// Store middleware configuration for monitoring/observability
	c.middlewareConfig["request_id"] = map[string]any{
		"enabled":     true,
		"header_name": "X-Request-ID",
	}

	c.middlewareConfig["circuit_breaker"] = map[string]any{
		"enabled": true,
		"api_routes": map[string]any{
			"name":              "api-routes",
			"max_requests":      3,
			"interval_seconds":  10,
			"timeout_seconds":   30,
			"failure_threshold": 0.6,
			"min_requests":      3,
		},
	}

	c.middlewareConfig["timeout"] = map[string]any{
		"enabled":                 true,
		"default_timeout_seconds": 30,
	}

	c.middlewareConfig["recovery"] = map[string]any{
		"enabled":         true,
		"log_stack_trace": true,
	}

	log.Println("Middleware configuration initialized")
}

// initializeRouter sets up the HTTP router with all routes.
func (c *Container) initializeRouter() {
	router := chi.NewRouter()

	// Observability middleware (applied to all routes)
	if c.MetricsCollector != nil {
		router.Use(observability.MetricsMiddleware(c.MetricsCollector))
	}
	if c.TracerProvider != nil {
		router.Use(observability.TracingMiddleware("brain2-api"))
	}

	// API Versioning middleware (applied before route handling)
	router.Use(c.createVersioningMiddleware())

	// Health check endpoints (public)
	router.Get("/health", c.HealthHandler.Check)
	router.Get("/ready", c.HealthHandler.Ready)

	// Metrics endpoint (public)
	router.Handle("/metrics", promhttp.Handler())

	// API routes (protected) - v1
	router.Route("/api/v1", func(r chi.Router) {
		// Apply authentication middleware to all API routes
		r.Use(v1handlers.Authenticator)

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

	// Backward compatibility redirects for old API routes
	router.Route("/api", func(r chi.Router) {
		// Redirect all /api/* to /v1/api/*
		r.HandleFunc("/*", func(w http.ResponseWriter, req *http.Request) {
			// Add deprecation warning header
			w.Header().Set("X-API-Deprecated", "true")
			w.Header().Set("X-API-Migration", "Please use /api/v1/ prefix")
			w.Header().Set("X-API-Sunset", "2025-06-01")

			// Redirect to v1 API
			newPath := req.URL.Path
			if newPath == "/api" || newPath == "/api/" {
				newPath = "/api/v1/"
			} else {
				newPath = "/api/v1" + newPath[4:] // Replace /api with /api/v1
			}
			http.Redirect(w, req, newPath, http.StatusMovedPermanently)
		})
	})

	c.Router = router
}

// initializeObservability sets up metrics collection and tracing.
func (c *Container) initializeObservability() error {
	log.Println("Initializing observability components...")

	// Initialize metrics collector
	c.MetricsCollector = observability.NewCollector("brain2")

	// Initialize tracing provider if enabled
	if c.Config.Tracing.Enabled {
		tracingConfig := observability.TracingConfig{
			ServiceName:  "brain2-backend",
			Environment:  string(c.Config.Environment),
			Endpoint:     c.Config.Tracing.Endpoint,
			SampleRate:   c.Config.Tracing.SampleRate,
			EnableXRay:   isRunningInLambda(),
			EnableDebug:  c.Config.Environment == "development",
		}
		tracerProvider, err := observability.InitTracing(tracingConfig)
		if err != nil {
			log.Printf("WARNING: Failed to initialize tracing: %v", err)
			// Don't fail the entire startup for tracing issues
		} else {
			c.TracerProvider = tracerProvider
			// TODO: Register shutdown handler for graceful tracing shutdown
			// c.registerShutdownHandler(func() error {
			// 	return tracerProvider.Shutdown(context.Background())
			// })
		}
	}

	log.Println("Observability initialized successfully")
	return nil
}

// createVersioningMiddleware creates and configures the API versioning middleware
func (c *Container) createVersioningMiddleware() func(http.Handler) http.Handler {
	// Get API version configuration
	apiConfig := config.GetAPIVersionConfig()
	
	// Create versioning configuration
	versionConfig := middleware.VersionConfig{
		SupportedVersions:   apiConfig.GetSupportedVersions(),
		DefaultVersion:      apiConfig.DefaultVersion,
		DeprecatedVersions:  make(map[string]middleware.DeprecationInfo),
		EnableVersionHeader: true,
		EnableAcceptHeader:  true,
		EnableQueryParam:    true,
		EnableURLPath:       true,
		StrictMode:          false, // Allow fallback to default version
		MetricsEnabled:      c.MetricsCollector != nil,
	}
	
	// Add deprecation info for any deprecated versions
	for version, versionInfo := range apiConfig.Versions {
		if versionInfo.Deprecated && versionInfo.DeprecatedAt != nil && versionInfo.SunsetDate != nil {
			versionConfig.DeprecatedVersions[version] = middleware.DeprecationInfo{
				DeprecatedAt: *versionInfo.DeprecatedAt,
				SunsetAt:     *versionInfo.SunsetDate,
				Message:      fmt.Sprintf("API version %s is deprecated. Please migrate to version %s", version, apiConfig.CurrentVersion),
				MigrationURL: fmt.Sprintf("https://docs.brain2.api/migration/v%s-to-v%s", version, apiConfig.CurrentVersion),
			}
		}
	}
	
	return middleware.Versioning(versionConfig)
}

// initializeTracing sets up distributed tracing if enabled with enhanced configuration.
func (c *Container) initializeTracing() error {
	if !c.Config.Tracing.Enabled {
		log.Println("Tracing is disabled in configuration")
		return nil
	}

	log.Println("Initializing enhanced distributed tracing...")

	// Create tracing configuration
	tracingConfig := observability.TracingConfig{
		ServiceName:  "brain2-backend",
		Environment:  string(c.Config.Environment),
		Endpoint:     c.Config.Tracing.Endpoint,
		SampleRate:   c.Config.Tracing.SampleRate,
		EnableXRay:   isRunningInLambda(), // Auto-detect Lambda environment
		EnableDebug:  c.Config.Environment == "development",
	}

	// Initialize tracing with enhanced configuration
	tracerProvider, err := observability.InitTracing(tracingConfig)
	if err != nil {
		return fmt.Errorf("failed to initialize tracing: %w", err)
	}

	c.TracerProvider = tracerProvider
	
	// Initialize propagator for trace context
	c.TracePropagator = observability.NewTracePropagator()
	
	// Initialize span attributes helper
	c.SpanAttributes = observability.NewSpanAttributes()
	
	// Add shutdown handler
	c.addShutdownFunction(func() error {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return tracerProvider.Shutdown(ctx)
	})

	log.Printf("Enhanced distributed tracing initialized successfully (sample rate: %.2f%%)", 
		tracingConfig.SampleRate * 100)
	return nil
}

// isRunningInLambda checks if the application is running in AWS Lambda
func isRunningInLambda() bool {
	return os.Getenv("AWS_LAMBDA_FUNCTION_NAME") != ""
}

// addShutdownFunction adds a function to be called during container shutdown.
func (c *Container) addShutdownFunction(fn func() error) {
	c.shutdownFunctions = append(c.shutdownFunctions, fn)
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

// GetCleanupService returns the cleanup service instance
func (c *Container) GetCleanupService() *services.CleanupService {
	return c.CleanupService
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
	if c.NodeRepository == nil {
		return fmt.Errorf("composed repository not initialized")
	}
	if c.IdempotencyStore == nil {
		return fmt.Errorf("idempotency store not initialized")
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

// Container methods for ColdStartInfoProvider are defined in types.go

// InitializeContainer will be generated by Wire
// See wire.go for the Wire configuration

// Placeholder implementations for Phase 2 components
// These would be replaced with actual implementations in production

// NoOpCache is a simple cache implementation that does nothing
type NoOpCache struct{}

// NewNoOpCache creates a new no-op cache
func NewNoOpCache() cache.Cache {
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

// NoOpMetricsCollector is a simple metrics collector that does nothing
type NoOpMetricsCollector struct{}

// NewNoOpMetricsCollector creates a new no-op metrics collector
func NewNoOpMetricsCollector() *observability.Collector {
	return nil // Return nil for no-op case
}

func (m *NoOpMetricsCollector) IncrementCounter(name string, tags map[string]string) {}

func (m *NoOpMetricsCollector) IncrementCounterBy(name string, value float64, tags map[string]string) {
}

func (m *NoOpMetricsCollector) SetGauge(name string, value float64, tags map[string]string) {}

func (m *NoOpMetricsCollector) IncrementGauge(name string, value float64, tags map[string]string) {}

func (m *NoOpMetricsCollector) RecordDuration(name string, duration time.Duration, tags map[string]string) {
}

func (m *NoOpMetricsCollector) RecordValue(name string, value float64, tags map[string]string) {}

func (m *NoOpMetricsCollector) RecordDistribution(name string, value float64, tags map[string]string) {
}

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
func NewInMemoryCache(maxItems int, defaultTTL time.Duration) cache.Cache {
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
}

// NewInMemoryMetricsCollector creates a new in-memory metrics collector
func NewInMemoryMetricsCollector(logger *zap.Logger) *observability.Collector {
	// For now, return a real observability.Collector instance
	return observability.NewCollector("brain2")
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

// transactionalRepositoryFactory implements repository.TransactionalRepositoryFactory
// with proper transaction support
type transactionalRepositoryFactory struct {
	nodeRepo     repository.NodeRepository
	edgeRepo     repository.EdgeRepository
	categoryRepo repository.CategoryRepository
	transaction  repository.Transaction
}

// NewTransactionalRepositoryFactory creates a new transactional repository factory
func NewTransactionalRepositoryFactory(
	nodeRepo repository.NodeRepository,
	edgeRepo repository.EdgeRepository,
	categoryRepo repository.CategoryRepository,
) repository.TransactionalRepositoryFactory {
	return &transactionalRepositoryFactory{
		nodeRepo:     nodeRepo,
		edgeRepo:     edgeRepo,
		categoryRepo: categoryRepo,
	}
}

func (f *transactionalRepositoryFactory) WithTransaction(tx repository.Transaction) repository.TransactionalRepositoryFactory {
	f.transaction = tx
	return f
}

func (f *transactionalRepositoryFactory) CreateNodeRepository(tx repository.Transaction) repository.NodeRepository {
	// If we have a transaction, wrap the repository
	if tx != nil {
		return &transactionalNodeWrapper{
			base: f.nodeRepo,
			tx:   tx,
		}
	}
	return f.nodeRepo
}

func (f *transactionalRepositoryFactory) CreateEdgeRepository(tx repository.Transaction) repository.EdgeRepository {
	if tx != nil {
		return &transactionalEdgeWrapper{
			base: f.edgeRepo,
			tx:   tx,
		}
	}
	return f.edgeRepo
}

func (f *transactionalRepositoryFactory) CreateCategoryRepository(tx repository.Transaction) repository.CategoryRepository {
	if tx != nil {
		return &transactionalCategoryWrapper{
			base: f.categoryRepo,
			tx:   tx,
		}
	}
	return f.categoryRepo
}

func (f *transactionalRepositoryFactory) CreateKeywordRepository(tx repository.Transaction) repository.KeywordRepository {
	// Return nil as we don't have a keyword repository yet
	return nil
}

func (f *transactionalRepositoryFactory) CreateGraphRepository(tx repository.Transaction) repository.GraphRepository {
	// Return nil as we don't have a graph repository for transactions yet
	return nil
}

func (f *transactionalRepositoryFactory) CreateNodeCategoryRepository(tx repository.Transaction) repository.NodeCategoryRepository {
	// Return nil as we'll use the existing mock
	return nil
}

// Simple wrapper that marks operations as part of transaction
type transactionalNodeWrapper struct {
	base repository.NodeRepository
	tx   repository.Transaction
}

func (w *transactionalNodeWrapper) CreateNodeAndKeywords(ctx context.Context, node *node.Node) error {
	// Mark context with transaction
	ctx = context.WithValue(ctx, txContextKey, w.tx)
	return w.base.CreateNodeAndKeywords(ctx, node)
}

func (w *transactionalNodeWrapper) FindNodeByID(ctx context.Context, userID, nodeID string) (*node.Node, error) {
	ctx = context.WithValue(ctx, txContextKey, w.tx)
	return w.base.FindNodeByID(ctx, userID, nodeID)
}

// UpdateNode is not part of the NodeRepository interface

func (w *transactionalNodeWrapper) DeleteNode(ctx context.Context, userID, nodeID string) error {
	ctx = context.WithValue(ctx, txContextKey, w.tx)
	return w.base.DeleteNode(ctx, userID, nodeID)
}

func (w *transactionalNodeWrapper) BatchDeleteNodes(ctx context.Context, userID string, nodeIDs []string) (deleted []string, failed []string, err error) {
	ctx = context.WithValue(ctx, txContextKey, w.tx)
	return w.base.BatchDeleteNodes(ctx, userID, nodeIDs)
}

func (w *transactionalNodeWrapper) FindNodes(ctx context.Context, query repository.NodeQuery) ([]*node.Node, error) {
	ctx = context.WithValue(ctx, txContextKey, w.tx)
	return w.base.FindNodes(ctx, query)
}

func (w *transactionalNodeWrapper) GetNodesPage(ctx context.Context, query repository.NodeQuery, pagination repository.Pagination) (*repository.NodePage, error) {
	ctx = context.WithValue(ctx, txContextKey, w.tx)
	return w.base.GetNodesPage(ctx, query, pagination)
}

func (w *transactionalNodeWrapper) GetNodeNeighborhood(ctx context.Context, userID, nodeID string, depth int) (*shared.Graph, error) {
	ctx = context.WithValue(ctx, txContextKey, w.tx)
	return w.base.GetNodeNeighborhood(ctx, userID, nodeID, depth)
}

func (w *transactionalNodeWrapper) CountNodes(ctx context.Context, userID string) (int, error) {
	ctx = context.WithValue(ctx, txContextKey, w.tx)
	return w.base.CountNodes(ctx, userID)
}

// Add missing methods from NodeRepository interface
func (w *transactionalNodeWrapper) FindNodesWithOptions(ctx context.Context, query repository.NodeQuery, opts ...repository.QueryOption) ([]*node.Node, error) {
	ctx = context.WithValue(ctx, txContextKey, w.tx)
	return w.base.FindNodesWithOptions(ctx, query, opts...)
}

func (w *transactionalNodeWrapper) FindNodesPageWithOptions(ctx context.Context, query repository.NodeQuery, pagination repository.Pagination, opts ...repository.QueryOption) (*repository.NodePage, error) {
	ctx = context.WithValue(ctx, txContextKey, w.tx)
	return w.base.FindNodesPageWithOptions(ctx, query, pagination, opts...)
}

// transactionalEdgeWrapper wraps edge repository with transaction context
type transactionalEdgeWrapper struct {
	base repository.EdgeRepository
	tx   repository.Transaction
}

func (w *transactionalEdgeWrapper) CreateEdge(ctx context.Context, edge *edge.Edge) error {
	ctx = context.WithValue(ctx, txContextKey, w.tx)
	return w.base.CreateEdge(ctx, edge)
}

func (w *transactionalEdgeWrapper) CreateEdges(ctx context.Context, userID, sourceNodeID string, relatedNodeIDs []string) error {
	ctx = context.WithValue(ctx, txContextKey, w.tx)
	return w.base.CreateEdges(ctx, userID, sourceNodeID, relatedNodeIDs)
}

func (w *transactionalEdgeWrapper) FindEdges(ctx context.Context, query repository.EdgeQuery) ([]*edge.Edge, error) {
	ctx = context.WithValue(ctx, txContextKey, w.tx)
	return w.base.FindEdges(ctx, query)
}

// DeleteEdge is not part of the EdgeRepository interface

// Add missing methods from EdgeRepository interface
func (w *transactionalEdgeWrapper) GetEdgesPage(ctx context.Context, query repository.EdgeQuery, pagination repository.Pagination) (*repository.EdgePage, error) {
	ctx = context.WithValue(ctx, txContextKey, w.tx)
	return w.base.GetEdgesPage(ctx, query, pagination)
}

func (w *transactionalEdgeWrapper) FindEdgesWithOptions(ctx context.Context, query repository.EdgeQuery, opts ...repository.QueryOption) ([]*edge.Edge, error) {
	ctx = context.WithValue(ctx, txContextKey, w.tx)
	return w.base.FindEdgesWithOptions(ctx, query, opts...)
}

func (w *transactionalEdgeWrapper) DeleteEdge(ctx context.Context, userID, edgeID string) error {
	ctx = context.WithValue(ctx, txContextKey, w.tx)
	return w.base.DeleteEdge(ctx, userID, edgeID)
}

func (w *transactionalEdgeWrapper) DeleteEdgesByNode(ctx context.Context, userID, nodeID string) error {
	ctx = context.WithValue(ctx, txContextKey, w.tx)
	return w.base.DeleteEdgesByNode(ctx, userID, nodeID)
}

func (w *transactionalEdgeWrapper) DeleteEdgesBetweenNodes(ctx context.Context, userID, sourceNodeID, targetNodeID string) error {
	ctx = context.WithValue(ctx, txContextKey, w.tx)
	return w.base.DeleteEdgesBetweenNodes(ctx, userID, sourceNodeID, targetNodeID)
}

// transactionalCategoryWrapper wraps category repository with transaction context
type transactionalCategoryWrapper struct {
	base repository.CategoryRepository
	tx   repository.Transaction
}

func (w *transactionalCategoryWrapper) CreateCategory(ctx context.Context, category category.Category) error {
	ctx = context.WithValue(ctx, txContextKey, w.tx)
	return w.base.CreateCategory(ctx, category)
}

func (w *transactionalCategoryWrapper) FindCategoryByID(ctx context.Context, userID, categoryID string) (*category.Category, error) {
	ctx = context.WithValue(ctx, txContextKey, w.tx)
	return w.base.FindCategoryByID(ctx, userID, categoryID)
}

func (w *transactionalCategoryWrapper) UpdateCategory(ctx context.Context, category category.Category) error {
	ctx = context.WithValue(ctx, txContextKey, w.tx)
	return w.base.UpdateCategory(ctx, category)
}

func (w *transactionalCategoryWrapper) DeleteCategory(ctx context.Context, userID, categoryID string) error {
	ctx = context.WithValue(ctx, txContextKey, w.tx)
	return w.base.DeleteCategory(ctx, userID, categoryID)
}

func (w *transactionalCategoryWrapper) FindCategories(ctx context.Context, query repository.CategoryQuery) ([]category.Category, error) {
	ctx = context.WithValue(ctx, txContextKey, w.tx)
	return w.base.FindCategories(ctx, query)
}

func (w *transactionalCategoryWrapper) AssignNodeToCategory(ctx context.Context, mapping node.NodeCategory) error {
	ctx = context.WithValue(ctx, txContextKey, w.tx)
	return w.base.AssignNodeToCategory(ctx, mapping)
}

func (w *transactionalCategoryWrapper) RemoveNodeFromCategory(ctx context.Context, userID, nodeID, categoryID string) error {
	ctx = context.WithValue(ctx, txContextKey, w.tx)
	return w.base.RemoveNodeFromCategory(ctx, userID, nodeID, categoryID)
}

// Add missing methods from CategoryRepository interface
func (w *transactionalCategoryWrapper) FindCategoriesByLevel(ctx context.Context, userID string, level int) ([]category.Category, error) {
	ctx = context.WithValue(ctx, txContextKey, w.tx)
	return w.base.FindCategoriesByLevel(ctx, userID, level)
}

func (w *transactionalCategoryWrapper) Save(ctx context.Context, category *category.Category) error {
	ctx = context.WithValue(ctx, txContextKey, w.tx)
	return w.base.Save(ctx, category)
}

func (w *transactionalCategoryWrapper) FindByID(ctx context.Context, userID, categoryID string) (*category.Category, error) {
	ctx = context.WithValue(ctx, txContextKey, w.tx)
	return w.base.FindByID(ctx, userID, categoryID)
}

func (w *transactionalCategoryWrapper) Delete(ctx context.Context, userID, categoryID string) error {
	ctx = context.WithValue(ctx, txContextKey, w.tx)
	return w.base.Delete(ctx, userID, categoryID)
}

func (w *transactionalCategoryWrapper) CreateCategoryHierarchy(ctx context.Context, hierarchy category.CategoryHierarchy) error {
	ctx = context.WithValue(ctx, txContextKey, w.tx)
	return w.base.CreateCategoryHierarchy(ctx, hierarchy)
}

func (w *transactionalCategoryWrapper) DeleteCategoryHierarchy(ctx context.Context, userID, parentID, childID string) error {
	ctx = context.WithValue(ctx, txContextKey, w.tx)
	return w.base.DeleteCategoryHierarchy(ctx, userID, parentID, childID)
}

func (w *transactionalCategoryWrapper) FindChildCategories(ctx context.Context, userID, parentID string) ([]category.Category, error) {
	ctx = context.WithValue(ctx, txContextKey, w.tx)
	return w.base.FindChildCategories(ctx, userID, parentID)
}

func (w *transactionalCategoryWrapper) FindParentCategory(ctx context.Context, userID, childID string) (*category.Category, error) {
	ctx = context.WithValue(ctx, txContextKey, w.tx)
	return w.base.FindParentCategory(ctx, userID, childID)
}

func (w *transactionalCategoryWrapper) GetCategoryTree(ctx context.Context, userID string) ([]category.Category, error) {
	ctx = context.WithValue(ctx, txContextKey, w.tx)
	return w.base.GetCategoryTree(ctx, userID)
}

func (w *transactionalCategoryWrapper) FindNodesByCategory(ctx context.Context, userID, categoryID string) ([]*node.Node, error) {
	ctx = context.WithValue(ctx, txContextKey, w.tx)
	return w.base.FindNodesByCategory(ctx, userID, categoryID)
}

func (w *transactionalCategoryWrapper) FindCategoriesForNode(ctx context.Context, userID, nodeID string) ([]category.Category, error) {
	ctx = context.WithValue(ctx, txContextKey, w.tx)
	return w.base.FindCategoriesForNode(ctx, userID, nodeID)
}

func (w *transactionalCategoryWrapper) BatchAssignCategories(ctx context.Context, mappings []node.NodeCategory) error {
	ctx = context.WithValue(ctx, txContextKey, w.tx)
	return w.base.BatchAssignCategories(ctx, mappings)
}

func (w *transactionalCategoryWrapper) UpdateCategoryNoteCounts(ctx context.Context, userID string, categoryCounts map[string]int) error {
	ctx = context.WithValue(ctx, txContextKey, w.tx)
	return w.base.UpdateCategoryNoteCounts(ctx, userID, categoryCounts)
}

// SimpleMemoryCacheWrapper wraps InMemoryCache to implement queries.Cache interface
type SimpleMemoryCacheWrapper struct {
	cache cache.Cache
}

// Get retrieves a value from the cache
func (c *SimpleMemoryCacheWrapper) Get(ctx context.Context, key string) ([]byte, bool, error) {
	return c.cache.Get(ctx, key)
}

// Set stores a value in the cache
func (c *SimpleMemoryCacheWrapper) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	return c.cache.Set(ctx, key, value, ttl)
}

// Delete removes a value from the cache
func (c *SimpleMemoryCacheWrapper) Delete(ctx context.Context, key string) error {
	return c.cache.Delete(ctx, key)
}

// Clear removes all values matching the pattern from the cache
func (c *SimpleMemoryCacheWrapper) Clear(ctx context.Context, pattern string) error {
	return c.cache.Clear(ctx, pattern)
}
func (w *transactionalNodeWrapper) BatchGetNodes(ctx context.Context, userID string, nodeIDs []string) (map[string]*node.Node, error) {
	return w.base.BatchGetNodes(ctx, userID, nodeIDs)
}
