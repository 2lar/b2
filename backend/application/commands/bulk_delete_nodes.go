package commands

import (
	"errors"
	"fmt"
)

// BulkDeleteNodesCommand represents a command to delete multiple nodes
type BulkDeleteNodesCommand struct {
	OperationID string   `json:"operation_id"` // For async operation tracking
	UserID      string   `json:"user_id"`
	NodeIDs     []string `json:"node_ids"`
}

// Validate validates the bulk delete command
func (c BulkDeleteNodesCommand) Validate() error {
	if c.UserID == "" {
		return errors.New("user ID is required")
	}

	if len(c.NodeIDs) == 0 {
		return errors.New("at least one node ID is required")
	}

	if len(c.NodeIDs) > 100 {
		return errors.New("cannot delete more than 100 nodes at once")
	}

	// Check for duplicate IDs
	seen := make(map[string]bool)
	for _, id := range c.NodeIDs {
		if id == "" {
			return errors.New("node ID cannot be empty")
		}
		if seen[id] {
			return fmt.Errorf("duplicate node ID: %s", id)
		}
		seen[id] = true
	}

	return nil
}

// BulkDeleteNodesResult represents the result of bulk delete operation
type BulkDeleteNodesResult struct {
	DeletedCount int      `json:"deleted_count"`
	FailedIDs    []string `json:"failed_ids,omitempty"`
	Errors       []string `json:"errors,omitempty"`
}
