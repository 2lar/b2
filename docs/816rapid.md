# Critical Fixes for Node Creation Errors

## Fix 1: Idempotency Store Type Handling

**File: `internal/application/services/node_service.go`**

Replace the broken idempotency check in `CreateNode` method:

```go
func (s *NodeService) CreateNode(ctx context.Context, cmd *commands.CreateNodeCommand) (*dto.CreateNodeResult, error) {
    // 1. Check idempotency first
    if cmd.IdempotencyKey != nil && *cmd.IdempotencyKey != "" {
        result, exists, err := s.checkIdempotency(ctx, *cmd.IdempotencyKey, "create_node", cmd.UserID)
        if err != nil {
            return nil, appErrors.Wrap(err, "failed to check idempotency")
        }
        
        if exists && result != nil {
            // FIX: Handle both possible return types from idempotency store
            switch v := result.(type) {
            case *dto.CreateNodeResult:
                // Direct type - just return it
                return v, nil
            case map[string]interface{}:
                // JSON deserialized - reconstruct it
                reconstructed, err := s.reconstructCreateNodeResult(v)
                if err != nil {
                    // If reconstruction fails, proceed with new creation
                    // Log the error but don't fail
                    s.logger.Warn("Failed to reconstruct cached result, creating new",
                        zap.Error(err),
                        zap.String("idempotency_key", *cmd.IdempotencyKey))
                } else {
                    return reconstructed, nil
                }
            default:
                // Unknown type - log and proceed with new creation
                s.logger.Warn("Unexpected type from idempotency store",
                    zap.String("type", fmt.Sprintf("%T", result)))
            }
        }
    }
    
    // ... rest of the method remains the same
}
```

## Fix 2: Transaction State Management

**File: `internal/application/adapters/repository_adapters.go`**

Fix the UnitOfWork adapter to handle transaction state properly:

```go
type unitOfWorkAdapter struct {
    unitOfWork          repository.UnitOfWork
    baseNodeAdapter     NodeRepositoryAdapter
    baseEdgeAdapter     EdgeRepositoryAdapter
    baseCategoryAdapter CategoryRepositoryAdapter
    baseGraphAdapter    GraphRepositoryAdapter
    baseNodeCategoryAdapter NodeCategoryRepositoryAdapter
    
    // Transaction state management
    mu                  sync.Mutex
    isTransactionActive bool
    transactionContext  context.Context
    
    // Transactional adapters (created during Begin)
    nodeAdapter         NodeRepositoryAdapter
    edgeAdapter         EdgeRepositoryAdapter
    categoryAdapter     CategoryRepositoryAdapter
    graphAdapter        GraphRepositoryAdapter
    nodeCategoryAdapter NodeCategoryRepositoryAdapter
}

func (a *unitOfWorkAdapter) Begin(ctx context.Context) error {
    a.mu.Lock()
    defer a.mu.Unlock()
    
    // Check if transaction is already active
    if a.isTransactionActive {
        return fmt.Errorf("transaction already in progress")
    }
    
    // Begin the underlying transaction
    if err := a.unitOfWork.Begin(ctx); err != nil {
        return err
    }
    
    // Set transaction state
    a.isTransactionActive = true
    a.transactionContext = ctx
    
    // Clear any previous transactional adapters
    a.nodeAdapter = nil
    a.edgeAdapter = nil
    a.categoryAdapter = nil
    a.graphAdapter = nil
    a.nodeCategoryAdapter = nil
    
    return nil
}

func (a *unitOfWorkAdapter) Commit() error {
    a.mu.Lock()
    defer a.mu.Unlock()
    
    if !a.isTransactionActive {
        return fmt.Errorf("no active transaction to commit")
    }
    
    err := a.unitOfWork.Commit()
    
    // Always reset state, even if commit fails
    a.resetTransactionState()
    
    return err
}

func (a *unitOfWorkAdapter) Rollback() error {
    a.mu.Lock()
    defer a.mu.Unlock()
    
    if !a.isTransactionActive {
        // Rollback on inactive transaction is a no-op (not an error)
        return nil
    }
    
    err := a.unitOfWork.Rollback()
    
    // Always reset state
    a.resetTransactionState()
    
    return err
}

func (a *unitOfWorkAdapter) resetTransactionState() {
    a.isTransactionActive = false
    a.transactionContext = nil
    a.nodeAdapter = nil
    a.edgeAdapter = nil
    a.categoryAdapter = nil
    a.graphAdapter = nil
    a.nodeCategoryAdapter = nil
}

// Add a method to check transaction state
func (a *unitOfWorkAdapter) IsActive() bool {
    a.mu.Lock()
    defer a.mu.Unlock()
    return a.isTransactionActive
}
```

## Fix 3: Add Proper Error Recovery

**File: `internal/application/services/node_service.go`**

Add transaction recovery for the defer:

```go
func (s *NodeService) CreateNode(ctx context.Context, cmd *commands.CreateNodeCommand) (*dto.CreateNodeResult, error) {
    // ... idempotency check ...
    
    // 2. Start unit of work
    if err := s.uow.Begin(ctx); err != nil {
        return nil, appErrors.Wrap(err, "failed to begin transaction")
    }
    
    // FIX: Ensure clean rollback even if panic occurs
    defer func() {
        if r := recover(); r != nil {
            // Attempt rollback on panic
            if rollbackErr := s.uow.Rollback(); rollbackErr != nil {
                s.logger.Error("Failed to rollback after panic",
                    zap.Any("panic", r),
                    zap.Error(rollbackErr))
            }
            // Re-panic to let it bubble up
            panic(r)
        } else if s.uow.IsActive() {
            // Only rollback if transaction is still active
            s.uow.Rollback()
        }
    }()
    
    // ... rest of the method
}
```

## Fix 4: Simplify Node Creation Flow (Remove Excess Adapters)

Instead of having multiple adapter layers, simplify the flow:

```go
// Before: Handler -> Service -> UoW Adapter -> Node Adapter -> Repository
// After:  Handler -> Service -> Repository (with transaction context)

// Add transaction context to repository calls directly
func (s *NodeService) CreateNode(ctx context.Context, cmd *commands.CreateNodeCommand) (*dto.CreateNodeResult, error) {
    // ... validation ...
    
    // Start transaction
    tx, err := s.transactionManager.Begin(ctx)
    if err != nil {
        return nil, err
    }
    defer tx.Rollback()
    
    // Pass transaction context directly
    txCtx := context.WithValue(ctx, "tx", tx)
    
    // Direct repository call with transaction context
    if err := s.nodeRepo.Save(txCtx, node); err != nil {
        return nil, err
    }
    
    // Create edges
    for _, edge := range connections {
        if err := s.edgeRepo.Save(txCtx, edge); err != nil {
            return nil, err
        }
    }
    
    // Commit
    if err := tx.Commit(); err != nil {
        return nil, err
    }
    
    // ... return result
}
```

## Testing the Fixes

After applying these fixes, test with:

```bash
# Test rapid succession creates
for i in {1..5}; do
    curl -X POST https://your-api/api/nodes \
        -H "Authorization: Bearer $TOKEN" \
        -H "Content-Type: application/json" \
        -H "X-Idempotency-Key: test-$i" \
        -d '{"content": "Test node '$i'", "tags": ["test"]}' &
done
wait
```

## Long-term Solution: Remove Adapter Layers

The adapters are multiplying because you're trying to bridge incompatible interfaces. The solution is to:

1. **Complete the CQRS migration** - Have repositories implement the CQRS interfaces directly
2. **Fix the repository interfaces** - Make them match what the services need
3. **Remove bridge adapters** - They're temporary scaffolding that's becoming permanent

### Step 1: Update Repository Interfaces

```go
// Instead of adapters, update the repository to match needs
type NodeRepository interface {
    // CQRS Write Operations
    Save(ctx context.Context, node *domain.Node) error
    Update(ctx context.Context, node *domain.Node) error
    Delete(ctx context.Context, id domain.NodeID) error
    
    // CQRS Read Operations
    FindByID(ctx context.Context, userID domain.UserID, nodeID domain.NodeID) (*domain.Node, error)
    FindByUser(ctx context.Context, userID domain.UserID, opts ...QueryOption) ([]*domain.Node, error)
    
    // Remove old methods that require adaptation
    // NO: FindNodeByID(ctx, userID string, nodeID string)
}
```

### Step 2: Implement Directly in DynamoDB Repository

```go
// infrastructure/dynamodb/node_repository.go
type dynamoDBNodeRepository struct {
    client *dynamodb.Client
    table  string
}

// Implement new interface directly
func (r *dynamoDBNodeRepository) Save(ctx context.Context, node *domain.Node) error {
    // Direct implementation, no adapter needed
}

func (r *dynamoDBNodeRepository) FindByID(ctx context.Context, userID domain.UserID, nodeID domain.NodeID) (*domain.Node, error) {
    // Direct implementation using domain types
}
```

### Step 3: Remove All Adapters

Once repositories implement the correct interfaces, delete:
- `NodeRepositoryAdapter`
- `NodeReaderBridge`
- `MemoryServiceAdapter`
- All other adapter files

## Summary

The immediate fixes will solve your critical issues:
1. **Idempotency panic** - Fixed by handling both return types
2. **Transaction errors** - Fixed by proper state management
3. **Performance issues** - Fixed by preventing transaction conflicts

The long-term solution is to complete the CQRS migration and remove all adapter layers. Adapters should be temporary migration tools, not permanent architecture.