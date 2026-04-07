# 007 — Performance & Scalability

## Priority: MEDIUM
## Effort: Medium-Large
## Files: 5-8 files modified

---

## Issue 1: N+1 Node Loading

**File:** `backend/infrastructure/persistence/dynamodb/node_repository.go`
**Lines:** 804-825 (`LoadNodes`)

**Bug:** Loads nodes one-by-one in a loop:
```go
for _, nodeID := range nodeIDs {
    node, err := r.GetByID(ctx, nodeID)
    // ...
}
```

For a graph with 500 nodes, this is 500 separate DynamoDB GetItem calls.

**Fix:** Use DynamoDB BatchGetItem (up to 100 items per call):
```go
func (r *NodeRepository) LoadNodes(ctx context.Context, nodeIDs []valueobjects.NodeID) ([]*entities.Node, error) {
    nodes := make([]*entities.Node, 0, len(nodeIDs))
    
    // BatchGetItem processes up to 100 keys per call
    for i := 0; i < len(nodeIDs); i += 100 {
        end := min(i+100, len(nodeIDs))
        batch := nodeIDs[i:end]
        
        keys := make([]map[string]types.AttributeValue, len(batch))
        for j, id := range batch {
            keys[j] = map[string]types.AttributeValue{
                "PK": &types.AttributeValueMemberS{Value: fmt.Sprintf("NODE#%s", id.String())},
                "SK": &types.AttributeValueMemberS{Value: "METADATA"},
            }
        }
        
        input := &dynamodb.BatchGetItemInput{
            RequestItems: map[string]types.KeysAndAttributes{
                r.tableName: {Keys: keys},
            },
        }
        
        result, err := r.client.BatchGetItem(ctx, input)
        // ... handle result + UnprocessedKeys retry ...
    }
    return nodes, nil
}
```

**Impact:** 500 nodes → 5 BatchGetItem calls instead of 500 GetItem calls.
~100x fewer network roundtrips.

**Effort:** Medium

---

## Issue 2: No Pagination for Large Graphs

**File:** `backend/infrastructure/persistence/dynamodb/node_repository.go`
**Lines:** 428-456 (`GetByGraphID`)

**Bug:** Loads ALL nodes for a graph in a single query with no pagination
token. For large graphs (10k+ nodes), this causes:
- High memory usage
- Slow response times
- Potential Lambda timeout (30s)

**Fix:** Add pagination support:
```go
type PaginatedResult struct {
    Nodes         []*entities.Node
    NextPageToken string
    TotalCount    int
}

func (r *NodeRepository) GetByGraphIDPaginated(
    ctx context.Context,
    graphID string,
    pageSize int,
    pageToken string,
) (*PaginatedResult, error) {
    input := &dynamodb.QueryInput{
        TableName:              aws.String(r.tableName),
        KeyConditionExpression: aws.String("PK = :pk AND begins_with(SK, :sk)"),
        Limit:                 aws.Int32(int32(pageSize)),
    }
    if pageToken != "" {
        input.ExclusiveStartKey = decodePageToken(pageToken)
    }
    // ...
}
```

**Note:** Existing `GetByGraphID` should remain for internal use (edge discovery
needs all nodes). Paginated version is for API responses and MCP tools.

**Effort:** Medium

---

## Issue 3: Embedding Deserialization on Every Load

**File:** `backend/infrastructure/persistence/dynamodb/node_repository.go`
**Lines:** 247-252

**Bug:** Every node load deserializes the full embedding vector (1536 floats
= ~12KB per node). Most operations don't need embeddings (listing, graph
visualization, basic CRUD).

**Fix:** Lazy embedding loading via projection:
```go
// For operations that don't need embeddings:
func (r *NodeRepository) GetByGraphIDLight(ctx context.Context, graphID string) ([]*entities.Node, error) {
    input := &dynamodb.QueryInput{
        // ... same query ...
        ProjectionExpression: aws.String("PK, SK, Title, Body, #S, Tags, CommunityID, UpdatedAt, CreatedAt, Position"),
        ExpressionAttributeNames: map[string]string{"#S": "Status"},
    }
    // Nodes loaded without Embedding field
}
```

Use the light version for:
- Graph visualization (`GetGraphData`)
- Node listing
- Community detection (doesn't need embeddings)
- Search results (only need embeddings for the query, not results)

Use the full version for:
- Edge discovery (needs embeddings for similarity)
- Individual node detail view

**Impact:** For a 500-node graph: saves ~6MB of unnecessary data transfer.

**Effort:** Medium

---

## Issue 4: Community Service Saves Nodes One-by-One

**File:** `backend/application/services/community_service.go`
**Lines:** 134-147

**Bug:** After Leiden runs, each node's community ID is saved individually:
```go
for _, node := range nodes {
    node.SetCommunityID(communityID)
    if err := s.nodeRepo.Save(ctx, node); err != nil {
        // ...
    }
}
```

For 500 nodes, this is 500 DynamoDB PutItem calls.

**Fix:** Use batch write:
```go
// Batch update community IDs using BatchWriteItem
func (r *NodeRepository) BatchUpdateCommunityIDs(
    ctx context.Context,
    assignments map[valueobjects.NodeID]string, // nodeID → communityID
) error {
    items := make([]types.WriteRequest, 0, len(assignments))
    for nodeID, communityID := range assignments {
        // Use UpdateItem instead of full PutItem to only change CommunityID
        // ...
    }
    // Batch in groups of 25 (DynamoDB BatchWriteItem limit)
}
```

Or better, use DynamoDB UpdateItem (only updates one attribute, not full node):
```go
for nodeID, communityID := range assignments {
    input := &dynamodb.UpdateItemInput{
        TableName: aws.String(r.tableName),
        Key:       nodeKey(nodeID),
        UpdateExpression: aws.String("SET CommunityID = :cid, UpdatedAt = :now"),
        ExpressionAttributeValues: map[string]types.AttributeValue{
            ":cid": &types.AttributeValueMemberS{Value: communityID},
            ":now": &types.AttributeValueMemberS{Value: time.Now().Format(time.RFC3339)},
        },
    }
    // ... batch these with goroutines or BatchWriteItem
}
```

**Impact:** Reduces community recompute from 500 PutItems (~6MB writes)
to 500 UpdateItems (~50KB writes) or 20 BatchWriteItem calls.

**Effort:** Medium

---

## Issue 5: Graph Loading for Analysis Is Inefficient

**File:** `backend/application/services/analysis_service.go`
**Lines:** 127-164

**Bug:** `loadGraph()` loads all nodes and edges, then calls
`graph.LoadNode()` and `graph.LoadEdge()` individually:
```go
for _, node := range nodes {
    graph.LoadNode(node) // One at a time
}
for _, edge := range edges {
    graph.LoadEdge(edge) // One at a time
}
```

This is fine algorithmically but creates unnecessary object allocations.

**Fix:** Add bulk load to Graph aggregate:
```go
func (g *Graph) LoadBulk(nodes []*entities.Node, edges []*Edge) {
    g.nodes = make(map[string]*entities.Node, len(nodes))
    for _, n := range nodes {
        g.nodes[n.ID().String()] = n
    }
    g.edges = make(map[string]*Edge, len(edges))
    for _, e := range edges {
        g.edges[e.ID()] = e
    }
    g.metadata.NodeCount = len(nodes)
    g.metadata.EdgeCount = len(edges)
}
```

**Impact:** Minor — reduces allocations, not network calls. Low priority
compared to other items.

**Effort:** Small

---

## Issue 6: Edge Discovery Loads All Graph Nodes

**File:** `backend/application/sagas/create_node_saga.go`
**Lines:** 398-412

**Bug:** Edge discovery loads ALL nodes in the graph to calculate
similarity against the new node. For a 10k-node graph, this means
loading 10k nodes with embeddings (~120MB) just to find the top 20 connections.

**Fix (future optimization):** Use approximate nearest neighbor (ANN) search
instead of brute-force:

### Short term: Limit candidate set
```go
// Only compare against recent nodes + most connected nodes
candidates := append(
    recentNodes(graph, 200),     // Last 200 nodes
    mostConnected(graph, 100)..., // Top 100 hub nodes
)
```

### Medium term: Pre-compute embedding index
Store embeddings in a vector index (e.g., DynamoDB + custom index,
or a sidecar like Pinecone/Qdrant) for O(log n) similarity search.

### Long term: Move to a proper vector database
Use pgvector, Qdrant, or Pinecone alongside DynamoDB for vector operations.

**Effort:** Large (short-term: Medium)

---

## Priority Order

1. **N+1 node loading** (#1) — biggest bang for buck, straightforward fix
2. **Embedding projection** (#3) — reduces data transfer significantly
3. **Community batch save** (#4) — removes N individual writes
4. **Pagination** (#2) — needed before graph grows large
5. **Edge discovery optimization** (#6) — needed at scale, not urgent now
6. **Bulk graph loading** (#5) — minor optimization
