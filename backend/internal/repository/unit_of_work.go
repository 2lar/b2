// Package repository - Unit of Work pattern implementation
//
// The Unit of Work pattern demonstrates enterprise-grade transaction management
// by maintaining a list of objects affected by a business transaction and
// coordinating writing out changes and resolving concurrency problems.
//
// Educational Goals:
//   - Show how to maintain transaction consistency across multiple repositories
//   - Demonstrate proper error handling and rollback mechanisms
//   - Illustrate domain event coordination
//   - Provide a clean abstraction for complex business transactions
package repository

import (
	"context"
	"fmt"

	"brain2-backend/internal/domain"
)

// UnitOfWork defines the interface for the Unit of Work pattern.
// This pattern ensures that a series of related operations either all succeed or all fail,
// maintaining data consistency across multiple repository operations.
//
// Key Responsibilities:
//   - Transaction lifecycle management (begin, commit, rollback)
//   - Repository coordination within transaction boundaries
//   - Domain event collection and publishing
//   - Consistency validation across aggregates
type UnitOfWork interface {
	// Begin starts a new unit of work transaction
	Begin(ctx context.Context) error
	
	// Commit persists all changes and publishes domain events
	Commit(ctx context.Context) error
	
	// Rollback discards all changes made within the unit of work
	Rollback(ctx context.Context) error
	
	// Repository access within the transaction context
	Nodes() NodeRepository
	Edges() EdgeRepository  
	Categories() CategoryRepository
	NodeCategories() NodeCategoryMapper
	Keywords() KeywordSearcher
	Graph() GraphReader
	
	// Domain event management
	RegisterEvents(events []domain.DomainEvent)
	GetRegisteredEvents() []domain.DomainEvent
	
	// Validation and consistency checks
	Validate(ctx context.Context) error
}

// Transaction represents a database transaction interface
// This abstraction allows the Unit of Work to work with different database implementations
type Transaction interface {
	Commit() error
	Rollback() error
	IsActive() bool
}

// EventPublisher handles publishing domain events after successful commits
type EventPublisher interface {
	Publish(ctx context.Context, events []domain.DomainEvent) error
}

// unitOfWork is the concrete implementation of the Unit of Work pattern
type unitOfWork struct {
	// Transaction management
	tx        Transaction
	committed bool
	rolledBack bool
	
	// Repository instances bound to this transaction
	nodeRepo         NodeRepository
	edgeRepo         EdgeRepository
	categoryRepo     CategoryRepository
	nodeCategoryRepo NodeCategoryMapper
	keywordRepo      KeywordSearcher
	graphRepo        GraphReader
	
	// Event management
	events      []domain.DomainEvent
	eventPublisher EventPublisher
	
	// Validation
	validators []UnitOfWorkValidator
}

// UnitOfWorkValidator defines custom validation logic that can be added to a unit of work
type UnitOfWorkValidator interface {
	Validate(ctx context.Context, uow UnitOfWork) error
}

// NewUnitOfWork creates a new unit of work with the provided dependencies.
// This factory function demonstrates dependency injection at the repository level.
func NewUnitOfWork(
	txFactory TransactionFactory,
	nodeRepo NodeRepository,
	edgeRepo EdgeRepository,
	categoryRepo CategoryRepository,
	nodeCategoryRepo NodeCategoryMapper,
	keywordRepo KeywordSearcher,
	graphRepo GraphReader,
	eventPublisher EventPublisher,
) UnitOfWork {
	return &unitOfWork{
		nodeRepo:         nodeRepo,
		edgeRepo:         edgeRepo,
		categoryRepo:     categoryRepo,
		nodeCategoryRepo: nodeCategoryRepo,
		keywordRepo:      keywordRepo,
		graphRepo:        graphRepo,
		eventPublisher:   eventPublisher,
		events:          make([]domain.DomainEvent, 0),
		validators:      make([]UnitOfWorkValidator, 0),
	}
}

// TransactionFactory creates new database transactions
type TransactionFactory interface {
	BeginTransaction(ctx context.Context) (Transaction, error)
}

// Begin starts a new unit of work transaction
func (uow *unitOfWork) Begin(ctx context.Context) error {
	if uow.tx != nil && uow.tx.IsActive() {
		return NewRepositoryError(
			ErrCodeTransactionActive,
			"transaction is already active",
			nil,
		)
	}
	
	// Reset state for new transaction
	uow.committed = false
	uow.rolledBack = false
	uow.events = make([]domain.DomainEvent, 0)
	
	// Begin database transaction would be handled here
	// For demonstration purposes, we'll assume success
	
	return nil
}

// Commit persists all changes and publishes domain events
func (uow *unitOfWork) Commit(ctx context.Context) error {
	if uow.committed {
		return NewRepositoryError(
			ErrCodeTransactionAlreadyCommitted,
			"transaction has already been committed",
			nil,
		)
	}
	
	if uow.rolledBack {
		return NewRepositoryError(
			ErrCodeTransactionRolledBack,
			"cannot commit a rolled back transaction",
			nil,
		)
	}
	
	// Validate consistency before committing
	if err := uow.Validate(ctx); err != nil {
		// Auto-rollback on validation failure
		if rollbackErr := uow.Rollback(ctx); rollbackErr != nil {
			return fmt.Errorf("commit validation failed: %w, rollback failed: %v", err, rollbackErr)
		}
		return fmt.Errorf("commit validation failed: %w", err)
	}
	
	// Commit the database transaction
	if uow.tx != nil {
		if err := uow.tx.Commit(); err != nil {
			return NewRepositoryError(
				ErrCodeCommitFailed,
				"failed to commit database transaction",
				err,
			)
		}
	}
	
	uow.committed = true
	
	// Publish domain events after successful commit
	if len(uow.events) > 0 {
		if err := uow.eventPublisher.Publish(ctx, uow.events); err != nil {
			// Log error but don't fail the transaction since data is already committed
			// In a production system, you might want to retry or use a message queue
			fmt.Printf("Warning: Failed to publish domain events: %v\n", err)
		}
	}
	
	return nil
}

// Rollback discards all changes made within the unit of work
func (uow *unitOfWork) Rollback(ctx context.Context) error {
	if uow.committed {
		return NewRepositoryError(
			ErrCodeTransactionAlreadyCommitted,
			"cannot rollback a committed transaction",
			nil,
		)
	}
	
	if uow.rolledBack {
		return nil // Already rolled back, this is idempotent
	}
	
	// Rollback the database transaction
	if uow.tx != nil && uow.tx.IsActive() {
		if err := uow.tx.Rollback(); err != nil {
			return NewRepositoryError(
				ErrCodeRollbackFailed,
				"failed to rollback database transaction",
				err,
			)
		}
	}
	
	// Clear events since they won't be published
	uow.events = make([]domain.DomainEvent, 0)
	uow.rolledBack = true
	
	return nil
}

// Repository access methods - these return repositories bound to the current transaction

func (uow *unitOfWork) Nodes() NodeRepository {
	return uow.nodeRepo
}

func (uow *unitOfWork) Edges() EdgeRepository {
	return uow.edgeRepo
}

func (uow *unitOfWork) Categories() CategoryRepository {
	return uow.categoryRepo
}

func (uow *unitOfWork) NodeCategories() NodeCategoryMapper {
	return uow.nodeCategoryRepo
}

func (uow *unitOfWork) Keywords() KeywordSearcher {
	return uow.keywordRepo
}

func (uow *unitOfWork) Graph() GraphReader {
	return uow.graphRepo
}

// Domain event management

// RegisterEvents adds domain events to be published after commit
func (uow *unitOfWork) RegisterEvents(events []domain.DomainEvent) {
	uow.events = append(uow.events, events...)
}

// GetRegisteredEvents returns all registered domain events
func (uow *unitOfWork) GetRegisteredEvents() []domain.DomainEvent {
	return uow.events
}

// Validate performs consistency validation across all aggregates
func (uow *unitOfWork) Validate(ctx context.Context) error {
	// Run custom validators
	for _, validator := range uow.validators {
		if err := validator.Validate(ctx, uow); err != nil {
			return fmt.Errorf("validation failed: %w", err)
		}
	}
	
	// Run built-in consistency checks
	if err := uow.validateNodeConsistency(ctx); err != nil {
		return fmt.Errorf("node consistency check failed: %w", err)
	}
	
	if err := uow.validateEdgeConsistency(ctx); err != nil {
		return fmt.Errorf("edge consistency check failed: %w", err)
	}
	
	return nil
}

// AddValidator adds a custom validator to the unit of work
func (uow *unitOfWork) AddValidator(validator UnitOfWorkValidator) {
	uow.validators = append(uow.validators, validator)
}

// Built-in consistency validation methods

func (uow *unitOfWork) validateNodeConsistency(ctx context.Context) error {
	// This would implement node-specific consistency checks
	// For example: ensuring nodes belong to the correct user
	return nil
}

func (uow *unitOfWork) validateEdgeConsistency(ctx context.Context) error {
	// This would implement edge-specific consistency checks  
	// For example: ensuring edges connect existing nodes
	return nil
}

// UnitOfWorkExecutor provides a convenient way to execute operations within a unit of work
type UnitOfWorkExecutor struct {
	uow UnitOfWork
}

// NewUnitOfWorkExecutor creates a new executor for the given unit of work
func NewUnitOfWorkExecutor(uow UnitOfWork) *UnitOfWorkExecutor {
	return &UnitOfWorkExecutor{uow: uow}
}

// Execute runs the provided function within a unit of work transaction.
// This method demonstrates the Execute Around idiom for transaction management.
//
// If the function succeeds, the transaction is committed.
// If the function fails or panics, the transaction is rolled back.
func (executor *UnitOfWorkExecutor) Execute(ctx context.Context, operation func(uow UnitOfWork) error) (err error) {
	// Begin transaction
	if err := executor.uow.Begin(ctx); err != nil {
		return fmt.Errorf("failed to begin unit of work: %w", err)
	}
	
	// Ensure cleanup happens regardless of how we exit
	defer func() {
		if r := recover(); r != nil {
			// Panic occurred, rollback and re-panic
			if rollbackErr := executor.uow.Rollback(ctx); rollbackErr != nil {
				panic(fmt.Sprintf("panic during operation: %v, rollback failed: %v", r, rollbackErr))
			}
			panic(r)
		} else if err != nil {
			// Error occurred, rollback
			if rollbackErr := executor.uow.Rollback(ctx); rollbackErr != nil {
				err = fmt.Errorf("operation failed: %w, rollback failed: %v", err, rollbackErr)
			}
		} else {
			// Success, commit
			if commitErr := executor.uow.Commit(ctx); commitErr != nil {
				err = fmt.Errorf("operation succeeded but commit failed: %w", commitErr)
			}
		}
	}()
	
	// Execute the operation
	return operation(executor.uow)
}

// Example validators

// NodeOwnershipValidator ensures all nodes belong to the specified user
type NodeOwnershipValidator struct {
	ExpectedUserID domain.UserID
}

func (v *NodeOwnershipValidator) Validate(ctx context.Context, uow UnitOfWork) error {
	// This would implement the actual validation logic
	// For educational purposes, we'll assume it passes
	return nil
}

// EdgeValidityValidator ensures all edges connect existing nodes
type EdgeValidityValidator struct{}

func (v *EdgeValidityValidator) Validate(ctx context.Context, uow UnitOfWork) error {
	// This would implement the actual validation logic
	// For educational purposes, we'll assume it passes
	return nil
}