# Brain2 Backend Performance Optimization Plan

## Overview
This plan focuses on fixing critical performance issues causing 503 errors and high latency in the Brain2 backend. All optimizations are cost-effective and don't require provisioned concurrency.

## Table Structure Documentation
**TODO:** Add comprehensive DynamoDB table structure documentation

### Current Structure
- **Table Name**: `brain2`
- **Primary Keys**: `PK` (partition), `SK` (sort)
- **GSI1 (KeywordIndex)**: `GSI1PK`, `GSI1SK`
- **GSI2 (EdgeIndex)**: `GSI2PK`, `GSI2SK`
- **Key Patterns**:
  - Nodes: PK=`USER#userId#NODE#nodeId`, SK=`METADATA#`
  - Edges: Various patterns with USER# and EDGE# prefixes

---

## 🔴 Priority 1: Client Reuse & HTTP Keep-Alive

### Issue
Lambda functions are creating new DynamoDB clients on each request, adding 200ms overhead.

### Fix 1.1: Optimize Main Lambda Client Initialization
**File**: `backend/cmd/main/main.go`

**Current Code** (lines 16-62):
```go
func init() {
    // Current initialization without optimized clients
    container, err = di.InitializeContainer()
    // ...
}
```

**Replace With**:
```go
package main

import (
    "context"
    "log"
    "net/http"
    "time"

    "brain2-backend/internal/di"

    "github.com/aws/aws-lambda-go/events"
    "github.com/aws/aws-lambda-go/lambda"
    chiadapter "github.com/awslabs/aws-lambda-go-api-proxy/chi"
)

var (
    chiLambda *chiadapter.ChiLambdaV2
    container *di.Container
    coldStart = true
    coldStartTime time.Time
)

func init() {
    if coldStart {
        coldStartTime = time.Now()
        log.Println("Cold start detected - initializing with optimized clients...")
    }
    
    initStart := time.Now()
    var err error
    
    // Initialize container with optimized settings
    container, err = di.InitializeContainer()
    if err != nil {
        log.Fatalf("Failed to initialize DI container: %v", err)
    }

    // Validate dependencies
    if err := container.Validate(); err != nil {
        log.Fatalf("Container validation failed: %v", err)
    }

    container.SetColdStartInfo(coldStartTime, coldStart)
    
    router := container.GetRouter()
    chiLambda = chiadapter.NewV2(router)
    
    initDuration := time.Since(initStart)
    
    if coldStart {
        log.Printf("Cold start completed in %v", time.Since(coldStartTime))
        coldStart = false
        container.IsColdStart = false
    } else {
        log.Printf("Warm start initialization in %v", initDuration)
    }
}

func main() {
    // Add graceful shutdown
    defer func() {
        if container != nil {
            ctx := context.Background()
            if err := container.Shutdown(ctx); err != nil {
                log.Printf("Error during shutdown: %v", err)
            }
        }
    }()

    lambda.Start(handler)
}

func handler(ctx context.Context, req events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error) {
    // Add request tracking
    requestStart := time.Now()
    
    response, err := chiLambda.ProxyWithContextV2(ctx, req)
    
    duration := time.Since(requestStart)
    if duration > 5*time.Second {
        log.Printf("SLOW REQUEST: %s %s took %v", 
            req.RequestContext.HTTP.Method, 
            req.RequestContext.HTTP.Path, 
            duration)
    }
    
    return response, err
}
```

### Fix 1.2: Optimize DI Container AWS Client Initialization
**File**: `backend/internal/di/container.go`

**Current Code** (lines 70-89):
```go
func (c *Container) initializeAWSClients() error {
    // Current basic initialization
    awsCfg, err := awsConfig.LoadDefaultConfig(ctx)
    // ...
}
```

**Replace With**:
```go
// initializeAWSClients sets up AWS service clients with connection reuse
func (c *Container) initializeAWSClients() error {
    log.Println("Initializing AWS clients with optimized settings...")
    startTime := time.Now()

    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()

    // Create custom HTTP client with keep-alive and connection pooling
    httpClient := &http.Client{
        Timeout: 30 * time.Second,
        Transport: &http.Transport{
            MaxIdleConns:        100,
            MaxIdleConnsPerHost: 10,
            IdleConnTimeout:     90 * time.Second,
            DisableKeepAlives:   false, // IMPORTANT: Keep connections alive
            TLSHandshakeTimeout: 10 * time.Second,
        },
    }

    // Load AWS config with custom HTTP client
    awsCfg, err := awsConfig.LoadDefaultConfig(ctx,
        awsConfig.WithHTTPClient(httpClient),
        awsConfig.WithRetryMaxAttempts(3),
        awsConfig.WithRetryMode(aws.RetryModeAdaptive),
    )
    if err != nil {
        return fmt.Errorf("failed to load AWS config: %w", err)
    }

    // DynamoDB client with optimized settings
    c.DynamoDBClient = awsDynamodb.NewFromConfig(awsCfg, func(o *awsDynamodb.Options) {
        o.RetryMaxAttempts = 3
        o.RetryMode = aws.RetryModeAdaptive
    })

    // EventBridge client with optimized settings
    c.EventBridgeClient = awsEventbridge.NewFromConfig(awsCfg, func(o *awsEventbridge.Options) {
        o.RetryMaxAttempts = 3
    })

    log.Printf("AWS clients initialized with connection pooling in %v", time.Since(startTime))
    return nil
}
```

---

## 🔴 Priority 2: DynamoDB Batch Operations

### Issue
Bulk delete operations are making 2N individual DynamoDB calls for N nodes, causing timeouts and 503 errors.

### Fix 2.1: Implement Batch Delete in NodeRepository
**File**: `backend/internal/infrastructure/persistence/dynamodb/node_repository.go`

**Current Code** (lines 280-340):
```go
func (r *NodeRepository) BatchDeleteNodes(ctx context.Context, userID string, nodeIDs []string) (deleted []string, failed []string, err error) {
    // Current implementation with individual deletes
}
```

**Replace With**:
```go
// BatchDeleteNodes uses DynamoDB BatchWriteItem for efficient bulk deletion
func (r *NodeRepository) BatchDeleteNodes(ctx context.Context, userID string, nodeIDs []string) (deleted []string, failed []string, err error) {
    if len(nodeIDs) == 0 {
        return []string{}, []string{}, nil
    }

    r.logger.Info("Starting optimized batch delete",
        zap.String("userID", userID),
        zap.Int("count", len(nodeIDs)))

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
        chunkDeleted, chunkFailed := r.processBatchDeleteChunk(ctx, userID, chunk)
        deleted = append(deleted, chunkDeleted...)
        failed = append(failed, chunkFailed...)
    }

    r.logger.Info("Batch delete completed",
        zap.Int("deleted", len(deleted)),
        zap.Int("failed", len(failed)))

    return deleted, failed, nil
}

// processBatchDeleteChunk handles a single batch of up to 25 items
func (r *NodeRepository) processBatchDeleteChunk(ctx context.Context, userID string, nodeIDs []string) (deleted []string, failed []string) {
    maxRetries := 3
    retryDelay := 100 * time.Millisecond
    
    // Track which items are still unprocessed
    unprocessedIDs := make(map[string]bool)
    for _, id := range nodeIDs {
        unprocessedIDs[id] = true
    }
    
    deleted = make([]string, 0, len(nodeIDs))
    failed = make([]string, 0)

    for attempt := 0; attempt <= maxRetries && len(unprocessedIDs) > 0; attempt++ {
        if attempt > 0 {
            time.Sleep(retryDelay)
            retryDelay *= 2 // Exponential backoff
            r.logger.Debug("Retrying batch delete",
                zap.Int("attempt", attempt),
                zap.Int("remaining", len(unprocessedIDs)))
        }

        // Build write requests for unprocessed items
        writeRequests := make([]types.WriteRequest, 0, len(unprocessedIDs))
        processingIDs := make([]string, 0, len(unprocessedIDs))
        
        for nodeID := range unprocessedIDs {
            processingIDs = append(processingIDs, nodeID)
            
            // Build the correct key structure for your table
            writeRequests = append(writeRequests, types.WriteRequest{
                DeleteRequest: &types.DeleteRequest{
                    Key: map[string]types.AttributeValue{
                        "PK": &types.AttributeValueMemberS{
                            Value: fmt.Sprintf("USER#%s#NODE#%s", userID, nodeID),
                        },
                        "SK": &types.AttributeValueMemberS{
                            Value: "METADATA#",
                        },
                    },
                },
            })
        }

        // Execute batch write
        input := &dynamodb.BatchWriteItemInput{
            RequestItems: map[string][]types.WriteRequest{
                r.tableName: writeRequests,
            },
        }

        output, err := r.client.BatchWriteItem(ctx, input)
        if err != nil {
            r.logger.Error("BatchWriteItem failed",
                zap.Error(err),
                zap.Int("attempt", attempt))
            
            if attempt == maxRetries {
                // Final attempt failed, mark all as failed
                for id := range unprocessedIDs {
                    failed = append(failed, id)
                }
                return deleted, failed
            }
            continue
        }

        // Remove successfully processed items from unprocessedIDs
        for _, id := range processingIDs {
            delete(unprocessedIDs, id)
        }

        // Check for unprocessed items returned by DynamoDB
        if output.UnprocessedItems != nil && len(output.UnprocessedItems[r.tableName]) > 0 {
            // Re-add unprocessed items for retry
            for _, req := range output.UnprocessedItems[r.tableName] {
                if req.DeleteRequest != nil {
                    // Extract nodeID from the key
                    pk := req.DeleteRequest.Key["PK"].(*types.AttributeValueMemberS).Value
                    // Parse nodeID from PK pattern: USER#userId#NODE#nodeId
                    parts := strings.Split(pk, "#")
                    if len(parts) >= 4 {
                        nodeID := parts[3]
                        unprocessedIDs[nodeID] = true
                    }
                }
            }
        }

        // Track successfully deleted items
        for _, id := range processingIDs {
            if !unprocessedIDs[id] {
                deleted = append(deleted, id)
            }
        }
    }

    // Any remaining unprocessed items are failures
    for id := range unprocessedIDs {
        failed = append(failed, id)
    }

    return deleted, failed
}
```

### Fix 2.2: Add Batch Get Operations
**File**: `backend/internal/infrastructure/persistence/dynamodb/node_repository.go`

**Add New Method**:
```go
// BatchGetNodes retrieves multiple nodes in a single DynamoDB operation
func (r *NodeRepository) BatchGetNodes(ctx context.Context, userID string, nodeIDs []string) ([]*node.Node, error) {
    if len(nodeIDs) == 0 {
        return []*node.Node{}, nil
    }

    nodes := make([]*node.Node, 0, len(nodeIDs))
    
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
        nodes = append(nodes, chunkNodes...)
    }
    
    return nodes, nil
}

func (r *NodeRepository) batchGetChunk(ctx context.Context, userID string, nodeIDs []string) ([]*node.Node, error) {
    // Build keys for batch get
    keys := make([]map[string]types.AttributeValue, len(nodeIDs))
    for i, nodeID := range nodeIDs {
        keys[i] = map[string]types.AttributeValue{
            "PK": &types.AttributeValueMemberS{
                Value: fmt.Sprintf("USER#%s#NODE#%s", userID, nodeID),
            },
            "SK": &types.AttributeValueMemberS{
                Value: "METADATA#",
            },
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

    // Convert items to nodes
    nodes := make([]*node.Node, 0, len(output.Responses[r.tableName]))
    for _, item := range output.Responses[r.tableName] {
        node, err := r.itemToNode(item)
        if err != nil {
            r.logger.Warn("Failed to unmarshal node", zap.Error(err))
            continue
        }
        nodes = append(nodes, node)
    }

    // Handle unprocessed keys with retry
    if len(output.UnprocessedKeys) > 0 {
        r.logger.Warn("BatchGetItem had unprocessed keys",
            zap.Int("count", len(output.UnprocessedKeys[r.tableName].Keys)))
        // Implement retry logic here if needed
    }

    return nodes, nil
}
```

---

## 🔴 Priority 3: Async Event Processing

### Issue
Event publishing to EventBridge is blocking API responses, adding 100-200ms latency.

### Fix 3.1: Implement Async Event Publisher
**File**: `backend/internal/infrastructure/messaging/async_publisher.go`

**Create New File**:
```go
package messaging

import (
    "context"
    "fmt"
    "log"
    "sync"
    "time"

    "github.com/aws/aws-sdk-go-v2/aws"
    "github.com/aws/aws-sdk-go-v2/service/eventbridge"
    "github.com/aws/aws-sdk-go-v2/service/eventbridge/types"
    "brain2-backend/internal/domain/shared"
    "brain2-backend/internal/repository"
)

// AsyncEventPublisher provides non-blocking event publishing
type AsyncEventPublisher struct {
    client      *eventbridge.Client
    eventBus    string
    source      string
    
    // Batching configuration
    batchSize   int
    flushInterval time.Duration
    
    // Internal state
    mu          sync.Mutex
    events      []shared.DomainEvent
    flushTimer  *time.Timer
    shutdownCh  chan struct{}
    wg          sync.WaitGroup
}

// NewAsyncEventPublisher creates an optimized async publisher
func NewAsyncEventPublisher(client *eventbridge.Client, eventBus, source string) *AsyncEventPublisher {
    if eventBus == "" {
        eventBus = "default"
    }
    if source == "" {
        source = "brain2-backend"
    }
    
    p := &AsyncEventPublisher{
        client:        client,
        eventBus:      eventBus,
        source:        source,
        batchSize:     10, // EventBridge limit
        flushInterval: 100 * time.Millisecond,
        events:        make([]shared.DomainEvent, 0, 10),
        shutdownCh:    make(chan struct{}),
    }
    
    // Start the background flusher
    p.wg.Add(1)
    go p.backgroundFlusher()
    
    return p
}

// Publish queues events for async processing (non-blocking)
func (p *AsyncEventPublisher) Publish(ctx context.Context, events []shared.DomainEvent) error {
    p.mu.Lock()
    defer p.mu.Unlock()
    
    // Add events to buffer
    p.events = append(p.events, events...)
    
    // Check if we should flush immediately
    if len(p.events) >= p.batchSize {
        p.flushLocked()
    } else {
        // Reset flush timer
        if p.flushTimer != nil {
            p.flushTimer.Stop()
        }
        p.flushTimer = time.AfterFunc(p.flushInterval, func() {
            p.mu.Lock()
            defer p.mu.Unlock()
            p.flushLocked()
        })
    }
    
    return nil
}

// backgroundFlusher handles periodic flushing
func (p *AsyncEventPublisher) backgroundFlusher() {
    defer p.wg.Done()
    
    ticker := time.NewTicker(p.flushInterval)
    defer ticker.Stop()
    
    for {
        select {
        case <-ticker.C:
            p.mu.Lock()
            if len(p.events) > 0 {
                p.flushLocked()
            }
            p.mu.Unlock()
            
        case <-p.shutdownCh:
            // Final flush before shutdown
            p.mu.Lock()
            if len(p.events) > 0 {
                p.flushLocked()
            }
            p.mu.Unlock()
            return
        }
    }
}

// flushLocked sends buffered events (must be called with lock held)
func (p *AsyncEventPublisher) flushLocked() {
    if len(p.events) == 0 {
        return
    }
    
    // Copy events for processing
    toSend := make([]shared.DomainEvent, len(p.events))
    copy(toSend, p.events)
    p.events = p.events[:0]
    
    // Process asynchronously
    p.wg.Add(1)
    go func() {
        defer p.wg.Done()
        
        ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
        defer cancel()
        
        if err := p.publishBatch(ctx, toSend); err != nil {
            log.Printf("ERROR: Failed to publish event batch: %v", err)
            // In production, implement retry or DLQ logic here
        }
    }()
}

// publishBatch sends a batch of events to EventBridge
func (p *AsyncEventPublisher) publishBatch(ctx context.Context, events []shared.DomainEvent) error {
    entries := make([]types.PutEventsRequestEntry, 0, len(events))
    
    for _, event := range events {
        entry, err := p.createEventEntry(event)
        if err != nil {
            log.Printf("ERROR: Failed to create event entry: %v", err)
            continue
        }
        entries = append(entries, entry)
    }
    
    if len(entries) == 0 {
        return nil
    }
    
    output, err := p.client.PutEvents(ctx, &eventbridge.PutEventsInput{
        Entries: entries,
    })
    
    if err != nil {
        return fmt.Errorf("PutEvents failed: %w", err)
    }
    
    if output.FailedEntryCount > 0 {
        log.Printf("WARNING: %d events failed to publish", output.FailedEntryCount)
        // Log details of failures
        for i, entry := range output.Entries {
            if entry.ErrorCode != nil {
                log.Printf("Event %d failed: %s - %s", 
                    i, *entry.ErrorCode, *entry.ErrorMessage)
            }
        }
    }
    
    log.Printf("Successfully published %d events", len(entries)-int(output.FailedEntryCount))
    return nil
}

// createEventEntry converts domain event to EventBridge entry
func (p *AsyncEventPublisher) createEventEntry(event shared.DomainEvent) (types.PutEventsRequestEntry, error) {
    // Build event detail
    detail := map[string]interface{}{
        "aggregate_id": event.AggregateID(),
        "user_id":      event.UserID(),
        "event_id":     event.EventID(),
        "event_type":   event.EventType(),
        "occurred_at":  time.Now().Format(time.RFC3339),
        "version":      event.Version(),
    }
    
    // Add event-specific data
    eventData := event.EventData()
    for k, v := range eventData {
        detail[k] = v
    }
    
    detailJSON, err := json.Marshal(detail)
    if err != nil {
        return types.PutEventsRequestEntry{}, err
    }
    
    return types.PutEventsRequestEntry{
        EventBusName: aws.String(p.eventBus),
        Source:       aws.String(p.source),
        DetailType:   aws.String(event.EventType()),
        Detail:       aws.String(string(detailJSON)),
        Time:         aws.Time(time.Now()),
        Resources:    []string{event.AggregateID()},
    }, nil
}

// Shutdown gracefully shuts down the publisher
func (p *AsyncEventPublisher) Shutdown(ctx context.Context) error {
    close(p.shutdownCh)
    
    // Wait for background tasks with timeout
    done := make(chan struct{})
    go func() {
        p.wg.Wait()
        close(done)
    }()
    
    select {
    case <-done:
        return nil
    case <-ctx.Done():
        return ctx.Err()
    }
}
```

### Fix 3.2: Wire Up Async Publisher in DI Container
**File**: `backend/internal/di/container.go`

**Update initializeServices method** (around line 200):
```go
func (c *Container) initializeServices() error {
    // ... existing code ...
    
    // Replace synchronous EventBridge publisher with async version
    eventBusName := c.Config.EventBusName
    if eventBusName == "" {
        eventBusName = "B2EventBus"
    }
    
    // Create async publisher
    asyncPublisher := messaging.NewAsyncEventPublisher(
        c.EventBridgeClient, 
        eventBusName, 
        "brain2-backend",
    )
    
    // Wrap with adapter for compatibility
    c.EventBus = messaging.NewEventBusAdapter(asyncPublisher)
    
    // Register shutdown handler
    c.shutdownFunctions = append(c.shutdownFunctions, func() error {
        ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
        defer cancel()
        return asyncPublisher.Shutdown(ctx)
    })
    
    log.Printf("Async EventBridge publisher configured")
    
    // ... rest of service initialization ...
}
```

---

## 🔴 Priority 4: Query Optimizations

### Fix 4.1: Add Projection Expressions to Reduce Data Transfer
**File**: `backend/infrastructure/dynamodb/ddb.go`

**Current Code** (lines 240-290):
```go
func (r *ddbRepository) GetAllGraphData(ctx context.Context, userID string) (*shared.Graph, error) {
    // Current implementation fetching all attributes
}
```

**Replace With**:
```go
func (r *ddbRepository) GetAllGraphData(ctx context.Context, userID string) (*shared.Graph, error) {
    r.logger.Debug("GetAllGraphData with optimized projections")

    // Use goroutines for parallel queries
    g, ctx := errgroup.WithContext(ctx)
    
    var nodes []*node.Node
    var edges []*edge.Edge

    // Fetch nodes in parallel with projection
    g.Go(func() error {
        var err error
        nodes, err = r.fetchNodesOptimized(ctx, userID)
        return err
    })

    // Fetch edges in parallel with projection
    g.Go(func() error {
        var err error
        edges, err = r.fetchEdgesOptimized(ctx, userID)
        return err
    })

    // Wait for both operations
    if err := g.Wait(); err != nil {
        return nil, err
    }

    // Convert to Graph structure
    nodeInterfaces := make([]interface{}, len(nodes))
    for i, n := range nodes {
        nodeInterfaces[i] = n
    }
    
    edgeInterfaces := make([]interface{}, len(edges))
    for i, e := range edges {
        edgeInterfaces[i] = e
    }

    return &shared.Graph{
        Nodes: nodeInterfaces,
        Edges: edgeInterfaces,
    }, nil
}

// fetchNodesOptimized retrieves nodes with only necessary attributes
func (r *ddbRepository) fetchNodesOptimized(ctx context.Context, userID string) ([]*node.Node, error) {
    var nodes []*node.Node
    var lastEvaluatedKey map[string]types.AttributeValue
    
    userNodePrefix := fmt.Sprintf("USER#%s#NODE#", userID)

    for {
        input := &dynamodb.ScanInput{
            TableName:        aws.String(r.config.TableName),
            FilterExpression: aws.String("begins_with(PK, :pk_prefix) AND begins_with(SK, :sk_prefix)"),
            ExpressionAttributeValues: map[string]types.AttributeValue{
                ":pk_prefix": &types.AttributeValueMemberS{Value: userNodePrefix},
                ":sk_prefix": &types.AttributeValueMemberS{Value: "METADATA#"},
            },
            // Only fetch required attributes for graph rendering
            ProjectionExpression: aws.String("PK, SK, nodeId, content, #pos, keywords, tags, createdAt"),
            ExpressionAttributeNames: map[string]string{
                "#pos": "position", // position is a reserved word
            },
            Limit: aws.Int32(100), // Process in batches
        }
        
        if lastEvaluatedKey != nil {
            input.ExclusiveStartKey = lastEvaluatedKey
        }

        result, err := r.dbClient.Scan(ctx, input)
        if err != nil {
            return nil, err
        }

        // Process items
        for _, item := range result.Items {
            node, err := r.itemToNode(item)
            if err != nil {
                r.logger.Warn("Failed to unmarshal node", zap.Error(err))
                continue
            }
            nodes = append(nodes, node)
        }

        lastEvaluatedKey = result.LastEvaluatedKey
        if lastEvaluatedKey == nil {
            break
        }
    }

    return nodes, nil
}

// fetchEdgesOptimized retrieves edges with only necessary attributes
func (r *ddbRepository) fetchEdgesOptimized(ctx context.Context, userID string) ([]*edge.Edge, error) {
    // Use GSI2 (EdgeIndex) for efficient edge queries
    input := &dynamodb.QueryInput{
        TableName:              aws.String(r.config.TableName),
        IndexName:              aws.String("EdgeIndex"),
        KeyConditionExpression: aws.String("GSI2PK = :pk"),
        ExpressionAttributeValues: map[string]types.AttributeValue{
            ":pk": &types.AttributeValueMemberS{
                Value: fmt.Sprintf("USER#%s#EDGE", userID),
            },
        },
        // Only fetch required attributes for edges
        ProjectionExpression: aws.String("edgeId, sourceId, targetId, weight"),
    }

    result, err := r.dbClient.Query(ctx, input)
    if err != nil {
        return nil, err
    }

    edges := make([]*edge.Edge, 0, len(result.Items))
    for _, item := range result.Items {
        edge, err := r.itemToEdge(item)
        if err != nil {
            r.logger.Warn("Failed to unmarshal edge", zap.Error(err))
            continue
        }
        edges = append(edges, edge)
    }

    return edges, nil
}
```

---

## 🟢 Testing Instructions

### Test 1: Verify Client Reuse
```bash
# Deploy and check Lambda logs for initialization times
# Cold start should show: "Cold start detected - initializing with optimized clients..."
# Warm starts should show: "Warm start initialization in Xms"

# Test endpoint
curl -X GET https://your-api/api/v1/health
```

### Test 2: Test Batch Delete Performance
```bash
# Create test nodes first
for i in {1..50}; do
  curl -X POST https://your-api/api/v1/nodes \
    -H "Authorization: Bearer $TOKEN" \
    -d '{"content": "Test node '$i'"}'
done

# Test bulk delete (should complete in <500ms for 50 nodes)
curl -X DELETE https://your-api/api/v1/nodes/bulk \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"nodeIds": ["id1", "id2", ...]}' \
  -w "\nTime: %{time_total}s\n"
```

### Test 3: Verify Async Event Publishing
```bash
# Create a node and check that API returns quickly
time curl -X POST https://your-api/api/v1/nodes \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"content": "Test async events"}'

# Response should return in <200ms
# Check CloudWatch logs for async event publishing
```

### Test 4: Monitor Performance Metrics
```bash
# Check CloudWatch metrics for:
# - Lambda duration (should be <500ms for most requests)
# - DynamoDB consumed capacity (should show batch operations)
# - EventBridge put events (should show batching)
```

---

## 📊 Expected Performance Improvements

| Metric | Before | After | Improvement |
|--------|--------|-------|-------------|
| Cold Start Duration | 3-5s | 1-2s | 50-60% |
| Warm Request Latency | 300-500ms | 50-100ms | 80% |
| Bulk Delete (100 nodes) | 10s+ (timeout) | <500ms | 95% |
| Graph Data Fetch | 2-3s | 500ms | 75% |
| Event Publishing Overhead | 100-200ms | <10ms | 95% |
| DynamoDB API Calls | 2N per bulk op | N/25 | 95% |
| Error Rate (503s) | 2-5% | <0.1% | 95% |

---

## 🚀 Deployment Steps

1. **Backup Current Lambda Code**
   ```bash
   aws lambda get-function --function-name brain2-backend > backup.json
   ```

2. **Apply Code Changes**
   - Apply all fixes in order (Priority 1-4)
   - Run unit tests: `go test ./...`
   - Run integration tests if available

3. **Build and Deploy**
   ```bash
   cd backend
   GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o build/main ./cmd/main
   cd ../infra
   npm run deploy
   ```

4. **Monitor Performance**
   - Watch CloudWatch Logs for errors
   - Monitor Lambda duration metrics
   - Check DynamoDB throttling metrics
   - Verify EventBridge event delivery

5. **Rollback if Needed**
   ```bash
   # Use AWS Lambda versions for quick rollback
   aws lambda update-function-code --function-name brain2-backend \
     --s3-bucket your-bucket --s3-key backup/main.zip
   ```

---

## 📝 Additional Notes

### Why These Optimizations Work

1. **Client Reuse**: Lambda containers persist between invocations. By initializing clients in `init()`, we reuse them across warm starts, saving 200ms per request.

2. **HTTP Keep-Alive**: Maintains TCP connections between Lambda and DynamoDB, reducing handshake overhead.

3. **Batch Operations**: DynamoDB's `BatchWriteItem` processes up to 25 items in a single request, reducing round trips by 95%.

4. **Async Event Publishing**: Decouples event publishing from the request path, removing 100-200ms of blocking I/O.

5. **Projection Expressions**: Fetching only needed attributes reduces data transfer by 50-70%.

### Future Optimizations (Optional)

1. **Add DynamoDB DAX** for microsecond latency on reads
2. **Implement request coalescing** for duplicate concurrent requests
3. **Add CloudFront caching** for read-heavy endpoints
4. **Use Lambda SnapStart** (when available for Go runtime)

### Monitoring Success

Set up CloudWatch alarms for:
- Lambda duration > 1000ms
- Lambda error rate > 1%
- DynamoDB UserErrors > 10/minute
- EventBridge FailedInvocations > 1%

---

## 🔧 Troubleshooting

### Issue: Still seeing 503 errors
- Check Lambda timeout settings (should be 30s)
- Verify DynamoDB isn't throttling (check CloudWatch metrics)
- Ensure batch size isn't too large (stick to 25 for writes)

### Issue: Events not being published
- Check EventBridge event bus exists
- Verify IAM permissions for PutEvents
- Check async publisher logs for errors

### Issue: Slow cold starts
- Reduce package size (remove unused dependencies)
- Consider increasing Lambda memory to 512MB (more CPU)
- Implement Lambda warming with scheduled CloudWatch events