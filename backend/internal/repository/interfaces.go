package repository

import (
	"brain2-backend/internal/domain"
	"context"
)

// NodeRepository handles node-specific operations
type NodeRepository interface {
	// Core node operations
	CreateNodeAndKeywords(ctx context.Context, node domain.Node) error
	FindNodeByID(ctx context.Context, userID, nodeID string) (*domain.Node, error)
	FindNodes(ctx context.Context, query NodeQuery) ([]domain.Node, error)
	DeleteNode(ctx context.Context, userID, nodeID string) error

	// Enhanced node operations with pagination
	GetNodesPage(ctx context.Context, query NodeQuery, pagination Pagination) (*NodePage, error)
	GetNodeNeighborhood(ctx context.Context, userID, nodeID string, depth int) (*domain.Graph, error)
}

// EdgeRepository handles edge-specific operations
type EdgeRepository interface {
	// Core edge operations  
	CreateEdges(ctx context.Context, userID, sourceNodeID string, relatedNodeIDs []string) error
	FindEdges(ctx context.Context, query EdgeQuery) ([]domain.Edge, error)
	
	// Enhanced edge operations with pagination
	GetEdgesPage(ctx context.Context, query EdgeQuery, pagination Pagination) (*EdgePage, error)
}

// KeywordRepository handles keyword indexing and search
type KeywordRepository interface {
	// Keyword-based search operations
	FindNodesByKeywords(ctx context.Context, userID string, keywords []string) ([]domain.Node, error)
}

// TransactionalRepository handles complex transactional operations
type TransactionalRepository interface {
	// Transactional operations that involve multiple entities
	CreateNodeWithEdges(ctx context.Context, node domain.Node, relatedNodeIDs []string) error
	UpdateNodeAndEdges(ctx context.Context, node domain.Node, relatedNodeIDs []string) error
}

// CategoryRepository handles category-specific operations
type CategoryRepository interface {
	// Core category operations
	CreateCategory(ctx context.Context, category domain.Category) error
	UpdateCategory(ctx context.Context, category domain.Category) error
	DeleteCategory(ctx context.Context, userID, categoryID string) error
	FindCategoryByID(ctx context.Context, userID, categoryID string) (*domain.Category, error)
	FindCategories(ctx context.Context, query CategoryQuery) ([]domain.Category, error)
	FindCategoriesByLevel(ctx context.Context, userID string, level int) ([]domain.Category, error)

	// Category hierarchy operations
	CreateCategoryHierarchy(ctx context.Context, hierarchy domain.CategoryHierarchy) error
	DeleteCategoryHierarchy(ctx context.Context, userID, parentID, childID string) error
	FindChildCategories(ctx context.Context, userID, parentID string) ([]domain.Category, error)
	FindParentCategory(ctx context.Context, userID, childID string) (*domain.Category, error)
	GetCategoryTree(ctx context.Context, userID string) ([]domain.Category, error)

	// Node-Category mapping operations
	AssignNodeToCategory(ctx context.Context, mapping domain.NodeCategory) error
	RemoveNodeFromCategory(ctx context.Context, userID, nodeID, categoryID string) error
	FindNodesByCategory(ctx context.Context, userID, categoryID string) ([]domain.Node, error)
	FindCategoriesForNode(ctx context.Context, userID, nodeID string) ([]domain.Category, error)

	// Batch operations for performance
	BatchAssignCategories(ctx context.Context, mappings []domain.NodeCategory) error
	UpdateCategoryNoteCounts(ctx context.Context, userID string, categoryCounts map[string]int) error
}

// GraphRepository handles graph-wide operations
type GraphRepository interface {
	// Graph data operations
	GetGraphData(ctx context.Context, query GraphQuery) (*domain.Graph, error)
	GetGraphDataPaginated(ctx context.Context, query GraphQuery, pagination Pagination) (*domain.Graph, string, error)
}