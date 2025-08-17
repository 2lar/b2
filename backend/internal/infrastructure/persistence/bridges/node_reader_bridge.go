package bridges

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

// NodeReaderBridge implements repository.NodeReader using the Store interface.
// This bridge provides CQRS read operations optimized for query scenarios.
type NodeReaderBridge struct {
	store  persistence.Store
	logger *zap.Logger
}

// NewNodeReaderBridge creates a new NodeReaderBridge.
func NewNodeReaderBridge(store persistence.Store, logger *zap.Logger) repository.NodeReader {
	return &NodeReaderBridge{
		store:  store,
		logger: logger,
	}
}

// FindByID retrieves a single node by ID.
func (b *NodeReaderBridge) FindByID(ctx context.Context, id domain.NodeID) (*domain.Node, error) {
	// For now, we need to scan to find the node since we don't have user context
	// In a real implementation, you might store a global node index
	// TODO: Improve this with proper indexing
	
	// This is a simplified implementation that would need optimization
	return nil, fmt.Errorf("FindByID without user context not implemented")
}

// Exists checks if a node exists.
func (b *NodeReaderBridge) Exists(ctx context.Context, id domain.NodeID) (bool, error) {
	node, err := b.FindByID(ctx, id)
	if err != nil {
		return false, err
	}
	return node != nil, nil
}

// FindByUser retrieves all nodes for a user.
func (b *NodeReaderBridge) FindByUser(ctx context.Context, userID domain.UserID, opts ...repository.QueryOption) ([]*domain.Node, error) {
	// Apply query options
	queryOpts := &repository.QueryOptions{}
	for _, opt := range opts {
		opt(queryOpts)
	}

	query := persistence.Query{
		PartitionKey:  fmt.Sprintf("USER#%s#NODE#", userID.String()),
		SortKeyPrefix: stringPtr("METADATA#"),
	}

	// Apply query options to store query
	if queryOpts.Limit > 0 {
		query.Limit = int32Ptr(int32(queryOpts.Limit))
	}

	result, err := b.store.Scan(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to scan user nodes: %w", err)
	}

	var nodes []*domain.Node
	for _, record := range result.Records {
		node, err := b.recordToNode(&record)
		if err != nil {
			b.logger.Warn("failed to convert record to node", zap.Error(err))
			continue
		}
		nodes = append(nodes, node)
	}

	return nodes, nil
}

// CountByUser counts nodes for a user.
func (b *NodeReaderBridge) CountByUser(ctx context.Context, userID domain.UserID) (int, error) {
	query := persistence.Query{
		PartitionKey:  fmt.Sprintf("USER#%s#NODE#", userID.String()),
		SortKeyPrefix: stringPtr("METADATA#"),
	}

	result, err := b.store.Scan(ctx, query)
	if err != nil {
		return 0, fmt.Errorf("failed to count nodes: %w", err)
	}

	return int(result.Count), nil
}

// CountNodes compatibility method that accepts string userID for query service compatibility
func (b *NodeReaderBridge) CountNodes(ctx context.Context, userID string) (int, error) {
	userIDObj, err := domain.ParseUserID(userID)
	if err != nil {
		return 0, fmt.Errorf("invalid user ID: %w", err)
	}
	return b.CountByUser(ctx, userIDObj)
}

// FindByKeywords finds nodes containing specific keywords.
func (b *NodeReaderBridge) FindByKeywords(ctx context.Context, userID domain.UserID, keywords []string, opts ...repository.QueryOption) ([]*domain.Node, error) {
	// Get all user nodes first
	allNodes, err := b.FindByUser(ctx, userID, opts...)
	if err != nil {
		return nil, err
	}

	// Filter nodes by keywords (simple text matching)
	var matchingNodes []*domain.Node
	for _, node := range allNodes {
		if b.nodeContainsKeywords(node, keywords) {
			matchingNodes = append(matchingNodes, node)
		}
	}

	return matchingNodes, nil
}

// FindByTitle finds nodes by title pattern.
func (b *NodeReaderBridge) FindByTitle(ctx context.Context, userID domain.UserID, titlePattern string, opts ...repository.QueryOption) ([]*domain.Node, error) {
	// Get all user nodes first
	allNodes, err := b.FindByUser(ctx, userID, opts...)
	if err != nil {
		return nil, err
	}

	// Filter nodes by title pattern
	var matchingNodes []*domain.Node
	for _, node := range allNodes {
		if strings.Contains(strings.ToLower(node.Content.String()), strings.ToLower(titlePattern)) {
			matchingNodes = append(matchingNodes, node)
		}
	}

	return matchingNodes, nil
}

// FindByContent finds nodes by content pattern.
func (b *NodeReaderBridge) FindByContent(ctx context.Context, userID domain.UserID, contentPattern string, fuzzy bool, opts ...repository.QueryOption) ([]*domain.Node, error) {
	// Get all user nodes first
	allNodes, err := b.FindByUser(ctx, userID, opts...)
	if err != nil {
		return nil, err
	}

	// Filter nodes by content pattern
	var matchingNodes []*domain.Node
	for _, node := range allNodes {
		if strings.Contains(strings.ToLower(node.Content.String()), strings.ToLower(contentPattern)) {
			matchingNodes = append(matchingNodes, node)
		}
	}

	return matchingNodes, nil
}

// FindRecentlyModified finds recently modified nodes.
func (b *NodeReaderBridge) FindRecentlyModified(ctx context.Context, userID domain.UserID, since time.Time, opts ...repository.QueryOption) ([]*domain.Node, error) {
	// Get all user nodes first
	allNodes, err := b.FindByUser(ctx, userID, opts...)
	if err != nil {
		return nil, err
	}

	// Filter nodes by modification time
	var recentNodes []*domain.Node
	for _, node := range allNodes {
		if node.UpdatedAt.After(since) {
			recentNodes = append(recentNodes, node)
		}
	}

	return recentNodes, nil
}

// FindByCreationDate finds nodes created within a date range.
func (b *NodeReaderBridge) FindByCreationDate(ctx context.Context, userID domain.UserID, from, to time.Time, opts ...repository.QueryOption) ([]*domain.Node, error) {
	// Get all user nodes first
	allNodes, err := b.FindByUser(ctx, userID, opts...)
	if err != nil {
		return nil, err
	}

	// Filter nodes by creation date
	var dateRangeNodes []*domain.Node
	for _, node := range allNodes {
		createdAt := node.CreatedAt
		if (createdAt.Equal(from) || createdAt.After(from)) && (createdAt.Equal(to) || createdAt.Before(to)) {
			dateRangeNodes = append(dateRangeNodes, node)
		}
	}

	return dateRangeNodes, nil
}

// GetNodesPage retrieves nodes with pagination.
func (b *NodeReaderBridge) GetNodesPage(ctx context.Context, query repository.NodeQuery, pagination repository.Pagination) (*repository.NodePage, error) {
	storeQuery := persistence.Query{
		PartitionKey:  fmt.Sprintf("USER#%s#NODE#", query.UserID),
		SortKeyPrefix: stringPtr("METADATA#"),
		Limit:         int32Ptr(int32(pagination.GetEffectiveLimit())),
	}

	// Add pagination cursor if provided
	if pagination.Cursor != "" {
		storeQuery.LastEvaluated = map[string]interface{}{
			"PK": fmt.Sprintf("USER#%s#NODE#%s", query.UserID, pagination.Cursor),
			"SK": "METADATA#v0",
		}
	}

	result, err := b.store.Scan(ctx, storeQuery)
	if err != nil {
		return nil, fmt.Errorf("failed to get nodes page: %w", err)
	}

	// Convert records to nodes
	nodes := make([]*domain.Node, 0, len(result.Records))
	for _, record := range result.Records {
		node, err := b.recordToNode(&record)
		if err != nil {
			b.logger.Warn("failed to convert record to node", zap.Error(err))
			continue
		}
		nodes = append(nodes, node)
	}

	// Filter by search text if provided
	if query.SearchText != "" {
		var filteredNodes []*domain.Node
		for _, node := range nodes {
			if strings.Contains(strings.ToLower(node.Content.String()), strings.ToLower(query.SearchText)) {
				filteredNodes = append(filteredNodes, node)
			}
		}
		nodes = filteredNodes
	}

	// Determine next cursor
	var nextCursor string
	if result.LastEvaluated != nil {
		if pk, ok := result.LastEvaluated["PK"].(string); ok {
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

// Helper methods

func (b *NodeReaderBridge) recordToNode(record *persistence.Record) (*domain.Node, error) {
	// Extract required fields
	nodeID, ok := record.Data["NodeID"].(string)
	if !ok {
		return nil, fmt.Errorf("missing NodeID in record")
	}

	userID, ok := record.Data["UserID"].(string)
	if !ok {
		return nil, fmt.Errorf("missing UserID in record")
	}

	// Title field is not used in reconstruction, content is used instead

	content, ok := record.Data["Content"].(string)
	if !ok {
		return nil, fmt.Errorf("missing Content in record")
	}

	// Extract optional fields
	keywords := []string{}
	if k, ok := record.Data["Keywords"].([]interface{}); ok {
		for _, keyword := range k {
			if str, ok := keyword.(string); ok {
				keywords = append(keywords, str)
			}
		}
	}

	// Reconstruct domain node using the appropriate method
	return domain.ReconstructNodeFromPrimitives(nodeID, userID, content, keywords, []string{}, record.CreatedAt, 0)
}

func (b *NodeReaderBridge) nodeContainsKeywords(node *domain.Node, keywords []string) bool {
	nodeText := strings.ToLower(node.Content.String())
	for _, keyword := range keywords {
		if strings.Contains(nodeText, strings.ToLower(keyword)) {
			return true
		}
	}
	return false
}

// CountBySpecification counts nodes matching a specification.
func (b *NodeReaderBridge) CountBySpecification(ctx context.Context, spec repository.Specification) (int, error) {
	// This would require implementing the specification pattern for nodes
	// For now, return 0
	return 0, nil
}

// FindByTags finds nodes by tags.
func (b *NodeReaderBridge) FindByTags(ctx context.Context, userID domain.UserID, tags []string, opts ...repository.QueryOption) ([]*domain.Node, error) {
	// Get all user nodes first
	allNodes, err := b.FindByUser(ctx, userID, opts...)
	if err != nil {
		return nil, err
	}

	// Filter nodes by tags
	var matchingNodes []*domain.Node
	for _, node := range allNodes {
		nodeTags := node.Tags.ToSlice()
		for _, tag := range tags {
			for _, nodeTag := range nodeTags {
				if strings.EqualFold(nodeTag, tag) {
					matchingNodes = append(matchingNodes, node)
					break
				}
			}
		}
	}

	return matchingNodes, nil
}

// FindRecentlyCreated finds recently created nodes.
func (b *NodeReaderBridge) FindRecentlyCreated(ctx context.Context, userID domain.UserID, days int, opts ...repository.QueryOption) ([]*domain.Node, error) {
	// Get all user nodes first
	allNodes, err := b.FindByUser(ctx, userID, opts...)
	if err != nil {
		return nil, err
	}

	// Filter nodes by creation date
	cutoffDate := time.Now().AddDate(0, 0, -days)
	var recentNodes []*domain.Node
	for _, node := range allNodes {
		if node.CreatedAt.After(cutoffDate) {
			recentNodes = append(recentNodes, node)
		}
	}

	return recentNodes, nil
}

// FindRecentlyUpdated finds recently updated nodes.
func (b *NodeReaderBridge) FindRecentlyUpdated(ctx context.Context, userID domain.UserID, days int, opts ...repository.QueryOption) ([]*domain.Node, error) {
	// Get all user nodes first
	allNodes, err := b.FindByUser(ctx, userID, opts...)
	if err != nil {
		return nil, err
	}

	// Filter nodes by update date
	cutoffDate := time.Now().AddDate(0, 0, -days)
	var recentNodes []*domain.Node
	for _, node := range allNodes {
		if node.UpdatedAt.After(cutoffDate) {
			recentNodes = append(recentNodes, node)
		}
	}

	return recentNodes, nil
}

// FindBySpecification finds nodes matching a specification.
func (b *NodeReaderBridge) FindBySpecification(ctx context.Context, spec repository.Specification, opts ...repository.QueryOption) ([]*domain.Node, error) {
	// TODO: Implement specification pattern for nodes
	// For now, return empty result
	return []*domain.Node{}, nil
}

// FindPage finds nodes with pagination.
func (b *NodeReaderBridge) FindPage(ctx context.Context, query repository.NodeQuery, pagination repository.Pagination) (*repository.NodePage, error) {
	// Delegate to GetNodesPage for now
	return b.GetNodesPage(ctx, query, pagination)
}

// FindConnected finds nodes connected to a specific node.
func (b *NodeReaderBridge) FindConnected(ctx context.Context, nodeID domain.NodeID, depth int, opts ...repository.QueryOption) ([]*domain.Node, error) {
	// TODO: Implement graph traversal with specified depth
	// For now, return empty result
	return []*domain.Node{}, nil
}

// FindSimilar finds nodes similar to a specific node.
func (b *NodeReaderBridge) FindSimilar(ctx context.Context, nodeID domain.NodeID, threshold float64, opts ...repository.QueryOption) ([]*domain.Node, error) {
	// TODO: Implement similarity calculation based on content, keywords, etc.
	// For now, return empty result
	return []*domain.Node{}, nil
}