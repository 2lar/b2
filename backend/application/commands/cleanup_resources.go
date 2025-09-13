package commands

import (
	"fmt"
)

// CleanupNodeResourcesCommand represents a command to clean up resources after node deletion
type CleanupNodeResourcesCommand struct {
	NodeID   string
	UserID   string
	Keywords []string
	Tags     []string
}

// Validate validates the command
func (c *CleanupNodeResourcesCommand) Validate() error {
	if c.NodeID == "" {
		return fmt.Errorf("node ID is required")
	}
	if c.UserID == "" {
		return fmt.Errorf("user ID is required")
	}
	return nil
}

// CleanupEdgeResourcesCommand represents a command to clean up resources after edge deletion
type CleanupEdgeResourcesCommand struct {
	EdgeID   string
	SourceID string
	TargetID string
	UserID   string
}

// Validate validates the command
func (c *CleanupEdgeResourcesCommand) Validate() error {
	if c.EdgeID == "" {
		return fmt.Errorf("edge ID is required")
	}
	if c.UserID == "" {
		return fmt.Errorf("user ID is required")
	}
	return nil
}
