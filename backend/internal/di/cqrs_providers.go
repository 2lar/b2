// Package di provides dependency injection for CQRS components
package di

import (
	"brain2-backend/internal/core/application/commands"
	"brain2-backend/internal/core/application/cqrs"
	"brain2-backend/internal/core/application/ports"
	corePorts "brain2-backend/internal/core/application/ports"
	"brain2-backend/internal/core/application/queries"
	coreCache "brain2-backend/internal/core/infrastructure/adapters/cache"
	coreDynamodb "brain2-backend/internal/core/infrastructure/adapters/dynamodb"
	"brain2-backend/internal/core/infrastructure/adapters/eventbridge"
	"brain2-backend/internal/core/infrastructure/adapters/graph"
	"brain2-backend/internal/core/infrastructure/adapters/metrics"
	"brain2-backend/internal/core/infrastructure/adapters/otel"
	"brain2-backend/internal/core/infrastructure/adapters/search"
	zapAdapter "brain2-backend/internal/core/infrastructure/adapters/zap"
	"brain2-backend/internal/config"
	
	awsDynamodb "github.com/aws/aws-sdk-go-v2/service/dynamodb"
	awsEventbridge "github.com/aws/aws-sdk-go-v2/service/eventbridge"
	"go.uber.org/zap"
)

// ============================================================================
// PORTS ADAPTERS - Bridge infrastructure to core interfaces
// ============================================================================

// provideCoreLogger adapts the zap logger to the core ports.Logger interface
func provideCoreLogger(logger *zap.Logger) corePorts.Logger {
	return zapAdapter.NewLoggerAdapter(logger)
}

// provideCoreMetrics provides the core metrics interface
func provideCoreMetrics(logger corePorts.Logger) corePorts.Metrics {
	return metrics.NewSimpleMetrics(logger)
}

// provideCoreTracer provides the core tracer interface
func provideCoreTracer() corePorts.Tracer {
	return otel.NewTracerAdapter("brain2-backend")
}

// provideCoreCache provides the core cache interface
func provideCoreCache() corePorts.Cache {
	return coreCache.NewSimpleCache()
}

// provideCoreNodeRepository adapts DynamoDB to core NodeRepository interface
func provideCoreNodeRepository(
	client *awsDynamodb.Client,
	cfg *config.Config,
	logger corePorts.Logger,
) corePorts.NodeRepository {
	return coreDynamodb.NewNodeRepository(
		client,
		cfg.Database.TableName,
		cfg.Database.IndexName,
		logger,
	)
}

// provideCoreEdgeRepository adapts DynamoDB to core EdgeRepository interface
func provideCoreEdgeRepository(
	client *awsDynamodb.Client,
	cfg *config.Config,
	logger corePorts.Logger,
) corePorts.EdgeRepository {
	return coreDynamodb.NewEdgeRepository(
		client,
		cfg.Database.TableName,
		logger,
	)
}

// provideCoreEventBus adapts EventBridge to core EventBus interface
func provideCoreEventBus(
	client *awsEventbridge.Client,
	cfg *config.Config,
	logger corePorts.Logger,
) corePorts.EventBus {
	return eventbridge.NewEventBridgePublisher(
		client,
		cfg.Events.EventBusName,
		"brain2.api",
		logger,
	)
}

// provideCoreUnitOfWorkFactory provides the UnitOfWork factory
func provideCoreUnitOfWorkFactory(
	client *awsDynamodb.Client,
	nodeRepo corePorts.NodeRepository,
	edgeRepo corePorts.EdgeRepository,
	eventStore corePorts.EventStore,
	logger corePorts.Logger,
) corePorts.UnitOfWorkFactory {
	return coreDynamodb.NewUnitOfWorkFactory(
		client,
		nodeRepo,
		edgeRepo,
		eventStore,
		logger,
	)
}

// provideCoreEventStore provides the event store
func provideCoreEventStore(
	client *awsDynamodb.Client,
	cfg *config.Config,
	logger corePorts.Logger,
) corePorts.EventStore {
	return coreDynamodb.NewEventStore(
		client,
		cfg.Database.TableName,
		logger,
	)
}

// provideCoreGraphAnalyzer provides graph analysis capabilities
func provideCoreGraphAnalyzer(
	nodeRepo corePorts.NodeRepository,
	edgeRepo corePorts.EdgeRepository,
	logger corePorts.Logger,
) corePorts.GraphAnalyzer {
	return graph.NewSimpleGraphAnalyzer(nodeRepo, edgeRepo, logger)
}

// provideCoreQueryRepository provides read model queries
func provideCoreQueryRepository(
	client *awsDynamodb.Client,
	cfg *config.Config,
	logger corePorts.Logger,
) corePorts.QueryRepository {
	return coreDynamodb.NewQueryRepository(
		client,
		cfg.Database.TableName,
		cfg.Database.IndexName,
		logger,
	)
}

// provideCoreSearchService provides search capabilities
func provideCoreSearchService(logger corePorts.Logger) corePorts.SearchService {
	return search.NewSimpleSearchService(logger)
}

// ============================================================================
// CQRS INFRASTRUCTURE - Command and Query Buses
// ============================================================================

// The basic bus providers have been removed as they're now created internally
// by provideConfiguredCommandBus and provideConfiguredQueryBus

// ============================================================================
// COMMAND HANDLERS - Write operations
// ============================================================================

// provideCreateNodeHandler creates the handler for CreateNode commands
func provideCreateNodeHandler(
	nodeRepo ports.NodeRepository,
	edgeRepo ports.EdgeRepository,
	eventStore ports.EventStore,
	eventBus ports.EventBus,
	uowFactory ports.UnitOfWorkFactory,
	logger ports.Logger,
	metrics ports.Metrics,
) *commands.CreateNodeHandler {
	return commands.NewCreateNodeHandler(
		nodeRepo,
		eventStore,
		eventBus,
		uowFactory,
		logger,
		metrics,
	)
}

// provideUpdateNodeHandler creates the handler for UpdateNode commands
func provideUpdateNodeHandler(
	nodeRepo ports.NodeRepository,
	eventStore ports.EventStore,
	eventBus ports.EventBus,
	uowFactory ports.UnitOfWorkFactory,
	logger ports.Logger,
	metrics ports.Metrics,
) *commands.UpdateNodeHandler {
	return commands.NewUpdateNodeHandler(
		nodeRepo,
		eventStore,
		eventBus,
		uowFactory,
		logger,
		metrics,
	)
}

// provideDeleteNodeHandler creates the handler for DeleteNode commands
func provideDeleteNodeHandler(
	nodeRepo ports.NodeRepository,
	edgeRepo ports.EdgeRepository,
	eventBus ports.EventBus,
	uowFactory ports.UnitOfWorkFactory,
	logger ports.Logger,
	metrics ports.Metrics,
) *commands.DeleteNodeHandler {
	return commands.NewDeleteNodeHandler(
		nodeRepo,
		edgeRepo,
		eventBus,
		uowFactory,
		logger,
		metrics,
	)
}

// provideConnectNodesHandler creates the handler for ConnectNodes commands
func provideConnectNodesHandler(
	nodeRepo ports.NodeRepository,
	edgeRepo ports.EdgeRepository,
	graphAnalyzer ports.GraphAnalyzer,
	eventBus ports.EventBus,
	logger ports.Logger,
	metrics ports.Metrics,
) *commands.ConnectNodesHandler {
	return commands.NewConnectNodesHandler(
		nodeRepo,
		edgeRepo,
		graphAnalyzer,
		eventBus,
		logger,
		metrics,
	)
}

// provideDisconnectNodesHandler creates the handler for DisconnectNodes commands
func provideDisconnectNodesHandler(
	edgeRepo ports.EdgeRepository,
	eventBus ports.EventBus,
	logger ports.Logger,
	metrics ports.Metrics,
) *commands.DisconnectNodesHandler {
	return commands.NewDisconnectNodesHandler(
		edgeRepo,
		eventBus,
		logger,
		metrics,
	)
}

// provideBulkDeleteNodesHandler creates the handler for BulkDeleteNodes commands
func provideBulkDeleteNodesHandler(
	nodeRepo ports.NodeRepository,
	edgeRepo ports.EdgeRepository,
	eventBus ports.EventBus,
	uowFactory ports.UnitOfWorkFactory,
	logger ports.Logger,
	metrics ports.Metrics,
) *commands.BulkDeleteNodesHandler {
	return commands.NewBulkDeleteNodesHandler(
		nodeRepo,
		edgeRepo,
		eventBus,
		uowFactory,
		logger,
		metrics,
	)
}

// ============================================================================
// QUERY HANDLERS - Read operations
// ============================================================================

// provideGetNodeByIDHandler creates the handler for GetNodeByID queries
func provideGetNodeByIDHandler(
	nodeRepo ports.NodeRepository,
	cache ports.Cache,
	logger ports.Logger,
) *queries.GetNodeByIDHandler {
	return queries.NewGetNodeByIDHandler(
		nodeRepo,
		cache,
		logger,
	)
}

// provideGetNodesByUserHandler creates the handler for GetNodesByUser queries
func provideGetNodesByUserHandler(
	queryRepo ports.QueryRepository,
	cache ports.Cache,
	logger ports.Logger,
) *queries.GetNodesByUserHandler {
	return queries.NewGetNodesByUserHandler(
		queryRepo,
		cache,
		logger,
	)
}

// provideSearchNodesHandler creates the handler for SearchNodes queries
func provideSearchNodesHandler(
	searchService ports.SearchService,
	logger ports.Logger,
) *queries.SearchNodesHandler {
	return queries.NewSearchNodesHandler(
		searchService,
		logger,
	)
}

// provideGetGraphHandler creates the handler for GetGraph queries
func provideGetGraphHandler(
	queryRepo ports.QueryRepository,
	cache ports.Cache,
	logger ports.Logger,
) *queries.GetGraphHandler {
	return queries.NewGetGraphHandler(
		queryRepo,
		cache,
		logger,
	)
}

// ============================================================================
// HANDLER REGISTRATION - Wire handlers to buses
// ============================================================================

// registerCommandHandlers registers all command handlers with the command bus
func registerCommandHandlers(
	bus *cqrs.CommandBus,
	createNodeHandler *commands.CreateNodeHandler,
	updateNodeHandler *commands.UpdateNodeHandler,
	deleteNodeHandler *commands.DeleteNodeHandler,
	connectNodesHandler *commands.ConnectNodesHandler,
	disconnectNodesHandler *commands.DisconnectNodesHandler,
	bulkDeleteNodesHandler *commands.BulkDeleteNodesHandler,
) error {
	// Register each handler for its command type
	if err := bus.Register("CreateNode", createNodeHandler); err != nil {
		return err
	}
	
	if err := bus.Register("UpdateNode", updateNodeHandler); err != nil {
		return err
	}
	
	if err := bus.Register("DeleteNode", deleteNodeHandler); err != nil {
		return err
	}
	
	if err := bus.Register("ConnectNodes", connectNodesHandler); err != nil {
		return err
	}
	
	if err := bus.Register("DisconnectNodes", disconnectNodesHandler); err != nil {
		return err
	}
	
	if err := bus.Register("BulkDeleteNodes", bulkDeleteNodesHandler); err != nil {
		return err
	}
	
	return nil
}

// registerQueryHandlers registers all query handlers with the query bus
func registerQueryHandlers(
	bus *cqrs.QueryBus,
	getNodeByIDHandler *queries.GetNodeByIDHandler,
	getNodesByUserHandler *queries.GetNodesByUserHandler,
	searchNodesHandler *queries.SearchNodesHandler,
	getGraphHandler *queries.GetGraphHandler,
) error {
	// Register each handler for its query type
	if err := bus.Register("GetNodeByID", getNodeByIDHandler); err != nil {
		return err
	}
	
	if err := bus.Register("GetNodesByUser", getNodesByUserHandler); err != nil {
		return err
	}
	
	if err := bus.Register("SearchNodes", searchNodesHandler); err != nil {
		return err
	}
	
	if err := bus.Register("GetGraph", getGraphHandler); err != nil {
		return err
	}
	
	return nil
}

// provideConfiguredCommandBus creates a fully configured command bus with all handlers registered
func provideConfiguredCommandBus(
	logger corePorts.Logger,
	metrics corePorts.Metrics,
	tracer corePorts.Tracer,
	createNodeHandler *commands.CreateNodeHandler,
	updateNodeHandler *commands.UpdateNodeHandler,
	deleteNodeHandler *commands.DeleteNodeHandler,
	connectNodesHandler *commands.ConnectNodesHandler,
	disconnectNodesHandler *commands.DisconnectNodesHandler,
	bulkDeleteNodesHandler *commands.BulkDeleteNodesHandler,
) (*cqrs.CommandBus, error) {
	// Create the bus with middleware
	bus := cqrs.NewCommandBus(logger, metrics, tracer)
	
	// Add middleware in order of execution
	bus.Use(cqrs.NewLoggingMiddleware(logger))
	bus.Use(cqrs.NewMetricsMiddleware(metrics))
	bus.Use(cqrs.NewValidationMiddleware())
	bus.Use(cqrs.NewRetryMiddleware(3, logger))
	
	// Register handlers
	if err := registerCommandHandlers(
		bus,
		createNodeHandler,
		updateNodeHandler,
		deleteNodeHandler,
		connectNodesHandler,
		disconnectNodesHandler,
		bulkDeleteNodesHandler,
	); err != nil {
		return nil, err
	}
	
	return bus, nil
}

// provideConfiguredQueryBus creates a fully configured query bus with all handlers registered
func provideConfiguredQueryBus(
	cache corePorts.Cache,
	logger corePorts.Logger,
	metrics corePorts.Metrics,
	tracer corePorts.Tracer,
	getNodeByIDHandler *queries.GetNodeByIDHandler,
	getNodesByUserHandler *queries.GetNodesByUserHandler,
	searchNodesHandler *queries.SearchNodesHandler,
	getGraphHandler *queries.GetGraphHandler,
) (*cqrs.QueryBus, error) {
	// Create the bus with middleware
	bus := cqrs.NewQueryBus(cache, logger, metrics, tracer)
	
	// Add middleware
	bus.Use(cqrs.NewQueryLoggingMiddleware(logger))
	bus.Use(cqrs.NewQueryMetricsMiddleware(metrics))
	bus.Use(cqrs.NewQueryCacheMiddleware(nil)) // Cache will be injected later if needed
	
	// Register handlers
	if err := registerQueryHandlers(
		bus,
		getNodeByIDHandler,
		getNodesByUserHandler,
		searchNodesHandler,
		getGraphHandler,
	); err != nil {
		return nil, err
	}
	
	return bus, nil
}