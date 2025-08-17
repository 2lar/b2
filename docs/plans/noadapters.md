# Major CQRS Rework & Adapter Elimination Plan

## Executive Summary

Transform the backend from adapter-heavy architecture to **clean CQRS implementation** with direct repository usage, eliminating all unnecessary translation layers.

## Current State Analysis

### Problems to Fix
1. **3 layers of adapters/bridges** causing unnecessary complexity
2. **Incomplete CQRS** - readers/writers defined but not properly used
3. **Mixed responsibilities** in services (commands and queries together)
4. **Interface misalignment** requiring translation layers
5. **Performance overhead** from multiple adapter calls

### Assets to Keep
- ✅ Good domain models
- ✅ Repository interfaces already support CQRS (`NodeReader`, `NodeWriter`)
- ✅ Query services exist (`NodeQueryService`, `CategoryQueryService`)
- ✅ Command patterns in services
- ✅ DynamoDB implementations

## Target Architecture

```
┌─────────────────────────────────────────────────┐
│                   HTTP Layer                     │
│              handlers/middleware                 │
└────────────┬────────────────┬───────────────────┘
             │                │
    Commands │                │ Queries
             ▼                ▼
┌─────────────────┐  ┌─────────────────┐
│ Command Handler │  │  Query Handler  │
│    (writes)     │  │    (reads)      │
└────────┬────────┘  └────────┬────────┘
         │                     │
         ▼                     ▼
┌─────────────────┐  ┌─────────────────┐
│ Command Service │  │  Query Service  │
│  (NodeService)  │  │(NodeQueryService)│
└────────┬────────┘  └────────┬────────┘
         │                     │
         ▼                     ▼
┌─────────────────┐  ┌─────────────────┐
│   NodeWriter    │  │   NodeReader    │
│  (repository)   │  │  (repository)   │
└────────┬────────┘  └────────┬────────┘
         │                     │
         └──────────┬──────────┘
                    ▼
           ┌─────────────────┐
           │    DynamoDB     │
           └─────────────────┘
```

## Implementation Plan

### Phase 1: Repository Consolidation (2 days)

#### 1.1 Create Unified Repository Interfaces

```go
// internal/repository/node_repository.go

// NodeRepository combines read and write operations
type NodeRepository interface {
    // Write operations (Commands)
    Create(ctx context.Context, node *domain.Node) error
    Update(ctx context.Context, node *domain.Node) error
    Delete(ctx context.Context, userID domain.UserID, nodeID domain.NodeID) error
    
    // Read operations (Queries)
    FindByID(ctx context.Context, nodeID domain.NodeID) (*domain.Node, error)
    FindByUserID(ctx context.Context, userID domain.UserID) ([]*domain.Node, error)
    FindBySpecification(ctx context.Context, spec Specification) ([]*domain.Node, error)
    GetPage(ctx context.Context, query Query, page Pagination) (*NodePage, error)
}

// For strict CQRS separation (optional)
type NodeCommandRepository interface {
    Create(ctx context.Context, node *domain.Node) error
    Update(ctx context.Context, node *domain.Node) error
    Delete(ctx context.Context, userID domain.UserID, nodeID domain.NodeID) error
}

type NodeQueryRepository interface {
    FindByID(ctx context.Context, nodeID domain.NodeID) (*domain.Node, error)
    FindByUserID(ctx context.Context, userID domain.UserID) ([]*domain.Node, error)
    FindBySpecification(ctx context.Context, spec Specification) ([]*domain.Node, error)
    GetPage(ctx context.Context, query Query, page Pagination) (*NodePage, error)
}
```

#### 1.2 Update DynamoDB Implementation

```go
// internal/infrastructure/dynamodb/node_repository.go

type DynamoDBNodeRepository struct {
    client *dynamodb.Client
    table  string
    logger *zap.Logger
}

// Implement unified interface directly - NO ADAPTERS!
func (r *DynamoDBNodeRepository) Create(ctx context.Context, node *domain.Node) error {
    item := r.marshalNode(node)
    
    input := &dynamodb.PutItemInput{
        TableName: aws.String(r.table),
        Item:      item,
    }
    
    _, err := r.client.PutItem(ctx, input)
    return err
}

func (r *DynamoDBNodeRepository) FindByID(ctx context.Context, nodeID domain.NodeID) (*domain.Node, error) {
    key := r.buildKey(nodeID)
    
    input := &dynamodb.GetItemInput{
        TableName: aws.String(r.table),
        Key:       key,
    }
    
    result, err := r.client.GetItem(ctx, input)
    if err != nil {
        return nil, err
    }
    
    return r.unmarshalNode(result.Item)
}
```

### Phase 2: Service Layer Separation (2 days)

#### 2.1 Command Service (Write Side)

```go
// internal/application/commands/node_commands.go

type CreateNodeCommand struct {
    UserID  string
    Content string
    Tags    []string
}

type UpdateNodeCommand struct {
    NodeID  string
    Content string
    Tags    []string
    Version int // for optimistic locking
}

// internal/application/commands/node_command_handler.go

type NodeCommandHandler struct {
    repo      repository.NodeRepository  // Direct usage!
    uow       repository.UnitOfWork
    eventBus  domain.EventBus
    validator Validator
}

func (h *NodeCommandHandler) Handle(ctx context.Context, cmd CreateNodeCommand) (*CreateNodeResult, error) {
    // 1. Validate command
    if err := h.validator.Validate(cmd); err != nil {
        return nil, err
    }
    
    // 2. Create domain object
    userID, _ := domain.NewUserID(cmd.UserID)
    content, _ := domain.NewContent(cmd.Content)
    node := domain.NewNode(userID, content)
    
    // 3. Begin transaction
    if err := h.uow.Begin(ctx); err != nil {
        return nil, err
    }
    defer h.uow.Rollback()
    
    // 4. Save using repository directly
    if err := h.repo.Create(ctx, node); err != nil {
        return nil, err
    }
    
    // 5. Publish event
    event := domain.NewNodeCreatedEvent(node)
    if err := h.eventBus.Publish(ctx, event); err != nil {
        return nil, err
    }
    
    // 6. Commit transaction
    if err := h.uow.Commit(); err != nil {
        return nil, err
    }
    
    // 7. Return result
    return &CreateNodeResult{
        NodeID: node.ID.String(),
        Success: true,
    }, nil
}
```

#### 2.2 Query Service (Read Side)

```go
// internal/application/queries/node_queries.go

type GetNodeQuery struct {
    UserID string
    NodeID string
}

type ListNodesQuery struct {
    UserID  string
    Tags    []string
    Limit   int
    Cursor  string
}

// internal/application/queries/node_query_handler.go

type NodeQueryHandler struct {
    repo  repository.NodeRepository  // Direct usage!
    cache Cache
}

func (h *NodeQueryHandler) Handle(ctx context.Context, query GetNodeQuery) (*NodeView, error) {
    // 1. Check cache
    cacheKey := fmt.Sprintf("node:%s:%s", query.UserID, query.NodeID)
    if cached, ok := h.cache.Get(cacheKey); ok {
        return cached.(*NodeView), nil
    }
    
    // 2. Query repository directly
    nodeID, _ := domain.ParseNodeID(query.NodeID)
    node, err := h.repo.FindByID(ctx, nodeID)
    if err != nil {
        return nil, err
    }
    
    // 3. Convert to view model
    view := &NodeView{
        ID:       node.ID.String(),
        Content:  node.Content.String(),
        Tags:     node.Tags.ToSlice(),
        Created:  node.CreatedAt,
        Modified: node.UpdatedAt,
    }
    
    // 4. Cache result
    h.cache.Set(cacheKey, view, 5*time.Minute)
    
    return view, nil
}
```

### Phase 3: Wire/DI Update (1 day)

#### 3.1 Remove All Adapter Creation

```go
// internal/di/wire.go

//go:build wireinject

func InitializeApp(ctx context.Context) (*App, error) {
    wire.Build(
        // Config
        config.Load,
        
        // Infrastructure - Direct repositories, no adapters!
        provideDynamoDBClient,
        provideNodeRepository,
        provideEdgeRepository,
        provideCategoryRepository,
        
        // Domain Services
        domainServices.NewConnectionAnalyzer,
        
        // Command Handlers
        commands.NewNodeCommandHandler,
        commands.NewEdgeCommandHandler,
        
        // Query Handlers  
        queries.NewNodeQueryHandler,
        queries.NewGraphQueryHandler,
        
        // HTTP Handlers
        handlers.NewNodeHandler,
        
        // Router
        NewRouter,
        
        // App
        NewApp,
    )
    return nil, nil
}

// providers.go

func provideNodeRepository(client *dynamodb.Client, cfg *config.Config) repository.NodeRepository {
    // Direct repository, no adapters!
    return dynamodb.NewNodeRepository(client, cfg.TableName, cfg.Logger)
}

func provideNodeCommandHandler(
    repo repository.NodeRepository,
    uow repository.UnitOfWork,
    bus domain.EventBus,
) *commands.NodeCommandHandler {
    // Direct injection, no adapters!
    return commands.NewNodeCommandHandler(repo, uow, bus)
}
```

### Phase 4: HTTP Handler Update (1 day)

#### 4.1 Route Commands and Queries

```go
// internal/interfaces/http/handlers/node_handler.go

type NodeHandler struct {
    commandHandler *commands.NodeCommandHandler
    queryHandler   *queries.NodeQueryHandler
}

// POST /nodes - Command
func (h *NodeHandler) CreateNode(w http.ResponseWriter, r *http.Request) {
    var req CreateNodeRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, err.Error(), http.StatusBadRequest)
        return
    }
    
    // Route to command handler
    cmd := commands.CreateNodeCommand{
        UserID:  getUserID(r.Context()),
        Content: req.Content,
        Tags:    req.Tags,
    }
    
    result, err := h.commandHandler.Handle(r.Context(), cmd)
    if err != nil {
        handleError(w, err)
        return
    }
    
    json.NewEncoder(w).Encode(result)
}

// GET /nodes/:id - Query
func (h *NodeHandler) GetNode(w http.ResponseWriter, r *http.Request) {
    nodeID := chi.URLParam(r, "id")
    
    // Route to query handler
    query := queries.GetNodeQuery{
        UserID: getUserID(r.Context()),
        NodeID: nodeID,
    }
    
    result, err := h.queryHandler.Handle(r.Context(), query)
    if err != nil {
        handleError(w, err)
        return
    }
    
    json.NewEncoder(w).Encode(result)
}
```

### Phase 5: Cleanup (1 day)

#### 5.1 Delete All Adapter Files

```bash
# Remove adapter directories
rm -rf internal/application/adapters/
rm -rf internal/di/repository_bridges.go
rm -rf internal/di/reader_adapters.go

# Remove stub implementations
rm -rf internal/application/adapters/stub_adapters.go

# Remove unused interfaces
rm internal/repository/adapter_interfaces.go
```

#### 5.2 Update Tests

```go
// internal/application/commands/node_command_handler_test.go

func TestNodeCommandHandler_CreateNode(t *testing.T) {
    // Mock repository directly
    mockRepo := &mocks.NodeRepository{}
    mockUOW := &mocks.UnitOfWork{}
    mockBus := &mocks.EventBus{}
    
    handler := commands.NewNodeCommandHandler(mockRepo, mockUOW, mockBus)
    
    // Test command handling
    cmd := commands.CreateNodeCommand{
        UserID:  "user123",
        Content: "Test content",
    }
    
    mockRepo.On("Create", mock.Anything, mock.Anything).Return(nil)
    mockUOW.On("Begin", mock.Anything).Return(nil)
    mockUOW.On("Commit").Return(nil)
    mockBus.On("Publish", mock.Anything, mock.Anything).Return(nil)
    
    result, err := handler.Handle(context.Background(), cmd)
    
    assert.NoError(t, err)
    assert.True(t, result.Success)
    mockRepo.AssertExpectations(t)
}
```

## Migration Timeline

### Week 1
- **Day 1-2**: Repository consolidation
  - Update interfaces
  - Modify DynamoDB implementations
  - Remove adapter requirements

- **Day 3-4**: Service layer separation  
  - Create command handlers
  - Create query handlers
  - Remove mixed services

- **Day 5**: Wire/DI updates
  - Update provider functions
  - Remove adapter creation
  - Simplify dependency graph

### Week 2
- **Day 1**: HTTP handler updates
  - Route to appropriate handlers
  - Clean request/response flow

- **Day 2**: Cleanup
  - Delete adapter files
  - Remove unused code
  - Update documentation

- **Day 3-4**: Testing
  - Update unit tests
  - Integration testing
  - Performance testing

- **Day 5**: Documentation & Review
  - Update architecture diagrams
  - Code review
  - Performance benchmarks

## Success Metrics

### Before Rework
- 5+ layers of indirection
- 3 adapter/bridge files
- Mixed command/query in services
- Complex dependency graph
- ~15-20ms overhead from adapters

### After Rework
- 2-3 layers maximum
- 0 adapter files
- Clear CQRS separation
- Simple dependency graph
- <5ms request overhead

## Risk Mitigation

### Backward Compatibility
```go
// Temporary compatibility layer during migration
type LegacyNodeService struct {
    commandHandler *commands.NodeCommandHandler
    queryHandler   *queries.NodeQueryHandler
}

func (s *LegacyNodeService) CreateNode(ctx context.Context, ...) {
    // Delegate to new handler
}
```

### Rollback Plan
1. Keep adapter code in separate branch
2. Feature flag for new/old implementation
3. Gradual rollout by endpoint

## Benefits After Rework

### 1. **Architectural Clarity**
- Clear separation of commands and queries
- No translation layers
- Direct, obvious data flow

### 2. **Performance**
- 50% reduction in function calls
- Direct repository access
- Optimized read/write paths

### 3. **Maintainability**
- Less code to maintain
- Clear responsibilities
- Easier debugging

### 4. **Scalability**
- Can scale reads/writes independently
- Can use different storage for reads
- Can add read replicas easily

### 5. **Developer Experience**
- Easier to understand
- Faster to add features
- Better testing

## Conclusion

This rework will transform your backend from a complex, adapter-heavy system to a **clean, efficient CQRS implementation**. The result will be:

- ✅ True CQRS with separated read/write paths
- ✅ Zero unnecessary adapters
- ✅ Direct repository usage
- ✅ Clear command/query handlers
- ✅ Better performance
- ✅ Easier maintenance

The investment of 1-2 weeks will pay off immediately in reduced complexity and improved developer velocity.