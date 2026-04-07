# 009 — Claude Code Integration

## Priority: CRITICAL
## Effort: Medium
## Depends on: 008 (MCP server)

## Goal

Make B2 seamlessly integrate with Claude Code so the agent is always aware
of your second brain. Three integration layers:
1. **Skill** — `/b2` slash command for on-demand interactions
2. **CLAUDE.md** — standing instructions for proactive behavior
3. **PreToolUse hook** — nudge before every search to check the brain first

Plus a CLI (`b2`) for install/uninstall/status management.

## Layer 1: Skill File

**File to create:** `skill.md` (in B2 repo root, copied to `~/.claude/skills/b2/SKILL.md` on install)

The skill file is what Claude Code reads when the user types `/b2`.
It should contain:

```markdown
# B2 — Second Brain

You have access to the user's personal knowledge graph (B2).
B2 stores memories, thoughts, and ideas as nodes in a graph.
Nodes are auto-connected by semantic similarity and can be organized
into communities (clusters of related ideas).

## Available MCP Tools

### Reading
- `recall` — Search memories by meaning. USE THIS FIRST for any question
  about the user's knowledge, notes, or past ideas.
- `get_memory` — Get full details of a specific memory by ID or title.
- `neighbors` — See what's connected to a memory (1-3 hops).
- `communities` — See the user's knowledge clusters.
- `god_nodes` — See the user's most central/connected ideas.
- `thought_chain` — Trace a path of connected ideas from a starting point.
- `recent` — See what the user has been thinking about recently.
- `graph_overview` — Summary stats about the knowledge graph.

### Writing
- `remember` — Save a new memory. Use when the user shares something
  they'd want to recall later. Ask before saving unless they explicitly
  say "remember this."
- `connect` — Link two existing memories. Use when the user points out
  a relationship between ideas.

## When to Use B2

- User asks about their notes, ideas, thoughts → `recall` first
- User says "remember this" or "save this" → `remember`
- User asks "what do I know about X" → `recall`
- User asks "how does X relate to Y" → `recall` both, then `thought_chain`
- User asks about their interests/focus areas → `communities` or `god_nodes`
- User asks "what have I been working on" → `recent`
- Starting a new session → `graph_overview` for context

## Behavior Guidelines

- Be proactive: if you find relevant memories during `recall`, surface them
- Don't save trivial things (greetings, small talk) — only meaningful knowledge
- When saving, suggest tags based on existing communities
- When a memory has few connections, mention it might be worth connecting
- Respect the user's graph — don't bulk-create or bulk-delete without asking
```

### Skill Registration

In `~/.claude/CLAUDE.md`, add:
```markdown
- **b2** (`~/.claude/skills/b2/SKILL.md`) — personal knowledge graph.
  Trigger: `/b2`
When the user types `/b2`, invoke the Skill tool with `skill: "b2"`
before doing anything else.
```

## Layer 2: CLAUDE.md Project Rules

Written to the local `./CLAUDE.md` (or any project the user runs `b2 claude install` in):

```markdown
## b2 — Second Brain

You have access to the user's personal knowledge graph via B2 MCP tools.

Rules:
- When the user asks about their notes, ideas, or past thoughts — use
  `recall` first before answering from general knowledge
- When the user shares something they want to remember — use `remember`
- When exploring connections between ideas — use `thought_chain` and `neighbors`
- Read ~/.b2/BRAIN_REPORT.md at the start of each session for context
  on the user's knowledge structure
- You are an extension of the user's memory. Be proactive about surfacing
  relevant context from their knowledge graph.
```

## Layer 3: PreToolUse Hook

Written to `.claude/settings.json`:

```json
{
  "hooks": {
    "PreToolUse": [
      {
        "matcher": "Glob|Grep",
        "hooks": [
          {
            "type": "command",
            "command": "echo 'b2: You have access to the user second brain via MCP tools. Consider using recall to check if relevant memories exist before searching files.'"
          }
        ]
      }
    ]
  }
}
```

This fires before every Glob/Grep, reminding Claude to check the brain.

## CLI Commands

**New binary:** `backend/cmd/b2cli/main.go`

```
b2 install                  # Copy skill.md to ~/.claude/skills/b2/SKILL.md
                            # Add registration to ~/.claude/CLAUDE.md
b2 uninstall                # Remove skill file and registration

b2 claude install            # Write CLAUDE.md section + PreToolUse hook
                            # (in current project directory)
b2 claude uninstall          # Remove CLAUDE.md section + hook

b2 status                   # Check: MCP binary installed? Config exists?
                            # API reachable? Auth valid? Graph stats?

b2 auth setup               # Interactive: prompt for API URL + token
                            # Writes ~/.b2/config.json
b2 auth refresh             # Refresh expired JWT token

b2 report                   # Generate BRAIN_REPORT.md (see Plan 010)
```

### Install/Uninstall Idempotency

All install commands must be idempotent:
- Use marker comments: `<!-- b2-start -->` ... `<!-- b2-end -->`
- Repeated installs update existing section, don't duplicate
- Uninstall removes only the B2 section, preserving other content

### CLAUDE.md Section Management
```go
const sectionStart = "<!-- b2-start -->"
const sectionEnd = "<!-- b2-end -->"

func installClaudeMD(projectDir string) error {
    path := filepath.Join(projectDir, "CLAUDE.md")
    content := readOrCreate(path)
    if strings.Contains(content, sectionStart) {
        // Replace existing section
        content = replaceBetween(content, sectionStart, sectionEnd, b2Section)
    } else {
        // Append new section
        content += "\n" + sectionStart + "\n" + b2Section + "\n" + sectionEnd + "\n"
    }
    return os.WriteFile(path, []byte(content), 0644)
}
```

### settings.json Hook Management
```go
func installHook(projectDir string) error {
    settingsPath := filepath.Join(projectDir, ".claude", "settings.json")
    settings := readOrCreateJSON(settingsPath)
    
    // Add PreToolUse hook if not present
    hooks := settings["hooks"].(map[string]interface{})
    preToolUse := hooks["PreToolUse"].([]interface{})
    
    // Check if b2 hook already exists
    for _, h := range preToolUse {
        if strings.Contains(h["command"], "b2:") {
            return nil // Already installed
        }
    }
    
    preToolUse = append(preToolUse, b2Hook)
    // ... save
}
```

## New Files

```
backend/cmd/b2cli/
├── main.go          # CLI entrypoint (cobra or raw flag parsing)
├── install.go       # Skill registration (copy file, update CLAUDE.md)
├── claude.go        # CLAUDE.md section + settings.json hook management
├── auth.go          # Config setup, token management
├── status.go        # Health check (API reachable, auth valid, stats)
└── report.go        # Brain report generation (delegates to Plan 010)

skill.md             # Skill definition file (in repo root)
```

## Dependencies

- `os`, `path/filepath`, `encoding/json` — stdlib for file management
- `net/http` — for status/health checks
- No external CLI framework needed (simple flag parsing is sufficient)

## Testing

- Unit test: install → verify files created with correct content
- Unit test: install twice → idempotent (no duplication)
- Unit test: uninstall → verify files cleaned up
- Integration test: full install → start Claude Code → verify `/b2` works
