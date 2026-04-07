# 001 — Enable Embeddings & Semantic Similarity

## Priority: CRITICAL
## Effort: Medium
## Files: 6-8 files modified

## Problem

B2's auto-connection pipeline is the heart of the second brain. It has a
fully implemented hybrid similarity system (60% semantic + 40% keyword),
but **embeddings are disabled by default**. This means:

- All edge discovery uses keyword-only Jaccard similarity
- "Machine learning" and "neural networks" won't auto-connect (no shared keywords)
- Hybrid search falls back to BM25-only (no semantic ranking)
- The `embed-node` Lambda silently does nothing

The architecture is there. The code is there. It just needs to be turned on
and made reliable.

## Root Cause

`backend/infrastructure/config/config.go:137`:
```go
Enabled: getEnvBool("EMBEDDING_ENABLED", false),
```

And the edge service creates its own similarity calculator without embeddings:
`backend/application/services/edge_service.go:54`:
```go
similarityCalc := services.NewHybridSimilarityCalculator(nil, textAnalyzer)
//                                                       ^^^ nil embedding service
```

## Changes Required

### 1. Default embeddings to enabled
**File:** `backend/infrastructure/config/config.go`
**Line:** 137

Change default to `true`. Require `EMBEDDING_API_KEY` to be set (fail loudly
if enabled but no key provided).

```go
Enabled: getEnvBool("EMBEDDING_ENABLED", true),
```

Add validation:
```go
if cfg.Embedding.Enabled && cfg.Embedding.APIKey == "" {
    return nil, fmt.Errorf("EMBEDDING_API_KEY required when embeddings are enabled")
}
```

### 2. Wire embedding service into edge discovery
**File:** `backend/application/services/edge_service.go`
**Lines:** 32-61

The EdgeService constructor creates its own `HybridSimilarityCalculator`
with `nil` embedding service. It should receive the embedding service via DI.

```go
// Current (broken):
similarityCalc := services.NewHybridSimilarityCalculator(nil, textAnalyzer)

// Fixed:
similarityCalc := services.NewHybridSimilarityCalculator(embeddingService, textAnalyzer)
```

**File:** `backend/infrastructure/di/providers.go`

Update `ProvideEdgeService` to accept and pass through the embedding service,
similar to how `ProvideHybridSearchService` does it.

### 3. Make embed-node Lambda log warnings when disabled
**File:** `backend/cmd/embed-node/main.go`
**Line:** 71

Change from silent skip to loud warning:
```go
if !cfg.Embedding.Enabled {
    logger.Warn("EMBEDDING DISABLED — edge discovery will use keyword-only similarity")
    return nil
}
```

### 4. Add embedding backfill on startup
**File:** `backend/cmd/embed-node/main.go`
**Lines:** 158-205

The backfill mode exists but is never triggered automatically. Add a
mechanism to detect nodes without embeddings and queue them for processing.
Could be:
- A CLI flag: `embed-node --backfill`
- A scheduled Lambda (CloudWatch Events rule, daily)
- A one-time migration script

### 5. Handle embedding timing in edge discovery
**File:** `backend/application/sagas/create_node_saga.go`

Current flow: Node created → edges discovered (keyword-only) → embedding generated async.
This means the FIRST node's embedding isn't available when its edges are discovered.

Options:
- **Option A (recommended):** After embedding is generated, re-run edge discovery
  for that node. The embed-node Lambda should publish a `node.embedding.generated`
  event, and a new handler re-evaluates edges.
- **Option B:** Generate embedding synchronously before edge discovery (adds latency
  but guarantees quality).
- **Option C:** Accept that first-pass edges are keyword-only, and improve them
  on a background sweep.

### 6. Ensure CDK stack has EMBEDDING_ENABLED=true
**File:** `infra/lib/stacks/compute-stack.ts`

Verify that the embed-node Lambda and the API Lambda both have:
```typescript
environment: {
  EMBEDDING_ENABLED: 'true',
  EMBEDDING_API_KEY: process.env.EMBEDDING_API_KEY || '',
  EMBEDDING_BASE_URL: 'https://api.openai.com/v1',
  EMBEDDING_MODEL: 'text-embedding-3-small',
}
```

## Verification

After these changes:
1. Create a node with title "Machine learning fundamentals"
2. Create a node with title "Neural network architectures"
3. Verify: embedding generated for node 1 (check DynamoDB `Embedding` attribute)
4. Verify: edge discovered between nodes 1 and 2 with `method: "hybrid"`
5. Search for "deep learning" — verify semantic results appear

## Dependencies

- Valid OpenAI API key (or compatible endpoint)
- CDK deployment to update Lambda environment variables
