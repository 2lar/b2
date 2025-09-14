package docs

// Additional request/response models for API documentation

// CreateGraphRequest represents the request to create a new graph
type CreateGraphRequest struct {
	// Graph name (required)
	// @example "Research Notes"
	Name string `json:"name" binding:"required" example:"Research Notes"`

	// Graph description (optional)
	// @example "Collection of research papers and notes"
	Description string `json:"description,omitempty" example:"Collection of research papers and notes"`

	// Initial metadata
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// UpdateGraphRequest represents the request to update graph properties
type UpdateGraphRequest struct {
	// New name (optional)
	Name *string `json:"name,omitempty"`

	// New description (optional)
	Description *string `json:"description,omitempty"`

	// Updated metadata
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// CreateEdgeRequest represents the request to create an edge
type CreateEdgeRequest struct {
	// Source node ID (required)
	// @example "550e8400-e29b-41d4-a716-446655440000"
	SourceID string `json:"source_id" binding:"required" example:"550e8400-e29b-41d4-a716-446655440000"`

	// Target node ID (required)
	// @example "660e8400-e29b-41d4-a716-446655440001"
	TargetID string `json:"target_id" binding:"required" example:"660e8400-e29b-41d4-a716-446655440001"`

	// Edge type (similarity, reference, hierarchy, temporal, causal)
	// @example "similarity"
	Type string `json:"type,omitempty" example:"similarity" enums:"similarity,reference,hierarchy,temporal,causal"`

	// Edge weight (0.0 to 1.0)
	// @example 0.85
	Weight float64 `json:"weight,omitempty" example:"0.85" minimum:"0" maximum:"1"`

	// Edge metadata
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// UpdateEdgeRequest represents the request to update edge properties
type UpdateEdgeRequest struct {
	// New weight (optional)
	Weight *float64 `json:"weight,omitempty" minimum:"0" maximum:"1"`

	// Updated metadata
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// DiscoverEdgesRequest represents parameters for edge discovery
type DiscoverEdgesRequest struct {
	// Maximum number of edges to discover
	// @example 10
	Limit int `json:"limit,omitempty" example:"10" minimum:"1" maximum:"50"`

	// Minimum similarity threshold (0.0 to 1.0)
	// @example 0.7
	Threshold float64 `json:"threshold,omitempty" example:"0.7" minimum:"0" maximum:"1"`

	// Edge types to consider
	// @example ["similarity", "reference"]
	Types []string `json:"types,omitempty" example:"similarity,reference"`

	// Whether to create edges automatically
	// @example false
	AutoCreate bool `json:"auto_create,omitempty" example:"false"`
}

// EdgeDiscoveryResponse represents discovered edge candidates
type EdgeDiscoveryResponse struct {
	// Discovered edge candidates
	Candidates []EdgeCandidate `json:"candidates"`

	// Number of edges automatically created
	Created int `json:"created,omitempty"`

	// Processing time in milliseconds
	ProcessingTime int64 `json:"processing_time_ms"`
}

// EdgeCandidate represents a potential edge
type EdgeCandidate struct {
	// Target node ID
	TargetID string `json:"target_id"`

	// Target node title
	TargetTitle string `json:"target_title"`

	// Suggested edge type
	Type string `json:"type"`

	// Similarity score
	Score float64 `json:"score"`

	// Reason for suggestion
	Reason string `json:"reason,omitempty"`
}

// SimilarityResponse represents nodes similar to a reference node
type SimilarityResponse struct {
	// Reference node ID
	ReferenceID string `json:"reference_id"`

	// Similar nodes with scores
	Similar []SimilarNode `json:"similar"`

	// Analysis method used
	Method string `json:"method" example:"cosine_similarity"`
}

// SimilarNode represents a node with similarity score
type SimilarNode struct {
	// Node information
	Node NodeResponse `json:"node"`

	// Similarity score (0.0 to 1.0)
	Score float64 `json:"score"`

	// Common tags
	CommonTags []string `json:"common_tags,omitempty"`

	// Common keywords
	CommonKeywords []string `json:"common_keywords,omitempty"`
}

// PatternSearchRequest represents graph pattern search parameters
type PatternSearchRequest struct {
	// Pattern type (path, cluster, hub, chain)
	// @example "cluster"
	Type string `json:"type" binding:"required" example:"cluster" enums:"path,cluster,hub,chain"`

	// Pattern parameters
	Parameters map[string]interface{} `json:"parameters"`

	// Maximum results
	Limit int `json:"limit,omitempty" default:"10"`
}

// PatternSearchResponse represents pattern search results
type PatternSearchResponse struct {
	// Matching patterns found
	Patterns []GraphPattern `json:"patterns"`

	// Total patterns found
	Total int `json:"total"`
}

// GraphPattern represents a discovered graph pattern
type GraphPattern struct {
	// Pattern type
	Type string `json:"type"`

	// Nodes involved in the pattern
	NodeIDs []string `json:"node_ids"`

	// Edges involved in the pattern
	EdgeIDs []string `json:"edge_ids,omitempty"`

	// Pattern score/strength
	Score float64 `json:"score"`

	// Pattern metadata
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// NodeListResponse represents a list of nodes
type NodeListResponse struct {
	// List of nodes
	Nodes []NodeResponse `json:"nodes"`

	// Total count
	Total int `json:"total"`
}

// HealthResponse represents the API health status
type HealthResponse struct {
	// Service status
	// @example "healthy"
	Status string `json:"status" example:"healthy"`

	// Service version
	// @example "1.0.0"
	Version string `json:"version" example:"1.0.0"`

	// Uptime in seconds
	// @example 3600
	Uptime int64 `json:"uptime" example:"3600"`

	// Component health checks
	Components map[string]ComponentHealth `json:"components"`
}

// ComponentHealth represents health status of a component
type ComponentHealth struct {
	// Component status
	Status string `json:"status" example:"healthy"`

	// Last check timestamp
	LastCheck string `json:"last_check" example:"2024-01-13T10:30:00Z"`

	// Additional details
	Details map[string]interface{} `json:"details,omitempty"`
}

// WebSocketMessage represents a real-time update message
type WebSocketMessage struct {
	// Message type (node_created, node_updated, edge_created, etc.)
	Type string `json:"type"`

	// Event data
	Data interface{} `json:"data"`

	// Timestamp
	Timestamp int64 `json:"timestamp"`
}

// BatchRequest represents a batch operation request
type BatchRequest struct {
	// Operations to execute
	Operations []BatchOperation `json:"operations"`

	// Whether to stop on first error
	StopOnError bool `json:"stop_on_error,omitempty"`
}

// BatchOperation represents a single operation in a batch
type BatchOperation struct {
	// Operation ID for reference
	ID string `json:"id"`

	// Operation method (POST, PUT, DELETE)
	Method string `json:"method"`

	// Operation path
	Path string `json:"path"`

	// Request body
	Body interface{} `json:"body,omitempty"`
}

// BatchResponse represents batch operation results
type BatchResponse struct {
	// Results for each operation
	Results []BatchResult `json:"results"`

	// Number of successful operations
	Successful int `json:"successful"`

	// Number of failed operations
	Failed int `json:"failed"`
}

// BatchResult represents the result of a single batch operation
type BatchResult struct {
	// Operation ID
	ID string `json:"id"`

	// HTTP status code
	Status int `json:"status"`

	// Response body
	Body interface{} `json:"body,omitempty"`

	// Error message if failed
	Error string `json:"error,omitempty"`
}