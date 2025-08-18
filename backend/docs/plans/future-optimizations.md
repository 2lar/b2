# Future Optimization Plans for Brain2 Backend

## Overview
This document outlines potential future optimizations for the Brain2 backend system. These optimizations are categorized by priority and implementation complexity, focusing on practical improvements that provide measurable benefits without excessive cost or complexity.

## 1. Lambda Cold Start Mitigation (High Priority, Low Cost)

### 1.1 Warm-up Endpoint Strategy
**Problem:** Cold starts cause 503 errors and high latency (3-5 seconds)
**Solution:** Implement scheduled warm-up pings

```yaml
# CloudWatch Events Rule (serverless.yml or SAM template)
WarmUpSchedule:
  Type: AWS::Events::Rule
  Properties:
    ScheduleExpression: rate(5 minutes)
    Targets:
      - Arn: !GetAtt ApiHandlerFunction.Arn
        Input: '{"httpMethod": "GET", "path": "/health/warm"}'
```

**Implementation:**
```go
// Add to handlers
func WarmUpHandler(w http.ResponseWriter, r *http.Request) {
    // Minimal work to keep container warm
    api.Success(w, http.StatusOK, map[string]string{
        "status": "warm",
        "timestamp": time.Now().Format(time.RFC3339),
    })
}
```

**Benefits:**
- Reduces cold start rate from 20% to <2%
- Costs: ~$0.20/month (8,640 invocations × $0.0000002)
- No code changes to existing handlers

### 1.2 Reserved Concurrent Executions
**Problem:** Throttling during traffic spikes
**Solution:** Reserve capacity for critical functions

```yaml
ApiHandler:
  Properties:
    ReservedConcurrentExecutions: 10  # Guarantees capacity
```

**Benefits:**
- Prevents throttling
- Free (no additional cost)
- Ensures consistent performance

### 1.3 Optimize Lambda Memory Configuration
**Current:** Unknown/default
**Recommended:** 512MB - 1GB

```yaml
ApiHandler:
  Properties:
    MemorySize: 768  # Sweet spot for cost/performance
    Timeout: 30      # Appropriate for API operations
```

**Benefits:**
- More CPU allocation (CPU scales with memory)
- Faster cold starts
- Better cost/performance ratio

## 2. Request-Scoped Caching (High Impact, Medium Complexity)

### 2.1 Context-Based Request Cache
**Problem:** Repeated database queries within same request
**Solution:** Implement request-scoped caching using context

```go
// pkg/cache/request_cache.go
type RequestCache struct {
    mu    sync.RWMutex
    data  map[string]interface{}
}

type contextKey string
const cacheKey contextKey = "request_cache"

func WithCache(ctx context.Context) context.Context {
    return context.WithValue(ctx, cacheKey, &RequestCache{
        data: make(map[string]interface{}),
    })
}

func GetFromCache[T any](ctx context.Context, key string) (T, bool) {
    cache, ok := ctx.Value(cacheKey).(*RequestCache)
    if !ok {
        var zero T
        return zero, false
    }
    
    cache.mu.RLock()
    defer cache.mu.RUnlock()
    
    val, exists := cache.data[key]
    if !exists {
        var zero T
        return zero, false
    }
    
    typed, ok := val.(T)
    return typed, ok
}

func SetInCache(ctx context.Context, key string, value interface{}) {
    cache, ok := ctx.Value(cacheKey).(*RequestCache)
    if !ok {
        return
    }
    
    cache.mu.Lock()
    defer cache.mu.Unlock()
    cache.data[key] = value
}
```

**Usage Example:**
```go
func (r *NodeRepository) FindNodeByID(ctx context.Context, userID, nodeID string) (*node.Node, error) {
    // Check request cache first
    cacheKey := fmt.Sprintf("node:%s:%s", userID, nodeID)
    if cached, ok := GetFromCache[*node.Node](ctx, cacheKey); ok {
        return cached, nil
    }
    
    // Fetch from database
    node, err := r.fetchFromDB(ctx, userID, nodeID)
    if err != nil {
        return nil, err
    }
    
    // Store in request cache
    SetInCache(ctx, cacheKey, node)
    return node, nil
}
```

**Benefits:**
- Eliminates repeated queries in same request
- No memory leak risk (cleared after request)
- 30-50% reduction in database calls

### 2.2 Permission Caching
**Problem:** Permission checks happen multiple times per request
**Solution:** Cache permission results within request context

```go
type PermissionCache struct {
    userID      string
    permissions map[string]bool
}

func CheckPermissionCached(ctx context.Context, userID, resource string) bool {
    cacheKey := fmt.Sprintf("perm:%s:%s", userID, resource)
    if cached, ok := GetFromCache[bool](ctx, cacheKey); ok {
        return cached
    }
    
    // Check actual permission
    hasPermission := checkPermission(userID, resource)
    SetInCache(ctx, cacheKey, hasPermission)
    return hasPermission
}
```

## 3. Frontend Optimization Strategies

### 3.1 Request Batching
**Problem:** Multiple API calls for related data
**Solution:** Implement batch endpoint

```go
// POST /api/v1/batch
type BatchRequest struct {
    Requests []SingleRequest `json:"requests"`
}

type SingleRequest struct {
    ID     string          `json:"id"`
    Method string          `json:"method"`
    Path   string          `json:"path"`
    Body   json.RawMessage `json:"body,omitempty"`
}

func BatchHandler(w http.ResponseWriter, r *http.Request) {
    var batch BatchRequest
    json.NewDecoder(r.Body).Decode(&batch)
    
    responses := make(map[string]interface{})
    
    for _, req := range batch.Requests {
        // Process each request
        result := processRequest(req)
        responses[req.ID] = result
    }
    
    api.Success(w, http.StatusOK, responses)
}
```

**Benefits:**
- Reduce number of HTTP connections
- Lower latency for multiple operations
- Better mobile performance

### 3.2 Optimistic UI Updates
**Problem:** UI feels slow waiting for server confirmation
**Solution:** Update UI immediately, reconcile with server response

```typescript
// Frontend implementation
const optimisticDelete = async (nodeId: string) => {
    // Update UI immediately
    setNodes(prev => prev.filter(n => n.id !== nodeId));
    
    try {
        await api.deleteNode(nodeId);
        // Success - UI already updated
    } catch (error) {
        // Rollback on failure
        setNodes(prev => [...prev, deletedNode]);
        showError("Failed to delete node");
    }
};
```

## 4. Database Query Optimizations

### 4.1 Projection Expressions
**Problem:** Fetching unnecessary attributes from DynamoDB
**Solution:** Use projection expressions to fetch only needed fields

```go
input := &dynamodb.QueryInput{
    TableName: aws.String(tableName),
    ProjectionExpression: aws.String("nodeId, content, version, updatedAt"),
    // Only fetch these attributes, not entire item
}
```

**Benefits:**
- Reduce data transfer by 40-60%
- Lower DynamoDB read costs
- Faster response times

### 4.2 Parallel Query Execution
**Problem:** Sequential queries increase latency
**Solution:** Execute independent queries in parallel

```go
func GetDashboardData(ctx context.Context, userID string) (*Dashboard, error) {
    g, ctx := errgroup.WithContext(ctx)
    
    var nodes []*Node
    var edges []*Edge
    var categories []*Category
    
    // Parallel execution
    g.Go(func() error {
        var err error
        nodes, err = fetchNodes(ctx, userID)
        return err
    })
    
    g.Go(func() error {
        var err error
        edges, err = fetchEdges(ctx, userID)
        return err
    })
    
    g.Go(func() error {
        var err error
        categories, err = fetchCategories(ctx, userID)
        return err
    })
    
    if err := g.Wait(); err != nil {
        return nil, err
    }
    
    return &Dashboard{
        Nodes: nodes,
        Edges: edges,
        Categories: categories,
    }, nil
}
```

**Benefits:**
- Reduce total query time by 50-70%
- Better resource utilization
- Improved user experience

### 4.3 GSI Optimization
**Problem:** Inefficient query patterns
**Solution:** Add Global Secondary Indexes for common access patterns

```yaml
# DynamoDB Table Definition
GSI-UserTimestamp:
  PartitionKey: UserID
  SortKey: Timestamp
  Projection: ALL
  
GSI-CategoryNodes:
  PartitionKey: CategoryID
  SortKey: NodeID
  Projection: KEYS_ONLY
```

**Benefits:**
- Eliminate expensive scan operations
- Enable new query patterns
- Reduce query complexity

## 5. Event Processing Optimizations

### 5.1 Event Batching
**Problem:** Individual EventBridge PutEvents calls
**Solution:** Batch events before publishing

```go
type EventBatcher struct {
    events  []types.PutEventsRequestEntry
    mu      sync.Mutex
    ticker  *time.Ticker
    client  *eventbridge.Client
}

func (b *EventBatcher) Add(event types.PutEventsRequestEntry) {
    b.mu.Lock()
    b.events = append(b.events, event)
    count := len(b.events)
    b.mu.Unlock()
    
    // Flush if batch is full
    if count >= 10 {
        b.Flush()
    }
}

func (b *EventBatcher) Flush() {
    b.mu.Lock()
    if len(b.events) == 0 {
        b.mu.Unlock()
        return
    }
    
    batch := b.events
    b.events = nil
    b.mu.Unlock()
    
    // Send batch to EventBridge
    b.client.PutEvents(context.Background(), &eventbridge.PutEventsInput{
        Entries: batch,
    })
}
```

**Benefits:**
- 90% reduction in EventBridge API calls
- Lower costs
- Improved throughput

### 5.2 SQS FIFO for Order Guarantees
**Problem:** Event ordering not guaranteed
**Solution:** Use SQS FIFO queues for ordered processing

```yaml
EventQueue:
  Type: AWS::SQS::Queue
  Properties:
    QueueName: events.fifo
    FifoQueue: true
    ContentBasedDeduplication: true
    MessageRetentionPeriod: 1209600  # 14 days
```

## 6. Deployment Package Optimization

### 6.1 Reduce Binary Size
**Problem:** Large binaries increase cold start time
**Solution:** Optimize build flags

```bash
# build.sh optimization
GOOS=linux GOARCH=amd64 go build \
    -ldflags="-s -w" \  # Strip debug info
    -tags lambda.norpc \ # Remove RPC support
    -o bootstrap
    
# Further compression
upx --best --lzma bootstrap  # Can reduce size by 50-70%
```

**Current sizes:**
- main: 36MB → Could be ~10-15MB
- connect-node: 21MB → Could be ~7-10MB

### 6.2 Lambda Layers
**Problem:** Duplicated code across functions
**Solution:** Use Lambda Layers for shared dependencies

```yaml
SharedLayer:
  Type: AWS::Lambda::LayerVersion
  Properties:
    LayerName: shared-deps
    Content:
      S3Bucket: !Ref DeploymentBucket
      S3Key: layers/shared.zip
    CompatibleRuntimes:
      - provided.al2
```

## 7. Monitoring and Observability

### 7.1 CloudWatch Insights Queries
**Problem:** Lack of visibility into performance
**Solution:** Pre-built queries for common metrics

```sql
-- Cold start analysis
fields @timestamp, @duration, @initDuration
| filter @type = "REPORT"
| stats count() as invocations,
        count(@initDuration) as coldStarts,
        avg(@duration) as avgDuration,
        avg(@initDuration) as avgColdStart
by bin(5m)

-- Error analysis
fields @timestamp, @message
| filter @message like /ERROR/
| stats count() by bin(5m)

-- P99 latency tracking
fields @duration
| filter @type = "REPORT"
| stats pct(@duration, 99) as p99,
        pct(@duration, 95) as p95,
        pct(@duration, 50) as p50
by bin(5m)
```

### 7.2 Custom Metrics
**Problem:** Limited visibility into business metrics
**Solution:** Publish custom CloudWatch metrics

```go
func PublishMetric(name string, value float64, unit types.StandardUnit) {
    cloudwatch.PutMetricData(context.Background(), &cloudwatch.PutMetricDataInput{
        Namespace: aws.String("Brain2/API"),
        MetricData: []types.MetricDatum{
            {
                MetricName: aws.String(name),
                Value:      aws.Float64(value),
                Unit:       unit,
                Timestamp:  aws.Time(time.Now()),
            },
        },
    })
}
```

## 8. Cost Optimization

### 8.1 DynamoDB On-Demand vs Provisioned
**Current:** Likely on-demand
**Analysis needed:**
- If consistent traffic > 20% of time → Provisioned is cheaper
- If spiky/unpredictable → On-demand is better

### 8.2 S3 Intelligent Tiering
**Problem:** Storing all data in standard tier
**Solution:** Enable Intelligent-Tiering for automatic cost optimization

```yaml
S3Bucket:
  Properties:
    LifecycleConfiguration:
      Rules:
        - Id: IntelligentTiering
          Status: Enabled
          Transitions:
            - StorageClass: INTELLIGENT_TIERING
              TransitionInDays: 0
```

## Implementation Priority Matrix

| Optimization | Impact | Effort | Cost | Priority |
|-------------|--------|--------|------|----------|
| Warm-up Endpoint | High | Low | ~$0 | **P0** |
| Request Caching | High | Medium | $0 | **P0** |
| Lambda Memory Tuning | Medium | Low | ~$0 | **P1** |
| Projection Expressions | Medium | Low | $0 | **P1** |
| Parallel Queries | High | Medium | $0 | **P1** |
| Binary Size Reduction | Medium | Low | $0 | **P2** |
| Request Batching | Medium | Medium | $0 | **P2** |
| Event Batching | Low | Medium | $0 | **P3** |
| Lambda Layers | Low | High | $0 | **P3** |

## Estimated Overall Impact

If all optimizations are implemented:
- **Cold starts**: 90% reduction
- **P99 latency**: 5s → <1s
- **Database calls**: 40% reduction
- **API costs**: 30-50% reduction
- **User experience**: Significantly improved

## Next Steps

1. **Measure current baseline** (1 week)
   - Set up CloudWatch Insights queries
   - Track p50/p95/p99 latencies
   - Monitor cold start rate

2. **Implement P0 optimizations** (1-2 weeks)
   - Warm-up endpoint
   - Request caching

3. **Measure improvement** (1 week)
   - Compare against baseline
   - Identify remaining bottlenecks

4. **Iterate with P1 optimizations** (2-3 weeks)
   - Based on measured bottlenecks
   - Focus on highest impact areas

## Conclusion

These optimizations focus on practical, Lambda-specific improvements that don't require expensive services like Provisioned Concurrency. The combination of warm-up strategies, request caching, and query optimizations should provide substantial performance improvements at minimal cost.