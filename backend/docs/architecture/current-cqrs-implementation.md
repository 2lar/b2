# Current CQRS Implementation in Brain2

## Overview

Brain2 implements Command Query Responsibility Segregation (CQRS) at the **application service layer**, providing a pragmatic separation between command (write) and query (read) operations while maintaining simplicity and avoiding over-engineering.

## Architecture Overview

```
┌─────────────────────────────────────────────────────┐
│                  HTTP Handlers                      │
│  ┌────────────────────────────────────────────┐     │
│  │  MemoryHandler / CategoryHandler           │     │
│  │  - Uses both Command & Query Services      │     │
│  └────────────────────────────────────────────┘     │
└─────────────────────────────────────────────────────┘
                      │
        ┌─────────────┴─────────────┐
        ▼                           ▼
┌──────────────────┐       ┌──────────────────┐
│ Command Services │       │  Query Services  │
│                  │       │                  │
│ NodeService      │       │ NodeQueryService │
│ CategoryService  │       │ CategoryQuerySvc │
│ CleanupService   │       │ EdgeQueryService │
│                  │       │ GraphQueryService│
│ • Validation     │       │ • Caching        │
│ • Business Logic │       │ • View Models    │
│ • Transactions   │       │ • Read Optimize  │
│ • Event Publish  │       │ • No Side Effects│
└──────────────────┘       └──────────────────┘
        │                           │
        └─────────────┬─────────────┘
                      ▼
        ┌──────────────────────────┐
        │  Unified Repositories    │
        │                          │
        │  NodeRepository          │
        │  EdgeRepository          │
        │  CategoryRepository      │
        │                          │
        │  (Single interface per   │
        │   entity, no R/W split)  │
        └──────────────────────────┘
                      │
                      ▼
            ┌──────────────────┐
            │    DynamoDB       │
            │  (Same storage    │
            │   for R/W ops)    │
            └──────────────────┘
```

## Key Design Decisions

### 1. Service-Level CQRS (Not Repository-Level)

**Decision**: Implement CQRS at the application service layer rather than the repository layer.

**Rationale**:
- Repositories remain simple with a single interface per entity
- Complexity is managed at the service layer where business logic lives
- Easier to test and maintain
- Avoids artificial Reader/Writer split that adds no value

### 2. Unified Repository Interfaces

**What We Have**:
```go
// Single, unified interface per entity
type NodeRepository interface {
    CreateNodeAndKeywords(ctx context.Context, node *node.Node) error
    FindNodeByID(ctx context.Context, userID, nodeID string) (*node.Node, error)
    FindNodes(ctx context.Context, query NodeQuery) ([]*node.Node, error)
    DeleteNode(ctx context.Context, userID, nodeID string) error
    // ... other methods
}
```

**What We Don't Have** (and why that's good):
```go
// We DON'T have separate Reader/Writer interfaces
// This was removed because it added complexity without benefits
type NodeReader interface { /* read methods */ }  // ❌ Not used
type NodeWriter interface { /* write methods */ } // ❌ Not used
```

### 3. Clear Separation at Service Layer

**Command Services** (`/internal/application/services/`):
- Handle all write operations
- Contain business logic and validation
- Manage transactions (Unit of Work)
- Publish domain events
- Implement idempotency

**Query Services** (`/internal/application/queries/`):
- Handle all read operations
- Implement caching strategies
- Transform to view models (DTOs)
- Optimize for read performance
- No side effects

## Implementation Examples

### Write Operation Flow (Command)

```go
// HTTP Handler
func (h *MemoryHandler) CreateNode(w http.ResponseWriter, r *http.Request) {
    // 1. Parse request into command
    cmd := commands.CreateNodeCommand{...}
    
    // 2. Execute through Command Service
    result, err := h.nodeService.CreateNode(r.Context(), cmd)
    
    // 3. Publish events, return response
}

// Command Service
func (s *NodeService) CreateNode(ctx context.Context, cmd CreateNodeCommand) (*dto.CreateNodeResult, error) {
    // Business logic
    // Validation
    // Transaction management
    // Event publishing
    // Repository interaction
}
```

### Read Operation Flow (Query)

```go
// HTTP Handler
func (h *MemoryHandler) ListNodes(w http.ResponseWriter, r *http.Request) {
    // 1. Create query
    query := queries.NewListNodesQuery(userID)
    
    // 2. Execute through Query Service
    response, err := h.nodeQueryService.ListNodes(r.Context(), query)
    
    // 3. Return view models
}

// Query Service
func (s *NodeQueryService) ListNodes(ctx context.Context, query ListNodesQuery) (*dto.ListNodesResult, error) {
    // Check cache
    // Query repository
    // Transform to view models
    // Apply pagination
    // Return results (no side effects)
}
```

## Benefits of Current Approach

### 1. **Simplicity**
- Single repository interface is easier to understand
- No confusing Reader/Writer split
- Clear service responsibilities

### 2. **Flexibility**
- Can optimize reads and writes differently at service level
- Easy to add caching to queries without affecting commands
- Can evolve to full CQRS if needed

### 3. **Maintainability**
- Less code duplication
- Fewer interfaces to maintain
- Clear separation of concerns

### 4. **Testability**
- Services are easy to mock
- Single repository interface simplifies testing
- Clear boundaries between layers

## What We Don't Have (And Don't Need Yet)

### 1. **Separate Read/Write Models**
- We use the same domain models for reads and writes
- This is fine for our current scale and complexity

### 2. **Event Sourcing**
- We publish events but don't source from them
- Events are for integration, not state reconstruction

### 3. **Separate Read Database**
- Both reads and writes use the same DynamoDB
- No specialized read stores (yet)

### 4. **Eventual Consistency**
- Reads immediately reflect writes
- No lag between command and query models

## When to Evolve Further

Consider adding more CQRS patterns when:

1. **Performance Requirements**
   - Need specialized read models (e.g., Elasticsearch for search)
   - Read/write ratio becomes extremely skewed
   - Complex aggregations require denormalized views

2. **Scale Requirements**
   - Need to scale reads and writes independently
   - Different SLAs for reads vs writes
   - Geographic distribution needs

3. **Complexity Requirements**
   - Event sourcing becomes necessary for audit
   - Need temporal queries (state at point in time)
   - Multiple bounded contexts need different views

## Migration Path to Full CQRS

If needed, the current architecture supports evolution:

1. **Add Read Models**: Create denormalized projections
2. **Event-Driven Sync**: Use existing events to update read models
3. **Separate Storage**: Move read models to optimized storage
4. **Eventually Consistent**: Accept lag between writes and reads

## Summary

Brain2's CQRS implementation is **pragmatic and effective**:
- ✅ Clear separation of commands and queries
- ✅ Different optimization strategies for reads/writes
- ✅ Foundation for event-driven architecture
- ✅ Simple, maintainable code
- ❌ No unnecessary complexity
- ❌ No premature optimization

This approach provides the **benefits of CQRS** (separation of concerns, independent optimization) without the **complexity of full CQRS** (separate models, eventual consistency, event sourcing).

## References

- [Martin Fowler on CQRS](https://martinfowler.com/bliki/CQRS.html)
- [Original ADR-002](/docs/architecture/adr/002-cqrs-implementation.md)
- [Code: Command Services](/internal/application/services/)
- [Code: Query Services](/internal/application/queries/)