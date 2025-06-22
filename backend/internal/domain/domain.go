// Package domain contains the core data structures for the application,
// independent of the database or API layers.
package domain

import "time"

// Node represents a single memory, thought, or idea.
type Node struct {
	ID        string
	UserID    string
	Content   string
	Keywords  []string
	CreatedAt time.Time
	Version   int
}

// Edge represents a connection between two nodes.
type Edge struct {
	SourceID string
	TargetID string
}

// Graph represents the entire collection of a user's nodes and the edges connecting them.
type Graph struct {
	Nodes []Node
	Edges []Edge
}
