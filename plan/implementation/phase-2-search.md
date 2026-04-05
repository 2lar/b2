# Phase 2: Hybrid Search (BM25 + Semantic + RRF)

## Overview

Implement real search for b2 — BM25 full-text scoring + semantic vector search, fused with Reciprocal Rank Fusion. At b2's scale (<10K nodes per user), everything runs in-app against DynamoDB without needing ElasticSearch.

## Key Decisions

### BM25 Implementation
**Choice: In-app BM25 scoring over DynamoDB data**
- Load user's nodes, compute TF-IDF/BM25 scores in Go
- No external search service needed at personal scale
- Standard BM25 parameters: k1=1.2, b=0.75
- Pre-filter candidates via keyword extraction to avoid scoring all nodes

### Semantic Search
**Choice: Query-time embedding + cosine similarity**
- Embed the search query using the same embedding service from Phase 1
- Compute cosine similarity against all nodes with embeddings
- Already have the foundation: `EmbeddingService`, `Embedding.CosineSimilarity()`

### Result Fusion
**Choice: Reciprocal Rank Fusion (K=60)**
- `score = 1/(K + rank)` for each method, summed per node
- Nodes found by both methods get boosted
- Source attribution: "bm25", "semantic", or "bm25 + semantic"
- No score normalization needed (RRF is rank-based)

## Implementation Steps

### Step 1: BM25 Scoring Service
- `domain/services/bm25.go` — BM25 scorer
- Operates on `[]ScoredDocument` (node ID + text + score)
- Computes IDF, TF, length normalization per standard BM25 formula
- Uses TextAnalyzer for tokenization (reuse existing stop words)

### Step 2: Search Service (Hybrid Orchestrator)
- `domain/services/search_service.go` — combines BM25 + semantic
- Loads candidate nodes from repository
- Runs BM25 scoring on title + body text
- Runs semantic cosine on embeddings (skips nodes without embeddings)
- Applies RRF fusion, returns ranked results with source attribution

### Step 3: Search Query + Handler
- Update `application/queries/search_nodes.go` — proper CQRS query
- Handler uses SearchService, returns structured results
- Register handler in DI

### Step 4: Search API Endpoint
- Fix `search_handler.go` to use the query through mediator
- Return results with scores, sources, and pagination

### Step 5: Frontend Search UI
- Search input component
- Results list with relevance indicators
- Source attribution badges (keyword, semantic, both)

## Files Created
| File | Purpose |
|------|---------|
| `domain/services/bm25.go` | BM25 scoring implementation |
| `domain/services/search_service.go` | Hybrid search orchestrator with RRF |

## Files Modified
| File | Change |
|------|--------|
| `application/queries/find_similar_nodes.go` | Fix SearchNodesQuery handler |
| `interfaces/http/rest/handlers/search_handler.go` | Wire to mediator query |
| `infrastructure/di/providers.go` | Register search handler |
| Frontend search components | Search UI |
