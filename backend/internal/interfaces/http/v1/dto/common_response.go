// Package dto provides common data transfer objects used across the API.
package dto

import "time"

// ============================================================================
// COMMON RESPONSE STRUCTURES
// ============================================================================

// PageInfo provides pagination information for list responses.
type PageInfo struct {
	HasNextPage     bool   `json:"hasNextPage"`
	HasPreviousPage bool   `json:"hasPreviousPage"`
	StartCursor     string `json:"startCursor,omitempty"`
	EndCursor       string `json:"endCursor,omitempty"`
	TotalPages      int    `json:"totalPages,omitempty"`
	CurrentPage     int    `json:"currentPage,omitempty"`
	PageSize        int    `json:"pageSize,omitempty"`
}

// ErrorResponse represents an error response structure.
type ErrorResponse struct {
	Error     string            `json:"error"`
	Code      string            `json:"code,omitempty"`
	Details   string            `json:"details,omitempty"`
	RequestID string            `json:"requestId,omitempty"`
	Fields    map[string]string `json:"fields,omitempty"`
}

// SuccessResponse represents a generic success response.
type SuccessResponse struct {
	Success bool        `json:"success"`
	Message string      `json:"message,omitempty"`
	Data    interface{} `json:"data,omitempty"`
}

// BulkOperationResponse represents responses for bulk operations.
type BulkOperationResponse struct {
	TotalRequested int      `json:"totalRequested"`
	Successful     int      `json:"successful"`
	Failed         int      `json:"failed"`
	SuccessfulIDs  []string `json:"successfulIds,omitempty"`
	FailedIDs      []string `json:"failedIds,omitempty"`
	Errors         []string `json:"errors,omitempty"`
}

// HealthResponse represents health check response.
type HealthResponse struct {
	Status    string            `json:"status"`
	Timestamp time.Time         `json:"timestamp"`
	Services  map[string]string `json:"services,omitempty"`
	Version   string            `json:"version,omitempty"`
	Uptime    string            `json:"uptime,omitempty"`
}

// ============================================================================
// GRAPH AND RELATIONSHIP RESPONSES
// ============================================================================

// GraphResponse represents graph data for visualization.
type GraphResponse struct {
	Nodes []GraphNode `json:"nodes"`
	Edges []GraphEdge `json:"edges"`
	Stats GraphStats  `json:"stats"`
}

// GraphNode represents a node in graph visualization.
type GraphNode struct {
	ID         string                 `json:"id"`
	Label      string                 `json:"label"`
	Type       string                 `json:"type"`
	Properties map[string]interface{} `json:"properties,omitempty"`
	Position   *Position              `json:"position,omitempty"`
	Style      *NodeStyle             `json:"style,omitempty"`
}

// GraphEdge represents an edge in graph visualization.
type GraphEdge struct {
	ID         string                 `json:"id"`
	Source     string                 `json:"source"`
	Target     string                 `json:"target"`
	Type       string                 `json:"type"`
	Label      string                 `json:"label,omitempty"`
	Properties map[string]interface{} `json:"properties,omitempty"`
	Style      *EdgeStyle             `json:"style,omitempty"`
}

// Position represents coordinates for graph layout.
type Position struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

// NodeStyle represents visual styling for graph nodes.
type NodeStyle struct {
	Color       string  `json:"color,omitempty"`
	Size        float64 `json:"size,omitempty"`
	Shape       string  `json:"shape,omitempty"`
	BorderColor string  `json:"borderColor,omitempty"`
	BorderWidth float64 `json:"borderWidth,omitempty"`
}

// EdgeStyle represents visual styling for graph edges.
type EdgeStyle struct {
	Color     string  `json:"color,omitempty"`
	Width     float64 `json:"width,omitempty"`
	Style     string  `json:"style,omitempty"` // solid, dashed, dotted
	Animated  bool    `json:"animated,omitempty"`
}

// GraphStats provides statistics about the graph.
type GraphStats struct {
	NodeCount int `json:"nodeCount"`
	EdgeCount int `json:"edgeCount"`
	Depth     int `json:"depth,omitempty"`
}

// ============================================================================
// SEARCH AND FILTER RESPONSES
// ============================================================================

// SearchResponse represents search results.
type SearchResponse struct {
	Query      string            `json:"query"`
	Results    []SearchResult    `json:"results"`
	TotalCount int               `json:"totalCount"`
	PageInfo   *PageInfo         `json:"pageInfo,omitempty"`
	Facets     map[string][]Facet `json:"facets,omitempty"`
	Took       int               `json:"took"` // milliseconds
}

// SearchResult represents a single search result.
type SearchResult struct {
	ID          string                 `json:"id"`
	Type        string                 `json:"type"`
	Title       string                 `json:"title"`
	Content     string                 `json:"content"`
	Excerpt     string                 `json:"excerpt,omitempty"`
	Score       float64                `json:"score"`
	Highlights  []string               `json:"highlights,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
	URL         string                 `json:"url,omitempty"`
}

// Facet represents a search facet for filtering.
type Facet struct {
	Value string `json:"value"`
	Count int    `json:"count"`
}

// ============================================================================
// OPERATION STATUS RESPONSES
// ============================================================================

// OperationResponse represents the status of an operation.
type OperationResponse struct {
	ID        string    `json:"id"`
	Status    string    `json:"status"` // pending, running, completed, failed
	Message   string    `json:"message,omitempty"`
	Progress  *Progress `json:"progress,omitempty"`
	Result    interface{} `json:"result,omitempty"`
	Error     string    `json:"error,omitempty"`
	StartedAt time.Time `json:"startedAt"`
	UpdatedAt time.Time `json:"updatedAt"`
	Duration  int       `json:"duration,omitempty"` // milliseconds
}

// Progress represents operation progress information.
type Progress struct {
	Current int    `json:"current"`
	Total   int    `json:"total"`
	Percent int    `json:"percent"`
	Stage   string `json:"stage,omitempty"`
}

// ============================================================================
// METADATA AND ANALYTICS RESPONSES
// ============================================================================

// MetricsResponse represents system metrics.
type MetricsResponse struct {
	Timestamp time.Time              `json:"timestamp"`
	Metrics   map[string]interface{} `json:"metrics"`
	Period    string                 `json:"period,omitempty"`
}

// AnalyticsResponse represents analytics data.
type AnalyticsResponse struct {
	Period     string                 `json:"period"`
	StartDate  time.Time              `json:"startDate"`
	EndDate    time.Time              `json:"endDate"`
	DataPoints []AnalyticsDataPoint   `json:"dataPoints"`
	Summary    map[string]interface{} `json:"summary"`
}

// AnalyticsDataPoint represents a single analytics data point.
type AnalyticsDataPoint struct {
	Timestamp time.Time              `json:"timestamp"`
	Values    map[string]interface{} `json:"values"`
}

// ============================================================================
// VERSION AND INFO RESPONSES
// ============================================================================

// VersionResponse represents API version information.
type VersionResponse struct {
	Version     string    `json:"version"`
	BuildHash   string    `json:"buildHash,omitempty"`
	BuildDate   time.Time `json:"buildDate,omitempty"`
	APIVersion  string    `json:"apiVersion"`
	Environment string    `json:"environment,omitempty"`
}

// InfoResponse represents general API information.
type InfoResponse struct {
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Version     VersionResponse   `json:"version"`
	Endpoints   []EndpointInfo    `json:"endpoints,omitempty"`
	Limits      map[string]int    `json:"limits,omitempty"`
	Features    []string          `json:"features,omitempty"`
}

// EndpointInfo represents information about an API endpoint.
type EndpointInfo struct {
	Path        string   `json:"path"`
	Methods     []string `json:"methods"`
	Description string   `json:"description,omitempty"`
	Deprecated  bool     `json:"deprecated,omitempty"`
}

// ============================================================================
// RESPONSE BUILDER UTILITIES
// ============================================================================

// ResponseBuilder provides a fluent interface for building responses.
type ResponseBuilder struct{}

// NewResponseBuilder creates a new response builder.
func NewResponseBuilder() *ResponseBuilder {
	return &ResponseBuilder{}
}

// Success creates a success response.
func (b *ResponseBuilder) Success(message string, data interface{}) SuccessResponse {
	return SuccessResponse{
		Success: true,
		Message: message,
		Data:    data,
	}
}

// Error creates an error response.
func (b *ResponseBuilder) Error(code, message, details string) ErrorResponse {
	return ErrorResponse{
		Error:   message,
		Code:    code,
		Details: details,
	}
}

// BulkOperation creates a bulk operation response.
func (b *ResponseBuilder) BulkOperation(total, successful, failed int, successfulIDs, failedIDs, errors []string) BulkOperationResponse {
	return BulkOperationResponse{
		TotalRequested: total,
		Successful:     successful,
		Failed:         failed,
		SuccessfulIDs:  successfulIDs,
		FailedIDs:      failedIDs,
		Errors:         errors,
	}
}

// PageInfo creates pagination information.
func (b *ResponseBuilder) PageInfo(hasNext, hasPrev bool, startCursor, endCursor string, totalPages, currentPage, pageSize int) PageInfo {
	return PageInfo{
		HasNextPage:     hasNext,
		HasPreviousPage: hasPrev,
		StartCursor:     startCursor,
		EndCursor:       endCursor,
		TotalPages:      totalPages,
		CurrentPage:     currentPage,
		PageSize:        pageSize,
	}
}