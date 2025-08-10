# Brain2 Application Improvement Implementation Plan

## Overview
This document provides a comprehensive implementation plan for improving the Brain2 application's architecture, performance, and reliability. Each section contains specific implementation details, code examples, and file paths for Claude Code to execute the improvements.

## Priority Order for Implementation
1. Race Condition Prevention (Critical)
2. Enhanced Middleware Implementation (High)
3. Idempotency Keys (High)
4. Transaction Rollback & Compensation (Medium)
5. Pagination & Query Optimization (Medium)
6. Dependency Injection Enhancements (Medium)
7. Metrics Collection (Low)

---

## 1. Race Condition Prevention

### 1.1 Implement Optimistic Locking

#### Files to Create/Modify:
- `backend/internal/domain/node.go`
- `backend/infrastructure/dynamodb/ddb.go`
- `backend/internal/repository/errors.go`

#### Implementation Steps:

1. **Add Version Field to Domain Model**
```go
// File: backend/internal/domain/node.go
// Add to Node struct:
type Node struct {
    // ... existing fields
    Version   int       `json:"version"`
    UpdatedAt time.Time `json:"updatedAt"`
}
```

2. **Create Optimistic Lock Error Type**
```go
// File: backend/internal/repository/errors.go
package repository

import "fmt"

type OptimisticLockError struct {
    ResourceID      string
    ExpectedVersion int
    ActualVersion   int
}

func (e *OptimisticLockError) Error() string {
    return fmt.Sprintf("optimistic lock failed for resource %s: expected version %d, got %d", 
        e.ResourceID, e.ExpectedVersion, e.ActualVersion)
}

func IsOptimisticLockError(err error) bool {
    _, ok := err.(*OptimisticLockError)
    return ok
}
```

3. **Update Repository Methods with Version Checking**
```go
// File: backend/infrastructure/dynamodb/ddb.go
// Modify UpdateNodeAndEdges method to include version checking:

func (r *ddbRepository) UpdateNodeAndEdges(ctx context.Context, node domain.Node, relatedNodeIDs []string) error {
    pk := fmt.Sprintf("USER#%s#NODE#%s", node.UserID, node.ID)
    
    // Use conditional update with version check
    _, err := r.dbClient.UpdateItem(ctx, &dynamodb.UpdateItemInput{
        TableName: aws.String(r.config.TableName),
        Key: map[string]types.AttributeValue{
            "PK": &types.AttributeValueMemberS{Value: pk},
            "SK": &types.AttributeValueMemberS{Value: "METADATA#v0"},
        },
        UpdateExpression: aws.String("SET Content = :c, Keywords = :k, Tags = :tg, #v = #v + :inc, UpdatedAt = :ua"),
        ConditionExpression: aws.String("#v = :expected_version"),
        ExpressionAttributeNames: map[string]string{
            "#v": "Version",
        },
        ExpressionAttributeValues: map[string]types.AttributeValue{
            ":c":                &types.AttributeValueMemberS{Value: node.Content},
            ":k":                &types.AttributeValueMemberL{Value: toAttributeValueList(node.Keywords)},
            ":tg":               &types.AttributeValueMemberL{Value: toAttributeValueList(node.Tags)},
            ":expected_version": &types.AttributeValueMemberN{Value: fmt.Sprintf("%d", node.Version)},
            ":inc":             &types.AttributeValueMemberN{Value: "1"},
            ":ua":              &types.AttributeValueMemberS{Value: time.Now().Format(time.RFC3339)},
        },
        ReturnValues: types.ReturnValueAllNew,
    })
    
    if err != nil {
        var ccf *types.ConditionalCheckFailedException
        if errors.As(err, &ccf) {
            return &OptimisticLockError{
                ResourceID:      node.ID,
                ExpectedVersion: node.Version,
                ActualVersion:   node.Version + 1,
            }
        }
        return err
    }
    
    // Continue with edge updates...
}
```

4. **Add Retry Logic in Service Layer**
```go
// File: backend/internal/service/memory/optimistic_retry.go
package memory

import (
    "context"
    "time"
    "brain2-backend/internal/repository"
)

const (
    maxRetries = 3
    retryDelay = 100 * time.Millisecond
)

func (s *service) UpdateNodeWithRetry(ctx context.Context, userID, nodeID string, updateFn func(*domain.Node) error) error {
    for attempt := 0; attempt < maxRetries; attempt++ {
        // Fetch latest version
        node, err := s.repo.FindNodeByID(ctx, userID, nodeID)
        if err != nil {
            return err
        }
        
        // Apply updates
        if err := updateFn(node); err != nil {
            return err
        }
        
        // Try to save
        err = s.repo.UpdateNode(ctx, *node)
        if err == nil {
            return nil
        }
        
        // Check if it's an optimistic lock error
        if repository.IsOptimisticLockError(err) && attempt < maxRetries-1 {
            time.Sleep(retryDelay * time.Duration(attempt+1))
            continue
        }
        
        return err
    }
    return fmt.Errorf("max retries exceeded for node update")
}
```

---

## 2. Enhanced Middleware Implementation

### 2.1 Create Middleware Package

#### Files to Create:
- `backend/internal/middleware/request_id.go`
- `backend/internal/middleware/rate_limit.go`
- `backend/internal/middleware/circuit_breaker.go`
- `backend/internal/middleware/metrics.go`
- `backend/internal/middleware/recovery.go`
- `backend/internal/middleware/timeout.go`

#### Implementation:

1. **Request ID Middleware**
```go
// File: backend/internal/middleware/request_id.go
package middleware

import (
    "context"
    "net/http"
    "github.com/google/uuid"
)

type contextKey string

const RequestIDKey contextKey = "requestID"

func RequestID(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        requestID := r.Header.Get("X-Request-ID")
        if requestID == "" {
            requestID = uuid.New().String()
        }
        
        ctx := context.WithValue(r.Context(), RequestIDKey, requestID)
        w.Header().Set("X-Request-ID", requestID)
        
        next.ServeHTTP(w, r.WithContext(ctx))
    })
}

func GetRequestID(ctx context.Context) string {
    if id, ok := ctx.Value(RequestIDKey).(string); ok {
        return id
    }
    return ""
}
```

2. **Rate Limiting Middleware**
```go
// File: backend/internal/middleware/rate_limit.go
package middleware

import (
    "net/http"
    "sync"
    "time"
    "golang.org/x/time/rate"
)

type RateLimiter struct {
    visitors map[string]*visitor
    mu       sync.RWMutex
    rate     rate.Limit
    burst    int
}

type visitor struct {
    limiter  *rate.Limiter
    lastSeen time.Time
}

func NewRateLimiter(r rate.Limit, b int) *RateLimiter {
    rl := &RateLimiter{
        visitors: make(map[string]*visitor),
        rate:     r,
        burst:    b,
    }
    
    // Clean up old visitors
    go rl.cleanupVisitors()
    
    return rl
}

func (rl *RateLimiter) Middleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        userID := r.Header.Get("X-User-ID")
        if userID == "" {
            userID = r.RemoteAddr
        }
        
        limiter := rl.getLimiter(userID)
        if !limiter.Allow() {
            http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)
            return
        }
        
        next.ServeHTTP(w, r)
    })
}

func (rl *RateLimiter) getLimiter(userID string) *rate.Limiter {
    rl.mu.Lock()
    defer rl.mu.Unlock()
    
    v, exists := rl.visitors[userID]
    if !exists {
        limiter := rate.NewLimiter(rl.rate, rl.burst)
        rl.visitors[userID] = &visitor{limiter, time.Now()}
        return limiter
    }
    
    v.lastSeen = time.Now()
    return v.limiter
}

func (rl *RateLimiter) cleanupVisitors() {
    for {
        time.Sleep(time.Minute)
        
        rl.mu.Lock()
        for userID, v := range rl.visitors {
            if time.Since(v.lastSeen) > 3*time.Minute {
                delete(rl.visitors, userID)
            }
        }
        rl.mu.Unlock()
    }
}
```

3. **Circuit Breaker Middleware**
```go
// File: backend/internal/middleware/circuit_breaker.go
package middleware

import (
    "net/http"
    "github.com/sony/gobreaker"
    "time"
)

func CircuitBreaker(name string) func(http.Handler) http.Handler {
    cb := gobreaker.NewCircuitBreaker(gobreaker.Settings{
        Name:        name,
        MaxRequests: 3,
        Interval:    10 * time.Second,
        Timeout:     30 * time.Second,
        ReadyToTrip: func(counts gobreaker.Counts) bool {
            failureRatio := float64(counts.TotalFailures) / float64(counts.Requests)
            return counts.Requests >= 3 && failureRatio >= 0.6
        },
    })
    
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            _, err := cb.Execute(func() (interface{}, error) {
                next.ServeHTTP(w, r)
                return nil, nil
            })
            
            if err != nil {
                http.Error(w, "Service temporarily unavailable", http.StatusServiceUnavailable)
            }
        })
    }
}
```

4. **Update Router Setup**
```go
// File: backend/internal/di/wire.go
// Modify setupRouter function:

func setupRouter(memoryHandler *handlers.MemoryHandler, categoryHandler *handlers.CategoryHandler) *chi.Mux {
    r := chi.NewRouter()
    
    // Global middleware
    r.Use(middleware.RequestID)
    r.Use(middleware.RealIP)
    r.Use(middleware.Logger)
    r.Use(middleware.Recoverer)
    r.Use(middleware.Timeout(30 * time.Second))
    
    // Rate limiting - 100 requests per second with burst of 200
    rateLimiter := middleware.NewRateLimiter(100, 200)
    r.Use(rateLimiter.Middleware)
    
    // Circuit breaker for external services
    r.Use(middleware.CircuitBreaker("api"))
    
    // Metrics collection
    r.Use(middleware.PrometheusMiddleware)
    
    // ... rest of router setup
}
```

---

## 3. Idempotency Keys Implementation

### 3.1 Create Idempotency System

#### Files to Create:
- `backend/internal/repository/idempotency.go`
- `backend/infrastructure/dynamodb/idempotency.go`

#### Implementation:

1. **Idempotency Key Structure**
```go
// File: backend/internal/repository/idempotency.go
package repository

import (
    "context"
    "crypto/sha256"
    "encoding/hex"
    "fmt"
    "time"
)

type IdempotencyKey struct {
    Key       string    `json:"key"`
    UserID    string    `json:"userId"`
    Operation string    `json:"operation"`
    Result    string    `json:"result"`
    CreatedAt time.Time `json:"createdAt"`
    ExpiresAt time.Time `json:"expiresAt"`
}

type IdempotencyStore interface {
    Store(ctx context.Context, key IdempotencyKey) error
    Get(ctx context.Context, userID, operation, key string) (*IdempotencyKey, error)
    Delete(ctx context.Context, userID, operation, key string) error
}

func GenerateIdempotencyKey(userID, operation string, payload interface{}) string {
    h := sha256.New()
    h.Write([]byte(fmt.Sprintf("%s:%s:%v", userID, operation, payload)))
    return hex.EncodeToString(h.Sum(nil))
}
```

2. **DynamoDB Implementation**
```go
// File: backend/infrastructure/dynamodb/idempotency.go
package dynamodb

import (
    "context"
    "time"
    "brain2-backend/internal/repository"
)

func (r *ddbRepository) StoreIdempotencyKey(ctx context.Context, key repository.IdempotencyKey) error {
    item := map[string]types.AttributeValue{
        "PK": &types.AttributeValueMemberS{
            Value: fmt.Sprintf("IDEMPOTENCY#%s#%s", key.UserID, key.Operation),
        },
        "SK": &types.AttributeValueMemberS{
            Value: key.Key,
        },
        "Result": &types.AttributeValueMemberS{
            Value: key.Result,
        },
        "TTL": &types.AttributeValueMemberN{
            Value: fmt.Sprintf("%d", key.ExpiresAt.Unix()),
        },
    }
    
    _, err := r.dbClient.PutItem(ctx, &dynamodb.PutItemInput{
        TableName:           aws.String(r.config.TableName),
        Item:               item,
        ConditionExpression: aws.String("attribute_not_exists(PK)"),
    })
    
    return err
}
```

3. **Service Layer Integration**
```go
// File: backend/internal/service/memory/idempotent_operations.go
package memory

import (
    "context"
    "encoding/json"
)

func (s *service) CreateNodeIdempotent(ctx context.Context, userID, idempotencyKey string, content string, tags []string) (*domain.Node, error) {
    // Check if operation was already performed
    existingKey, err := s.repo.GetIdempotencyKey(ctx, userID, "CREATE_NODE", idempotencyKey)
    if err == nil && existingKey != nil {
        // Operation already performed, return cached result
        var node domain.Node
        json.Unmarshal([]byte(existingKey.Result), &node)
        return &node, nil
    }
    
    // Perform operation
    node, edges, err := s.CreateNodeWithEdges(ctx, userID, content, tags)
    if err != nil {
        return nil, err
    }
    
    // Store idempotency key with result
    result, _ := json.Marshal(node)
    key := repository.IdempotencyKey{
        Key:       idempotencyKey,
        UserID:    userID,
        Operation: "CREATE_NODE",
        Result:    string(result),
        CreatedAt: time.Now(),
        ExpiresAt: time.Now().Add(24 * time.Hour),
    }
    
    s.repo.StoreIdempotencyKey(ctx, key)
    
    return node, nil
}
```

4. **Handler Integration**
```go
// File: backend/internal/handlers/memory.go
// Modify CreateNode handler:

func (h *MemoryHandler) CreateNode(w http.ResponseWriter, r *http.Request) {
    userID, _ := getUserID(r)
    
    // Get idempotency key from header
    idempotencyKey := r.Header.Get("Idempotency-Key")
    if idempotencyKey == "" {
        idempotencyKey = repository.GenerateIdempotencyKey(userID, "CREATE_NODE", req)
    }
    
    // Use idempotent service method
    node, err := h.memoryService.CreateNodeIdempotent(r.Context(), userID, idempotencyKey, req.Content, tags)
    // ... rest of handler
}
```

---

## 4. Transaction Rollback & Compensation

### 4.1 Implement Saga Pattern

#### Files to Create:
- `backend/internal/saga/saga.go`
- `backend/internal/saga/coordinator.go`
- `backend/internal/service/memory/saga_operations.go`

#### Implementation:

1. **Saga Framework**
```go
// File: backend/internal/saga/saga.go
package saga

import (
    "context"
    "fmt"
)

type Step struct {
    Name        string
    Execute     func(context.Context) (interface{}, error)
    Compensate  func(context.Context, interface{}) error
}

type Saga struct {
    Name  string
    Steps []Step
}

type Coordinator struct {
    sagas map[string]*Saga
}

func NewCoordinator() *Coordinator {
    return &Coordinator{
        sagas: make(map[string]*Saga),
    }
}

func (c *Coordinator) Register(saga *Saga) {
    c.sagas[saga.Name] = saga
}

func (c *Coordinator) Execute(ctx context.Context, sagaName string) error {
    saga, exists := c.sagas[sagaName]
    if !exists {
        return fmt.Errorf("saga %s not found", sagaName)
    }
    
    completedSteps := make([]interface{}, 0)
    
    for i, step := range saga.Steps {
        result, err := step.Execute(ctx)
        if err != nil {
            // Rollback in reverse order
            for j := i - 1; j >= 0; j-- {
                if saga.Steps[j].Compensate != nil {
                    compensateErr := saga.Steps[j].Compensate(ctx, completedSteps[j])
                    if compensateErr != nil {
                        // Log compensation failure
                        fmt.Printf("Compensation failed for step %s: %v\n", saga.Steps[j].Name, compensateErr)
                    }
                }
            }
            return fmt.Errorf("saga %s failed at step %s: %w", sagaName, step.Name, err)
        }
        completedSteps = append(completedSteps, result)
    }
    
    return nil
}
```

2. **Complex Operation Saga**
```go
// File: backend/internal/service/memory/saga_operations.go
package memory

import (
    "context"
    "brain2-backend/internal/saga"
)

func (s *service) CreateNodeWithSaga(ctx context.Context, userID, content string, tags []string, categoryIDs []string) (*domain.Node, error) {
    var node *domain.Node
    var edges []domain.Edge
    
    nodeSaga := &saga.Saga{
        Name: "CreateNodeWithCategories",
        Steps: []saga.Step{
            {
                Name: "CreateNode",
                Execute: func(ctx context.Context) (interface{}, error) {
                    n, e, err := s.CreateNodeWithEdges(ctx, userID, content, tags)
                    if err != nil {
                        return nil, err
                    }
                    node = n
                    edges = e
                    return n, nil
                },
                Compensate: func(ctx context.Context, data interface{}) error {
                    if n, ok := data.(*domain.Node); ok {
                        return s.repo.DeleteNode(ctx, userID, n.ID)
                    }
                    return nil
                },
            },
            {
                Name: "AssignCategories",
                Execute: func(ctx context.Context) (interface{}, error) {
                    for _, categoryID := range categoryIDs {
                        if err := s.categoryService.AssignNodeToCategory(ctx, userID, node.ID, categoryID); err != nil {
                            return nil, err
                        }
                    }
                    return categoryIDs, nil
                },
                Compensate: func(ctx context.Context, data interface{}) error {
                    if ids, ok := data.([]string); ok {
                        for _, categoryID := range ids {
                            s.categoryService.RemoveNodeFromCategory(ctx, userID, node.ID, categoryID)
                        }
                    }
                    return nil
                },
            },
            {
                Name: "PublishEvent",
                Execute: func(ctx context.Context) (interface{}, error) {
                    return nil, s.publishNodeCreatedEvent(ctx, node, edges)
                },
                Compensate: func(ctx context.Context, data interface{}) error {
                    // Publish compensation event
                    return s.publishNodeDeletedEvent(ctx, node.ID)
                },
            },
        },
    }
    
    coordinator := saga.NewCoordinator()
    coordinator.Register(nodeSaga)
    
    if err := coordinator.Execute(ctx, "CreateNodeWithCategories"); err != nil {
        return nil, err
    }
    
    return node, nil
}
```

---

## 5. Pagination & Query Optimization

### 5.1 Implement Efficient Pagination

#### Files to Create/Modify:
- `backend/internal/repository/pagination.go`
- `backend/infrastructure/dynamodb/pagination.go`

#### Implementation:

1. **Pagination Types**
```go
// File: backend/internal/repository/pagination.go
package repository

import (
    "encoding/base64"
    "encoding/json"
)

type PageRequest struct {
    Limit      int    `json:"limit"`
    NextToken  string `json:"nextToken,omitempty"`
    SortBy     string `json:"sortBy,omitempty"`
    SortOrder  string `json:"sortOrder,omitempty"`
}

type PageResponse struct {
    Items     interface{} `json:"items"`
    NextToken string      `json:"nextToken,omitempty"`
    HasMore   bool        `json:"hasMore"`
    Total     int         `json:"total,omitempty"`
}

type LastEvaluatedKey struct {
    PK string `json:"pk"`
    SK string `json:"sk"`
}

func EncodeNextToken(key LastEvaluatedKey) string {
    data, _ := json.Marshal(key)
    return base64.StdEncoding.EncodeToString(data)
}

func DecodeNextToken(token string) (*LastEvaluatedKey, error) {
    data, err := base64.StdEncoding.DecodeString(token)
    if err != nil {
        return nil, err
    }
    
    var key LastEvaluatedKey
    err = json.Unmarshal(data, &key)
    return &key, err
}
```

2. **Optimized Edge Query with GSI**
```go
// File: backend/infrastructure/dynamodb/pagination.go
package dynamodb

// Add new GSI for efficient edge queries
func (r *ddbRepository) CreateEdgeIndex() {
    // GSI2: For efficient edge queries
    // GSI2PK: USER#<userId>#EDGE
    // GSI2SK: NODE#<sourceId>#TARGET#<targetId>
}

func (r *ddbRepository) GetNodesPaginated(ctx context.Context, userID string, req repository.PageRequest) (*repository.PageResponse, error) {
    limit := req.Limit
    if limit == 0 || limit > 100 {
        limit = 20
    }
    
    input := &dynamodb.QueryInput{
        TableName:              aws.String(r.config.TableName),
        KeyConditionExpression: aws.String("PK = :pk AND begins_with(SK, :sk)"),
        ExpressionAttributeValues: map[string]types.AttributeValue{
            ":pk": &types.AttributeValueMemberS{Value: fmt.Sprintf("USER#%s", userID)},
            ":sk": &types.AttributeValueMemberS{Value: "NODE#"},
        },
        Limit: aws.Int32(int32(limit)),
    }
    
    // Add pagination token if provided
    if req.NextToken != "" {
        lastKey, err := repository.DecodeNextToken(req.NextToken)
        if err == nil {
            input.ExclusiveStartKey = map[string]types.AttributeValue{
                "PK": &types.AttributeValueMemberS{Value: lastKey.PK},
                "SK": &types.AttributeValueMemberS{Value: lastKey.SK},
            }
        }
    }
    
    result, err := r.dbClient.Query(ctx, input)
    if err != nil {
        return nil, err
    }
    
    // Process items
    nodes := make([]domain.Node, 0, len(result.Items))
    for _, item := range result.Items {
        var node ddbNode
        if err := attributevalue.UnmarshalMap(item, &node); err == nil {
            nodes = append(nodes, node.ToDomain())
        }
    }
    
    // Create response
    response := &repository.PageResponse{
        Items:   nodes,
        HasMore: result.LastEvaluatedKey != nil,
    }
    
    // Encode next token if there are more results
    if result.LastEvaluatedKey != nil {
        pk := result.LastEvaluatedKey["PK"].(*types.AttributeValueMemberS).Value
        sk := result.LastEvaluatedKey["SK"].(*types.AttributeValueMemberS).Value
        response.NextToken = repository.EncodeNextToken(repository.LastEvaluatedKey{
            PK: pk,
            SK: sk,
        })
    }
    
    return response, nil
}
```

3. **Parallel Query for Graph Data**
```go
// File: backend/infrastructure/dynamodb/graph_optimization.go
package dynamodb

import (
    "sync"
    "golang.org/x/sync/errgroup"
)

func (r *ddbRepository) GetGraphDataOptimized(ctx context.Context, userID string) (*domain.Graph, error) {
    g, ctx := errgroup.WithContext(ctx)
    
    var nodes []domain.Node
    var edges []domain.Edge
    var nodesErr, edgesErr error
    
    // Fetch nodes in parallel
    g.Go(func() error {
        nodes, nodesErr = r.fetchAllNodes(ctx, userID)
        return nodesErr
    })
    
    // Fetch edges in parallel
    g.Go(func() error {
        edges, edgesErr = r.fetchAllEdges(ctx, userID)
        return edgesErr
    })
    
    if err := g.Wait(); err != nil {
        return nil, err
    }
    
    return &domain.Graph{
        Nodes: nodes,
        Edges: edges,
    }, nil
}

func (r *ddbRepository) fetchAllNodes(ctx context.Context, userID string) ([]domain.Node, error) {
    var nodes []domain.Node
    var lastEvaluatedKey map[string]types.AttributeValue
    
    for {
        input := &dynamodb.QueryInput{
            TableName:              aws.String(r.config.TableName),
            KeyConditionExpression: aws.String("PK = :pk AND begins_with(SK, :sk)"),
            ExpressionAttributeValues: map[string]types.AttributeValue{
                ":pk": &types.AttributeValueMemberS{Value: fmt.Sprintf("USER#%s", userID)},
                ":sk": &types.AttributeValueMemberS{Value: "NODE#METADATA"},
            },
            ExclusiveStartKey: lastEvaluatedKey,
        }
        
        result, err := r.dbClient.Query(ctx, input)
        if err != nil {
            return nil, err
        }
        
        for _, item := range result.Items {
            var node ddbNode
            if err := attributevalue.UnmarshalMap(item, &node); err == nil {
                nodes = append(nodes, node.ToDomain())
            }
        }
        
        lastEvaluatedKey = result.LastEvaluatedKey
        if lastEvaluatedKey == nil {
            break
        }
    }
    
    return nodes, nil
}
```

---

## 6. Dependency Injection Enhancements

### 6.1 Interface Segregation

#### Files to Create:
- `backend/internal/repository/interfaces.go`
- `backend/internal/service/interfaces.go`

#### Implementation:

1. **Segregated Repository Interfaces**
```go
// File: backend/internal/repository/interfaces.go
package repository

import (
    "context"
    "brain2-backend/internal/domain"
)

// NodeRepository handles node-specific operations
type NodeRepository interface {
    CreateNode(ctx context.Context, node domain.Node) error
    UpdateNode(ctx context.Context, node domain.Node) error
    DeleteNode(ctx context.Context, userID, nodeID string) error
    FindNodeByID(ctx context.Context, userID, nodeID string) (*domain.Node, error)
    FindNodes(ctx context.Context, query NodeQuery) ([]domain.Node, error)
    GetNodesPage(ctx context.Context, query NodeQuery, pagination PageRequest) (*PageResponse, error)
}

// EdgeRepository handles edge-specific operations
type EdgeRepository interface {
    CreateEdges(ctx context.Context, userID, sourceNodeID string, relatedNodeIDs []string) error
    DeleteEdges(ctx context.Context, userID, nodeID string) error
    FindEdges(ctx context.Context, query EdgeQuery) ([]domain.Edge, error)
    GetEdgesPage(ctx context.Context, query EdgeQuery, pagination PageRequest) (*PageResponse, error)
}

// KeywordRepository handles keyword indexing
type KeywordRepository interface {
    IndexKeywords(ctx context.Context, userID, nodeID string, keywords []string) error
    FindNodesByKeywords(ctx context.Context, userID string, keywords []string) ([]domain.Node, error)
    DeleteKeywords(ctx context.Context, userID, nodeID string) error
}

// TransactionalRepository handles transactional operations
type TransactionalRepository interface {
    CreateNodeWithEdges(ctx context.Context, node domain.Node, relatedNodeIDs []string) error
    UpdateNodeAndEdges(ctx context.Context, node domain.Node, relatedNodeIDs []string) error
    ExecuteTransaction(ctx context.Context, operations []TransactionOperation) error
}

// CategoryRepository handles category operations
type CategoryRepository interface {
    CreateCategory(ctx context.Context, category domain.Category) error
    UpdateCategory(ctx context.Context, category domain.Category) error
    DeleteCategory(ctx context.Context, userID, categoryID string) error
    FindCategoryByID(ctx context.Context, userID, categoryID string) (*domain.Category, error)
    FindCategories(ctx context.Context, query CategoryQuery) ([]domain.Category, error)
}

// Repository aggregates all repository interfaces
type Repository interface {
    NodeRepository
    EdgeRepository
    KeywordRepository
    TransactionalRepository
    CategoryRepository
    IdempotencyStore
}
```

2. **Update Container with Segregated Interfaces**
```go
// File: backend/internal/di/container.go
// Modify Container struct:

type Container struct {
    // Configuration
    Config *config.Config
    
    // AWS Clients
    DynamoDBClient    *awsDynamodb.Client
    EventBridgeClient *awsEventbridge.Client
    
    // Repository Layer - Segregated interfaces
    NodeRepo         repository.NodeRepository
    EdgeRepo         repository.EdgeRepository
    KeywordRepo      repository.KeywordRepository
    TransactionRepo  repository.TransactionalRepository
    CategoryRepo     repository.CategoryRepository
    IdempotencyStore repository.IdempotencyStore
    
    // Aggregated repository for backward compatibility
    Repository repository.Repository
    
    // Service Layer
    MemoryService   memoryService.Service
    CategoryService categoryService.Service
    
    // Handler Layer
    MemoryHandler   *handlers.MemoryHandler
    CategoryHandler *handlers.CategoryHandler
    
    // HTTP Router
    Router *chi.Mux
    
    // Metrics
    Metrics *Metrics
    
    // Lifecycle management
    shutdownFunctions []func() error
}
```

---

## 7. Metrics Collection

### 7.1 Prometheus Metrics Implementation

#### Files to Create:
- `backend/internal/metrics/metrics.go`
- `backend/internal/metrics/collector.go`
- `backend/internal/middleware/metrics.go`

#### Implementation:

1. **Metrics Definition**
```go
// File: backend/internal/metrics/metrics.go
package metrics

import (
    "github.com/prometheus/client_golang/prometheus"
    "github.com/prometheus/client_golang/prometheus/promauto"
)

type Metrics struct {
    // HTTP metrics
    HTTPRequestDuration *prometheus.HistogramVec
    HTTPRequestTotal    *prometheus.CounterVec
    HTTPRequestsInFlight prometheus.Gauge
    
    // DynamoDB metrics
    DynamoDBOperationDuration *prometheus.HistogramVec
    DynamoDBOperationErrors   *prometheus.CounterVec
    
    // Business metrics
    NodesCreated   prometheus.Counter
    NodesDeleted   prometheus.Counter
    EdgesCreated   prometheus.Counter
    ActiveUsers    prometheus.Gauge
    
    // Lambda metrics
    ColdStarts     prometheus.Counter
    InvocationTotal prometheus.Counter
    InvocationErrors prometheus.Counter
    InvocationDuration prometheus.Histogram
}

func NewMetrics() *Metrics {
    return &Metrics{
        HTTPRequestDuration: promauto.NewHistogramVec(
            prometheus.HistogramOpts{
                Name:    "http_request_duration_seconds",
                Help:    "Duration of HTTP requests in seconds",
                Buckets: prometheus.DefBuckets,
            },
            []string{"method", "endpoint", "status"},
        ),
        
        HTTPRequestTotal: promauto.NewCounterVec(
            prometheus.CounterOpts{
                Name: "http_requests_total",
                Help: "Total number of HTTP requests",
            },
            []string{"method", "endpoint", "status"},
        ),
        
        HTTPRequestsInFlight: promauto.NewGauge(
            prometheus.GaugeOpts{
                Name: "http_requests_in_flight",
                Help: "Number of HTTP requests currently being processed",
            },
        ),
        
        DynamoDBOperationDuration: promauto.NewHistogramVec(
            prometheus.HistogramOpts{
                Name:    "dynamodb_operation_duration_seconds",
                Help:    "Duration of DynamoDB operations in seconds",
                Buckets: prometheus.DefBuckets,
            },
            []string{"operation", "table"},
        ),
        
        DynamoDBOperationErrors: promauto.NewCounterVec(
            prometheus.CounterOpts{
                Name: "dynamodb_operation_errors_total",
                Help: "Total number of DynamoDB operation errors",
            },
            []string{"operation", "table", "error_type"},
        ),
        
        NodesCreated: promauto.NewCounter(
            prometheus.CounterOpts{
                Name: "nodes_created_total",
                Help: "Total number of nodes created",
            },
        ),
        
        NodesDeleted: promauto.NewCounter(
            prometheus.CounterOpts{
                Name: "nodes_deleted_total",
                Help: "Total number of nodes deleted",
            },
        ),
        
        EdgesCreated: promauto.NewCounter(
            prometheus.CounterOpts{
                Name: "edges_created_total",
                Help: "Total number of edges created",
            },
        ),
        
        ActiveUsers: promauto.NewGauge(
            prometheus.GaugeOpts{
                Name: "active_users",
                Help: "Number of active users",
            },
        ),
        
        ColdStarts: promauto.NewCounter(
            prometheus.CounterOpts{
                Name: "lambda_cold_starts_total",
                Help: "Total number of Lambda cold starts",
            },
        ),
        
        InvocationTotal: promauto.NewCounter(
            prometheus.CounterOpts{
                Name: "lambda_invocations_total",
                Help: "Total number of Lambda invocations",
            },
        ),
        
        InvocationErrors: promauto.NewCounter(
            prometheus.CounterOpts{
                Name: "lambda_invocation_errors_total",
                Help: "Total number of Lambda invocation errors",
            },
        ),
        
        InvocationDuration: promauto.NewHistogram(
            prometheus.HistogramOpts{
                Name:    "lambda_invocation_duration_seconds",
                Help:    "Duration of Lambda invocations in seconds",
                Buckets: prometheus.DefBuckets,
            },
        ),
    }
}
```

2. **Metrics Middleware**
```go
// File: backend/internal/middleware/metrics.go
package middleware

import (
    "net/http"
    "strconv"
    "time"
    "brain2-backend/internal/metrics"
)

func PrometheusMiddleware(m *metrics.Metrics) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            start := time.Now()
            
            // Track in-flight requests
            m.HTTPRequestsInFlight.Inc()
            defer m.HTTPRequestsInFlight.Dec()
            
            // Wrap response writer to capture status code
            wrapped := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}
            
            // Process request
            next.ServeHTTP(wrapped, r)
            
            // Record metrics
            duration := time.Since(start).Seconds()
            statusStr := strconv.Itoa(wrapped.statusCode)
            
            m.HTTPRequestDuration.WithLabelValues(r.Method, r.URL.Path, statusStr).Observe(duration)
            m.HTTPRequestTotal.WithLabelValues(r.Method, r.URL.Path, statusStr).Inc()
        })
    }
}

type responseWriter struct {
    http.ResponseWriter
    statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
    rw.statusCode = code
    rw.ResponseWriter.WriteHeader(code)
}
```

3. **Repository Metrics Integration**
```go
// File: backend/infrastructure/dynamodb/metrics.go
package dynamodb

import (
    "context"
    "time"
)

func (r *ddbRepository) instrumentedQuery(ctx context.Context, input *dynamodb.QueryInput) (*dynamodb.QueryOutput, error) {
    start := time.Now()
    operation := "Query"
    
    result, err := r.dbClient.Query(ctx, input)
    
    duration := time.Since(start).Seconds()
    r.metrics.DynamoDBOperationDuration.WithLabelValues(operation, r.config.TableName).Observe(duration)
    
    if err != nil {
        r.metrics.DynamoDBOperationErrors.WithLabelValues(operation, r.config.TableName, "query_error").Inc()
    }
    
    return result, err
}
```

---

## 8. Lambda-Specific Optimizations

### 8.1 Connection Pooling Considerations

**Note**: AWS SDK v2 handles connection pooling internally. For Lambda, we focus on client reuse and configuration optimization.

#### Implementation:

```go
// File: backend/internal/config/aws.go
package config

import (
    "context"
    "time"
    "github.com/aws/aws-sdk-go-v2/aws"
    "github.com/aws/aws-sdk-go-v2/config"
)

func LoadOptimizedAWSConfig() (aws.Config, error) {
    return config.LoadDefaultConfig(context.TODO(),
        config.WithHTTPClient(&http.Client{
            Timeout: 30 * time.Second,
            Transport: &http.Transport{
                MaxIdleConns:        100,
                MaxIdleConnsPerHost: 10,
                IdleConnTimeout:     90 * time.Second,
            },
        }),
        config.WithRetryMode(aws.RetryModeAdaptive),
        config.WithRetryMaxAttempts(3),
    )
}
```

### 8.2 Cold Start Detection

```go
// File: backend/cmd/main/main.go
// Add to init():

var coldStart = true

func init() {
    if coldStart {
        log.Println("Cold start detected")
        if container != nil && container.Metrics != nil {
            container.Metrics.ColdStarts.Inc()
        }
        coldStart = false
    }
    // ... rest of init
}
```

---

## Implementation Order & Testing Strategy

### Phase 1: Foundation (Week 1)
1. Implement Race Condition Prevention (optimistic locking)
2. Add Enhanced Middleware (request ID, rate limiting)
3. Set up Metrics Collection framework

### Phase 2: Reliability (Week 2)
1. Implement Idempotency Keys
2. Add Transaction Rollback & Compensation (Saga pattern)
3. Circuit Breaker implementation

### Phase 3: Performance (Week 3)
1. Pagination & Query Optimization
2. Add parallel query execution
3. Implement caching layer (if needed)

### Phase 4: Polish (Week 4)
1. Dependency Injection enhancements
2. Complete metrics integration
3. Performance testing and optimization

## Testing Requirements

### Unit Tests to Add:
- `backend/internal/repository/optimistic_lock_test.go`
- `backend/internal/middleware/rate_limit_test.go`
- `backend/internal/saga/saga_test.go`
- `backend/internal/metrics/metrics_test.go`

### Integration Tests:
- Test idempotency with duplicate requests
- Test saga rollback scenarios
- Test pagination with large datasets
- Test concurrent updates with optimistic locking

### Load Tests:
- Test rate limiting under load
- Test circuit breaker behavior
- Measure metrics accuracy
- Validate pagination performance

## Monitoring & Observability

### CloudWatch Dashboards to Create:
1. **API Performance**: Request rates, latencies, error rates
2. **DynamoDB**: Read/write capacity, throttles, latencies
3. **Business Metrics**: Nodes created, edges created, active users
4. **Lambda Performance**: Cold starts, duration, errors

### Alarms to Configure:
- High error rate (>1% 5xx errors)
- High latency (p99 > 1s)
- DynamoDB throttling
- Circuit breaker open state

## Documentation Updates

### Update README.md with:
- New middleware documentation
- Idempotency key usage
- Pagination API documentation
- Metrics endpoints
- Saga pattern usage examples

### Create ARCHITECTURE.md with:
- Updated architecture diagrams
- Data flow diagrams
- Sequence diagrams for complex operations
- Deployment considerations

## Environment Variables to Add

```bash
# Rate Limiting
RATE_LIMIT_RPS=100
RATE_LIMIT_BURST=200

# Circuit Breaker
CIRCUIT_BREAKER_THRESHOLD=0.6
CIRCUIT_BREAKER_TIMEOUT=30s

# Idempotency
IDEMPOTENCY_KEY_TTL=86400

# Metrics
METRICS_ENABLED=true
METRICS_PORT=9090

# Pagination
DEFAULT_PAGE_SIZE=20
MAX_PAGE_SIZE=100
```

## Success Criteria

1. **No race conditions** in concurrent operations
2. **99.9% uptime** with circuit breakers preventing cascading failures
3. **<100ms p50 latency** for read operations
4. **<200ms p50 latency** for write operations
5. **100% idempotent** write operations
6. **Full observability** with metrics and tracing
7. **Automatic rollback** for failed multi-step operations
8. **Efficient pagination** for large datasets

## Notes for Claude Code

- Start with Phase 1 implementations as they form the foundation
- Run tests after each major component implementation
- Update the dependency injection container after adding new components
- Ensure backward compatibility when modifying interfaces
- Add comprehensive logging for debugging
- Document any deviations from this plan with justification