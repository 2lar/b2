package repository

import (
	"context"
	"fmt"
	"strings"
	"time"

	"brain2-backend/internal/domain/shared"
	"go.uber.org/zap"
)

// UnitOfWork ensures transactional consistency across multiple repository operations.
//
// Key Concepts Illustrated:
//   1. Transaction Management: Coordinates multiple operations as a single unit
//   2. Consistency: Ensures all operations succeed or none do
//   3. Event Publishing: Domain events are published atomically with data changes
//   4. Resource Management: Proper cleanup of database connections and transactions
//   5. Repository Access: Provides transactional versions of all repositories
//
// This implementation demonstrates the Unit of Work pattern as described in
// Martin Fowler's "Patterns of Enterprise Application Architecture".
//
// Example Usage:
//   uow := NewUnitOfWork(db)
//   if err := uow.Begin(ctx); err != nil { return err }
//   defer uow.Rollback() // Safe to call multiple times
//   
//   // Use transactional repositories
//   node := node.NewNode(userID, content, tags)
//   if err := uow.Nodes().Save(ctx, node); err != nil { return err }
//   
//   edges := analyzer.FindPotentialConnections(node, existingNodes)
//   for _, edge := range edges {
//       if err := uow.Edges().Save(ctx, edge); err != nil { return err }
//   }
//   
//   // Publish domain events atomically
//   for _, event := range node.GetUncommittedEvents() {
//       uow.PublishEvent(event)
//   }
//   
//   return uow.Commit() // All or nothing
type UnitOfWork interface {
	// Transaction Management
	Begin(ctx context.Context) error
	Commit() error
	Rollback() error
	
	// Repository Access - Returns transactional repositories
	Nodes() NodeRepository
	Edges() EdgeRepository
	Categories() CategoryRepository
	Keywords() KeywordRepository
	Graph() GraphRepository
	NodeCategories() NodeCategoryRepository
	
	// Event Publishing - Events are published atomically with transaction
	PublishEvent(event shared.DomainEvent)
	GetPendingEvents() []shared.DomainEvent
	
	// State Queries
	IsActive() bool
	IsCommitted() bool
	IsRolledBack() bool
}

// EventPublisher handles domain event publishing
type EventPublisher interface {
	Publish(ctx context.Context, events []shared.DomainEvent) error
}

// TransactionProvider provides database transaction capabilities
type TransactionProvider interface {
	BeginTransaction(ctx context.Context) (Transaction, error)
}

// Transaction represents a database transaction
type Transaction interface {
	Commit() error
	Rollback() error
	IsActive() bool
}

// unitOfWork implements the UnitOfWork interface
// This demonstrates proper separation of concerns and dependency injection
// Now integrated with existing TransactionManager for low-level transaction operations
type unitOfWork struct {
	// External dependencies injected via constructor
	transactionProvider TransactionProvider
	eventPublisher      EventPublisher
	repositoryFactory   TransactionalRepositoryFactory
	
	// Transaction management - using existing TransactionManager as foundation
	transactionManager *TransactionManager
	transaction        Transaction
	committed          bool
	rolledBack         bool
	
	// Repository instances (created when transaction begins)
	nodeRepo         NodeRepository
	edgeRepo         EdgeRepository
	categoryRepo     CategoryRepository
	keywordRepo      KeywordRepository
	graphRepo        GraphRepository
	nodeCategoryRepo NodeCategoryRepository
	
	// Domain events to be published atomically
	pendingEvents []shared.DomainEvent
	
	// Logger for error reporting
	logger *zap.Logger
}

// NewUnitOfWork creates a new Unit of Work instance.
// This factory function demonstrates proper dependency injection.
// Now integrates with existing TransactionManager for enhanced transaction capabilities.
func NewUnitOfWork(
	transactionProvider TransactionProvider,
	eventPublisher EventPublisher,
	repositoryFactory TransactionalRepositoryFactory,
	logger *zap.Logger,
) UnitOfWork {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &unitOfWork{
		transactionProvider: transactionProvider,
		eventPublisher:      eventPublisher,
		repositoryFactory:   repositoryFactory,
		transactionManager:  NewTransactionManager(), // Initialize with existing transaction manager
		pendingEvents:       make([]shared.DomainEvent, 0),
		logger:              logger,
	}
}

// Begin starts a new unit of work by beginning a database transaction.
// This method demonstrates proper resource initialization and error handling.
// Now uses TransactionManager for enhanced transaction step management.
func (uow *unitOfWork) Begin(ctx context.Context) error {
	// Handle reuse in warm Lambda containers: if transaction exists but was completed, reset state
	if uow.transaction != nil {
		if uow.committed || uow.rolledBack {
			// Previous transaction was completed, reset state for new transaction
			uow.transaction = nil
			uow.committed = false
			uow.rolledBack = false
			uow.pendingEvents = nil
			if uow.transactionManager != nil {
				uow.transactionManager = NewTransactionManager() // Reset transaction manager
			}
		} else {
			// Transaction is still active
			return fmt.Errorf("invalid operation: unit of work already begun")
		}
	}
	
	// Start database transaction
	tx, err := uow.transactionProvider.BeginTransaction(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	
	uow.transaction = tx
	
	// Set up core transaction step for database operations
	// This demonstrates how UnitOfWork uses TransactionManager as a building block
	uow.transactionManager.AddStep(
		"database_transaction",
		func(ctx context.Context) error {
			// Database operations will be added here during repository calls
			return nil
		},
		func(ctx context.Context) error {
			// Rollback database transaction if needed
			if uow.transaction != nil && !uow.committed && !uow.rolledBack {
				return uow.transaction.Rollback()
			}
			return nil
		},
	)
	
	// Create transactional repository instances
	// This demonstrates the Factory pattern for creating related objects
	uow.nodeRepo = uow.repositoryFactory.CreateNodeRepository(tx)
	uow.edgeRepo = uow.repositoryFactory.CreateEdgeRepository(tx)
	uow.categoryRepo = uow.repositoryFactory.CreateCategoryRepository(tx)
	uow.keywordRepo = uow.repositoryFactory.CreateKeywordRepository(tx)
	uow.graphRepo = uow.repositoryFactory.CreateGraphRepository(tx)
	uow.nodeCategoryRepo = uow.repositoryFactory.CreateNodeCategoryRepository(tx)
	
	return nil
}

// Commit persists all changes and publishes domain events atomically.
// This method demonstrates the two-phase commit pattern for consistency.
// Now uses TransactionManager for coordinated commit operations.
func (uow *unitOfWork) Commit() error {
	if uow.transaction == nil {
		return fmt.Errorf("invalid operation: no active transaction")
	}
	
	if uow.committed || uow.rolledBack {
		return fmt.Errorf("invalid operation: transaction already completed")
	}
	
	// Use TransactionManager for coordinated two-phase commit
	// Add commit steps that need to be executed atomically
	uow.transactionManager.AddStep(
		"commit_database",
		func(ctx context.Context) error {
			return uow.transaction.Commit()
		},
		func(ctx context.Context) error {
			// Rollback already handled by database transaction
			return nil
		},
	)
	
	// Add event publishing step (only if there are events)
	if len(uow.pendingEvents) > 0 {
		uow.transactionManager.AddStep(
			"publish_events",
			func(ctx context.Context) error {
				return uow.eventPublisher.Publish(ctx, uow.pendingEvents)
			},
			func(ctx context.Context) error {
				// Event publishing rollback - in production this might involve
				// compensating actions or storing failed events for retry
				return nil
			},
		)
	}
	
	// Execute all commit steps using TransactionManager
	if err := uow.transactionManager.Execute(context.Background()); err != nil {
		uow.rollbackTransaction()
		return fmt.Errorf("unit of work commit failed: %w", err)
	}
	
	uow.committed = true
	uow.transaction = nil // Clear transaction reference to allow new Begin() calls
	
	// Clear pending events after successful publishing
	uow.pendingEvents = nil
	
	return nil
}

// Rollback discards all changes made in this unit of work.
// This method is safe to call multiple times and implements proper cleanup.
func (uow *unitOfWork) Rollback() error {
	if uow.transaction == nil {
		return nil // No active transaction, nothing to rollback
	}
	
	if uow.committed {
		return nil // Transaction already committed successfully, silently succeed
	}
	
	if uow.rolledBack {
		return nil // Already rolled back, safe to call multiple times
	}
	
	return uow.rollbackTransaction()
}

// rollbackTransaction performs the actual rollback operation
func (uow *unitOfWork) rollbackTransaction() error {
	if err := uow.transaction.Rollback(); err != nil {
		uow.rolledBack = true // Mark as rolled back even on error to prevent retry
		uow.transaction = nil // Clear transaction reference to allow new Begin() calls
		return fmt.Errorf("failed to rollback transaction: %w", err)
	}
	
	uow.rolledBack = true
	uow.transaction = nil // Clear transaction reference to allow new Begin() calls
	
	// Clear pending events on rollback
	uow.pendingEvents = nil
	
	return nil
}

// Repository Access Methods
// These methods provide access to transactional repositories

func (uow *unitOfWork) Nodes() NodeRepository {
	if uow.nodeRepo == nil {
		uow.logger.Error("unit of work not begun - call Begin() first")
		return nil
	}
	return uow.nodeRepo
}

func (uow *unitOfWork) Edges() EdgeRepository {
	if uow.edgeRepo == nil {
		uow.logger.Error("unit of work not begun - call Begin() first")
		return nil
	}
	return uow.edgeRepo
}

func (uow *unitOfWork) Categories() CategoryRepository {
	if uow.categoryRepo == nil {
		uow.logger.Error("unit of work not begun - call Begin() first")
		return nil
	}
	return uow.categoryRepo
}

func (uow *unitOfWork) Keywords() KeywordRepository {
	if uow.keywordRepo == nil {
		uow.logger.Error("unit of work not begun - call Begin() first")
		return nil
	}
	return uow.keywordRepo
}

func (uow *unitOfWork) Graph() GraphRepository {
	if uow.graphRepo == nil {
		uow.logger.Error("unit of work not begun - call Begin() first")
		return nil
	}
	return uow.graphRepo
}

func (uow *unitOfWork) NodeCategories() NodeCategoryRepository {
	if uow.nodeCategoryRepo == nil {
		panic("unit of work not begun - call Begin() first")
	}
	return uow.nodeCategoryRepo
}

// Event Management Methods
// These methods handle domain event publishing atomically with data changes

// PublishEvent adds a domain event to be published when the transaction commits.
// Events are only published if the transaction commits successfully.
func (uow *unitOfWork) PublishEvent(event shared.DomainEvent) {
	uow.pendingEvents = append(uow.pendingEvents, event)
}

// GetPendingEvents returns all events scheduled for publishing.
// This is useful for testing and debugging.
func (uow *unitOfWork) GetPendingEvents() []shared.DomainEvent {
	// Return a copy to prevent external modification
	events := make([]shared.DomainEvent, len(uow.pendingEvents))
	copy(events, uow.pendingEvents)
	return events
}

// State Query Methods
// These methods provide insight into the current state of the unit of work

func (uow *unitOfWork) IsActive() bool {
	return uow.transaction != nil && uow.transaction.IsActive() && !uow.committed && !uow.rolledBack
}

func (uow *unitOfWork) IsCommitted() bool {
	return uow.committed
}

func (uow *unitOfWork) IsRolledBack() bool {
	return uow.rolledBack
}

// TransactionalRepositoryFactory creates transactional repository instances.
// This interface demonstrates the Abstract Factory pattern for transactional contexts.
type TransactionalRepositoryFactory interface {
	CreateNodeRepository(tx Transaction) NodeRepository
	CreateEdgeRepository(tx Transaction) EdgeRepository
	CreateCategoryRepository(tx Transaction) CategoryRepository
	CreateKeywordRepository(tx Transaction) KeywordRepository
	CreateGraphRepository(tx Transaction) GraphRepository
	CreateNodeCategoryRepository(tx Transaction) NodeCategoryRepository
}

// UnitOfWorkManager provides higher-level unit of work operations.
// This demonstrates the facade pattern for complex operations.
type UnitOfWorkManager interface {
	// ExecuteInTransaction runs a function within a unit of work
	ExecuteInTransaction(ctx context.Context, fn func(uow UnitOfWork) error) error
	
	// ExecuteWithRetry runs a function with automatic retry on transient failures
	ExecuteWithRetry(ctx context.Context, maxRetries int, fn func(uow UnitOfWork) error) error
}

// unitOfWorkManager implements UnitOfWorkManager
type unitOfWorkManager struct {
	uowFactory func() UnitOfWork
}

// NewUnitOfWorkManager creates a new unit of work manager.
func NewUnitOfWorkManager(uowFactory func() UnitOfWork) UnitOfWorkManager {
	return &unitOfWorkManager{
		uowFactory: uowFactory,
	}
}

// ExecuteInTransaction provides a convenient way to execute operations within a transaction.
// This method demonstrates proper resource management and error handling patterns.
func (m *unitOfWorkManager) ExecuteInTransaction(ctx context.Context, fn func(uow UnitOfWork) error) error {
	uow := m.uowFactory()
	
	// Begin transaction
	if err := uow.Begin(ctx); err != nil {
		return fmt.Errorf("failed to begin unit of work: %w", err)
	}
	
	// Ensure cleanup (rollback is safe to call multiple times)
	defer func() {
		if !uow.IsCommitted() {
			uow.Rollback()
		}
	}()
	
	// Execute user function
	if err := fn(uow); err != nil {
		return err
	}
	
	// Commit transaction
	if err := uow.Commit(); err != nil {
		return fmt.Errorf("failed to commit unit of work: %w", err)
	}
	
	return nil
}

// ExecuteWithRetry executes a function with automatic retry on transient failures.
// This demonstrates resilience patterns in repository operations.
func (m *unitOfWorkManager) ExecuteWithRetry(ctx context.Context, maxRetries int, fn func(uow UnitOfWork) error) error {
	var lastErr error
	
	for attempt := 0; attempt <= maxRetries; attempt++ {
		err := m.ExecuteInTransaction(ctx, fn)
		if err == nil {
			return nil // Success
		}
		
		lastErr = err
		
		// Check if error is retryable
		if !IsTransientError(err) {
			return err // Don't retry non-transient errors
		}
		
		if attempt < maxRetries {
			// Wait before retry (exponential backoff could be implemented here)
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(time.Millisecond * time.Duration(100*(attempt+1))):
				// Continue to next attempt
			}
		}
	}
	
	return fmt.Errorf("operation failed after %d retries: %w", maxRetries, lastErr)
}

// IsTransientError determines if an error is transient and worth retrying.
// This checks for common transient error patterns in the error message.
func IsTransientError(err error) bool {
	if err == nil {
		return false
	}
	
	errStr := err.Error()
	// Check for common transient error patterns
	transientPatterns := []string{
		"transaction conflict",
		"timeout",
		"connection failed",
		"connection refused",
		"temporary failure",
		"deadlock",
	}
	
	for _, pattern := range transientPatterns {
		if strings.Contains(strings.ToLower(errStr), pattern) {
			return true
		}
	}
	return false
}