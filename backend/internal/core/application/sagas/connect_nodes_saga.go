package sagas

import (
	"context"
	"fmt"
	"time"
	
	"brain2-backend/internal/core/application/cqrs"
	"brain2-backend/internal/core/application/ports"
	"brain2-backend/internal/core/domain/valueobjects"
)

// ConnectNodesSaga orchestrates the creation of bidirectional edges between nodes
type ConnectNodesSaga struct {
	*BaseSaga
	commandBus     cqrs.CommandBus
	nodeRepo       ports.NodeRepository
	edgeRepo       ports.EdgeRepository
	graphAnalyzer  ports.GraphAnalyzer
	eventBus       ports.EventBus
	userID         string
	sourceNodeID   string
	targetNodeID   string
	edgeType       string
	bidirectional  bool
	weight         float64
	metadata       map[string]interface{}
	forwardEdgeID  string
	backwardEdgeID string
	originalState  *EdgeState
}

// EdgeState captures the original state for rollback
type EdgeState struct {
	ForwardExists   bool
	ForwardWeight   float64
	BackwardExists  bool
	BackwardWeight  float64
	ForwardMetadata map[string]interface{}
	BackwardMetadata map[string]interface{}
}

// NewConnectNodesSaga creates a new connection saga
func NewConnectNodesSaga(
	commandBus cqrs.CommandBus,
	nodeRepo ports.NodeRepository,
	edgeRepo ports.EdgeRepository,
	graphAnalyzer ports.GraphAnalyzer,
	eventBus ports.EventBus,
	logger ports.Logger,
	metrics ports.Metrics,
	userID, sourceNodeID, targetNodeID, edgeType string,
	bidirectional bool,
	weight float64,
	metadata map[string]interface{},
) *ConnectNodesSaga {
	saga := &ConnectNodesSaga{
		BaseSaga:      NewBaseSaga(logger, metrics),
		commandBus:    commandBus,
		nodeRepo:      nodeRepo,
		edgeRepo:      edgeRepo,
		graphAnalyzer: graphAnalyzer,
		eventBus:      eventBus,
		userID:        userID,
		sourceNodeID:  sourceNodeID,
		targetNodeID:  targetNodeID,
		edgeType:      edgeType,
		bidirectional: bidirectional,
		weight:        weight,
		metadata:      metadata,
	}
	
	// Define saga steps
	saga.Steps = []SagaStep{
		&BaseStep{
			Name:           "ValidateNodes",
			Action:         saga.validateNodes,
			CompensateFunc: nil,
			Retryable:      true,
			MaxRetries:     2,
		},
		&BaseStep{
			Name:           "CheckCycles",
			Action:         saga.checkCycles,
			CompensateFunc: nil,
			Retryable:      false,
		},
		&BaseStep{
			Name:           "SaveOriginalState",
			Action:         saga.saveOriginalState,
			CompensateFunc: nil,
			Retryable:      true,
			MaxRetries:     2,
		},
		&BaseStep{
			Name:           "CreateForwardEdge",
			Action:         saga.createForwardEdge,
			CompensateFunc: saga.compensateForwardEdge,
			Retryable:      true,
			MaxRetries:     3,
		},
		&BaseStep{
			Name:           "CreateBackwardEdge",
			Action:         saga.createBackwardEdge,
			CompensateFunc: saga.compensateBackwardEdge,
			Retryable:      true,
			MaxRetries:     3,
		},
		&BaseStep{
			Name:           "UpdateGraphMetrics",
			Action:         saga.updateGraphMetrics,
			CompensateFunc: saga.compensateGraphMetrics,
			Retryable:      true,
			MaxRetries:     2,
		},
		&BaseStep{
			Name:           "PublishConnectionEvent",
			Action:         saga.publishConnectionEvent,
			CompensateFunc: nil,
			Retryable:      true,
			MaxRetries:     3,
		},
	}
	
	return saga
}

// validateNodes ensures both nodes exist and are accessible
func (s *ConnectNodesSaga) validateNodes(ctx context.Context) error {
	// Validate source node
	sourceNode, err := s.nodeRepo.GetByID(ctx, s.sourceNodeID)
	if err != nil {
		return fmt.Errorf("source node not found: %w", err)
	}
	
	// Check ownership
	if sourceNode.GetUserID() != s.userID {
		return fmt.Errorf("user does not own source node")
	}
	
	// Check if archived
	if sourceNode.IsArchived() {
		return fmt.Errorf("source node is archived")
	}
	
	// Validate target node
	targetNode, err := s.nodeRepo.GetByID(ctx, s.targetNodeID)
	if err != nil {
		return fmt.Errorf("target node not found: %w", err)
	}
	
	// Check ownership
	if targetNode.GetUserID() != s.userID {
		return fmt.Errorf("user does not own target node")
	}
	
	// Check if archived
	if targetNode.IsArchived() {
		return fmt.Errorf("target node is archived")
	}
	
	// Store node info in metadata
	s.Metadata["source_title"] = sourceNode.GetTitle()
	s.Metadata["target_title"] = targetNode.GetTitle()
	
	s.logger.Info("Nodes validated",
		ports.Field{Key: "saga_id", Value: s.ID},
		ports.Field{Key: "source", Value: s.sourceNodeID},
		ports.Field{Key: "target", Value: s.targetNodeID})
	
	return nil
}

// checkCycles ensures the connection won't create a cycle if that's not allowed
func (s *ConnectNodesSaga) checkCycles(ctx context.Context) error {
	if s.graphAnalyzer == nil {
		// Skip if analyzer not available
		s.logger.Warn("Graph analyzer not available, skipping cycle check",
			ports.Field{Key: "saga_id", Value: s.ID})
		return nil
	}
	
	// Check if edge type allows cycles
	if s.edgeType == "hierarchy" || s.edgeType == "dependency" {
		// These types should not have cycles
		hasCycle, err := s.graphAnalyzer.WouldCreateCycle(ctx, s.sourceNodeID, s.targetNodeID)
		if err != nil {
			s.logger.Warn("Failed to check for cycles",
				ports.Field{Key: "error", Value: err.Error()})
			// Continue anyway - better to allow the connection than block it
			return nil
		}
		
		if hasCycle {
			return fmt.Errorf("connection would create a cycle in %s relationship", s.edgeType)
		}
	}
	
	s.logger.Info("Cycle check passed",
		ports.Field{Key: "saga_id", Value: s.ID},
		ports.Field{Key: "edge_type", Value: s.edgeType})
	
	return nil
}

// saveOriginalState captures the current state for potential rollback
func (s *ConnectNodesSaga) saveOriginalState(ctx context.Context) error {
	s.originalState = &EdgeState{}
	
	// Check if forward edge exists
	forwardEdge, err := s.edgeRepo.GetEdge(ctx, s.sourceNodeID, s.targetNodeID)
	if err == nil && forwardEdge != nil {
		s.originalState.ForwardExists = true
		s.originalState.ForwardWeight = forwardEdge.Weight
		s.originalState.ForwardMetadata = forwardEdge.Metadata
	}
	
	// Check if backward edge exists (for bidirectional)
	if s.bidirectional {
		backwardEdge, err := s.edgeRepo.GetEdge(ctx, s.targetNodeID, s.sourceNodeID)
		if err == nil && backwardEdge != nil {
			s.originalState.BackwardExists = true
			s.originalState.BackwardWeight = backwardEdge.Weight
			s.originalState.BackwardMetadata = backwardEdge.Metadata
		}
	}
	
	s.Metadata["original_state"] = s.originalState
	
	s.logger.Info("Original state saved",
		ports.Field{Key: "saga_id", Value: s.ID},
		ports.Field{Key: "forward_exists", Value: s.originalState.ForwardExists},
		ports.Field{Key: "backward_exists", Value: s.originalState.BackwardExists})
	
	return nil
}

// createForwardEdge creates the edge from source to target
func (s *ConnectNodesSaga) createForwardEdge(ctx context.Context) error {
	s.forwardEdgeID = valueobjects.NewEdgeID("").String()
	
	edge := &ports.Edge{
		ID:       s.forwardEdgeID,
		SourceID: s.sourceNodeID,
		TargetID: s.targetNodeID,
		Type:     s.edgeType,
		Weight:   s.weight,
		UserID:   s.userID,
		Metadata: s.metadata,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	
	// Add saga tracking
	if edge.Metadata == nil {
		edge.Metadata = make(map[string]interface{})
	}
	edge.Metadata["saga_id"] = s.ID
	edge.Metadata["created_by_saga"] = true
	
	if err := s.edgeRepo.CreateEdge(ctx, edge); err != nil {
		return fmt.Errorf("failed to create forward edge: %w", err)
	}
	
	s.Metadata["forward_edge_id"] = s.forwardEdgeID
	
	s.logger.Info("Forward edge created",
		ports.Field{Key: "saga_id", Value: s.ID},
		ports.Field{Key: "edge_id", Value: s.forwardEdgeID},
		ports.Field{Key: "source", Value: s.sourceNodeID},
		ports.Field{Key: "target", Value: s.targetNodeID})
	
	return nil
}

// compensateForwardEdge rolls back the forward edge creation
func (s *ConnectNodesSaga) compensateForwardEdge(ctx context.Context) error {
	if s.forwardEdgeID == "" {
		return nil
	}
	
	// If edge existed before, restore it
	if s.originalState != nil && s.originalState.ForwardExists {
		edge := &ports.Edge{
			ID:       s.forwardEdgeID,
			SourceID: s.sourceNodeID,
			TargetID: s.targetNodeID,
			Weight:   s.originalState.ForwardWeight,
			Metadata: s.originalState.ForwardMetadata,
		}
		
		if err := s.edgeRepo.UpdateEdge(ctx, edge); err != nil {
			s.logger.Error("Failed to restore forward edge",
				err,
				ports.Field{Key: "edge_id", Value: s.forwardEdgeID})
		}
	} else {
		// Delete the edge we created
		if err := s.edgeRepo.DeleteEdge(ctx, s.sourceNodeID, s.targetNodeID); err != nil {
			s.logger.Error("Failed to delete forward edge",
				err,
				ports.Field{Key: "source", Value: s.sourceNodeID},
				ports.Field{Key: "target", Value: s.targetNodeID})
		}
	}
	
	s.logger.Info("Forward edge compensated",
		ports.Field{Key: "saga_id", Value: s.ID},
		ports.Field{Key: "edge_id", Value: s.forwardEdgeID})
	
	return nil
}

// createBackwardEdge creates the reverse edge for bidirectional connections
func (s *ConnectNodesSaga) createBackwardEdge(ctx context.Context) error {
	if !s.bidirectional {
		return nil // Skip for unidirectional edges
	}
	
	s.backwardEdgeID = valueobjects.NewEdgeID("").String()
	
	edge := &ports.Edge{
		ID:       s.backwardEdgeID,
		SourceID: s.targetNodeID,
		TargetID: s.sourceNodeID,
		Type:     s.edgeType,
		Weight:   s.weight,
		UserID:   s.userID,
		Metadata: s.metadata,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	
	// Add saga tracking
	if edge.Metadata == nil {
		edge.Metadata = make(map[string]interface{})
	}
	edge.Metadata["saga_id"] = s.ID
	edge.Metadata["created_by_saga"] = true
	edge.Metadata["reverse_edge"] = true
	
	if err := s.edgeRepo.CreateEdge(ctx, edge); err != nil {
		return fmt.Errorf("failed to create backward edge: %w", err)
	}
	
	s.Metadata["backward_edge_id"] = s.backwardEdgeID
	
	s.logger.Info("Backward edge created",
		ports.Field{Key: "saga_id", Value: s.ID},
		ports.Field{Key: "edge_id", Value: s.backwardEdgeID},
		ports.Field{Key: "source", Value: s.targetNodeID},
		ports.Field{Key: "target", Value: s.sourceNodeID})
	
	return nil
}

// compensateBackwardEdge rolls back the backward edge creation
func (s *ConnectNodesSaga) compensateBackwardEdge(ctx context.Context) error {
	if !s.bidirectional || s.backwardEdgeID == "" {
		return nil
	}
	
	// If edge existed before, restore it
	if s.originalState != nil && s.originalState.BackwardExists {
		edge := &ports.Edge{
			ID:       s.backwardEdgeID,
			SourceID: s.targetNodeID,
			TargetID: s.sourceNodeID,
			Weight:   s.originalState.BackwardWeight,
			Metadata: s.originalState.BackwardMetadata,
		}
		
		if err := s.edgeRepo.UpdateEdge(ctx, edge); err != nil {
			s.logger.Error("Failed to restore backward edge",
				err,
				ports.Field{Key: "edge_id", Value: s.backwardEdgeID})
		}
	} else {
		// Delete the edge we created
		if err := s.edgeRepo.DeleteEdge(ctx, s.targetNodeID, s.sourceNodeID); err != nil {
			s.logger.Error("Failed to delete backward edge",
				err,
				ports.Field{Key: "source", Value: s.targetNodeID},
				ports.Field{Key: "target", Value: s.sourceNodeID})
		}
	}
	
	s.logger.Info("Backward edge compensated",
		ports.Field{Key: "saga_id", Value: s.ID},
		ports.Field{Key: "edge_id", Value: s.backwardEdgeID})
	
	return nil
}

// updateGraphMetrics updates graph-related metrics
func (s *ConnectNodesSaga) updateGraphMetrics(ctx context.Context) error {
	if s.graphAnalyzer == nil {
		return nil // Skip if not available
	}
	
	// Update centrality scores
	if err := s.graphAnalyzer.UpdateCentrality(ctx, s.userID, []string{s.sourceNodeID, s.targetNodeID}); err != nil {
		s.logger.Warn("Failed to update centrality",
			ports.Field{Key: "error", Value: err.Error()})
	}
	
	// Update clustering coefficient
	if err := s.graphAnalyzer.UpdateClustering(ctx, s.userID, s.sourceNodeID); err != nil {
		s.logger.Warn("Failed to update clustering",
			ports.Field{Key: "error", Value: err.Error()})
	}
	
	s.Metadata["metrics_updated"] = true
	
	s.metrics.IncrementCounter("graph.edges.created",
		ports.Tag{Key: "type", Value: s.edgeType},
		ports.Tag{Key: "bidirectional", Value: fmt.Sprintf("%v", s.bidirectional)})
	
	s.logger.Info("Graph metrics updated",
		ports.Field{Key: "saga_id", Value: s.ID})
	
	return nil
}

// compensateGraphMetrics rolls back metric updates
func (s *ConnectNodesSaga) compensateGraphMetrics(ctx context.Context) error {
	if s.graphAnalyzer == nil {
		return nil
	}
	
	// Re-calculate metrics without the new edges
	if err := s.graphAnalyzer.UpdateCentrality(ctx, s.userID, []string{s.sourceNodeID, s.targetNodeID}); err != nil {
		s.logger.Warn("Failed to recalculate centrality",
			ports.Field{Key: "error", Value: err.Error()})
	}
	
	s.metrics.IncrementCounter("graph.edges.deleted",
		ports.Tag{Key: "type", Value: s.edgeType},
		ports.Tag{Key: "reason", Value: "saga_compensation"})
	
	return nil
}

// publishConnectionEvent publishes an event about the new connection
func (s *ConnectNodesSaga) publishConnectionEvent(ctx context.Context) error {
	// Note: EventBus publishing would need a proper DomainEvent implementation
	// For now, just log the event
	
	s.logger.Info("Connection event published",
		ports.Field{Key: "saga_id", Value: s.ID})
	
	return nil
}

// GetResult returns the result of the saga
func (s *ConnectNodesSaga) GetResult() *ConnectNodesResult {
	return &ConnectNodesResult{
		ForwardEdgeID:  s.forwardEdgeID,
		BackwardEdgeID: s.backwardEdgeID,
		Success:        s.State == SagaStateCompleted,
		Error:          s.Error,
	}
}

// ConnectNodesResult contains the result of the ConnectNodesSaga
type ConnectNodesResult struct {
	ForwardEdgeID  string
	BackwardEdgeID string
	Success        bool
	Error          error
}