package queries

import "errors"

// ListGraphsQuery represents a query to list graphs
type ListGraphsQuery struct {
	UserID string
	Limit  int
	Offset int
	SortBy string // "created", "updated", "name"
	Order  string // "asc", "desc"
}

// Validate validates the query
func (q ListGraphsQuery) Validate() error {
	if q.UserID == "" {
		return errors.New("user ID is required")
	}
	if q.Limit < 0 {
		return errors.New("limit cannot be negative")
	}
	if q.Offset < 0 {
		return errors.New("offset cannot be negative")
	}
	if q.SortBy != "" && q.SortBy != "created" && q.SortBy != "updated" && q.SortBy != "name" {
		return errors.New("invalid sort field")
	}
	if q.Order != "" && q.Order != "asc" && q.Order != "desc" {
		return errors.New("invalid sort order")
	}
	return nil
}

// ListGraphsResult represents the result of listing graphs
type ListGraphsResult struct {
	Graphs     []GraphSummary `json:"graphs"`
	TotalCount int            `json:"totalCount"`
	Limit      int            `json:"limit"`
	Offset     int            `json:"offset"`
}

// GraphSummary represents a summary of a graph
type GraphSummary struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	NodeCount   int    `json:"nodeCount"`
	EdgeCount   int    `json:"edgeCount"`
	IsDefault   bool   `json:"isDefault"`
	CreatedAt   string `json:"createdAt"`
	UpdatedAt   string `json:"updatedAt"`
}