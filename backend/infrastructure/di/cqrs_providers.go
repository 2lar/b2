package di

import (
	"time"

	appevents "backend/application/events"
	"backend/application/events/listeners"
	"backend/application/mediator"
	"backend/application/ports"
	"backend/application/projections"
	commandbus "backend/application/commands/bus"
	querybus "backend/application/queries/bus"
	"backend/pkg/observability"
	"go.uber.org/zap"
)

// ProvideEventHandlerRegistry creates the event handler registry
func ProvideEventHandlerRegistry(logger *zap.Logger) *appevents.HandlerRegistry {
	return appevents.NewHandlerRegistry(logger)
}

// ProvideOperationEventListener creates the operation event listener
func ProvideOperationEventListener(
	operationStore ports.OperationStore,
	logger *zap.Logger,
) *listeners.OperationEventListener {
	return listeners.NewOperationEventListener(operationStore, logger)
}

// ProvideGraphStatsProjection creates the graph statistics projection
func ProvideGraphStatsProjection(
	cache ports.Cache,
	logger *zap.Logger,
) *projections.GraphStatsProjection {
	return projections.NewGraphStatsProjection(cache, logger)
}

// ProvideMediator creates the mediator with all behaviors
func ProvideMediator(
	commandBus *commandbus.CommandBus,
	queryBus *querybus.QueryBus,
	metrics *observability.Metrics,
	logger *zap.Logger,
) *mediator.Mediator {
	// Create mediator
	med := mediator.NewMediator(commandBus, queryBus, logger)
	
	// Add behaviors in order of execution
	
	// 1. Validation - fail fast if invalid
	med.AddBehavior(mediator.NewValidationBehavior(logger))
	
	// 2. Logging - log all requests
	med.AddBehavior(mediator.NewLoggingBehavior(logger))
	
	// 3. Metrics - record metrics
	if metrics != nil {
		med.AddBehavior(mediator.NewMetricsBehavior(metrics, logger))
	}
	
	// 4. Performance monitoring - detect slow operations
	med.AddBehavior(mediator.NewPerformanceBehavior(
		logger,
		500*time.Millisecond, // Command threshold
		200*time.Millisecond, // Query threshold
	))
	
	return med
}

// WireEventHandlers wires all event handlers and projections
// This should be called during application startup
func WireEventHandlers(
	registry *appevents.HandlerRegistry,
	operationListener *listeners.OperationEventListener,
	graphStatsProjection *projections.GraphStatsProjection,
	logger *zap.Logger,
) error {
	// Subscribe operation event listener
	if err := operationListener.Subscribe(registry); err != nil {
		logger.Error("Failed to subscribe operation event listener", zap.Error(err))
		return err
	}
	
	// Register graph statistics projection
	// Use the actual event type strings returned by GetEventType()
	eventTypes := []string{
		"node.created.with.pending.edges", // From TypeNodeCreatedWithPending
		"NodeDeleted",                      // From NodeDeletedEvent
		"BulkNodesDeleted",                 // From BulkNodesDeletedEvent
	}
	
	if err := registry.Register(eventTypes, graphStatsProjection); err != nil {
		logger.Error("Failed to register graph stats projection", zap.Error(err))
		return err
	}
	
	logger.Info("Event handlers and projections wired successfully")
	return nil
}