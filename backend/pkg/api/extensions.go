package api

// Extensions to the generated API types for performance optimizations

// PageInfo contains pagination metadata
type PageInfo struct {
	CurrentPage *int `json:"current_page,omitempty"`
	PageSize    *int `json:"page_size,omitempty"`
	TotalPages  *int `json:"total_pages,omitempty"`
	ItemsInPage *int `json:"items_in_page,omitempty"`
}

// NodePageResponse represents a paginated list of nodes
type NodePageResponse struct {
	Items      *[]EnhancedNode `json:"items,omitempty"`
	HasMore    *bool           `json:"has_more,omitempty"`
	NextCursor *string         `json:"next_cursor,omitempty"`
	PageInfo   *PageInfo       `json:"page_info,omitempty"`
}

// NodeNeighborhoodResponse represents a node's neighborhood graph
type NodeNeighborhoodResponse struct {
	Elements   *[]GraphDataResponse_Elements_Item `json:"elements,omitempty"`
	Depth      *int                              `json:"depth,omitempty"`
	CenterNode *Node                             `json:"center_node,omitempty"`
}

// EnhancedNode extends the basic Node with additional fields for performance
type EnhancedNode struct {
	Node
	Keywords  *[]string `json:"keywords,omitempty"`
	CreatedAt *string   `json:"created_at,omitempty"`
}