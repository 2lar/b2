# Context Usage to Explicit UserID Migration Guide

## Executive Summary

This guide documents the migration from context-based userID passing to explicit parameter passing throughout the Brain2 backend. This migration improves type safety, testability, performance, and code clarity.

## Current State Analysis

### Problems with Context-Based Approach

1. **Hidden Dependencies**: UserID dependency not visible in method signatures
2. **Runtime Errors**: Missing userID only discovered at runtime
3. **Testing Complexity**: Tests require context setup with specific values
4. **Performance Overhead**: Context lookups at multiple layers
5. **Unclear Data Flow**: Hard to track where userID is used

### Statistics
- **103 occurrences** of `context.WithValue`/`context.Value`
- **21 files** affected
- **10+ repository methods** extracting userID from context
- **Multiple layers** performing redundant extractions

## Target Architecture

### Improved Pattern

```go
// BEFORE: Context-based (hidden dependency)
func (r *NodeRepository) FindByID(ctx context.Context, nodeID string) (*Node, error) {
    userID, ok := GetUserIDFromContext(ctx) // Runtime extraction
    if !ok {
        return nil, errors.New("userID not found")
    }
    // ...
}

// AFTER: Explicit parameter (clear dependency)
func (r *NodeRepository) FindByID(ctx context.Context, userID, nodeID string) (*Node, error) {
    // UserID is guaranteed to be present
    // ...
}
```

### Benefits

1. ✅ **Compile-time Safety**: Missing userID caught during compilation
2. ✅ **Clear Dependencies**: Method signatures show all requirements
3. ✅ **Easy Testing**: No context mocking needed
4. ✅ **Better Performance**: No context lookups
5. ✅ **Clear Data Flow**: Explicit parameter passing

## Migration Plan

### Phase 1: Repository Layer (Week 1)

#### Step 1.1: Create New Interfaces
- [x] Create `NodeReaderV2` and `NodeWriterV2` with explicit userID
- [x] Create `EdgeReaderV2` and `EdgeWriterV2` with explicit userID
- [x] Category interfaces already have explicit userID

**Files Created:**
- `internal/repository/interfaces_improved.go`

#### Step 1.2: Implement New Repositories
- [x] Create `NodeRepositoryImproved` implementing V2 interfaces
- [ ] Create `EdgeRepositoryImproved` implementing V2 interfaces
- [ ] Update existing implementations gradually

**Files Created:**
- `internal/infrastructure/persistence/dynamodb/node_repository_improved.go`

#### Step 1.3: Create Adapters for Transition
- [x] Implement `NodeReaderAdapter` to bridge old and new interfaces
- [ ] Implement `EdgeReaderAdapter` similarly
- [ ] Use adapters to allow gradual migration

### Phase 2: Service Layer (Week 1-2)

#### Step 2.1: Update Command Objects
- [x] Ensure all commands have UserID field
- [x] Commands already properly structured

**Existing Files:**
- `internal/application/commands/node_commands.go` ✅
- `internal/application/commands/category_commands.go` ✅

#### Step 2.2: Update Services
- [x] Create `NodeServiceImproved` using V2 repositories
- [ ] Update `CategoryService` to use explicit userID
- [ ] Update query services similarly

**Files Created:**
- `internal/application/services/node_service_improved.go`

### Phase 3: Handler Layer (Week 2)

#### Step 3.1: Update Handlers
- [x] Create `MemoryHandlerImproved` as example
- [ ] Update all handlers to extract userID once
- [ ] Pass userID explicitly to services

**Files Created:**
- `internal/handlers/memory_improved.go`

#### Step 3.2: Maintain Security Boundary
- Keep userID extraction at handler layer only
- This is the authentication/authorization boundary
- Services should trust the userID they receive

### Phase 4: Dependency Injection (Week 2-3)

#### Step 4.1: Update Wire Configuration
```go
// Add new providers for improved implementations
var ImprovedProviders = wire.NewSet(
    NewNodeRepositoryImproved,
    NewNodeServiceImproved,
    NewMemoryHandlerImproved,
)
```

#### Step 4.2: Gradual Switchover
1. Deploy new implementations alongside old ones
2. Use feature flags to control which version is used
3. Monitor for issues
4. Gradually increase traffic to new implementation
5. Remove old implementation once stable

### Phase 5: Cleanup (Week 3)

#### Step 5.1: Remove Old Code
- [ ] Delete old repository implementations
- [ ] Delete old service implementations
- [ ] Remove context helper functions
- [ ] Update all tests

#### Step 5.2: Documentation
- [ ] Update API documentation
- [ ] Update architecture diagrams
- [ ] Create developer guide for new pattern

## Implementation Examples

### Repository Implementation

```go
// Before
func (r *NodeRepository) FindByID(ctx context.Context, id NodeID) (*Node, error) {
    userID, ok := GetUserIDFromContext(ctx)
    if !ok {
        return nil, ErrUserIDNotFound
    }
    // Query logic
}

// After
func (r *NodeRepositoryImproved) FindByID(ctx context.Context, userID UserID, nodeID NodeID) (*Node, error) {
    // Direct usage, no extraction needed
    input := &dynamodb.GetItemInput{
        Key: map[string]types.AttributeValue{
            "PK": &types.AttributeValueMemberS{Value: fmt.Sprintf("USER#%s", userID)},
            "SK": &types.AttributeValueMemberS{Value: fmt.Sprintf("NODE#%s", nodeID)},
        },
    }
    // ...
}
```

### Service Implementation

```go
// Before
func (s *NodeService) GetNode(ctx context.Context, nodeID string) (*NodeDTO, error) {
    // Repository extracts userID from context internally
    node, err := s.nodeRepo.FindByID(ctx, nodeID)
    // ...
}

// After
func (s *NodeServiceImproved) GetNode(ctx context.Context, userID, nodeID string) (*NodeDTO, error) {
    // Pass userID explicitly
    node, err := s.nodeRepo.FindByID(ctx, userID, nodeID)
    // ...
}
```

### Handler Implementation

```go
// Handler remains the security boundary
func (h *MemoryHandlerImproved) GetNode(w http.ResponseWriter, r *http.Request) {
    // Extract userID ONCE at handler level
    userID, ok := GetUserIDFromContext(r.Context())
    if !ok {
        api.Error(w, http.StatusUnauthorized, "Authentication required")
        return
    }
    
    nodeID := mux.Vars(r)["id"]
    
    // Pass explicitly to service
    node, err := h.nodeService.GetNode(r.Context(), userID, nodeID)
    // ...
}
```

## Testing Improvements

### Before: Complex Context Setup

```go
func TestGetNode(t *testing.T) {
    // Need to setup context with userID
    ctx := context.WithValue(context.Background(), "userID", "test-user")
    
    // Test with context
    node, err := service.GetNode(ctx, "node-123")
    assert.NoError(t, err)
}
```

### After: Simple Direct Testing

```go
func TestGetNode(t *testing.T) {
    // Direct parameter passing, no context setup
    node, err := service.GetNode(context.Background(), "test-user", "node-123")
    assert.NoError(t, err)
}
```

## Migration Checklist

### Repository Layer
- [ ] Create V2 interfaces with explicit userID
- [ ] Implement new repositories
- [ ] Create adapters for transition
- [ ] Test new implementations

### Service Layer
- [ ] Update command objects (if needed)
- [ ] Create improved services
- [ ] Update dependency injection
- [ ] Test service layer

### Handler Layer
- [ ] Update handlers to extract userID once
- [ ] Pass userID explicitly to services
- [ ] Maintain security boundary
- [ ] Test end-to-end

### Infrastructure
- [ ] Update Wire configuration
- [ ] Add feature flags for gradual rollout
- [ ] Update monitoring and logging
- [ ] Performance testing

### Cleanup
- [ ] Remove old implementations
- [ ] Delete context helper functions
- [ ] Update all tests
- [ ] Update documentation

## Risk Mitigation

### Risks
1. **Breaking Changes**: Changing method signatures
2. **Missed Updates**: Some code paths not updated
3. **Performance Impact**: During transition period

### Mitigation Strategies
1. **Adapter Pattern**: Use adapters to maintain compatibility
2. **Gradual Migration**: Update one layer at a time
3. **Feature Flags**: Control rollout percentage
4. **Comprehensive Testing**: Test each phase thoroughly
5. **Monitoring**: Watch for errors during migration

## Success Metrics

### Quantitative Metrics
- ✅ Zero `context.WithValue` calls for userID
- ✅ 100% of repository methods with explicit userID
- ✅ Reduced test complexity (measured by lines of test setup)
- ✅ Performance improvement (no context lookups)

### Qualitative Metrics
- ✅ Improved code readability
- ✅ Better developer experience
- ✅ Easier onboarding for new developers
- ✅ Reduced debugging time

## Timeline

### Week 1
- Repository layer migration
- Create adapters
- Initial testing

### Week 2
- Service layer migration
- Handler updates
- Integration testing

### Week 3
- Deployment with feature flags
- Monitoring and adjustment
- Cleanup and documentation

## Conclusion

This migration from context-based to explicit userID passing represents a significant improvement in code quality, maintainability, and performance. The gradual migration approach using adapters ensures zero downtime and allows for safe rollback if issues arise.

The investment in this refactoring will pay dividends in:
- Reduced bugs from missing userID
- Easier testing and maintenance
- Better performance
- Clearer code architecture

## Appendix: File List

### New Files Created
1. `internal/repository/interfaces_improved.go` - New V2 interfaces
2. `internal/infrastructure/persistence/dynamodb/node_repository_improved.go` - Improved implementation
3. `internal/application/services/node_service_improved.go` - Improved service
4. `internal/handlers/memory_improved.go` - Improved handler
5. `docs/migration/context-to-explicit-userid.md` - This guide

### Files to Modify
1. `internal/di/wire.go` - Add new providers
2. `internal/di/factories.go` - Update factories
3. Various test files - Update tests to use new pattern

### Files to Eventually Remove
1. `internal/context/keys.go` - Context helpers (after migration)
2. Old repository implementations
3. Old service implementations