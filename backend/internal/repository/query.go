package repository

// NodeQuery represents query parameters for finding nodes.
type NodeQuery struct {
	UserID   string   // Required: The user ID to query nodes for
	Keywords []string // Optional: Keywords to search for
	NodeIDs  []string // Optional: Specific node IDs to retrieve
	Limit    int      // Optional: Maximum number of results (0 = no limit)
	Offset   int      // Optional: Number of results to skip
}

// Validate checks if the NodeQuery has valid parameters.
func (q NodeQuery) Validate() error {
	if q.UserID == "" {
		return NewInvalidQuery("UserID", "cannot be empty")
	}
	if q.Limit < 0 {
		return NewInvalidQuery("Limit", "cannot be negative")
	}
	if q.Offset < 0 {
		return NewInvalidQuery("Offset", "cannot be negative")
	}
	return nil
}

// HasKeywords returns true if the query includes keyword filtering.
func (q NodeQuery) HasKeywords() bool {
	return len(q.Keywords) > 0
}

// HasNodeIDs returns true if the query includes specific node IDs.
func (q NodeQuery) HasNodeIDs() bool {
	return len(q.NodeIDs) > 0
}

// HasPagination returns true if the query includes pagination parameters.
func (q NodeQuery) HasPagination() bool {
	return q.Limit > 0 || q.Offset > 0
}

// EdgeQuery represents query parameters for finding edges.
type EdgeQuery struct {
	UserID   string   // Required: The user ID to query edges for
	NodeIDs  []string // Optional: Specific node IDs to find edges for
	SourceID string   // Optional: Find edges originating from this node
	TargetID string   // Optional: Find edges pointing to this node
	Limit    int      // Optional: Maximum number of results (0 = no limit)
	Offset   int      // Optional: Number of results to skip
}

// Validate checks if the EdgeQuery has valid parameters.
func (q EdgeQuery) Validate() error {
	if q.UserID == "" {
		return NewInvalidQuery("UserID", "cannot be empty")
	}
	if q.Limit < 0 {
		return NewInvalidQuery("Limit", "cannot be negative")
	}
	if q.Offset < 0 {
		return NewInvalidQuery("Offset", "cannot be negative")
	}
	return nil
}

// HasNodeIDs returns true if the query includes specific node IDs.
func (q EdgeQuery) HasNodeIDs() bool {
	return len(q.NodeIDs) > 0
}

// HasSourceFilter returns true if the query filters by source node.
func (q EdgeQuery) HasSourceFilter() bool {
	return q.SourceID != ""
}

// HasTargetFilter returns true if the query filters by target node.
func (q EdgeQuery) HasTargetFilter() bool {
	return q.TargetID != ""
}

// HasPagination returns true if the query includes pagination parameters.
func (q EdgeQuery) HasPagination() bool {
	return q.Limit > 0 || q.Offset > 0
}

// GraphQuery represents query parameters for retrieving graph data.
type GraphQuery struct {
	UserID      string   // Required: The user ID to query graph data for
	NodeIDs     []string // Optional: Specific node IDs to include in the graph
	MaxDepth    int      // Optional: Maximum depth of connections to include (0 = all)
	IncludeEdges bool     // Whether to include edge information (default: true)
}

// Validate checks if the GraphQuery has valid parameters.
func (q GraphQuery) Validate() error {
	if q.UserID == "" {
		return NewInvalidQuery("UserID", "cannot be empty")
	}
	if q.MaxDepth < 0 {
		return NewInvalidQuery("MaxDepth", "cannot be negative")
	}
	return nil
}

// HasNodeFilter returns true if the query filters by specific node IDs.
func (q GraphQuery) HasNodeFilter() bool {
	return len(q.NodeIDs) > 0
}

// HasDepthLimit returns true if the query limits the depth of connections.
func (q GraphQuery) HasDepthLimit() bool {
	return q.MaxDepth > 0
}