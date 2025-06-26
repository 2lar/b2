// Package domain contains the core data structures for the application.
// These structures are independent of any database or API layer concerns.
package domain

// Graph represents a collection of nodes and edges that form a user's memory graph.
// This is the top-level structure that contains all of a user's memories and their connections.
type Graph struct {
	Nodes []Node `json:"nodes"` // All nodes in the graph
	Edges []Edge `json:"edges"` // All edges connecting the nodes
}
