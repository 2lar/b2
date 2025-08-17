package bridges

import (
	"context"
	"fmt"
	"strings"

	"brain2-backend/internal/domain"
	"brain2-backend/internal/infrastructure/persistence"
	"brain2-backend/internal/repository"
	"go.uber.org/zap"
)

// EdgeReaderBridge implements repository.EdgeReader using the Store interface.
// This bridge provides CQRS read operations optimized for query scenarios.
type EdgeReaderBridge struct {
	store  persistence.Store
	logger *zap.Logger
}

// NewEdgeReaderBridge creates a new EdgeReaderBridge.
func NewEdgeReaderBridge(store persistence.Store, logger *zap.Logger) repository.EdgeReader {
	return &EdgeReaderBridge{
		store:  store,
		logger: logger,
	}
}

// FindByID retrieves a single edge by ID.
func (b *EdgeReaderBridge) FindByID(ctx context.Context, id domain.NodeID) (*domain.Edge, error) {
	// Note: This method signature seems incorrect in the interface - it should take an EdgeID, not NodeID
	// For now, we'll implement a workaround that searches for edges
	return nil, fmt.Errorf("FindByID with NodeID parameter not implemented - interface needs EdgeID parameter")
}

// Exists checks if an edge exists.
func (b *EdgeReaderBridge) Exists(ctx context.Context, id domain.NodeID) (bool, error) {
	edge, err := b.FindByID(ctx, id)
	if err != nil {
		return false, err
	}
	return edge != nil, nil
}

// FindByUser retrieves all edges for a user.
func (b *EdgeReaderBridge) FindByUser(ctx context.Context, userID domain.UserID, opts ...repository.QueryOption) ([]*domain.Edge, error) {
	// Apply query options
	queryOpts := &repository.QueryOptions{}
	for _, opt := range opts {
		opt(queryOpts)
	}

	query := persistence.Query{
		PartitionKey:  fmt.Sprintf("USER#%s#EDGE#", userID.String()),
		SortKeyPrefix: stringPtr("METADATA#"),
	}

	// Apply query options to store query
	if queryOpts.Limit > 0 {
		query.Limit = int32Ptr(int32(queryOpts.Limit))
	}

	result, err := b.store.Scan(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to scan user edges: %w", err)
	}

	var edges []*domain.Edge
	for _, record := range result.Records {
		edge, err := b.recordToEdge(&record)
		if err != nil {
			b.logger.Warn("failed to convert record to edge", zap.Error(err))
			continue
		}
		edges = append(edges, edge)
	}

	return edges, nil
}

// CountByUser counts edges for a user.
func (b *EdgeReaderBridge) CountByUser(ctx context.Context, userID domain.UserID) (int, error) {
	query := persistence.Query{
		PartitionKey:  fmt.Sprintf("USER#%s#EDGE#", userID.String()),
		SortKeyPrefix: stringPtr("METADATA#"),
	}

	result, err := b.store.Scan(ctx, query)
	if err != nil {
		return 0, fmt.Errorf("failed to count edges: %w", err)
	}

	return int(result.Count), nil
}

// FindBySourceNode finds edges originating from a specific source node.
func (b *EdgeReaderBridge) FindBySourceNode(ctx context.Context, sourceID domain.NodeID, opts ...repository.QueryOption) ([]*domain.Edge, error) {
	// Get all edges and filter by source node
	// In a real implementation, you might want to use a GSI for better performance
	userID := b.extractUserIDFromNodeID(sourceID)
	allEdges, err := b.FindByUser(ctx, userID, opts...)
	if err != nil {
		return nil, err
	}

	var sourceEdges []*domain.Edge
	for _, edge := range allEdges {
		if edge.SourceID.String() == sourceID.String() {
			sourceEdges = append(sourceEdges, edge)
		}
	}

	return sourceEdges, nil
}

// FindByTargetNode finds edges pointing to a specific target node.
func (b *EdgeReaderBridge) FindByTargetNode(ctx context.Context, targetID domain.NodeID, opts ...repository.QueryOption) ([]*domain.Edge, error) {
	// Get all edges and filter by target node
	userID := b.extractUserIDFromNodeID(targetID)
	allEdges, err := b.FindByUser(ctx, userID, opts...)
	if err != nil {
		return nil, err
	}

	var targetEdges []*domain.Edge
	for _, edge := range allEdges {
		if edge.TargetID.String() == targetID.String() {
			targetEdges = append(targetEdges, edge)
		}
	}

	return targetEdges, nil
}

// FindByNode finds edges connected to a specific node (either as source or target).
func (b *EdgeReaderBridge) FindByNode(ctx context.Context, nodeID domain.NodeID, opts ...repository.QueryOption) ([]*domain.Edge, error) {
	// Get all edges and filter by node (source or target)
	userID := b.extractUserIDFromNodeID(nodeID)
	allEdges, err := b.FindByUser(ctx, userID, opts...)
	if err != nil {
		return nil, err
	}

	var connectedEdges []*domain.Edge
	nodeIDStr := nodeID.String()
	for _, edge := range allEdges {
		if edge.SourceID.String() == nodeIDStr || edge.TargetID.String() == nodeIDStr {
			connectedEdges = append(connectedEdges, edge)
		}
	}

	return connectedEdges, nil
}

// FindBetweenNodes finds edges between two specific nodes.
func (b *EdgeReaderBridge) FindBetweenNodes(ctx context.Context, node1ID, node2ID domain.NodeID) ([]*domain.Edge, error) {
	// Get all edges for the user and filter for connections between the two nodes
	userID := b.extractUserIDFromNodeID(node1ID)
	allEdges, err := b.FindByUser(ctx, userID)
	if err != nil {
		return nil, err
	}

	var betweenEdges []*domain.Edge
	node1Str := node1ID.String()
	node2Str := node2ID.String()
	
	for _, edge := range allEdges {
		sourceStr := edge.SourceID.String()
		targetStr := edge.TargetID.String()
		
		// Check if edge connects the two nodes in either direction
		if (sourceStr == node1Str && targetStr == node2Str) ||
		   (sourceStr == node2Str && targetStr == node1Str) {
			betweenEdges = append(betweenEdges, edge)
		}
	}

	return betweenEdges, nil
}

// FindStrongConnections finds edges with strength above a threshold.
func (b *EdgeReaderBridge) FindStrongConnections(ctx context.Context, userID domain.UserID, threshold float64, opts ...repository.QueryOption) ([]*domain.Edge, error) {
	// Get all edges and filter by strength
	allEdges, err := b.FindByUser(ctx, userID, opts...)
	if err != nil {
		return nil, err
	}

	var strongEdges []*domain.Edge
	for _, edge := range allEdges {
		if edge.Strength >= threshold {
			strongEdges = append(strongEdges, edge)
		}
	}

	return strongEdges, nil
}

// FindWeakConnections finds edges with strength below a threshold.
func (b *EdgeReaderBridge) FindWeakConnections(ctx context.Context, userID domain.UserID, threshold float64, opts ...repository.QueryOption) ([]*domain.Edge, error) {
	// Get all edges and filter by strength
	allEdges, err := b.FindByUser(ctx, userID, opts...)
	if err != nil {
		return nil, err
	}

	var weakEdges []*domain.Edge
	for _, edge := range allEdges {
		if edge.Strength < threshold {
			weakEdges = append(weakEdges, edge)
		}
	}

	return weakEdges, nil
}

// FindBySpecification finds edges matching a specification.
func (b *EdgeReaderBridge) FindBySpecification(ctx context.Context, spec repository.Specification, opts ...repository.QueryOption) ([]*domain.Edge, error) {
	// This would require implementing the specification pattern for edges
	// For now, return empty result
	return []*domain.Edge{}, nil
}

// CountBySourceID counts edges from a specific source node.
func (b *EdgeReaderBridge) CountBySourceID(ctx context.Context, sourceID domain.NodeID) (int, error) {
	userID := b.extractUserIDFromNodeID(sourceID)
	allEdges, err := b.FindByUser(ctx, userID)
	if err != nil {
		return 0, err
	}

	count := 0
	sourceIDStr := sourceID.String()
	for _, edge := range allEdges {
		if edge.SourceID.String() == sourceIDStr {
			count++
		}
	}

	return count, nil
}

// CountBySpecification counts edges matching a specification.
func (b *EdgeReaderBridge) CountBySpecification(ctx context.Context, spec repository.Specification) (int, error) {
	// This would require implementing the specification pattern for edges
	// For now, return 0
	return 0, nil
}

// FindEdges finds edges based on query criteria (compatibility method).
func (b *EdgeReaderBridge) FindEdges(ctx context.Context, query repository.EdgeQuery) ([]*domain.Edge, error) {
	// Delegate to the existing method pattern based on query parameters
	userIDObj, err := domain.ParseUserID(query.UserID)
	if err != nil {
		return nil, fmt.Errorf("invalid user ID: %w", err)
	}

	// If specific node IDs are provided, find edges connected to those nodes
	if len(query.NodeIDs) > 0 {
		// Get all user edges and filter by node IDs
		allEdges, err := b.FindByUser(ctx, userIDObj)
		if err != nil {
			return nil, err
		}

		// Create a map for faster lookup
		nodeIDMap := make(map[string]bool)
		for _, nodeID := range query.NodeIDs {
			nodeIDMap[nodeID] = true
		}

		// Filter edges that have source or target in the node IDs
		var filteredEdges []*domain.Edge
		for _, edge := range allEdges {
			if nodeIDMap[edge.SourceID.String()] || nodeIDMap[edge.TargetID.String()] {
				filteredEdges = append(filteredEdges, edge)
			}
		}
		
		return filteredEdges, nil
	}

	// If source ID is provided, find edges from that source
	if query.SourceID != "" {
		sourceNodeID, err := domain.ParseNodeID(query.SourceID)
		if err != nil {
			return nil, fmt.Errorf("invalid source node ID: %w", err)
		}
		return b.FindBySourceNode(ctx, sourceNodeID)
	}

	// If target ID is provided, find edges to that target
	if query.TargetID != "" {
		targetNodeID, err := domain.ParseNodeID(query.TargetID)
		if err != nil {
			return nil, fmt.Errorf("invalid target node ID: %w", err)
		}
		return b.FindByTargetNode(ctx, targetNodeID)
	}

	// Default: return all user edges
	return b.FindByUser(ctx, userIDObj)
}

// FindPage finds edges with pagination.
func (b *EdgeReaderBridge) FindPage(ctx context.Context, query repository.EdgeQuery, pagination repository.Pagination) (*repository.EdgePage, error) {
	// Delegate to GetEdgesPage for now
	return b.GetEdgesPage(ctx, query, pagination)
}

// GetEdgesPage retrieves edges with pagination.
func (b *EdgeReaderBridge) GetEdgesPage(ctx context.Context, query repository.EdgeQuery, pagination repository.Pagination) (*repository.EdgePage, error) {
	storeQuery := persistence.Query{
		PartitionKey:  fmt.Sprintf("USER#%s#EDGE#", query.UserID),
		SortKeyPrefix: stringPtr("METADATA#"),
		Limit:         int32Ptr(int32(pagination.GetEffectiveLimit())),
	}

	// Add pagination cursor if provided
	if pagination.Cursor != "" {
		storeQuery.LastEvaluated = map[string]interface{}{
			"PK": fmt.Sprintf("USER#%s#EDGE#%s", query.UserID, pagination.Cursor),
			"SK": "METADATA#v0",
		}
	}

	result, err := b.store.Scan(ctx, storeQuery)
	if err != nil {
		return nil, fmt.Errorf("failed to get edges page: %w", err)
	}

	// Convert records to edges
	edges := make([]*domain.Edge, 0, len(result.Records))
	for _, record := range result.Records {
		edge, err := b.recordToEdge(&record)
		if err != nil {
			b.logger.Warn("failed to convert record to edge", zap.Error(err))
			continue
		}
		edges = append(edges, edge)
	}

	// Filter by node IDs if provided
	if len(query.NodeIDs) > 0 {
		nodeIDMap := make(map[string]bool)
		for _, nodeID := range query.NodeIDs {
			nodeIDMap[nodeID] = true
		}

		var filteredEdges []*domain.Edge
		for _, edge := range edges {
			if nodeIDMap[edge.SourceID.String()] || nodeIDMap[edge.TargetID.String()] {
				filteredEdges = append(filteredEdges, edge)
			}
		}
		edges = filteredEdges
	}

	// Filter by source ID if provided
	if query.SourceID != "" {
		var filteredEdges []*domain.Edge
		for _, edge := range edges {
			if edge.SourceID.String() == query.SourceID {
				filteredEdges = append(filteredEdges, edge)
			}
		}
		edges = filteredEdges
	}

	// Filter by target ID if provided
	if query.TargetID != "" {
		var filteredEdges []*domain.Edge
		for _, edge := range edges {
			if edge.TargetID.String() == query.TargetID {
				filteredEdges = append(filteredEdges, edge)
			}
		}
		edges = filteredEdges
	}

	// Determine next cursor
	var nextCursor string
	if result.LastEvaluated != nil {
		if pk, ok := result.LastEvaluated["PK"].(string); ok {
			parts := strings.Split(pk, "#")
			if len(parts) >= 4 {
				nextCursor = parts[3] // EDGE ID
			}
		}
	}

	return &repository.EdgePage{
		Items:      edges,
		HasMore:    nextCursor != "",
		NextCursor: nextCursor,
		TotalCount: int(result.Count),
		PageInfo: repository.PageInfo{
			PageSize:    pagination.GetEffectiveLimit(),
			ItemsInPage: len(edges),
		},
	}, nil
}

// Helper methods

func (b *EdgeReaderBridge) recordToEdge(record *persistence.Record) (*domain.Edge, error) {
	// Extract required fields
	userID, ok := record.Data["UserID"].(string)
	if !ok {
		return nil, fmt.Errorf("missing UserID in record")
	}

	sourceID, ok := record.Data["SourceID"].(string)
	if !ok {
		return nil, fmt.Errorf("missing SourceID in record")
	}

	targetID, ok := record.Data["TargetID"].(string)
	if !ok {
		return nil, fmt.Errorf("missing TargetID in record")
	}

	// Extract optional fields with defaults
	var strength float64 = 1.0
	if s, ok := record.Data["Strength"].(float64); ok {
		strength = s
	}

	// Reconstruct domain edge using the appropriate method
	return domain.ReconstructEdgeFromPrimitives(sourceID, targetID, userID, strength)
}

func (b *EdgeReaderBridge) extractUserIDFromNodeID(nodeID domain.NodeID) domain.UserID {
	// This is a helper method to extract userID from nodeID
	// In a real implementation, you might need to store user context differently
	// For now, we'll assume userID can be extracted from the nodeID string format
	// or use a different approach based on your domain model
	
	// This is a placeholder implementation - you'll need to implement based on your domain model
	userIDStr := "unknown" // This should be properly implemented
	userID, _ := domain.ParseUserID(userIDStr)
	return userID
}

