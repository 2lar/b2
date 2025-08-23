// Package dynamodb provides DynamoDB implementations of repository interfaces.
// This file implements NodeReader and NodeWriter interfaces using direct CQRS patterns.
package dynamodb

import (
	"context"
	"fmt"
	"strings"
	"time"
	
	"brain2-backend/internal/domain/node"
	"brain2-backend/internal/domain/shared"
	"brain2-backend/internal/repository"
	sharedContext "brain2-backend/internal/context"
	errorContext "brain2-backend/internal/errors"
	
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/expression"
	"github.com/aws/aws-sdk-go-v2/aws"
	"go.uber.org/zap"
)

// NodeRepository implements both NodeReader and NodeWriter interfaces directly.
type NodeRepository struct {
	client    *dynamodb.Client
	tableName string
	indexName string
	logger    *zap.Logger
}

// NewNodeRepository creates a new node repository with direct CQRS support.
func NewNodeRepository(client *dynamodb.Client, tableName, indexName string, logger *zap.Logger) *NodeRepository {
	return &NodeRepository{
		client:    client,
		tableName: tableName,
		indexName: indexName,
		logger:    logger,
	}
}

// Ensure interfaces are implemented
var (
	_ repository.NodeReader     = (*NodeRepository)(nil)
	_ repository.NodeWriter     = (*NodeRepository)(nil)
	_ repository.NodeRepository = (*NodeRepository)(nil)
)

// ============================================================================
// NODE READER INTERFACE - Read Operations
// ============================================================================

// FindByID retrieves a node by its ID.
func (r *NodeRepository) FindByID(ctx context.Context, id shared.NodeID) (*node.Node, error) {
	// Extract userID from context - required for DynamoDB composite key
	userID, ok := sharedContext.GetUserIDFromContext(ctx)
	if !ok {
		return nil, fmt.Errorf("user ID not found in context")
	}
	
	// Build the composite key for DynamoDB
	key := map[string]types.AttributeValue{
		"PK": StringAttr(BuildUserPK(userID)),
		"SK": StringAttr(BuildNodeSK(id.String())),
	}
	
	input := &dynamodb.GetItemInput{
		TableName: aws.String(r.tableName),
		Key:       key,
	}
	
	result, err := r.client.GetItem(ctx, input)
	if err != nil {
		return nil, errorContext.WrapWithContext(err, "DynamoDB GetItem failed for node %s", id.String())
	}
	
	if result.Item == nil {
		return nil, repository.ErrNodeNotFound
	}
	
	// Use custom parsing to handle different data formats
	node, err := r.parseNodeFromItem(result.Item)
	if err != nil {
		return nil, errorContext.WrapWithContext(err, "failed to parse node %s", id.String())
	}
	
	return node, nil
}

// Exists checks if a node exists.
func (r *NodeRepository) Exists(ctx context.Context, id shared.NodeID) (bool, error) {
	node, err := r.FindByID(ctx, id)
	if err == repository.ErrNodeNotFound {
		return false, nil
	}
	return node != nil, err
}

// FindByUser retrieves all nodes for a user.
func (r *NodeRepository) FindByUser(ctx context.Context, userID shared.UserID, opts ...repository.QueryOption) ([]*node.Node, error) {
	// Apply query options
	options := repository.ApplyQueryOptions(opts...)
	
	// Build key condition expression
	keyEx := expression.Key("PK").Equal(expression.Value(fmt.Sprintf("USER#%s", userID.String())))
	keyEx = keyEx.And(expression.Key("SK").BeginsWith("NODE#"))
	
	// Build the expression
	expr, err := expression.NewBuilder().
		WithKeyCondition(keyEx).
		Build()
	if err != nil {
		return nil, fmt.Errorf("failed to build expression: %w", err)
	}
	
	input := &dynamodb.QueryInput{
		TableName:                 aws.String(r.tableName),
		KeyConditionExpression:    expr.KeyCondition(),
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
		Limit:                     aws.Int32(int32(options.Limit)),
	}
	
	// Handle pagination cursor if provided
	if options.Cursor != "" {
		startKey, err := repository.DecodeCursor(options.Cursor)
		if err != nil {
			return nil, fmt.Errorf("failed to decode cursor: %w", err)
		}
		if startKey != nil {
			input.ExclusiveStartKey = startKey
		}
	}
	
	if options.SortOrder == repository.SortOrderAsc {
		input.ScanIndexForward = aws.Bool(true)
	} else {
		input.ScanIndexForward = aws.Bool(false)
	}
	
	result, err := r.client.Query(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to query nodes: %w", err)
	}
	
	nodes := make([]*node.Node, 0, len(result.Items))
	for _, item := range result.Items {
		// Use custom parsing to handle different data formats
		node, err := r.parseNodeFromItem(item)
		if err != nil {
			r.logger.Warn("Failed to parse node", zap.Error(err))
			continue
		}
		nodes = append(nodes, node)
	}
	
	// Store the LastEvaluatedKey for the next page (if needed by caller)
	// Note: The caller needs to be updated to receive this cursor
	// For now, we'll need to update FindPage to handle this properly
	
	return nodes, nil
}

// CountByUser counts nodes for a user.
func (r *NodeRepository) CountByUser(ctx context.Context, userID shared.UserID) (int, error) {
	keyEx := expression.Key("PK").Equal(expression.Value(fmt.Sprintf("USER#%s", userID.String())))
	keyEx = keyEx.And(expression.Key("SK").BeginsWith("NODE#"))
	
	expr, err := expression.NewBuilder().
		WithKeyCondition(keyEx).
		Build()
	if err != nil {
		return 0, fmt.Errorf("failed to build expression: %w", err)
	}
	
	input := &dynamodb.QueryInput{
		TableName:                 aws.String(r.tableName),
		KeyConditionExpression:    expr.KeyCondition(),
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
		Select:                    types.SelectCount,
	}
	
	result, err := r.client.Query(ctx, input)
	if err != nil {
		return 0, fmt.Errorf("failed to count nodes: %w", err)
	}
	
	return int(result.Count), nil
}

// FindByKeywords searches nodes by keywords.
func (r *NodeRepository) FindByKeywords(ctx context.Context, userID shared.UserID, keywords []string, opts ...repository.QueryOption) ([]*node.Node, error) {
	// This would typically use a GSI or search service
	// For now, fetch all nodes and filter in memory
	nodes, err := r.FindByUser(ctx, userID, opts...)
	if err != nil {
		return nil, err
	}
	
	// Filter nodes by keywords - pre-allocate with capacity to reduce allocations
	filtered := make([]*node.Node, 0, len(nodes))
	for _, node := range nodes {
		for _, keyword := range keywords {
			if node.HasKeyword(keyword) {
				filtered = append(filtered, node)
				break
			}
		}
	}
	
	return filtered, nil
}

// FindByTags searches nodes by tags.
func (r *NodeRepository) FindByTags(ctx context.Context, userID shared.UserID, tags []string, opts ...repository.QueryOption) ([]*node.Node, error) {
	// Similar to keywords, this would use a GSI in production
	nodes, err := r.FindByUser(ctx, userID, opts...)
	if err != nil {
		return nil, err
	}
	
	// Filter nodes by tags - pre-allocate with capacity to reduce allocations
	filtered := make([]*node.Node, 0, len(nodes))
	for _, node := range nodes {
		for _, tag := range tags {
			if node.HasTag(tag) {
				filtered = append(filtered, node)
				break
			}
		}
	}
	
	return filtered, nil
}

// FindByContent searches nodes by content.
func (r *NodeRepository) FindByContent(ctx context.Context, userID shared.UserID, searchTerm string, fuzzy bool, opts ...repository.QueryOption) ([]*node.Node, error) {
	// This would typically use a search service like ElasticSearch
	// For now, fetch all nodes and filter in memory
	nodes, err := r.FindByUser(ctx, userID, opts...)
	if err != nil {
		return nil, err
	}
	
	// Filter nodes by content - pre-allocate with capacity to reduce allocations
	filtered := make([]*node.Node, 0, len(nodes))
	// Pre-lowercase search term to avoid repeated allocations in loop
	lowerSearchTerm := strings.ToLower(searchTerm)
	for _, node := range nodes {
		// Check if search term is in content
		if strings.Contains(strings.ToLower(node.GetContent().String()), lowerSearchTerm) {
			filtered = append(filtered, node)
		}
	}
	
	return filtered, nil
}

// FindRecentlyCreated finds nodes created within the specified number of days.
func (r *NodeRepository) FindRecentlyCreated(ctx context.Context, userID shared.UserID, days int, opts ...repository.QueryOption) ([]*node.Node, error) {
	nodes, err := r.FindByUser(ctx, userID, opts...)
	if err != nil {
		return nil, err
	}
	
	cutoff := time.Now().AddDate(0, 0, -days)
	filtered := make([]*node.Node, 0)
	for _, node := range nodes {
		if node.CreatedAt().After(cutoff) {
			filtered = append(filtered, node)
		}
	}
	
	return filtered, nil
}

// FindRecentlyUpdated finds nodes updated within the specified number of days.
func (r *NodeRepository) FindRecentlyUpdated(ctx context.Context, userID shared.UserID, days int, opts ...repository.QueryOption) ([]*node.Node, error) {
	nodes, err := r.FindByUser(ctx, userID, opts...)
	if err != nil {
		return nil, err
	}
	
	cutoff := time.Now().AddDate(0, 0, -days)
	filtered := make([]*node.Node, 0)
	for _, node := range nodes {
		if node.UpdatedAt().After(cutoff) {
			filtered = append(filtered, node)
		}
	}
	
	return filtered, nil
}

// FindBySpecification finds nodes matching a specification.
func (r *NodeRepository) FindBySpecification(ctx context.Context, spec repository.Specification, opts ...repository.QueryOption) ([]*node.Node, error) {
	// This would be implemented based on the specification pattern
	// For now, return empty result
	return []*node.Node{}, nil
}

// CountBySpecification counts nodes matching a specification.
func (r *NodeRepository) CountBySpecification(ctx context.Context, spec repository.Specification) (int, error) {
	return 0, nil
}

// FindPage retrieves a page of nodes.
func (r *NodeRepository) FindPage(ctx context.Context, query repository.NodeQuery, pagination repository.Pagination) (*repository.NodePage, error) {
	userID, err := shared.NewUserID(query.UserID)
	if err != nil {
		return nil, fmt.Errorf("invalid user ID: %w", err)
	}
	
	// Build key condition expression
	keyEx := expression.Key("PK").Equal(expression.Value(fmt.Sprintf("USER#%s", userID.String())))
	keyEx = keyEx.And(expression.Key("SK").BeginsWith("NODE#"))
	
	// Build the expression
	expr, err := expression.NewBuilder().
		WithKeyCondition(keyEx).
		Build()
	if err != nil {
		return nil, fmt.Errorf("failed to build expression: %w", err)
	}
	
	input := &dynamodb.QueryInput{
		TableName:                 aws.String(r.tableName),
		KeyConditionExpression:    expr.KeyCondition(),
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
		Limit:                     aws.Int32(int32(pagination.Limit)),
		ScanIndexForward:          aws.Bool(false), // Sort descending by default
	}
	
	// Handle pagination cursor if provided
	if pagination.Cursor != "" {
		startKey, err := repository.DecodeCursor(pagination.Cursor)
		if err != nil {
			return nil, fmt.Errorf("failed to decode cursor: %w", err)
		}
		if startKey != nil {
			input.ExclusiveStartKey = startKey
		}
	}
	
	result, err := r.client.Query(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to query nodes: %w", err)
	}
	
	nodes := make([]*node.Node, 0, len(result.Items))
	for _, item := range result.Items {
		node, err := r.parseNodeFromItem(item)
		if err != nil {
			r.logger.Warn("Failed to parse node", zap.Error(err))
			continue
		}
		nodes = append(nodes, node)
	}
	
	// Generate next cursor from DynamoDB's LastEvaluatedKey
	nextCursor := ""
	if result.LastEvaluatedKey != nil {
		nextCursor = repository.EncodeCursor(result.LastEvaluatedKey)
	}
	
	return &repository.NodePage{
		Items:      nodes,
		NextCursor: nextCursor,
		HasMore:    result.LastEvaluatedKey != nil,
	}, nil
}

// FindConnected finds nodes connected to a specific node.
func (r *NodeRepository) FindConnected(ctx context.Context, nodeID shared.NodeID, depth int, opts ...repository.QueryOption) ([]*node.Node, error) {
	// This would require graph traversal, typically done with a graph database
	// For now, return empty result
	return []*node.Node{}, nil
}

// FindSimilar finds nodes similar to a specific node.
func (r *NodeRepository) FindSimilar(ctx context.Context, nodeID shared.NodeID, threshold float64, opts ...repository.QueryOption) ([]*node.Node, error) {
	// This would require similarity calculation, typically done with ML/vector DB
	// For now, return empty result
	return []*node.Node{}, nil
}

// GetNodesPage is a compatibility method for query service.
func (r *NodeRepository) GetNodesPage(ctx context.Context, query repository.NodeQuery, pagination repository.Pagination) (*repository.NodePage, error) {
	return r.FindPage(ctx, query, pagination)
}

// CountNodes is a compatibility method for query service.
func (r *NodeRepository) CountNodes(ctx context.Context, userID string) (int, error) {
	uid, err := shared.NewUserID(userID)
	if err != nil {
		return 0, err
	}
	return r.CountByUser(ctx, uid)
}

// ============================================================================
// NODE REPOSITORY INTERFACE - Additional Methods for Compatibility
// ============================================================================

// CreateNodeAndKeywords creates a new node with keywords (compatibility method).
func (r *NodeRepository) CreateNodeAndKeywords(ctx context.Context, node *node.Node) error {
	// Simply delegate to Save which already handles keywords
	return r.Save(ctx, node)
}

// FindNodeByID retrieves a node by user ID and node ID (compatibility method).
func (r *NodeRepository) FindNodeByID(ctx context.Context, userID, nodeID string) (*node.Node, error) {
	// Add userID to context for the FindByID method
	ctx = sharedContext.WithUserID(ctx, userID)
	
	nid, err := shared.ParseNodeID(nodeID)
	if err != nil {
		return nil, fmt.Errorf("invalid node ID: %w", err)
	}
	
	return r.FindByID(ctx, nid)
}

// FindNodes retrieves nodes based on a query (compatibility method).
func (r *NodeRepository) FindNodes(ctx context.Context, query repository.NodeQuery) ([]*node.Node, error) {
	// Add userID to context
	ctx = sharedContext.WithUserID(ctx, query.UserID)
	
	userID, err := shared.NewUserID(query.UserID)
	if err != nil {
		return nil, fmt.Errorf("invalid user ID: %w", err)
	}
	
	// Start with all user's nodes
	nodes, err := r.FindByUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	
	// Apply filters based on query
	if len(query.Keywords) > 0 {
		nodes, err = r.FindByKeywords(ctx, userID, query.Keywords)
		if err != nil {
			return nil, err
		}
	}
	
	if len(query.Tags) > 0 {
		filtered := make([]*node.Node, 0)
		for _, node := range nodes {
			for _, tag := range query.Tags {
				if node.HasTag(tag) {
					filtered = append(filtered, node)
					break
				}
			}
		}
		nodes = filtered
	}
	
	if query.SearchText != "" {
		filtered := make([]*node.Node, 0)
		for _, node := range nodes {
			if strings.Contains(strings.ToLower(node.GetContent().String()), strings.ToLower(query.SearchText)) {
				filtered = append(filtered, node)
			}
		}
		nodes = filtered
	}
	
	return nodes, nil
}

// DeleteNode deletes a node by user ID and node ID (compatibility method).
func (r *NodeRepository) DeleteNode(ctx context.Context, userID, nodeID string) error {
	// Add userID to context
	ctx = sharedContext.WithUserID(ctx, userID)
	
	nid, err := shared.ParseNodeID(nodeID)
	if err != nil {
		return fmt.Errorf("invalid node ID: %w", err)
	}
	
	return r.Delete(ctx, nid)
}

// GetNodeNeighborhood retrieves the neighborhood graph for a node.
func (r *NodeRepository) GetNodeNeighborhood(ctx context.Context, userID, nodeID string, depth int) (*shared.Graph, error) {
	// This would require graph traversal logic
	// For now, return an empty graph
	return &shared.Graph{
		Nodes: []interface{}{},
		Edges: []interface{}{},
	}, nil
}

// FindNodesWithOptions retrieves nodes with query options.
func (r *NodeRepository) FindNodesWithOptions(ctx context.Context, query repository.NodeQuery, opts ...repository.QueryOption) ([]*node.Node, error) {
	ctx = sharedContext.WithUserID(ctx, query.UserID)
	
	userID, err := shared.NewUserID(query.UserID)
	if err != nil {
		return nil, fmt.Errorf("invalid user ID: %w", err)
	}
	
	return r.FindByUser(ctx, userID, opts...)
}

// FindNodesPageWithOptions retrieves a page of nodes with options.
func (r *NodeRepository) FindNodesPageWithOptions(ctx context.Context, query repository.NodeQuery, pagination repository.Pagination, opts ...repository.QueryOption) (*repository.NodePage, error) {
	// Apply pagination as query options
	opts = append(opts,
		repository.WithLimit(pagination.Limit),
		repository.WithOffset(pagination.Offset),
	)
	
	if pagination.Cursor != "" {
		opts = append(opts, repository.WithCursor(pagination.Cursor))
	}
	
	nodes, err := r.FindNodesWithOptions(ctx, query, opts...)
	if err != nil {
		return nil, err
	}
	
	// Generate next cursor if we have a full page
	nextCursor := ""
	if len(nodes) == pagination.Limit && len(nodes) > 0 {
		lastNode := nodes[len(nodes)-1]
		nextCursor = lastNode.ID().String()
	}
	
	return &repository.NodePage{
		Items:      nodes,
		NextCursor: nextCursor,
		HasMore:    nextCursor != "",
	}, nil
}

// ============================================================================
// NODE WRITER INTERFACE - Write Operations
// ============================================================================

// Save creates a new node.
func (r *NodeRepository) Save(ctx context.Context, node *node.Node) error {
	// Extract userID from context
	userID, ok := sharedContext.GetUserIDFromContext(ctx)
	if !ok {
		return fmt.Errorf("user ID not found in context")
	}
	
	// Build the item with composite keys
	item := map[string]types.AttributeValue{
		"PK":        &types.AttributeValueMemberS{Value: fmt.Sprintf("USER#%s", userID)},
		"SK":        &types.AttributeValueMemberS{Value: fmt.Sprintf("NODE#%s", node.GetID())},
		"EntityType": &types.AttributeValueMemberS{Value: "NODE"},
		"NodeID":    &types.AttributeValueMemberS{Value: node.GetID()},
		"UserID":    &types.AttributeValueMemberS{Value: node.GetUserID().String()},
		"Content":   &types.AttributeValueMemberS{Value: node.GetContent().String()},
		"CreatedAt": &types.AttributeValueMemberS{Value: node.CreatedAt().Format(time.RFC3339)},
		"UpdatedAt": &types.AttributeValueMemberS{Value: node.UpdatedAt().Format(time.RFC3339)},
		"Version":   &types.AttributeValueMemberN{Value: fmt.Sprintf("%d", node.Version())},
	}
	
	// Add title if present
	if !node.GetTitle().IsEmpty() {
		item["Title"] = &types.AttributeValueMemberS{Value: node.GetTitle().String()}
	}
	
	// Add tags if present
	if node.GetTags().Count() > 0 {
		tagSlice := node.GetTags().ToSlice()
		tagList := &types.AttributeValueMemberL{
			Value: make([]types.AttributeValue, len(tagSlice)),
		}
		for i, tag := range tagSlice {
			tagList.Value[i] = &types.AttributeValueMemberS{Value: tag}
		}
		item["Tags"] = tagList
	}
	
	// Add keywords if present
	keywords := node.Keywords()
	if keywords.Count() > 0 {
		kwSlice := keywords.ToSlice()
		kwList := &types.AttributeValueMemberL{
			Value: make([]types.AttributeValue, len(kwSlice)),
		}
		for i, kw := range kwSlice {
			kwList.Value[i] = &types.AttributeValueMemberS{Value: kw}
		}
		item["Keywords"] = kwList
	}
	
	// Add metadata if present
	if node.Metadata() != nil {
		metaMap, err := attributevalue.Marshal(node.Metadata())
		if err == nil {
			item["Metadata"] = metaMap
		}
	}
	
	input := &dynamodb.PutItemInput{
		TableName:           aws.String(r.tableName),
		Item:                item,
		ConditionExpression: aws.String("attribute_not_exists(PK) AND attribute_not_exists(SK)"),
	}
	
	_, err := r.client.PutItem(ctx, input)
	if err != nil {
		return errorContext.WrapWithContext(err, "DynamoDB PutItem failed for node %s", node.GetID())
	}
	
	return nil
}

// SaveBatch saves multiple nodes in a batch.
func (r *NodeRepository) SaveBatch(ctx context.Context, nodes []*node.Node) error {
	// Process in batches of 25 (DynamoDB limit)
	const batchSize = 25
	
	for i := 0; i < len(nodes); i += batchSize {
		end := i + batchSize
		if end > len(nodes) {
			end = len(nodes)
		}
		
		batch := nodes[i:end]
		if err := r.saveBatch(ctx, batch); err != nil {
			return fmt.Errorf("failed to save batch: %w", err)
		}
	}
	
	return nil
}

func (r *NodeRepository) saveBatch(ctx context.Context, nodes []*node.Node) error {
	for _, node := range nodes {
		// Save each node individually for now
		// In production, use BatchWriteItem
		if err := r.Save(ctx, node); err != nil {
			return err
		}
	}
	
	return nil
}

// Update updates an existing node.
func (r *NodeRepository) Update(ctx context.Context, node *node.Node) error {
	// Extract userID from context
	userID, ok := sharedContext.GetUserIDFromContext(ctx)
	if !ok {
		return fmt.Errorf("user ID not found in context")
	}
	
	// Build update expression
	update := expression.Set(expression.Name("Content"), expression.Value(node.GetContent().String())).
		Set(expression.Name("UpdatedAt"), expression.Value(node.UpdatedAt().Format(time.RFC3339))).
		Set(expression.Name("Version"), expression.Value(node.Version()))
	
	// Add title if present, otherwise remove it
	if !node.GetTitle().IsEmpty() {
		update = update.Set(expression.Name("Title"), expression.Value(node.GetTitle().String()))
	} else {
		update = update.Remove(expression.Name("Title"))
	}
	
	// Add tags if present
	if node.GetTags().Count() > 0 {
		tags := node.GetTags().ToSlice()
		update = update.Set(expression.Name("Tags"), expression.Value(tags))
	}
	
	// Add keywords if present
	keywords := node.Keywords()
	if keywords.Count() > 0 {
		update = update.Set(expression.Name("Keywords"), expression.Value(keywords.ToSlice()))
	}
	
	// Build condition expression for optimistic locking
	condition := expression.Equal(expression.Name("Version"), expression.Value(node.Version()-1))
	
	expr, err := expression.NewBuilder().
		WithUpdate(update).
		WithCondition(condition).
		Build()
	if err != nil {
		return fmt.Errorf("failed to build expression: %w", err)
	}
	
	key := map[string]types.AttributeValue{
		"PK": &types.AttributeValueMemberS{Value: fmt.Sprintf("USER#%s", userID)},
		"SK": &types.AttributeValueMemberS{Value: fmt.Sprintf("NODE#%s", node.GetID())},
	}
	
	input := &dynamodb.UpdateItemInput{
		TableName:                 aws.String(r.tableName),
		Key:                       key,
		UpdateExpression:          expr.Update(),
		ConditionExpression:       expr.Condition(),
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
	}
	
	_, err = r.client.UpdateItem(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to update node: %w", err)
	}
	
	return nil
}

// UpdateBatch updates multiple nodes in a batch.
func (r *NodeRepository) UpdateBatch(ctx context.Context, nodes []*node.Node) error {
	for _, node := range nodes {
		if err := r.Update(ctx, node); err != nil {
			return err
		}
	}
	return nil
}

// Delete deletes a node.
func (r *NodeRepository) Delete(ctx context.Context, id shared.NodeID) error {
	// Extract userID from context
	userID, ok := sharedContext.GetUserIDFromContext(ctx)
	if !ok {
		return fmt.Errorf("user ID not found in context")
	}
	
	key := map[string]types.AttributeValue{
		"PK": &types.AttributeValueMemberS{Value: fmt.Sprintf("USER#%s", userID)},
		"SK": &types.AttributeValueMemberS{Value: fmt.Sprintf("NODE#%s", id.String())},
	}
	
	input := &dynamodb.DeleteItemInput{
		TableName: aws.String(r.tableName),
		Key:       key,
	}
	
	_, err := r.client.DeleteItem(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to delete node: %w", err)
	}
	
	return nil
}

// DeleteBatch deletes multiple nodes in a batch.
func (r *NodeRepository) DeleteBatch(ctx context.Context, ids []shared.NodeID) error {
	for _, id := range ids {
		if err := r.Delete(ctx, id); err != nil {
			return err
		}
	}
	return nil
}

// BatchDeleteNodes implements optimized batch deletion using DynamoDB BatchWriteItem.
// It processes nodes in chunks of 25 (DynamoDB limit) with automatic retry for unprocessed items.
// Returns slices of successfully deleted and failed node IDs.
func (r *NodeRepository) BatchDeleteNodes(ctx context.Context, userID string, nodeIDs []string) (deleted []string, failed []string, err error) {
	if len(nodeIDs) == 0 {
		return []string{}, []string{}, nil
	}

	deleted = make([]string, 0, len(nodeIDs))
	failed = make([]string, 0)

	// Process in chunks of 25 (DynamoDB BatchWriteItem limit)
	const batchSize = 25
	for i := 0; i < len(nodeIDs); i += batchSize {
		end := i + batchSize
		if end > len(nodeIDs) {
			end = len(nodeIDs)
		}
		chunk := nodeIDs[i:end]

		// Process this chunk with retry logic
		chunkDeleted, chunkFailed := r.processBatchDeleteChunk(ctx, userID, chunk)
		deleted = append(deleted, chunkDeleted...)
		failed = append(failed, chunkFailed...)
	}

	r.logger.Info("batch delete completed",
		zap.String("userID", userID),
		zap.Int("total", len(nodeIDs)),
		zap.Int("deleted", len(deleted)),
		zap.Int("failed", len(failed)))

	return deleted, failed, nil
}

// processBatchDeleteChunk processes a single chunk of up to 25 nodes with retry logic
func (r *NodeRepository) processBatchDeleteChunk(ctx context.Context, userID string, nodeIDs []string) (deleted []string, failed []string) {
	maxRetries := 3
	retryDelay := time.Millisecond * 100
	unprocessedIDs := nodeIDs
	deleted = make([]string, 0, len(nodeIDs))
	failed = make([]string, 0)

	for attempt := 0; attempt <= maxRetries && len(unprocessedIDs) > 0; attempt++ {
		if attempt > 0 {
			// Exponential backoff for retries
			time.Sleep(retryDelay)
			retryDelay *= 2
			r.logger.Debug("retrying batch delete",
				zap.Int("attempt", attempt),
				zap.Int("unprocessed", len(unprocessedIDs)))
		}

		// Build write requests for this attempt
		writeRequests := make([]types.WriteRequest, 0, len(unprocessedIDs))
		requestMap := make(map[string]bool) // Track which IDs are in this request

		for _, nodeID := range unprocessedIDs {
			writeRequests = append(writeRequests, types.WriteRequest{
				DeleteRequest: &types.DeleteRequest{
					Key: map[string]types.AttributeValue{
						"PK": &types.AttributeValueMemberS{Value: fmt.Sprintf("USER#%s", userID)},
						"SK": &types.AttributeValueMemberS{Value: fmt.Sprintf("NODE#%s", nodeID)},
					},
				},
			})
			requestMap[nodeID] = true
		}

		// Execute batch write
		input := &dynamodb.BatchWriteItemInput{
			RequestItems: map[string][]types.WriteRequest{
				r.tableName: writeRequests,
			},
		}

		output, err := r.client.BatchWriteItem(ctx, input)
		if err != nil {
			r.logger.Error("batch write failed",
				zap.Error(err),
				zap.Int("attempt", attempt))
			// On error, all items in this attempt are considered failed
			failed = append(failed, unprocessedIDs...)
			return deleted, failed
		}

		// Check for unprocessed items
		newUnprocessed := make([]string, 0)
		if output.UnprocessedItems != nil && len(output.UnprocessedItems[r.tableName]) > 0 {
			for _, req := range output.UnprocessedItems[r.tableName] {
				if req.DeleteRequest != nil {
					// Extract node ID from the SK
					sk := req.DeleteRequest.Key["SK"].(*types.AttributeValueMemberS).Value
					if strings.HasPrefix(sk, "NODE#") {
						nodeID := strings.TrimPrefix(sk, "NODE#")
						newUnprocessed = append(newUnprocessed, nodeID)
					}
				}
			}
		}

		// Calculate successfully deleted items in this attempt
		for nodeID := range requestMap {
			isUnprocessed := false
			for _, unprocessedID := range newUnprocessed {
				if nodeID == unprocessedID {
					isUnprocessed = true
					break
				}
			}
			if !isUnprocessed {
				deleted = append(deleted, nodeID)
			}
		}

		unprocessedIDs = newUnprocessed

		r.logger.Debug("batch delete attempt completed",
			zap.Int("attempt", attempt),
			zap.Int("processed", len(requestMap)-len(newUnprocessed)),
			zap.Int("remaining", len(newUnprocessed)))
	}

	// Any remaining unprocessed items after all retries are considered failed
	if len(unprocessedIDs) > 0 {
		failed = append(failed, unprocessedIDs...)
		r.logger.Warn("batch delete has unprocessed items after retries",
			zap.Int("unprocessed", len(unprocessedIDs)))
	}

	return deleted, failed
}

// BatchGetNodes retrieves multiple nodes in a single DynamoDB operation.
// Uses BatchGetItem to fetch up to 100 nodes at once, significantly reducing API calls.
// Returns a map of nodeID to node for efficient lookup.
func (r *NodeRepository) BatchGetNodes(ctx context.Context, userID string, nodeIDs []string) (map[string]*node.Node, error) {
	if len(nodeIDs) == 0 {
		return make(map[string]*node.Node), nil
	}

	result := make(map[string]*node.Node)
	
	// Process in chunks of 100 (DynamoDB BatchGetItem limit)
	const batchSize = 100
	for i := 0; i < len(nodeIDs); i += batchSize {
		end := i + batchSize
		if end > len(nodeIDs) {
			end = len(nodeIDs)
		}
		
		chunk := nodeIDs[i:end]
		chunkNodes, err := r.batchGetChunk(ctx, userID, chunk)
		if err != nil {
			return nil, err
		}
		
		// Add to result map
		for nodeID, node := range chunkNodes {
			result[nodeID] = node
		}
	}
	
	return result, nil
}

// batchGetChunk retrieves a chunk of up to 100 nodes using BatchGetItem
func (r *NodeRepository) batchGetChunk(ctx context.Context, userID string, nodeIDs []string) (map[string]*node.Node, error) {
	// Build keys for batch get
	keys := make([]map[string]types.AttributeValue, len(nodeIDs))
	for i, nodeID := range nodeIDs {
		keys[i] = map[string]types.AttributeValue{
			"PK": &types.AttributeValueMemberS{Value: fmt.Sprintf("USER#%s", userID)},
			"SK": &types.AttributeValueMemberS{Value: fmt.Sprintf("NODE#%s", nodeID)},
		}
	}

	input := &dynamodb.BatchGetItemInput{
		RequestItems: map[string]types.KeysAndAttributes{
			r.tableName: {
				Keys: keys,
			},
		},
	}

	output, err := r.client.BatchGetItem(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("BatchGetItem failed: %w", err)
	}

	// Convert items to nodes map
	result := make(map[string]*node.Node)
	for _, item := range output.Responses[r.tableName] {
		// Extract node ID from SK
		skAttr, ok := item["SK"]
		if !ok {
			continue
		}
		sk := skAttr.(*types.AttributeValueMemberS).Value
		if !strings.HasPrefix(sk, "NODE#") {
			continue
		}
		nodeID := strings.TrimPrefix(sk, "NODE#")
		
		// Parse node from item using existing method
		domainNode, err := r.parseNodeFromItem(item)
		if err != nil {
			r.logger.Warn("failed to parse node",
				zap.String("nodeID", nodeID),
				zap.Error(err))
			continue
		}
		
		result[nodeID] = domainNode
	}

	// Handle unprocessed keys with retry if needed
	if len(output.UnprocessedKeys) > 0 && len(output.UnprocessedKeys[r.tableName].Keys) > 0 {
		r.logger.Warn("BatchGetItem had unprocessed keys",
			zap.Int("count", len(output.UnprocessedKeys[r.tableName].Keys)))
		// For now, we'll just log this. In production, implement retry logic.
	}

	return result, nil
}

// Archive archives a node (soft delete).
func (r *NodeRepository) Archive(ctx context.Context, id shared.NodeID) error {
	// Extract userID from context
	userID, ok := sharedContext.GetUserIDFromContext(ctx)
	if !ok {
		return fmt.Errorf("user ID not found in context")
	}
	
	// Build update expression to set archived flag
	update := expression.Set(expression.Name("Archived"), expression.Value(true)).
		Set(expression.Name("ArchivedAt"), expression.Value(time.Now().Format(time.RFC3339)))
	
	expr, err := expression.NewBuilder().
		WithUpdate(update).
		Build()
	if err != nil {
		return fmt.Errorf("failed to build expression: %w", err)
	}
	
	key := map[string]types.AttributeValue{
		"PK": &types.AttributeValueMemberS{Value: fmt.Sprintf("USER#%s", userID)},
		"SK": &types.AttributeValueMemberS{Value: fmt.Sprintf("NODE#%s", id.String())},
	}
	
	input := &dynamodb.UpdateItemInput{
		TableName:                 aws.String(r.tableName),
		Key:                       key,
		UpdateExpression:          expr.Update(),
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
	}
	
	_, err = r.client.UpdateItem(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to archive node: %w", err)
	}
	
	return nil
}

// Unarchive unarchives a node.
func (r *NodeRepository) Unarchive(ctx context.Context, id shared.NodeID) error {
	// Extract userID from context
	userID, ok := sharedContext.GetUserIDFromContext(ctx)
	if !ok {
		return fmt.Errorf("user ID not found in context")
	}
	
	// Build update expression to remove archived flag
	update := expression.Remove(expression.Name("Archived")).
		Remove(expression.Name("ArchivedAt"))
	
	expr, err := expression.NewBuilder().
		WithUpdate(update).
		Build()
	if err != nil {
		return fmt.Errorf("failed to build expression: %w", err)
	}
	
	key := map[string]types.AttributeValue{
		"PK": &types.AttributeValueMemberS{Value: fmt.Sprintf("USER#%s", userID)},
		"SK": &types.AttributeValueMemberS{Value: fmt.Sprintf("NODE#%s", id.String())},
	}
	
	input := &dynamodb.UpdateItemInput{
		TableName:                 aws.String(r.tableName),
		Key:                       key,
		UpdateExpression:          expr.Update(),
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
	}
	
	_, err = r.client.UpdateItem(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to unarchive node: %w", err)
	}
	
	return nil
}

// UpdateVersion updates the version for optimistic locking.
func (r *NodeRepository) UpdateVersion(ctx context.Context, id shared.NodeID, expectedVersion shared.Version) error {
	// Extract userID from context
	userID, ok := sharedContext.GetUserIDFromContext(ctx)
	if !ok {
		return fmt.Errorf("user ID not found in context")
	}
	
	// Build update expression
	newVersion := expectedVersion.Int() + 1
	update := expression.Set(expression.Name("Version"), expression.Value(newVersion))
	
	// Build condition expression for optimistic locking
	condition := expression.Equal(expression.Name("Version"), expression.Value(expectedVersion.Int()))
	
	expr, err := expression.NewBuilder().
		WithUpdate(update).
		WithCondition(condition).
		Build()
	if err != nil {
		return fmt.Errorf("failed to build expression: %w", err)
	}
	
	key := map[string]types.AttributeValue{
		"PK": &types.AttributeValueMemberS{Value: fmt.Sprintf("USER#%s", userID)},
		"SK": &types.AttributeValueMemberS{Value: fmt.Sprintf("NODE#%s", id.String())},
	}
	
	input := &dynamodb.UpdateItemInput{
		TableName:                 aws.String(r.tableName),
		Key:                       key,
		UpdateExpression:          expr.Update(),
		ConditionExpression:       expr.Condition(),
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
	}
	
	_, err = r.client.UpdateItem(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to update version: %w", err)
	}
	
	return nil
}

// ============================================================================
// HELPER METHODS
// ============================================================================

// parseNodeFromItem parses a DynamoDB item into a Node domain object.
//
// This method handles backward compatibility by supporting multiple data storage formats:
// - Direct field storage (NodeID, UserID fields)
// - Composite key storage (extracted from PK/SK fields)
// - Multiple collection formats (List vs StringSet for tags/keywords)
//
// The parsing process follows these steps:
// 1. Extract scalar fields (IDs, content, version, timestamps)
// 2. Parse and validate domain value objects
// 3. Extract collections (tags, keywords) with format flexibility
// 4. Reconstruct the complete Node using domain factory methods
//
// Returns:
//   - *node.Node: Successfully parsed and validated node
//   - error: Validation error if any required field is invalid
func (r *NodeRepository) parseNodeFromItem(item map[string]types.AttributeValue) (*node.Node, error) {
	// Extract basic fields using helper methods
	nodeID := r.extractNodeID(item)
	userIDStr := r.extractUserID(item)
	contentStr := r.extractStringField(item, "Content")
	titleStr := r.extractStringField(item, "Title")
	version := r.extractVersion(item)
	
	// Parse timestamps using helper method
	createdAt, updatedAt := r.extractTimestamps(item)
	
	// Create domain objects
	nid, err := shared.ParseNodeID(nodeID)
	if err != nil {
		return nil, fmt.Errorf("invalid node ID: %w", err)
	}
	
	uid, err := shared.NewUserID(userIDStr)
	if err != nil {
		return nil, fmt.Errorf("invalid user ID: %w", err)
	}
	
	content, err := shared.NewContent(contentStr)
	if err != nil {
		return nil, fmt.Errorf("invalid content: %w", err)
	}
	
	title, err := shared.NewTitle(titleStr)
	if err != nil {
		return nil, fmt.Errorf("invalid title: %w", err)
	}
	
	// Parse collections using helper methods
	tags := r.extractStringArray(item, "Tags")
	keywords := r.extractStringArray(item, "Keywords")
	
	// Reconstruct the node using domain methods
	node := node.ReconstructNode(
		nid,
		uid,
		content,
		title,
		shared.NewKeywords(keywords),
		shared.NewTags(tags...),
		createdAt,
		updatedAt,
		shared.ParseVersion(version),
		false, // archived
	)
	
	return node, nil
}

// extractNodeID extracts node ID from DynamoDB item, handling both direct and SK formats
func (r *NodeRepository) extractNodeID(item map[string]types.AttributeValue) string {
	return ExtractNodeID(item)
}

// extractUserID extracts user ID from DynamoDB item, handling both direct and PK formats
func (r *NodeRepository) extractUserID(item map[string]types.AttributeValue) string {
	return ExtractUserID(item)
}

// extractStringField extracts a string field from DynamoDB item
func (r *NodeRepository) extractStringField(item map[string]types.AttributeValue, fieldName string) string {
	if attr, exists := item[fieldName]; exists {
		return ExtractStringValue(attr)
	}
	return ""
}

// extractVersion extracts version number from DynamoDB item
func (r *NodeRepository) extractVersion(item map[string]types.AttributeValue) int {
	if attr, exists := item["Version"]; exists {
		if version := ExtractNumberValue(attr); version > 0 {
			return version
		}
	}
	return 1 // default version
}

// extractTimestamps extracts created and updated timestamps from DynamoDB item
func (r *NodeRepository) extractTimestamps(item map[string]types.AttributeValue) (time.Time, time.Time) {
	now := time.Now()
	createdAt := now
	updatedAt := now
	
	if attr, exists := item["CreatedAt"]; exists {
		createdAt = ExtractTime(attr)
	}
	
	if attr, exists := item["UpdatedAt"]; exists {
		updatedAt = ExtractTime(attr)
	}
	
	return createdAt, updatedAt
}

// extractStringArray extracts string arrays from DynamoDB item, handling both List and StringSet formats
func (r *NodeRepository) extractStringArray(item map[string]types.AttributeValue, fieldName string) []string {
	if attr, exists := item[fieldName]; exists {
		return ExtractStringSet(attr)
	}
	return nil
}