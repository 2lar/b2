package services

import (
	"brain2-backend/internal/domain/node"
	"brain2-backend/internal/domain/edge"
)

// Graph represents a complete knowledge network containing all memory nodes and relationships
type Graph struct {
	Nodes []*node.Node `json:"nodes"`
	Edges []*edge.Edge `json:"edges"`
}

// GetNodes implements the shared.Graph interface
func (g *Graph) GetNodes() []interface{} {
	result := make([]interface{}, len(g.Nodes))
	for i, n := range g.Nodes {
		result[i] = n
	}
	return result
}

// GetEdges implements the shared.Graph interface
func (g *Graph) GetEdges() []interface{} {
	result := make([]interface{}, len(g.Edges))
	for i, e := range g.Edges {
		result[i] = e
	}
	return result
}