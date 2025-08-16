// Package di provides dependency injection and service migration adapters
package di

import (
	"context"
	"fmt"

	"brain2-backend/internal/application/commands"
	"brain2-backend/internal/application/queries"
	"brain2-backend/internal/application/services"
	"brain2-backend/internal/domain"
	"brain2-backend/internal/repository"
	memoryService "brain2-backend/internal/service/memory"
)

// MemoryServiceAdapter adapts the new CQRS services to the old MemoryService interface.
// This allows for gradual migration from the legacy service to the new architecture.
//
// Key Concepts:
//   - Adapter Pattern: Bridges incompatible interfaces
//   - Gradual Migration: Allows old and new code to coexist
//   - CQRS Integration: Routes commands to NodeService, queries to NodeQueryService
//   - Fallback Support: Delegates unsupported operations to legacy service
type MemoryServiceAdapter struct {
	nodeAppService   *services.NodeService
	nodeQueryService *queries.NodeQueryService
	legacyService    memoryService.Service
}

// NewMemoryServiceAdapter creates a new adapter that bridges CQRS services with the legacy interface
func NewMemoryServiceAdapter(
	nodeAppService *services.NodeService,
	nodeQueryService *queries.NodeQueryService,
	legacyService memoryService.Service,
) memoryService.Service {
	return &MemoryServiceAdapter{
		nodeAppService:   nodeAppService,
		nodeQueryService: nodeQueryService,
		legacyService:    legacyService,
	}
}

// CreateNode uses the new CQRS command service
func (a *MemoryServiceAdapter) CreateNode(ctx context.Context, userID, content string, tags []string) (*domain.Node, []*domain.Edge, error) {
	// Create command
	cmd := &commands.CreateNodeCommand{
		UserID:  userID,
		Content: content,
		Tags:    tags,
	}

	// Execute command through new service
	result, err := a.nodeAppService.CreateNode(ctx, cmd)
	if err != nil {
		// Fallback to legacy service if CQRS fails
		return a.legacyService.CreateNode(ctx, userID, content, tags)
	}

	// Convert result to domain models
	// Parse domain values
	userIDDomain, _ := domain.ParseUserID(userID)
	nodeIDDomain, _ := domain.ParseNodeID(result.Node.ID)
	contentDomain, _ := domain.NewContent(result.Node.Content)
	tagsDomain := domain.NewTags(result.Node.Tags...)
	keywordsDomain := domain.NewKeywords(result.Node.Keywords)

	// Reconstruct the node
	node := domain.ReconstructNode(
		nodeIDDomain,
		userIDDomain,
		contentDomain,
		keywordsDomain,
		tagsDomain,
		result.Node.CreatedAt,
		result.Node.UpdatedAt,
		domain.NewVersion(),
		false, // not archived
	)

	// Convert connections to edges
	edges := make([]*domain.Edge, 0, len(result.Connections))
	for _, conn := range result.Connections {
		sourceID, _ := domain.ParseNodeID(conn.SourceNodeID)
		targetID, _ := domain.ParseNodeID(conn.TargetNodeID)
		
		edge, err := domain.NewEdge(sourceID, targetID, userIDDomain, conn.Strength)
		if err == nil {
			edges = append(edges, edge)
		}
	}

	return node, edges, nil
}

// UpdateNode uses the new CQRS command service
func (a *MemoryServiceAdapter) UpdateNode(ctx context.Context, userID, nodeID, content string, tags []string) (*domain.Node, error) {
	// Create update command
	cmd := &commands.UpdateNodeCommand{
		UserID:  userID,
		NodeID:  nodeID,
		Content: content,
		Tags:    tags,
		Version: 0, // Will be handled by optimistic locking
	}

	// Execute command through new service
	result, err := a.nodeAppService.UpdateNode(ctx, cmd)
	if err != nil {
		// Fallback to legacy service if CQRS fails
		return a.legacyService.UpdateNode(ctx, userID, nodeID, content, tags)
	}

	// Convert result to domain model
	userIDDomain, _ := domain.ParseUserID(userID)
	nodeIDDomain, _ := domain.ParseNodeID(result.Node.ID)
	contentDomain, _ := domain.NewContent(result.Node.Content)
	tagsDomain := domain.NewTags(result.Node.Tags...)
	keywordsDomain := domain.NewKeywords(result.Node.Keywords)

	node := domain.ReconstructNode(
		nodeIDDomain,
		userIDDomain,
		contentDomain,
		keywordsDomain,
		tagsDomain,
		result.Node.CreatedAt,
		result.Node.UpdatedAt,
		domain.NewVersion(),
		false, // not archived
	)

	return node, nil
}

// DeleteNode uses the new CQRS command service
func (a *MemoryServiceAdapter) DeleteNode(ctx context.Context, userID, nodeID string) error {
	// Create delete command
	cmd := &commands.DeleteNodeCommand{
		UserID: userID,
		NodeID: nodeID,
	}

	// Execute command through new service
	_, err := a.nodeAppService.DeleteNode(ctx, cmd)
	if err != nil {
		// Fallback to legacy service if CQRS fails
		return a.legacyService.DeleteNode(ctx, userID, nodeID)
	}

	return nil
}

// BulkDeleteNodes uses the new CQRS command service
func (a *MemoryServiceAdapter) BulkDeleteNodes(ctx context.Context, userID string, nodeIDs []string) (int, []string, error) {
	// Create bulk delete command
	cmd := &commands.BulkDeleteNodesCommand{
		UserID:  userID,
		NodeIDs: nodeIDs,
	}

	// Execute command through new service
	result, err := a.nodeAppService.BulkDeleteNodes(ctx, cmd)
	if err != nil {
		// Fallback to legacy service if CQRS fails
		return a.legacyService.BulkDeleteNodes(ctx, userID, nodeIDs)
	}

	return result.DeletedCount, result.FailedIDs, nil
}

// GetNodeDetails uses the new CQRS query service
func (a *MemoryServiceAdapter) GetNodeDetails(ctx context.Context, userID, nodeID string) (*domain.Node, []*domain.Edge, error) {
	// Create query
	query := &queries.GetNodeQuery{
		UserID:             userID,
		NodeID:             nodeID,
		IncludeConnections: true,
		IncludeMetadata:    false,
	}

	// Execute query through new service
	result, err := a.nodeQueryService.GetNode(ctx, query)
	if err != nil {
		// Fallback to legacy service if CQRS fails
		return a.legacyService.GetNodeDetails(ctx, userID, nodeID)
	}

	if result.Node == nil {
		return nil, nil, fmt.Errorf("node not found")
	}

	// Convert DTO to domain model
	userIDDomain, _ := domain.ParseUserID(userID)
	nodeIDDomain, _ := domain.ParseNodeID(result.Node.ID)
	content, _ := domain.NewContent(result.Node.Content)
	tags := domain.NewTags(result.Node.Tags...)
	keywords := domain.NewKeywords(result.Node.Keywords)

	node := domain.ReconstructNode(
		nodeIDDomain,
		userIDDomain,
		content,
		keywords,
		tags,
		result.Node.CreatedAt,
		result.Node.UpdatedAt,
		domain.NewVersion(),
		false, // not archived
	)

	// Convert connections to edges
	edges := make([]*domain.Edge, 0, len(result.Connections))
	for _, conn := range result.Connections {
		sourceID, _ := domain.ParseNodeID(conn.SourceNodeID)
		targetID, _ := domain.ParseNodeID(conn.TargetNodeID)
		
		edge, err := domain.NewEdge(sourceID, targetID, userIDDomain, conn.Strength)
		if err == nil {
			edges = append(edges, edge)
		}
	}

	return node, edges, nil
}

// GetNodes uses the new CQRS query service with pagination
func (a *MemoryServiceAdapter) GetNodes(ctx context.Context, userID string, pageReq repository.PageRequest) (*repository.PageResponse, error) {
	// The new query service uses a different query structure
	// We need to adapt the PageRequest to what the query service expects
	
	// For now, delegate to legacy service since ListNodesQuery has different structure
	return a.legacyService.GetNodes(ctx, userID, pageReq)
}

// GetGraphData delegates to legacy service (graph operations not yet migrated)
func (a *MemoryServiceAdapter) GetGraphData(ctx context.Context, userID string) (*domain.Graph, error) {
	return a.legacyService.GetGraphData(ctx, userID)
}