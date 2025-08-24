// Package repository provides focused repository interfaces following SOLID principles.
// This file provides additional specialized interfaces that complement the CQRS
// interfaces defined in read_write_separation.go.
package repository

import (
	"context"
	"time"
	"brain2-backend/internal/domain/node"
	"brain2-backend/internal/domain/edge"
	"brain2-backend/internal/domain/category"
	"brain2-backend/internal/domain/shared"
)

// ============================================================================
// SPECIALIZED SEARCH AND RELATIONSHIP INTERFACES
// ============================================================================

// NodeSearcher provides search capabilities for nodes.
// This interface is focused solely on search functionality and extends the basic NodeReader.
type NodeSearcher interface {
	// Keyword-based search
	FindNodesByKeywords(ctx context.Context, userID string, keywords []string) ([]*node.Node, error)
	
	// Content search
	FindNodesByContent(ctx context.Context, userID, content string) ([]*node.Node, error)
	
	// Advanced search with options
	Search(ctx context.Context, query NodeSearchQuery) ([]*node.Node, error)
}

// NodeRelationshipReader provides read access to node relationships.
// This interface focuses on relationship and graph operations.
type NodeRelationshipReader interface {
	// Neighborhood operations
	GetNodeNeighborhood(ctx context.Context, userID, nodeID string, depth int) (*shared.Graph, error)
	GetConnectedNodes(ctx context.Context, userID, nodeID string) ([]*node.Node, error)
	
	// Path operations
	FindShortestPath(ctx context.Context, userID, sourceID, targetID string) ([]*node.Node, error)
}

// EdgeRelationshipManager provides relationship management for edges.
type EdgeRelationshipManager interface {
	// Node-specific edge operations
	FindEdgesFromNode(ctx context.Context, userID, nodeID string) ([]*edge.Edge, error)
	FindEdgesToNode(ctx context.Context, userID, nodeID string) ([]*edge.Edge, error)
	DeleteAllNodeEdges(ctx context.Context, userID, nodeID string) error
}

// CategoryHierarchyManager provides hierarchy management for categories.
type CategoryHierarchyManager interface {
	// Hierarchy operations
	CreateCategoryHierarchy(ctx context.Context, hierarchy category.CategoryHierarchy) error
	DeleteCategoryHierarchy(ctx context.Context, userID, parentID, childID string) error
	FindChildCategories(ctx context.Context, userID, parentID string) ([]category.Category, error)
	FindParentCategory(ctx context.Context, userID, childID string) (*category.Category, error)
	GetCategoryTree(ctx context.Context, userID string) ([]category.Category, error)
}

// CategoryNodeMapper provides node-category mapping operations.
type CategoryNodeMapper interface {
	// Mapping operations
	AssignNodeToCategory(ctx context.Context, mapping node.NodeCategory) error
	RemoveNodeFromCategory(ctx context.Context, userID, nodeID, categoryID string) error
	FindNodesByCategory(ctx context.Context, userID, categoryID string) ([]*node.Node, error)
	FindCategoriesForNode(ctx context.Context, userID, nodeID string) ([]category.Category, error)
	
	// Batch operations
	BatchAssignCategories(ctx context.Context, mappings []node.NodeCategory) error
	UpdateCategoryNoteCounts(ctx context.Context, userID string, categoryCounts map[string]int) error
}

// ============================================================================
// DEPRECATED: COMPOSITE INTERFACES - TO BE REMOVED
// ============================================================================
// The following composite interfaces are deprecated and will be removed.
// Use specific Reader/Writer interfaces instead for true CQRS separation.
// Migration guide:
//   - Replace NodeRepositoryComposite with NodeReader + NodeWriter
//   - Replace EdgeRepositoryComposite with EdgeReader + EdgeWriter  
//   - Replace CategoryRepositoryComposite with CategoryReader + CategoryWriter
// ============================================================================

// ============================================================================
// SPECIALIZED QUERY INTERFACES
// ============================================================================

// NodeSearchQuery represents advanced search parameters for nodes.
type NodeSearchQuery struct {
	UserID      string
	Keywords    []string
	Content     string
	Tags        []string
	DateRange   *DateRange
	Limit       int
	Offset      int
	SortBy      string
	SortOrder   string
}

// DateRange represents a date range for queries.
type DateRange struct {
	Start time.Time
	End   time.Time
}

// ============================================================================
// REPOSITORY PROVIDER WITH FOCUSED INTERFACES
// ============================================================================

// FocusedRepositoryProvider provides access to focused repository interfaces.
// This provider encourages clients to depend only on the specific interfaces they need.
type FocusedRepositoryProvider interface {
	// CQRS repositories
	GetNodeReader() NodeReader
	GetNodeWriter() NodeWriter
	GetEdgeReader() EdgeReader
	GetEdgeWriter() EdgeWriter
	GetCategoryReader() CategoryReader
	GetCategoryWriter() CategoryWriter
	
	// Specialized repositories
	GetNodeSearcher() NodeSearcher
	GetNodeRelationshipReader() NodeRelationshipReader
	GetEdgeRelationshipManager() EdgeRelationshipManager
	GetCategoryHierarchyManager() CategoryHierarchyManager
	GetCategoryNodeMapper() CategoryNodeMapper
	
	// DEPRECATED: Composite repositories - use specific readers/writers instead
	// These will be removed in the next version
}