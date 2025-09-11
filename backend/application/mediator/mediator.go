package mediator

import (
	"context"
	"fmt"
	"time"

	commandbus "backend/application/commands/bus"
	querybus "backend/application/queries/bus"
	"go.uber.org/zap"
)

// IMediator defines the interface for the mediator pattern
// This provides a single entry point for all commands and queries,
// decoupling the presentation layer from the application layer
type IMediator interface {
	// Send dispatches a command and returns an error
	Send(ctx context.Context, command commandbus.Command) error
	
	// SendWithResult dispatches a command that returns a result (for special cases)
	SendWithResult(ctx context.Context, command commandbus.Command) (interface{}, error)
	
	// Query dispatches a query and returns the result
	Query(ctx context.Context, query querybus.Query) (interface{}, error)
}

// Mediator implements the mediator pattern for CQRS
type Mediator struct {
	commandBus *commandbus.CommandBus
	queryBus   *querybus.QueryBus
	logger     *zap.Logger
	behaviors  []Behavior
}

// NewMediator creates a new mediator instance
func NewMediator(
	commandBus *commandbus.CommandBus,
	queryBus *querybus.QueryBus,
	logger *zap.Logger,
) *Mediator {
	return &Mediator{
		commandBus: commandBus,
		queryBus:   queryBus,
		logger:     logger,
		behaviors:  []Behavior{},
	}
}

// Send dispatches a command through the pipeline
func (m *Mediator) Send(ctx context.Context, command commandbus.Command) error {
	startTime := time.Now()
	
	// Apply pre-processing behaviors
	for _, behavior := range m.behaviors {
		if err := behavior.PreProcess(ctx, command); err != nil {
			m.logger.Error("Pre-processing behavior failed",
				zap.String("command", fmt.Sprintf("%T", command)),
				zap.Error(err),
				zap.Duration("duration", time.Since(startTime)))
			return err
		}
	}
	
	// Send command through command bus
	err := m.commandBus.Send(ctx, command)
	
	// Apply post-processing behaviors
	for _, behavior := range m.behaviors {
		behavior.PostProcess(ctx, command, err)
	}
	
	if err != nil {
		m.logger.Error("Command execution failed",
			zap.String("command", fmt.Sprintf("%T", command)),
			zap.Error(err),
			zap.Duration("duration", time.Since(startTime)))
		return err
	}
	
	m.logger.Debug("Command executed successfully",
		zap.String("command", fmt.Sprintf("%T", command)),
		zap.Duration("duration", time.Since(startTime)))
	
	return nil
}

// SendWithResult dispatches a command that returns a result
// This is for special cases where a command needs to return data (e.g., created ID)
func (m *Mediator) SendWithResult(ctx context.Context, command commandbus.Command) (interface{}, error) {
	startTime := time.Now()
	
	// Apply pre-processing behaviors
	for _, behavior := range m.behaviors {
		if err := behavior.PreProcess(ctx, command); err != nil {
			m.logger.Error("Pre-processing behavior failed",
				zap.String("command", fmt.Sprintf("%T", command)),
				zap.Error(err),
				zap.Duration("duration", time.Since(startTime)))
			return nil, err
		}
	}
	
	// For now, we'll use the standard Send since commands return only errors
	// In the future, we could extend this if needed
	err := m.commandBus.Send(ctx, command)
	
	// Apply post-processing behaviors
	for _, behavior := range m.behaviors {
		behavior.PostProcess(ctx, command, err)
	}
	
	if err != nil {
		m.logger.Error("Command execution failed",
			zap.String("command", fmt.Sprintf("%T", command)),
			zap.Error(err),
			zap.Duration("duration", time.Since(startTime)))
		return nil, err
	}
	
	// For create commands, the ID should be extracted from the command
	// This requires type assertion to the specific command type
	return nil, nil
}

// Query dispatches a query through the pipeline
func (m *Mediator) Query(ctx context.Context, query querybus.Query) (interface{}, error) {
	startTime := time.Now()
	
	// Apply pre-processing behaviors for queries
	for _, behavior := range m.behaviors {
		if err := behavior.PreProcessQuery(ctx, query); err != nil {
			m.logger.Error("Query pre-processing behavior failed",
				zap.String("query", fmt.Sprintf("%T", query)),
				zap.Error(err),
				zap.Duration("duration", time.Since(startTime)))
			return nil, err
		}
	}
	
	// Execute query through query bus
	result, err := m.queryBus.Ask(ctx, query)
	
	// Apply post-processing behaviors for queries
	for _, behavior := range m.behaviors {
		behavior.PostProcessQuery(ctx, query, result, err)
	}
	
	if err != nil {
		m.logger.Error("Query execution failed",
			zap.String("query", fmt.Sprintf("%T", query)),
			zap.Error(err),
			zap.Duration("duration", time.Since(startTime)))
		return nil, err
	}
	
	m.logger.Debug("Query executed successfully",
		zap.String("query", fmt.Sprintf("%T", query)),
		zap.Duration("duration", time.Since(startTime)))
	
	return result, nil
}

// AddBehavior adds a behavior to the mediator pipeline
func (m *Mediator) AddBehavior(behavior Behavior) {
	m.behaviors = append(m.behaviors, behavior)
	m.logger.Info("Added behavior to mediator pipeline",
		zap.String("behavior", fmt.Sprintf("%T", behavior)))
}

// RemoveBehavior removes a behavior from the pipeline
func (m *Mediator) RemoveBehavior(behavior Behavior) {
	filtered := []Behavior{}
	for _, b := range m.behaviors {
		if b != behavior {
			filtered = append(filtered, b)
		}
	}
	m.behaviors = filtered
}

// GetBehaviors returns all registered behaviors
func (m *Mediator) GetBehaviors() []Behavior {
	return m.behaviors
}