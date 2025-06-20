package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/google/uuid"
)

type NodeMetadata struct {
	PK        string `dynamodbav:"PK"`
	SK        string `dynamodbav:"SK"`
	NodeID    string `dynamodbav:"NodeID"`
	Content   string `dynamodbav:"Content"`
	IsLatest  bool   `dynamodbav:"IsLatest"`
	Version   int    `dynamodbav:"Version"`
	Timestamp string `dynamodbav:"Timestamp"`
	GSI1PK    string `dynamodbav:"GSI1PK,omitempty"`
	GSI1SK    string `dynamodbav:"GSI1SK,omitempty"`
}

type Edge struct {
	PK        string `dynamodbav:"PK"`
	SK        string `dynamodbav:"SK"`
	TargetID  string `dynamodbav:"TargetID"`
	Timestamp string `dynamodbav:"Timestamp"`
}

type CreateNodeRequest struct {
	Content string `json:"content"`
}

type UpdateNodeRequest struct {
	Content string `json:"content"`
}

var stopWords = map[string]bool{
	"the": true, "a": true, "an": true, "and": true, "or": true, "but": true,
	"in": true, "on": true, "at": true, "to": true, "for": true, "of": true,
	"with": true, "by": true, "from": true, "up": true, "about": true, "into": true,
	"through": true, "during": true, "before": true, "after": true, "above": true,
	"below": true, "between": true, "under": true, "again": true, "further": true,
	"then": true, "once": true, "is": true, "am": true, "are": true, "was": true,
	"were": true, "be": true, "been": true, "being": true, "have": true, "has": true,
	"had": true, "do": true, "does": true, "did": true, "will": true, "would": true,
	"should": true, "could": true, "ought": true, "i": true, "me": true, "my": true,
	"myself": true, "we": true, "our": true, "ours": true, "ourselves": true, "you": true,
	"your": true, "yours": true, "yourself": true, "yourselves": true, "he": true,
	"him": true, "his": true, "himself": true, "she": true, "her": true, "hers": true,
	"herself": true, "it": true, "its": true, "itself": true, "they": true, "them": true,
	"their": true, "theirs": true, "themselves": true, "what": true, "which": true,
	"who": true, "whom": true, "this": true, "that": true, "these": true, "those": true,
	"as": true, "if": true, "each": true, "how": true, "than": true, "too": true,
	"very": true, "can": true, "just": true, "also": true,
}

func (app *App) createNode(ctx context.Context, userID, body string) (Response, error) {
	log.Printf("createNode called with userID: %s, body: %s", userID, body)

	var req CreateNodeRequest
	if err := json.Unmarshal([]byte(body), &req); err != nil {
		log.Printf("Failed to unmarshal request body: %v", err)
		return errorResponse(400, "Invalid request body"), nil
	}

	log.Printf("Parsed request: Content=%s", req.Content)

	if req.Content == "" {
		return errorResponse(400, "Content cannot be empty"), nil
	}

	nodeID := uuid.New().String()
	timestamp := time.Now().UTC().Format(time.RFC3339)

	log.Printf("Generated nodeID: %s, timestamp: %s", nodeID, timestamp)

	// Create the node metadata
	node := NodeMetadata{
		PK:        fmt.Sprintf("USER#%s#NODE#%s", userID, nodeID),
		SK:        "METADATA#v0",
		NodeID:    nodeID,
		Content:   req.Content,
		IsLatest:  true,
		Version:   0,
		Timestamp: timestamp,
	}

	log.Printf("Created node struct: PK=%s, SK=%s", node.PK, node.SK)

	nodeAV, err := attributevalue.MarshalMap(node)
	if err != nil {
		log.Printf("Failed to marshal node: %v", err)
		return errorResponse(500, "Failed to process node"), nil
	}

	log.Printf("Marshaled node successfully, table name: %s", app.tableName)

	// Put the node
	_, err = app.db.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(app.tableName),
		Item:      nodeAV,
	})
	if err != nil {
		log.Printf("Failed to put item in DynamoDB: %v", err)
		return errorResponse(500, "Failed to create node"), nil
	}

	log.Printf("Node created successfully in DynamoDB")

	// Extract keywords and create connections
	keywords := extractKeywords(req.Content)
	log.Printf("Extracted keywords: %v", keywords)

	// --- Start of FIX ---
	// Return an error if indexing or connecting fails.
	if err := app.indexNodeKeywords(ctx, userID, nodeID, keywords); err != nil {
		log.Printf("Failed to index keywords, will not connect node: %v", err)
		// This is now a critical error. The node was created, but is not indexed.
		return errorResponse(500, "Failed to index new memory node."), nil
	}
	log.Printf("Successfully indexed keywords")

	if err := app.connectNode(ctx, userID, nodeID, keywords); err != nil {
		log.Printf("Failed to connect node to existing nodes: %v", err)
		// Also a critical error.
		return errorResponse(500, "Failed to connect new memory to existing ones."), nil
	}
	log.Printf("Successfully connected node")
	// --- End of FIX ---

	log.Printf("Returning success response for nodeID: %s", nodeID)
	return successResponse(map[string]interface{}{
		"nodeId":    nodeID,
		"timestamp": timestamp,
	}), nil
}

func extractKeywords(content string) []string {
	// Convert to lowercase
	content = strings.ToLower(content)

	// Remove non-alphanumeric characters
	reg := regexp.MustCompile(`[^a-zA-Z0-9 ]+`)
	content = reg.ReplaceAllString(content, "")

	// Split into words
	words := strings.Fields(content)

	// Filter stop words and create unique set
	uniqueWords := make(map[string]bool)
	for _, word := range words {
		if !stopWords[word] && len(word) > 2 {
			uniqueWords[word] = true
		}
	}

	// Convert to slice
	keywords := make([]string, 0, len(uniqueWords))
	for word := range uniqueWords {
		keywords = append(keywords, word)
	}

	return keywords
}

func (app *App) indexNodeKeywords(ctx context.Context, userID, nodeID string, keywords []string) error {
	timestamp := time.Now().UTC().Format(time.RFC3339)
	log.Printf("Indexing %d keywords for node %s", len(keywords), nodeID)

	for _, keyword := range keywords {
		item := map[string]types.AttributeValue{
			"PK":        &types.AttributeValueMemberS{Value: fmt.Sprintf("USER#%s#NODE#%s", userID, nodeID)},
			"SK":        &types.AttributeValueMemberS{Value: fmt.Sprintf("KEYWORD#%s", keyword)},
			"GSI1PK":    &types.AttributeValueMemberS{Value: fmt.Sprintf("KEYWORD#%s", keyword)},
			"GSI1SK":    &types.AttributeValueMemberS{Value: fmt.Sprintf("NODE#%s", nodeID)},
			"Timestamp": &types.AttributeValueMemberS{Value: timestamp},
		}

		log.Printf("Indexing keyword: %s", keyword)
		_, err := app.db.PutItem(ctx, &dynamodb.PutItemInput{
			TableName: aws.String(app.tableName),
			Item:      item,
		})
		if err != nil {
			log.Printf("Failed to index keyword %s: %v", keyword, err)
			return err
		}
	}

	return nil
}

// connectNode finds related nodes based on keywords and creates bidirectional edges concurrently.
func (app *App) connectNode(ctx context.Context, userID, nodeID string, keywords []string) error {
	relatedNodes := make(map[string]bool)
	log.Printf("Connecting node %s using %d keywords", nodeID, len(keywords))

	// Find all nodes with matching keywords
	for _, keyword := range keywords {
		log.Printf("Finding nodes with keyword: %s", keyword)
		nodes, err := app.findNodesByKeyword(ctx, userID, keyword)
		if err != nil {
			log.Printf("Failed to find nodes for keyword %s: %v", keyword, err)
			continue // Continue to the next keyword on error
		}

		log.Printf("Found %d nodes for keyword %s", len(nodes), keyword)
		for _, relatedNodeID := range nodes {
			if relatedNodeID != nodeID {
				relatedNodes[relatedNodeID] = true
			}
		}
	}

	if len(relatedNodes) == 0 {
		log.Printf("No related nodes found for node %s. No edges will be created.", nodeID)
		return nil
	}

	log.Printf("Total unique related nodes found: %d. Creating edges concurrently.", len(relatedNodes))

	// Create bidirectional edges concurrently
	var wg sync.WaitGroup
	errChan := make(chan error, len(relatedNodes)*2) // Buffered channel to prevent blocking
	timestamp := time.Now().UTC().Format(time.RFC3339)

	for relatedNodeID := range relatedNodes {
		wg.Add(1)
		go func(relatedNodeID string) {
			defer wg.Done()

			// Edge from new node to related node
			edge1 := map[string]types.AttributeValue{
				"PK":        &types.AttributeValueMemberS{Value: fmt.Sprintf("USER#%s#NODE#%s", userID, nodeID)},
				"SK":        &types.AttributeValueMemberS{Value: fmt.Sprintf("EDGE#RELATES_TO#NODE#%s", relatedNodeID)},
				"TargetID":  &types.AttributeValueMemberS{Value: relatedNodeID},
				"Timestamp": &types.AttributeValueMemberS{Value: timestamp},
			}

			// Edge from related node to new node
			edge2 := map[string]types.AttributeValue{
				"PK":        &types.AttributeValueMemberS{Value: fmt.Sprintf("USER#%s#NODE#%s", userID, relatedNodeID)},
				"SK":        &types.AttributeValueMemberS{Value: fmt.Sprintf("EDGE#RELATES_TO#NODE#%s", nodeID)},
				"TargetID":  &types.AttributeValueMemberS{Value: nodeID},
				"Timestamp": &types.AttributeValueMemberS{Value: timestamp},
			}

			// Put both edges
			_, err := app.db.PutItem(ctx, &dynamodb.PutItemInput{
				TableName: aws.String(app.tableName),
				Item:      edge1,
			})
			if err != nil {
				errChan <- fmt.Errorf("failed to create edge from %s to %s: %w", nodeID, relatedNodeID, err)
				return
			}

			_, err = app.db.PutItem(ctx, &dynamodb.PutItemInput{
				TableName: aws.String(app.tableName),
				Item:      edge2,
			})
			if err != nil {
				errChan <- fmt.Errorf("failed to create edge from %s to %s: %w", relatedNodeID, nodeID, err)
				return
			}
			log.Printf("Successfully created bidirectional edge between %s and %s", nodeID, relatedNodeID)
		}(relatedNodeID)
	}

	wg.Wait()
	close(errChan)

	// Check if any errors occurred in the goroutines
	for err := range errChan {
		if err != nil {
			// Return the first error encountered
			log.Printf("An error occurred during concurrent edge creation: %v", err)
			return err
		}
	}

	log.Printf("Finished creating all edges for node %s", nodeID)
	return nil
}

func (app *App) findNodesByKeyword(ctx context.Context, userID, keyword string) ([]string, error) {
	log.Printf("Querying for keyword: %s", keyword)
	resp, err := app.db.Query(ctx, &dynamodb.QueryInput{
		TableName:              aws.String(app.tableName),
		IndexName:              aws.String("KeywordIndex"),
		KeyConditionExpression: aws.String("GSI1PK = :keyword"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":keyword": &types.AttributeValueMemberS{Value: fmt.Sprintf("KEYWORD#%s", keyword)},
		},
	})
	if err != nil {
		log.Printf("Query failed for keyword %s: %v", keyword, err)
		// If the GSI doesn't exist, return empty result instead of failing
		if strings.Contains(err.Error(), "GlobalSecondaryIndex") || strings.Contains(err.Error(), "KeywordIndex") {
			log.Printf("KeywordIndex GSI not found, returning empty result")
			return []string{}, nil
		}
		return nil, err
	}

	nodeIDs := make([]string, 0)
	for _, item := range resp.Items {
		// Extract node ID from GSI1SK
		if skAttr, ok := item["GSI1SK"].(*types.AttributeValueMemberS); ok {
			nodeID := strings.TrimPrefix(skAttr.Value, "NODE#")

			// Verify this node belongs to the current user
			if pkAttr, ok := item["PK"].(*types.AttributeValueMemberS); ok {
				if strings.HasPrefix(pkAttr.Value, fmt.Sprintf("USER#%s", userID)) {
					nodeIDs = append(nodeIDs, nodeID)
				}
			}
		}
	}

	return nodeIDs, nil
}

// Add this function in backend/node_operations.go

func (app *App) updateNode(ctx context.Context, userID, nodeID, body string) (Response, error) {
	var req UpdateNodeRequest
	if err := json.Unmarshal([]byte(body), &req); err != nil {
		return errorResponse(400, "Invalid request body"), nil
	}
	if req.Content == "" {
		return errorResponse(400, "Content cannot be empty"), nil
	}

	pk := fmt.Sprintf("USER#%s#NODE#%s", userID, nodeID)
	timestamp := time.Now().UTC().Format(time.RFC3339)

	// 1. Query all existing items for this node (metadata, keywords, edges)
	queryResp, err := app.db.Query(ctx, &dynamodb.QueryInput{
		TableName:              aws.String(app.tableName),
		KeyConditionExpression: aws.String("PK = :pk"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":pk": &types.AttributeValueMemberS{Value: pk},
		},
	})
	if err != nil || len(queryResp.Items) == 0 {
		return errorResponse(404, "Node not found or failed to query items"), nil
	}

	// 2. Separate old items and prepare for batch deletion
	var itemsToDelete []types.WriteRequest
	for _, item := range queryResp.Items {
		skValue := item["SK"].(*types.AttributeValueMemberS).Value
		// Delete old keywords and edges, but not the metadata item itself (we'll update it)
		if strings.HasPrefix(skValue, "KEYWORD#") || strings.HasPrefix(skValue, "EDGE#") {
			itemsToDelete = append(itemsToDelete, types.WriteRequest{
				DeleteRequest: &types.DeleteRequest{
					Key: map[string]types.AttributeValue{
						"PK": item["PK"],
						"SK": item["SK"],
					},
				},
			})
		}
	}

	// 3. Perform the batch delete if there are items to delete
	if len(itemsToDelete) > 0 {
		_, err = app.db.BatchWriteItem(ctx, &dynamodb.BatchWriteItemInput{
			RequestItems: map[string][]types.WriteRequest{
				app.tableName: itemsToDelete,
			},
		})
		if err != nil {
			log.Printf("Failed to batch delete old keywords/edges: %v", err)
			return errorResponse(500, "Failed to clear old node connections"), nil
		}
	}
	// Note: This does not delete INCOMING edges from other nodes. That remains a complex problem.

	// 4. Update the main metadata item with new content and timestamp
	updateExpression := "SET Content = :content, Timestamp = :timestamp"
	expressionAttributeValues := map[string]types.AttributeValue{
		":content":   &types.AttributeValueMemberS{Value: req.Content},
		":timestamp": &types.AttributeValueMemberS{Value: timestamp},
	}
	_, err = app.db.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: aws.String(app.tableName),
		Key: map[string]types.AttributeValue{
			"PK": &types.AttributeValueMemberS{Value: pk},
			"SK": &types.AttributeValueMemberS{Value: "METADATA#v0"},
		},
		UpdateExpression:          aws.String(updateExpression),
		ExpressionAttributeValues: expressionAttributeValues,
	})
	if err != nil {
		log.Printf("Failed to update node metadata: %v", err)
		return errorResponse(500, "Failed to update node content"), nil
	}

	// 5. Re-index and re-connect the node with the new content
	keywords := extractKeywords(req.Content)
	if err := app.indexNodeKeywords(ctx, userID, nodeID, keywords); err != nil {
		return errorResponse(500, "Failed to index updated node"), nil
	}
	if err := app.connectNode(ctx, userID, nodeID, keywords); err != nil {
		return errorResponse(500, "Failed to connect updated node"), nil
	}

	return successResponse(map[string]interface{}{
		"message": "Node updated successfully",
	}), nil
}
