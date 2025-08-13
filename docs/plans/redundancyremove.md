# Brain2 Codebase Consolidation & Optimization Plan

## Overview
This document provides a **safe, step-by-step implementation plan** to consolidate the Brain2 codebase, eliminating redundancy while maintaining functionality. Each step includes verification procedures to prevent breaking changes.

## ⚠️ Critical Issues Identified

### Root Causes of 500 Errors:
1. **Circular Dependencies**: Methods calling each other in loops
2. **Nil Idempotency Store**: Not properly initialized in container
3. **Version Mismatch**: Version 0 vs Version 1 causing optimistic lock failures
4. **Missing Error Handling**: Silent failures in fallback logic

---

## Phase 0: Pre-Consolidation Fixes (MUST DO FIRST)

### Fix 1: Initialize Idempotency Store in Container
**File**: `backend/internal/di/container.go`

```go
// In initializeRepository() method, ADD this:
func (c *Container) initializeRepository() error {
    // ... existing code ...
    
    // Initialize idempotency store - THIS WAS MISSING!
    c.IdempotencyStore = repository.NewInMemoryIdempotencyStore()
    
    return nil
}
```

### Fix 2: Consistent Version Initialization
**File**: `backend/internal/domain/node.go`

```go
// Add a constructor for consistent initialization
func NewNode(userID, content string, tags []string) Node {
    return Node{
        ID:        uuid.New().String(),
        UserID:    userID,
        Content:   content,
        Keywords:  []string{}, // Will be set by service
        Tags:      tags,
        CreatedAt: time.Now(),
        Version:   0, // ALWAYS start at 0 for new nodes
    }
}
```

### Fix 3: Fix Version Increment in Repository
**File**: `backend/infrastructure/dynamodb/ddb.go`

```go
// In CreateNodeWithEdges method, ensure version is set correctly:
nodeItem, err := attributevalue.MarshalMap(ddbNode{
    PK:       pk,
    SK:       "METADATA#v0",
    NodeID:   node.ID,
    UserID:   node.UserID,
    Content:  node.Content,
    Keywords: node.Keywords,
    Tags:     node.Tags,
    IsLatest: true,
    Version:  0, // New nodes ALWAYS start at version 0
    Timestamp: node.CreatedAt.Format(time.RFC3339),
})
```

---

## Phase 1: Service Layer Consolidation

### Step 1.1: Create Internal Helper Methods
**File**: `backend/internal/service/memory/service_internal.go` (NEW FILE)

```go
package memory

import (
    "context"
    "brain2-backend/internal/domain"
    "brain2-backend/internal/repository"
    appErrors "brain2-backend/pkg/errors"
)

// internal methods that won't be exposed in the interface

// createNodeCore handles the actual node creation logic
func (s *service) createNodeCore(ctx context.Context, userID, content string, tags []string) (*domain.Node, []domain.Edge, error) {
    if content == "" {
        return nil, nil, appErrors.NewValidation("content cannot be empty")
    }

    keywords := ExtractKeywords(content)
    node := domain.NewNode(userID, content, tags)
    node.Keywords = keywords

    // Find related nodes
    relatedNodes, err := s.keywordRepo.FindNodesByKeywords(ctx, userID, keywords)
    if err != nil {
        // Log but don't fail - connections are non-critical
        log.Printf("WARN: Failed to find related nodes: %v", err)
        relatedNodes = []domain.Node{}
    }

    var relatedNodeIDs []string
    for _, rn := range relatedNodes {
        if rn.ID != node.ID {
            relatedNodeIDs = append(relatedNodeIDs, rn.ID)
        }
    }

    // Create node with edges in transaction
    if err := s.transactionRepo.CreateNodeWithEdges(ctx, node, relatedNodeIDs); err != nil {
        return nil, nil, appErrors.Wrap(err, "failed to create node in repository")
    }

    // Build edge list for response
    edges := make([]domain.Edge, 0, len(relatedNodeIDs))
    for _, relatedID := range relatedNodeIDs {
        edges = append(edges, domain.Edge{
            SourceID: node.ID,
            TargetID: relatedID,
        })
    }

    return &node, edges, nil
}

// updateNodeCore handles the actual update logic
func (s *service) updateNodeCore(ctx context.Context, userID, nodeID, content string, tags []string) (*domain.Node, error) {
    if content == "" {
        return nil, appErrors.NewValidation("content cannot be empty")
    }

    // Fetch current node
    existingNode, err := s.nodeRepo.FindNodeByID(ctx, userID, nodeID)
    if err != nil {
        return nil, appErrors.Wrap(err, "failed to find node")
    }
    if existingNode == nil {
        return nil, appErrors.NewNotFound("node not found")
    }

    // Prepare updated node
    keywords := ExtractKeywords(content)
    updatedNode := *existingNode // Copy existing node
    updatedNode.Content = content
    updatedNode.Keywords = keywords
    updatedNode.Tags = tags
    // Version will be incremented by repository layer

    // Find new connections
    relatedNodes, err := s.keywordRepo.FindNodesByKeywords(ctx, userID, keywords)
    if err != nil {
        log.Printf("WARN: Failed to find related nodes for update: %v", err)
        relatedNodes = []domain.Node{}
    }

    var relatedNodeIDs []string
    for _, rn := range relatedNodes {
        if rn.ID != nodeID {
            relatedNodeIDs = append(relatedNodeIDs, rn.ID)
        }
    }

    // Update with optimistic locking
    if err := s.transactionRepo.UpdateNodeAndEdges(ctx, updatedNode, relatedNodeIDs); err != nil {
        return nil, err
    }

    return &updatedNode, nil
}

// bulkDeleteCore handles the actual bulk delete logic
func (s *service) bulkDeleteCore(ctx context.Context, userID string, nodeIDs []string) (int, []string, error) {
    if len(nodeIDs) == 0 {
        return 0, nil, appErrors.NewValidation("nodeIds cannot be empty")
    }
    if len(nodeIDs) > 100 {
        return 0, nil, appErrors.NewValidation("cannot delete more than 100 nodes at once")
    }

    var failedNodeIDs []string
    deletedCount := 0

    for _, nodeID := range nodeIDs {
        if err := s.nodeRepo.DeleteNode(ctx, userID, nodeID); err != nil {
            log.Printf("Failed to delete node %s: %v", nodeID, err)
            failedNodeIDs = append(failedNodeIDs, nodeID)
            continue
        }
        deletedCount++
    }

    return deletedCount, failedNodeIDs, nil
}
```

### Step 1.2: Consolidate Public Service Methods
**File**: `backend/internal/service/memory/service.go`

```go
// KEEP ONLY THESE PUBLIC METHODS - Delete all others

// CreateNode creates a new node with automatic edge discovery and idempotency
func (s *service) CreateNode(ctx context.Context, userID, content string, tags []string) (*domain.Node, []domain.Edge, error) {
    // Check for idempotency key in context
    if idempotencyKey := GetIdempotencyKeyFromContext(ctx); idempotencyKey != "" && s.idempotencyStore != nil {
        key := repository.IdempotencyKey{
            UserID:    userID,
            Operation: "CREATE_NODE",
            Hash:      idempotencyKey,
            CreatedAt: time.Now(),
        }

        // Check if already processed
        if result, exists, _ := s.idempotencyStore.Get(ctx, key); exists {
            if nodeResult, ok := result.(*domain.Node); ok {
                // Return cached result
                edges, _ := s.edgeRepo.FindEdges(ctx, repository.EdgeQuery{
                    UserID:   userID,
                    SourceID: nodeResult.ID,
                })
                return nodeResult, edges, nil
            }
        }

        // Execute and store
        node, edges, err := s.createNodeCore(ctx, userID, content, tags)
        if err != nil {
            return nil, nil, err
        }

        s.idempotencyStore.Store(ctx, key, node)
        return node, edges, nil
    }

    // Non-idempotent path
    return s.createNodeCore(ctx, userID, content, tags)
}

// UpdateNode updates a node with automatic retry on conflicts
func (s *service) UpdateNode(ctx context.Context, userID, nodeID, content string, tags []string) (*domain.Node, error) {
    // Check for idempotency
    if idempotencyKey := GetIdempotencyKeyFromContext(ctx); idempotencyKey != "" && s.idempotencyStore != nil {
        key := repository.IdempotencyKey{
            UserID:    userID,
            Operation: "UPDATE_NODE",
            Hash:      idempotencyKey,
            CreatedAt: time.Now(),
        }

        if result, exists, _ := s.idempotencyStore.Get(ctx, key); exists {
            if nodeResult, ok := result.(*domain.Node); ok {
                return nodeResult, nil
            }
        }

        // Try with retries for optimistic locking
        var node *domain.Node
        var err error
        
        for attempt := 0; attempt < maxRetries; attempt++ {
            node, err = s.updateNodeCore(ctx, userID, nodeID, content, tags)
            if err == nil {
                break
            }
            
            if !repository.IsOptimisticLockError(err) {
                return nil, err
            }
            
            if attempt < maxRetries-1 {
                time.Sleep(baseDelay * time.Duration(1<<attempt))
            }
        }
        
        if err != nil {
            return nil, err
        }

        s.idempotencyStore.Store(ctx, key, node)
        return node, nil
    }

    // Non-idempotent path with retry
    for attempt := 0; attempt < maxRetries; attempt++ {
        node, err := s.updateNodeCore(ctx, userID, nodeID, content, tags)
        if err == nil {
            return node, nil
        }
        
        if !repository.IsOptimisticLockError(err) {
            return nil, err
        }
        
        if attempt < maxRetries-1 {
            time.Sleep(baseDelay * time.Duration(1<<attempt))
        }
    }
    
    return nil, appErrors.NewRepositoryError(repository.ErrCodeOptimisticLock, "max retries exceeded", nil)
}

// DeleteNode removes a single node
func (s *service) DeleteNode(ctx context.Context, userID, nodeID string) error {
    return s.nodeRepo.DeleteNode(ctx, userID, nodeID)
}

// BulkDeleteNodes removes multiple nodes with idempotency
func (s *service) BulkDeleteNodes(ctx context.Context, userID string, nodeIDs []string) (int, []string, error) {
    if idempotencyKey := GetIdempotencyKeyFromContext(ctx); idempotencyKey != "" && s.idempotencyStore != nil {
        key := repository.IdempotencyKey{
            UserID:    userID,
            Operation: "BULK_DELETE",
            Hash:      idempotencyKey,
            CreatedAt: time.Now(),
        }

        if result, exists, _ := s.idempotencyStore.Get(ctx, key); exists {
            if deleteResult, ok := result.(map[string]interface{}); ok {
                count := deleteResult["count"].(int)
                failed := deleteResult["failed"].([]string)
                return count, failed, nil
            }
        }

        count, failed, err := s.bulkDeleteCore(ctx, userID, nodeIDs)
        if err != nil {
            return 0, nil, err
        }

        s.idempotencyStore.Store(ctx, key, map[string]interface{}{
            "count":  count,
            "failed": failed,
        })
        return count, failed, nil
    }

    return s.bulkDeleteCore(ctx, userID, nodeIDs)
}

// GetNodes retrieves paginated nodes (single pagination method)
func (s *service) GetNodes(ctx context.Context, userID string, pageReq repository.PageRequest) (*repository.PageResponse, error) {
    query := repository.NodeQuery{
        UserID: userID,
    }
    
    // Convert to old pagination for now (until repository is updated)
    pagination := repository.Pagination{
        Limit:  pageReq.Limit,
        Cursor: pageReq.NextToken,
    }
    
    page, err := s.nodeRepo.GetNodesPage(ctx, query, pagination)
    if err != nil {
        return nil, appErrors.Wrap(err, "failed to get nodes page")
    }
    
    return &repository.PageResponse{
        Items:     page.Items,
        NextToken: page.NextCursor,
        HasMore:   page.HasMore,
    }, nil
}

// GetNodeDetails retrieves a single node with its edges
func (s *service) GetNodeDetails(ctx context.Context, userID, nodeID string) (*domain.Node, []domain.Edge, error) {
    node, err := s.nodeRepo.FindNodeByID(ctx, userID, nodeID)
    if err != nil {
        return nil, nil, err
    }
    if node == nil {
        return nil, nil, appErrors.NewNotFound("node not found")
    }

    edges, err := s.edgeRepo.FindEdges(ctx, repository.EdgeQuery{
        UserID:   userID,
        SourceID: nodeID,
    })
    if err != nil {
        return nil, nil, err
    }

    return node, edges, nil
}

// GetGraphData retrieves the complete graph
func (s *service) GetGraphData(ctx context.Context, userID string) (*domain.Graph, error) {
    return s.graphRepo.GetGraphData(ctx, repository.GraphQuery{
        UserID:       userID,
        IncludeEdges: true,
    })
}
```

### Step 1.3: Add Context Helper for Idempotency
**File**: `backend/internal/service/memory/context.go` (NEW FILE)

```go
package memory

import "context"

type contextKey string

const idempotencyKeyContext contextKey = "idempotency-key"

// WithIdempotencyKey adds an idempotency key to context
func WithIdempotencyKey(ctx context.Context, key string) context.Context {
    return context.WithValue(ctx, idempotencyKeyContext, key)
}

// GetIdempotencyKeyFromContext retrieves idempotency key from context
func GetIdempotencyKeyFromContext(ctx context.Context) string {
    if key, ok := ctx.Value(idempotencyKeyContext).(string); ok {
        return key
    }
    return ""
}
```

### Step 1.4: Update Service Interface
**File**: `backend/internal/service/memory/service.go`

```go
type Service interface {
    // Core operations - simplified interface
    CreateNode(ctx context.Context, userID, content string, tags []string) (*domain.Node, []domain.Edge, error)
    UpdateNode(ctx context.Context, userID, nodeID, content string, tags []string) (*domain.Node, error)
    DeleteNode(ctx context.Context, userID, nodeID string) error
    BulkDeleteNodes(ctx context.Context, userID string, nodeIDs []string) (int, []string, error)
    
    // Query operations
    GetNodeDetails(ctx context.Context, userID, nodeID string) (*domain.Node, []domain.Edge, error)
    GetNodes(ctx context.Context, userID string, pageReq repository.PageRequest) (*repository.PageResponse, error)
    GetGraphData(ctx context.Context, userID string) (*domain.Graph, error)
}

// DELETE ALL OTHER METHODS FROM INTERFACE
```

---

## Phase 2: Handler Layer Updates

### Step 2.1: Update Memory Handler
**File**: `backend/internal/handlers/memory.go`

```go
// CreateNode handler - simplified
func (h *MemoryHandler) CreateNode(w http.ResponseWriter, r *http.Request) {
    userID, ok := getUserID(r)
    if !ok {
        api.Error(w, http.StatusUnauthorized, "Authentication required")
        return
    }

    var req api.CreateNodeRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        api.Error(w, http.StatusBadRequest, "Invalid request body")
        return
    }

    if req.Content == "" {
        api.Error(w, http.StatusBadRequest, "Content cannot be empty")
        return
    }

    tags := []string{}
    if req.Tags != nil {
        tags = *req.Tags
    }

    // Add idempotency key to context
    ctx := r.Context()
    if idempotencyKey := r.Header.Get("Idempotency-Key"); idempotencyKey != "" {
        ctx = memory.WithIdempotencyKey(ctx, idempotencyKey)
    } else {
        // Generate automatic key
        key := generateIdempotencyKey(userID, "CREATE_NODE", req)
        ctx = memory.WithIdempotencyKey(ctx, key)
    }

    // Call simplified service method
    node, edges, err := h.memoryService.CreateNode(ctx, userID, req.Content, tags)
    if err != nil {
        handleServiceError(w, err)
        return
    }

    // Publish event
    h.publishNodeCreatedEvent(ctx, node, edges)

    api.Success(w, http.StatusCreated, api.CreateNodeResponse{
        Node:  convertToAPINode(node),
        Edges: convertToAPIEdges(edges),
    })
}

// UpdateNode handler - simplified
func (h *MemoryHandler) UpdateNode(w http.ResponseWriter, r *http.Request) {
    userID, ok := getUserID(r)
    if !ok {
        api.Error(w, http.StatusUnauthorized, "Authentication required")
        return
    }

    nodeID := chi.URLParam(r, "nodeId")
    
    var req api.UpdateNodeRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        api.Error(w, http.StatusBadRequest, "Invalid request body")
        return
    }

    tags := []string{}
    if req.Tags != nil {
        tags = *req.Tags
    }

    // Add idempotency key to context
    ctx := r.Context()
    if idempotencyKey := r.Header.Get("Idempotency-Key"); idempotencyKey != "" {
        ctx = memory.WithIdempotencyKey(ctx, idempotencyKey)
    } else {
        key := generateIdempotencyKey(userID, "UPDATE_NODE", map[string]interface{}{
            "nodeId": nodeID,
            "content": req.Content,
            "tags": tags,
        })
        ctx = memory.WithIdempotencyKey(ctx, key)
    }

    node, err := h.memoryService.UpdateNode(ctx, userID, nodeID, req.Content, tags)
    if err != nil {
        handleServiceError(w, err)
        return
    }

    api.Success(w, http.StatusOK, convertToAPINode(node))
}

// ListNodes handler - use single pagination method
func (h *MemoryHandler) ListNodes(w http.ResponseWriter, r *http.Request) {
    userID, ok := getUserID(r)
    if !ok {
        api.Error(w, http.StatusUnauthorized, "Authentication required")
        return
    }

    // Parse query parameters
    query := r.URL.Query()
    limit := 20
    if l := query.Get("limit"); l != "" {
        if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 && parsed <= 100 {
            limit = parsed
        }
    }

    pageReq := repository.PageRequest{
        Limit:     limit,
        NextToken: query.Get("nextToken"),
    }

    response, err := h.memoryService.GetNodes(r.Context(), userID, pageReq)
    if err != nil {
        handleServiceError(w, err)
        return
    }

    api.Success(w, http.StatusOK, response)
}

// DELETE these methods:
// - GetNodesPage (redundant)
// - GetNodesPageOptimized (redundant)
```

---

## Phase 3: Repository Layer Cleanup

### Step 3.1: Consolidate Repository Methods
**File**: `backend/infrastructure/dynamodb/ddb.go`

```go
// Ensure these methods work correctly:

// CreateNodeWithEdges - should handle version 0 for new nodes
func (r *ddbRepository) CreateNodeWithEdges(ctx context.Context, node domain.Node, relatedNodeIDs []string) error {
    // Ensure node starts with version 0
    node.Version = 0
    
    // ... rest of existing implementation
}

// UpdateNodeAndEdges - should increment version properly
func (r *ddbRepository) UpdateNodeAndEdges(ctx context.Context, node domain.Node, relatedNodeIDs []string) error {
    pk := fmt.Sprintf("USER#%s#NODE#%s", node.UserID, node.ID)
    
    // The update expression should increment version
    _, err := r.dbClient.UpdateItem(ctx, &dynamodb.UpdateItemInput{
        TableName: aws.String(r.config.TableName),
        Key: map[string]types.AttributeValue{
            "PK": &types.AttributeValueMemberS{Value: pk},
            "SK": &types.AttributeValueMemberS{Value: "METADATA#v0"},
        },
        UpdateExpression: aws.String("SET Content = :c, Keywords = :k, Tags = :tg, Version = Version + :inc"),
        ConditionExpression: aws.String("Version = :expected_version"),
        ExpressionAttributeValues: map[string]types.AttributeValue{
            ":c": &types.AttributeValueMemberS{Value: node.Content},
            ":k": &types.AttributeValueMemberL{Value: toAttributeValueList(node.Keywords)},
            ":tg": &types.AttributeValueMemberL{Value: toAttributeValueList(node.Tags)},
            ":expected_version": &types.AttributeValueMemberN{Value: fmt.Sprintf("%d", node.Version)},
            ":inc": &types.AttributeValueMemberN{Value: "1"},
        },
    })
    
    // ... handle error and edges
}
```

---

## Phase 4: Testing & Verification

### Step 4.1: Create Integration Test
**File**: `backend/internal/service/memory/integration_test.go` (NEW)

```go
package memory_test

import (
    "context"
    "testing"
    "brain2-backend/internal/service/memory"
    "brain2-backend/internal/repository/mocks"
    "github.com/stretchr/testify/assert"
)

func TestConsolidatedServiceFlow(t *testing.T) {
    // Setup
    mockRepo := mocks.NewMockRepository()
    service := memory.NewServiceFromRepository(mockRepo)
    ctx := context.Background()
    userID := "test-user"

    t.Run("Create -> Update -> Delete Flow", func(t *testing.T) {
        // Create
        node, edges, err := service.CreateNode(ctx, userID, "Test content", []string{"test"})
        assert.NoError(t, err)
        assert.NotNil(t, node)
        assert.Equal(t, 0, node.Version)

        // Update
        updated, err := service.UpdateNode(ctx, userID, node.ID, "Updated content", []string{"updated"})
        assert.NoError(t, err)
        assert.NotNil(t, updated)
        assert.Equal(t, 1, updated.Version)

        // Delete
        err = service.DeleteNode(ctx, userID, node.ID)
        assert.NoError(t, err)
    })

    t.Run("Idempotency", func(t *testing.T) {
        idempotencyKey := "test-key-123"
        ctx := memory.WithIdempotencyKey(context.Background(), idempotencyKey)

        // First call
        node1, _, err := service.CreateNode(ctx, userID, "Idempotent test", []string{})
        assert.NoError(t, err)

        // Second call with same key - should return same result
        node2, _, err := service.CreateNode(ctx, userID, "Different content", []string{})
        assert.NoError(t, err)
        assert.Equal(t, node1.ID, node2.ID)
    })
}
```

### Step 4.2: Manual Testing Checklist

```bash
# 1. Test node creation
curl -X POST http://localhost:8080/api/nodes \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"content": "Test node", "tags": ["test"]}'

# 2. Test node update
curl -X PUT http://localhost:8080/api/nodes/{nodeId} \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"content": "Updated content", "tags": ["updated"]}'

# 3. Test listing
curl http://localhost:8080/api/nodes?limit=10 \
  -H "Authorization: Bearer $TOKEN"

# 4. Test idempotency
curl -X POST http://localhost:8080/api/nodes \
  -H "Authorization: Bearer $TOKEN" \
  -H "Idempotency-Key: test-123" \
  -H "Content-Type: application/json" \
  -d '{"content": "Idempotent node", "tags": []}'
```

---

## Phase 5: Cleanup

### Files to DELETE:
```bash
backend/internal/service/memory/optimistic_retry.go  # Entire file
backend/internal/service/memory/idempotent_operations.go  # If exists
```

### Methods to DELETE from service.go:
- `CreateNodeAndKeywords`
- `CreateNodeWithEdges`
- `CreateNodeWithEdgesIdempotent`
- `UpdateNodeIdempotent`
- `UpdateNodeWithRetry`
- `UpdateNodeWithEdgesRetry`
- `SafeUpdateNode`
- `SafeUpdateNodeWithConnections`
- `BulkDeleteNodesIdempotent`
- `GetNodesPage`
- `GetNodesPageOptimized`
- `GetNodeNeighborhood`
- `GetGraphDataPaginated`

### Update imports:
Remove any unused imports after deletion.

---

## Implementation Order

### Day 1: Foundation (2-3 hours)
1. **Phase 0**: Fix initialization issues (30 min)
2. **Phase 1.1**: Create internal helper methods (30 min)
3. **Phase 1.2**: Consolidate public methods (1 hour)
4. **Phase 1.3**: Add context helpers (15 min)
5. **Phase 1.4**: Update interface (15 min)
6. **Test**: Run basic creation test

### Day 2: Integration (2-3 hours)
1. **Phase 2**: Update handlers (1 hour)
2. **Phase 3**: Repository cleanup (30 min)
3. **Phase 4**: Create and run tests (1 hour)
4. **Test**: Full integration test

### Day 3: Cleanup (1 hour)
1. **Phase 5**: Delete redundant code
2. **Final test**: Complete application test
3. **Deploy**: Test in staging environment

---

## Verification Steps After Each Phase

### After Phase 0:
```go
// Verify in main.go or handler init:
if container.IdempotencyStore == nil {
    log.Fatal("IdempotencyStore not initialized!")
}
```

### After Phase 1:
```bash
# Build should succeed
go build ./...

# Tests should pass
go test ./internal/service/memory/...
```

### After Phase 2:
```bash
# API should respond
curl http://localhost:8080/health

# Node creation should work
curl -X POST http://localhost:8080/api/nodes ...
```

### After Phase 5:
```bash
# No compilation errors
go build ./...

# All tests pass
go test ./...

# Coverage maintained
go test -cover ./...
```

---

## Common Pitfalls to Avoid

1. **DON'T delete methods that are still referenced**
   - Search for all usages before deleting
   - Use IDE's "Find Usages" feature

2. **DON'T forget to initialize IdempotencyStore**
   - This causes nil pointer panics

3. **DON'T mix version numbers**
   - New nodes: Version = 0
   - First update: Version = 1

4. **DON'T remove error handling**
   - Keep all error wrapping and logging

5. **DON'T change API contracts**
   - Keep same request/response formats

---

## Success Metrics

After implementation, you should see:
- ✅ 50% reduction in service layer code (~500 lines removed)
- ✅ Single path for each operation
- ✅ Consistent error handling
- ✅ No 500 errors
- ✅ Sub-200ms response times
- ✅ Idempotency working correctly
- ✅ Version conflicts handled gracefully

---

## Rollback Plan

If issues occur:
1. **Git stash changes**: `git stash`
2. **Revert to last working commit**: `git reset --hard HEAD`
3. **Implement one phase at a time**
4. **Test after each phase**

---

## Questions to Answer Before Starting

1. **Is the IdempotencyStore initialized?** Check container.go
2. **Are all tests passing currently?** Run `go test ./...`
3. **Is staging environment available for testing?** 
4. **Do you have database backups?**
5. **Is monitoring in place to detect issues?**

Start with Phase 0 fixes FIRST - they are critical for preventing the 500 errors you experienced.