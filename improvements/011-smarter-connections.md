# 011 — Smarter Auto-Connections

## Priority: HIGH
## Effort: Medium-Large
## Depends on: 001 (embeddings working)

## Goal

Make B2's auto-connection pipeline produce BETTER edges. Currently it
discovers connections based on keyword overlap (and semantic similarity
once embeddings are enabled). This plan adds:

1. Re-evaluation after embedding generation
2. Cross-community bridge detection
3. Temporal connections
4. Connection explanations (WHY two memories are connected)
5. Surprising connection discovery
6. Agent-enriched metadata

## Change 1: Re-Evaluate Edges After Embedding

### Problem
Current flow: Node created → edges discovered (keyword-only) → embedding
generated async (seconds later). The first pass misses semantic connections.

### Solution
When the embed-node Lambda generates an embedding, publish a
`node.embedding.generated` event. A new handler re-runs edge discovery
for that node with the full hybrid similarity.

**File:** `backend/cmd/embed-node/main.go`
```go
// After saving embedding:
event := events.NewNodeEmbeddingGenerated(nodeID, graphID, time.Now())
eventPublisher.Publish(ctx, event)
```

**New event:** `backend/domain/events/node_events.go`
```go
type NodeEmbeddingGenerated struct {
    NodeID  string
    GraphID string
    // ...
}
```

**New EventBridge rule:** Route `node.embedding.generated` → connect-node Lambda

**In connect-node Lambda:** Re-run edge discovery with embeddings available.
Compare new edges against existing edges. Add new ones, optionally upgrade
weak keyword-only edges to strong hybrid edges.

### Impact
Every memory gets two passes: fast keyword-only on creation, then
high-quality hybrid after embedding is ready.

---

## Change 2: Cross-Community Bridge Detection

### Problem
When a new memory has high similarity to nodes in MULTIPLE communities,
it's a bridge — connecting different areas of thought. Currently B2
doesn't detect or highlight this.

### Solution

**File:** `backend/domain/services/edge_discovery.go`

After discovering edges, analyze community distribution:
```go
func (s *DefaultEdgeDiscoveryService) DetectBridges(
    node *entities.Node,
    candidates []EdgeCandidate,
    communityOf map[string]string,
) []BridgeInfo {
    // Group candidates by community
    communityCandidates := groupByCommunity(candidates, communityOf)
    
    // If edges span 2+ communities with score > 0.5 in each, it's a bridge
    if len(communityCandidates) >= 2 {
        bridges := []BridgeInfo{}
        for commID, edges := range communityCandidates {
            if maxScore(edges) >= 0.5 {
                bridges = append(bridges, BridgeInfo{
                    CommunityID: commID,
                    TopEdge:     edges[0],
                })
            }
        }
        return bridges
    }
    return nil
}
```

Store bridge status on the node:
```go
node.SetMetadataProperty("is_bridge", true)
node.SetMetadataProperty("bridges_communities", []string{"comm1", "comm2"})
```

### Impact
Brain Report can highlight bridge memories. Agent can say "This idea
connects your Machine Learning and Knowledge Graphs clusters."

---

## Change 3: Temporal Connections

### Problem
Memories created around the same time often relate to the same train of
thought, but B2 only connects based on content similarity.

### Solution

**File:** `backend/domain/services/edge_discovery.go`

Add temporal proximity as a signal in edge scoring:
```go
func temporalBoost(node1, node2 *entities.Node) float64 {
    timeDiff := node1.CreatedAt().Sub(node2.CreatedAt()).Abs()
    
    // Same hour: +0.1 boost
    // Same day: +0.05 boost
    // Same week: +0.02 boost
    switch {
    case timeDiff < 1*time.Hour:
        return 0.10
    case timeDiff < 24*time.Hour:
        return 0.05
    case timeDiff < 7*24*time.Hour:
        return 0.02
    default:
        return 0.0
    }
}
```

Integrate into similarity calculation:
```go
// In CalculateDetailed():
baseSimilarity := hybridScore(node1, node2)
temporalBonus := temporalBoost(node1, node2)
finalScore := math.Min(baseSimilarity + temporalBonus, 1.0)
```

### Impact
Notes taken during the same research session automatically cluster together.

---

## Change 4: Connection Explanations

### Problem
Edges have a weight (0.73) and a type (strong/weak) but no explanation of
WHY two memories are connected. When an agent asks "how are these related?",
it can only say "similarity score 0.73."

### Solution

**File:** `backend/domain/core/aggregates/edge.go` (or wherever Edge is defined)

Add an `Explanation` field to edges:
```go
type Edge struct {
    // ... existing fields ...
    Explanation     string  // Human-readable reason for this connection
    DiscoveryMethod string  // "hybrid", "keyword", "semantic", "manual", "temporal"
}
```

Generate explanations during edge discovery:
```go
func generateExplanation(node1, node2 *entities.Node, result SimilarityResult) string {
    sharedKeywords := intersectKeywords(node1, node2)
    sharedTags := intersectTags(node1, node2)
    
    parts := []string{}
    
    if len(sharedKeywords) > 0 {
        parts = append(parts, fmt.Sprintf("Share keywords: %s", 
            strings.Join(sharedKeywords[:min(3, len(sharedKeywords))], ", ")))
    }
    if len(sharedTags) > 0 {
        parts = append(parts, fmt.Sprintf("Share tags: %s",
            strings.Join(sharedTags, ", ")))
    }
    if result.Method == "hybrid" || result.Method == "semantic" {
        parts = append(parts, "Semantically similar content")
    }
    if temporalBoost(node1, node2) > 0 {
        parts = append(parts, "Created around the same time")
    }
    
    return strings.Join(parts, ". ")
}
```

**DynamoDB:** Add `Explanation` and `DiscoveryMethod` attributes to edge records.

### Impact
Agent can say: "These are connected because they share keywords 'transformer,
attention' and are semantically similar. They were also created the same day."

---

## Change 5: Surprising Connection Discovery

### Problem
Graphify identifies "surprising connections" — edges that are unexpected
based on graph structure. B2 doesn't surface these insights.

### Solution

**New file:** `backend/domain/services/surprising_connections.go`

```go
type SurprisingConnection struct {
    Edge         *aggregates.Edge
    SurpriseScore float64
    Reason       string
}

func FindSurprisingConnections(graph *aggregates.Graph, limit int) []SurprisingConnection {
    surprises := []SurprisingConnection{}
    
    for _, edge := range graph.Edges() {
        score := 0.0
        reasons := []string{}
        
        sourceNode := graph.Node(edge.SourceID)
        targetNode := graph.Node(edge.TargetID)
        
        // Cross-community connections are surprising
        if sourceNode.CommunityID() != targetNode.CommunityID() {
            score += 0.3
            reasons = append(reasons, "Bridges different knowledge areas")
        }
        
        // Low keyword overlap but high overall score = semantic surprise
        keywordSim := keywordSimilarity(sourceNode, targetNode)
        if keywordSim < 0.2 && edge.Weight() > 0.6 {
            score += 0.3
            reasons = append(reasons, "Connected by meaning, not words")
        }
        
        // Peripheral node connected to hub = surprising
        sourceDeg := graph.Degree(edge.SourceID)
        targetDeg := graph.Degree(edge.TargetID)
        if (sourceDeg <= 2 && targetDeg >= 10) || (targetDeg <= 2 && sourceDeg >= 10) {
            score += 0.2
            reasons = append(reasons, "Connects a niche idea to a core concept")
        }
        
        // Large temporal gap
        timeDiff := sourceNode.CreatedAt().Sub(targetNode.CreatedAt()).Abs()
        if timeDiff > 30*24*time.Hour {
            score += 0.2
            reasons = append(reasons, fmt.Sprintf("Connects ideas %d days apart",
                int(timeDiff.Hours()/24)))
        }
        
        if score > 0.3 {
            surprises = append(surprises, SurprisingConnection{
                Edge:          edge,
                SurpriseScore: score,
                Reason:        strings.Join(reasons, ". "),
            })
        }
    }
    
    sort.Slice(surprises, func(i, j int) bool {
        return surprises[i].SurpriseScore > surprises[j].SurpriseScore
    })
    
    return surprises[:min(limit, len(surprises))]
}
```

### Impact
Brain Report includes "Surprising Connections" section. Agent can surface
"Did you know your old note about X is connected to your recent note about Y?"

---

## Change 6: Agent-Enriched Metadata

### Problem
When Claude Code saves a memory via `remember`, it loses the conversation
context. The memory is just title + body, disconnected from why it was saved.

### Solution

The MCP `remember` tool should accept optional context:
```go
// In tools_write.go:
type RememberInput struct {
    Title   string   `json:"title"`
    Body    string   `json:"body"`
    Tags    []string `json:"tags,omitempty"`
    Context string   `json:"context,omitempty"` // Conversation context
    Source  string   `json:"source,omitempty"`  // "claude-code", "web-ui", etc.
}
```

Store context as metadata on the node:
```go
node.SetMetadataProperty("source", input.Source)
node.SetMetadataProperty("context", input.Context)
```

This enriches edge discovery — the context field provides additional
text for keyword and semantic similarity matching.

### Impact
Memories saved by the agent have richer metadata, leading to better
auto-connections.

---

## Files Modified

```
backend/domain/services/edge_discovery.go          # Bridge detection, temporal boost
backend/domain/services/similarity_calculator.go    # Temporal signal, explanation generation
backend/domain/services/surprising_connections.go   # NEW — surprising connection analysis
backend/domain/core/aggregates/edge.go              # Explanation + DiscoveryMethod fields
backend/domain/events/node_events.go                # NodeEmbeddingGenerated event
backend/cmd/embed-node/main.go                      # Publish event after embedding
backend/cmd/connect-node/main.go                    # Handle re-evaluation event
backend/infrastructure/persistence/dynamodb/edge_repository.go  # Save new fields
backend/cmd/mcp/tools_write.go                      # Context field on remember
```

## Testing

- Create two memories about similar topics → edge has explanation
- Create two memories 5 minutes apart → temporal boost applied
- Create memory that spans two communities → bridge detected
- Run surprising connections on 50-node graph → meaningful results
- Save memory via agent with context → context stored in metadata
