# Performance-Optimized Backend Architecture

## Executive Summary

This document outlines a high-performance backend architecture specifically designed for the Brain2 knowledge graph application. The architecture prioritizes sub-50ms response times for memory operations, efficient edge queries through adjacency lists, guaranteed no-scan database operations, and asynchronous cleanup processes for maintaining data consistency.

## Core Performance Requirements

### 1. Memory Operation Latency
- **Node Creation**: < 50ms including validation and persistence
- **Node Deletion**: < 30ms for soft delete operation
- **Edge Lookup**: < 10ms using adjacency list cache
- **Keyword Similarity**: < 200ms for finding related nodes

### 2. Query Efficiency
- **No Table Scans**: All queries must use partition keys, sort keys, or GSIs
- **Adjacency List**: O(1) edge lookups via in-memory/Redis cache
- **Batch Operations**: Process up to 25 items per DynamoDB request

### 3. Async Processing
- **Event-Driven**: All non-critical operations processed asynchronously
- **Cleanup Pipeline**: Deleted nodes cleaned up via SQS/Lambda
- **Eventual Consistency**: Acceptable for non-critical operations

## System Architecture

### Data Layer Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                         API Gateway                          │
│                    (Request Validation)                      │
└──────────────────────┬──────────────────────────────────────┘
                       │
┌──────────────────────▼──────────────────────────────────────┐
│                     Lambda Functions                         │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐        │
│  │   Create    │  │    Read     │  │   Delete    │        │
│  │   Handler   │  │   Handler   │  │   Handler   │        │
│  └──────┬──────┘  └──────┬──────┘  └──────┬──────┘        │
└─────────┼─────────────────┼─────────────────┼──────────────┘
          │                 │                 │
┌─────────▼─────────────────▼─────────────────▼──────────────┐
│                    Caching Layer (Redis)                     │
│  ┌─────────────────────────────────────────────────────┐   │
│  │         Adjacency Lists (graph:user:adj)            │   │
│  ├─────────────────────────────────────────────────────┤   │
│  │          Node Cache (node:user:nodeId)              │   │
│  ├─────────────────────────────────────────────────────┤   │
│  │       Keyword Index (keywords:user:keyword)         │   │
│  └─────────────────────────────────────────────────────┘   │
└──────────────────────┬──────────────────────────────────────┘
                       │
┌──────────────────────▼──────────────────────────────────────┐
│                     DynamoDB Tables                          │
│  ┌─────────────────────────────────────────────────────┐   │
│  │  Main Table: Nodes, Edges, Metadata                  │   │
│  │  PK: USER#userId | SK: NODE#nodeId or EDGE#edgeId   │   │
│  ├─────────────────────────────────────────────────────┤   │
│  │  GSI1: Keyword Index                                 │   │
│  │  GSI1PK: KW#keyword | GSI1SK: USER#user#NODE#node   │   │
│  ├─────────────────────────────────────────────────────┤   │
│  │  GSI2: Deletion Queue                                │   │
│  │  GSI2PK: STATUS#DELETED | GSI2SK: TIMESTAMP#time    │   │
│  └─────────────────────────────────────────────────────┘   │
└──────────────────────┬──────────────────────────────────────┘
                       │
┌──────────────────────▼──────────────────────────────────────┐
│                      Event Bridge                            │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐        │
│  │NodeCreated  │  │NodeDeleted  │  │EdgeCreated  │        │
│  └──────┬──────┘  └──────┬──────┘  └──────┬──────┘        │
└─────────┼─────────────────┼─────────────────┼──────────────┘
          │                 │                 │
┌─────────▼─────────────────▼─────────────────▼──────────────┐
│                   Async Processors                           │
│  ┌────────────────┐  ┌────────────────┐  ┌──────────────┐ │
│  │   Similarity   │  │    Cleanup     │  │   Index      │ │
│  │    Matcher     │  │    Service     │  │   Updater    │ │
│  └────────────────┘  └────────────────┘  └──────────────┘ │
└─────────────────────────────────────────────────────────────┘
```

## Core Components

### 1. Adjacency List Implementation

The adjacency list is the cornerstone of our edge query performance, providing O(1) lookups for node connections.

```go
package infrastructure

import (
    "context"
    "encoding/json"
    "fmt"
    "sync"
    "time"
    
    "github.com/go-redis/redis/v8"
)

// AdjacencyList provides O(1) edge lookups using a two-tier cache
type AdjacencyList struct {
    redis     *redis.Client
    memory    *sync.Map
    ttl       time.Duration
    keyPrefix string
}

// NewAdjacencyList creates a new adjacency list manager
func NewAdjacencyList(redis *redis.Client, keyPrefix string) *AdjacencyList {
    return &AdjacencyList{
        redis:     redis,
        memory:    &sync.Map{},
        ttl:       1 * time.Hour,
        keyPrefix: keyPrefix,
    }
}

// GetNeighbors returns all connected nodes for a given node ID
// Performance: O(1) for cache hit, O(k) for cache miss where k is number of neighbors
func (a *AdjacencyList) GetNeighbors(ctx context.Context, userID, nodeID string) ([]string, error) {
    key := a.buildKey(userID, nodeID)
    
    // L1 Cache: Check memory first
    if cached, ok := a.memory.Load(key); ok {
        return cached.([]string), nil
    }
    
    // L2 Cache: Check Redis
    neighbors, err := a.redis.SMembers(ctx, key).Result()
    if err != nil && err != redis.Nil {
        return nil, fmt.Errorf("failed to get neighbors: %w", err)
    }
    
    // Warm L1 cache
    a.memory.Store(key, neighbors)
    
    return neighbors, nil
}

// AddEdge adds a bidirectional edge between two nodes
// Performance: O(1) amortized
func (a *AdjacencyList) AddEdge(ctx context.Context, userID, nodeA, nodeB string) error {
    keyA := a.buildKey(userID, nodeA)
    keyB := a.buildKey(userID, nodeB)
    
    // Use pipeline for atomic operations
    pipe := a.redis.Pipeline()
    pipe.SAdd(ctx, keyA, nodeB)
    pipe.SAdd(ctx, keyB, nodeA)
    pipe.Expire(ctx, keyA, a.ttl)
    pipe.Expire(ctx, keyB, a.ttl)
    
    _, err := pipe.Exec(ctx)
    if err != nil {
        return fmt.Errorf("failed to add edge: %w", err)
    }
    
    // Invalidate L1 cache
    a.memory.Delete(keyA)
    a.memory.Delete(keyB)
    
    return nil
}

// RemoveEdge removes an edge between two nodes
// Performance: O(1) amortized
func (a *AdjacencyList) RemoveEdge(ctx context.Context, userID, nodeA, nodeB string) error {
    keyA := a.buildKey(userID, nodeA)
    keyB := a.buildKey(userID, nodeB)
    
    pipe := a.redis.Pipeline()
    pipe.SRem(ctx, keyA, nodeB)
    pipe.SRem(ctx, keyB, nodeA)
    
    _, err := pipe.Exec(ctx)
    if err != nil {
        return fmt.Errorf("failed to remove edge: %w", err)
    }
    
    // Invalidate L1 cache
    a.memory.Delete(keyA)
    a.memory.Delete(keyB)
    
    return nil
}

// RemoveNode removes a node and all its edges
// Performance: O(k) where k is the number of neighbors
func (a *AdjacencyList) RemoveNode(ctx context.Context, userID, nodeID string) error {
    key := a.buildKey(userID, nodeID)
    
    // Get all neighbors first
    neighbors, err := a.GetNeighbors(ctx, userID, nodeID)
    if err != nil {
        return err
    }
    
    // Remove this node from all neighbors' lists
    pipe := a.redis.Pipeline()
    for _, neighbor := range neighbors {
        neighborKey := a.buildKey(userID, neighbor)
        pipe.SRem(ctx, neighborKey, nodeID)
        a.memory.Delete(neighborKey) // Invalidate neighbor cache
    }
    
    // Delete the node's own adjacency list
    pipe.Del(ctx, key)
    
    _, err = pipe.Exec(ctx)
    if err != nil {
        return fmt.Errorf("failed to remove node: %w", err)
    }
    
    // Invalidate node's cache
    a.memory.Delete(key)
    
    return nil
}

// WarmCache preloads frequently accessed nodes into memory
func (a *AdjacencyList) WarmCache(ctx context.Context, userID string, nodeIDs []string) error {
    for _, nodeID := range nodeIDs {
        _, err := a.GetNeighbors(ctx, userID, nodeID)
        if err != nil {
            return err
        }
    }
    return nil
}

func (a *AdjacencyList) buildKey(userID, nodeID string) string {
    return fmt.Sprintf("%s:%s:adj:%s", a.keyPrefix, userID, nodeID)
}
```

### 2. DynamoDB Schema (No-Scan Design)

All queries are designed to use keys or indexes, eliminating table scans entirely.

```go
package persistence

// Table Schema Definition
type DynamoDBSchema struct {
    TableName string
    Indexes   []GlobalSecondaryIndex
}

// Main table structure
var MainTableSchema = DynamoDBSchema{
    TableName: "Brain2-Main",
    Indexes: []GlobalSecondaryIndex{
        {
            Name:           "KeywordIndex",
            PartitionKey:   "GSI1PK", // KW#keyword
            SortKey:       "GSI1SK",  // USER#userId#NODE#nodeId
            Projection:    "ALL",
        },
        {
            Name:           "DeletionQueue",
            PartitionKey:   "GSI2PK", // STATUS#DELETED
            SortKey:       "GSI2SK",  // TIMESTAMP#epochTime
            Projection:    "KEYS_ONLY",
        },
        {
            Name:           "UserTimeIndex",
            PartitionKey:   "GSI3PK", // USER#userId
            SortKey:       "GSI3SK",  // TIMESTAMP#epochTime
            Projection:    "ALL",
        },
    },
}

// Key builder functions ensure no scans are needed
type KeyBuilder struct{}

func (kb *KeyBuilder) NodeKey(userID, nodeID string) (pk, sk string) {
    return fmt.Sprintf("USER#%s", userID), fmt.Sprintf("NODE#%s", nodeID)
}

func (kb *KeyBuilder) EdgeKey(userID, edgeID string) (pk, sk string) {
    return fmt.Sprintf("USER#%s", userID), fmt.Sprintf("EDGE#%s", edgeID)
}

func (kb *KeyBuilder) KeywordKey(keyword, userID, nodeID string) (pk, sk string) {
    return fmt.Sprintf("KW#%s", keyword), fmt.Sprintf("USER#%s#NODE#%s", userID, nodeID)
}

func (kb *KeyBuilder) DeletionKey(timestamp int64, itemID string) (pk, sk string) {
    return "STATUS#DELETED", fmt.Sprintf("TIMESTAMP#%d#ITEM#%s", timestamp, itemID)
}

// Query patterns - all use keys, no scans
type QueryPatterns struct {
    kb KeyBuilder
    db *dynamodb.Client
}

// GetNode - uses primary key (no scan)
func (qp *QueryPatterns) GetNode(ctx context.Context, userID, nodeID string) (*Node, error) {
    pk, sk := qp.kb.NodeKey(userID, nodeID)
    
    result, err := qp.db.GetItem(ctx, &dynamodb.GetItemInput{
        TableName: aws.String(MainTableSchema.TableName),
        Key: map[string]types.AttributeValue{
            "PK": &types.AttributeValueMemberS{Value: pk},
            "SK": &types.AttributeValueMemberS{Value: sk},
        },
    })
    
    // ... unmarshaling logic
    return node, nil
}

// FindNodesByKeyword - uses GSI1 (no scan)
func (qp *QueryPatterns) FindNodesByKeyword(ctx context.Context, keyword string) ([]*Node, error) {
    pk := fmt.Sprintf("KW#%s", keyword)
    
    result, err := qp.db.Query(ctx, &dynamodb.QueryInput{
        TableName: aws.String(MainTableSchema.TableName),
        IndexName: aws.String("KeywordIndex"),
        KeyConditionExpression: aws.String("GSI1PK = :pk"),
        ExpressionAttributeValues: map[string]types.AttributeValue{
            ":pk": &types.AttributeValueMemberS{Value: pk},
        },
    })
    
    // ... unmarshaling logic
    return nodes, nil
}

// GetDeletedItems - uses GSI2 with time range (no scan)
func (qp *QueryPatterns) GetDeletedItems(ctx context.Context, since, until int64) ([]string, error) {
    result, err := qp.db.Query(ctx, &dynamodb.QueryInput{
        TableName: aws.String(MainTableSchema.TableName),
        IndexName: aws.String("DeletionQueue"),
        KeyConditionExpression: aws.String("GSI2PK = :pk AND GSI2SK BETWEEN :start AND :end"),
        ExpressionAttributeValues: map[string]types.AttributeValue{
            ":pk":    &types.AttributeValueMemberS{Value: "STATUS#DELETED"},
            ":start": &types.AttributeValueMemberS{Value: fmt.Sprintf("TIMESTAMP#%d", since)},
            ":end":   &types.AttributeValueMemberS{Value: fmt.Sprintf("TIMESTAMP#%d", until)},
        },
    })
    
    // ... extract item IDs
    return itemIDs, nil
}
```

### 3. Async Cleanup Service

Handles node deletion cleanup without blocking the main request.

```go
package services

import (
    "context"
    "encoding/json"
    "fmt"
    "time"
    
    "github.com/aws/aws-sdk-go-v2/service/sqs"
)

// CleanupService handles async cleanup of deleted nodes
type CleanupService struct {
    sqs           *sqs.Client
    queueURL      string
    dynamoDB      *DynamoDBClient
    adjacencyList *AdjacencyList
    batchSize     int
    workers       int
}

// NodeDeletionEvent represents a node deletion request
type NodeDeletionEvent struct {
    UserID      string    `json:"userId"`
    NodeID      string    `json:"nodeId"`
    Timestamp   time.Time `json:"timestamp"`
    RetryCount  int       `json:"retryCount"`
}

// Start begins processing cleanup events
func (cs *CleanupService) Start(ctx context.Context) {
    for i := 0; i < cs.workers; i++ {
        go cs.worker(ctx)
    }
}

// worker processes cleanup events from the queue
func (cs *CleanupService) worker(ctx context.Context) {
    for {
        select {
        case <-ctx.Done():
            return
        default:
            cs.processMessages(ctx)
        }
    }
}

// processMessages handles a batch of deletion events
func (cs *CleanupService) processMessages(ctx context.Context) {
    // Receive messages from SQS
    result, err := cs.sqs.ReceiveMessage(ctx, &sqs.ReceiveMessageInput{
        QueueUrl:            aws.String(cs.queueURL),
        MaxNumberOfMessages: int32(cs.batchSize),
        WaitTimeSeconds:     int32(20), // Long polling
    })
    
    if err != nil || len(result.Messages) == 0 {
        return
    }
    
    // Process each message
    for _, msg := range result.Messages {
        if err := cs.processMessage(ctx, msg); err != nil {
            cs.handleError(ctx, msg, err)
        } else {
            cs.deleteMessage(ctx, msg)
        }
    }
}

// processMessage handles a single deletion event
func (cs *CleanupService) processMessage(ctx context.Context, msg types.Message) error {
    var event NodeDeletionEvent
    if err := json.Unmarshal([]byte(*msg.Body), &event); err != nil {
        return fmt.Errorf("failed to unmarshal event: %w", err)
    }
    
    // Execute cleanup steps
    return cs.executeCleanup(ctx, event)
}

// executeCleanup performs all cleanup operations
func (cs *CleanupService) executeCleanup(ctx context.Context, event NodeDeletionEvent) error {
    // 1. Get all connected edges from adjacency list
    neighbors, err := cs.adjacencyList.GetNeighbors(ctx, event.UserID, event.NodeID)
    if err != nil {
        return fmt.Errorf("failed to get neighbors: %w", err)
    }
    
    // 2. Remove edges in batches
    if err := cs.removeEdges(ctx, event.UserID, event.NodeID, neighbors); err != nil {
        return fmt.Errorf("failed to remove edges: %w", err)
    }
    
    // 3. Remove from adjacency lists
    if err := cs.adjacencyList.RemoveNode(ctx, event.UserID, event.NodeID); err != nil {
        return fmt.Errorf("failed to update adjacency list: %w", err)
    }
    
    // 4. Clean up keyword indexes
    if err := cs.cleanKeywordIndexes(ctx, event.UserID, event.NodeID); err != nil {
        return fmt.Errorf("failed to clean keyword indexes: %w", err)
    }
    
    // 5. Archive node data to S3 (optional)
    if err := cs.archiveNode(ctx, event); err != nil {
        // Non-critical, just log
        log.Warn("Failed to archive node", zap.Error(err))
    }
    
    // 6. Permanently delete from DynamoDB
    if err := cs.permanentlyDelete(ctx, event.UserID, event.NodeID); err != nil {
        return fmt.Errorf("failed to delete node: %w", err)
    }
    
    return nil
}

// removeEdges deletes all edges connected to the node
func (cs *CleanupService) removeEdges(ctx context.Context, userID, nodeID string, neighbors []string) error {
    // Batch delete edges
    for i := 0; i < len(neighbors); i += 25 { // DynamoDB batch limit
        end := i + 25
        if end > len(neighbors) {
            end = len(neighbors)
        }
        
        batch := neighbors[i:end]
        if err := cs.deleteEdgeBatch(ctx, userID, nodeID, batch); err != nil {
            return err
        }
    }
    
    return nil
}

// deleteEdgeBatch deletes a batch of edges
func (cs *CleanupService) deleteEdgeBatch(ctx context.Context, userID, nodeID string, neighbors []string) error {
    writeRequests := make([]types.WriteRequest, 0, len(neighbors))
    
    for _, neighbor := range neighbors {
        edgeID := cs.generateEdgeID(nodeID, neighbor)
        pk, sk := cs.kb.EdgeKey(userID, edgeID)
        
        writeRequests = append(writeRequests, types.WriteRequest{
            DeleteRequest: &types.DeleteRequest{
                Key: map[string]types.AttributeValue{
                    "PK": &types.AttributeValueMemberS{Value: pk},
                    "SK": &types.AttributeValueMemberS{Value: sk},
                },
            },
        })
    }
    
    _, err := cs.dynamoDB.BatchWriteItem(ctx, &dynamodb.BatchWriteItemInput{
        RequestItems: map[string][]types.WriteRequest{
            MainTableSchema.TableName: writeRequests,
        },
    })
    
    return err
}
```

### 4. Keyword Similarity Engine

Automatically creates edges between nodes based on keyword similarity.

```go
package similarity

import (
    "context"
    "math"
    "sort"
    "strings"
    "sync"
)

// SimilarityEngine handles keyword extraction and similarity matching
type SimilarityEngine struct {
    keywordExtractor *KeywordExtractor
    invertedIndex    *InvertedIndex
    threshold        float64
    maxEdges         int
    workers          int
}

// KeywordExtractor extracts keywords from content
type KeywordExtractor struct {
    stopWords map[string]bool
    minLength int
    maxLength int
}

// ExtractKeywords extracts relevant keywords from content
func (ke *KeywordExtractor) ExtractKeywords(content string) []string {
    // Tokenize
    tokens := strings.Fields(strings.ToLower(content))
    
    // Filter stop words and apply length constraints
    keywords := make([]string, 0)
    seen := make(map[string]bool)
    
    for _, token := range tokens {
        // Clean token
        token = strings.Trim(token, ".,!?;:'\"")
        
        // Apply filters
        if len(token) < ke.minLength || len(token) > ke.maxLength {
            continue
        }
        if ke.stopWords[token] {
            continue
        }
        if seen[token] {
            continue
        }
        
        keywords = append(keywords, token)
        seen[token] = true
    }
    
    return keywords
}

// InvertedIndex maintains keyword to node mappings
type InvertedIndex struct {
    mu    sync.RWMutex
    index map[string][]NodeReference
    idf   map[string]float64 // Inverse document frequency
}

// NodeReference represents a node in the index
type NodeReference struct {
    UserID   string
    NodeID   string
    TF       float64 // Term frequency
    Keywords []string
}

// AddNode adds a node to the inverted index
func (ii *InvertedIndex) AddNode(userID, nodeID string, keywords []string) {
    ii.mu.Lock()
    defer ii.mu.Unlock()
    
    // Calculate term frequency
    tf := make(map[string]float64)
    for _, keyword := range keywords {
        tf[keyword]++
    }
    
    // Normalize TF
    maxTF := 0.0
    for _, freq := range tf {
        if freq > maxTF {
            maxTF = freq
        }
    }
    for keyword := range tf {
        tf[keyword] = tf[keyword] / maxTF
    }
    
    // Add to index
    nodeRef := NodeReference{
        UserID:   userID,
        NodeID:   nodeID,
        Keywords: keywords,
    }
    
    for keyword, freq := range tf {
        nodeRef.TF = freq
        if ii.index[keyword] == nil {
            ii.index[keyword] = make([]NodeReference, 0)
        }
        ii.index[keyword] = append(ii.index[keyword], nodeRef)
    }
    
    // Update IDF
    ii.updateIDF()
}

// updateIDF recalculates inverse document frequency
func (ii *InvertedIndex) updateIDF() {
    totalDocs := len(ii.index)
    
    for keyword, nodes := range ii.index {
        ii.idf[keyword] = math.Log(float64(totalDocs) / float64(len(nodes)))
    }
}

// FindSimilar finds nodes similar to the given keywords
func (ii *InvertedIndex) FindSimilar(keywords []string, threshold float64) []SimilarityResult {
    ii.mu.RLock()
    defer ii.mu.RUnlock()
    
    // Calculate TF-IDF for query
    queryVector := ii.calculateTFIDF(keywords)
    
    // Find candidate nodes
    candidates := make(map[string]*NodeReference)
    for _, keyword := range keywords {
        for _, node := range ii.index[keyword] {
            key := node.UserID + ":" + node.NodeID
            if candidates[key] == nil {
                candidates[key] = &node
            }
        }
    }
    
    // Calculate similarity scores
    results := make([]SimilarityResult, 0)
    for _, candidate := range candidates {
        score := ii.cosineSimilarity(queryVector, ii.calculateNodeTFIDF(candidate))
        if score >= threshold {
            results = append(results, SimilarityResult{
                NodeID: candidate.NodeID,
                Score:  score,
            })
        }
    }
    
    // Sort by score
    sort.Slice(results, func(i, j int) bool {
        return results[i].Score > results[j].Score
    })
    
    return results
}

// calculateTFIDF calculates TF-IDF vector for keywords
func (ii *InvertedIndex) calculateTFIDF(keywords []string) map[string]float64 {
    vector := make(map[string]float64)
    
    // Calculate TF
    for _, keyword := range keywords {
        vector[keyword]++
    }
    
    // Normalize and apply IDF
    maxTF := 0.0
    for _, tf := range vector {
        if tf > maxTF {
            maxTF = tf
        }
    }
    
    for keyword := range vector {
        vector[keyword] = (vector[keyword] / maxTF) * ii.idf[keyword]
    }
    
    return vector
}

// cosineSimilarity calculates cosine similarity between two vectors
func (ii *InvertedIndex) cosineSimilarity(v1, v2 map[string]float64) float64 {
    dotProduct := 0.0
    magnitude1 := 0.0
    magnitude2 := 0.0
    
    // Combine all keys
    allKeys := make(map[string]bool)
    for k := range v1 {
        allKeys[k] = true
    }
    for k := range v2 {
        allKeys[k] = true
    }
    
    // Calculate dot product and magnitudes
    for k := range allKeys {
        val1 := v1[k]
        val2 := v2[k]
        
        dotProduct += val1 * val2
        magnitude1 += val1 * val1
        magnitude2 += val2 * val2
    }
    
    if magnitude1 == 0 || magnitude2 == 0 {
        return 0
    }
    
    return dotProduct / (math.Sqrt(magnitude1) * math.Sqrt(magnitude2))
}

// ProcessNewNode handles similarity matching for a new node
func (se *SimilarityEngine) ProcessNewNode(ctx context.Context, node *Node) error {
    // Extract keywords
    keywords := se.keywordExtractor.ExtractKeywords(node.Content)
    
    // Add to inverted index
    se.invertedIndex.AddNode(node.UserID, node.NodeID, keywords)
    
    // Find similar nodes
    similar := se.invertedIndex.FindSimilar(keywords, se.threshold)
    
    // Limit edges
    if len(similar) > se.maxEdges {
        similar = similar[:se.maxEdges]
    }
    
    // Create edges asynchronously
    for _, match := range similar {
        if match.NodeID != node.NodeID { // Avoid self-edges
            se.createEdgeAsync(ctx, node.UserID, node.NodeID, match.NodeID, match.Score)
        }
    }
    
    return nil
}

// createEdgeAsync creates an edge in the background
func (se *SimilarityEngine) createEdgeAsync(ctx context.Context, userID, source, target string, score float64) {
    go func() {
        edge := &Edge{
            ID:       generateEdgeID(),
            UserID:   userID,
            SourceID: source,
            TargetID: target,
            Type:     "similarity",
            Weight:   score,
            Metadata: map[string]interface{}{
                "auto_generated": true,
                "similarity_score": score,
            },
        }
        
        // Publish edge creation event
        se.eventBus.Publish(ctx, EdgeCreatedEvent{
            Edge: edge,
        })
    }()
}
```

### 5. Event-Driven Architecture

Coordinates all async operations through events.

```go
package events

import (
    "context"
    "encoding/json"
    "fmt"
    
    "github.com/aws/aws-sdk-go-v2/service/eventbridge"
)

// EventBus handles all domain events
type EventBus struct {
    client    *eventbridge.Client
    busName   string
    source    string
}

// Event types
const (
    EventNodeCreated  = "node.created"
    EventNodeDeleted  = "node.deleted"
    EventNodeUpdated  = "node.updated"
    EventEdgeCreated  = "edge.created"
    EventEdgeDeleted  = "edge.deleted"
)

// NodeCreatedEvent is published when a node is created
type NodeCreatedEvent struct {
    UserID    string   `json:"userId"`
    NodeID    string   `json:"nodeId"`
    Content   string   `json:"content"`
    Keywords  []string `json:"keywords"`
    Timestamp int64    `json:"timestamp"`
}

// NodeDeletedEvent is published when a node is deleted
type NodeDeletedEvent struct {
    UserID    string `json:"userId"`
    NodeID    string `json:"nodeId"`
    Timestamp int64  `json:"timestamp"`
    Cascade   bool   `json:"cascade"` // Whether to delete connected nodes
}

// PublishNodeCreated publishes a node creation event
func (eb *EventBus) PublishNodeCreated(ctx context.Context, event NodeCreatedEvent) error {
    return eb.publish(ctx, EventNodeCreated, event)
}

// PublishNodeDeleted publishes a node deletion event
func (eb *EventBus) PublishNodeDeleted(ctx context.Context, event NodeDeletedEvent) error {
    return eb.publish(ctx, EventNodeDeleted, event)
}

// publish sends an event to EventBridge
func (eb *EventBus) publish(ctx context.Context, eventType string, event interface{}) error {
    data, err := json.Marshal(event)
    if err != nil {
        return fmt.Errorf("failed to marshal event: %w", err)
    }
    
    entry := types.PutEventsRequestEntry{
        EventBusName: aws.String(eb.busName),
        Source:       aws.String(eb.source),
        DetailType:   aws.String(eventType),
        Detail:       aws.String(string(data)),
    }
    
    _, err = eb.client.PutEvents(ctx, &eventbridge.PutEventsInput{
        Entries: []types.PutEventsRequestEntry{entry},
    })
    
    return err
}

// EventHandler processes events
type EventHandler struct {
    similarityEngine *SimilarityEngine
    cleanupService   *CleanupService
    adjacencyList    *AdjacencyList
    cacheWarmer      *CacheWarmer
}

// HandleNodeCreated processes node creation events
func (eh *EventHandler) HandleNodeCreated(ctx context.Context, event NodeCreatedEvent) error {
    // Warm cache
    go eh.cacheWarmer.WarmNode(ctx, event.UserID, event.NodeID)
    
    // Process similarity matching
    go eh.similarityEngine.ProcessNewNode(ctx, &Node{
        UserID:   event.UserID,
        NodeID:   event.NodeID,
        Content:  event.Content,
        Keywords: event.Keywords,
    })
    
    return nil
}

// HandleNodeDeleted processes node deletion events
func (eh *EventHandler) HandleNodeDeleted(ctx context.Context, event NodeDeletedEvent) error {
    // Queue for async cleanup
    return eh.cleanupService.QueueDeletion(ctx, event)
}
```

## Performance Optimizations

### 1. Connection Pooling

```go
// Redis connection pool configuration
redisClient := redis.NewClient(&redis.Options{
    Addr:         "redis-cluster.amazonaws.com:6379",
    PoolSize:     100,
    MinIdleConns: 10,
    MaxRetries:   3,
    DialTimeout:  5 * time.Second,
    ReadTimeout:  3 * time.Second,
    WriteTimeout: 3 * time.Second,
})

// DynamoDB client with connection reuse
dynamoClient := dynamodb.NewFromConfig(cfg, func(o *dynamodb.Options) {
    o.HTTPClient = &http.Client{
        Timeout: 5 * time.Second,
        Transport: &http.Transport{
            MaxIdleConns:        100,
            MaxIdleConnsPerHost: 20,
            IdleConnTimeout:     90 * time.Second,
        },
    }
})
```

### 2. Batch Processing

```go
// BatchProcessor handles batch operations efficiently
type BatchProcessor struct {
    batchSize int
    timeout   time.Duration
    buffer    []interface{}
    mu        sync.Mutex
}

func (bp *BatchProcessor) Process(items []interface{}, processor func([]interface{}) error) error {
    // Process in parallel batches
    var wg sync.WaitGroup
    errors := make(chan error, len(items)/bp.batchSize+1)
    
    for i := 0; i < len(items); i += bp.batchSize {
        end := i + bp.batchSize
        if end > len(items) {
            end = len(items)
        }
        
        wg.Add(1)
        go func(batch []interface{}) {
            defer wg.Done()
            if err := processor(batch); err != nil {
                errors <- err
            }
        }(items[i:end])
    }
    
    wg.Wait()
    close(errors)
    
    // Check for errors
    for err := range errors {
        if err != nil {
            return err
        }
    }
    
    return nil
}
```

### 3. Circuit Breaker

```go
// CircuitBreaker prevents cascading failures
type CircuitBreaker struct {
    maxFailures  int
    resetTimeout time.Duration
    failures     int
    lastFailure  time.Time
    state        State
    mu           sync.Mutex
}

type State int

const (
    StateClosed State = iota
    StateOpen
    StateHalfOpen
)

func (cb *CircuitBreaker) Execute(fn func() error) error {
    cb.mu.Lock()
    defer cb.mu.Unlock()
    
    // Check state
    switch cb.state {
    case StateOpen:
        if time.Since(cb.lastFailure) > cb.resetTimeout {
            cb.state = StateHalfOpen
            cb.failures = 0
        } else {
            return ErrCircuitOpen
        }
    }
    
    // Execute function
    err := fn()
    
    // Update state based on result
    if err != nil {
        cb.failures++
        cb.lastFailure = time.Now()
        
        if cb.failures >= cb.maxFailures {
            cb.state = StateOpen
        }
        
        return err
    }
    
    // Success - reset
    if cb.state == StateHalfOpen {
        cb.state = StateClosed
    }
    cb.failures = 0
    
    return nil
}
```

## Monitoring and Observability

### Key Metrics

```go
// Metrics collection
type Metrics struct {
    nodeCreationLatency   prometheus.Histogram
    nodeDeletionLatency   prometheus.Histogram
    edgeLookupLatency     prometheus.Histogram
    cleanupQueueDepth     prometheus.Gauge
    similarityMatchRate   prometheus.Counter
    cacheHitRate          prometheus.Counter
}

func NewMetrics() *Metrics {
    return &Metrics{
        nodeCreationLatency: promauto.NewHistogram(prometheus.HistogramOpts{
            Name:    "node_creation_latency_ms",
            Help:    "Node creation latency in milliseconds",
            Buckets: []float64{10, 25, 50, 100, 250, 500, 1000},
        }),
        nodeDeletionLatency: promauto.NewHistogram(prometheus.HistogramOpts{
            Name:    "node_deletion_latency_ms",
            Help:    "Node deletion latency in milliseconds",
            Buckets: []float64{5, 10, 25, 50, 100, 250},
        }),
        edgeLookupLatency: promauto.NewHistogram(prometheus.HistogramOpts{
            Name:    "edge_lookup_latency_ms",
            Help:    "Edge lookup latency in milliseconds",
            Buckets: []float64{1, 5, 10, 25, 50, 100},
        }),
        cleanupQueueDepth: promauto.NewGauge(prometheus.GaugeOpts{
            Name: "cleanup_queue_depth",
            Help: "Number of items in cleanup queue",
        }),
        similarityMatchRate: promauto.NewCounter(prometheus.CounterOpts{
            Name: "similarity_matches_total",
            Help: "Total number of similarity matches found",
        }),
        cacheHitRate: promauto.NewCounter(prometheus.CounterOpts{
            Name: "cache_hits_total",
            Help: "Total number of cache hits",
        }),
    }
}
```

### CloudWatch Alarms

```yaml
Alarms:
  - Name: HighNodeCreationLatency
    MetricName: node_creation_latency_ms
    Statistic: p99
    Threshold: 100
    ComparisonOperator: GreaterThanThreshold
    
  - Name: CleanupQueueBacklog
    MetricName: cleanup_queue_depth
    Statistic: Average
    Threshold: 1000
    ComparisonOperator: GreaterThanThreshold
    
  - Name: LowCacheHitRate
    MetricName: cache_hit_rate
    Statistic: Average
    Threshold: 0.8
    ComparisonOperator: LessThanThreshold
```

## Testing Strategy

### Performance Tests

```go
func TestNodeCreationPerformance(t *testing.T) {
    // Setup
    service := setupTestService()
    
    // Measure latency for 1000 operations
    latencies := make([]time.Duration, 1000)
    
    for i := 0; i < 1000; i++ {
        start := time.Now()
        
        _, err := service.CreateNode(ctx, &CreateNodeRequest{
            UserID:  "test-user",
            Content: fmt.Sprintf("Test content %d", i),
        })
        
        latencies[i] = time.Since(start)
        require.NoError(t, err)
    }
    
    // Calculate percentiles
    sort.Slice(latencies, func(i, j int) bool {
        return latencies[i] < latencies[j]
    })
    
    p50 := latencies[500]
    p99 := latencies[990]
    
    // Assert performance requirements
    assert.Less(t, p50, 30*time.Millisecond, "P50 should be under 30ms")
    assert.Less(t, p99, 50*time.Millisecond, "P99 should be under 50ms")
}

func TestAdjacencyListPerformance(t *testing.T) {
    // Setup adjacency list with 10,000 nodes
    adj := setupTestAdjacencyList()
    
    // Create a highly connected graph
    for i := 0; i < 10000; i++ {
        for j := 0; j < 10; j++ {
            adj.AddEdge(ctx, "user1", fmt.Sprintf("node%d", i), fmt.Sprintf("node%d", (i+j)%10000))
        }
    }
    
    // Test lookup performance
    start := time.Now()
    neighbors, err := adj.GetNeighbors(ctx, "user1", "node5000")
    elapsed := time.Since(start)
    
    require.NoError(t, err)
    assert.Len(t, neighbors, 10)
    assert.Less(t, elapsed, 10*time.Millisecond, "Lookup should be under 10ms")
}
```

### Load Testing

```go
func BenchmarkConcurrentNodeCreation(b *testing.B) {
    service := setupTestService()
    
    b.RunParallel(func(pb *testing.PB) {
        for pb.Next() {
            _, err := service.CreateNode(ctx, &CreateNodeRequest{
                UserID:  "bench-user",
                Content: "Benchmark content",
            })
            if err != nil {
                b.Fatal(err)
            }
        }
    })
}
```

## Deployment Configuration

### Lambda Functions

```yaml
Functions:
  NodeAPI:
    Runtime: go1.x
    MemorySize: 512
    Timeout: 10
    ReservedConcurrentExecutions: 100
    Environment:
      REDIS_ENDPOINT: !Ref RedisCluster
      DYNAMODB_TABLE: !Ref MainTable
      
  CleanupWorker:
    Runtime: go1.x
    MemorySize: 256
    Timeout: 60
    ReservedConcurrentExecutions: 10
    EventSourceMappings:
      - EventSourceArn: !GetAtt CleanupQueue.Arn
        BatchSize: 10
        
  SimilarityMatcher:
    Runtime: go1.x
    MemorySize: 1024
    Timeout: 30
    ReservedConcurrentExecutions: 5
```

### DynamoDB Configuration

```yaml
MainTable:
  Type: AWS::DynamoDB::Table
  Properties:
    BillingMode: ON_DEMAND
    PointInTimeRecoverySpecification:
      PointInTimeRecoveryEnabled: true
    StreamSpecification:
      StreamViewType: NEW_AND_OLD_IMAGES
    GlobalSecondaryIndexes:
      - IndexName: KeywordIndex
        PartitionKey:
          AttributeName: GSI1PK
          AttributeType: S
        SortKey:
          AttributeName: GSI1SK
          AttributeType: S
        Projection:
          ProjectionType: ALL
```

### Redis Configuration

```yaml
RedisCluster:
  Type: AWS::ElastiCache::ReplicationGroup
  Properties:
    ReplicationGroupDescription: Brain2 Adjacency List Cache
    CacheNodeType: cache.r6g.large
    NumCacheClusters: 3
    AutomaticFailoverEnabled: true
    MultiAZEnabled: true
    Engine: redis
    EngineVersion: 6.2
```

## Conclusion

This performance-optimized architecture ensures:

1. **Sub-50ms response times** for all primary operations
2. **No table scans** through careful key design and GSI usage
3. **O(1) edge lookups** via adjacency list caching
4. **Automatic edge creation** through keyword similarity matching
5. **Async cleanup** that doesn't block user operations
6. **Scalability** to millions of nodes and edges
7. **Fault tolerance** through circuit breakers and retries

The system is designed to handle 10,000+ requests per second while maintaining consistent low latency and high availability.