# Backend Architecture Refactor Plan

## Executive Summary
This document outlines a comprehensive refactor plan to transform the Brain2 backend into a best-practices exemplar using hexagonal architecture, domain-driven design, and modern Go patterns.

## Current State Analysis

### Strengths
1. **Wire-based DI**: Already using compile-time dependency injection
2. **Domain Models**: Rich domain models with value objects (Node, Edge, Category)
3. **CQRS Foundation**: Separate read/write interfaces in repositories
4. **Clean Architecture**: Good separation between layers
5. **Observability**: OpenTelemetry integration for tracing

### Areas for Improvement
1. **God Container Anti-pattern**: `Container` struct has too many responsibilities
2. **Mixed Concerns**: Infrastructure mixed with business logic in places
3. **Incomplete CQRS**: Read/write separation not fully implemented
4. **Limited Event Sourcing**: Domain events exist but not fully leveraged
5. **Missing Patterns**: No specification pattern, saga orchestration, or outbox pattern
6. **Test Coverage**: Limited unit and integration test infrastructure

## Target Architecture

### Core Principles
- **Hexagonal Architecture**: Ports and adapters for complete decoupling
- **Domain-Driven Design**: Rich domain models with business logic
- **CQRS + Event Sourcing**: Complete read/write separation with event store
- **Microkernel Pattern**: Plugin architecture for extensibility
- **Clean Code**: SOLID principles throughout

### Layer Structure

```
backend/
├── domain/                 # Core business logic (no dependencies)
│   ├── aggregates/        # Aggregate roots
│   ├── entities/          # Business entities
│   ├── valueobjects/      # Value objects
│   ├── events/            # Domain events
│   ├── specifications/    # Business rules
│   └── services/          # Domain services
│
├── application/           # Use case orchestration
│   ├── commands/         # Write operations (CQRS)
│   ├── queries/          # Read operations (CQRS)
│   ├── sagas/           # Distributed transaction orchestration
│   ├── handlers/        # Command/Query handlers
│   └── ports/           # Application port interfaces
│
├── infrastructure/       # External adapters
│   ├── persistence/     # Database adapters
│   ├── messaging/       # Event bus, message queue
│   ├── web/            # HTTP/WebSocket adapters
│   ├── cache/          # Caching strategies
│   └── monitoring/     # Metrics, logging, tracing
│
└── interfaces/          # Primary adapters
    ├── http/           # REST API
    ├── grpc/           # gRPC API
    ├── graphql/        # GraphQL API
    └── cli/            # Command-line interface
```

## Implementation Phases

### Phase 1: Core Domain Refactor
**Goal**: Establish pure domain layer with zero external dependencies

#### 1.1 Domain Aggregates
```go
// domain/aggregates/node/aggregate.go
package node

type Aggregate struct {
    root       *Node
    events     []domain.Event
    version    int64
    uncommitted []domain.Event
}

func (a *Aggregate) Apply(event domain.Event) error {
    switch e := event.(type) {
    case *NodeCreatedEvent:
        return a.applyNodeCreated(e)
    case *NodeUpdatedEvent:
        return a.applyNodeUpdated(e)
    }
    return nil
}
```

#### 1.2 Specification Pattern
```go
// domain/specifications/node_specifications.go
package specifications

type Specification[T any] interface {
    IsSatisfiedBy(T) bool
    And(Specification[T]) Specification[T]
    Or(Specification[T]) Specification[T]
    Not() Specification[T]
}

type ActiveNodeSpec struct{}

func (s ActiveNodeSpec) IsSatisfiedBy(n *node.Node) bool {
    return !n.IsArchived() && n.IsValid()
}
```

#### 1.3 Domain Services
```go
// domain/services/connection_analyzer.go
package services

type ConnectionAnalyzer struct {
    similarity SimilarityCalculator
    ranker     ConnectionRanker
}

func (a *ConnectionAnalyzer) FindConnections(
    ctx context.Context,
    source *node.Node,
    candidates []*node.Node,
    opts ...AnalysisOption,
) ([]*Connection, error) {
    // Pure domain logic, no infrastructure dependencies
}
```

### Phase 2: CQRS Implementation
**Goal**: Complete separation of read and write models

#### 2.1 Command Bus
```go
// application/commands/bus.go
package commands

type CommandBus interface {
    Register(cmdType reflect.Type, handler CommandHandler)
    Send(ctx context.Context, cmd Command) error
}

type CommandHandler interface {
    Handle(ctx context.Context, cmd Command) error
}
```

#### 2.2 Query Bus
```go
// application/queries/bus.go
package queries

type QueryBus interface {
    Register(queryType reflect.Type, handler QueryHandler)
    Send(ctx context.Context, query Query) (interface{}, error)
}

type QueryHandler interface {
    Handle(ctx context.Context, query Query) (interface{}, error)
}
```

#### 2.3 Read Model Projections
```go
// application/projections/node_projection.go
package projections

type NodeProjection struct {
    store ReadModelStore
}

func (p *NodeProjection) Handle(event domain.Event) error {
    switch e := event.(type) {
    case *NodeCreatedEvent:
        return p.store.Create(NodeReadModel{
            ID:      e.NodeID,
            Content: e.Content,
            // Denormalized for query performance
            ConnectionCount: 0,
            CategoryNames:   []string{},
        })
    }
}
```

### Phase 3: Event Sourcing & Saga Pattern
**Goal**: Event-driven architecture with saga orchestration

#### 3.1 Event Store
```go
// infrastructure/eventstore/store.go
package eventstore

type EventStore interface {
    Save(aggregateID string, events []domain.Event, expectedVersion int64) error
    Load(aggregateID string) ([]domain.Event, error)
    LoadSnapshot(aggregateID string) (*Snapshot, error)
    SaveSnapshot(snapshot *Snapshot) error
}
```

#### 3.2 Saga Orchestrator
```go
// application/sagas/create_node_saga.go
package sagas

type CreateNodeSaga struct {
    *BaseSaga
    steps []SagaStep
}

func (s *CreateNodeSaga) Execute(ctx context.Context) error {
    for _, step := range s.steps {
        if err := step.Execute(ctx); err != nil {
            return s.compensate(ctx, step)
        }
    }
    return nil
}
```

#### 3.3 Outbox Pattern
```go
// infrastructure/persistence/outbox.go
package persistence

type OutboxStore struct {
    db *sql.DB
}

func (s *OutboxStore) SaveWithEvents(
    ctx context.Context,
    aggregate domain.Aggregate,
    events []domain.Event,
) error {
    tx, _ := s.db.BeginTx(ctx, nil)
    defer tx.Rollback()
    
    // Save aggregate and events in same transaction
    if err := s.saveAggregate(tx, aggregate); err != nil {
        return err
    }
    
    if err := s.saveOutboxEvents(tx, events); err != nil {
        return err
    }
    
    return tx.Commit()
}
```

### Phase 4: Advanced Patterns
**Goal**: Production-ready resilience and performance

#### 4.1 Circuit Breaker
```go
// infrastructure/resilience/circuit_breaker.go
package resilience

type CircuitBreaker struct {
    maxFailures      int
    resetTimeout     time.Duration
    state           State
    failures        int
    lastFailureTime time.Time
}

func (cb *CircuitBreaker) Execute(fn func() error) error {
    if cb.state == Open {
        if time.Since(cb.lastFailureTime) > cb.resetTimeout {
            cb.state = HalfOpen
        } else {
            return ErrCircuitOpen
        }
    }
    
    err := fn()
    cb.recordResult(err)
    return err
}
```

#### 4.2 Bulkhead Pattern
```go
// infrastructure/resilience/bulkhead.go
package resilience

type Bulkhead struct {
    maxConcurrent int
    semaphore    chan struct{}
}

func (b *Bulkhead) Execute(ctx context.Context, fn func() error) error {
    select {
    case b.semaphore <- struct{}{}:
        defer func() { <-b.semaphore }()
        return fn()
    case <-ctx.Done():
        return ctx.Err()
    }
}
```

#### 4.3 Rate Limiting
```go
// infrastructure/middleware/rate_limiter.go
package middleware

type RateLimiter struct {
    store   RateLimitStore
    limiter *rate.Limiter
}

func (rl *RateLimiter) Middleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        key := rl.extractKey(r)
        
        if !rl.limiter.Allow() {
            http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)
            return
        }
        
        next.ServeHTTP(w, r)
    })
}
```

### Phase 5: Testing Infrastructure
**Goal**: Comprehensive testing with high coverage

#### 5.1 Test Fixtures
```go
// tests/fixtures/node_fixtures.go
package fixtures

type NodeFixture struct {
    Builder *NodeBuilder
}

func (f *NodeFixture) ValidNode() *node.Node {
    return f.Builder.
        WithContent("Test content").
        WithTags("test", "fixture").
        Build()
}
```

#### 5.2 BDD Tests
```go
// tests/features/node_test.go
package features

func TestNodeCreation(t *testing.T) {
    given := NewScenario(t)
    
    given.
        UserExists("user-123").
        When().
        CreateNode(CreateNodeCommand{
            UserID:  "user-123",
            Content: "Test node",
        }).
        Then().
        NodeShouldExist().
        WithContent("Test node").
        EventShouldBePublished("NodeCreated")
}
```

#### 5.3 Contract Testing
```go
// tests/contracts/repository_contract.go
package contracts

type RepositoryContract struct {
    Suite func(repo repository.NodeRepository) []Test
}

func NodeRepositoryContract() RepositoryContract {
    return RepositoryContract{
        Suite: func(repo repository.NodeRepository) []Test {
            return []Test{
                {"should save and retrieve node", testSaveAndRetrieve},
                {"should handle concurrent updates", testConcurrency},
                {"should respect specifications", testSpecifications},
            }
        },
    }
}
```

## Migration Strategy

### Step 1: Parallel Development
- Create new structure alongside existing code
- Use feature flags to switch between implementations
- Gradually migrate endpoints

### Step 2: Strangler Fig Pattern
- Route new features to refactored code
- Gradually replace old implementations
- Maintain backward compatibility

### Step 3: Data Migration
- Implement dual-write strategy
- Migrate historical data
- Verify data integrity

### Step 4: Cleanup
- Remove old implementations
- Update documentation
- Performance optimization

## Success Metrics

### Code Quality
- **Test Coverage**: >80% unit, >60% integration
- **Cyclomatic Complexity**: <10 per function
- **Code Duplication**: <3%
- **Technical Debt Ratio**: <5%

### Performance
- **API Latency**: p99 <100ms
- **Throughput**: >10,000 req/s
- **Cold Start**: <500ms
- **Memory Usage**: <256MB per Lambda

### Maintainability
- **Mean Time to Feature**: Reduced by 50%
- **Bug Rate**: Reduced by 70%
- **Code Review Time**: Reduced by 40%
- **Onboarding Time**: Reduced to 1 week

## Risk Mitigation

### Technical Risks
1. **Breaking Changes**: Use API versioning
2. **Data Loss**: Implement backup/restore
3. **Performance Regression**: Continuous benchmarking
4. **Complexity**: Incremental migration

### Organizational Risks
1. **Knowledge Gap**: Team training sessions
2. **Timeline Pressure**: Phased approach
3. **Stakeholder Buy-in**: Regular demos

## Timeline

- **Phase 1**: 2 weeks - Core domain refactor
- **Phase 2**: 2 weeks - CQRS implementation
- **Phase 3**: 3 weeks - Event sourcing & sagas
- **Phase 4**: 2 weeks - Advanced patterns
- **Phase 5**: 1 week - Testing infrastructure
- **Migration**: 4 weeks - Gradual cutover
- **Total**: 14 weeks

## Next Steps

1. Review and approve plan
2. Set up parallel development environment
3. Begin Phase 1 implementation
4. Establish monitoring and metrics
5. Schedule regular architecture reviews