package handlers

import (
	"context"
	"fmt"

	"brain2-backend/internal/application/commands"
	"brain2-backend/internal/application/dto"
	"brain2-backend/internal/domain/edge"
	"brain2-backend/internal/domain/node"
	"brain2-backend/internal/domain/shared"
	domainServices "brain2-backend/internal/domain/services"
	"brain2-backend/internal/errors"
	"brain2-backend/internal/repository"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// CreateNodeHandler handles the creation of nodes with automatic connection discovery
type CreateNodeHandler struct {
	uowFactory         repository.UnitOfWorkFactory
	eventBus           shared.EventBus
	connectionAnalyzer *domainServices.ConnectionAnalyzer
	idempotencyStore   repository.IdempotencyStore
	tracer             trace.Tracer
}

// NewCreateNodeHandler creates a new handler for node creation
func NewCreateNodeHandler(
	uowFactory repository.UnitOfWorkFactory,
	eventBus shared.EventBus,
	connectionAnalyzer *domainServices.ConnectionAnalyzer,
	idempotencyStore repository.IdempotencyStore,
	tracer trace.Tracer,
) *CreateNodeHandler {
	return &CreateNodeHandler{
		uowFactory:         uowFactory,
		eventBus:           eventBus,
		connectionAnalyzer: connectionAnalyzer,
		idempotencyStore:   idempotencyStore,
		tracer:             tracer,
	}
}

// Handle processes the CreateNodeCommand
func (h *CreateNodeHandler) Handle(ctx context.Context, cmd *commands.CreateNodeCommand) (*dto.CreateNodeResult, error) {
	// Start tracing span for the entire operation
	ctx, span := h.tracer.Start(ctx, "CreateNodeHandler.Handle",
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
	uow, err := h.uowFactory.Create(ctx)
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
			span.RecordError(errors.Internal("HANDLER_PANIC", fmt.Sprintf("panic: %v", r)).Build())
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
		if result, exists, err := h.checkIdempotency(ctx, cmd.IdempotencyKey, "CREATE_NODE", cmd.UserID); err != nil {
			return nil, err
		} else if exists {
			// Handle type assertion safely
			switch v := result.(type) {
			case *dto.CreateNodeResult:
				return v, nil
			case map[string]interface{}:
				reconstructed, err := h.reconstructCreateNodeResult(v)
				if err != nil {
					// If reconstruction fails, proceed with new creation
				} else if reconstructed != nil && reconstructed.Node != nil {
					return reconstructed, nil
				}
			}
		}
	}

	// 3. Convert application command to domain objects
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
	title, _ := shared.NewTitle(cmd.Title)
	node, err := node.NewNode(userID, content, title, tags)
	if err != nil {
		return nil, errors.ApplicationError(ctx, "CreateNode", err)
	}

	// 5. Find potential connections using domain service
	query := repository.NodeQuery{
		UserID: userID.String(),
	}
	existingNodes, err := uow.Nodes().FindNodes(ctx, query)
	if err != nil {
		return nil, errors.ApplicationError(ctx, "FindExistingNodes", err)
	}

	// Use domain service to analyze connections
	potentialConnections, err := h.connectionAnalyzer.FindPotentialConnections(node, existingNodes)
	if err != nil {
		return nil, errors.ApplicationError(ctx, "AnalyzeConnections", err)
	}

	// 6. Save the node first
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
		if err := h.eventBus.Publish(ctx, event); err != nil {
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

	// 10. Convert domain objects to DTOs for response
	result := &dto.CreateNodeResult{
		Node:         dto.NodeFromDomain(node),
		CreatedEdges: h.convertEdgesToDTOs(createdEdges),
	}

	// 11. Store idempotency result if key was provided
	if cmd.IdempotencyKey != "" {
		h.storeIdempotencyResult(ctx, cmd.IdempotencyKey, "CREATE_NODE", cmd.UserID, result)
	}

	return result, nil
}

// checkIdempotency checks if operation already exists
func (h *CreateNodeHandler) checkIdempotency(ctx context.Context, key, operation, userID string) (interface{}, bool, error) {
	// Placeholder implementation - would check idempotency store
	return nil, false, nil
}

// reconstructCreateNodeResult reconstructs result from map
func (h *CreateNodeHandler) reconstructCreateNodeResult(data map[string]interface{}) (*dto.CreateNodeResult, error) {
	// Placeholder implementation - would reconstruct DTO from map
	return nil, errors.NotFound("RECONSTRUCTION_NOT_IMPLEMENTED", "Reconstruction not implemented").
		WithOperation("ReconstructFromCache").
		WithResource("node").
		Build()
}

// storeIdempotencyResult stores result for idempotency
func (h *CreateNodeHandler) storeIdempotencyResult(ctx context.Context, key, operation, userID string, result interface{}) {
	// Placeholder implementation - would store in idempotency store
}

// convertEdgesToDTOs converts domain edges to DTOs
func (h *CreateNodeHandler) convertEdgesToDTOs(edges []*edge.Edge) []*dto.EdgeDTO {
	dtos := make([]*dto.EdgeDTO, len(edges))
	for i, e := range edges {
		dtos[i] = dto.EdgeFromDomain(e)
	}
	return dtos
}