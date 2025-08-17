package adapters

import (
	"context"
	"fmt"
	"strings"
	"time"

	"brain2-backend/internal/domain"
	"brain2-backend/internal/infrastructure/persistence"
	"brain2-backend/internal/repository"
	"go.uber.org/zap"
)

// EdgeRepositoryAdapter implements repository.EdgeRepository using the Store interface.
// This adapter provides database-agnostic edge operations through the persistence layer.
type EdgeRepositoryAdapter struct {
	store  persistence.Store
	logger *zap.Logger
}

// NewEdgeRepositoryAdapter creates a new EdgeRepositoryAdapter.
func NewEdgeRepositoryAdapter(store persistence.Store, logger *zap.Logger) repository.EdgeRepository {
	return &EdgeRepositoryAdapter{
		store:  store,
		logger: logger,
	}
}

// CreateEdges creates multiple edges from a source node to related nodes.
func (r *EdgeRepositoryAdapter) CreateEdges(ctx context.Context, userID, sourceNodeID string, relatedNodeIDs []string) error {
	// Create edges for each related node
	for _, targetNodeID := range relatedNodeIDs {
		// Parse IDs
		sourceID, err := domain.ParseNodeID(sourceNodeID)
		if err != nil {
			return fmt.Errorf("invalid source node ID: %w", err)
		}
		
		targetID, err := domain.ParseNodeID(targetNodeID)
		if err != nil {
			return fmt.Errorf("invalid target node ID: %w", err)
		}
		
		userIDObj, err := domain.ParseUserID(userID)
		if err != nil {
			return fmt.Errorf("invalid user ID: %w", err)
		}
		
		// Create edge using domain constructor
		edge, err := domain.NewEdge(sourceID, targetID, userIDObj, 1.0) // Default strength
		if err != nil {
			return fmt.Errorf("failed to create edge: %w", err)
		}
		
		if err := r.CreateEdge(ctx, edge); err != nil {
			return fmt.Errorf("failed to create edge from %s to %s: %w", sourceNodeID, targetNodeID, err)
		}
	}
	return nil
}

// CreateEdge creates a new edge.
func (r *EdgeRepositoryAdapter) CreateEdge(ctx context.Context, edge *domain.Edge) error {
	r.logger.Debug("creating edge",
		zap.String("edge_id", edge.ID.String()),
		zap.String("source_id", edge.SourceID.String()),
		zap.String("target_id", edge.TargetID.String()))

	// Create the edge record
	edgeKey := persistence.Key{
		PartitionKey: fmt.Sprintf("USER#%s#EDGE#%s", edge.UserID().String(), edge.ID.String()),
		SortKey:      "METADATA#v0",
	}

	edgeData := map[string]interface{}{
		"EdgeID":      edge.ID.String(),
		"UserID":      edge.UserID().String(),
		"SourceID":    edge.SourceID.String(),
		"TargetID":    edge.TargetID.String(),
		"Strength":    edge.Strength,
		"EdgeType":    edge.EdgeType,
		"IsLatest":    true,
		"Version":     edge.Version,
		"Timestamp":   edge.CreatedAt.Format(time.RFC3339),
	}

	edgeRecord := persistence.Record{
		Key:       edgeKey,
		Data:      edgeData,
		Version:   int64(edge.Version),
		CreatedAt: edge.CreatedAt,
		UpdatedAt: edge.UpdatedAt,
	}

	err := r.store.Put(ctx, edgeRecord)
	if err != nil {
		return fmt.Errorf("failed to create edge: %w", err)
	}

	r.logger.Debug("successfully created edge", zap.String("edge_id", edge.ID.String()))
	return nil
}

// FindEdgeByID retrieves an edge by its ID.
func (r *EdgeRepositoryAdapter) FindEdgeByID(ctx context.Context, userID, edgeID string) (*domain.Edge, error) {
	key := persistence.Key{
		PartitionKey: fmt.Sprintf("USER#%s#EDGE#%s", userID, edgeID),
		SortKey:      "METADATA#v0",
	}

	record, err := r.store.Get(ctx, key)
	if err != nil {
		return nil, fmt.Errorf("failed to get edge: %w", err)
	}

	if record == nil {
		return nil, nil // Edge not found
	}

	// Convert record to domain.Edge
	edge, err := r.recordToEdge(record)
	if err != nil {
		return nil, fmt.Errorf("failed to convert record to edge: %w", err)
	}

	return edge, nil
}

// FindEdges finds edges based on query criteria.
func (r *EdgeRepositoryAdapter) FindEdges(ctx context.Context, query repository.EdgeQuery) ([]*domain.Edge, error) {
	r.logger.Debug("finding edges",
		zap.String("user_id", query.UserID),
		zap.Strings("node_ids", query.NodeIDs),
		zap.String("source_id", query.SourceID),
		zap.String("target_id", query.TargetID))

	// If specific node IDs are provided, find edges connected to those nodes
	if len(query.NodeIDs) > 0 {
		return r.findEdgesByNodeIDs(ctx, query.UserID, query.NodeIDs)
	}

	// If source ID is provided, find edges from that source
	if query.SourceID != "" {
		return r.findEdgesBySourceID(ctx, query.UserID, query.SourceID)
	}

	// If target ID is provided, find edges to that target
	if query.TargetID != "" {
		return r.findEdgesByTargetID(ctx, query.UserID, query.TargetID)
	}

	// Default: scan all user edges
	return r.findAllUserEdges(ctx, query.UserID)
}

// DeleteEdge removes an edge.
func (r *EdgeRepositoryAdapter) DeleteEdge(ctx context.Context, userID, edgeID string) error {
	key := persistence.Key{
		PartitionKey: fmt.Sprintf("USER#%s#EDGE#%s", userID, edgeID),
		SortKey:      "METADATA#v0",
	}

	err := r.store.Delete(ctx, key)
	if err != nil {
		return fmt.Errorf("failed to delete edge: %w", err)
	}

	r.logger.Debug("successfully deleted edge", zap.String("edge_id", edgeID))
	return nil
}

// GetEdgesPage retrieves a paginated list of edges.
func (r *EdgeRepositoryAdapter) GetEdgesPage(ctx context.Context, query repository.EdgeQuery, pagination repository.Pagination) (*repository.EdgePage, error) {
	// Convert pagination to store query
	storeQuery := persistence.Query{
		PartitionKey:  fmt.Sprintf("USER#%s#EDGE#", query.UserID),
		SortKeyPrefix: stringPtr("METADATA#"),
		Limit:         int32Ptr(int32(pagination.GetEffectiveLimit())),
	}

	// Add pagination cursor if provided
	if pagination.Cursor != "" {
		// Parse cursor to last evaluated key
		lastEdgeID := pagination.Cursor
		storeQuery.LastEvaluated = map[string]interface{}{
			"PK": fmt.Sprintf("USER#%s#EDGE#%s", query.UserID, lastEdgeID),
			"SK": "METADATA#v0",
		}
	}

	result, err := r.store.Scan(ctx, storeQuery)
	if err != nil {
		return nil, fmt.Errorf("failed to get edges page: %w", err)
	}

	// Convert records to edges
	edges := make([]*domain.Edge, 0, len(result.Records))
	for _, record := range result.Records {
		edge, err := r.recordToEdge(&record)
		if err != nil {
			r.logger.Warn("failed to convert record to edge", zap.Error(err))
			continue
		}
		edges = append(edges, edge)
	}

	// Determine next cursor
	var nextCursor string
	if result.LastEvaluated != nil {
		if pk, ok := result.LastEvaluated["PK"].(string); ok {
			// Extract edge ID from PK
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

// CountEdges counts the total number of edges for a user.
func (r *EdgeRepositoryAdapter) CountEdges(ctx context.Context, userID string) (int, error) {
	query := persistence.Query{
		PartitionKey:  fmt.Sprintf("USER#%s#EDGE#", userID),
		SortKeyPrefix: stringPtr("METADATA#"),
	}

	result, err := r.store.Scan(ctx, query)
	if err != nil {
		return 0, fmt.Errorf("failed to count edges: %w", err)
	}

	return int(result.Count), nil
}

// FindEdgesWithOptions finds edges with additional query options.
func (r *EdgeRepositoryAdapter) FindEdgesWithOptions(ctx context.Context, query repository.EdgeQuery, opts ...repository.QueryOption) ([]*domain.Edge, error) {
	// Apply query options to create QueryOptions
	queryOpts := &repository.QueryOptions{}
	for _, opt := range opts {
		opt(queryOpts)
	}

	// Use the regular FindEdges method for now
	// TODO: Apply queryOpts for filtering/sorting
	return r.FindEdges(ctx, query)
}

// Helper methods

func (r *EdgeRepositoryAdapter) findEdgesBySourceID(ctx context.Context, userID, sourceID string) ([]*domain.Edge, error) {
	// TODO: Implement finding edges from specific source
	return r.findAllUserEdges(ctx, userID)
}

func (r *EdgeRepositoryAdapter) findEdgesByTargetID(ctx context.Context, userID, targetID string) ([]*domain.Edge, error) {
	// TODO: Implement finding edges to specific target
	return r.findAllUserEdges(ctx, userID)
}

func (r *EdgeRepositoryAdapter) findEdgesByIDs(ctx context.Context, userID string, edgeIDs []string) ([]*domain.Edge, error) {
	keys := make([]persistence.Key, len(edgeIDs))
	for i, edgeID := range edgeIDs {
		keys[i] = persistence.Key{
			PartitionKey: fmt.Sprintf("USER#%s#EDGE#%s", userID, edgeID),
			SortKey:      "METADATA#v0",
		}
	}

	records, err := r.store.BatchGet(ctx, keys)
	if err != nil {
		return nil, fmt.Errorf("failed to batch get edges: %w", err)
	}

	edges := make([]*domain.Edge, 0, len(records))
	for _, record := range records {
		edge, err := r.recordToEdge(&record)
		if err != nil {
			r.logger.Warn("failed to convert record to edge", zap.Error(err))
			continue
		}
		edges = append(edges, edge)
	}

	return edges, nil
}

func (r *EdgeRepositoryAdapter) findEdgesByNodeIDs(ctx context.Context, userID string, nodeIDs []string) ([]*domain.Edge, error) {
	// For edge queries by node IDs, we need to scan all edges and filter
	// In a real implementation, you might want to use GSI for better performance
	allEdges, err := r.findAllUserEdges(ctx, userID)
	if err != nil {
		return nil, err
	}

	// Create a map for faster lookup
	nodeIDMap := make(map[string]bool)
	for _, nodeID := range nodeIDs {
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

func (r *EdgeRepositoryAdapter) findAllUserEdges(ctx context.Context, userID string) ([]*domain.Edge, error) {
	query := persistence.Query{
		PartitionKey:  fmt.Sprintf("USER#%s#EDGE#", userID),
		SortKeyPrefix: stringPtr("METADATA#"),
	}

	result, err := r.store.Scan(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to scan user edges: %w", err)
	}

	edges := make([]*domain.Edge, 0, len(result.Records))
	for _, record := range result.Records {
		edge, err := r.recordToEdge(&record)
		if err != nil {
			r.logger.Warn("failed to convert record to edge", zap.Error(err))
			continue
		}
		edges = append(edges, edge)
	}

	return edges, nil
}

func (r *EdgeRepositoryAdapter) recordToEdge(record *persistence.Record) (*domain.Edge, error) {
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

	// Reconstruct domain edge
	return domain.ReconstructEdgeFromPrimitives(sourceID, targetID, userID, strength)
}