# Learnings from Graphify — What B2 Should Adopt

Graphify (at `/Users/larry/workspace/kg/graphify`) is a knowledge graph tool
for code repositories and research papers. While B2 focuses on personal
knowledge (not code), Graphify solved several problems that B2 needs to solve
too. This document captures those patterns so we don't need to reference
the Graphify codebase when working on B2.

---

## 1. MCP Server Pattern

Graphify exposes 7 tools via MCP (Model Context Protocol) over stdio.
B2 should follow this exact pattern.

### How Graphify Does It (`graphify/serve.py`)

```
Server("graphify") → registers tools → runs via stdio_server()

Tools:
- query_graph:  BFS/DFS traversal with keyword search + token budget
- get_node:     Full details by label or ID (case-insensitive match)
- get_neighbors: Direct neighbors with edge relation filtering
- get_community: All nodes in a Leiden community by ID
- god_nodes:    Most connected nodes (uses analyze.god_nodes())
- graph_stats:  Node/edge counts, community count, confidence breakdown
- shortest_path: Shortest path between two concepts with hop limit
```

**Key design decisions to adopt:**
- Token budget on query results (default 2000 tokens, ~3 chars/token)
- Text-formatted output (not JSON) — optimized for LLM consumption
- BFS for broad context, DFS for tracing specific paths
- Case-insensitive node matching by label
- Sorted by degree (most connected first) in subgraph output

**B2's MCP should extend this with:**
- `recall`: semantic search (Graphify doesn't have embeddings)
- `remember`: write back to graph (Graphify is read-only)
- `thought_chain`: trace thought paths (Graphify doesn't have this)
- `recent`: time-based queries (personal knowledge is temporal)

### Go MCP Libraries
- `github.com/mark3labs/mcp-go` — well-maintained Go MCP server
- Supports stdio transport (what Claude Code uses)
- Tool registration with JSON Schema input validation

---

## 2. Claude Code Integration Pattern

Graphify uses a three-layer integration that B2 should replicate.

### Layer 1: Skill Registration
- File: `~/.claude/skills/graphify/SKILL.md`
- Trigger: user types `/graphify`
- Content: Full instruction prompt for Claude (what tools exist, when to use them)
- Registration: One line in `~/.claude/CLAUDE.md` pointing to the skill file

### Layer 2: CLAUDE.md Project Rules
- File: `./CLAUDE.md` in the project root (local, project-specific)
- Content: Standing instructions that persist across sessions
- Graphify writes:
  ```
  ## graphify
  - Before answering architecture questions, read graphify-out/GRAPH_REPORT.md
  - Navigate wiki/index.md instead of reading raw files
  - After modifying code, run rebuild to keep graph current
  ```
- **B2 equivalent:**
  ```
  ## b2 — Second Brain
  - When user asks about their notes/ideas/past thoughts, use `recall` first
  - When user shares something worth remembering, use `remember`
  - When exploring connections, use `thought_chain` and `neighbors`
  - You are an extension of the user's memory. Be proactive.
  ```

### Layer 3: PreToolUse Hook
- File: `.claude/settings.json`
- Hook type: `PreToolUse` (fires before `Glob|Grep` tool calls)
- Effect: Shell command echoes a nudge message that Claude sees
- Graphify's hook:
  ```json
  {
    "matcher": "Glob|Grep",
    "hooks": [{
      "type": "command",
      "command": "[ -f graphify-out/graph.json ] && echo 'Read GRAPH_REPORT.md before searching' || true"
    }]
  }
  ```
- **B2 equivalent:** Nudge Claude to check the brain before searching files
  ```json
  {
    "matcher": "Glob|Grep",
    "hooks": [{
      "type": "command",
      "command": "echo 'B2: You have access to the user second brain. Use recall to check if relevant memories exist before searching files.'"
    }]
  }
  ```

### Install/Uninstall CLI
Graphify has idempotent install/uninstall commands:
- `graphify install` — copy skill file
- `graphify claude install` — write CLAUDE.md + hook
- `graphify claude uninstall` — clean removal (regex-based section removal)
- `graphify hook install` — git hooks (post-commit, post-checkout)
- `graphify hook status` — check if hooks are installed

B2 should have equivalent: `b2 install`, `b2 claude install`, etc.

---

## 3. Graph Report Pattern

Graphify generates `GRAPH_REPORT.md` — a one-page summary that Claude reads
before doing anything. This is the single most impactful integration feature.

### What Graphify's Report Contains
1. **Corpus check**: file count, word count, size warnings
2. **Summary**: node/edge counts, community count, confidence breakdown
3. **God nodes**: most-connected entities with edge counts
4. **Surprising connections**: cross-community and cross-file-type edges
5. **Communities**: cohesion scores and node lists per cluster
6. **Ambiguous edges**: flagged for human review
7. **Hyperedges**: group relationships (3+ nodes)
8. **Suggested questions**: generated from graph structure

### B2's Brain Report Should Contain
1. **Graph overview**: total memories, edges, communities, last updated
2. **Knowledge clusters**: Leiden communities with auto-generated labels
   (top keywords from each community's nodes)
3. **Core ideas**: god nodes — most connected memories with edge counts
4. **Recent activity**: last 10 memories added/updated with dates
5. **Bridges**: memories that connect multiple communities
6. **Orphans**: memories with zero or one connection
7. **Suggested explorations**: pairs of memories that SHOULD be connected
   but aren't (high similarity, no edge)
8. **Knowledge gaps**: communities with very few members (thin areas)

### How the Report Is Used
- Generated by `b2 report` CLI command
- Saved to `~/.b2/BRAIN_REPORT.md` or project-local
- Claude reads it on session start (via CLAUDE.md instruction)
- Refreshed periodically or on demand

---

## 4. God Node Analysis Pattern

Graphify identifies "god nodes" — the most connected entities in the graph.
It filters out synthetic nodes (file-level hubs, generic concepts) to surface
only meaningful core abstractions.

### How Graphify Does It (`graphify/analyze.py`)
```python
def god_nodes(G, top_n=10):
    # Sort by degree, filter out file-type nodes
    # Return [{label, edges, community, source_file}]
```

### B2 Equivalent
B2 already has `GetMostConnected()` in the repository interface (currently
a stub — Plan 002 implements it). The analysis should:
1. Get nodes sorted by edge count (in-degree + out-degree)
2. No filtering needed (all nodes are user-created memories, not synthetic)
3. Include community membership and top tags
4. Present as "Your most central ideas"

---

## 5. Surprising Connections Pattern

Graphify ranks "surprising" edges — connections that are unexpected based on
graph structure. These are often the most insightful.

### How Graphify Scores Surprise
- Higher confidence = less surprising (AMBIGUOUS > INFERRED > EXTRACTED)
- Cross file-type = more surprising (code↔paper > code↔code)
- Cross-community = more surprising (Leiden structural distance)
- Peripheral→hub = more surprising than hub↔hub

### B2 Equivalent — "Unexpected Connections"
For personal knowledge, "surprise" means:
- Cross-community edges (connecting different areas of thought)
- Low keyword overlap but high semantic similarity (meaning-based, not word-based)
- Connections between recently added and old memories (temporal surprise)
- Connections between memories with different tags/categories

This would be a new domain service: `SurprisingConnectionService`.

---

## 6. Confidence Tracking Pattern

Graphify labels every edge with a confidence level:
- `EXTRACTED` (1.0): directly stated in source
- `INFERRED` (0.0-1.0): reasonable deduction with score
- `AMBIGUOUS`: uncertain, flagged for human review

### B2 Equivalent
B2 already has edge types (strong, normal, weak) and similarity scores.
Extend with:
- `discovery_method`: "hybrid", "keyword", "semantic", "manual"
- `confidence`: 0.0-1.0 (from similarity score)
- `explanation`: WHY this edge exists ("Both discuss transformer architectures")

The explanation field is the key differentiator. When an agent asks "why are
these connected?", it should have an answer beyond "similarity score 0.73".

---

## 7. Export Patterns

Graphify exports to: JSON, HTML (vis.js interactive), SVG, GraphML, Neo4j
(Cypher), Obsidian vault, Wikipedia-style wiki.

### B2 Should Adopt
- **Obsidian export** (high value): Each memory → markdown file with wikilinks
  to connected memories. Drop into Obsidian vault for visual graph exploration.
- **JSON export**: Full graph dump for backup/migration
- **Brain Report** (already planned): Markdown summary

Lower priority:
- HTML interactive (B2 already has Sigma.js frontend)
- Neo4j export (nice to have, not critical)

---

## 8. Watch Mode / Auto-Sync Pattern

Graphify has `--watch` mode that auto-rebuilds the graph when files change.
Git hooks rebuild on commit/checkout.

### B2 Equivalent
B2 already has real-time WebSocket updates. The equivalent for agent
integration:
- When a memory is added/updated via web UI → MCP server sees it immediately
  (it calls the API, which always has latest data)
- When a memory is added via agent (`remember`) → web UI updates in real-time
  (existing WebSocket infrastructure)
- Brain Report could auto-regenerate on significant graph changes
  (new community detected, new god node, etc.)

---

## Summary: What to Port from Graphify to B2

| Graphify Feature | B2 Plan | Priority |
|-----------------|---------|----------|
| MCP server (7 tools) | Plan 008 — MCP Server | CRITICAL |
| Claude Code skill + hooks + CLAUDE.md | Plan 009 — Claude Code Integration | CRITICAL |
| GRAPH_REPORT.md | Plan 010 — Brain Report | HIGH |
| God node analysis | Already exists (needs stub fix, Plan 002) | Fixed in 002 |
| Surprising connections | Plan 011 — Smarter Connections | HIGH |
| Confidence/explanation on edges | Plan 011 — Smarter Connections | HIGH |
| Obsidian export | Plan 012 — Export & Multi-Medium | MEDIUM |
| Watch/auto-sync | Already works via WebSocket + API | N/A |
| AST extraction (13 languages) | NOT porting — B2 is personal, not code | N/A |
| Tree-sitter integration | NOT porting | N/A |
