package repository

import (
	"brain2-backend/internal/domain"
	"context"
)

type Repository interface {
	CreateNodeAndKeywords(ctx context.Context, node domain.Node) error
	CreateEdges(ctx context.Context, userID, sourceNodeID string, relatedNodeIDs []string) error
	CreateNodeWithEdges(ctx context.Context, node domain.Node, relatedNodeIDs []string) error
	UpdateNodeAndEdges(ctx context.Context, node domain.Node, relatedNodeIDs []string) error
	DeleteNode(ctx context.Context, userID, nodeID string) error
	FindNodeByID(ctx context.Context, userID, nodeID string) (*domain.Node, error)
	FindNodes(ctx context.Context, query NodeQuery) ([]domain.Node, error)
	FindEdges(ctx context.Context, query EdgeQuery) ([]domain.Edge, error)
	GetGraphData(ctx context.Context, query GraphQuery) (*domain.Graph, error)
	FindNodesByKeywords(ctx context.Context, userID string, keywords []string) ([]domain.Node, error)

	// Enhanced paginated queries for performance
	GetNodesPage(ctx context.Context, query NodeQuery, pagination Pagination) (*NodePage, error)
	GetEdgesPage(ctx context.Context, query EdgeQuery, pagination Pagination) (*EdgePage, error)
	GetNodeNeighborhood(ctx context.Context, userID, nodeID string, depth int) (*domain.Graph, error)

	// Optimized graph data retrieval with pagination
	GetGraphDataPaginated(ctx context.Context, query GraphQuery, pagination Pagination) (*domain.Graph, string, error)

	// Category operations
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

	// Node-Category operations (enhanced)
	AssignNodeToCategory(ctx context.Context, mapping domain.NodeCategory) error
	RemoveNodeFromCategory(ctx context.Context, userID, nodeID, categoryID string) error
	FindNodesByCategory(ctx context.Context, userID, categoryID string) ([]domain.Node, error)
	FindCategoriesForNode(ctx context.Context, userID, nodeID string) ([]domain.Category, error)

	// Batch operations for performance
	BatchAssignCategories(ctx context.Context, mappings []domain.NodeCategory) error
	UpdateCategoryNoteCounts(ctx context.Context, userID string, categoryCounts map[string]int) error
}
