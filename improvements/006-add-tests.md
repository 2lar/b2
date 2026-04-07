# 006 — Add Missing Tests

## Priority: MEDIUM
## Effort: Large
## Files: 10-15 new test files

## Current Test State

The test suite has:
- `tests/unit/domain/node_test.go` (212 lines) — entity lifecycle
- `tests/unit/sagas/create_node_saga_test.go` (313 lines) — saga framework
- `tests/fixtures/builders.go` (474 lines) — test data builders

That's it. No repository tests, no service tests, no integration tests,
no concurrent tests.

## What's Missing (by priority)

### Tier 1 — Validates core correctness

#### 1.1 Edge Discovery Integration Test
**New file:** `tests/integration/edge_discovery_test.go`

Tests the full flow: create node → discover edges → verify connections.

```
Test cases:
- Two similar nodes → edge discovered with score > threshold
- Two dissimilar nodes → no edge discovered
- Node with embedding + node with embedding → hybrid similarity used
- Node without embedding → keyword-only fallback
- MaxEdgesPerNode respected
- MinSimilarity threshold respected
- Sync vs async edge split works correctly
```

#### 1.2 Similarity Calculator Unit Tests
**New file:** `tests/unit/domain/similarity_calculator_test.go`

```
Test cases:
- Keyword similarity: identical content → 1.0
- Keyword similarity: no overlap → 0.0
- Keyword similarity: partial overlap → correct Jaccard score
- Hybrid similarity: both embeddings → 60/40 blend
- Hybrid similarity: one missing embedding → keyword-only fallback
- Hybrid similarity: tag overlap bonus
- Edge case: empty content
- Edge case: stop words only
```

#### 1.3 DynamoDB Repository Tests (with DynamoDB Local)
**New file:** `tests/integration/dynamodb/node_repo_test.go`
**New file:** `tests/integration/dynamodb/edge_repo_test.go`
**New file:** `tests/integration/dynamodb/graph_repo_test.go`

```
Test cases per repo:
- Save and retrieve
- Update existing record
- Delete and verify gone
- GetByGraphID returns all nodes in graph
- GetByNodeID returns edges in both directions (source + target)
- GetOrCreateDefaultGraph — single call succeeds
- GetOrCreateDefaultGraph — concurrent calls return same graph
- SaveWithUoW + Commit → items visible
- SaveWithUoW + Rollback → items not visible
- Batch operations handle partial failures
```

**Setup:** Docker container running DynamoDB Local. Test creates table
with all GSIs before each test suite.

### Tier 2 — Validates domain services

#### 2.1 BM25 Edge Case Tests
**New file:** `tests/unit/domain/bm25_extended_test.go`

```
Test cases:
- Query "C++" → matches documents about C++
- Query "node-id" → matches "node-id" not "nodeid"
- Query "Go" → not filtered out
- Query with special chars: @, #, $
- Empty query → empty results (not error)
- Single-word query
- Very long query (100+ words)
- Documents with identical content
```

#### 2.2 Leiden Community Detection Tests
**New file:** `tests/unit/domain/leiden_extended_test.go`

```
Test cases:
- Merge loop terminates (graph with many small communities)
- Refinement doesn't over-fragment
- Deterministic output (same input → same communities)
- Large graph performance (1000 nodes, < 5 seconds)
- Resolution parameter affects granularity
```

#### 2.3 Thought Chain Tests
**New file:** `tests/unit/domain/thought_chain_extended_test.go`

```
Test cases:
- Graph with cycles → no infinite loop
- Dense graph → chains bounded by MaxChains
- Hub node detection accurate
- Cross-community chains identified correctly
- Performance: 500-node graph completes in < 1 second
```

#### 2.4 Hybrid Search Integration Test
**New file:** `tests/integration/search_test.go`

```
Test cases:
- BM25-only search (no embeddings) → returns keyword matches
- Hybrid search (with embeddings) → semantic results ranked higher
- RRF fusion → results from both sources merged correctly
- Empty query → empty results
- Query matching no documents → empty results
- Pagination (offset + limit)
```

### Tier 3 — Validates robustness

#### 3.1 Concurrent Operation Tests
**New file:** `tests/integration/concurrent_test.go`

```
Test cases:
- 10 concurrent node creations → all succeed, no duplicate edges
- Concurrent graph metadata updates → no lost updates
- Concurrent edge discovery on same graph → no duplicate edges
- Concurrent GetOrCreateDefaultGraph → single graph returned
```

#### 3.2 Event System Tests
**New file:** `tests/integration/event_system_test.go`

```
Test cases:
- Node creation → NodeCreated event published
- Event saved to store with pending status
- Outbox processor publishes pending events
- Failed events retry up to max attempts
- Duplicate events are idempotent
```

#### 3.3 Saga Failure & Compensation Tests
**New file:** `tests/unit/sagas/create_node_saga_failures_test.go`

```
Test cases:
- Edge discovery fails → node still created (partial success)
- Event publishing fails → node + edges still committed
- Graph metadata update fails → node + edges still committed
- Transaction commit fails → full rollback
```

## Test Infrastructure Needed

### DynamoDB Local Docker Setup
**New file:** `tests/docker-compose.yml`
```yaml
services:
  dynamodb-local:
    image: amazon/dynamodb-local
    ports:
      - "8000:8000"
```

### Test Helper: Table Creation
**New file:** `tests/integration/helpers.go`

Creates the DynamoDB table with all GSIs matching the CDK stack definition.
Tears down between test suites.

### Test Helper: Embedding Mock
**New file:** `tests/mocks/embedding_service.go`

Returns deterministic embeddings based on content hash.
Allows testing hybrid similarity without an API key.

## Running Tests

```bash
# Unit tests (no external deps)
go test ./tests/unit/...

# Integration tests (requires DynamoDB Local)
docker-compose -f tests/docker-compose.yml up -d
go test ./tests/integration/... -tags=integration
docker-compose -f tests/docker-compose.yml down
```
