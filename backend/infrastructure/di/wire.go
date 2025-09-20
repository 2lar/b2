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

// SuperSet is the main provider set containing all providers
var SuperSet = wire.NewSet(
	ProvideLogger,
	ProvideErrorHandler,
	ProvideAWSConfig,
	ProvideDynamoDBClient,
	ProvideEventBridgeClient,
	ProvideCloudWatchClient,
	ProvideNodeRepository,
	ProvideGraphRepository,
	ProvideEdgeRepository,
	ProvideGraphLazyService,
	ProvideGraphLoader,
	ProvideEventBus,
	ProvideEventPublisher,
	ProvideEventStore,
	ProvideUnitOfWork,
	ProvideMetrics,
	ProvideDistributedRateLimiter,
	ProvideDistributedLock,
	ProvideEdgeService,
	ProvideCommandBus,
	ProvideQueryBus,
	ProvideInMemoryCache,
	ProvideOperationStore,
	ProvideEventHandlerRegistry,
	ProvideOperationEventListener,
	ProvideGraphStatsProjection,
	ProvideMediator,
	ProvideAuthMiddleware,
	wire.Struct(new(Container), "*"),
)

// InitializeContainer creates a fully wired container
func InitializeContainer(ctx context.Context, cfg *config.Config) (*Container, error) {
	wire.Build(SuperSet)
	return nil, nil // Wire will replace this
}
