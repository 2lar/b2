package di

import (
	"context"
	"fmt"
	"time"

	"backend2/application/commands"
	"backend2/application/commands/bus"
	commands_handlers "backend2/application/commands/handlers"
	"backend2/application/ports"
	"backend2/application/queries"
	querybus "backend2/application/queries/bus"
	queries_handlers "backend2/application/queries/handlers"
	"backend2/domain/events"
	"backend2/infrastructure/config"
	"backend2/infrastructure/messaging/eventbridge"
	"backend2/infrastructure/persistence/dynamodb"
	"backend2/pkg/auth"
	"backend2/pkg/observability"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	awscloudwatch "github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	awsdynamodb "github.com/aws/aws-sdk-go-v2/service/dynamodb"
	awseventbridge "github.com/aws/aws-sdk-go-v2/service/eventbridge"
	"go.uber.org/zap"
)

// ProvideLogger creates a new logger instance
func ProvideLogger(cfg *config.Config) (*zap.Logger, error) {
	var logger *zap.Logger
	var err error

	if cfg.Environment == "production" {
		logger, err = zap.NewProduction()
	} else {
		logger, err = zap.NewDevelopment()
	}

	if err != nil {
		return nil, err
	}

	return logger, nil
}

// ProvideAWSConfig creates AWS configuration
func ProvideAWSConfig(ctx context.Context, cfg *config.Config) (aws.Config, error) {
	return awsconfig.LoadDefaultConfig(ctx,
		awsconfig.WithRegion(cfg.AWSRegion),
	)
}

// ProvideDynamoDBClient creates a DynamoDB client
func ProvideDynamoDBClient(awsCfg aws.Config) *awsdynamodb.Client {
	return awsdynamodb.NewFromConfig(awsCfg)
}

// ProvideEventBridgeClient creates an EventBridge client
func ProvideEventBridgeClient(awsCfg aws.Config) *awseventbridge.Client {
	return awseventbridge.NewFromConfig(awsCfg)
}

// ProvideNodeRepository creates a node repository
func ProvideNodeRepository(client *awsdynamodb.Client, cfg *config.Config, logger *zap.Logger) ports.NodeRepository {
	return dynamodb.NewNodeRepository(
		client,
		cfg.DynamoDBTable,
		cfg.IndexName,     // GSI1 for user-level queries
		cfg.GSI2IndexName, // GSI2 for direct NodeID lookups
		logger,
	)
}

// ProvideGraphRepository creates a graph repository
func ProvideGraphRepository(
	client *awsdynamodb.Client,
	nodeRepo ports.NodeRepository,
	edgeRepo ports.EdgeRepository,
	cfg *config.Config,
	logger *zap.Logger,
) ports.GraphRepository {
	graphRepo := dynamodb.NewGraphRepository(
		client,
		cfg.DynamoDBTable,
		logger,
	)

	// Set the edge repository for saving edges
	if gr, ok := graphRepo.(*dynamodb.GraphRepository); ok {
		gr.SetEdgeRepository(edgeRepo)
		gr.SetNodeRepository(nodeRepo)
	}

	return graphRepo
}

// ProvideEdgeRepository creates an edge repository
func ProvideEdgeRepository(
	client *awsdynamodb.Client,
	cfg *config.Config,
	logger *zap.Logger,
) ports.EdgeRepository {
	return dynamodb.NewEdgeRepository(
		client,
		cfg.DynamoDBTable,
		logger,
	)
}

// ProvideEventBus creates an event bus
func ProvideEventBus(client *awseventbridge.Client, cfg *config.Config, logger *zap.Logger) ports.EventBus {
	return eventbridge.NewEventBridgePublisher(
		client,
		cfg.EventBusName,
		logger,
	)
}

// ProvideEventPublisher creates an event publisher (adapter for EventBus)
func ProvideEventPublisher(eventBus ports.EventBus) ports.EventPublisher {
	return &eventPublisherAdapter{eventBus: eventBus}
}

// eventPublisherAdapter adapts EventBus to EventPublisher interface
type eventPublisherAdapter struct {
	eventBus ports.EventBus
}

func (a *eventPublisherAdapter) Publish(ctx context.Context, event events.DomainEvent) error {
	// EventBus expects interface{}, so we can pass DomainEvent directly
	return a.eventBus.Publish(ctx, event)
}

func (a *eventPublisherAdapter) PublishBatch(ctx context.Context, events []events.DomainEvent) error {
	// EventBus already expects []events.DomainEvent, so pass through directly
	return a.eventBus.PublishBatch(ctx, events)
}

// CommandHandlerAdapter adapts specific command handlers to the generic interface
type CommandHandlerAdapter struct {
	handler func(context.Context, bus.Command) error
}

func (a *CommandHandlerAdapter) Handle(ctx context.Context, cmd bus.Command) error {
	return a.handler(ctx, cmd)
}

// ProvideUnitOfWork creates a unit of work for transactions
func ProvideUnitOfWork(
	client *awsdynamodb.Client,
	nodeRepo ports.NodeRepository,
	edgeRepo ports.EdgeRepository,
	graphRepo ports.GraphRepository,
	eventStore ports.EventStore,
	eventPublisher ports.EventPublisher,
) ports.UnitOfWork {
	return dynamodb.NewDynamoDBUnitOfWork(
		client,
		nodeRepo,
		edgeRepo,
		graphRepo,
		eventStore,
		eventPublisher,
	)
}

// ProvideEventStore creates an event store
func ProvideEventStore(client *awsdynamodb.Client, cfg *config.Config) ports.EventStore {
	// Use a separate table for events or the same table with different keys
	return dynamodb.NewDynamoDBEventStore(client, cfg.DynamoDBTable)
}

// ProvideCloudWatchClient creates a CloudWatch client
func ProvideCloudWatchClient(awsCfg aws.Config) *awscloudwatch.Client {
	return awscloudwatch.NewFromConfig(awsCfg)
}

// ProvideMetrics creates metrics instance
func ProvideMetrics(client *awscloudwatch.Client, cfg *config.Config) *observability.Metrics {
	namespace := fmt.Sprintf("Brain2/%s", cfg.Environment)
	return observability.NewMetrics(namespace, client)
}

// ProvideDistributedRateLimiter creates a distributed rate limiter
func ProvideDistributedRateLimiter(client *awsdynamodb.Client, cfg *config.Config) *auth.DistributedRateLimiter {
	// Use a separate table for rate limits or the same table with different keys
	tableName := cfg.DynamoDBTable // Could be a separate rate limit table
	return auth.NewDistributedRateLimiter(
		client,
		tableName,
		100,           // 100 requests
		1*time.Minute, // per minute
		"API",         // key prefix for API rate limiting
	)
}

// ProvideDistributedLock creates a distributed lock instance
func ProvideDistributedLock(client *awsdynamodb.Client, cfg *config.Config, logger *zap.Logger) *dynamodb.DistributedLock {
	return dynamodb.NewDistributedLock(client, cfg.DynamoDBTable, logger)
}

// ProvideCommandBus creates a command bus with registered handlers
func ProvideCommandBus(
	uow ports.UnitOfWork,
	nodeRepo ports.NodeRepository,
	edgeRepo ports.EdgeRepository,
	graphRepo ports.GraphRepository,
	eventStore ports.EventStore,
	eventBus ports.EventBus,
	eventPublisher ports.EventPublisher,
	distributedLock *dynamodb.DistributedLock,
	metrics *observability.Metrics,
	logger *zap.Logger,
) *bus.CommandBus {
	// Create command bus with dependencies
	commandBus := bus.NewCommandBusWithDependencies(uow, metrics)

	// Create orchestrator for complex commands
	orchestrator := commands_handlers.NewCreateNodeOrchestrator(
		uow,
		nodeRepo,
		graphRepo,
		edgeRepo,
		eventPublisher,
		distributedLock,
		&zapLoggerAdapter{logger},
	)

	// Register CreateNodeCommand with orchestrator
	commandBus.Register(commands.CreateNodeCommand{}, &CommandHandlerAdapter{
		handler: func(ctx context.Context, cmd bus.Command) error {
			createCmd, ok := cmd.(commands.CreateNodeCommand)
			if !ok {
				return fmt.Errorf("invalid command type")
			}
			_, err := orchestrator.Handle(ctx, createCmd)
			return err
		},
	})

	// Register UpdateNodeCommand handler
	// Note: Using nil for EventStore as it's not implemented yet
	updateNodeHandler := commands_handlers.NewUpdateNodeHandler(nodeRepo, nil, eventBus, logger)
	commandBus.Register(commands.UpdateNodeCommand{}, &CommandHandlerAdapter{
		handler: func(ctx context.Context, cmd bus.Command) error {
			updateCmd, ok := cmd.(commands.UpdateNodeCommand)
			if !ok {
				return fmt.Errorf("invalid command type")
			}
			return updateNodeHandler.Handle(ctx, updateCmd)
		},
	})

	// Register DeleteNodeCommand handler
	deleteNodeHandler := commands_handlers.NewDeleteNodeHandler(nodeRepo, edgeRepo, graphRepo, eventStore, eventBus, logger)
	commandBus.Register(commands.DeleteNodeCommand{}, &CommandHandlerAdapter{
		handler: func(ctx context.Context, cmd bus.Command) error {
			deleteCmd, ok := cmd.(commands.DeleteNodeCommand)
			if !ok {
				return fmt.Errorf("invalid command type")
			}
			return deleteNodeHandler.Handle(ctx, deleteCmd)
		},
	})

	// Register BulkDeleteNodesCommand handler
	bulkDeleteHandler := commands_handlers.NewBulkDeleteNodesHandler(uow, nodeRepo, edgeRepo, graphRepo, eventStore, eventBus, logger)
	commandBus.Register(commands.BulkDeleteNodesCommand{}, &CommandHandlerAdapter{
		handler: func(ctx context.Context, cmd bus.Command) error {
			bulkCmd, ok := cmd.(commands.BulkDeleteNodesCommand)
			if !ok {
				return fmt.Errorf("invalid command type")
			}
			_, err := bulkDeleteHandler.Handle(ctx, bulkCmd)
			return err
		},
	})

	// Register CreateEdgeCommand handler
	createEdgeHandler := commands.NewCreateEdgeHandler(nodeRepo, graphRepo, eventBus)
	commandBus.Register(commands.CreateEdgeCommand{}, &CommandHandlerAdapter{
		handler: func(ctx context.Context, cmd bus.Command) error {
			edgeCmd, ok := cmd.(commands.CreateEdgeCommand)
			if !ok {
				return fmt.Errorf("invalid command type")
			}
			return createEdgeHandler.Handle(ctx, &edgeCmd)
		},
	})

	// Register CleanupNodeResourcesCommand handler
	cleanupHandler := commands.NewCleanupNodeResourcesHandler()
	commandBus.Register(&commands.CleanupNodeResourcesCommand{}, &CommandHandlerAdapter{
		handler: func(ctx context.Context, cmd bus.Command) error {
			cleanupCmd, ok := cmd.(*commands.CleanupNodeResourcesCommand)
			if !ok {
				return fmt.Errorf("invalid command type")
			}
			return cleanupHandler.Handle(ctx, cleanupCmd)
		},
	})

	return commandBus
}

// QueryHandlerAdapter adapts specific query handlers to the generic interface
type QueryHandlerAdapter struct {
	handler func(context.Context, querybus.Query) (interface{}, error)
}

func (a *QueryHandlerAdapter) Handle(ctx context.Context, query querybus.Query) (interface{}, error) {
	return a.handler(ctx, query)
}

// ProvideQueryBus creates a query bus with registered handlers
func ProvideQueryBus(
	graphRepo ports.GraphRepository,
	nodeRepo ports.NodeRepository,
	edgeRepo ports.EdgeRepository,
	cache ports.Cache,
	logger *zap.Logger,
) *querybus.QueryBus {
	queryBus := querybus.NewQueryBus()

	// Register GetGraphQuery handler
	getGraphHandler := queries.NewGetGraphHandler(graphRepo, cache)
	queryBus.Register(queries.GetGraphQuery{}, &QueryHandlerAdapter{
		handler: func(ctx context.Context, query querybus.Query) (interface{}, error) {
			getQuery, ok := query.(queries.GetGraphQuery)
			if !ok {
				return nil, fmt.Errorf("invalid query type")
			}
			return getGraphHandler.Handle(ctx, getQuery)
		},
	})

	// Register GetGraphDataQuery handler
	getGraphDataHandler := queries_handlers.NewGetGraphDataHandler(graphRepo, nodeRepo, edgeRepo, logger)
	queryBus.Register(queries.GetGraphDataQuery{}, &QueryHandlerAdapter{
		handler: func(ctx context.Context, query querybus.Query) (interface{}, error) {
			getQuery, ok := query.(queries.GetGraphDataQuery)
			if !ok {
				return nil, fmt.Errorf("invalid query type")
			}
			return getGraphDataHandler.Handle(ctx, getQuery)
		},
	})

	// Register ListNodesQuery handler
	listNodesHandler := queries_handlers.NewListNodesHandler(nodeRepo, logger)
	queryBus.Register(queries.ListNodesQuery{}, &QueryHandlerAdapter{
		handler: func(ctx context.Context, query querybus.Query) (interface{}, error) {
			listQuery, ok := query.(queries.ListNodesQuery)
			if !ok {
				return nil, fmt.Errorf("invalid query type")
			}
			return listNodesHandler.Handle(ctx, listQuery)
		},
	})

	// Register GetNodeQuery handler
	getNodeHandler := queries_handlers.NewGetNodeHandler(nodeRepo, logger)
	queryBus.Register(queries.GetNodeQuery{}, &QueryHandlerAdapter{
		handler: func(ctx context.Context, query querybus.Query) (interface{}, error) {
			getQuery, ok := query.(queries.GetNodeQuery)
			if !ok {
				return nil, fmt.Errorf("invalid query type")
			}
			return getNodeHandler.Handle(ctx, getQuery)
		},
	})

	// Register ListGraphsQuery handler
	listGraphsHandler := queries_handlers.NewListGraphsHandler(graphRepo, logger)
	queryBus.Register(queries.ListGraphsQuery{}, &QueryHandlerAdapter{
		handler: func(ctx context.Context, query querybus.Query) (interface{}, error) {
			listQuery, ok := query.(queries.ListGraphsQuery)
			if !ok {
				return nil, fmt.Errorf("invalid query type")
			}
			return listGraphsHandler.Handle(ctx, listQuery)
		},
	})

	// Register GetGraphByIDQuery handler
	getGraphByIDHandler := queries_handlers.NewGetGraphHandler(graphRepo, nodeRepo, logger)
	queryBus.Register(queries.GetGraphByIDQuery{}, &QueryHandlerAdapter{
		handler: func(ctx context.Context, query querybus.Query) (interface{}, error) {
			getQuery, ok := query.(queries.GetGraphByIDQuery)
			if !ok {
				return nil, fmt.Errorf("invalid query type")
			}
			return getGraphByIDHandler.Handle(ctx, getQuery)
		},
	})

	// Register FindSimilarNodesQuery handler
	findSimilarHandler := queries.NewFindSimilarNodesHandler(nodeRepo)
	queryBus.Register(&queries.FindSimilarNodesQuery{}, &QueryHandlerAdapter{
		handler: func(ctx context.Context, query querybus.Query) (interface{}, error) {
			findQuery, ok := query.(*queries.FindSimilarNodesQuery)
			if !ok {
				return nil, fmt.Errorf("invalid query type")
			}
			return findSimilarHandler.Handle(ctx, findQuery)
		},
	})

	return queryBus
}

// ProvideInMemoryCache creates a simple in-memory cache
// In production, this would be Redis or similar
func ProvideInMemoryCache() ports.Cache {
	return NewInMemoryCache()
}

// zapLoggerAdapter adapts zap.Logger to the handlers.Logger interface
type zapLoggerAdapter struct {
	logger *zap.Logger
}

func (a *zapLoggerAdapter) Debug(msg string, fields ...interface{}) {
	a.logger.Debug(msg, a.fieldsToZap(fields...)...)
}

func (a *zapLoggerAdapter) Info(msg string, fields ...interface{}) {
	a.logger.Info(msg, a.fieldsToZap(fields...)...)
}

func (a *zapLoggerAdapter) Error(msg string, fields ...interface{}) {
	a.logger.Error(msg, a.fieldsToZap(fields...)...)
}

func (a *zapLoggerAdapter) fieldsToZap(fields ...interface{}) []zap.Field {
	var zapFields []zap.Field
	for i := 0; i < len(fields); i += 2 {
		if i+1 < len(fields) {
			key, _ := fields[i].(string)
			zapFields = append(zapFields, zap.Any(key, fields[i+1]))
		}
	}
	return zapFields
}
