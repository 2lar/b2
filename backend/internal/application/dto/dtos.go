// Package dto contains Data Transfer Objects for the application layer.
package dto

import (
	"time"

	"brain2-backend/internal/domain/category"
	"brain2-backend/internal/domain/edge"
	"brain2-backend/internal/domain/node"
)

// ============================================================================
// NODE DTOs
// ============================================================================

// NodeDTO represents a node for external consumption.
type NodeDTO struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	Title     string    `json:"title"`
	Content   string    `json:"content"`
	Keywords  []string  `json:"keywords"`
	Tags      []string  `json:"tags"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Version   int       `json:"version"`
	Archived  bool      `json:"archived"`
}

// NodeFromDomain converts a domain node to a DTO.
func NodeFromDomain(n *node.Node) *NodeDTO {
	if n == nil {
		return nil
	}
	return &NodeDTO{
		ID:        n.ID().String(),
		UserID:    n.UserID().String(),
		Title:     n.Title().String(),
		Content:   n.Content().String(),
		Keywords:  n.Keywords().ToSlice(),
		Tags:      n.Tags().ToSlice(),
		CreatedAt: n.CreatedAt(),
		UpdatedAt: n.UpdatedAt(),
		Version:   n.Version(),
		Archived:  n.IsArchived(),
	}
}

// CreateNodeResult represents the result of creating a node.
type CreateNodeResult struct {
	Node             *NodeDTO   `json:"node"`
	SuggestedEdges   []*EdgeDTO `json:"suggested_edges,omitempty"`
	CreatedEdges     []*EdgeDTO `json:"created_edges,omitempty"`
	AssignedCategory string     `json:"assigned_category,omitempty"`
}

// NodeListResult represents a paginated list of nodes.
type NodeListResult struct {
	Nodes      []*NodeDTO `json:"nodes"`
	NextCursor string     `json:"next_cursor,omitempty"`
	HasMore    bool       `json:"has_more"`
	TotalCount int        `json:"total_count"`
}

// NodeSearchResult represents node search results.
type NodeSearchResult struct {
	Nodes      []*NodeDTO `json:"nodes"`
	TotalCount int        `json:"total_count"`
	Facets     map[string][]FacetValue `json:"facets,omitempty"`
}

// NodeWithRelationsDTO includes a node with its relationships.
type NodeWithRelationsDTO struct {
	*NodeDTO
	IncomingEdges []*EdgeDTO `json:"incoming_edges"`
	OutgoingEdges []*EdgeDTO `json:"outgoing_edges"`
	Categories    []string   `json:"categories"`
}

// NodeStatistics represents node analytics.
type NodeStatistics struct {
	EdgeCount      int     `json:"edge_count"`
	InDegree       int     `json:"in_degree"`
	OutDegree      int     `json:"out_degree"`
	Centrality     float64 `json:"centrality"`
	ClusteringCoef float64 `json:"clustering_coefficient"`
}

// ============================================================================
// CATEGORY DTOs
// ============================================================================

// CategoryDTO represents a category for external consumption.
type CategoryDTO struct {
	ID          string     `json:"id"`
	UserID      string     `json:"user_id"`
	Name        string     `json:"name"`
	Description string     `json:"description,omitempty"`
	ParentID    *string    `json:"parent_id,omitempty"`
	Level       int        `json:"level"`
	NodeCount   int        `json:"node_count"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

// CategoryFromDomain converts a domain category to a DTO.
func CategoryFromDomain(c category.Category) *CategoryDTO {
	var parentID *string
	if c.ParentID != nil {
		s := string(*c.ParentID)
		parentID = &s
	}
	
	return &CategoryDTO{
		ID:          string(c.ID),
		UserID:      c.UserID,
		Name:        c.Name,
		Description: c.Description,
		ParentID:    parentID,
		Level:       c.Level,
		NodeCount:   0, // Will be populated from repository
		CreatedAt:   c.CreatedAt,
		UpdatedAt:   c.UpdatedAt,
	}
}

// CategoryTreeNode represents a category in a tree structure.
type CategoryTreeNode struct {
	*CategoryDTO
	Children []*CategoryTreeNode `json:"children"`
}

// CategoryWithNodesDTO includes a category with its nodes.
type CategoryWithNodesDTO struct {
	*CategoryDTO
	Nodes []*NodeDTO `json:"nodes"`
}

// CategoryStatistics represents category analytics.
type CategoryStatistics struct {
	NodeCount        int `json:"node_count"`
	SubcategoryCount int `json:"subcategory_count"`
	TotalNodeCount   int `json:"total_node_count"` // Including subcategories
	Depth            int `json:"depth"`
}

// ============================================================================
// EDGE DTOs
// ============================================================================

// EdgeDTO represents an edge for external consumption.
type EdgeDTO struct {
	ID           string    `json:"id"`
	UserID       string    `json:"user_id"`
	SourceNodeID string    `json:"source_node_id"`
	TargetNodeID string    `json:"target_node_id"`
	Weight       float64   `json:"weight"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// EdgeFromDomain converts a domain edge to a DTO.
func EdgeFromDomain(e *edge.Edge) *EdgeDTO {
	if e == nil {
		return nil
	}
	return &EdgeDTO{
		ID:           e.ID.String(), // Use public field
		UserID:       e.UserID().String(),
		SourceNodeID: e.Source().String(),
		TargetNodeID: e.Target().String(),
		Weight:       e.Weight(),
		CreatedAt:    e.CreatedAt, // Use public field
		UpdatedAt:    e.UpdatedAt, // Use public field
	}
}

// ============================================================================
// GRAPH DTOs
// ============================================================================

// GraphDTO represents a graph structure.
type GraphDTO struct {
	Nodes []*NodeDTO `json:"nodes"`
	Edges []*EdgeDTO `json:"edges"`
	Stats GraphStats `json:"stats,omitempty"`
}

// GraphStats represents graph statistics.
type GraphStats struct {
	NodeCount int     `json:"node_count"`
	EdgeCount int     `json:"edge_count"`
	Density   float64 `json:"density"`
	Diameter  int     `json:"diameter,omitempty"`
}

// PathDTO represents a path through the graph.
type PathDTO struct {
	Nodes      []string `json:"nodes"`
	Edges      []string `json:"edges"`
	TotalWeight float64 `json:"total_weight"`
	Length     int     `json:"length"`
}

// ComponentDTO represents a connected component.
type ComponentDTO struct {
	ID        string     `json:"id"`
	NodeIDs   []string   `json:"node_ids"`
	EdgeIDs   []string   `json:"edge_ids"`
	Size      int        `json:"size"`
	Density   float64    `json:"density"`
}

// CentralityMetrics represents node centrality measures.
type CentralityMetrics struct {
	Degree      float64 `json:"degree"`
	Betweenness float64 `json:"betweenness"`
	Closeness   float64 `json:"closeness"`
	Eigenvector float64 `json:"eigenvector"`
}

// GraphLayout represents a graph layout for visualization.
type GraphLayout struct {
	Nodes []NodePosition `json:"nodes"`
	Algorithm string      `json:"algorithm"`
}

// NodePosition represents a node's position in a layout.
type NodePosition struct {
	NodeID string  `json:"node_id"`
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Z      float64 `json:"z,omitempty"`
}

// GraphStatistics represents comprehensive graph analytics.
type GraphStatistics struct {
	NodeCount          int     `json:"node_count"`
	EdgeCount          int     `json:"edge_count"`
	Density            float64 `json:"density"`
	AverageDegree      float64 `json:"average_degree"`
	ClusteringCoef     float64 `json:"clustering_coefficient"`
	ConnectedComponents int     `json:"connected_components"`
	Diameter           int     `json:"diameter"`
	Radius             int     `json:"radius"`
}

// ============================================================================
// COMMON DTOs
// ============================================================================

// FacetValue represents a facet value in search results.
type FacetValue struct {
	Value string `json:"value"`
	Count int    `json:"count"`
}

// DomainEventDTO represents a domain event.
type DomainEventDTO struct {
	ID            string                 `json:"id"`
	Type          string                 `json:"type"`
	AggregateID   string                 `json:"aggregate_id"`
	AggregateType string                 `json:"aggregate_type"`
	UserID        string                 `json:"user_id"`
	Timestamp     time.Time              `json:"timestamp"`
	Version       int                    `json:"version"`
	Data          map[string]interface{} `json:"data"`
}