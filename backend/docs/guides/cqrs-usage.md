# CQRS Usage Guide for Brain2 Developers

## Overview

This guide explains how to work with Brain2's service-level CQRS implementation. We separate command (write) and query (read) operations at the application service layer while maintaining unified repository interfaces.

## Architecture Quick Reference

```
HTTP Handler → Command Service → Repository → DynamoDB (for writes)
HTTP Handler → Query Service → Repository → DynamoDB (for reads)
```

## When to Use Each Service Type

### Use Command Services When:
- Creating new entities (POST requests)
- Updating existing entities (PUT/PATCH requests)
- Deleting entities (DELETE requests)
- Any operation that changes state
- Operations requiring business validation
- Operations that should publish events

### Use Query Services When:
- Retrieving entities (GET requests)
- Searching or filtering data
- Getting aggregated views
- Any read-only operation
- Operations that benefit from caching

## Code Patterns

### 1. HTTP Handler Pattern

Handlers should have both command and query service dependencies:

```go
type YourHandler struct {
    // Command service for writes
    yourService      *services.YourService
    // Query service for reads
    yourQueryService *queries.YourQueryService
}

func NewYourHandler(
    yourService *services.YourService,
    yourQueryService *queries.YourQueryService,
) *YourHandler {
    return &YourHandler{
        yourService:      yourService,
        yourQueryService: yourQueryService,
    }
}
```

### 2. Routing Requests to Services

#### Write Operations → Command Service

```go
func (h *YourHandler) Create(w http.ResponseWriter, r *http.Request) {
    // 1. Parse request into command
    var req CreateRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        api.Error(w, http.StatusBadRequest, "Invalid request")
        return
    }

    // 2. Create command
    cmd := commands.CreateYourEntityCommand{
        UserID:  getUserID(r),
        Field1:  req.Field1,
        Field2:  req.Field2,
    }

    // 3. Execute through command service
    result, err := h.yourService.CreateEntity(r.Context(), cmd)
    if err != nil {
        handleServiceError(w, err)
        return
    }

    // 4. Return response
    api.Success(w, http.StatusCreated, result)
}
```

#### Read Operations → Query Service

```go
func (h *YourHandler) List(w http.ResponseWriter, r *http.Request) {
    // 1. Build query from request
    query := queries.ListEntitiesQuery{
        UserID: getUserID(r),
        Limit:  getLimit(r),
        Offset: getOffset(r),
    }

    // 2. Execute through query service
    result, err := h.yourQueryService.ListEntities(r.Context(), query)
    if err != nil {
        handleServiceError(w, err)
        return
    }

    // 3. Return view models
    api.Success(w, http.StatusOK, result)
}
```

### 3. Command Service Pattern

Command services handle business logic, validation, and events:

```go
type YourService struct {
    // Dependencies
    repo       repository.YourRepository  // Unified repository
    eventBus   shared.EventBus
    uowFactory repository.UnitOfWorkFactory
}

func (s *YourService) CreateEntity(ctx context.Context, cmd CreateEntityCommand) (*dto.CreateResult, error) {
    // 1. Start transaction
    uow := s.uowFactory.Create()
    defer uow.Rollback()

    // 2. Validate command
    if err := s.validateCommand(cmd); err != nil {
        return nil, err
    }

    // 3. Create domain entity
    entity := domain.NewEntity(cmd.Field1, cmd.Field2)

    // 4. Apply business rules
    if err := entity.ApplyBusinessRules(); err != nil {
        return nil, err
    }

    // 5. Persist
    if err := s.repo.Create(ctx, entity); err != nil {
        return nil, err
    }

    // 6. Publish events
    event := events.EntityCreated{
        EntityID: entity.ID,
        UserID:   entity.UserID,
    }
    if err := s.eventBus.Publish(ctx, event); err != nil {
        // Log but don't fail
    }

    // 7. Commit transaction
    if err := uow.Commit(); err != nil {
        return nil, err
    }

    // 8. Return result
    return &dto.CreateResult{
        Entity: dto.ToEntityView(entity),
    }, nil
}
```

### 4. Query Service Pattern

Query services focus on efficient data retrieval and transformation:

```go
type YourQueryService struct {
    // Dependencies
    repo  repository.YourRepository  // Same unified repository
    cache Cache
}

func (s *YourQueryService) GetEntity(ctx context.Context, query GetEntityQuery) (*dto.EntityView, error) {
    // 1. Check cache
    cacheKey := fmt.Sprintf("entity:%s:%s", query.UserID, query.EntityID)
    if cached := s.cache.Get(cacheKey); cached != nil {
        return cached.(*dto.EntityView), nil
    }

    // 2. Query repository
    entity, err := s.repo.FindByID(ctx, query.UserID, query.EntityID)
    if err != nil {
        return nil, err
    }

    // 3. Transform to view model
    view := dto.ToEntityView(entity)

    // 4. Cache result
    s.cache.Set(cacheKey, view, 5*time.Minute)

    // 5. Return view (no side effects!)
    return view, nil
}

func (s *YourQueryService) ListEntities(ctx context.Context, query ListEntitiesQuery) (*dto.ListResult, error) {
    // 1. Build repository query
    repoQuery := repository.EntityQuery{
        UserID: query.UserID,
        Limit:  query.Limit,
        Offset: query.Offset,
    }

    // 2. Execute query
    entities, err := s.repo.FindEntities(ctx, repoQuery)
    if err != nil {
        return nil, err
    }

    // 3. Transform to view models
    views := make([]*dto.EntityView, len(entities))
    for i, entity := range entities {
        views[i] = dto.ToEntityView(entity)
    }

    // 4. Return paginated result
    return &dto.ListResult{
        Items:   views,
        HasMore: len(entities) == query.Limit,
    }, nil
}
```

## Key Differences Between Services

| Aspect | Command Service | Query Service |
|--------|----------------|---------------|
| **Purpose** | Modify state | Retrieve data |
| **Side Effects** | Yes (events, state changes) | No |
| **Transactions** | Uses Unit of Work | Not needed |
| **Events** | Publishes domain events | Never publishes |
| **Caching** | Invalidates cache | Uses cache |
| **Validation** | Business rule validation | Query parameter validation |
| **Return Type** | Command results/DTOs | View models/DTOs |
| **Error Handling** | Business errors | Not found errors |
| **Idempotency** | Required for safety | Not needed |

## Repository Usage

Both service types use the **same unified repository interfaces**:

```go
// Both services use this single interface
type NodeRepository interface {
    CreateNodeAndKeywords(ctx context.Context, node *node.Node) error
    FindNodeByID(ctx context.Context, userID, nodeID string) (*node.Node, error)
    FindNodes(ctx context.Context, query NodeQuery) ([]*node.Node, error)
    DeleteNode(ctx context.Context, userID, nodeID string) error
}

// NOT this (we don't have Reader/Writer split)
type NodeReader interface { ... }  // ❌ We don't use this
type NodeWriter interface { ... }  // ❌ We don't use this
```

## Event Publishing

Only command services publish events:

```go
// ✅ In command service
func (s *NodeService) CreateNode(...) {
    // ... create node ...
    
    event := events.NodeCreated{...}
    s.eventBus.Publish(ctx, event)  // ✅ Commands publish events
}

// ❌ Never in query service
func (s *NodeQueryService) GetNode(...) {
    // ... get node ...
    
    // Never publish events from queries! ❌
}
```

## Caching Strategy

### Query Services: Use Cache

```go
func (s *NodeQueryService) GetNode(ctx context.Context, query GetNodeQuery) (*dto.NodeView, error) {
    // Check cache first
    if cached := s.cache.Get(cacheKey); cached != nil {
        return cached.(*dto.NodeView), nil  // ✅ Return cached
    }
    
    // ... fetch from DB ...
    
    s.cache.Set(cacheKey, result, 5*time.Minute)  // ✅ Cache result
    return result, nil
}
```

### Command Services: Invalidate Cache

```go
func (s *NodeService) UpdateNode(ctx context.Context, cmd UpdateNodeCommand) error {
    // ... update node ...
    
    // Invalidate related cache entries
    s.cache.Delete(fmt.Sprintf("node:%s:%s", cmd.UserID, cmd.NodeID))  // ✅ Invalidate
    
    return nil
}
```

## Testing

### Testing Command Services

```go
func TestNodeService_CreateNode(t *testing.T) {
    // Mock dependencies
    mockRepo := &mocks.MockNodeRepository{}
    mockEventBus := &mocks.MockEventBus{}
    
    service := services.NewNodeService(mockRepo, mockEventBus, ...)
    
    // Test business logic
    cmd := commands.CreateNodeCommand{...}
    
    // Expect repository call
    mockRepo.On("CreateNodeAndKeywords", mock.Anything, mock.Anything).Return(nil)
    
    // Expect event publication
    mockEventBus.On("Publish", mock.Anything, mock.MatchedBy(func(e interface{}) bool {
        _, ok := e.(events.NodeCreated)
        return ok
    })).Return(nil)
    
    // Execute
    result, err := service.CreateNode(context.Background(), cmd)
    
    // Assert
    assert.NoError(t, err)
    assert.NotNil(t, result)
    mockRepo.AssertExpectations(t)
    mockEventBus.AssertExpectations(t)
}
```

### Testing Query Services

```go
func TestNodeQueryService_GetNode(t *testing.T) {
    // Mock dependencies
    mockRepo := &mocks.MockNodeRepository{}
    mockCache := &mocks.MockCache{}
    
    service := queries.NewNodeQueryService(mockRepo, mockCache)
    
    // Test cache hit
    mockCache.On("Get", "node:user1:node1").Return(&dto.NodeView{...})
    
    result, err := service.GetNode(context.Background(), queries.GetNodeQuery{
        UserID: "user1",
        NodeID: "node1",
    })
    
    assert.NoError(t, err)
    assert.NotNil(t, result)
    mockRepo.AssertNotCalled(t, "FindNodeByID")  // Should not hit DB
}
```

## Common Patterns

### 1. Bulk Operations

Command services handle bulk writes:
```go
func (s *NodeService) BulkDelete(ctx context.Context, cmd BulkDeleteCommand) error {
    // Transactional bulk operation
}
```

Query services handle bulk reads:
```go
func (s *NodeQueryService) GetNodesByIDs(ctx context.Context, query GetNodesByIDsQuery) ([]*dto.NodeView, error) {
    // Optimized batch fetch
}
```

### 2. Complex Queries

Keep complex business queries in query services:
```go
func (s *GraphQueryService) GetNodeNeighborhood(ctx context.Context, query NeighborhoodQuery) (*dto.GraphView, error) {
    // Complex graph traversal logic
}
```

### 3. Validation

- **Command Services**: Business rule validation
- **Query Services**: Parameter validation only

```go
// Command service - business validation
func (s *NodeService) validateCreateNode(cmd CreateNodeCommand) error {
    if len(cmd.Content) < 10 {
        return errors.New("content too short")  // Business rule
    }
}

// Query service - parameter validation
func (s *NodeQueryService) validateQuery(query ListNodesQuery) error {
    if query.Limit > 100 {
        return errors.New("limit too high")  // Parameter constraint
    }
}
```

## Anti-Patterns to Avoid

### ❌ Don't Mix Responsibilities

```go
// BAD: Query service modifying state
func (s *NodeQueryService) GetAndIncrementViewCount(ctx context.Context, ...) {
    node := s.repo.FindByID(...)
    node.ViewCount++  // ❌ Queries shouldn't modify!
    s.repo.Update(node)
}
```

### ❌ Don't Skip Service Layer

```go
// BAD: Handler directly using repository
func (h *Handler) GetNode(w http.ResponseWriter, r *http.Request) {
    node := h.nodeRepo.FindByID(...)  // ❌ Should use query service
}
```

### ❌ Don't Create Reader/Writer Splits

```go
// BAD: Trying to split repositories
type NodeReader interface { ... }  // ❌ We use unified interfaces
type NodeWriter interface { ... }  // ❌ Not needed
```

## Summary

Brain2's CQRS implementation is **pragmatic and effective**:

1. **Service-Level Separation**: Commands and Queries separated at service layer
2. **Unified Repositories**: Single repository interface per entity
3. **Clear Responsibilities**: Commands write and publish events, Queries read and cache
4. **Simple Testing**: Mock services, not complex repository splits
5. **Future-Ready**: Can evolve to full CQRS if needed

Follow these patterns for consistent, maintainable code that captures CQRS benefits without unnecessary complexity.