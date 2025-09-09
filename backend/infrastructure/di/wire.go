//go:build wireinject
// +build wireinject

package di

import (
	"context"

	"backend/application/commands/bus"
	"backend/application/ports"
	querybus "backend/application/queries/bus"
	"backend/infrastructure/config"
	"backend/pkg/auth"
	"backend/pkg/observability"
	"github.com/google/wire"
	"go.uber.org/zap"
)

// Container holds all application dependencies
type Container struct {
	Config      *config.Config
	Logger      *zap.Logger
	NodeRepo    ports.NodeRepository
	GraphRepo   ports.GraphRepository
	EdgeRepo    ports.EdgeRepository
	EventBus    ports.EventBus
	EventStore  ports.EventStore
	UnitOfWork  ports.UnitOfWork
	CommandBus  *bus.CommandBus
	QueryBus    *querybus.QueryBus
	Cache       ports.Cache
	Metrics     *observability.Metrics
	RateLimiter *auth.DistributedRateLimiter
}

// SuperSet is the main provider set containing all providers
var SuperSet = wire.NewSet(
	ProvideLogger,
	ProvideAWSConfig,
	ProvideDynamoDBClient,
	ProvideEventBridgeClient,
	ProvideCloudWatchClient,
	ProvideNodeRepository,
	ProvideGraphRepository,
	ProvideEdgeRepository,
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
	wire.Struct(new(Container), "*"),
)

// InitializeContainer creates a fully wired container
func InitializeContainer(ctx context.Context, cfg *config.Config) (*Container, error) {
	wire.Build(SuperSet)
	return nil, nil // Wire will replace this
}
