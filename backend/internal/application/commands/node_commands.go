// Package commands contains command objects for write operations.
// Commands represent the intent to change state and encapsulate all data needed for the operation.
//
// Key Concepts Illustrated:
//   - Command Pattern: Encapsulates a request as an object
//   - Input Validation: Commands validate their own data
//   - Immutability: Commands should be immutable once created
//   - Clear Intent: Each command represents a specific business operation
package commands

import (
	"errors"
	"strings"
	"time"
)

// CreateNodeCommand represents the intent to create a new node.
// This command encapsulates all the data and validation needed for node creation.
type CreateNodeCommand struct {
	UserID        string    `json:"user_id" validate:"required"`
	Content       string    `json:"content" validate:"required,min=1,max=10000"`
	Tags          []string  `json:"tags" validate:"max=10,dive,min=1,max=50"`
	IdempotencyKey *string  `json:"idempotency_key,omitempty"`
	RequestedAt   time.Time `json:"requested_at"`
}

// NewCreateNodeCommand creates a new CreateNodeCommand with validation.
func NewCreateNodeCommand(userID, content string, tags []string) (*CreateNodeCommand, error) {
	cmd := &CreateNodeCommand{
		UserID:      userID,
		Content:     content,
		Tags:        tags,
		RequestedAt: time.Now(),
	}
	
	if err := cmd.Validate(); err != nil {
		return nil, err
	}
	
	return cmd, nil
}

// Validate performs business validation on the command.
func (c *CreateNodeCommand) Validate() error {
	if strings.TrimSpace(c.UserID) == "" {
		return errors.New("user_id is required")
	}
	
	if strings.TrimSpace(c.Content) == "" {
		return errors.New("content is required")
	}
	
	if len(c.Content) > 10000 {
		return errors.New("content exceeds maximum length of 10,000 characters")
	}
	
	if len(c.Tags) > 10 {
		return errors.New("maximum of 10 tags allowed")
	}
	
	for _, tag := range c.Tags {
		if strings.TrimSpace(tag) == "" {
			return errors.New("empty tags are not allowed")
		}
		if len(tag) > 50 {
			return errors.New("tag exceeds maximum length of 50 characters")
		}
	}
	
	return nil
}

// WithIdempotencyKey adds an idempotency key to the command.
func (c *CreateNodeCommand) WithIdempotencyKey(key string) *CreateNodeCommand {
	c.IdempotencyKey = &key
	return c
}

// UpdateNodeCommand represents the intent to update an existing node.
type UpdateNodeCommand struct {
	UserID        string    `json:"user_id" validate:"required"`
	NodeID        string    `json:"node_id" validate:"required"`
	Content       *string   `json:"content,omitempty" validate:"omitempty,min=1,max=10000"`
	Tags          []string  `json:"tags,omitempty" validate:"omitempty,max=10,dive,min=1,max=50"`
	Version       int       `json:"version,omitempty" validate:"omitempty,min=0"` // For optimistic locking
	IdempotencyKey *string  `json:"idempotency_key,omitempty"`
	RequestedAt   time.Time `json:"requested_at"`
	
	// Flags to indicate what should be updated
	UpdateContent bool `json:"-"`
	UpdateTags    bool `json:"-"`
	CheckVersion  bool `json:"-"` // Whether to enforce version check
}

// NewUpdateNodeCommand creates a new UpdateNodeCommand with validation.
func NewUpdateNodeCommand(userID, nodeID string) (*UpdateNodeCommand, error) {
	cmd := &UpdateNodeCommand{
		UserID:      userID,
		NodeID:      nodeID,
		RequestedAt: time.Now(),
	}
	
	if err := cmd.Validate(); err != nil {
		return nil, err
	}
	
	return cmd, nil
}

// WithContent sets the content to be updated.
func (c *UpdateNodeCommand) WithContent(content string) *UpdateNodeCommand {
	c.Content = &content
	c.UpdateContent = true
	return c
}

// WithTags sets the tags to be updated.
func (c *UpdateNodeCommand) WithTags(tags []string) *UpdateNodeCommand {
	c.Tags = tags
	c.UpdateTags = true
	return c
}

// WithIdempotencyKey adds an idempotency key to the command.
func (c *UpdateNodeCommand) WithIdempotencyKey(key string) *UpdateNodeCommand {
	c.IdempotencyKey = &key
	return c
}

// Validate performs business validation on the command.
func (c *UpdateNodeCommand) Validate() error {
	if strings.TrimSpace(c.UserID) == "" {
		return errors.New("user_id is required")
	}
	
	if strings.TrimSpace(c.NodeID) == "" {
		return errors.New("node_id is required")
	}
	
	if c.Content != nil {
		if strings.TrimSpace(*c.Content) == "" {
			return errors.New("content cannot be empty when provided")
		}
		if len(*c.Content) > 10000 {
			return errors.New("content exceeds maximum length of 10,000 characters")
		}
	}
	
	if len(c.Tags) > 10 {
		return errors.New("maximum of 10 tags allowed")
	}
	
	for _, tag := range c.Tags {
		if strings.TrimSpace(tag) == "" {
			return errors.New("empty tags are not allowed")
		}
		if len(tag) > 50 {
			return errors.New("tag exceeds maximum length of 50 characters")
		}
	}
	
	return nil
}

// HasChanges returns true if the command contains any changes to apply.
func (c *UpdateNodeCommand) HasChanges() bool {
	return c.UpdateContent || c.UpdateTags
}

// DeleteNodeCommand represents the intent to delete a node.
type DeleteNodeCommand struct {
	UserID        string    `json:"user_id" validate:"required"`
	NodeID        string    `json:"node_id" validate:"required"`
	IdempotencyKey *string  `json:"idempotency_key,omitempty"`
	RequestedAt   time.Time `json:"requested_at"`
}

// NewDeleteNodeCommand creates a new DeleteNodeCommand with validation.
func NewDeleteNodeCommand(userID, nodeID string) (*DeleteNodeCommand, error) {
	cmd := &DeleteNodeCommand{
		UserID:      userID,
		NodeID:      nodeID,
		RequestedAt: time.Now(),
	}
	
	if err := cmd.Validate(); err != nil {
		return nil, err
	}
	
	return cmd, nil
}

// WithIdempotencyKey adds an idempotency key to the command.
func (c *DeleteNodeCommand) WithIdempotencyKey(key string) *DeleteNodeCommand {
	c.IdempotencyKey = &key
	return c
}

// Validate performs business validation on the command.
func (c *DeleteNodeCommand) Validate() error {
	if strings.TrimSpace(c.UserID) == "" {
		return errors.New("user_id is required")
	}
	
	if strings.TrimSpace(c.NodeID) == "" {
		return errors.New("node_id is required")
	}
	
	return nil
}

// ConnectNodesCommand represents the intent to create a connection between two nodes.
type ConnectNodesCommand struct {
	UserID       string    `json:"user_id" validate:"required"`
	SourceNodeID string    `json:"source_node_id" validate:"required"`
	TargetNodeID string    `json:"target_node_id" validate:"required"`
	Weight       float64   `json:"weight" validate:"min=0,max=1"`
	RequestedAt  time.Time `json:"requested_at"`
}

// NewConnectNodesCommand creates a new ConnectNodesCommand with validation.
func NewConnectNodesCommand(userID, sourceNodeID, targetNodeID string, weight float64) (*ConnectNodesCommand, error) {
	cmd := &ConnectNodesCommand{
		UserID:       userID,
		SourceNodeID: sourceNodeID,
		TargetNodeID: targetNodeID,
		Weight:       weight,
		RequestedAt:  time.Now(),
	}
	
	if err := cmd.Validate(); err != nil {
		return nil, err
	}
	
	return cmd, nil
}

// Validate performs business validation on the command.
func (c *ConnectNodesCommand) Validate() error {
	if strings.TrimSpace(c.UserID) == "" {
		return errors.New("user_id is required")
	}
	
	if strings.TrimSpace(c.SourceNodeID) == "" {
		return errors.New("source_node_id is required")
	}
	
	if strings.TrimSpace(c.TargetNodeID) == "" {
		return errors.New("target_node_id is required")
	}
	
	if c.SourceNodeID == c.TargetNodeID {
		return errors.New("cannot connect a node to itself")
	}
	
	if c.Weight < 0 || c.Weight > 1 {
		return errors.New("weight must be between 0 and 1")
	}
	
	return nil
}

// BulkDeleteNodesCommand represents the intent to delete multiple nodes.
type BulkDeleteNodesCommand struct {
	UserID        string    `json:"user_id" validate:"required"`
	NodeIDs       []string  `json:"node_ids" validate:"required,min=1,max=100,dive,required"`
	IdempotencyKey *string  `json:"idempotency_key,omitempty"`
	RequestedAt   time.Time `json:"requested_at"`
}

// NewBulkDeleteNodesCommand creates a new BulkDeleteNodesCommand with validation.
func NewBulkDeleteNodesCommand(userID string, nodeIDs []string) (*BulkDeleteNodesCommand, error) {
	cmd := &BulkDeleteNodesCommand{
		UserID:      userID,
		NodeIDs:     nodeIDs,
		RequestedAt: time.Now(),
	}
	
	if err := cmd.Validate(); err != nil {
		return nil, err
	}
	
	return cmd, nil
}

// WithIdempotencyKey adds an idempotency key to the command.
func (c *BulkDeleteNodesCommand) WithIdempotencyKey(key string) *BulkDeleteNodesCommand {
	c.IdempotencyKey = &key
	return c
}

// Validate performs business validation on the command.
func (c *BulkDeleteNodesCommand) Validate() error {
	if strings.TrimSpace(c.UserID) == "" {
		return errors.New("user_id is required")
	}
	
	if len(c.NodeIDs) == 0 {
		return errors.New("at least one node_id is required")
	}
	
	if len(c.NodeIDs) > 100 {
		return errors.New("maximum of 100 nodes can be deleted at once")
	}
	
	for i, nodeID := range c.NodeIDs {
		if strings.TrimSpace(nodeID) == "" {
			return errors.New("node_id at index " + string(rune(i)) + " is empty")
		}
	}
	
	return nil
}

// BulkCreateNodesCommand represents the intent to create multiple nodes in a single operation.
type BulkCreateNodesCommand struct {
	UserID            string                  `json:"user_id" validate:"required"`
	Nodes             []BulkCreateNodeRequest `json:"nodes" validate:"required,min=1,max=50,dive"`
	CreateConnections bool                    `json:"create_connections"`
	IdempotencyKey    *string                 `json:"idempotency_key,omitempty"`
	RequestedAt       time.Time               `json:"requested_at"`
}

// BulkCreateNodeRequest represents a single node creation request within a bulk operation.
type BulkCreateNodeRequest struct {
	Content string   `json:"content" validate:"required,min=1,max=10000"`
	Tags    []string `json:"tags" validate:"max=10,dive,min=1,max=50"`
}

// NewBulkCreateNodesCommand creates a new BulkCreateNodesCommand with validation.
func NewBulkCreateNodesCommand(userID string, nodes []BulkCreateNodeRequest, createConnections bool) (*BulkCreateNodesCommand, error) {
	cmd := &BulkCreateNodesCommand{
		UserID:            userID,
		Nodes:             nodes,
		CreateConnections: createConnections,
		RequestedAt:       time.Now(),
	}
	
	if err := cmd.Validate(); err != nil {
		return nil, err
	}
	
	return cmd, nil
}

// Validate performs business validation on the command.
func (c *BulkCreateNodesCommand) Validate() error {
	if strings.TrimSpace(c.UserID) == "" {
		return errors.New("user_id is required")
	}
	
	if len(c.Nodes) == 0 {
		return errors.New("at least one node is required")
	}
	
	if len(c.Nodes) > 50 {
		return errors.New("maximum of 50 nodes can be created at once")
	}
	
	for i, nodeReq := range c.Nodes {
		if strings.TrimSpace(nodeReq.Content) == "" {
			return errors.New("content is required for node at index " + string(rune(i)))
		}
		
		if len(nodeReq.Content) > 10000 {
			return errors.New("content too long for node at index " + string(rune(i)))
		}
		
		if len(nodeReq.Tags) > 10 {
			return errors.New("too many tags for node at index " + string(rune(i)))
		}
		
		for j, tag := range nodeReq.Tags {
			if strings.TrimSpace(tag) == "" {
				return errors.New("empty tag at index " + string(rune(j)) + " for node at index " + string(rune(i)))
			}
			if len(tag) > 50 {
				return errors.New("tag too long at index " + string(rune(j)) + " for node at index " + string(rune(i)))
			}
		}
	}
	
	return nil
}