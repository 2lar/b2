package shared

import (
	"context"
)

// GraphRepository defines the persistence methods for a Graph.
type GraphRepository interface {
	// Core operations
	FindByID(ctx context.Context, userID string, id GraphID) (*Graph, error)
	FindAll(ctx context.Context, userID string) ([]*Graph, error)
	Save(ctx context.Context, graph *Graph) error
	Delete(ctx context.Context, userID string, id GraphID) error
	
	// Graph data operations
	GetGraphData(ctx context.Context, query GraphQuery) (*Graph, error)
	GetGraphDataPaginated(ctx context.Context, query GraphQuery, pagination GraphPagination) (*Graph, string, error)
	
	// Advanced Graph Operations
	GetSubgraph(ctx context.Context, nodeIDs []string) (*Graph, error)
	GetConnectedComponents(ctx context.Context, userID string) ([]Graph, error)
}

// Graph represents a complete knowledge network containing nodes and edges
// This is now a concrete type to support direct field access
type Graph struct {
	Nodes []interface{} `json:"nodes"`
	Edges []interface{} `json:"edges"`
}

// GetNodes returns the nodes in the graph (interface compatibility)
func (g *Graph) GetNodes() []interface{} {
	return g.Nodes
}

// GetEdges returns the edges in the graph (interface compatibility)
func (g *Graph) GetEdges() []interface{} {
	return g.Edges
}

// GraphQuery represents query parameters for graph operations
type GraphQuery struct {
	UserID       string
	NodeIDs      []string
	IncludeEdges bool
	Depth        int
	Filters      map[string]interface{}
}

// GraphPagination represents pagination parameters for graph operations
type GraphPagination struct {
	Limit  int
	Offset int
	Cursor string
}
