# 000 — Vision: B2 as Your Second Brain

## What B2 Is

B2 is a personal knowledge graph — a "second brain" that grows with you.
You add notes, memories, thoughts, and ideas through a web UI (or eventually
through any connected agent). The system automatically discovers connections
between your thoughts using semantic similarity, keyword analysis, and
community detection.

## The Goal

When you connect an agent (like Claude Code) to B2, that agent becomes an
extension of you. It has access to everything you've ever put into your
knowledge graph. It can:

- Recall what you know about any topic
- Trace chains of thought across your ideas
- Identify your core themes and knowledge clusters
- Notice connections you haven't made yet
- Remember new things on your behalf
- Grow and evolve as you do

It's like taking notes in Obsidian, except your notes can talk back.
You're not just storing information — you're building a version of yourself
that any AI agent can tap into.

## How It Should Work

### Adding Knowledge
- Web UI: Go to the app, create memories with title + body + tags
- Agent: Tell Claude Code "remember this" and it saves to your graph
- Eventually: Ingest documents, URLs, PDFs, code (inspired by Graphify)

### Auto-Connections
When you add a memory, B2 should:
1. Generate a semantic embedding (vector representation of meaning)
2. Compare against ALL existing memories using hybrid similarity
   (60% semantic meaning + 40% keyword overlap)
3. Discover edges above a similarity threshold
4. Classify edge types (strong, normal, weak, reference)
5. Detect which Leiden community the memory belongs to
6. Identify if it bridges multiple communities (cross-pollination)

### Querying Your Brain
Any connected agent can:
- Search semantically ("What do I know about distributed systems?")
- Traverse the graph (neighbors, shortest path, thought chains)
- See the big picture (communities, god nodes, graph stats)
- Understand impact (what happens if I change/remove this idea?)

### The "Second Me" Quality
The agent should feel like it KNOWS you because:
- It has your full knowledge graph as context
- It reads a Brain Report summarizing your knowledge structure
- It can trace how your ideas connect and evolve
- It's proactive — surfacing relevant memories before you ask
- It grows — every interaction can add to the graph

## What B2 Already Has (Backend)
- Go backend with DDD/CQRS architecture on AWS
- DynamoDB single-table design with GSIs
- Node/Edge/Graph entities with rich domain logic
- Hybrid similarity calculator (keyword + semantic)
- Edge discovery pipeline (sync + async)
- Leiden community detection
- BM25 + semantic hybrid search with RRF fusion
- Thought chain tracing (DFS with community crossing)
- Impact analysis (blast radius with tier classification)
- WebSocket real-time updates
- EventBridge async event processing
- React 19 + Sigma.js frontend (needs work but functional)

## What B2 Needs (This Improvement Plan)

### Phase A: Fix the Foundation (Plans 001-007)
The backend has gaps — embeddings disabled, stub methods, race conditions,
missing tests. Fix these first so the core is reliable.

### Phase B: Build the Bridge (Plans 008-012)
Add the agent interface layer — MCP server, Claude Code integration,
brain report, smarter connections. This is what makes B2 a "second brain
you can talk to" instead of just "a notes app with a graph."

## Non-Goals (For Now)
- Code ingestion / AST extraction (Graphify does this; B2 focuses on personal knowledge first)
- Multi-user collaboration (single-user second brain)
- Mobile app (web UI + agent interface is sufficient)
- Local-first / offline mode (cloud API is the source of truth)
