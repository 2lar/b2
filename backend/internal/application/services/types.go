// Package services provides application services for the Brain2 backend.
package services

import (
	"regexp"
	"strings"
	
	"brain2-backend/internal/domain/node"
	"brain2-backend/internal/domain/edge"
	"brain2-backend/internal/domain/category"
)

// ============================================================================
// COMMAND TYPES
// ============================================================================

// CreateNodeCommand represents a command to create a new node.
type CreateNodeCommand struct {
	UserID         string                 `json:"user_id" validate:"required,uuid"`
	Content        string                 `json:"content" validate:"required,min=1,max=10000"`
	Tags           []string               `json:"tags" validate:"max=10,dive,min=1,max=50"`
	Metadata       map[string]interface{} `json:"metadata"`
	IdempotencyKey string                 `json:"idempotency_key"`
}

// UpdateNodeCommand represents a command to update an existing node.
type UpdateNodeCommand struct {
	NodeID   string                 `json:"node_id" validate:"required"`
	Content  string                 `json:"content" validate:"min=1,max=10000"`
	Tags     []string               `json:"tags" validate:"max=10,dive,min=1,max=50"`
	Metadata map[string]interface{} `json:"metadata"`
	Version  int                    `json:"version"` // For optimistic locking
}

// CreateConnectionCommand represents a command to create a connection between nodes.
type CreateConnectionCommand struct {
	SourceID string           `json:"source_id" validate:"required"`
	TargetID string           `json:"target_id" validate:"required"`
	EdgeType edge.EdgeType  `json:"edge_type" validate:"required"`
	Strength float64          `json:"strength" validate:"min=0,max=1"`
}

// DeleteConnectionCommand represents a command to delete a connection.
type DeleteConnectionCommand struct {
	EdgeID string `json:"edge_id" validate:"required"`
}

// ============================================================================
// QUERY RESULT TYPES
// ============================================================================

// NodeView represents a read-optimized view of a node.
type NodeView struct {
	ID          string                 `json:"id"`
	UserID      string                 `json:"user_id"`
	Content     string                 `json:"content"`
	Tags        []string               `json:"tags"`
	Metadata    map[string]interface{} `json:"metadata"`
	CreatedAt   string                 `json:"created_at"`
	UpdatedAt   string                 `json:"updated_at"`
	Version     int                    `json:"version"`
	Connections int                    `json:"connections"`
}

// EdgeView represents a read-optimized view of an edge.
type EdgeView struct {
	ID         string  `json:"id"`
	SourceID   string  `json:"source_id"`
	TargetID   string  `json:"target_id"`
	EdgeType   string  `json:"edge_type"`
	Strength   float64 `json:"strength"`
	CreatedAt  string  `json:"created_at"`
	UpdatedAt  string  `json:"updated_at"`
}

// GraphData represents the complete graph structure.
type GraphData struct {
	Nodes []*node.Node `json:"nodes"`
	Edges []*edge.Edge `json:"edges"`
}

// CategoryNodeView represents a node with its category associations.
type CategoryNodeView struct {
	Node       *NodeView      `json:"node"`
	Categories []CategoryView `json:"categories"`
}

// CategoryView represents a read-optimized view of a category.
type CategoryView struct {
	ID          string                 `json:"id"`
	UserID      string                 `json:"user_id"`
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Color       string                 `json:"color"`
	Icon        string                 `json:"icon"`
	Metadata    map[string]interface{} `json:"metadata"`
	NodeCount   int                    `json:"node_count"`
	CreatedAt   string                 `json:"created_at"`
	UpdatedAt   string                 `json:"updated_at"`
}

// ============================================================================
// HELPER FUNCTIONS
// ============================================================================

var (
	// Regular expressions for keyword extraction
	hashtagRegex = regexp.MustCompile(`#\w+`)
	mentionRegex = regexp.MustCompile(`@\w+`)
)

// containsQuery checks if content contains the search query (case-insensitive).
func containsQuery(content, query string) bool {
	content = strings.ToLower(content)
	query = strings.ToLower(query)
	return strings.Contains(content, query)
}

// toNodeView converts a domain node to a view model.
func toNodeView(node *node.Node) *NodeView {
	return &NodeView{
		ID:        node.ID.String(),
		UserID:    node.UserID.String(),
		Content:   node.Content.String(),
		Tags:      node.Tags.ToSlice(),
		Metadata:  node.Metadata,
		CreatedAt: node.CreatedAt.Format("2006-01-02T15:04:05Z"),
		UpdatedAt: node.UpdatedAt.Format("2006-01-02T15:04:05Z"),
		Version:   node.Version,
	}
}

// toEdgeView converts a domain edge to a view model.
func toEdgeView(edge *edge.Edge) *EdgeView {
	return &EdgeView{
		ID:        edge.ID.String(),
		SourceID:  edge.SourceID.String(),
		TargetID:  edge.TargetID.String(),
		EdgeType:  string(edge.EdgeType),
		Strength:  edge.Strength,
		CreatedAt: edge.CreatedAt.Format("2006-01-02T15:04:05Z"),
		UpdatedAt: edge.UpdatedAt.Format("2006-01-02T15:04:05Z"),
	}
}

// toCategoryView converts a domain category to a view model.
func toCategoryView(category *category.Category) *CategoryView {
	color := ""
	if category.Color != nil {
		color = *category.Color
	}
	icon := ""
	if category.Icon != nil {
		icon = *category.Icon
	}
	
	return &CategoryView{
		ID:          string(category.ID),
		UserID:      category.UserID,
		Name:        category.Name,
		Description: category.Description,
		Color:       color,
		Icon:        icon,
		Metadata:    nil, // Category doesn't have Metadata field
		CreatedAt:   category.CreatedAt.Format("2006-01-02T15:04:05Z"),
		UpdatedAt:   category.UpdatedAt.Format("2006-01-02T15:04:05Z"),
	}
}