package dynamodb

import (
	"context"
	"fmt"
	"strings"
	"time"

	"backend/application/ports"
	"backend/domain/core/aggregates"
	"backend/domain/core/entities"
	"backend/domain/core/valueobjects"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"go.uber.org/zap"
)

// NodeRepository implements both the application ports.NodeRepository interface
// and the abstractions.NodeRepositoryAbstraction interface using DynamoDB
type NodeRepository struct {
	*GenericRepository[*NodeEntity]
	gsi2IndexName string // For direct NodeID lookups
}

// Compile-time interface check
var _ ports.NodeRepository = (*NodeRepository)(nil)

// NodeLoader interface implementation for lazy loading
var _ aggregates.NodeLoader = (*NodeRepository)(nil)

// NodeEntity is a wrapper to satisfy the Entity interface
type NodeEntity struct {
	node *entities.Node
}

func (n *NodeEntity) GetID() string {
	return n.node.ID().String()
}

func (n *NodeEntity) GetUserID() string {
	return n.node.UserID()
}

func (n *NodeEntity) GetVersion() int {
	return n.node.Version()
}

// NodeEntityConfig implements EntityConfig for Node entities
type NodeEntityConfig struct{}

func (c *NodeEntityConfig) GetEntityType() string {
	return "NODE"
}

func (c *NodeEntityConfig) BuildKey(graphID, entityID string) map[string]types.AttributeValue {
	// Nodes are now scoped to graphs, not directly to users
	return map[string]types.AttributeValue{
		"PK": &types.AttributeValueMemberS{Value: fmt.Sprintf("GRAPH#%s", graphID)},
		"SK": &types.AttributeValueMemberS{Value: fmt.Sprintf("NODE#%s", entityID)},
	}
}

func (c *NodeEntityConfig) ToItem(entity *NodeEntity) (map[string]types.AttributeValue, error) {
	node := entity.node

	// Build the DynamoDB item - nodes are now scoped to graphs
	graphID := node.GraphID()
	if graphID == "" {
		return nil, fmt.Errorf("node must belong to a graph")
	}

	item := map[string]types.AttributeValue{
		"PK":         &types.AttributeValueMemberS{Value: fmt.Sprintf("GRAPH#%s", graphID)},
		"SK":         &types.AttributeValueMemberS{Value: fmt.Sprintf("NODE#%s", node.ID().String())},
		"EntityType": &types.AttributeValueMemberS{Value: "NODE"},
		"NodeID":     &types.AttributeValueMemberS{Value: node.ID().String()},
		"UserID":     &types.AttributeValueMemberS{Value: node.UserID()},
		"GraphID":    &types.AttributeValueMemberS{Value: node.GraphID()},
		"Title":      &types.AttributeValueMemberS{Value: node.Content().Title()},
		"Content":    &types.AttributeValueMemberS{Value: node.Content().Body()},
		"Format":     &types.AttributeValueMemberS{Value: string(node.Content().Format())},
		"PositionX":  &types.AttributeValueMemberN{Value: fmt.Sprintf("%f", node.Position().X())},
		"PositionY":  &types.AttributeValueMemberN{Value: fmt.Sprintf("%f", node.Position().Y())},
		"PositionZ":  &types.AttributeValueMemberN{Value: fmt.Sprintf("%f", node.Position().Z())},
		"Status":     &types.AttributeValueMemberS{Value: string(node.Status())},
		"Version":    &types.AttributeValueMemberN{Value: fmt.Sprintf("%d", node.Version())},
		"CreatedAt":  &types.AttributeValueMemberS{Value: node.CreatedAt().Format(time.RFC3339)},
		"UpdatedAt":  &types.AttributeValueMemberS{Value: node.UpdatedAt().Format(time.RFC3339)},
	}

	// Add GSI attributes for user-level queries
	item["GSI1PK"] = &types.AttributeValueMemberS{Value: fmt.Sprintf("USER#%s", node.UserID())}
	item["GSI1SK"] = &types.AttributeValueMemberS{Value: fmt.Sprintf("NODE#%s", node.ID().String())}

	// Add GSI2 attributes for direct NodeID lookups (eliminates table scans)
	item["GSI2PK"] = &types.AttributeValueMemberS{Value: fmt.Sprintf("NODE#%s", node.ID().String())}
	item["GSI2SK"] = &types.AttributeValueMemberS{Value: fmt.Sprintf("GRAPH#%s", node.GraphID())}

	// Add tags if present
	tags := node.GetTags()
	if len(tags) > 0 {
		tagList := make([]types.AttributeValue, len(tags))
		for i, tag := range tags {
			tagList[i] = &types.AttributeValueMemberS{Value: tag}
		}
		item["Tags"] = &types.AttributeValueMemberL{Value: tagList}
	}

	// Add connections if present
	connections := node.GetConnections()
	if len(connections) > 0 {
		connList := make([]types.AttributeValue, len(connections))
		for i, conn := range connections {
			connMap := map[string]types.AttributeValue{
				"EdgeID":   &types.AttributeValueMemberS{Value: conn.EdgeID},
				"TargetID": &types.AttributeValueMemberS{Value: conn.TargetID.String()},
				"Type":     &types.AttributeValueMemberS{Value: string(conn.Type)},
			}
			connItem, _ := attributevalue.MarshalMap(connMap)
			connList[i] = &types.AttributeValueMemberM{Value: connItem}
		}
		item["Connections"] = &types.AttributeValueMemberL{Value: connList}
	}

	return item, nil
}

func (c *NodeEntityConfig) ParseItem(item map[string]types.AttributeValue) (*NodeEntity, error) {
	// Extract NodeID from SK
	nodeIDStr := ""
	if v, ok := item["SK"].(*types.AttributeValueMemberS); ok {
		if strings.HasPrefix(v.Value, "NODE#") {
			nodeIDStr = strings.TrimPrefix(v.Value, "NODE#")
		}
	}

	// Extract basic fields
	userID := ""
	if v, ok := item["UserID"].(*types.AttributeValueMemberS); ok {
		userID = v.Value
	}

	title := ""
	if v, ok := item["Title"].(*types.AttributeValueMemberS); ok {
		title = v.Value
	}

	body := ""
	if v, ok := item["Content"].(*types.AttributeValueMemberS); ok {
		body = v.Value
	}

	format := "text"
	if v, ok := item["Format"].(*types.AttributeValueMemberS); ok {
		format = v.Value
	}

	graphID := ""
	if v, ok := item["GraphID"].(*types.AttributeValueMemberS); ok {
		graphID = v.Value
	}

	// Parse position
	var x, y, z float64
	if v, ok := item["PositionX"].(*types.AttributeValueMemberN); ok {
		fmt.Sscanf(v.Value, "%f", &x)
	}
	if v, ok := item["PositionY"].(*types.AttributeValueMemberN); ok {
		fmt.Sscanf(v.Value, "%f", &y)
	}
	if v, ok := item["PositionZ"].(*types.AttributeValueMemberN); ok {
		fmt.Sscanf(v.Value, "%f", &z)
	}

	// Parse timestamps
	var createdAt, updatedAt time.Time
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

	// Parse status
	status := entities.StatusDraft
	if v, ok := item["Status"].(*types.AttributeValueMemberS); ok {
		switch v.Value {
		case "published":
			status = entities.StatusPublished
		case "archived":
			status = entities.StatusArchived
		default:
			status = entities.StatusDraft
		}
	}

	// Create NodeID value object
	nodeID, err := valueobjects.NewNodeIDFromString(nodeIDStr)
	if err != nil {
		return nil, fmt.Errorf("invalid node ID: %w", err)
	}

	// Create value objects
	content, err := valueobjects.NewNodeContent(title, body, valueobjects.ContentFormat(format))
	if err != nil {
		return nil, fmt.Errorf("invalid content: %w", err)
	}

	position, err := valueobjects.NewPosition3D(x, y, z)
	if err != nil {
		return nil, fmt.Errorf("invalid position: %w", err)
	}

	// Reconstruct the node with preserved timestamps
	node, err := entities.ReconstructNode(
		nodeID,
		userID,
		content,
		position,
		graphID,
		createdAt,
		updatedAt,
		status,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to reconstruct node: %w", err)
	}

	// Add tags if present
	if tagsAttr, ok := item["Tags"].(*types.AttributeValueMemberL); ok {
		for _, tagAttr := range tagsAttr.Value {
			if tagStr, ok := tagAttr.(*types.AttributeValueMemberS); ok {
				node.AddTag(tagStr.Value)
			}
		}
	}

	return &NodeEntity{node: node}, nil
}

// NewNodeRepository creates a new node repository
func NewNodeRepository(client *dynamodb.Client, tableName, gsi1IndexName, gsi2IndexName string, logger *zap.Logger) ports.NodeRepository {
	config := &NodeEntityConfig{}
	genericRepo := NewGenericRepository(client, tableName, gsi1IndexName, config, logger)

	return &NodeRepository{
		GenericRepository: genericRepo,
		gsi2IndexName:     gsi2IndexName,
	}
}

// Implementation of ports.NodeRepository interface

func (r *NodeRepository) Save(ctx context.Context, node *entities.Node) error {
	return r.saveNode(ctx, node)
}

// Update updates an existing node
func (r *NodeRepository) Update(ctx context.Context, node *entities.Node) error {
	return r.saveNode(ctx, node)
}

// saveNode is the internal save implementation
func (r *NodeRepository) saveNode(ctx context.Context, node *entities.Node) error {
	// Ensure node has a graph ID
	if node.GraphID() == "" {
		return fmt.Errorf("node must belong to a graph before saving")
	}

	entity := &NodeEntity{node: node}
	// Note: GenericRepository.Save will need to be updated to use GraphID instead of UserID
	// For now, we'll directly save using the client
	config := &NodeEntityConfig{}
	item, err := config.ToItem(entity)
	if err != nil {
		return err
	}

	input := &dynamodb.PutItemInput{
		TableName: aws.String(r.GenericRepository.tableName),
		Item:      item,
	}

	_, err = r.GenericRepository.client.PutItem(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to save node: %w", err)
	}

	r.GenericRepository.logger.Debug("Node saved",
		zap.String("nodeID", node.ID().String()),
		zap.String("graphID", node.GraphID()),
		zap.String("userID", node.UserID()),
	)

	return nil
}

// SaveWithUoW saves a node within a unit of work transaction
func (r *NodeRepository) SaveWithUoW(ctx context.Context, node *entities.Node, uow interface{}) error {
	// Type assert to DynamoDBUnitOfWork
	dynamoUoW, ok := uow.(*DynamoDBUnitOfWork)
	if !ok {
		return fmt.Errorf("invalid unit of work type")
	}

	// Ensure node has a graph ID
	if node.GraphID() == "" {
		return fmt.Errorf("node must belong to a graph before saving")
	}

	entity := &NodeEntity{node: node}
	config := &NodeEntityConfig{}
	item, err := config.ToItem(entity)
	if err != nil {
		return fmt.Errorf("failed to convert node to item: %w", err)
	}

	// Register the save operation with the unit of work
	transactItem := types.TransactWriteItem{
		Put: &types.Put{
			TableName: aws.String(r.GenericRepository.tableName),
			Item:      item,
		},
	}

	if err := dynamoUoW.RegisterSave(transactItem); err != nil {
		return fmt.Errorf("failed to register node save: %w", err)
	}

	// Register any uncommitted events from the node
	for _, event := range node.GetUncommittedEvents() {
		if err := dynamoUoW.RegisterEvent(event); err != nil {
			return fmt.Errorf("failed to register node event: %w", err)
		}
	}

	r.GenericRepository.logger.Debug("Node registered for transactional save",
		zap.String("nodeID", node.ID().String()),
		zap.String("graphID", node.GraphID()),
		zap.String("userID", node.UserID()),
	)

	return nil
}

func (r *NodeRepository) GetByID(ctx context.Context, id valueobjects.NodeID) (*entities.Node, error) {
	// Since nodes are now scoped to graphs, we need to find it by scanning
	// or maintaining a GSI for direct node lookups
	return r.searchForNodeByID(ctx, id)
}

// FindByID retrieves a node by ID (alias for GetByID to satisfy interface)
func (r *NodeRepository) FindByID(ctx context.Context, id valueobjects.NodeID) (*entities.Node, error) {
	return r.GetByID(ctx, id)
}

func (r *NodeRepository) GetByUserID(ctx context.Context, userID string) ([]*entities.Node, error) {
	return r.FindByUserID(ctx, userID)
}

// FindByUserID retrieves all nodes for a user (interface method)
func (r *NodeRepository) FindByUserID(ctx context.Context, userID string) ([]*entities.Node, error) {
	// Query using GSI1 where GSI1PK = USER#userID and GSI1SK begins_with NODE#
	input := &dynamodb.QueryInput{
		TableName:              aws.String(r.GenericRepository.tableName),
		IndexName:              aws.String(r.GenericRepository.indexName),
		KeyConditionExpression: aws.String("GSI1PK = :pk AND begins_with(GSI1SK, :sk)"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":pk": &types.AttributeValueMemberS{Value: fmt.Sprintf("USER#%s", userID)},
			":sk": &types.AttributeValueMemberS{Value: "NODE#"},
		},
	}

	result, err := r.GenericRepository.client.Query(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to query nodes by user: %w", err)
	}

	nodes := make([]*entities.Node, 0, len(result.Items))
	config := &NodeEntityConfig{}
	for _, item := range result.Items {
		entity, err := config.ParseItem(item)
		if err != nil {
			r.GenericRepository.logger.Warn("Failed to unmarshal node", zap.Error(err))
			continue
		}
		nodes = append(nodes, entity.node)
	}

	return nodes, nil
}

func (r *NodeRepository) GetByGraphID(ctx context.Context, graphID string) ([]*entities.Node, error) {
	return r.FindByGraphID(ctx, graphID)
}

// FindByGraphID retrieves all nodes for a graph (interface method)
func (r *NodeRepository) FindByGraphID(ctx context.Context, graphID string) ([]*entities.Node, error) {
	// Now this is a direct query since nodes are scoped to graphs
	input := &dynamodb.QueryInput{
		TableName:              aws.String(r.GenericRepository.tableName),
		KeyConditionExpression: aws.String("PK = :pk AND begins_with(SK, :sk)"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":pk": &types.AttributeValueMemberS{Value: fmt.Sprintf("GRAPH#%s", graphID)},
			":sk": &types.AttributeValueMemberS{Value: "NODE#"},
		},
	}

	result, err := r.GenericRepository.client.Query(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to query nodes by graph: %w", err)
	}

	nodes := make([]*entities.Node, 0, len(result.Items))
	config := &NodeEntityConfig{}
	for _, item := range result.Items {
		entity, err := config.ParseItem(item)
		if err != nil {
			r.GenericRepository.logger.Warn("Failed to unmarshal node", zap.Error(err))
			continue
		}
		nodes = append(nodes, entity.node)
	}

	return nodes, nil
}

func (r *NodeRepository) Delete(ctx context.Context, id valueobjects.NodeID) error {
	// First, find the node to get its graph ID
	node, err := r.searchForNodeByID(ctx, id)
	if err != nil {
		return fmt.Errorf("failed to find node for deletion: %w", err)
	}

	// Delete the node using its graph ID
	key := map[string]types.AttributeValue{
		"PK": &types.AttributeValueMemberS{Value: fmt.Sprintf("GRAPH#%s", node.GraphID())},
		"SK": &types.AttributeValueMemberS{Value: fmt.Sprintf("NODE#%s", id.String())},
	}

	input := &dynamodb.DeleteItemInput{
		TableName: aws.String(r.GenericRepository.tableName),
		Key:       key,
	}

	_, err = r.GenericRepository.client.DeleteItem(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to delete node: %w", err)
	}

	r.GenericRepository.logger.Debug("Node deleted",
		zap.String("nodeID", id.String()),
		zap.String("graphID", node.GraphID()),
	)

	return nil
}

func (r *NodeRepository) Search(ctx context.Context, criteria ports.SearchCriteria) ([]*entities.Node, error) {
	// For now, use GetByUserID as a simple search
	// In a real implementation, you'd build complex queries based on criteria
	return r.GetByUserID(ctx, criteria.UserID)
}

func (r *NodeRepository) BulkSave(ctx context.Context, nodes []*entities.Node) error {
	nodeEntities := make([]*NodeEntity, len(nodes))
	for i, node := range nodes {
		nodeEntities[i] = &NodeEntity{node: node}
	}

	return r.GenericRepository.BatchSave(ctx, nodeEntities)
}

// searchForNodeByID searches for a node by ID using GSI2 for efficient O(1) lookup
func (r *NodeRepository) searchForNodeByID(ctx context.Context, id valueobjects.NodeID) (*entities.Node, error) {
	// Use GSI2 with NODE#nodeId as partition key for O(1) lookup
	// IMPORTANT: Filter by GSI2SK to get the node itself, not edges where this node is the source
	// Nodes have GSI2SK=GRAPH#graphId, while edges have GSI2SK=EDGE#edgeId
	input := &dynamodb.QueryInput{
		TableName:              aws.String(r.GenericRepository.tableName),
		IndexName:              aws.String(r.gsi2IndexName),
		KeyConditionExpression: aws.String("GSI2PK = :pk AND begins_with(GSI2SK, :sk)"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":pk": &types.AttributeValueMemberS{Value: fmt.Sprintf("NODE#%s", id.String())},
			":sk": &types.AttributeValueMemberS{Value: "GRAPH#"}, // Filter for nodes only, not edges
		},
		Limit: aws.Int32(1), // We only expect one node result
	}

	result, err := r.GenericRepository.client.Query(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to query for node: %w", err)
	}

	if len(result.Items) == 0 {
		return nil, fmt.Errorf("node not found: %s", id.String())
	}

	// Parse the node using the config
	entity, err := r.GenericRepository.config.ParseItem(result.Items[0])
	if err != nil {
		return nil, fmt.Errorf("failed to parse node: %w", err)
	}

	return entity.node, nil
}

// CountNodesByGraph counts the number of nodes in a graph
func (r *NodeRepository) CountNodesByGraph(ctx context.Context, graphID string) (int64, error) {
	tableName := r.GenericRepository.tableName
	input := &dynamodb.QueryInput{
		TableName:              &tableName,
		KeyConditionExpression: aws.String("PK = :pk"),
		FilterExpression:       aws.String("EntityType = :type"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":pk":   &types.AttributeValueMemberS{Value: fmt.Sprintf("GRAPH#%s", graphID)},
			":type": &types.AttributeValueMemberS{Value: "NODE"},
		},
		Select: types.SelectCount,
	}

	result, err := r.GenericRepository.client.Query(ctx, input)
	if err != nil {
		return 0, fmt.Errorf("failed to count nodes: %w", err)
	}

	return int64(result.Count), nil
}

// FindSimilarNodes finds nodes similar to the given node
func (r *NodeRepository) FindSimilarNodes(ctx context.Context, nodeID valueobjects.NodeID, threshold float64) ([]*entities.Node, error) {
	// This is a simplified implementation
	// In production, you'd use a similarity algorithm or vector database
	node, err := r.GetByID(ctx, nodeID)
	if err != nil {
		return nil, fmt.Errorf("failed to find source node: %w", err)
	}

	// For now, return nodes with similar tags
	tags := node.GetTags()
	if len(tags) == 0 {
		return []*entities.Node{}, nil
	}

	return r.FindByTags(ctx, node.UserID(), tags)
}

// FindOrphanedNodes finds nodes that have no edges
func (r *NodeRepository) FindOrphanedNodes(ctx context.Context, graphID string) ([]*entities.Node, error) {
	// Get all nodes in the graph
	nodes, err := r.GetByGraphID(ctx, graphID)
	if err != nil {
		return nil, err
	}

	// Filter for nodes with no connections
	orphaned := make([]*entities.Node, 0)
	for _, node := range nodes {
		if len(node.GetConnections()) == 0 {
			orphaned = append(orphaned, node)
		}
	}

	return orphaned, nil
}

// FindByTags finds nodes by their tags
func (r *NodeRepository) FindByTags(ctx context.Context, userID string, tags []string) ([]*entities.Node, error) {
	if len(tags) == 0 {
		return []*entities.Node{}, nil
	}

	tableName := r.GenericRepository.tableName
	indexName := r.GenericRepository.indexName

	// Use GSI1 to query by user first, then filter by tags
	filterParts := make([]string, len(tags))
	expAttrValues := map[string]types.AttributeValue{
		":pk": &types.AttributeValueMemberS{Value: fmt.Sprintf("USER#%s", userID)},
		":sk": &types.AttributeValueMemberS{Value: "NODE#"},
	}

	for i, tag := range tags {
		key := fmt.Sprintf(":tag%d", i)
		filterParts[i] = fmt.Sprintf("contains(Tags, %s)", key)
		expAttrValues[key] = &types.AttributeValueMemberS{Value: tag}
	}

	filterExpr := strings.Join(filterParts, " OR ")

	input := &dynamodb.QueryInput{
		TableName:                 &tableName,
		IndexName:                 &indexName,
		KeyConditionExpression:    aws.String("GSI1PK = :pk AND begins_with(GSI1SK, :sk)"),
		FilterExpression:          aws.String(filterExpr),
		ExpressionAttributeValues: expAttrValues,
	}

	result, err := r.GenericRepository.client.Query(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to find nodes by tags: %w", err)
	}

	nodes := make([]*entities.Node, 0, len(result.Items))
	for _, item := range result.Items {
		entity, err := r.GenericRepository.config.ParseItem(item)
		if err != nil {
			continue
		}
		nodes = append(nodes, entity.node)
	}

	return nodes, nil
}

// SearchByContent searches nodes by content
// TODO: Replace with ElasticSearch or OpenSearch for production use
// Current implementation uses Scan which is inefficient for large datasets
func (r *NodeRepository) SearchByContent(ctx context.Context, query string, limit int) ([]*entities.Node, error) {
	if query == "" {
		return []*entities.Node{}, nil
	}

	tableName := r.GenericRepository.tableName

	// WARNING: This uses Scan operation - inefficient for large tables
	// In production, index content in ElasticSearch/OpenSearch
	input := &dynamodb.ScanInput{
		TableName:        &tableName,
		FilterExpression: aws.String("EntityType = :type AND (contains(Title, :query) OR contains(Content, :query))"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":type":  &types.AttributeValueMemberS{Value: "NODE"},
			":query": &types.AttributeValueMemberS{Value: query},
		},
	}

	if limit > 0 {
		limitInt32 := int32(limit)
		input.Limit = &limitInt32
	}

	result, err := r.GenericRepository.client.Scan(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to search nodes: %w", err)
	}

	nodes := make([]*entities.Node, 0, len(result.Items))
	for _, item := range result.Items {
		entity, err := r.GenericRepository.config.ParseItem(item)
		if err != nil {
			continue
		}
		nodes = append(nodes, entity.node)
	}

	return nodes, nil
}

// SaveBatch saves multiple nodes in a batch using the improved generic batch operation
func (r *NodeRepository) SaveBatch(ctx context.Context, nodes []*entities.Node) error {
	if len(nodes) == 0 {
		return nil
	}

	// Convert nodes to entities for the generic batch save
	entities := make([]*NodeEntity, len(nodes))
	for i, node := range nodes {
		// Ensure each node has a GraphID before saving
		if node.GraphID() == "" {
			return fmt.Errorf("node %s must belong to a graph before batch saving", node.ID().String())
		}
		entities[i] = &NodeEntity{node: node}
	}

	// Use the improved generic BatchSave with retry logic and error handling
	return r.GenericRepository.BatchSave(ctx, entities)
}

// DeleteBatch deletes multiple nodes in a batch using improved batch operations
func (r *NodeRepository) DeleteBatch(ctx context.Context, nodeIDs []valueobjects.NodeID) error {
	if len(nodeIDs) == 0 {
		return nil
	}

	// First, we need to find each node to get their GraphIDs for proper key construction
	// This is necessary because DynamoDB requires the full key (PK + SK) for deletion
	keys := make([]map[string]types.AttributeValue, 0, len(nodeIDs))
	notFoundNodes := make([]string, 0)

	for _, nodeID := range nodeIDs {
		// Use the GSI2 lookup to find the node and get its GraphID
		node, err := r.searchForNodeByID(ctx, nodeID)
		if err != nil {
			r.GenericRepository.logger.Warn("Node not found for batch delete",
				zap.String("nodeID", nodeID.String()),
				zap.Error(err),
			)
			notFoundNodes = append(notFoundNodes, nodeID.String())
			continue
		}

		// Construct the key for deletion
		key := map[string]types.AttributeValue{
			"PK": &types.AttributeValueMemberS{Value: fmt.Sprintf("GRAPH#%s", node.GraphID())},
			"SK": &types.AttributeValueMemberS{Value: fmt.Sprintf("NODE#%s", nodeID.String())},
		}
		keys = append(keys, key)
	}

	if len(notFoundNodes) > 0 {
		r.GenericRepository.logger.Warn("Some nodes were not found during batch delete",
			zap.Strings("notFoundNodes", notFoundNodes),
		)
	}

	if len(keys) == 0 {
		return fmt.Errorf("no valid nodes found for batch delete")
	}

	// Use the improved generic BatchDelete with retry logic
	if err := r.GenericRepository.BatchDelete(ctx, keys); err != nil {
		return fmt.Errorf("failed to batch delete nodes: %w", err)
	}

	r.GenericRepository.logger.Info("Batch deleted nodes successfully",
		zap.Int("requestedCount", len(nodeIDs)),
		zap.Int("deletedCount", len(keys)),
		zap.Int("notFoundCount", len(notFoundNodes)),
	)

	return nil
}

// GetConnectedNodes gets all nodes connected to the given node
func (r *NodeRepository) GetConnectedNodes(ctx context.Context, nodeID valueobjects.NodeID) ([]*entities.Node, error) {
	// First get the node
	node, err := r.GetByID(ctx, nodeID)
	if err != nil {
		return nil, fmt.Errorf("failed to find node: %w", err)
	}

	// Get connected node IDs
	connections := node.GetConnections()
	if len(connections) == 0 {
		return []*entities.Node{}, nil
	}

	// Fetch each connected node
	connectedNodes := make([]*entities.Node, 0, len(connections))
	for _, conn := range connections {
		connNodeID := conn.TargetID

		connNode, err := r.GetByID(ctx, connNodeID)
		if err != nil {
			// Skip nodes that can't be found
			continue
		}

		connectedNodes = append(connectedNodes, connNode)
	}

	return connectedNodes, nil
}

// NodeLoader interface implementation for lazy loading

// LoadNode implements aggregates.NodeLoader interface - loads a single node
func (r *NodeRepository) LoadNode(ctx context.Context, nodeID valueobjects.NodeID) (*entities.Node, error) {
	// Simply delegate to existing GetByID method
	return r.GetByID(ctx, nodeID)
}

// LoadNodes implements aggregates.NodeLoader interface - loads multiple nodes
func (r *NodeRepository) LoadNodes(ctx context.Context, nodeIDs []valueobjects.NodeID) ([]*entities.Node, error) {
	if len(nodeIDs) == 0 {
		return []*entities.Node{}, nil
	}

	// Load nodes one by one (can be optimized with batch get if needed)
	nodes := make([]*entities.Node, 0, len(nodeIDs))
	
	for _, nodeID := range nodeIDs {
		node, err := r.GetByID(ctx, nodeID)
		if err != nil {
			r.logger.Warn("Failed to load node in batch", 
				zap.String("nodeID", nodeID.String()),
				zap.Error(err))
			// Continue loading other nodes even if some fail
			continue
		}
		nodes = append(nodes, node)
	}
	
	return nodes, nil
}
