# Phase 1: Embeddings + Semantic Similarity

## Overview

Replace b2's keyword-only similarity with hybrid keyword + semantic similarity using vector embeddings. This is the foundation for all subsequent phases.

## Key Decisions

### Embedding Provider
**Choice: AWS Bedrock Titan Embeddings v2**
- Already on AWS — no new vendor, IAM-based auth
- Works from Lambda without cold-start penalty of local models
- 1024-dim vectors (configurable down to 256/512 for cost)
- Supports batching (up to 2048 tokens per text)
- Alternative: OpenAI `text-embedding-3-small` (1536-dim, better quality, external dependency)

### Vector Storage
**Choice: DynamoDB with in-memory cosine computation**
- b2 is a personal tool — realistically <10K nodes per user
- Store embedding as binary blob on node item (base64-encoded float64 array)
- During edge discovery, load candidate embeddings and compute cosine in Go
- No need for OpenSearch/Pinecone at this scale
- If scale demands it later, swap to OpenSearch Serverless with k-NN

### Similarity Blending
**Choice: Weighted fusion with configurable ratio**
- Default: 0.6 semantic + 0.4 keyword (semantic is primary signal)
- Fall back to keyword-only when embedding is missing (backward compat during backfill)
- Configurable per environment via DomainConfig

## Implementation Steps

### Step 1: Embedding Value Object + Node Entity Update
- New value object: `Embedding` in `domain/core/valueobjects/embedding.go`
  - Wraps `[]float64`, dimension validation, cosine similarity method
- Add `embedding` field to Node entity
- Add `SetEmbedding()` / `GetEmbedding()` methods
- Add `HasEmbedding()` check
- Node stays backward-compatible — embedding is nullable

### Step 2: Embedding Service
- New interface: `EmbeddingService` in `domain/services/embedding_service.go`
  - `GenerateEmbedding(ctx, text string) (valueobjects.Embedding, error)`
  - `GenerateEmbeddings(ctx, texts []string) ([]valueobjects.Embedding, error)`
- New implementation: `BedrockEmbeddingService` in `infrastructure/embeddings/bedrock.go`
  - Calls AWS Bedrock Titan Embeddings v2
  - Batched requests, retry with backoff
  - Configurable model ID and dimensions

### Step 3: Rewrite SimilarityCalculator
- Complete rewrite — current implementation is the bottleneck
- New `HybridSimilarityCalculator` replaces `DefaultSimilarityCalculator`
  - Computes semantic similarity (cosine on embeddings) when both nodes have vectors
  - Computes keyword similarity (existing Jaccard/cosine on word bags)
  - Blends: `score = (semanticWeight * semanticSim) + (keywordWeight * keywordSim)`
  - Falls back to keyword-only when either node lacks embedding
  - Returns confidence alongside score (higher when both signals available)

### Step 4: Update EdgeDiscoveryService
- Use new similarity scores with confidence
- Edge type classification uses finer granularity:
  - 0.85+ strong (was 0.7)
  - 0.6-0.85 normal (new tier)
  - 0.3-0.6 weak
  - Below 0.3 no edge
- Store `discovery_method` on edge (keyword, semantic, hybrid)

### Step 5: DynamoDB Persistence Update
- Update `ToItem` / `ParseItem` in node_repository.go to serialize/deserialize embedding
- Embedding stored as DynamoDB Binary attribute (efficient, no JSON overhead)
- Null-safe — existing nodes without embeddings load fine

### Step 6: Embedding Computation Lambda
- New Lambda: `cmd/embed-node/main.go`
- Triggered by EventBridge on `node.created` and `node.content.updated`
- Flow: receive event → load node → generate embedding → save node
- Idempotent — safe to retry or re-trigger
- Add to CDK compute stack

### Step 7: Backfill Command
- New CLI command or one-shot Lambda to embed all existing nodes
- Processes in batches (50 nodes per batch)
- Skips nodes that already have embeddings
- Progress logging

### Step 8: Tests
- Unit tests for Embedding value object (cosine, validation)
- Unit tests for HybridSimilarityCalculator (semantic-only, keyword-only, hybrid, fallback)
- Integration test for BedrockEmbeddingService (mocked AWS client)
- Integration test for updated EdgeDiscoveryService
- E2E test: create node → embedding generated → edges discovered

## Files Created
| File | Purpose |
|------|---------|
| `domain/core/valueobjects/embedding.go` | Embedding value object |
| `domain/services/embedding_service.go` | EmbeddingService interface |
| `infrastructure/embeddings/bedrock.go` | Bedrock implementation |
| `cmd/embed-node/main.go` | Embedding Lambda handler |

## Files Modified
| File | Change |
|------|--------|
| `domain/core/entities/node.go` | Add embedding field + methods |
| `domain/services/similarity_calculator.go` | Rewrite as HybridSimilarityCalculator |
| `domain/services/edge_discovery.go` | Updated thresholds + discovery_method |
| `domain/config/domain_config.go` | Add embedding config fields |
| `infrastructure/persistence/dynamodb/node_repository.go` | Serialize/deserialize embedding |
| `infrastructure/di/wire.go` | Wire new services |
| `infrastructure/di/providers.go` | New providers |
| `infra/lib/stacks/compute-stack.ts` | Add embed-node Lambda |

## Commit Plan
```
feat(embedding): add Embedding value object with cosine similarity
feat(node): add embedding field to Node entity
feat(embedding): add EmbeddingService interface and Bedrock implementation
refactor(similarity): rewrite SimilarityCalculator as hybrid keyword + semantic
feat(discovery): update EdgeDiscoveryService with finer edge classification
feat(persistence): add embedding serialization to DynamoDB node repository
feat(lambda): add embed-node Lambda for async embedding generation
infra(compute): add embed-node Lambda to CDK stack
feat(backfill): add existing node embedding backfill command
test(embedding): add unit and integration tests for embedding pipeline
```
