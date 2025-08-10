// Package mocks provides mock implementations of repository interfaces for testing.
package mocks

import (
	"context"
	"fmt"
	"sync"

	"brain2-backend/internal/domain"
	"brain2-backend/internal/repository"
	appErrors "brain2-backend/pkg/errors"
)

// MockRepository provides an in-memory mock implementation of the Repository interface.
// This is useful for unit testing services without requiring a real database.
type MockRepository struct {
	mu sync.RWMutex

	// In-memory storage
	nodes          map[string]*domain.Node     // nodeID -> Node
	edges          map[string][]domain.Edge    // sourceNodeID -> []Edge
	categories     map[string]*domain.Category // categoryID -> Category
	nodeCategories map[string][]string         // nodeID -> []categoryID
	categoryNodes  map[string][]string         // categoryID -> []nodeID

	// Category hierarchy
	categoryHierarchy map[string][]string // parentID -> []childID
	parentCategories  map[string]string   // childID -> parentID

	// For testing error scenarios
	shouldFailOn map[string]error
}

// NewMockRepository creates a new mock repository instance.
func NewMockRepository() *MockRepository {
	return &MockRepository{
		nodes:             make(map[string]*domain.Node),
		edges:             make(map[string][]domain.Edge),
		categories:        make(map[string]*domain.Category),
		nodeCategories:    make(map[string][]string),
		categoryNodes:     make(map[string][]string),
		categoryHierarchy: make(map[string][]string),
		parentCategories:  make(map[string]string),
		shouldFailOn:      make(map[string]error),
	}
}

// SetError configures the mock to return an error for a specific method.
// Useful for testing error handling in services.
func (m *MockRepository) SetError(method string, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.shouldFailOn[method] = err
}

// ClearErrors removes all configured errors.
func (m *MockRepository) ClearErrors() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.shouldFailOn = make(map[string]error)
}

// checkError returns an error if one is configured for the given method.
func (m *MockRepository) checkError(method string) error {
	if err, exists := m.shouldFailOn[method]; exists {
		return err
	}
	return nil
}

// Node operations

func (m *MockRepository) CreateNodeAndKeywords(ctx context.Context, node domain.Node) error {
	if err := m.checkError("CreateNodeAndKeywords"); err != nil {
		return err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if node already exists
	if _, exists := m.nodes[node.ID]; exists {
		return appErrors.NewValidation("node already exists")
	}

	// Store the node
	nodeCopy := node
	m.nodes[node.ID] = &nodeCopy
	return nil
}

func (m *MockRepository) CreateEdges(ctx context.Context, userID, sourceNodeID string, relatedNodeIDs []string) error {
	if err := m.checkError("CreateEdges"); err != nil {
		return err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Verify source node exists
	if _, exists := m.nodes[sourceNodeID]; !exists {
		return appErrors.NewNotFound("source node not found")
	}

	// Create bidirectional edges
	for _, targetID := range relatedNodeIDs {
		if _, exists := m.nodes[targetID]; !exists {
			return appErrors.NewNotFound(fmt.Sprintf("target node %s not found", targetID))
		}

		// Add edge from source to target
		m.edges[sourceNodeID] = append(m.edges[sourceNodeID], domain.Edge{
			SourceID: sourceNodeID,
			TargetID: targetID,
		})

		// Add edge from target to source (bidirectional)
		m.edges[targetID] = append(m.edges[targetID], domain.Edge{
			SourceID: targetID,
			TargetID: sourceNodeID,
		})
	}

	return nil
}

func (m *MockRepository) CreateNodeWithEdges(ctx context.Context, node domain.Node, relatedNodeIDs []string) error {
	if err := m.checkError("CreateNodeWithEdges"); err != nil {
		return err
	}

	// First create the node
	if err := m.CreateNodeAndKeywords(ctx, node); err != nil {
		return err
	}

	// Then create the edges
	if len(relatedNodeIDs) > 0 {
		return m.CreateEdges(ctx, node.UserID, node.ID, relatedNodeIDs)
	}

	return nil
}

func (m *MockRepository) UpdateNodeAndEdges(ctx context.Context, node domain.Node, relatedNodeIDs []string) error {
	if err := m.checkError("UpdateNodeAndEdges"); err != nil {
		return err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if node exists
	if _, exists := m.nodes[node.ID]; !exists {
		return appErrors.NewNotFound("node not found")
	}

	// Update the node
	nodeCopy := node
	m.nodes[node.ID] = &nodeCopy

	// Clear existing edges for this node
	delete(m.edges, node.ID)

	// Remove references to this node from other nodes' edges
	for nodeID, edges := range m.edges {
		var filteredEdges []domain.Edge
		for _, edge := range edges {
			if edge.TargetID != node.ID {
				filteredEdges = append(filteredEdges, edge)
			}
		}
		m.edges[nodeID] = filteredEdges
	}

	// Create new edges
	for _, targetID := range relatedNodeIDs {
		if _, exists := m.nodes[targetID]; !exists {
			continue // Skip non-existent nodes
		}

		// Add edge from source to target
		m.edges[node.ID] = append(m.edges[node.ID], domain.Edge{
			SourceID: node.ID,
			TargetID: targetID,
		})

		// Add edge from target to source
		m.edges[targetID] = append(m.edges[targetID], domain.Edge{
			SourceID: targetID,
			TargetID: node.ID,
		})
	}

	return nil
}

func (m *MockRepository) DeleteNode(ctx context.Context, userID, nodeID string) error {
	if err := m.checkError("DeleteNode"); err != nil {
		return err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if node exists and belongs to user
	if node, exists := m.nodes[nodeID]; !exists {
		return appErrors.NewNotFound("node not found")
	} else if node.UserID != userID {
		return appErrors.NewNotFound("node not found")
	}

	// Delete the node
	delete(m.nodes, nodeID)

	// Delete all edges involving this node
	delete(m.edges, nodeID)
	for sourceID, edges := range m.edges {
		var filteredEdges []domain.Edge
		for _, edge := range edges {
			if edge.TargetID != nodeID {
				filteredEdges = append(filteredEdges, edge)
			}
		}
		m.edges[sourceID] = filteredEdges
	}

	// Remove from category associations
	if categoryIDs, exists := m.nodeCategories[nodeID]; exists {
		for _, categoryID := range categoryIDs {
			if nodeIDs, exists := m.categoryNodes[categoryID]; exists {
				var filteredNodeIDs []string
				for _, nID := range nodeIDs {
					if nID != nodeID {
						filteredNodeIDs = append(filteredNodeIDs, nID)
					}
				}
				m.categoryNodes[categoryID] = filteredNodeIDs
			}
		}
		delete(m.nodeCategories, nodeID)
	}

	return nil
}

func (m *MockRepository) FindNodeByID(ctx context.Context, userID, nodeID string) (*domain.Node, error) {
	if err := m.checkError("FindNodeByID"); err != nil {
		return nil, err
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	node, exists := m.nodes[nodeID]
	if !exists {
		return nil, nil
	}

	// Verify ownership
	if node.UserID != userID {
		return nil, nil
	}

	// Return a copy to prevent external modification
	nodeCopy := *node
	return &nodeCopy, nil
}

func (m *MockRepository) FindNodes(ctx context.Context, query repository.NodeQuery) ([]domain.Node, error) {
	if err := m.checkError("FindNodes"); err != nil {
		return nil, err
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	var nodes []domain.Node

	// If specific node IDs are requested
	if query.HasNodeIDs() {
		for _, nodeID := range query.NodeIDs {
			if node, exists := m.nodes[nodeID]; exists && node.UserID == query.UserID {
				nodes = append(nodes, *node)
			}
		}
		return nodes, nil
	}

	// Otherwise, return all nodes for the user
	for _, node := range m.nodes {
		if node.UserID == query.UserID {
			// Apply keyword filter if specified
			if query.HasKeywords() {
				hasKeyword := false
				for _, keyword := range query.Keywords {
					for _, nodeKeyword := range node.Keywords {
						if nodeKeyword == keyword {
							hasKeyword = true
							break
						}
					}
					if hasKeyword {
						break
					}
				}
				if !hasKeyword {
					continue
				}
			}

			nodes = append(nodes, *node)
		}
	}

	// Apply pagination
	if query.HasPagination() {
		start := query.Offset
		if start >= len(nodes) {
			return []domain.Node{}, nil
		}

		end := len(nodes)
		if query.Limit > 0 && start+query.Limit < len(nodes) {
			end = start + query.Limit
		}

		nodes = nodes[start:end]
	}

	return nodes, nil
}

func (m *MockRepository) FindEdges(ctx context.Context, query repository.EdgeQuery) ([]domain.Edge, error) {
	if err := m.checkError("FindEdges"); err != nil {
		return nil, err
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	var edges []domain.Edge

	// If specific node IDs are requested
	if query.HasNodeIDs() {
		for _, nodeID := range query.NodeIDs {
			if nodeEdges, exists := m.edges[nodeID]; exists {
				edges = append(edges, nodeEdges...)
			}
		}
		return edges, nil
	}

	// Return all edges for nodes belonging to the user
	for nodeID, nodeEdges := range m.edges {
		if node, exists := m.nodes[nodeID]; exists && node.UserID == query.UserID {
			for _, edge := range nodeEdges {
				// Apply source filter if specified
				if query.HasSourceFilter() && edge.SourceID != query.SourceID {
					continue
				}

				// Apply target filter if specified
				if query.HasTargetFilter() && edge.TargetID != query.TargetID {
					continue
				}

				edges = append(edges, edge)
			}
		}
	}

	return edges, nil
}

func (m *MockRepository) GetGraphData(ctx context.Context, query repository.GraphQuery) (*domain.Graph, error) {
	if err := m.checkError("GetGraphData"); err != nil {
		return nil, err
	}

	nodeQuery := repository.NodeQuery{UserID: query.UserID}
	if query.HasNodeFilter() {
		nodeQuery.NodeIDs = query.NodeIDs
	}

	nodes, err := m.FindNodes(ctx, nodeQuery)
	if err != nil {
		return nil, err
	}

	var edges []domain.Edge
	if query.IncludeEdges {
		edgeQuery := repository.EdgeQuery{UserID: query.UserID}
		if query.HasNodeFilter() {
			edgeQuery.NodeIDs = query.NodeIDs
		}

		edges, err = m.FindEdges(ctx, edgeQuery)
		if err != nil {
			return nil, err
		}
	}

	return &domain.Graph{
		Nodes: nodes,
		Edges: edges,
	}, nil
}

func (m *MockRepository) FindNodesByKeywords(ctx context.Context, userID string, keywords []string) ([]domain.Node, error) {
	if err := m.checkError("FindNodesByKeywords"); err != nil {
		return nil, err
	}

	query := repository.NodeQuery{
		UserID:   userID,
		Keywords: keywords,
	}

	return m.FindNodes(ctx, query)
}

// Category operations

func (m *MockRepository) CreateCategory(ctx context.Context, category domain.Category) error {
	if err := m.checkError("CreateCategory"); err != nil {
		return err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if category already exists
	if _, exists := m.categories[category.ID]; exists {
		return appErrors.NewValidation("category already exists")
	}

	// Store the category
	categoryCopy := category
	m.categories[category.ID] = &categoryCopy
	return nil
}

func (m *MockRepository) UpdateCategory(ctx context.Context, category domain.Category) error {
	if err := m.checkError("UpdateCategory"); err != nil {
		return err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if category exists
	if _, exists := m.categories[category.ID]; !exists {
		return appErrors.NewNotFound("category not found")
	}

	// Update the category
	categoryCopy := category
	m.categories[category.ID] = &categoryCopy
	return nil
}

func (m *MockRepository) DeleteCategory(ctx context.Context, userID, categoryID string) error {
	if err := m.checkError("DeleteCategory"); err != nil {
		return err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if category exists and belongs to user
	if category, exists := m.categories[categoryID]; !exists {
		return appErrors.NewNotFound("category not found")
	} else if category.UserID != userID {
		return appErrors.NewNotFound("category not found")
	}

	// Delete the category
	delete(m.categories, categoryID)

	// Remove from node associations
	if nodeIDs, exists := m.categoryNodes[categoryID]; exists {
		for _, nodeID := range nodeIDs {
			if categoryIDs, exists := m.nodeCategories[nodeID]; exists {
				var filteredCategoryIDs []string
				for _, cID := range categoryIDs {
					if cID != categoryID {
						filteredCategoryIDs = append(filteredCategoryIDs, cID)
					}
				}
				m.nodeCategories[nodeID] = filteredCategoryIDs
			}
		}
		delete(m.categoryNodes, categoryID)
	}

	// Remove from hierarchy
	if parentID := m.parentCategories[categoryID]; parentID != "" {
		if children, exists := m.categoryHierarchy[parentID]; exists {
			var filteredChildren []string
			for _, childID := range children {
				if childID != categoryID {
					filteredChildren = append(filteredChildren, childID)
				}
			}
			m.categoryHierarchy[parentID] = filteredChildren
		}
		delete(m.parentCategories, categoryID)
	}

	// Remove any children (make them orphans)
	if children, exists := m.categoryHierarchy[categoryID]; exists {
		for _, childID := range children {
			delete(m.parentCategories, childID)
		}
		delete(m.categoryHierarchy, categoryID)
	}

	return nil
}

func (m *MockRepository) FindCategoryByID(ctx context.Context, userID, categoryID string) (*domain.Category, error) {
	if err := m.checkError("FindCategoryByID"); err != nil {
		return nil, err
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	category, exists := m.categories[categoryID]
	if !exists {
		return nil, nil
	}

	// Verify ownership
	if category.UserID != userID {
		return nil, nil
	}

	// Return a copy
	categoryCopy := *category
	return &categoryCopy, nil
}

func (m *MockRepository) FindCategories(ctx context.Context, query repository.CategoryQuery) ([]domain.Category, error) {
	if err := m.checkError("FindCategories"); err != nil {
		return nil, err
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	var categories []domain.Category

	for _, category := range m.categories {
		if category.UserID == query.UserID {
			categories = append(categories, *category)
		}
	}

	// Apply pagination
	if query.HasPagination() {
		start := query.Offset
		if start >= len(categories) {
			return []domain.Category{}, nil
		}

		end := len(categories)
		if query.Limit > 0 && start+query.Limit < len(categories) {
			end = start + query.Limit
		}

		categories = categories[start:end]
	}

	return categories, nil
}

func (m *MockRepository) FindCategoriesByLevel(ctx context.Context, userID string, level int) ([]domain.Category, error) {
	if err := m.checkError("FindCategoriesByLevel"); err != nil {
		return nil, err
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	var categories []domain.Category

	for _, category := range m.categories {
		if category.UserID == userID && category.Level == level {
			categories = append(categories, *category)
		}
	}

	return categories, nil
}

// Category hierarchy operations

func (m *MockRepository) CreateCategoryHierarchy(ctx context.Context, hierarchy domain.CategoryHierarchy) error {
	if err := m.checkError("CreateCategoryHierarchy"); err != nil {
		return err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Verify both categories exist
	if _, exists := m.categories[hierarchy.ParentID]; !exists {
		return appErrors.NewNotFound("parent category not found")
	}
	if _, exists := m.categories[hierarchy.ChildID]; !exists {
		return appErrors.NewNotFound("child category not found")
	}

	// Add to hierarchy
	m.categoryHierarchy[hierarchy.ParentID] = append(m.categoryHierarchy[hierarchy.ParentID], hierarchy.ChildID)
	m.parentCategories[hierarchy.ChildID] = hierarchy.ParentID

	return nil
}

func (m *MockRepository) DeleteCategoryHierarchy(ctx context.Context, userID, parentID, childID string) error {
	if err := m.checkError("DeleteCategoryHierarchy"); err != nil {
		return err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Remove from hierarchy
	if children, exists := m.categoryHierarchy[parentID]; exists {
		var filteredChildren []string
		for _, cID := range children {
			if cID != childID {
				filteredChildren = append(filteredChildren, cID)
			}
		}
		m.categoryHierarchy[parentID] = filteredChildren
	}

	delete(m.parentCategories, childID)
	return nil
}

func (m *MockRepository) FindChildCategories(ctx context.Context, userID, parentID string) ([]domain.Category, error) {
	if err := m.checkError("FindChildCategories"); err != nil {
		return nil, err
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	var categories []domain.Category

	if childIDs, exists := m.categoryHierarchy[parentID]; exists {
		for _, childID := range childIDs {
			if category, exists := m.categories[childID]; exists && category.UserID == userID {
				categories = append(categories, *category)
			}
		}
	}

	return categories, nil
}

func (m *MockRepository) FindParentCategory(ctx context.Context, userID, childID string) (*domain.Category, error) {
	if err := m.checkError("FindParentCategory"); err != nil {
		return nil, err
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	if parentID, exists := m.parentCategories[childID]; exists {
		if category, exists := m.categories[parentID]; exists && category.UserID == userID {
			categoryCopy := *category
			return &categoryCopy, nil
		}
	}

	return nil, nil
}

func (m *MockRepository) GetCategoryTree(ctx context.Context, userID string) ([]domain.Category, error) {
	if err := m.checkError("GetCategoryTree"); err != nil {
		return nil, err
	}

	query := repository.CategoryQuery{UserID: userID}
	return m.FindCategories(ctx, query)
}

// Node-Category operations

func (m *MockRepository) AssignNodeToCategory(ctx context.Context, mapping domain.NodeCategory) error {
	if err := m.checkError("AssignNodeToCategory"); err != nil {
		return err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Verify node and category exist
	if _, exists := m.nodes[mapping.NodeID]; !exists {
		return appErrors.NewNotFound("node not found")
	}
	if _, exists := m.categories[mapping.CategoryID]; !exists {
		return appErrors.NewNotFound("category not found")
	}

	// Add to mappings
	m.nodeCategories[mapping.NodeID] = append(m.nodeCategories[mapping.NodeID], mapping.CategoryID)
	m.categoryNodes[mapping.CategoryID] = append(m.categoryNodes[mapping.CategoryID], mapping.NodeID)

	return nil
}

func (m *MockRepository) RemoveNodeFromCategory(ctx context.Context, userID, nodeID, categoryID string) error {
	if err := m.checkError("RemoveNodeFromCategory"); err != nil {
		return err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Remove from mappings
	if categoryIDs, exists := m.nodeCategories[nodeID]; exists {
		var filteredCategoryIDs []string
		for _, cID := range categoryIDs {
			if cID != categoryID {
				filteredCategoryIDs = append(filteredCategoryIDs, cID)
			}
		}
		m.nodeCategories[nodeID] = filteredCategoryIDs
	}

	if nodeIDs, exists := m.categoryNodes[categoryID]; exists {
		var filteredNodeIDs []string
		for _, nID := range nodeIDs {
			if nID != nodeID {
				filteredNodeIDs = append(filteredNodeIDs, nID)
			}
		}
		m.categoryNodes[categoryID] = filteredNodeIDs
	}

	return nil
}

func (m *MockRepository) FindNodesByCategory(ctx context.Context, userID, categoryID string) ([]domain.Node, error) {
	if err := m.checkError("FindNodesByCategory"); err != nil {
		return nil, err
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	var nodes []domain.Node

	if nodeIDs, exists := m.categoryNodes[categoryID]; exists {
		for _, nodeID := range nodeIDs {
			if node, exists := m.nodes[nodeID]; exists && node.UserID == userID {
				nodes = append(nodes, *node)
			}
		}
	}

	return nodes, nil
}

func (m *MockRepository) FindCategoriesForNode(ctx context.Context, userID, nodeID string) ([]domain.Category, error) {
	if err := m.checkError("FindCategoriesForNode"); err != nil {
		return nil, err
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	var categories []domain.Category

	if categoryIDs, exists := m.nodeCategories[nodeID]; exists {
		for _, categoryID := range categoryIDs {
			if category, exists := m.categories[categoryID]; exists && category.UserID == userID {
				categories = append(categories, *category)
			}
		}
	}

	return categories, nil
}

// Batch operations

func (m *MockRepository) BatchAssignCategories(ctx context.Context, mappings []domain.NodeCategory) error {
	if err := m.checkError("BatchAssignCategories"); err != nil {
		return err
	}

	for _, mapping := range mappings {
		if err := m.AssignNodeToCategory(ctx, mapping); err != nil {
			return err
		}
	}

	return nil
}

func (m *MockRepository) UpdateCategoryNoteCounts(ctx context.Context, userID string, categoryCounts map[string]int) error {
	if err := m.checkError("UpdateCategoryNoteCounts"); err != nil {
		return err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	for categoryID, count := range categoryCounts {
		if category, exists := m.categories[categoryID]; exists && category.UserID == userID {
			category.NoteCount = count
		}
	}

	return nil
}

// GetEdgesPage returns paginated edges based on query and pagination parameters
func (m *MockRepository) GetEdgesPage(ctx context.Context, query repository.EdgeQuery, pagination repository.Pagination) (*repository.EdgePage, error) {
	if err := m.checkError("GetEdgesPage"); err != nil {
		return nil, err
	}

	// For the mock, just return all edges matching the query
	edges, err := m.FindEdges(ctx, query)
	if err != nil {
		return nil, err
	}

	return &repository.EdgePage{
		Items:      edges,
		TotalCount: len(edges),
		HasMore:    false,
		NextCursor: "",
	}, nil
}

// GetGraphDataPaginated returns paginated graph data
func (m *MockRepository) GetGraphDataPaginated(ctx context.Context, query repository.GraphQuery, pagination repository.Pagination) (*domain.Graph, string, error) {
	if err := m.checkError("GetGraphDataPaginated"); err != nil {
		return nil, "", err
	}

	// For the mock, just return the full graph data
	graph, err := m.GetGraphData(ctx, query)
	if err != nil {
		return nil, "", err
	}

	return graph, "", nil
}

// GetNodeNeighborhood returns the neighborhood graph for a specific node within a given depth
func (m *MockRepository) GetNodeNeighborhood(ctx context.Context, userID, nodeID string, depth int) (*domain.Graph, error) {
	if err := m.checkError("GetNodeNeighborhood"); err != nil {
		return nil, err
	}

	// For the mock, just return a simplified neighborhood (node + its direct edges)
	node, err := m.FindNodeByID(ctx, userID, nodeID)
	if err != nil {
		return nil, err
	}
	if node == nil {
		return nil, repository.NewNotFoundError("node", nodeID, userID)
	}

	// Get edges for this node
	edgeQuery := repository.EdgeQuery{
		UserID:   userID,
		SourceID: nodeID,
	}
	edges, err := m.FindEdges(ctx, edgeQuery)
	if err != nil {
		return nil, err
	}

	return &domain.Graph{
		Nodes: []domain.Node{*node},
		Edges: edges,
	}, nil
}

// GetNodesPage returns paginated nodes based on query and pagination parameters
func (m *MockRepository) GetNodesPage(ctx context.Context, query repository.NodeQuery, pagination repository.Pagination) (*repository.NodePage, error) {
	if err := m.checkError("GetNodesPage"); err != nil {
		return nil, err
	}

	// For the mock, just return all nodes matching the query
	nodes, err := m.FindNodes(ctx, query)
	if err != nil {
		return nil, err
	}

	return &repository.NodePage{
		Items:      nodes,
		TotalCount: len(nodes),
		HasMore:    false,
		NextCursor: "",
	}, nil
}
