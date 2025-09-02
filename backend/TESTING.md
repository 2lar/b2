# Brain2 Backend Testing Guide

## Overview

The Brain2 backend implements a comprehensive testing strategy following the **Test Pyramid** principle with multiple levels of testing, each serving a specific purpose in ensuring code quality and system reliability.

## Table of Contents
- [Test Architecture](#test-architecture)
- [Test Categories](#test-categories)
- [Writing Tests](#writing-tests)
- [Test Organization](#test-organization)
- [Running Tests](#running-tests)
- [Mocking and Test Doubles](#mocking-and-test-doubles)
- [Test Fixtures and Builders](#test-fixtures-and-builders)
- [Best Practices](#best-practices)

## Test Architecture

### Test Pyramid

```
         /\
        /  \  E2E Tests (Manual)
       /    \
      /------\ BDD Tests
     /        \ Contract Tests
    /          \ Integration Tests
   /            \
  /--------------\ Unit Tests
```

- **Unit Tests (70%)**: Fast, isolated, business logic focused
- **Integration Tests (20%)**: Database, external services
- **Contract Tests (5%)**: Repository implementations
- **BDD Tests (4%)**: User scenarios, acceptance criteria
- **E2E Tests (1%)**: Full system validation (mostly manual)

### Test Philosophy

1. **Fast Feedback**: Unit tests run in milliseconds
2. **Isolation**: Tests don't depend on external services
3. **Deterministic**: Same input always produces same output
4. **Clear Failure Messages**: Know exactly what broke and why
5. **Documentation**: Tests serve as living documentation

## Test Categories

### 1. Unit Tests (`-tags=unit`)

**Purpose**: Test business logic in isolation

**Location**: Alongside production code
- `internal/core/domain/*_test.go`
- `internal/core/application/*_test.go`

**Example**:
```go
//go:build unit
// +build unit

package domain

import (
    "testing"
    "github.com/stretchr/testify/assert"
)

func TestNode_AddConnection(t *testing.T) {
    // Arrange
    node := NewNode("user-123", "content")
    targetNode := NewNode("user-123", "target")
    
    // Act
    err := node.AddConnection(targetNode.ID, 0.8)
    
    // Assert
    assert.NoError(t, err)
    assert.Len(t, node.Connections, 1)
    assert.Equal(t, 0.8, node.Connections[0].Weight)
}
```

### 2. Integration Tests (`-tags=integration`)

**Purpose**: Test interactions with external systems

**Location**: `tests/integration/`

**Requirements**: Docker for databases/services

**Example**:
```go
//go:build integration
// +build integration

package integration

func TestNodeRepository_Save(t *testing.T) {
    // Setup: Start test database
    db := setupTestDatabase(t)
    defer db.Cleanup()
    
    repo := repository.NewNodeRepository(db)
    node := fixtures.NewNodeBuilder().Build()
    
    // Act
    err := repo.Save(context.Background(), node)
    
    // Assert
    assert.NoError(t, err)
    
    // Verify in database
    saved, err := repo.FindByID(context.Background(), node.ID)
    assert.NoError(t, err)
    assert.Equal(t, node.Content, saved.Content)
}
```

### 3. Contract Tests (`-tags=contracts`)

**Purpose**: Ensure all repository implementations follow the same contract

**Location**: `tests/contracts/`

**Example**:
```go
//go:build contracts
// +build contracts

package contracts

type NodeRepositoryContract struct {
    repo ports.NodeRepository
}

func (c *NodeRepositoryContract) TestSave(t *testing.T) {
    // This test runs against ALL implementations
    // (DynamoDB, PostgreSQL, In-Memory, etc.)
    
    node := builders.NewNodeBuilder().Build()
    
    err := c.repo.Save(context.Background(), node)
    assert.NoError(t, err)
    
    retrieved, err := c.repo.GetByID(context.Background(), node.ID)
    assert.NoError(t, err)
    assert.Equal(t, node, retrieved)
}
```

### 4. BDD Tests (`-tags=bdd`)

**Purpose**: Validate business scenarios

**Location**: `tests/features/`

**Example**:
```go
//go:build bdd
// +build bdd

package features

func TestUserCanConnectRelatedNodes(t *testing.T) {
    framework.NewScenario(t).
        Given().
            UserExists("user-123").
            And().NodeExists("node-1", "Machine Learning").
            And().NodeExists("node-2", "Neural Networks").
        When().
            ConnectingNodes("node-1", "node-2", 0.9).
        Then().
            ConnectionShouldExist("node-1", "node-2").
            And().WeightShouldBe(0.9).
            And().EventShouldBePublished("NodesConnected")
}
```

### 5. Performance Tests (`-tags=bench`)

**Purpose**: Measure and track performance

**Location**: `tests/performance/benchmarks/`

**Example**:
```go
//go:build bench
// +build bench

package benchmarks

func BenchmarkNodeCreation(b *testing.B) {
    builder := fixtures.NewNodeBuilder()
    
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        node := builder.
            WithContent(fmt.Sprintf("Content %d", i)).
            Build()
        _ = node
    }
}
```

## Writing Tests

### Test Structure (AAA Pattern)

All tests follow the **Arrange-Act-Assert** pattern:

```go
func TestSomething(t *testing.T) {
    // Arrange - Set up test data and dependencies
    service := NewService(mockDep)
    input := CreateTestInput()
    
    // Act - Execute the code under test
    result, err := service.DoSomething(input)
    
    // Assert - Verify the results
    assert.NoError(t, err)
    assert.Equal(t, expected, result)
}
```

### Table-Driven Tests

For testing multiple scenarios:

```go
func TestValidation(t *testing.T) {
    tests := []struct {
        name    string
        input   string
        wantErr bool
        errMsg  string
    }{
        {
            name:    "valid input",
            input:   "valid",
            wantErr: false,
        },
        {
            name:    "empty input",
            input:   "",
            wantErr: true,
            errMsg:  "input cannot be empty",
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            err := Validate(tt.input)
            
            if tt.wantErr {
                assert.Error(t, err)
                assert.Contains(t, err.Error(), tt.errMsg)
            } else {
                assert.NoError(t, err)
            }
        })
    }
}
```

## Test Organization

### File Structure

```
backend/
├── internal/
│   ├── core/
│   │   ├── domain/
│   │   │   ├── aggregates/
│   │   │   │   ├── node.go
│   │   │   │   └── node_test.go        # Unit test
│   │   └── application/
│   │       ├── commands/
│   │       │   ├── create_node.go
│   │       │   └── create_node_test.go  # Unit test
├── tests/
│   ├── integration/                     # Integration tests
│   │   ├── repository_test.go
│   │   └── saga_test.go
│   ├── contracts/                       # Contract tests
│   │   └── repository/
│   │       └── node_repository_contract.go
│   ├── features/                        # BDD tests
│   │   ├── node_management_test.go
│   │   └── framework/
│   │       └── scenario.go
│   ├── performance/                     # Benchmarks
│   │   └── benchmarks/
│   │       └── saga_bench_test.go
│   ├── fixtures/                        # Test data builders
│   │   └── builders/
│   │       ├── node_builder.go
│   │       └── event_builder.go
│   └── utils/                          # Test utilities
│       ├── assertions/
│       │   └── domain_assertions.go
│       └── helpers/
│           └── context_helper.go
```

### Build Tags

Each test file should have appropriate build tags:

```go
//go:build unit
// +build unit

// For multiple tags:
//go:build integration && !race
// +build integration,!race
```

## Running Tests

### Using Makefile (Recommended)

```bash
# Quick feedback during development
make test-unit

# Before committing
make test-unit lint fmt

# Before deployment
make test-all

# CI/CD pipeline
make ci
```

### Using build.sh

```bash
# Build with unit tests
./build.sh

# Build with all tests
./build.sh --test-level all

# Quick iteration
./dev.sh
```

### Direct Go Commands

```bash
# Run specific category
go test -tags=unit ./...
go test -tags=integration ./...

# Run with coverage
go test -tags=unit -cover ./...

# Run specific test
go test -run TestNodeCreation ./...

# Verbose output
go test -v -tags=unit ./...
```

## Mocking and Test Doubles

### Using Mockery

Generate mocks for interfaces:

```bash
# Generate all mocks
make mocks

# Manual generation
mockery --name=NodeRepository --dir=internal/core/application/ports --output=tests/mocks
```

### Using Mocks

```go
func TestCreateNodeCommand(t *testing.T) {
    // Create mock
    mockRepo := new(mocks.NodeRepository)
    mockEventBus := new(mocks.EventBus)
    
    // Set expectations
    mockRepo.On("Save", mock.Anything, mock.AnythingOfType("*node.Aggregate")).
        Return(nil)
    mockEventBus.On("Publish", mock.AnythingOfType("*events.NodeCreatedEvent")).
        Return(nil)
    
    // Create command handler with mocks
    handler := commands.NewCreateNodeHandler(mockRepo, mockEventBus)
    
    // Execute test
    cmd := commands.CreateNodeCommand{
        UserID:  "user-123",
        Content: "test content",
    }
    err := handler.Handle(context.Background(), cmd)
    
    // Verify
    assert.NoError(t, err)
    mockRepo.AssertExpectations(t)
    mockEventBus.AssertExpectations(t)
}
```

## Test Fixtures and Builders

### Node Builder

```go
// Create test nodes easily
node := builders.NewNodeBuilder().
    WithUserID("user-123").
    WithContent("Machine Learning").
    WithTags("AI", "ML").
    AsArchived().
    Build()
```

### Event Builder

```go
// Create test events
event := builders.NewEventBuilder().
    WithAggregateID("node-123").
    WithUserID("user-123").
    BuildNodeCreated("content", "title", []string{"tag1"})
```

### Scenario Builder (BDD)

```go
scenario := framework.NewScenario(t).
    Given().SystemIsHealthy().
    When().UserCreatesNode(nodeData).
    Then().NodeShouldBeCreated()
```

## Best Practices

### 1. Test Naming

Use descriptive names that explain the scenario:

```go
// Good
func TestNode_WhenArchived_ShouldNotAcceptConnections(t *testing.T)
func TestCreateNode_WithEmptyContent_ShouldReturnValidationError(t *testing.T)

// Bad
func TestNode1(t *testing.T)
func TestError(t *testing.T)
```

### 2. Test Independence

Tests should not depend on each other:

```go
// Bad - depends on execution order
var sharedNode *Node

func TestCreate(t *testing.T) {
    sharedNode = CreateNode() // Sets shared state
}

func TestUpdate(t *testing.T) {
    UpdateNode(sharedNode) // Depends on TestCreate
}

// Good - independent tests
func TestCreate(t *testing.T) {
    node := CreateNode()
    // test with local node
}

func TestUpdate(t *testing.T) {
    node := CreateNode() // Create own test data
    UpdateNode(node)
}
```

### 3. Use Test Helpers

Extract common setup into helper functions:

```go
func setupTestService(t *testing.T) (*Service, *mocks.Repository) {
    t.Helper() // Marks this as a test helper
    
    mockRepo := new(mocks.Repository)
    service := NewService(mockRepo)
    
    t.Cleanup(func() {
        // Cleanup after test
    })
    
    return service, mockRepo
}

func TestSomething(t *testing.T) {
    service, mockRepo := setupTestService(t)
    // Use service and mock
}
```

### 4. Prefer Real Objects Over Mocks

Use real implementations when possible:

```go
// Good - use real value object
node := domain.NewNode("user-123", "content")

// Unnecessary mock
mockNode := new(mocks.Node)
mockNode.On("GetContent").Return("content")
```

### 5. Test Error Cases

Always test both success and failure paths:

```go
func TestService_Create(t *testing.T) {
    t.Run("success", func(t *testing.T) {
        // Test happy path
    })
    
    t.Run("validation error", func(t *testing.T) {
        // Test with invalid input
    })
    
    t.Run("repository error", func(t *testing.T) {
        // Test when repository fails
    })
}
```

### 6. Use Assertions Effectively

```go
// Use specific assertions
assert.Equal(t, expected, actual, "Node content should match")
assert.Len(t, items, 3, "Should have 3 items")
assert.Contains(t, err.Error(), "validation", "Error should mention validation")

// For critical checks, use require to stop test
require.NoError(t, err, "Setup should not fail")
require.NotNil(t, result, "Result must not be nil")
```

### 7. Document Complex Tests

Add comments for complex test scenarios:

```go
func TestComplexScenario(t *testing.T) {
    // This test verifies that when a node is archived,
    // all its connections are marked as inactive,
    // but the connection history is preserved
    
    // Setup: Create connected nodes
    // ...
    
    // When: Archive the source node
    // ...
    
    // Then: Connections should be inactive but still exist
    // ...
}
```

## Coverage Goals

- **Overall**: > 80%
- **Domain Layer**: > 90% (critical business logic)
- **Application Layer**: > 85%
- **Infrastructure Layer**: > 70%
- **Handlers/Controllers**: > 60%

Check coverage:
```bash
make test-coverage
go tool cover -html=coverage.out
```

## Continuous Integration

Tests run automatically on:
- Pull requests (unit tests)
- Merge to main (all tests)
- Nightly builds (full test suite + benchmarks)

See [.github/workflows/deploy-backend.yml](../.github/workflows/deploy-backend.yml) for CI configuration.

## Troubleshooting

### Common Issues

1. **Tests fail locally but pass in CI**
   - Check Docker is running (for integration tests)
   - Verify environment variables
   - Check test tags are correct

2. **Slow tests**
   - Use `t.Parallel()` for independent tests
   - Mock external services in unit tests
   - Use test containers for integration tests

3. **Flaky tests**
   - Remove time dependencies
   - Use deterministic test data
   - Avoid shared state between tests

4. **Can't find test files**
   - Check build tags match command
   - Verify file naming (*_test.go)
   - Ensure package names are correct

## Resources

- [Testing in Go](https://go.dev/doc/tutorial/add-a-test)
- [Testify Assertions](https://github.com/stretchr/testify)
- [Mockery](https://github.com/vektra/mockery)
- [Table Driven Tests](https://dave.cheney.net/2019/05/07/prefer-table-driven-tests)
- [BDD in Go](https://github.com/onsi/ginkgo)