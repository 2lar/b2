package queries

import (
	"fmt"
	"time"
)

// GetGraphStatsQuery retrieves cached graph statistics
type GetGraphStatsQuery struct {
	UserID  string `validate:"required"`
	GraphID string `validate:"required"`
}

// Validate validates the query
func (q GetGraphStatsQuery) Validate() error {
	if q.UserID == "" {
		return fmt.Errorf("user ID is required")
	}
	if q.GraphID == "" {
		return fmt.Errorf("graph ID is required")
	}
	return nil
}

// GetGraphStatsResult represents graph statistics
type GetGraphStatsResult struct {
	GraphID            string    `json:"graph_id"`
	NodeCount          int       `json:"node_count"`
	EdgeCount          int       `json:"edge_count"`
	AverageConnections float64   `json:"average_connections"`
	LastUpdated        time.Time `json:"last_updated"`
}