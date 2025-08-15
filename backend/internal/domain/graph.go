package domain

import "context"

// Graph represents a complete knowledge network containing all memory nodes and relationships
type Graph struct {
	Nodes []*Node `json:"nodes"`
	Edges []*Edge `json:"edges"`
}

// GraphRepository defines the persistence methods for a Graph.
type GraphRepository interface {
	FindByID(ctx context.Context, userID string, id GraphID) (*Graph, error)
	FindAll(ctx context.Context, userID string) ([]*Graph, error)
	Save(ctx context.Context, graph *Graph) error
	Delete(ctx context.Context, userID string, id GraphID) error
}
