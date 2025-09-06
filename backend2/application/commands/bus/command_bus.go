package bus

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"sync"
)

// Command represents a command that changes state
type Command interface {
	Validate() error
}

// CommandHandler handles a specific command type
type CommandHandler interface {
	Handle(ctx context.Context, cmd Command) error
}

// CommandBus dispatches commands to their handlers
type CommandBus struct {
	handlers map[reflect.Type]CommandHandler
	mu       sync.RWMutex
}

// NewCommandBus creates a new command bus
func NewCommandBus() *CommandBus {
	return &CommandBus{
		handlers: make(map[reflect.Type]CommandHandler),
	}
}

// Register registers a handler for a command type
func (b *CommandBus) Register(cmdType Command, handler CommandHandler) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	
	t := reflect.TypeOf(cmdType)
	if _, exists := b.handlers[t]; exists {
		return fmt.Errorf("handler already registered for command type %s", t.Name())
	}
	
	b.handlers[t] = handler
	return nil
}

// Send dispatches a command to its handler
func (b *CommandBus) Send(ctx context.Context, cmd Command) error {
	// Validate command
	if err := cmd.Validate(); err != nil {
		return fmt.Errorf("command validation failed: %w", err)
	}
	
	b.mu.RLock()
	handler, exists := b.handlers[reflect.TypeOf(cmd)]
	b.mu.RUnlock()
	
	if !exists {
		return fmt.Errorf("no handler registered for command type %T", cmd)
	}
	
	// Execute handler
	if err := handler.Handle(ctx, cmd); err != nil {
		return fmt.Errorf("command handler failed: %w", err)
	}
	
	return nil
}

// Middleware defines command middleware
type Middleware func(next CommandHandler) CommandHandler

// CommandHandlerFunc is an adapter to allow functions to be used as handlers
type CommandHandlerFunc func(ctx context.Context, cmd Command) error

// Handle implements CommandHandler
func (f CommandHandlerFunc) Handle(ctx context.Context, cmd Command) error {
	return f(ctx, cmd)
}

// LoggingMiddleware logs command execution
func LoggingMiddleware(logger Logger) Middleware {
	return func(next CommandHandler) CommandHandler {
		return CommandHandlerFunc(func(ctx context.Context, cmd Command) error {
			cmdType := reflect.TypeOf(cmd).Name()
			logger.Info("Executing command", "type", cmdType)
			
			err := next.Handle(ctx, cmd)
			if err != nil {
				logger.Error("Command failed", "type", cmdType, "error", err)
			} else {
				logger.Info("Command succeeded", "type", cmdType)
			}
			
			return err
		})
	}
}

// ValidationMiddleware ensures commands are valid
func ValidationMiddleware() Middleware {
	return func(next CommandHandler) CommandHandler {
		return CommandHandlerFunc(func(ctx context.Context, cmd Command) error {
			if err := cmd.Validate(); err != nil {
				return fmt.Errorf("validation failed: %w", err)
			}
			return next.Handle(ctx, cmd)
		})
	}
}

// TransactionMiddleware wraps command execution in a transaction
func TransactionMiddleware(txManager TransactionManager) Middleware {
	return func(next CommandHandler) CommandHandler {
		return CommandHandlerFunc(func(ctx context.Context, cmd Command) error {
			tx, err := txManager.Begin(ctx)
			if err != nil {
				return fmt.Errorf("failed to begin transaction: %w", err)
			}
			
			// Store transaction in context
			ctx = context.WithValue(ctx, "tx", tx)
			
			err = next.Handle(ctx, cmd)
			if err != nil {
				if rbErr := tx.Rollback(); rbErr != nil {
					return fmt.Errorf("rollback failed: %v, original error: %w", rbErr, err)
				}
				return err
			}
			
			if err := tx.Commit(); err != nil {
				return fmt.Errorf("commit failed: %w", err)
			}
			
			return nil
		})
	}
}

// Logger interface for logging
type Logger interface {
	Info(msg string, keysAndValues ...interface{})
	Error(msg string, keysAndValues ...interface{})
}

// TransactionManager interface for transaction management
type TransactionManager interface {
	Begin(ctx context.Context) (Transaction, error)
}

// Transaction interface
type Transaction interface {
	Commit() error
	Rollback() error
}

// CommandResult represents the result of a command execution
type CommandResult struct {
	Success bool
	Data    interface{}
	Error   error
}

// Pipeline chains multiple middleware together
type Pipeline struct {
	middlewares []Middleware
}

// NewPipeline creates a new middleware pipeline
func NewPipeline(middlewares ...Middleware) *Pipeline {
	return &Pipeline{
		middlewares: middlewares,
	}
}

// Execute runs the command through the pipeline
func (p *Pipeline) Execute(handler CommandHandler) CommandHandler {
	// Apply middleware in reverse order
	for i := len(p.middlewares) - 1; i >= 0; i-- {
		handler = p.middlewares[i](handler)
	}
	return handler
}

// Errors
var (
	ErrHandlerNotFound = errors.New("command handler not found")
	ErrValidationFailed = errors.New("command validation failed")
	ErrExecutionFailed = errors.New("command execution failed")
)