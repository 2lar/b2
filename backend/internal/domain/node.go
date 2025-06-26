// Package domain contains the core data structures for the application.
// These structures are independent of any database or API layer concerns.
package domain

import "time"

// Node represents a single memory, thought, or idea in the graph.
// Each node contains content and metadata for organization and retrieval.
type Node struct {
	ID        string    `json:"id"`         // Unique identifier for the node
	UserID    string    `json:"user_id"`    // ID of the user who owns this node
	Content   string    `json:"content"`    // The actual content of the memory/thought
	Keywords  []string  `json:"keywords"`   // Keywords associated with this node for search
	CreatedAt time.Time `json:"created_at"` // When this node was created
	Version   int       `json:"version"`    // Version number for this node
}
