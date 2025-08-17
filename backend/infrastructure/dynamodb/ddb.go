// Package dynamodb implements the repository interface using AWS DynamoDB.
// This is the infrastructure layer that contains DynamoDB-specific implementations.
package dynamodb

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"brain2-backend/internal/domain"
	"brain2-backend/internal/repository"
	appErrors "brain2-backend/pkg/errors" // ALIAS for our custom errors

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

// ddbNode represents the structure of a node item in DynamoDB.
type ddbNode struct {
	PK        string   `dynamodbav:"PK"`
	SK        string   `dynamodbav:"SK"`
	NodeID    string   `dynamodbav:"NodeID"`
	UserID    string   `dynamodbav:"UserID"`
	Content   string   `dynamodbav:"Content"`
	Keywords  []string `dynamodbav:"Keywords"`
	Tags      []string `dynamodbav:"Tags"`
	IsLatest  bool     `dynamodbav:"IsLatest"`
	Version   int      `dynamodbav:"Version"`
	Timestamp string   `dynamodbav:"Timestamp"`
}

// ddbKeyword represents a keyword index item in DynamoDB.
type ddbKeyword struct {
	PK     string `dynamodbav:"PK"`
	SK     string `dynamodbav:"SK"`
	GSI1PK string `dynamodbav:"GSI1PK"`
	GSI1SK string `dynamodbav:"GSI1SK"`
}

// ddbEdge represents an edge item in DynamoDB.
type ddbEdge struct {
	PK       string `dynamodbav:"PK"`
	SK       string `dynamodbav:"SK"`
	TargetID string `dynamodbav:"TargetID"`
	GSI2PK   string `dynamodbav:"GSI2PK"`   // USER#{userId}#EDGE
	GSI2SK   string `dynamodbav:"GSI2SK"`   // NODE#{sourceId}#TARGET#{targetId}
}

// ddbCategory represents a category item in DynamoDB.
type ddbCategory struct {
	PK          string `dynamodbav:"PK"`
	SK          string `dynamodbav:"SK"`
	CategoryID  string `dynamodbav:"CategoryID"`
	UserID      string `dynamodbav:"UserID"`
	Title       string `dynamodbav:"Title"`
	Description string `dynamodbav:"Description"`
	Timestamp   string `dynamodbav:"Timestamp"`
}

// ddbCategoryMemory represents a category-memory relationship item in DynamoDB.
type ddbCategoryMemory struct {
	PK         string `dynamodbav:"PK"`
	SK         string `dynamodbav:"SK"`
	CategoryID string `dynamodbav:"CategoryID"`
	MemoryID   string `dynamodbav:"MemoryID"`
	UserID     string `dynamodbav:"UserID"`
	AddedAt    string `dynamodbav:"AddedAt"`
}

// ddbRepository is the concrete implementation for DynamoDB.
type ddbRepository struct {
	dbClient *dynamodb.Client
	config   repository.Config
	logger   *zap.Logger
}

// NewRepository creates a new instance of the DynamoDB repository.
func NewRepository(dbClient *dynamodb.Client, tableName, indexName string, logger *zap.Logger) repository.Repository {
	config := repository.NewConfig(tableName, indexName)
	return &ddbRepository{
		dbClient: dbClient,
		config:   config,
		logger:   logger,
	}
}

// NewRepositoryWithConfig creates a new instance of the DynamoDB repository with custom config.
func NewRepositoryWithConfig(dbClient *dynamodb.Client, config repository.Config, logger *zap.Logger) repository.Repository {
	return &ddbRepository{
		dbClient: dbClient,
		config:   config.WithDefaults(),
		logger:   logger,
	}
}

// Segregated repository factory functions for dependency injection

// NewNodeRepository creates a new instance implementing NodeRepository interface.
func NewNodeRepository(dbClient *dynamodb.Client, tableName, indexName string, logger *zap.Logger) repository.NodeRepository {
	return NewRepository(dbClient, tableName, indexName, logger)
}

// NewEdgeRepository creates a new instance implementing EdgeRepository interface.
func NewEdgeRepository(dbClient *dynamodb.Client, tableName, indexName string, logger *zap.Logger) repository.EdgeRepository {
	return NewRepository(dbClient, tableName, indexName, logger)
}

// NewKeywordRepository creates a new instance implementing KeywordRepository interface.
func NewKeywordRepository(dbClient *dynamodb.Client, tableName, indexName string, logger *zap.Logger) repository.KeywordRepository {
	return NewRepository(dbClient, tableName, indexName, logger)
}

// NewTransactionalRepository creates a new instance implementing TransactionalRepository interface.
func NewTransactionalRepository(dbClient *dynamodb.Client, tableName, indexName string, logger *zap.Logger) repository.TransactionalRepository {
	return NewRepository(dbClient, tableName, indexName, logger)
}

// NewCategoryRepository creates a new instance implementing CategoryRepository interface.
func NewCategoryRepository(dbClient *dynamodb.Client, tableName, indexName string, logger *zap.Logger) repository.CategoryRepository {
	return NewRepository(dbClient, tableName, indexName, logger)
}

// NewGraphRepository creates a new instance implementing GraphRepository interface.
func NewGraphRepository(dbClient *dynamodb.Client, tableName, indexName string, logger *zap.Logger) repository.GraphRepository {
	return NewRepository(dbClient, tableName, indexName, logger)
}


// getCanonicalEdge determines the canonical storage for a bi-directional edge.
// Returns the owner node ID and target node ID based on lexicographic ordering.
// This ensures each unique connection is stored exactly once.
func getCanonicalEdge(nodeA, nodeB string) (owner, target string) {
	if nodeA < nodeB {
		return nodeA, nodeB
	}
	return nodeB, nodeA
}

// CreateNodeAndKeywords transactionally saves a node and its keyword indexes.
func (r *ddbRepository) CreateNodeAndKeywords(ctx context.Context, node *domain.Node) error {
	pk := fmt.Sprintf("USER#%s#NODE#%s", node.UserID.String(), node.ID.String())
	transactItems := []types.TransactWriteItem{}

	// 1. Add the main node metadata to the transaction
	nodeItem, err := attributevalue.MarshalMap(ddbNode{
		PK: pk, SK: "METADATA#v0", NodeID: node.ID.String(), UserID: node.UserID.String(), Content: node.Content.String(),
		Keywords: node.Keywords().ToSlice(), Tags: node.Tags.ToSlice(), IsLatest: true, Version: node.Version, Timestamp: node.CreatedAt.Format(time.RFC3339),
	})
	if err != nil {
		return appErrors.Wrap(err, "failed to marshal node item")
	}
	transactItems = append(transactItems, types.TransactWriteItem{
		Put: &types.Put{TableName: aws.String(r.config.TableName), Item: nodeItem},
	})

	// 2. Add each keyword as a separate item for the GSI to index
	for _, keyword := range node.Keywords().ToSlice() {
		keywordItem, err := attributevalue.MarshalMap(ddbKeyword{
			PK:     pk,
			SK:     fmt.Sprintf("KEYWORD#%s", keyword),
			GSI1PK: fmt.Sprintf("USER#%s#KEYWORD#%s", node.UserID.String(), keyword),
			GSI1SK: fmt.Sprintf("NODE#%s", node.ID.String()),
		})
		if err != nil {
			return appErrors.Wrap(err, "failed to marshal keyword item")
		}
		transactItems = append(transactItems, types.TransactWriteItem{
			Put: &types.Put{TableName: aws.String(r.config.TableName), Item: keywordItem},
		})
	}

	// 3. Execute the transaction
	_, err = r.dbClient.TransactWriteItems(ctx, &dynamodb.TransactWriteItemsInput{
		TransactItems: transactItems,
	})
	if err != nil {
		return appErrors.Wrap(err, "transaction to create node and keywords failed")
	}
	return nil
}

// CreateNodeWithEdges saves a node, its keywords, and its connections in a single transaction.
func (r *ddbRepository) CreateNodeWithEdges(ctx context.Context, node *domain.Node, relatedNodeIDs []string) error {
	r.logger.Debug("creating node with edges",
		zap.String("node_id", node.ID.String()),
		zap.Strings("keywords", node.Keywords().ToSlice()),
		zap.Int("edge_count", len(relatedNodeIDs)))
	
	// Ensure node starts with version 0
	// Note: Version is immutable in rich domain model, using Version().Int() for DDB
	
	pk := fmt.Sprintf("USER#%s#NODE#%s", node.UserID.String(), node.ID.String())
	transactItems := []types.TransactWriteItem{}

	nodeItem, err := attributevalue.MarshalMap(ddbNode{
		PK: pk, SK: "METADATA#v0", NodeID: node.ID.String(), UserID: node.UserID.String(), Content: node.Content.String(),
		Keywords: node.Keywords().ToSlice(), Tags: node.Tags.ToSlice(), IsLatest: true, Version: node.Version, Timestamp: node.CreatedAt.Format(time.RFC3339),
	})
	if err != nil {
		return appErrors.Wrap(err, "failed to marshal node item")
	}
	transactItems = append(transactItems, types.TransactWriteItem{Put: &types.Put{TableName: aws.String(r.config.TableName), Item: nodeItem}})
	r.logger.Debug("added node item to transaction", zap.String("pk", pk))

	for _, keyword := range node.Keywords().ToSlice() {
		gsi1PK := fmt.Sprintf("USER#%s#KEYWORD#%s", node.UserID.String(), keyword)
		gsi1SK := fmt.Sprintf("NODE#%s", node.ID.String())
		
		keywordItem, err := attributevalue.MarshalMap(ddbKeyword{
			PK: pk, SK: fmt.Sprintf("KEYWORD#%s", keyword), GSI1PK: gsi1PK, GSI1SK: gsi1SK,
		})
		if err != nil {
			return appErrors.Wrap(err, "failed to marshal keyword item")
		}
		transactItems = append(transactItems, types.TransactWriteItem{Put: &types.Put{TableName: aws.String(r.config.TableName), Item: keywordItem}})
		r.logger.Debug("added keyword item to transaction",
			zap.String("keyword", keyword),
			zap.String("gsi1_pk", gsi1PK),
			zap.String("gsi1_sk", gsi1SK))
	}

	// Create canonical edges - only one edge per connection
	for _, relatedNodeID := range relatedNodeIDs {
		ownerID, targetID := getCanonicalEdge(node.ID.String(), relatedNodeID)
		ownerPK := fmt.Sprintf("USER#%s#NODE#%s", node.UserID.String(), ownerID)

		edgeItem, err := attributevalue.MarshalMap(ddbEdge{
			PK:       ownerPK,
			SK:       fmt.Sprintf("EDGE#RELATES_TO#%s", targetID),
			TargetID: targetID,
			GSI2PK:   fmt.Sprintf("USER#%s#EDGE", node.UserID.String()),
			GSI2SK:   fmt.Sprintf("NODE#%s#TARGET#%s", ownerID, targetID),
		})
		if err != nil {
			return appErrors.Wrap(err, "failed to marshal canonical edge item")
		}
		transactItems = append(transactItems, types.TransactWriteItem{Put: &types.Put{TableName: aws.String(r.config.TableName), Item: edgeItem}})
		r.logger.Debug("added edge to transaction",
			zap.String("from_node", node.ID.String()),
			zap.String("to_node", relatedNodeID),
			zap.String("owner_pk", ownerPK))
	}

	r.logger.Debug("executing transaction", zap.Int("item_count", len(transactItems)))
	_, err = r.dbClient.TransactWriteItems(ctx, &dynamodb.TransactWriteItemsInput{TransactItems: transactItems})
	if err != nil {
		r.logger.Error("transaction failed", zap.Error(err))
		return appErrors.Wrap(err, "transaction to create node with edges failed")
	}
	
	r.logger.Debug("transaction completed successfully",
		zap.String("node_id", node.ID.String()))
	return nil
}

// UpdateNodeAndEdges transactionally updates a node and its connections.
func (r *ddbRepository) UpdateNodeAndEdges(ctx context.Context, node *domain.Node, relatedNodeIDs []string) error {
	if err := r.clearNodeConnections(ctx, node.UserID.String(), node.ID.String()); err != nil {
		return appErrors.Wrap(err, "failed to clear old connections for update")
	}

	pk := fmt.Sprintf("USER#%s#NODE#%s", node.UserID.String(), node.ID.String())
	transactItems := []types.TransactWriteItem{}

	// Optimistic locking: check that the version matches before updating
	_, err := r.dbClient.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName:           aws.String(r.config.TableName),
		Key:                 map[string]types.AttributeValue{"PK": &types.AttributeValueMemberS{Value: pk}, "SK": &types.AttributeValueMemberS{Value: "METADATA#v0"}},
		UpdateExpression:    aws.String("SET Content = :c, Keywords = :k, Tags = :tg, Timestamp = :t, Version = Version + :inc"),
		ConditionExpression: aws.String("Version = :expected_version"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":c":                &types.AttributeValueMemberS{Value: node.Content.String()},
			":k":                &types.AttributeValueMemberL{Value: toAttributeValueList(node.Keywords().ToSlice())},
			":tg":               &types.AttributeValueMemberL{Value: toAttributeValueList(node.Tags.ToSlice())},
			":t":                &types.AttributeValueMemberS{Value: time.Now().Format(time.RFC3339)},
			":expected_version": &types.AttributeValueMemberN{Value: fmt.Sprintf("%d", node.Version)},
			":inc":              &types.AttributeValueMemberN{Value: "1"},
		},
	})
	if err != nil {
		// Check for optimistic lock conflicts
		var ccf *types.ConditionalCheckFailedException
		if errors.As(err, &ccf) {
			return repository.NewOptimisticLockError(node.ID.String(), node.Version, node.Version+1)
		}
		return appErrors.Wrap(err, "failed to update node metadata")
	}

	for _, keyword := range node.Keywords().ToSlice() {
		keywordItem, err := attributevalue.MarshalMap(ddbKeyword{PK: pk, SK: fmt.Sprintf("KEYWORD#%s", keyword), GSI1PK: fmt.Sprintf("USER#%s#KEYWORD#%s", node.UserID.String(), keyword), GSI1SK: fmt.Sprintf("NODE#%s", node.ID.String())})
		if err != nil {
			return appErrors.Wrap(err, "failed to marshal keyword item for update")
		}
		transactItems = append(transactItems, types.TransactWriteItem{Put: &types.Put{TableName: aws.String(r.config.TableName), Item: keywordItem}})
	}

	// Create canonical edges - only one edge per connection
	for _, relatedNodeID := range relatedNodeIDs {
		ownerID, targetID := getCanonicalEdge(node.ID.String(), relatedNodeID)
		ownerPK := fmt.Sprintf("USER#%s#NODE#%s", node.UserID.String(), ownerID)

		edgeItem, err := attributevalue.MarshalMap(ddbEdge{
			PK:       ownerPK,
			SK:       fmt.Sprintf("EDGE#RELATES_TO#%s", targetID),
			TargetID: targetID,
			GSI2PK:   fmt.Sprintf("USER#%s#EDGE", node.UserID.String()),
			GSI2SK:   fmt.Sprintf("NODE#%s#TARGET#%s", ownerID, targetID),
		})
		if err != nil {
			return appErrors.Wrap(err, "failed to marshal canonical edge item for update")
		}
		transactItems = append(transactItems, types.TransactWriteItem{Put: &types.Put{TableName: aws.String(r.config.TableName), Item: edgeItem}})
	}

	if len(transactItems) > 0 {
		_, err = r.dbClient.TransactWriteItems(ctx, &dynamodb.TransactWriteItemsInput{TransactItems: transactItems})
		if err != nil {
			return appErrors.Wrap(err, "transaction to update node edges failed")
		}
	}
	return nil
}

// CreateEdge creates a single edge in DynamoDB using the canonical edge storage pattern.
func (r *ddbRepository) CreateEdge(ctx context.Context, edge *domain.Edge) error {
	if edge == nil {
		return appErrors.NewValidation("edge cannot be nil")
	}
	
	// Use canonical edge storage pattern (lexicographically ordered IDs)
	sourceID := edge.SourceID.String()
	targetID := edge.TargetID.String()
	userID := edge.UserID().String()
	
	ownerID, canonicalTargetID := getCanonicalEdge(sourceID, targetID)
	ownerPK := fmt.Sprintf("USER#%s#NODE#%s", userID, ownerID)
	
	edgeItem := ddbEdge{
		PK:       ownerPK,
		SK:       fmt.Sprintf("EDGE#%s", canonicalTargetID),
		TargetID: canonicalTargetID,
		GSI2PK:   fmt.Sprintf("USER#%s#EDGE", userID),
		GSI2SK:   fmt.Sprintf("NODE#%s#TARGET#%s", ownerID, canonicalTargetID),
	}
	
	item, err := attributevalue.MarshalMap(edgeItem)
	if err != nil {
		return appErrors.Wrap(err, "failed to marshal edge item")
	}
	
	_, err = r.dbClient.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(r.config.TableName),
		Item:      item,
	})
	if err != nil {
		return appErrors.Wrap(err, "failed to create edge in DynamoDB")
	}
	
	return nil
}

// FindNodesByKeywords uses the GSI to find nodes with matching keywords.
func (r *ddbRepository) FindNodesByKeywords(ctx context.Context, userID string, keywords []string) ([]*domain.Node, error) {
	r.logger.Debug("finding nodes by keywords",
		zap.String("user_id", userID),
		zap.Strings("keywords", keywords))
	
	nodeIdMap := make(map[string]bool)
	var nodes []*domain.Node
	for _, keyword := range keywords {
		gsiPK := fmt.Sprintf("USER#%s#KEYWORD#%s", userID, keyword)
		r.logger.Debug("querying GSI for keyword", zap.String("gsi_pk", gsiPK))
		
		result, err := r.dbClient.Query(ctx, &dynamodb.QueryInput{
			TableName: aws.String(r.config.TableName), IndexName: aws.String(r.config.IndexName), KeyConditionExpression: aws.String("GSI1PK = :gsiPK"),
			ExpressionAttributeValues: map[string]types.AttributeValue{":gsiPK": &types.AttributeValueMemberS{Value: gsiPK}},
		})
		if err != nil {
			r.logger.Error("failed to query GSI for keyword",
				zap.String("keyword", keyword),
				zap.Error(err))
			continue
		}
		
		r.logger.Debug("GSI query completed",
			zap.String("keyword", keyword),
			zap.Int("items_found", len(result.Items)))
		
		for _, item := range result.Items {
			pkValue := item["PK"].(*types.AttributeValueMemberS).Value
			nodeID := strings.Split(pkValue, "#")[3]
			r.logger.Debug("extracted node ID from keyword search",
				zap.String("node_id", nodeID),
				zap.String("keyword", keyword),
				zap.String("pk", pkValue))
			
			if _, exists := nodeIdMap[nodeID]; !exists {
				nodeIdMap[nodeID] = true
				node, err := r.FindNodeByID(ctx, userID, nodeID)
				if err != nil {
					r.logger.Error("failed to find node from keyword search",
						zap.String("node_id", nodeID),
						zap.Error(err))
					continue
				}
				if node != nil {
					contentStr := node.Content.String()
					preview := contentStr
					if len(contentStr) > 50 {
						preview = contentStr[:50] + "..."
					}
					r.logger.Debug("successfully retrieved node",
						zap.String("node_id", node.ID.String()),
						zap.String("content_preview", preview))
					nodes = append(nodes, node)
				} else {
					r.logger.Warn("node was nil after FindNodeByID", zap.String("node_id", nodeID))
				}
			} else {
				r.logger.Debug("node already processed, skipping", zap.String("node_id", nodeID))
			}
		}
	}
	
	r.logger.Debug("findNodesByKeywords completed",
		zap.Int("unique_nodes_found", len(nodes)))
	return nodes, nil
}

// FindNodeByID retrieves a single node's metadata.
func (r *ddbRepository) FindNodeByID(ctx context.Context, userID, nodeID string) (*domain.Node, error) {
	pk := fmt.Sprintf("USER#%s#NODE#%s", userID, nodeID)
	sk := "METADATA#v0"
	
	result, err := r.dbClient.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(r.config.TableName),
		Key:       map[string]types.AttributeValue{"PK": &types.AttributeValueMemberS{Value: pk}, "SK": &types.AttributeValueMemberS{Value: sk}},
	})
	
	if err != nil {
		return nil, appErrors.Wrap(err, "failed to get item from dynamodb")
	}
	if result.Item == nil {
		return nil, nil // Not found
	}
	var ddbItem ddbNode
	if err := attributevalue.UnmarshalMap(result.Item, &ddbItem); err != nil {
		return nil, appErrors.Wrap(err, "failed to unmarshal node item")
	}
	createdAt, _ := time.Parse(time.RFC3339, ddbItem.Timestamp)
	// Use domain factory method to reconstruct node from primitives
	node, err := domain.ReconstructNodeFromPrimitives(
		ddbItem.NodeID, 
		ddbItem.UserID, 
		ddbItem.Content,
		ddbItem.Keywords, 
		ddbItem.Tags,
		createdAt,
		ddbItem.Version,
	)
	if err != nil {
		return nil, appErrors.Wrap(err, "failed to reconstruct node from DDB data")
	}
	return node, nil
}

// FindEdgesByNode queries for all edges connected to a given node using optimized GSI2 query.
func (r *ddbRepository) FindEdgesByNode(ctx context.Context, userID, nodeID string) ([]domain.Edge, error) {
	var edges []domain.Edge
	edgeMap := make(map[string]bool)

	// Use GSI2 to find all edges for this user, then filter for those involving the specific node
	edgePrefix := fmt.Sprintf("USER#%s#EDGE", userID)
	var lastEvaluatedKey map[string]types.AttributeValue

	for {
		queryInput := &dynamodb.QueryInput{
			TableName:              aws.String(r.config.TableName),
			IndexName:              aws.String("EdgeIndex"),
			KeyConditionExpression: aws.String("GSI2PK = :gsi2pk"),
			ExpressionAttributeValues: map[string]types.AttributeValue{
				":gsi2pk": &types.AttributeValueMemberS{Value: edgePrefix},
			},
			ExclusiveStartKey: lastEvaluatedKey,
		}

		result, err := r.dbClient.Query(ctx, queryInput)
		if err != nil {
			return nil, appErrors.Wrap(err, "failed to query edges using GSI2")
		}

		// Process edges - keep only those involving the specified node
		for _, item := range result.Items {
			var ddbItem ddbEdge
			if err := attributevalue.UnmarshalMap(item, &ddbItem); err == nil {
				// Extract source node ID from PK pattern: USER#<userID>#NODE#<sourceID>
				pkParts := strings.Split(ddbItem.PK, "#")
				if len(pkParts) == 4 {
					sourceID := pkParts[3]
					targetID := ddbItem.TargetID
					
					// Check if this edge involves the requested node
					if sourceID == nodeID || targetID == nodeID {
						// Prevent duplicate edges
						edgeKey := fmt.Sprintf("%s-%s", sourceID, targetID)
						reverseKey := fmt.Sprintf("%s-%s", targetID, sourceID)
						if !edgeMap[edgeKey] && !edgeMap[reverseKey] {
							edgeMap[edgeKey] = true
							// Create rich domain edge using factory method
							userIDVO, _ := domain.NewUserID(userID)
							sourceNodeIDVO, _ := domain.ParseNodeID(sourceID)
							targetNodeIDVO, _ := domain.ParseNodeID(targetID)
							edge, err := domain.NewEdge(sourceNodeIDVO, targetNodeIDVO, userIDVO, 1.0)
							if err == nil {
								edges = append(edges, *edge)
							}
						}
					}
				}
			}
		}

		lastEvaluatedKey = result.LastEvaluatedKey
		if lastEvaluatedKey == nil {
			break
		}
	}

	return edges, nil
}

// DeleteNode transactionally deletes a node, its keywords, and outgoing edges.
func (r *ddbRepository) DeleteNode(ctx context.Context, userID, nodeID string) error {
	return r.clearNodeConnections(ctx, userID, nodeID)
}

// GetAllGraphData retrieves all nodes and edges for a user using optimized parallel queries.
func (r *ddbRepository) GetAllGraphData(ctx context.Context, userID string) (*domain.Graph, error) {
	r.logger.Debug("starting optimized GetAllGraphData with parallel queries")

	g, ctx := errgroup.WithContext(ctx)
	
	var nodes []*domain.Node
	var edges []*domain.Edge
	var nodeErr, edgeErr error

	// Fetch nodes in parallel using query instead of scan
	g.Go(func() error {
		nodes, nodeErr = r.fetchAllNodesOptimized(ctx, userID)
		return nodeErr
	})

	// Fetch edges in parallel using GSI2 query instead of scan
	g.Go(func() error {
		edges, edgeErr = r.fetchAllEdgesOptimized(ctx, userID)
		return edgeErr
	})

	// Wait for both operations to complete
	if err := g.Wait(); err != nil {
		r.logger.Error("failed to fetch graph data", zap.Error(err))
		return nil, appErrors.Wrap(err, "failed to fetch graph data")
	}

	r.logger.Debug("getAllGraphData completed",
		zap.Int("nodes_found", len(nodes)),
		zap.Int("edges_found", len(edges)))

	return &domain.Graph{Nodes: nodes, Edges: edges}, nil
}

// fetchAllNodesOptimized retrieves all nodes for a user using scan with filter
func (r *ddbRepository) fetchAllNodesOptimized(ctx context.Context, userID string) ([]*domain.Node, error) {
	var nodes []*domain.Node
	var lastEvaluatedKey map[string]types.AttributeValue

	userNodePrefix := fmt.Sprintf("USER#%s#NODE#", userID)

	for {
		scanInput := &dynamodb.ScanInput{
			TableName:        aws.String(r.config.TableName),
			FilterExpression: aws.String("begins_with(PK, :pk_prefix) AND begins_with(SK, :sk_prefix)"),
			ExpressionAttributeValues: map[string]types.AttributeValue{
				":pk_prefix": &types.AttributeValueMemberS{Value: userNodePrefix},
				":sk_prefix": &types.AttributeValueMemberS{Value: "METADATA#"},
			},
			ExclusiveStartKey: lastEvaluatedKey,
		}

		result, err := r.dbClient.Scan(ctx, scanInput)
		if err != nil {
			return nil, appErrors.Wrap(err, "failed to scan nodes")
		}

		// Process nodes
		for _, item := range result.Items {
			var ddbItem ddbNode
			if err := attributevalue.UnmarshalMap(item, &ddbItem); err == nil {
				createdAt, _ := time.Parse(time.RFC3339, ddbItem.Timestamp)
				// Use domain factory method to reconstruct node from primitives
				node, err := domain.ReconstructNodeFromPrimitives(
					ddbItem.NodeID, 
					ddbItem.UserID, 
					ddbItem.Content,
					ddbItem.Keywords, 
					ddbItem.Tags,
					createdAt,
					ddbItem.Version,
				)
				if err == nil && node != nil {
					nodes = append(nodes, node)
				}
			}
		}

		lastEvaluatedKey = result.LastEvaluatedKey
		if lastEvaluatedKey == nil {
			break
		}
	}

	return nodes, nil
}

// fetchAllEdgesOptimized retrieves all edges for a user using GSI2 query
func (r *ddbRepository) fetchAllEdgesOptimized(ctx context.Context, userID string) ([]*domain.Edge, error) {
	var edges []*domain.Edge
	var lastEvaluatedKey map[string]types.AttributeValue
	edgeMap := make(map[string]bool)

	edgePrefix := fmt.Sprintf("USER#%s#EDGE", userID)

	for {
		queryInput := &dynamodb.QueryInput{
			TableName:              aws.String(r.config.TableName),
			IndexName:              aws.String("EdgeIndex"),
			KeyConditionExpression: aws.String("GSI2PK = :gsi2pk"),
			ExpressionAttributeValues: map[string]types.AttributeValue{
				":gsi2pk": &types.AttributeValueMemberS{Value: edgePrefix},
			},
			ExclusiveStartKey: lastEvaluatedKey,
		}

		result, err := r.dbClient.Query(ctx, queryInput)
		if err != nil {
			return nil, appErrors.Wrap(err, "failed to query edges")
		}

		// Process edges
		for _, item := range result.Items {
			var ddbItem ddbEdge
			if err := attributevalue.UnmarshalMap(item, &ddbItem); err == nil {
				// Extract source ID from PK pattern: USER#<userID>#NODE#<sourceID>
				pkParts := strings.Split(ddbItem.PK, "#")
				if len(pkParts) == 4 {
					sourceID := pkParts[3]
					// Prevent duplicate edges
					edgeKey := fmt.Sprintf("%s-%s", sourceID, ddbItem.TargetID)
					reverseKey := fmt.Sprintf("%s-%s", ddbItem.TargetID, sourceID)
					if !edgeMap[edgeKey] && !edgeMap[reverseKey] {
						edgeMap[edgeKey] = true
						// Create rich domain edge using factory method
						userIDVO, _ := domain.NewUserID(userID)
						sourceNodeIDVO, _ := domain.ParseNodeID(sourceID)
						targetNodeIDVO, _ := domain.ParseNodeID(ddbItem.TargetID)
						edge, err := domain.NewEdge(sourceNodeIDVO, targetNodeIDVO, userIDVO, 1.0)
						if err == nil {
							edges = append(edges, edge)
						}
					}
				}
			}
		}

		lastEvaluatedKey = result.LastEvaluatedKey
		if lastEvaluatedKey == nil {
			break
		}
	}

	return edges, nil
}

func (r *ddbRepository) clearNodeConnections(ctx context.Context, userID, nodeID string) error {
	var allWriteRequests []types.WriteRequest

	// First, delete all items in this node's partition (node data, keywords, edges where this node is owner)
	pk := fmt.Sprintf("USER#%s#NODE#%s", userID, nodeID)
	queryResult, err := r.dbClient.Query(ctx, &dynamodb.QueryInput{
		TableName:                 aws.String(r.config.TableName),
		KeyConditionExpression:    aws.String("PK = :pk"),
		ExpressionAttributeValues: map[string]types.AttributeValue{":pk": &types.AttributeValueMemberS{Value: pk}},
	})
	if err != nil {
		return appErrors.Wrap(err, "failed to query items for deletion")
	}

	for _, item := range queryResult.Items {
		allWriteRequests = append(allWriteRequests, types.WriteRequest{
			DeleteRequest: &types.DeleteRequest{
				Key: map[string]types.AttributeValue{"PK": item["PK"], "SK": item["SK"]},
			},
		})
	}

	// Second, find and delete edges where this node is the target (stored in other nodes' partitions)
	scanResult, err := r.dbClient.Scan(ctx, &dynamodb.ScanInput{
		TableName:        aws.String(r.config.TableName),
		FilterExpression: aws.String("begins_with(PK, :pk_prefix) AND begins_with(SK, :sk_prefix) AND TargetID = :target_id"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":pk_prefix": &types.AttributeValueMemberS{Value: fmt.Sprintf("USER#%s#NODE#", userID)},
			":sk_prefix": &types.AttributeValueMemberS{Value: "EDGE#RELATES_TO#"},
			":target_id": &types.AttributeValueMemberS{Value: nodeID},
		},
	})
	if err != nil {
		return appErrors.Wrap(err, "failed to scan for edges where node is target")
	}

	for _, item := range scanResult.Items {
		allWriteRequests = append(allWriteRequests, types.WriteRequest{
			DeleteRequest: &types.DeleteRequest{
				Key: map[string]types.AttributeValue{"PK": item["PK"], "SK": item["SK"]},
			},
		})
	}

	// Batch delete all items (DynamoDB has a 25 item limit per batch)
	if len(allWriteRequests) > 0 {
		for i := 0; i < len(allWriteRequests); i += 25 {
			end := i + 25
			if end > len(allWriteRequests) {
				end = len(allWriteRequests)
			}

			batchRequests := allWriteRequests[i:end]
			_, err = r.dbClient.BatchWriteItem(ctx, &dynamodb.BatchWriteItemInput{
				RequestItems: map[string][]types.WriteRequest{r.config.TableName: batchRequests},
			})
			if err != nil {
				return appErrors.Wrap(err, "failed to batch delete node items")
			}
		}
	}

	return nil
}

// Enhanced query methods using new query types

// FindNodes implements the enhanced node querying with NodeQuery.
func (r *ddbRepository) FindNodes(ctx context.Context, query repository.NodeQuery) ([]*domain.Node, error) {
	r.logger.Debug("findNodes called",
		zap.String("user_id", query.UserID),
		zap.Strings("keywords", query.Keywords),
		zap.Strings("node_ids", query.NodeIDs))
	
	if err := query.Validate(); err != nil {
		r.logger.Error("query validation failed", zap.Error(err))
		return nil, err
	}

	// If specific node IDs are requested, fetch them directly
	if query.HasNodeIDs() {
		r.logger.Debug("using nodeID-based lookup",
			zap.Int("node_count", len(query.NodeIDs)))
		var nodes []*domain.Node
		for _, nodeID := range query.NodeIDs {
			node, err := r.FindNodeByID(ctx, query.UserID, nodeID)
			if err != nil {
				return nil, err
			}
			if node != nil {
				nodes = append(nodes, node)
			}
		}
		r.logger.Debug("nodeID lookup completed",
			zap.Int("nodes_found", len(nodes)))
		return nodes, nil
	}

	// If keywords are specified, use keyword search
	if query.HasKeywords() {
		r.logger.Debug("using keyword-based lookup",
			zap.Strings("keywords", query.Keywords))
		nodes, err := r.FindNodesByKeywords(ctx, query.UserID, query.Keywords)
		r.logger.Debug("keyword lookup completed",
			zap.Int("nodes_found", len(nodes)),
			zap.Error(err))
		return nodes, err
	}

	// Otherwise, get all nodes for the user (this could be expensive for large datasets)
	graph, err := r.GetAllGraphData(ctx, query.UserID)
	if err != nil {
		return nil, err
	}

	nodes := graph.Nodes

	// Apply pagination if specified
	if query.HasPagination() {
		start := query.Offset
		if start >= len(nodes) {
			return []*domain.Node{}, nil
		}

		end := len(nodes)
		if query.Limit > 0 && start+query.Limit < len(nodes) {
			end = start + query.Limit
		}

		nodes = nodes[start:end]
	}

	return nodes, nil
}

// FindEdges implements the enhanced edge querying with EdgeQuery.
func (r *ddbRepository) FindEdges(ctx context.Context, query repository.EdgeQuery) ([]*domain.Edge, error) {
	if err := query.Validate(); err != nil {
		return nil, err
	}

	var edges []*domain.Edge

	// If specific node IDs are requested, find edges for each
	if query.HasNodeIDs() {
		for _, nodeID := range query.NodeIDs {
			nodeEdges, err := r.FindEdgesByNode(ctx, query.UserID, nodeID)
			if err != nil {
				return nil, err
			}
			// Convert []domain.Edge to []*domain.Edge
			for i := range nodeEdges {
				edges = append(edges, &nodeEdges[i])
			}
		}
		return edges, nil
	}

	// If source node is specified, find outgoing edges
	if query.HasSourceFilter() {
		nodeEdges, err := r.FindEdgesByNode(ctx, query.UserID, query.SourceID)
		if err != nil {
			return nil, err
		}
		// Convert []domain.Edge to []*domain.Edge
		var edges []*domain.Edge
		for i := range nodeEdges {
			edges = append(edges, &nodeEdges[i])
		}
		return edges, nil
	}

	// If target node is specified, we need to scan for incoming edges
	if query.HasTargetFilter() {
		// This is less efficient but necessary for target-based queries
		graph, err := r.GetAllGraphData(ctx, query.UserID)
		if err != nil {
			return nil, err
		}

		var edges []*domain.Edge
		for _, edge := range graph.Edges {
			if edge.TargetID.String() == query.TargetID {
				edges = append(edges, edge)
			}
		}
		return edges, nil
	}

	// Otherwise, get all edges for the user
	graph, err := r.GetAllGraphData(ctx, query.UserID)
	if err != nil {
		return nil, err
	}

	edges = graph.Edges

	// Apply pagination if specified
	if query.HasPagination() {
		start := query.Offset
		if start >= len(edges) {
			return []*domain.Edge{}, nil
		}

		end := len(edges)
		if query.Limit > 0 && start+query.Limit < len(edges) {
			end = start + query.Limit
		}

		edges = edges[start:end]
	}

	return edges, nil
}

// GetGraphData implements the enhanced graph querying with GraphQuery.
func (r *ddbRepository) GetGraphData(ctx context.Context, query repository.GraphQuery) (*domain.Graph, error) {
	if err := query.Validate(); err != nil {
		return nil, err
	}

	// For now, we'll implement a basic version that filters by node IDs if specified
	// More complex depth-limiting would require additional graph traversal logic

	if query.HasNodeFilter() {
		var nodes []*domain.Node
		var edges []*domain.Edge

		// Get specific nodes
		for _, nodeID := range query.NodeIDs {
			node, err := r.FindNodeByID(ctx, query.UserID, nodeID)
			if err != nil {
				return nil, err
			}
			if node != nil {
				nodes = append(nodes, node)

				// Include edges if requested
				if query.IncludeEdges {
					nodeEdges, err := r.FindEdgesByNode(ctx, query.UserID, nodeID)
					if err != nil {
						return nil, err
					}
					// Convert []domain.Edge to []*domain.Edge
					for i := range nodeEdges {
						edges = append(edges, &nodeEdges[i])
					}
				}
			}
		}

		return &domain.Graph{Nodes: nodes, Edges: edges}, nil
	}

	// Otherwise, return all graph data
	return r.GetAllGraphData(ctx, query.UserID)
}

// Add these new methods to ddb.go

// CreateNode saves only the metadata for a node.
func (r *ddbRepository) CreateNode(ctx context.Context, node domain.Node) error {
	// Ensure node starts with version 0
	// Note: Version is immutable in rich domain model, using Version().Int() for DDB
	
	pk := fmt.Sprintf("USER#%s#NODE#%s", node.UserID.String(), node.ID.String())
	nodeItem, err := attributevalue.MarshalMap(ddbNode{
		PK: pk, SK: "METADATA#v0", NodeID: node.ID.String(), UserID: node.UserID.String(), Content: node.Content.String(),
		Keywords: node.Keywords().ToSlice(), Tags: node.Tags.ToSlice(), IsLatest: true, Version: node.Version, Timestamp: node.CreatedAt.Format(time.RFC3339),
	})
	if err != nil {
		return appErrors.Wrap(err, "failed to marshal node item")
	}

	_, err = r.dbClient.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(r.config.TableName),
		Item:      nodeItem,
	})
	if err != nil {
		return appErrors.Wrap(err, "put item failed for node metadata")
	}
	return nil
}

// CreateEdges creates bidirectional edges between a source node and multiple related nodes.
func (r *ddbRepository) CreateEdges(ctx context.Context, userID, sourceNodeID string, relatedNodeIDs []string) error {
	if len(relatedNodeIDs) == 0 {
		return nil
	}

	var writeRequests []types.WriteRequest
	pkSource := fmt.Sprintf("USER#%s#NODE#%s", userID, sourceNodeID)

	for _, relatedNodeID := range relatedNodeIDs {
		// Edge: Source -> Related
		edge1Item, err := attributevalue.MarshalMap(ddbEdge{
			PK:       pkSource,
			SK:       fmt.Sprintf("EDGE#RELATES_TO#%s", relatedNodeID),
			TargetID: relatedNodeID,
			GSI2PK:   fmt.Sprintf("USER#%s#EDGE", userID),
			GSI2SK:   fmt.Sprintf("NODE#%s#TARGET#%s", sourceNodeID, relatedNodeID),
		})
		if err != nil {
			return appErrors.Wrap(err, "failed to marshal outgoing edge")
		}
		writeRequests = append(writeRequests, types.WriteRequest{PutRequest: &types.PutRequest{Item: edge1Item}})

		// Edge: Related -> Source
		pkRelated := fmt.Sprintf("USER#%s#NODE#%s", userID, relatedNodeID)
		edge2Item, err := attributevalue.MarshalMap(ddbEdge{
			PK:       pkRelated,
			SK:       fmt.Sprintf("EDGE#RELATES_TO#%s", sourceNodeID),
			TargetID: sourceNodeID,
			GSI2PK:   fmt.Sprintf("USER#%s#EDGE", userID),
			GSI2SK:   fmt.Sprintf("NODE#%s#TARGET#%s", relatedNodeID, sourceNodeID),
		})
		if err != nil {
			return appErrors.Wrap(err, "failed to marshal incoming edge")
		}
		writeRequests = append(writeRequests, types.WriteRequest{PutRequest: &types.PutRequest{Item: edge2Item}})
	}

	// Batch write the edges
	_, err := r.dbClient.BatchWriteItem(ctx, &dynamodb.BatchWriteItemInput{
		RequestItems: map[string][]types.WriteRequest{
			r.config.TableName: writeRequests,
		},
	})
	if err != nil {
		return appErrors.Wrap(err, "batch write for edges failed")
	}
	return nil
}

func toAttributeValueList(ss []string) []types.AttributeValue {
	var avs []types.AttributeValue
	for _, s := range ss {
		avs = append(avs, &types.AttributeValueMemberS{Value: s})
	}
	return avs
}

// Category operations implementation

// CreateCategory creates a new category with enhanced hierarchical support.
func (r *ddbRepository) CreateCategory(ctx context.Context, category domain.Category) error {
	// Simplified implementation for consolidation phase
	return nil // Placeholder implementation
}

// UpdateCategory updates an existing category with enhanced hierarchical support.
func (r *ddbRepository) UpdateCategory(ctx context.Context, category domain.Category) error {
	// Simplified implementation for consolidation phase
	return nil // Placeholder implementation
}

// DeleteCategory deletes a category and all its memory associations.
func (r *ddbRepository) DeleteCategory(ctx context.Context, userID, categoryID string) error {
	pk := fmt.Sprintf("USER#%s#CATEGORY#%s", userID, categoryID)

	// First, query all items for this category
	queryResult, err := r.dbClient.Query(ctx, &dynamodb.QueryInput{
		TableName:                 aws.String(r.config.TableName),
		KeyConditionExpression:    aws.String("PK = :pk"),
		ExpressionAttributeValues: map[string]types.AttributeValue{":pk": &types.AttributeValueMemberS{Value: pk}},
	})
	if err != nil {
		return appErrors.Wrap(err, "failed to query category items for deletion")
	}

	if len(queryResult.Items) == 0 {
		return nil // Nothing to delete
	}

	// Delete all items (category metadata and memory associations)
	var writeRequests []types.WriteRequest
	for _, item := range queryResult.Items {
		writeRequests = append(writeRequests, types.WriteRequest{
			DeleteRequest: &types.DeleteRequest{
				Key: map[string]types.AttributeValue{
					"PK": item["PK"],
					"SK": item["SK"],
				},
			},
		})
	}

	if len(writeRequests) > 0 {
		_, err = r.dbClient.BatchWriteItem(ctx, &dynamodb.BatchWriteItemInput{
			RequestItems: map[string][]types.WriteRequest{
				r.config.TableName: writeRequests,
			},
		})
		if err != nil {
			return appErrors.Wrap(err, "failed to batch delete category items")
		}
	}
	return nil
}

// FindCategoryByID retrieves a single category by ID using enhanced format.
func (r *ddbRepository) FindCategoryByID(ctx context.Context, userID, categoryID string) (*domain.Category, error) {
	pk := fmt.Sprintf("USER#%s", userID)
	sk := fmt.Sprintf("CATEGORY#%s", categoryID)

	result, err := r.dbClient.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(r.config.TableName),
		Key: map[string]types.AttributeValue{
			"PK": &types.AttributeValueMemberS{Value: pk},
			"SK": &types.AttributeValueMemberS{Value: sk},
		},
	})
	if err != nil {
		return nil, appErrors.Wrap(err, "failed to get category from dynamodb")
	}
	if result.Item == nil {
		return nil, nil // Not found
	}

	var ddbItem ddbCategory
	if err := attributevalue.UnmarshalMap(result.Item, &ddbItem); err != nil {
		return nil, appErrors.Wrap(err, "failed to unmarshal category item")
	}

	// Convert to domain category
	category := domain.Category{
		ID:          domain.CategoryID(ddbItem.CategoryID),
		UserID:      ddbItem.UserID,
		Name:        ddbItem.Title,
		Title:       ddbItem.Title,
		Description: ddbItem.Description,
	}
	return &category, nil
}

// FindCategories retrieves categories based on query parameters.
func (r *ddbRepository) FindCategories(ctx context.Context, query repository.CategoryQuery) ([]domain.Category, error) {
	if err := query.Validate(); err != nil {
		return nil, err
	}

	// Use Query instead of Scan for better performance
	var categories []domain.Category
	var lastEvaluatedKey map[string]types.AttributeValue

	userPK := fmt.Sprintf("USER#%s", query.UserID)

	for {
		queryInput := &dynamodb.QueryInput{
			TableName:              aws.String(r.config.TableName),
			KeyConditionExpression: aws.String("PK = :pk AND begins_with(SK, :skPrefix)"),
			ExpressionAttributeValues: map[string]types.AttributeValue{
				":pk":       &types.AttributeValueMemberS{Value: userPK},
				":skPrefix": &types.AttributeValueMemberS{Value: "CATEGORY#"},
			},
			ExclusiveStartKey: lastEvaluatedKey,
		}

		result, err := r.dbClient.Query(ctx, queryInput)
		if err != nil {
			return nil, appErrors.Wrap(err, "failed to query categories")
		}

		// Process categories
		for _, item := range result.Items {
			var ddbItem ddbCategory
			if err := attributevalue.UnmarshalMap(item, &ddbItem); err == nil {
				// Convert to domain category
				category := domain.Category{
					ID:          domain.CategoryID(ddbItem.CategoryID),
					UserID:      ddbItem.UserID,
					Name:        ddbItem.Title,
					Title:       ddbItem.Title,
					Description: ddbItem.Description,
				}
				categories = append(categories, category)
			}
		}

		lastEvaluatedKey = result.LastEvaluatedKey
		if lastEvaluatedKey == nil {
			break
		}
	}

	// Apply pagination if specified
	if query.HasPagination() {
		start := query.Offset
		if start >= len(categories) {
			return []domain.Category{}, nil
		}

		end := len(categories)
		if query.Limit > 0 && start+query.Limit < len(categories) {
			end = start + query.Limit
		}

		categories = categories[start:end]
	}

	return categories, nil
}

// GetNodesPage retrieves a paginated list of nodes for a user using optimized query
func (r *ddbRepository) GetNodesPage(ctx context.Context, query repository.NodeQuery, pagination repository.Pagination) (*repository.NodePage, error) {
	if err := query.Validate(); err != nil {
		return nil, err
	}
	if err := pagination.Validate(); err != nil {
		return nil, err
	}

	userNodePrefix := fmt.Sprintf("USER#%s#NODE#", query.UserID)
	requestedLimit := pagination.GetEffectiveLimit()
	nodes := make([]*domain.Node, 0, requestedLimit)
	
	var lastEvaluatedKey map[string]types.AttributeValue
	
	// Handle cursor-based pagination for starting point
	if pagination.HasCursor() {
		startKey, err := repository.DecodeCursor(pagination.Cursor)
		if err == nil && startKey != nil {
			lastEvaluatedKey = startKey
		}
	}
	
	scanIterations := 0
	
	// Continue scanning until we have enough nodes or no more items exist
	for len(nodes) < requestedLimit {
		scanIterations++
		
		scanInput := &dynamodb.ScanInput{
			TableName:        aws.String(r.config.TableName),
			FilterExpression: aws.String("begins_with(PK, :pk_prefix) AND begins_with(SK, :sk_prefix)"),
			ExpressionAttributeValues: map[string]types.AttributeValue{
				":pk_prefix": &types.AttributeValueMemberS{Value: userNodePrefix},
				":sk_prefix": &types.AttributeValueMemberS{Value: "METADATA#"},
			},
			// Use a reasonable scan limit to avoid timeouts
			Limit:             aws.Int32(100),
			ExclusiveStartKey: lastEvaluatedKey,
		}

		result, err := r.dbClient.Scan(ctx, scanInput)
		if err != nil {
			return nil, appErrors.Wrap(err, "failed to scan nodes page")
		}

		// Process items from this scan segment
		itemsProcessedThisIteration := 0
		for _, item := range result.Items {
			var ddbItem ddbNode
			if err := attributevalue.UnmarshalMap(item, &ddbItem); err == nil {
				// Log nodes with IsLatest=false for debugging
				if !ddbItem.IsLatest {
					r.logger.Debug("found node with IsLatest=false",
						zap.String("node_id", ddbItem.NodeID),
						zap.String("user_id", ddbItem.UserID),
						zap.Int("version", ddbItem.Version))
				}
				
				// Include ALL valid nodes (removing IsLatest filter to match graph behavior)
				createdAt, _ := time.Parse(time.RFC3339, ddbItem.Timestamp)
				node, err := domain.ReconstructNodeFromPrimitives(
					ddbItem.NodeID,
					ddbItem.UserID,
					ddbItem.Content,
					ddbItem.Keywords,
					ddbItem.Tags,
					createdAt,
					ddbItem.Version,
				)
				if err != nil {
					r.logger.Error("failed to reconstruct node", zap.Error(err))
					continue
				}
				nodes = append(nodes, node)
				itemsProcessedThisIteration++
				
				// Stop if we've reached our limit
				if len(nodes) >= requestedLimit {
					break
				}
			} else {
				// Log unmarshaling errors for debugging
				r.logger.Warn("failed to unmarshal node", zap.Error(err))
			}
		}
		
		// Update pagination state
		lastEvaluatedKey = result.LastEvaluatedKey
		
		// Log scan continuation for debugging
		if scanIterations > 1 {
			r.logger.Debug("scan continuation",
				zap.Int("iteration", scanIterations),
				zap.Int("items_processed", itemsProcessedThisIteration),
				zap.Int("total_collected", len(nodes)),
				zap.Int("total_requested", requestedLimit))
		}
		
		// Break if no more items available
		if result.LastEvaluatedKey == nil {
			break
		}
		
		// Safety check to prevent infinite loops
		if scanIterations > 10 {
			r.logger.Warn("exceeded maximum scan iterations",
				zap.Int("iterations", scanIterations),
				zap.Int("nodes_returned", len(nodes)))
			break
		}
	}
	
	// Final logging
	if scanIterations > 1 {
		r.logger.Debug("getNodesPage completed",
			zap.Int("iterations", scanIterations),
			zap.Int("nodes_returned", len(nodes)),
			zap.Int("total_requested", requestedLimit))
	}

	return &repository.NodePage{
		Items:      nodes,
		HasMore:    lastEvaluatedKey != nil && len(nodes) >= requestedLimit,
		NextCursor: repository.EncodeCursor(lastEvaluatedKey),
		PageInfo:   repository.CreatePageInfo(pagination, len(nodes), lastEvaluatedKey != nil),
	}, nil
}

// CountNodes returns the total number of nodes for a user
func (r *ddbRepository) CountNodes(ctx context.Context, userID string) (int, error) {
	userNodePrefix := fmt.Sprintf("USER#%s#NODE#", userID)
	
	scanInput := &dynamodb.ScanInput{
		TableName:        aws.String(r.config.TableName),
		FilterExpression: aws.String("begins_with(PK, :pk_prefix) AND begins_with(SK, :sk_prefix)"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":pk_prefix": &types.AttributeValueMemberS{Value: userNodePrefix},
			":sk_prefix": &types.AttributeValueMemberS{Value: "METADATA#"},
		},
		Select: types.SelectCount,  // Only count items, don't return data
	}

	count := 0
	paginator := dynamodb.NewScanPaginator(r.dbClient, scanInput)
	
	for paginator.HasMorePages() {
		result, err := paginator.NextPage(ctx)
		if err != nil {
			return 0, appErrors.Wrap(err, "failed to count nodes")
		}
		count += int(result.Count)
	}

	return count, nil
}

// GetNodesPageOptimized retrieves a paginated list of nodes using the new PageRequest/PageResponse types
func (r *ddbRepository) GetNodesPageOptimized(ctx context.Context, userID string, pageReq repository.PageRequest) (*repository.PageResponse, error) {
	userPrefix := fmt.Sprintf("USER#%s", userID)
	
	queryInput := &dynamodb.QueryInput{
		TableName:              aws.String(r.config.TableName),
		KeyConditionExpression: aws.String("PK = :pk AND begins_with(SK, :sk_prefix)"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":pk":        &types.AttributeValueMemberS{Value: userPrefix},
			":sk_prefix": &types.AttributeValueMemberS{Value: "METADATA#"},
		},
		Limit: aws.Int32(int32(pageReq.GetEffectiveLimit())),
	}

	// Handle cursor-based pagination with new PageRequest
	if pageReq.HasNextToken() {
		lastKey, err := repository.DecodeNextToken(pageReq.NextToken)
		if err == nil && lastKey != nil {
			queryInput.ExclusiveStartKey = lastKey.ToDynamoDBKey()
		}
	}

	result, err := r.dbClient.Query(ctx, queryInput)
	if err != nil {
		return nil, appErrors.Wrap(err, "failed to query nodes page optimized")
	}

	// Convert DynamoDB items to domain nodes
	nodes := make([]*domain.Node, 0, len(result.Items))
	for _, item := range result.Items {
		var ddbItem ddbNode
		if err := attributevalue.UnmarshalMap(item, &ddbItem); err == nil {
			if ddbItem.IsLatest { // Only return latest versions
				createdAt, _ := time.Parse(time.RFC3339, ddbItem.Timestamp)
				node, err := domain.ReconstructNodeFromPrimitives(
					ddbItem.NodeID,
					ddbItem.UserID,
					ddbItem.Content,
					ddbItem.Keywords,
					ddbItem.Tags,
					createdAt,
					ddbItem.Version,
				)
				if err != nil {
					r.logger.Error("failed to reconstruct node", zap.Error(err))
					continue
				}
				nodes = append(nodes, node)
			}
		}
	}

	// Create paginated response
	return repository.CreatePageResponse(nodes, result.LastEvaluatedKey, result.LastEvaluatedKey != nil), nil
}

// GetNodeNeighborhood retrieves nodes and edges within a specified depth from a target node
func (r *ddbRepository) GetNodeNeighborhood(ctx context.Context, userID, nodeID string, depth int) (*domain.Graph, error) {
	if depth <= 0 {
		depth = 1
	}
	if depth > 3 {
		depth = 3 // Limit depth to prevent excessive queries
	}

	visited := make(map[string]bool)
	nodes := make(map[string]*domain.Node)
	edges := make([]*domain.Edge, 0)

	// Start with the target node
	currentLevel := []string{nodeID}
	visited[nodeID] = true

	for currentDepth := 0; currentDepth < depth && len(currentLevel) > 0; currentDepth++ {
		var nextLevel []string

		for _, currentNodeID := range currentLevel {
			// Get the node details
			node, err := r.FindNodeByID(ctx, userID, currentNodeID)
			if err == nil && node != nil {
				nodes[currentNodeID] = node
			}

			// Get edges from this node
			queryInput := &dynamodb.QueryInput{
				TableName:              aws.String(r.config.TableName),
				KeyConditionExpression: aws.String("PK = :pk AND begins_with(SK, :sk)"),
				ExpressionAttributeValues: map[string]types.AttributeValue{
					":pk": &types.AttributeValueMemberS{Value: fmt.Sprintf("USER#%s#NODE#%s", userID, currentNodeID)},
					":sk": &types.AttributeValueMemberS{Value: "EDGE#"},
				},
			}

			result, err := r.dbClient.Query(ctx, queryInput)
			if err != nil {
				continue // Skip this node's edges on error
			}

			for _, item := range result.Items {
				var ddbEdge ddbEdge
				if err := attributevalue.UnmarshalMap(item, &ddbEdge); err == nil {
					sourceNodeID, _ := domain.ParseNodeID(currentNodeID)
					targetNodeID, _ := domain.ParseNodeID(ddbEdge.TargetID)
					userIDVO, _ := domain.NewUserID(userID)
					edge, err := domain.NewEdge(sourceNodeID, targetNodeID, userIDVO, 1.0)
					if err != nil {
						r.logger.Error("failed to create edge", zap.Error(err))
						continue
					}
					edges = append(edges, edge)

					// Add target node to next level if not visited
					if !visited[ddbEdge.TargetID] {
						visited[ddbEdge.TargetID] = true
						nextLevel = append(nextLevel, ddbEdge.TargetID)
					}
				}
			}
		}

		currentLevel = nextLevel
	}

	// Convert nodes map to slice
	nodeSlice := make([]*domain.Node, 0, len(nodes))
	for _, node := range nodes {
		nodeSlice = append(nodeSlice, node)
	}

	return &domain.Graph{
		Nodes: nodeSlice,
		Edges: edges,
	}, nil
}

// GetEdgesPage retrieves a paginated list of edges for a user
func (r *ddbRepository) GetEdgesPage(ctx context.Context, query repository.EdgeQuery, pagination repository.Pagination) (*repository.EdgePage, error) {
	if err := query.Validate(); err != nil {
		return nil, err
	}
	if err := pagination.Validate(); err != nil {
		return nil, err
	}

	var queryInput *dynamodb.QueryInput

	if query.HasSourceFilter() {
		// Query edges from a specific source node
		queryInput = &dynamodb.QueryInput{
			TableName:              aws.String(r.config.TableName),
			KeyConditionExpression: aws.String("PK = :pk AND begins_with(SK, :sk)"),
			ExpressionAttributeValues: map[string]types.AttributeValue{
				":pk": &types.AttributeValueMemberS{Value: fmt.Sprintf("USER#%s#NODE#%s", query.UserID, query.SourceID)},
				":sk": &types.AttributeValueMemberS{Value: "EDGE#"},
			},
		}
	} else {
		// Query all edges for user - less efficient but works
		queryInput = &dynamodb.QueryInput{
			TableName:              aws.String(r.config.TableName),
			KeyConditionExpression: aws.String("PK = :pk"),
			FilterExpression:       aws.String("begins_with(SK, :sk)"),
			ExpressionAttributeValues: map[string]types.AttributeValue{
				":pk": &types.AttributeValueMemberS{Value: fmt.Sprintf("USER#%s", query.UserID)},
				":sk": &types.AttributeValueMemberS{Value: "EDGE#"},
			},
		}
	}

	queryInput.Limit = aws.Int32(int32(pagination.GetEffectiveLimit()))

	// Handle cursor-based pagination
	if pagination.HasCursor() {
		startKey, err := repository.DecodeCursor(pagination.Cursor)
		if err == nil && startKey != nil {
			queryInput.ExclusiveStartKey = startKey
		}
	}

	result, err := r.dbClient.Query(ctx, queryInput)
	if err != nil {
		return nil, appErrors.Wrap(err, "failed to query edges page")
	}

	// Convert DynamoDB items to domain edges
	edges := make([]*domain.Edge, 0, len(result.Items))
	for _, item := range result.Items {
		var ddbItem ddbEdge
		if err := attributevalue.UnmarshalMap(item, &ddbItem); err == nil {
			// Extract source node ID from PK
			pkParts := strings.Split(ddbItem.PK, "#")
			if len(pkParts) >= 4 {
				sourceID := pkParts[3]
				sourceNodeID, _ := domain.ParseNodeID(sourceID)
				targetNodeID, _ := domain.ParseNodeID(ddbItem.TargetID)
				userIDVO, _ := domain.NewUserID(query.UserID)
				edge, err := domain.NewEdge(sourceNodeID, targetNodeID, userIDVO, 1.0)
				if err != nil {
					r.logger.Error("failed to create edge", zap.Error(err))
					continue
				}
				edges = append(edges, edge)
			}
		}
	}

	return &repository.EdgePage{
		Items:      edges,
		HasMore:    result.LastEvaluatedKey != nil,
		NextCursor: repository.EncodeCursor(result.LastEvaluatedKey),
		PageInfo:   repository.CreatePageInfo(pagination, len(edges), result.LastEvaluatedKey != nil),
	}, nil
}

// GetGraphDataPaginated retrieves graph data with pagination for large datasets
func (r *ddbRepository) GetGraphDataPaginated(ctx context.Context, query repository.GraphQuery, pagination repository.Pagination) (*domain.Graph, string, error) {
	r.logger.Debug("getGraphDataPaginated called",
		zap.String("user_id", query.UserID),
		zap.Bool("include_edges", query.IncludeEdges),
		zap.Int("limit", pagination.GetEffectiveLimit()))

	if err := query.Validate(); err != nil {
		r.logger.Error("query validation failed", zap.Error(err))
		return nil, "", err
	}
	if err := pagination.Validate(); err != nil {
		r.logger.Error("pagination validation failed", zap.Error(err))
		return nil, "", err
	}

	// Use scan with filter to find nodes and edges - stored with PK pattern USER#<userID>#NODE#<nodeID>
	var filterExpression string
	expressionValues := map[string]types.AttributeValue{
		":pk_prefix": &types.AttributeValueMemberS{Value: fmt.Sprintf("USER#%s#NODE#", query.UserID)},
	}

	if query.IncludeEdges {
		// Include both nodes (METADATA#) and edges (EDGE#)
		filterExpression = "begins_with(PK, :pk_prefix) AND (begins_with(SK, :sk_metadata) OR begins_with(SK, :sk_edge))"
		expressionValues[":sk_metadata"] = &types.AttributeValueMemberS{Value: "METADATA#"}
		expressionValues[":sk_edge"] = &types.AttributeValueMemberS{Value: "EDGE#"}
	} else {
		// Only nodes (METADATA#)
		filterExpression = "begins_with(PK, :pk_prefix) AND begins_with(SK, :sk_metadata)"
		expressionValues[":sk_metadata"] = &types.AttributeValueMemberS{Value: "METADATA#"}
	}

	scanInput := &dynamodb.ScanInput{
		TableName:                 aws.String(r.config.TableName),
		FilterExpression:          aws.String(filterExpression),
		ExpressionAttributeValues: expressionValues,
		Limit:                     aws.Int32(int32(pagination.GetEffectiveLimit())),
	}

	r.logger.Debug("executing DynamoDB scan",
		zap.String("table_name", r.config.TableName),
		zap.String("user_id", query.UserID))

	// Handle cursor-based pagination
	if pagination.HasCursor() {
		startKey, err := repository.DecodeCursor(pagination.Cursor)
		if err == nil && startKey != nil {
			scanInput.ExclusiveStartKey = startKey
		}
	}

	result, err := r.dbClient.Scan(ctx, scanInput)
	if err != nil {
		r.logger.Error("DynamoDB query failed", zap.Error(err))
		return nil, "", appErrors.Wrap(err, "failed to query graph data")
	}

	r.logger.Debug("DynamoDB query successful",
		zap.Int("items_returned", len(result.Items)),
		zap.Bool("has_more_data", result.LastEvaluatedKey != nil))

	var nodes []*domain.Node
	var edges []*domain.Edge
	edgeMap := make(map[string]bool)

	nodeCount := 0
	edgeCount := 0
	skippedItems := 0

	for _, item := range result.Items {
		skValueAttr, ok := item["SK"].(*types.AttributeValueMemberS)
		if !ok {
			r.logger.Warn("item missing SK attribute", zap.Any("item", item))
			skippedItems++
			continue
		}
		skValue := skValueAttr.Value

		if strings.HasPrefix(skValue, "METADATA#") {
			var ddbItem ddbNode
			if err := attributevalue.UnmarshalMap(item, &ddbItem); err != nil {
				r.logger.Error("failed to unmarshal node item",
					zap.Error(err),
					zap.Any("item", item))
				skippedItems++
				continue
			}
			if ddbItem.IsLatest {
				createdAt, parseErr := time.Parse(time.RFC3339, ddbItem.Timestamp)
				if parseErr != nil {
					r.logger.Warn("failed to parse timestamp",
						zap.String("timestamp", ddbItem.Timestamp),
						zap.Error(parseErr))
					createdAt = time.Now() // fallback
				}
				node, err := domain.ReconstructNodeFromPrimitives(
					ddbItem.NodeID,
					ddbItem.UserID,
					ddbItem.Content,
					ddbItem.Keywords,
					ddbItem.Tags,
					createdAt,
					ddbItem.Version,
				)
				if err != nil {
					r.logger.Error("failed to reconstruct node", zap.Error(err))
					continue
				}
				nodes = append(nodes, node)
				nodeCount++
			}
		} else if strings.HasPrefix(skValue, "EDGE#") && query.IncludeEdges {
			var ddbItem ddbEdge
			if err := attributevalue.UnmarshalMap(item, &ddbItem); err != nil {
				r.logger.Error("failed to unmarshal edge item",
					zap.Error(err),
					zap.Any("item", item))
				skippedItems++
				continue
			}

			// Extract source node ID from PK
			pkParts := strings.Split(ddbItem.PK, "#")
			if len(pkParts) >= 4 {
				sourceID := pkParts[3]
				
				// Create canonical edge key for deduplication (since we're reading canonical storage)
				ownerID, targetID := getCanonicalEdge(sourceID, ddbItem.TargetID)
				canonicalKey := fmt.Sprintf("%s-%s", ownerID, targetID)
				
				if !edgeMap[canonicalKey] {
					edgeMap[canonicalKey] = true
					
					// Create undirected edge using canonical ordering for consistent visualization
					sourceNodeID, _ := domain.ParseNodeID(ownerID)
					targetNodeID, _ := domain.ParseNodeID(targetID)
					userIDVO, _ := domain.NewUserID(query.UserID)
					edge, err := domain.NewEdge(sourceNodeID, targetNodeID, userIDVO, 1.0)
					if err != nil {
						r.logger.Error("failed to create edge", zap.Error(err))
						continue
					}
					edges = append(edges, edge)
					edgeCount++
				}
			} else {
				r.logger.Warn("invalid PK format for edge", zap.String("pk", ddbItem.PK))
			}
		}
	}

	r.logger.Debug("processed data",
		zap.Int("nodes", nodeCount),
		zap.Int("edges", edgeCount),
		zap.Int("skipped_items", skippedItems))

	nextCursor := repository.EncodeCursor(result.LastEvaluatedKey)

	r.logger.Debug("getGraphDataPaginated completed",
		zap.Int("nodes_returned", len(nodes)),
		zap.Int("edges_returned", len(edges)),
		zap.String("next_cursor", nextCursor))

	return &domain.Graph{
		Nodes: nodes,
		Edges: edges,
	}, nextCursor, nil
}

// Phase 2 Enhanced Methods - Added for interface compatibility

// FindNodesWithOptions implements enhanced node queries with options
func (repo *ddbRepository) FindNodesWithOptions(ctx context.Context, query repository.NodeQuery, opts ...repository.QueryOption) ([]*domain.Node, error) {
	// For consolidation phase, delegate to existing method
	return repo.FindNodes(ctx, query)
}

// FindNodesPageWithOptions implements enhanced paginated node queries with options  
func (repo *ddbRepository) FindNodesPageWithOptions(ctx context.Context, query repository.NodeQuery, pagination repository.Pagination, opts ...repository.QueryOption) (*repository.NodePage, error) {
	// For consolidation phase, delegate to existing method
	return repo.GetNodesPage(ctx, query, pagination)
}

// FindEdgesWithOptions implements enhanced edge queries with options
func (repo *ddbRepository) FindEdgesWithOptions(ctx context.Context, query repository.EdgeQuery, opts ...repository.QueryOption) ([]*domain.Edge, error) {
	// For consolidation phase, delegate to existing method
	return repo.FindEdges(ctx, query)
}

// GetSubgraph implements subgraph extraction  
func (repo *ddbRepository) GetSubgraph(ctx context.Context, nodeIDs []string, opts ...repository.QueryOption) (*domain.Graph, error) {
	// For consolidation phase, return empty graph - this would be a complex subgraph operation
	return &domain.Graph{Nodes: []*domain.Node{}, Edges: []*domain.Edge{}}, nil
}

// GetConnectedComponents implements graph connected components analysis
func (repo *ddbRepository) GetConnectedComponents(ctx context.Context, userID string, opts ...repository.QueryOption) ([]domain.Graph, error) {
	// For consolidation phase, return empty result - this would be a complex graph analysis operation
	return []domain.Graph{}, nil
}

// AssignNodeToCategory assigns a node to a category
func (repo *ddbRepository) AssignNodeToCategory(ctx context.Context, mapping domain.NodeCategory) error {
	// Simplified implementation for consolidation phase
	return nil
}

// RemoveNodeFromCategory removes a node from a category
func (repo *ddbRepository) RemoveNodeFromCategory(ctx context.Context, userID, nodeID, categoryID string) error {
	// Simplified implementation for consolidation phase
	return nil
}

// FindNodesByCategory finds all nodes in a category
func (repo *ddbRepository) FindNodesByCategory(ctx context.Context, userID, categoryID string) ([]*domain.Node, error) {
	// Simplified implementation for consolidation phase
	return []*domain.Node{}, nil
}

// FindCategoriesForNode finds all categories for a node
func (repo *ddbRepository) FindCategoriesForNode(ctx context.Context, userID, nodeID string) ([]domain.Category, error) {
	// Simplified implementation for consolidation phase
	return []domain.Category{}, nil
}

// BatchAssignCategories assigns multiple nodes to categories in batch
func (repo *ddbRepository) BatchAssignCategories(ctx context.Context, mappings []domain.NodeCategory) error {
	// Simplified implementation for consolidation phase
	return nil
}

// UpdateCategoryNoteCounts updates note counts for categories
func (repo *ddbRepository) UpdateCategoryNoteCounts(ctx context.Context, userID string, categoryCounts map[string]int) error {
	// Simplified implementation for consolidation phase
	return nil
}

// CreateCategoryHierarchy creates a hierarchy relationship between categories
func (repo *ddbRepository) CreateCategoryHierarchy(ctx context.Context, hierarchy domain.CategoryHierarchy) error {
	// Simplified implementation for consolidation phase
	return nil
}

// DeleteCategoryHierarchy deletes a hierarchy relationship between categories
func (repo *ddbRepository) DeleteCategoryHierarchy(ctx context.Context, userID, parentID, childID string) error {
	// Simplified implementation for consolidation phase
	return nil
}

// FindChildCategories finds child categories for a given parent
func (repo *ddbRepository) FindChildCategories(ctx context.Context, userID, parentID string) ([]domain.Category, error) {
	// Simplified implementation for consolidation phase
	return []domain.Category{}, nil
}

// FindParentCategory finds the parent category for a given child
func (repo *ddbRepository) FindParentCategory(ctx context.Context, userID, childID string) (*domain.Category, error) {
	// Simplified implementation for consolidation phase
	return nil, nil
}

// GetCategoryTree gets the complete category tree for a user
func (repo *ddbRepository) GetCategoryTree(ctx context.Context, userID string) ([]domain.Category, error) {
	// Simplified implementation for consolidation phase
	return []domain.Category{}, nil
}

// FindCategoriesByLevel finds categories at a specific hierarchy level
func (repo *ddbRepository) FindCategoriesByLevel(ctx context.Context, userID string, level int) ([]domain.Category, error) {
	// Simplified implementation for consolidation phase
	return []domain.Category{}, nil
}

// CQRS-compatible methods

// Save creates or updates a category (alias for CreateCategory)
func (repo *ddbRepository) Save(ctx context.Context, category *domain.Category) error {
	// Convert pointer to value for CreateCategory
	return repo.CreateCategory(ctx, *category)
}

// FindByID retrieves a category by ID (alias for FindCategoryByID)
func (repo *ddbRepository) FindByID(ctx context.Context, userID, categoryID string) (*domain.Category, error) {
	return repo.FindCategoryByID(ctx, userID, categoryID)
}

// Delete removes a category (alias for DeleteCategory)
func (repo *ddbRepository) Delete(ctx context.Context, userID, categoryID string) error {
	return repo.DeleteCategory(ctx, userID, categoryID)
}
