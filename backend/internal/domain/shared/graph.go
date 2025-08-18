package shared

import (
	"context"
)

// GraphRepository defines the persistence methods for a Graph.
type GraphRepository interface {
	FindByID(ctx context.Context, userID string, id GraphID) (*Graph, error)
	FindAll(ctx context.Context, userID string) ([]*Graph, error)
	Save(ctx context.Context, graph *Graph) error
	Delete(ctx context.Context, userID string, id GraphID) error
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
