# B2 Improvement Plan — GitNexus-Informed Upgrades

> **Goal:** Elevate B2 from a basic note-graph into a production-grade second brain with intelligent connections, meaningful clustering, powerful search, and performant visualization — drawing on proven patterns from the GitNexus codebase.

---

## Current State Assessment

### What B2 Does Well
- Clean DDD + CQRS architecture (Go backend)
- Event-driven async edge discovery via EventBridge
- Real-time WebSocket updates
- Solid infrastructure (AWS CDK, Lambda, DynamoDB)
- Interactive Cytoscape.js graph with drag physics

### Where B2 Falls Short
- **Similarity is keyword-only** — Jaccard/cosine on word bags misses semantic relationships ("machine learning" won't connect to "neural networks")
- **No real search** — `FindSimilarNodes` reuses the same shallow similarity; no full-text index, no semantic search
- **Clustering is naive** — DFS connected-components, not meaningful community detection within a connected graph
- **Visualization hits a ceiling** — Cytoscape.js Canvas rendering struggles past ~500 nodes; node coloring uses 3 arbitrary connectivity buckets instead of real clusters
- **No higher-order intelligence** — no thought chains, no impact analysis, no auto-categorization
- **Edge types are underutilized** — strong/weak classification based on a single 0.7 threshold with no nuance

---

## Improvement Phases

### Phase 1: Embeddings + Semantic Similarity
**Priority:** Critical — everything else builds on better connections

| Aspect | Detail |
|--------|--------|
| **Problem** | Two notes about the same concept using different words get 0.0 similarity |
| **Solution** | Generate vector embeddings for each node's content; use cosine similarity on vectors alongside keyword matching |
| **Approach** | Add an embedding service (local model via Go bindings or API call to OpenAI/Voyage); store vectors as a new field on nodes in DynamoDB; upgrade `SimilarityCalculator` to blend keyword + semantic scores |
| **GitNexus Pattern** | Uses Snowflake Arctic Embed XS (384-dim) via transformers.js with batched inference, cached embeddings in metadata, incremental re-embedding on content change |
| **Key Decisions** | Model choice (local vs API), vector storage strategy (DynamoDB attribute vs dedicated vector store), embedding dimensions, re-embedding triggers |
| **Success Metric** | Notes with related concepts but different vocabulary now connect automatically |
| **Implementation Plan** | [Phase 1: Embeddings + Semantic Similarity](./implementation/phase-1-embeddings.md) |

---

### Phase 2: Hybrid Search (BM25 + Semantic + RRF)
**Priority:** High — immediate UX win, users can actually find their notes

| Aspect | Detail |
|--------|--------|
| **Problem** | No real search capability; finding a note requires scrolling or exact keyword recall |
| **Solution** | Implement BM25 full-text search + semantic vector search, fused with Reciprocal Rank Fusion |
| **Approach** | Add BM25 index (DynamoDB-backed or OpenSearch); semantic search via embedding cosine distance; RRF merger with `score = 1/(K + rank)`, K=60 |
| **GitNexus Pattern** | `hybrid-search.ts` — dual-index query, per-result source attribution, process-grouped output |
| **Key Decisions** | BM25 implementation (in-app vs managed service), search index update strategy (sync vs async), result grouping (flat vs clustered) |
| **Success Metric** | Sub-200ms search returning relevant results for both exact terms and conceptual queries |
| **Implementation Plan** | [Phase 2: Hybrid Search](./implementation/phase-2-search.md) |

---

### Phase 3: Leiden Community Detection
**Priority:** High — enables meaningful auto-clustering and graph intelligence

| Aspect | Detail |
|--------|--------|
| **Problem** | DFS grouping only finds disconnected components; within a connected graph, all nodes are one "cluster" |
| **Solution** | Implement Leiden algorithm for community detection; assign nodes to functional clusters with cohesion scores |
| **Approach** | Run Leiden on the edge graph (weighted by similarity); store community membership on nodes; extract cluster keywords for auto-naming; recompute on significant graph changes |
| **GitNexus Pattern** | `community-processor.ts` — Leiden over CALLS graph, 8-15 communities per 1000 symbols, cohesion scoring 0-1, keyword extraction per cluster |
| **Key Decisions** | Run frequency (on every edge change vs periodic batch), resolution parameter tuning, cluster naming strategy (keyword extraction vs LLM summarization) |
| **Success Metric** | Graph automatically groups notes into meaningful topics ("Health", "Project Ideas", "Book Notes") without user tagging |
| **Implementation Plan** | [Phase 3: Community Detection](./implementation/phase-3-communities.md) |

---

### Phase 4: Sigma.js + WebGL Visualization
**Priority:** Medium-High — performance unlock + enables community-colored rendering

| Aspect | Detail |
|--------|--------|
| **Problem** | Cytoscape.js Canvas rendering degrades past ~500 nodes; layout runs on main thread; coloring doesn't reflect real clusters |
| **Solution** | Replace Cytoscape.js with Sigma.js (WebGL) + Graphology + ForceAtlas2 worker-thread layout |
| **Approach** | Migrate graph data model to Graphology; render with Sigma.js; run ForceAtlas2 in web worker; color nodes by Leiden community membership; add depth-based filtering (1/2/3 hops from selection) |
| **GitNexus Pattern** | `GraphCanvas.tsx` + `graph-adapter.ts` — Sigma.js 3.0, ForceAtlas2 worker, mass-based layout (hub nodes heavier), 15+ community colors, depth filtering, search highlighting |
| **Key Decisions** | Migration strategy (incremental vs full rewrite of GraphVisualization.tsx), preserving existing interactions (drag, select, document mode), mobile WebGL support |
| **What Changes** | `GraphVisualization.tsx` (964 lines) gets replaced; `GraphControls`, `NodeDetailsPanel`, `DocumentModeView` stay with adapter changes |
| **Success Metric** | Smooth 60fps rendering at 5,000+ nodes; community clusters visually obvious; layout doesn't block UI |
| **Implementation Plan** | [Phase 4: WebGL Visualization](./implementation/phase-4-visualization.md) |

---

### Phase 5: Thought Chains + Impact Analysis
**Priority:** Medium — higher-order intelligence features that differentiate B2

| Aspect | Detail |
|--------|--------|
| **Problem** | No way to trace how ideas flow across the graph or understand the "weight" of a note |
| **Solution** | Two features: (A) Thought Chains — trace paths through the graph from hub notes outward, especially cross-cluster paths; (B) Impact Analysis — show blast radius when editing/deleting a note |
| **Approach** | **(A)** Identify hub nodes (high connectivity + high centrality); BFS trace outward up to N hops with branch limiting; classify as intra-cluster vs cross-cluster; present as navigable paths. **(B)** Compute upstream/downstream dependencies grouped by depth; assign risk levels based on connection count and cluster bridging |
| **GitNexus Pattern** | `process-processor.ts` — entry point scoring, BFS trace (10 hops, 4 branches/node), cross-community classification. `impact` MCP tool — depth-grouped dependents, WILL_BREAK / LIKELY_AFFECTED / MAY_AFFECT tiers |
| **Key Decisions** | Hub detection algorithm (degree centrality vs betweenness centrality), max trace depth, how to present chains in UI (timeline? flow diagram? animated path?), impact computation timing (on-demand vs precomputed) |
| **Success Metric** | Users can see "how did I get from thinking about X to concluding Y" and understand which notes are load-bearing vs peripheral |
| **Implementation Plan** | [Phase 5: Thought Chains + Impact](./implementation/phase-5-chains-impact.md) |

---

## Cross-Cutting Concerns

### Performance & Scalability
| Concern | Current | Target |
|---------|---------|--------|
| Max nodes before UI degrades | ~500 | 5,000+ |
| Similarity computation | O(n) keyword comparison per node | O(1) vector lookup + O(n) cosine, batched |
| Search latency | No search | <200ms hybrid search |
| Layout computation | Main thread (blocks UI) | Web Worker (non-blocking) |
| Edge discovery | Sync in Lambda, all-pairs | Async, ANN index for candidate retrieval |

### Data Model Changes
- **Node:** Add `embedding` field (float64 array or binary blob), `community_id`, `centrality_score`
- **Edge:** Add `confidence` field (0.0-1.0, finer than current strong/weak binary), `discovery_method` (keyword, semantic, manual)
- **New Entity — Community:** `id`, `name`, `keywords[]`, `cohesion_score`, `member_count`, `created_at`
- **New Entity — ThoughtChain:** `id`, `entry_node_id`, `steps[]` (ordered node IDs), `crosses_communities[]`, `created_at`

### Infrastructure Changes
- **Embedding computation Lambda** — triggered on NodeCreated/NodeContentUpdated events
- **Community recomputation Lambda** — triggered periodically or on significant graph changes (>N new edges)
- **Vector index** — either DynamoDB with scan-and-filter (small scale) or OpenSearch Serverless with k-NN (large scale)
- **Search index** — DynamoDB Streams → OpenSearch for BM25, or in-app BM25 with inverted index in DynamoDB

### Migration Strategy
Each phase is independently deployable and backward-compatible:
1. **Phase 1** adds embedding field (nullable) — existing nodes get embeddings via backfill Lambda
2. **Phase 2** adds search endpoint — new API route, no breaking changes
3. **Phase 3** adds community_id to nodes — nullable, computed async
4. **Phase 4** swaps frontend rendering — backend API unchanged
5. **Phase 5** adds new query endpoints — additive only

---

## Dependencies Between Phases

```
Phase 1 (Embeddings)
  |
  +---> Phase 2 (Search) -- needs embeddings for semantic search
  |
  +---> Phase 3 (Communities) -- benefits from semantic edge weights
           |
           +---> Phase 4 (Visualization) -- needs communities for coloring
           |
           +---> Phase 5 (Chains + Impact) -- needs communities for cross-cluster detection
```

Phase 1 is the foundation. Phases 2 and 3 can run in parallel after Phase 1. Phase 4 depends on Phase 3 for full value but can start with connectivity-based coloring. Phase 5 depends on Phase 3.

---

## Estimated Scope

| Phase | Backend | Frontend | Infra | New Files | Modified Files |
|-------|---------|----------|-------|-----------|----------------|
| 1 | Embedding service, updated similarity calc | — | Embedding Lambda, vector storage | ~8 | ~5 |
| 2 | Search service, BM25 index, search API | Search UI component | Search index infra | ~10 | ~4 |
| 3 | Leiden service, community entity, recompute job | Community labels in graph | Community Lambda | ~8 | ~6 |
| 4 | — | Sigma.js migration, ForceAtlas2 worker | — | ~6 | ~4 |
| 5 | Chain detection service, impact query | Chain/impact UI panels | — | ~8 | ~4 |

---

## References

- **GitNexus source:** `/Users/larry/workspace/kg/GitNexus/`
  - Hybrid search: `gitnexus/src/core/search/hybrid-search.ts`
  - Community detection: `gitnexus/src/core/ingestion/community-processor.ts`
  - Process tracing: `gitnexus/src/core/ingestion/process-processor.ts`
  - Type resolution: `gitnexus/src/core/ingestion/type-env.ts`
  - Sigma.js viz: `gitnexus-web/src/components/GraphCanvas.tsx`
  - Graph adapter: `gitnexus-web/src/lib/graph-adapter.ts`
  - Embeddings: `gitnexus/src/core/embeddings/`
- **B2 source:** `/Users/larry/workspace/kg/b2/`
  - Current similarity: `backend/domain/services/similarity_calculator.go`
  - Edge discovery: `backend/domain/services/edge_discovery.go`
  - Graph visualization: `frontend/src/features/memories/components/GraphVisualization.tsx`
  - Graph analytics: `backend/domain/services/graph_analytics_service.go`
