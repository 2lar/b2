package commands

import (
	"errors"
)

// CreateNodeCommand represents the command to create a new node
type CreateNodeCommand struct {
	NodeID  string   `json:"node_id" validate:"required"`
	UserID  string   `json:"user_id" validate:"required"`
	Title   string   `json:"title" validate:"required,min=1,max=200"`
	Content string   `json:"content" validate:"max=50000"`
	Format  string   `json:"format" validate:"oneof=text markdown html json"`
	X       float64  `json:"x" validate:"required"`
	Y       float64  `json:"y" validate:"required"`
	Z       float64  `json:"z"`
	Tags    []string `json:"tags" validate:"max=20,dive,min=1,max=30"`
}

// Validate validates the command
func (cmd CreateNodeCommand) Validate() error {
	if cmd.UserID == "" {
		return errors.New("user ID is required")
	}
	if cmd.Title == "" {
		return errors.New("title is required")
	}
	if len(cmd.Title) > MaxTitleLength {
		return errors.New("title exceeds maximum length")
	}
	if len(cmd.Content) > MaxContentLength {
		return errors.New("content exceeds maximum length")
	}
	return nil
}

const (
	MaxTitleLength   = 200
	MaxContentLength = 50000
)
