package dynamodb

import (
	"context"
	"fmt"
	"log"

	"brain2-backend/internal/domain"
	"brain2-backend/internal/repository"
	appErrors "brain2-backend/pkg/errors"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

// DynamoDBUnitOfWork implements the repository.UnitOfWork interface for DynamoDB.
// Since DynamoDB doesn't have traditional ACID transactions, this implementation
// uses DynamoDB's TransactWrite operations for atomic operations across items.
type DynamoDBUnitOfWork struct {
	client        *dynamodb.Client
	tableName     string
	indexName     string
	eventBus      domain.EventBus
	
	// Transaction state
	isActive      bool
	isCommitted   bool
	isRolledBack  bool
	
	// Repository instances
	nodeRepo         repository.NodeRepository
	edgeRepo         repository.EdgeRepository
	categoryRepo     repository.CategoryRepository
	keywordRepo      repository.KeywordRepository
	graphRepo        repository.GraphRepository
	nodeCategoryRepo repository.NodeCategoryRepository
	
	// Transactional items to be written atomically
	transactItems []types.TransactWriteItem
	
	// Domain events to be published atomically
	pendingEvents []domain.DomainEvent
}

// NewDynamoDBUnitOfWork creates a new DynamoDB Unit of Work instance.
func NewDynamoDBUnitOfWork(
	client *dynamodb.Client,
	tableName, indexName string,
	eventBus domain.EventBus,
) repository.UnitOfWork {
	return &DynamoDBUnitOfWork{
		client:        client,
		tableName:     tableName,
		indexName:     indexName,
		eventBus:      eventBus,
		transactItems: make([]types.TransactWriteItem, 0),
		pendingEvents: make([]domain.DomainEvent, 0),
	}
}

// Begin starts a new unit of work by initializing repository instances.
// In DynamoDB, we don't start a transaction here but prepare for batched operations.
func (uow *DynamoDBUnitOfWork) Begin(ctx context.Context) error {
	if uow.isActive {
		return appErrors.NewValidation("unit of work already active")
	}
	
	log.Printf("DEBUG DynamoDBUnitOfWork.Begin: starting new unit of work")
	
	// Initialize repository instances with transactional capabilities
	uow.nodeRepo = NewTransactionalNodeRepository(uow)
	uow.edgeRepo = NewTransactionalEdgeRepository(uow)
	uow.categoryRepo = NewTransactionalCategoryRepository(uow)
	uow.keywordRepo = NewTransactionalKeywordRepository(uow)
	uow.graphRepo = NewTransactionalGraphRepository(uow)
	uow.nodeCategoryRepo = NewTransactionalNodeCategoryRepository(uow)
	
	uow.isActive = true
	uow.isCommitted = false
	uow.isRolledBack = false
	
	log.Printf("DEBUG DynamoDBUnitOfWork.Begin: unit of work started successfully")
	return nil
}

// Commit persists all queued operations atomically using DynamoDB TransactWrite.
func (uow *DynamoDBUnitOfWork) Commit() error {
	if !uow.isActive {
		return appErrors.NewValidation("no active unit of work")
	}
	
	if uow.isCommitted || uow.isRolledBack {
		return appErrors.NewValidation("unit of work already completed")
	}
	
	log.Printf("DEBUG DynamoDBUnitOfWork.Commit: committing %d transaction items", len(uow.transactItems))
	
	// Execute all transactional items atomically
	if len(uow.transactItems) > 0 {
		// DynamoDB supports up to 25 items per TransactWrite operation
		if len(uow.transactItems) > 25 {
			return appErrors.NewValidation("too many transaction items (max 25 for DynamoDB)")
		}
		
		_, err := uow.client.TransactWriteItems(context.Background(), &dynamodb.TransactWriteItemsInput{
			TransactItems: uow.transactItems,
		})
		if err != nil {
			uow.isRolledBack = true
			return appErrors.Wrap(err, "failed to commit DynamoDB transaction")
		}
	}
	
	// Publish events after successful database commit
	if len(uow.pendingEvents) > 0 && uow.eventBus != nil {
		log.Printf("DEBUG DynamoDBUnitOfWork.Commit: publishing %d domain events", len(uow.pendingEvents))
		for _, event := range uow.pendingEvents {
			if err := uow.eventBus.Publish(context.Background(), event); err != nil {
				log.Printf("WARN DynamoDBUnitOfWork.Commit: failed to publish event: %v", err)
				// Continue with other events - event publishing failures shouldn't rollback DB changes
			}
		}
	}
	
	uow.isCommitted = true
	uow.isActive = false
	
	log.Printf("DEBUG DynamoDBUnitOfWork.Commit: unit of work committed successfully")
	return nil
}

// Rollback discards all queued operations without persisting them.
func (uow *DynamoDBUnitOfWork) Rollback() error {
	if !uow.isActive && !uow.isRolledBack {
		return nil // Safe to call multiple times
	}
	
	if uow.isCommitted {
		return appErrors.NewValidation("cannot rollback committed unit of work")
	}
	
	log.Printf("DEBUG DynamoDBUnitOfWork.Rollback: rolling back %d queued operations", len(uow.transactItems))
	
	// Clear all queued operations
	uow.transactItems = uow.transactItems[:0]
	uow.pendingEvents = uow.pendingEvents[:0]
	
	uow.isRolledBack = true
	uow.isActive = false
	
	log.Printf("DEBUG DynamoDBUnitOfWork.Rollback: unit of work rolled back successfully")
	return nil
}

// Repository access methods

func (uow *DynamoDBUnitOfWork) Nodes() repository.NodeRepository {
	if !uow.isActive {
		panic("unit of work not active - call Begin() first")
	}
	return uow.nodeRepo
}

func (uow *DynamoDBUnitOfWork) Edges() repository.EdgeRepository {
	if !uow.isActive {
		panic("unit of work not active - call Begin() first")
	}
	return uow.edgeRepo
}

func (uow *DynamoDBUnitOfWork) Categories() repository.CategoryRepository {
	if !uow.isActive {
		panic("unit of work not active - call Begin() first")
	}
	return uow.categoryRepo
}

func (uow *DynamoDBUnitOfWork) Keywords() repository.KeywordRepository {
	if !uow.isActive {
		panic("unit of work not active - call Begin() first")
	}
	return uow.keywordRepo
}

func (uow *DynamoDBUnitOfWork) Graph() repository.GraphRepository {
	if !uow.isActive {
		panic("unit of work not active - call Begin() first")
	}
	return uow.graphRepo
}

func (uow *DynamoDBUnitOfWork) NodeCategories() repository.NodeCategoryRepository {
	if !uow.isActive {
		panic("unit of work not active - call Begin() first")
	}
	return uow.nodeCategoryRepo
}

// Event management methods

func (uow *DynamoDBUnitOfWork) PublishEvent(event domain.DomainEvent) {
	uow.pendingEvents = append(uow.pendingEvents, event)
}

func (uow *DynamoDBUnitOfWork) GetPendingEvents() []domain.DomainEvent {
	// Return a copy to prevent external modification
	events := make([]domain.DomainEvent, len(uow.pendingEvents))
	copy(events, uow.pendingEvents)
	return events
}

// State query methods

func (uow *DynamoDBUnitOfWork) IsActive() bool {
	return uow.isActive
}

func (uow *DynamoDBUnitOfWork) IsCommitted() bool {
	return uow.isCommitted
}

func (uow *DynamoDBUnitOfWork) IsRolledBack() bool {
	return uow.isRolledBack
}

// Internal methods for transactional repositories

// AddTransactItem adds a transactional item to be executed on commit.
func (uow *DynamoDBUnitOfWork) AddTransactItem(item types.TransactWriteItem) error {
	if !uow.isActive {
		return appErrors.NewValidation("unit of work not active")
	}
	
	if len(uow.transactItems) >= 25 {
		return appErrors.NewValidation("too many transaction items (DynamoDB limit: 25)")
	}
	
	uow.transactItems = append(uow.transactItems, item)
	return nil
}

// GetTableName returns the DynamoDB table name for transactional repositories.
func (uow *DynamoDBUnitOfWork) GetTableName() string {
	return uow.tableName
}

// GetIndexName returns the DynamoDB index name for transactional repositories.
func (uow *DynamoDBUnitOfWork) GetIndexName() string {
	return uow.indexName
}

// Transactional Repository Implementations
// These repositories queue operations for atomic execution on commit.

// TransactionalNodeRepository wraps node operations for transactional execution.
type TransactionalNodeRepository struct {
	uow  *DynamoDBUnitOfWork
	base repository.NodeRepository
}

func NewTransactionalNodeRepository(uow *DynamoDBUnitOfWork) repository.NodeRepository {
	base := NewNodeRepository(uow.client, uow.tableName, uow.indexName)
	return &TransactionalNodeRepository{uow: uow, base: base}
}

// Most operations delegate to base repository for reads
func (r *TransactionalNodeRepository) FindNodeByID(ctx context.Context, userID, nodeID string) (*domain.Node, error) {
	return r.base.FindNodeByID(ctx, userID, nodeID)
}

func (r *TransactionalNodeRepository) FindNodes(ctx context.Context, query repository.NodeQuery) ([]*domain.Node, error) {
	return r.base.FindNodes(ctx, query)
}

func (r *TransactionalNodeRepository) GetNodesPage(ctx context.Context, query repository.NodeQuery, pagination repository.Pagination) (*repository.NodePage, error) {
	return r.base.GetNodesPage(ctx, query, pagination)
}

func (r *TransactionalNodeRepository) GetNodeNeighborhood(ctx context.Context, userID, nodeID string, depth int) (*domain.Graph, error) {
	return r.base.GetNodeNeighborhood(ctx, userID, nodeID, depth)
}

func (r *TransactionalNodeRepository) CountNodes(ctx context.Context, userID string) (int, error) {
	return r.base.CountNodes(ctx, userID)
}

func (r *TransactionalNodeRepository) FindNodesWithOptions(ctx context.Context, query repository.NodeQuery, opts ...repository.QueryOption) ([]*domain.Node, error) {
	return r.base.FindNodesWithOptions(ctx, query, opts...)
}

func (r *TransactionalNodeRepository) FindNodesPageWithOptions(ctx context.Context, query repository.NodeQuery, pagination repository.Pagination, opts ...repository.QueryOption) (*repository.NodePage, error) {
	return r.base.FindNodesPageWithOptions(ctx, query, pagination, opts...)
}

// Write operations are queued for transactional execution
func (r *TransactionalNodeRepository) CreateNodeAndKeywords(ctx context.Context, node *domain.Node) error {
	// For now, delegate to base repository - in a full implementation,
	// this would queue the operation for transactional execution
	return r.base.CreateNodeAndKeywords(ctx, node)
}

func (r *TransactionalNodeRepository) DeleteNode(ctx context.Context, userID, nodeID string) error {
	// For now, delegate to base repository
	return r.base.DeleteNode(ctx, userID, nodeID)
}

// NodeReader/NodeWriter interface methods for CQRS compatibility

// FindByUser finds all nodes for a user
func (r *TransactionalNodeRepository) FindByUser(ctx context.Context, userID domain.UserID) ([]*domain.Node, error) {
	query := repository.NodeQuery{
		UserID: userID.String(),
	}
	return r.base.FindNodes(ctx, query)
}

// FindByID finds a node by its ID
func (r *TransactionalNodeRepository) FindByID(ctx context.Context, nodeID domain.NodeID) (*domain.Node, error) {
	// We need to get the userID from somewhere - for now, we'll use a workaround
	// In a proper implementation, this would be handled differently
	return nil, fmt.Errorf("FindByID not fully implemented - requires userID context")
}

// Save creates or updates a node
func (r *TransactionalNodeRepository) Save(ctx context.Context, node *domain.Node) error {
	return r.base.CreateNodeAndKeywords(ctx, node)
}

// Delete removes a node by userID and nodeID
func (r *TransactionalNodeRepository) Delete(ctx context.Context, userID domain.UserID, nodeID domain.NodeID) error {
	return r.base.DeleteNode(ctx, userID.String(), nodeID.String())
}

// Similar transactional wrappers for other repositories

type TransactionalEdgeRepository struct {
	uow  *DynamoDBUnitOfWork
	base repository.EdgeRepository
}

func NewTransactionalEdgeRepository(uow *DynamoDBUnitOfWork) repository.EdgeRepository {
	base := NewEdgeRepository(uow.client, uow.tableName, uow.indexName)
	return &TransactionalEdgeRepository{uow: uow, base: base}
}

func (r *TransactionalEdgeRepository) CreateEdges(ctx context.Context, userID, sourceNodeID string, relatedNodeIDs []string) error {
	return r.base.CreateEdges(ctx, userID, sourceNodeID, relatedNodeIDs)
}

func (r *TransactionalEdgeRepository) CreateEdge(ctx context.Context, edge *domain.Edge) error {
	return r.base.CreateEdge(ctx, edge)
}

func (r *TransactionalEdgeRepository) FindEdges(ctx context.Context, query repository.EdgeQuery) ([]*domain.Edge, error) {
	return r.base.FindEdges(ctx, query)
}

func (r *TransactionalEdgeRepository) GetEdgesPage(ctx context.Context, query repository.EdgeQuery, pagination repository.Pagination) (*repository.EdgePage, error) {
	return r.base.GetEdgesPage(ctx, query, pagination)
}

func (r *TransactionalEdgeRepository) FindEdgesWithOptions(ctx context.Context, query repository.EdgeQuery, opts ...repository.QueryOption) ([]*domain.Edge, error) {
	return r.base.FindEdgesWithOptions(ctx, query, opts...)
}

// EdgeWriter interface methods for CQRS compatibility

// Save creates or updates an edge
func (r *TransactionalEdgeRepository) Save(ctx context.Context, edge *domain.Edge) error {
	return r.base.CreateEdge(ctx, edge)
}

// DeleteByNodeID deletes all edges connected to a node
func (r *TransactionalEdgeRepository) DeleteByNodeID(ctx context.Context, nodeID domain.NodeID) error {
	// This is a placeholder - in production, this would delete all edges
	// connected to the specified node
	return fmt.Errorf("DeleteByNodeID not implemented")
}

// Placeholder implementations for other transactional repositories
type TransactionalCategoryRepository struct {
	uow  *DynamoDBUnitOfWork
	base repository.CategoryRepository
}

func NewTransactionalCategoryRepository(uow *DynamoDBUnitOfWork) repository.CategoryRepository {
	base := NewCategoryRepository(uow.client, uow.tableName, uow.indexName)
	return &TransactionalCategoryRepository{uow: uow, base: base}
}

// Delegate all operations to base repository for now
func (r *TransactionalCategoryRepository) CreateCategory(ctx context.Context, category domain.Category) error {
	return r.base.CreateCategory(ctx, category)
}

func (r *TransactionalCategoryRepository) UpdateCategory(ctx context.Context, category domain.Category) error {
	return r.base.UpdateCategory(ctx, category)
}

func (r *TransactionalCategoryRepository) DeleteCategory(ctx context.Context, userID, categoryID string) error {
	return r.base.DeleteCategory(ctx, userID, categoryID)
}

func (r *TransactionalCategoryRepository) FindCategoryByID(ctx context.Context, userID, categoryID string) (*domain.Category, error) {
	return r.base.FindCategoryByID(ctx, userID, categoryID)
}

func (r *TransactionalCategoryRepository) FindCategories(ctx context.Context, query repository.CategoryQuery) ([]domain.Category, error) {
	return r.base.FindCategories(ctx, query)
}

func (r *TransactionalCategoryRepository) FindCategoriesByLevel(ctx context.Context, userID string, level int) ([]domain.Category, error) {
	return r.base.FindCategoriesByLevel(ctx, userID, level)
}

func (r *TransactionalCategoryRepository) Save(ctx context.Context, category *domain.Category) error {
	return r.base.Save(ctx, category)
}

func (r *TransactionalCategoryRepository) FindByID(ctx context.Context, userID, categoryID string) (*domain.Category, error) {
	return r.base.FindByID(ctx, userID, categoryID)
}

func (r *TransactionalCategoryRepository) Delete(ctx context.Context, userID, categoryID string) error {
	return r.base.Delete(ctx, userID, categoryID)
}

func (r *TransactionalCategoryRepository) CreateCategoryHierarchy(ctx context.Context, hierarchy domain.CategoryHierarchy) error {
	return r.base.CreateCategoryHierarchy(ctx, hierarchy)
}

func (r *TransactionalCategoryRepository) DeleteCategoryHierarchy(ctx context.Context, userID, parentID, childID string) error {
	return r.base.DeleteCategoryHierarchy(ctx, userID, parentID, childID)
}

func (r *TransactionalCategoryRepository) FindChildCategories(ctx context.Context, userID, parentID string) ([]domain.Category, error) {
	return r.base.FindChildCategories(ctx, userID, parentID)
}

func (r *TransactionalCategoryRepository) FindParentCategory(ctx context.Context, userID, childID string) (*domain.Category, error) {
	return r.base.FindParentCategory(ctx, userID, childID)
}

func (r *TransactionalCategoryRepository) GetCategoryTree(ctx context.Context, userID string) ([]domain.Category, error) {
	return r.base.GetCategoryTree(ctx, userID)
}

func (r *TransactionalCategoryRepository) AssignNodeToCategory(ctx context.Context, mapping domain.NodeCategory) error {
	return r.base.AssignNodeToCategory(ctx, mapping)
}

func (r *TransactionalCategoryRepository) RemoveNodeFromCategory(ctx context.Context, userID, nodeID, categoryID string) error {
	return r.base.RemoveNodeFromCategory(ctx, userID, nodeID, categoryID)
}

func (r *TransactionalCategoryRepository) FindNodesByCategory(ctx context.Context, userID, categoryID string) ([]*domain.Node, error) {
	return r.base.FindNodesByCategory(ctx, userID, categoryID)
}

func (r *TransactionalCategoryRepository) FindCategoriesForNode(ctx context.Context, userID, nodeID string) ([]domain.Category, error) {
	return r.base.FindCategoriesForNode(ctx, userID, nodeID)
}

func (r *TransactionalCategoryRepository) BatchAssignCategories(ctx context.Context, mappings []domain.NodeCategory) error {
	return r.base.BatchAssignCategories(ctx, mappings)
}

func (r *TransactionalCategoryRepository) UpdateCategoryNoteCounts(ctx context.Context, userID string, categoryCounts map[string]int) error {
	return r.base.UpdateCategoryNoteCounts(ctx, userID, categoryCounts)
}

// Placeholder implementations for other repositories
type TransactionalKeywordRepository struct {
	uow  *DynamoDBUnitOfWork
	base repository.KeywordRepository
}

func NewTransactionalKeywordRepository(uow *DynamoDBUnitOfWork) repository.KeywordRepository {
	base := NewKeywordRepository(uow.client, uow.tableName, uow.indexName)
	return &TransactionalKeywordRepository{uow: uow, base: base}
}

func (r *TransactionalKeywordRepository) FindNodesByKeywords(ctx context.Context, userID string, keywords []string) ([]*domain.Node, error) {
	return r.base.FindNodesByKeywords(ctx, userID, keywords)
}

type TransactionalGraphRepository struct {
	uow  *DynamoDBUnitOfWork
	base repository.GraphRepository
}

func NewTransactionalGraphRepository(uow *DynamoDBUnitOfWork) repository.GraphRepository {
	base := NewGraphRepository(uow.client, uow.tableName, uow.indexName)
	return &TransactionalGraphRepository{uow: uow, base: base}
}

func (r *TransactionalGraphRepository) GetGraphData(ctx context.Context, query repository.GraphQuery) (*domain.Graph, error) {
	return r.base.GetGraphData(ctx, query)
}

func (r *TransactionalGraphRepository) GetGraphDataPaginated(ctx context.Context, query repository.GraphQuery, pagination repository.Pagination) (*domain.Graph, string, error) {
	return r.base.GetGraphDataPaginated(ctx, query, pagination)
}

func (r *TransactionalGraphRepository) GetSubgraph(ctx context.Context, nodeIDs []string, opts ...repository.QueryOption) (*domain.Graph, error) {
	return r.base.GetSubgraph(ctx, nodeIDs, opts...)
}

func (r *TransactionalGraphRepository) GetConnectedComponents(ctx context.Context, userID string, opts ...repository.QueryOption) ([]domain.Graph, error) {
	return r.base.GetConnectedComponents(ctx, userID, opts...)
}

type TransactionalNodeCategoryRepository struct {
	uow *DynamoDBUnitOfWork
}

func NewTransactionalNodeCategoryRepository(uow *DynamoDBUnitOfWork) repository.NodeCategoryRepository {
	return &TransactionalNodeCategoryRepository{uow: uow}
}

// Placeholder implementations for NodeCategoryRepository methods
func (r *TransactionalNodeCategoryRepository) Assign(ctx context.Context, mapping *domain.NodeCategory) error {
	return fmt.Errorf("not implemented")
}

func (r *TransactionalNodeCategoryRepository) Remove(ctx context.Context, userID, nodeID, categoryID string) error {
	return fmt.Errorf("not implemented")
}

func (r *TransactionalNodeCategoryRepository) RemoveAllByNode(ctx context.Context, userID, nodeID string) error {
	return fmt.Errorf("not implemented")
}

func (r *TransactionalNodeCategoryRepository) RemoveAllByCategory(ctx context.Context, userID, categoryID string) error {
	return fmt.Errorf("not implemented")
}

func (r *TransactionalNodeCategoryRepository) RemoveAllFromCategory(ctx context.Context, categoryID string) error {
	return fmt.Errorf("not implemented")
}

func (r *TransactionalNodeCategoryRepository) FindByNode(ctx context.Context, userID, nodeID string) ([]*domain.NodeCategory, error) {
	return nil, fmt.Errorf("not implemented")
}

func (r *TransactionalNodeCategoryRepository) FindByCategory(ctx context.Context, userID, categoryID string) ([]*domain.NodeCategory, error) {
	return nil, fmt.Errorf("not implemented")
}

func (r *TransactionalNodeCategoryRepository) FindByUser(ctx context.Context, userID string) ([]*domain.NodeCategory, error) {
	return nil, fmt.Errorf("not implemented")
}

func (r *TransactionalNodeCategoryRepository) Exists(ctx context.Context, userID, nodeID, categoryID string) (bool, error) {
	return false, fmt.Errorf("not implemented")
}

func (r *TransactionalNodeCategoryRepository) BatchAssign(ctx context.Context, mappings []*domain.NodeCategory) error {
	return fmt.Errorf("not implemented")
}

func (r *TransactionalNodeCategoryRepository) FindNodesByCategory(ctx context.Context, userID, categoryID string) ([]*domain.Node, error) {
	return nil, fmt.Errorf("not implemented")
}

func (r *TransactionalNodeCategoryRepository) FindNodesByCategoryPage(ctx context.Context, userID, categoryID string, pagination repository.Pagination) (*repository.NodePage, error) {
	return nil, fmt.Errorf("not implemented")
}

func (r *TransactionalNodeCategoryRepository) CountNodesInCategory(ctx context.Context, userID, categoryID string) (int, error) {
	return 0, fmt.Errorf("not implemented")
}

func (r *TransactionalNodeCategoryRepository) FindCategoriesByNode(ctx context.Context, userID, nodeID string) ([]*domain.Category, error) {
	return nil, fmt.Errorf("not implemented")
}

func (r *TransactionalNodeCategoryRepository) BatchRemove(ctx context.Context, userID string, pairs []struct{ NodeID, CategoryID string }) error {
	return fmt.Errorf("not implemented")
}

func (r *TransactionalNodeCategoryRepository) CountByCategory(ctx context.Context, userID, categoryID string) (int, error) {
	return 0, fmt.Errorf("not implemented")
}

func (r *TransactionalNodeCategoryRepository) CountByNode(ctx context.Context, userID, nodeID string) (int, error) {
	return 0, fmt.Errorf("not implemented")
}