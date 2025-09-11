package entities

// EdgeType represents the type of relationship between nodes
type EdgeType string

const (
	// EdgeTypeNormal represents a standard connection
	EdgeTypeNormal EdgeType = "normal"
	
	// EdgeTypeStrong represents a strong connection
	EdgeTypeStrong EdgeType = "strong"
	
	// EdgeTypeWeak represents a weak connection
	EdgeTypeWeak EdgeType = "weak"
	
	// EdgeTypeReference represents a reference connection
	EdgeTypeReference EdgeType = "reference"
	
	// EdgeTypeHierarchical represents a parent-child relationship
	EdgeTypeHierarchical EdgeType = "hierarchical"
	
	// EdgeTypeTemporal represents a time-based connection
	EdgeTypeTemporal EdgeType = "temporal"
)

// IsValid checks if the edge type is valid
func (e EdgeType) IsValid() bool {
	switch e {
	case EdgeTypeNormal, EdgeTypeStrong, EdgeTypeWeak, 
		EdgeTypeReference, EdgeTypeHierarchical, EdgeTypeTemporal:
		return true
	default:
		return false
	}
}

// String returns the string representation of the edge type
func (e EdgeType) String() string {
	return string(e)
}