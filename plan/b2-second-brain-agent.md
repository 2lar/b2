# B2: Second Brain Agent — Implementation Plan

## Vision

B2 is your second brain. You add notes, memories, and thoughts via the web UI.
The system auto-connects them into a knowledge graph (Leiden communities, semantic
similarity, thought chains). **The missing piece**: an agent interface that lets
Claude Code (or any agent) tap into this brain — so it "knows" everything you know.

The knowledge graph already exists. We're building the bridge.

## Key Insight: B2's API Already Exists

B2 has a full REST API running on AWS:
- CRUD nodes, edges, graphs
- Hybrid search (BM25 + semantic)
- Community detection, thought chains, impact analysis
- Graph data for visualization

**We don't need local storage.** The MCP server is a thin Go binary that calls
B2's cloud API. This is the fastest path to "connect any agent to my second brain."

---

## Phase 1: MCP Server — The Brain Interface
**Goal:** Claude Code can query your second brain.

### What to build

**New file: `backend/cmd/mcp/main.go`** — MCP stdio server (~400 lines)

A Go binary that speaks MCP over stdio and calls B2's REST API.

Tools to expose (10 tools):

| Tool | What it does | Maps to |
|------|-------------|---------|
| `recall` | "What do I know about X?" — semantic search across all memories | `GET /api/v1/search?query=X` |
| `remember` | "Remember this for me: ..." — add a new memory | `POST /api/v1/nodes` |
| `get_memory` | Get full details of a specific memory | `GET /api/v1/nodes/{id}` |
| `connect` | Manually link two memories | `POST /api/v1/edges` |
| `neighbors` | What's connected to this memory? | `GET /api/v1/nodes` + edge traversal |
| `communities` | What are my main clusters of thought? | Leiden community data from graph |
| `god_nodes` | What are my most central ideas? | `GET /api/v1/graphs/{id}/stats` + most connected |
| `thought_chain` | Trace a chain of thought from a memory | `GET /api/v1/nodes/{id}/chains` |
| `recent` | What have I been thinking about lately? | `GET /api/v1/nodes` (sorted by updated) |
| `graph_overview` | Summary stats — how big is my brain? | `GET /api/v1/graphs/{id}/stats` |

Config: `~/.b2/config.json`
```json
{
  "api_url": "https://api.brain2.com",
  "api_key": "...",
  "default_graph_id": "..."
}
```

### New files
```
backend/cmd/mcp/
├── main.go          # entrypoint, config loading
├── server.go        # MCP server setup, tool registration
├── tools.go         # tool handler implementations
└── client.go        # B2 API client (HTTP calls to cloud)
```

### Dependencies
- `github.com/mark3labs/mcp-go` (Go MCP library)
- B2's existing REST API (no new backend changes needed)

---

## Phase 2: Claude Code Integration
**Goal:** Claude Code automatically knows about your second brain.

### What to build

**New file: `backend/cmd/b2cli/main.go`** — CLI for setup/management

Commands:
```
b2 install              # Register as Claude Code skill
b2 claude install       # Write CLAUDE.md rules + PreToolUse hook
b2 claude uninstall     # Clean removal
b2 status               # Show connection status, graph stats
```

**Skill file: `skill.md`**
- Registered at `~/.claude/skills/b2/SKILL.md`
- Triggered by `/b2` slash command
- Instructions telling Claude: "You have access to the user's second brain.
  Before answering personal questions, check the knowledge graph. When the user
  shares something worth remembering, offer to save it."

**PreToolUse hook** (`.claude/settings.json`):
- Fires before searches, nudges Claude: "The user has a second brain (B2).
  Use the `recall` tool to check if relevant memories exist."

**CLAUDE.md section:**
```markdown
## b2 — Second Brain

You have access to the user's personal knowledge graph via B2 MCP tools.

Rules:
- When the user asks about their notes, ideas, or past thoughts — use `recall` first
- When the user shares something they want to remember — use `remember`
- When exploring connections — use `thought_chain` and `neighbors`
- You are an extension of the user's memory. Be proactive about surfacing relevant context.
```

### New files
```
backend/cmd/b2cli/
├── main.go          # CLI entrypoint
├── install.go       # skill registration
├── claude.go        # CLAUDE.md + hook management
└── status.go        # connection health check
skill.md             # Claude Code skill definition
```

---

## Phase 3: Graph Report + Richer Context
**Goal:** The agent doesn't just query — it understands the shape of your knowledge.

### What to build

**`b2 report`** — generates `B2_BRAIN_REPORT.md`

Like Graphify's GRAPH_REPORT.md but for personal knowledge:
- **Knowledge clusters**: your Leiden communities with human-readable labels
  (e.g., "Community 3: Machine Learning, Transformers, Attention — 12 memories")
- **Core ideas**: god nodes — your most connected thoughts
- **Recent activity**: what you've been adding/updating
- **Bridges**: memories that connect different areas of your thinking
- **Orphans**: isolated memories that might need connections
- **Suggested explorations**: "You have thoughts about X and Y but haven't
  connected them — explore?"

This report gets auto-regenerated and Claude reads it for context.

### New files
```
backend/cmd/mcp/report.go    # report generation logic
```

---

## Phase 4: Smarter Auto-Connections
**Goal:** When you add a memory, B2 doesn't just find similar keywords —
it understands semantic meaning and creates richer connections.

### What to improve

B2 already has edge discovery (similarity calculator + edge discovery service).
Improvements:

1. **Better embedding model** — ensure the embed-node Lambda uses a strong model
   (e.g., Voyage, Cohere, or OpenAI ada-002) for richer semantic similarity
2. **Cross-community bridging** — when a new memory touches multiple communities,
   automatically create bridge edges (not just nearest-neighbor)
3. **Temporal connections** — link memories created around the same time/context
4. **Agent-assisted connections** — when Claude Code adds a memory via `remember`,
   include the conversation context as metadata so future edge discovery is richer
5. **Connection explanations** — store WHY two memories are connected (not just
   a weight), so the agent can explain relationships

### Files to modify
```
backend/domain/services/similarity_calculator.go
backend/domain/services/edge_discovery.go
backend/cmd/connect-node/        # the async edge discovery Lambda
```

---

## Phase 5: Multi-Medium Support
**Goal:** Connect B2 to more than just Claude Code.

- **Obsidian export**: `b2 export obsidian` — vault with wikilinks
- **API key management**: multiple agents can connect with different permissions
- **Webhook on graph change**: notify connected agents when knowledge updates
- **Mobile-friendly API**: lightweight endpoints for a future mobile app

---

## Build Order

```
Phase 1 (MCP Server)     ← THIS FIRST — gets the bridge working
  ↓
Phase 2 (Claude Code)    ← makes it seamless in daily workflow
  ↓
Phase 3 (Graph Report)   ← gives the agent deeper understanding
  ↓
Phase 4 (Smarter Auto)   ← improves the brain itself
  ↓
Phase 5 (Multi-Medium)   ← expand beyond Claude Code
```

Phase 1 is ~400 lines of Go. Phase 2 is ~300 lines of Go + config.
Both can be done quickly because they're thin layers over B2's existing API.

## What Makes This Different From Graphify

- **Personal, not code** — memories, thoughts, ideas (not AST extraction)
- **Persistent cloud graph** — always available, not project-local
- **Grows over time** — the more you use it, the better the agent "knows" you
- **Richer analysis** — thought chains, impact analysis, Leiden communities
- **The "second me" quality** — the agent isn't just searching files,
  it's navigating your personal knowledge network
