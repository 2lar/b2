package api

// CreateNodeRequest represents the request to create a node
type CreateNodeRequest struct {
	Content string   `json:"content"`
	Tags    []string `json:"tags,omitempty"`
}

// UpdateNodeRequest represents the request to update a node
type UpdateNodeRequest struct {
	Content string   `json:"content,omitempty"`
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