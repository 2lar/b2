package api

// ErrorResponse represents a standardized error response
type ErrorResponse struct {
	Error string `json:"error" example:"Invalid request body"`
}

// HealthResponse represents a health check response
type HealthResponse struct {
	Status string `json:"status" example:"ok"`
}

// CreateNodeRequest represents the request to create a node
type CreateNodeRequest struct {
	Content string   `json:"content"`
	Title   string   `json:"title,omitempty"`
	Tags    []string `json:"tags,omitempty"`
}

// CreateCategoryRequest represents the request to create a category
type CreateCategoryRequest struct {
	Title       string  `json:"title" validate:"required" example:"Machine Learning"`
	Description string  `json:"description,omitempty" example:"AI and ML related content"`
	ParentID    *string `json:"parentId,omitempty" example:"parent-category-id"`
	Color       *string `json:"color,omitempty" example:"#FF5722"`
}

// UpdateCategoryRequest represents the request to update a category
type UpdateCategoryRequest struct {
	Title       string  `json:"title,omitempty" example:"Deep Learning"`
	Description string  `json:"description,omitempty" example:"Neural networks and deep learning"`
	Color       *string `json:"color,omitempty" example:"#2196F3"`
}

// CategoryResponse represents a category in API responses
type CategoryResponse struct {
	ID          string  `json:"id" example:"cat-123"`
	Title       string  `json:"title" example:"Machine Learning"`
	Description string  `json:"description" example:"AI and ML related content"`
	Level       int     `json:"level" example:"1"`
	ParentID    *string `json:"parentId,omitempty" example:"parent-category-id"`
	Color       *string `json:"color,omitempty" example:"#FF5722"`
	NodeCount   int     `json:"nodeCount" example:"15"`
	CreatedAt   string  `json:"createdAt" example:"2024-01-15T10:30:00Z"`
	UpdatedAt   string  `json:"updatedAt" example:"2024-01-16T10:30:00Z"`
}

// UpdateNodeRequest represents the request to update a node
type UpdateNodeRequest struct {
	Content string   `json:"content,omitempty"`
	Title   string   `json:"title,omitempty"`
	Tags    []string `json:"tags,omitempty"`
}

// BulkDeleteRequest represents the request to delete multiple nodes
type BulkDeleteRequest struct {
	NodeIDs []string `json:"nodeIds"`
}

// BulkDeleteResponse represents the response for bulk delete
type BulkDeleteResponse struct {
	DeletedCount int      `json:"deletedCount"`
	FailedIDs    []string `json:"failedIds,omitempty"`
}

// GraphDataResponse represents graph data
type GraphDataResponse struct {
	Elements *[]GraphDataResponse_Elements_Item `json:"elements,omitempty"`
}

// GraphNode represents a graph node
type GraphNode struct {
	Data *NodeData `json:"data,omitempty"`
}

// NodeData represents node data
type NodeData struct {
	Id    string `json:"id"`
	Label string `json:"label"`
}

// GraphEdge represents a graph edge
type GraphEdge struct {
	Data *EdgeData `json:"data,omitempty"`
}

// EdgeData represents edge data
type EdgeData struct {
	Id     string `json:"id"`
	Source string `json:"source"`
	Target string `json:"target"`
}

// Node represents a node in the API
type Node struct {
	NodeID    string                 `json:"nodeId"`
	UserID    string                 `json:"user_id"`
	Content   string                 `json:"content"`
	Title     string                 `json:"title,omitempty"`
	Tags      []string               `json:"tags,omitempty"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
	Timestamp string                 `json:"timestamp"`
	CreatedAt string                 `json:"createdAt,omitempty"`
	UpdatedAt string                 `json:"updatedAt,omitempty"`
}

// GraphDataResponse_Elements_Item represents an element in the graph response
type GraphDataResponse_Elements_Item struct {
	Data  interface{} `json:"data,omitempty"`
	Group string      `json:"group,omitempty"`
}

// FromGraphNode converts a GraphNode to a GraphDataResponse_Elements_Item
func (e *GraphDataResponse_Elements_Item) FromGraphNode(node GraphNode) error {
	e.Data = node.Data
	e.Group = "nodes"
	return nil
}

// FromGraphEdge converts a GraphEdge to a GraphDataResponse_Elements_Item
func (e *GraphDataResponse_Elements_Item) FromGraphEdge(edge GraphEdge) error {
	e.Data = edge.Data
	e.Group = "edges"
	return nil
}