# Backend2 - Clean Architecture Implementation

## Overview

Backend2 is a complete rewrite of the Brain2 backend following Domain-Driven Design (DDD) principles, Clean Architecture patterns, and CQRS for optimal maintainability, scalability, and developer experience.

## Architecture

### Layers

```
┌─────────────────────────────────────────────────┐
│                  Interfaces                      │
│         (HTTP, GraphQL, WebSocket, gRPC)        │
├─────────────────────────────────────────────────┤
│                 Application                      │
│    (Commands, Queries, Handlers, Use Cases)     │
├─────────────────────────────────────────────────┤
│                   Domain                         │
│  (Entities, Value Objects, Aggregates, Events)  │
├─────────────────────────────────────────────────┤
│               Infrastructure                     │
│   (Database, Messaging, Cache, External APIs)   │
└─────────────────────────────────────────────────┘
```

### Directory Structure

```
backend2/
├── domain/                 # Core business logic (no external dependencies)
│   ├── core/
│   │   ├── entities/      # Domain entities with business logic
│   │   ├── valueobjects/  # Immutable value objects
│   │   └── aggregates/    # Aggregate roots ensuring consistency
│   ├── events/            # Domain events
│   ├── services/          # Domain services
│   └── specifications/    # Business rule specifications
│
├── application/           # Application services and use cases
│   ├── commands/         # Command handlers (write operations)
│   │   ├── handlers/
│   │   ├── validators/
│   │   └── bus/
│   ├── queries/          # Query handlers (read operations)
│   │   ├── handlers/
│   │   ├── projections/
│   │   └── bus/
│   ├── ports/            # Interfaces for external dependencies
│   └── sagas/           # Long-running business processes
│
├── infrastructure/       # External service implementations
│   ├── persistence/     # Database repositories
│   │   ├── dynamodb/
│   │   ├── cache/
│   │   └── search/
│   ├── messaging/       # Event publishing/consuming
│   │   ├── eventbridge/
│   │   └── sqs/
│   └── observability/   # Logging, metrics, tracing
│
├── interfaces/          # API and presentation layer
│   ├── http/           # REST API
│   │   ├── rest/
│   │   ├── graphql/
│   │   └── websocket/
│   ├── grpc/           # gRPC services
│   └── cli/            # Command-line interface
│
├── pkg/                # Shared utilities
│   ├── errors/
│   ├── utils/
│   └── validation/
│
└── cmd/               # Application entry points
    ├── api/          # HTTP API server
    ├── worker/       # Background worker
    └── migrate/      # Database migrations
```

## Key Design Patterns

### 1. Domain-Driven Design (DDD)

- **Rich Domain Models**: Entities contain business logic, not just data
- **Value Objects**: Immutable objects representing domain concepts
- **Aggregates**: Consistency boundaries with aggregate roots
- **Domain Events**: First-class representation of business events
- **Ubiquitous Language**: Shared vocabulary between business and code

### 2. Clean Architecture (Hexagonal)

- **Dependency Inversion**: Domain layer has no external dependencies
- **Ports & Adapters**: Clear interfaces between layers
- **Use Cases**: Application services orchestrating domain logic
- **Infrastructure Independence**: Swappable implementations

### 3. CQRS (Command Query Responsibility Segregation)

- **Command Model**: Optimized for writes with domain validation
- **Query Model**: Denormalized views for read performance
- **Event Sourcing Ready**: Foundation for event-driven architecture
- **Separate Concerns**: Different models for different operations

## Core Components

### Domain Layer

The domain layer contains the core business logic:

- **Node Entity**: Rich domain model for knowledge nodes
- **Graph Aggregate**: Maintains consistency for the entire graph
- **Value Objects**: NodeID, Position, Content, etc.
- **Domain Events**: NodeCreated, NodesConnected, etc.

### Application Layer

The application layer orchestrates use cases:

- **Command Handlers**: Handle state-changing operations
- **Query Handlers**: Handle read operations
- **Command/Query Bus**: Dispatches commands/queries to handlers
- **Validation**: Input validation and business rule enforcement

### Infrastructure Layer

The infrastructure layer provides implementations:

- **Repositories**: DynamoDB implementations of domain repositories
- **Event Publisher**: EventBridge integration for domain events
- **Cache**: Redis/in-memory caching implementations
- **Observability**: Logging, metrics, and distributed tracing

## Getting Started

### Prerequisites

- Go 1.23+
- AWS CLI configured
- Docker (for local DynamoDB)

### Installation

```bash
# Clone the repository
git clone <repository-url>
cd backend2

# Install dependencies
go mod download

# Run tests
go test ./...

# Build the application
go build -o bin/api cmd/api/main.go

# Run the application
./bin/api
```

### Configuration

Environment variables:

```bash
SERVER_ADDRESS=:8080
AWS_REGION=us-east-1
DYNAMODB_TABLE=brain2-backend2
EVENT_BUS_NAME=brain2-events
LOG_LEVEL=info
ENVIRONMENT=development
```

## Development

### Adding a New Feature

1. **Define Domain Model**: Start with domain entities/value objects
2. **Create Command/Query**: Define the operation interface
3. **Implement Handler**: Write the business logic
4. **Add Repository Method**: If needed, extend repository interface
5. **Wire Dependencies**: Update dependency injection
6. **Add API Endpoint**: Expose via REST/GraphQL
7. **Write Tests**: Unit, integration, and e2e tests

### Testing Strategy

- **Unit Tests**: Test domain logic in isolation
- **Integration Tests**: Test repository implementations
- **E2E Tests**: Test complete user journeys
- **Contract Tests**: Ensure API compatibility

## Best Practices

1. **Keep Domain Pure**: No external dependencies in domain layer
2. **Use Value Objects**: For type safety and validation
3. **Aggregate Boundaries**: Maintain consistency within aggregates
4. **Event-Driven**: Use domain events for decoupling
5. **Fail Fast**: Validate early and return clear errors
6. **Immutability**: Prefer immutable objects where possible
7. **Dependency Injection**: Use interfaces, not concrete types

## Migration from Backend v1

The migration strategy involves:

1. **Parallel Operation**: Run both backends simultaneously
2. **Gradual Migration**: Move features incrementally
3. **Data Sync**: Keep data synchronized during transition
4. **Feature Flags**: Control rollout and rollback
5. **Monitoring**: Track both systems during migration

## Performance Optimizations

- **Read/Write Separation**: CQRS for optimized models
- **Caching Strategy**: Multi-level caching (L1: in-memory, L2: Redis)
- **Event Streaming**: Async processing for non-critical operations
- **Connection Pooling**: Efficient resource utilization
- **Query Optimization**: Denormalized projections for reads

## Security

- **Input Validation**: Comprehensive validation at boundaries
- **Authentication**: JWT-based authentication
- **Authorization**: Role-based access control (RBAC)
- **Encryption**: At rest and in transit
- **Audit Logging**: Complete audit trail via events

## Monitoring

- **Metrics**: Prometheus metrics for performance monitoring
- **Logging**: Structured logging with correlation IDs
- **Tracing**: Distributed tracing with OpenTelemetry
- **Health Checks**: Liveness and readiness probes
- **Alerts**: Automated alerting for critical issues

## Contributing

1. Follow the established patterns and architecture
2. Write tests for new functionality
3. Update documentation
4. Ensure code passes linting and formatting
5. Create focused, atomic commits

## License

[Your License Here]