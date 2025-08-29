# Wire Dependency Injection Troubleshooting Guide

This guide covers common Wire dependency injection issues and their solutions. Wire can be tricky to debug, so this guide provides practical solutions for the most frequent problems.

## Table of Contents
- [Quick Wire Reference](#quick-wire-reference)
- [Common Error Messages and Solutions](#common-error-messages-and-solutions)
- [Wire Development Workflow](#wire-development-workflow)
- [Debugging Techniques](#debugging-techniques)
- [Best Practices](#best-practices)
- [Advanced Wire Patterns](#advanced-wire-patterns)

---

## Quick Wire Reference

### Essential Wire Commands

```bash
# Navigate to DI directory first
cd internal/di

# Check Wire configuration (validates without generating)
wire check

# Generate dependency injection code
wire

# Generate with debug output
WIRE_DEBUG=1 wire

# Show Wire version
wire version

# Clean and regenerate (if wire_gen.go gets corrupted)
rm wire_gen.go && wire

# Diff current vs generated (show what would change)
wire diff
```

### Basic Wire File Structure

```go
// internal/di/wire.go
//go:build wireinject

package di

import "github.com/google/wire"

func InitializeContainer() (*Container, error) {
    wire.Build(SuperSet)
    return nil, nil
}

// internal/di/wire_sets.go  
var SuperSet = wire.NewSet(
    ConfigSet,
    InfrastructureSet,
    RepositorySet,
    ServiceSet,
    HandlerSet,
    provideContainer,
)

// internal/di/wire_providers.go
func provideMyService(dep1 Dep1, dep2 Dep2) *MyService {
    return NewMyService(dep1, dep2)
}
```

---

## Common Error Messages and Solutions

### 1. "no provider found for X"

**Error:**
```
wire: /path/to/wire.go:15:1: inject Initialize: no provider found for YourService
```

**Cause:** Wire can't find a provider function for `YourService`.

**Solutions:**

#### Solution 1: Add Missing Provider
```go
// internal/di/wire_providers.go
func provideYourService(dependency SomeDependency) *YourService {
    return services.NewYourService(dependency)
}

// Add to wire set
var ServiceSet = wire.NewSet(
    provideNodeService,
    provideYourService,  // Add this line
)
```

#### Solution 2: Check Provider Function Signature
```go
// WRONG: Return type doesn't match
func provideNodeService() NodeService {  // Returns interface
    return &services.NodeService{}       // But implementation is pointer
}

// CORRECT: Return concrete type
func provideNodeService() *services.NodeService {  // Returns pointer
    return &services.NodeService{}
}

// OR: Return interface if that's what's expected
func provideNodeService() repository.NodeRepository {  // Interface
    return &dynamodb.NodeRepository{}                   // Implementation
}
```

#### Solution 3: Import Missing Package
```go
// wire_providers.go - Make sure you import the package
import (
    "brain2-backend/internal/application/services"  // Add missing import
)
```

### 2. "unused provider"

**Error:**
```
wire: /path/to/wire_providers.go:45:1: unused provider "provideYourService"
```

**Cause:** Provider function is defined but not used in any wire set.

**Solutions:**

#### Solution 1: Add Provider to Wire Set
```go
// internal/di/wire_sets.go
var ServiceSet = wire.NewSet(
    provideNodeService,
    provideYourService,  // Add the unused provider here
)
```

#### Solution 2: Remove Unused Provider
If the provider is truly not needed, remove it:
```go
// Remove this if not needed
func provideYourService() *YourService {
    return &YourService{}
}
```

### 3. "cannot provide function argument X"

**Error:**
```
wire: /path/to/wire_providers.go:23:1: cannot provide function argument "dependency": no provider found
```

**Cause:** A provider function needs a dependency that Wire can't create.

**Solutions:**

#### Solution 1: Add Missing Dependency Provider
```go
// Provider needs MissingDependency
func provideMyService(dep MissingDependency) *MyService {
    return NewMyService(dep)
}

// Add provider for MissingDependency
func provideMissingDependency() MissingDependency {
    return NewMissingDependency()
}
```

#### Solution 2: Check Parameter Names and Types
```go
// WRONG: Parameter type doesn't match available provider
func provideMyService(cfg *config.Config) *MyService {  // Expects pointer
    return NewMyService(cfg)
}

func provideConfig() config.Config {  // Returns value, not pointer
    return config.LoadConfig()
}

// CORRECT: Match types
func provideConfig() *config.Config {  // Returns pointer
    cfg := config.LoadConfig()
    return &cfg
}
```

### 4. "cycle detected"

**Error:**
```
wire: /path/to/wire.go:15:1: inject Initialize: cycle detected:
ServiceA -> ServiceB -> ServiceA
```

**Cause:** Circular dependency between services.

**Solutions:**

#### Solution 1: Extract Common Dependency
```go
// PROBLEM: A depends on B, B depends on A
func provideServiceA(b *ServiceB) *ServiceA { return NewServiceA(b) }
func provideServiceB(a *ServiceA) *ServiceB { return NewServiceB(a) }

// SOLUTION: Extract shared dependency
func provideSharedService() *SharedService { return NewSharedService() }
func provideServiceA(shared *SharedService) *ServiceA { return NewServiceA(shared) }
func provideServiceB(shared *SharedService) *ServiceB { return NewServiceB(shared) }
```

#### Solution 2: Use Interface to Break Cycle
```go
// Define interface in domain layer
type NodeReader interface {
    FindByID(ctx context.Context, id string) (*Node, error)
}

// ServiceA depends on interface, not concrete type
func provideServiceA(reader NodeReader) *ServiceA {
    return NewServiceA(reader)
}

// Wire binds implementation to interface
var InterfaceBindings = wire.NewSet(
    wire.Bind(new(NodeReader), new(*NodeRepository)),
)
```

#### Solution 3: Refactor Architecture
Sometimes circular dependencies indicate design problems:
```go
// Consider extracting shared logic into domain service
// Or using event-driven communication between services
```

### 5. "provider returns error"

**Error:**
```
wire: /path/to/wire_providers.go:15:1: provider returns error, but injection does not
```

**Cause:** Provider function returns an error, but the injector signature doesn't handle errors.

**Solutions:**

#### Solution 1: Update Injector to Handle Error
```go
// WRONG: Injector doesn't return error
func InitializeContainer() *Container {
    wire.Build(SuperSet)
    return nil
}

// CORRECT: Injector returns error
func InitializeContainer() (*Container, error) {
    wire.Build(SuperSet)
    return nil, nil
}
```

#### Solution 2: Remove Error from Provider
If the provider never actually fails:
```go
// Before: Returns error
func provideConfig() (*config.Config, error) {
    return config.LoadConfig(), nil  // Never fails
}

// After: No error
func provideConfig() *config.Config {
    return config.LoadConfig()
}
```

### 6. "interface contains type constraints"

**Error:**
```
wire: cannot use interface X as Y: interface contains type constraints
```

**Cause:** Wire doesn't support generic interfaces with type constraints.

**Solutions:**

#### Solution 1: Use Concrete Types
```go
// PROBLEM: Generic interface
type Repository[T any] interface {
    Save(ctx context.Context, entity T) error
}

// SOLUTION: Concrete interface per type
type NodeRepository interface {
    Save(ctx context.Context, node *Node) error
}

type EdgeRepository interface {
    Save(ctx context.Context, edge *Edge) error
}
```

#### Solution 2: Use Wire with Generic Factories
```go
// Factory function that returns concrete type
func CreateNodeRepository() *GenericRepository[*Node] {
    return NewGenericRepository[*Node](/* params */)
}

// Wire provider
func provideNodeRepository() *GenericRepository[*Node] {
    return CreateNodeRepository()
}
```

---

## Wire Development Workflow

### Safe Development Process

1. **Before Making Changes**
   ```bash
   cd internal/di
   wire check  # Ensure current state is valid
   ```

2. **Make Your Changes**
   - Add new provider functions
   - Update wire sets
   - Modify constructors

3. **Validate Changes**
   ```bash
   wire check  # Check for errors before generating
   ```

4. **Generate Code**
   ```bash
   wire        # Generate wire_gen.go
   ```

5. **Verify Build**
   ```bash
   cd ../..
   go build ./cmd/main/main.go  # Ensure everything compiles
   ```

6. **Test Integration**
   ```bash
   go test ./internal/di/...    # Test dependency injection
   ```

### Incremental Development

When adding complex new features:

```bash
# 1. Start with basic provider
func provideNewService() *NewService {
    return &NewService{}  # Minimal constructor
}

# 2. Add to wire set and test
wire check && wire

# 3. Add dependencies one by one
func provideNewService(dep1 Dep1) *NewService {
    return &NewService{dep1: dep1}
}

# 4. Test after each dependency
wire check && wire
```

---

## Debugging Techniques

### 1. Use Wire Debug Mode

```bash
WIRE_DEBUG=1 wire
```

This shows:
- Which providers Wire is considering
- Dependency resolution order
- Why certain providers are rejected

### 2. Check Generated Code

Look at `wire_gen.go` to understand what Wire generated:

```go
// wire_gen.go - Generated by Wire
func InitializeContainer() (*Container, error) {
    config, err := provideConfig()         // 1. Config first
    if err != nil {
        return nil, err
    }
    
    dynamoDBClient := provideDynamoDBClient(config)  // 2. Then DynamoDB
    nodeRepository := provideNodeRepository(dynamoDBClient, config)  // 3. Repository
    nodeService := provideNodeService(nodeRepository)  // 4. Service
    
    container := provideContainer(config, nodeService)  // 5. Finally container
    return container, nil
}
```

### 3. Manual Dependency Testing

Test your providers manually:

```go
// test/wire_test.go
func TestProviders(t *testing.T) {
    // Test individual providers
    config := provideConfig()
    assert.NotNil(t, config)
    
    client := provideDynamoDBClient(config)
    assert.NotNil(t, client)
    
    repo := provideNodeRepository(client, config)
    assert.NotNil(t, repo)
}
```

### 4. Wire Set Isolation

Test wire sets individually:

```go
// internal/di/wire_debug.go (temporary file for debugging)
//go:build wireinject && debug

package di

import "github.com/google/wire"

// Test specific wire set
func DebugRepositorySet() (*Repository, error) {
    wire.Build(RepositorySet)
    return nil, nil
}
```

### 5. Provider Function Tracing

Add temporary logging to understand call order:

```go
func provideNodeService(repo repository.NodeRepository) *services.NodeService {
    log.Printf("Creating NodeService with repo: %T", repo)  // Temporary debug
    return services.NewNodeService(repo)
}
```

---

## Best Practices

### 1. Organize Providers by Layer

```go
// internal/di/wire_providers.go - Organize by architectural layer

// ============================================================================
// INFRASTRUCTURE LAYER
// ============================================================================
func provideConfig() *config.Config { /* */ }
func provideDynamoDBClient(cfg *config.Config) *dynamodb.Client { /* */ }
func provideEventBridgeClient(cfg *config.Config) *eventbridge.Client { /* */ }

// ============================================================================
// REPOSITORY LAYER
// ============================================================================
func provideNodeRepository(client *dynamodb.Client, cfg *config.Config) repository.NodeRepository { /* */ }
func provideEdgeRepository(client *dynamodb.Client, cfg *config.Config) repository.EdgeRepository { /* */ }

// ============================================================================
// SERVICE LAYER
// ============================================================================
func provideNodeService(nodeRepo repository.NodeRepository) *services.NodeService { /* */ }
```

### 2. Use Descriptive Provider Names

```go
// GOOD: Clear what it provides
func provideNodeRepository() repository.NodeRepository { /* */ }
func provideAuthenticatedHTTPClient() *http.Client { /* */ }
func provideCacheWithTTL() *cache.Cache { /* */ }

// BAD: Unclear names
func newRepo() repository.NodeRepository { /* */ }
func getClient() *http.Client { /* */ }
func makeCache() *cache.Cache { /* */ }
```

### 3. Group Related Providers in Sets

```go
// internal/di/wire_sets.go

var InfrastructureSet = wire.NewSet(
    provideConfig,
    provideDynamoDBClient,
    provideEventBridgeClient,
    provideLogger,
)

var RepositorySet = wire.NewSet(
    provideNodeRepository,
    provideEdgeRepository,
    provideCategoryRepository,
)

var ServiceSet = wire.NewSet(
    provideNodeService,
    provideCategoryService,
    provideCleanupService,
)
```

### 4. Use Interface Bindings Sparingly

```go
// Use wire.Bind when you need interface flexibility
var InterfaceBindings = wire.NewSet(
    provideDynamoDBNodeRepository,
    wire.Bind(new(repository.NodeRepository), new(*dynamodb.NodeRepository)),
)

// But prefer concrete types when possible for clarity
func provideNodeService(repo *dynamodb.NodeRepository) *NodeService {
    return NewNodeService(repo)
}
```

### 5. Handle Errors Consistently

```go
// All providers that can fail should return error
func provideConfig() (*config.Config, error) {
    cfg, err := config.Load()
    if err != nil {
        return nil, fmt.Errorf("failed to load config: %w", err)
    }
    return cfg, nil
}

// Injector must handle errors if any provider returns error
func InitializeContainer() (*Container, error) {
    wire.Build(SuperSet)
    return nil, nil  // Wire replaces this
}
```

---

## Advanced Wire Patterns

### 1. Conditional Providers

```go
func provideCache(cfg *config.Config) Cache {
    if cfg.Environment == "production" {
        return redis.NewCache(cfg.RedisURL)
    }
    return memory.NewCache()
}
```

### 2. Provider with Multiple Return Values

```go
func provideClientWithCleanup(cfg *config.Config) (*http.Client, func(), error) {
    client := &http.Client{Timeout: cfg.HTTPTimeout}
    cleanup := func() {
        // Cleanup logic
    }
    return client, cleanup, nil
}
```

### 3. Wire with Generic Factories

```go
// Generic factory function
func CreateRepository[T Entity](client *dynamodb.Client, config EntityConfig[T]) *GenericRepository[T] {
    return NewGenericRepository[T](client, config)
}

// Specific provider using the factory
func provideNodeRepository(client *dynamodb.Client) *GenericRepository[*node.Node] {
    config := &NodeEntityConfig{}
    return CreateRepository[*node.Node](client, config)
}
```

### 4. Layered Wire Sets

```go
// Base infrastructure that's always needed
var BaseSet = wire.NewSet(
    provideConfig,
    provideLogger,
)

// Development-specific additions
var DevelopmentSet = wire.NewSet(
    BaseSet,
    provideLocalDynamoDB,
    provideTestEventBus,
)

// Production-specific additions  
var ProductionSet = wire.NewSet(
    BaseSet,
    provideAWSDynamoDB,
    provideEventBridgeEventBus,
)
```

### 5. Factory Pattern with Wire

```go
// Factory interface
type RepositoryFactory interface {
    CreateNodeRepository() repository.NodeRepository
    CreateEdgeRepository() repository.EdgeRepository
}

// Factory implementation
type DynamoDBRepositoryFactory struct {
    client    *dynamodb.Client
    tableName string
}

func (f *DynamoDBRepositoryFactory) CreateNodeRepository() repository.NodeRepository {
    return dynamodb.NewNodeRepository(f.client, f.tableName)
}

// Wire provider for factory
func provideRepositoryFactory(client *dynamodb.Client, cfg *config.Config) RepositoryFactory {
    return &DynamoDBRepositoryFactory{
        client:    client,
        tableName: cfg.TableName,
    }
}

// Use factory in other providers
func provideNodeService(factory RepositoryFactory) *NodeService {
    nodeRepo := factory.CreateNodeRepository()
    return NewNodeService(nodeRepo)
}
```

---

## Quick Recovery Commands

If Wire gets into a bad state:

```bash
# 1. Clean everything and start fresh
cd internal/di
rm wire_gen.go
go clean -cache
wire

# 2. If Wire command fails, check for syntax errors
go build .  # Should show Go syntax errors

# 3. If provider conflicts, check for duplicate functions
grep -n "func provide" *.go  # Find all providers

# 4. If circular dependency, visualize dependencies  
go mod graph | grep brain2-backend  # Show module dependencies

# 5. Check Wire version compatibility
wire version
go list -m github.com/google/wire  # Check Wire module version
```

Remember: Wire is a compile-time tool. If it works during `wire` generation, it will work at runtime. Focus on getting `wire check` to pass first, then `wire` to generate successfully.