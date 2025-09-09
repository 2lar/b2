package commands

import (
	"errors"
	"strings"
)

// UpdateNodeCommand represents a command to update a node
type UpdateNodeCommand struct {
	UserID  string
	NodeID  string
	Title   *string // Pointer allows partial updates
	Content *string
	Format  *string
	X       *float64
	Y       *float64
	Z       *float64
	Tags    *[]string
}

// Validate validates the UpdateNodeCommand
func (c UpdateNodeCommand) Validate() error {
	if c.UserID == "" {
		return errors.New("user ID is required")
	}
	if c.NodeID == "" {
		return errors.New("node ID is required")
	}

	// Check if at least one field is being updated
	if c.Title == nil && c.Content == nil && c.Format == nil &&
		c.X == nil && c.Y == nil && c.Z == nil && c.Tags == nil {
		return errors.New("no fields to update")
	}

	// Validate title if provided
	if c.Title != nil && strings.TrimSpace(*c.Title) == "" {
		return errors.New("title cannot be empty")
	}

	// Validate format if provided
	if c.Format != nil {
		validFormats := map[string]bool{
			"text":     true,
			"markdown": true,
			"html":     true,
			"code":     true,
		}
		if !validFormats[*c.Format] {
			return errors.New("invalid format")
		}
	}

	return nil
}
