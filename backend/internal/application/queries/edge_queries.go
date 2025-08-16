package queries

import (
	"errors"
	"strings"
	"time"
)

// GetEdgeQuery represents a request to retrieve a specific edge between two nodes.
type GetEdgeQuery struct {
	UserID       string    `json:"user_id" validate:"required"`
	SourceNodeID string    `json:"source_node_id" validate:"required"`
	TargetNodeID string    `json:"target_node_id" validate:"required"`
	RequestedAt  time.Time `json:"requested_at"`

	// Optional flags for controlling what data to include
	IncludeNodes bool `json:"include_nodes"`
}

// NewGetEdgeQuery creates a new GetEdgeQuery with validation.
func NewGetEdgeQuery(userID, sourceNodeID, targetNodeID string) (*GetEdgeQuery, error) {
	query := &GetEdgeQuery{
		UserID:       userID,
		SourceNodeID: sourceNodeID,
		TargetNodeID: targetNodeID,
		RequestedAt:  time.Now(),
		IncludeNodes: false,
	}

	if err := query.Validate(); err != nil {
		return nil, err
	}

	return query, nil
}

// WithNodes includes node data in the query result.
func (q *GetEdgeQuery) WithNodes() *GetEdgeQuery {
	q.IncludeNodes = true
	return q
}

// Validate performs validation on the query parameters.
func (q *GetEdgeQuery) Validate() error {
	if strings.TrimSpace(q.UserID) == "" {
		return errors.New("user_id is required")
	}

	if strings.TrimSpace(q.SourceNodeID) == "" {
		return errors.New("source_node_id is required")
	}

	if strings.TrimSpace(q.TargetNodeID) == "" {
		return errors.New("target_node_id is required")
	}

	return nil
}

// ListEdgesQuery represents a request to retrieve a paginated list of edges.
type ListEdgesQuery struct {
	UserID      string    `json:"user_id" validate:"required"`
	RequestedAt time.Time `json:"requested_at"`

	// Pagination parameters
	Limit     int    `json:"limit" validate:"min=1,max=100"`
	NextToken string `json:"next_token,omitempty"`

	// Filtering options
	SourceNodeID string   `json:"source_node_id,omitempty"`
	TargetNodeID string   `json:"target_node_id,omitempty"`
	NodeIDs      []string `json:"node_ids,omitempty"`
	MinWeight    float64  `json:"min_weight,omitempty"`
	MaxWeight    float64  `json:"max_weight,omitempty"`

	// Sorting options
	SortBy        string `json:"sort_by" validate:"omitempty,oneof=created_at weight"`
	SortDirection string `json:"sort_direction" validate:"omitempty,oneof=asc desc"`
}

// NewListEdgesQuery creates a new ListEdgesQuery with validation and defaults.
func NewListEdgesQuery(userID string) (*ListEdgesQuery, error) {
	query := &ListEdgesQuery{
		UserID:        userID,
		RequestedAt:   time.Now(),
		Limit:         20, // Default limit
		SortBy:        "created_at",
		SortDirection: "desc",
	}

	if err := query.Validate(); err != nil {
		return nil, err
	}

	return query, nil
}

// WithPagination sets pagination parameters.
func (q *ListEdgesQuery) WithPagination(limit int, nextToken string) *ListEdgesQuery {
	q.Limit = limit
	q.NextToken = nextToken
	return q
}

// WithSourceNode filters results by source node.
func (q *ListEdgesQuery) WithSourceNode(sourceNodeID string) *ListEdgesQuery {
	q.SourceNodeID = sourceNodeID
	return q
}

// WithTargetNode filters results by target node.
func (q *ListEdgesQuery) WithTargetNode(targetNodeID string) *ListEdgesQuery {
	q.TargetNodeID = targetNodeID
	return q
}

// WithNodeIDs filters results by specific node IDs (either source or target).
func (q *ListEdgesQuery) WithNodeIDs(nodeIDs []string) *ListEdgesQuery {
	q.NodeIDs = nodeIDs
	return q
}

// WithWeightRange filters results by weight range.
func (q *ListEdgesQuery) WithWeightRange(minWeight, maxWeight float64) *ListEdgesQuery {
	q.MinWeight = minWeight
	q.MaxWeight = maxWeight
	return q
}

// WithSort sets the sorting parameters.
func (q *ListEdgesQuery) WithSort(sortBy, direction string) *ListEdgesQuery {
	q.SortBy = sortBy
	q.SortDirection = direction
	return q
}

// Validate performs validation on the query parameters.
func (q *ListEdgesQuery) Validate() error {
	if strings.TrimSpace(q.UserID) == "" {
		return errors.New("user_id is required")
	}

	if q.Limit < 1 || q.Limit > 100 {
		return errors.New("limit must be between 1 and 100")
	}

	if q.SortBy != "" {
		validSortFields := map[string]bool{
			"created_at": true,
			"weight":     true,
		}
		if !validSortFields[q.SortBy] {
			return errors.New("invalid sort_by field")
		}
	}

	if q.SortDirection != "" && q.SortDirection != "asc" && q.SortDirection != "desc" {
		return errors.New("sort_direction must be 'asc' or 'desc'")
	}

	if q.MinWeight < 0 || q.MaxWeight < 0 {
		return errors.New("weight values must be non-negative")
	}

	if q.MinWeight > 0 && q.MaxWeight > 0 && q.MinWeight > q.MaxWeight {
		return errors.New("min_weight cannot be greater than max_weight")
	}

	return nil
}

// GetConnectionStatisticsQuery represents a request to retrieve connection statistics.
type GetConnectionStatisticsQuery struct {
	UserID      string    `json:"user_id" validate:"required"`
	RequestedAt time.Time `json:"requested_at"`

	// Connection strength thresholds
	StrongConnectionThreshold float64 `json:"strong_connection_threshold" validate:"min=0,max=1"`
	WeakConnectionThreshold   float64 `json:"weak_connection_threshold" validate:"min=0,max=1"`
}

// NewGetConnectionStatisticsQuery creates a new GetConnectionStatisticsQuery with validation and defaults.
func NewGetConnectionStatisticsQuery(userID string) (*GetConnectionStatisticsQuery, error) {
	query := &GetConnectionStatisticsQuery{
		UserID:                    userID,
		RequestedAt:               time.Now(),
		StrongConnectionThreshold: 0.7, // Default: connections with weight >= 0.7 are strong
		WeakConnectionThreshold:   0.3, // Default: connections with weight <= 0.3 are weak
	}

	if err := query.Validate(); err != nil {
		return nil, err
	}

	return query, nil
}

// WithThresholds sets the connection strength thresholds.
func (q *GetConnectionStatisticsQuery) WithThresholds(strongThreshold, weakThreshold float64) *GetConnectionStatisticsQuery {
	q.StrongConnectionThreshold = strongThreshold
	q.WeakConnectionThreshold = weakThreshold
	return q
}

// Validate performs validation on the query parameters.
func (q *GetConnectionStatisticsQuery) Validate() error {
	if strings.TrimSpace(q.UserID) == "" {
		return errors.New("user_id is required")
	}

	if q.StrongConnectionThreshold < 0 || q.StrongConnectionThreshold > 1 {
		return errors.New("strong_connection_threshold must be between 0 and 1")
	}

	if q.WeakConnectionThreshold < 0 || q.WeakConnectionThreshold > 1 {
		return errors.New("weak_connection_threshold must be between 0 and 1")
	}

	if q.WeakConnectionThreshold > q.StrongConnectionThreshold {
		return errors.New("weak_connection_threshold cannot be greater than strong_connection_threshold")
	}

	return nil
}

// GetNodeConnectionsQuery represents a request to retrieve detailed connections for a specific node.
// This extends the basic GetNodeConnectionsQuery from node_queries.go with additional options.
type GetNodeConnectionsEnrichedQuery struct {
	UserID      string    `json:"user_id" validate:"required"`
	NodeID      string    `json:"node_id" validate:"required"`
	RequestedAt time.Time `json:"requested_at"`

	// Connection filtering
	ConnectionType string `json:"connection_type,omitempty" validate:"omitempty,oneof=outgoing incoming bidirectional"`
	Limit          int    `json:"limit" validate:"min=1,max=50"`

	// Additional enrichment options
	IncludeNodeData    bool    `json:"include_node_data"`
	MinWeight          float64 `json:"min_weight,omitempty"`
	MaxWeight          float64 `json:"max_weight,omitempty"`
	SortByWeight       bool    `json:"sort_by_weight"`
	SortDirection      string  `json:"sort_direction" validate:"omitempty,oneof=asc desc"`
}

// NewGetNodeConnectionsEnrichedQuery creates a new GetNodeConnectionsEnrichedQuery with validation.
func NewGetNodeConnectionsEnrichedQuery(userID, nodeID string) (*GetNodeConnectionsEnrichedQuery, error) {
	query := &GetNodeConnectionsEnrichedQuery{
		UserID:          userID,
		NodeID:          nodeID,
		RequestedAt:     time.Now(),
		ConnectionType:  "outgoing", // Default to outgoing connections
		Limit:           20,         // Default limit
		IncludeNodeData: false,
		SortByWeight:    false,
		SortDirection:   "desc",
	}

	if err := query.Validate(); err != nil {
		return nil, err
	}

	return query, nil
}

// WithConnectionType sets the type of connections to retrieve.
func (q *GetNodeConnectionsEnrichedQuery) WithConnectionType(connectionType string) *GetNodeConnectionsEnrichedQuery {
	q.ConnectionType = connectionType
	return q
}

// WithLimit sets the maximum number of connections to retrieve.
func (q *GetNodeConnectionsEnrichedQuery) WithLimit(limit int) *GetNodeConnectionsEnrichedQuery {
	q.Limit = limit
	return q
}

// WithNodeData includes enriched node data in the response.
func (q *GetNodeConnectionsEnrichedQuery) WithNodeData() *GetNodeConnectionsEnrichedQuery {
	q.IncludeNodeData = true
	return q
}

// WithWeightRange filters connections by weight range.
func (q *GetNodeConnectionsEnrichedQuery) WithWeightRange(minWeight, maxWeight float64) *GetNodeConnectionsEnrichedQuery {
	q.MinWeight = minWeight
	q.MaxWeight = maxWeight
	return q
}

// WithWeightSorting enables sorting by connection weight.
func (q *GetNodeConnectionsEnrichedQuery) WithWeightSorting(direction string) *GetNodeConnectionsEnrichedQuery {
	q.SortByWeight = true
	q.SortDirection = direction
	return q
}

// Validate performs validation on the query parameters.
func (q *GetNodeConnectionsEnrichedQuery) Validate() error {
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

	if q.SortDirection != "" && q.SortDirection != "asc" && q.SortDirection != "desc" {
		return errors.New("sort_direction must be 'asc' or 'desc'")
	}

	if q.MinWeight < 0 || q.MaxWeight < 0 {
		return errors.New("weight values must be non-negative")
	}

	if q.MinWeight > 0 && q.MaxWeight > 0 && q.MinWeight > q.MaxWeight {
		return errors.New("min_weight cannot be greater than max_weight")
	}

	return nil
}