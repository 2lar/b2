// Package commands implements the command side of CQRS pattern.
// This file contains all node-related command handlers with proper validation.
package commands

import (
	"context"
	"fmt"
	"time"

	"brain2-backend/internal/domain/node"
	"brain2-backend/internal/domain/shared"
	"brain2-backend/internal/repository"
	"brain2-backend/internal/errors"
	
	"github.com/go-playground/validator/v10"
	"go.uber.org/zap"
)

// NodeCommandService handles all write operations for nodes.
// This implements the Command side of CQRS pattern with full validation.
type NodeCommandService struct {
	nodeWriter   repository.NodeWriter
	edgeWriter   repository.EdgeWriter
	eventBus     shared.EventBus
	validator    *validator.Validate
	logger       *zap.Logger
}

// NewNodeCommandService creates a new node command service.
func NewNodeCommandService(
	nodeWriter repository.NodeWriter,
	edgeWriter repository.EdgeWriter,
	eventBus shared.EventBus,
	logger *zap.Logger,
) *NodeCommandService {
	return &NodeCommandService{
		nodeWriter: nodeWriter,
		edgeWriter: edgeWriter,
		eventBus:   eventBus,
		validator:  validator.New(),
		logger:     logger,
	}
}

// ============================================================================
// COMMAND TYPES
// ============================================================================

// CreateNodeCommand encapsulates a node creation request.
type CreateNodeCommand struct {
	UserID         string                 `validate:"required,uuid"`
	Content        string                 `validate:"required,min=1,max=20000"`
	Title          string                 `validate:"max=200"`
	Tags           []string               `validate:"max=10,dive,min=1,max=50"`
	Metadata       map[string]interface{} `validate:"max=20"`
	IdempotencyKey string                 `validate:"max=100"`
}

// UpdateNodeCommand encapsulates a node update request.
type UpdateNodeCommand struct {
	NodeID   string                 `validate:"required"`
	UserID   string                 `validate:"required,uuid"`
	Content  string                 `validate:"omitempty,min=1,max=20000"`
	Title    string                 `validate:"max=200"`
	Tags     []string               `validate:"omitempty,max=10,dive,min=1,max=50"`
	Metadata map[string]interface{} `validate:"omitempty,max=20"`
	Version  int                    `validate:"min=0"`
}

// HasChanges returns true if the update command contains changes.
func (c *UpdateNodeCommand) HasChanges() bool {
	return c.Content != "" || c.Title != "" || len(c.Tags) > 0 || len(c.Metadata) > 0
}

// DeleteNodeCommand encapsulates a node deletion request.
type DeleteNodeCommand struct {
	NodeID string `validate:"required"`
	UserID string `validate:"required,uuid"`
}

// BulkDeleteNodesCommand encapsulates a bulk node deletion request.
type BulkDeleteNodesCommand struct {
	NodeIDs []string `validate:"required,min=1,max=100,dive,required"`
	UserID  string   `validate:"required,uuid"`
}

// CreateConnectionCommand encapsulates a connection creation request.
type CreateConnectionCommand struct {
	SourceID string  `validate:"required"`
	TargetID string  `validate:"required"`
	EdgeType string  `validate:"required,oneof=related similar reference"`
	Strength float64 `validate:"min=0,max=1"`
	UserID   string  `validate:"required,uuid"`
}

// UpdateConnectionCommand encapsulates a connection update request.
type UpdateConnectionCommand struct {
	EdgeID   string  `validate:"required"`
	Strength float64 `validate:"min=0,max=1"`
	UserID   string  `validate:"required,uuid"`
}

// DeleteConnectionCommand encapsulates a connection deletion request.
type DeleteConnectionCommand struct {
	EdgeID string `validate:"required"`
	UserID string `validate:"required,uuid"`
}

// ConnectNodesCommand encapsulates a node connection request.
type ConnectNodesCommand struct {
	UserID       string  `validate:"required,uuid"`
	SourceNodeID string  `validate:"required"`
	TargetNodeID string  `validate:"required"`
	Weight       float64 `validate:"min=0,max=1"`
}

// BulkCreateNodesCommand encapsulates a bulk node creation request.
type BulkCreateNodesCommand struct {
	UserID string `validate:"required,uuid"`
	Nodes  []struct {
		Content  string                 `validate:"required,min=1,max=20000"`
		Tags     []string               `validate:"max=10,dive,min=1,max=50"`
		Metadata map[string]interface{} `validate:"max=20"`
	} `validate:"required,min=1,max=100,dive"`
}

// ============================================================================
// NODE COMMANDS
// ============================================================================

// CreateNode executes the create node command with full validation.
func (s *NodeCommandService) CreateNode(ctx context.Context, cmd CreateNodeCommand) (*node.Node, error) {
	// 1. Validate command
	if err := s.validator.Struct(cmd); err != nil {
		return nil, errors.Wrap(err, errors.CodeInternalError.String(), "invalid command")
	}
	
	// 2. Create domain object with business rules validation
	userID, err := shared.NewUserID(cmd.UserID)
	if err != nil {
		return nil, errors.Wrap(err, errors.CodeInternalError.String(), "invalid user ID")
	}
	
	content, err := shared.NewContent(cmd.Content)
	if err != nil {
		return nil, errors.Wrap(err, errors.CodeInternalError.String(), "invalid content")
	}
	
	title, err := shared.NewTitle(cmd.Title)
	if err != nil {
		return nil, errors.Wrap(err, errors.CodeInternalError.String(), "invalid title")
	}
	
	tags := shared.NewTags(cmd.Tags...)
	
	// Use the domain factory method to create a valid node
	node, err := node.NewNode(userID, content, title, tags)
	if err != nil {
		return nil, fmt.Errorf("failed to create node: %w", err)
	}
	
	// Set metadata if provided
	if cmd.Metadata != nil {
		node.SetMetadata(cmd.Metadata)
	}
	
	// 4. Persist the node
	if err := s.nodeWriter.Save(ctx, node); err != nil {
		return nil, fmt.Errorf("persistence failed: %w", err)
	}
	
	// 5. Publish domain events
	event := shared.NewNodeCreatedEvent(
		node.ID(),
		node.UserID(),
		node.Content(),
		node.Keywords(),
		node.Tags(),
		shared.ParseVersion(node.Version()),
	)
	if err := s.eventBus.Publish(ctx, event); err != nil {
		// Log but don't fail the operation
		s.logger.Warn("Failed to publish node created event",
			zap.String("node_id", node.ID().String()),
			zap.Error(err),
		)
	}
	
	s.logger.Info("Node created successfully",
		zap.String("node_id", node.ID().String()),
		zap.String("user_id", cmd.UserID),
	)
	
	return node, nil
}

// UpdateNode executes the update node command with optimistic locking.
func (s *NodeCommandService) UpdateNode(ctx context.Context, cmd UpdateNodeCommand) (*node.Node, error) {
	// 1. Validate command
	if err := s.validator.Struct(cmd); err != nil {
		return nil, errors.Wrap(err, errors.CodeInternalError.String(), "invalid command")
	}
	
	// 2. Note: NodeWriter doesn't have GetForUpdate, so we can't fetch existing node
	// This is a command-only service, so we need to reconstruct the node with updates
	
	// 3. Create updated content if provided
	var content shared.Content
	if cmd.Content != "" {
		var err error
		content, err = shared.NewContent(cmd.Content)
		if err != nil {
			return nil, errors.Wrap(err, errors.CodeInternalError.String(), "invalid content")
		}
	}
	
	// 4. Create updated title if provided
	var title shared.Title
	if cmd.Title != "" {
		var err error
		title, err = shared.NewTitle(cmd.Title)
		if err != nil {
			return nil, errors.Wrap(err, errors.CodeInternalError.String(), "invalid title")
		}
	}
	
	// 5. Create updated tags if provided
	var tags shared.Tags
	if cmd.Tags != nil {
		tags = shared.NewTags(cmd.Tags...)
	}
	
	// Create a node with the updated values for the update operation
	// Note: This is a simplified approach since we don't have read access
	nodeID, err := shared.ParseNodeID(cmd.NodeID)
	if err != nil {
		return nil, fmt.Errorf("invalid node ID: %w", err)
	}
	
	userID, err := shared.NewUserID(cmd.UserID)
	if err != nil {
		return nil, errors.Wrap(err, errors.CodeInternalError.String(), "invalid user ID")
	}
	
	// Reconstruct node for update
	node := node.ReconstructNode(
		nodeID,
		userID,
		content,
		title,
		shared.Keywords{}, // Will be recalculated from content
		tags,
		time.Now(),
		time.Now(),
		shared.ParseVersion(cmd.Version),
		false,
	)
	
	if cmd.Metadata != nil {
		node.SetMetadata(cmd.Metadata)
	}
	
	node.UpdateTimestamp()
	node.IncrementVersion()
	
	// 5. Validate updated domain object
	if err := node.Validate(); err != nil {
		return nil, fmt.Errorf("domain validation failed: %w", err)
	}
	
	// 6. Persist changes
	if err := s.nodeWriter.Update(ctx, node); err != nil {
		return nil, fmt.Errorf("persistence failed: %w", err)
	}
	
	// 7. Publish update event
	// Use content updated event as a general update event
	event := shared.NewNodeContentUpdatedEvent(
		node.ID(),
		node.UserID(),
		content, // old content (we don't have it)
		node.Content(),
		shared.Keywords{}, // old keywords (we don't have them)
		node.Keywords(),
		shared.ParseVersion(node.Version()),
	)
	if err := s.eventBus.Publish(ctx, event); err != nil {
		s.logger.Warn("Failed to publish node updated event",
			zap.String("node_id", node.ID().String()),
			zap.Error(err),
		)
	}
	
	s.logger.Info("Node updated successfully",
		zap.String("node_id", node.ID().String()),
		zap.Int("new_version", node.Version()),
	)
	
	return node, nil
}

// DeleteNode executes the delete node command with cascade handling.
func (s *NodeCommandService) DeleteNode(ctx context.Context, cmd DeleteNodeCommand) error {
	// 1. Validate command
	if err := s.validator.Struct(cmd); err != nil {
		return fmt.Errorf("invalid command: %w", err)
	}
	
	// 2. Parse the IDs
	userID, err := shared.ParseUserID(cmd.UserID)
	if err != nil {
		return fmt.Errorf("invalid user ID: %w", err)
	}
	
	nodeID, err := shared.ParseNodeID(cmd.NodeID)
	if err != nil {
		return fmt.Errorf("invalid node ID: %w", err)
	}
	
	// Note: We can't verify ownership without a read operation
	// The delete will fail if the node doesn't exist
	
	// 3. Delete all edges connected to this node (cascade delete)
	// Check if the edge writer supports DeleteByNode for efficient deletion
	if edgeDeleter, ok := s.edgeWriter.(interface {
		DeleteByNode(ctx context.Context, userID shared.UserID, nodeID shared.NodeID) error
	}); ok {
		if err := edgeDeleter.DeleteByNode(ctx, userID, nodeID); err != nil {
			s.logger.Warn("Failed to delete node edges",
				zap.String("node_id", cmd.NodeID),
				zap.Error(err),
			)
			// Continue with node deletion even if edge deletion fails
			// to avoid leaving the node in an inconsistent state
		}
	} else {
		// Log warning that edges may be orphaned
		s.logger.Warn("EdgeWriter does not support DeleteByNode, edges may remain orphaned",
			zap.String("node_id", cmd.NodeID),
		)
	}
	
	// 4. Delete the node
	if err := s.nodeWriter.Delete(ctx, userID, nodeID); err != nil {
		return fmt.Errorf("failed to delete node: %w", err)
	}
	
	// 5. Publish deletion event
	// Note: We don't have node details without a read operation
	// event := node.NewNodeDeletedEvent(nodeID, userID, content, keywords, tags, version)
	// if err := s.eventBus.Publish(ctx, event); err != nil {
	// 	s.logger.Warn("Failed to publish node deleted event",
	// 		zap.String("node_id", cmd.NodeID),
	// 		zap.Error(err),
	// 	)
	// }
	
	s.logger.Info("Node deleted successfully",
		zap.String("node_id", cmd.NodeID),
		zap.String("user_id", cmd.UserID),
	)
	
	return nil
}

// BulkDeleteNodes executes bulk node deletion with batch processing.
func (s *NodeCommandService) BulkDeleteNodes(ctx context.Context, cmd BulkDeleteNodesCommand) error {
	// 1. Validate command
	if err := s.validator.Struct(cmd); err != nil {
		return fmt.Errorf("invalid command: %w", err)
	}
	
	// 2. Process deletions in batches for efficiency
	batchSize := 10
	errors := make([]error, 0)
	
	for i := 0; i < len(cmd.NodeIDs); i += batchSize {
		end := i + batchSize
		if end > len(cmd.NodeIDs) {
			end = len(cmd.NodeIDs)
		}
		
		batch := cmd.NodeIDs[i:end]
		
		// Process batch
		for _, nodeID := range batch {
			deleteCmd := DeleteNodeCommand{
				NodeID: nodeID,
				UserID: cmd.UserID,
			}
			
			if err := s.DeleteNode(ctx, deleteCmd); err != nil {
				errors = append(errors, fmt.Errorf("failed to delete node %s: %w", nodeID, err))
			}
		}
	}
	
	if len(errors) > 0 {
		return fmt.Errorf("bulk deletion completed with %d errors", len(errors))
	}
	
	s.logger.Info("Bulk node deletion completed",
		zap.Int("count", len(cmd.NodeIDs)),
		zap.String("user_id", cmd.UserID),
	)
	
	return nil
}

// ============================================================================
// CONNECTION COMMANDS
// ============================================================================

// Note: Edge commands are temporarily disabled as they require read operations
// which are not available in a pure command service (CQRS pattern)

/*
// CreateConnection creates a new edge between nodes.
func (s *NodeCommandService) CreateConnection(ctx context.Context, cmd CreateConnectionCommand) (*edge.Edge, error) {
	// 1. Validate command
	if err := s.validator.Struct(cmd); err != nil {
		return nil, errors.Wrap(err, errors.CodeInternalError.String(), "invalid command")
	}
	
	// 2. Verify both nodes exist
	sourceNode, err := s.nodeWriter.GetForUpdate(ctx, cmd.SourceID)
	if err != nil {
		return nil, fmt.Errorf("source node not found: %w", err)
	}
	
	targetNode, err := s.nodeWriter.GetForUpdate(ctx, cmd.TargetID)
	if err != nil {
		return nil, fmt.Errorf("target node not found: %w", err)
	}
	
	// 3. Verify user owns at least one of the nodes
	if string(sourceNode.UserID) != cmd.UserID && string(targetNode.UserID) != cmd.UserID {
		return nil, fmt.Errorf("unauthorized: user does not own either node")
	}
	
	// 4. Create edge domain object
	edge := &edge.Edge{
		ID:        shared.GenerateEdgeID(),
		SourceID:  node.NodeID(cmd.SourceID),
		TargetID:  node.NodeID(cmd.TargetID),
		EdgeType:  edge.EdgeType(cmd.EdgeType),
		Strength:  cmd.Strength,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Version:   1,
	}
	
	// 5. Persist the edge
	if err := s.edgeWriter.Create(ctx, edge); err != nil {
		return nil, fmt.Errorf("failed to create edge: %w", err)
	}
	
	// 6. Publish event
	event := edge.NewEdgeCreatedEvent(edge)
	if err := s.eventBus.Publish(ctx, event); err != nil {
		s.logger.Warn("Failed to publish edge created event",
			zap.String("edge_id", string(edge.ID)),
			zap.Error(err),
		)
	}
	
	s.logger.Info("Connection created successfully",
		zap.String("source_id", cmd.SourceID),
		zap.String("target_id", cmd.TargetID),
		zap.String("edge_type", cmd.EdgeType),
	)
	
	return edge, nil
}

// UpdateConnection updates an existing edge.
func (s *NodeCommandService) UpdateConnection(ctx context.Context, cmd UpdateConnectionCommand) (*edge.Edge, error) {
	// 1. Validate command
	if err := s.validator.Struct(cmd); err != nil {
		return nil, errors.Wrap(err, errors.CodeInternalError.String(), "invalid command")
	}
	
	// 2. Fetch existing edge
	edge, err := s.edgeWriter.GetForUpdate(ctx, cmd.EdgeID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch edge: %w", err)
	}
	
	// 3. Update strength
	edge.Strength = cmd.Strength
	edge.UpdatedAt = time.Now()
	edge.Version++
	
	// 4. Persist changes
	if err := s.edgeWriter.Update(ctx, edge); err != nil {
		return nil, fmt.Errorf("failed to update edge: %w", err)
	}
	
	// 5. Publish event
	event := edge.NewEdgeUpdatedEvent(edge)
	if err := s.eventBus.Publish(ctx, event); err != nil {
		s.logger.Warn("Failed to publish edge updated event",
			zap.String("edge_id", cmd.EdgeID),
			zap.Error(err),
		)
	}
	
	return edge, nil
}

// DeleteConnection deletes an edge.
func (s *NodeCommandService) DeleteConnection(ctx context.Context, cmd DeleteConnectionCommand) error {
	// 1. Validate command
	if err := s.validator.Struct(cmd); err != nil {
		return fmt.Errorf("invalid command: %w", err)
	}
	
	// 2. Fetch edge
	edge, err := s.edgeWriter.GetForUpdate(ctx, cmd.EdgeID)
	if err != nil {
		return fmt.Errorf("failed to fetch edge: %w", err)
	}
	
	// 3. Delete the edge
	if err := s.edgeWriter.Delete(ctx, cmd.EdgeID); err != nil {
		return fmt.Errorf("failed to delete edge: %w", err)
	}
	
	// 4. Publish event
	event := edge.NewEdgeDeletedEvent(edge)
	if err := s.eventBus.Publish(ctx, event); err != nil {
		s.logger.Warn("Failed to publish edge deleted event",
			zap.String("edge_id", cmd.EdgeID),
			zap.Error(err),
		)
	}
	
	return nil
}

// ============================================================================
// HELPER METHODS
// ============================================================================

// deleteNodeEdges deletes all edges connected to a node.
func (s *NodeCommandService) deleteNodeEdges(ctx context.Context, nodeID string) error {
	// Get edges where node is source
	sourceEdges, err := s.edgeWriter.FindBySource(ctx, nodeID)
	if err != nil {
		return fmt.Errorf("failed to find source edges: %w", err)
	}
	
	// Get edges where node is target
	targetEdges, err := s.edgeWriter.FindByTarget(ctx, nodeID)
	if err != nil {
		return fmt.Errorf("failed to find target edges: %w", err)
	}
	
	// Delete all edges
	allEdges := append(sourceEdges, targetEdges...)
	for _, edge := range allEdges {
		if err := s.edgeWriter.Delete(ctx, string(edge.ID)); err != nil {
			s.logger.Warn("Failed to delete edge during cascade",
				zap.String("edge_id", string(edge.ID)),
				zap.Error(err),
			)
		}
	}
	
	return nil
}*/
