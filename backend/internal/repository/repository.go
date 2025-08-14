package repository

import (
	"context"
	"brain2-backend/internal/domain"
)

// Repository provides access to all segregated repository interfaces.
// This follows the Interface Segregation Principle by providing access to focused interfaces
// rather than inheriting all methods, avoiding method name conflicts.
type Repository interface {
	// Access segregated repository interfaces
	Nodes() NodeRepository
	Edges() EdgeRepository
	Categories() CategoryRepository
	NodeCategories() NodeCategoryMapper
	Keywords() KeywordSearcher
	Graph() GraphReader
	
	// Transaction management
	UnitOfWork() UnitOfWork
	
	// Factory access for decorated repositories
	WithDecorators(decorators ...RepositoryDecorator) Repository
	
	// Temporary legacy methods for backward compatibility (will be removed)
	FindNodeByID(ctx context.Context, userID, nodeID string) (*domain.Node, error)
	DeleteNode(ctx context.Context, userID, nodeID string) error
	GetGraphData(ctx context.Context, query GraphQuery) (*domain.Graph, error)
	Save(ctx context.Context, node *domain.Node) error
	
	// Category legacy methods
	FindCategories(ctx context.Context, userID string) ([]domain.Category, error)
	BatchAssignCategories(ctx context.Context, assignments []*domain.NodeCategory) error
	CreateCategoryHierarchy(ctx context.Context, hierarchy *domain.CategoryHierarchy) error
	GetCategoryTree(ctx context.Context, userID string) ([]domain.Category, error)
	FindCategoryByID(ctx context.Context, userID, categoryID string) (*domain.Category, error)
	CreateCategory(ctx context.Context, category *domain.Category) error
}

// RepositoryDecorator defines how to decorate repository interfaces
type RepositoryDecorator interface {
	DecorateNode(NodeRepository) NodeRepository
	DecorateEdge(EdgeRepository) EdgeRepository
	DecorateCategory(CategoryRepository) CategoryRepository
}
