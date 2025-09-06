package queries

import (
	"errors"
)

// GetGraphDataQuery represents a query for full graph visualization data
type GetGraphDataQuery struct {
	UserID  string `json:"user_id"`
	GraphID string `json:"graph_id,omitempty"` // Optional, uses default if not provided
}

// Validate validates the query
func (q GetGraphDataQuery) Validate() error {
	if q.UserID == "" {
		return errors.New("userID is required")
	}
	return nil
}

// GetGraphDataResult represents the complete graph data for visualization
type GetGraphDataResult struct {
	Nodes []GraphNode `json:"nodes"`
	Edges []GraphEdge `json:"edges"`
	Stats GraphStats  `json:"stats"`
}

// GraphStats contains graph statistics
type GraphStats struct {
	NodeCount    int     `json:"node_count"`
	EdgeCount    int     `json:"edge_count"`
	ClusterCount int     `json:"cluster_count"`
	Density      float64 `json:"density"`
}