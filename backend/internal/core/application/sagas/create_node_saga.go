// Package sagas implements concrete saga implementations for complex business operations
package sagas

import (
	"context"
	"fmt"
	"time"
	
	"brain2-backend/internal/core/application/commands"
	"brain2-backend/internal/core/application/cqrs"
	"brain2-backend/internal/core/application/ports"
	"brain2-backend/internal/core/domain/valueobjects"
)

// CreateNodeSaga orchestrates the creation of a node with all related operations
type CreateNodeSaga struct {
	*BaseSaga
	commandBus      cqrs.CommandBus
	nodeRepo        ports.NodeRepository
	edgeRepo        ports.EdgeRepository
	keywordService  ports.KeywordExtractor
	connectionSvc   ports.ConnectionAnalyzer
	searchService   ports.SearchService
	nodeID          string
	userID          string
	content         string
	title           string
	tags            []string
	categoryIDs     []string
	extractedKeywords []string
	suggestedConnections []string
	createdNodeID   string
}

// NewCreateNodeSaga creates a new node creation saga
func NewCreateNodeSaga(
	commandBus cqrs.CommandBus,
	nodeRepo ports.NodeRepository,
	edgeRepo ports.EdgeRepository,
	keywordService ports.KeywordExtractor,
	connectionSvc ports.ConnectionAnalyzer,
	searchService ports.SearchService,
	logger ports.Logger,
	metrics ports.Metrics,
	userID, content, title string,
	tags []string,
	categoryIDs []string,
) *CreateNodeSaga {
	saga := &CreateNodeSaga{
		BaseSaga:       NewBaseSaga(logger, metrics),
		commandBus:     commandBus,
		nodeRepo:       nodeRepo,
		edgeRepo:       edgeRepo,
		keywordService: keywordService,
		connectionSvc:  connectionSvc,
		searchService:  searchService,
		userID:         userID,
		content:        content,
		title:          title,
		tags:           tags,
		categoryIDs:    categoryIDs,
	}
	
	// Define saga steps
	saga.Steps = []SagaStep{
		&BaseStep{
			Name:           "ValidateInput",
			Action:         saga.validateInput,
			CompensateFunc: nil, // No compensation needed
			Retryable:      false,
		},
		&BaseStep{
			Name:           "ExtractKeywords",
			Action:         saga.extractKeywords,
			CompensateFunc: nil, // No compensation needed
			Retryable:      true,
			MaxRetries:     3,
		},
		&BaseStep{
			Name:           "CreateNode",
			Action:         saga.createNode,
			CompensateFunc: saga.compensateCreateNode,
			Retryable:      true,
			MaxRetries:     2,
		},
		&BaseStep{
			Name:           "FindConnections",
			Action:         saga.findConnections,
			CompensateFunc: nil, // Non-critical step
			Retryable:      true,
			MaxRetries:     2,
		},
		&BaseStep{
			Name:           "CreateConnections",
			Action:         saga.createConnections,
			CompensateFunc: saga.compensateCreateConnections,
			Retryable:      true,
			MaxRetries:     2,
		},
		&BaseStep{
			Name:           "UpdateSearchIndex",
			Action:         saga.updateSearchIndex,
			CompensateFunc: saga.compensateUpdateSearchIndex,
			Retryable:      true,
			MaxRetries:     3,
		},
		&BaseStep{
			Name:           "NotifySubscribers",
			Action:         saga.notifySubscribers,
			CompensateFunc: nil, // Non-critical step
			Retryable:      true,
			MaxRetries:     1,
		},
	}
	
	return saga
}

// validateInput validates the input data
func (s *CreateNodeSaga) validateInput(ctx context.Context) error {
	if s.userID == "" {
		return fmt.Errorf("user ID is required")
	}
	if s.content == "" {
		return fmt.Errorf("content is required")
	}
	if len(s.content) > 10000 {
		return fmt.Errorf("content exceeds maximum length")
	}
	if len(s.title) > 200 {
		return fmt.Errorf("title exceeds maximum length")
	}
	if len(s.tags) > 20 {
		return fmt.Errorf("too many tags (maximum 20)")
	}
	
	s.logger.Info("Input validated",
		ports.Field{Key: "saga_id", Value: s.ID},
		ports.Field{Key: "user_id", Value: s.userID})
	
	return nil
}

// extractKeywords extracts keywords from the content
func (s *CreateNodeSaga) extractKeywords(ctx context.Context) error {
	if s.keywordService == nil {
		// Skip if service not available
		s.logger.Warn("Keyword service not available, skipping",
			ports.Field{Key: "saga_id", Value: s.ID})
		return nil
	}
	
	keywords, err := s.keywordService.Extract(ctx, s.content)
	if err != nil {
		return fmt.Errorf("failed to extract keywords: %w", err)
	}
	
	s.extractedKeywords = keywords
	s.Metadata["keywords"] = keywords
	
	s.logger.Info("Keywords extracted",
		ports.Field{Key: "saga_id", Value: s.ID},
		ports.Field{Key: "count", Value: len(keywords)})
	
	return nil
}

// createNode creates the node using the command bus
func (s *CreateNodeSaga) createNode(ctx context.Context) error {
	// Generate node ID
	s.nodeID = valueobjects.NewNodeID("").String()
	
	// Create command
	cmd := &commands.CreateNodeCommand{}
	cmd.UserID = s.userID
	cmd.Content = s.content
	cmd.Title = s.title
	cmd.Tags = s.tags
	cmd.CategoryIDs = s.categoryIDs
	cmd.IdempotencyKey = fmt.Sprintf("saga-%s-node", s.ID)
	
	// Send command
	if err := s.commandBus.Send(ctx, cmd); err != nil {
		return fmt.Errorf("failed to create node: %w", err)
	}
	
	s.createdNodeID = s.nodeID
	s.Metadata["node_id"] = s.nodeID
	
	s.logger.Info("Node created",
		ports.Field{Key: "saga_id", Value: s.ID},
		ports.Field{Key: "node_id", Value: s.nodeID})
	
	return nil
}

// compensateCreateNode rolls back node creation
func (s *CreateNodeSaga) compensateCreateNode(ctx context.Context) error {
	if s.createdNodeID == "" {
		return nil // Nothing to compensate
	}
	
	// Archive the node instead of deleting it
	cmd := &commands.ArchiveNodeCommand{}
	cmd.UserID = s.userID
	cmd.NodeID = s.createdNodeID
	cmd.Reason = fmt.Sprintf("Saga compensation: %s", s.ID)
	
	if err := s.commandBus.Send(ctx, cmd); err != nil {
		s.logger.Error("Failed to compensate node creation",
			err,
			ports.Field{Key: "saga_id", Value: s.ID},
			ports.Field{Key: "node_id", Value: s.createdNodeID})
		return err
	}
	
	s.logger.Info("Node creation compensated",
		ports.Field{Key: "saga_id", Value: s.ID},
		ports.Field{Key: "node_id", Value: s.createdNodeID})
	
	return nil
}

// findConnections finds potential connections for the node
func (s *CreateNodeSaga) findConnections(ctx context.Context) error {
	if s.connectionSvc == nil {
		// Skip if service not available
		s.logger.Warn("Connection service not available, skipping",
			ports.Field{Key: "saga_id", Value: s.ID})
		return nil
	}
	
	// Find similar nodes
	connections, err := s.connectionSvc.FindSimilarNodes(ctx, s.userID, s.content, s.extractedKeywords, 5)
	if err != nil {
		// Non-critical error, log and continue
		s.logger.Warn("Failed to find connections",
			ports.Field{Key: "saga_id", Value: s.ID},
			ports.Field{Key: "error", Value: err.Error()})
		return nil
	}
	
	s.suggestedConnections = connections
	s.Metadata["suggested_connections"] = connections
	
	s.logger.Info("Connections found",
		ports.Field{Key: "saga_id", Value: s.ID},
		ports.Field{Key: "count", Value: len(connections)})
	
	return nil
}

// createConnections creates edges to suggested nodes
func (s *CreateNodeSaga) createConnections(ctx context.Context) error {
	if len(s.suggestedConnections) == 0 {
		return nil // No connections to create
	}
	
	createdEdges := []string{}
	
	for _, targetID := range s.suggestedConnections {
		// Create edge command
		cmd := &CreateEdgeCommand{
			UserID:   s.userID,
			SourceID: s.createdNodeID,
			TargetID: targetID,
			Weight:   0.5, // Default weight for auto-connections
			Metadata: map[string]interface{}{
				"auto_connected": true,
				"saga_id":        s.ID,
			},
		}
		
		if err := s.commandBus.Send(ctx, cmd); err != nil {
			s.logger.Warn("Failed to create edge",
				ports.Field{Key: "source", Value: s.createdNodeID},
				ports.Field{Key: "target", Value: targetID},
				ports.Field{Key: "error", Value: err.Error()})
			continue
		}
		
		createdEdges = append(createdEdges, targetID)
	}
	
	s.Metadata["created_edges"] = createdEdges
	
	s.logger.Info("Connections created",
		ports.Field{Key: "saga_id", Value: s.ID},
		ports.Field{Key: "count", Value: len(createdEdges)})
	
	return nil
}

// compensateCreateConnections rolls back edge creation
func (s *CreateNodeSaga) compensateCreateConnections(ctx context.Context) error {
	edges, ok := s.Metadata["created_edges"].([]string)
	if !ok || len(edges) == 0 {
		return nil // Nothing to compensate
	}
	
	for _, targetID := range edges {
		cmd := &DeleteEdgeCommand{
			UserID:   s.userID,
			SourceID: s.createdNodeID,
			TargetID: targetID,
		}
		
		if err := s.commandBus.Send(ctx, cmd); err != nil {
			s.logger.Warn("Failed to compensate edge creation",
				ports.Field{Key: "source", Value: s.createdNodeID},
				ports.Field{Key: "target", Value: targetID})
		}
	}
	
	s.logger.Info("Connections compensated",
		ports.Field{Key: "saga_id", Value: s.ID},
		ports.Field{Key: "count", Value: len(edges)})
	
	return nil
}

// updateSearchIndex updates the search index with the new node
func (s *CreateNodeSaga) updateSearchIndex(ctx context.Context) error {
	if s.searchService == nil {
		// Skip if service not available
		s.logger.Warn("Search service not available, skipping",
			ports.Field{Key: "saga_id", Value: s.ID})
		return nil
	}
	
	// Index the node for search
	doc := ports.SearchDocument{
		ID:       s.createdNodeID,
		UserID:   s.userID,
		Content:  s.content,
		Title:    s.title,
		Tags:     s.tags,
		Keywords: s.extractedKeywords,
		Type:     "node",
		UpdatedAt: time.Now(),
	}
	
	if err := s.searchService.Index(ctx, doc); err != nil {
		return fmt.Errorf("failed to update search index: %w", err)
	}
	
	s.Metadata["indexed"] = true
	
	s.logger.Info("Search index updated",
		ports.Field{Key: "saga_id", Value: s.ID},
		ports.Field{Key: "node_id", Value: s.createdNodeID})
	
	return nil
}

// compensateUpdateSearchIndex rolls back search index update
func (s *CreateNodeSaga) compensateUpdateSearchIndex(ctx context.Context) error {
	indexed, ok := s.Metadata["indexed"].(bool)
	if !ok || !indexed {
		return nil // Nothing to compensate
	}
	
	if s.searchService == nil {
		return nil
	}
	
	if err := s.searchService.Delete(ctx, s.createdNodeID); err != nil {
		s.logger.Warn("Failed to compensate search index update",
			ports.Field{Key: "node_id", Value: s.createdNodeID},
			ports.Field{Key: "error", Value: err.Error()})
	}
	
	s.logger.Info("Search index compensated",
		ports.Field{Key: "saga_id", Value: s.ID},
		ports.Field{Key: "node_id", Value: s.createdNodeID})
	
	return nil
}

// notifySubscribers notifies subscribers about the new node
func (s *CreateNodeSaga) notifySubscribers(ctx context.Context) error {
	// This would publish an event or send notifications
	// For now, just log
	s.logger.Info("Subscribers notified",
		ports.Field{Key: "saga_id", Value: s.ID},
		ports.Field{Key: "node_id", Value: s.createdNodeID})
	
	s.metrics.IncrementCounter("saga.create_node.notifications_sent",
		ports.Tag{Key: "user_id", Value: s.userID})
	
	return nil
}

// GetResult returns the result of the saga
func (s *CreateNodeSaga) GetResult() *CreateNodeResult {
	return &CreateNodeResult{
		NodeID:               s.createdNodeID,
		ExtractedKeywords:    s.extractedKeywords,
		SuggestedConnections: s.suggestedConnections,
		Success:              s.State == SagaStateCompleted,
		Error:                s.Error,
	}
}

// CreateNodeResult contains the result of the CreateNodeSaga
type CreateNodeResult struct {
	NodeID               string
	ExtractedKeywords    []string
	SuggestedConnections []string
	Success              bool
	Error                error
}

// CreateEdgeCommand is a placeholder for the edge creation command
type CreateEdgeCommand struct {
	cqrs.BaseCommand
	UserID   string
	SourceID string
	TargetID string
	Weight   float64
	Metadata map[string]interface{}
}

func (c *CreateEdgeCommand) GetCommandName() string {
	return "CreateEdge"
}

func (c *CreateEdgeCommand) Validate() error {
	if c.UserID == "" {
		return fmt.Errorf("user ID is required")
	}
	if c.SourceID == "" {
		return fmt.Errorf("source ID is required")
	}
	if c.TargetID == "" {
		return fmt.Errorf("target ID is required")
	}
	if c.Weight < 0 || c.Weight > 1 {
		return fmt.Errorf("weight must be between 0 and 1")
	}
	return nil
}

// DeleteEdgeCommand is a placeholder for the edge deletion command
type DeleteEdgeCommand struct {
	cqrs.BaseCommand
	UserID   string
	SourceID string
	TargetID string
}

func (c *DeleteEdgeCommand) GetCommandName() string {
	return "DeleteEdge"
}

func (c *DeleteEdgeCommand) Validate() error {
	if c.UserID == "" {
		return fmt.Errorf("user ID is required")
	}
	if c.SourceID == "" {
		return fmt.Errorf("source ID is required")
	}
	if c.TargetID == "" {
		return fmt.Errorf("target ID is required")
	}
	return nil
}