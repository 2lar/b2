# Before Phase 4: Complete Phase 3 Implementation Plan

## Overview
This document outlines the remaining steps to complete Phase 3 (Service Layer Architecture) before proceeding to Phase 4. The AI categorization features are intentionally excluded as they're not ready for integration.

## Current Status
- ✅ Phase 1 (Domain Layer) - COMPLETE
- ✅ Phase 2 (Repository Pattern) - COMPLETE  
- 🔄 Phase 3 (Service Layer Architecture) - 70% COMPLETE
- ⏳ Phase 4 (Dependency Injection) - NOT STARTED

---

## Implementation Steps

### 1. Update Container with Phase 3 Services

**File**: `backend/internal/di/container.go`

#### 1.1 Add Phase 3 fields to Container struct
```go
type Container struct {
    // ... existing fields ...
    
    // Phase 3: Application Service Layer (CQRS)
    NodeAppService   *services.NodeService
    NodeQueryService *queries.NodeQueryService
    // CategoryAppService will be added when AI is ready
    
    // ... rest of fields ...
}
```

#### 1.2 Update initializePhase3Services function
```go
// initializePhase3Services initializes the CQRS application services
func (c *Container) initializePhase3Services() {
    startTime := time.Now()
    
    // Create mock dependencies for demonstration
    c.EventBus = &MockEventBus{}
    c.ConnectionAnalyzer = domainServices.NewConnectionAnalyzer(0.3)
    
    // Create mock UnitOfWork components
    transactionProvider := &MockTransactionProvider{}
    eventPublisher := &MockEventPublisher{}
    repositoryFactory := &MockTransactionalRepositoryFactory{}
    
    c.UnitOfWork = repository.NewUnitOfWork(transactionProvider, eventPublisher, repositoryFactory)
    
    // Create repository adapters
    nodeAdapter := adapters.NewNodeRepositoryAdapter(c.NodeRepository, c.TransactionalRepository)
    edgeAdapter := adapters.NewEdgeRepositoryAdapter(c.EdgeRepository)
    categoryAdapter := adapters.NewCategoryRepositoryAdapter(c.CategoryRepository)
    graphAdapter := adapters.NewGraphRepositoryAdapter(c.GraphRepository)
    
    // Create NodeCategoryRepository using the factory
    nodeCategoryRepo := repositoryFactory.CreateNodeCategoryRepository(nil)
    nodeCategoryAdapter := adapters.NewNodeCategoryRepositoryAdapter(nodeCategoryRepo)
    
    // Create unit of work adapter
    uowAdapter := adapters.NewUnitOfWorkAdapter(
        c.UnitOfWork, 
        nodeAdapter, 
        edgeAdapter, 
        categoryAdapter, 
        graphAdapter, 
        nodeCategoryAdapter,
    )
    
    // Initialize Node Application Service (Command side)
    c.NodeAppService = services.NewNodeService(
        nodeAdapter,
        c.EdgeRepository,
        uowAdapter,
        c.EventBus,
        c.ConnectionAnalyzer,
        c.IdempotencyStore,
    )
    
    // Initialize Node Query Service (Query side)
    c.NodeQueryService = queries.NewNodeQueryService(
        nodeAdapter,
        c.EdgeRepository,
        graphAdapter,
        nil, // Cache can be nil for now
    )
    
    log.Printf("Phase 3 Application Services initialized in %v", time.Since(startTime))
}
```

#### 1.3 Update initializeServices to use migration adapter
```go
func (c *Container) initializeServices() {
    // First, initialize the legacy service for operations not yet migrated
    legacyMemoryService := memoryService.NewServiceWithIdempotency(
        c.NodeRepository,
        c.EdgeRepository,
        c.KeywordRepository,
        c.TransactionalRepository,
        c.GraphRepository,
        c.IdempotencyStore,
    )
    
    // Initialize Phase 3 CQRS services
    c.initializePhase3Services()
    
    // Create the migration adapter that uses new services where available
    if c.NodeAppService != nil && c.NodeQueryService != nil {
        // Use the adapter for gradual migration
        c.MemoryService = adapters.NewMemoryServiceAdapter(
            c.NodeAppService,
            c.NodeQueryService,
            legacyMemoryService,
        )
        log.Println("Using CQRS-based MemoryService with migration adapter")
    } else {
        // Fallback to legacy service if CQRS services aren't ready
        c.MemoryService = legacyMemoryService
        log.Println("Using legacy MemoryService")
    }
    
    // Category service remains unchanged until AI integration is ready
    c.CategoryService = categoryService.NewEnhancedService(c.Repository, nil)
}
```

---

### 2. Create Service Migration Adapter

**File**: `backend/internal/application/adapters/service_migration_adapter.go`

Create a new file with the MemoryServiceAdapter that bridges old and new services:

```go
package adapters

import (
    "context"
    "brain2-backend/internal/application/commands"
    "brain2-backend/internal/application/dto"
    "brain2-backend/internal/application/queries"
    "brain2-backend/internal/application/services"
    memoryService "brain2-backend/internal/service/memory"
)

// MemoryServiceAdapter adapts the new CQRS services to the old MemoryService interface
type MemoryServiceAdapter struct {
    nodeAppService   *services.NodeService
    nodeQueryService *queries.NodeQueryService
    legacyService    memoryService.Service
}

// Implementation includes:
// - CreateNode (uses new CQRS command)
// - GetNode (uses new CQRS query)
// - UpdateNode (uses new CQRS command)
// - DeleteNode (uses new CQRS command)
// - GetNodes (uses new CQRS query)
// - Other methods delegate to legacy service
```

---

### 3. Add Missing NodeService Methods

**File**: `backend/internal/application/services/node_service.go`

Add the DeleteNode method and helper methods for idempotency:

```go
// DeleteNode implements the use case for deleting a node and its connections
func (s *NodeService) DeleteNode(ctx context.Context, cmd *commands.DeleteNodeCommand) (*dto.DeleteNodeResult, error) {
    // Implementation includes:
    // 1. Start unit of work
    // 2. Handle idempotency
    // 3. Parse domain identifiers
    // 4. Verify node exists and user owns it
    // 5. Delete associated edges
    // 6. Delete the node
    // 7. Emit domain event
    // 8. Commit transaction
    // 9. Return result
}

// Helper methods
func (s *NodeService) checkIdempotency(ctx context.Context, key, operation, userID string) (interface{}, bool, error)
func (s *NodeService) storeIdempotencyResult(ctx context.Context, key, operation, userID string, result interface{})
```

---

### 4. Complete Edge Repository Adapter

**File**: `backend/internal/application/adapters/repository_adapters.go`

Add the edge repository adapter implementation:

```go
type edgeRepositoryAdapter struct {
    edgeRepo repository.EdgeRepository
}

func NewEdgeRepositoryAdapter(edgeRepo repository.EdgeRepository) EdgeRepositoryAdapter {
    return &edgeRepositoryAdapter{
        edgeRepo: edgeRepo,
    }
}

// Methods to implement:
// - Save(ctx context.Context, edge *domain.Edge) error
// - DeleteByNodeID(ctx context.Context, nodeID domain.NodeID) error
// - CountBySourceID(ctx context.Context, nodeID domain.NodeID) (int, error)
```

---

### 5. Create Mock Implementations

**File**: `backend/internal/di/mocks.go`

Create a new file with mock implementations for testing:

```go
package di

import (
    "context"
    "brain2-backend/internal/domain"
    "brain2-backend/internal/repository"
)

// MockEventBus implements domain.EventBus for testing
type MockEventBus struct{}

func (m *MockEventBus) Publish(ctx context.Context, event domain.DomainEvent) error {
    // Log the event for debugging
    return nil
}

// MockTransactionProvider implements transaction provider interface
type MockTransactionProvider struct{}

func (m *MockTransactionProvider) BeginTransaction(ctx context.Context) (interface{}, error) {
    return nil, nil
}

// MockEventPublisher implements event publisher interface
type MockEventPublisher struct{}

func (m *MockEventPublisher) PublishEvents(ctx context.Context, events []domain.DomainEvent) error {
    return nil
}

// MockTransactionalRepositoryFactory implements repository factory
type MockTransactionalRepositoryFactory struct{}

func (m *MockTransactionalRepositoryFactory) CreateNodeRepository(tx interface{}) repository.NodeRepository {
    // Return the existing repository for now
    return nil
}

func (m *MockTransactionalRepositoryFactory) CreateNodeCategoryRepository(tx interface{}) repository.NodeCategoryRepository {
    // Return a mock implementation
    return &MockNodeCategoryRepository{}
}

// MockNodeCategoryRepository implements NodeCategoryRepository
type MockNodeCategoryRepository struct{}

// Implement required methods...
```

---

### 6. Add Domain Events

**File**: `backend/internal/domain/events.go`

Ensure these domain events are defined:

```go
// NodeDeletedEvent represents a node deletion
type NodeDeletedEvent struct {
    NodeID    NodeID
    UserID    UserID
    DeletedAt time.Time
}

func NewNodeDeletedEvent(nodeID NodeID, userID UserID) DomainEvent {
    return NodeDeletedEvent{
        NodeID:    nodeID,
        UserID:    userID,
        DeletedAt: time.Now(),
    }
}

func (e NodeDeletedEvent) EventType() string {
    return "node.deleted"
}

func (e NodeDeletedEvent) AggregateID() string {
    return e.NodeID.String()
}

func (e NodeDeletedEvent) OccurredAt() time.Time {
    return e.DeletedAt
}
```

---

## Testing Plan

### 1. Integration Tests

**File**: `backend/internal/application/services/node_service_test.go`

Create integration tests for the CQRS flow:

```go
func TestNodeService_CreateNode_CQRS(t *testing.T) {
    // Test the complete CQRS flow for node creation
}

func TestNodeService_UpdateNode_CQRS(t *testing.T) {
    // Test the complete CQRS flow for node updates
}

func TestNodeService_DeleteNode_CQRS(t *testing.T) {
    // Test the complete CQRS flow for node deletion
}
```

### 2. Migration Adapter Tests

**File**: `backend/internal/application/adapters/service_migration_adapter_test.go`

Test that the adapter correctly bridges old and new services:

```go
func TestMemoryServiceAdapter_CreateNode(t *testing.T) {
    // Verify adapter correctly converts between old and new formats
}

func TestMemoryServiceAdapter_GetNode(t *testing.T) {
    // Verify query delegation works correctly
}
```

---

## Verification Checklist

Before proceeding to Phase 4, verify:

- [ ] Container successfully initializes with Phase 3 services
- [ ] NodeAppService handles all CRUD operations
- [ ] NodeQueryService handles all read operations
- [ ] Migration adapter allows existing handlers to work unchanged
- [ ] All tests pass
- [ ] No regression in existing functionality
- [ ] Logs show "Using CQRS-based MemoryService with migration adapter"

---

## Benefits of This Approach

1. **Gradual Migration**: Existing code continues to work while new CQRS patterns are introduced
2. **No Breaking Changes**: Handlers don't need immediate updates
3. **Easy Rollback**: Can switch back to legacy service if issues arise
4. **Clear Separation**: Commands and queries are properly separated
5. **Ready for Phase 4**: Clean dependency injection patterns are in place

---

## Next Phase Preview

Once this is complete, Phase 4 (Dependency Injection Perfection) will:
- Replace mock implementations with proper providers
- Implement factory patterns for service creation
- Add decorator pattern for cross-cutting concerns
- Consider Wire for dependency injection automation

---

## Notes

- AI categorization features are intentionally excluded and will be added when ready
- The migration adapter pattern allows for incremental adoption of CQRS
- Mock implementations are temporary and will be replaced in Phase 4
- Focus is on maintaining system stability while improving architecture