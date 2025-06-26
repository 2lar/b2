// Package domain contains the core data structures for the application.
// These structures are independent of any database or API layer concerns.
package domain

// Edge represents a connection between two nodes in the graph.
// This allows for creating relationships between memories/thoughts.
type Edge struct {
	SourceID string `json:"source_id"` // ID of the source node
	TargetID string `json:"target_id"` // ID of the target node
}
