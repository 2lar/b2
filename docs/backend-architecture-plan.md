# Backend Architecture Rewrite Plan

## Executive Summary

This document outlines a comprehensive architectural plan for rewriting the Brain2 backend following industry best practices and modern design patterns. The goal is to create a scalable, maintainable, and testable system that can evolve with business requirements while maintaining high performance and reliability.

## Current State Analysis

The existing backend already implements several good patterns:
- Domain-Driven Design (DDD) principles
- Clean Architecture with layered approach
- CQRS pattern for command/query separation
- Event-driven architecture with EventBridge
- Repository pattern for data access
- Dependency injection with Wire

However, there are opportunities for improvement:
- Domain models lack richness and business logic encapsulation
- Inconsistent error handling across layers
- Limited use of value objects and domain events
- Missing comprehensive testing patterns
- Lack of proper bounded contexts definition

## Architectural Principles

### 1. Domain-Driven Design (DDD)
- **Rich Domain Models**: Entities with encapsulated business logic
- **Value Objects**: Immutable types for domain concepts
- **Aggregates**: Consistency boundaries with aggregate roots
- **Domain Events**: First-class citizens for state changes
- **Ubiquitous Language**: Shared vocabulary between business and code

### 2. Clean Architecture (Hexagonal)
- **Dependency Inversion**: Domain layer has no external dependencies
- **Ports & Adapters**: Clear interfaces between layers
- **Use Cases**: Application services orchestrating domain logic
- **Infrastructure Independence**: Swappable implementations

### 3. CQRS & Event Sourcing
- **Command Model**: Optimized for writes with domain validation
- **Query Model**: Denormalized views for read performance
- **Event Store**: Immutable audit log of all state changes
- **Projections**: Materialized views from event streams

## Core Components

### 1. Domain Layer
```
domain/
├── core/
│   ├── entities/
│   │   ├── node.go          # Node aggregate root
│   │   ├── edge.go          # Edge entity
│   │   └── graph.go         # Graph aggregate
│   ├── valueobjects/
│   │   ├── node_id.go       # Strongly-typed ID
│   │   ├── node_content.go  # Rich content type
│   │   ├── coordinates.go   # Position value object
│   │   └── metadata.go      # Structured metadata
│   ├── events/
│   │   ├── node_created.go
│   │   ├── edge_connected.go
│   │   └── graph_updated.go
│   └── services/
│       ├── graph_analyzer.go  # Domain service
│       └── connection_validator.go
```

### 2. Application Layer
```
application/
├── commands/
│   ├── handlers/
│   │   ├── create_node.go
│   │   ├── update_node.go
│   │   └── connect_nodes.go
│   ├── validators/
│   │   └── command_validator.go
│   └── bus/
│       └── command_bus.go
├── queries/
│   ├── handlers/
│   │   ├── get_graph.go
│   │   ├── search_nodes.go
│   │   └── analyze_connections.go
│   └── projections/
│       ├── graph_projection.go
│       └── search_projection.go
├── sagas/
│   ├── graph_sync_saga.go
│   └── cleanup_saga.go
└── ports/
    ├── repositories.go
    ├── event_publisher.go
    └── cache.go
```

### 3. Infrastructure Layer
```
infrastructure/
├── persistence/
│   ├── dynamodb/
│   │   ├── node_repository.go
│   │   ├── edge_repository.go
│   │   └── event_store.go
│   ├── cache/
│   │   ├── redis_cache.go
│   │   └── memory_cache.go
│   └── search/
│       └── elasticsearch.go
├── messaging/
│   ├── eventbridge/
│   │   └── event_publisher.go
│   ├── sqs/
│   │   └── command_queue.go
│   └── sns/
│       └── notification_service.go
├── observability/
│   ├── metrics/
│   │   └── cloudwatch.go
│   ├── tracing/
│   │   └── xray.go
│   └── logging/
│       └── structured_logger.go
└── http/
    ├── handlers/
    ├── middleware/
    └── validators/
```

### 4. Interfaces Layer
```
interfaces/
├── http/
│   ├── rest/
│   │   ├── routes.go
│   │   ├── handlers/
│   │   └── middleware/
│   ├── graphql/
│   │   ├── schema.go
│   │   └── resolvers/
│   └── websocket/
│       ├── hub.go
│       └── handlers/
├── grpc/
│   ├── services/
│   └── interceptors/
└── cli/
    └── commands/
```

## Key Design Patterns

### 1. Repository Pattern with Specification
```go
type NodeRepository interface {
    Find(ctx context.Context, spec Specification) ([]*Node, error)
    FindByID(ctx context.Context, id NodeID) (*Node, error)
    Save(ctx context.Context, node *Node) error
    Delete(ctx context.Context, id NodeID) error
}

type Specification interface {
    IsSatisfiedBy(node *Node) bool
    ToSQL() string
    ToDynamoDBExpression() expression.Expression
}
```

### 2. Unit of Work Pattern
```go
type UnitOfWork interface {
    Begin(ctx context.Context) error
    Commit(ctx context.Context) error
    Rollback(ctx context.Context) error
    NodeRepository() NodeRepository
    EdgeRepository() EdgeRepository
    EventStore() EventStore
}
```

### 3. Domain Events with Event Bus
```go
type DomainEvent interface {
    AggregateID() string
    EventType() string
    EventVersion() int
    OccurredAt() time.Time
}

type EventBus interface {
    Publish(ctx context.Context, events ...DomainEvent) error
    Subscribe(eventType string, handler EventHandler) error
}
```

### 4. Command and Query Handlers
```go
type CommandHandler[T any] interface {
    Handle(ctx context.Context, cmd T) error
}

type QueryHandler[Q any, R any] interface {
    Handle(ctx context.Context, query Q) (R, error)
}
```

### 5. Saga Pattern for Distributed Transactions
```go
type Saga interface {
    Start(ctx context.Context) error
    Compensate(ctx context.Context) error
    GetSteps() []SagaStep
}
```

## Testing Strategy

### 1. Unit Testing
- **Domain Logic**: Pure functions with no external dependencies
- **Value Objects**: Property-based testing with quickcheck
- **Entities**: Behavior testing with mocks for dependencies

### 2. Integration Testing
- **Repository Tests**: Against real DynamoDB Local
- **API Tests**: Full HTTP request/response cycle
- **Event Tests**: Message publishing and consumption

### 3. End-to-End Testing
- **User Journeys**: Complete workflows from API to database
- **Performance Tests**: Load testing with k6 or Gatling
- **Chaos Engineering**: Failure injection and recovery

### 4. Testing Patterns
```go
// Builder Pattern for Test Data
func NewNodeBuilder() *NodeBuilder {
    return &NodeBuilder{
        node: &Node{
            ID: NewNodeID(),
            CreatedAt: time.Now(),
        },
    }
}

// Mother Object Pattern
func MotherNode() *Node {
    return NewNodeBuilder().
        WithTitle("Test Node").
        WithContent("Test content").
        Build()
}

// Test Fixtures
type NodeFixture struct {
    ValidNode   *Node
    InvalidNode *Node
    DeletedNode *Node
}
```

## Infrastructure Patterns

### 1. Circuit Breaker
```go
type CircuitBreaker interface {
    Execute(fn func() error) error
    GetState() State
}
```

### 2. Retry with Exponential Backoff
```go
type RetryPolicy struct {
    MaxAttempts int
    InitialDelay time.Duration
    MaxDelay time.Duration
    Multiplier float64
}
```

### 3. Bulkhead Pattern
```go
type Bulkhead interface {
    Execute(ctx context.Context, fn func() error) error
    GetAvailableSlots() int
}
```

### 4. Rate Limiting
```go
type RateLimiter interface {
    Allow(key string) bool
    AllowN(key string, n int) bool
}
```

## API Design

### 1. RESTful Endpoints
```
POST   /api/v2/nodes           # Create node
GET    /api/v2/nodes/{id}      # Get node
PUT    /api/v2/nodes/{id}      # Update node
DELETE /api/v2/nodes/{id}      # Delete node
GET    /api/v2/graphs/{id}     # Get graph
POST   /api/v2/edges           # Create edge
GET    /api/v2/search          # Search nodes
```

### 2. GraphQL Schema
```graphql
type Node {
    id: ID!
    title: String!
    content: String
    position: Position!
    edges: [Edge!]!
    createdAt: DateTime!
    updatedAt: DateTime!
}

type Query {
    node(id: ID!): Node
    graph(userId: ID!): Graph
    searchNodes(query: String!): [Node!]!
}

type Mutation {
    createNode(input: CreateNodeInput!): Node!
    updateNode(id: ID!, input: UpdateNodeInput!): Node!
    connectNodes(source: ID!, target: ID!): Edge!
}
```

### 3. WebSocket Events
```typescript
interface GraphUpdate {
    type: 'NODE_CREATED' | 'NODE_UPDATED' | 'EDGE_CREATED';
    payload: any;
    timestamp: string;
}
```

## Observability

### 1. Structured Logging
```go
logger.Info("Node created",
    zap.String("node_id", node.ID.String()),
    zap.String("user_id", userID),
    zap.Duration("duration", elapsed),
)
```

### 2. Distributed Tracing
```go
span, ctx := tracer.StartSpan(ctx, "CreateNode",
    trace.WithAttributes(
        attribute.String("node.id", nodeID),
        attribute.String("user.id", userID),
    ),
)
defer span.End()
```

### 3. Metrics and Monitoring
```go
metrics.RecordLatency("node.create", elapsed)
metrics.IncrementCounter("node.created")
metrics.UpdateGauge("graph.nodes", nodeCount)
```

### 4. Health Checks
```go
type HealthChecker interface {
    Check(ctx context.Context) HealthStatus
}

type HealthStatus struct {
    Status string
    Checks map[string]CheckResult
}
```

## Security Considerations

### 1. Authentication & Authorization
- JWT tokens with refresh mechanism
- Role-Based Access Control (RBAC)
- API key management for service-to-service

### 2. Data Protection
- Encryption at rest and in transit
- Field-level encryption for sensitive data
- PII data masking in logs

### 3. Input Validation
- Request validation at API gateway
- Domain validation in command handlers
- SQL injection prevention

### 4. Rate Limiting & DDoS Protection
- API Gateway throttling
- WAF rules
- Distributed rate limiting

## Performance Optimizations

### 1. Caching Strategy
- **L1 Cache**: In-memory for hot data
- **L2 Cache**: Redis for shared cache
- **L3 Cache**: CDN for static content

### 2. Database Optimizations
- **Read Replicas**: For query separation
- **Sharding**: For horizontal scaling
- **Indexing**: Composite indexes for queries

### 3. Async Processing
- **Command Queue**: SQS for write operations
- **Event Streaming**: Kinesis for real-time updates
- **Batch Processing**: Step Functions for workflows

## Migration Strategy

### Phase 1: Foundation (Weeks 1-2)
- Set up project structure
- Define domain models and value objects
- Implement core domain logic
- Create repository interfaces

### Phase 2: Application Layer (Weeks 3-4)
- Implement command handlers
- Create query handlers
- Set up event bus
- Add validation layer

### Phase 3: Infrastructure (Weeks 5-6)
- DynamoDB repositories
- EventBridge integration
- Caching layer
- API handlers

### Phase 4: Testing & Observability (Week 7)
- Unit and integration tests
- Performance testing
- Monitoring setup
- Documentation

### Phase 5: Migration & Deployment (Week 8)
- Data migration scripts
- Blue-green deployment
- Feature toggles
- Rollback procedures

## Success Metrics

### Technical Metrics
- **Latency**: P99 < 200ms for reads, < 500ms for writes
- **Availability**: 99.9% uptime
- **Error Rate**: < 0.1% of requests
- **Test Coverage**: > 80% code coverage

### Business Metrics
- **Time to Market**: 50% faster feature delivery
- **Maintenance Cost**: 30% reduction in bug fixes
- **Developer Productivity**: 2x increase in velocity
- **System Scalability**: Support 10x user growth

## Risks and Mitigations

### Risk 1: Over-Engineering
**Mitigation**: Start with core features, iterate based on needs

### Risk 2: Migration Complexity
**Mitigation**: Parallel run with gradual cutover

### Risk 3: Learning Curve
**Mitigation**: Team training and pair programming

### Risk 4: Performance Regression
**Mitigation**: Continuous benchmarking and monitoring

## Next Steps

1. Review and approve architecture plan
2. Set up development environment
3. Create proof of concept for core domain
4. Define API contracts
5. Begin incremental implementation

## Conclusion

This architecture plan provides a solid foundation for building a scalable, maintainable backend system. By following DDD principles, Clean Architecture, and modern cloud patterns, we ensure the system can evolve with changing business requirements while maintaining high quality and performance standards.