// Package repository demonstrates Repository Pattern Excellence using Interface Segregation Principle.
//
// This package showcases:
//   - Interface Segregation: Small, focused interfaces instead of large ones
//   - Functional Options: Flexible query configuration
//   - Query Builder: Composable query construction
//   - Repository Composition: Combining read/write interfaces
//
// Educational Goals:
//   - Show how to properly segregate repository responsibilities
//   - Demonstrate clean abstraction over data access
//   - Illustrate enterprise-grade repository patterns
package repository

import (
	"brain2-backend/internal/domain"
	"context"
	"time"
)

// NodeReader handles read-only operations for nodes.
// This interface follows the Interface Segregation Principle by containing
// only read operations, allowing clients to depend only on what they need.
type NodeReader interface {
	// FindByID retrieves a single node by its ID
	FindByID(ctx context.Context, id domain.NodeID) (*domain.Node, error)
	
	// FindByUser retrieves nodes for a specific user with optional filtering
	FindByUser(ctx context.Context, userID domain.UserID, opts ...QueryOption) ([]*domain.Node, error)
	
	// Exists checks if a node exists without loading it
	Exists(ctx context.Context, id domain.NodeID) (bool, error)
	
	// Count returns the total number of nodes for a user
	Count(ctx context.Context, userID domain.UserID, opts ...QueryOption) (int, error)
	
	// FindByKeywords finds nodes containing specific keywords
	FindByKeywords(ctx context.Context, userID domain.UserID, keywords []string, opts ...QueryOption) ([]*domain.Node, error)
	
	// FindSimilar finds nodes similar to the given node
	FindSimilar(ctx context.Context, node *domain.Node, opts ...QueryOption) ([]*domain.Node, error)
}

// NodeWriter handles write operations for nodes.
// Separate interface ensures read-only clients cannot accidentally modify data.
type NodeWriter interface {
	// Save creates or updates a node
	Save(ctx context.Context, node *domain.Node) error
	
	// Delete removes a node
	Delete(ctx context.Context, id domain.NodeID) error
	
	// SaveBatch saves multiple nodes efficiently
	SaveBatch(ctx context.Context, nodes []*domain.Node) error
	
	// DeleteBatch removes multiple nodes efficiently
	DeleteBatch(ctx context.Context, ids []domain.NodeID) error
}

// NodeRepository combines read and write operations.
// This composition pattern allows clients to depend on exactly what they need:
// - Read-only operations can depend on NodeReader
// - Write operations can depend on NodeWriter  
// - Full CRUD operations can depend on NodeRepository
type NodeRepository interface {
	NodeReader
	NodeWriter
}

// EdgeReader handles read-only operations for edges
type EdgeReader interface {
	// FindByNodes finds edges between specific nodes
	FindByNodes(ctx context.Context, sourceID, targetID domain.NodeID) (*domain.Edge, error)
	
	// FindBySource finds all edges originating from a node
	FindBySource(ctx context.Context, sourceID domain.NodeID, opts ...QueryOption) ([]*domain.Edge, error)
	
	// FindByTarget finds all edges pointing to a node
	FindByTarget(ctx context.Context, targetID domain.NodeID, opts ...QueryOption) ([]*domain.Edge, error)
	
	// FindByUser finds all edges for a user
	FindByUser(ctx context.Context, userID domain.UserID, opts ...QueryOption) ([]*domain.Edge, error)
	
	// GetNeighborhood retrieves the graph neighborhood around a node
	GetNeighborhood(ctx context.Context, nodeID domain.NodeID, depth int) ([]*domain.Edge, error)
}

// EdgeWriter handles write operations for edges
type EdgeWriter interface {
	// Save creates or updates an edge
	Save(ctx context.Context, edge *domain.Edge) error
	
	// Delete removes an edge
	Delete(ctx context.Context, sourceID, targetID domain.NodeID) error
	
	// SaveBatch saves multiple edges efficiently
	SaveBatch(ctx context.Context, edges []*domain.Edge) error
	
	// DeleteByNode removes all edges connected to a node
	DeleteByNode(ctx context.Context, nodeID domain.NodeID) error
}

// EdgeRepository combines read and write operations for edges
type EdgeRepository interface {
	EdgeReader
	EdgeWriter
}

// KeywordSearcher handles keyword-based search operations.
// This focused interface allows for specialized keyword search implementations.
type KeywordSearcher interface {
	// SearchNodes finds nodes matching the given keywords
	SearchNodes(ctx context.Context, userID domain.UserID, keywords []string, opts ...QueryOption) ([]*domain.Node, error)
	
	// SuggestKeywords provides keyword suggestions based on existing content
	SuggestKeywords(ctx context.Context, userID domain.UserID, partial string, limit int) ([]string, error)
	
	// FindRelatedByKeywords finds nodes with similar keywords
	FindRelatedByKeywords(ctx context.Context, userID domain.UserID, node *domain.Node, opts ...QueryOption) ([]*domain.Node, error)
}

// GraphReader handles read operations for graph-wide queries
type GraphReader interface {
	// GetGraph retrieves the complete graph for a user
	GetGraph(ctx context.Context, userID domain.UserID, opts ...QueryOption) (*domain.Graph, error)
	
	// GetSubgraph retrieves a subgraph around specific nodes
	GetSubgraph(ctx context.Context, nodeIDs []domain.NodeID, depth int) (*domain.Graph, error)
	
	// AnalyzeConnectivity provides graph connectivity analysis
	AnalyzeConnectivity(ctx context.Context, userID domain.UserID) (*GraphAnalysis, error)
}

// GraphAnalysis contains graph connectivity metrics
type GraphAnalysis struct {
	TotalNodes       int     `json:"total_nodes"`
	TotalEdges       int     `json:"total_edges"`
	ConnectedClusters int     `json:"connected_clusters"`
	Density          float64 `json:"density"`
	AverageConnections float64 `json:"average_connections"`
}

// CategoryReader handles read operations for categories
type CategoryReader interface {
	// FindByID retrieves a single category
	FindByID(ctx context.Context, userID domain.UserID, categoryID string) (*domain.Category, error)
	
	// FindByUser retrieves categories for a user
	FindByUser(ctx context.Context, userID domain.UserID, opts ...QueryOption) ([]domain.Category, error)
	
	// FindByLevel retrieves categories at a specific hierarchy level
	FindByLevel(ctx context.Context, userID domain.UserID, level int) ([]domain.Category, error)
	
	// GetTree retrieves the complete category tree for a user
	GetTree(ctx context.Context, userID domain.UserID) ([]domain.Category, error)
	
	// FindChildren retrieves child categories
	FindChildren(ctx context.Context, userID domain.UserID, parentID string) ([]domain.Category, error)
	
	// FindParent retrieves the parent category
	FindParent(ctx context.Context, userID domain.UserID, childID string) (*domain.Category, error)
}

// CategoryWriter handles write operations for categories
type CategoryWriter interface {
	// Save creates or updates a category
	Save(ctx context.Context, category *domain.Category) error
	
	// Delete removes a category
	Delete(ctx context.Context, userID domain.UserID, categoryID string) error
	
	// CreateHierarchy creates a parent-child relationship
	CreateHierarchy(ctx context.Context, hierarchy *domain.CategoryHierarchy) error
	
	// DeleteHierarchy removes a parent-child relationship
	DeleteHierarchy(ctx context.Context, userID domain.UserID, parentID, childID string) error
}

// CategoryRepository combines read and write operations for categories
type CategoryRepository interface {
	CategoryReader
	CategoryWriter
}

// NodeCategoryMapper handles relationships between nodes and categories
type NodeCategoryMapper interface {
	// AssignNodeToCategory creates a node-category relationship
	AssignNodeToCategory(ctx context.Context, mapping *domain.NodeCategory) error
	
	// RemoveNodeFromCategory removes a node-category relationship
	RemoveNodeFromCategory(ctx context.Context, userID domain.UserID, nodeID, categoryID string) error
	
	// FindNodesByCategory retrieves nodes in a category
	FindNodesByCategory(ctx context.Context, userID domain.UserID, categoryID string, opts ...QueryOption) ([]*domain.Node, error)
	
	// FindCategoriesForNode retrieves categories for a node
	FindCategoriesForNode(ctx context.Context, userID domain.UserID, nodeID string) ([]*domain.Category, error)
	
	// BatchAssignCategories assigns multiple categories efficiently
	BatchAssignCategories(ctx context.Context, mappings []*domain.NodeCategory) error
}

// QueryOption implements the functional options pattern for flexible query configuration.
// This pattern demonstrates how to create flexible APIs that can be extended without
// breaking existing code. Each option modifies the QueryOptions struct.
//
// Educational Benefits:
//   - Shows functional options pattern in practice
//   - Demonstrates how to create extensible APIs
//   - Illustrates clean configuration management
type QueryOption func(*QueryOptions)

// QueryOptions contains all possible query configuration parameters.
// This struct is modified by functional options to build complex queries.
type QueryOptions struct {
	// Pagination options
	Limit      int    `json:"limit,omitempty"`
	Offset     int    `json:"offset,omitempty"`
	Cursor     string `json:"cursor,omitempty"`
	
	// Sorting options
	OrderBy    string `json:"order_by,omitempty"`
	Descending bool   `json:"descending,omitempty"`
	
	// Filtering options
	Filters    []Filter      `json:"filters,omitempty"`
	DateRange  *DateRange    `json:"date_range,omitempty"`
	
	// Advanced options
	IncludeArchived bool `json:"include_archived,omitempty"`
	PreloadEdges    bool `json:"preload_edges,omitempty"`
}

// Filter represents a query filter condition
type Filter struct {
	Field     string      `json:"field"`
	Operator  string      `json:"operator"` // eq, ne, gt, lt, gte, lte, in, contains
	Value     interface{} `json:"value"`
}

// DateRange represents a date range filter
type DateRange struct {
	Start *time.Time `json:"start,omitempty"`
	End   *time.Time `json:"end,omitempty"`
}

// Functional options for common query patterns

// WithLimit sets the maximum number of results to return
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

// WithCursor sets cursor-based pagination
func WithCursor(cursor string) QueryOption {
	return func(opts *QueryOptions) {
		opts.Cursor = cursor
	}
}

// WithOrderBy sets the field to order by and direction
func WithOrderBy(field string, descending bool) QueryOption {
	return func(opts *QueryOptions) {
		opts.OrderBy = field
		opts.Descending = descending
	}
}

// WithFilter adds a filter condition
func WithFilter(field, operator string, value interface{}) QueryOption {
	return func(opts *QueryOptions) {
		opts.Filters = append(opts.Filters, Filter{
			Field:    field,
			Operator: operator,
			Value:    value,
		})
	}
}

// WithDateRange sets a date range filter
func WithDateRange(start, end *time.Time) QueryOption {
	return func(opts *QueryOptions) {
		opts.DateRange = &DateRange{
			Start: start,
			End:   end,
		}
	}
}

// WithIncludeArchived includes archived items in results
func WithIncludeArchived() QueryOption {
	return func(opts *QueryOptions) {
		opts.IncludeArchived = true
	}
}

// WithPreloadEdges includes edge relationships in the result
func WithPreloadEdges() QueryOption {
	return func(opts *QueryOptions) {
		opts.PreloadEdges = true
	}
}

// ApplyQueryOptions applies functional options and returns configured QueryOptions
func ApplyQueryOptions(opts ...QueryOption) *QueryOptions {
	return buildQueryOptions(opts)
}

// buildQueryOptions applies functional options and returns configured QueryOptions
func buildQueryOptions(opts []QueryOption) *QueryOptions {
	options := &QueryOptions{
		Limit:   50, // Default limit
		Filters: make([]Filter, 0),
	}
	
	for _, opt := range opts {
		opt(options)
	}
	
	return options
}