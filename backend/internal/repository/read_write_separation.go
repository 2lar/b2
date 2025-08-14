package repository

import (
	"context"

	"brain2-backend/internal/domain"
)

// Read/Write Separation (CQRS) Implementation
//
// Key Concepts Illustrated:
//   1. Command Query Responsibility Segregation (CQRS)
//   2. Interface Segregation Principle (ISP)
//   3. Single Responsibility Principle (SRP)
//   4. Optimized operations for specific concerns
//   5. Flexibility for different storage backends
//
// This implementation demonstrates how to separate read and write operations
// for better performance, scalability, and maintainability.
//
// Benefits:
//   - Read operations can be optimized for queries (denormalization, indexing)
//   - Write operations can be optimized for consistency (normalization, validation)
//   - Different storage backends can be used for reads vs writes
//   - Easier to scale reads and writes independently
//   - Clearer code with focused interfaces

// Reader Interfaces - Optimized for Query Operations

// NodeReader handles read-only operations for nodes
// This interface is optimized for query performance and can use
// read replicas, caching, or specialized query stores
type NodeReader interface {
	// Single entity queries
	FindByID(ctx context.Context, id domain.NodeID) (*domain.Node, error)
	Exists(ctx context.Context, id domain.NodeID) (bool, error)
	
	// User-scoped queries
	FindByUser(ctx context.Context, userID domain.UserID, opts ...QueryOption) ([]*domain.Node, error)
	CountByUser(ctx context.Context, userID domain.UserID) (int, error)
	
	// Content-based queries
	FindByKeywords(ctx context.Context, userID domain.UserID, keywords []string, opts ...QueryOption) ([]*domain.Node, error)
	FindByTags(ctx context.Context, userID domain.UserID, tags []string, opts ...QueryOption) ([]*domain.Node, error)
	FindByContent(ctx context.Context, userID domain.UserID, searchTerm string, fuzzy bool, opts ...QueryOption) ([]*domain.Node, error)
	
	// Time-based queries
	FindRecentlyCreated(ctx context.Context, userID domain.UserID, days int, opts ...QueryOption) ([]*domain.Node, error)
	FindRecentlyUpdated(ctx context.Context, userID domain.UserID, days int, opts ...QueryOption) ([]*domain.Node, error)
	
	// Specification-based queries
	FindBySpecification(ctx context.Context, spec Specification, opts ...QueryOption) ([]*domain.Node, error)
	CountBySpecification(ctx context.Context, spec Specification) (int, error)
	
	// Paginated queries
	FindPage(ctx context.Context, query NodeQuery, pagination Pagination) (*NodePage, error)
	
	// Relationship queries
	FindConnected(ctx context.Context, nodeID domain.NodeID, depth int, opts ...QueryOption) ([]*domain.Node, error)
	FindSimilar(ctx context.Context, nodeID domain.NodeID, threshold float64, opts ...QueryOption) ([]*domain.Node, error)
}

// EdgeReader handles read-only operations for edges
type EdgeReader interface {
	// Single entity queries
	FindByID(ctx context.Context, id domain.NodeID) (*domain.Edge, error)
	Exists(ctx context.Context, id domain.NodeID) (bool, error)
	
	// User-scoped queries
	FindByUser(ctx context.Context, userID domain.UserID, opts ...QueryOption) ([]*domain.Edge, error)
	CountByUser(ctx context.Context, userID domain.UserID) (int, error)
	
	// Node relationship queries
	FindBySourceNode(ctx context.Context, sourceID domain.NodeID, opts ...QueryOption) ([]*domain.Edge, error)
	FindByTargetNode(ctx context.Context, targetID domain.NodeID, opts ...QueryOption) ([]*domain.Edge, error)
	FindByNode(ctx context.Context, nodeID domain.NodeID, opts ...QueryOption) ([]*domain.Edge, error)
	FindBetweenNodes(ctx context.Context, node1ID, node2ID domain.NodeID) ([]*domain.Edge, error)
	
	// Weight-based queries
	FindStrongConnections(ctx context.Context, userID domain.UserID, threshold float64, opts ...QueryOption) ([]*domain.Edge, error)
	FindWeakConnections(ctx context.Context, userID domain.UserID, threshold float64, opts ...QueryOption) ([]*domain.Edge, error)
	
	// Specification-based queries
	FindBySpecification(ctx context.Context, spec Specification, opts ...QueryOption) ([]*domain.Edge, error)
	CountBySpecification(ctx context.Context, spec Specification) (int, error)
	
	// Paginated queries
	FindPage(ctx context.Context, query EdgeQuery, pagination Pagination) (*EdgePage, error)
}

// CategoryReader handles read-only operations for categories
type CategoryReader interface {
	// Single entity queries
	FindByID(ctx context.Context, userID string, categoryID string) (*domain.Category, error)
	Exists(ctx context.Context, userID string, categoryID string) (bool, error)
	
	// User-scoped queries
	FindByUser(ctx context.Context, userID string, opts ...QueryOption) ([]domain.Category, error)
	CountByUser(ctx context.Context, userID string) (int, error)
	
	// Hierarchy queries
	FindRootCategories(ctx context.Context, userID string, opts ...QueryOption) ([]domain.Category, error)
	FindChildCategories(ctx context.Context, userID string, parentID string) ([]domain.Category, error)
	FindCategoryPath(ctx context.Context, userID string, categoryID string) ([]domain.Category, error)
	FindCategoryTree(ctx context.Context, userID string) ([]domain.Category, error)
	
	// Level-based queries
	FindByLevel(ctx context.Context, userID string, level int, opts ...QueryOption) ([]domain.Category, error)
	
	// Activity queries
	FindMostActive(ctx context.Context, userID string, limit int) ([]domain.Category, error)
	FindRecentlyUsed(ctx context.Context, userID string, days int, opts ...QueryOption) ([]domain.Category, error)
	
	// Specification-based queries
	FindBySpecification(ctx context.Context, spec Specification, opts ...QueryOption) ([]domain.Category, error)
	CountBySpecification(ctx context.Context, spec Specification) (int, error)
}

// Writer Interfaces - Optimized for Consistency and Validation

// NodeWriter handles write operations for nodes
// This interface is optimized for consistency, validation, and event publishing
type NodeWriter interface {
	// Create operations
	Save(ctx context.Context, node *domain.Node) error
	SaveBatch(ctx context.Context, nodes []*domain.Node) error
	
	// Update operations
	Update(ctx context.Context, node *domain.Node) error
	UpdateBatch(ctx context.Context, nodes []*domain.Node) error
	
	// Delete operations
	Delete(ctx context.Context, id domain.NodeID) error
	DeleteBatch(ctx context.Context, ids []domain.NodeID) error
	
	// Soft delete operations (archiving)
	Archive(ctx context.Context, id domain.NodeID) error
	Unarchive(ctx context.Context, id domain.NodeID) error
	
	// Version management for optimistic locking
	UpdateVersion(ctx context.Context, id domain.NodeID, expectedVersion domain.Version) error
}

// EdgeWriter handles write operations for edges
type EdgeWriter interface {
	// Create operations
	Save(ctx context.Context, edge *domain.Edge) error
	SaveBatch(ctx context.Context, edges []*domain.Edge) error
	
	// Update operations (edges are typically immutable, but weight can change)
	UpdateWeight(ctx context.Context, id domain.NodeID, newWeight float64, expectedVersion domain.Version) error
	
	// Delete operations
	Delete(ctx context.Context, id domain.NodeID) error
	DeleteBatch(ctx context.Context, ids []domain.NodeID) error
	DeleteByNode(ctx context.Context, nodeID domain.NodeID) error // Delete all edges for a node
	
	// Bulk operations for performance
	SaveManyToOne(ctx context.Context, sourceID domain.NodeID, targetIDs []domain.NodeID, weights []float64) error
	SaveOneToMany(ctx context.Context, sourceIDs []domain.NodeID, targetID domain.NodeID, weights []float64) error
}

// CategoryWriter handles write operations for categories
type CategoryWriter interface {
	// Create operations
	Save(ctx context.Context, category *domain.Category) error
	SaveBatch(ctx context.Context, categories []*domain.Category) error
	
	// Update operations
	Update(ctx context.Context, category *domain.Category) error
	UpdateBatch(ctx context.Context, categories []*domain.Category) error
	
	// Delete operations
	Delete(ctx context.Context, userID string, categoryID string) error
	DeleteBatch(ctx context.Context, userID string, categoryIDs []string) error
	DeleteHierarchy(ctx context.Context, userID string, categoryID string) error // Delete category and all children
	
	// Hierarchy operations
	CreateHierarchy(ctx context.Context, hierarchy domain.CategoryHierarchy) error
	DeleteHierarchyRelation(ctx context.Context, userID string, parentID string, childID string) error
	
	// Node-Category mapping operations
	AssignNodeToCategory(ctx context.Context, mapping domain.NodeCategory) error
	RemoveNodeFromCategory(ctx context.Context, userID string, nodeID string, categoryID string) error
	BatchAssignNodes(ctx context.Context, mappings []domain.NodeCategory) error
	
	// Maintenance operations
	UpdateNoteCounts(ctx context.Context, userID string) error
	RecalculateHierarchy(ctx context.Context, userID string) error
}

// Combined Interfaces - For Backward Compatibility and Convenience

// CQRSNodeRepository combines read and write operations for nodes with CQRS support
type CQRSNodeRepository interface {
	NodeReader
	NodeWriter
}

// CQRSEdgeRepository combines read and write operations for edges with CQRS support
type CQRSEdgeRepository interface {
	EdgeReader
	EdgeWriter
}

// CQRSCategoryRepository combines read and write operations for categories with CQRS support
type CQRSCategoryRepository interface {
	CategoryReader
	CategoryWriter
}

// Repository Aggregates for Complex Operations

// GraphReader provides read access to graph data across multiple entities
type GraphReader interface {
	// Full graph operations
	GetGraphData(ctx context.Context, userID domain.UserID) (*domain.Graph, error)
	GetGraphDataFiltered(ctx context.Context, userID domain.UserID, spec Specification) (*domain.Graph, error)
	GetGraphDataPaginated(ctx context.Context, userID domain.UserID, pagination Pagination) (*domain.Graph, string, error)
	
	// Neighborhood operations
	GetNeighborhood(ctx context.Context, nodeID domain.NodeID, depth int) (*domain.Graph, error)
	GetConnectedComponents(ctx context.Context, userID domain.UserID) ([]domain.Graph, error)
	
	// Analytics operations
	GetNodeStatistics(ctx context.Context, userID domain.UserID) (*NodeStatistics, error)
	GetConnectionStatistics(ctx context.Context, userID domain.UserID) (*ConnectionStatistics, error)
}

// TransactionalWriter provides atomic write operations across multiple repositories
type TransactionalWriter interface {
	// Atomic operations across entities
	CreateNodeWithEdges(ctx context.Context, node *domain.Node, edges []*domain.Edge) error
	DeleteNodeAndEdges(ctx context.Context, nodeID domain.NodeID) error
	MergeNodes(ctx context.Context, sourceID, targetID domain.NodeID) error
	
	// Category operations with consistency
	MoveCategoryWithNodes(ctx context.Context, categoryID string, newParentID string) error
	DeleteCategoryAndReassignNodes(ctx context.Context, categoryID string, newCategoryID string) error
}

// Statistics Types for Analytics

// NodeStatistics provides analytics about nodes
type NodeStatistics struct {
	TotalNodes     int                    `json:"total_nodes"`
	ArchivedNodes  int                    `json:"archived_nodes"`
	AverageWordCount float64             `json:"average_word_count"`
	MostUsedKeywords []KeywordCount      `json:"most_used_keywords"`
	CreationTrend  []DateCount          `json:"creation_trend"`
	TagDistribution []TagCount          `json:"tag_distribution"`
}

// ConnectionStatistics provides analytics about edges
type ConnectionStatistics struct {
	TotalEdges        int                `json:"total_edges"`
	AverageConnections float64          `json:"average_connections"`
	StrongConnections int               `json:"strong_connections"`
	WeakConnections   int               `json:"weak_connections"`
	ConnectionDensity float64           `json:"connection_density"`
	MostConnectedNodes []NodeConnection `json:"most_connected_nodes"`
}

// Supporting types for statistics
type KeywordCount struct {
	Keyword string `json:"keyword"`
	Count   int    `json:"count"`
}

type DateCount struct {
	Date  string `json:"date"`
	Count int    `json:"count"`
}

type TagCount struct {
	Tag   string `json:"tag"`
	Count int    `json:"count"`
}

type NodeConnection struct {
	NodeID          string `json:"node_id"`
	ConnectionCount int    `json:"connection_count"`
	AverageWeight   float64 `json:"average_weight"`
}

// Advanced Query Options

// QueryOption represents functional options for queries
type QueryOption func(*QueryOptions)

// QueryOptions contains all possible query configuration
type QueryOptions struct {
	// Pagination
	Limit  int
	Offset int
	Cursor string
	
	// Sorting
	SortBy    string
	SortOrder SortOrder
	
	// Filtering
	Filters []Filter
	
	// Include/Exclude options
	IncludeArchived bool
	IncludeDeleted  bool
	
	// Performance options
	UseCache      bool
	CacheTimeout  int // seconds
	ReadPreference ReadPreference
	
	// Projection (which fields to include)
	Fields []string
	
	// Aggregation options
	GroupBy []string
}

// SortOrder represents sorting direction
type SortOrder string

const (
	SortOrderAsc  SortOrder = "asc"
	SortOrderDesc SortOrder = "desc"
)

// ReadPreference represents where to read from
type ReadPreference string

const (
	ReadPreferencePrimary           ReadPreference = "primary"
	ReadPreferenceSecondary         ReadPreference = "secondary"
	ReadPreferenceSecondaryPreferred ReadPreference = "secondary_preferred"
)

// Functional options for building queries

// WithLimit sets the maximum number of results
func WithLimit(limit int) QueryOption {
	return func(opts *QueryOptions) {
		opts.Limit = limit
	}
}

// WithOffset sets the number of results to skip
func WithOffset(offset int) QueryOption {
	return func(opts *QueryOptions) {
		opts.Offset = offset
	}
}

// WithCursor sets the cursor for cursor-based pagination
func WithCursor(cursor string) QueryOption {
	return func(opts *QueryOptions) {
		opts.Cursor = cursor
	}
}

// WithSort sets the sorting field and direction
func WithSort(field string, order SortOrder) QueryOption {
	return func(opts *QueryOptions) {
		opts.SortBy = field
		opts.SortOrder = order
	}
}

// WithFilter adds a filter to the query
func WithFilter(filter Filter) QueryOption {
	return func(opts *QueryOptions) {
		opts.Filters = append(opts.Filters, filter)
	}
}

// WithSpecification adds a specification filter to the query
func WithSpecification(spec Specification) QueryOption {
	return func(opts *QueryOptions) {
		opts.Filters = append(opts.Filters, spec.ToFilter())
	}
}

// IncludeArchived includes archived entities in results
func IncludeArchived() QueryOption {
	return func(opts *QueryOptions) {
		opts.IncludeArchived = true
	}
}

// WithCache enables caching for the query
func WithCache(timeoutSeconds int) QueryOption {
	return func(opts *QueryOptions) {
		opts.UseCache = true
		opts.CacheTimeout = timeoutSeconds
	}
}

// WithReadPreference sets the read preference
func WithReadPreference(preference ReadPreference) QueryOption {
	return func(opts *QueryOptions) {
		opts.ReadPreference = preference
	}
}

// WithFields specifies which fields to include in the result
func WithFields(fields ...string) QueryOption {
	return func(opts *QueryOptions) {
		opts.Fields = fields
	}
}

// WithGroupBy adds grouping to the query
func WithGroupBy(fields ...string) QueryOption {
	return func(opts *QueryOptions) {
		opts.GroupBy = fields
	}
}

// Helper function to apply query options
func ApplyQueryOptions(opts ...QueryOption) *QueryOptions {
	options := &QueryOptions{
		Limit:           50,  // Default limit
		SortOrder:       SortOrderDesc,
		ReadPreference:  ReadPreferencePrimary,
		UseCache:        false,
		IncludeArchived: false,
		IncludeDeleted:  false,
		Filters:         make([]Filter, 0),
	}
	
	for _, opt := range opts {
		opt(options)
	}
	
	return options
}