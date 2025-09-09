package commands

import "errors"

// DeleteNodeCommand represents a command to delete a node
type DeleteNodeCommand struct {
	UserID string
	NodeID string
}

// Validate validates the DeleteNodeCommand
func (c DeleteNodeCommand) Validate() error {
	if c.UserID == "" {
		return errors.New("user ID is required")
	}
	if c.NodeID == "" {
		return errors.New("node ID is required")
	}
	return nil
}
