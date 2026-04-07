# 012 — Export & Multi-Medium Support

## Priority: MEDIUM
## Effort: Medium
## Depends on: 008 (MCP server), 010 (brain report)

## Goal

Let B2's knowledge graph flow into other tools and mediums beyond
the web UI and Claude Code.

---

## Feature 1: Obsidian Export

Export the entire knowledge graph as an Obsidian-compatible vault.
Each memory becomes a markdown file with wikilinks to connected memories.

### Output Structure
```
b2-vault/
├── Machine Learning/
│   ├── Transformer Architecture.md
│   ├── Attention Mechanisms.md
│   └── LoRA Adapters.md
├── Distributed Systems/
│   ├── CAP Theorem.md
│   └── Raft Consensus.md
├── _index.md          # Overview with links to all clusters
└── _god-nodes.md      # Most connected ideas
```

### File Format
```markdown
---
tags: [machine-learning, transformers]
created: 2026-03-15
community: Machine Learning
connections: 12
---

# Transformer Architecture

The transformer architecture uses self-attention mechanisms to process
sequences in parallel, unlike RNNs which process sequentially...

## Connected Ideas
- [[Attention Mechanisms]] — Share keywords: attention, self-attention
- [[Embeddings]] — Semantically similar content
- [[BERT Notes]] — Share tags: transformers, nlp

## Part of: Machine Learning cluster
```

### Implementation

**CLI command:** `b2 export obsidian [--output ./b2-vault]`

```go
func ExportObsidian(client *B2Client, graphID, outputDir string) error {
    graphData := client.GetGraphData(graphID)
    
    // Group nodes by community
    communities := groupByCommunity(graphData.Nodes, graphData.Communities)
    
    for commName, nodes := range communities {
        commDir := filepath.Join(outputDir, sanitizeFilename(commName))
        os.MkdirAll(commDir, 0755)
        
        for _, node := range nodes {
            filename := sanitizeFilename(node.Title) + ".md"
            content := renderObsidianFile(node, graphData.Edges)
            os.WriteFile(filepath.Join(commDir, filename), content, 0644)
        }
    }
    
    // Generate index
    renderIndex(outputDir, communities, graphData)
    renderGodNodes(outputDir, graphData)
    
    return nil
}
```

### Wikilink Generation
```go
func renderConnections(node Node, edges []Edge, allNodes map[string]Node) string {
    lines := []string{"## Connected Ideas"}
    for _, edge := range edgesFor(node, edges) {
        target := allNodes[edge.TargetID]
        link := fmt.Sprintf("- [[%s]]", target.Title)
        if edge.Explanation != "" {
            link += fmt.Sprintf(" — %s", edge.Explanation)
        }
        lines = append(lines, link)
    }
    return strings.Join(lines, "\n")
}
```

### Effort: Medium

---

## Feature 2: JSON Full Export

Complete graph dump for backup, migration, or external analysis.

**CLI command:** `b2 export json [--output graph.json]`

```json
{
  "exported_at": "2026-04-07T12:00:00Z",
  "graph_id": "...",
  "stats": {
    "nodes": 247,
    "edges": 891,
    "communities": 12
  },
  "nodes": [
    {
      "id": "...",
      "title": "Transformer Architecture",
      "body": "...",
      "tags": ["ml", "transformers"],
      "community_id": "3",
      "created_at": "2026-03-15T...",
      "updated_at": "2026-04-01T...",
      "metadata": { "source": "web-ui" }
    }
  ],
  "edges": [
    {
      "source_id": "...",
      "target_id": "...",
      "type": "strong",
      "weight": 0.87,
      "explanation": "Share keywords: transformer, attention",
      "discovery_method": "hybrid"
    }
  ],
  "communities": [
    {
      "id": "3",
      "label": "Machine Learning",
      "keywords": ["transformer", "attention", "embeddings"],
      "node_count": 42,
      "cohesion": 0.78
    }
  ]
}
```

### Effort: Small

---

## Feature 3: JSON Import

Import a previously exported graph or merge with existing.

**CLI command:** `b2 import json [--file graph.json] [--merge]`

- Without `--merge`: replace entire graph (destructive, confirm first)
- With `--merge`: add new nodes/edges, skip duplicates (by title match)

### Effort: Medium

---

## Feature 4: API Key Management

Currently B2 uses JWT auth. For agent connections, we need long-lived
API keys that can be scoped and revoked.

### Endpoints
```
POST   /api/v1/api-keys          # Create new API key
GET    /api/v1/api-keys          # List active keys
DELETE /api/v1/api-keys/{keyID}  # Revoke a key
```

### Key Properties
```json
{
  "id": "key_...",
  "name": "claude-code-laptop",
  "permissions": ["read", "write"],  // or ["read"] for read-only agents
  "created_at": "2026-04-07T...",
  "last_used_at": "2026-04-07T...",
  "expires_at": null  // or a date
}
```

### Auth Middleware Update
Accept both JWT (web UI) and API key (agents) in the Authorization header:
```
Authorization: Bearer <jwt-token>     # Web UI
Authorization: ApiKey <api-key>       # MCP server / agents
```

### Effort: Medium

---

## Feature 5: Webhook Notifications

Notify connected agents when the graph changes significantly.

### Events to Notify
- New memory created (with auto-discovered connections)
- New community detected (Leiden found a new cluster)
- New god node (a memory became highly connected)
- Bridge detected (a memory connects two communities)

### Implementation
Store webhook URLs per API key:
```json
{
  "api_key_id": "key_...",
  "webhook_url": "https://...",
  "events": ["memory.created", "community.new", "god_node.new"]
}
```

On event, POST to webhook URL with event data.

### Effort: Medium-Large (new infrastructure)

---

## Feature 6: Markdown/Plain Text Import

Bulk import from a folder of markdown files (e.g., existing Obsidian vault).

**CLI command:** `b2 import markdown [--dir ./notes]`

For each `.md` file:
1. Extract title from `# heading` or filename
2. Extract body from content
3. Extract tags from YAML frontmatter
4. Create node via API
5. Let B2's auto-connection pipeline discover edges

### Effort: Small-Medium

---

## Priority Order

1. **Obsidian export** — highest value, lets you browse your brain in Obsidian
2. **JSON export/import** — backup and migration
3. **Markdown import** — onboard existing notes
4. **API key management** — needed for secure multi-agent access
5. **Webhook notifications** — nice to have, not critical

## New Files

```
backend/cmd/b2cli/
├── export_obsidian.go   # Obsidian vault generation
├── export_json.go       # JSON full export
├── import_json.go       # JSON import with merge support
└── import_markdown.go   # Markdown folder import
```

For API keys and webhooks (if implemented):
```
backend/interfaces/http/rest/handlers/apikey_handler.go
backend/domain/core/entities/api_key.go
backend/infrastructure/persistence/dynamodb/apikey_repository.go
```
