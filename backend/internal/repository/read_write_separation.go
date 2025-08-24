package repository

import (
	"context"

	"brain2-backend/internal/domain/node"
	"brain2-backend/internal/domain/edge"
	"brain2-backend/internal/domain/category"
	"brain2-backend/internal/domain/shared"
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
	// Single entity queries - now with explicit userID for security
	FindByID(ctx context.Context, userID shared.UserID, nodeID shared.NodeID) (*node.Node, error)
	Exists(ctx context.Context, userID shared.UserID, nodeID shared.NodeID) (bool, error)
	
	// User-scoped queries
	FindByUser(ctx context.Context, userID shared.UserID, opts ...QueryOption) ([]*node.Node, error)
	CountByUser(ctx context.Context, userID shared.UserID) (int, error)
	
	// Content-based queries
	FindByKeywords(ctx context.Context, userID shared.UserID, keywords []string, opts ...QueryOption) ([]*node.Node, error)
	FindByTags(ctx context.Context, userID shared.UserID, tags []string, opts ...QueryOption) ([]*node.Node, error)
	FindByContent(ctx context.Context, userID shared.UserID, searchTerm string, fuzzy bool, opts ...QueryOption) ([]*node.Node, error)
	
	// Time-based queries
	FindRecentlyCreated(ctx context.Context, userID shared.UserID, days int, opts ...QueryOption) ([]*node.Node, error)
	FindRecentlyUpdated(ctx context.Context, userID shared.UserID, days int, opts ...QueryOption) ([]*node.Node, error)
	
	// Specification-based queries
	FindBySpecification(ctx context.Context, spec Specification, opts ...QueryOption) ([]*node.Node, error)
	CountBySpecification(ctx context.Context, spec Specification) (int, error)
	
	// Paginated queries
	FindPage(ctx context.Context, query NodeQuery, pagination Pagination) (*NodePage, error)
	
	// Relationship queries - with explicit userID for validation
	FindConnected(ctx context.Context, userID shared.UserID, nodeID shared.NodeID, depth int, opts ...QueryOption) ([]*node.Node, error)
	FindSimilar(ctx context.Context, userID shared.UserID, nodeID shared.NodeID, threshold float64, opts ...QueryOption) ([]*node.Node, error)
	
	// Query service compatibility methods
	GetNodesPage(ctx context.Context, query NodeQuery, pagination Pagination) (*NodePage, error)
	CountNodes(ctx context.Context, userID string) (int, error)
}

// EdgeReader handles read-only operations for edges
type EdgeReader interface {
	// Single entity queries - now with explicit userID
	FindByID(ctx context.Context, userID shared.UserID, edgeID shared.NodeID) (*edge.Edge, error)
	Exists(ctx context.Context, userID shared.UserID, edgeID shared.NodeID) (bool, error)
	
	// User-scoped queries
	FindByUser(ctx context.Context, userID shared.UserID, opts ...QueryOption) ([]*edge.Edge, error)
	CountByUser(ctx context.Context, userID shared.UserID) (int, error)
	
	// Node relationship queries - with userID for validation
	FindBySourceNode(ctx context.Context, userID shared.UserID, sourceID shared.NodeID, opts ...QueryOption) ([]*edge.Edge, error)
	FindByTargetNode(ctx context.Context, userID shared.UserID, targetID shared.NodeID, opts ...QueryOption) ([]*edge.Edge, error)
	FindByNode(ctx context.Context, userID shared.UserID, nodeID shared.NodeID, opts ...QueryOption) ([]*edge.Edge, error)
	FindBetweenNodes(ctx context.Context, userID shared.UserID, node1ID, node2ID shared.NodeID) ([]*edge.Edge, error)
	
	// Weight-based queries
	FindStrongConnections(ctx context.Context, userID shared.UserID, threshold float64, opts ...QueryOption) ([]*edge.Edge, error)
	FindWeakConnections(ctx context.Context, userID shared.UserID, threshold float64, opts ...QueryOption) ([]*edge.Edge, error)
	
	// Specification-based queries
	FindBySpecification(ctx context.Context, spec Specification, opts ...QueryOption) ([]*edge.Edge, error)
	CountBySpecification(ctx context.Context, spec Specification) (int, error)
	
	// Paginated queries
	FindPage(ctx context.Context, query EdgeQuery, pagination Pagination) (*EdgePage, error)
	
	// Query service compatibility methods
	FindEdges(ctx context.Context, query EdgeQuery) ([]*edge.Edge, error)
	CountBySourceID(ctx context.Context, sourceID shared.NodeID) (int, error)
}

// CategoryReader handles read-only operations for categories
type CategoryReader interface {
	// Single entity queries
	FindByID(ctx context.Context, userID string, categoryID string) (*category.Category, error)
	Exists(ctx context.Context, userID string, categoryID string) (bool, error)
	
	// User-scoped queries
	FindByUser(ctx context.Context, userID string, opts ...QueryOption) ([]category.Category, error)
	CountByUser(ctx context.Context, userID string) (int, error)
	
	// Hierarchy queries
	FindRootCategories(ctx context.Context, userID string, opts ...QueryOption) ([]category.Category, error)
	FindChildCategories(ctx context.Context, userID string, parentID string) ([]category.Category, error)
	FindCategoryPath(ctx context.Context, userID string, categoryID string) ([]category.Category, error)
	FindCategoryTree(ctx context.Context, userID string) ([]category.Category, error)
	
	// Level-based queries
	FindByLevel(ctx context.Context, userID string, level int, opts ...QueryOption) ([]category.Category, error)
	
	// Activity queries
	FindMostActive(ctx context.Context, userID string, limit int) ([]category.Category, error)
	FindRecentlyUsed(ctx context.Context, userID string, days int, opts ...QueryOption) ([]category.Category, error)
	
	// Specification-based queries
	FindBySpecification(ctx context.Context, spec Specification, opts ...QueryOption) ([]category.Category, error)
	CountBySpecification(ctx context.Context, spec Specification) (int, error)
	
	// Query service compatibility methods
	GetCategoriesPage(ctx context.Context, query CategoryQuery, pagination Pagination) (*CategoryPage, error)
	CountCategories(ctx context.Context, userID string) (int, error)
}

// Writer Interfaces - Optimized for Consistency and Validation

// NodeWriter handles write operations for nodes
// This interface is optimized for consistency, validation, and event publishing
type NodeWriter interface {
	// Create operations - node already contains userID
	Save(ctx context.Context, node *node.Node) error
	SaveBatch(ctx context.Context, nodes []*node.Node) error
	
	// Update operations - node already contains userID for validation
	Update(ctx context.Context, node *node.Node) error
	UpdateBatch(ctx context.Context, nodes []*node.Node) error
	
	// Delete operations - now with explicit userID for security
	Delete(ctx context.Context, userID shared.UserID, nodeID shared.NodeID) error
	DeleteBatch(ctx context.Context, userID shared.UserID, nodeIDs []shared.NodeID) error
	
	// Soft delete operations (archiving) - with explicit userID
	Archive(ctx context.Context, userID shared.UserID, nodeID shared.NodeID) error
	Unarchive(ctx context.Context, userID shared.UserID, nodeID shared.NodeID) error
	
	// Version management for optimistic locking - with userID for validation
	UpdateVersion(ctx context.Context, userID shared.UserID, nodeID shared.NodeID, expectedVersion shared.Version) error
}

// EdgeWriter handles write operations for edges
type EdgeWriter interface {
	// Create operations - edge already contains userID implicitly
	Save(ctx context.Context, edge *edge.Edge) error
	SaveBatch(ctx context.Context, edges []*edge.Edge) error
	
	// Update operations (edges are typically immutable, but weight can change) - with explicit userID
	UpdateWeight(ctx context.Context, userID shared.UserID, edgeID shared.NodeID, newWeight float64, expectedVersion shared.Version) error
	
	// Delete operations - with explicit userID
	Delete(ctx context.Context, userID shared.UserID, edgeID shared.NodeID) error
	DeleteBatch(ctx context.Context, userID shared.UserID, edgeIDs []shared.NodeID) error
	DeleteByNode(ctx context.Context, userID shared.UserID, nodeID shared.NodeID) error // Delete all edges for a node
	
	// Bulk operations for performance - with explicit userID
	SaveManyToOne(ctx context.Context, userID shared.UserID, sourceID shared.NodeID, targetIDs []shared.NodeID, weights []float64) error
	SaveOneToMany(ctx context.Context, userID shared.UserID, sourceIDs []shared.NodeID, targetID shared.NodeID, weights []float64) error
}

// CategoryWriter handles write operations for categories
type CategoryWriter interface {
	// Create operations
	Save(ctx context.Context, category *category.Category) error
	SaveBatch(ctx context.Context, categories []*category.Category) error
	
	// Update operations
	Update(ctx context.Context, category *category.Category) error
	UpdateBatch(ctx context.Context, categories []*category.Category) error
	
	// Delete operations
	Delete(ctx context.Context, userID string, categoryID string) error
	DeleteBatch(ctx context.Context, userID string, categoryIDs []string) error
	DeleteHierarchy(ctx context.Context, userID string, categoryID string) error // Delete category and all children
	
	// Hierarchy operations
	CreateHierarchy(ctx context.Context, hierarchy category.CategoryHierarchy) error
	DeleteHierarchyRelation(ctx context.Context, userID string, parentID string, childID string) error
	
	// Node-Category mapping operations
	AssignNodeToCategory(ctx context.Context, mapping node.NodeCategory) error
	RemoveNodeFromCategory(ctx context.Context, userID string, nodeID string, categoryID string) error
	BatchAssignNodes(ctx context.Context, mappings []node.NodeCategory) error
	
	// Maintenance operations
	UpdateNoteCounts(ctx context.Context, userID string) error
	RecalculateHierarchy(ctx context.Context, userID string) error
}

// Repository Aggregates for Complex Operations

// GraphReader provides read access to graph data across multiple entities
type GraphReader interface {
	// Full graph operations
	GetGraphData(ctx context.Context, userID shared.UserID) (*shared.Graph, error)
	GetGraphDataFiltered(ctx context.Context, userID shared.UserID, spec Specification) (*shared.Graph, error)
	GetGraphDataPaginated(ctx context.Context, userID shared.UserID, pagination Pagination) (*shared.Graph, string, error)
	
	// Neighborhood operations
	GetNeighborhood(ctx context.Context, nodeID shared.NodeID, depth int) (*shared.Graph, error)
	GetConnectedComponents(ctx context.Context, userID shared.UserID) ([]shared.Graph, error)
	
	// Analytics operations
	GetNodeStatistics(ctx context.Context, userID shared.UserID) (*NodeStatistics, error)
	GetConnectionStatistics(ctx context.Context, userID shared.UserID) (*ConnectionStatistics, error)
}

// TransactionalWriter provides atomic write operations across multiple repositories
type TransactionalWriter interface {
	// Atomic operations across entities
	CreateNodeWithEdges(ctx context.Context, node *node.Node, edges []*edge.Edge) error
	DeleteNodeAndEdges(ctx context.Context, userID shared.UserID, nodeID shared.NodeID) error
	MergeNodes(ctx context.Context, userID shared.UserID, sourceID, targetID shared.NodeID) error
	
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