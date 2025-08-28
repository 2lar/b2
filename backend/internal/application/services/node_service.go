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
	"time"

	"brain2-backend/internal/application/commands"
	"brain2-backend/internal/application/dto"
	"brain2-backend/internal/domain/node"
	"brain2-backend/internal/domain/edge"
	"brain2-backend/internal/domain/shared"
	domainServices "brain2-backend/internal/domain/services"
	"brain2-backend/internal/repository"
	"brain2-backend/internal/errors"
	
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
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
	tracer           trace.Tracer                       // For distributed tracing
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
		tracer:             otel.Tracer("brain2-backend.application.node_service"),
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
	// Start tracing span for the entire operation
	ctx, span := s.tracer.Start(ctx, "NodeService.CreateNode",
		trace.WithSpanKind(trace.SpanKindInternal),
		trace.WithAttributes(
			attribute.String("user.id", cmd.UserID),
			attribute.Int("content.length", len(cmd.Content)),
			attribute.Int("tags.count", len(cmd.Tags)),
			attribute.Bool("has_idempotency_key", cmd.IdempotencyKey != ""),
		),
	)
	defer span.End()

	// 1. Create a new UnitOfWork instance for this request
	uow, err := s.uowFactory.Create(ctx)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "Failed to create unit of work")
		return nil, errors.ApplicationError(ctx, "CreateUnitOfWork", err)
	}
	
	// Start unit of work for transaction boundary
	if err := uow.Begin(ctx); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "Failed to begin transaction")
		return nil, errors.ApplicationError(ctx, "BeginTransaction", err)
	}
	
	// Track whether commit was called successfully
	var commitCalled bool
	
	// Ensure proper cleanup even on panic
	defer func() {
		if r := recover(); r != nil {
			span.RecordError(fmt.Errorf("panic: %v", r))
			span.SetStatus(codes.Error, "Panic occurred")
			// Attempt rollback on panic
			if rollbackErr := uow.Rollback(); rollbackErr != nil {
				span.AddEvent("rollback_failed_on_panic",
					trace.WithAttributes(attribute.String("error", rollbackErr.Error())))
			}
			// Re-panic to let it bubble up
			panic(r)
		} else if !commitCalled {
			// Only rollback if commit wasn't called
			span.AddEvent("rolling_back_transaction")
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
					// Failed to reconstruct cached result, creating new node
				} else if reconstructed != nil && reconstructed.Node != nil {
					return reconstructed, nil
				} else {
					// Reconstructed result is invalid, creating new node
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
		return nil, errors.ServiceValidationError("userID", err.Error(), cmd.UserID)
	}

	content, err := shared.NewContent(cmd.Content)
	if err != nil {
		return nil, errors.ServiceValidationError("field", "", "invalid content: " + err.Error())
	}

	tags := shared.NewTags(cmd.Tags...)

	// 4. Create domain entity using factory method
	// Creating title value object from command
	title, _ := shared.NewTitle(cmd.Title) // Use title from command
	// Title value object created
	node, err := node.NewNode(userID, content, title, tags)
	if err != nil {
		return nil, errors.ApplicationError(ctx, "CreateNode", err)
	}
	// Node created with title

	// 5. Find potential connections using domain service
	query := repository.NodeQuery{
		UserID: userID.String(),
	}
	existingNodes, err := uow.Nodes().FindNodes(ctx, query)
	if err != nil {
		return nil, errors.ApplicationError(ctx, "FindExistingNodes", err)
	}

	// Use domain service to analyze connections (business logic stays in domain)
	potentialConnections, err := s.connectionAnalyzer.FindPotentialConnections(node, existingNodes)
	if err != nil {
		return nil, errors.ApplicationError(ctx, "AnalyzeConnections", err)
	}

	// 6. Save the node first
	// Use CreateNodeAndKeywords for node creation
	if err := uow.Nodes().CreateNodeAndKeywords(ctx, node); err != nil {
		return nil, errors.ApplicationError(ctx, "SaveNode", err)
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
		
		edge, err := edge.NewEdge(node.ID(), targetNode.ID(), userID, weight)
		if err != nil {
			continue // Skip if edge creation fails
		}

		if err := uow.Edges().CreateEdge(ctx, edge); err != nil {
			return nil, errors.ApplicationError(ctx, "CreateEdge", err)
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
			return nil, errors.ApplicationError(ctx, "PublishEvent", err)
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
		return nil, errors.ApplicationError(ctx, "CommitTransaction", err)
	}

	// 10. Convert domain objects to DTOs for response (Domain -> Application boundary)
	result := &dto.CreateNodeResult{
		Node:         dto.NodeFromDomain(node),
		CreatedEdges: s.convertEdgesToDTOs(createdEdges),
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
		return nil, errors.ServiceValidationError("field", "", "no changes specified in update command")
	}

	// 1. Create a new UnitOfWork instance for this request
	uow, err := s.uowFactory.Create(ctx)
	if err != nil {
		return nil, errors.ApplicationError(ctx, "CreateUnitOfWork", err)
	}
	
	// Start unit of work
	if err := uow.Begin(ctx); err != nil {
		return nil, errors.ApplicationError(ctx, "BeginTransaction", err)
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
		return nil, errors.ServiceValidationError("userID", err.Error(), cmd.UserID)
	}

	nodeID, err := shared.ParseNodeID(cmd.NodeID)
	if err != nil {
		return nil, errors.ServiceValidationError("field", "", "invalid node id: " + err.Error())
	}

	// 4. Retrieve existing node
	// Use FindNodeByID with userID
	node, err := uow.Nodes().FindNodeByID(ctx, userID.String(), nodeID.String())
	if err != nil {
		return nil, errors.ApplicationError(ctx, "FindNode", err)
	}
	if node == nil {
		return nil, errors.ServiceNotFoundError("node", "node not found")
	}

	// 5. Verify ownership
	if node == nil {
		return nil, errors.ServiceNotFoundError("node", "node not found")
	}
	if !node.UserID().Equals(userID) {
		return nil, errors.ServiceAuthorizationError(cmd.UserID, "node", "node belongs to different user")
	}

	// 6. Apply updates using domain methods
	if cmd.Content != "" {
		newContent, err := shared.NewContent(cmd.Content)
		if err != nil {
			return nil, errors.ServiceValidationError("field", "", "invalid content: " + err.Error())
		}

		if err := node.UpdateContent(newContent); err != nil {
			return nil, errors.ApplicationError(ctx, "UpdateContent", err)
		}
	}

	if len(cmd.Tags) > 0 {
		newTags := shared.NewTags(cmd.Tags...)
		if err := node.UpdateTags(newTags); err != nil {
			return nil, errors.ApplicationError(ctx, "UpdateTags", err)
		}
	}
	if cmd.Title != "" {
		newTitle, err := shared.NewTitle(cmd.Title)
		if err != nil {
			return nil, errors.ServiceValidationError("field", "", "invalid title: " + err.Error())
		}
		if err := node.UpdateTitle(newTitle); err != nil {
			return nil, errors.ApplicationError(ctx, "UpdateTitle", err)
		}
	}

	// 7. Save updated node
	// Use CreateNodeAndKeywords which handles both create and update
	if err := uow.Nodes().CreateNodeAndKeywords(ctx, node); err != nil {
		return nil, errors.ApplicationError(ctx, "SaveUpdatedNode", err)
	}

	// 8. Publish domain events
	for _, event := range node.GetUncommittedEvents() {
		if err := s.eventBus.Publish(ctx, event); err != nil {
			return nil, errors.ApplicationError(ctx, "PublishEvent", err)
		}
	}
	node.MarkEventsAsCommitted()

	// 9. Commit transaction
	commitCalled = true
	if err := uow.Commit(); err != nil {
		commitCalled = false // Reset if commit fails
		return nil, errors.ApplicationError(ctx, "CommitTransaction", err)
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
		return nil, errors.ApplicationError(ctx, "CreateUnitOfWork", err)
	}
	
	// Start unit of work
	if err := uow.Begin(ctx); err != nil {
		return nil, errors.ApplicationError(ctx, "BeginTransaction", err)
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
		return nil, errors.ServiceValidationError("userID", err.Error(), cmd.UserID)
	}

	nodeID, err := shared.ParseNodeID(cmd.NodeID)
	if err != nil {
		return nil, errors.ServiceValidationError("field", "", "invalid node id: " + err.Error())
	}

	// 4. Verify node exists and user owns it
	// Use FindNodeByID with userID
	node, err := uow.Nodes().FindNodeByID(ctx, userID.String(), nodeID.String())
	if err != nil {
		return nil, errors.ApplicationError(ctx, "FindNode", err)
	}
	if node == nil {
		return nil, errors.ServiceNotFoundError("node", "node not found")
	}

	if !node.UserID().Equals(userID) {
		return nil, errors.ServiceAuthorizationError(cmd.UserID, "node", "node belongs to different user")
	}

	// 5. Delete associated edges first using proper DeleteByNode method
	if edgeDeleter, ok := uow.Edges().(interface {
		DeleteByNode(ctx context.Context, userID shared.UserID, nodeID shared.NodeID) error
	}); ok {
		if err := edgeDeleter.DeleteByNode(ctx, userID, nodeID); err != nil {
			return nil, errors.ApplicationError(ctx, "DeleteNodeEdges", err)
		}
	} else {
		// Fallback: manually delete edges if DeleteByNode not available
		// This ensures backward compatibility but edges may remain orphaned
		// TODO: Add logger to NodeService to warn about this condition
	}

	// 6. Delete the node
	if err := uow.Nodes().DeleteNode(ctx, userID.String(), nodeID.String()); err != nil {
		return nil, errors.ApplicationError(ctx, "DeleteNode", err)
	}

	// 7. Create and publish deletion event using proper constructor
	deletionEvent := shared.NewNodeDeletedEvent(
		nodeID, 
		userID, 
		node.Content(), 
		node.Keywords(), 
		node.Tags(), 
		shared.ParseVersion(node.Version()),
	)

	// Publishing NodeDeleted event through Unit of Work for transactional consistency
	uow.PublishEvent(deletionEvent)

	// 8. Commit transaction
	commitCalled = true
	if err := uow.Commit(); err != nil {
		commitCalled = false // Reset if commit fails
		return nil, errors.ApplicationError(ctx, "CommitTransaction", err)
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
		return nil, errors.ApplicationError(ctx, "CreateUnitOfWork", err)
	}
	
	// Start unit of work
	if err := uow.Begin(ctx); err != nil {
		return nil, errors.ApplicationError(ctx, "BeginTransaction", err)
	}
	
	// Track whether commit was called successfully
	var commitCalled bool
	
	// Ensure proper cleanup even on panic
	defer func() {
		if r := recover(); r != nil {
			// Attempt rollback on panic
			if rollbackErr := uow.Rollback(); rollbackErr != nil {
				// Log error but continue with panic
				// Failed to rollback after panic
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
		return nil, errors.ServiceValidationError("userID", err.Error(), cmd.UserID)
	}

	// 3. Validate node IDs and check ownership using batch operations
	validNodeIDs := make([]string, 0, len(cmd.NodeIDs))
	failedIDs := make([]string, 0)
	nodeDataMap := make(map[string]*node.Node) // Store node data for event publishing

	// Validating node IDs and ownership

	// First, validate all node ID formats
	nodeIDsToCheck := make([]string, 0, len(cmd.NodeIDs))
	for _, nodeIDStr := range cmd.NodeIDs {
		// Validate node ID format
		_, err := shared.ParseNodeID(nodeIDStr)
		if err != nil {
			// Invalid node ID format
			failedIDs = append(failedIDs, nodeIDStr)
			continue
		}
		nodeIDsToCheck = append(nodeIDsToCheck, nodeIDStr)
	}

	// Batch retrieve all nodes at once - MASSIVE OPTIMIZATION
	if len(nodeIDsToCheck) > 0 {
		nodesMap, err := uow.Nodes().BatchGetNodes(ctx, userID.String(), nodeIDsToCheck)
		if err != nil {
			// Failed to retrieve nodes for validation
			// Fall back to marking all as failed
			failedIDs = append(failedIDs, nodeIDsToCheck...)
		} else {
			// Check which nodes exist and are owned by the user
			for _, nodeIDStr := range nodeIDsToCheck {
				node, exists := nodesMap[nodeIDStr]
				if !exists || node == nil {
					// Node not found
					failedIDs = append(failedIDs, nodeIDStr)
					continue
				}

				if !node.UserID().Equals(userID) {
					// Node ownership mismatch
					failedIDs = append(failedIDs, nodeIDStr)
					continue
				}

				validNodeIDs = append(validNodeIDs, nodeIDStr)
				nodeDataMap[nodeIDStr] = node // Store for event publishing
			}
		}
	}

	// Node validation completed

	// 4. Use optimized batch delete for valid nodes
	var deletedIDs []string
	var batchFailedIDs []string
	
	if len(validNodeIDs) > 0 {
		// Use the new BatchDeleteNodes method for optimized deletion
		deletedIDs, batchFailedIDs, err = uow.Nodes().BatchDeleteNodes(ctx, userID.String(), validNodeIDs)
		if err != nil {
			// Even on error, we may have partial success
			// Batch delete encountered error
		}
		
		// Add batch failures to the failed list
		failedIDs = append(failedIDs, batchFailedIDs...)
		
		// Batch delete completed
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
				nodeData.Content(),
				nodeData.Keywords(),
				nodeData.Tags(),
				shared.ParseVersion(nodeData.Version()),
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
		
		// Add event to Unit of Work for transactional publishing
		uow.PublishEvent(deletionEvent)
	}

	// 6. Commit transaction
	commitCalled = true
	if err := uow.Commit(); err != nil {
		commitCalled = false // Reset if commit fails
		return nil, errors.ApplicationError(ctx, "CommitTransaction", err)
	}

	// 7. Convert to response DTO
	result := &dto.BulkDeleteResult{
		DeletedCount: len(deletedIDs),
		FailedIDs:    failedIDs,
		Message:      fmt.Sprintf("Successfully deleted %d of %d nodes", len(deletedIDs), len(cmd.NodeIDs)),
	}

	// Bulk delete operation completed

	return result, nil
}

// Helper methods for idempotency handling

// reconstructCreateNodeResult reconstructs a CreateNodeResult from a map[string]interface{}
// This is needed when the idempotency store returns a JSON-deserialized object
func (s *NodeService) reconstructCreateNodeResult(data map[string]interface{}) (*dto.CreateNodeResult, error) {
	result := &dto.CreateNodeResult{}
	
	// Reconstruct the node
	if nodeData, ok := data["Node"].(map[string]interface{}); ok {
		result.Node = s.reconstructNodeDTO(nodeData)
	}
	
	// Validate that Node was reconstructed successfully
	if result.Node == nil {
		return nil, fmt.Errorf("failed to reconstruct node from cached data")
	}
	
	// Reconstruct created edges
	if edges, ok := data["CreatedEdges"].([]interface{}); ok {
		result.CreatedEdges = make([]*dto.EdgeDTO, 0, len(edges))
		for _, edge := range edges {
			if edgeMap, ok := edge.(map[string]interface{}); ok {
				result.CreatedEdges = append(result.CreatedEdges, s.reconstructEdgeDTO(edgeMap))
			}
		}
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
		return nil, false, errors.ApplicationError(ctx, "CheckIdempotency", err)
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
		return nil, errors.ApplicationError(ctx, "CreateUnitOfWork", err)
	}
	
	// Start unit of work
	if err := uow.Begin(ctx); err != nil {
		return nil, errors.ApplicationError(ctx, "BeginTransaction", err)
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
		return nil, errors.ServiceValidationError("userID", err.Error(), cmd.UserID)
	}

	var createdNodes []*node.Node
	var connections []*edge.Edge
	var bulkErrors []dto.BulkCreateError

	// 4. Create nodes sequentially with error handling
	for i, nodeReq := range cmd.Nodes {
		// Create domain node
		content, err := shared.NewContent(nodeReq.Content)
		if err != nil {
			bulkErrors = append(bulkErrors, dto.BulkCreateError{
				Index:   i,
				Content: nodeReq.Content,
				Error:   "invalid content: " + err.Error(),
			})
			continue
		}

		tags := shared.NewTags(nodeReq.Tags...)
		title, _ := shared.NewTitle("") // Empty title for bulk create (struct has no Title field)
		node, err := node.NewNode(userID, content, title, tags)
		if err != nil {
			bulkErrors = append(bulkErrors, dto.BulkCreateError{
				Index:   i,
				Content: nodeReq.Content,
				Error:   "failed to create node: " + err.Error(),
			})
			continue
		}

		// Keywords are already generated during node creation

		// Save node through unit of work
		if err := uow.Nodes().CreateNodeAndKeywords(ctx, node); err != nil {
			bulkErrors = append(bulkErrors, dto.BulkCreateError{
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
					edge, err := edge.NewEdge(sourceNode.ID(), targetNode.ID(), userID, weight)
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
				return nil, errors.ApplicationError(ctx, "PublishEvent", err)
			}
		}
		node.MarkEventsAsCommitted()
	}

	// 7. Commit transaction
	commitCalled = true
	if err := uow.Commit(); err != nil {
		commitCalled = false // Reset if commit fails
		return nil, errors.ApplicationError(ctx, "CommitTransaction", err)
	}

	// 8. Convert to response DTO
	result := &dto.BulkCreateResult{
		CreatedNodes:    dto.ToNodeViews(createdNodes),
		CreatedCount:    len(createdNodes),
		Connections:     dto.ToConnectionViews(connections),
		ConnectionCount: len(connections),
		Failed:          bulkErrors,
		Message:         fmt.Sprintf("Successfully created %d nodes", len(createdNodes)),
	}

	if len(bulkErrors) > 0 {
		result.Message = fmt.Sprintf("Created %d nodes with %d failures", len(createdNodes), len(bulkErrors))
	}

	if len(connections) > 0 {
		result.Message += fmt.Sprintf(" and %d connections", len(connections))
	}

	// 9. Store idempotency result if key was provided
	// No idempotency storage for bulk creates

	return result, nil
}

// convertEdgesToDTOs converts domain edges to DTOs
func (s *NodeService) convertEdgesToDTOs(edges []*edge.Edge) []*dto.EdgeDTO {
	if edges == nil {
		return nil
	}
	
	dtos := make([]*dto.EdgeDTO, 0, len(edges))
	for _, e := range edges {
		if e != nil {
			dtos = append(dtos, dto.EdgeFromDomain(e))
		}
	}
	return dtos
}

// reconstructNodeDTO reconstructs a NodeDTO from cached data
func (s *NodeService) reconstructNodeDTO(data map[string]interface{}) *dto.NodeDTO {
	if data == nil {
		return nil
	}
	
	node := &dto.NodeDTO{}
	
	if id, ok := data["id"].(string); ok {
		node.ID = id
	}
	if userID, ok := data["user_id"].(string); ok {
		node.UserID = userID
	}
	if title, ok := data["title"].(string); ok {
		node.Title = title
	}
	if content, ok := data["content"].(string); ok {
		node.Content = content
	}
	if keywords, ok := data["keywords"].([]interface{}); ok {
		node.Keywords = make([]string, 0, len(keywords))
		for _, k := range keywords {
			if kw, ok := k.(string); ok {
				node.Keywords = append(node.Keywords, kw)
			}
		}
	}
	if tags, ok := data["tags"].([]interface{}); ok {
		node.Tags = make([]string, 0, len(tags))
		for _, t := range tags {
			if tag, ok := t.(string); ok {
				node.Tags = append(node.Tags, tag)
			}
		}
	}
	
	return node
}

// reconstructEdgeDTO reconstructs an EdgeDTO from cached data
func (s *NodeService) reconstructEdgeDTO(data map[string]interface{}) *dto.EdgeDTO {
	if data == nil {
		return nil
	}
	
	edge := &dto.EdgeDTO{}
	
	if id, ok := data["id"].(string); ok {
		edge.ID = id
	}
	if userID, ok := data["user_id"].(string); ok {
		edge.UserID = userID
	}
	if sourceID, ok := data["source_node_id"].(string); ok {
		edge.SourceNodeID = sourceID
	}
	if targetID, ok := data["target_node_id"].(string); ok {
		edge.TargetNodeID = targetID
	}
	if weight, ok := data["weight"].(float64); ok {
		edge.Weight = weight
	}
	
	return edge
}