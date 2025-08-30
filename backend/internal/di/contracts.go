// Package di provides dependency injection contracts for clean architecture.
// These interfaces define the contracts between layers, enabling loose coupling
// and better testability through dependency inversion.
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
	"brain2-backend/internal/infrastructure/observability"
	"brain2-backend/internal/infrastructure/persistence"
	persistenceCache "brain2-backend/internal/infrastructure/persistence/cache"
	"brain2-backend/internal/repository"

	awsDynamodb "github.com/aws/aws-sdk-go-v2/service/dynamodb"
	awsEventbridge "github.com/aws/aws-sdk-go-v2/service/eventbridge"
	"go.uber.org/zap"
)

// ============================================================================
// INFRASTRUCTURE CONTRACTS
// ============================================================================

// IInfrastructureContainer defines the contract for infrastructure dependencies.
type IInfrastructureContainer interface {
	GetConfig() *config.Config
	GetDynamoDBClient() *awsDynamodb.Client
	GetEventBridgeClient() *awsEventbridge.Client
	GetHTTPClient() *http.Client
	GetLogger() *zap.Logger
	GetCache() persistenceCache.Cache
	GetMetricsCollector() *observability.Collector
	GetTracerProvider() *observability.TracerProvider
	GetStore() persistence.Store
	Shutdown() error
}

// ============================================================================
// REPOSITORY CONTRACTS
// ============================================================================

// IRepositoryContainer defines the contract for repository dependencies.
type IRepositoryContainer interface {
	// Combined repositories (for backward compatibility during migration)
	GetNodeRepository() repository.NodeRepository
	GetEdgeRepository() repository.EdgeRepository
	GetCategoryRepository() repository.CategoryRepository
	GetGraphRepository() repository.GraphRepository
	GetKeywordRepository() repository.KeywordRepository
	GetTransactionalRepository() repository.TransactionalRepository
	GetIdempotencyStore() repository.IdempotencyStore
	
	// CQRS Readers
	GetNodeReader() repository.NodeReader
	GetEdgeReader() repository.EdgeReader
	GetCategoryReader() repository.CategoryReader
	
	// CQRS Writers
	GetNodeWriter() repository.NodeWriter
	GetEdgeWriter() repository.EdgeWriter
	GetCategoryWriter() repository.CategoryWriter
	
	// Unit of Work
	GetUnitOfWork() repository.UnitOfWork
	GetUnitOfWorkFactory() repository.UnitOfWorkFactory
	
	// Repository Factory
	GetRepositoryFactory() repository.RepositoryFactory
}

// ============================================================================
// SERVICE CONTRACTS
// ============================================================================

// IServiceContainer defines the contract for service dependencies.
type IServiceContainer interface {
	// Command Services
	GetNodeCommandService() *services.NodeService
	GetCategoryCommandService() *services.CategoryService
	
	// Query Services
	GetNodeQueryService() *queries.NodeQueryService
	GetCategoryQueryService() *queries.CategoryQueryService
	GetGraphQueryService() *queries.GraphQueryService
	
	// Domain Services
	GetConnectionAnalyzer() *domainServices.ConnectionAnalyzer
	GetEventBus() shared.EventBus
	
	// Supporting Services
	GetCleanupService() *services.CleanupService
}

// ============================================================================
// HANDLER CONTRACTS
// ============================================================================

// IHandlerContainer defines the contract for HTTP handler dependencies.
type IHandlerContainer interface {
	GetRouter() http.Handler
	GetMiddleware() []func(http.Handler) http.Handler
	
	// Individual handlers can be accessed if needed
	GetNodeHandler() interface{}
	GetCategoryHandler() interface{}
	GetHealthHandler() interface{}
	GetMetricsHandler() http.HandlerFunc
}

// ============================================================================
// APPLICATION CONTAINER CONTRACT
// ============================================================================

// IApplicationContainer defines the root container contract.
type IApplicationContainer interface {
	// Sub-containers
	GetInfrastructure() IInfrastructureContainer
	GetRepositories() IRepositoryContainer
	GetServices() IServiceContainer
	GetHandlers() IHandlerContainer
	
	// Application metadata
	GetVersion() string
	GetEnvironment() string
	GetStartTime() time.Time
	IsColdStart() bool
	
	// Lifecycle
	Shutdown(ctx context.Context) error
	Validate() error
	Health(ctx context.Context) map[string]interface{}
	
	// HTTP handler
	GetHTTPHandler() http.Handler
	
	// Cold start tracking
	SetColdStartInfo(coldStartTime time.Time, isColdStart bool)
}

// ============================================================================
// FACTORY CONTRACTS
// ============================================================================

// IContainerFactory defines the contract for creating containers.
type IContainerFactory interface {
	CreateApplicationContainer(cfg *config.Config) (IApplicationContainer, error)
	CreateInfrastructureContainer(cfg *config.Config) (IInfrastructureContainer, error)
	CreateRepositoryContainer(infra IInfrastructureContainer) (IRepositoryContainer, error)
	CreateServiceContainer(repos IRepositoryContainer, infra IInfrastructureContainer) (IServiceContainer, error)
	CreateHandlerContainer(services IServiceContainer, infra IInfrastructureContainer) (IHandlerContainer, error)
}

// ============================================================================
// PROVIDER CONTRACTS
// ============================================================================

// IDependencyProvider defines a contract for providing dependencies.
// This can be used for more advanced DI scenarios.
type IDependencyProvider interface {
	// Infrastructure providers
	ProvideLogger() *zap.Logger
	ProvideCache() persistenceCache.Cache
	ProvideDynamoDBClient() *awsDynamodb.Client
	ProvideEventBridgeClient() *awsEventbridge.Client
	
	// Repository providers
	ProvideNodeRepository() repository.NodeRepository
	ProvideEdgeRepository() repository.EdgeRepository
	ProvideCategoryRepository() repository.CategoryRepository
	
	// Service providers
	ProvideNodeService() *services.NodeService
	ProvideCategoryService() *services.CategoryService
	
	// Handler providers
	ProvideRouter() http.Handler
}

// ============================================================================
// LIFECYCLE CONTRACTS
// ============================================================================

// IInitializable defines the contract for components that need initialization.
type IInitializable interface {
	Initialize(ctx context.Context) error
}

// IShutdownable defines the contract for components that need graceful shutdown.
type IShutdownable interface {
	Shutdown(ctx context.Context) error
}

// IHealthCheckable defines the contract for components that provide health status.
type IHealthCheckable interface {
	HealthCheck(ctx context.Context) error
}

// ILifecycle combines all lifecycle contracts.
type ILifecycle interface {
	IInitializable
	IShutdownable
	IHealthCheckable
}

// ============================================================================
// HOOK CONTRACTS
// ============================================================================

// IContainerHooks defines hooks for container lifecycle events.
type IContainerHooks interface {
	OnBeforeInitialize(ctx context.Context) error
	OnAfterInitialize(ctx context.Context) error
	OnBeforeShutdown(ctx context.Context) error
	OnAfterShutdown(ctx context.Context) error
}

// ============================================================================
// MIDDLEWARE CONTRACTS
// ============================================================================

// IMiddlewareProvider defines the contract for providing middleware.
type IMiddlewareProvider interface {
	GetRequestIDMiddleware() func(http.Handler) http.Handler
	GetLoggingMiddleware() func(http.Handler) http.Handler
	GetRecoveryMiddleware() func(http.Handler) http.Handler
	GetTimeoutMiddleware(timeout time.Duration) func(http.Handler) http.Handler
	GetCircuitBreakerMiddleware() func(http.Handler) http.Handler
	GetRateLimitMiddleware() func(http.Handler) http.Handler
	GetCORSMiddleware() func(http.Handler) http.Handler
	GetAuthMiddleware() func(http.Handler) http.Handler
}

// ============================================================================
// CONFIGURATION CONTRACTS
// ============================================================================

// IConfigProvider defines the contract for configuration management.
type IConfigProvider interface {
	GetConfig() *config.Config
	ReloadConfig() error
	WatchConfig(callback func(*config.Config)) error
	ValidateConfig() error
}

// IFeatureFlagProvider defines the contract for feature flag management.
type IFeatureFlagProvider interface {
	IsEnabled(feature string) bool
	GetFeatureValue(feature string) interface{}
	SetFeature(feature string, enabled bool) error
	GetAllFeatures() map[string]bool
}