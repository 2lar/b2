package domain

// Edge represents a directed relationship between two memory nodes
type Edge struct {
	SourceID string `json:"source_id"`
	TargetID string `json:"target_id"`
}
