// Package services contains application services that orchestrate use cases.
// Application services implement the Application Layer in Clean Architecture.
//
// Key Concepts Illustrated:
//   - Application Service Pattern: Orchestrates business operations
//   - Command/Query Responsibility Segregation (CQRS): Separates reads from writes
//   - Transaction Management: Uses Unit of Work pattern for consistency
//   - Domain Event Publishing: Communicates changes to other parts of the system
//   - DTO Conversion: Transforms between domain objects and data transfer objects
//   - Error Handling: Wraps domain errors with application context
//
// This service is intentionally kept thin - it orchestrates but doesn't contain business logic.
// Business logic belongs in the domain layer (see domain.Node for examples).
package services

import (
	"context"
	"fmt"

	"brain2-backend/internal/application/adapters"
	"brain2-backend/internal/application/commands"
	"brain2-backend/internal/application/dto"
	"brain2-backend/internal/domain"
	domainServices "brain2-backend/internal/domain/services"
	"brain2-backend/internal/repository"
	appErrors "brain2-backend/pkg/errors"
)

// NodeService implements the Application Service pattern for node operations.
// It orchestrates use cases by coordinating between domain objects, repositories, and services.
type NodeService struct {
	// Dependencies are injected, not created (Dependency Inversion Principle)
	nodeAdapter      adapters.NodeRepositoryAdapter    // Adapter for node persistence
	edgeRepo         repository.EdgeRepository         // For edge persistence
	uow              adapters.UnitOfWorkAdapter        // Adapter for transaction management
	eventBus         domain.EventBus                   // For domain event publishing
	connectionAnalyzer *domainServices.ConnectionAnalyzer // Domain service for complex business logic
	idempotencyStore repository.IdempotencyStore       // For idempotent operations
}

// NewNodeService creates a new NodeService with all required dependencies.
func NewNodeService(
	nodeAdapter adapters.NodeRepositoryAdapter,
	edgeRepo repository.EdgeRepository,
	uow adapters.UnitOfWorkAdapter,
	eventBus domain.EventBus,
	connectionAnalyzer *domainServices.ConnectionAnalyzer,
	idempotencyStore repository.IdempotencyStore,
) *NodeService {
	return &NodeService{
		nodeAdapter:        nodeAdapter,
		edgeRepo:           edgeRepo,
		uow:                uow,
		eventBus:           eventBus,
		connectionAnalyzer: connectionAnalyzer,
		idempotencyStore:   idempotencyStore,
	}
}

// CreateNode implements the use case for creating a node with automatic connection discovery.
// This method demonstrates the complete application service pattern:
// 1. Start unit of work for transaction management
// 2. Convert application DTOs to domain objects
// 3. Apply business logic using domain objects and services
// 4. Persist changes through repositories
// 5. Publish domain events
// 6. Convert domain objects back to DTOs for response
func (s *NodeService) CreateNode(ctx context.Context, cmd *commands.CreateNodeCommand) (*dto.CreateNodeResult, error) {
	// 1. Start unit of work for transaction boundary
	if err := s.uow.Begin(ctx); err != nil {
		return nil, appErrors.Wrap(err, "failed to begin transaction")
	}
	defer s.uow.Rollback() // Rollback if not committed

	// 2. Handle idempotency if key is provided
	if cmd.IdempotencyKey != "" {
		if result, exists, err := s.checkIdempotency(ctx, cmd.IdempotencyKey, "CREATE_NODE", cmd.UserID); err != nil {
			return nil, err
		} else if exists {
			return result.(*dto.CreateNodeResult), nil
		}
	}

	// 3. Convert application command to domain objects (Application -> Domain boundary)
	userID, err := domain.ParseUserID(cmd.UserID)
	if err != nil {
		return nil, appErrors.NewValidation("invalid user id: " + err.Error())
	}

	content, err := domain.NewContent(cmd.Content)
	if err != nil {
		return nil, appErrors.NewValidation("invalid content: " + err.Error())
	}

	tags := domain.NewTags(cmd.Tags...)

	// 4. Create domain entity using factory method
	node, err := domain.NewNode(userID, content, tags)
	if err != nil {
		return nil, appErrors.Wrap(err, "failed to create node")
	}

	// 5. Find potential connections using domain service
	existingNodes, err := s.uow.Nodes().FindByUser(ctx, userID)
	if err != nil {
		return nil, appErrors.Wrap(err, "failed to find existing nodes for connection analysis")
	}

	// Use domain service to analyze connections (business logic stays in domain)
	potentialConnections, err := s.connectionAnalyzer.FindPotentialConnections(node, existingNodes)
	if err != nil {
		return nil, appErrors.Wrap(err, "failed to analyze potential connections")
	}

	// 6. Save the node first
	// Use CreateNodeAndKeywords instead of Save
	if err := s.uow.Nodes().Save(ctx, node); err != nil {
		return nil, appErrors.Wrap(err, "failed to save node")
	}

	// 7. Create edges for discovered connections
	var createdEdges []*domain.Edge
	for _, candidate := range potentialConnections {
		targetNode := candidate.Node
		
		// Use domain method to check if connection is allowed
		if err := node.CanConnectTo(targetNode); err != nil {
			continue // Skip invalid connections
		}

		// Use the similarity score from the candidate as the edge weight
		weight := candidate.SimilarityScore
		
		edge, err := domain.NewEdge(node.ID, targetNode.ID, userID, weight)
		if err != nil {
			continue // Skip if edge creation fails
		}

		if err := s.uow.Edges().Save(ctx, edge); err != nil {
			return nil, appErrors.Wrap(err, "failed to create edge")
		}

		createdEdges = append(createdEdges, edge)
	}

	// 8. Publish domain events before committing
	allEvents := node.GetUncommittedEvents()
	for _, edge := range createdEdges {
		allEvents = append(allEvents, edge.GetUncommittedEvents()...)
	}

	for _, event := range allEvents {
		if err := s.eventBus.Publish(ctx, event); err != nil {
			return nil, appErrors.Wrap(err, "failed to publish domain event")
		}
	}

	// Mark events as committed
	node.MarkEventsAsCommitted()
	for _, edge := range createdEdges {
		edge.MarkEventsAsCommitted()
	}

	// 9. Commit transaction
	if err := s.uow.Commit(); err != nil {
		return nil, appErrors.Wrap(err, "failed to commit transaction")
	}

	// 10. Convert domain objects to DTOs for response (Domain -> Application boundary)
	result := &dto.CreateNodeResult{
		Node:        dto.ToNodeView(node),
		Connections: dto.ToConnectionViews(createdEdges),
		Message:     fmt.Sprintf("Node created successfully with %d automatic connections", len(createdEdges)),
	}

	// 11. Store idempotency result if key was provided
	if cmd.IdempotencyKey != "" {
		s.storeIdempotencyResult(ctx, cmd.IdempotencyKey, "CREATE_NODE", cmd.UserID, result)
	}

	return result, nil
}

// UpdateNode implements the use case for updating an existing node.
func (s *NodeService) UpdateNode(ctx context.Context, cmd *commands.UpdateNodeCommand) (*dto.UpdateNodeResult, error) {
	if !cmd.HasChanges() {
		return nil, appErrors.NewValidation("no changes specified in update command")
	}

	// 1. Start unit of work
	if err := s.uow.Begin(ctx); err != nil {
		return nil, appErrors.Wrap(err, "failed to begin transaction")
	}
	defer s.uow.Rollback()

	// 2. No idempotency for updates (they are idempotent by nature)

	// 3. Parse domain identifiers
	userID, err := domain.ParseUserID(cmd.UserID)
	if err != nil {
		return nil, appErrors.NewValidation("invalid user id: " + err.Error())
	}

	nodeID, err := domain.ParseNodeID(cmd.NodeID)
	if err != nil {
		return nil, appErrors.NewValidation("invalid node id: " + err.Error())
	}

	// 4. Retrieve existing node
	// Use FindNodeByID with userID instead of FindByID
	node, err := s.uow.Nodes().GetByID(ctx, userID, nodeID)
	if err != nil {
		return nil, appErrors.Wrap(err, "failed to find node")
	}
	if node == nil {
		return nil, appErrors.NewNotFound("node not found")
	}

	// 5. Verify ownership
	if node == nil {
		return nil, appErrors.NewNotFound("node not found")
	}
	if !node.UserID.Equals(userID) {
		return nil, appErrors.NewUnauthorized("node belongs to different user")
	}

	// 6. Apply updates using domain methods
	if cmd.Content != "" {
		newContent, err := domain.NewContent(cmd.Content)
		if err != nil {
			return nil, appErrors.NewValidation("invalid content: " + err.Error())
		}

		if err := node.UpdateContent(newContent); err != nil {
			return nil, appErrors.Wrap(err, "failed to update node content")
		}
	}

	if len(cmd.Tags) > 0 {
		newTags := domain.NewTags(cmd.Tags...)
		if err := node.UpdateTags(newTags); err != nil {
			return nil, appErrors.Wrap(err, "failed to update node tags")
		}
	}

	// 7. Save updated node
	// Use CreateNodeAndKeywords which handles both create and update
	if err := s.uow.Nodes().Save(ctx, node); err != nil {
		return nil, appErrors.Wrap(err, "failed to save updated node")
	}

	// 8. Publish domain events
	for _, event := range node.GetUncommittedEvents() {
		if err := s.eventBus.Publish(ctx, event); err != nil {
			return nil, appErrors.Wrap(err, "failed to publish domain event")
		}
	}
	node.MarkEventsAsCommitted()

	// 9. Commit transaction
	if err := s.uow.Commit(); err != nil {
		return nil, appErrors.Wrap(err, "failed to commit transaction")
	}

	// 10. Convert to response DTO
	result := &dto.UpdateNodeResult{
		Node:    dto.ToNodeView(node),
		Message: "Node updated successfully",
	}

	// 11. No idempotency storage for updates

	return result, nil
}

// DeleteNode implements the use case for deleting a node.
func (s *NodeService) DeleteNode(ctx context.Context, cmd *commands.DeleteNodeCommand) (*dto.DeleteNodeResult, error) {
	// 1. Start unit of work
	if err := s.uow.Begin(ctx); err != nil {
		return nil, appErrors.Wrap(err, "failed to begin transaction")
	}
	defer s.uow.Rollback()

	// 2. Handle idempotency if key is provided
	// No idempotency for deletes (they are idempotent by nature)

	// 3. Parse domain identifiers
	userID, err := domain.ParseUserID(cmd.UserID)
	if err != nil {
		return nil, appErrors.NewValidation("invalid user id: " + err.Error())
	}

	nodeID, err := domain.ParseNodeID(cmd.NodeID)
	if err != nil {
		return nil, appErrors.NewValidation("invalid node id: " + err.Error())
	}

	// 4. Verify node exists and user owns it
	// Use FindNodeByID with userID
	node, err := s.uow.Nodes().GetByID(ctx, userID, nodeID)
	if err != nil {
		return nil, appErrors.Wrap(err, "failed to find node")
	}
	if node == nil {
		return nil, appErrors.NewNotFound("node not found")
	}

	if !node.UserID.Equals(userID) {
		return nil, appErrors.NewUnauthorized("node belongs to different user")
	}

	// 5. Delete associated edges first
	// For now, we'll skip edge deletion as it's not fully implemented
	// TODO: Implement proper edge deletion
	// if err := s.uow.Edges().DeleteByNodeID(ctx, nodeID); err != nil {
	//	return nil, appErrors.Wrap(err, "failed to delete node edges")
	// }

	// 6. Delete the node
	if err := s.uow.Nodes().Delete(ctx, userID, nodeID); err != nil {
		return nil, appErrors.Wrap(err, "failed to delete node")
	}

	// 7. Create and publish deletion event using proper constructor
	deletionEvent := domain.NewNodeDeletedEvent(
		nodeID, 
		userID, 
		node.Content, 
		node.Keywords(), 
		node.Tags, 
		domain.ParseVersion(node.Version),
	)

	if err := s.eventBus.Publish(ctx, deletionEvent); err != nil {
		return nil, appErrors.Wrap(err, "failed to publish deletion event")
	}

	// 8. Commit transaction
	if err := s.uow.Commit(); err != nil {
		return nil, appErrors.Wrap(err, "failed to commit transaction")
	}

	// 9. Convert to response DTO
	result := &dto.DeleteNodeResult{
		Success: true,
		Message: "Node deleted successfully",
	}

	// 10. No idempotency storage for deletes

	return result, nil
}

// BulkDeleteNodes implements the use case for deleting multiple nodes.
func (s *NodeService) BulkDeleteNodes(ctx context.Context, cmd *commands.BulkDeleteNodesCommand) (*dto.BulkDeleteResult, error) {
	// 1. Start unit of work
	if err := s.uow.Begin(ctx); err != nil {
		return nil, appErrors.Wrap(err, "failed to begin transaction")
	}
	defer s.uow.Rollback()

	// 2. No idempotency for bulk deletes

	// 3. Parse domain identifiers
	userID, err := domain.ParseUserID(cmd.UserID)
	if err != nil {
		return nil, appErrors.NewValidation("invalid user id: " + err.Error())
	}

	var nodeIDs []domain.NodeID
	var failedIDs []string
	deletedCount := 0

	// 4. Process each node deletion
	for _, nodeIDStr := range cmd.NodeIDs {
		nodeID, err := domain.ParseNodeID(nodeIDStr)
		if err != nil {
			failedIDs = append(failedIDs, nodeIDStr)
			continue
		}

		// Verify node exists and user owns it
		// Use FindNodeByID with userID
		node, err := s.uow.Nodes().GetByID(ctx, userID, nodeID)
		if err != nil || node == nil {
			failedIDs = append(failedIDs, nodeIDStr)
			continue
		}

		if !node.UserID.Equals(userID) {
			failedIDs = append(failedIDs, nodeIDStr)
			continue
		}

		nodeIDs = append(nodeIDs, nodeID)
	}

	// 5. Delete edges for all valid nodes
	// TODO: Implement proper edge deletion
	// for _, nodeID := range nodeIDs {
	//	if err := s.uow.Edges().DeleteByNodeID(ctx, nodeID); err != nil {
	//		failedIDs = append(failedIDs, nodeID.String())
	//		continue
	//	}
	// }

	// 6. Delete all valid nodes
	for _, nodeID := range nodeIDs {
		if err := s.uow.Nodes().Delete(ctx, userID, nodeID); err != nil {
			failedIDs = append(failedIDs, nodeID.String())
		} else {
			deletedCount++

			// Publish deletion event using proper constructor
			// Note: For bulk operations, we might want to use default values since we don't have individual node data
			emptyContent, _ := domain.NewContent(" ") // Create minimal valid content
			emptyKeywords := domain.NewKeywords([]string{})
			emptyTags := domain.NewTags()
			emptyVersion := domain.NewVersion()
			
			deletionEvent := domain.NewNodeDeletedEvent(
				nodeID, 
				userID, 
				emptyContent,
				emptyKeywords,
				emptyTags,
				emptyVersion,
			)
			s.eventBus.Publish(ctx, deletionEvent)
		}
	}

	// 7. Commit transaction
	if err := s.uow.Commit(); err != nil {
		return nil, appErrors.Wrap(err, "failed to commit transaction")
	}

	// 8. Convert to response DTO
	result := &dto.BulkDeleteResult{
		DeletedCount: deletedCount,
		FailedIDs:    failedIDs,
		Message:      fmt.Sprintf("Successfully deleted %d of %d nodes", deletedCount, len(cmd.NodeIDs)),
	}

	// 9. No idempotency storage for bulk deletes

	return result, nil
}

// Helper methods for idempotency handling

func (s *NodeService) checkIdempotency(ctx context.Context, key, operation, userID string) (interface{}, bool, error) {
	if s.idempotencyStore == nil {
		return nil, false, nil
	}

	idempotencyKey := repository.IdempotencyKey{
		UserID:    userID,
		Operation: operation,
		Hash:      key,
	}

	result, exists, err := s.idempotencyStore.Get(ctx, idempotencyKey)
	if err != nil {
		return nil, false, appErrors.Wrap(err, "failed to check idempotency")
	}

	return result, exists, nil
}

func (s *NodeService) storeIdempotencyResult(ctx context.Context, key, operation, userID string, result interface{}) {
	if s.idempotencyStore == nil {
		return
	}

	idempotencyKey := repository.IdempotencyKey{
		UserID:    userID,
		Operation: operation,
		Hash:      key,
	}

	s.idempotencyStore.Store(ctx, idempotencyKey, result)
}

// BulkCreateNodes implements the use case for creating multiple nodes in a single transaction.
func (s *NodeService) BulkCreateNodes(ctx context.Context, cmd *commands.BulkCreateNodesCommand) (*dto.BulkCreateResult, error) {
	// 1. Start unit of work
	if err := s.uow.Begin(ctx); err != nil {
		return nil, appErrors.Wrap(err, "failed to begin transaction")
	}
	defer s.uow.Rollback()

	// 2. No idempotency for bulk creates

	// 3. Parse user ID
	userID, err := domain.ParseUserID(cmd.UserID)
	if err != nil {
		return nil, appErrors.NewValidation("invalid user id: " + err.Error())
	}

	var createdNodes []*domain.Node
	var connections []*domain.Edge
	var errors []dto.BulkCreateError

	// 4. Create nodes sequentially with error handling
	for i, nodeReq := range cmd.Nodes {
		// Create domain node
		content, err := domain.NewContent(nodeReq.Content)
		if err != nil {
			errors = append(errors, dto.BulkCreateError{
				Index:   i,
				Content: nodeReq.Content,
				Error:   "invalid content: " + err.Error(),
			})
			continue
		}

		tags := domain.NewTags(nodeReq.Tags...)
		node, err := domain.NewNode(userID, content, tags)
		if err != nil {
			errors = append(errors, dto.BulkCreateError{
				Index:   i,
				Content: nodeReq.Content,
				Error:   "failed to create node: " + err.Error(),
			})
			continue
		}

		// Keywords are already generated during node creation

		// Save node through unit of work
		if err := s.uow.Nodes().Save(ctx, node); err != nil {
			errors = append(errors, dto.BulkCreateError{
				Index:   i,
				Content: nodeReq.Content,
				Error:   "failed to save node: " + err.Error(),
			})
			continue
		}

		createdNodes = append(createdNodes, node)
	}

	// 5. Create connections if we have multiple successful nodes
	if len(createdNodes) > 1 {
		// Analyze connections between all created nodes
		for i, sourceNode := range createdNodes {
			for j, targetNode := range createdNodes {
				if i >= j { // Avoid duplicates and self-connections
					continue
				}

				// Use connection analyzer to determine if nodes should be connected
				analysis, err := s.connectionAnalyzer.AnalyzeBidirectionalConnection(sourceNode, targetNode)
				if err == nil && analysis.ShouldConnect {
					// Create edge between nodes
					weight := analysis.ForwardConnection.RelevanceScore
					edge, err := domain.NewEdge(sourceNode.ID, targetNode.ID, userID, weight)
					if err != nil {
						// Log error but don't fail the entire operation
						continue
					}

					// Save edge through unit of work
					if err := s.uow.Edges().Save(ctx, edge); err != nil {
						// Log error but don't fail the entire operation
						continue
					}

					connections = append(connections, edge)
				}
			}
		}
	}

	// 6. Publish domain events for all created nodes
	for _, node := range createdNodes {
		for _, event := range node.GetUncommittedEvents() {
			if err := s.eventBus.Publish(ctx, event); err != nil {
				return nil, appErrors.Wrap(err, "failed to publish domain event")
			}
		}
		node.MarkEventsAsCommitted()
	}

	// 7. Commit transaction
	if err := s.uow.Commit(); err != nil {
		return nil, appErrors.Wrap(err, "failed to commit transaction")
	}

	// 8. Convert to response DTO
	result := &dto.BulkCreateResult{
		CreatedNodes:    dto.ToNodeViews(createdNodes),
		CreatedCount:    len(createdNodes),
		Connections:     dto.ToConnectionViews(connections),
		ConnectionCount: len(connections),
		Failed:          errors,
		Message:         fmt.Sprintf("Successfully created %d nodes", len(createdNodes)),
	}

	if len(errors) > 0 {
		result.Message = fmt.Sprintf("Created %d nodes with %d failures", len(createdNodes), len(errors))
	}

	if len(connections) > 0 {
		result.Message += fmt.Sprintf(" and %d connections", len(connections))
	}

	// 9. Store idempotency result if key was provided
	// No idempotency storage for bulk creates

	return result, nil
}