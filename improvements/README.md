# B2 Improvements — Complete Roadmap

This directory contains everything needed to make B2 a robust, production-ready
second brain with agent integration. No external references needed — all context,
decisions, and implementation details are self-contained here.

## Start Here

- [000 — Vision](./000-vision.md) — What B2 is, what it should become, the "second me" goal
- [Graphify Learnings](./GRAPHIFY-LEARNINGS.md) — Patterns extracted from the Graphify project
  that B2 should adopt (MCP server, Claude Code integration, brain report, etc.)

## Phase A: Fix the Foundation

The backend has real bugs and gaps that need fixing before we build new features.
These are ordered — each builds on the previous.

| Plan | Priority | Effort | Summary |
|------|----------|--------|---------|
| [001](./001-enable-embeddings.md) | CRITICAL | Medium | Enable embeddings — currently disabled, all connections are keyword-only |
| [002](./002-implement-stubs.md) | CRITICAL | Large | Implement 15 stub repository methods that return "not implemented" |
| [003](./003-fix-data-integrity.md) | CRITICAL | Medium | Fix race conditions, non-atomic transactions, silent failures |
| [004](./004-fix-domain-services.md) | HIGH | Medium | Fix thought chain cycles, impact classification, BM25 tokenizer, Leiden loop |
| [005](./005-fix-event-system.md) | HIGH | Medium | Fix event outbox (table scan → GSI), add outbox processor, dead letters |
| [006](./006-add-tests.md) | MEDIUM | Large | Add missing test coverage (repos, services, concurrency, integration) |
| [007](./007-performance.md) | MEDIUM | Med-Large | Fix N+1 queries, add pagination, optimize embedding loading |

## Phase B: Build the Bridge

New features that turn B2 from "a notes app with a graph" into "a second brain
you can talk to." Depends on Phase A fixes being done first.

| Plan | Priority | Effort | Summary |
|------|----------|--------|---------|
| [008](./008-mcp-server.md) | CRITICAL | Medium | MCP stdio server — 10 tools (recall, remember, neighbors, etc.) |
| [009](./009-claude-code-integration.md) | CRITICAL | Medium | Skill file, CLAUDE.md rules, PreToolUse hook, install CLI |
| [010](./010-brain-report.md) | HIGH | Small-Med | Generate BRAIN_REPORT.md — one-page knowledge summary for agents |
| [011](./011-smarter-connections.md) | HIGH | Med-Large | Edge re-evaluation, bridges, temporal connections, explanations |
| [012](./012-export-multi-medium.md) | MEDIUM | Medium | Obsidian export, JSON import/export, markdown import, API keys |

## Dependency Graph

```
Phase A (Fix Foundation):
  001 (Embeddings) ──┐
  002 (Stubs)     ───┼──→ 003 (Data Integrity) ──→ 004 (Domain Bugs)
                     │                                    │
                     │    005 (Events) ──────────────────→│
                     │                                    │
                     │    006 (Tests) ←── can run in parallel
                     │    007 (Performance) ←── can run in parallel
                     │
Phase B (New Features):
                     ├──→ 008 (MCP Server) ──→ 009 (Claude Code) ──→ 010 (Brain Report)
                     │                                                       │
                     └──→ 011 (Smarter Connections)                         │
                                                                            │
                          012 (Export) ←────────────────────────────────────┘
```

## Recommended Build Order

### Sprint 1: Core fixes
1. **001** — Enable embeddings (unblocks meaningful connections)
2. **002** — Implement stubs (unblocks MCP tools and brain report)
3. **003** — Fix data integrity (unblocks reliable operation)

### Sprint 2: Domain quality
4. **004** — Fix domain service bugs
5. **005** — Fix event system

### Sprint 3: Agent interface
6. **008** — Build MCP server
7. **009** — Claude Code integration
8. **010** — Brain report

### Sprint 4: Intelligence & polish
9. **011** — Smarter connections
10. **006** — Add tests
11. **007** — Performance

### Sprint 5: Ecosystem
12. **012** — Export, import, multi-medium

## Architecture After All Plans

```
┌─────────────────────────────────────────────────────┐
│                    B2 Second Brain                    │
├─────────────────────────────────────────────────────┤
│                                                      │
│  ┌──────────┐  ┌──────────┐  ┌────────────────────┐ │
│  │ Web UI   │  │ MCP      │  │ CLI (b2)           │ │
│  │ React 19 │  │ Server   │  │ install/export/    │ │
│  │ Sigma.js │  │ (stdio)  │  │ report/import      │ │
│  └────┬─────┘  └────┬─────┘  └────────┬───────────┘ │
│       │              │                 │              │
│       ▼              ▼                 ▼              │
│  ┌──────────────────────────────────────────────┐    │
│  │           B2 REST API (AWS Lambda)            │    │
│  │   Nodes / Edges / Graphs / Search / Analysis  │    │
│  └──────────────────────┬───────────────────────┘    │
│                         │                            │
│  ┌──────────────────────▼───────────────────────┐    │
│  │         Domain Layer (Go, DDD/CQRS)           │    │
│  │  Similarity · Edge Discovery · Leiden · BM25  │    │
│  │  Thought Chains · Impact · Surprises          │    │
│  └──────────────────────┬───────────────────────┘    │
│                         │                            │
│  ┌──────────────────────▼───────────────────────┐    │
│  │         Infrastructure (AWS)                   │    │
│  │  DynamoDB · EventBridge · Lambda · CloudWatch │    │
│  └──────────────────────────────────────────────┘    │
│                                                      │
├─────────────────────────────────────────────────────┤
│  Integration Layer                                   │
│  ┌──────────┐  ┌──────────┐  ┌──────────────────┐  │
│  │ Claude   │  │ Obsidian │  │ Brain Report     │  │
│  │ Code     │  │ Vault    │  │ (~/.b2/          │  │
│  │ (skill,  │  │ Export   │  │  BRAIN_REPORT.md)│  │
│  │  hooks)  │  │          │  │                  │  │
│  └──────────┘  └──────────┘  └──────────────────┘  │
└─────────────────────────────────────────────────────┘
```

## Key Decisions Captured

1. **No local storage** — B2's cloud API is the single source of truth.
   MCP server calls the API. No SQLite/BoltDB needed.

2. **Personal knowledge first** — B2 is not porting Graphify's code
   extraction (AST/tree-sitter). It's focused on memories, thoughts, ideas.

3. **Embeddings via OpenAI-compatible API** — the embed-node Lambda uses
   `text-embedding-3-small` by default. Can be swapped for any compatible endpoint.

4. **Go for everything** — MCP server, CLI, and backend are all Go.
   No Python dependency. Keeps the stack uniform.

5. **Eventual consistency for edges** — Node creation + edge discovery
   is a two-pass process (keyword-only sync, then hybrid async after embedding).
   This is acceptable for a personal knowledge graph.
