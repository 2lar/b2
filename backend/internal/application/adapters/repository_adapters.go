package adapters

import (
	"context"
	"fmt"

	"brain2-backend/internal/domain"
	"brain2-backend/internal/repository"
)

// NodeRepositoryAdapter bridges the gap between CQRS services and existing repository interfaces
type NodeRepositoryAdapter interface {
	// Application Service Methods (new CQRS patterns)
	Save(ctx context.Context, node *domain.Node) error
	GetByID(ctx context.Context, userID domain.UserID, nodeID domain.NodeID) (*domain.Node, error)
	FindByID(ctx context.Context, nodeID domain.NodeID) (*domain.Node, error)
	FindByUser(ctx context.Context, userID domain.UserID) ([]*domain.Node, error)
	Delete(ctx context.Context, userID domain.UserID, nodeID domain.NodeID) error
	
	// Query Methods for CQRS read models
	GetNodesForUser(ctx context.Context, userID domain.UserID, limit, offset int) ([]*domain.Node, error)
	GetConnectedNodes(ctx context.Context, userID domain.UserID, nodeID domain.NodeID) ([]*domain.Node, error)
}

// CategoryRepositoryAdapter bridges category operations between CQRS and existing interfaces
type CategoryRepositoryAdapter interface {
	// Application Service Methods (new CQRS patterns)
	Save(ctx context.Context, category *domain.Category) error
	GetByID(ctx context.Context, userID domain.UserID, categoryID domain.CategoryID) (*domain.Category, error)
	FindByID(ctx context.Context, userID, categoryID string) (*domain.Category, error)
	Delete(ctx context.Context, userID domain.UserID, categoryID domain.CategoryID) error
	
	// Query Methods for CQRS read models
	GetCategoriesForUser(ctx context.Context, userID domain.UserID) ([]*domain.Category, error)
	AssignNodeToCategory(ctx context.Context, userID domain.UserID, nodeID domain.NodeID, categoryID domain.CategoryID) error
	RemoveNodeFromCategory(ctx context.Context, userID domain.UserID, nodeID domain.NodeID, categoryID domain.CategoryID) error
}

// GraphRepositoryAdapter provides graph operations for CQRS queries
type GraphRepositoryAdapter interface {
	GetGraphForUser(ctx context.Context, userID domain.UserID) (*domain.Graph, error)
	GetSubGraph(ctx context.Context, userID domain.UserID, nodeIDs []domain.NodeID) (*domain.Graph, error)
}

// nodeRepositoryAdapter implements NodeRepositoryAdapter using existing repositories
type nodeRepositoryAdapter struct {
	nodeRepo        repository.NodeRepository
	transactionalRepo repository.TransactionalRepository
}

// NewNodeRepositoryAdapter creates a new node repository adapter
func NewNodeRepositoryAdapter(nodeRepo repository.NodeRepository, transactionalRepo repository.TransactionalRepository) NodeRepositoryAdapter {
	return &nodeRepositoryAdapter{
		nodeRepo:        nodeRepo,
		transactionalRepo: transactionalRepo,
	}
}

// Save creates or updates a node using existing repository interface
func (a *nodeRepositoryAdapter) Save(ctx context.Context, node *domain.Node) error {
	return a.nodeRepo.CreateNodeAndKeywords(ctx, node)
}

// GetByID retrieves a node by ID, converting types as needed
func (a *nodeRepositoryAdapter) GetByID(ctx context.Context, userID domain.UserID, nodeID domain.NodeID) (*domain.Node, error) {
	return a.nodeRepo.FindNodeByID(ctx, userID.String(), nodeID.String())
}

// Delete removes a node using existing repository interface
func (a *nodeRepositoryAdapter) Delete(ctx context.Context, userID domain.UserID, nodeID domain.NodeID) error {
	return a.nodeRepo.DeleteNode(ctx, userID.String(), nodeID.String())
}

// GetNodesForUser retrieves nodes for a user with pagination
func (a *nodeRepositoryAdapter) GetNodesForUser(ctx context.Context, userID domain.UserID, limit, offset int) ([]*domain.Node, error) {
	query := repository.NodeQuery{
		UserID: userID.String(),
	}
	pagination := repository.Pagination{
		Limit:  limit,
		Offset: offset,
	}
	
	page, err := a.nodeRepo.GetNodesPage(ctx, query, pagination)
	if err != nil {
		return nil, err
	}
	
	return page.Items, nil
}

// FindByID retrieves a node by ID without user verification (for internal use)
func (a *nodeRepositoryAdapter) FindByID(ctx context.Context, nodeID domain.NodeID) (*domain.Node, error) {
	// This is a simplified implementation that might need user context
	// For now, we'll use an empty userID - this may need to be enhanced based on repository implementation
	return a.nodeRepo.FindNodeByID(ctx, "", nodeID.String())
}

// FindByUser retrieves all nodes for a user
func (a *nodeRepositoryAdapter) FindByUser(ctx context.Context, userID domain.UserID) ([]*domain.Node, error) {
	query := repository.NodeQuery{
		UserID: userID.String(),
	}
	
	return a.nodeRepo.FindNodes(ctx, query)
}

// GetConnectedNodes retrieves nodes connected to a specific node
func (a *nodeRepositoryAdapter) GetConnectedNodes(ctx context.Context, userID domain.UserID, nodeID domain.NodeID) ([]*domain.Node, error) {
	graph, err := a.nodeRepo.GetNodeNeighborhood(ctx, userID.String(), nodeID.String(), 1)
	if err != nil {
		return nil, err
	}
	
	return graph.Nodes, nil
}

// categoryRepositoryAdapter implements CategoryRepositoryAdapter using existing repositories
type categoryRepositoryAdapter struct {
	categoryRepo repository.CategoryRepository
}

// NewCategoryRepositoryAdapter creates a new category repository adapter
func NewCategoryRepositoryAdapter(categoryRepo repository.CategoryRepository) CategoryRepositoryAdapter {
	return &categoryRepositoryAdapter{
		categoryRepo: categoryRepo,
	}
}

// Save creates or updates a category using existing repository interface
func (a *categoryRepositoryAdapter) Save(ctx context.Context, category *domain.Category) error {
	return a.categoryRepo.CreateCategory(ctx, *category)
}

// GetByID retrieves a category by ID, converting types as needed
func (a *categoryRepositoryAdapter) GetByID(ctx context.Context, userID domain.UserID, categoryID domain.CategoryID) (*domain.Category, error) {
	return a.categoryRepo.FindCategoryByID(ctx, userID.String(), string(categoryID))
}

// FindByID retrieves a category by ID using string parameters
func (a *categoryRepositoryAdapter) FindByID(ctx context.Context, userID, categoryID string) (*domain.Category, error) {
	return a.categoryRepo.FindCategoryByID(ctx, userID, categoryID)
}

// Delete removes a category using existing repository interface
func (a *categoryRepositoryAdapter) Delete(ctx context.Context, userID domain.UserID, categoryID domain.CategoryID) error {
	return a.categoryRepo.DeleteCategory(ctx, userID.String(), string(categoryID))
}

// GetCategoriesForUser retrieves all categories for a user
func (a *categoryRepositoryAdapter) GetCategoriesForUser(ctx context.Context, userID domain.UserID) ([]*domain.Category, error) {
	query := repository.CategoryQuery{
		UserID: userID.String(),
	}
	
	categories, err := a.categoryRepo.FindCategories(ctx, query)
	if err != nil {
		return nil, err
	}
	
	// Convert slice of Category to slice of *Category
	result := make([]*domain.Category, len(categories))
	for i := range categories {
		result[i] = &categories[i]
	}
	
	return result, nil
}

// AssignNodeToCategory assigns a node to a category
func (a *categoryRepositoryAdapter) AssignNodeToCategory(ctx context.Context, userID domain.UserID, nodeID domain.NodeID, categoryID domain.CategoryID) error {
	mapping := domain.NodeCategory{
		UserID:     userID.String(),
		NodeID:     nodeID.String(),
		CategoryID: string(categoryID),
	}
	
	return a.categoryRepo.AssignNodeToCategory(ctx, mapping)
}

// RemoveNodeFromCategory removes a node from a category
func (a *categoryRepositoryAdapter) RemoveNodeFromCategory(ctx context.Context, userID domain.UserID, nodeID domain.NodeID, categoryID domain.CategoryID) error {
	return a.categoryRepo.RemoveNodeFromCategory(ctx, userID.String(), nodeID.String(), string(categoryID))
}

// graphRepositoryAdapter implements GraphRepositoryAdapter using existing repositories
type graphRepositoryAdapter struct {
	graphRepo repository.GraphRepository
}

// NewGraphRepositoryAdapter creates a new graph repository adapter
func NewGraphRepositoryAdapter(graphRepo repository.GraphRepository) GraphRepositoryAdapter {
	return &graphRepositoryAdapter{
		graphRepo: graphRepo,
	}
}

// GetGraphForUser retrieves the complete graph for a user
func (a *graphRepositoryAdapter) GetGraphForUser(ctx context.Context, userID domain.UserID) (*domain.Graph, error) {
	query := repository.GraphQuery{
		UserID: userID.String(),
	}
	
	return a.graphRepo.GetGraphData(ctx, query)
}

// GetSubGraph retrieves a subgraph containing specific nodes
func (a *graphRepositoryAdapter) GetSubGraph(ctx context.Context, userID domain.UserID, nodeIDs []domain.NodeID) (*domain.Graph, error) {
	// Convert NodeIDs to strings
	nodeIDStrings := make([]string, len(nodeIDs))
	for i, nodeID := range nodeIDs {
		nodeIDStrings[i] = nodeID.String()
	}
	
	return a.graphRepo.GetSubgraph(ctx, nodeIDStrings)
}

// EdgeRepositoryAdapter provides edge operations for CQRS services
type EdgeRepositoryAdapter interface {
	Save(ctx context.Context, edge *domain.Edge) error
	DeleteByNodeID(ctx context.Context, nodeID domain.NodeID) error
}

// NodeCategoryRepositoryAdapter provides node-category operations for CQRS services
type NodeCategoryRepositoryAdapter interface {
	Assign(ctx context.Context, mapping *domain.NodeCategory) error
	Remove(ctx context.Context, userID, nodeID, categoryID string) error
	RemoveAllFromCategory(ctx context.Context, categoryID string) error
	Save(ctx context.Context, mapping *domain.NodeCategory) error
}

// UnitOfWorkAdapter provides CQRS-compatible Unit of Work interface
type UnitOfWorkAdapter interface {
	Begin(ctx context.Context) error
	Commit() error
	Rollback() error
	
	Nodes() NodeRepositoryAdapter
	Edges() EdgeRepositoryAdapter
	Categories() CategoryRepositoryAdapter
	Graph() GraphRepositoryAdapter
	NodeCategories() NodeCategoryRepositoryAdapter
	
	PublishEvent(event domain.DomainEvent)
}

// edgeRepositoryAdapter implements EdgeRepositoryAdapter using existing repository
type edgeRepositoryAdapter struct {
	edgeRepo repository.EdgeRepository
}

// NewEdgeRepositoryAdapter creates a new edge repository adapter
func NewEdgeRepositoryAdapter(edgeRepo repository.EdgeRepository) EdgeRepositoryAdapter {
	return &edgeRepositoryAdapter{
		edgeRepo: edgeRepo,
	}
}

// Save creates an edge using existing repository interface
func (a *edgeRepositoryAdapter) Save(ctx context.Context, edge *domain.Edge) error {
	return a.edgeRepo.CreateEdge(ctx, edge)
}

// DeleteByNodeID deletes all edges associated with a node
func (a *edgeRepositoryAdapter) DeleteByNodeID(ctx context.Context, nodeID domain.NodeID) error {
	// For now, we'll delegate to the underlying repository
	// In a full implementation, this would check for a DeleteByNodeID method
	// or implement bulk deletion logic
	
	// Since the EdgeRepository interface doesn't have a DeleteByNodeID method,
	// we would need to extend it or implement the logic here
	// For now, return a not implemented error to signal this needs proper implementation
	return fmt.Errorf("DeleteByNodeID not yet implemented in EdgeRepositoryAdapter")
}

// nodeCategoryRepositoryAdapter implements NodeCategoryRepositoryAdapter
type nodeCategoryRepositoryAdapter struct {
	nodeCategoryRepo repository.NodeCategoryRepository
}

// NewNodeCategoryRepositoryAdapter creates a new node category repository adapter
func NewNodeCategoryRepositoryAdapter(nodeCategoryRepo repository.NodeCategoryRepository) NodeCategoryRepositoryAdapter {
	return &nodeCategoryRepositoryAdapter{
		nodeCategoryRepo: nodeCategoryRepo,
	}
}

// Assign creates a node-category mapping
func (a *nodeCategoryRepositoryAdapter) Assign(ctx context.Context, mapping *domain.NodeCategory) error {
	return a.nodeCategoryRepo.Assign(ctx, mapping)
}

// Remove removes a node-category mapping
func (a *nodeCategoryRepositoryAdapter) Remove(ctx context.Context, userID, nodeID, categoryID string) error {
	return a.nodeCategoryRepo.Remove(ctx, userID, nodeID, categoryID)
}

// RemoveAllFromCategory removes all node-category mappings for a specific category
func (a *nodeCategoryRepositoryAdapter) RemoveAllFromCategory(ctx context.Context, categoryID string) error {
	return a.nodeCategoryRepo.RemoveAllFromCategory(ctx, categoryID)
}

// Save creates a node-category mapping (alias for Assign)
func (a *nodeCategoryRepositoryAdapter) Save(ctx context.Context, mapping *domain.NodeCategory) error {
	return a.nodeCategoryRepo.Assign(ctx, mapping)
}

// unitOfWorkAdapter implements UnitOfWorkAdapter using existing UnitOfWork
type unitOfWorkAdapter struct {
	unitOfWork repository.UnitOfWork
	// Cache adapters that wrap the transactional repositories
	nodeAdapter NodeRepositoryAdapter
	edgeAdapter EdgeRepositoryAdapter
	categoryAdapter CategoryRepositoryAdapter
	graphAdapter GraphRepositoryAdapter
	nodeCategoryAdapter NodeCategoryRepositoryAdapter
	// Flag to track if we've initialized the transactional adapters
	isTransactionActive bool
}

// NewUnitOfWorkAdapter creates a new unit of work adapter
func NewUnitOfWorkAdapter(
	unitOfWork repository.UnitOfWork,
	nodeAdapter NodeRepositoryAdapter,
	edgeAdapter EdgeRepositoryAdapter,
	categoryAdapter CategoryRepositoryAdapter,
	graphAdapter GraphRepositoryAdapter,
	nodeCategoryAdapter NodeCategoryRepositoryAdapter,
) UnitOfWorkAdapter {
	return &unitOfWorkAdapter{
		unitOfWork: unitOfWork,
		nodeAdapter: nodeAdapter,
		edgeAdapter: edgeAdapter,
		categoryAdapter: categoryAdapter,
		graphAdapter: graphAdapter,
		nodeCategoryAdapter: nodeCategoryAdapter,
	}
}

// Begin starts the unit of work and initializes transactional adapters
func (a *unitOfWorkAdapter) Begin(ctx context.Context) error {
	// Reset state in case previous transaction wasn't properly cleaned up
	// This handles warm Lambda containers where the adapter is reused
	a.isTransactionActive = false
	a.nodeAdapter = nil
	a.edgeAdapter = nil
	a.categoryAdapter = nil
	a.graphAdapter = nil
	a.nodeCategoryAdapter = nil
	
	// For high TPS scenarios, ensure clean rollback of any previous state
	// This is safe to call multiple times and handles edge cases
	a.unitOfWork.Rollback()
	
	err := a.unitOfWork.Begin(ctx)
	if err != nil {
		return err
	}
	
	// Don't initialize adapters here - they will be lazily initialized on first access
	// This avoids calling unitOfWork.Nodes() etc. before the underlying repositories are ready
	
	a.isTransactionActive = true
	return nil
}

// Commit commits the unit of work and cleans up transactional state
func (a *unitOfWorkAdapter) Commit() error {
	err := a.unitOfWork.Commit()
	// Always reset state regardless of error to prevent stuck transactions
	a.isTransactionActive = false
	a.nodeAdapter = nil
	a.edgeAdapter = nil
	a.categoryAdapter = nil
	a.graphAdapter = nil
	a.nodeCategoryAdapter = nil
	return err
}

// Rollback rolls back the unit of work and cleans up transactional state
func (a *unitOfWorkAdapter) Rollback() error {
	err := a.unitOfWork.Rollback()
	// Always reset state regardless of error to prevent stuck transactions
	a.isTransactionActive = false
	a.nodeAdapter = nil
	a.edgeAdapter = nil
	a.categoryAdapter = nil
	a.graphAdapter = nil
	a.nodeCategoryAdapter = nil
	return err
}

// Nodes returns the node repository adapter
func (a *unitOfWorkAdapter) Nodes() NodeRepositoryAdapter {
	if !a.isTransactionActive {
		panic("unit of work not active - call Begin() first")
	}
	// Lazy initialization - create adapter on first access
	if a.nodeAdapter == nil {
		a.nodeAdapter = NewNodeRepositoryAdapter(a.unitOfWork.Nodes(), nil)
	}
	return a.nodeAdapter
}

// Edges returns the edge repository adapter
func (a *unitOfWorkAdapter) Edges() EdgeRepositoryAdapter {
	if !a.isTransactionActive {
		panic("unit of work not active - call Begin() first")
	}
	// Lazy initialization - create adapter on first access
	if a.edgeAdapter == nil {
		a.edgeAdapter = NewEdgeRepositoryAdapter(a.unitOfWork.Edges())
	}
	return a.edgeAdapter
}

// Categories returns the category repository adapter
func (a *unitOfWorkAdapter) Categories() CategoryRepositoryAdapter {
	if !a.isTransactionActive {
		panic("unit of work not active - call Begin() first")
	}
	// Lazy initialization - create adapter on first access
	if a.categoryAdapter == nil {
		a.categoryAdapter = NewCategoryRepositoryAdapter(a.unitOfWork.Categories())
	}
	return a.categoryAdapter
}

// Graph returns the graph repository adapter
func (a *unitOfWorkAdapter) Graph() GraphRepositoryAdapter {
	if !a.isTransactionActive {
		panic("unit of work not active - call Begin() first")
	}
	// Lazy initialization - create adapter on first access
	if a.graphAdapter == nil {
		a.graphAdapter = NewGraphRepositoryAdapter(a.unitOfWork.Graph())
	}
	return a.graphAdapter
}

// NodeCategories returns the node category repository adapter
func (a *unitOfWorkAdapter) NodeCategories() NodeCategoryRepositoryAdapter {
	if !a.isTransactionActive {
		panic("unit of work not active - call Begin() first")
	}
	// Lazy initialization - create adapter on first access
	if a.nodeCategoryAdapter == nil {
		a.nodeCategoryAdapter = NewNodeCategoryRepositoryAdapter(a.unitOfWork.NodeCategories())
	}
	return a.nodeCategoryAdapter
}

// PublishEvent publishes a domain event
func (a *unitOfWorkAdapter) PublishEvent(event domain.DomainEvent) {
	a.unitOfWork.PublishEvent(event)
}