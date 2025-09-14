package docs

import "time"

// CreateNodeRequest represents the request payload for creating a new node
// @Description Request body for creating a new knowledge node in the graph
type CreateNodeRequest struct {
	// Title of the node (required, max 255 characters)
	// @example "Understanding CQRS Pattern"
	Title string `json:"title" binding:"required" example:"Understanding CQRS Pattern"`

	// Content body of the node (optional, max 50000 characters)
	// @example "CQRS stands for Command Query Responsibility Segregation..."
	Content string `json:"content,omitempty" example:"CQRS stands for Command Query Responsibility Segregation..."`

	// Content format (text, markdown, html, json)
	// @example "markdown"
	Format string `json:"format,omitempty" example:"markdown" enums:"text,markdown,html,json"`

	// Tags for categorization (optional)
	// @example ["architecture", "patterns", "cqrs"]
	Tags []string `json:"tags,omitempty" example:"architecture,patterns,cqrs"`

	// 3D position coordinates
	Position Position3D `json:"position,omitempty"`

	// Additional metadata
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// Position3D represents 3D coordinates for node positioning
type Position3D struct {
	// X coordinate (-10000 to 10000)
	// @example 100.5
	X float64 `json:"x" example:"100.5"`

	// Y coordinate (-10000 to 10000)
	// @example 200.5
	Y float64 `json:"y" example:"200.5"`

	// Z coordinate (-10000 to 10000)
	// @example 50.0
	Z float64 `json:"z" example:"50.0"`
}

// CreateNodeResponse represents the response after creating a node
// @Description Response containing the created node details and operation status
type CreateNodeResponse struct {
	// Unique identifier of the created node
	// @example "550e8400-e29b-41d4-a716-446655440000"
	NodeID string `json:"node_id" example:"550e8400-e29b-41d4-a716-446655440000"`

	// Graph ID where the node was created
	// @example "GRAPH#user123#default"
	GraphID string `json:"graph_id" example:"GRAPH#user123#default"`

	// Number of edges automatically created
	// @example 5
	EdgesCreated int `json:"edges_created" example:"5"`

	// Operation ID for tracking async operations
	// @example "op_123456789"
	OperationID string `json:"operation_id,omitempty" example:"op_123456789"`

	// Creation timestamp
	// @example "2024-01-13T10:30:00Z"
	CreatedAt time.Time `json:"created_at" example:"2024-01-13T10:30:00Z"`
}

// UpdateNodeRequest represents the request payload for updating a node
type UpdateNodeRequest struct {
	// New title (optional, max 255 characters)
	Title *string `json:"title,omitempty"`

	// New content (optional, max 50000 characters)
	Content *string `json:"content,omitempty"`

	// New tags (replaces existing tags)
	Tags []string `json:"tags,omitempty"`

	// New position
	Position *Position3D `json:"position,omitempty"`

	// Updated metadata (merged with existing)
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// NodeResponse represents a complete node object
// @Description Complete node information including content, position, and relationships
type NodeResponse struct {
	// Unique node identifier
	ID string `json:"id" example:"550e8400-e29b-41d4-a716-446655440000"`

	// Node title
	Title string `json:"title" example:"Understanding CQRS Pattern"`

	// Node content
	Content string `json:"content" example:"CQRS stands for..."`

	// Content format
	Format string `json:"format" example:"markdown"`

	// Associated tags
	Tags []string `json:"tags" example:"architecture,patterns"`

	// 3D position
	Position Position3D `json:"position"`

	// Graph ID
	GraphID string `json:"graph_id" example:"GRAPH#user123#default"`

	// Owner user ID
	UserID string `json:"user_id" example:"user123"`

	// Number of incoming edges
	IncomingEdges int `json:"incoming_edges" example:"3"`

	// Number of outgoing edges
	OutgoingEdges int `json:"outgoing_edges" example:"7"`

	// Metadata
	Metadata map[string]interface{} `json:"metadata,omitempty"`

	// Timestamps
	CreatedAt time.Time `json:"created_at" example:"2024-01-13T10:30:00Z"`
	UpdatedAt time.Time `json:"updated_at" example:"2024-01-13T11:45:00Z"`
}

// GraphResponse represents a graph with its nodes and edges
type GraphResponse struct {
	// Graph identifier
	ID string `json:"id" example:"GRAPH#user123#default"`

	// Graph name
	Name string `json:"name" example:"Default Graph"`

	// Owner user ID
	UserID string `json:"user_id" example:"user123"`

	// Total number of nodes
	NodeCount int `json:"node_count" example:"150"`

	// Total number of edges
	EdgeCount int `json:"edge_count" example:"450"`

	// List of nodes (paginated)
	Nodes []NodeResponse `json:"nodes,omitempty"`

	// List of edges (paginated)
	Edges []EdgeResponse `json:"edges,omitempty"`

	// Graph metadata
	Metadata GraphMetadata `json:"metadata"`

	// Timestamps
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// EdgeResponse represents an edge between two nodes
type EdgeResponse struct {
	// Edge identifier
	ID string `json:"id" example:"EDGE#node1#node2"`

	// Source node ID
	SourceID string `json:"source_id" example:"550e8400-e29b-41d4-a716-446655440000"`

	// Target node ID
	TargetID string `json:"target_id" example:"660e8400-e29b-41d4-a716-446655440001"`

	// Edge type (similarity, reference, hierarchy, etc.)
	Type string `json:"type" example:"similarity"`

	// Edge weight/strength (0.0 to 1.0)
	Weight float64 `json:"weight" example:"0.85"`

	// Edge metadata
	Metadata map[string]interface{} `json:"metadata,omitempty"`

	// Creation timestamp
	CreatedAt time.Time `json:"created_at"`
}

// GraphMetadata contains graph statistics and configuration
type GraphMetadata struct {
	// Average node degree
	AvgDegree float64 `json:"avg_degree" example:"6.5"`

	// Graph density (0.0 to 1.0)
	Density float64 `json:"density" example:"0.045"`

	// Number of connected components
	Components int `json:"components" example:"1"`

	// Maximum node degree
	MaxDegree int `json:"max_degree" example:"25"`

	// Last analysis timestamp
	LastAnalyzed time.Time `json:"last_analyzed"`
}

// BulkDeleteRequest represents a request to delete multiple nodes
type BulkDeleteRequest struct {
	// List of node IDs to delete
	NodeIDs []string `json:"node_ids" binding:"required" example:"[\"node1\", \"node2\", \"node3\"]"`

	// Whether to delete orphaned edges
	DeleteOrphanedEdges bool `json:"delete_orphaned_edges" example:"true"`
}

// BulkDeleteResponse represents the result of bulk deletion
type BulkDeleteResponse struct {
	// Number of nodes successfully deleted
	NodesDeleted int `json:"nodes_deleted" example:"3"`

	// Number of edges deleted
	EdgesDeleted int `json:"edges_deleted" example:"7"`

	// List of node IDs that failed to delete
	Failed []string `json:"failed,omitempty"`

	// Error messages for failed deletions
	Errors map[string]string `json:"errors,omitempty"`
}

// SearchRequest represents search parameters
type SearchRequest struct {
	// Search query string
	Query string `json:"query" binding:"required" example:"CQRS architecture"`

	// Search in title, content, or both
	SearchIn []string `json:"search_in,omitempty" example:"[\"title\", \"content\"]"`

	// Filter by tags
	Tags []string `json:"tags,omitempty"`

	// Filter by graph ID
	GraphID string `json:"graph_id,omitempty"`

	// Maximum number of results
	Limit int `json:"limit,omitempty" example:"20"`

	// Offset for pagination
	Offset int `json:"offset,omitempty" example:"0"`
}

// SearchResponse represents search results
type SearchResponse struct {
	// Total number of matching results
	Total int `json:"total" example:"42"`

	// Current page results
	Results []SearchResult `json:"results"`

	// Search execution time in milliseconds
	ExecutionTime int64 `json:"execution_time_ms" example:"15"`
}

// SearchResult represents a single search result
type SearchResult struct {
	// Node information
	Node NodeResponse `json:"node"`

	// Relevance score (0.0 to 1.0)
	Score float64 `json:"score" example:"0.95"`

	// Highlighted snippets
	Highlights map[string][]string `json:"highlights,omitempty"`
}

// OperationStatus represents the status of an async operation
type OperationStatus struct {
	// Operation identifier
	ID string `json:"id" example:"op_123456789"`

	// Operation type
	Type string `json:"type" example:"CREATE_NODE"`

	// Current status (pending, running, completed, failed)
	Status string `json:"status" example:"completed"`

	// Progress percentage (0-100)
	Progress int `json:"progress" example:"100"`

	// Operation result (when completed)
	Result interface{} `json:"result,omitempty"`

	// Error message (when failed)
	Error string `json:"error,omitempty"`

	// Start timestamp
	StartedAt time.Time `json:"started_at"`

	// Completion timestamp
	CompletedAt *time.Time `json:"completed_at,omitempty"`
}

// ErrorResponse represents an API error response
// @Description Standard error response format
type ErrorResponse struct {
	// Error code for programmatic handling
	// @example "VALIDATION_ERROR"
	Code string `json:"code" example:"VALIDATION_ERROR"`

	// Human-readable error message
	// @example "Title cannot be empty"
	Message string `json:"message" example:"Title cannot be empty"`

	// Detailed error information
	Details map[string]interface{} `json:"details,omitempty"`

	// Request ID for tracking
	// @example "req_abc123"
	RequestID string `json:"request_id,omitempty" example:"req_abc123"`

	// Timestamp of the error
	// @example "2024-01-13T10:30:00Z"
	Timestamp time.Time `json:"timestamp" example:"2024-01-13T10:30:00Z"`
}

// PaginationParams represents common pagination parameters
type PaginationParams struct {
	// Page number (1-based)
	Page int `json:"page" query:"page" example:"1"`

	// Items per page
	PerPage int `json:"per_page" query:"per_page" example:"20"`

	// Sort field
	SortBy string `json:"sort_by" query:"sort_by" example:"created_at"`

	// Sort direction (asc or desc)
	SortOrder string `json:"sort_order" query:"sort_order" example:"desc"`
}

// PaginatedResponse wraps paginated results
type PaginatedResponse struct {
	// Current page data
	Data interface{} `json:"data"`

	// Pagination metadata
	Pagination PaginationMeta `json:"pagination"`
}

// PaginationMeta contains pagination metadata
type PaginationMeta struct {
	// Current page number
	Page int `json:"page" example:"1"`

	// Items per page
	PerPage int `json:"per_page" example:"20"`

	// Total number of items
	Total int `json:"total" example:"150"`

	// Total number of pages
	TotalPages int `json:"total_pages" example:"8"`

	// Has next page
	HasNext bool `json:"has_next" example:"true"`

	// Has previous page
	HasPrev bool `json:"has_prev" example:"false"`
}