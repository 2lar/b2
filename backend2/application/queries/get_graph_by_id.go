package queries

import "errors"

// GetGraphByIDQuery represents a query to get a graph by ID
type GetGraphByIDQuery struct {
	UserID  string
	GraphID string
}

// Validate validates the query
func (q GetGraphByIDQuery) Validate() error {
	if q.UserID == "" {
		return errors.New("user ID is required")
	}
	if q.GraphID == "" {
		return errors.New("graph ID is required")
	}
	return nil
}

// GetGraphByIDResult represents the result of getting a graph
type GetGraphByIDResult struct {
	ID          string                 `json:"id"`
	UserID      string                 `json:"userId"`
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	NodeCount   int                    `json:"nodeCount"`
	EdgeCount   int                    `json:"edgeCount"`
	Nodes       []GraphNode            `json:"nodes"`
	Edges       []GraphEdge            `json:"edges"`
	Metadata    map[string]interface{} `json:"metadata"`
	CreatedAt   string                 `json:"createdAt"`
	UpdatedAt   string                 `json:"updatedAt"`
}

// GraphNode represents a node in the graph result
type GraphNode struct {
	ID       string            `json:"id"`
	Title    string            `json:"title"`
	Content  string            `json:"content"`
	Position Position          `json:"position"`
	Tags     []string          `json:"tags"`
	Metadata map[string]string `json:"metadata"`
}

// GraphEdge represents an edge in the graph result
type GraphEdge struct {
	ID       string                 `json:"id"`
	SourceID string                 `json:"source"`
	TargetID string                 `json:"target"`
	Type     string                 `json:"type"`
	Weight   float64                `json:"weight"`
	Metadata map[string]interface{} `json:"metadata"`
}