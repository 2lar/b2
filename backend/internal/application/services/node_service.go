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
// Business logic belongs in the domain layer (see node.Node for examples).
package services

import (
	"context"
	"fmt"
	"log"
	"time"

	"brain2-backend/internal/application/commands"
	"brain2-backend/internal/application/dto"
	"brain2-backend/internal/domain/node"
	"brain2-backend/internal/domain/edge"
	"brain2-backend/internal/domain/shared"
	domainServices "brain2-backend/internal/domain/services"
	"brain2-backend/internal/repository"
	appErrors "brain2-backend/pkg/errors"
)

// NodeService implements the Application Service pattern for node operations.
// It orchestrates use cases by coordinating between domain objects, repositories, and services.
type NodeService struct {
	// Dependencies are injected, not created (Dependency Inversion Principle)
	nodeRepo         repository.NodeRepository          // For node persistence
	edgeRepo         repository.EdgeRepository          // For edge persistence
	uowFactory       repository.UnitOfWorkFactory       // Factory for creating request-scoped UnitOfWork instances
	eventBus         shared.EventBus                    // For domain event publishing
	connectionAnalyzer *domainServices.ConnectionAnalyzer // Domain service for complex business logic
	idempotencyStore repository.IdempotencyStore        // For idempotent operations
}

// NewNodeService creates a new NodeService with all required dependencies.
func NewNodeService(
	nodeRepo repository.NodeRepository,
	edgeRepo repository.EdgeRepository,
	uowFactory repository.UnitOfWorkFactory,
	eventBus shared.EventBus,
	connectionAnalyzer *domainServices.ConnectionAnalyzer,
	idempotencyStore repository.IdempotencyStore,
) *NodeService {
	return &NodeService{
		nodeRepo:           nodeRepo,
		edgeRepo:           edgeRepo,
		uowFactory:         uowFactory,
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
	// 1. Create a new UnitOfWork instance for this request
	uow, err := s.uowFactory.Create(ctx)
	if err != nil {
		return nil, appErrors.Wrap(err, "failed to create unit of work")
	}
	
	// Start unit of work for transaction boundary
	if err := uow.Begin(ctx); err != nil {
		return nil, appErrors.Wrap(err, "failed to begin transaction")
	}
	
	// Track whether commit was called successfully
	var commitCalled bool
	
	// Ensure proper cleanup even on panic
	defer func() {
		if r := recover(); r != nil {
			// Attempt rollback on panic
			if rollbackErr := uow.Rollback(); rollbackErr != nil {
				// Log error but continue with panic
				// TODO: Add proper logging
			}
			// Re-panic to let it bubble up
			panic(r)
		} else if !commitCalled {
			// Only rollback if commit wasn't called
			uow.Rollback()
		}
	}()

	// 2. Handle idempotency if key is provided
	if cmd.IdempotencyKey != "" {
		if result, exists, err := s.checkIdempotency(ctx, cmd.IdempotencyKey, "CREATE_NODE", cmd.UserID); err != nil {
			return nil, err
		} else if exists {
			// Handle type assertion safely - the stored result might be a map after JSON serialization
			switch v := result.(type) {
			case *dto.CreateNodeResult:
				// Direct type match - ideal case
				return v, nil
			case map[string]interface{}:
				// JSON deserialized as map - need to reconstruct
				// This happens when Lambda reuses warm containers and the in-memory
				// idempotency store returns objects that have been through JSON marshaling
				reconstructed, err := s.reconstructCreateNodeResult(v)
				if err != nil {
					// If reconstruction fails, proceed with new creation rather than failing
					// This ensures the API remains available even with cache issues
					log.Printf("WARN: Failed to reconstruct cached result for idempotency key, creating new node: %v", err)
				} else if reconstructed != nil && reconstructed.Node != nil {
					return reconstructed, nil
				} else {
					log.Printf("WARN: Reconstructed result is invalid (nil or nil Node), creating new node")
				}
			default:
				// Unexpected type from idempotency store - proceed with new creation
				// This could happen if the store implementation changes or has issues
				// In production, this should be logged as a warning
			}
		}
	}

	// 3. Convert application command to domain objects (Application -> Domain boundary)
	userID, err := shared.ParseUserID(cmd.UserID)
	if err != nil {
		return nil, appErrors.NewValidation("invalid user id: " + err.Error())
	}

	content, err := shared.NewContent(cmd.Content)
	if err != nil {
		return nil, appErrors.NewValidation("invalid content: " + err.Error())
	}

	tags := shared.NewTags(cmd.Tags...)

	// 4. Create domain entity using factory method
	node, err := node.NewNode(userID, content, tags)
	if err != nil {
		return nil, appErrors.Wrap(err, "failed to create node")
	}

	// 5. Find potential connections using domain service
	query := repository.NodeQuery{
		UserID: userID.String(),
	}
	existingNodes, err := uow.Nodes().FindNodes(ctx, query)
	if err != nil {
		return nil, appErrors.Wrap(err, "failed to find existing nodes for connection analysis")
	}

	// Use domain service to analyze connections (business logic stays in domain)
	potentialConnections, err := s.connectionAnalyzer.FindPotentialConnections(node, existingNodes)
	if err != nil {
		return nil, appErrors.Wrap(err, "failed to analyze potential connections")
	}

	// 6. Save the node first
	// Use CreateNodeAndKeywords for node creation
	if err := uow.Nodes().CreateNodeAndKeywords(ctx, node); err != nil {
		return nil, appErrors.Wrap(err, "failed to save node")
	}

	// 7. Create edges for discovered connections
	var createdEdges []*edge.Edge
	for _, candidate := range potentialConnections {
		targetNode := candidate.Node
		
		// Use domain method to check if connection is allowed
		if err := node.CanConnectTo(targetNode); err != nil {
			continue // Skip invalid connections
		}

		// Use the similarity score from the candidate as the edge weight
		weight := candidate.SimilarityScore
		
		edge, err := edge.NewEdge(node.ID, targetNode.ID, userID, weight)
		if err != nil {
			continue // Skip if edge creation fails
		}

		if err := uow.Edges().CreateEdge(ctx, edge); err != nil {
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
	commitCalled = true
	if err := uow.Commit(); err != nil {
		commitCalled = false // Reset if commit fails
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

	// 1. Create a new UnitOfWork instance for this request
	uow, err := s.uowFactory.Create(ctx)
	if err != nil {
		return nil, appErrors.Wrap(err, "failed to create unit of work")
	}
	
	// Start unit of work
	if err := uow.Begin(ctx); err != nil {
		return nil, appErrors.Wrap(err, "failed to begin transaction")
	}
	
	// Track whether commit was called successfully
	var commitCalled bool
	
	// Ensure proper cleanup even on panic
	defer func() {
		if r := recover(); r != nil {
			// Attempt rollback on panic
			if rollbackErr := uow.Rollback(); rollbackErr != nil {
				// Log error but continue with panic
				// TODO: Add proper logging
			}
			// Re-panic to let it bubble up
			panic(r)
		} else if !commitCalled {
			// Only rollback if commit wasn't called
			uow.Rollback()
		}
	}()

	// 2. No idempotency for updates (they are idempotent by nature)

	// 3. Parse domain identifiers
	userID, err := shared.ParseUserID(cmd.UserID)
	if err != nil {
		return nil, appErrors.NewValidation("invalid user id: " + err.Error())
	}

	nodeID, err := shared.ParseNodeID(cmd.NodeID)
	if err != nil {
		return nil, appErrors.NewValidation("invalid node id: " + err.Error())
	}

	// 4. Retrieve existing node
	// Use FindNodeByID with userID
	node, err := uow.Nodes().FindNodeByID(ctx, userID.String(), nodeID.String())
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
		newContent, err := shared.NewContent(cmd.Content)
		if err != nil {
			return nil, appErrors.NewValidation("invalid content: " + err.Error())
		}

		if err := node.UpdateContent(newContent); err != nil {
			return nil, appErrors.Wrap(err, "failed to update node content")
		}
	}

	if len(cmd.Tags) > 0 {
		newTags := shared.NewTags(cmd.Tags...)
		if err := node.UpdateTags(newTags); err != nil {
			return nil, appErrors.Wrap(err, "failed to update node tags")
		}
	}

	// 7. Save updated node
	// Use CreateNodeAndKeywords which handles both create and update
	if err := uow.Nodes().CreateNodeAndKeywords(ctx, node); err != nil {
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
	commitCalled = true
	if err := uow.Commit(); err != nil {
		commitCalled = false // Reset if commit fails
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
	// 1. Create a new UnitOfWork instance for this request
	uow, err := s.uowFactory.Create(ctx)
	if err != nil {
		return nil, appErrors.Wrap(err, "failed to create unit of work")
	}
	
	// Start unit of work
	if err := uow.Begin(ctx); err != nil {
		return nil, appErrors.Wrap(err, "failed to begin transaction")
	}
	
	// Track whether commit was called successfully
	var commitCalled bool
	
	// Ensure proper cleanup even on panic
	defer func() {
		if r := recover(); r != nil {
			// Attempt rollback on panic
			if rollbackErr := uow.Rollback(); rollbackErr != nil {
				// Log error but continue with panic
				// TODO: Add proper logging
			}
			// Re-panic to let it bubble up
			panic(r)
		} else if !commitCalled {
			// Only rollback if commit wasn't called
			uow.Rollback()
		}
	}()

	// 2. Handle idempotency if key is provided
	// No idempotency for deletes (they are idempotent by nature)

	// 3. Parse domain identifiers
	userID, err := shared.ParseUserID(cmd.UserID)
	if err != nil {
		return nil, appErrors.NewValidation("invalid user id: " + err.Error())
	}

	nodeID, err := shared.ParseNodeID(cmd.NodeID)
	if err != nil {
		return nil, appErrors.NewValidation("invalid node id: " + err.Error())
	}

	// 4. Verify node exists and user owns it
	// Use FindNodeByID with userID
	node, err := uow.Nodes().FindNodeByID(ctx, userID.String(), nodeID.String())
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
	// if err := uow.Edges().DeleteByNodeID(ctx, nodeID); err != nil {
	//	return nil, appErrors.Wrap(err, "failed to delete node edges")
	// }

	// 6. Delete the node
	if err := uow.Nodes().DeleteNode(ctx, userID.String(), nodeID.String()); err != nil {
		return nil, appErrors.Wrap(err, "failed to delete node")
	}

	// 7. Create and publish deletion event using proper constructor
	deletionEvent := shared.NewNodeDeletedEvent(
		nodeID, 
		userID, 
		node.Content, 
		node.Keywords(), 
		node.Tags, 
		shared.ParseVersion(node.Version),
	)

	log.Printf("DEBUG: NodeService.DeleteNode - Publishing NodeDeleted event for node %s, user %s", nodeID.String(), userID.String())
	
	if err := s.eventBus.Publish(ctx, deletionEvent); err != nil {
		log.Printf("ERROR: NodeService.DeleteNode - Failed to publish NodeDeleted event: %v", err)
		return nil, appErrors.Wrap(err, "failed to publish deletion event")
	}
	
	log.Printf("DEBUG: NodeService.DeleteNode - Successfully published NodeDeleted event for node %s", nodeID.String())

	// 8. Commit transaction
	commitCalled = true
	if err := uow.Commit(); err != nil {
		commitCalled = false // Reset if commit fails
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

// BulkDeleteNodes implements the use case for deleting multiple nodes using optimized batch operations.
func (s *NodeService) BulkDeleteNodes(ctx context.Context, cmd *commands.BulkDeleteNodesCommand) (*dto.BulkDeleteResult, error) {
	// 1. Create a new UnitOfWork instance for this request
	uow, err := s.uowFactory.Create(ctx)
	if err != nil {
		return nil, appErrors.Wrap(err, "failed to create unit of work")
	}
	
	// Start unit of work
	if err := uow.Begin(ctx); err != nil {
		return nil, appErrors.Wrap(err, "failed to begin transaction")
	}
	
	// Track whether commit was called successfully
	var commitCalled bool
	
	// Ensure proper cleanup even on panic
	defer func() {
		if r := recover(); r != nil {
			// Attempt rollback on panic
			if rollbackErr := uow.Rollback(); rollbackErr != nil {
				// Log error but continue with panic
				log.Printf("ERROR: Failed to rollback after panic: %v", rollbackErr)
			}
			// Re-panic to let it bubble up
			panic(r)
		} else if !commitCalled {
			// Only rollback if commit wasn't called
			uow.Rollback()
		}
	}()

	// 2. Parse domain identifiers
	userID, err := shared.ParseUserID(cmd.UserID)
	if err != nil {
		return nil, appErrors.NewValidation("invalid user id: " + err.Error())
	}

	// 3. Validate node IDs and check ownership
	validNodeIDs := make([]string, 0, len(cmd.NodeIDs))
	failedIDs := make([]string, 0)
	nodeDataMap := make(map[string]*node.Node) // Store node data for event publishing

	log.Printf("DEBUG: BulkDeleteNodes - Validating %d node IDs for user %s", len(cmd.NodeIDs), userID.String())

	for _, nodeIDStr := range cmd.NodeIDs {
		// Validate node ID format
		nodeID, err := shared.ParseNodeID(nodeIDStr)
		if err != nil {
			log.Printf("DEBUG: Invalid node ID format: %s", nodeIDStr)
			failedIDs = append(failedIDs, nodeIDStr)
			continue
		}

		// Verify node exists and user owns it
		node, err := uow.Nodes().FindNodeByID(ctx, userID.String(), nodeID.String())
		if err != nil || node == nil {
			log.Printf("DEBUG: Node not found or error: %s, err: %v", nodeIDStr, err)
			failedIDs = append(failedIDs, nodeIDStr)
			continue
		}

		if !node.UserID.Equals(userID) {
			log.Printf("DEBUG: Node ownership mismatch for node: %s", nodeIDStr)
			failedIDs = append(failedIDs, nodeIDStr)
			continue
		}

		validNodeIDs = append(validNodeIDs, nodeIDStr)
		nodeDataMap[nodeIDStr] = node // Store for event publishing
	}

	log.Printf("DEBUG: BulkDeleteNodes - Validated nodes: %d valid, %d failed", len(validNodeIDs), len(failedIDs))

	// 4. Use optimized batch delete for valid nodes
	var deletedIDs []string
	var batchFailedIDs []string
	
	if len(validNodeIDs) > 0 {
		// Use the new BatchDeleteNodes method for optimized deletion
		deletedIDs, batchFailedIDs, err = uow.Nodes().BatchDeleteNodes(ctx, userID.String(), validNodeIDs)
		if err != nil {
			// Even on error, we may have partial success
			log.Printf("ERROR: Batch delete encountered error: %v", err)
		}
		
		// Add batch failures to the failed list
		failedIDs = append(failedIDs, batchFailedIDs...)
		
		log.Printf("DEBUG: BulkDeleteNodes - Batch delete results: %d deleted, %d failed", len(deletedIDs), len(batchFailedIDs))
	}

	// 5. Publish deletion events for successfully deleted nodes
	for _, nodeIDStr := range deletedIDs {
		nodeID, _ := shared.ParseNodeID(nodeIDStr) // Already validated
		
		// Use actual node data if available, otherwise use defaults
		var deletionEvent shared.DomainEvent
		if nodeData, exists := nodeDataMap[nodeIDStr]; exists {
			deletionEvent = shared.NewNodeDeletedEvent(
				nodeID,
				userID,
				nodeData.Content,
				nodeData.Keywords(),
				nodeData.Tags,
				shared.ParseVersion(nodeData.Version),
			)
		} else {
			// Fallback to minimal event data
			emptyContent, _ := shared.NewContent(" ")
			emptyKeywords := shared.NewKeywords([]string{})
			emptyTags := shared.NewTags()
			emptyVersion := shared.NewVersion()
			
			deletionEvent = shared.NewNodeDeletedEvent(
				nodeID,
				userID,
				emptyContent,
				emptyKeywords,
				emptyTags,
				emptyVersion,
			)
		}
		
		// Publish event asynchronously - don't block the bulk operation
		if err := s.eventBus.Publish(ctx, deletionEvent); err != nil {
			log.Printf("WARN: Failed to publish NodeDeleted event for node %s: %v", nodeIDStr, err)
			// Don't fail the operation for event publishing failures
		}
	}

	// 6. Commit transaction
	commitCalled = true
	if err := uow.Commit(); err != nil {
		commitCalled = false // Reset if commit fails
		return nil, appErrors.Wrap(err, "failed to commit transaction")
	}

	// 7. Convert to response DTO
	result := &dto.BulkDeleteResult{
		DeletedCount: len(deletedIDs),
		FailedIDs:    failedIDs,
		Message:      fmt.Sprintf("Successfully deleted %d of %d nodes", len(deletedIDs), len(cmd.NodeIDs)),
	}

	log.Printf("INFO: BulkDeleteNodes completed - deleted: %d, failed: %d, total: %d", 
		len(deletedIDs), len(failedIDs), len(cmd.NodeIDs))

	return result, nil
}

// Helper methods for idempotency handling

// reconstructCreateNodeResult reconstructs a CreateNodeResult from a map[string]interface{}
// This is needed when the idempotency store returns a JSON-deserialized object
func (s *NodeService) reconstructCreateNodeResult(data map[string]interface{}) (*dto.CreateNodeResult, error) {
	result := &dto.CreateNodeResult{}
	
	// Reconstruct the node view
	if nodeData, ok := data["Node"].(map[string]interface{}); ok {
		result.Node = s.reconstructNodeView(nodeData)
	}
	
	// Validate that Node was reconstructed successfully
	if result.Node == nil {
		return nil, fmt.Errorf("failed to reconstruct node from cached data")
	}
	
	// Reconstruct connections
	if connections, ok := data["Connections"].([]interface{}); ok {
		result.Connections = make([]*dto.ConnectionView, 0, len(connections))
		for _, conn := range connections {
			if connMap, ok := conn.(map[string]interface{}); ok {
				result.Connections = append(result.Connections, s.reconstructConnectionView(connMap))
			}
		}
	}
	
	// Reconstruct message
	if msg, ok := data["Message"].(string); ok {
		result.Message = msg
	}
	
	return result, nil
}

// reconstructNodeView reconstructs a NodeView from a map
func (s *NodeService) reconstructNodeView(data map[string]interface{}) *dto.NodeView {
	view := &dto.NodeView{}
	
	if id, ok := data["ID"].(string); ok {
		view.ID = id
	}
	if userID, ok := data["UserID"].(string); ok {
		view.UserID = userID
	}
	if content, ok := data["Content"].(string); ok {
		view.Content = content
	}
	if keywords, ok := data["Keywords"].([]interface{}); ok {
		view.Keywords = make([]string, 0, len(keywords))
		for _, k := range keywords {
			if str, ok := k.(string); ok {
				view.Keywords = append(view.Keywords, str)
			}
		}
	}
	if tags, ok := data["Tags"].([]interface{}); ok {
		view.Tags = make([]string, 0, len(tags))
		for _, t := range tags {
			if str, ok := t.(string); ok {
				view.Tags = append(view.Tags, str)
			}
		}
	}
	if version, ok := data["Version"].(float64); ok {
		view.Version = int(version)
	}
	
	// Parse timestamps - they might be strings in JSON
	if createdAt, ok := data["CreatedAt"].(string); ok {
		if t, err := time.Parse(time.RFC3339, createdAt); err == nil {
			view.CreatedAt = t
		}
	}
	if updatedAt, ok := data["UpdatedAt"].(string); ok {
		if t, err := time.Parse(time.RFC3339, updatedAt); err == nil {
			view.UpdatedAt = t
		}
	}
	
	return view
}

// reconstructConnectionView reconstructs a ConnectionView from a map
func (s *NodeService) reconstructConnectionView(data map[string]interface{}) *dto.ConnectionView {
	view := &dto.ConnectionView{}
	
	// Map the correct field names from ConnectionView struct
	if id, ok := data["id"].(string); ok {
		view.ID = id
	}
	if sourceNodeID, ok := data["source_node_id"].(string); ok {
		view.SourceNodeID = sourceNodeID
	}
	if targetNodeID, ok := data["target_node_id"].(string); ok {
		view.TargetNodeID = targetNodeID
	}
	if strength, ok := data["strength"].(float64); ok {
		view.Strength = strength
	}
	if createdAt, ok := data["created_at"].(string); ok {
		if t, err := time.Parse(time.RFC3339, createdAt); err == nil {
			view.CreatedAt = t
		}
	}
	
	return view
}

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
	// 1. Create a new UnitOfWork instance for this request
	uow, err := s.uowFactory.Create(ctx)
	if err != nil {
		return nil, appErrors.Wrap(err, "failed to create unit of work")
	}
	
	// Start unit of work
	if err := uow.Begin(ctx); err != nil {
		return nil, appErrors.Wrap(err, "failed to begin transaction")
	}
	
	// Track whether commit was called successfully
	var commitCalled bool
	
	// Ensure proper cleanup even on panic
	defer func() {
		if r := recover(); r != nil {
			// Attempt rollback on panic
			if rollbackErr := uow.Rollback(); rollbackErr != nil {
				// Log error but continue with panic
				// TODO: Add proper logging
			}
			// Re-panic to let it bubble up
			panic(r)
		} else if !commitCalled {
			// Only rollback if commit wasn't called
			uow.Rollback()
		}
	}()

	// 2. No idempotency for bulk creates

	// 3. Parse user ID
	userID, err := shared.ParseUserID(cmd.UserID)
	if err != nil {
		return nil, appErrors.NewValidation("invalid user id: " + err.Error())
	}

	var createdNodes []*node.Node
	var connections []*edge.Edge
	var errors []dto.BulkCreateError

	// 4. Create nodes sequentially with error handling
	for i, nodeReq := range cmd.Nodes {
		// Create domain node
		content, err := shared.NewContent(nodeReq.Content)
		if err != nil {
			errors = append(errors, dto.BulkCreateError{
				Index:   i,
				Content: nodeReq.Content,
				Error:   "invalid content: " + err.Error(),
			})
			continue
		}

		tags := shared.NewTags(nodeReq.Tags...)
		node, err := node.NewNode(userID, content, tags)
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
		if err := uow.Nodes().CreateNodeAndKeywords(ctx, node); err != nil {
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
					edge, err := edge.NewEdge(sourceNode.ID, targetNode.ID, userID, weight)
					if err != nil {
						// Log error but don't fail the entire operation
						continue
					}

					// Save edge through unit of work
					if err := uow.Edges().CreateEdge(ctx, edge); err != nil {
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
	commitCalled = true
	if err := uow.Commit(); err != nil {
		commitCalled = false // Reset if commit fails
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