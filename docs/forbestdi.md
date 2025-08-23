# Dependency Injection Architecture Analysis & Best Practices Guide

## Executive Summary

This document analyzes the dependency injection (DI) architecture for the Brain2 backend application, evaluating the trade-offs between Google Wire (compile-time DI) and the current manual container approach. Based on extensive investigation, I recommend a **hybrid approach** that leverages the strengths of both systems while following industry best practices.

## Current State Analysis

### What Exists Today

1. **Manual Container System** (`container.go`)
   - 700+ lines of manual dependency wiring
   - Lifecycle management (init, validate, shutdown)
   - Cold start optimization for AWS Lambda
   - Runtime configuration based on environment
   - Phase-based initialization (repositories → services → handlers)

2. **Partial Wire Infrastructure**
   - Provider functions exist but with incorrect signatures
   - Wire configuration files present but not functional
   - Build scripts reference Wire but it's been disabled

3. **Lambda-Specific Optimizations**
   - Cold start detection and tracking
   - Connection pooling for warm invocations
   - Lazy initialization of expensive resources
   - Graceful shutdown handling

### Why Wire Was Removed

Based on git history analysis (commit b734e9a):

1. **Lifecycle Management Needs**: Wire only handles creation, not lifecycle
2. **Runtime Flexibility**: Need to adjust configuration at runtime
3. **Lambda Optimization**: Cold start handling requires runtime decisions
4. **Complex Patterns**: Repository decorators, factories, and UoW patterns
5. **Incremental Refactoring**: Easier to modify without regenerating Wire

## Industry Best Practices Analysis

### Dependency Injection Patterns

#### 1. **Constructor Injection** ✅
- Most testable and explicit
- Clear dependencies
- Immutable after construction

#### 2. **Interface Segregation** ✅
- Small, focused interfaces
- Repository pattern with readers/writers
- CQRS support

#### 3. **Dependency Inversion** ✅
- Domain doesn't depend on infrastructure
- Interfaces defined in domain/application layers
- Implementations in infrastructure layer

### DI Framework Comparison

| Aspect | Google Wire | Manual Container | Uber Fx | Microsoft DI |
|--------|------------|------------------|---------|--------------|
| **Type** | Compile-time | Runtime | Runtime | Runtime |
| **Safety** | Compile-time checks | Runtime validation | Runtime checks | Runtime checks |
| **Performance** | Zero overhead | Minimal overhead | Some overhead | Minimal overhead |
| **Lifecycle** | Creation only | Full lifecycle | Full lifecycle | Full lifecycle |
| **Learning Curve** | Moderate | Low | High | Moderate |
| **Flexibility** | Limited | High | High | High |
| **Lambda Suitability** | Good | Excellent | Poor (heavy) | Good |

### Lambda-Specific Considerations

1. **Cold Start Performance**: Critical for user experience
2. **Memory Usage**: Affects Lambda pricing
3. **Connection Reuse**: Essential for warm invocations
4. **Graceful Shutdown**: Prevent data loss
5. **Initialization Time**: Must be under 10 seconds

## Recommended Architecture: Hybrid Approach

### Core Principles

1. **Use Wire for Pure Dependency Graph**
   - Let Wire handle the complex wiring of repositories, services, handlers
   - Compile-time safety for dependency resolution
   - Clear visualization of dependency graph

2. **Use Container for Lifecycle & Runtime**
   - Lifecycle management (init, validate, shutdown)
   - Cold start optimization
   - Runtime configuration
   - Connection pooling
   - Graceful shutdown

3. **Clear Separation of Concerns**
   - Wire: WHAT to create and HOW to wire
   - Container: WHEN to create and lifecycle management
   - Providers: HOW to construct each component

### Proposed Architecture

```go
// Wire generates this
func InitializeDependencies() (*Dependencies, error) {
    // Wire-generated code
    // Returns all wired dependencies
}

// Container manages this
type Container struct {
    deps       *Dependencies
    lifecycle  *LifecycleManager
    coldStart  *ColdStartTracker
    shutdown   []func() error
}

func NewContainer() (*Container, error) {
    // Use Wire for dependency creation
    deps, err := InitializeDependencies()
    if err != nil {
        return nil, err
    }
    
    // Add lifecycle management on top
    return &Container{
        deps:      deps,
        lifecycle: NewLifecycleManager(deps),
        coldStart: NewColdStartTracker(),
    }, nil
}
```

## Implementation Plan

### Phase 1: Fix Wire Infrastructure (Week 1)

#### Step 1.1: Fix Provider Signatures
```go
// Fix provideCleanupService
func provideCleanupService(
    nodeRepo repository.NodeRepository,
    edgeRepo repository.EdgeRepository,
    idempotencyStore repository.IdempotencyStore,
    uowFactory repository.UnitOfWorkFactory,
) *services.CleanupService {
    // EdgeWriter can be obtained from EdgeRepository
    edgeWriter := edgeRepo.(repository.EdgeWriter)
    return services.NewCleanupService(
        nodeRepo, edgeRepo, edgeWriter, 
        idempotencyStore, uowFactory,
    )
}

// Fix provideGraphQueryService  
func provideGraphQueryService(
    store persistence.Store,
    logger *zap.Logger,
    cache queries.Cache,
) *queries.GraphQueryService {
    return queries.NewGraphQueryService(store, logger, cache)
}

// Fix provideHealthHandler
func provideHealthHandler() *handlers.HealthHandler {
    return handlers.NewHealthHandler()
}
```

#### Step 1.2: Add Missing Providers
```go
// Add UnitOfWorkFactory provider
func provideUnitOfWorkFactory(
    nodeRepo repository.NodeRepository,
    edgeRepo repository.EdgeRepository,
    categoryRepo repository.CategoryRepository,
    transactionalRepo repository.TransactionalRepository,
) repository.UnitOfWorkFactory {
    return repository.NewUnitOfWorkFactory(
        nodeRepo, edgeRepo, categoryRepo, transactionalRepo,
    )
}

// Fix RepositoryFactory
func provideRepositoryFactory(cfg *config.Config) *repository.RepositoryFactory {
    factoryConfig := repository.FactoryConfig{ // Note: FactoryConfig, not RepositoryFactoryConfig
        EnableCache:   cfg.Cache.Provider != "none",
        EnableMetrics: cfg.Metrics.Provider != "none",
        // Remove EnableCircuitBreaker or add to config
    }
    return repository.NewRepositoryFactory(factoryConfig)
}
```

#### Step 1.3: Fix Type Issues
- Change `repository.RepositoryFactoryConfig` → `repository.FactoryConfig`
- Use `InitTracing` instead of non-existent `NewTracerProvider`
- Add `EnableCircuitBreaker` to config or remove from factory

### Phase 2: Create Hybrid Container (Week 2)

#### Step 2.1: Define Dependencies Structure
```go
// types.go
type Dependencies struct {
    // Core infrastructure
    Config            *config.Config
    Logger            *zap.Logger
    DynamoDBClient    *awsDynamodb.Client
    EventBridgeClient *awsEventbridge.Client
    
    // Repositories
    NodeRepository     repository.NodeRepository
    EdgeRepository     repository.EdgeRepository
    CategoryRepository repository.CategoryRepository
    // ... etc
    
    // Services
    NodeService     *services.NodeService
    CategoryService *services.CategoryService
    // ... etc
    
    // Handlers
    MemoryHandler   *handlers.MemoryHandler
    CategoryHandler *handlers.CategoryHandler
    HealthHandler   *handlers.HealthHandler
    
    // Router
    Router *chi.Mux
}
```

#### Step 2.2: Wire Configuration
```go
// wire.go
//go:build wireinject
// +build wireinject

func InitializeDependencies() (*Dependencies, error) {
    wire.Build(
        // All provider sets
        ConfigProviders,
        InfrastructureProviders,
        DomainProviders,
        ApplicationProviders,
        InterfaceProviders,
        
        // Constructor for Dependencies struct
        NewDependencies,
    )
    return nil, nil
}
```

#### Step 2.3: Lifecycle Container
```go
// container.go
type Container struct {
    *Dependencies
    
    // Lifecycle management
    coldStartTracker  *ColdStartTracker
    shutdownFunctions []func() error
    initialized       bool
    
    // Runtime state
    warmInvocations int
    lastActivity    time.Time
}

func NewContainer() (*Container, error) {
    deps, err := InitializeDependencies()
    if err != nil {
        return nil, fmt.Errorf("wire initialization failed: %w", err)
    }
    
    container := &Container{
        Dependencies:     deps,
        coldStartTracker: NewColdStartTracker(),
    }
    
    // Add shutdown functions
    container.registerShutdownHandlers()
    
    // Validate
    if err := container.Validate(); err != nil {
        return nil, fmt.Errorf("validation failed: %w", err)
    }
    
    return container, nil
}
```

### Phase 3: Testing & Validation (Week 3)

#### Step 3.1: Unit Tests
- Test each provider function independently
- Mock dependencies using interfaces
- Verify Wire generation succeeds

#### Step 3.2: Integration Tests
- Test full container initialization
- Verify lifecycle management
- Test cold start scenarios

#### Step 3.3: Lambda Testing
- Deploy to test environment
- Measure cold start times
- Verify warm invocation performance
- Test graceful shutdown

### Phase 4: Migration & Documentation (Week 4)

#### Step 4.1: Gradual Migration
1. Keep existing container working
2. Implement new hybrid container alongside
3. Switch Lambda functions one by one
4. Remove old container after validation

#### Step 4.2: Documentation
- Update README with DI architecture
- Document provider patterns
- Create troubleshooting guide
- Add performance benchmarks

## Best Practices Implementation

### 1. Provider Organization
```go
// Group by layer, not by type
var InfrastructureProviders = wire.NewSet(
    // AWS Clients
    provideAWSConfig,
    provideDynamoDBClient,
    
    // Repositories
    provideNodeRepository,
    provideEdgeRepository,
    
    // Cross-cutting
    provideCache,
    provideMetrics,
)
```

### 2. Interface Design
```go
// Small, focused interfaces
type NodeReader interface {
    FindNodeByID(ctx context.Context, id string) (*Node, error)
    FindNodes(ctx context.Context, query Query) ([]*Node, error)
}

type NodeWriter interface {
    CreateNode(ctx context.Context, node *Node) error
    UpdateNode(ctx context.Context, node *Node) error
    DeleteNode(ctx context.Context, id string) error
}

type NodeRepository interface {
    NodeReader
    NodeWriter
}
```

### 3. Error Handling
```go
// Wrap errors with context
func provideNodeRepository(...) (repository.NodeRepository, error) {
    repo := dynamodb.NewNodeRepository(...)
    if err := repo.Validate(); err != nil {
        return nil, fmt.Errorf("node repository validation failed: %w", err)
    }
    return repo, nil
}
```

### 4. Configuration Validation
```go
// Validate at provider level
func provideConfig() (*config.Config, error) {
    cfg := config.LoadConfig()
    if err := cfg.Validate(); err != nil {
        return nil, fmt.Errorf("invalid configuration: %w", err)
    }
    return cfg, nil
}
```

### 5. Graceful Shutdown
```go
// Register cleanup in providers
func provideDynamoDBClient(...) (*dynamodb.Client, func()) {
    client := dynamodb.New(...)
    cleanup := func() {
        // Close connections
        client.Close()
    }
    return client, cleanup
}
```

## Performance Considerations

### Cold Start Optimization
1. **Lazy Initialization**: Defer expensive operations
2. **Connection Pooling**: Reuse connections in warm containers
3. **Minimal Dependencies**: Only inject what's needed
4. **Parallel Initialization**: Where possible

### Memory Optimization
1. **Interface Pointers**: Use pointer receivers
2. **Singleton Pattern**: Share stateless components
3. **Resource Pooling**: Reuse expensive objects
4. **Cleanup Unused**: Release resources when idle

## Monitoring & Observability

### Metrics to Track
1. **Cold Start Duration**: Time from init to ready
2. **Dependency Creation Time**: Per component
3. **Memory Usage**: Before/after initialization
4. **Warm Invocation Reuse**: Connection pool efficiency

### Logging Strategy
```go
// Structured logging at each phase
logger.Info("initializing repository layer",
    zap.Duration("elapsed", time.Since(start)),
    zap.Int("repositories", len(repos)),
)
```

## Common Pitfalls to Avoid

1. **Circular Dependencies**: Wire will catch these at compile time
2. **Missing Cleanup**: Always register shutdown handlers
3. **Runtime Type Assertions**: Prefer compile-time bindings
4. **Global State**: Everything should be injected
5. **Synchronous Initialization**: Use goroutines where safe

## Recommended Reading & Resources

### Dependency Injection
- [Martin Fowler - Inversion of Control Containers](https://martinfowler.com/articles/injection.html)
- [Google Wire Best Practices](https://github.com/google/wire/blob/main/docs/best-practices.md)
- [Uber Fx Documentation](https://uber-go.github.io/fx/)

### Lambda Optimization
- [AWS Lambda Cold Start Optimization](https://aws.amazon.com/blogs/compute/operating-lambda-performance-optimization-part-1/)
- [Lambda Container Reuse](https://docs.aws.amazon.com/lambda/latest/dg/lambda-runtime-environment.html)

### Go Patterns
- [Go Concurrency Patterns](https://go.dev/blog/pipelines)
- [Effective Go](https://go.dev/doc/effective_go)
- [Go Code Review Comments](https://github.com/golang/go/wiki/CodeReviewComments)

## Next Steps

### Immediate Actions (This Week)
1. ✅ Fix provider function signatures
2. ✅ Add missing provider functions  
3. ✅ Fix type naming issues
4. ✅ Generate Wire code successfully
5. ✅ Test compilation

### Short Term (Next 2 Weeks)
1. ⏱️ Implement hybrid container
2. ⏱️ Add comprehensive tests
3. ⏱️ Benchmark performance
4. ⏱️ Deploy to test environment

### Long Term (Next Month)
1. 📅 Migrate all Lambda functions
2. 📅 Remove old container code
3. 📅 Document patterns for team
4. 📅 Create project template
5. 📅 Set up CI/CD with Wire generation

## Conclusion

The hybrid approach combining Wire's compile-time safety with manual lifecycle management provides the best of both worlds for this Lambda-based application. This architecture:

1. **Maintains compile-time safety** through Wire
2. **Provides full lifecycle management** through the container
3. **Optimizes for Lambda** cold starts and warm invocations
4. **Follows industry best practices** for DI and clean architecture
5. **Remains maintainable** and testable

By implementing this approach, the application will have a robust, performant, and maintainable dependency injection system that serves as an excellent example of production-grade Go architecture.

## Appendix A: Code Examples

### Complete Provider Example
```go
// providers.go
func provideNodeService(
    nodeRepo repository.NodeRepository,
    edgeRepo repository.EdgeRepository,
    analyzer *services.ConnectionAnalyzer,
    eventBus shared.EventBus,
    logger *zap.Logger,
) (*services.NodeService, func(), error) {
    // Validate dependencies
    if nodeRepo == nil {
        return nil, nil, errors.New("node repository is required")
    }
    
    // Create service
    service := services.NewNodeService(
        nodeRepo, edgeRepo, analyzer, eventBus, logger,
    )
    
    // Initialize if needed
    if err := service.Initialize(); err != nil {
        return nil, nil, fmt.Errorf("failed to initialize node service: %w", err)
    }
    
    // Cleanup function
    cleanup := func() {
        if err := service.Shutdown(); err != nil {
            logger.Error("failed to shutdown node service", zap.Error(err))
        }
    }
    
    return service, cleanup, nil
}
```

### Wire Injector Example
```go
// wire.go
//go:build wireinject
// +build wireinject

package di

import (
    "github.com/google/wire"
    // ... imports
)

func InitializeDependencies() (*Dependencies, func(), error) {
    wire.Build(
        // Providers
        ConfigProviders,
        InfrastructureProviders,
        DomainProviders,
        ApplicationProviders,
        InterfaceProviders,
        
        // Cleanup aggregator
        wire.Struct(new(Dependencies), "*"),
        aggregateCleanup,
    )
    return nil, nil, nil
}

func aggregateCleanup(cleanups ...func()) func() {
    return func() {
        // Execute in reverse order
        for i := len(cleanups) - 1; i >= 0; i-- {
            if cleanups[i] != nil {
                cleanups[i]()
            }
        }
    }
}
```

### Container Usage Example
```go
// main.go
func init() {
    container, err := di.NewContainer()
    if err != nil {
        log.Fatalf("Failed to initialize container: %v", err)
    }
    
    // Track cold start
    if container.IsColdStart() {
        metrics.RecordColdStart(container.GetColdStartDuration())
    }
    
    // Set up Lambda adapter
    router := container.GetRouter()
    lambdaAdapter = chiadapter.NewV2(router)
}

func main() {
    defer container.Shutdown(context.Background())
    lambda.Start(handler)
}
```

## Appendix B: Performance Benchmarks

### Expected Performance Metrics

| Metric | Current (Manual) | Target (Hybrid) | Industry Best |
|--------|-----------------|-----------------|---------------|
| Cold Start | 3-5s | 2-3s | 1-2s |
| Warm Invocation | <100ms | <50ms | <20ms |
| Memory Usage | 128MB | 100MB | 64MB |
| Init Time | 2s | 1s | <500ms |
| Shutdown Time | 500ms | 200ms | <100ms |

### Optimization Techniques

1. **Parallel Initialization**
```go
var eg errgroup.Group
eg.Go(func() error { return initRepositories() })
eg.Go(func() error { return initServices() })
eg.Go(func() error { return initHandlers() })
if err := eg.Wait(); err != nil {
    return err
}
```

2. **Lazy Loading**
```go
type LazyService struct {
    once    sync.Once
    service *Service
    initFn  func() *Service
}

func (l *LazyService) Get() *Service {
    l.once.Do(func() {
        l.service = l.initFn()
    })
    return l.service
}
```

3. **Connection Pooling**
```go
type ConnectionPool struct {
    connections chan *Connection
    factory     func() *Connection
}

func (p *ConnectionPool) Get() *Connection {
    select {
    case conn := <-p.connections:
        return conn
    default:
        return p.factory()
    }
}
```

---

*Document Version: 1.0*  
*Last Updated: 2024*  
*Author: Assistant*  
*Review Status: Ready for Implementation*