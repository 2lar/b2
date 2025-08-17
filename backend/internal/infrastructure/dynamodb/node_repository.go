// Package dynamodb provides DynamoDB implementations of repository interfaces.
// This file implements NodeReader and NodeWriter interfaces using direct CQRS patterns.
package dynamodb

import (
	"context"
	"fmt"
	"strings"
	"time"
	
	"brain2-backend/internal/domain"
	"brain2-backend/internal/repository"
	sharedContext "brain2-backend/internal/context"
	
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
func (r *NodeRepository) FindByID(ctx context.Context, id domain.NodeID) (*domain.Node, error) {
	// Extract userID from context - required for DynamoDB composite key
	userID, ok := sharedContext.GetUserIDFromContext(ctx)
	if !ok {
		return nil, fmt.Errorf("user ID not found in context")
	}
	
	// Build the composite key for DynamoDB
	key := map[string]types.AttributeValue{
		"PK": &types.AttributeValueMemberS{Value: fmt.Sprintf("USER#%s", userID)},
		"SK": &types.AttributeValueMemberS{Value: fmt.Sprintf("NODE#%s", id.String())},
	}
	
	input := &dynamodb.GetItemInput{
		TableName: aws.String(r.tableName),
		Key:       key,
	}
	
	result, err := r.client.GetItem(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to get node: %w", err)
	}
	
	if result.Item == nil {
		return nil, repository.ErrNodeNotFound
	}
	
	// Use custom parsing to handle different data formats
	node, err := r.parseNodeFromItem(result.Item)
	if err != nil {
		return nil, fmt.Errorf("failed to parse node: %w", err)
	}
	
	return node, nil
}

// Exists checks if a node exists.
func (r *NodeRepository) Exists(ctx context.Context, id domain.NodeID) (bool, error) {
	node, err := r.FindByID(ctx, id)
	if err == repository.ErrNodeNotFound {
		return false, nil
	}
	return node != nil, err
}

// FindByUser retrieves all nodes for a user.
func (r *NodeRepository) FindByUser(ctx context.Context, userID domain.UserID, opts ...repository.QueryOption) ([]*domain.Node, error) {
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
	
	if options.SortOrder == repository.SortOrderAsc {
		input.ScanIndexForward = aws.Bool(true)
	} else {
		input.ScanIndexForward = aws.Bool(false)
	}
	
	result, err := r.client.Query(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to query nodes: %w", err)
	}
	
	nodes := make([]*domain.Node, 0, len(result.Items))
	for _, item := range result.Items {
		// Use custom parsing to handle different data formats
		node, err := r.parseNodeFromItem(item)
		if err != nil {
			r.logger.Warn("Failed to parse node", zap.Error(err))
			continue
		}
		nodes = append(nodes, node)
	}
	
	return nodes, nil
}

// CountByUser counts nodes for a user.
func (r *NodeRepository) CountByUser(ctx context.Context, userID domain.UserID) (int, error) {
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
func (r *NodeRepository) FindByKeywords(ctx context.Context, userID domain.UserID, keywords []string, opts ...repository.QueryOption) ([]*domain.Node, error) {
	// This would typically use a GSI or search service
	// For now, fetch all nodes and filter in memory
	nodes, err := r.FindByUser(ctx, userID, opts...)
	if err != nil {
		return nil, err
	}
	
	// Filter nodes by keywords
	filtered := make([]*domain.Node, 0)
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
func (r *NodeRepository) FindByTags(ctx context.Context, userID domain.UserID, tags []string, opts ...repository.QueryOption) ([]*domain.Node, error) {
	// Similar to keywords, this would use a GSI in production
	nodes, err := r.FindByUser(ctx, userID, opts...)
	if err != nil {
		return nil, err
	}
	
	// Filter nodes by tags
	filtered := make([]*domain.Node, 0)
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
func (r *NodeRepository) FindByContent(ctx context.Context, userID domain.UserID, searchTerm string, fuzzy bool, opts ...repository.QueryOption) ([]*domain.Node, error) {
	// This would typically use a search service like ElasticSearch
	// For now, fetch all nodes and filter in memory
	nodes, err := r.FindByUser(ctx, userID, opts...)
	if err != nil {
		return nil, err
	}
	
	// Filter nodes by content
	filtered := make([]*domain.Node, 0)
	for _, node := range nodes {
		// Check if search term is in content
		if strings.Contains(strings.ToLower(node.Content.String()), strings.ToLower(searchTerm)) {
			filtered = append(filtered, node)
		}
	}
	
	return filtered, nil
}

// FindRecentlyCreated finds nodes created within the specified number of days.
func (r *NodeRepository) FindRecentlyCreated(ctx context.Context, userID domain.UserID, days int, opts ...repository.QueryOption) ([]*domain.Node, error) {
	nodes, err := r.FindByUser(ctx, userID, opts...)
	if err != nil {
		return nil, err
	}
	
	cutoff := time.Now().AddDate(0, 0, -days)
	filtered := make([]*domain.Node, 0)
	for _, node := range nodes {
		if node.CreatedAt.After(cutoff) {
			filtered = append(filtered, node)
		}
	}
	
	return filtered, nil
}

// FindRecentlyUpdated finds nodes updated within the specified number of days.
func (r *NodeRepository) FindRecentlyUpdated(ctx context.Context, userID domain.UserID, days int, opts ...repository.QueryOption) ([]*domain.Node, error) {
	nodes, err := r.FindByUser(ctx, userID, opts...)
	if err != nil {
		return nil, err
	}
	
	cutoff := time.Now().AddDate(0, 0, -days)
	filtered := make([]*domain.Node, 0)
	for _, node := range nodes {
		if node.UpdatedAt.After(cutoff) {
			filtered = append(filtered, node)
		}
	}
	
	return filtered, nil
}

// FindBySpecification finds nodes matching a specification.
func (r *NodeRepository) FindBySpecification(ctx context.Context, spec repository.Specification, opts ...repository.QueryOption) ([]*domain.Node, error) {
	// This would be implemented based on the specification pattern
	// For now, return empty result
	return []*domain.Node{}, nil
}

// CountBySpecification counts nodes matching a specification.
func (r *NodeRepository) CountBySpecification(ctx context.Context, spec repository.Specification) (int, error) {
	return 0, nil
}

// FindPage retrieves a page of nodes.
func (r *NodeRepository) FindPage(ctx context.Context, query repository.NodeQuery, pagination repository.Pagination) (*repository.NodePage, error) {
	userID, err := domain.NewUserID(query.UserID)
	if err != nil {
		return nil, fmt.Errorf("invalid user ID: %w", err)
	}
	
	opts := []repository.QueryOption{
		repository.WithLimit(pagination.Limit),
	}
	
	if pagination.Cursor != "" {
		opts = append(opts, repository.WithCursor(pagination.Cursor))
	}
	
	nodes, err := r.FindByUser(ctx, userID, opts...)
	if err != nil {
		return nil, err
	}
	
	// Generate next cursor if we have a full page
	nextCursor := ""
	if len(nodes) == pagination.Limit && len(nodes) > 0 {
		lastNode := nodes[len(nodes)-1]
		nextCursor = lastNode.ID.String()
	}
	
	return &repository.NodePage{
		Items:      nodes,
		NextCursor: nextCursor,
		HasMore:    nextCursor != "",
	}, nil
}

// FindConnected finds nodes connected to a specific node.
func (r *NodeRepository) FindConnected(ctx context.Context, nodeID domain.NodeID, depth int, opts ...repository.QueryOption) ([]*domain.Node, error) {
	// This would require graph traversal, typically done with a graph database
	// For now, return empty result
	return []*domain.Node{}, nil
}

// FindSimilar finds nodes similar to a specific node.
func (r *NodeRepository) FindSimilar(ctx context.Context, nodeID domain.NodeID, threshold float64, opts ...repository.QueryOption) ([]*domain.Node, error) {
	// This would require similarity calculation, typically done with ML/vector DB
	// For now, return empty result
	return []*domain.Node{}, nil
}

// GetNodesPage is a compatibility method for query service.
func (r *NodeRepository) GetNodesPage(ctx context.Context, query repository.NodeQuery, pagination repository.Pagination) (*repository.NodePage, error) {
	return r.FindPage(ctx, query, pagination)
}

// CountNodes is a compatibility method for query service.
func (r *NodeRepository) CountNodes(ctx context.Context, userID string) (int, error) {
	uid, err := domain.NewUserID(userID)
	if err != nil {
		return 0, err
	}
	return r.CountByUser(ctx, uid)
}

// ============================================================================
// NODE REPOSITORY INTERFACE - Additional Methods for Compatibility
// ============================================================================

// CreateNodeAndKeywords creates a new node with keywords (compatibility method).
func (r *NodeRepository) CreateNodeAndKeywords(ctx context.Context, node *domain.Node) error {
	// Simply delegate to Save which already handles keywords
	return r.Save(ctx, node)
}

// FindNodeByID retrieves a node by user ID and node ID (compatibility method).
func (r *NodeRepository) FindNodeByID(ctx context.Context, userID, nodeID string) (*domain.Node, error) {
	// Add userID to context for the FindByID method
	ctx = sharedContext.WithUserID(ctx, userID)
	
	nid, err := domain.ParseNodeID(nodeID)
	if err != nil {
		return nil, fmt.Errorf("invalid node ID: %w", err)
	}
	
	return r.FindByID(ctx, nid)
}

// FindNodes retrieves nodes based on a query (compatibility method).
func (r *NodeRepository) FindNodes(ctx context.Context, query repository.NodeQuery) ([]*domain.Node, error) {
	// Add userID to context
	ctx = sharedContext.WithUserID(ctx, query.UserID)
	
	userID, err := domain.NewUserID(query.UserID)
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
		filtered := make([]*domain.Node, 0)
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
		filtered := make([]*domain.Node, 0)
		for _, node := range nodes {
			if strings.Contains(strings.ToLower(node.Content.String()), strings.ToLower(query.SearchText)) {
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
	
	nid, err := domain.ParseNodeID(nodeID)
	if err != nil {
		return fmt.Errorf("invalid node ID: %w", err)
	}
	
	return r.Delete(ctx, nid)
}

// GetNodeNeighborhood retrieves the neighborhood graph for a node.
func (r *NodeRepository) GetNodeNeighborhood(ctx context.Context, userID, nodeID string, depth int) (*domain.Graph, error) {
	// This would require graph traversal logic
	// For now, return an empty graph
	return &domain.Graph{
		Nodes: []*domain.Node{},
		Edges: []*domain.Edge{},
	}, nil
}

// FindNodesWithOptions retrieves nodes with query options.
func (r *NodeRepository) FindNodesWithOptions(ctx context.Context, query repository.NodeQuery, opts ...repository.QueryOption) ([]*domain.Node, error) {
	ctx = sharedContext.WithUserID(ctx, query.UserID)
	
	userID, err := domain.NewUserID(query.UserID)
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
		nextCursor = lastNode.ID.String()
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
func (r *NodeRepository) Save(ctx context.Context, node *domain.Node) error {
	// Extract userID from context
	userID, ok := sharedContext.GetUserIDFromContext(ctx)
	if !ok {
		return fmt.Errorf("user ID not found in context")
	}
	
	// Build the item with composite keys
	item := map[string]types.AttributeValue{
		"PK":        &types.AttributeValueMemberS{Value: fmt.Sprintf("USER#%s", userID)},
		"SK":        &types.AttributeValueMemberS{Value: fmt.Sprintf("NODE#%s", node.ID.String())},
		"EntityType": &types.AttributeValueMemberS{Value: "NODE"},
		"NodeID":    &types.AttributeValueMemberS{Value: node.ID.String()},
		"UserID":    &types.AttributeValueMemberS{Value: node.UserID.String()},
		"Content":   &types.AttributeValueMemberS{Value: node.Content.String()},
		"CreatedAt": &types.AttributeValueMemberS{Value: node.CreatedAt.Format(time.RFC3339)},
		"UpdatedAt": &types.AttributeValueMemberS{Value: node.UpdatedAt.Format(time.RFC3339)},
		"Version":   &types.AttributeValueMemberN{Value: fmt.Sprintf("%d", node.Version)},
	}
	
	// Add tags if present
	if node.Tags.Count() > 0 {
		tagSlice := node.Tags.ToSlice()
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
	if node.Metadata != nil {
		metaMap, err := attributevalue.Marshal(node.Metadata)
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
		return fmt.Errorf("failed to save node: %w", err)
	}
	
	return nil
}

// SaveBatch saves multiple nodes in a batch.
func (r *NodeRepository) SaveBatch(ctx context.Context, nodes []*domain.Node) error {
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

func (r *NodeRepository) saveBatch(ctx context.Context, nodes []*domain.Node) error {
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
func (r *NodeRepository) Update(ctx context.Context, node *domain.Node) error {
	// Extract userID from context
	userID, ok := sharedContext.GetUserIDFromContext(ctx)
	if !ok {
		return fmt.Errorf("user ID not found in context")
	}
	
	// Build update expression
	update := expression.Set(expression.Name("Content"), expression.Value(node.Content.String())).
		Set(expression.Name("UpdatedAt"), expression.Value(node.UpdatedAt.Format(time.RFC3339))).
		Set(expression.Name("Version"), expression.Value(node.Version))
	
	// Add tags if present
	if node.Tags.Count() > 0 {
		tags := node.Tags.ToSlice()
		update = update.Set(expression.Name("Tags"), expression.Value(tags))
	}
	
	// Add keywords if present
	keywords := node.Keywords()
	if keywords.Count() > 0 {
		update = update.Set(expression.Name("Keywords"), expression.Value(keywords.ToSlice()))
	}
	
	// Build condition expression for optimistic locking
	condition := expression.Equal(expression.Name("Version"), expression.Value(node.Version-1))
	
	expr, err := expression.NewBuilder().
		WithUpdate(update).
		WithCondition(condition).
		Build()
	if err != nil {
		return fmt.Errorf("failed to build expression: %w", err)
	}
	
	key := map[string]types.AttributeValue{
		"PK": &types.AttributeValueMemberS{Value: fmt.Sprintf("USER#%s", userID)},
		"SK": &types.AttributeValueMemberS{Value: fmt.Sprintf("NODE#%s", node.ID.String())},
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
func (r *NodeRepository) UpdateBatch(ctx context.Context, nodes []*domain.Node) error {
	for _, node := range nodes {
		if err := r.Update(ctx, node); err != nil {
			return err
		}
	}
	return nil
}

// Delete deletes a node.
func (r *NodeRepository) Delete(ctx context.Context, id domain.NodeID) error {
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
func (r *NodeRepository) DeleteBatch(ctx context.Context, ids []domain.NodeID) error {
	for _, id := range ids {
		if err := r.Delete(ctx, id); err != nil {
			return err
		}
	}
	return nil
}

// Archive archives a node (soft delete).
func (r *NodeRepository) Archive(ctx context.Context, id domain.NodeID) error {
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
func (r *NodeRepository) Unarchive(ctx context.Context, id domain.NodeID) error {
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
func (r *NodeRepository) UpdateVersion(ctx context.Context, id domain.NodeID, expectedVersion domain.Version) error {
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
// This handles the case where data might be stored in different formats.
func (r *NodeRepository) parseNodeFromItem(item map[string]types.AttributeValue) (*domain.Node, error) {
	// Extract basic fields from DynamoDB item
	var nodeID, userIDStr, contentStr string
	var version int = 1
	
	// Extract NodeID
	if v, ok := item["NodeID"].(*types.AttributeValueMemberS); ok {
		nodeID = v.Value
	} else if v, ok := item["SK"].(*types.AttributeValueMemberS); ok {
		// Extract from SK if NodeID not directly available
		if strings.HasPrefix(v.Value, "NODE#") {
			nodeID = strings.TrimPrefix(v.Value, "NODE#")
		}
	}
	
	// Extract UserID
	if v, ok := item["UserID"].(*types.AttributeValueMemberS); ok {
		userIDStr = v.Value
	} else if v, ok := item["PK"].(*types.AttributeValueMemberS); ok {
		// Extract from PK if UserID not directly available
		if strings.HasPrefix(v.Value, "USER#") {
			userIDStr = strings.TrimPrefix(v.Value, "USER#")
		}
	}
	
	// Extract Content
	if v, ok := item["Content"].(*types.AttributeValueMemberS); ok {
		contentStr = v.Value
	}
	
	// Extract Version
	if v, ok := item["Version"].(*types.AttributeValueMemberN); ok {
		fmt.Sscanf(v.Value, "%d", &version)
	}
	
	// Parse timestamps
	createdAt := time.Now()
	updatedAt := time.Now()
	
	if v, ok := item["CreatedAt"].(*types.AttributeValueMemberS); ok {
		if t, err := time.Parse(time.RFC3339, v.Value); err == nil {
			createdAt = t
		}
	}
	if v, ok := item["UpdatedAt"].(*types.AttributeValueMemberS); ok {
		if t, err := time.Parse(time.RFC3339, v.Value); err == nil {
			updatedAt = t
		}
	}
	
	// Create domain objects
	nid, err := domain.ParseNodeID(nodeID)
	if err != nil {
		return nil, fmt.Errorf("invalid node ID: %w", err)
	}
	
	uid, err := domain.NewUserID(userIDStr)
	if err != nil {
		return nil, fmt.Errorf("invalid user ID: %w", err)
	}
	
	content, err := domain.NewContent(contentStr)
	if err != nil {
		return nil, fmt.Errorf("invalid content: %w", err)
	}
	
	// Parse Tags
	var tags []string
	if v, ok := item["Tags"].(*types.AttributeValueMemberL); ok {
		for _, tagVal := range v.Value {
			if tagStr, ok := tagVal.(*types.AttributeValueMemberS); ok {
				tags = append(tags, tagStr.Value)
			}
		}
	} else if v, ok := item["Tags"].(*types.AttributeValueMemberSS); ok {
		tags = v.Value
	}
	
	// Parse Keywords
	var keywords []string
	if v, ok := item["Keywords"].(*types.AttributeValueMemberL); ok {
		for _, kwVal := range v.Value {
			if kwStr, ok := kwVal.(*types.AttributeValueMemberS); ok {
				keywords = append(keywords, kwStr.Value)
			}
		}
	} else if v, ok := item["Keywords"].(*types.AttributeValueMemberSS); ok {
		keywords = v.Value
	}
	
	// Reconstruct the node using domain methods
	node := domain.ReconstructNode(
		nid,
		uid,
		content,
		domain.NewKeywords(keywords),
		domain.NewTags(tags...),
		createdAt,
		updatedAt,
		domain.ParseVersion(version),
		false, // archived
	)
	
	return node, nil
}