// Package dto contains Data Transfer Objects for application layer responses.
// These are optimized view models that represent data in a format suitable for external consumption.
//
// Key Concepts Illustrated:
//   - View Models: Optimized for presentation, not domain logic
//   - Data Transfer Objects: Simple structures for moving data across boundaries
//   - Separation of Concerns: Different from domain models
//   - Performance: Flat structures for efficient serialization
//   - API Contracts: Stable interfaces for external consumers
package dto

import (
	"time"
	"brain2-backend/internal/domain"
)

// NodeView represents a node optimized for read operations and API responses.
type NodeView struct {
	ID        string    `json:"id"`
	Content   string    `json:"content"`
	Keywords  []string  `json:"keywords"`
	Tags      []string  `json:"tags"`
	UserID    string    `json:"user_id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Version   int       `json:"version"`
	Archived  bool      `json:"archived"`
}

// ToNodeView converts a domain Node to a NodeView.
func ToNodeView(node *domain.Node) *NodeView {
	if node == nil {
		return nil
	}
	
	return &NodeView{
		ID:        node.ID.String(),
		Content:   node.Content.String(),
		Keywords:  node.Keywords().ToSlice(),
		Tags:      node.Tags.ToSlice(),
		UserID:    node.UserID.String(),
		CreatedAt: node.CreatedAt,
		UpdatedAt: node.UpdatedAt,
		Version:   node.Version,
		Archived:  node.IsArchived(),
	}
}

// ToNodeViews converts a slice of domain Nodes to NodeViews.
func ToNodeViews(nodes []*domain.Node) []*NodeView {
	views := make([]*NodeView, len(nodes))
	for i, node := range nodes {
		views[i] = ToNodeView(node)
	}
	return views
}

// ConnectionView represents an edge/connection optimized for API responses.
type ConnectionView struct {
	ID           string    `json:"id"`
	SourceNodeID string    `json:"source_node_id"`
	TargetNodeID string    `json:"target_node_id"`
	Strength     float64   `json:"strength"`
	CreatedAt    time.Time `json:"created_at"`
}

// ToConnectionView converts a domain Edge to a ConnectionView.
func ToConnectionView(edge *domain.Edge) *ConnectionView {
	if edge == nil {
		return nil
	}
	
	return &ConnectionView{
		ID:           edge.ID.String(),
		SourceNodeID: edge.SourceID.String(),
		TargetNodeID: edge.TargetID.String(),
		Strength:     edge.Weight(),
		CreatedAt:    edge.CreatedAt,
	}
}

// ToConnectionViews converts a slice of domain Edges to ConnectionViews.
func ToConnectionViews(edges []*domain.Edge) []*ConnectionView {
	views := make([]*ConnectionView, len(edges))
	for i, edge := range edges {
		views[i] = ToConnectionView(edge)
	}
	return views
}

// NodeMetadata contains metadata about a node for detailed views.
type NodeMetadata struct {
	WordCount         int       `json:"word_count"`
	KeywordCount      int       `json:"keyword_count"`
	TagCount          int       `json:"tag_count"`
	ConnectionCount   int       `json:"connection_count"`
	LastModified      time.Time `json:"last_modified"`
	Version           int       `json:"version"`
}

// CreateNodeResult represents the result of creating a node.
type CreateNodeResult struct {
	Node        *NodeView        `json:"node"`
	Connections []*ConnectionView `json:"connections"`
	Message     string           `json:"message,omitempty"`
}

// UpdateNodeResult represents the result of updating a node.
type UpdateNodeResult struct {
	Node    *NodeView `json:"node"`
	Message string    `json:"message,omitempty"`
}

// DeleteNodeResult represents the result of deleting a node.
type DeleteNodeResult struct {
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
}

// BulkDeleteResult represents the result of bulk deleting nodes.
type BulkDeleteResult struct {
	DeletedCount int      `json:"deleted_count"`
	FailedIDs    []string `json:"failed_ids,omitempty"`
	Message      string   `json:"message,omitempty"`
}

// BulkCreateResult represents the result of bulk creating nodes.
type BulkCreateResult struct {
	CreatedNodes    []*NodeView       `json:"created_nodes"`
	CreatedCount    int               `json:"created_count"`
	Connections     []*ConnectionView `json:"connections,omitempty"`
	ConnectionCount int               `json:"connection_count"`
	Failed          []BulkCreateError `json:"failed,omitempty"`
	Message         string            `json:"message,omitempty"`
}

// BulkCreateError represents an error that occurred during bulk creation.
type BulkCreateError struct {
	Index   int    `json:"index"`
	Content string `json:"content"`
	Error   string `json:"error"`
}

// CreateConnectionResult represents the result of creating a connection between nodes.
type CreateConnectionResult struct {
	Success    bool              `json:"success"`
	Connection *ConnectionView   `json:"connection,omitempty"`
	Message    string            `json:"message,omitempty"`
}

// GetNodeResult represents the result of retrieving a single node.
type GetNodeResult struct {
	Node        *NodeView         `json:"node"`
	Connections []*ConnectionView `json:"connections,omitempty"`
	Metadata    *NodeMetadata     `json:"metadata,omitempty"`
}

// ListNodesResult represents the result of listing nodes with pagination.
type ListNodesResult struct {
	Nodes     []*NodeView `json:"nodes"`
	NextToken string      `json:"next_token,omitempty"`
	HasMore   bool        `json:"has_more"`
	Total     int         `json:"total"`
	Count     int         `json:"count"`
}

// GetEdgeResult represents the result of retrieving a single edge.
type GetEdgeResult struct {
	Edge       *ConnectionView `json:"edge"`
	SourceNode *NodeView       `json:"source_node,omitempty"`
	TargetNode *NodeView       `json:"target_node,omitempty"`
}

// ListEdgesResult represents the result of listing edges with pagination.
type ListEdgesResult struct {
	Edges     []*ConnectionView `json:"edges"`
	NextToken string            `json:"next_token,omitempty"`
	HasMore   bool              `json:"has_more"`
	Total     int               `json:"total"`
	Count     int               `json:"count"`
}

// ConnectionStatisticsResult represents connection statistics for a user.
type ConnectionStatisticsResult struct {
	TotalEdges           int                  `json:"total_edges"`
	StrongConnections    int                  `json:"strong_connections"`
	WeakConnections      int                  `json:"weak_connections"`
	AverageWeight        float64              `json:"average_weight"`
	AverageConnections   float64              `json:"average_connections"`
	ConnectionDensity    float64              `json:"connection_density"`
	TotalNodes           int                  `json:"total_nodes"`
	MostConnectedNodes   []NodeConnectionInfo `json:"most_connected_nodes"`
	CalculatedAt         time.Time            `json:"calculated_at"`
}

// NodeConnectionInfo represents connection information for a specific node.
type NodeConnectionInfo struct {
	NodeID          string `json:"node_id"`
	ConnectionCount int    `json:"connection_count"`
}

// GetNodeConnectionsResult represents enriched connections for a node.
type GetNodeConnectionsResult struct {
	NodeID        string                     `json:"node_id"`
	Connections   []*ConnectionView          `json:"connections"`
	Count         int                        `json:"count"`
	EnrichedNodes map[string]*NodeView       `json:"enriched_nodes,omitempty"`
}

// GetGraphDataResult represents the result of retrieving graph data.
type GetGraphDataResult struct {
	Graph *GraphView `json:"graph"`
}

// GraphView represents a graph optimized for API responses.
type GraphView struct {
	Nodes       []*NodeView       `json:"nodes"`
	Connections []*ConnectionView `json:"connections"`
	NodeCount   int               `json:"node_count"`
	EdgeCount   int               `json:"edge_count"`
	Density     float64           `json:"density"`
	GeneratedAt time.Time         `json:"generated_at"`
}

// ToGraphView converts a domain Graph to a GraphView.
func ToGraphView(graph *domain.Graph) *GraphView {
	if graph == nil {
		return &GraphView{
			Nodes:       []*NodeView{},
			Connections: []*ConnectionView{},
			NodeCount:   0,
			EdgeCount:   0,
			Density:     0,
			GeneratedAt: time.Now(),
		}
	}

	nodeViews := ToNodeViews(graph.Nodes)
	connectionViews := ToConnectionViews(graph.Edges)

	// Calculate density
	nodeCount := len(graph.Nodes)
	edgeCount := len(graph.Edges)
	density := float64(0)
	if nodeCount > 1 {
		possibleEdges := nodeCount * (nodeCount - 1) / 2
		density = float64(edgeCount) / float64(possibleEdges)
	}

	return &GraphView{
		Nodes:       nodeViews,
		Connections: connectionViews,
		NodeCount:   nodeCount,
		EdgeCount:   edgeCount,
		Density:     density,
		GeneratedAt: time.Now(),
	}
}

// ErrorResponse represents an error response from the application layer.
type ErrorResponse struct {
	Error   string                 `json:"error"`
	Message string                 `json:"message"`
	Details map[string]interface{} `json:"details,omitempty"`
	Code    string                 `json:"code,omitempty"`
}

// ValidationErrorResponse represents validation errors in a structured format.
type ValidationErrorResponse struct {
	Error  string            `json:"error"`
	Fields map[string]string `json:"fields"`
}

// SuccessResponse represents a generic success response.
type SuccessResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

// PaginationInfo contains pagination metadata for list responses.
type PaginationInfo struct {
	NextToken string `json:"next_token,omitempty"`
	HasMore   bool   `json:"has_more"`
	Total     int    `json:"total"`
	Count     int    `json:"count"`
	Limit     int    `json:"limit"`
}

// CategoryView represents a category optimized for read operations and API responses.
type CategoryView struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Color       string    `json:"color,omitempty"`
	UserID      string    `json:"user_id"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	NodeCount   int       `json:"node_count,omitempty"`
}

// ToCategoryView converts a domain Category to a CategoryView.
func ToCategoryView(category *domain.Category) *CategoryView {
	if category == nil {
		return nil
	}
	
	view := &CategoryView{
		ID:          string(category.ID),
		Title:       category.Title,
		Description: category.Description,
		UserID:      category.UserID,
		CreatedAt:   category.CreatedAt,
		UpdatedAt:   category.UpdatedAt,
	}
	
	// Handle optional color field
	if category.Color != nil {
		view.Color = *category.Color
	}
	
	return view
}

// ToCategoryViews converts a slice of domain Categories to CategoryViews.
func ToCategoryViews(categories []*domain.Category) []*CategoryView {
	views := make([]*CategoryView, len(categories))
	for i, category := range categories {
		views[i] = ToCategoryView(category)
	}
	return views
}

// CategorySuggestionView represents an AI-powered category suggestion.
type CategorySuggestionView struct {
	Title       string  `json:"title"`
	Description string  `json:"description"`
	Confidence  float64 `json:"confidence"`
	Reason      string  `json:"reason"`
	IsExisting  bool    `json:"is_existing"`
	CategoryID  string  `json:"category_id,omitempty"`
}

// ToCategorySuggestionView converts a domain CategorySuggestion to a CategorySuggestionView.
func ToCategorySuggestionView(suggestion *domain.CategorySuggestion) *CategorySuggestionView {
	if suggestion == nil {
		return nil
	}
	
	view := &CategorySuggestionView{
		Title:       suggestion.Name,
		Description: suggestion.Reason, // Use reason as description
		Confidence:  suggestion.Confidence,
		Reason:      suggestion.Reason,
		IsExisting:  false,
	}
	
	return view
}

// ToCategorySuggestionViews converts a slice of domain CategorySuggestions to CategorySuggestionViews.
func ToCategorySuggestionViews(suggestions []domain.CategorySuggestion) []*CategorySuggestionView {
	views := make([]*CategorySuggestionView, len(suggestions))
	for i, suggestion := range suggestions {
		views[i] = ToCategorySuggestionView(&suggestion)
	}
	return views
}

// CategoryStats contains statistics about a category.
type CategoryStats struct {
	NodeCount       int       `json:"node_count"`
	LastNodeAdded   time.Time `json:"last_node_added,omitempty"`
	AvgWordsPerNode float64   `json:"avg_words_per_node"`
	TopKeywords     []string  `json:"top_keywords,omitempty"`
}

// CreateCategoryResult represents the result of creating a category.
type CreateCategoryResult struct {
	Category *CategoryView `json:"category"`
	Message  string        `json:"message,omitempty"`
}

// UpdateCategoryResult represents the result of updating a category.
type UpdateCategoryResult struct {
	Category *CategoryView `json:"category"`
	Message  string        `json:"message,omitempty"`
}

// DeleteCategoryResult represents the result of deleting a category.
type DeleteCategoryResult struct {
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
}

// GetCategoryResult represents the result of retrieving a single category.
type GetCategoryResult struct {
	Category *CategoryView  `json:"category"`
	Nodes    []*NodeView    `json:"nodes,omitempty"`
	Stats    *CategoryStats `json:"stats,omitempty"`
}

// ListCategoriesResult represents the result of listing categories with pagination.
type ListCategoriesResult struct {
	Categories []*CategoryView `json:"categories"`
	NextToken  string          `json:"next_token,omitempty"`
	HasMore    bool            `json:"has_more"`
	Total      int             `json:"total"`
	Count      int             `json:"count"`
}

// GetNodesInCategoryResult represents the result of retrieving nodes in a category.
type GetNodesInCategoryResult struct {
	CategoryID string      `json:"category_id"`
	Nodes      []*NodeView `json:"nodes"`
	NextToken  string      `json:"next_token,omitempty"`
	HasMore    bool        `json:"has_more"`
	Total      int         `json:"total"`
	Count      int         `json:"count"`
}

// GetCategoriesForNodeResult represents the result of retrieving categories for a node.
type GetCategoriesForNodeResult struct {
	NodeID     string          `json:"node_id"`
	Categories []*CategoryView `json:"categories"`
	Count      int             `json:"count"`
}

// SuggestCategoriesResult represents the result of AI-powered category suggestions.
type SuggestCategoriesResult struct {
	Suggestions []*CategorySuggestionView `json:"suggestions"`
	Count       int                       `json:"count"`
	Message     string                    `json:"message,omitempty"`
	Source      string                    `json:"source"` // "ai" or "fallback"
}

// AssignNodeToCategoryResult represents the result of assigning a node to a category.
type AssignNodeToCategoryResult struct {
	Success    bool   `json:"success"`
	CategoryID string `json:"category_id"`
	NodeID     string `json:"node_id"`
	Message    string `json:"message,omitempty"`
}

// RemoveNodeFromCategoryResult represents the result of removing a node from a category.
type RemoveNodeFromCategoryResult struct {
	Success    bool   `json:"success"`
	CategoryID string `json:"category_id"`
	NodeID     string `json:"node_id"`
	Message    string `json:"message,omitempty"`
}