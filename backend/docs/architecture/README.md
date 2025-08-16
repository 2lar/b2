# Brain2 Backend Architecture

## Overview

The Brain2 backend follows **Clean Architecture** principles with **CQRS** pattern implementation. This architecture ensures:
- Clear separation of concerns
- Technology independence
- High testability
- Scalability
- Maintainability

## Architecture Layers

```
┌─────────────────────────────────────────────────────────┐
│                     External Clients                     │
│                  (Web, Mobile, API, CLI)                 │
└────────────────────┬────────────────────────────────────┘
                     │
┌────────────────────▼────────────────────────────────────┐
│                  Interface Layer                         │
│              (HTTP Handlers, DTOs, Auth)                 │
├──────────────────────────────────────────────────────────┤
│                 Application Layer                        │
│         (Use Cases, Services, CQRS Handlers)            │
├──────────────────────────────────────────────────────────┤
│                   Domain Layer                           │
│      (Entities, Value Objects, Domain Services)          │
├──────────────────────────────────────────────────────────┤
│               Infrastructure Layer                       │
│     (Repositories, External Services, Database)          │
└──────────────────────────────────────────────────────────┘
```

### 1. Domain Layer (`internal/domain/`)
The heart of the application containing business logic and rules.

**Components:**
- **Entities**: Core business objects (Node, Edge, Category)
- **Value Objects**: Immutable domain concepts (NodeID, UserID, Content)
- **Domain Services**: Business logic that spans multiple entities
- **Domain Events**: Business events (NodeCreated, EdgeDeleted)

**Example:**
```go
// Domain entity with rich behavior
type Node struct {
    id       NodeID
    userID   UserID
    content  Content
    keywords Keywords
    tags     Tags
    version  int
}

// Business logic in the domain
func (n *Node) UpdateContent(content string) error {
    if err := n.validateContent(content); err != nil {
        return err
    }
    n.content = NewContent(content)
    n.version++
    n.RaiseEvent(ContentUpdated{NodeID: n.id})
    return nil
}
```

### 2. Application Layer (`internal/application/`)
Orchestrates use cases and coordinates domain objects.

**Components:**
- **Command Handlers**: Handle write operations
- **Query Handlers**: Handle read operations
- **Application Services**: Coordinate complex operations
- **DTOs**: Data transfer objects for layer communication

**CQRS Implementation:**
```go
// Write side - Commands
type CreateNodeCommand struct {
    UserID  string
    Content string
    Tags    []string
}

// Read side - Queries
type GetNodeQuery struct {
    UserID             string
    NodeID             string
    IncludeConnections bool
}
```

### 3. Interface Layer (`internal/interfaces/`)
Handles external communication and adapts to specific protocols.

**Structure:**
```
interfaces/
├── http/
│   ├── handlers/     # HTTP request handlers
│   ├── dto/          # HTTP-specific DTOs
│   ├── middleware/   # Auth, logging, etc.
│   ├── validation/   # Request validation
│   └── errors/       # HTTP error handling
└── grpc/            # Future gRPC support
```

**Clean Handler Example:**
```go
func (h *NodeHandler) CreateNode(w http.ResponseWriter, r *http.Request) {
    // 1. Parse HTTP request
    var req CreateNodeRequest
    json.NewDecoder(r.Body).Decode(&req)
    
    // 2. Validate
    if err := h.validator.Validate(req); err != nil {
        httpErrors.WriteError(w, httpErrors.NewValidation(err))
        return
    }
    
    // 3. Convert to command
    cmd := commands.CreateNode{
        UserID:  r.Context().Value("userID").(string),
        Content: req.Content,
    }
    
    // 4. Execute use case
    result, err := h.commandBus.Execute(cmd)
    
    // 5. Return HTTP response
    response.WriteJSON(w, http.StatusCreated, result)
}
```

### 4. Infrastructure Layer (`internal/infrastructure/`)
Implements external dependencies and technical details.

**Components:**
- **Persistence**: DynamoDB repositories
- **External Services**: AWS EventBridge, S3
- **Decorators**: Logging, caching, metrics
- **Adapters**: Third-party service integrations

## Key Patterns

### 1. Repository Pattern
Abstracts data access with clean interfaces:

```go
// Simplified repository interface
type SimpleNodeRepository interface {
    FindNode(ctx context.Context, userID, nodeID string) (*domain.Node, error)
    SaveNode(ctx context.Context, node *domain.Node) error
    DeleteNode(ctx context.Context, userID, nodeID string) error
    FindNodesByUser(ctx context.Context, userID string, spec Specification) ([]*domain.Node, error)
}
```

### 2. Specification Pattern
Enables flexible queries without method explosion:

```go
// Combine specifications for complex queries
spec := NewKeywordSpec("golang").
    And(NewDateRangeSpec(lastWeek, today)).
    And(NewUserSpec(userID))

nodes, err := repo.FindNodes(ctx, spec)
```

### 3. Unit of Work Pattern
Ensures transactional consistency:

```go
err := txManager.Execute(ctx, func(tx Transaction) error {
    node := domain.NewNode(content)
    tx.Nodes().Save(ctx, node)
    
    edge := domain.NewEdge(node1ID, node2ID)
    tx.Edges().Create(ctx, edge)
    
    return nil // Commits on success, rolls back on error
})
```

### 4. Dependency Injection
All dependencies are injected using Google Wire:

```go
// Wire provider functions
func ProvideNodeService(
    repo repository.NodeRepository,
    bus events.EventBus,
    cache cache.Cache,
) *NodeService {
    return &NodeService{
        repo:  repo,
        bus:   bus,
        cache: cache,
    }
}
```

## Request Flow

1. **HTTP Request** → Interface Layer (Handler)
2. **Validation** → Convert to Command/Query
3. **Application Layer** → Execute use case
4. **Domain Layer** → Business logic
5. **Infrastructure Layer** → Persistence
6. **Response** → Convert to DTO → HTTP Response

## Testing Strategy

### 1. Unit Tests
- Domain logic tests (no dependencies)
- Pure business rule validation

### 2. Integration Tests
- Repository tests with test database
- Service tests with mocked dependencies

### 3. E2E Tests
- Full request/response cycle
- Real infrastructure (test environment)

### 4. Test Organization
```
tests/
├── unit/          # Domain logic tests
├── integration/   # Service and repository tests
├── e2e/          # End-to-end tests
└── fixtures/     # Test data and helpers
```

## Configuration

Environment-based configuration with validation:

```go
type Config struct {
    Environment string           `validate:"required,oneof=development staging production"`
    Database    DatabaseConfig   `validate:"required"`
    Cache       CacheConfig      `validate:"required"`
    Features    FeatureFlags     `validate:"required"`
}
```

## Performance Optimizations

### 1. Caching Strategy
- Read-through caching for queries
- Cache invalidation on writes
- TTL-based expiration

### 2. Database Optimization
- Efficient DynamoDB key design
- GSI for common query patterns
- Batch operations where possible

### 3. Async Processing
- Event-driven architecture
- Background job processing
- Message queuing for heavy operations

## Security Considerations

### 1. Authentication & Authorization
- JWT token validation
- User context injection
- Role-based access control

### 2. Input Validation
- Request validation at boundaries
- Domain validation in entities
- SQL injection prevention

### 3. Error Handling
- No sensitive data in errors
- Structured error responses
- Proper HTTP status codes

## Deployment

### 1. AWS Lambda Functions
- Separate functions per concern
- Shared layer for common code
- Environment-specific configurations

### 2. Container Support
- Docker support for local development
- ECS/Fargate deployment option
- Kubernetes ready

## Future Enhancements

### Short Term
- [ ] Complete CQRS separation
- [ ] Implement event sourcing
- [ ] Add GraphQL interface
- [ ] Enhance caching layer

### Long Term
- [ ] Microservices migration
- [ ] Real-time subscriptions
- [ ] ML-powered features
- [ ] Multi-region support

## References

- [Architecture Decision Records](./adr/) - Detailed architecture decisions
- [API Documentation](../api/) - OpenAPI specifications
- [Developer Guide](../developer-guide.md) - Getting started guide
- [Operations Guide](../operations/) - Deployment and monitoring

## Contributing

Please follow the architecture principles and patterns documented here. For significant changes, create an ADR documenting the decision.