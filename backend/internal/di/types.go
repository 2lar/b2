// Package di provides types for dependency injection.
// This file contains shared types that are used by both Wire and the manual container.
package di

import (
	"context"
	"time"

	"brain2-backend/internal/application/queries"
	"brain2-backend/internal/application/services"
	"brain2-backend/internal/config"
	domainServices "brain2-backend/internal/domain/services"
	"brain2-backend/internal/domain/shared"
	"brain2-backend/internal/handlers"
	"brain2-backend/internal/infrastructure/observability"
	"brain2-backend/internal/infrastructure/persistence"
	"brain2-backend/internal/infrastructure/persistence/cache"
	"brain2-backend/internal/repository"

	awsDynamodb "github.com/aws/aws-sdk-go-v2/service/dynamodb"
	awsEventbridge "github.com/aws/aws-sdk-go-v2/service/eventbridge"
	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"
)

// Container holds all application dependencies with lifecycle management.
// Enhanced for Phase 2 repository pattern excellence
type Container struct {
	// Configuration
	Config    *config.Config
	TableName string
	IndexName string

	// Cold start tracking
	ColdStartTime *time.Time
	IsColdStart   bool

	// AWS Clients
	DynamoDBClient    *awsDynamodb.Client
	EventBridgeClient *awsEventbridge.Client

	// Repository Layer - Phase 2 Enhanced Architecture
	NodeRepository          repository.NodeRepository
	EdgeRepository          repository.EdgeRepository
	KeywordRepository       repository.KeywordRepository
	TransactionalRepository repository.TransactionalRepository
	CategoryRepository      repository.CategoryRepository
	GraphRepository         repository.GraphRepository

	// Composed repository for backward compatibility
	Repository       repository.Repository
	IdempotencyStore repository.IdempotencyStore

	// Phase 2 Repository Pattern Enhancements
	RepositoryFactory  *repository.RepositoryFactory
	UnitOfWorkProvider repository.UnitOfWorkProvider
	QueryExecutor      repository.QueryExecutor
	RepositoryManager  repository.RepositoryManager

	// Cross-cutting concerns
	Logger           *zap.Logger
	Cache            cache.Cache
	MetricsCollector *observability.Collector
	TracerProvider   *observability.TracerProvider
	Store            persistence.Store

	// Phase 3: Application Service Layer (CQRS)
	NodeAppService       *services.NodeService
	CategoryAppService   *services.CategoryService
	CleanupService       *services.CleanupService
	NodeQueryService     *queries.NodeQueryService
	CategoryQueryService *queries.CategoryQueryService
	GraphQueryService    *queries.GraphQueryService

	// Domain Services
	ConnectionAnalyzer *domainServices.ConnectionAnalyzer
	EventBus           shared.EventBus
	UnitOfWork         repository.UnitOfWork
	UnitOfWorkFactory  repository.UnitOfWorkFactory

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

// ColdStartInfoProvider interface for cold start tracking.
type ColdStartInfoProvider interface {
	GetTimeSinceColdStart() time.Duration
	IsPostColdStartRequest() bool
}

// HealthChecker interface for health checks.
type HealthChecker interface {
	Health(ctx context.Context) map[string]string
}

// queryCacheAdapter adapts cache.Cache to queries.Cache.
type queryCacheAdapter struct {
	inner cache.Cache
}

func (a *queryCacheAdapter) Get(ctx context.Context, key string) (interface{}, bool) {
	data, found, _ := a.inner.Get(ctx, key)
	if !found {
		return nil, false
	}
	return data, true
}

func (a *queryCacheAdapter) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) {
	// Would need to serialize value to []byte
	a.inner.Set(ctx, key, nil, ttl)
}

func (a *queryCacheAdapter) Delete(ctx context.Context, key string) {
	a.inner.Delete(ctx, key)
}

