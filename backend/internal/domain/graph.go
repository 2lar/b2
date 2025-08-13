package domain

// Graph represents a complete knowledge network containing all memory nodes and relationships
type Graph struct {
	Nodes []*Node `json:"nodes"`
	Edges []*Edge `json:"edges"`
}
