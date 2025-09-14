package commands

import (
	"fmt"
)

// CreateEdgeCommand represents a command to create an edge between two nodes
type CreateEdgeCommand struct {
	EdgeID   string                 `json:"edge_id"`
	UserID   string                 `json:"user_id"`
	GraphID  string                 `json:"graph_id"`
	SourceID string                 `json:"source_id"`
	TargetID string                 `json:"target_id"`
	Type     string                 `json:"type"`
	Weight   float64                `json:"weight"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// Validate validates the command
func (c CreateEdgeCommand) Validate() error {
	if c.EdgeID == "" {
		return fmt.Errorf("edge ID is required")
	}
	if c.SourceID == "" {
		return fmt.Errorf("source node ID is required")
	}
	if c.TargetID == "" {
		return fmt.Errorf("target node ID is required")
	}
	if c.UserID == "" {
		return fmt.Errorf("user ID is required")
	}
	if c.Type == "" {
		return fmt.Errorf("edge type is required")
	}
	if c.Weight < 0 || c.Weight > 1 {
		return fmt.Errorf("weight must be between 0 and 1")
	}
	return nil
}

// DeleteEdgeCommand represents a command to delete an edge between two nodes
type DeleteEdgeCommand struct {
	UserID  string `json:"user_id"`
	GraphID string `json:"graph_id"`
	EdgeID  string `json:"edge_id"`
}

// Validate validates the command
func (c DeleteEdgeCommand) Validate() error {
	if c.UserID == "" {
		return fmt.Errorf("user ID is required")
	}
	if c.GraphID == "" {
		return fmt.Errorf("graph ID is required")
	}
	if c.EdgeID == "" {
		return fmt.Errorf("edge ID is required")
	}
	return nil
}
