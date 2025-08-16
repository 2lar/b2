// Package services provides application services for the Brain2 backend.
// This file implements the transaction manager for Unit of Work pattern.
package services

import (
	"context"
	"fmt"
	"sync"
	
	"brain2-backend/internal/domain"
	"brain2-backend/internal/repository"
	
	"go.uber.org/zap"
)

// TransactionManager handles transaction boundaries for operations.
// This implements the Unit of Work pattern for transactional consistency.
type TransactionManager struct {
	// Repository instances
	nodeRepo     repository.NodeRepository
	edgeRepo     repository.EdgeRepository
	categoryRepo repository.CategoryRepository
	
	// Transaction state
	mu            sync.Mutex
	inTransaction bool
	operations    []Operation
	rollbacks     []func() error
	
	// Event handling
	eventBus      domain.EventBus
	pendingEvents []domain.DomainEvent
	
	logger *zap.Logger
}

// Operation represents a single operation within a transaction.
type Operation struct {
	Type   OperationType
	Entity interface{}
	ID     string
}

// OperationType defines the type of operation.
type OperationType string

const (
	OperationCreate OperationType = "CREATE"
	OperationUpdate OperationType = "UPDATE"
	OperationDelete OperationType = "DELETE"
)

// NewTransactionManager creates a new transaction manager.
func NewTransactionManager(
	nodeRepo repository.NodeRepository,
	edgeRepo repository.EdgeRepository,
	categoryRepo repository.CategoryRepository,
	eventBus domain.EventBus,
	logger *zap.Logger,
) *TransactionManager {
	return &TransactionManager{
		nodeRepo:      nodeRepo,
		edgeRepo:      edgeRepo,
		categoryRepo:  categoryRepo,
		eventBus:      eventBus,
		operations:    make([]Operation, 0),
		rollbacks:     make([]func() error, 0),
		pendingEvents: make([]domain.DomainEvent, 0),
		logger:        logger,
	}
}

// ============================================================================
// TRANSACTION METHODS
// ============================================================================

// ExecuteInTransaction runs operations within a transaction boundary.
func (tm *TransactionManager) ExecuteInTransaction(
	ctx context.Context,
	fn func(ctx context.Context) error,
) error {
	tm.mu.Lock()
	if tm.inTransaction {
		tm.mu.Unlock()
		return fmt.Errorf("nested transactions are not supported")
	}
	tm.inTransaction = true
	tm.mu.Unlock()
	
	// Reset state
	tm.operations = make([]Operation, 0)
	tm.rollbacks = make([]func() error, 0)
	tm.pendingEvents = make([]domain.DomainEvent, 0)
	
	// Create transaction context
	txCtx := context.WithValue(ctx, "transaction", tm)
	
	// Execute the function
	err := fn(txCtx)
	
	tm.mu.Lock()
	defer func() {
		tm.inTransaction = false
		tm.mu.Unlock()
	}()
	
	if err != nil {
		// Rollback on error
		tm.logger.Info("Rolling back transaction due to error",
			zap.Error(err),
			zap.Int("operations", len(tm.operations)),
		)
		
		if rollbackErr := tm.rollback(); rollbackErr != nil {
			tm.logger.Error("Rollback failed",
				zap.Error(rollbackErr),
			)
			return fmt.Errorf("transaction failed and rollback failed: %w", rollbackErr)
		}
		
		return fmt.Errorf("transaction failed: %w", err)
	}
	
	// Commit - publish events after successful transaction
	if err := tm.commit(ctx); err != nil {
		// Try to rollback if commit fails
		if rollbackErr := tm.rollback(); rollbackErr != nil {
			tm.logger.Error("Rollback after commit failure failed",
				zap.Error(rollbackErr),
			)
		}
		return fmt.Errorf("commit failed: %w", err)
	}
	
	tm.logger.Info("Transaction completed successfully",
		zap.Int("operations", len(tm.operations)),
		zap.Int("events", len(tm.pendingEvents)),
	)
	
	return nil
}

// rollback reverses all operations in the transaction.
func (tm *TransactionManager) rollback() error {
	var errs []error
	
	// Execute rollback functions in reverse order
	for i := len(tm.rollbacks) - 1; i >= 0; i-- {
		if err := tm.rollbacks[i](); err != nil {
			errs = append(errs, err)
			tm.logger.Warn("Rollback operation failed",
				zap.Int("index", i),
				zap.Error(err),
			)
		}
	}
	
	if len(errs) > 0 {
		return fmt.Errorf("rollback completed with %d errors", len(errs))
	}
	
	return nil
}

// commit finalizes the transaction by publishing all pending events.
func (tm *TransactionManager) commit(ctx context.Context) error {
	// Publish all pending events
	for _, event := range tm.pendingEvents {
		if err := tm.eventBus.Publish(ctx, event); err != nil {
			tm.logger.Warn("Failed to publish event",
				zap.String("event_type", event.EventType()),
				zap.Error(err),
			)
			// Continue publishing other events even if one fails
		}
	}
	
	return nil
}

// ============================================================================
// NODE OPERATIONS
// ============================================================================

// CreateNode creates a node within the transaction.
// TODO: Update to use NodeWriter interface methods
func (tm *TransactionManager) CreateNode(ctx context.Context, node *domain.Node) error {
	if !tm.inTransaction {
		return fmt.Errorf("operation must be executed within a transaction")
	}
	
	// TODO: Implement using proper repository methods
	// The current NodeRepository doesn't have CreateNode method
	// Should use NodeWriter.Save() instead
	return fmt.Errorf("CreateNode not implemented - needs repository refactoring")
}

// UpdateNode updates a node within the transaction.
// TODO: Update to use NodeWriter interface methods
func (tm *TransactionManager) UpdateNode(ctx context.Context, node *domain.Node, originalNode *domain.Node) error {
	if !tm.inTransaction {
		return fmt.Errorf("operation must be executed within a transaction")
	}
	
	// TODO: Implement using proper repository methods
	// The current NodeRepository doesn't have UpdateNode method
	// Should use NodeWriter.Update() instead
	return fmt.Errorf("UpdateNode not implemented - needs repository refactoring")
}

// DeleteNode deletes a node within the transaction.
// TODO: Update to use NodeWriter interface methods
func (tm *TransactionManager) DeleteNode(ctx context.Context, node *domain.Node) error {
	if !tm.inTransaction {
		return fmt.Errorf("operation must be executed within a transaction")
	}
	
	// TODO: Implement using proper repository methods
	// The current NodeRepository.DeleteNode needs userID and nodeID strings
	// Should use NodeWriter.Delete() instead
	return fmt.Errorf("DeleteNode not implemented - needs repository refactoring")
}

// ============================================================================
// EDGE OPERATIONS
// ============================================================================

// CreateEdge creates an edge within the transaction.
// TODO: Update to use EdgeWriter interface methods
func (tm *TransactionManager) CreateEdge(ctx context.Context, edge *domain.Edge) error {
	if !tm.inTransaction {
		return fmt.Errorf("operation must be executed within a transaction")
	}
	
	// TODO: Implement using proper repository methods
	return fmt.Errorf("CreateEdge not implemented - needs repository refactoring")
}

// UpdateEdge updates an edge within the transaction.
// TODO: Update to use EdgeWriter interface methods
func (tm *TransactionManager) UpdateEdge(ctx context.Context, edge *domain.Edge, originalEdge *domain.Edge) error {
	if !tm.inTransaction {
		return fmt.Errorf("operation must be executed within a transaction")
	}
	
	// TODO: Implement using proper repository methods
	return fmt.Errorf("UpdateEdge not implemented - needs repository refactoring")
}

// DeleteEdge deletes an edge within the transaction.
// TODO: Update to use EdgeWriter interface methods
func (tm *TransactionManager) DeleteEdge(ctx context.Context, edge *domain.Edge) error {
	if !tm.inTransaction {
		return fmt.Errorf("operation must be executed within a transaction")
	}
	
	// TODO: Implement using proper repository methods
	return fmt.Errorf("DeleteEdge not implemented - needs repository refactoring")
}

// ============================================================================
// CATEGORY OPERATIONS
// ============================================================================

// CreateCategory creates a category within the transaction.
// TODO: Update to use CategoryWriter interface methods
func (tm *TransactionManager) CreateCategory(ctx context.Context, category *domain.Category) error {
	if !tm.inTransaction {
		return fmt.Errorf("operation must be executed within a transaction")
	}
	
	// TODO: Implement using proper repository methods
	return fmt.Errorf("CreateCategory not implemented - needs repository refactoring")
}

// UpdateCategory updates a category within the transaction.
// TODO: Update to use CategoryWriter interface methods
func (tm *TransactionManager) UpdateCategory(ctx context.Context, category *domain.Category, originalCategory *domain.Category) error {
	if !tm.inTransaction {
		return fmt.Errorf("operation must be executed within a transaction")
	}
	
	// TODO: Implement using proper repository methods
	return fmt.Errorf("UpdateCategory not implemented - needs repository refactoring")
}

// DeleteCategory deletes a category within the transaction.
// TODO: Update to use CategoryWriter interface methods
func (tm *TransactionManager) DeleteCategory(ctx context.Context, category *domain.Category) error {
	if !tm.inTransaction {
		return fmt.Errorf("operation must be executed within a transaction")
	}
	
	// TODO: Implement using proper repository methods
	return fmt.Errorf("DeleteCategory not implemented - needs repository refactoring")
}

// ============================================================================
// HELPER METHODS
// ============================================================================

// GetTransactionManager extracts the transaction manager from context.
func GetTransactionManager(ctx context.Context) (*TransactionManager, bool) {
	tm, ok := ctx.Value("transaction").(*TransactionManager)
	return tm, ok
}

// IsInTransaction checks if the context is within a transaction.
func IsInTransaction(ctx context.Context) bool {
	_, ok := GetTransactionManager(ctx)
	return ok
}

// GetOperationCount returns the number of operations in the current transaction.
func (tm *TransactionManager) GetOperationCount() int {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	return len(tm.operations)
}

// GetPendingEventCount returns the number of pending events.
func (tm *TransactionManager) GetPendingEventCount() int {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	return len(tm.pendingEvents)
}