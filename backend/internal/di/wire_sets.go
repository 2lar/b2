// Package di provides Wire provider sets.
// This file contains the provider sets used by Wire to generate dependency injection code.
package di

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/wire"
)

// SuperSet combines all provider sets for the complete application.
// This demonstrates how to compose multiple provider sets in Wire.
var SuperSet = wire.NewSet(
	ConfigProviders,
	InfrastructureProviders,
	DomainProviders,
	ApplicationProviders,
	InterfaceProviders,
	AdditionalProviders,
	provideContainer,
	wire.Bind(new(http.Handler), new(*chi.Mux)), // Bind router as http.Handler
)

// ConfigProviders provides configuration-related dependencies.
// These are the foundation that other layers depend upon.
var ConfigProviders = wire.NewSet(
	provideConfig,
	provideLogger,
	provideEnvironment,
	provideContext,
)

// InfrastructureProviders provides all infrastructure components.
// This layer implements interfaces defined by inner layers (Dependency Inversion).
var InfrastructureProviders = wire.NewSet(
	// AWS Clients
	provideAWSConfig,
	provideDynamoDBClient,
	provideEventBridgeClient,
	
	// Repository Implementations
	provideNodeRepository,
	provideEdgeRepository,
	provideCategoryRepository,
	
	// Cross-cutting Concerns
	provideCache,
	provideMetricsCollector,
	provideCacheAdapter,
)

// DomainProviders provides domain services and business logic components.
// This layer has no external dependencies (Pure Domain).
var DomainProviders = wire.NewSet(
	provideFeatureService,
	provideConnectionAnalyzer,
	provideEventBus,
	provideUnitOfWork,
)

// ApplicationProviders provides application services (use cases).
// This layer orchestrates domain logic and infrastructure.
var ApplicationProviders = wire.NewSet(
	// Application Services (Command Side)
	provideNodeService,
	provideCategoryAppService,
	
	// Query Services (Query Side - CQRS)
	provideNodeQueryService,
	provideCategoryQueryService,
)

// InterfaceProviders provides interface layer components (handlers, middleware).
// This is the outermost layer that adapts external requests to application services.
var InterfaceProviders = wire.NewSet(
	provideRouter,
)

// AdditionalProviders provides all additional components not in the main sets.
var AdditionalProviders = wire.NewSet(
	// Additional Repositories
	provideKeywordRepository,
	provideTransactionalRepository,
	provideGraphRepository,
	provideRepository,
	provideIdempotencyStore,
	provideStore,
	
	// Additional Services
	provideCleanupService,
	provideGraphQueryService,
	
	// Handlers
	provideMemoryHandler,
	provideCategoryHandler,
	provideHealthHandler,
	
	// Advanced Components
	provideRepositoryFactory,
	provideTracerProvider,
	provideEventStore,
	provideUnitOfWorkFactory,
	
	// Cold Start Tracking
	ProvideColdStartTracker,
	wire.Bind(new(ColdStartInfoProvider), new(*ColdStartTracker)),
)