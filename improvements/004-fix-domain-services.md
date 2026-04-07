# 004 — Fix Domain Service Bugs

## Priority: HIGH
## Effort: Medium
## Files: 4 files modified

---

## Issue 1: Thought Chain Cycle Detection Bug

**File:** `backend/domain/services/thought_chain_service.go`
**Lines:** 156-160

**Bug:** The DFS backtracking deletes nodes from `visited` map after recursion,
which is correct for finding ALL paths. But the `visited` check at the top
(line ~148: `if visited[nb.id] { continue }`) uses the same map that gets
mutated by backtracking. This means:

- Path A→B→C is explored, B and C are unvisited after backtracking
- If there's an edge C→B, and we explore A→D→C→B, B is allowed because
  it was unmarked during backtracking from the first path
- This is actually correct behavior for enumerating distinct paths

**Real bug:** The `visited` map is shared across ALL calls to `dfsTrace`
from the same starting node. The starting node itself is marked visited
before the first call but never unmarked. If the starting node appears as
a neighbor deeper in the graph, it's correctly skipped. BUT:

```go
// Line 156-159:
visited[nb.id] = true
newPath := append(append([]string{}, path...), nb.id)
s.dfsTrace(adj, communityOf, nb.id, newPath, visited, depth+1, cfg, chains)
delete(visited, nb.id)
```

The real issue: if two branches from the same node converge on a shared
descendant, the second branch can revisit nodes the first branch already
explored and backtracked from. This can cause exponential path explosion
on dense graphs.

**Fix:** Use the path itself for cycle detection (O(path_length) per check,
but paths are bounded by MaxDepth):

```go
// Replace visited check with path-based cycle check:
inPath := false
for _, p := range path {
    if p == nb.id {
        inPath = true
        break
    }
}
if inPath {
    continue
}
```

Or convert path to a set for O(1) lookup:
```go
pathSet := make(map[string]bool, len(path))
for _, p := range path {
    pathSet[p] = true
}
// Then: if pathSet[nb.id] { continue }
```

**Also add:** A global chain limit to prevent exponential blowup:
```go
if len(*chains) >= cfg.MaxChains * 10 { // Hard cap
    return
}
```

**Effort:** Small — ~10 lines changed

---

## Issue 2: Impact Analysis Tier Classification

**File:** `backend/domain/services/impact_analysis_service.go`
**Lines:** 131-152

**Bug:** Tier classification uses only depth and community membership.
It ignores edge weight (a 0.95 weight connection at depth 2 should be
LIKELY_AFFECTED, not MAY_AFFECT) and doesn't consider alternative paths
(removing a node with only one path to a dependent is higher impact than
one with multiple paths).

**Fix:** Incorporate edge weight into classification:

```go
if depth == 1 {
    willBreak[depth] = append(willBreak[depth], nodeID)
} else if depth == 2 {
    // Check if any edge to this node has high weight
    maxWeight := maxEdgeWeightTo(nodeID, adj, edgeWeights)
    if nodeCommunity != "" && nodeCommunity == targetCommunity {
        likelyAffected[depth] = append(likelyAffected[depth], nodeID)
    } else if maxWeight >= 0.7 {
        // Strong indirect connection
        likelyAffected[depth] = append(likelyAffected[depth], nodeID)
    } else {
        mayAffect[depth] = append(mayAffect[depth], nodeID)
    }
} else {
    mayAffect[depth] = append(mayAffect[depth], nodeID)
}
```

**Also:** The risk level calculation (lines 195-205) should factor in
total weight of affected edges, not just count:

```go
totalWeight := sumEdgeWeights(targetStr, adj, edgeWeights)
if bridges && (directCount >= 5 || totalWeight >= 4.0) {
    riskLevel = "CRITICAL"
}
```

**Effort:** Medium — ~30 lines changed, need to thread edge weights through

---

## Issue 3: BM25 Tokenizer Edge Cases

**File:** `backend/domain/services/bm25.go`
**Lines:** 122-129

**Bug:** The tokenizer:
- Strips all non-alphanumeric chars: "node-id" → "nodeid", "C++" → "c"
- Filters words <= 1 char: "Go", "R", "C" are dropped
- Doesn't handle contractions: "don't" → ["don", "t"]

**Fix:**
```go
func (t *DefaultTextAnalyzer) Tokenize(text string) []string {
    // Preserve hyphens and underscores within words
    // Split on whitespace and punctuation (except hyphens/underscores between alphanums)
    re := regexp.MustCompile(`[a-zA-Z0-9]+(?:[-_][a-zA-Z0-9]+)*|\S+`)
    rawTokens := re.FindAllString(strings.ToLower(text), -1)

    tokens := make([]string, 0, len(rawTokens))
    for _, token := range rawTokens {
        // Keep single-char tokens if they're uppercase in original (likely proper nouns/languages)
        // Or keep any token > 1 char
        if len(token) > 1 || isSignificantSingleChar(token) {
            tokens = append(tokens, token)
        }
    }
    return tokens
}

func isSignificantSingleChar(s string) bool {
    // Known single-char terms worth keeping
    significant := map[string]bool{"c": true, "r": true, "go": true}
    return significant[s]
}
```

**Effort:** Small — ~20 lines changed

---

## Issue 4: Leiden Community Detection Merge Loop

**File:** `backend/domain/services/leiden.go`
**Lines:** 293-339 (`mergeSmall()`)

**Bug:** The merge loop for small communities runs with `for` and no
iteration limit. If merging creates new small communities, it could
theoretically loop forever.

**Fix:** Add max iterations:
```go
func (lg *LeidenGraph) mergeSmall(minSize int) {
    maxIterations := len(lg.communities) // Can't merge more times than communities exist
    for iteration := 0; iteration < maxIterations; iteration++ {
        merged := false
        for cid, members := range lg.communities {
            if len(members) >= minSize {
                continue
            }
            // ... existing merge logic ...
            merged = true
        }
        if !merged {
            break // No more merges possible
        }
    }
}
```

**Effort:** Small — 5 lines changed

---

## Issue 5: Leiden Refinement Fragmentation

**File:** `backend/domain/services/leiden.go`
**Lines:** 223-291 (`refine()`)

**Bug:** The refine step splits disconnected components within communities
into separate communities. But it doesn't re-evaluate whether these
fragments should merge with other communities. This can over-fragment
the graph.

**Fix:** After refine, run one additional modularity optimization pass:
```go
func (s *LeidenService) RunLeiden(graph *LeidenGraph) *LeidenResult {
    // ... existing phases ...
    graph.refine()
    
    // Additional optimization pass after refinement to re-merge fragments
    graph.optimizeModularity(s.config.Resolution)
    
    // ... rest of method ...
}
```

**Effort:** Small — 3 lines added

---

## Verification

- [ ] Create 10 interconnected nodes → thought chains don't explode exponentially
- [ ] Impact analysis on a hub node → depth-2 strong connections are LIKELY_AFFECTED
- [ ] Search for "Go programming" → "Go" not filtered out
- [ ] Leiden on 50-node graph → no infinite loop, communities are stable
- [ ] Leiden on graph with thin bridges → communities not over-fragmented
