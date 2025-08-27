# Backend Enhancement Plan - Priority Improvements

## Overview
This document outlines the priority improvements identified to elevate the backend architecture from its current excellent state (85-90% complete) to near-perfection. These enhancements focus on practical, high-impact areas that will improve maintainability, performance, and production readiness.

## Priority 1: API Versioning Strategy

### Current State
- No explicit API versioning mechanism
- All endpoints served under `/api/` without version prefix
- No backward compatibility guarantees

### Target Implementation
```go
// 1. URL Path Versioning (Recommended)
/api/v1/nodes
/api/v2/nodes  // Breaking changes

// 2. Header-based versioning alternative
Accept: application/vnd.brain2.v1+json

// 3. Version middleware
type APIVersion struct {
    Major int
    Minor int
    Patch int
}

func VersionMiddleware(minVersion, maxVersion APIVersion) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            version := extractVersion(r)
            if !isVersionSupported(version, minVersion, maxVersion) {
                api.Error(w, http.StatusGone, "API version not supported")
                return
            }
            ctx := context.WithValue(r.Context(), "api_version", version)
            next.ServeHTTP(w, r.WithContext(ctx))
        })
    }
}
```

### Implementation Steps
1. Add version extraction middleware
2. Implement version-specific routers
3. Create deprecation headers for old versions
4. Add version documentation

## Priority 2: Domain Model Refinements

### Issues to Address
1. **Anemic models in some areas**
   - Some DTOs have logic that belongs in domain
   - Missing behavior in Category and Edge entities

2. **Inconsistent validation**
   - Validation scattered between layers
   - Some entities missing invariant checks

3. **Missing domain events**
   - Edge entity lacks proper event generation
   - Category updates don't emit events

### Target Implementation
```go
// Enhanced Edge entity with rich behavior
type Edge struct {
    shared.BaseAggregateRoot
    id         shared.EdgeID
    sourceNode shared.NodeID
    targetNode shared.NodeID
    weight     Weight        // Value object with validation
    metadata   EdgeMetadata
    createdAt  time.Time
    updatedAt  time.Time
    version    shared.Version
}

// Add missing behavior
func (e *Edge) UpdateWeight(newWeight float64) error {
    if e.weight.Value() == newWeight {
        return nil
    }
    
    oldWeight := e.weight
    e.weight = NewWeight(newWeight)
    e.updatedAt = time.Now()
    e.version = e.version.Next()
    
    // Emit domain event
    e.AddEvent(EdgeWeightUpdatedEvent{
        EdgeID:    e.id,
        OldWeight: oldWeight,
        NewWeight: e.weight,
    })
    
    return nil
}

// Add invariant validation
func (e *Edge) ValidateInvariants() error {
    if e.sourceNode.Equals(e.targetNode) {
        return ErrSelfConnection
    }
    if !e.weight.IsValid() {
        return ErrInvalidWeight
    }
    return nil
}
```

## Priority 3: Repository Code Duplication Reduction

### Current Issues
- Similar CRUD patterns repeated across repositories
- Duplicate error handling logic
- Common query patterns reimplemented

### Solution: Generic Repository Base
```go
// internal/infrastructure/persistence/dynamodb/base_repository.go
type BaseRepository[T any, ID comparable] struct {
    client    *dynamodb.Client
    tableName string
    logger    *zap.Logger
}

func (r *BaseRepository[T, ID]) FindByID(ctx context.Context, pk, sk string) (*T, error) {
    // Common implementation
    key := map[string]types.AttributeValue{
        "PK": &types.AttributeValueMemberS{Value: pk},
        "SK": &types.AttributeValueMemberS{Value: sk},
    }
    
    result, err := r.client.GetItem(ctx, &dynamodb.GetItemInput{
        TableName: aws.String(r.tableName),
        Key:       key,
    })
    
    if err != nil {
        return nil, r.wrapError(err, "FindByID", pk, sk)
    }
    
    if result.Item == nil {
        return nil, ErrNotFound
    }
    
    var entity T
    if err := attributevalue.UnmarshalMap(result.Item, &entity); err != nil {
        return nil, r.wrapError(err, "UnmarshalMap", pk, sk)
    }
    
    return &entity, nil
}

func (r *BaseRepository[T, ID]) BatchGet(ctx context.Context, keys []map[string]types.AttributeValue) ([]T, error) {
    // Common batch implementation with chunking
    const batchSize = 100
    var results []T
    
    for i := 0; i < len(keys); i += batchSize {
        end := min(i+batchSize, len(keys))
        batch := keys[i:end]
        
        // Execute batch with retry logic
        items, err := r.executeBatchGet(ctx, batch)
        if err != nil {
            return nil, err
        }
        results = append(results, items...)
    }
    
    return results, nil
}

// Specific repositories compose the base
type NodeRepository struct {
    *BaseRepository[node.Node, shared.NodeID]
    // Additional node-specific methods
}
```

## Priority 4: Standardized Error Handling

### Current State
- Multiple error packages (pkg/errors, internal/errors, domain/shared)
- Inconsistent error wrapping
- Missing context in some errors

### Unified Error System
```go
// internal/errors/errors.go
type AppError struct {
    Type       ErrorType
    Code       string
    Message    string
    Details    map[string]interface{}
    Cause      error
    StackTrace []string
    RequestID  string
    UserID     string
    Operation  string
    Timestamp  time.Time
}

// Domain layer errors
func NewDomainError(code string, message string, cause error) *AppError {
    return &AppError{
        Type:      ErrorTypeDomain,
        Code:      code,
        Message:   message,
        Cause:     cause,
        Timestamp: time.Now(),
        StackTrace: captureStackTrace(),
    }
}

// Repository layer errors
func NewRepositoryError(operation string, cause error) *AppError {
    return &AppError{
        Type:      ErrorTypeInfrastructure,
        Code:      "REPO_ERROR",
        Operation: operation,
        Message:   fmt.Sprintf("Repository operation failed: %s", operation),
        Cause:     cause,
        Timestamp: time.Now(),
        StackTrace: captureStackTrace(),
    }
}

// Error middleware to enrich with context
func ErrorEnrichmentMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // Wrap response writer to capture errors
        wrapped := &errorCapturingResponseWriter{ResponseWriter: w}
        
        defer func() {
            if err := recover(); err != nil {
                appErr := ConvertToAppError(err)
                appErr.RequestID = middleware.GetReqID(r.Context())
                appErr.UserID = getUserID(r.Context())
                
                logger.Error("Request failed", 
                    zap.String("request_id", appErr.RequestID),
                    zap.String("user_id", appErr.UserID),
                    zap.Error(appErr))
                
                writeErrorResponse(w, appErr)
            }
        }()
        
        next.ServeHTTP(wrapped, r)
    })
}
```

## Priority 5: Structured Logging with Correlation IDs

### Implementation Plan
```go
// internal/observability/logging.go
type StructuredLogger struct {
    *zap.Logger
}

func NewStructuredLogger() *StructuredLogger {
    config := zap.NewProductionConfig()
    config.OutputPaths = []string{"stdout"}
    config.ErrorOutputPaths = []string{"stderr"}
    
    logger, _ := config.Build(
        zap.AddCaller(),
        zap.AddStacktrace(zap.ErrorLevel),
    )
    
    return &StructuredLogger{logger}
}

// Context-aware logging
func (l *StructuredLogger) WithContext(ctx context.Context) *StructuredLogger {
    fields := []zap.Field{
        zap.String("correlation_id", GetCorrelationID(ctx)),
        zap.String("request_id", GetRequestID(ctx)),
        zap.String("user_id", GetUserID(ctx)),
        zap.String("trace_id", GetTraceID(ctx)),
    }
    
    return &StructuredLogger{l.Logger.With(fields...)}
}

// Correlation ID middleware
func CorrelationIDMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        correlationID := r.Header.Get("X-Correlation-ID")
        if correlationID == "" {
            correlationID = uuid.New().String()
        }
        
        ctx := context.WithValue(r.Context(), "correlation_id", correlationID)
        w.Header().Set("X-Correlation-ID", correlationID)
        
        next.ServeHTTP(w, r.WithContext(ctx))
    })
}

// Usage in services
func (s *NodeService) CreateNode(ctx context.Context, cmd *CreateNodeCommand) (*NodeDTO, error) {
    logger := s.logger.WithContext(ctx)
    logger.Info("Creating node",
        zap.String("operation", "CreateNode"),
        zap.Int("content_length", len(cmd.Content)))
    
    // Service logic with consistent logging
    defer func() {
        if r := recover(); r != nil {
            logger.Error("Panic in CreateNode",
                zap.Any("panic", r),
                zap.Stack("stack"))
            panic(r) // Re-panic after logging
        }
    }()
    
    // ... rest of implementation
}
```

## Priority 6: Comprehensive Health Checks

### Health Check System
```go
// internal/health/checks.go
type HealthChecker interface {
    Name() string
    Check(ctx context.Context) HealthStatus
}

type HealthStatus struct {
    Status      string                 `json:"status"` // "up", "down", "degraded"
    Message     string                 `json:"message,omitempty"`
    Details     map[string]interface{} `json:"details,omitempty"`
    LastChecked time.Time             `json:"last_checked"`
    Duration    time.Duration         `json:"duration_ms"`
}

// DynamoDB health check
type DynamoDBHealthCheck struct {
    client    *dynamodb.Client
    tableName string
}

func (h *DynamoDBHealthCheck) Check(ctx context.Context) HealthStatus {
    start := time.Now()
    
    _, err := h.client.DescribeTable(ctx, &dynamodb.DescribeTableInput{
        TableName: aws.String(h.tableName),
    })
    
    status := HealthStatus{
        LastChecked: time.Now(),
        Duration:    time.Since(start),
        Details: map[string]interface{}{
            "table": h.tableName,
        },
    }
    
    if err != nil {
        status.Status = "down"
        status.Message = err.Error()
    } else {
        status.Status = "up"
    }
    
    return status
}

// Composite health endpoint
type HealthService struct {
    checkers []HealthChecker
}

func (s *HealthService) GetHealth(ctx context.Context) map[string]interface{} {
    results := make(map[string]HealthStatus)
    overall := "up"
    
    var wg sync.WaitGroup
    mu := sync.Mutex{}
    
    for _, checker := range s.checkers {
        wg.Add(1)
        go func(c HealthChecker) {
            defer wg.Done()
            
            status := c.Check(ctx)
            
            mu.Lock()
            results[c.Name()] = status
            if status.Status == "down" {
                overall = "down"
            } else if status.Status == "degraded" && overall != "down" {
                overall = "degraded"
            }
            mu.Unlock()
        }(checker)
    }
    
    wg.Wait()
    
    return map[string]interface{}{
        "status":     overall,
        "timestamp":  time.Now(),
        "components": results,
    }
}
```

## Priority 7: Optimized Goroutine Usage

### Current State Assessment
You already have some goroutine optimizations, but they're LIMITED to specific areas:

**What You Have ✅:**
1. **Infrastructure Layer:**
   - Parallel DynamoDB scanning with 4 segments (`infrastructure/dynamodb/ddb.go`)
   - Batch operations with semaphore pattern (`internal/repository/transaction.go`)
   - Sequential chunk processing in `BatchDeleteOrchestrator`

2. **Basic Concurrency:**
   - Event publishing with goroutines
   - Timeout middleware with goroutines
   - Background cache cleanup

**What You're MISSING ❌:**
1. **Application Service Layer:** All bulk operations are SEQUENTIAL
   - `BulkCreateNodes` processes nodes one-by-one
   - No parallel connection analysis
   - Sequential validation and processing

2. **Repository Layer:** Limited parallel batch operations
   - `BatchGetNodes` could be parallelized
   - No concurrent fetching strategies

3. **No Worker Pool Patterns:** Missing reusable worker pools

### Required Enhancements

#### 1. Parallel Bulk Node Creation (Currently Sequential)
```go
// internal/application/services/node_service_parallel.go

// CURRENT: Sequential processing
// for i, nodeReq := range cmd.Nodes {
//     node, err := node.NewNode(...) // One by one
// }

// ENHANCED: Parallel node creation with controlled concurrency
func (s *NodeService) BulkCreateNodesOptimized(ctx context.Context, cmd *BulkCreateCommand) (*BulkCreateResult, error) {
    const maxConcurrency = 10
    semaphore := make(chan struct{}, maxConcurrency)
    
    var (
        mu           sync.Mutex
        wg           sync.WaitGroup
        createdNodes []*node.Node
        errors       []error
    )
    
    // Process nodes in parallel with controlled concurrency
    for i, nodeData := range cmd.Nodes {
        wg.Add(1)
        semaphore <- struct{}{} // Acquire semaphore
        
        go func(idx int, data NodeData) {
            defer wg.Done()
            defer func() { <-semaphore }() // Release semaphore
            
            // Create node with timeout
            nodeCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
            defer cancel()
            
            node, err := s.createSingleNode(nodeCtx, data)
            
            mu.Lock()
            if err != nil {
                errors = append(errors, fmt.Errorf("node %d: %w", idx, err))
            } else {
                createdNodes = append(createdNodes, node)
            }
            mu.Unlock()
        }(i, nodeData)
    }
    
    wg.Wait()
    
    // Parallel connection analysis
    connections := s.analyzeConnectionsParallel(ctx, createdNodes)
    
    return &BulkCreateResult{
        CreatedNodes: createdNodes,
        Connections:  connections,
        Errors:       errors,
    }, nil
}

#### 2. Parallel Connection Analysis (Currently Sequential O(n²))
```go
// CURRENT: Nested loops, sequential processing
// for i, sourceNode := range createdNodes {
//     for j, targetNode := range createdNodes {
//         if i >= j { continue } // Sequential analysis
//     }
// }

// ENHANCED: Worker pool pattern for connection analysis
func (s *NodeService) analyzeConnectionsParallel(ctx context.Context, nodes []*node.Node) []*edge.Edge {
    type connectionJob struct {
        source *node.Node
        target *node.Node
    }
    
    // Create job channel
    jobs := make(chan connectionJob, len(nodes)*len(nodes))
    results := make(chan *edge.Edge, len(nodes)*len(nodes))
    
    // Start workers
    const numWorkers = 5
    var wg sync.WaitGroup
    
    for w := 0; w < numWorkers; w++ {
        wg.Add(1)
        go func() {
            defer wg.Done()
            for job := range jobs {
                if edge := s.analyzeConnection(job.source, job.target); edge != nil {
                    results <- edge
                }
            }
        }()
    }
    
    // Send jobs
    for i, source := range nodes {
        for j, target := range nodes {
            if i < j { // Avoid duplicates
                jobs <- connectionJob{source, target}
            }
        }
    }
    close(jobs)
    
    // Collect results
    go func() {
        wg.Wait()
        close(results)
    }()
    
    var edges []*edge.Edge
    for edge := range results {
        edges = append(edges, edge)
    }
    
    return edges
}

```

#### 3. Optimized Repository Batch Operations
```go
// CURRENT: Limited parallelism in batch operations
// Your BatchGetNodes doesn't leverage full parallel fetching

// ENHANCED: Parallel repository operations with chunking
func (r *NodeRepository) BatchGetOptimized(ctx context.Context, nodeIDs []string) (map[string]*node.Node, error) {
    // Split into chunks for parallel processing
    const chunkSize = 25
    chunks := chunkSlice(nodeIDs, chunkSize)
    
    resultChan := make(chan map[string]*node.Node, len(chunks))
    errChan := make(chan error, len(chunks))
    
    var wg sync.WaitGroup
    for _, chunk := range chunks {
        wg.Add(1)
        go func(ids []string) {
            defer wg.Done()
            
            nodes, err := r.batchGetChunk(ctx, ids)
            if err != nil {
                errChan <- err
                return
            }
            resultChan <- nodes
        }(chunk)
    }
    
    go func() {
        wg.Wait()
        close(resultChan)
        close(errChan)
    }()
    
    // Collect results
    allNodes := make(map[string]*node.Node)
    for nodes := range resultChan {
        for id, node := range nodes {
            allNodes[id] = node
        }
    }
    
    // Check for errors
    select {
    case err := <-errChan:
        return nil, err
    default:
        return allNodes, nil
    }
}
```

#### 4. Worker Pool Pattern for Reusable Concurrency
```go
// MISSING: No reusable worker pool pattern

// ENHANCED: Generic worker pool for any job type
type WorkerPool[T any, R any] struct {
    workers   int
    jobQueue  chan T
    results   chan R
    processor func(T) R
}

func NewWorkerPool[T any, R any](workers int, processor func(T) R) *WorkerPool[T, R] {
    pool := &WorkerPool[T, R]{
        workers:   workers,
        jobQueue:  make(chan T, workers*2),
        results:   make(chan R, workers*2),
        processor: processor,
    }
    pool.start()
    return pool
}

func (p *WorkerPool[T, R]) start() {
    for i := 0; i < p.workers; i++ {
        go func() {
            for job := range p.jobQueue {
                p.results <- p.processor(job)
            }
        }()
    }
}

// Usage example
nodeProcessor := NewWorkerPool(10, func(data NodeData) *node.Node {
    node, _ := createNode(data)
    return node
})
```

### Performance Impact
- **Current:** Bulk operations with 100 nodes take ~10 seconds (sequential)
- **Enhanced:** Same operation would take ~1-2 seconds (10x improvement with 10 workers)
- **Connection Analysis:** O(n²) to O(n²/workers) time complexity improvement

## Priority 8: WebSocket Support for Real-time

### Current State
**You ALREADY HAVE WebSocket support** via AWS API Gateway WebSocket APIs:
- Lambda functions: `ws-connect`, `ws-disconnect`, `ws-send-message`
- EventBridge integration for real-time updates
- DynamoDB for connection management

### Recommended Enhancement
Instead of replacing your serverless WebSocket, enhance the existing implementation:

```go
// Enhanced message types for your existing WebSocket Lambda
type EnhancedWebSocketMessage struct {
    Action    string                 `json:"action"`
    Type      string                 `json:"type"` // "node.created", "edge.updated", etc.
    NodeID    string                 `json:"nodeId,omitempty"`
    Data      map[string]interface{} `json:"data"`
    Version   int                    `json:"version"`
    Timestamp time.Time              `json:"timestamp"`
}

// cmd/ws-send-message/main.go - Enhanced event broadcasting
func handleEnhancedEvent(ctx context.Context, event events.EventBridgeEvent) error {
    // CURRENT: Basic message
    // message := map[string]string{
    //     "action": "graphUpdated",
    //     "nodeId": detail.NodeID,
    // }
    
    // ENHANCED: Rich event data
    var detail map[string]interface{}
    json.Unmarshal(event.Detail, &detail)
    
    message := EnhancedWebSocketMessage{
        Action:    determineAction(event.DetailType),
        Type:      event.DetailType,
        NodeID:    detail["nodeId"].(string),
        Data:      detail,
        Version:   detail["version"].(int),
        Timestamp: event.Time,
    
    // Broadcast to all connections
    _, err = apiGatewayManagementClient.PostToConnection(ctx, &apigatewaymanagementapi.PostToConnectionInput{
        ConnectionId: &connectionID,
        Data:         messageBytes,
    })
    
    return err
}

// Additional enhancements for serverless WebSocket
func enhanceWebSocketSupport() {
    // 1. Add subscription filtering
    // Store user preferences in DynamoDB alongside connections
    
    // 2. Add heartbeat/keepalive
    // Periodic ping to maintain connections
    
    // 3. Add message batching
    // Group multiple events for efficiency
    
    // 4. Add event replay
    // Allow clients to request missed events
}
```

## Implementation Priority Order

1. **Week 1-2: Foundation**
   - Standardized error handling
   - Structured logging with correlation IDs
   - Health checks

2. **Week 3-4: Code Quality**
   - Repository code deduplication
   - Domain model refinements
   - Comprehensive API versioning

3. **Week 5-6: Performance & Features**
   - Optimized goroutine usage
   - WebSocket support for real-time

## Success Metrics

- **Code Quality**
  - 40% reduction in repository code duplication
  - 100% correlation ID coverage in logs
  - All domain entities with proper invariant validation

- **Performance**
  - 50% improvement in bulk operations
  - <100ms p99 latency for parallel operations
  - Real-time updates within 50ms

- **API Maturity**
  - Version strategy implemented
  - Deprecation warnings in place
  - WebSocket connections stable

## Testing Strategy

Each enhancement should include:
1. Unit tests for new functionality
2. Integration tests for API changes
3. Load tests for performance improvements
4. E2E tests for WebSocket functionality

## Migration Notes

- API versioning can be rolled out gradually
- Error handling changes should be backward compatible
- WebSocket is additive, no breaking changes
- Repository refactoring should maintain existing interfaces