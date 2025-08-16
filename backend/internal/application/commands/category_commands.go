// Package commands contains command objects for category write operations.
package commands

import (
	"errors"
	"strings"
	"time"
)

// CreateCategoryCommand represents the intent to create a new category.
type CreateCategoryCommand struct {
	UserID        string    `json:"user_id" validate:"required"`
	Title         string    `json:"title" validate:"required,min=1,max=100"`
	Description   string    `json:"description" validate:"max=500"`
	Color         string    `json:"color,omitempty" validate:"omitempty,hexcolor"`
	IdempotencyKey *string  `json:"idempotency_key,omitempty"`
	RequestedAt   time.Time `json:"requested_at"`
}

// NewCreateCategoryCommand creates a new CreateCategoryCommand with validation.
func NewCreateCategoryCommand(userID, title, description string) (*CreateCategoryCommand, error) {
	cmd := &CreateCategoryCommand{
		UserID:      userID,
		Title:       title,
		Description: description,
		RequestedAt: time.Now(),
	}
	
	if err := cmd.Validate(); err != nil {
		return nil, err
	}
	
	return cmd, nil
}

// WithColor sets the category color.
func (c *CreateCategoryCommand) WithColor(color string) *CreateCategoryCommand {
	c.Color = color
	return c
}

// WithIdempotencyKey adds an idempotency key to the command.
func (c *CreateCategoryCommand) WithIdempotencyKey(key string) *CreateCategoryCommand {
	c.IdempotencyKey = &key
	return c
}

// Validate performs business validation on the command.
func (c *CreateCategoryCommand) Validate() error {
	if strings.TrimSpace(c.UserID) == "" {
		return errors.New("user_id is required")
	}
	
	if strings.TrimSpace(c.Title) == "" {
		return errors.New("title is required")
	}
	
	if len(c.Title) > 100 {
		return errors.New("title exceeds maximum length of 100 characters")
	}
	
	if len(c.Description) > 500 {
		return errors.New("description exceeds maximum length of 500 characters")
	}
	
	return nil
}

// UpdateCategoryCommand represents the intent to update an existing category.
type UpdateCategoryCommand struct {
	UserID        string    `json:"user_id" validate:"required"`
	CategoryID    string    `json:"category_id" validate:"required"`
	Title         *string   `json:"title,omitempty" validate:"omitempty,min=1,max=100"`
	Description   *string   `json:"description,omitempty" validate:"omitempty,max=500"`
	Color         *string   `json:"color,omitempty" validate:"omitempty,hexcolor"`
	IdempotencyKey *string  `json:"idempotency_key,omitempty"`
	RequestedAt   time.Time `json:"requested_at"`
	
	// Flags to indicate what should be updated
	UpdateTitle       bool `json:"-"`
	UpdateDescription bool `json:"-"`
	UpdateColor       bool `json:"-"`
}

// NewUpdateCategoryCommand creates a new UpdateCategoryCommand with validation.
func NewUpdateCategoryCommand(userID, categoryID string) (*UpdateCategoryCommand, error) {
	cmd := &UpdateCategoryCommand{
		UserID:      userID,
		CategoryID:  categoryID,
		RequestedAt: time.Now(),
	}
	
	if err := cmd.Validate(); err != nil {
		return nil, err
	}
	
	return cmd, nil
}

// WithTitle sets the title to be updated.
func (c *UpdateCategoryCommand) WithTitle(title string) *UpdateCategoryCommand {
	c.Title = &title
	c.UpdateTitle = true
	return c
}

// WithDescription sets the description to be updated.
func (c *UpdateCategoryCommand) WithDescription(description string) *UpdateCategoryCommand {
	c.Description = &description
	c.UpdateDescription = true
	return c
}

// WithColor sets the color to be updated.
func (c *UpdateCategoryCommand) WithColor(color string) *UpdateCategoryCommand {
	c.Color = &color
	c.UpdateColor = true
	return c
}

// WithIdempotencyKey adds an idempotency key to the command.
func (c *UpdateCategoryCommand) WithIdempotencyKey(key string) *UpdateCategoryCommand {
	c.IdempotencyKey = &key
	return c
}

// Validate performs business validation on the command.
func (c *UpdateCategoryCommand) Validate() error {
	if strings.TrimSpace(c.UserID) == "" {
		return errors.New("user_id is required")
	}
	
	if strings.TrimSpace(c.CategoryID) == "" {
		return errors.New("category_id is required")
	}
	
	if c.Title != nil {
		if strings.TrimSpace(*c.Title) == "" {
			return errors.New("title cannot be empty when provided")
		}
		if len(*c.Title) > 100 {
			return errors.New("title exceeds maximum length of 100 characters")
		}
	}
	
	if c.Description != nil && len(*c.Description) > 500 {
		return errors.New("description exceeds maximum length of 500 characters")
	}
	
	return nil
}

// HasChanges returns true if the command contains any changes to apply.
func (c *UpdateCategoryCommand) HasChanges() bool {
	return c.UpdateTitle || c.UpdateDescription || c.UpdateColor
}

// DeleteCategoryCommand represents the intent to delete a category.
type DeleteCategoryCommand struct {
	UserID        string    `json:"user_id" validate:"required"`
	CategoryID    string    `json:"category_id" validate:"required"`
	IdempotencyKey *string  `json:"idempotency_key,omitempty"`
	RequestedAt   time.Time `json:"requested_at"`
}

// NewDeleteCategoryCommand creates a new DeleteCategoryCommand with validation.
func NewDeleteCategoryCommand(userID, categoryID string) (*DeleteCategoryCommand, error) {
	cmd := &DeleteCategoryCommand{
		UserID:      userID,
		CategoryID:  categoryID,
		RequestedAt: time.Now(),
	}
	
	if err := cmd.Validate(); err != nil {
		return nil, err
	}
	
	return cmd, nil
}

// WithIdempotencyKey adds an idempotency key to the command.
func (c *DeleteCategoryCommand) WithIdempotencyKey(key string) *DeleteCategoryCommand {
	c.IdempotencyKey = &key
	return c
}

// Validate performs business validation on the command.
func (c *DeleteCategoryCommand) Validate() error {
	if strings.TrimSpace(c.UserID) == "" {
		return errors.New("user_id is required")
	}
	
	if strings.TrimSpace(c.CategoryID) == "" {
		return errors.New("category_id is required")
	}
	
	return nil
}

// AssignNodeToCategoryCommand represents the intent to assign a node to a category.
type AssignNodeToCategoryCommand struct {
	UserID        string    `json:"user_id" validate:"required"`
	CategoryID    string    `json:"category_id" validate:"required"`
	NodeID        string    `json:"node_id" validate:"required"`
	IdempotencyKey *string  `json:"idempotency_key,omitempty"`
	RequestedAt   time.Time `json:"requested_at"`
}

// NewAssignNodeToCategoryCommand creates a new AssignNodeToCategoryCommand with validation.
func NewAssignNodeToCategoryCommand(userID, categoryID, nodeID string) (*AssignNodeToCategoryCommand, error) {
	cmd := &AssignNodeToCategoryCommand{
		UserID:      userID,
		CategoryID:  categoryID,
		NodeID:      nodeID,
		RequestedAt: time.Now(),
	}
	
	if err := cmd.Validate(); err != nil {
		return nil, err
	}
	
	return cmd, nil
}

// WithIdempotencyKey adds an idempotency key to the command.
func (c *AssignNodeToCategoryCommand) WithIdempotencyKey(key string) *AssignNodeToCategoryCommand {
	c.IdempotencyKey = &key
	return c
}

// Validate performs business validation on the command.
func (c *AssignNodeToCategoryCommand) Validate() error {
	if strings.TrimSpace(c.UserID) == "" {
		return errors.New("user_id is required")
	}
	
	if strings.TrimSpace(c.CategoryID) == "" {
		return errors.New("category_id is required")
	}
	
	if strings.TrimSpace(c.NodeID) == "" {
		return errors.New("node_id is required")
	}
	
	return nil
}

// RemoveNodeFromCategoryCommand represents the intent to remove a node from a category.
type RemoveNodeFromCategoryCommand struct {
	UserID        string    `json:"user_id" validate:"required"`
	CategoryID    string    `json:"category_id" validate:"required"`
	NodeID        string    `json:"node_id" validate:"required"`
	IdempotencyKey *string  `json:"idempotency_key,omitempty"`
	RequestedAt   time.Time `json:"requested_at"`
}

// NewRemoveNodeFromCategoryCommand creates a new RemoveNodeFromCategoryCommand with validation.
func NewRemoveNodeFromCategoryCommand(userID, categoryID, nodeID string) (*RemoveNodeFromCategoryCommand, error) {
	cmd := &RemoveNodeFromCategoryCommand{
		UserID:      userID,
		CategoryID:  categoryID,
		NodeID:      nodeID,
		RequestedAt: time.Now(),
	}
	
	if err := cmd.Validate(); err != nil {
		return nil, err
	}
	
	return cmd, nil
}

// WithIdempotencyKey adds an idempotency key to the command.
func (c *RemoveNodeFromCategoryCommand) WithIdempotencyKey(key string) *RemoveNodeFromCategoryCommand {
	c.IdempotencyKey = &key
	return c
}

// Validate performs business validation on the command.
func (c *RemoveNodeFromCategoryCommand) Validate() error {
	if strings.TrimSpace(c.UserID) == "" {
		return errors.New("user_id is required")
	}
	
	if strings.TrimSpace(c.CategoryID) == "" {
		return errors.New("category_id is required")
	}
	
	if strings.TrimSpace(c.NodeID) == "" {
		return errors.New("node_id is required")
	}
	
	return nil
}