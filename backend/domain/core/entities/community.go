package entities

import "time"

// Community represents a detected group of densely-connected nodes
// discovered by the Leiden algorithm.
type Community struct {
	ID             string
	GraphID        string
	Name           string
	Keywords       []string
	CohesionScore  float64
	MemberCount    int
	CentralNodeID  string // Most connected node within the community
	CreatedAt      time.Time
	UpdatedAt      time.Time
}
