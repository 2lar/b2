// Package domain contains domain models and business logic.
// This file defines cluster and community types for graph analysis.
package domain

// Cluster represents a group of related nodes.
type Cluster struct {
	ID       ClusterID `json:"id"`
	NodeIDs  []NodeID  `json:"node_ids"`
	Centroid NodeID    `json:"centroid"`
	Density  float64   `json:"density"`
	Size     int       `json:"size"`
}

// ClusterID is a unique identifier for a cluster.
type ClusterID int

// Community represents a community of nodes in a graph.
type Community struct {
	ID      CommunityID `json:"id"`
	NodeIDs []NodeID    `json:"node_ids"`
	Size    int         `json:"size"`
}

// CommunityID is a unique identifier for a community.
type CommunityID int