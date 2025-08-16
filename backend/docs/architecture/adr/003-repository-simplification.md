# ADR-003: Repository Interface Simplification

## Status
Accepted

## Context
The repository layer had become overly complex with:
- **37+ methods** in NodeRepository interface
- Multiple overlapping query methods
- Inconsistent method naming
- Complex factory patterns with excessive configuration
- Tight coupling to AWS DynamoDB implementation

This complexity led to:
- Difficult maintenance and testing
- Confusion about which method to use
- Increased cognitive load
- Hard to mock for testing
- Vendor lock-in concerns

## Decision
Simplify repository interfaces following these principles:

### 1. Generic Base Repository
```go
type Repository[T any, ID comparable] interface {
    Find(ctx context.Context, id ID) (*T, error)
    Save(ctx context.Context, entity *T) error
    Delete(ctx context.Context, id ID) error
    Exists(ctx context.Context, id ID) (bool, error)
}
```

### 2. Specification Pattern for Queries
```go
type Specification interface {
    IsSatisfiedBy(entity interface{}) bool
    And(other Specification) Specification
    Or(other Specification) Specification
    Not() Specification
}
```

### 3. Simplified Domain Repositories
```go
type SimpleNodeRepository interface {
    // Only essential operations
    FindNode(ctx context.Context, userID, nodeID string) (*domain.Node, error)
    SaveNode(ctx context.Context, node *domain.Node) error
    DeleteNode(ctx context.Context, userID, nodeID string) error
    
    // Specification-based queries
    FindNodesByUser(ctx context.Context, userID string, spec Specification) ([]*domain.Node, error)
    
    // One pagination method
    GetNodesPage(ctx context.Context, userID string, page PageRequest) (*Page[domain.Node], error)
}
```

## Consequences

### Positive:
- Reduced interface complexity (from 37+ to ~10 methods)
- Consistent patterns across all repositories
- Easier to test and mock
- Better separation of concerns
- Flexible querying through specifications

### Negative:
- Breaking changes to existing code
- Need for migration adapters
- Learning curve for specification pattern
- Initial implementation overhead

### Neutral:
- Different approach from traditional repository pattern
- Requires careful specification design

## Implementation Notes

### Before (Complex):
```go
type NodeRepository interface {
    CreateNodeAndKeywords(ctx context.Context, node *domain.Node) error
    FindNodeByID(ctx context.Context, userID, nodeID string) (*domain.Node, error)
    FindNodes(ctx context.Context, query NodeQuery) ([]*domain.Node, error)
    FindNodesByKeywords(ctx context.Context, userID string, keywords []string) ([]*domain.Node, error)
    FindNodesByTags(ctx context.Context, userID string, tags []string) ([]*domain.Node, error)
    FindNodesByContent(ctx context.Context, userID string, content string) ([]*domain.Node, error)
    FindRecentNodes(ctx context.Context, userID string, days int) ([]*domain.Node, error)
    GetNodesPage(ctx context.Context, query NodeQuery, pagination Pagination) (*NodePage, error)
    GetNodeNeighborhood(ctx context.Context, userID, nodeID string, depth int) (*Graph, error)
    CountNodes(ctx context.Context, userID string) (int, error)
    // ... 27+ more methods
}
```

### After (Simplified):
```go
type SimpleNodeRepository interface {
    // Core CRUD - 4 methods
    FindNode(ctx context.Context, userID, nodeID string) (*domain.Node, error)
    SaveNode(ctx context.Context, node *domain.Node) error
    DeleteNode(ctx context.Context, userID, nodeID string) error
    
    // Flexible queries - 3 methods
    FindNodesByUser(ctx context.Context, userID string, spec Specification) ([]*domain.Node, error)
    CountNodes(ctx context.Context, userID string) (int, error)
    GetNodesPage(ctx context.Context, userID string, page PageRequest) (*Page[domain.Node], error)
}
```

### Specification Examples:

```go
// Combine specifications for complex queries
keywordSpec := NewKeywordSpecification("golang", "backend")
recentSpec := NewDateRangeSpecification(time.Now().AddDate(0, 0, -7), time.Now())
combinedSpec := keywordSpec.And(recentSpec)

// Use in repository
nodes, err := repo.FindNodesByUser(ctx, userID, combinedSpec)
```

### Migration Strategy:

1. **Create Adapters** (Completed):
```go
type NodeRepositoryAdapter struct {
    simple SimpleNodeRepository
}

func (a *NodeRepositoryAdapter) FindNodesByKeywords(...) {
    spec := NewKeywordSpecification(keywords...)
    return a.simple.FindNodesByUser(ctx, userID, spec)
}
```

2. **Gradual Migration**:
- New features use simplified interfaces
- Existing code uses adapters
- Migrate old code incrementally

3. **Remove Legacy Interfaces**:
- After full migration
- Remove adapter layer
- Clean up old repository implementations

## Performance Considerations

### Query Optimization:
- Specifications can be translated to efficient database queries
- Allows for query optimization at the infrastructure layer
- Caching can be applied at specification level

### Example Translation:
```go
// Specification to DynamoDB query
func (r *DynamoDBRepository) translateSpec(spec Specification) *dynamodb.QueryInput {
    switch s := spec.(type) {
    case *KeywordSpecification:
        return r.buildKeywordQuery(s.Keywords)
    case *DateRangeSpecification:
        return r.buildDateRangeQuery(s.Start, s.End)
    case *AndSpecification:
        return r.combineQueries(r.translateSpec(s.Left), r.translateSpec(s.Right))
    }
}
```

## Future Enhancements

1. **Query Builder Pattern**:
```go
query := NewQueryBuilder().
    WithKeywords("golang", "backend").
    WithDateRange(start, end).
    WithPagination(1, 20).
    Build()
```

2. **Reactive Repositories**:
```go
type ReactiveRepository[T any] interface {
    FindStream(ctx context.Context, spec Specification) (<-chan T, error)
    Watch(ctx context.Context, spec Specification) (<-chan Event[T], error)
}
```

3. **Batch Operations**:
```go
type BatchRepository[T any] interface {
    SaveBatch(ctx context.Context, entities []*T) error
    DeleteBatch(ctx context.Context, ids []ID) error
}
```

## References
- [Specification Pattern by Eric Evans](https://www.martinfowler.com/apsupp/spec.pdf)
- [Repository Pattern](https://martinfowler.com/eaaCatalog/repository.html)
- [Generic Repository Anti-Pattern](https://rob.conery.io/2014/03/04/repositories-and-unitofwork-are-not-a-good-idea/)