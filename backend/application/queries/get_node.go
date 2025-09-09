package queries

import "errors"

// GetNodeQuery represents a query to get a single node
type GetNodeQuery struct {
	UserID string
	NodeID string
}

// Validate validates the GetNodeQuery
func (q GetNodeQuery) Validate() error {
	if q.UserID == "" {
		return errors.New("user ID is required")
	}
	if q.NodeID == "" {
		return errors.New("node ID is required")
	}
	return nil
}

// GetNodeResult represents the result of getting a node
type GetNodeResult struct {
	ID        string            `json:"id"`
	UserID    string            `json:"userId"`
	Title     string            `json:"title"`
	Content   string            `json:"content"`
	Format    string            `json:"format"`
	Position  Position          `json:"position"`
	Tags      []string          `json:"tags"`
	Metadata  map[string]string `json:"metadata"`
	Version   int               `json:"version"`
	CreatedAt string            `json:"createdAt"`
	UpdatedAt string            `json:"updatedAt"`
}

// Position represents spatial coordinates
type Position struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
	Z float64 `json:"z"`
}
