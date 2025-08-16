package repository

import (
	"brain2-backend/internal/domain"
	"context"
)

// Enhanced Repository Interfaces - Phase 2 Best Practices Implementation
//
// This file demonstrates repository pattern excellence through:
//   1. Interface Segregation Principle - Focused, single-purpose interfaces
//   2. Specification Pattern Integration - Complex query capabilities
//   3. CQRS Support - Separate read/write optimizations
//   4. Unit of Work Pattern - Transactional consistency
//   5. Functional Options - Flexible query configuration
//
// The enhanced interfaces maintain backward compatibility while providing
// advanced repository pattern capabilities for complex domain scenarios.

// NodeRepository handles node-specific operations with backward compatibility
// Enhanced with Phase 2 patterns while maintaining existing functionality
type NodeRepository interface {
	// Core node operations (existing - maintained for compatibility)
	CreateNodeAndKeywords(ctx context.Context, node *domain.Node) error
	FindNodeByID(ctx context.Context, userID, nodeID string) (*domain.Node, error)
	FindNodes(ctx context.Context, query NodeQuery) ([]*domain.Node, error)
	DeleteNode(ctx context.Context, userID, nodeID string) error

	// Enhanced node operations with pagination (existing - maintained)
	GetNodesPage(ctx context.Context, query NodeQuery, pagination Pagination) (*NodePage, error)
	GetNodeNeighborhood(ctx context.Context, userID, nodeID string, depth int) (*domain.Graph, error)
	CountNodes(ctx context.Context, userID string) (int, error)
	
	// Phase 2 Enhancements - Advanced Query Support  
	FindNodesWithOptions(ctx context.Context, query NodeQuery, opts ...QueryOption) ([]*domain.Node, error)
	FindNodesPageWithOptions(ctx context.Context, query NodeQuery, pagination Pagination, opts ...QueryOption) (*NodePage, error)
}

// EdgeRepository handles edge-specific operations with Phase 2 enhancements
type EdgeRepository interface {
	// Core edge operations (existing - maintained for compatibility)
	CreateEdges(ctx context.Context, userID, sourceNodeID string, relatedNodeIDs []string) error
	CreateEdge(ctx context.Context, edge *domain.Edge) error
	FindEdges(ctx context.Context, query EdgeQuery) ([]*domain.Edge, error)
	
	// Enhanced edge operations with pagination (existing - maintained)
	GetEdgesPage(ctx context.Context, query EdgeQuery, pagination Pagination) (*EdgePage, error)
	
	// Phase 2 Enhancements - Advanced Edge Operations
	FindEdgesWithOptions(ctx context.Context, query EdgeQuery, opts ...QueryOption) ([]*domain.Edge, error)
}

// KeywordRepository handles keyword indexing and search
type KeywordRepository interface {
	// Keyword-based search operations
	FindNodesByKeywords(ctx context.Context, userID string, keywords []string) ([]*domain.Node, error)
}

// TransactionalRepository handles complex transactional operations
type TransactionalRepository interface {
	// Transactional operations that involve multiple entities
	CreateNodeWithEdges(ctx context.Context, node *domain.Node, relatedNodeIDs []string) error
	UpdateNodeAndEdges(ctx context.Context, node *domain.Node, relatedNodeIDs []string) error
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
	
	// Adapter-compatible methods for CQRS
	Save(ctx context.Context, category *domain.Category) error
	FindByID(ctx context.Context, userID, categoryID string) (*domain.Category, error)
	Delete(ctx context.Context, userID, categoryID string) error

	// Category hierarchy operations
	CreateCategoryHierarchy(ctx context.Context, hierarchy domain.CategoryHierarchy) error
	DeleteCategoryHierarchy(ctx context.Context, userID, parentID, childID string) error
	FindChildCategories(ctx context.Context, userID, parentID string) ([]domain.Category, error)
	FindParentCategory(ctx context.Context, userID, childID string) (*domain.Category, error)
	GetCategoryTree(ctx context.Context, userID string) ([]domain.Category, error)

	// Node-Category mapping operations
	AssignNodeToCategory(ctx context.Context, mapping domain.NodeCategory) error
	RemoveNodeFromCategory(ctx context.Context, userID, nodeID, categoryID string) error
	FindNodesByCategory(ctx context.Context, userID, categoryID string) ([]*domain.Node, error)
	FindCategoriesForNode(ctx context.Context, userID, nodeID string) ([]domain.Category, error)

	// Batch operations for performance
	BatchAssignCategories(ctx context.Context, mappings []domain.NodeCategory) error
	UpdateCategoryNoteCounts(ctx context.Context, userID string, categoryCounts map[string]int) error
}

// NodeCategoryRepository handles node-category mapping operations
type NodeCategoryRepository interface {
	// Core mapping operations
	Assign(ctx context.Context, mapping *domain.NodeCategory) error
	Remove(ctx context.Context, userID, nodeID, categoryID string) error
	RemoveAllByNode(ctx context.Context, userID, nodeID string) error
	RemoveAllByCategory(ctx context.Context, userID, categoryID string) error
	RemoveAllFromCategory(ctx context.Context, categoryID string) error
	
	// Query operations
	FindByNode(ctx context.Context, userID, nodeID string) ([]*domain.NodeCategory, error)
	FindByCategory(ctx context.Context, userID, categoryID string) ([]*domain.NodeCategory, error)
	FindByUser(ctx context.Context, userID string) ([]*domain.NodeCategory, error)
	Exists(ctx context.Context, userID, nodeID, categoryID string) (bool, error)
	
	// Batch operations
	BatchAssign(ctx context.Context, mappings []*domain.NodeCategory) error
	
	// Category-specific queries 
	FindNodesByCategory(ctx context.Context, userID, categoryID string) ([]*domain.Node, error)
	FindNodesByCategoryPage(ctx context.Context, userID, categoryID string, pagination Pagination) (*NodePage, error)
	CountNodesInCategory(ctx context.Context, userID, categoryID string) (int, error)
	FindCategoriesByNode(ctx context.Context, userID, nodeID string) ([]*domain.Category, error)
	BatchRemove(ctx context.Context, userID string, pairs []struct{ NodeID, CategoryID string }) error
	
	// Statistics
	CountByCategory(ctx context.Context, userID, categoryID string) (int, error)
	CountByNode(ctx context.Context, userID, nodeID string) (int, error)
}

// GraphRepository handles graph-wide operations with Phase 2 enhancements
type GraphRepository interface {
	// Graph data operations (existing - maintained for compatibility)
	GetGraphData(ctx context.Context, query GraphQuery) (*domain.Graph, error)
	GetGraphDataPaginated(ctx context.Context, query GraphQuery, pagination Pagination) (*domain.Graph, string, error)
	
	// Phase 2 Enhancements - Advanced Graph Operations
	GetSubgraph(ctx context.Context, nodeIDs []string, opts ...QueryOption) (*domain.Graph, error)
	GetConnectedComponents(ctx context.Context, userID string, opts ...QueryOption) ([]domain.Graph, error)
}

// Advanced Repository Interfaces for Phase 2 Excellence

// SpecificationRepository provides specification-based query capabilities
// This interface can be implemented by any repository to provide advanced querying
type SpecificationRepository interface {
	FindBySpecification(ctx context.Context, spec Specification, opts ...QueryOption) ([]interface{}, error)
	CountBySpecification(ctx context.Context, spec Specification) (int, error)
	ExistsBySpecification(ctx context.Context, spec Specification) (bool, error)
	DeleteBySpecification(ctx context.Context, spec Specification) (int, error)
}

// UnitOfWorkProvider provides access to Unit of Work instances
// This interface enables transactional operations across multiple repositories
type UnitOfWorkProvider interface {
	BeginUnitOfWork(ctx context.Context) (UnitOfWork, error)
	ExecuteInTransaction(ctx context.Context, operation func(uow UnitOfWork) error) error
}

// RepositoryProvider provides access to all repository types
// This interface supports the Factory pattern and dependency injection
type RepositoryProvider interface {
	GetNodeRepository() NodeRepository
	GetEdgeRepository() EdgeRepository
	GetCategoryRepository() CategoryRepository
	GetKeywordRepository() KeywordRepository
	GetTransactionalRepository() TransactionalRepository
	GetGraphRepository() GraphRepository
	
	// Phase 2 additions
	GetUnitOfWorkProvider() UnitOfWorkProvider
	GetSpecificationRepository() SpecificationRepository
}

// RepositoryManager provides high-level repository management
// This interface demonstrates the Facade pattern for complex repository operations
type RepositoryManager interface {
	// Transaction management
	WithTransaction(ctx context.Context, operation func(provider RepositoryProvider) error) error
	WithRetry(ctx context.Context, maxAttempts int, operation func(provider RepositoryProvider) error) error
	
	// Repository lifecycle
	Initialize() error
	Shutdown() error
	HealthCheck() error
	
	// Configuration
	UpdateConfiguration(config interface{}) error
	GetConfiguration() interface{}
}

// QueryExecutor provides advanced query execution capabilities
// This interface demonstrates the Command pattern for complex queries
type QueryExecutor interface {
	ExecuteQuery(ctx context.Context, query interface{}, opts ...QueryOption) (interface{}, error)
	ExecuteQueryBuilder(ctx context.Context, builder *QueryBuilder) (interface{}, error)
	ExecuteBatch(ctx context.Context, queries []interface{}, opts ...QueryOption) ([]interface{}, error)
	
	// Query planning and optimization
	ExplainQuery(ctx context.Context, query interface{}) (interface{}, error)
	OptimizeQuery(ctx context.Context, query interface{}) (interface{}, error)
}

// EventDrivenRepository provides event-based repository operations
// This interface demonstrates integration with domain events and CQRS
type EventDrivenRepository interface {
	// Event handling
	OnEntityCreated(ctx context.Context, entity interface{}) error
	OnEntityUpdated(ctx context.Context, entity interface{}) error
	OnEntityDeleted(ctx context.Context, entityID string) error
	
	// Event publishing
	PublishEvents(ctx context.Context, events []domain.DomainEvent) error
	GetPendingEvents(ctx context.Context) ([]domain.DomainEvent, error)
	
	// Event sourcing support
	ReplayEvents(ctx context.Context, entityID string) error
	GetEventHistory(ctx context.Context, entityID string) ([]domain.DomainEvent, error)
}