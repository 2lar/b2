// Package dynamodb provides DynamoDB implementations of repository interfaces
package dynamodb

import (
	"context"
	"fmt"
	
	"brain2-backend/internal/core/application/ports"
	
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

// UnitOfWork manages transactional boundaries for DynamoDB operations
type UnitOfWork struct {
	client           *dynamodb.Client
	nodeRepo        ports.NodeRepository
	edgeRepo        ports.EdgeRepository
	eventStore      ports.EventStore
	transactItems   []types.TransactWriteItem
	ctx             context.Context
	logger          ports.Logger
	isActive        bool
}

// NewUnitOfWork creates a new unit of work
func NewUnitOfWork(
	client *dynamodb.Client,
	nodeRepo ports.NodeRepository,
	edgeRepo ports.EdgeRepository,
	eventStore ports.EventStore,
	logger ports.Logger,
) *UnitOfWork {
	return &UnitOfWork{
		client:        client,
		nodeRepo:      nodeRepo,
		edgeRepo:      edgeRepo,
		eventStore:    eventStore,
		transactItems: make([]types.TransactWriteItem, 0),
		logger:        logger,
		isActive:      false,
	}
}

// Begin starts a new unit of work
func (u *UnitOfWork) Begin(ctx context.Context) error {
	if u.isActive {
		return fmt.Errorf("unit of work already active")
	}
	
	u.ctx = ctx
	u.transactItems = make([]types.TransactWriteItem, 0)
	u.isActive = true
	
	u.logger.Debug("Unit of work started")
	return nil
}

// Commit commits all changes
func (u *UnitOfWork) Commit() error {
	if !u.isActive {
		return fmt.Errorf("no active unit of work")
	}
	
	defer func() {
		u.isActive = false
		u.transactItems = nil
	}()
	
	if len(u.transactItems) == 0 {
		u.logger.Debug("No items to commit")
		return nil
	}
	
	// DynamoDB transaction limit is 100 items
	if len(u.transactItems) > 100 {
		return fmt.Errorf("transaction exceeds DynamoDB limit of 100 items: %d", len(u.transactItems))
	}
	
	// Execute the transaction
	input := &dynamodb.TransactWriteItemsInput{
		TransactItems: u.transactItems,
	}
	
	_, err := u.client.TransactWriteItems(u.ctx, input)
	if err != nil {
		u.logger.Error("Failed to commit transaction", err,
			ports.Field{Key: "item_count", Value: len(u.transactItems)})
		return fmt.Errorf("transaction commit failed: %w", err)
	}
	
	u.logger.Info("Transaction committed",
		ports.Field{Key: "item_count", Value: len(u.transactItems)})
	
	return nil
}

// Rollback rolls back all changes
func (u *UnitOfWork) Rollback() error {
	if !u.isActive {
		return fmt.Errorf("no active unit of work")
	}
	
	u.isActive = false
	u.transactItems = nil
	
	u.logger.Debug("Unit of work rolled back")
	return nil
}

// NodeRepository returns the node repository for this unit of work
func (u *UnitOfWork) NodeRepository() ports.NodeRepository {
	// In a transactional context, we would return a wrapper that
	// collects operations for batch execution. For now, return
	// the regular repository as DynamoDB transactions are handled differently
	return u.nodeRepo
}

// EdgeRepository returns the edge repository for this unit of work
func (u *UnitOfWork) EdgeRepository() ports.EdgeRepository {
	return u.edgeRepo
}

// EventStore returns the event store for this unit of work
func (u *UnitOfWork) EventStore() ports.EventStore {
	return u.eventStore
}

// AddTransactItem adds a transaction item to the batch
// This is an internal method that repositories can use when operating
// within a unit of work context
func (u *UnitOfWork) AddTransactItem(item types.TransactWriteItem) error {
	if !u.isActive {
		return fmt.Errorf("no active unit of work")
	}
	
	if len(u.transactItems) >= 100 {
		return fmt.Errorf("transaction limit reached")
	}
	
	u.transactItems = append(u.transactItems, item)
	return nil
}

// IsActive returns whether the unit of work is active
func (u *UnitOfWork) IsActive() bool {
	return u.isActive
}

// UnitOfWorkFactory creates new units of work
type UnitOfWorkFactory struct {
	client     *dynamodb.Client
	nodeRepo   ports.NodeRepository
	edgeRepo   ports.EdgeRepository
	eventStore ports.EventStore
	logger     ports.Logger
}

// NewUnitOfWorkFactory creates a new unit of work factory
func NewUnitOfWorkFactory(
	client *dynamodb.Client,
	nodeRepo ports.NodeRepository,
	edgeRepo ports.EdgeRepository,
	eventStore ports.EventStore,
	logger ports.Logger,
) *UnitOfWorkFactory {
	return &UnitOfWorkFactory{
		client:     client,
		nodeRepo:   nodeRepo,
		edgeRepo:   edgeRepo,
		eventStore: eventStore,
		logger:     logger,
	}
}

// Create creates a new unit of work
func (f *UnitOfWorkFactory) Create(ctx context.Context) (ports.UnitOfWork, error) {
	uow := NewUnitOfWork(f.client, f.nodeRepo, f.edgeRepo, f.eventStore, f.logger)
	if err := uow.Begin(ctx); err != nil {
		return nil, err
	}
	return uow, nil
}