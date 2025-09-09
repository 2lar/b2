package queries

import "errors"

// ListNodesQuery represents a query to list nodes
type ListNodesQuery struct {
	UserID string
	Limit  int
	Offset int
	SortBy string // "created", "updated", "title"
	Order  string // "asc", "desc"
}

// Validate validates the ListNodesQuery
func (q ListNodesQuery) Validate() error {
	if q.UserID == "" {
		return errors.New("user ID is required")
	}
	if q.Limit < 0 {
		return errors.New("limit cannot be negative")
	}
	if q.Offset < 0 {
		return errors.New("offset cannot be negative")
	}
	if q.SortBy != "" && q.SortBy != "created" && q.SortBy != "updated" && q.SortBy != "title" {
		return errors.New("invalid sort field")
	}
	if q.Order != "" && q.Order != "asc" && q.Order != "desc" {
		return errors.New("invalid sort order")
	}
	return nil
}

// ListNodesResult represents the result of listing nodes
type ListNodesResult struct {
	Nodes      []NodeSummary `json:"nodes"`
	TotalCount int           `json:"totalCount"`
	Limit      int           `json:"limit"`
	Offset     int           `json:"offset"`
}

// NodeSummary represents a summary of a node
type NodeSummary struct {
	ID        string   `json:"id"`
	Title     string   `json:"title"`
	Format    string   `json:"format"`
	Tags      []string `json:"tags"`
	CreatedAt string   `json:"createdAt"`
	UpdatedAt string   `json:"updatedAt"`
}
