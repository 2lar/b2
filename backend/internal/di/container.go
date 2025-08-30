// Package di provides dependency injection for the Brain2 backend.
// This file contains the minimal Container implementation for backward compatibility.
package di

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"brain2-backend/internal/application/services"
	"brain2-backend/internal/config"
	"brain2-backend/internal/errors"
)

// NewContainer creates a new Container by wrapping ApplicationContainer.
// This maintains backward compatibility while using the clean architecture.
func NewContainer() (*Container, error) {
	// Load configuration first
	cfg := config.LoadConfig()
	
	// Create the new application container with focused sub-containers
	appContainer, err := NewApplicationContainer(&cfg)
	if err != nil {
		return nil, errors.Internal("APP_CONTAINER_INIT_FAILED", "Failed to initialize application container").
			WithCause(err).
			Build()
	}
	
	// Create a compatibility wrapper that maps the old Container interface
	// to the new ApplicationContainer structure
	container := createCompatibilityWrapper(appContainer, &cfg)
	
	return container, nil
}

// createCompatibilityWrapper creates a Container that wraps the ApplicationContainer
// for backward compatibility with existing code.
func createCompatibilityWrapper(app *ApplicationContainer, cfg *config.Config) *Container {
	// Create the Container with all fields mapped from ApplicationContainer
	c := &Container{
		// Configuration
		Config:    cfg,
		TableName: cfg.Database.TableName,
		IndexName: cfg.Database.IndexName,
		
		// Cold start tracking
		ColdStartTime: &app.StartTime,
		IsColdStart:   app.IsColdStart,
		
		// AWS Clients (from Infrastructure container)
		DynamoDBClient:    app.Infrastructure.DynamoDBClient,
		EventBridgeClient: app.Infrastructure.EventBridgeClient,
		
		// Repository Layer (from Repository container)
		NodeRepository:          app.Repositories.Node,
		EdgeRepository:          app.Repositories.Edge,
		KeywordRepository:       app.Repositories.Keyword,
		TransactionalRepository: app.Repositories.Transactional,
		CategoryRepository:      app.Repositories.Category,
		GraphRepository:         app.Repositories.Graph,
		IdempotencyStore:        app.Repositories.Idempotency,
		// UnitOfWorkProvider is intentionally left nil as it's being deprecated
		
		// Cross-cutting concerns (from Infrastructure container)
		Logger:           app.Infrastructure.Logger,
		Cache:            app.Infrastructure.Cache,
		MetricsCollector: app.Infrastructure.MetricsCollector,
		TracerProvider:   app.Infrastructure.TracerProvider,
		
		// Application Services (from Service container)
		NodeAppService:       app.Services.NodeCommandService,
		CategoryService:      app.Services.CategoryCommandService,
		CategoryAppService:   app.Services.CategoryCommandService,  // Set both for backward compatibility
		NodeQueryService:     app.Services.NodeQueryService,
		CategoryQueryService: app.Services.CategoryQueryService,
		GraphQueryService:    app.Services.GraphQueryService,
		CleanupService:       app.Services.CleanupService,
		
		// Domain Services (from Service container)
		ConnectionAnalyzer: app.Services.ConnectionAnalyzer,
		EventBus:           app.Services.EventBus,
		
		// HTTP Handlers (from Handler container)
		Memory:          app.Handlers.Memory,
		MemoryHandler:   app.Handlers.Memory,  // Set both for backward compatibility
		Category:        app.Handlers.Category,
		CategoryHandler: app.Handlers.Category,  // Set both for backward compatibility
		HealthHandler:   app.Handlers.HealthHandler,
		MetricsHandler:  app.Handlers.MetricsHandler,
		
		// Router and middleware
		Router:     app.Handlers.Router,
		Middleware: app.Handlers.Middleware,
		
		// Store reference to app container for shutdown
		appContainer:      app,
		shutdownFunctions: make([]func() error, 0),
		middlewareConfig:  make(map[string]any),
	}
	
	// Add shutdown function that delegates to app container
	c.shutdownFunctions = append(c.shutdownFunctions, func() error {
		return app.Shutdown(context.Background())
	})
	
	return c
}

// Shutdown gracefully shuts down the container and all its components.
func (c *Container) Shutdown(ctx context.Context) error {
	log.Println("Shutting down dependency injection container...")
	
	// Execute registered shutdown functions
	for _, fn := range c.shutdownFunctions {
		if err := fn(); err != nil {
			log.Printf("Error during shutdown: %v", err)
		}
	}
	
	// Delegate to ApplicationContainer if available
	if c.appContainer != nil {
		if err := c.appContainer.Shutdown(ctx); err != nil {
			log.Printf("Error shutting down application container: %v", err)
		}
	}
	
	log.Println("Container shutdown completed successfully")
	return nil
}

// Validate ensures all required dependencies are initialized.
func (c *Container) Validate() error {
	if c.Config == nil {
		return fmt.Errorf("config not initialized")
	}
	if c.DynamoDBClient == nil {
		return errors.Internal("DYNAMODB_NIL", "DynamoDB client not initialized").
			Build()
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

// GetNodeHandler returns the node handler.
func (c *Container) GetNodeHandler() interface{} {
	return c.MemoryHandler
}

// GetCategoryHandler returns the category handler.
func (c *Container) GetCategoryHandler() interface{} {
	return c.CategoryHandler
}

// AddShutdownFunction registers a function to be called during shutdown.
func (c *Container) AddShutdownFunction(fn func() error) {
	c.shutdownFunctions = append(c.shutdownFunctions, fn)
}

// GetColdStartTime returns the cold start time.
func (c *Container) GetColdStartTime() time.Time {
	if c.ColdStartTime != nil {
		return *c.ColdStartTime
	}
	return time.Now()
}

// SetColdStartProvider allows setting a cold start provider (for testing).
func (c *Container) SetColdStartProvider(provider interface{}) {
	// This is a no-op for compatibility
}

// SetColdStartInfo sets the cold start information.
func (c *Container) SetColdStartInfo(startTime time.Time, isColdStart bool) {
	c.IsColdStart = isColdStart
	c.ColdStartTime = &startTime
}

// GetRouter returns the HTTP router.
func (c *Container) GetRouter() http.Handler {
	return c.Router
}

// GetCleanupService returns the cleanup service instance.
func (c *Container) GetCleanupService() *services.CleanupService {
	return c.CleanupService
}

// Health returns the health status of all components.
func (c *Container) Health(ctx context.Context) map[string]string {
	health := make(map[string]string)
	health["container"] = "healthy"
	health["config"] = "loaded"
	
	if c.DynamoDBClient != nil {
		health["dynamodb"] = "connected"
	} else {
		health["dynamodb"] = "not initialized"
	}
	
	if c.EventBridgeClient != nil {
		health["eventbridge"] = "connected"
	} else {
		health["eventbridge"] = "not initialized"
	}
	
	if c.NodeRepository != nil {
		health["repositories"] = "initialized"
	} else {
		health["repositories"] = "not initialized"
	}
	
	return health
}