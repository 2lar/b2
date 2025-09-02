# Brain2 Backend Testing Infrastructure

## Overview

This directory contains the comprehensive testing infrastructure for the Brain2 backend, implementing industry best practices for testing a production-ready system.

## Test Organization

```
tests/
├── fixtures/        # Test data builders and fixtures
├── features/        # BDD-style feature tests
├── contracts/       # Contract tests for interfaces
├── integration/     # Integration tests
├── performance/     # Performance benchmarks
├── utils/          # Testing utilities and helpers
├── mocks/          # Generated mocks
└── examples/       # Example tests and patterns
```

## Testing Layers

### 1. Unit Tests
- **Location**: Alongside source code (`*_test.go`)
- **Purpose**: Test individual functions and methods in isolation
- **Coverage Target**: >85%
- **Run**: `make test-unit`

### 2. Integration Tests
- **Location**: `tests/integration/`
- **Purpose**: Test component interactions with real dependencies
- **Coverage Target**: >70%
- **Run**: `make test-integration`

### 3. Contract Tests
- **Location**: `tests/contracts/`
- **Purpose**: Ensure all implementations follow the same interface contract
- **Run**: `make test-contracts`

### 4. BDD Tests
- **Location**: `tests/features/`
- **Purpose**: Test complete user journeys and business requirements
- **Run**: `make test-bdd`

### 5. Performance Tests
- **Location**: `tests/performance/`
- **Purpose**: Benchmark critical paths and identify bottlenecks
- **Run**: `make test-bench`

## Test Fixtures and Builders

### Node Builder Example

```go
import "brain2-backend/tests/fixtures/builders"

// Create a simple node
node := builders.NewNodeBuilder().
    WithUserID("user-123").
    WithContent("Test content").
    Build()

// Create a rich node with all fields
node := builders.NewNodeBuilder().
    WithUserID("user-123").
    WithTitle("Important Note").
    WithContent("Detailed content").
    WithTags("important", "work").
    WithKeywords("meeting", "planning").
    WithCategories("work-category").
    WithMetadata("priority", "high").
    Build()

// Create an archived node
node := builders.NewNodeBuilder().
    WithUserID("user-123").
    AsArchived().
    Build()
```

### Event Builder Example

```go
// Create a node created event
event := builders.NewEventBuilder().
    WithAggregateID("node-123").
    WithUserID("user-123").
    BuildNodeCreated("Content", "Title", []string{"tag"})

// Create an event sequence
events := builders.NewEventBuilderPresets().
    EventSequence("node-123", "user-123")
```

## BDD Testing Framework

### Writing BDD Tests

```go
func TestNodeCreationWorkflow(t *testing.T) {
    framework.NewScenario(t).
        Given().
            UserExists("user-123").
            And().CategoryExists("cat-456").
        When().
            CreatingNode(builders.NewNodeBuilder().
                WithUserID("user-123").
                WithContent("Important notes").
                WithCategories("cat-456").
                Build()).
        Then().
            NodeShouldExist().
            And().EventShouldBePublished("NodeCreated").
            And().ConnectionsShouldBeCreated(3).
            And().NoErrorShouldOccur()
}
```

### Available BDD Steps

#### Given (Preconditions)
- `UserExists(userID)` - Create a test user
- `NodeExists(nodeID, userID)` - Create a test node
- `NodesExist(count, userID)` - Create multiple nodes
- `CategoryExists(categoryID)` - Create a test category
- `SystemIsHealthy()` - Setup healthy system state

#### When (Actions)
- `CreatingNode(node)` - Create a new node
- `UpdatingNode(nodeID, updates)` - Update an existing node
- `DeletingNode(nodeID)` - Delete a node
- `ConnectingNodes(source, target)` - Create connection
- `ExecutingBulkOperation(op, nodeIDs)` - Bulk operations

#### Then (Assertions)
- `NodeShouldExist(nodeID)` - Assert node exists
- `NodeShouldNotExist(nodeID)` - Assert node doesn't exist
- `EventShouldBePublished(eventType)` - Assert event published
- `ConnectionsShouldBeCreated(count)` - Assert connections
- `NoErrorShouldOccur()` - Assert no errors
- `ErrorShouldContain(text)` - Assert error message

## Contract Testing

### Repository Contract Example

```go
func TestNodeRepositoryImplementations(t *testing.T) {
    implementations := map[string]ports.NodeRepository{
        "DynamoDB": dynamodbRepo,
        "InMemory": inMemoryRepo,
        "Mock":     mockRepo,
    }
    
    repository.RunContractTests(t, implementations)
}
```

Contract tests ensure all implementations:
- Support CRUD operations correctly
- Handle concurrent access
- Implement pagination properly
- Support specifications
- Handle optimistic locking
- Support batch operations

## Testing Utilities

### Event Collector

```go
collector := collectors.NewEventCollector(t)

// Collect events
collector.Collect(event1)
collector.CollectMany([]events.DomainEvent{event2, event3})

// Assert on events
collector.AssertEventPublished("NodeCreated")
collector.AssertEventCount(3)
collector.AssertEventSequence([]string{
    "NodeCreated",
    "NodeUpdated",
    "NodeConnected",
})

// Wait for events
collector.WaitForEvents(5, 1*time.Second)
collector.WaitForEventType("NodeCreated", 500*time.Millisecond)
```

### Context Helper

```go
helper := helpers.NewContextHelper(t)

// Create test context with timeout
ctx := helper.WithTestDeadline()

// Add test metadata
ctx = helper.WithUserID(ctx, "user-123")
ctx = helper.WithCorrelationID(ctx, "correlation-123")
ctx = helper.WithTestMetadata(ctx)

// Or use the convenience function
ctx = helpers.CreateTestContext(t, "user-123")
```

### Domain Assertions

```go
assertions := assertions.NewDomainAssertions(t)

// Assert node properties
assertions.AssertNodeEqual(expected, actual)
assertions.AssertNodeHasContent(node, "Expected content")
assertions.AssertNodeHasTags(node, []string{"tag1", "tag2"})
assertions.AssertNodeIsArchived(node)
assertions.AssertNodeVersion(node, 2)

// Assert value objects
assertions.AssertNodeIDValid(nodeID)
assertions.AssertContentValid(content)
assertions.AssertTagsValid(tags)
```

## Performance Benchmarks

### Running Benchmarks

```bash
# Run all benchmarks
make test-bench

# Run specific benchmark
go test -bench=BenchmarkNodeBuilder ./tests/performance/benchmarks/

# Run with memory profiling
go test -bench=. -benchmem ./tests/performance/benchmarks/

# Run for specific duration
go test -bench=. -benchtime=30s ./tests/performance/benchmarks/
```

### Benchmark Results

Benchmarks measure:
- Node creation performance
- Event building performance
- Saga execution overhead
- Bulk operation performance
- Memory allocations
- Concurrent operations

## Mock Generation

### Generating Mocks

```bash
# Generate all mocks
make mocks

# Generate specific interface
mockery --name=NodeRepository --dir=internal/core/application/ports --output=tests/mocks
```

### Using Mocks

```go
import "brain2-backend/tests/mocks"

// Create mock
mockRepo := new(mocks.NodeRepository)

// Set expectations
mockRepo.On("GetByID", ctx, "node-123").Return(node, nil)

// Use in test
service := NewNodeService(mockRepo)
result, err := service.GetNode(ctx, "node-123")

// Assert expectations
mockRepo.AssertExpectations(t)
```

## Integration Testing

### Setup

Integration tests require external dependencies:

```bash
# Start dependencies
make docker-up

# Run integration tests
make test-integration

# Stop dependencies
make docker-down
```

### Docker Compose Configuration

Create `tests/docker-compose.yml`:

```yaml
version: '3.8'
services:
  dynamodb-local:
    image: amazon/dynamodb-local
    ports:
      - "8000:8000"
    command: "-jar DynamoDBLocal.jar -inMemory"
  
  redis:
    image: redis:alpine
    ports:
      - "6379:6379"
```

## Test Execution

### Quick Commands

```bash
# Run all tests
make test

# Run unit tests only
make test-unit

# Run with coverage
make test-coverage

# Run with race detection
make test-race

# Run specific layer
make test-domain
make test-application
make test-infrastructure

# Clean test artifacts
make clean
```

### CI/CD Integration

```bash
# Run CI checks
make ci

# Full CD pipeline
make cd
```

## Best Practices

### 1. Test Naming
- Use descriptive names: `TestNodeCreation_WithValidData_ShouldSucceed`
- Group related tests: `TestNodeValidation_*`

### 2. Test Isolation
- Each test should be independent
- Use fresh fixtures for each test
- Clean up resources in defer statements

### 3. Assertion Messages
- Always provide context in assertions
- Use meaningful error messages
- Include relevant data in failures

### 4. Performance
- Keep unit tests fast (<100ms)
- Use t.Parallel() for independent tests
- Mock expensive operations

### 5. Coverage
- Aim for >85% unit test coverage
- Focus on business logic coverage
- Don't test generated code

## Troubleshooting

### Common Issues

1. **Tests timing out**
   - Check context deadlines
   - Verify mock expectations
   - Look for deadlocks

2. **Flaky tests**
   - Remove time dependencies
   - Use deterministic data
   - Avoid relying on goroutine scheduling

3. **Coverage gaps**
   - Run coverage report: `make test-coverage`
   - Focus on untested branches
   - Add edge case tests

## Contributing

When adding new features:
1. Write tests first (TDD)
2. Ensure contracts are satisfied
3. Add BDD scenarios for user journeys
4. Include benchmarks for performance-critical code
5. Update this documentation

## Resources

- [Go Testing](https://golang.org/pkg/testing/)
- [Testify](https://github.com/stretchr/testify)
- [Mockery](https://github.com/vektra/mockery)
- [BDD in Go](https://github.com/cucumber/godog)
- [Benchmarking Guide](https://dave.cheney.net/2013/06/30/how-to-write-benchmarks-in-go)