// Package queries contains query objects for read operations.
// Queries represent the intent to retrieve data and encapsulate all parameters needed.
//
// Key Concepts Illustrated:
//   - Query Pattern: Encapsulates a read request as an object
//   - CQRS: Separates read models from write models
//   - Input Validation: Queries validate their own parameters
//   - Immutability: Queries should be immutable once created
//   - Performance: Optimized for read scenarios with caching support
package queries

import (
	"errors"
	"strings"
	"time"
)

// GetNodeQuery represents a request to retrieve a single node with its details.
type GetNodeQuery struct {
	UserID      string    `json:"user_id" validate:"required"`
	NodeID      string    `json:"node_id" validate:"required"`
	RequestedAt time.Time `json:"requested_at"`
	
	// Optional flags for controlling what data to include
	IncludeConnections bool `json:"include_connections"`
	IncludeMetadata    bool `json:"include_metadata"`
}

// NewGetNodeQuery creates a new GetNodeQuery with validation.
func NewGetNodeQuery(userID, nodeID string) (*GetNodeQuery, error) {
	query := &GetNodeQuery{
		UserID:             userID,
		NodeID:             nodeID,
		RequestedAt:        time.Now(),
		IncludeConnections: false,
		IncludeMetadata:    false,
	}
	
	if err := query.Validate(); err != nil {
		return nil, err
	}
	
	return query, nil
}

// WithConnections includes connection data in the query result.
func (q *GetNodeQuery) WithConnections() *GetNodeQuery {
	q.IncludeConnections = true
	return q
}

// WithMetadata includes metadata in the query result.
func (q *GetNodeQuery) WithMetadata() *GetNodeQuery {
	q.IncludeMetadata = true
	return q
}

// Validate performs validation on the query parameters.
func (q *GetNodeQuery) Validate() error {
	if strings.TrimSpace(q.UserID) == "" {
		return errors.New("user_id is required")
	}
	
	if strings.TrimSpace(q.NodeID) == "" {
		return errors.New("node_id is required")
	}
	
	return nil
}

// ListNodesQuery represents a request to retrieve a paginated list of nodes.
type ListNodesQuery struct {
	UserID      string    `json:"user_id" validate:"required"`
	RequestedAt time.Time `json:"requested_at"`
	
	// Pagination parameters
	Limit     int    `json:"limit" validate:"min=1,max=100"`
	NextToken string `json:"next_token,omitempty"`
	
	// Filtering options
	TagFilter    []string `json:"tag_filter,omitempty"`
	SearchQuery  string   `json:"search_query,omitempty"`
	
	// Sorting options
	SortBy        string `json:"sort_by" validate:"omitempty,oneof=created_at updated_at content"`
	SortDirection string `json:"sort_direction" validate:"omitempty,oneof=asc desc"`
}

// NewListNodesQuery creates a new ListNodesQuery with validation and defaults.
func NewListNodesQuery(userID string) (*ListNodesQuery, error) {
	query := &ListNodesQuery{
		UserID:        userID,
		RequestedAt:   time.Now(),
		Limit:         20, // Default limit
		SortBy:        "updated_at",
		SortDirection: "desc",
	}
	
	if err := query.Validate(); err != nil {
		return nil, err
	}
	
	return query, nil
}

// WithPagination sets pagination parameters.
func (q *ListNodesQuery) WithPagination(limit int, nextToken string) *ListNodesQuery {
	q.Limit = limit
	q.NextToken = nextToken
	return q
}

// WithTagFilter filters results by tags.
func (q *ListNodesQuery) WithTagFilter(tags []string) *ListNodesQuery {
	q.TagFilter = tags
	return q
}

// WithSearch adds a search query to filter results.
func (q *ListNodesQuery) WithSearch(searchQuery string) *ListNodesQuery {
	q.SearchQuery = searchQuery
	return q
}

// WithSort sets the sorting parameters.
func (q *ListNodesQuery) WithSort(sortBy, direction string) *ListNodesQuery {
	q.SortBy = sortBy
	q.SortDirection = direction
	return q
}

// Validate performs validation on the query parameters.
func (q *ListNodesQuery) Validate() error {
	if strings.TrimSpace(q.UserID) == "" {
		return errors.New("user_id is required")
	}
	
	if q.Limit < 1 || q.Limit > 100 {
		return errors.New("limit must be between 1 and 100")
	}
	
	if q.SortBy != "" {
		validSortFields := map[string]bool{
			"created_at": true,
			"updated_at": true,
			"content":    true,
		}
		if !validSortFields[q.SortBy] {
			return errors.New("invalid sort_by field")
		}
	}
	
	if q.SortDirection != "" && q.SortDirection != "asc" && q.SortDirection != "desc" {
		return errors.New("sort_direction must be 'asc' or 'desc'")
	}
	
	return nil
}

// GetNodeConnectionsQuery represents a request to retrieve connections for a specific node.
type GetNodeConnectionsQuery struct {
	UserID      string    `json:"user_id" validate:"required"`
	NodeID      string    `json:"node_id" validate:"required"`
	RequestedAt time.Time `json:"requested_at"`
	
	// Connection filtering
	ConnectionType string `json:"connection_type,omitempty" validate:"omitempty,oneof=outgoing incoming bidirectional"`
	Limit          int    `json:"limit" validate:"min=1,max=50"`
}

// NewGetNodeConnectionsQuery creates a new GetNodeConnectionsQuery with validation.
func NewGetNodeConnectionsQuery(userID, nodeID string) (*GetNodeConnectionsQuery, error) {
	query := &GetNodeConnectionsQuery{
		UserID:         userID,
		NodeID:         nodeID,
		RequestedAt:    time.Now(),
		ConnectionType: "outgoing", // Default to outgoing connections
		Limit:          20,         // Default limit
	}
	
	if err := query.Validate(); err != nil {
		return nil, err
	}
	
	return query, nil
}

// WithConnectionType sets the type of connections to retrieve.
func (q *GetNodeConnectionsQuery) WithConnectionType(connectionType string) *GetNodeConnectionsQuery {
	q.ConnectionType = connectionType
	return q
}

// WithLimit sets the maximum number of connections to retrieve.
func (q *GetNodeConnectionsQuery) WithLimit(limit int) *GetNodeConnectionsQuery {
	q.Limit = limit
	return q
}

// Validate performs validation on the query parameters.
func (q *GetNodeConnectionsQuery) Validate() error {
	if strings.TrimSpace(q.UserID) == "" {
		return errors.New("user_id is required")
	}
	
	if strings.TrimSpace(q.NodeID) == "" {
		return errors.New("node_id is required")
	}
	
	if q.Limit < 1 || q.Limit > 50 {
		return errors.New("limit must be between 1 and 50")
	}
	
	if q.ConnectionType != "" {
		validTypes := map[string]bool{
			"outgoing":      true,
			"incoming":      true,
			"bidirectional": true,
		}
		if !validTypes[q.ConnectionType] {
			return errors.New("invalid connection_type")
		}
	}
	
	return nil
}

// GetGraphDataQuery represents a request to retrieve the complete graph data for a user.
type GetGraphDataQuery struct {
	UserID      string    `json:"user_id" validate:"required"`
	RequestedAt time.Time `json:"requested_at"`
	
	// Graph filtering options
	IncludeArchived bool     `json:"include_archived"`
	TagFilter       []string `json:"tag_filter,omitempty"`
	MaxNodes        int      `json:"max_nodes" validate:"min=1,max=1000"`
	MaxEdges        int      `json:"max_edges" validate:"min=1,max=5000"`
}

// NewGetGraphDataQuery creates a new GetGraphDataQuery with validation and defaults.
func NewGetGraphDataQuery(userID string) (*GetGraphDataQuery, error) {
	query := &GetGraphDataQuery{
		UserID:          userID,
		RequestedAt:     time.Now(),
		IncludeArchived: false,
		MaxNodes:        500,  // Default limits to prevent performance issues
		MaxEdges:        2500,
	}
	
	if err := query.Validate(); err != nil {
		return nil, err
	}
	
	return query, nil
}

// WithArchived includes archived nodes in the graph.
func (q *GetGraphDataQuery) WithArchived() *GetGraphDataQuery {
	q.IncludeArchived = true
	return q
}

// WithTagFilter filters the graph by specific tags.
func (q *GetGraphDataQuery) WithTagFilter(tags []string) *GetGraphDataQuery {
	q.TagFilter = tags
	return q
}

// WithLimits sets the maximum number of nodes and edges to retrieve.
func (q *GetGraphDataQuery) WithLimits(maxNodes, maxEdges int) *GetGraphDataQuery {
	q.MaxNodes = maxNodes
	q.MaxEdges = maxEdges
	return q
}

// Validate performs validation on the query parameters.
func (q *GetGraphDataQuery) Validate() error {
	if strings.TrimSpace(q.UserID) == "" {
		return errors.New("user_id is required")
	}
	
	if q.MaxNodes < 1 || q.MaxNodes > 1000 {
		return errors.New("max_nodes must be between 1 and 1000")
	}
	
	if q.MaxEdges < 1 || q.MaxEdges > 5000 {
		return errors.New("max_edges must be between 1 and 5000")
	}
	
	return nil
}