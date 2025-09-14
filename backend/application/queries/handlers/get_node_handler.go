package handlers

import (
	"context"
	"fmt"
	"time"

	"backend/application/ports"
	"backend/application/queries"
	"backend/domain/core/valueobjects"
	"go.uber.org/zap"
)

// GetNodeHandler handles get node queries
type GetNodeHandler struct {
	nodeRepo ports.NodeRepository
	logger   *zap.Logger
}

// NewGetNodeHandler creates a new get node handler
func NewGetNodeHandler(nodeRepo ports.NodeRepository, logger *zap.Logger) *GetNodeHandler {
	return &GetNodeHandler{
		nodeRepo: nodeRepo,
		logger:   logger,
	}
}

// Handle executes the get node query
func (h *GetNodeHandler) Handle(ctx context.Context, query queries.GetNodeQuery) (*queries.GetNodeResult, error) {
	// Validate UserID first
	if query.UserID == "" {
		return nil, fmt.Errorf("invalid node ID: user ID is required")
	}

	// Create NodeID value object from string
	nodeID, err := valueobjects.NewNodeIDFromString(query.NodeID)
	if err != nil {
		return nil, fmt.Errorf("invalid node ID: %w", err)
	}

	// Get node from repository
	node, err := h.nodeRepo.GetByID(ctx, nodeID)
	if err != nil {
		return nil, fmt.Errorf("failed to get node: %w", err)
	}

	// Verify ownership
	if node.UserID() != query.UserID {
		return nil, fmt.Errorf("node does not belong to user")
	}

	// Map domain model to query result
	content := node.Content()
	position := node.Position()

	result := &queries.GetNodeResult{
		ID:      node.ID().String(),
		UserID:  node.UserID(),
		Title:   content.Title(),
		Content: content.Body(),
		Format:  string(content.Format()),
		Position: queries.Position{
			X: position.X(),
			Y: position.Y(),
			Z: position.Z(),
		},
		Tags:      node.GetTags(),
		Metadata:  make(map[string]string),
		Version:   node.Version(),
		CreatedAt: node.CreatedAt().Format(time.RFC3339),
		UpdatedAt: node.UpdatedAt().Format(time.RFC3339),
	}

	h.logger.Debug("Node retrieved",
		zap.String("nodeID", query.NodeID),
		zap.String("userID", query.UserID),
	)

	return result, nil
}
