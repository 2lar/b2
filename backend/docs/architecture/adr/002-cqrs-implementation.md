# ADR-002: CQRS Pattern Implementation

## Status
Accepted

## Context
The Brain2 backend has different requirements for read and write operations:
- **Writes**: Need strong consistency, validation, business rules, and event generation
- **Reads**: Need performance, caching, flexible querying, and denormalization

Initially, we had a single repository interface handling both concerns, leading to:
- Complex repository interfaces with many methods
- Difficulty optimizing for specific use cases
- Mixed read/write concerns in the same code
- Challenges in scaling reads independently from writes

## Decision
Implement Command Query Responsibility Segregation (CQRS) with separate models for reads and writes:

### Write Side (Commands):
```go
// Commands modify state
type CreateNodeCommand struct {
    UserID  string
    Content string
    Tags    []string
}

// Command handlers contain business logic
type NodeCommandHandler struct {
    repository NodeWriter
    eventBus   EventBus
}
```

### Read Side (Queries):
```go
// Queries retrieve data
type GetNodeQuery struct {
    UserID             string
    NodeID             string
    IncludeConnections bool
}

// Query handlers optimize for reading
type NodeQueryHandler struct {
    reader NodeReader
    cache  Cache
}
```

## Consequences

### Positive:
- Independent optimization of reads and writes
- Better performance through specialized queries
- Clearer separation of concerns
- Ability to use different storage for reads (e.g., Elasticsearch for search)
- Event sourcing compatibility

### Negative:
- Increased complexity
- Potential eventual consistency between read and write models
- More code to maintain
- Need for synchronization mechanisms

### Neutral:
- Requires understanding of CQRS principles
- Different mental model from traditional CRUD

## Implementation Notes

### Current Implementation Status:
1. ✅ Separate read/write interfaces defined
2. ✅ Query services implemented for reads
3. ✅ Command services implemented for writes
4. ⚠️ Bridge adapters created (temporary solution)
5. ❌ Event synchronization not fully implemented

### Bridge Pattern (Temporary):
Due to incomplete CQRS migration, we use bridges to adapt between old and new interfaces:

```go
// Bridges adapt old repositories to new CQRS interfaces
type NodeReaderBridge struct {
    repo repository.NodeRepository
}

func (b *NodeReaderBridge) FindByID(ctx context.Context, id domain.NodeID) (*domain.Node, error) {
    // Adapt to old interface (note: empty userID is a known issue)
    return b.repo.FindNodeByID(ctx, "", id.String())
}
```

### Future Improvements:
1. Complete separation of read and write models
2. Implement event-driven synchronization
3. Add read model projections
4. Remove bridge adapters
5. Implement event sourcing for audit trail

### Example Usage:

**Write Operation:**
```go
// Application service for commands
func (s *NodeService) CreateNode(ctx context.Context, cmd CreateNodeCommand) error {
    // Business logic and validation
    node := domain.NewNode(cmd.Content)
    
    // Write through command repository
    err := s.writer.Save(ctx, node)
    
    // Publish domain event
    s.eventBus.Publish(NodeCreated{NodeID: node.ID})
    
    return err
}
```

**Read Operation:**
```go
// Query service for reads
func (s *NodeQueryService) GetNode(ctx context.Context, query GetNodeQuery) (*NodeView, error) {
    // Check cache first
    if cached := s.cache.Get(query.NodeID); cached != nil {
        return cached, nil
    }
    
    // Read through query repository
    node := s.reader.FindByID(ctx, query.NodeID)
    
    // Transform to read model
    view := toNodeView(node)
    
    // Cache for future reads
    s.cache.Set(query.NodeID, view)
    
    return view, nil
}
```

## Migration Path

### Phase 1: Interface Separation ✅
- Create separate Reader and Writer interfaces
- Implement bridge adapters for compatibility

### Phase 2: Service Layer Separation (In Progress)
- Separate command and query services
- Implement proper DTOs for each side

### Phase 3: Storage Optimization (Future)
- Optimize write storage for consistency
- Optimize read storage for queries
- Implement materialized views

### Phase 4: Event-Driven Synchronization (Future)
- Implement event bus
- Create projections for read models
- Ensure eventual consistency

## References
- [CQRS by Martin Fowler](https://martinfowler.com/bliki/CQRS.html)
- [CQRS Journey by Microsoft](https://docs.microsoft.com/en-us/previous-versions/msp-n-p/jj554200(v=pandp.10))
- [Event Sourcing](https://martinfowler.com/eaaDev/EventSourcing.html)