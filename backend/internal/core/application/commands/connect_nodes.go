// Package commands contains CQRS command implementations for write operations
package commands

import (
	"context"
	"fmt"
	"time"
	
	"brain2-backend/internal/core/application/cqrs"
	"brain2-backend/internal/core/application/ports"
	"brain2-backend/internal/core/domain/events"
)

// ConnectNodesCommand represents a command to connect two nodes
type ConnectNodesCommand struct {
	cqrs.BaseCommand
	SourceNodeID string  `json:"source_node_id"`
	TargetNodeID string  `json:"target_node_id"`
	Type         string  `json:"type"`
	Weight       float64 `json:"weight"`
}

// GetCommandName returns the command name
func (c ConnectNodesCommand) GetCommandName() string {
	return "ConnectNodes"
}

// Validate validates the command
func (c ConnectNodesCommand) Validate() error {
	if c.UserID == "" {
		return fmt.Errorf("user ID is required")
	}
	if c.SourceNodeID == "" {
		return fmt.Errorf("source node ID is required")
	}
	if c.TargetNodeID == "" {
		return fmt.Errorf("target node ID is required")
	}
	if c.SourceNodeID == c.TargetNodeID {
		return fmt.Errorf("cannot connect node to itself")
	}
	if c.Weight < 0 || c.Weight > 1 {
		return fmt.Errorf("weight must be between 0 and 1")
	}
	return nil
}

// ConnectNodesHandler handles node connection commands
type ConnectNodesHandler struct {
	nodeRepo      ports.NodeRepository
	edgeRepo      ports.EdgeRepository
	graphAnalyzer ports.GraphAnalyzer
	eventBus      ports.EventBus
	logger        ports.Logger
	metrics       ports.Metrics
}

// NewConnectNodesHandler creates a new connect nodes handler
func NewConnectNodesHandler(
	nodeRepo ports.NodeRepository,
	edgeRepo ports.EdgeRepository,
	graphAnalyzer ports.GraphAnalyzer,
	eventBus ports.EventBus,
	logger ports.Logger,
	metrics ports.Metrics,
) *ConnectNodesHandler {
	return &ConnectNodesHandler{
		nodeRepo:      nodeRepo,
		edgeRepo:      edgeRepo,
		graphAnalyzer: graphAnalyzer,
		eventBus:      eventBus,
		logger:        logger,
		metrics:       metrics,
	}
}

// Handle processes the connect nodes command
func (h *ConnectNodesHandler) Handle(ctx context.Context, command cqrs.Command) error {
	cmd, ok := command.(*ConnectNodesCommand)
	if !ok {
		return fmt.Errorf("invalid command type")
	}
	
	// Start metrics
	timer := h.metrics.StartTimer("command.connect_nodes.duration")
	defer timer.Stop()
	
	// Verify both nodes exist and belong to the user
	sourceNode, err := h.nodeRepo.FindByID(ctx, cmd.SourceNodeID)
	if err != nil {
		h.metrics.IncrementCounter("command.connect_nodes.source_not_found")
		return fmt.Errorf("source node not found: %w", err)
	}
	
	if sourceNode.GetUserID() != cmd.UserID {
		h.metrics.IncrementCounter("command.connect_nodes.unauthorized")
		return fmt.Errorf("unauthorized: source node does not belong to user")
	}
	
	targetNode, err := h.nodeRepo.FindByID(ctx, cmd.TargetNodeID)
	if err != nil {
		h.metrics.IncrementCounter("command.connect_nodes.target_not_found")
		return fmt.Errorf("target node not found: %w", err)
	}
	
	if targetNode.GetUserID() != cmd.UserID {
		h.metrics.IncrementCounter("command.connect_nodes.unauthorized")
		return fmt.Errorf("unauthorized: target node does not belong to user")
	}
	
	// Check if connection would create a cycle (if graph should be acyclic)
	if h.graphAnalyzer != nil {
		wouldCycle, err := h.graphAnalyzer.WouldCreateCycle(ctx, cmd.SourceNodeID, cmd.TargetNodeID)
		if err != nil {
			h.logger.Warn("Failed to check for cycles",
				ports.Field{Key: "error", Value: err.Error()})
		} else if wouldCycle {
			h.metrics.IncrementCounter("command.connect_nodes.cycle_detected")
			return fmt.Errorf("connection would create a cycle")
		}
	}
	
	// Check if edge already exists
	existingEdge, err := h.edgeRepo.GetEdge(ctx, cmd.SourceNodeID, cmd.TargetNodeID)
	if err == nil && existingEdge != nil {
		// Update existing edge
		existingEdge.Weight = cmd.Weight
		existingEdge.UpdatedAt = time.Now()
		if cmd.Type != "" {
			existingEdge.Type = cmd.Type
		}
		
		if err := h.edgeRepo.UpdateEdge(ctx, existingEdge); err != nil {
			return fmt.Errorf("failed to update edge: %w", err)
		}
		
		h.logger.Info("Edge updated",
			ports.Field{Key: "source", Value: cmd.SourceNodeID},
			ports.Field{Key: "target", Value: cmd.TargetNodeID})
	} else {
		// Create new edge
		edge := &ports.Edge{
			ID:        fmt.Sprintf("%s-%s", cmd.SourceNodeID, cmd.TargetNodeID),
			SourceID:  cmd.SourceNodeID,
			TargetID:  cmd.TargetNodeID,
			Type:      cmd.Type,
			Weight:    cmd.Weight,
			Strength:  cmd.Weight, // Initial strength equals weight
			UserID:    cmd.UserID,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
			Metadata:  make(map[string]interface{}),
		}
		
		if edge.Type == "" {
			edge.Type = "related"
		}
		
		if err := h.edgeRepo.CreateEdge(ctx, edge); err != nil {
			return fmt.Errorf("failed to create edge: %w", err)
		}
		
		h.logger.Info("Edge created",
			ports.Field{Key: "source", Value: cmd.SourceNodeID},
			ports.Field{Key: "target", Value: cmd.TargetNodeID})
	}
	
	// Update graph metrics
	if h.graphAnalyzer != nil {
		nodeIDs := []string{cmd.SourceNodeID, cmd.TargetNodeID}
		if err := h.graphAnalyzer.UpdateCentrality(ctx, cmd.UserID, nodeIDs); err != nil {
			h.logger.Warn("Failed to update centrality",
				ports.Field{Key: "error", Value: err.Error()})
		}
		
		if err := h.graphAnalyzer.UpdateClustering(ctx, cmd.UserID, cmd.SourceNodeID); err != nil {
			h.logger.Warn("Failed to update clustering",
				ports.Field{Key: "error", Value: err.Error()})
		}
	}
	
	// Publish nodes connected event
	event := events.NodesConnected{
		SourceNodeID: cmd.SourceNodeID,
		TargetNodeID: cmd.TargetNodeID,
		UserID:       cmd.UserID,
		Weight:       cmd.Weight,
		Timestamp:    cmd.Timestamp.Unix(),
	}
	
	if err := h.eventBus.Publish(ctx, event); err != nil {
		h.logger.Error("Failed to publish nodes connected event", err,
			ports.Field{Key: "source", Value: cmd.SourceNodeID},
			ports.Field{Key: "target", Value: cmd.TargetNodeID})
	}
	
	h.metrics.IncrementCounter("command.connect_nodes.success")
	
	return nil
}

// CanHandle checks if this handler can handle the command
func (h *ConnectNodesHandler) CanHandle(command cqrs.Command) bool {
	_, ok := command.(*ConnectNodesCommand)
	return ok
}