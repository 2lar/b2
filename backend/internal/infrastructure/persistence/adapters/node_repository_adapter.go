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

// NodeRepositoryAdapter implements repository.NodeRepository using the Store interface.
// This adapter provides database-agnostic node operations through the persistence layer.
type NodeRepositoryAdapter struct {
	store  persistence.Store
	logger *zap.Logger
}

// NewNodeRepositoryAdapter creates a new NodeRepositoryAdapter.
func NewNodeRepositoryAdapter(store persistence.Store, logger *zap.Logger) repository.NodeRepository {
	return &NodeRepositoryAdapter{
		store:  store,
		logger: logger,
	}
}

// CreateNodeAndKeywords creates a node and its keyword indexes atomically.
func (r *NodeRepositoryAdapter) CreateNodeAndKeywords(ctx context.Context, node *domain.Node) error {
	r.logger.Debug("creating node with keywords",
		zap.String("node_id", node.ID.String()),
		zap.Strings("keywords", node.Keywords().ToSlice()))

	// Prepare transaction operations
	operations := make([]persistence.Operation, 0)

	// 1. Add the main node record
	nodeKey := persistence.Key{
		PartitionKey: fmt.Sprintf("USER#%s#NODE#%s", node.UserID.String(), node.ID.String()),
		SortKey:      "METADATA#v0",
	}

	nodeData := map[string]interface{}{
		"NodeID":    node.ID.String(),
		"UserID":    node.UserID.String(),
		"Content":   node.Content.String(),
		"Keywords":  node.Keywords().ToSlice(),
		"Tags":      node.Tags.ToSlice(),
		"IsLatest":  true,
		"Version":   node.Version,
		"Timestamp": node.CreatedAt.Format(time.RFC3339),
	}

	operations = append(operations, persistence.Operation{
		Type: persistence.OperationTypePut,
		Key:  nodeKey,
		Data: nodeData,
	})

	// 2. Add keyword index records
	for _, keyword := range node.Keywords().ToSlice() {
		keywordKey := persistence.Key{
			PartitionKey: fmt.Sprintf("USER#%s#NODE#%s", node.UserID.String(), node.ID.String()),
			SortKey:      fmt.Sprintf("KEYWORD#%s", keyword),
		}

		keywordData := map[string]interface{}{
			"PK":     keywordKey.PartitionKey,
			"SK":     keywordKey.SortKey,
			"GSI1PK": fmt.Sprintf("USER#%s#KEYWORD#%s", node.UserID.String(), keyword),
			"GSI1SK": fmt.Sprintf("NODE#%s", node.ID.String()),
		}

		operations = append(operations, persistence.Operation{
			Type: persistence.OperationTypePut,
			Key:  keywordKey,
			Data: keywordData,
		})
	}

	// Execute transaction
	err := r.store.Transaction(ctx, operations)
	if err != nil {
		return fmt.Errorf("failed to create node with keywords: %w", err)
	}

	r.logger.Debug("successfully created node with keywords",
		zap.String("node_id", node.ID.String()),
		zap.Int("keyword_count", len(node.Keywords().ToSlice())))

	return nil
}

// FindNodeByID retrieves a node by its ID.
func (r *NodeRepositoryAdapter) FindNodeByID(ctx context.Context, userID, nodeID string) (*domain.Node, error) {
	key := persistence.Key{
		PartitionKey: fmt.Sprintf("USER#%s#NODE#%s", userID, nodeID),
		SortKey:      "METADATA#v0",
	}

	record, err := r.store.Get(ctx, key)
	if err != nil {
		return nil, fmt.Errorf("failed to get node: %w", err)
	}

	if record == nil {
		return nil, nil // Node not found
	}

	// Convert record to domain.Node
	node, err := r.recordToNode(record)
	if err != nil {
		return nil, fmt.Errorf("failed to convert record to node: %w", err)
	}

	return node, nil
}

// FindNodes finds nodes based on query criteria.
func (r *NodeRepositoryAdapter) FindNodes(ctx context.Context, query repository.NodeQuery) ([]*domain.Node, error) {
	r.logger.Debug("finding nodes",
		zap.String("user_id", query.UserID),
		zap.Strings("keywords", query.Keywords),
		zap.Strings("node_ids", query.NodeIDs))

	// If specific node IDs are provided, use direct lookup
	if len(query.NodeIDs) > 0 {
		return r.findNodesByIDs(ctx, query.UserID, query.NodeIDs)
	}

	// If keywords are provided, use keyword-based search
	if len(query.Keywords) > 0 {
		return r.findNodesByKeywords(ctx, query.UserID, query.Keywords)
	}

	// Default: scan all user nodes
	return r.findAllUserNodes(ctx, query.UserID)
}

// DeleteNode removes a node and its associated data.
func (r *NodeRepositoryAdapter) DeleteNode(ctx context.Context, userID, nodeID string) error {
	// First, find all related records (node + keywords)
	nodePrefix := fmt.Sprintf("USER#%s#NODE#%s", userID, nodeID)
	
	query := persistence.Query{
		PartitionKey:  nodePrefix,
		SortKeyPrefix: nil, // Get all records for this node
	}

	result, err := r.store.Query(ctx, query)
	if err != nil {
		return fmt.Errorf("failed to find node records for deletion: %w", err)
	}

	// Prepare delete operations
	operations := make([]persistence.Operation, len(result.Records))
	for i, record := range result.Records {
		operations[i] = persistence.Operation{
			Type: persistence.OperationTypeDelete,
			Key:  record.Key,
		}
	}

	// Execute transaction
	err = r.store.Transaction(ctx, operations)
	if err != nil {
		return fmt.Errorf("failed to delete node: %w", err)
	}

	r.logger.Debug("successfully deleted node",
		zap.String("node_id", nodeID),
		zap.Int("records_deleted", len(operations)))

	return nil
}

// GetNodesPage retrieves a paginated list of nodes.
func (r *NodeRepositoryAdapter) GetNodesPage(ctx context.Context, query repository.NodeQuery, pagination repository.Pagination) (*repository.NodePage, error) {
	// Convert pagination to store query
	storeQuery := persistence.Query{
		PartitionKey:  fmt.Sprintf("USER#%s#NODE#", query.UserID),
		SortKeyPrefix: stringPtr("METADATA#"),
		Limit:         int32Ptr(int32(pagination.GetEffectiveLimit())),
	}

	// Add pagination cursor if provided
	if pagination.Cursor != "" {
		// Parse cursor to last evaluated key
		// For simplicity, we'll assume cursor contains the last node ID
		lastNodeID := pagination.Cursor
		storeQuery.LastEvaluated = map[string]interface{}{
			"PK": fmt.Sprintf("USER#%s#NODE#%s", query.UserID, lastNodeID),
			"SK": "METADATA#v0",
		}
	}

	result, err := r.store.Scan(ctx, storeQuery) // Use Scan for pagination across all nodes
	if err != nil {
		return nil, fmt.Errorf("failed to get nodes page: %w", err)
	}

	// Convert records to nodes
	nodes := make([]*domain.Node, 0, len(result.Records))
	for _, record := range result.Records {
		node, err := r.recordToNode(&record)
		if err != nil {
			r.logger.Warn("failed to convert record to node", zap.Error(err))
			continue
		}
		nodes = append(nodes, node)
	}

	// Determine next cursor
	var nextCursor string
	if result.LastEvaluated != nil {
		if pk, ok := result.LastEvaluated["PK"].(string); ok {
			// Extract node ID from PK
			parts := strings.Split(pk, "#")
			if len(parts) >= 4 {
				nextCursor = parts[3] // NODE ID
			}
		}
	}

	return &repository.NodePage{
		Items:      nodes,
		HasMore:    nextCursor != "",
		NextCursor: nextCursor,
		TotalCount: int(result.Count),
		PageInfo: repository.PageInfo{
			PageSize:    pagination.GetEffectiveLimit(),
			ItemsInPage: len(nodes),
		},
	}, nil
}

// GetNodeNeighborhood retrieves nodes connected to a specific node.
func (r *NodeRepositoryAdapter) GetNodeNeighborhood(ctx context.Context, userID, nodeID string, depth int) (*domain.Graph, error) {
	// This is a complex operation that would require edge data
	// For now, return a simple implementation
	node, err := r.FindNodeByID(ctx, userID, nodeID)
	if err != nil {
		return nil, err
	}

	if node == nil {
		return &domain.Graph{Nodes: []*domain.Node{}, Edges: []*domain.Edge{}}, nil
	}

	return &domain.Graph{Nodes: []*domain.Node{node}, Edges: []*domain.Edge{}}, nil
}

// CountNodes counts the total number of nodes for a user.
func (r *NodeRepositoryAdapter) CountNodes(ctx context.Context, userID string) (int, error) {
	query := persistence.Query{
		PartitionKey:  fmt.Sprintf("USER#%s#NODE#", userID),
		SortKeyPrefix: stringPtr("METADATA#"),
	}

	result, err := r.store.Scan(ctx, query)
	if err != nil {
		return 0, fmt.Errorf("failed to count nodes: %w", err)
	}

	return int(result.Count), nil
}

// FindNodesWithOptions finds nodes with additional query options.
func (r *NodeRepositoryAdapter) FindNodesWithOptions(ctx context.Context, query repository.NodeQuery, opts ...repository.QueryOption) ([]*domain.Node, error) {
	// Apply query options to create QueryOptions
	queryOpts := &repository.QueryOptions{}
	for _, opt := range opts {
		opt(queryOpts)
	}

	// Use the regular FindNodes method for now
	// TODO: Apply queryOpts for filtering/sorting
	return r.FindNodes(ctx, query)
}

// FindNodesPageWithOptions finds nodes with pagination and additional options.
func (r *NodeRepositoryAdapter) FindNodesPageWithOptions(ctx context.Context, query repository.NodeQuery, pagination repository.Pagination, opts ...repository.QueryOption) (*repository.NodePage, error) {
	// Apply query options to create QueryOptions
	queryOpts := &repository.QueryOptions{}
	for _, opt := range opts {
		opt(queryOpts)
	}

	return r.GetNodesPage(ctx, query, pagination)
}

// Helper methods

func (r *NodeRepositoryAdapter) findNodesByIDs(ctx context.Context, userID string, nodeIDs []string) ([]*domain.Node, error) {
	keys := make([]persistence.Key, len(nodeIDs))
	for i, nodeID := range nodeIDs {
		keys[i] = persistence.Key{
			PartitionKey: fmt.Sprintf("USER#%s#NODE#%s", userID, nodeID),
			SortKey:      "METADATA#v0",
		}
	}

	records, err := r.store.BatchGet(ctx, keys)
	if err != nil {
		return nil, fmt.Errorf("failed to batch get nodes: %w", err)
	}

	nodes := make([]*domain.Node, 0, len(records))
	for _, record := range records {
		node, err := r.recordToNode(&record)
		if err != nil {
			r.logger.Warn("failed to convert record to node", zap.Error(err))
			continue
		}
		nodes = append(nodes, node)
	}

	return nodes, nil
}

func (r *NodeRepositoryAdapter) findNodesByKeywords(ctx context.Context, userID string, keywords []string) ([]*domain.Node, error) {
	nodeIDMap := make(map[string]bool)
	
	for _, keyword := range keywords {
		query := persistence.Query{
			PartitionKey: fmt.Sprintf("USER#%s#KEYWORD#%s", userID, keyword),
			IndexName:    stringPtr("GSI1"),
		}

		result, err := r.store.Query(ctx, query)
		if err != nil {
			r.logger.Warn("failed to query keyword index",
				zap.String("keyword", keyword),
				zap.Error(err))
			continue
		}

		// Extract node IDs from GSI1SK
		for _, record := range result.Records {
			if gsi1sk, ok := record.Data["GSI1SK"].(string); ok {
				if strings.HasPrefix(gsi1sk, "NODE#") {
					nodeID := strings.TrimPrefix(gsi1sk, "NODE#")
					nodeIDMap[nodeID] = true
				}
			}
		}
	}

	// Convert map to slice
	nodeIDs := make([]string, 0, len(nodeIDMap))
	for nodeID := range nodeIDMap {
		nodeIDs = append(nodeIDs, nodeID)
	}

	return r.findNodesByIDs(ctx, userID, nodeIDs)
}

func (r *NodeRepositoryAdapter) findAllUserNodes(ctx context.Context, userID string) ([]*domain.Node, error) {
	query := persistence.Query{
		PartitionKey:  fmt.Sprintf("USER#%s#NODE#", userID),
		SortKeyPrefix: stringPtr("METADATA#"),
	}

	result, err := r.store.Scan(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to scan user nodes: %w", err)
	}

	nodes := make([]*domain.Node, 0, len(result.Records))
	for _, record := range result.Records {
		node, err := r.recordToNode(&record)
		if err != nil {
			r.logger.Warn("failed to convert record to node", zap.Error(err))
			continue
		}
		nodes = append(nodes, node)
	}

	return nodes, nil
}

func (r *NodeRepositoryAdapter) recordToNode(record *persistence.Record) (*domain.Node, error) {
	// Extract required fields
	nodeID, ok := record.Data["NodeID"].(string)
	if !ok {
		return nil, fmt.Errorf("missing NodeID in record")
	}

	userID, ok := record.Data["UserID"].(string)
	if !ok {
		return nil, fmt.Errorf("missing UserID in record")
	}

	content, ok := record.Data["Content"].(string)
	if !ok {
		return nil, fmt.Errorf("missing Content in record")
	}

	// Extract optional fields with defaults
	var keywords []string
	if kw, ok := record.Data["Keywords"].([]interface{}); ok {
		keywords = make([]string, len(kw))
		for i, k := range kw {
			if s, ok := k.(string); ok {
				keywords[i] = s
			}
		}
	}

	var tags []string
	if tg, ok := record.Data["Tags"].([]interface{}); ok {
		tags = make([]string, len(tg))
		for i, t := range tg {
			if s, ok := t.(string); ok {
				tags[i] = s
			}
		}
	}

	var version int
	if v, ok := record.Data["Version"].(int); ok {
		version = v
	}

	// Parse timestamp
	var createdAt time.Time
	if ts, ok := record.Data["Timestamp"].(string); ok {
		createdAt, _ = time.Parse(time.RFC3339, ts)
	} else {
		createdAt = record.CreatedAt
	}

	// Reconstruct domain node
	return domain.ReconstructNodeFromPrimitives(
		nodeID,
		userID,
		content,
		keywords,
		tags,
		createdAt,
		version,
	)
}

// Helper functions
func stringPtr(s string) *string {
	return &s
}

func int32Ptr(i int32) *int32 {
	return &i
}