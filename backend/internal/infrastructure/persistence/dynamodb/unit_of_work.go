// Package dynamodb provides DynamoDB implementations of repository interfaces.
// This file provides a proper Unit of Work implementation without panics.
package dynamodb

import (
	"context"
	"fmt"
	"sync"
	
	"brain2-backend/internal/domain/shared"
	"brain2-backend/internal/repository"
	"brain2-backend/pkg/errors"
	
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"go.uber.org/zap"
)

// ProperUnitOfWork implements the Unit of Work pattern properly without panics.
type ProperUnitOfWork struct {
	client    *dynamodb.Client
	tableName string
	indexName string
	logger    *zap.Logger
	eventBus  shared.EventBus
	
	// Transaction state
	mu              sync.Mutex
	isInTransaction bool
	isCommitted     bool
	isRolledBack    bool
	transactItems   []types.TransactWriteItem
	pendingEvents   []shared.DomainEvent
	
	// Repositories
	nodeRepo     repository.NodeRepository
	edgeRepo     repository.EdgeRepository
	categoryRepo repository.CategoryRepository
	keywordRepo  repository.KeywordRepository
	graphRepo    repository.GraphRepository
}

// NewProperUnitOfWork creates a new unit of work without panics.
func NewProperUnitOfWork(
	client *dynamodb.Client,
	tableName string,
	indexName string,
	logger *zap.Logger,
	eventBus shared.EventBus,
) *ProperUnitOfWork {
	uow := &ProperUnitOfWork{
		client:        client,
		tableName:     tableName,
		indexName:     indexName,
		logger:        logger,
		eventBus:      eventBus,
		transactItems: make([]types.TransactWriteItem, 0),
		pendingEvents: make([]shared.DomainEvent, 0),
	}
	
	// Initialize repositories
	uow.nodeRepo = NewNodeRepository(client, tableName, indexName, logger)
	uow.edgeRepo = NewEdgeRepositoryV2(client, tableName, indexName, logger)
	uow.categoryRepo = NewCategoryRepositoryCQRS(client, tableName, indexName, logger)
	uow.keywordRepo = NewKeywordRepository(client, tableName, indexName)
	uow.graphRepo = NewGraphRepository(client, tableName, indexName, logger)
	
	return uow
}

// NewDynamoDBUnitOfWork is an alias for NewProperUnitOfWork to maintain backward compatibility.
func NewDynamoDBUnitOfWork(
	client *dynamodb.Client,
	tableName string,
	indexName string,
	eventBus shared.EventBus,
	eventStore repository.EventStore,
	logger *zap.Logger,
) repository.UnitOfWork {
	// Note: eventStore parameter is not used in the new implementation
	_ = eventStore
	return NewProperUnitOfWork(client, tableName, indexName, logger, eventBus)
}

// Begin starts a new transaction.
func (uow *ProperUnitOfWork) Begin(ctx context.Context) error {
	uow.mu.Lock()
	defer uow.mu.Unlock()
	
	if uow.isInTransaction {
		return errors.NewValidation("transaction already in progress")
	}
	
	if uow.isCommitted {
		return errors.NewValidation("unit of work already committed")
	}
	
	if uow.isRolledBack {
		return errors.NewValidation("unit of work already rolled back")
	}
	
	uow.isInTransaction = true
	uow.transactItems = make([]types.TransactWriteItem, 0)
	uow.pendingEvents = make([]shared.DomainEvent, 0)
	
	uow.logger.Debug("Transaction started")
	return nil
}

// Commit commits the transaction.
func (uow *ProperUnitOfWork) Commit() error {
	uow.mu.Lock()
	defer uow.mu.Unlock()
	
	if !uow.isInTransaction {
		return errors.NewValidation("no active transaction to commit")
	}
	
	if uow.isCommitted {
		return errors.NewValidation("transaction already committed")
	}
	
	if uow.isRolledBack {
		return errors.NewValidation("transaction already rolled back")
	}
	
	// Execute transactional writes if any
	if len(uow.transactItems) > 0 {
		input := &dynamodb.TransactWriteItemsInput{
			TransactItems: uow.transactItems,
		}
		
		_, err := uow.client.TransactWriteItems(context.TODO(), input)
		if err != nil {
			uow.logger.Error("Transaction commit failed",
				zap.Error(err),
				zap.Int("items", len(uow.transactItems)),
			)
			// Attempt rollback
			uow.isInTransaction = false
			uow.isRolledBack = true
			return fmt.Errorf("failed to commit transaction: %w", err)
		}
	}
	
	// Publish domain events
	uow.logger.Info("DEBUG: UoW.Commit starting event publishing",
		zap.Int("pending_events_count", len(uow.pendingEvents)),
	)
	
	for i, event := range uow.pendingEvents {
		uow.logger.Info("DEBUG: UoW.Commit publishing event",
			zap.Int("event_index", i),
			zap.String("event_type", event.EventType()),
			zap.String("event_id", event.EventID()),
		)
		
		if err := uow.eventBus.Publish(context.TODO(), event); err != nil {
			uow.logger.Warn("Failed to publish event",
				zap.String("event_type", event.EventType()),
				zap.Error(err),
			)
			// Continue publishing other events
		} else {
			uow.logger.Info("DEBUG: UoW.Commit successfully published event",
				zap.String("event_type", event.EventType()),
			)
		}
	}
	
	uow.logger.Info("DEBUG: UoW.Commit finished event publishing",
		zap.Int("total_events_processed", len(uow.pendingEvents)),
	)
	
	uow.isInTransaction = false
	uow.isCommitted = true
	
	uow.logger.Debug("Transaction committed",
		zap.Int("items", len(uow.transactItems)),
		zap.Int("events", len(uow.pendingEvents)),
	)
	
	return nil
}

// Rollback rolls back the transaction.
func (uow *ProperUnitOfWork) Rollback() error {
	uow.mu.Lock()
	defer uow.mu.Unlock()
	
	if !uow.isInTransaction {
		// Not an error - idempotent rollback
		return nil
	}
	
	if uow.isCommitted {
		return errors.NewValidation("cannot rollback committed transaction")
	}
	
	// Clear pending items and events
	uow.transactItems = make([]types.TransactWriteItem, 0)
	uow.pendingEvents = make([]shared.DomainEvent, 0)
	
	uow.isInTransaction = false
	uow.isRolledBack = true
	
	uow.logger.Debug("Transaction rolled back")
	return nil
}

// Nodes returns the NodeRepository.
func (uow *ProperUnitOfWork) Nodes() repository.NodeRepository {
	if !uow.isInTransaction {
		uow.logger.Warn("Accessing Nodes repository outside of transaction")
	}
	return uow.nodeRepo
}

// Edges returns the EdgeRepository.
func (uow *ProperUnitOfWork) Edges() repository.EdgeRepository {
	if !uow.isInTransaction {
		uow.logger.Warn("Accessing Edges repository outside of transaction")
	}
	return uow.edgeRepo
}

// Categories returns the CategoryRepository.
func (uow *ProperUnitOfWork) Categories() repository.CategoryRepository {
	if !uow.isInTransaction {
		uow.logger.Warn("Accessing Categories repository outside of transaction")
	}
	return uow.categoryRepo
}

// Keywords returns the KeywordRepository.
func (uow *ProperUnitOfWork) Keywords() repository.KeywordRepository {
	if !uow.isInTransaction {
		uow.logger.Warn("Accessing Keywords repository outside of transaction")
	}
	return uow.keywordRepo
}

// Graph returns the GraphRepository.
func (uow *ProperUnitOfWork) Graph() repository.GraphRepository {
	if !uow.isInTransaction {
		uow.logger.Warn("Accessing Graph repository outside of transaction")
	}
	return uow.graphRepo
}

// NodeCategories returns the NodeCategoryRepository.
func (uow *ProperUnitOfWork) NodeCategories() repository.NodeCategoryRepository {
	// Not implemented - this is a known limitation
	uow.logger.Warn("NodeCategories repository not implemented")
	return nil
}

// PublishEvent adds an event to be published on commit.
func (uow *ProperUnitOfWork) PublishEvent(event shared.DomainEvent) {
	uow.mu.Lock()
	defer uow.mu.Unlock()
	
	// Debug logging for event publishing
	uow.logger.Info("DEBUG: UoW.PublishEvent called",
		zap.String("event_type", event.EventType()),
		zap.String("event_id", event.EventID()),
		zap.String("aggregate_id", event.AggregateID()),
		zap.Bool("in_transaction", uow.isInTransaction),
		zap.Int("current_pending_events", len(uow.pendingEvents)),
	)
	
	if !uow.isInTransaction {
		uow.logger.Warn("Publishing event outside of transaction",
			zap.String("event_type", event.EventType()),
		)
	}
	
	uow.pendingEvents = append(uow.pendingEvents, event)
	
	// Debug logging after adding event
	uow.logger.Info("DEBUG: UoW.PublishEvent completed",
		zap.String("event_type", event.EventType()),
		zap.Int("new_pending_events_count", len(uow.pendingEvents)),
	)
}

// GetPendingEvents returns pending domain events.
func (uow *ProperUnitOfWork) GetPendingEvents() []shared.DomainEvent {
	uow.mu.Lock()
	defer uow.mu.Unlock()
	
	events := make([]shared.DomainEvent, len(uow.pendingEvents))
	copy(events, uow.pendingEvents)
	return events
}

// IsActive checks if the unit of work is active.
func (uow *ProperUnitOfWork) IsActive() bool {
	uow.mu.Lock()
	defer uow.mu.Unlock()
	
	return uow.isInTransaction && !uow.isCommitted && !uow.isRolledBack
}

// IsCommitted checks if the unit of work has been committed.
func (uow *ProperUnitOfWork) IsCommitted() bool {
	uow.mu.Lock()
	defer uow.mu.Unlock()
	
	return uow.isCommitted
}

// IsRolledBack checks if the unit of work has been rolled back.
func (uow *ProperUnitOfWork) IsRolledBack() bool {
	uow.mu.Lock()
	defer uow.mu.Unlock()
	
	return uow.isRolledBack
}

// AddTransactItem adds a transactional write item.
func (uow *ProperUnitOfWork) AddTransactItem(item types.TransactWriteItem) error {
	uow.mu.Lock()
	defer uow.mu.Unlock()
	
	if !uow.isInTransaction {
		return errors.NewValidation("no active transaction")
	}
	
	if uow.isCommitted || uow.isRolledBack {
		return errors.NewValidation("transaction already completed")
	}
	
	// DynamoDB has a limit of 100 items per transaction
	if len(uow.transactItems) >= 100 {
		return errors.NewValidation("transaction item limit reached (100)")
	}
	
	uow.transactItems = append(uow.transactItems, item)
	return nil
}

// Reset resets the unit of work for reuse.
func (uow *ProperUnitOfWork) Reset() {
	uow.mu.Lock()
	defer uow.mu.Unlock()
	
	uow.isInTransaction = false
	uow.isCommitted = false
	uow.isRolledBack = false
	uow.transactItems = make([]types.TransactWriteItem, 0)
	uow.pendingEvents = make([]shared.DomainEvent, 0)
}

// Validate validates the unit of work state.
func (uow *ProperUnitOfWork) Validate() error {
	uow.mu.Lock()
	defer uow.mu.Unlock()
	
	if uow.isCommitted && uow.isRolledBack {
		return errors.NewInternal("invalid state: both committed and rolled back", nil)
	}
	
	if uow.isInTransaction && (uow.isCommitted || uow.isRolledBack) {
		return errors.NewInternal("invalid state: transaction active but completed", nil)
	}
	
	return nil
}