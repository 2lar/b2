package dynamodb

import (
	"context"
	"fmt"
	"sync"

	"brain2-backend/internal/domain/category"
	"brain2-backend/internal/domain/edge"
	"brain2-backend/internal/domain/node"
	"brain2-backend/internal/domain/shared"
	"brain2-backend/internal/repository"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"go.uber.org/zap"
)

// DynamoDBUnitOfWorkClean implements the Unit of Work pattern with PURE CQRS.
// NO mixed interfaces, PERFECT separation of concerns.
type DynamoDBUnitOfWorkClean struct {
	client    *dynamodb.Client
	tableName string
	indexName string
	logger    *zap.Logger
	eventBus  shared.EventBus
	
	// Transaction state
	mu            sync.Mutex
	isInTransaction bool
	isCommitted   bool
	isRolledBack  bool
	
	// CQRS Repository instances - separate readers and writers
	nodeReader     repository.NodeReader
	nodeWriter     repository.NodeWriter
	edgeReader     repository.EdgeReader
	edgeWriter     repository.EdgeWriter
	categoryReader repository.CategoryReader
	categoryWriter repository.CategoryWriter
	
	// Transactional items to be written atomically
	transactItems []types.TransactWriteItem
	
	// Domain events to be published atomically
	pendingEvents []shared.DomainEvent
	
	// Aggregates tracked for optimistic locking
	trackedAggregates map[string]shared.AggregateRoot
}

// DynamoDBUnitOfWorkFactoryClean creates UnitOfWork instances with CQRS.
type DynamoDBUnitOfWorkFactoryClean struct {
	client    *dynamodb.Client
	tableName string
	indexName string
	eventBus  shared.EventBus
	logger    *zap.Logger
}

// NewDynamoDBUnitOfWorkFactoryClean creates a new factory.
func NewDynamoDBUnitOfWorkFactoryClean(
	client *dynamodb.Client,
	tableName string,
	indexName string,
	eventBus shared.EventBus,
	logger *zap.Logger,
) repository.UnitOfWorkFactory {
	return &DynamoDBUnitOfWorkFactoryClean{
		client:    client,
		tableName: tableName,
		indexName: indexName,
		eventBus:  eventBus,
		logger:    logger,
	}
}

// Create creates a new UnitOfWork instance.
func (f *DynamoDBUnitOfWorkFactoryClean) Create(ctx context.Context) (repository.UnitOfWork, error) {
	// Create base repositories
	nodeRepo := NewNodeRepository(f.client, f.tableName, f.indexName, f.logger)
	edgeRepo := NewEdgeRepositoryCQRS(f.client, f.tableName, f.indexName, f.logger)
	categoryRepo := NewCategoryRepositoryCQRS(f.client, f.tableName, f.indexName, f.logger)
	
	return &DynamoDBUnitOfWorkClean{
		client:            f.client,
		tableName:         f.tableName,
		indexName:         f.indexName,
		logger:            f.logger,
		eventBus:          f.eventBus,
		nodeReader:        nodeRepo,  // NodeRepository implements both Reader and Writer
		nodeWriter:        nodeRepo,
		edgeReader:        edgeRepo,  // EdgeRepositoryCQRS implements both
		edgeWriter:        edgeRepo,
		categoryReader:    categoryRepo,  // CategoryRepositoryCQRS implements both
		categoryWriter:    categoryRepo,
		transactItems:     make([]types.TransactWriteItem, 0),
		pendingEvents:     make([]shared.DomainEvent, 0),
		trackedAggregates: make(map[string]shared.AggregateRoot),
	}, nil
}

// Begin starts a new transaction.
func (uow *DynamoDBUnitOfWorkClean) Begin(ctx context.Context) error {
	uow.mu.Lock()
	defer uow.mu.Unlock()
	
	if uow.isInTransaction {
		return fmt.Errorf("transaction already in progress")
	}
	
	uow.isInTransaction = true
	uow.isCommitted = false
	uow.isRolledBack = false
	uow.transactItems = make([]types.TransactWriteItem, 0)
	uow.pendingEvents = make([]shared.DomainEvent, 0)
	
	uow.logger.Debug("Unit of Work transaction started")
	return nil
}

// Commit commits the transaction.
func (uow *DynamoDBUnitOfWorkClean) Commit() error {
	ctx := context.Background() // Use background context for now
	uow.mu.Lock()
	defer uow.mu.Unlock()
	
	if !uow.isInTransaction {
		return fmt.Errorf("no transaction in progress")
	}
	
	if uow.isCommitted {
		return fmt.Errorf("transaction already committed")
	}
	
	if uow.isRolledBack {
		return fmt.Errorf("transaction already rolled back")
	}
	
	// Execute transactional writes if any
	if len(uow.transactItems) > 0 {
		input := &dynamodb.TransactWriteItemsInput{
			TransactItems: uow.transactItems,
		}
		
		if _, err := uow.client.TransactWriteItems(ctx, input); err != nil {
			uow.logger.Error("Failed to commit transaction", zap.Error(err))
			return fmt.Errorf("failed to commit transaction: %w", err)
		}
	}
	
	// Mark as committed
	uow.isCommitted = true
	uow.isInTransaction = false
	
	// Publish domain events after successful commit
	for _, event := range uow.pendingEvents {
		if err := uow.eventBus.Publish(ctx, event); err != nil {
			uow.logger.Error("Failed to publish domain event", 
				zap.String("eventType", event.EventType()),
				zap.Error(err))
			// Continue publishing other events even if one fails
		}
	}
	
	uow.logger.Debug("Unit of Work transaction committed",
		zap.Int("transactItems", len(uow.transactItems)),
		zap.Int("events", len(uow.pendingEvents)))
	
	return nil
}

// Rollback rolls back the transaction.
func (uow *DynamoDBUnitOfWorkClean) Rollback() error {
	uow.mu.Lock()
	defer uow.mu.Unlock()
	
	if !uow.isInTransaction {
		return fmt.Errorf("no transaction in progress")
	}
	
	if uow.isCommitted {
		return fmt.Errorf("transaction already committed")
	}
	
	if uow.isRolledBack {
		return fmt.Errorf("transaction already rolled back")
	}
	
	// Clear transactional items and events
	uow.transactItems = make([]types.TransactWriteItem, 0)
	uow.pendingEvents = make([]shared.DomainEvent, 0)
	uow.trackedAggregates = make(map[string]shared.AggregateRoot)
	
	// Mark as rolled back
	uow.isRolledBack = true
	uow.isInTransaction = false
	
	uow.logger.Debug("Unit of Work transaction rolled back")
	return nil
}

// NodeReader returns the node reader repository.
func (uow *DynamoDBUnitOfWorkClean) NodeReader() repository.NodeReader {
	return uow.nodeReader
}

// NodeWriter returns the node writer repository.
func (uow *DynamoDBUnitOfWorkClean) NodeWriter() repository.NodeWriter {
	if !uow.isInTransaction {
		uow.logger.Warn("NodeWriter accessed outside of transaction")
	}
	return &TransactionalNodeWriter{
		uow:  uow,
		base: uow.nodeWriter,
	}
}

// EdgeReader returns the edge reader repository.
func (uow *DynamoDBUnitOfWorkClean) EdgeReader() repository.EdgeReader {
	return uow.edgeReader
}

// EdgeWriter returns the edge writer repository.
func (uow *DynamoDBUnitOfWorkClean) EdgeWriter() repository.EdgeWriter {
	if !uow.isInTransaction {
		uow.logger.Warn("EdgeWriter accessed outside of transaction")
	}
	return &TransactionalEdgeWriter{
		uow:  uow,
		base: uow.edgeWriter,
	}
}

// CategoryReader returns the category reader repository.
func (uow *DynamoDBUnitOfWorkClean) CategoryReader() repository.CategoryReader {
	return uow.categoryReader
}

// CategoryWriter returns the category writer repository.
func (uow *DynamoDBUnitOfWorkClean) CategoryWriter() repository.CategoryWriter {
	if !uow.isInTransaction {
		uow.logger.Warn("CategoryWriter accessed outside of transaction")
	}
	return &TransactionalCategoryWriter{
		uow:  uow,
		base: uow.categoryWriter,
	}
}

// RegisterDomainEvent registers a domain event to be published after commit.
func (uow *DynamoDBUnitOfWorkClean) RegisterDomainEvent(event shared.DomainEvent) {
	uow.mu.Lock()
	defer uow.mu.Unlock()
	
	uow.pendingEvents = append(uow.pendingEvents, event)
}

// GetDomainEvents returns all pending domain events.
func (uow *DynamoDBUnitOfWorkClean) GetDomainEvents() []shared.DomainEvent {
	uow.mu.Lock()
	defer uow.mu.Unlock()
	
	events := make([]shared.DomainEvent, len(uow.pendingEvents))
	copy(events, uow.pendingEvents)
	return events
}

// Legacy accessor methods for backward compatibility with UnitOfWork interface

// Nodes returns the NodeRepository (combines reader and writer).
func (uow *DynamoDBUnitOfWorkClean) Nodes() repository.NodeRepository {
	// Create a combined repository that implements the full NodeRepository interface
	return NewNodeRepository(uow.client, uow.tableName, uow.indexName, uow.logger)
}

// Edges returns the EdgeRepository (combines reader and writer).
func (uow *DynamoDBUnitOfWorkClean) Edges() repository.EdgeRepository {
	// Return the EdgeRepositoryCQRS which implements EdgeRepository
	return NewEdgeRepositoryCQRS(uow.client, uow.tableName, uow.indexName, uow.logger)
}

// Categories returns the CategoryRepository (combines reader and writer).
func (uow *DynamoDBUnitOfWorkClean) Categories() repository.CategoryRepository {
	// Return the CategoryRepositoryCQRS which implements CategoryRepository
	return NewCategoryRepositoryCQRS(uow.client, uow.tableName, uow.indexName, uow.logger)
}

// NodeCategories returns the NodeCategoryRepository.
func (uow *DynamoDBUnitOfWorkClean) NodeCategories() repository.NodeCategoryRepository {
	// For now, return nil as we don't have a NodeCategoryRepository implementation
	// This would need to be implemented if actually used
	return nil
}

// GetPendingEvents returns pending domain events.
func (uow *DynamoDBUnitOfWorkClean) GetPendingEvents() []shared.DomainEvent {
	return uow.pendingEvents
}

// PublishEvent adds an event to be published on commit.
func (uow *DynamoDBUnitOfWorkClean) PublishEvent(event shared.DomainEvent) {
	uow.pendingEvents = append(uow.pendingEvents, event)
}

// IsActive checks if the unit of work is active.
func (uow *DynamoDBUnitOfWorkClean) IsActive() bool {
	return uow.isInTransaction && !uow.isCommitted && !uow.isRolledBack
}

// IsCommitted checks if the unit of work has been committed.
func (uow *DynamoDBUnitOfWorkClean) IsCommitted() bool {
	return uow.isCommitted
}

// IsRolledBack checks if the unit of work has been rolled back.
func (uow *DynamoDBUnitOfWorkClean) IsRolledBack() bool {
	return uow.isRolledBack
}

// Keywords returns the KeywordRepository.
func (uow *DynamoDBUnitOfWorkClean) Keywords() repository.KeywordRepository {
	// For now, return nil as we don't have a KeywordRepository implementation
	return nil
}

// Graph returns the GraphRepository.
func (uow *DynamoDBUnitOfWorkClean) Graph() repository.GraphRepository {
	// For now, return nil as we don't have a GraphRepository implementation
	return nil
}

// addTransactItem adds a transactional write item.
func (uow *DynamoDBUnitOfWorkClean) addTransactItem(item types.TransactWriteItem) {
	uow.mu.Lock()
	defer uow.mu.Unlock()
	
	uow.transactItems = append(uow.transactItems, item)
}

// TransactionalNodeWriter wraps NodeWriter for transactional operations.
type TransactionalNodeWriter struct {
	uow  *DynamoDBUnitOfWorkClean
	base repository.NodeWriter
}

// Save queues a node save operation.
func (w *TransactionalNodeWriter) Save(ctx context.Context, node *node.Node) error {
	// TODO: Convert node to DynamoDB item and add to transaction
	// For now, execute directly
	return w.base.Save(ctx, node)
}

// Update queues a node update operation.
func (w *TransactionalNodeWriter) Update(ctx context.Context, node *node.Node) error {
	// TODO: Convert to transactional item
	return w.base.Update(ctx, node)
}

// Delete queues a node delete operation.
func (w *TransactionalNodeWriter) Delete(ctx context.Context, id shared.NodeID) error {
	// TODO: Convert to transactional item
	return w.base.Delete(ctx, id)
}

// SaveBatch queues a batch save operation.
func (w *TransactionalNodeWriter) SaveBatch(ctx context.Context, nodes []*node.Node) error {
	// TODO: Convert to transactional items
	return w.base.SaveBatch(ctx, nodes)
}

// UpdateBatch queues a batch update operation.
func (w *TransactionalNodeWriter) UpdateBatch(ctx context.Context, nodes []*node.Node) error {
	// TODO: Convert to transactional items
	return w.base.UpdateBatch(ctx, nodes)
}

// DeleteBatch queues a batch delete operation.
func (w *TransactionalNodeWriter) DeleteBatch(ctx context.Context, ids []shared.NodeID) error {
	// TODO: Convert to transactional items
	return w.base.DeleteBatch(ctx, ids)
}

// Archive queues an archive operation.
func (w *TransactionalNodeWriter) Archive(ctx context.Context, id shared.NodeID) error {
	// TODO: Convert to transactional item
	return w.base.Archive(ctx, id)
}

// Unarchive queues an unarchive operation.
func (w *TransactionalNodeWriter) Unarchive(ctx context.Context, id shared.NodeID) error {
	// TODO: Convert to transactional item
	return w.base.Unarchive(ctx, id)
}

// UpdateVersion updates the version for optimistic locking.
func (w *TransactionalNodeWriter) UpdateVersion(ctx context.Context, id shared.NodeID, expectedVersion shared.Version) error {
	// TODO: Convert to transactional item with condition
	return w.base.UpdateVersion(ctx, id, expectedVersion)
}

// TransactionalEdgeWriter wraps EdgeWriter for transactional operations.
type TransactionalEdgeWriter struct {
	uow  *DynamoDBUnitOfWorkClean
	base repository.EdgeWriter
}

// Save queues an edge save operation.
func (w *TransactionalEdgeWriter) Save(ctx context.Context, edge *edge.Edge) error {
	// TODO: Convert to transactional item
	return w.base.Save(ctx, edge)
}

// SaveBatch queues a batch save operation.
func (w *TransactionalEdgeWriter) SaveBatch(ctx context.Context, edges []*edge.Edge) error {
	// TODO: Convert to transactional items
	return w.base.SaveBatch(ctx, edges)
}

// UpdateWeight updates the weight of an edge.
func (w *TransactionalEdgeWriter) UpdateWeight(ctx context.Context, id shared.NodeID, newWeight float64, expectedVersion shared.Version) error {
	// TODO: Convert to transactional item
	return w.base.UpdateWeight(ctx, id, newWeight, expectedVersion)
}

// Delete queues an edge delete operation.
func (w *TransactionalEdgeWriter) Delete(ctx context.Context, id shared.NodeID) error {
	// TODO: Convert to transactional item
	return w.base.Delete(ctx, id)
}

// DeleteBatch queues a batch delete operation.
func (w *TransactionalEdgeWriter) DeleteBatch(ctx context.Context, ids []shared.NodeID) error {
	// TODO: Convert to transactional items
	return w.base.DeleteBatch(ctx, ids)
}

// DeleteByNode deletes all edges for a node.
func (w *TransactionalEdgeWriter) DeleteByNode(ctx context.Context, nodeID shared.NodeID) error {
	// TODO: Query and delete in transaction
	return w.base.DeleteByNode(ctx, nodeID)
}

// SaveManyToOne saves multiple edges from one source.
func (w *TransactionalEdgeWriter) SaveManyToOne(ctx context.Context, sourceID shared.NodeID, targetIDs []shared.NodeID, weights []float64) error {
	// TODO: Convert to transactional items
	return w.base.SaveManyToOne(ctx, sourceID, targetIDs, weights)
}

// SaveOneToMany saves multiple edges to one target.
func (w *TransactionalEdgeWriter) SaveOneToMany(ctx context.Context, sourceIDs []shared.NodeID, targetID shared.NodeID, weights []float64) error {
	// TODO: Convert to transactional items
	return w.base.SaveOneToMany(ctx, sourceIDs, targetID, weights)
}

// TransactionalCategoryWriter wraps CategoryWriter for transactional operations.
type TransactionalCategoryWriter struct {
	uow  *DynamoDBUnitOfWorkClean
	base repository.CategoryWriter
}

// Save queues a category save operation.
func (w *TransactionalCategoryWriter) Save(ctx context.Context, category *category.Category) error {
	// TODO: Convert to transactional item
	return w.base.Save(ctx, category)
}

// SaveBatch queues a batch save operation.
func (w *TransactionalCategoryWriter) SaveBatch(ctx context.Context, categories []*category.Category) error {
	// TODO: Convert to transactional items
	return w.base.SaveBatch(ctx, categories)
}

// Update queues a category update operation.
func (w *TransactionalCategoryWriter) Update(ctx context.Context, category *category.Category) error {
	// TODO: Convert to transactional item
	return w.base.Update(ctx, category)
}

// UpdateBatch queues a batch update operation.
func (w *TransactionalCategoryWriter) UpdateBatch(ctx context.Context, categories []*category.Category) error {
	// TODO: Convert to transactional items
	return w.base.UpdateBatch(ctx, categories)
}

// Delete queues a category delete operation.
func (w *TransactionalCategoryWriter) Delete(ctx context.Context, userID string, categoryID string) error {
	// TODO: Convert to transactional item
	return w.base.Delete(ctx, userID, categoryID)
}

// DeleteBatch queues a batch delete operation.
func (w *TransactionalCategoryWriter) DeleteBatch(ctx context.Context, userID string, categoryIDs []string) error {
	// TODO: Convert to transactional items
	return w.base.DeleteBatch(ctx, userID, categoryIDs)
}

// DeleteHierarchy deletes a category and all its children.
func (w *TransactionalCategoryWriter) DeleteHierarchy(ctx context.Context, userID string, categoryID string) error {
	// TODO: Query and delete in transaction
	return w.base.DeleteHierarchy(ctx, userID, categoryID)
}

// CreateHierarchy creates a category hierarchy.
func (w *TransactionalCategoryWriter) CreateHierarchy(ctx context.Context, hierarchy category.CategoryHierarchy) error {
	// TODO: Convert to transactional item
	return w.base.CreateHierarchy(ctx, hierarchy)
}

// DeleteHierarchyRelation deletes a hierarchy relation.
func (w *TransactionalCategoryWriter) DeleteHierarchyRelation(ctx context.Context, userID string, parentID string, childID string) error {
	// TODO: Convert to transactional item
	return w.base.DeleteHierarchyRelation(ctx, userID, parentID, childID)
}

// AssignNodeToCategory assigns a node to a category.
func (w *TransactionalCategoryWriter) AssignNodeToCategory(ctx context.Context, mapping node.NodeCategory) error {
	// TODO: Convert to transactional item
	return w.base.AssignNodeToCategory(ctx, mapping)
}

// RemoveNodeFromCategory removes a node from a category.
func (w *TransactionalCategoryWriter) RemoveNodeFromCategory(ctx context.Context, userID string, nodeID string, categoryID string) error {
	// TODO: Convert to transactional item
	return w.base.RemoveNodeFromCategory(ctx, userID, nodeID, categoryID)
}

// BatchAssignNodes assigns multiple nodes to categories.
func (w *TransactionalCategoryWriter) BatchAssignNodes(ctx context.Context, mappings []node.NodeCategory) error {
	// TODO: Convert to transactional items
	return w.base.BatchAssignNodes(ctx, mappings)
}

// UpdateNoteCounts updates note counts for categories.
func (w *TransactionalCategoryWriter) UpdateNoteCounts(ctx context.Context, userID string) error {
	// TODO: Convert to transactional item
	return w.base.UpdateNoteCounts(ctx, userID)
}

// RecalculateHierarchy recalculates the category hierarchy.
func (w *TransactionalCategoryWriter) RecalculateHierarchy(ctx context.Context, userID string) error {
	// TODO: Complex operation in transaction
	return w.base.RecalculateHierarchy(ctx, userID)
}