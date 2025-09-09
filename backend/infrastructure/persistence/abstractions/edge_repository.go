package abstractions

import (
	"backend/domain/core/valueobjects"
	"context"
)

// EdgeRepositoryAbstraction provides a database-agnostic interface for edge persistence
type EdgeRepositoryAbstraction interface {
	// Core CRUD operations
	Save(ctx context.Context, edge *Edge) error
	FindByID(ctx context.Context, edgeID string) (*Edge, error)
	Update(ctx context.Context, edge *Edge) error
	Delete(ctx context.Context, edgeID string) error

	// Query operations
	FindBySourceNode(ctx context.Context, nodeID valueobjects.NodeID) ([]*Edge, error)
	FindByTargetNode(ctx context.Context, nodeID valueobjects.NodeID) ([]*Edge, error)
	FindByNodes(ctx context.Context, sourceID, targetID valueobjects.NodeID) (*Edge, error)
	FindByGraphID(ctx context.Context, graphID string) ([]*Edge, error)

	// Batch operations
	SaveBatch(ctx context.Context, edges []*Edge) error
	DeleteBatch(ctx context.Context, edgeIDs []string) error
	DeleteByNode(ctx context.Context, nodeID valueobjects.NodeID) error

	// Graph operations
	GetNodeDegree(ctx context.Context, nodeID valueobjects.NodeID) (in int, out int, err error)
	GetShortestPath(ctx context.Context, sourceID, targetID valueobjects.NodeID) ([]*Edge, error)
	GetSubgraph(ctx context.Context, nodeID valueobjects.NodeID, depth int) ([]*Edge, error)

	// Statistics
	CountEdgesByGraph(ctx context.Context, graphID string) (int64, error)
	CountEdgesByType(ctx context.Context, graphID string) (map[string]int64, error)
}

// Edge represents a connection between two nodes
type Edge struct {
	ID        string
	GraphID   string
	SourceID  valueobjects.NodeID
	TargetID  valueobjects.NodeID
	Type      EdgeType
	Weight    float64
	Metadata  map[string]interface{}
	CreatedAt string
	UpdatedAt string
	CreatedBy string
}

// EdgeType represents the type of connection
type EdgeType string

const (
	EdgeTypeDirected      EdgeType = "directed"
	EdgeTypeUndirected    EdgeType = "undirected"
	EdgeTypeBidirectional EdgeType = "bidirectional"
	EdgeTypeHierarchical  EdgeType = "hierarchical"
	EdgeTypeAssociation   EdgeType = "association"
	EdgeTypeDependency    EdgeType = "dependency"
)

// EdgeFilter provides filtering options for edge queries
type EdgeFilter struct {
	GraphID       *string
	Type          *EdgeType
	MinWeight     *float64
	MaxWeight     *float64
	CreatedAfter  *string
	CreatedBefore *string
}

// EdgePage represents a paginated result of edges
type EdgePage struct {
	Edges      []*Edge
	Total      int64
	PageSize   int
	PageNumber int
	HasNext    bool
	HasPrev    bool
}
