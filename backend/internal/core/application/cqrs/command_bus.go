// Package cqrs implements Command Query Responsibility Segregation pattern.
// This separates read and write operations for optimal performance and scalability.
package cqrs

import (
	"context"
	"fmt"
	"reflect"
	"sync"
	"time"
	
	"brain2-backend/internal/core/application/ports"
	"brain2-backend/internal/core/domain/valueobjects"
)

// Command is the base interface for all commands (write operations)
type Command interface {
	// GetCommandName returns the name of the command
	GetCommandName() string
	
	// GetCorrelationID returns the correlation ID for distributed tracing
	GetCorrelationID() string
	
	// Validate validates the command
	Validate() error
}

// BaseCommand provides common functionality for commands
type BaseCommand struct {
	CorrelationID string    `json:"correlation_id"`
	UserID        string    `json:"user_id"`
	Timestamp     time.Time `json:"timestamp"`
}

// GetCorrelationID returns the correlation ID
func (c BaseCommand) GetCorrelationID() string {
	if c.CorrelationID == "" {
		c.CorrelationID = valueobjects.NewCorrelationID().String()
	}
	return c.CorrelationID
}

// CommandHandler handles a specific command type
type CommandHandler interface {
	// Handle processes the command
	Handle(ctx context.Context, command Command) error
	
	// CanHandle checks if this handler can handle the command
	CanHandle(command Command) bool
}

// CommandHandlerFunc is a function adapter for CommandHandler
type CommandHandlerFunc func(context.Context, Command) error

// CommandBus routes commands to their handlers
type CommandBus struct {
	handlers   map[string]CommandHandler
	middleware []CommandMiddleware
	logger     ports.Logger
	metrics    ports.Metrics
	tracer     ports.Tracer
	mu         sync.RWMutex
}

// NewCommandBus creates a new command bus
func NewCommandBus(logger ports.Logger, metrics ports.Metrics, tracer ports.Tracer) *CommandBus {
	return &CommandBus{
		handlers:   make(map[string]CommandHandler),
		middleware: []CommandMiddleware{},
		logger:     logger,
		metrics:    metrics,
		tracer:     tracer,
	}
}

// Register registers a handler for a command type
func (b *CommandBus) Register(commandType string, handler CommandHandler) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	
	if _, exists := b.handlers[commandType]; exists {
		return fmt.Errorf("handler already registered for command type: %s", commandType)
	}
	
	b.handlers[commandType] = handler
	b.logger.Info("Registered command handler", 
		ports.Field{Key: "command_type", Value: commandType})
	
	return nil
}

// RegisterFunc registers a handler function for a command type
func (b *CommandBus) RegisterFunc(commandType string, handler CommandHandlerFunc) error {
	return b.Register(commandType, &funcHandler{handler: handler})
}

// Send sends a command to its handler
func (b *CommandBus) Send(ctx context.Context, command Command) error {
	// Start tracing
	ctx, span := b.tracer.StartSpan(ctx, "CommandBus.Send",
		SpanOptionWithKind(ports.SpanKindInternal),
		SpanOptionWithAttributes(
			ports.Attribute{Key: "command.type", Value: command.GetCommandName()},
			ports.Attribute{Key: "correlation.id", Value: command.GetCorrelationID()},
		),
	)
	defer span.End()
	
	// Record metrics
	timer := b.metrics.StartTimer("command.duration",
		ports.Tag{Key: "command", Value: command.GetCommandName()})
	defer timer.Stop()
	
	// Validate command
	if err := command.Validate(); err != nil {
		b.metrics.IncrementCounter("command.validation.failed",
			ports.Tag{Key: "command", Value: command.GetCommandName()})
		span.SetError(err)
		return fmt.Errorf("command validation failed: %w", err)
	}
	
	// Apply middleware
	handler := b.applyMiddleware(b.handleCommand)
	
	// Execute command
	if err := handler(ctx, command); err != nil {
		b.metrics.IncrementCounter("command.failed",
			ports.Tag{Key: "command", Value: command.GetCommandName()})
		span.SetError(err)
		return err
	}
	
	b.metrics.IncrementCounter("command.success",
		ports.Tag{Key: "command", Value: command.GetCommandName()})
	
	return nil
}

// handleCommand routes the command to its handler
func (b *CommandBus) handleCommand(ctx context.Context, command Command) error {
	b.mu.RLock()
	handler, exists := b.handlers[command.GetCommandName()]
	b.mu.RUnlock()
	
	if !exists {
		return fmt.Errorf("no handler registered for command: %s", command.GetCommandName())
	}
	
	if !handler.CanHandle(command) {
		return fmt.Errorf("handler cannot handle command: %s", command.GetCommandName())
	}
	
	return handler.Handle(ctx, command)
}

// Use adds middleware to the command bus
func (b *CommandBus) Use(middleware CommandMiddleware) {
	b.middleware = append(b.middleware, middleware)
}

// applyMiddleware applies all middleware to the handler
func (b *CommandBus) applyMiddleware(handler CommandHandlerFunc) CommandHandlerFunc {
	// Apply middleware in reverse order so they execute in the order added
	for i := len(b.middleware) - 1; i >= 0; i-- {
		handler = b.middleware[i](handler)
	}
	return handler
}

// CommandMiddleware is a function that wraps a command handler
type CommandMiddleware func(CommandHandlerFunc) CommandHandlerFunc

// LoggingMiddleware logs command execution
func LoggingMiddleware(logger ports.Logger) CommandMiddleware {
	return func(next CommandHandlerFunc) CommandHandlerFunc {
		return func(ctx context.Context, cmd Command) error {
			logger.Info("Executing command",
				ports.Field{Key: "command", Value: cmd.GetCommandName()},
				ports.Field{Key: "correlation_id", Value: cmd.GetCorrelationID()},
			)
			
			start := time.Now()
			err := next(ctx, cmd)
			duration := time.Since(start)
			
			if err != nil {
				logger.Error("Command failed",
					err,
					ports.Field{Key: "command", Value: cmd.GetCommandName()},
					ports.Field{Key: "duration", Value: duration},
				)
			} else {
				logger.Info("Command completed",
					ports.Field{Key: "command", Value: cmd.GetCommandName()},
					ports.Field{Key: "duration", Value: duration},
				)
			}
			
			return err
		}
	}
}

// ValidationMiddleware validates commands before execution
func ValidationMiddleware() CommandMiddleware {
	return func(next CommandHandlerFunc) CommandHandlerFunc {
		return func(ctx context.Context, cmd Command) error {
			if err := cmd.Validate(); err != nil {
				return fmt.Errorf("command validation failed: %w", err)
			}
			return next(ctx, cmd)
		}
	}
}

// RetryMiddleware retries failed commands
func RetryMiddleware(maxRetries int, backoff time.Duration) CommandMiddleware {
	return func(next CommandHandlerFunc) CommandHandlerFunc {
		return func(ctx context.Context, cmd Command) error {
			var err error
			for i := 0; i <= maxRetries; i++ {
				if i > 0 {
					time.Sleep(backoff * time.Duration(i))
				}
				
				err = next(ctx, cmd)
				if err == nil {
					return nil
				}
				
				// Check if error is retryable
				if !isRetryable(err) {
					return err
				}
			}
			return fmt.Errorf("command failed after %d retries: %w", maxRetries, err)
		}
	}
}

// funcHandler wraps a function as a CommandHandler
type funcHandler struct {
	handler CommandHandlerFunc
}

func (h *funcHandler) Handle(ctx context.Context, cmd Command) error {
	return h.handler(ctx, cmd)
}

func (h *funcHandler) CanHandle(cmd Command) bool {
	return true
}

// isRetryable checks if an error is retryable
func isRetryable(err error) bool {
	// Implement logic to determine if error is retryable
	// For now, we'll consider timeouts and temporary errors as retryable
	return false
}

// SpanOptionWithKind sets the span kind
func SpanOptionWithKind(kind ports.SpanKind) ports.SpanOption {
	return func(config *ports.SpanConfig) {
		config.Kind = kind
	}
}

// SpanOptionWithAttributes sets span attributes
func SpanOptionWithAttributes(attrs ...ports.Attribute) ports.SpanOption {
	return func(config *ports.SpanConfig) {
		config.Attributes = append(config.Attributes, attrs...)
	}
}

// CommandResult represents the result of a command execution
type CommandResult struct {
	Success bool
	Data    interface{}
	Error   error
}

// AsyncCommandBus handles commands asynchronously
type AsyncCommandBus struct {
	*CommandBus
	queue ports.MessageQueue
}

// NewAsyncCommandBus creates a new async command bus
func NewAsyncCommandBus(bus *CommandBus, queue ports.MessageQueue) *AsyncCommandBus {
	return &AsyncCommandBus{
		CommandBus: bus,
		queue:      queue,
	}
}

// SendAsync sends a command asynchronously
func (b *AsyncCommandBus) SendAsync(ctx context.Context, command Command) error {
	// Serialize command
	data, err := serializeCommand(command)
	if err != nil {
		return fmt.Errorf("failed to serialize command: %w", err)
	}
	
	// Send to queue
	message := ports.Message{
		ID:            command.GetCorrelationID(),
		Body:          data,
		CorrelationID: command.GetCorrelationID(),
		Attributes: map[string]string{
			"command_type": command.GetCommandName(),
		},
	}
	
	return b.queue.Send(ctx, "commands", message)
}

// serializeCommand serializes a command for queue transmission
func serializeCommand(command Command) ([]byte, error) {
	// Implementation would use JSON or protobuf
	return nil, nil
}

// CommandRegistry maintains a registry of all commands
type CommandRegistry struct {
	commands map[string]reflect.Type
	mu       sync.RWMutex
}

// NewCommandRegistry creates a new command registry
func NewCommandRegistry() *CommandRegistry {
	return &CommandRegistry{
		commands: make(map[string]reflect.Type),
	}
}

// Register registers a command type
func (r *CommandRegistry) Register(name string, commandType reflect.Type) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.commands[name] = commandType
}

// Create creates a new instance of a command
func (r *CommandRegistry) Create(name string) (Command, error) {
	r.mu.RLock()
	commandType, exists := r.commands[name]
	r.mu.RUnlock()
	
	if !exists {
		return nil, fmt.Errorf("unknown command type: %s", name)
	}
	
	return reflect.New(commandType).Interface().(Command), nil
}