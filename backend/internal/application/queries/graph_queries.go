// Package queries contains query types for graph operations.
package queries

// GetGraphQuery represents a request to get a complete graph for a user.
type GetGraphQuery struct {
	UserID         string `json:"user_id"`
	Limit          int    `json:"limit,omitempty"`          // Optional limit on nodes/edges returned
	IncludeMetrics bool   `json:"include_metrics,omitempty"` // Whether to include graph metrics
}

// GetNodeNeighborhoodQuery represents a request to get nodes connected to a specific node.
type GetNodeNeighborhoodQuery struct {
	UserID string `json:"user_id"`
	NodeID string `json:"node_id"`
	Depth  int    `json:"depth"` // How many hops to traverse
}

// GetGraphAnalyticsQuery represents a request to get graph analytics and metrics.
type GetGraphAnalyticsQuery struct {
	UserID string `json:"user_id"`
}