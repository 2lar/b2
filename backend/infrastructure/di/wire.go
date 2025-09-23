//go:build wireinject
// +build wireinject

package di

import (
	"context"
	"net/http"

	"backend/application/commands/bus"
	appevents "backend/application/events"
	"backend/application/events/listeners"
	"backend/application/mediator"
	"backend/application/ports"
	"backend/application/projections"
	querybus "backend/application/queries/bus"
	"backend/application/services"
	"backend/infrastructure/config"
	"backend/pkg/auth"
	"backend/pkg/errors"
	"backend/pkg/observability"
	"github.com/google/wire"
	"go.uber.org/zap"
)

// Container holds all application dependencies
type Container struct {
	Config                 *config.Config
	Logger                 *zap.Logger
	ErrorHandler           *errors.ErrorHandler
	NodeRepo               ports.NodeRepository
	GraphRepo              ports.GraphRepository
	EdgeRepo               ports.EdgeRepository
	EventBus               ports.EventBus
	EventStore             ports.EventStore
	UnitOfWork             ports.UnitOfWork
	CommandBus             *bus.CommandBus
	QueryBus               *querybus.QueryBus
	Cache                  ports.Cache
	Metrics                *observability.Metrics
	RateLimiter            *auth.DistributedRateLimiter
	OperationStore         ports.OperationStore
	Mediator               *mediator.Mediator
	EventHandlerRegistry   *appevents.HandlerRegistry
	OperationEventListener *listeners.OperationEventListener
	GraphStatsProjection   *projections.GraphStatsProjection
	GraphLazyService       *services.GraphLazyService
	GraphLoader            *services.GraphLoader
	AuthMiddleware         func(http.Handler) http.Handler
}

// SuperSet is the main provider set. Order here is for readability only;
// Wire determines the actual initialization order from dependencies and
// generates it in wire_gen.go. Grouped from leaves → higher-level wiring.
var SuperSet = wire.NewSet(
    // 1) Core configuration and logging
    ProvideLogger,        // leaf
    ProvideErrorHandler,  // deps: logger, config (debug)

    // 2) AWS configuration and low-level clients
    ProvideAWSConfig,         // leaf (uses cfg env/region)
    ProvideDynamoDBClient,    // deps: AWS config
    ProvideEventBridgeClient, // deps: AWS config
    ProvideCloudWatchClient,  // deps: AWS config

    // 3) Local cache and operation tracking stores (leaf, few deps)
    ProvideInMemoryCache, // leaf
    ProvideOperationStore, // leaf (in-memory)

    // 4) Infra utilities
    // Both depend on DynamoDB client + cfg; lock also logs
    ProvideDistributedRateLimiter, // deps: dynamodb client, config
    ProvideDistributedLock,        // deps: dynamodb client, config, logger

    // 5) Persistence layer (repos, event store)
    // Repositories needing DynamoDB client + config + logger:
    ProvideNodeRepository, // deps: dynamodb client, config (table/index), logger
    ProvideEdgeRepository, // deps: dynamodb client, config (table/index), logger
    // Graph repository additionally wires NodeRepo + EdgeRepo for aggregate saves:
    ProvideGraphRepository, // deps: dynamodb client, node repo, edge repo, config, logger
    // Event store uses DynamoDB to persist outbox events
    ProvideEventStore,      // deps: dynamodb client, config (table)

    // 6) Messaging and metrics
    // Event bus and metrics (AWS clients + cfg + logger)
    ProvideEventBus,       // deps: EventBridge client, config (bus), logger
    ProvideEventPublisher, // deps: event bus
    ProvideMetrics,        // deps: CloudWatch client, config (namespace)

    // 7) Unit of Work (placed after event publisher for readability)
    // Coordinates transactional writes and outbox publishing
    ProvideUnitOfWork,      // deps: dynamodb client, node/edge/graph repos, event store, event publisher

    // 8) Application services
    // Services depending on repos + cfg + logger
    ProvideGraphLazyService, // deps: node repo, edge repo, config, logger
    ProvideGraphLoader,      // deps: graph repo, node repo, edge repo, logger
    ProvideEdgeService,      // deps: node repo, graph repo, edge repo, cfg.EdgeCreation, logger

    // 9) CQRS buses and mediator
    // Command bus wires handlers requiring many deps (UoW, repos, services, events)
    ProvideCommandBus, // deps: uow, node/edge/graph repos, graph lazy service, event store, event bus/publisher, distributed lock, metrics, cfg, logger
    ProvideQueryBus,   // deps: graph/node/edge repos, cache, operation store, logger
    ProvideMediator,   // deps: command bus, query bus, metrics, logger

    // 10) Event handlers and projections
    ProvideEventHandlerRegistry,   // deps: logger
    ProvideOperationEventListener, // deps: operation store, logger
    ProvideGraphStatsProjection,   // deps: cache, logger

    // 11) HTTP
    ProvideAuthMiddleware, // deps: cfg, logger

    // 12) Container assembly
    wire.Struct(new(Container), "*"),
)

// InitializeContainer creates a fully wired container
func InitializeContainer(ctx context.Context, cfg *config.Config) (*Container, error) {
	wire.Build(SuperSet)
	return nil, nil // Wire will replace this
}
