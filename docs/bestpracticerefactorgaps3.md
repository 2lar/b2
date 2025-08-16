# Brain2 Backend Enhancement Tasks - Path to 10/10

## Overview
This document provides detailed, actionable tasks to enhance your backend codebase from 8.5/10 to 10/10. Each task includes specific implementation details, code examples, and file paths.

---

## 🚨 Priority 1: Critical Code Cleanup (Day 1)

### Task 1.1: Remove Dead Code and Optimize Swagger Handling

#### A. Remove Large Generated API File
```bash
# Delete the file with embedded base64 swagger
rm backend/pkg/api/generated-api.go
```

#### B. Create External Swagger Management
```go
// backend/pkg/api/swagger.go
package api

import (
    "embed"
    "encoding/json"
    "net/http"
)

//go:embed swagger.yaml
var swaggerFS embed.FS

// GetSwaggerSpec returns the swagger specification
func GetSwaggerSpec() ([]byte, error) {
    return swaggerFS.ReadFile("swagger.yaml")
}

// SwaggerHandler serves the swagger specification
func SwaggerHandler() http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        spec, err := GetSwaggerSpec()
        if err != nil {
            http.Error(w, "Failed to load swagger spec", http.StatusInternalServerError)
            return
        }
        
        w.Header().Set("Content-Type", "application/yaml")
        w.Write(spec)
    }
}
```

#### C. Move Swagger Spec
```bash
# Move the swagger spec to the api package
cp openapi.yaml backend/pkg/api/swagger.yaml
```

### Task 1.2: Clean Up Mock Implementations in Container

#### Replace Mock Implementations with Real Ones

```go
// backend/internal/infrastructure/transactions/provider.go
package transactions

import (
    "context"
    "fmt"
    
    "github.com/aws/aws-sdk-go-v2/service/dynamodb"
    "brain2-backend/internal/repository"
)

// DynamoDBTransactionProvider implements TransactionProvider using DynamoDB transactions
type DynamoDBTransactionProvider struct {
    client *dynamodb.Client
}

func NewDynamoDBTransactionProvider(client *dynamodb.Client) repository.TransactionProvider {
    return &DynamoDBTransactionProvider{
        client: client,
    }
}

func (p *DynamoDBTransactionProvider) BeginTransaction(ctx context.Context) (repository.Transaction, error) {
    return &DynamoDBTransaction{
        client: p.client,
        items:  make([]dynamodb.TransactWriteItem, 0),
    }, nil
}

// DynamoDBTransaction implements Transaction interface
type DynamoDBTransaction struct {
    client *dynamodb.Client
    items  []dynamodb.TransactWriteItem
}

func (t *DynamoDBTransaction) Commit(ctx context.Context) error {
    if len(t.items) == 0 {
        return nil
    }
    
    _, err := t.client.TransactWriteItems(ctx, &dynamodb.TransactWriteItemsInput{
        TransactItems: t.items,
    })
    return err
}

func (t *DynamoDBTransaction) Rollback(ctx context.Context) error {
    // DynamoDB transactions auto-rollback on failure
    t.items = nil
    return nil
}

func (t *DynamoDBTransaction) AddItem(item dynamodb.TransactWriteItem) {
    t.items = append(t.items, item)
}
```

```go
// backend/internal/infrastructure/events/publisher.go
package events

import (
    "context"
    "encoding/json"
    "fmt"
    
    "github.com/aws/aws-sdk-go-v2/service/eventbridge"
    "github.com/aws/aws-sdk-go-v2/service/eventbridge/types"
    "brain2-backend/internal/domain"
    "brain2-backend/internal/repository"
)

// EventBridgePublisher implements EventPublisher using AWS EventBridge
type EventBridgePublisher struct {
    client    *eventbridge.Client
    eventBus  string
    source    string
}

func NewEventBridgePublisher(client *eventbridge.Client, eventBus, source string) repository.EventPublisher {
    return &EventBridgePublisher{
        client:   client,
        eventBus: eventBus,
        source:   source,
    }
}

func (p *EventBridgePublisher) Publish(ctx context.Context, events []domain.DomainEvent) error {
    entries := make([]types.PutEventsRequestEntry, 0, len(events))
    
    for _, event := range events {
        eventData, err := json.Marshal(event)
        if err != nil {
            return fmt.Errorf("failed to marshal event: %w", err)
        }
        
        entry := types.PutEventsRequestEntry{
            EventBusName: &p.eventBus,
            Source:       &p.source,
            DetailType:   stringPtr(event.EventType()),
            Detail:       stringPtr(string(eventData)),
        }
        entries = append(entries, entry)
    }
    
    _, err := p.client.PutEvents(ctx, &eventbridge.PutEventsInput{
        Entries: entries,
    })
    
    return err
}

func stringPtr(s string) *string {
    return &s
}
```

#### Update Container Initialization

```go
// backend/internal/di/container.go - Update initializePhase3Services method

func (c *Container) initializePhase3Services() {
    log.Println("Initializing Phase 3 Application Services with CQRS pattern...")
    startTime := time.Now()

    // Initialize domain services first
    c.ConnectionAnalyzer = domainServices.NewConnectionAnalyzer(0.3, 5, 0.2)
    c.EventBus = domain.NewMockEventBus() // Keep for now, or implement real one
    
    // Initialize REAL transaction provider
    transactionProvider := transactions.NewDynamoDBTransactionProvider(c.DynamoDBClient)
    
    // Initialize REAL event publisher
    eventPublisher := events.NewEventBridgePublisher(
        c.EventBridgeClient,
        c.Config.Events.EventBusName,
        "brain2-backend",
    )
    
    // Create repository factory
    repositoryFactory := repository.NewTransactionalRepositoryFactory(
        c.NodeRepository,
        c.EdgeRepository,
        c.CategoryRepository,
    )
    
    c.UnitOfWork = repository.NewUnitOfWork(transactionProvider, eventPublisher, repositoryFactory)
    
    // ... rest of the method
}
```

---

## 🔧 Priority 2: Complete Decorator Implementation (Day 2)

### Task 2.1: Implement DecoratorChain

```go
// backend/internal/infrastructure/decorators/chain.go
package decorators

import (
    "brain2-backend/internal/config"
    "brain2-backend/internal/repository"
    "go.uber.org/zap"
)

// DecoratorChain provides a clean way to apply multiple decorators
type DecoratorChain struct {
    config           *config.Config
    logger           *zap.Logger
    cache            Cache
    metricsCollector MetricsCollector
}

// NewDecoratorChain creates a new decorator chain
func NewDecoratorChain(
    config *config.Config,
    logger *zap.Logger,
    cache Cache,
    metrics MetricsCollector,
) *DecoratorChain {
    return &DecoratorChain{
        config:           config,
        logger:           logger,
        cache:            cache,
        metricsCollector: metrics,
    }
}

// DecorateNodeRepository applies configured decorators to a NodeRepository
func (c *DecoratorChain) DecorateNodeRepository(base repository.NodeRepository) repository.NodeRepository {
    repo := base
    
    // Apply decorators in specific order (innermost to outermost)
    // Order: Retry -> Circuit Breaker -> Cache -> Metrics -> Logging
    
    if c.config.Features.EnableRetries {
        repo = NewRetryNodeRepository(repo, c.config.Infrastructure.RetryConfig)
    }
    
    if c.config.Features.EnableCircuitBreaker {
        repo = NewCircuitBreakerNodeRepository(repo, c.config.Infrastructure.CircuitBreakerConfig)
    }
    
    if c.config.Features.EnableCaching && c.cache != nil {
        cachingConfig := CachingConfig{
            DefaultTTL: int(c.config.Cache.TTL.Seconds()),
            KeyPrefix:  "node:",
        }
        repo = NewCachingNodeRepository(repo, c.cache, cachingConfig)
    }
    
    if c.config.Features.EnableMetrics && c.metricsCollector != nil {
        repo = NewMetricsNodeRepository(repo, c.metricsCollector)
    }
    
    if c.config.Features.EnableLogging && c.logger != nil {
        loggingConfig := LoggingConfig{
            LogRequests:  true,
            LogResponses: c.config.Environment != config.Production,
            LogErrors:    true,
            LogTiming:    true,
        }
        repo = NewLoggingNodeRepository(repo, c.logger, loggingConfig)
    }
    
    return repo
}

// DecorateEdgeRepository applies configured decorators to an EdgeRepository
func (c *DecoratorChain) DecorateEdgeRepository(base repository.EdgeRepository) repository.EdgeRepository {
    repo := base
    
    if c.config.Features.EnableRetries {
        repo = NewRetryEdgeRepository(repo, c.config.Infrastructure.RetryConfig)
    }
    
    if c.config.Features.EnableCaching && c.cache != nil {
        cachingConfig := CachingConfig{
            DefaultTTL: int(c.config.Cache.TTL.Seconds()),
            KeyPrefix:  "edge:",
        }
        repo = NewCachingEdgeRepository(repo, c.cache, cachingConfig)
    }
    
    if c.config.Features.EnableMetrics && c.metricsCollector != nil {
        repo = NewMetricsEdgeRepository(repo, c.metricsCollector)
    }
    
    if c.config.Features.EnableLogging && c.logger != nil {
        loggingConfig := LoggingConfig{
            LogRequests:  true,
            LogResponses: false, // Edge responses can be large
            LogErrors:    true,
            LogTiming:    true,
        }
        repo = NewLoggingEdgeRepository(repo, c.logger, loggingConfig)
    }
    
    return repo
}

// DecorateCategoryRepository applies configured decorators to a CategoryRepository
func (c *DecoratorChain) DecorateCategoryRepository(base repository.CategoryRepository) repository.CategoryRepository {
    repo := base
    
    if c.config.Features.EnableRetries {
        repo = NewRetryCategoryRepository(repo, c.config.Infrastructure.RetryConfig)
    }
    
    if c.config.Features.EnableCaching && c.cache != nil {
        cachingConfig := CachingConfig{
            DefaultTTL: int(c.config.Cache.TTL.Seconds()),
            KeyPrefix:  "category:",
        }
        repo = NewCachingCategoryRepository(repo, c.cache, cachingConfig)
    }
    
    if c.config.Features.EnableMetrics && c.metricsCollector != nil {
        repo = NewMetricsCategoryRepository(repo, c.metricsCollector)
    }
    
    return repo
}
```

### Task 2.2: Implement Missing Decorators

#### A. Caching Decorator

```go
// backend/internal/infrastructure/decorators/caching_node_repository.go
package decorators

import (
    "context"
    "encoding/json"
    "fmt"
    "time"
    
    "brain2-backend/internal/domain"
    "brain2-backend/internal/repository"
)

// CachingNodeRepository adds caching to any NodeRepository
type CachingNodeRepository struct {
    inner  repository.NodeRepository
    cache  Cache
    config CachingConfig
}

// CachingConfig configures caching behavior
type CachingConfig struct {
    DefaultTTL int    // seconds
    KeyPrefix  string
}

func NewCachingNodeRepository(
    inner repository.NodeRepository,
    cache Cache,
    config CachingConfig,
) repository.NodeRepository {
    return &CachingNodeRepository{
        inner:  inner,
        cache:  cache,
        config: config,
    }
}

func (r *CachingNodeRepository) FindNodeByID(ctx context.Context, userID, nodeID string) (*domain.Node, error) {
    // Try cache first
    cacheKey := fmt.Sprintf("%s%s:%s", r.config.KeyPrefix, userID, nodeID)
    
    if cached, found, err := r.cache.Get(ctx, cacheKey); err == nil && found {
        var node domain.Node
        if err := json.Unmarshal(cached, &node); err == nil {
            return &node, nil
        }
    }
    
    // Cache miss - fetch from inner repository
    node, err := r.inner.FindNodeByID(ctx, userID, nodeID)
    if err != nil {
        return nil, err
    }
    
    // Cache the result
    if node != nil {
        if data, err := json.Marshal(node); err == nil {
            ttl := time.Duration(r.config.DefaultTTL) * time.Second
            _ = r.cache.Set(ctx, cacheKey, data, ttl)
        }
    }
    
    return node, nil
}

func (r *CachingNodeRepository) CreateNodeAndKeywords(ctx context.Context, node *domain.Node) error {
    // Perform the create operation
    err := r.inner.CreateNodeAndKeywords(ctx, node)
    if err != nil {
        return err
    }
    
    // Invalidate related caches
    pattern := fmt.Sprintf("%s%s:*", r.config.KeyPrefix, node.UserID)
    _ = r.cache.Clear(ctx, pattern)
    
    return nil
}

func (r *CachingNodeRepository) DeleteNode(ctx context.Context, userID, nodeID string) error {
    // Perform the delete operation
    err := r.inner.DeleteNode(ctx, userID, nodeID)
    if err != nil {
        return err
    }
    
    // Invalidate specific cache entry and user's cache
    cacheKey := fmt.Sprintf("%s%s:%s", r.config.KeyPrefix, userID, nodeID)
    _ = r.cache.Delete(ctx, cacheKey)
    
    pattern := fmt.Sprintf("%s%s:*", r.config.KeyPrefix, userID)
    _ = r.cache.Clear(ctx, pattern)
    
    return nil
}

// Implement other methods with similar caching logic...
func (r *CachingNodeRepository) FindNodes(ctx context.Context, query repository.NodeQuery) ([]*domain.Node, error) {
    return r.inner.FindNodes(ctx, query)
}

func (r *CachingNodeRepository) GetNodesPage(ctx context.Context, query repository.NodeQuery, pagination repository.Pagination) (*repository.NodePage, error) {
    return r.inner.GetNodesPage(ctx, query, pagination)
}

func (r *CachingNodeRepository) GetNodeNeighborhood(ctx context.Context, userID, nodeID string, depth int) (*domain.Graph, error) {
    return r.inner.GetNodeNeighborhood(ctx, userID, nodeID, depth)
}

func (r *CachingNodeRepository) CountNodes(ctx context.Context, userID string) (int, error) {
    return r.inner.CountNodes(ctx, userID)
}

func (r *CachingNodeRepository) FindNodesWithOptions(ctx context.Context, query repository.NodeQuery, opts ...repository.QueryOption) ([]*domain.Node, error) {
    return r.inner.FindNodesWithOptions(ctx, query, opts...)
}

func (r *CachingNodeRepository) FindNodesPageWithOptions(ctx context.Context, query repository.NodeQuery, pagination repository.Pagination, opts ...repository.QueryOption) (*repository.NodePage, error) {
    return r.inner.FindNodesPageWithOptions(ctx, query, pagination, opts...)
}
```

#### B. Metrics Decorator

```go
// backend/internal/infrastructure/decorators/metrics_node_repository.go
package decorators

import (
    "context"
    "fmt"
    "time"
    
    "brain2-backend/internal/domain"
    "brain2-backend/internal/repository"
)

// MetricsNodeRepository adds metrics collection to any NodeRepository
type MetricsNodeRepository struct {
    inner     repository.NodeRepository
    collector MetricsCollector
}

func NewMetricsNodeRepository(
    inner repository.NodeRepository,
    collector MetricsCollector,
) repository.NodeRepository {
    return &MetricsNodeRepository{
        inner:     inner,
        collector: collector,
    }
}

func (r *MetricsNodeRepository) FindNodeByID(ctx context.Context, userID, nodeID string) (*domain.Node, error) {
    start := time.Now()
    
    node, err := r.inner.FindNodeByID(ctx, userID, nodeID)
    
    duration := time.Since(start)
    tags := map[string]string{
        "operation": "FindNodeByID",
        "success":   fmt.Sprintf("%t", err == nil),
    }
    
    r.collector.RecordDuration("repository.node.operation.duration", duration, tags)
    r.collector.IncrementCounter("repository.node.operation.count", tags)
    
    if err != nil {
        r.collector.IncrementCounter("repository.node.operation.errors", tags)
    }
    
    return node, err
}

func (r *MetricsNodeRepository) CreateNodeAndKeywords(ctx context.Context, node *domain.Node) error {
    start := time.Now()
    
    err := r.inner.CreateNodeAndKeywords(ctx, node)
    
    duration := time.Since(start)
    tags := map[string]string{
        "operation": "CreateNodeAndKeywords",
        "success":   fmt.Sprintf("%t", err == nil),
    }
    
    r.collector.RecordDuration("repository.node.operation.duration", duration, tags)
    r.collector.IncrementCounter("repository.node.operation.count", tags)
    
    if err != nil {
        r.collector.IncrementCounter("repository.node.operation.errors", tags)
    }
    
    return err
}

// Implement other methods with metrics collection...
func (r *MetricsNodeRepository) DeleteNode(ctx context.Context, userID, nodeID string) error {
    start := time.Now()
    err := r.inner.DeleteNode(ctx, userID, nodeID)
    r.recordMetrics("DeleteNode", time.Since(start), err)
    return err
}

func (r *MetricsNodeRepository) recordMetrics(operation string, duration time.Duration, err error) {
    tags := map[string]string{
        "operation": operation,
        "success":   fmt.Sprintf("%t", err == nil),
    }
    
    r.collector.RecordDuration("repository.node.operation.duration", duration, tags)
    r.collector.IncrementCounter("repository.node.operation.count", tags)
    
    if err != nil {
        r.collector.IncrementCounter("repository.node.operation.errors", tags)
    }
}

// ... implement remaining methods
```

---

## 🚀 Priority 3: Scalability Enhancements (Day 3)

### Task 3.1: Implement Connection Pooling

```go
// backend/internal/infrastructure/pool/dynamodb_pool.go
package pool

import (
    "context"
    "sync"
    "time"
    
    "github.com/aws/aws-sdk-go-v2/service/dynamodb"
    "go.uber.org/zap"
)

// DynamoDBConnectionPool manages DynamoDB client connections
type DynamoDBConnectionPool struct {
    clients    chan *dynamodb.Client
    factory    func() (*dynamodb.Client, error)
    config     PoolConfig
    logger     *zap.Logger
    mu         sync.RWMutex
    closed     bool
    stats      PoolStats
}

// PoolConfig configures the connection pool
type PoolConfig struct {
    MinConnections     int
    MaxConnections     int
    MaxIdleTime        time.Duration
    HealthCheckPeriod  time.Duration
    ConnectionTimeout  time.Duration
}

// PoolStats tracks pool statistics
type PoolStats struct {
    ActiveConnections   int
    IdleConnections     int
    TotalCreated        int64
    TotalDestroyed      int64
    TotalCheckouts      int64
    TotalReturns        int64
    FailedCheckouts     int64
}

// NewDynamoDBConnectionPool creates a new connection pool
func NewDynamoDBConnectionPool(
    config PoolConfig,
    factory func() (*dynamodb.Client, error),
    logger *zap.Logger,
) (*DynamoDBConnectionPool, error) {
    pool := &DynamoDBConnectionPool{
        clients: make(chan *dynamodb.Client, config.MaxConnections),
        factory: factory,
        config:  config,
        logger:  logger,
    }
    
    // Pre-warm the pool with minimum connections
    for i := 0; i < config.MinConnections; i++ {
        client, err := factory()
        if err != nil {
            return nil, fmt.Errorf("failed to create initial connection: %w", err)
        }
        pool.clients <- client
        pool.stats.TotalCreated++
    }
    
    // Start health check routine
    go pool.healthCheckLoop()
    
    return pool, nil
}

// Get retrieves a client from the pool
func (p *DynamoDBConnectionPool) Get(ctx context.Context) (*dynamodb.Client, error) {
    p.mu.RLock()
    if p.closed {
        p.mu.RUnlock()
        return nil, fmt.Errorf("pool is closed")
    }
    p.mu.RUnlock()
    
    select {
    case client := <-p.clients:
        p.stats.TotalCheckouts++
        return client, nil
        
    case <-time.After(p.config.ConnectionTimeout):
        p.stats.FailedCheckouts++
        
        // Try to create a new connection if under max
        if len(p.clients) < p.config.MaxConnections {
            client, err := p.factory()
            if err != nil {
                return nil, fmt.Errorf("failed to create new connection: %w", err)
            }
            p.stats.TotalCreated++
            p.stats.TotalCheckouts++
            return client, nil
        }
        
        return nil, fmt.Errorf("connection pool timeout")
        
    case <-ctx.Done():
        return nil, ctx.Err()
    }
}

// Return returns a client to the pool
func (p *DynamoDBConnectionPool) Return(client *dynamodb.Client) {
    p.mu.RLock()
    if p.closed {
        p.mu.RUnlock()
        return
    }
    p.mu.RUnlock()
    
    select {
    case p.clients <- client:
        p.stats.TotalReturns++
    default:
        // Pool is full, discard the connection
        p.stats.TotalDestroyed++
    }
}

// healthCheckLoop periodically checks connection health
func (p *DynamoDBConnectionPool) healthCheckLoop() {
    ticker := time.NewTicker(p.config.HealthCheckPeriod)
    defer ticker.Stop()
    
    for {
        select {
        case <-ticker.C:
            p.performHealthCheck()
        }
        
        p.mu.RLock()
        if p.closed {
            p.mu.RUnlock()
            return
        }
        p.mu.RUnlock()
    }
}

func (p *DynamoDBConnectionPool) performHealthCheck() {
    // Implement health check logic
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    
    // Try to describe limits as a health check
    clientsToCheck := make([]*dynamodb.Client, 0)
    
    // Get all idle clients
    for {
        select {
        case client := <-p.clients:
            clientsToCheck = append(clientsToCheck, client)
        default:
            // No more clients to check
            goto checkClients
        }
    }
    
checkClients:
    for _, client := range clientsToCheck {
        _, err := client.DescribeLimits(ctx, &dynamodb.DescribeLimitsInput{})
        if err != nil {
            // Unhealthy connection, replace it
            p.logger.Warn("Unhealthy connection detected, replacing", zap.Error(err))
            p.stats.TotalDestroyed++
            
            if newClient, err := p.factory(); err == nil {
                p.clients <- newClient
                p.stats.TotalCreated++
            }
        } else {
            // Healthy connection, return to pool
            p.clients <- client
        }
    }
    
    // Ensure minimum connections
    for len(p.clients) < p.config.MinConnections {
        if client, err := p.factory(); err == nil {
            p.clients <- client
            p.stats.TotalCreated++
        }
    }
}

// Close closes the pool and all connections
func (p *DynamoDBConnectionPool) Close() error {
    p.mu.Lock()
    defer p.mu.Unlock()
    
    if p.closed {
        return nil
    }
    
    p.closed = true
    close(p.clients)
    
    p.logger.Info("Connection pool closed",
        zap.Int64("total_created", p.stats.TotalCreated),
        zap.Int64("total_destroyed", p.stats.TotalDestroyed),
        zap.Int64("total_checkouts", p.stats.TotalCheckouts),
        zap.Int64("total_returns", p.stats.TotalReturns),
    )
    
    return nil
}

// GetStats returns current pool statistics
func (p *DynamoDBConnectionPool) GetStats() PoolStats {
    p.mu.RLock()
    defer p.mu.RUnlock()
    
    stats := p.stats
    stats.IdleConnections = len(p.clients)
    stats.ActiveConnections = p.config.MaxConnections - stats.IdleConnections
    
    return stats
}
```

### Task 3.2: Implement Batch Processing

```go
// backend/internal/infrastructure/batch/node_batch_processor.go
package batch

import (
    "context"
    "fmt"
    "sync"
    "time"
    
    "brain2-backend/internal/domain"
    "brain2-backend/internal/repository"
    "go.uber.org/zap"
)

// NodeBatchProcessor handles batch operations for nodes
type NodeBatchProcessor struct {
    repo            repository.NodeRepository
    batchSize       int
    flushInterval   time.Duration
    maxRetries      int
    logger          *zap.Logger
    
    mu              sync.Mutex
    pendingCreates  []*domain.Node
    pendingUpdates  []*domain.Node
    pendingDeletes  []string
    
    flushTimer      *time.Timer
    ctx             context.Context
    cancel          context.CancelFunc
    wg              sync.WaitGroup
}

// BatchProcessorConfig configures the batch processor
type BatchProcessorConfig struct {
    BatchSize     int
    FlushInterval time.Duration
    MaxRetries    int
}

// NewNodeBatchProcessor creates a new batch processor
func NewNodeBatchProcessor(
    repo repository.NodeRepository,
    config BatchProcessorConfig,
    logger *zap.Logger,
) *NodeBatchProcessor {
    ctx, cancel := context.WithCancel(context.Background())
    
    bp := &NodeBatchProcessor{
        repo:           repo,
        batchSize:      config.BatchSize,
        flushInterval:  config.FlushInterval,
        maxRetries:     config.MaxRetries,
        logger:         logger,
        pendingCreates: make([]*domain.Node, 0),
        pendingUpdates: make([]*domain.Node, 0),
        pendingDeletes: make([]string, 0),
        ctx:            ctx,
        cancel:         cancel,
    }
    
    // Start the auto-flush goroutine
    bp.wg.Add(1)
    go bp.autoFlushLoop()
    
    return bp
}

// AddCreate adds a node to the create batch
func (bp *NodeBatchProcessor) AddCreate(node *domain.Node) error {
    bp.mu.Lock()
    defer bp.mu.Unlock()
    
    bp.pendingCreates = append(bp.pendingCreates, node)
    
    // Flush if batch is full
    if len(bp.pendingCreates) >= bp.batchSize {
        go bp.flushCreates()
    } else {
        bp.resetFlushTimer()
    }
    
    return nil
}

// AddUpdate adds a node to the update batch
func (bp *NodeBatchProcessor) AddUpdate(node *domain.Node) error {
    bp.mu.Lock()
    defer bp.mu.Unlock()
    
    bp.pendingUpdates = append(bp.pendingUpdates, node)
    
    if len(bp.pendingUpdates) >= bp.batchSize {
        go bp.flushUpdates()
    } else {
        bp.resetFlushTimer()
    }
    
    return nil
}

// AddDelete adds a node ID to the delete batch
func (bp *NodeBatchProcessor) AddDelete(nodeID string) error {
    bp.mu.Lock()
    defer bp.mu.Unlock()
    
    bp.pendingDeletes = append(bp.pendingDeletes, nodeID)
    
    if len(bp.pendingDeletes) >= bp.batchSize {
        go bp.flushDeletes()
    } else {
        bp.resetFlushTimer()
    }
    
    return nil
}

// FlushAll forces all pending operations to be processed
func (bp *NodeBatchProcessor) FlushAll() error {
    bp.mu.Lock()
    defer bp.mu.Unlock()
    
    var errs []error
    
    if err := bp.flushCreates(); err != nil {
        errs = append(errs, fmt.Errorf("flush creates: %w", err))
    }
    
    if err := bp.flushUpdates(); err != nil {
        errs = append(errs, fmt.Errorf("flush updates: %w", err))
    }
    
    if err := bp.flushDeletes(); err != nil {
        errs = append(errs, fmt.Errorf("flush deletes: %w", err))
    }
    
    if len(errs) > 0 {
        return fmt.Errorf("batch flush errors: %v", errs)
    }
    
    return nil
}

// flushCreates processes pending create operations
func (bp *NodeBatchProcessor) flushCreates() error {
    if len(bp.pendingCreates) == 0 {
        return nil
    }
    
    nodes := bp.pendingCreates
    bp.pendingCreates = make([]*domain.Node, 0)
    
    // Process in chunks with retry logic
    for i := 0; i < len(nodes); i += bp.batchSize {
        end := i + bp.batchSize
        if end > len(nodes) {
            end = len(nodes)
        }
        
        chunk := nodes[i:end]
        
        if err := bp.processCreateChunk(chunk); err != nil {
            bp.logger.Error("Failed to process create batch",
                zap.Error(err),
                zap.Int("chunk_size", len(chunk)),
            )
            // Re-add failed items for retry
            bp.pendingCreates = append(bp.pendingCreates, chunk...)
            return err
        }
    }
    
    return nil
}

func (bp *NodeBatchProcessor) processCreateChunk(nodes []*domain.Node) error {
    ctx, cancel := context.WithTimeout(bp.ctx, 30*time.Second)
    defer cancel()
    
    var lastErr error
    for attempt := 0; attempt < bp.maxRetries; attempt++ {
        // In production, implement batch create in repository
        for _, node := range nodes {
            if err := bp.repo.CreateNodeAndKeywords(ctx, node); err != nil {
                lastErr = err
                break
            }
        }
        
        if lastErr == nil {
            return nil
        }
        
        // Exponential backoff
        backoff := time.Duration(1<<uint(attempt)) * time.Second
        time.Sleep(backoff)
    }
    
    return fmt.Errorf("failed after %d retries: %w", bp.maxRetries, lastErr)
}

// flushUpdates processes pending update operations
func (bp *NodeBatchProcessor) flushUpdates() error {
    // Similar implementation to flushCreates
    if len(bp.pendingUpdates) == 0 {
        return nil
    }
    
    nodes := bp.pendingUpdates
    bp.pendingUpdates = make([]*domain.Node, 0)
    
    // Process updates...
    return nil
}

// flushDeletes processes pending delete operations
func (bp *NodeBatchProcessor) flushDeletes() error {
    // Similar implementation to flushCreates
    if len(bp.pendingDeletes) == 0 {
        return nil
    }
    
    nodeIDs := bp.pendingDeletes
    bp.pendingDeletes = make([]string, 0)
    
    // Process deletes...
    return nil
}

// autoFlushLoop periodically flushes pending operations
func (bp *NodeBatchProcessor) autoFlushLoop() {
    defer bp.wg.Done()
    
    ticker := time.NewTicker(bp.flushInterval)
    defer ticker.Stop()
    
    for {
        select {
        case <-ticker.C:
            if err := bp.FlushAll(); err != nil {
                bp.logger.Error("Auto-flush failed", zap.Error(err))
            }
            
        case <-bp.ctx.Done():
            // Final flush before shutdown
            _ = bp.FlushAll()
            return
        }
    }
}

func (bp *NodeBatchProcessor) resetFlushTimer() {
    if bp.flushTimer != nil {
        bp.flushTimer.Stop()
    }
    
    bp.flushTimer = time.AfterFunc(bp.flushInterval, func() {
        _ = bp.FlushAll()
    })
}

// Close gracefully shuts down the batch processor
func (bp *NodeBatchProcessor) Close() error {
    bp.cancel()
    bp.wg.Wait()
    
    // Final flush
    return bp.FlushAll()
}
```

---

## 🔐 Priority 4: Configuration Hot Reloading (Day 4)

### Task 4.1: Implement Configuration Watcher

```go
// backend/internal/config/watcher.go
package config

import (
    "context"
    "fmt"
    "path/filepath"
    "sync"
    
    "github.com/fsnotify/fsnotify"
    "go.uber.org/zap"
)

// ConfigWatcher watches for configuration changes and reloads automatically
type ConfigWatcher struct {
    config      *Config
    configPath  string
    environment Environment
    callbacks   []func(*Config)
    watcher     *fsnotify.Watcher
    logger      *zap.Logger
    mu          sync.RWMutex
    ctx         context.Context
    cancel      context.CancelFunc
}

// NewConfigWatcher creates a new configuration watcher
func NewConfigWatcher(
    initialConfig *Config,
    configPath string,
    logger *zap.Logger,
) (*ConfigWatcher, error) {
    watcher, err := fsnotify.NewWatcher()
    if err != nil {
        return nil, fmt.Errorf("failed to create file watcher: %w", err)
    }
    
    ctx, cancel := context.WithCancel(context.Background())
    
    cw := &ConfigWatcher{
        config:      initialConfig,
        configPath:  configPath,
        environment: initialConfig.Environment,
        callbacks:   make([]func(*Config), 0),
        watcher:     watcher,
        logger:      logger,
        ctx:         ctx,
        cancel:      cancel,
    }
    
    // Watch configuration files
    files := []string{
        filepath.Join(configPath, "base.yaml"),
        filepath.Join(configPath, fmt.Sprintf("%s.yaml", cw.environment)),
    }
    
    // Only watch local.yaml in development
    if cw.environment == Development {
        files = append(files, filepath.Join(configPath, "local.yaml"))
    }
    
    for _, file := range files {
        if err := watcher.Add(file); err != nil {
            logger.Warn("Failed to watch config file", 
                zap.String("file", file),
                zap.Error(err),
            )
        }
    }
    
    // Start watching
    go cw.watchLoop()
    
    return cw, nil
}

// OnChange registers a callback for configuration changes
func (cw *ConfigWatcher) OnChange(callback func(*Config)) {
    cw.mu.Lock()
    defer cw.mu.Unlock()
    
    cw.callbacks = append(cw.callbacks, callback)
}

// GetConfig returns the current configuration
func (cw *ConfigWatcher) GetConfig() *Config {
    cw.mu.RLock()
    defer cw.mu.RUnlock()
    
    return cw.config
}

// watchLoop monitors for file changes
func (cw *ConfigWatcher) watchLoop() {
    for {
        select {
        case event, ok := <-cw.watcher.Events:
            if !ok {
                return
            }
            
            if event.Op&fsnotify.Write == fsnotify.Write {
                cw.logger.Info("Configuration file changed",
                    zap.String("file", event.Name),
                )
                
                if err := cw.reload(); err != nil {
                    cw.logger.Error("Failed to reload configuration",
                        zap.Error(err),
                    )
                }
            }
            
        case err, ok := <-cw.watcher.Errors:
            if !ok {
                return
            }
            cw.logger.Error("Watcher error", zap.Error(err))
            
        case <-cw.ctx.Done():
            return
        }
    }
}

// reload reloads the configuration from files
func (cw *ConfigWatcher) reload() error {
    cw.mu.Lock()
    defer cw.mu.Unlock()
    
    // Load new configuration
    loader := NewLoader(cw.configPath, cw.environment)
    newConfig, err := loader.Load()
    if err != nil {
        return fmt.Errorf("failed to load configuration: %w", err)
    }
    
    // Validate new configuration
    if err := newConfig.Validate(); err != nil {
        return fmt.Errorf("invalid configuration: %w", err)
    }
    
    // Check if configuration actually changed
    if cw.isConfigEqual(cw.config, newConfig) {
        return nil
    }
    
    oldConfig := cw.config
    cw.config = newConfig
    
    // Notify callbacks
    for _, callback := range cw.callbacks {
        go func(cb func(*Config)) {
            defer func() {
                if r := recover(); r != nil {
                    cw.logger.Error("Callback panic",
                        zap.Any("panic", r),
                    )
                }
            }()
            cb(newConfig)
        }(callback)
    }
    
    cw.logger.Info("Configuration reloaded successfully",
        zap.String("environment", string(newConfig.Environment)),
        zap.Any("changes", cw.getChanges(oldConfig, newConfig)),
    )
    
    return nil
}

// isConfigEqual checks if two configurations are equal
func (cw *ConfigWatcher) isConfigEqual(c1, c2 *Config) bool {
    // Implement deep equality check
    // For simplicity, compare key fields
    return c1.Server.Port == c2.Server.Port &&
           c1.Database.TableName == c2.Database.TableName &&
           c1.Features.EnableCaching == c2.Features.EnableCaching
}

// getChanges returns the differences between two configurations
func (cw *ConfigWatcher) getChanges(oldConfig, newConfig *Config) map[string]interface{} {
    changes := make(map[string]interface{})
    
    if oldConfig.Server.Port != newConfig.Server.Port {
        changes["server.port"] = map[string]interface{}{
            "old": oldConfig.Server.Port,
            "new": newConfig.Server.Port,
        }
    }
    
    if oldConfig.Features.EnableCaching != newConfig.Features.EnableCaching {
        changes["features.enable_caching"] = map[string]interface{}{
            "old": oldConfig.Features.EnableCaching,
            "new": newConfig.Features.EnableCaching,
        }
    }
    
    // Add more change detection as needed
    
    return changes
}

// Close stops watching for configuration changes
func (cw *ConfigWatcher) Close() error {
    cw.cancel()
    return cw.watcher.Close()
}
```

---

## 📝 Priority 5: Complete Integration (Day 5)

### Task 5.1: Wire Everything Together

```go
// backend/internal/di/providers.go - Update provider functions

func provideCache(cfg *config.Config, logger *zap.Logger) decorators.Cache {
    switch cfg.Cache.Provider {
    case "redis":
        // Implement Redis cache
        return NewRedisCache(cfg.Cache.Redis, logger)
    case "memory":
        // Implement in-memory cache
        return NewMemoryCache(cfg.Cache.MaxItems, cfg.Cache.TTL)
    default:
        return NewMemoryCache(1000, 5*time.Minute)
    }
}

func provideMetricsCollector(cfg *config.Config) decorators.MetricsCollector {
    switch cfg.Metrics.Provider {
    case "prometheus":
        return NewPrometheusCollector(cfg.Metrics.Prometheus)
    case "cloudwatch":
        return NewCloudWatchCollector(cfg.Metrics.CloudWatch)
    default:
        return NewNoOpMetricsCollector()
    }
}

func provideConnectionPool(cfg *config.Config, awsCfg aws.Config) (*pool.DynamoDBConnectionPool, error) {
    poolConfig := pool.PoolConfig{
        MinConnections:    cfg.Database.ConnectionPool / 2,
        MaxConnections:    cfg.Database.ConnectionPool,
        MaxIdleTime:       5 * time.Minute,
        HealthCheckPeriod: 30 * time.Second,
        ConnectionTimeout: cfg.Database.Timeout,
    }
    
    factory := func() (*dynamodb.Client, error) {
        return dynamodb.NewFromConfig(awsCfg), nil
    }
    
    logger := zap.L().Named("connection_pool")
    
    return pool.NewDynamoDBConnectionPool(poolConfig, factory, logger)
}

func provideBatchProcessor(
    repo repository.NodeRepository,
    cfg *config.Config,
    logger *zap.Logger,
) *batch.NodeBatchProcessor {
    batchConfig := batch.BatchProcessorConfig{
        BatchSize:     25, // DynamoDB batch write limit
        FlushInterval: 5 * time.Second,
        MaxRetries:    cfg.Database.MaxRetries,
    }
    
    return batch.NewNodeBatchProcessor(repo, batchConfig, logger)
}
```

### Task 5.2: Update Container Initialization

```go
// backend/internal/di/container.go - Update initialization

func (c *Container) initialize() error {
    // 1. Load configuration with watcher
    cfg := config.LoadConfig()
    c.Config = &cfg
    
    // Setup configuration hot reloading in development
    if cfg.Environment == config.Development {
        watcher, err := config.NewConfigWatcher(&cfg, "config", c.Logger)
        if err == nil {
            watcher.OnChange(func(newConfig *config.Config) {
                c.handleConfigChange(newConfig)
            })
            c.ConfigWatcher = watcher
        }
    }
    
    // 2. Initialize connection pool
    pool, err := provideConnectionPool(&cfg, c.AWSConfig)
    if err != nil {
        return fmt.Errorf("failed to create connection pool: %w", err)
    }
    c.ConnectionPool = pool
    
    // 3. Initialize cache
    c.Cache = provideCache(&cfg, c.Logger)
    
    // 4. Initialize metrics collector
    c.MetricsCollector = provideMetricsCollector(&cfg)
    
    // 5. Initialize batch processor
    c.BatchProcessor = provideBatchProcessor(c.NodeRepository, &cfg, c.Logger)
    
    // Continue with rest of initialization...
    
    return nil
}

func (c *Container) handleConfigChange(newConfig *config.Config) {
    c.Logger.Info("Handling configuration change")
    
    // Update configuration
    c.Config = newConfig
    
    // Recreate components that depend on configuration
    if newConfig.Features.EnableCaching != c.Config.Features.EnableCaching {
        c.Cache = provideCache(newConfig, c.Logger)
        c.Logger.Info("Cache reconfigured")
    }
    
    // Notify services of configuration change
    // Services should implement ConfigChangeHandler interface
}
```

---

## ✅ Verification Checklist

After implementing these changes, verify:

### Code Quality
- [ ] No dead code remains (check with `deadcode` tool)
- [ ] All TODOs are resolved or documented with tickets
- [ ] No mock implementations in production code
- [ ] All decorators properly implemented

### Functionality
- [ ] Decorators apply correctly based on configuration
- [ ] Connection pooling works under load
- [ ] Batch processing handles failures gracefully
- [ ] Configuration hot reloading works in development

### Performance
- [ ] Response times improved with caching
- [ ] Database connections efficiently pooled
- [ ] Batch operations reduce API calls
- [ ] Metrics show improved throughput

### Testing
```bash
# Run all tests
go test ./...

# Run with race detection
go test -race ./...

# Check test coverage
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out

# Run benchmarks
go test -bench=. ./...
```

---

## 📊 Expected Outcome

After completing these tasks:

1. **Clean Architecture**: 10/10
   - Perfect layer separation
   - No circular dependencies
   - Clear boundaries

2. **Code Efficiency**: 10/10
   - Zero dead code
   - Optimal abstractions
   - Clean implementations

3. **Configuration Management**: 10/10
   - Hot reloading in dev
   - Multi-source support
   - Comprehensive validation

4. **Dependency Injection**: 10/10
   - Complete decorator chain
   - Clean Wire setup
   - Factory pattern perfection

5. **Scalability**: 9/10
   - Connection pooling
   - Batch processing
   - Caching layer
   - Ready for high load

Your backend will serve as a **reference implementation** for Go best practices and clean architecture principles.