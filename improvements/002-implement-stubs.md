# 002 — Implement Stub Repository Methods

## Priority: CRITICAL
## Effort: Large
## Files: 1 main file + tests

## Problem

`backend/infrastructure/persistence/dynamodb/repository_extensions.go` contains
15 repository methods that all return `"not yet implemented"` errors. Several
of these are called by application services or will be needed by the MCP server.

## Current State

Every method in `repository_extensions.go` (lines 17-109) looks like:
```go
func (r *NodeRepository) FindConnectedNodes(ctx context.Context, nodeID valueobjects.NodeID, maxDepth int) ([]*entities.Node, error) {
    return nil, fmt.Errorf("FindConnectedNodes not yet implemented")
}
```

## Methods to Implement (by priority)

### Tier 1 — Needed for core graph operations and MCP server

#### `NodeRepository.FindConnectedNodes(nodeID, maxDepth)` — LARGE
- **Used by:** Graph traversal, MCP `neighbors` tool, thought chains
- **Implementation:** BFS from nodeID using edge queries
  1. Get edges for nodeID via `EdgeRepository.GetByNodeID()`
  2. Collect target node IDs
  3. For each depth level, repeat edge query on frontier nodes
  4. Batch load all discovered node IDs
  5. Return unique nodes up to maxDepth
- **DynamoDB pattern:** Multiple Query calls (one per frontier node per depth)
- **Consider:** Caching adjacency for frequently traversed nodes

#### `NodeRepository.GetMostConnected(graphID, limit)` — MEDIUM
- **Used by:** MCP `god_nodes` tool, graph report
- **Implementation:**
  1. Get all edges for graph via `EdgeRepository.GetByGraphID()`
  2. Count edges per node (both source and target)
  3. Sort by count descending
  4. Load top N nodes by ID
- **DynamoDB pattern:** Single Query for edges, in-memory aggregation

#### `NodeRepository.FindRecentlyUpdated(userID, limit)` — SMALL
- **Used by:** MCP `recent` tool, dashboard
- **Implementation:**
  1. Query nodes by PK (user's graph)
  2. Sort by UpdatedAt descending
  3. Limit to N results
- **DynamoDB pattern:** Query with ScanIndexForward=false on a GSI that
  includes UpdatedAt as sort key. If no such GSI exists, Query + in-memory sort.

#### `NodeRepository.FindByContentPattern(userID, pattern)` — MEDIUM
- **Used by:** Search fallback, content matching
- **Implementation:**
  1. Query all nodes for user
  2. Filter in-memory where title or body contains pattern (case-insensitive)
- **DynamoDB pattern:** Query + FilterExpression with `contains()`.
  Note: DynamoDB `contains` is case-sensitive, so may need to store
  lowercase variants or filter in application code.

### Tier 2 — Needed for analytics and community features

#### `NodeRepository.CountByStatus(userID)` — SMALL
- **Implementation:** Query nodes for user, group by Status field, count
- **DynamoDB pattern:** Query + in-memory grouping (no native GROUP BY)

#### `NodeRepository.FindOrphanedNodes(graphID)` — MEDIUM
- **Implementation:**
  1. Get all node IDs in graph
  2. Get all edge source/target IDs in graph
  3. Nodes not in any edge = orphaned
- **DynamoDB pattern:** Two Queries (nodes + edges), set difference in memory

#### `EdgeRepository.FindByType(graphID, edgeType)` — SMALL
- **Implementation:** Query edges for graph, filter by Type attribute
- **DynamoDB pattern:** Query with FilterExpression `Type = :type`

#### `EdgeRepository.FindStrongConnections(graphID, minWeight)` — SMALL
- **Implementation:** Query edges for graph, filter by Weight >= minWeight
- **DynamoDB pattern:** Query with FilterExpression `Weight >= :min`

#### `EdgeRepository.FindBidirectionalEdges(graphID)` — SMALL
- **Implementation:** Query edges for graph, filter Bidirectional=true
- **DynamoDB pattern:** Query with FilterExpression

#### `EdgeRepository.CountByType(graphID)` — SMALL
- **Implementation:** Query edges for graph, group by Type, count
- **DynamoDB pattern:** Query + in-memory grouping

#### `EdgeRepository.GetEdgesBetweenNodes(graphID, nodeIDs)` — MEDIUM
- **Implementation:**
  1. For each nodeID pair, check if edge exists
  2. Or: get all edges for graph, filter where both source and target in nodeIDs set
- **DynamoDB pattern:** Single Query for all edges + in-memory filter
  (more efficient than N^2 point lookups for large node sets)

### Tier 3 — Nice to have

#### `GraphRepository.FindByNodeCount(userID, min, max)` — SMALL
- **Implementation:** Query graphs for user, filter by NodeCount range
- **DynamoDB pattern:** Query + FilterExpression

#### `GraphRepository.FindMostActive(userID, limit)` — SMALL
- **Implementation:** Query graphs for user, sort by UpdatedAt descending
- **DynamoDB pattern:** Query + in-memory sort

#### `GraphRepository.FindPublicGraphs(limit)` — SMALL
- **Implementation:** Scan with filter IsPublic=true (needs schema field)
- **Note:** May need to add `IsPublic` field to graph entity first

#### `GraphRepository.GetGraphStatistics(graphID)` — SMALL
- **Implementation:** Aggregate from existing metadata + queries
  ```go
  return GraphStatistics{
      NodeCount:          graph.Metadata().NodeCount,
      EdgeCount:          graph.Metadata().EdgeCount,
      OrphanedNodeCount:  len(orphanedNodes),
      AverageConnections: float64(edgeCount*2) / float64(nodeCount),
      MaxConnections:     maxDegree,
      ClusterCount:       communityCount,
  }
  ```

#### `GraphRepository.CountUserGraphs(userID)` — SMALL
- **Implementation:** Query graphs for user, return count
- **DynamoDB pattern:** Query with Select=COUNT

## Implementation Order

1. `FindConnectedNodes` + `GetMostConnected` (needed for MCP server)
2. `FindRecentlyUpdated` + `FindOrphanedNodes` (needed for graph report)
3. `GetGraphStatistics` + `CountByStatus` (needed for overview)
4. Edge query methods (needed for analysis features)
5. Remaining graph methods

## Testing

Each method needs:
- Unit test with in-memory repository (already exists as test doubles)
- Integration test against DynamoDB Local (docker)
- Edge case tests: empty graph, single node, large graph (1000+ nodes)
