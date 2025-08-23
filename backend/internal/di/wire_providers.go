//go:build wireinject
// +build wireinject

// Package di provides provider functions for Wire dependency injection.
// This file contains provider function declarations for Wire to use during code generation.
// The actual implementations are in providers.go (excluded during Wire generation).
package di

import (
	"context"

	"brain2-backend/internal/application/queries"
	"brain2-backend/internal/application/services"
	"brain2-backend/internal/config"
	domainServices "brain2-backend/internal/domain/services"
	"brain2-backend/internal/domain/shared"
	"brain2-backend/internal/features"
	"brain2-backend/internal/handlers"
	"brain2-backend/internal/infrastructure/observability"
	"brain2-backend/internal/infrastructure/persistence"
	"brain2-backend/internal/infrastructure/persistence/cache"
	"brain2-backend/internal/repository"

	aws "github.com/aws/aws-sdk-go-v2/aws"
	awsDynamodb "github.com/aws/aws-sdk-go-v2/service/dynamodb"
	awsEventbridge "github.com/aws/aws-sdk-go-v2/service/eventbridge"
	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"
)

// Configuration Providers
func provideConfig() (*config.Config, error) { panic("wire") }
func provideLogger(cfg *config.Config) (*zap.Logger, error) { panic("wire") }
func provideEnvironment(cfg *config.Config) config.Environment { panic("wire") }
func provideContext() context.Context { panic("wire") }

// AWS Infrastructure Providers
func provideAWSConfig(ctx context.Context, cfg *config.Config) (aws.Config, error) { panic("wire") }
func provideDynamoDBClient(awsCfg aws.Config, cfg *config.Config) *awsDynamodb.Client { panic("wire") }
func provideEventBridgeClient(awsCfg aws.Config) *awsEventbridge.Client { panic("wire") }

// Repository Providers
func provideNodeRepository(
	dynamoClient *awsDynamodb.Client,
	cfg *config.Config,
	logger *zap.Logger,
	factory *repository.RepositoryFactory,
	cache cache.Cache,
	metricsCollector *observability.Collector,
) repository.NodeRepository { panic("wire") }

func provideEdgeRepository(
	dynamoClient *awsDynamodb.Client,
	cfg *config.Config,
	logger *zap.Logger,
	cache cache.Cache,
	metricsCollector *observability.Collector,
) repository.EdgeRepository { panic("wire") }

func provideCategoryRepository(
	dynamoClient *awsDynamodb.Client,
	cfg *config.Config,
	logger *zap.Logger,
	factory *repository.RepositoryFactory,
	cache cache.Cache,
	metricsCollector *observability.Collector,
) repository.CategoryRepository { panic("wire") }

func provideKeywordRepository(
	dynamoClient *awsDynamodb.Client,
	cfg *config.Config,
) repository.KeywordRepository { panic("wire") }

func provideTransactionalRepository(
	dynamoClient *awsDynamodb.Client,
	cfg *config.Config,
	logger *zap.Logger,
) repository.TransactionalRepository { panic("wire") }

func provideGraphRepository(
	dynamoClient *awsDynamodb.Client,
	cfg *config.Config,
	logger *zap.Logger,
) repository.GraphRepository { panic("wire") }

func provideRepository(
	dynamoClient *awsDynamodb.Client,
	cfg *config.Config,
	logger *zap.Logger,
) repository.Repository { panic("wire") }

func provideIdempotencyStore(
	dynamoClient *awsDynamodb.Client,
	cfg *config.Config,
) repository.IdempotencyStore { panic("wire") }

func provideStore(
	dynamoClient *awsDynamodb.Client,
	cfg *config.Config,
	logger *zap.Logger,
) persistence.Store { panic("wire") }

// Cross-cutting Providers
func provideCache(cfg *config.Config, logger *zap.Logger) cache.Cache { panic("wire") }
func provideCacheAdapter(cache cache.Cache) queries.Cache { panic("wire") }
func provideMetricsCollector(cfg *config.Config, logger *zap.Logger) *observability.Collector { panic("wire") }
func provideRepositoryFactory(cfg *config.Config) *repository.RepositoryFactory { panic("wire") }
func provideTracerProvider(cfg *config.Config) (*observability.TracerProvider, error) { panic("wire") }

// Domain Service Providers
func provideFeatureService(cfg *config.Config) *features.FeatureService { panic("wire") }
func provideConnectionAnalyzer(cfg *config.Config) *domainServices.ConnectionAnalyzer { panic("wire") }
func provideEventBus(cfg *config.Config, logger *zap.Logger) shared.EventBus { panic("wire") }
func provideUnitOfWork(
	nodeRepo repository.NodeRepository,
	edgeRepo repository.EdgeRepository,
	keywordRepo repository.KeywordRepository,
	transactionalRepo repository.TransactionalRepository,
) repository.UnitOfWork { panic("wire") }

func provideEventStore(
	dynamoClient *awsDynamodb.Client,
	cfg *config.Config,
) repository.EventStore { panic("wire") }

func provideUnitOfWorkFactory(
	dynamoClient *awsDynamodb.Client,
	cfg *config.Config,
	eventBus shared.EventBus,
	eventStore repository.EventStore,
	logger *zap.Logger,
) repository.UnitOfWorkFactory { panic("wire") }

// Application Service Providers
func provideNodeService(
	nodeRepo repository.NodeRepository,
	edgeRepo repository.EdgeRepository,
	uowFactory repository.UnitOfWorkFactory,
	eventBus shared.EventBus,
	analyzer *domainServices.ConnectionAnalyzer,
	idempotencyStore repository.IdempotencyStore,
) *services.NodeService { panic("wire") }

func provideCategoryAppService(
	categoryRepo repository.CategoryRepository,
	uow repository.UnitOfWork,
	eventBus shared.EventBus,
) *services.CategoryService { panic("wire") }

func provideCleanupService(
	nodeRepo repository.NodeRepository,
	edgeRepo repository.EdgeRepository,
	idempotencyStore repository.IdempotencyStore,
	uowFactory repository.UnitOfWorkFactory,
) *services.CleanupService { panic("wire") }

// Query Service Providers
func provideNodeQueryService(
	nodeRepo repository.NodeRepository,
	edgeRepo repository.EdgeRepository,
	graphRepo repository.GraphRepository,
	cache cache.Cache,
) *queries.NodeQueryService { panic("wire") }

func provideCategoryQueryService(
	categoryRepo repository.CategoryRepository,
	nodeRepo repository.NodeRepository,
	cache cache.Cache,
	logger *zap.Logger,
) *queries.CategoryQueryService { panic("wire") }

func provideGraphQueryService(
	store persistence.Store,
	logger *zap.Logger,
	cache queries.Cache,
) *queries.GraphQueryService { panic("wire") }

// Handler Providers
func provideMemoryHandler(
	nodeService *services.NodeService,
	nodeQueryService *queries.NodeQueryService,
	graphQueryService *queries.GraphQueryService,
	eventBridgeClient *awsEventbridge.Client,
	coldStartProvider ColdStartInfoProvider,
) *handlers.MemoryHandler { panic("wire") }

func provideCategoryHandler(
	categoryService *services.CategoryService,
	categoryQueryService *queries.CategoryQueryService,
) *handlers.CategoryHandler { panic("wire") }

func provideHealthHandler() *handlers.HealthHandler { panic("wire") }

// Router Provider
func provideRouter(
	memoryHandler *handlers.MemoryHandler,
	categoryHandler *handlers.CategoryHandler,
	healthHandler *handlers.HealthHandler,
	cfg *config.Config,
) *chi.Mux { panic("wire") }

// Container Provider
func provideContainer(
	cfg *config.Config,
	logger *zap.Logger,
	dynamoClient *awsDynamodb.Client,
	eventBridgeClient *awsEventbridge.Client,
	nodeRepo repository.NodeRepository,
	edgeRepo repository.EdgeRepository,
	categoryRepo repository.CategoryRepository,
	keywordRepo repository.KeywordRepository,
	transactionalRepo repository.TransactionalRepository,
	graphRepo repository.GraphRepository,
	repository repository.Repository,
	idempotencyStore repository.IdempotencyStore,
	store persistence.Store,
	cache cache.Cache,
	metricsCollector *observability.Collector,
	tracerProvider *observability.TracerProvider,
	nodeService *services.NodeService,
	categoryService *services.CategoryService,
	cleanupService *services.CleanupService,
	nodeQueryService *queries.NodeQueryService,
	categoryQueryService *queries.CategoryQueryService,
	graphQueryService *queries.GraphQueryService,
	connectionAnalyzer *domainServices.ConnectionAnalyzer,
	eventBus shared.EventBus,
	unitOfWork repository.UnitOfWork,
	memoryHandler *handlers.MemoryHandler,
	categoryHandler *handlers.CategoryHandler,
	healthHandler *handlers.HealthHandler,
	router *chi.Mux,
	repositoryFactory *repository.RepositoryFactory,
	coldStartTracker *ColdStartTracker,
) *Container { panic("wire") }