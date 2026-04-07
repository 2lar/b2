# 008 — MCP Server: The Brain Interface

## Priority: CRITICAL (first new feature after backend fixes)
## Effort: Medium
## Depends on: 001 (embeddings), 002 (stub methods)

## Goal

Build an MCP (Model Context Protocol) stdio server in Go that lets any
agent — Claude Code, or anything that speaks MCP — query and write to
your second brain.

This is the single most important new feature. Everything else (Claude Code
integration, brain report, etc.) builds on this.

## Architecture

```
Claude Code ←→ MCP stdio ←→ b2-mcp binary ←→ B2 REST API (AWS)
```

The MCP server is a thin Go binary. It:
1. Reads config from `~/.b2/config.json` (API URL, auth token, default graph)
2. Starts an MCP stdio server
3. Registers 10 tools
4. Each tool handler makes HTTP calls to B2's existing REST API
5. Formats results as text optimized for LLM consumption

No new backend changes needed — it's a client of the existing API.

## Config

**File:** `~/.b2/config.json`
```json
{
  "api_url": "https://api.brain2.com",
  "token": "jwt-token-here",
  "default_graph_id": "graph-uuid",
  "token_budget": 2000
}
```

The token should be a long-lived JWT or API key. The MCP binary reads this
on startup. If the file doesn't exist, it prints setup instructions.

## Tools (10 total)

### Read Tools

#### `recall` — "What do I know about X?"
- **Input:** `{ "query": string, "limit?": int }`
- **Backend:** `GET /api/v1/search?query={query}&limit={limit}`
- **Output:** Ranked list of matching memories with title, snippet, similarity score, community
- **Token budget:** Truncate results to fit within configured budget
- **This is the primary tool.** If the agent doesn't know what tool to use, it uses this.

#### `get_memory` — Full details of a specific memory
- **Input:** `{ "id": string }` or `{ "title": string }` (fuzzy match)
- **Backend:** `GET /api/v1/nodes/{id}` or search by title
- **Output:** Full title, body, tags, categories, community, connections, created/updated dates

#### `neighbors` — What's connected to this memory?
- **Input:** `{ "id": string, "depth?": int (default 1), "limit?": int }`
- **Backend:** Uses FindConnectedNodes (Plan 002) or edge traversal
- **Output:** List of connected memories with edge type, weight, and direction

#### `communities` — What are my main knowledge clusters?
- **Input:** `{ "community_id?": int }` (optional — all communities if omitted)
- **Backend:** `GET /api/v1/graph-data` → extract community groupings
- **Output:** List of communities with member count, top keywords, and sample memories

#### `god_nodes` — What are my most central ideas?
- **Input:** `{ "top_n?": int (default 10) }`
- **Backend:** Uses GetMostConnected (Plan 002)
- **Output:** Ranked list of most-connected memories with edge count and community

#### `thought_chain` — Trace a chain of thought
- **Input:** `{ "id": string, "max_depth?": int, "max_branches?": int }`
- **Backend:** `GET /api/v1/nodes/{id}/chains`
- **Output:** Paths through the graph, highlighting community crossings

#### `recent` — What have I been thinking about lately?
- **Input:** `{ "limit?": int (default 20) }`
- **Backend:** Uses FindRecentlyUpdated (Plan 002)
- **Output:** Recent memories sorted by last updated, with title and tags

#### `graph_overview` — How big is my brain?
- **Input:** `{}`
- **Backend:** `GET /api/v1/graphs/{id}/stats`
- **Output:** Total memories, edges, communities, most active community, growth trend

### Write Tools

#### `remember` — Save a new memory
- **Input:** `{ "title": string, "body": string, "tags?": string[] }`
- **Backend:** `POST /api/v1/nodes`
- **Output:** Confirmation with node ID and auto-discovered connections
- **Note:** After creating, the backend's edge discovery pipeline kicks in
  automatically. The response should mention the discovered connections.

#### `connect` — Link two memories
- **Input:** `{ "source_id": string, "target_id": string, "type?": string }`
- **Backend:** `POST /api/v1/edges`
- **Output:** Confirmation with edge details

## Output Formatting

All tool outputs should be text, not JSON. Optimized for LLM consumption.

Example `recall` output:
```
Found 5 memories matching "distributed systems":

1. "CAP Theorem Notes" (score: 0.89)
   Community: Distributed Systems (#3)
   Tags: [distributed, consistency, availability]
   Connected to: 4 memories
   Snippet: "The CAP theorem states that a distributed system can only..."

2. "Raft Consensus Algorithm" (score: 0.82)
   Community: Distributed Systems (#3)
   Tags: [consensus, raft, distributed]
   Connected to: 6 memories
   Snippet: "Raft is a consensus algorithm designed to be understandable..."

[3 more results truncated to token budget]
```

## New Files

```
backend/cmd/mcp/
├── main.go          # Entrypoint: load config, start MCP server
├── config.go        # Config loading from ~/.b2/config.json
├── server.go        # MCP server setup, tool registration (all 10 tools)
├── tools_read.go    # Read tool handlers (recall, get_memory, neighbors, etc.)
├── tools_write.go   # Write tool handlers (remember, connect)
├── client.go        # HTTP client for B2 REST API (auth, retry, error handling)
└── format.go        # Output formatting (text, token budgeting, truncation)
```

## Dependencies

- `github.com/mark3labs/mcp-go` — Go MCP library (stdio transport)
- `net/http` — stdlib HTTP client for B2 API calls
- No B2 domain imports — this is a pure API client

## Build & Install

```bash
cd backend/cmd/mcp
go build -o b2-mcp .
# Copy to PATH
cp b2-mcp /usr/local/bin/
```

Or install via `go install`:
```bash
go install github.com/yourusername/b2/backend/cmd/mcp@latest
```

## Claude Code MCP Config

After building, register in Claude Code's MCP config:
```json
{
  "mcpServers": {
    "b2": {
      "command": "b2-mcp",
      "args": []
    }
  }
}
```

## Error Handling

- API unreachable → return "B2 is not reachable. Check your connection and ~/.b2/config.json"
- Auth expired → return "B2 auth token expired. Run `b2 auth refresh` to re-authenticate."
- Empty results → return "No memories found matching that query." (not an error)
- Rate limited → retry once after 1s, then return "B2 is rate limited. Try again in a moment."

## Testing

- Unit tests with mock HTTP server (test each tool handler)
- Integration test against running B2 instance
- Manual test: install in Claude Code, create a memory, recall it
