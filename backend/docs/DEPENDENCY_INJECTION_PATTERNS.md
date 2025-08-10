# Dependency Injection Patterns in Brain2

## Overview

This document explains the dependency injection patterns implemented in the Brain2 application, focusing on interface segregation and clean architecture principles.

## Architecture Summary

The Brain2 application uses a centralized dependency injection container with segregated repository interfaces, following the Interface Segregation Principle (ISP) from SOLID design principles.

## Interface Segregation Pattern

### Problem Solved

Previously, we had a large monolithic `Repository` interface with 50+ methods covering all operations. This violated the Interface Segregation Principle because:

- Services were forced to depend on methods they didn't need
- Mock testing required implementing all methods even for focused tests
- Code became harder to understand and maintain
- Future scaling to microservices would be difficult

### Solution: Segregated Interfaces

We've broken down the repository layer into focused, cohesive interfaces:

```go
// Core domain interfaces
type NodeRepository interface {
    CreateNodeAndKeywords(ctx context.Context, node domain.Node) error
    FindNodeByID(ctx context.Context, userID, nodeID string) (*domain.Node, error)
    // ... other node operations
}

type EdgeRepository interface {
    CreateEdges(ctx context.Context, userID, sourceNodeID string, relatedNodeIDs []string) error
    FindEdges(ctx context.Context, query EdgeQuery) ([]domain.Edge, error)
    // ... other edge operations
}

type CategoryRepository interface {
    CreateCategory(ctx context.Context, category domain.Category) error
    FindCategoryByID(ctx context.Context, userID, categoryID string) (*domain.Category, error)
    // ... other category operations
}

// Additional segregated interfaces
type KeywordRepository interface { ... }
type TransactionalRepository interface { ... }
type GraphRepository interface { ... }
```

### Composed Interface for Backward Compatibility

```go
type Repository interface {
    NodeRepository
    EdgeRepository
    KeywordRepository
    TransactionalRepository
    CategoryRepository
    GraphRepository
}
```

## Service Layer Benefits

### Memory Service Dependencies

The memory service now explicitly declares only what it needs:

```go
type service struct {
    nodeRepo         repository.NodeRepository
    edgeRepo         repository.EdgeRepository
    keywordRepo      repository.KeywordRepository
    transactionRepo  repository.TransactionalRepository
    graphRepo        repository.GraphRepository
    idempotencyStore repository.IdempotencyStore
}
```

**Benefits:**
- Clear dependency visibility
- Easier unit testing with focused mocks
- Better understanding of service responsibilities
- Preparation for future microservices architecture

### Category Service Dependencies

```go
type service struct {
    categoryRepo repository.CategoryRepository
    nodeRepo     repository.NodeRepository  // For node-category mappings
}
```

**Benefits:**
- Only depends on what it actually uses
- No unnecessary dependencies on edges, keywords, or graph operations
- Clearer separation of concerns

## Dependency Injection Container

### Container Structure

```go
type Container struct {
    // Segregated repositories
    NodeRepository         repository.NodeRepository
    EdgeRepository         repository.EdgeRepository
    KeywordRepository      repository.KeywordRepository
    TransactionalRepository repository.TransactionalRepository
    CategoryRepository     repository.CategoryRepository
    GraphRepository        repository.GraphRepository
    
    // Composed repository for backward compatibility
    Repository       repository.Repository
    IdempotencyStore repository.IdempotencyStore
    
    // Services
    MemoryService   memoryService.Service
    CategoryService categoryService.Service
    // ...
}
```

### Initialization Flow

```go
func (c *Container) initializeRepository() error {
    // Create segregated repositories
    c.NodeRepository = dynamodb.NewNodeRepository(c.DynamoDBClient, c.Config.TableName, c.Config.IndexName)
    c.EdgeRepository = dynamodb.NewEdgeRepository(c.DynamoDBClient, c.Config.TableName, c.Config.IndexName)
    // ... other repositories
    
    // Create composed repository for backward compatibility
    c.Repository = dynamodb.NewRepository(c.DynamoDBClient, c.Config.TableName, c.Config.IndexName)
}

func (c *Container) initializeServices() {
    // Services use segregated dependencies
    c.MemoryService = memoryService.NewServiceWithIdempotency(
        c.NodeRepository,
        c.EdgeRepository,
        c.KeywordRepository,
        c.TransactionalRepository,
        c.GraphRepository,
        c.IdempotencyStore,
    )
    
    c.CategoryService = categoryService.NewService(c.CategoryRepository, c.NodeRepository)
}
```

## Constructor Patterns

### New Pattern: Segregated Dependencies

```go
// Memory service with explicit dependencies
func NewService(nodeRepo repository.NodeRepository, edgeRepo repository.EdgeRepository, keywordRepo repository.KeywordRepository, transactionRepo repository.TransactionalRepository, graphRepo repository.GraphRepository) Service

// Category service with focused dependencies  
func NewService(categoryRepo repository.CategoryRepository, nodeRepo repository.NodeRepository) Service
```

### Backward Compatibility

```go
// Backward compatible constructors
func NewServiceFromRepository(repo repository.Repository) Service {
    return &service{
        nodeRepo:        repo,
        edgeRepo:        repo,
        keywordRepo:     repo,
        transactionRepo: repo,
        graphRepo:       repo,
    }
}
```

## Implementation Details

### Factory Functions

Each segregated interface has its own factory function:

```go
func NewNodeRepository(dbClient *dynamodb.Client, tableName, indexName string) repository.NodeRepository
func NewEdgeRepository(dbClient *dynamodb.Client, tableName, indexName string) repository.EdgeRepository
func NewCategoryRepository(dbClient *dynamodb.Client, tableName, indexName string) repository.CategoryRepository
```

All factory functions return the same underlying DynamoDB implementation, but with different interface types for compile-time safety.

### Testing Benefits

Before (monolithic interface):
```go
mockRepo := &mocks.MockRepository{} // Must implement 50+ methods
service := NewService(mockRepo)
```

After (segregated interfaces):
```go
mockNodeRepo := &mocks.MockNodeRepository{} // Only 6 methods needed
mockEdgeRepo := &mocks.MockEdgeRepository{} // Only 3 methods needed
service := NewService(mockNodeRepo, mockEdgeRepo, ...)
```

## Migration Guide

### For New Code

Use the segregated constructors:
```go
// Create specific repositories
nodeRepo := dynamodb.NewNodeRepository(client, table, index)
categoryRepo := dynamodb.NewCategoryRepository(client, table, index)

// Create services with specific dependencies
categoryService := categoryService.NewService(categoryRepo, nodeRepo)
```

### For Existing Code

Use backward-compatible constructors:
```go
// Existing code continues to work
repo := dynamodb.NewRepository(client, table, index)
categoryService := categoryService.NewServiceFromRepository(repo)
```

## Future Benefits

### Microservices Preparation

With segregated interfaces, it becomes easier to:

1. **Split services**: Each service has clear dependencies
2. **Create adapters**: Interface with remote services  
3. **Mock external dependencies**: Test services in isolation
4. **Implement caching**: Add caching layers for specific operations

### Example: Node Service Microservice

```go
// In a future microservices architecture
type RemoteNodeRepository struct {
    client NodeServiceClient
}

func (r *RemoteNodeRepository) FindNodeByID(ctx context.Context, userID, nodeID string) (*domain.Node, error) {
    return r.client.GetNode(ctx, &pb.GetNodeRequest{UserID: userID, NodeID: nodeID})
}

// Memory service works unchanged
memoryService := memory.NewService(remoteNodeRepo, localEdgeRepo, ...)
```

## Performance Considerations

### Memory Usage

- **No overhead**: Segregated interfaces point to the same implementation
- **Compile-time safety**: Interface segregation is enforced at compile time
- **Runtime efficiency**: No additional indirection or allocations

### Development Workflow

- **Faster builds**: Cleaner dependency graphs
- **Better IDE support**: IntelliSense shows only relevant methods
- **Easier debugging**: Clear service boundaries and dependencies

## Best Practices

### When to Use Segregated Interfaces

✅ **Use segregated interfaces when:**
- Creating new services
- Service has focused responsibilities
- Testing requires specific mocks
- Preparing for microservices

✅ **Use composed interface when:**
- Migrating existing code gradually
- Service needs many repository operations
- Backward compatibility is required

### Interface Design Principles

1. **Single Responsibility**: Each interface should have one reason to change
2. **Cohesion**: Methods in an interface should be related
3. **Minimal Dependencies**: Services should depend only on what they use
4. **Backward Compatibility**: Provide migration paths for existing code

## Conclusion

The interface segregation pattern in Brain2 provides:

- **Better testability** with focused mock interfaces
- **Clearer dependencies** between services and repositories  
- **Preparation for scaling** to microservices architecture
- **Maintainability** through explicit dependency management
- **Learning opportunity** for SOLID principles implementation

This pattern demonstrates how to evolve a monolithic interface into a clean, segregated architecture while maintaining backward compatibility and system stability.