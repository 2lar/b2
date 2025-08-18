// Package dto provides unified data transfer objects for HTTP API responses.
// This file eliminates NodeResponse duplication found throughout the codebase.
package dto

import (
	"time"
)

// Placeholder view types until proper CQRS query views are implemented
type NodeView struct {
	ID          string
	Content     string
	Tags        []string
	Keywords    []string
	CreatedAt   time.Time
	UpdatedAt   time.Time
	Version     int
}

// NodeResponse represents a node in HTTP API responses.
// This DTO is the single source of truth for node representation,
// eliminating duplication across multiple handler files.
type NodeResponse struct {
	NodeID    string   `json:"nodeId"`
	Content   string   `json:"content"`
	Tags      []string `json:"tags,omitempty"`
	Keywords  []string `json:"keywords,omitempty"`
	Timestamp string   `json:"timestamp"`
	Version   int      `json:"version"`
	CreatedAt string   `json:"createdAt"`
	UpdatedAt string   `json:"updatedAt"`
}

// NodeListResponse represents the response for list nodes endpoints.
type NodeListResponse struct {
	Nodes      []NodeResponse `json:"nodes"`
	TotalCount int            `json:"totalCount,omitempty"`
	PageInfo   *PageInfo      `json:"pageInfo,omitempty"`
}

// NodeDetailResponse represents a detailed node response with relationships.
type NodeDetailResponse struct {
	NodeResponse
	Categories   []CategoryResponse `json:"categories,omitempty"`
	ConnectedTo  []NodeResponse     `json:"connectedTo,omitempty"`
	ConnectedFrom []NodeResponse    `json:"connectedFrom,omitempty"`
}

// NodeConverter handles conversion between domain/query models and Node DTOs.
type NodeConverter struct{}

// NewNodeConverter creates a new node converter.
func NewNodeConverter() *NodeConverter {
	return &NodeConverter{}
}

// FromNodeView converts a NodeView to NodeResponse.
func (c *NodeConverter) FromNodeView(view NodeView) NodeResponse {
	return NodeResponse{
		NodeID:    view.ID,
		Content:   view.Content,
		Tags:      view.Tags,
		Keywords:  view.Keywords,
		Timestamp: view.CreatedAt.Format(time.RFC3339),
		Version:   view.Version,
		CreatedAt: view.CreatedAt.Format(time.RFC3339),
		UpdatedAt: view.UpdatedAt.Format(time.RFC3339),
	}
}

// FromNodeViews converts a slice of NodeView to NodeListResponse.
func (c *NodeConverter) FromNodeViews(views []NodeView) NodeListResponse {
	nodes := make([]NodeResponse, 0, len(views))
	for _, view := range views {
		nodes = append(nodes, c.FromNodeView(view))
	}
	
	return NodeListResponse{
		Nodes:      nodes,
		TotalCount: len(nodes),
	}
}

// FromNodeViewsWithPaging converts NodeViews with pagination info.
func (c *NodeConverter) FromNodeViewsWithPaging(views []NodeView, totalCount int, pageInfo *PageInfo) NodeListResponse {
	nodes := make([]NodeResponse, 0, len(views))
	for _, view := range views {
		nodes = append(nodes, c.FromNodeView(view))
	}
	
	return NodeListResponse{
		Nodes:      nodes,
		TotalCount: totalCount,
		PageInfo:   pageInfo,
	}
}

// ToDetailResponse converts a NodeResponse to NodeDetailResponse with additional data.
func (c *NodeConverter) ToDetailResponse(node NodeResponse, categories []CategoryResponse, connectedTo, connectedFrom []NodeResponse) NodeDetailResponse {
	return NodeDetailResponse{
		NodeResponse:  node,
		Categories:    categories,
		ConnectedTo:   connectedTo,
		ConnectedFrom: connectedFrom,
	}
}