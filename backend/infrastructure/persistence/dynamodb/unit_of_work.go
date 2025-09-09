package dynamodb

import (
	"context"
	"fmt"

	"backend/application/ports"
	"backend/domain/events"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

// DynamoDBUnitOfWork implements the UnitOfWork pattern for DynamoDB
// It manages transactions across multiple aggregates and ensures consistency
type DynamoDBUnitOfWork struct {
	client         *dynamodb.Client
	nodeRepo       ports.NodeRepository
	edgeRepo       ports.EdgeRepository
	graphRepo      ports.GraphRepository
	eventStore     ports.EventStore
	eventPublisher ports.EventPublisher

	// Transaction tracking
	transactItems   []types.TransactWriteItem
	pendingEvents   []events.DomainEvent
	rollbackActions []func() error
	inTransaction   bool
}

// NewDynamoDBUnitOfWork creates a new unit of work instance
func NewDynamoDBUnitOfWork(
	client *dynamodb.Client,
	nodeRepo ports.NodeRepository,
	edgeRepo ports.EdgeRepository,
	graphRepo ports.GraphRepository,
	eventStore ports.EventStore,
	eventPublisher ports.EventPublisher,
) *DynamoDBUnitOfWork {
	return &DynamoDBUnitOfWork{
		client:         client,
		nodeRepo:       nodeRepo,
		edgeRepo:       edgeRepo,
		graphRepo:      graphRepo,
		eventStore:     eventStore,
		eventPublisher: eventPublisher,
		transactItems:  make([]types.TransactWriteItem, 0),
		pendingEvents:  make([]events.DomainEvent, 0),
	}
}

// Begin starts a new transaction
func (uow *DynamoDBUnitOfWork) Begin(ctx context.Context) error {
	if uow.inTransaction {
		return fmt.Errorf("transaction already in progress")
	}
	uow.inTransaction = true
	uow.Clear()
	return nil
}

// RegisterSave registers an entity save operation in the transaction
func (uow *DynamoDBUnitOfWork) RegisterSave(item types.TransactWriteItem) error {
	if !uow.inTransaction {
		return fmt.Errorf("no transaction in progress")
	}
	uow.transactItems = append(uow.transactItems, item)
	return nil
}

// RegisterDelete registers an entity delete operation in the transaction
func (uow *DynamoDBUnitOfWork) RegisterDelete(tableName, pk, sk string) error {
	if !uow.inTransaction {
		return fmt.Errorf("no transaction in progress")
	}

	item := types.TransactWriteItem{
		Delete: &types.Delete{
			TableName: aws.String(tableName),
			Key: map[string]types.AttributeValue{
				"PK": &types.AttributeValueMemberS{Value: pk},
				"SK": &types.AttributeValueMemberS{Value: sk},
			},
		},
	}
	uow.transactItems = append(uow.transactItems, item)
	return nil
}

// RegisterEvent registers a domain event to be published after commit
func (uow *DynamoDBUnitOfWork) RegisterEvent(event events.DomainEvent) error {
	if !uow.inTransaction {
		return fmt.Errorf("no transaction in progress")
	}
	uow.pendingEvents = append(uow.pendingEvents, event)
	return nil
}

// RegisterRollback registers a rollback action
func (uow *DynamoDBUnitOfWork) RegisterRollback(action func() error) error {
	if !uow.inTransaction {
		return fmt.Errorf("no transaction in progress")
	}
	uow.rollbackActions = append(uow.rollbackActions, action)
	return nil
}

// Commit executes all registered operations atomically
func (uow *DynamoDBUnitOfWork) Commit(ctx context.Context) error {
	if !uow.inTransaction {
		return fmt.Errorf("no transaction in progress")
	}

	defer func() {
		uow.inTransaction = false
	}()

	// Validate transaction size (DynamoDB limit is 100 items for TransactWriteItems)
	// Note: The actual limit is 100, but we use 25 for safety and to leave room for event items
	if len(uow.transactItems) > 25 {
		return fmt.Errorf("transaction exceeds safe limit of 25 items: %d items", len(uow.transactItems))
	}

	// Add event store items to transaction if event store supports it
	if uow.eventStore != nil {
		for _, event := range uow.pendingEvents {
			// Check if event store supports transactional writes
			if transactionalStore, ok := uow.eventStore.(interface {
				PrepareEventItem(events.DomainEvent) (types.TransactWriteItem, error)
			}); ok {
				eventItem, err := transactionalStore.PrepareEventItem(event)
				if err != nil {
					uow.executeRollback()
					return fmt.Errorf("failed to prepare event item: %w", err)
				}
				uow.transactItems = append(uow.transactItems, eventItem)
			}
		}
	}

	// Execute DynamoDB transaction if there are items to commit
	if len(uow.transactItems) > 0 {
		input := &dynamodb.TransactWriteItemsInput{
			TransactItems: uow.transactItems,
		}

		_, err := uow.client.TransactWriteItems(ctx, input)
		if err != nil {
			uow.executeRollback()
			return fmt.Errorf("transaction failed: %w", err)
		}
	}

	// Events are now persisted with "pending" status using the Outbox pattern
	// A separate background process will handle publishing them to EventBridge
	// This ensures that events are never lost even if publishing fails

	// Note: The events were already saved to the event store during the transaction
	// with PublishStatus = "pending", so we don't need to publish them here.
	// The OutboxProcessor will handle publishing asynchronously.

	// Clear transaction state
	uow.Clear()

	return nil
}

// Rollback cancels the current transaction
func (uow *DynamoDBUnitOfWork) Rollback() error {
	if !uow.inTransaction {
		return fmt.Errorf("no transaction in progress")
	}

	defer func() {
		uow.inTransaction = false
	}()

	uow.executeRollback()
	uow.Clear()
	return nil
}

// executeRollback runs all registered rollback actions
func (uow *DynamoDBUnitOfWork) executeRollback() {
	for i := len(uow.rollbackActions) - 1; i >= 0; i-- {
		if err := uow.rollbackActions[i](); err != nil {
			// Log but continue with other rollbacks
			fmt.Printf("Warning: rollback action failed: %v\n", err)
		}
	}
}

// Clear resets the unit of work state
func (uow *DynamoDBUnitOfWork) Clear() {
	uow.transactItems = make([]types.TransactWriteItem, 0)
	uow.pendingEvents = make([]events.DomainEvent, 0)
	uow.rollbackActions = make([]func() error, 0)
}

// NodeRepository returns the node repository
func (uow *DynamoDBUnitOfWork) NodeRepository() ports.NodeRepository {
	return uow.nodeRepo
}

// EdgeRepository returns the edge repository
func (uow *DynamoDBUnitOfWork) EdgeRepository() ports.EdgeRepository {
	return uow.edgeRepo
}

// GraphRepository returns the graph repository
func (uow *DynamoDBUnitOfWork) GraphRepository() ports.GraphRepository {
	return uow.graphRepo
}

// IsInTransaction returns whether a transaction is currently active
func (uow *DynamoDBUnitOfWork) IsInTransaction() bool {
	return uow.inTransaction
}
