// Package dynamodb implements the repository interface using AWS DynamoDB.
// This is the infrastructure layer that contains DynamoDB-specific implementations.
package dynamodb

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"brain2-backend/internal/domain"
	"brain2-backend/internal/repository"
	appErrors "brain2-backend/pkg/errors" // ALIAS for our custom errors

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
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
}

// NewRepository creates a new instance of the DynamoDB repository.
func NewRepository(dbClient *dynamodb.Client, tableName, indexName string) repository.Repository {
	config := repository.NewConfig(tableName, indexName)
	return &ddbRepository{
		dbClient: dbClient,
		config:   config,
	}
}

// NewRepositoryWithConfig creates a new instance of the DynamoDB repository with custom config.
func NewRepositoryWithConfig(dbClient *dynamodb.Client, config repository.Config) repository.Repository {
	return &ddbRepository{
		dbClient: dbClient,
		config:   config.WithDefaults(),
	}
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
func (r *ddbRepository) CreateNodeAndKeywords(ctx context.Context, node domain.Node) error {
	pk := fmt.Sprintf("USER#%s#NODE#%s", node.UserID, node.ID)
	transactItems := []types.TransactWriteItem{}

	// 1. Add the main node metadata to the transaction
	nodeItem, err := attributevalue.MarshalMap(ddbNode{
		PK: pk, SK: "METADATA#v0", NodeID: node.ID, UserID: node.UserID, Content: node.Content,
		Keywords: node.Keywords, Tags: node.Tags, IsLatest: true, Version: node.Version, Timestamp: node.CreatedAt.Format(time.RFC3339),
	})
	if err != nil {
		return appErrors.Wrap(err, "failed to marshal node item")
	}
	transactItems = append(transactItems, types.TransactWriteItem{
		Put: &types.Put{TableName: aws.String(r.config.TableName), Item: nodeItem},
	})

	// 2. Add each keyword as a separate item for the GSI to index
	for _, keyword := range node.Keywords {
		keywordItem, err := attributevalue.MarshalMap(ddbKeyword{
			PK:     pk,
			SK:     fmt.Sprintf("KEYWORD#%s", keyword),
			GSI1PK: fmt.Sprintf("USER#%s#KEYWORD#%s", node.UserID, keyword),
			GSI1SK: fmt.Sprintf("NODE#%s", node.ID),
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
func (r *ddbRepository) CreateNodeWithEdges(ctx context.Context, node domain.Node, relatedNodeIDs []string) error {
	log.Printf("DEBUG CreateNodeWithEdges: creating node ID=%s with keywords=%v and %d edges", node.ID, node.Keywords, len(relatedNodeIDs))
	
	pk := fmt.Sprintf("USER#%s#NODE#%s", node.UserID, node.ID)
	transactItems := []types.TransactWriteItem{}

	nodeItem, err := attributevalue.MarshalMap(ddbNode{
		PK: pk, SK: "METADATA#v0", NodeID: node.ID, UserID: node.UserID, Content: node.Content,
		Keywords: node.Keywords, Tags: node.Tags, IsLatest: true, Version: node.Version, Timestamp: node.CreatedAt.Format(time.RFC3339),
	})
	if err != nil {
		return appErrors.Wrap(err, "failed to marshal node item")
	}
	transactItems = append(transactItems, types.TransactWriteItem{Put: &types.Put{TableName: aws.String(r.config.TableName), Item: nodeItem}})
	log.Printf("DEBUG CreateNodeWithEdges: added node item with PK=%s", pk)

	for _, keyword := range node.Keywords {
		gsi1PK := fmt.Sprintf("USER#%s#KEYWORD#%s", node.UserID, keyword)
		gsi1SK := fmt.Sprintf("NODE#%s", node.ID)
		
		keywordItem, err := attributevalue.MarshalMap(ddbKeyword{
			PK: pk, SK: fmt.Sprintf("KEYWORD#%s", keyword), GSI1PK: gsi1PK, GSI1SK: gsi1SK,
		})
		if err != nil {
			return appErrors.Wrap(err, "failed to marshal keyword item")
		}
		transactItems = append(transactItems, types.TransactWriteItem{Put: &types.Put{TableName: aws.String(r.config.TableName), Item: keywordItem}})
		log.Printf("DEBUG CreateNodeWithEdges: added keyword item for '%s' with GSI1PK=%s, GSI1SK=%s", keyword, gsi1PK, gsi1SK)
	}

	// Create canonical edges - only one edge per connection
	for _, relatedNodeID := range relatedNodeIDs {
		ownerID, targetID := getCanonicalEdge(node.ID, relatedNodeID)
		ownerPK := fmt.Sprintf("USER#%s#NODE#%s", node.UserID, ownerID)

		edgeItem, err := attributevalue.MarshalMap(ddbEdge{
			PK:       ownerPK,
			SK:       fmt.Sprintf("EDGE#RELATES_TO#%s", targetID),
			TargetID: targetID,
		})
		if err != nil {
			return appErrors.Wrap(err, "failed to marshal canonical edge item")
		}
		transactItems = append(transactItems, types.TransactWriteItem{Put: &types.Put{TableName: aws.String(r.config.TableName), Item: edgeItem}})
		log.Printf("DEBUG CreateNodeWithEdges: added edge from %s to %s (canonical: ownerPK=%s)", node.ID, relatedNodeID, ownerPK)
	}

	log.Printf("DEBUG CreateNodeWithEdges: executing transaction with %d items", len(transactItems))
	_, err = r.dbClient.TransactWriteItems(ctx, &dynamodb.TransactWriteItemsInput{TransactItems: transactItems})
	if err != nil {
		log.Printf("ERROR CreateNodeWithEdges: transaction failed: %v", err)
		return appErrors.Wrap(err, "transaction to create node with edges failed")
	}
	
	log.Printf("DEBUG CreateNodeWithEdges: transaction completed successfully for node %s", node.ID)
	return nil
}

// UpdateNodeAndEdges transactionally updates a node and its connections.
func (r *ddbRepository) UpdateNodeAndEdges(ctx context.Context, node domain.Node, relatedNodeIDs []string) error {
	if err := r.clearNodeConnections(ctx, node.UserID, node.ID); err != nil {
		return appErrors.Wrap(err, "failed to clear old connections for update")
	}

	pk := fmt.Sprintf("USER#%s#NODE#%s", node.UserID, node.ID)
	transactItems := []types.TransactWriteItem{}

	// Optimistic locking: check that the version matches before updating
	_, err := r.dbClient.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName:           aws.String(r.config.TableName),
		Key:                 map[string]types.AttributeValue{"PK": &types.AttributeValueMemberS{Value: pk}, "SK": &types.AttributeValueMemberS{Value: "METADATA#v0"}},
		UpdateExpression:    aws.String("SET Content = :c, Keywords = :k, Tags = :tg, Timestamp = :t, Version = Version + :inc"),
		ConditionExpression: aws.String("Version = :expected_version"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":c":                &types.AttributeValueMemberS{Value: node.Content},
			":k":                &types.AttributeValueMemberL{Value: toAttributeValueList(node.Keywords)},
			":tg":               &types.AttributeValueMemberL{Value: toAttributeValueList(node.Tags)},
			":t":                &types.AttributeValueMemberS{Value: time.Now().Format(time.RFC3339)},
			":expected_version": &types.AttributeValueMemberN{Value: fmt.Sprintf("%d", node.Version)},
			":inc":              &types.AttributeValueMemberN{Value: "1"},
		},
	})
	if err != nil {
		// Check for optimistic lock conflicts
		var ccf *types.ConditionalCheckFailedException
		if errors.As(err, &ccf) {
			return repository.NewOptimisticLockError(node.ID, node.Version, node.Version+1)
		}
		return appErrors.Wrap(err, "failed to update node metadata")
	}

	for _, keyword := range node.Keywords {
		keywordItem, err := attributevalue.MarshalMap(ddbKeyword{PK: pk, SK: fmt.Sprintf("KEYWORD#%s", keyword), GSI1PK: fmt.Sprintf("USER#%s#KEYWORD#%s", node.UserID, keyword), GSI1SK: fmt.Sprintf("NODE#%s", node.ID)})
		if err != nil {
			return appErrors.Wrap(err, "failed to marshal keyword item for update")
		}
		transactItems = append(transactItems, types.TransactWriteItem{Put: &types.Put{TableName: aws.String(r.config.TableName), Item: keywordItem}})
	}

	// Create canonical edges - only one edge per connection
	for _, relatedNodeID := range relatedNodeIDs {
		ownerID, targetID := getCanonicalEdge(node.ID, relatedNodeID)
		ownerPK := fmt.Sprintf("USER#%s#NODE#%s", node.UserID, ownerID)

		edgeItem, err := attributevalue.MarshalMap(ddbEdge{
			PK:       ownerPK,
			SK:       fmt.Sprintf("EDGE#RELATES_TO#%s", targetID),
			TargetID: targetID,
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

// FindNodesByKeywords uses the GSI to find nodes with matching keywords.
func (r *ddbRepository) FindNodesByKeywords(ctx context.Context, userID string, keywords []string) ([]domain.Node, error) {
	log.Printf("DEBUG FindNodesByKeywords: called with userID=%s, keywords=%v", userID, keywords)
	
	nodeIdMap := make(map[string]bool)
	var nodes []domain.Node
	for _, keyword := range keywords {
		gsiPK := fmt.Sprintf("USER#%s#KEYWORD#%s", userID, keyword)
		log.Printf("DEBUG FindNodesByKeywords: querying GSI with gsiPK=%s", gsiPK)
		
		result, err := r.dbClient.Query(ctx, &dynamodb.QueryInput{
			TableName: aws.String(r.config.TableName), IndexName: aws.String(r.config.IndexName), KeyConditionExpression: aws.String("GSI1PK = :gsiPK"),
			ExpressionAttributeValues: map[string]types.AttributeValue{":gsiPK": &types.AttributeValueMemberS{Value: gsiPK}},
		})
		if err != nil {
			log.Printf("ERROR FindNodesByKeywords: failed to query GSI for keyword %s: %v", keyword, err)
			continue
		}
		
		log.Printf("DEBUG FindNodesByKeywords: GSI query for keyword '%s' returned %d items", keyword, len(result.Items))
		
		for _, item := range result.Items {
			pkValue := item["PK"].(*types.AttributeValueMemberS).Value
			nodeID := strings.Split(pkValue, "#")[3]
			log.Printf("DEBUG FindNodesByKeywords: found nodeID=%s from keyword '%s' (PK=%s)", nodeID, keyword, pkValue)
			
			if _, exists := nodeIdMap[nodeID]; !exists {
				nodeIdMap[nodeID] = true
				node, err := r.FindNodeByID(ctx, userID, nodeID)
				if err != nil {
					log.Printf("ERROR FindNodesByKeywords: failed to find node %s from keyword search: %v", nodeID, err)
					continue
				}
				if node != nil {
					log.Printf("DEBUG FindNodesByKeywords: successfully retrieved node ID=%s, content='%s'", node.ID, node.Content)
					nodes = append(nodes, *node)
				} else {
					log.Printf("WARN FindNodesByKeywords: node %s was nil after FindNodeByID", nodeID)
				}
			} else {
				log.Printf("DEBUG FindNodesByKeywords: nodeID=%s already processed, skipping", nodeID)
			}
		}
	}
	
	log.Printf("DEBUG FindNodesByKeywords: returning %d unique nodes", len(nodes))
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
	return &domain.Node{
		ID: ddbItem.NodeID, UserID: ddbItem.UserID, Content: ddbItem.Content,
		Keywords: ddbItem.Keywords, Tags: ddbItem.Tags, CreatedAt: createdAt, Version: ddbItem.Version,
	}, nil
}

// FindEdgesByNode queries for all outgoing edges from a given node.
func (r *ddbRepository) FindEdgesByNode(ctx context.Context, userID, nodeID string) ([]domain.Edge, error) {
	var edges []domain.Edge

	// With canonical edge storage, we need to look for edges in two ways:
	// 1. Where this node is the "owner" (stored in this node's partition)
	// 2. Where this node is the "target" (stored in other nodes' partitions)

	// First, check edges where this node is the owner
	pk := fmt.Sprintf("USER#%s#NODE#%s", userID, nodeID)
	result, err := r.dbClient.Query(ctx, &dynamodb.QueryInput{
		TableName:              aws.String(r.config.TableName),
		KeyConditionExpression: aws.String("PK = :pk AND begins_with(SK, :skPrefix)"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":pk":       &types.AttributeValueMemberS{Value: pk},
			":skPrefix": &types.AttributeValueMemberS{Value: "EDGE#RELATES_TO#"},
		},
	})
	if err != nil {
		return nil, appErrors.Wrap(err, "failed to query edges where node is owner")
	}

	// Process edges where this node is the owner
	for _, item := range result.Items {
		var ddbItem ddbEdge
		if err := attributevalue.UnmarshalMap(item, &ddbItem); err == nil {
			edges = append(edges, domain.Edge{SourceID: nodeID, TargetID: ddbItem.TargetID})
		}
	}

	// Second, scan for edges where this node is the target (stored in other nodes' partitions)
	// This requires scanning all user's node partitions for edges pointing to this node
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
		return nil, appErrors.Wrap(err, "failed to scan for edges where node is target")
	}

	// Process edges where this node is the target
	for _, item := range scanResult.Items {
		var ddbItem ddbEdge
		if err := attributevalue.UnmarshalMap(item, &ddbItem); err == nil {
			// Extract source node ID from PK
			pkParts := strings.Split(ddbItem.PK, "#")
			if len(pkParts) >= 4 {
				sourceID := pkParts[3]
				// Only add if not already added (avoid duplicates)
				alreadyAdded := false
				for _, existingEdge := range edges {
					if (existingEdge.SourceID == sourceID && existingEdge.TargetID == nodeID) ||
						(existingEdge.SourceID == nodeID && existingEdge.TargetID == sourceID) {
						alreadyAdded = true
						break
					}
				}
				if !alreadyAdded {
					edges = append(edges, domain.Edge{SourceID: sourceID, TargetID: nodeID})
				}
			}
		}
	}

	return edges, nil
}

// DeleteNode transactionally deletes a node, its keywords, and outgoing edges.
func (r *ddbRepository) DeleteNode(ctx context.Context, userID, nodeID string) error {
	return r.clearNodeConnections(ctx, userID, nodeID)
}

// GetAllGraphData retrieves all nodes and edges for a user.
func (r *ddbRepository) GetAllGraphData(ctx context.Context, userID string) (*domain.Graph, error) {
	log.Println("Starting GetAllGraphData")

	scanInput := &dynamodb.ScanInput{
		TableName:        aws.String(r.config.TableName),
		FilterExpression: aws.String("begins_with(PK, :pkPrefix)"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":pkPrefix": &types.AttributeValueMemberS{Value: fmt.Sprintf("USER#%s#", userID)},
		},
	}

	var nodes []domain.Node
	var edges []domain.Edge
	edgeMap := make(map[string]bool)
	totalItemsScanned := 0

	paginator := dynamodb.NewScanPaginator(r.dbClient, scanInput)
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			log.Printf("ERROR: failed to scan graph data page: %v", err)
			return nil, appErrors.Wrap(err, "failed to scan graph data page")
		}

		totalItemsScanned += len(page.Items)

		for _, item := range page.Items {
			skValueAttr, ok := item["SK"].(*types.AttributeValueMemberS)
			if !ok {
				continue
			}
			skValue := skValueAttr.Value

			pkValueAttr, ok := item["PK"].(*types.AttributeValueMemberS)
			if !ok {
				continue
			}
			pkValue := pkValueAttr.Value

			if strings.HasPrefix(skValue, "METADATA#") {
				var ddbItem ddbNode
				if err := attributevalue.UnmarshalMap(item, &ddbItem); err == nil {
					createdAt, _ := time.Parse(time.RFC3339, ddbItem.Timestamp)
					nodes = append(nodes, domain.Node{
						ID:        ddbItem.NodeID,
						UserID:    ddbItem.UserID,
						Content:   ddbItem.Content,
						Keywords:  ddbItem.Keywords,
						Tags:      ddbItem.Tags,
						CreatedAt: createdAt,
						Version:   ddbItem.Version,
					})
				}
			} else if strings.HasPrefix(skValue, "EDGE#RELATES_TO#") {
				var ddbItem ddbEdge
				if err := attributevalue.UnmarshalMap(item, &ddbItem); err == nil {
					pkParts := strings.Split(pkValue, "#")
					if len(pkParts) == 4 {
						sourceID := pkParts[3]
						edgeKey := fmt.Sprintf("%s-%s", sourceID, ddbItem.TargetID)
						reverseKey := fmt.Sprintf("%s-%s", ddbItem.TargetID, sourceID)
						if !edgeMap[edgeKey] && !edgeMap[reverseKey] {
							edgeMap[edgeKey] = true
							edges = append(edges, domain.Edge{SourceID: sourceID, TargetID: ddbItem.TargetID})
						}
					}
				}
			}
		}
	}

	log.Printf("Finished GetAllGraphData. Scanned %d total items, found %d nodes and %d unique edges.", totalItemsScanned, len(nodes), len(edges))

	return &domain.Graph{Nodes: nodes, Edges: edges}, nil
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
func (r *ddbRepository) FindNodes(ctx context.Context, query repository.NodeQuery) ([]domain.Node, error) {
	log.Printf("DEBUG FindNodes: called with userID=%s, keywords=%v, nodeIDs=%v", query.UserID, query.Keywords, query.NodeIDs)
	
	if err := query.Validate(); err != nil {
		log.Printf("ERROR FindNodes: query validation failed: %v", err)
		return nil, err
	}

	// If specific node IDs are requested, fetch them directly
	if query.HasNodeIDs() {
		log.Printf("DEBUG FindNodes: using nodeID-based lookup for %d nodes", len(query.NodeIDs))
		var nodes []domain.Node
		for _, nodeID := range query.NodeIDs {
			node, err := r.FindNodeByID(ctx, query.UserID, nodeID)
			if err != nil {
				return nil, err
			}
			if node != nil {
				nodes = append(nodes, *node)
			}
		}
		log.Printf("DEBUG FindNodes: found %d nodes by nodeID lookup", len(nodes))
		return nodes, nil
	}

	// If keywords are specified, use keyword search
	if query.HasKeywords() {
		log.Printf("DEBUG FindNodes: using keyword-based lookup with keywords=%v", query.Keywords)
		nodes, err := r.FindNodesByKeywords(ctx, query.UserID, query.Keywords)
		log.Printf("DEBUG FindNodes: keyword lookup returned %d nodes, error=%v", len(nodes), err)
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
			return []domain.Node{}, nil
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
func (r *ddbRepository) FindEdges(ctx context.Context, query repository.EdgeQuery) ([]domain.Edge, error) {
	if err := query.Validate(); err != nil {
		return nil, err
	}

	var edges []domain.Edge

	// If specific node IDs are requested, find edges for each
	if query.HasNodeIDs() {
		for _, nodeID := range query.NodeIDs {
			nodeEdges, err := r.FindEdgesByNode(ctx, query.UserID, nodeID)
			if err != nil {
				return nil, err
			}
			edges = append(edges, nodeEdges...)
		}
		return edges, nil
	}

	// If source node is specified, find outgoing edges
	if query.HasSourceFilter() {
		return r.FindEdgesByNode(ctx, query.UserID, query.SourceID)
	}

	// If target node is specified, we need to scan for incoming edges
	if query.HasTargetFilter() {
		// This is less efficient but necessary for target-based queries
		graph, err := r.GetAllGraphData(ctx, query.UserID)
		if err != nil {
			return nil, err
		}

		for _, edge := range graph.Edges {
			if edge.TargetID == query.TargetID {
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
			return []domain.Edge{}, nil
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
		var nodes []domain.Node
		var edges []domain.Edge

		// Get specific nodes
		for _, nodeID := range query.NodeIDs {
			node, err := r.FindNodeByID(ctx, query.UserID, nodeID)
			if err != nil {
				return nil, err
			}
			if node != nil {
				nodes = append(nodes, *node)

				// Include edges if requested
				if query.IncludeEdges {
					nodeEdges, err := r.FindEdgesByNode(ctx, query.UserID, nodeID)
					if err != nil {
						return nil, err
					}
					edges = append(edges, nodeEdges...)
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
	pk := fmt.Sprintf("USER#%s#NODE#%s", node.UserID, node.ID)
	nodeItem, err := attributevalue.MarshalMap(ddbNode{
		PK: pk, SK: "METADATA#v0", NodeID: node.ID, UserID: node.UserID, Content: node.Content,
		Keywords: node.Keywords, Tags: node.Tags, IsLatest: true, Version: 0, Timestamp: node.CreatedAt.Format(time.RFC3339),
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
	return r.CreateEnhancedCategory(ctx, category)
}

// UpdateCategory updates an existing category with enhanced hierarchical support.
func (r *ddbRepository) UpdateCategory(ctx context.Context, category domain.Category) error {
	return r.UpdateEnhancedCategory(ctx, category)
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

	var ddbItem ddbEnhancedCategory
	if err := attributevalue.UnmarshalMap(result.Item, &ddbItem); err != nil {
		return nil, appErrors.Wrap(err, "failed to unmarshal enhanced category item")
	}

	category := r.toDomainCategory(ddbItem)
	return &category, nil
}

// FindCategories retrieves categories based on query parameters.
func (r *ddbRepository) FindCategories(ctx context.Context, query repository.CategoryQuery) ([]domain.Category, error) {
	if err := query.Validate(); err != nil {
		return nil, err
	}

	// Scan for all categories for the user using enhanced format
	scanInput := &dynamodb.ScanInput{
		TableName:        aws.String(r.config.TableName),
		FilterExpression: aws.String("PK = :pk AND begins_with(SK, :skPrefix)"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":pk":       &types.AttributeValueMemberS{Value: fmt.Sprintf("USER#%s", query.UserID)},
			":skPrefix": &types.AttributeValueMemberS{Value: "CATEGORY#"},
		},
	}

	var categories []domain.Category
	paginator := dynamodb.NewScanPaginator(r.dbClient, scanInput)
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, appErrors.Wrap(err, "failed to scan categories")
		}

		for _, item := range page.Items {
			var ddbItem ddbEnhancedCategory
			if err := attributevalue.UnmarshalMap(item, &ddbItem); err == nil {
				category := r.toDomainCategory(ddbItem)
				categories = append(categories, category)
			}
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

// GetNodesPage retrieves a paginated list of nodes for a user
func (r *ddbRepository) GetNodesPage(ctx context.Context, query repository.NodeQuery, pagination repository.Pagination) (*repository.NodePage, error) {
	if err := query.Validate(); err != nil {
		return nil, err
	}
	if err := pagination.Validate(); err != nil {
		return nil, err
	}

	// Use Scan with filter to find nodes - nodes are stored with PK pattern USER#<userID>#NODE#<nodeID>
	scanInput := &dynamodb.ScanInput{
		TableName:        aws.String(r.config.TableName),
		FilterExpression: aws.String("begins_with(PK, :pk_prefix) AND begins_with(SK, :sk_prefix)"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":pk_prefix": &types.AttributeValueMemberS{Value: fmt.Sprintf("USER#%s#NODE#", query.UserID)},
			":sk_prefix": &types.AttributeValueMemberS{Value: "METADATA#"},
		},
		Limit: aws.Int32(int32(pagination.GetEffectiveLimit())),
	}

	// Handle cursor-based pagination
	if pagination.HasCursor() {
		startKey, err := repository.DecodeCursor(pagination.Cursor)
		if err == nil && startKey != nil {
			scanInput.ExclusiveStartKey = startKey
		}
	}

	result, err := r.dbClient.Scan(ctx, scanInput)
	if err != nil {
		return nil, appErrors.Wrap(err, "failed to scan nodes page")
	}

	// Convert DynamoDB items to domain nodes
	nodes := make([]domain.Node, 0, len(result.Items))
	for _, item := range result.Items {
		var ddbItem ddbNode
		if err := attributevalue.UnmarshalMap(item, &ddbItem); err == nil {
			if ddbItem.IsLatest { // Only return latest versions
				createdAt, _ := time.Parse(time.RFC3339, ddbItem.Timestamp)
				nodes = append(nodes, domain.Node{
					ID:        ddbItem.NodeID,
					UserID:    ddbItem.UserID,
					Content:   ddbItem.Content,
					Keywords:  ddbItem.Keywords,
					Tags:      ddbItem.Tags,
					CreatedAt: createdAt,
					Version:   ddbItem.Version,
				})
			}
		}
	}

	return &repository.NodePage{
		Items:      nodes,
		HasMore:    result.LastEvaluatedKey != nil,
		NextCursor: repository.EncodeCursor(result.LastEvaluatedKey),
		PageInfo:   repository.CreatePageInfo(pagination, len(nodes), result.LastEvaluatedKey != nil),
	}, nil
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
	nodes := make(map[string]domain.Node)
	edges := make([]domain.Edge, 0)

	// Start with the target node
	currentLevel := []string{nodeID}
	visited[nodeID] = true

	for currentDepth := 0; currentDepth < depth && len(currentLevel) > 0; currentDepth++ {
		var nextLevel []string

		for _, currentNodeID := range currentLevel {
			// Get the node details
			node, err := r.FindNodeByID(ctx, userID, currentNodeID)
			if err == nil && node != nil {
				nodes[currentNodeID] = *node
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
					edges = append(edges, domain.Edge{
						SourceID: currentNodeID,
						TargetID: ddbEdge.TargetID,
					})

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
	nodeSlice := make([]domain.Node, 0, len(nodes))
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
	edges := make([]domain.Edge, 0, len(result.Items))
	for _, item := range result.Items {
		var ddbItem ddbEdge
		if err := attributevalue.UnmarshalMap(item, &ddbItem); err == nil {
			// Extract source node ID from PK
			pkParts := strings.Split(ddbItem.PK, "#")
			if len(pkParts) >= 4 {
				sourceID := pkParts[3]
				edges = append(edges, domain.Edge{
					SourceID: sourceID,
					TargetID: ddbItem.TargetID,
				})
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
	log.Printf("DEBUG: GetGraphDataPaginated called for userID: %s, includeEdges: %t, limit: %d", query.UserID, query.IncludeEdges, pagination.GetEffectiveLimit())

	if err := query.Validate(); err != nil {
		log.Printf("ERROR: Query validation failed: %v", err)
		return nil, "", err
	}
	if err := pagination.Validate(); err != nil {
		log.Printf("ERROR: Pagination validation failed: %v", err)
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

	log.Printf("DEBUG: DynamoDB scan input - TableName: %s, filtering for USER#%s#NODE# prefix", r.config.TableName, query.UserID)

	// Handle cursor-based pagination
	if pagination.HasCursor() {
		startKey, err := repository.DecodeCursor(pagination.Cursor)
		if err == nil && startKey != nil {
			scanInput.ExclusiveStartKey = startKey
		}
	}

	result, err := r.dbClient.Scan(ctx, scanInput)
	if err != nil {
		log.Printf("ERROR: DynamoDB query failed: %v", err)
		return nil, "", appErrors.Wrap(err, "failed to query graph data")
	}

	log.Printf("DEBUG: DynamoDB query successful - returned %d items, LastEvaluatedKey: %v", len(result.Items), result.LastEvaluatedKey != nil)

	var nodes []domain.Node
	var edges []domain.Edge
	edgeMap := make(map[string]bool)

	nodeCount := 0
	edgeCount := 0
	skippedItems := 0

	for _, item := range result.Items {
		skValueAttr, ok := item["SK"].(*types.AttributeValueMemberS)
		if !ok {
			log.Printf("WARN: Item missing SK attribute: %v", item)
			skippedItems++
			continue
		}
		skValue := skValueAttr.Value

		if strings.HasPrefix(skValue, "METADATA#") {
			var ddbItem ddbNode
			if err := attributevalue.UnmarshalMap(item, &ddbItem); err != nil {
				log.Printf("ERROR: Failed to unmarshal node item: %v, item: %v", err, item)
				skippedItems++
				continue
			}
			if ddbItem.IsLatest {
				createdAt, parseErr := time.Parse(time.RFC3339, ddbItem.Timestamp)
				if parseErr != nil {
					log.Printf("WARN: Failed to parse timestamp %s: %v", ddbItem.Timestamp, parseErr)
					createdAt = time.Now() // fallback
				}
				nodes = append(nodes, domain.Node{
					ID:        ddbItem.NodeID,
					UserID:    ddbItem.UserID,
					Content:   ddbItem.Content,
					Keywords:  ddbItem.Keywords,
					Tags:      ddbItem.Tags,
					CreatedAt: createdAt,
					Version:   ddbItem.Version,
				})
				nodeCount++
			}
		} else if strings.HasPrefix(skValue, "EDGE#") && query.IncludeEdges {
			var ddbItem ddbEdge
			if err := attributevalue.UnmarshalMap(item, &ddbItem); err != nil {
				log.Printf("ERROR: Failed to unmarshal edge item: %v, item: %v", err, item)
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
					edges = append(edges, domain.Edge{
						SourceID: ownerID,
						TargetID: targetID,
					})
					edgeCount++
				}
			} else {
				log.Printf("WARN: Invalid PK format for edge: %s", ddbItem.PK)
			}
		}
	}

	log.Printf("DEBUG: Processed data - nodes: %d, edges: %d, skipped items: %d", nodeCount, edgeCount, skippedItems)

	nextCursor := repository.EncodeCursor(result.LastEvaluatedKey)

	log.Printf("DEBUG: GetGraphDataPaginated completed - returning %d nodes, %d edges, nextCursor: %s", len(nodes), len(edges), nextCursor)

	return &domain.Graph{
		Nodes: nodes,
		Edges: edges,
	}, nextCursor, nil
}
