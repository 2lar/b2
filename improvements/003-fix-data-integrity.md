# 003 — Fix Data Integrity & Race Conditions

## Priority: CRITICAL
## Effort: Medium
## Files: 5-6 files modified

## Problem

The persistence layer has several data integrity issues that can cause
graph corruption, duplicate records, and silent data loss under
concurrent usage.

---

## Issue 1: GetOrCreateDefaultGraph Race Condition

**File:** `backend/infrastructure/persistence/dynamodb/graph_repository.go`
**Lines:** 494-555

**Bug:** Two concurrent calls for the same user both succeed, creating
two default graphs. The PutItem has no ConditionExpression.

**Fix:** Add conditional write:
```go
input := &dynamodb.PutItemInput{
    TableName:           aws.String(r.tableName),
    Item:                av,
    ConditionExpression: aws.String("attribute_not_exists(PK) AND attribute_not_exists(SK)"),
}

_, err = r.client.PutItem(ctx, input)
if err != nil {
    // ConditionalCheckFailed means another request created it first — just load it
    var condErr *types.ConditionalCheckFailedException
    if errors.As(err, &condErr) {
        return r.GetUserDefaultGraph(ctx, userID)
    }
    return nil, fmt.Errorf("failed to save default graph: %w", err)
}
```

**Effort:** Small — 5 lines changed

---

## Issue 2: Bidirectional Edge Queries Silently Fail

**File:** `backend/infrastructure/persistence/dynamodb/edge_repository.go`
**Lines:** 276-312

**Bug:** When GSI3 (TargetNodeIndex) is not configured or query fails,
`GetByNodeID()` returns only edges where the node is SOURCE. Target edges
are silently dropped. A `Warn` log is emitted but the caller gets incomplete data.

**Fix (two parts):**

### Part A: Make GSI3 required, not optional
```go
// Constructor should validate:
func NewEdgeRepository(client *dynamodb.Client, tableName string, cfg *config.Config, logger *zap.Logger) *EdgeRepository {
    if cfg.GSI3IndexName == "" {
        logger.Fatal("GSI3IndexName (TargetNodeIndex) is required for bidirectional edge queries")
    }
    // ...
}
```

### Part B: Return error instead of partial results
```go
// Lines 308-312: Change from warn+continue to error
if err != nil {
    return nil, fmt.Errorf("failed to query target edges via GSI3: %w", err)
}
```

### Part C: Fix hardcoded index name
**Line 245:** Uses hardcoded `"EdgeIndex"` instead of `r.gsi2IndexName`
```go
// Current:
IndexName: aws.String("EdgeIndex"),
// Fix:
IndexName: aws.String(r.gsi2IndexName),
```

**Effort:** Small — ~15 lines changed

---

## Issue 3: Concurrent Node Creation Edge Asymmetry

**File:** `backend/application/sagas/create_node_saga.go`
**Lines:** 390-462

**Bug:** When nodes A and B are created simultaneously:
- A's edge discovery loads existing nodes (doesn't include B)
- B's edge discovery loads existing nodes (may include A)
- Result: B→A edge exists, A→B does not

**Fix options:**

### Option A: Distributed lock per graph (recommended)
Acquire a per-graph distributed lock before edge discovery. The lock
already exists in the codebase (`infrastructure/persistence/dynamodb/distributed_lock.go`).

```go
// In discoverEdges step:
lock, err := cns.distributedLock.Acquire(ctx, fmt.Sprintf("graph-edges:%s", d.GraphID), 30*time.Second)
if err != nil {
    // Proceed without lock — edges may be suboptimal but not lost
    logger.Warn("Could not acquire edge discovery lock", zap.Error(err))
}
defer lock.Release(ctx)
```

### Option B: Post-creation edge reconciliation
After embedding is generated (async), re-run edge discovery for the node.
This catches any nodes that were created concurrently.

### Option C: Accept eventual consistency
Document that edges are eventually consistent. The async connect-node
Lambda already re-runs edge discovery, which will catch missed connections.
This is the simplest option if we also implement Option B from Plan 001
(re-run edges after embedding).

**Recommended:** Option C (accept eventual consistency) + ensure the async
reconnection path works reliably. This is a knowledge graph, not a financial
ledger — eventual consistency is acceptable.

**Effort:** Small (Option C) to Medium (Option A)

---

## Issue 4: Batch Delete Ignores UnprocessedItems

**File:** `backend/infrastructure/persistence/dynamodb/edge_repository.go`
**Lines:** 482-519

**Bug:** DynamoDB BatchWriteItem may not process all items (throttling).
Unprocessed items are silently dropped.

**Fix:**
```go
result, err := r.client.BatchWriteItem(ctx, input)
if err != nil {
    return fmt.Errorf("batch delete failed: %w", err)
}

// Retry unprocessed items with exponential backoff
unprocessed := result.UnprocessedItems
retries := 0
for len(unprocessed) > 0 && retries < 3 {
    time.Sleep(time.Duration(math.Pow(2, float64(retries))) * 100 * time.Millisecond)
    retryInput := &dynamodb.BatchWriteItemInput{RequestItems: unprocessed}
    retryResult, err := r.client.BatchWriteItem(ctx, retryInput)
    if err != nil {
        return fmt.Errorf("batch delete retry failed: %w", err)
    }
    unprocessed = retryResult.UnprocessedItems
    retries++
}
if len(unprocessed) > 0 {
    return fmt.Errorf("batch delete: %d items still unprocessed after retries", len(unprocessed))
}
```

**Effort:** Small — ~15 lines added

---

## Issue 5: Graph SaveWithUoW Missing Condition Expression

**File:** `backend/infrastructure/persistence/dynamodb/graph_repository.go`
**Lines:** 194-200

**Bug:** Graph SaveWithUoW has no ConditionExpression, allowing concurrent
graph updates to silently overwrite each other.

**Fix:**
```go
transactItem := types.TransactWriteItem{
    Put: &types.Put{
        TableName:           aws.String(r.tableName),
        Item:                av,
        ConditionExpression: aws.String("attribute_not_exists(PK) OR Version = :v"),
        ExpressionAttributeValues: map[string]types.AttributeValue{
            ":v": &types.AttributeValueMemberN{Value: fmt.Sprintf("%d", graph.Version()-1)},
        },
    },
}
```

**Effort:** Small — 5 lines changed

---

## Issue 6: Community Service Persistence Not Transactional

**File:** `backend/application/services/community_service.go`
**Lines:** 134-147

**Bug:** Community IDs are saved node-by-node in a loop. If node 10 fails,
nodes 1-9 have community IDs but 10-50 don't.

**Fix:** Use UnitOfWork for batch save:
```go
uow := s.uowFactory.Create()
if err := uow.Begin(ctx); err != nil {
    return nil, err
}

for _, node := range nodes {
    node.SetCommunityID(communityID)
    if err := uow.NodeRepository().Save(ctx, node); err != nil {
        uow.Rollback()
        return nil, fmt.Errorf("failed to save community assignment: %w", err)
    }
}

if err := uow.Commit(ctx); err != nil {
    return nil, fmt.Errorf("failed to commit community assignments: %w", err)
}
```

**Note:** DynamoDB TransactWriteItems has a 100-item limit. For graphs
with >100 nodes, batch into groups of 100.

**Effort:** Medium — requires UoW injection into community service

---

## Verification Checklist

- [ ] Create two nodes simultaneously — both should have edges to each other
- [ ] Call GetOrCreateDefaultGraph twice rapidly — only one graph created
- [ ] GetByNodeID returns edges where node is both source AND target
- [ ] Bulk delete 100 edges — verify all deleted, none left behind
- [ ] Recompute communities on 150-node graph — all nodes assigned
