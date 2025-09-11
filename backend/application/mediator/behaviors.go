package mediator

import (
	"context"
	"fmt"
	"time"

	commandbus "backend/application/commands/bus"
	querybus "backend/application/queries/bus"
	"backend/pkg/observability"
	"go.uber.org/zap"
)

// Behavior defines the interface for mediator pipeline behaviors
// Behaviors are cross-cutting concerns that apply to all requests
type Behavior interface {
	// PreProcess is called before command execution
	PreProcess(ctx context.Context, command commandbus.Command) error
	
	// PostProcess is called after command execution
	PostProcess(ctx context.Context, command commandbus.Command, err error)
	
	// PreProcessQuery is called before query execution
	PreProcessQuery(ctx context.Context, query querybus.Query) error
	
	// PostProcessQuery is called after query execution
	PostProcessQuery(ctx context.Context, query querybus.Query, result interface{}, err error)
}

// LoggingBehavior logs all commands and queries
type LoggingBehavior struct {
	logger *zap.Logger
}

// NewLoggingBehavior creates a new logging behavior
func NewLoggingBehavior(logger *zap.Logger) *LoggingBehavior {
	return &LoggingBehavior{logger: logger}
}

func (b *LoggingBehavior) PreProcess(ctx context.Context, command commandbus.Command) error {
	b.logger.Info("Executing command",
		zap.String("type", fmt.Sprintf("%T", command)),
		zap.Any("command", command))
	return nil
}

func (b *LoggingBehavior) PostProcess(ctx context.Context, command commandbus.Command, err error) {
	if err != nil {
		b.logger.Error("Command failed",
			zap.String("type", fmt.Sprintf("%T", command)),
			zap.Error(err))
	} else {
		b.logger.Info("Command succeeded",
			zap.String("type", fmt.Sprintf("%T", command)))
	}
}

func (b *LoggingBehavior) PreProcessQuery(ctx context.Context, query querybus.Query) error {
	b.logger.Debug("Executing query",
		zap.String("type", fmt.Sprintf("%T", query)),
		zap.Any("query", query))
	return nil
}

func (b *LoggingBehavior) PostProcessQuery(ctx context.Context, query querybus.Query, result interface{}, err error) {
	if err != nil {
		b.logger.Error("Query failed",
			zap.String("type", fmt.Sprintf("%T", query)),
			zap.Error(err))
	} else {
		b.logger.Debug("Query succeeded",
			zap.String("type", fmt.Sprintf("%T", query)))
	}
}

// ValidationBehavior validates commands and queries before execution
type ValidationBehavior struct {
	logger *zap.Logger
}

// NewValidationBehavior creates a new validation behavior
func NewValidationBehavior(logger *zap.Logger) *ValidationBehavior {
	return &ValidationBehavior{logger: logger}
}

func (b *ValidationBehavior) PreProcess(ctx context.Context, command commandbus.Command) error {
	// Commands already have Validate() method
	if err := command.Validate(); err != nil {
		b.logger.Warn("Command validation failed",
			zap.String("type", fmt.Sprintf("%T", command)),
			zap.Error(err))
		return fmt.Errorf("command validation failed: %w", err)
	}
	return nil
}

func (b *ValidationBehavior) PostProcess(ctx context.Context, command commandbus.Command, err error) {
	// No post-processing needed for validation
}

func (b *ValidationBehavior) PreProcessQuery(ctx context.Context, query querybus.Query) error {
	// Queries already have Validate() method
	if err := query.Validate(); err != nil {
		b.logger.Warn("Query validation failed",
			zap.String("type", fmt.Sprintf("%T", query)),
			zap.Error(err))
		return fmt.Errorf("query validation failed: %w", err)
	}
	return nil
}

func (b *ValidationBehavior) PostProcessQuery(ctx context.Context, query querybus.Query, result interface{}, err error) {
	// No post-processing needed for validation
}

// MetricsBehavior records metrics for commands and queries
type MetricsBehavior struct {
	metrics   *observability.Metrics
	logger    *zap.Logger
	startTime map[string]time.Time
}

// NewMetricsBehavior creates a new metrics behavior
func NewMetricsBehavior(metrics *observability.Metrics, logger *zap.Logger) *MetricsBehavior {
	return &MetricsBehavior{
		metrics:   metrics,
		logger:    logger,
		startTime: make(map[string]time.Time),
	}
}

func (b *MetricsBehavior) PreProcess(ctx context.Context, command commandbus.Command) error {
	requestID := fmt.Sprintf("%p", command) // Use pointer address as unique ID
	b.startTime[requestID] = time.Now()
	return nil
}

func (b *MetricsBehavior) PostProcess(ctx context.Context, command commandbus.Command, err error) {
	requestID := fmt.Sprintf("%p", command)
	if startTime, exists := b.startTime[requestID]; exists {
		duration := time.Since(startTime)
		delete(b.startTime, requestID)
		
		if b.metrics != nil {
			b.metrics.RecordCommandExecution(ctx, fmt.Sprintf("%T", command), duration, err)
		}
	}
}

func (b *MetricsBehavior) PreProcessQuery(ctx context.Context, query querybus.Query) error {
	requestID := fmt.Sprintf("%p", query)
	b.startTime[requestID] = time.Now()
	return nil
}

func (b *MetricsBehavior) PostProcessQuery(ctx context.Context, query querybus.Query, result interface{}, err error) {
	requestID := fmt.Sprintf("%p", query)
	if startTime, exists := b.startTime[requestID]; exists {
		duration := time.Since(startTime)
		delete(b.startTime, requestID)
		
		if b.metrics != nil {
			b.metrics.RecordLatency(ctx, fmt.Sprintf("query.%T", query), duration)
			if err != nil {
				b.metrics.RecordError(ctx, "query_error", fmt.Sprintf("%T", query))
			}
		}
	}
}

// PerformanceBehavior logs slow commands and queries
type PerformanceBehavior struct {
	logger           *zap.Logger
	commandThreshold time.Duration
	queryThreshold   time.Duration
	startTime        map[string]time.Time
}

// NewPerformanceBehavior creates a new performance monitoring behavior
func NewPerformanceBehavior(logger *zap.Logger, commandThreshold, queryThreshold time.Duration) *PerformanceBehavior {
	return &PerformanceBehavior{
		logger:           logger,
		commandThreshold: commandThreshold,
		queryThreshold:   queryThreshold,
		startTime:        make(map[string]time.Time),
	}
}

func (b *PerformanceBehavior) PreProcess(ctx context.Context, command commandbus.Command) error {
	requestID := fmt.Sprintf("%p", command)
	b.startTime[requestID] = time.Now()
	return nil
}

func (b *PerformanceBehavior) PostProcess(ctx context.Context, command commandbus.Command, err error) {
	requestID := fmt.Sprintf("%p", command)
	if startTime, exists := b.startTime[requestID]; exists {
		duration := time.Since(startTime)
		delete(b.startTime, requestID)
		
		if duration > b.commandThreshold {
			b.logger.Warn("Slow command detected",
				zap.String("type", fmt.Sprintf("%T", command)),
				zap.Duration("duration", duration),
				zap.Duration("threshold", b.commandThreshold))
		}
	}
}

func (b *PerformanceBehavior) PreProcessQuery(ctx context.Context, query querybus.Query) error {
	requestID := fmt.Sprintf("%p", query)
	b.startTime[requestID] = time.Now()
	return nil
}

func (b *PerformanceBehavior) PostProcessQuery(ctx context.Context, query querybus.Query, result interface{}, err error) {
	requestID := fmt.Sprintf("%p", query)
	if startTime, exists := b.startTime[requestID]; exists {
		duration := time.Since(startTime)
		delete(b.startTime, requestID)
		
		if duration > b.queryThreshold {
			b.logger.Warn("Slow query detected",
				zap.String("type", fmt.Sprintf("%T", query)),
				zap.Duration("duration", duration),
				zap.Duration("threshold", b.queryThreshold))
		}
	}
}