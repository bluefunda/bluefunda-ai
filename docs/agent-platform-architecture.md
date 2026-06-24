# bai Agent Platform: Deep Comparative Architecture Study

> Principal architect analysis · May 2026  
> Objective: Evolve `bai` into a world-class agentic coding platform  
> Approach: Selective adoption of proven patterns, preservation of existing infrastructure

---

## Table of Contents

1. [Executive Summary](#1-executive-summary)
2. [Claude Code Architecture Reverse-Engineering](#2-claude-code-architecture-reverse-engineering)
3. [Existing bai Architecture Inventory](#3-existing-bai-architecture-inventory)
4. [Comparative Feature Matrix](#4-comparative-feature-matrix)
5. [Gap Analysis](#5-gap-analysis)
6. [bai Agent Runtime Architecture](#6-bai-agent-runtime-architecture)
7. [Tool Abstraction Design](#7-tool-abstraction-design)
8. [Agent Loop Design](#8-agent-loop-design)
9. [Context & Memory Architecture](#9-context--memory-architecture)
10. [Permission & Security Model](#10-permission--security-model)
11. [Enterprise Integration Design](#11-enterprise-integration-design)
12. [Package Structure](#12-package-structure)
13. [Key Interfaces & Abstractions](#13-key-interfaces--abstractions)
14. [Event-Driven Architecture](#14-event-driven-architecture)
15. [Testing Strategy](#15-testing-strategy)
16. [Token Efficiency & Performance](#16-token-efficiency--performance)
17. [Licensing Analysis](#17-licensing-analysis)
18. [Phased Implementation Roadmap](#18-phased-implementation-roadmap)
19. [Concrete Implementation Examples](#19-concrete-implementation-examples)
20. [Final Recommendations](#20-final-recommendations)

---

## 1. Executive Summary

`bai` has a production-grade foundation that Claude Code lacks: enterprise auth (Keycloak OAuth2), gRPC backend, multi-realm tenancy, billing integration, MCP server management, and a polished Bubble Tea TUI. The core gap is the **agent runtime** — the machinery that makes multi-step autonomous coding loops reliable, recoverable, and observable.

Claude Code's key architectural insight is treating the CLI as a **local execution substrate** with the LLM as the planner. All state-modifying operations happen locally; the backend is stateless. `bai` has already reached this design (Phase 1 shipped), but needs to deepen the execution engine significantly.

**What to port:** Agent loop discipline, tool abstraction patterns, context compaction strategy, permission gate design, CLAUDE.md project context injection, git integration primitives.

**What NOT to port:** TypeScript/Node.js runtime, React Ink TUI (Bubble Tea is superior for Go), direct Anthropic SDK coupling (bai's gRPC backend is a competitive advantage), Claude-specific prompt engineering.

**Critical differentiators to build natively:** Enterprise RBAC-aware tool permissions, Keycloak-integrated audit trail, Temporal workflow integration, multi-tenant session isolation, SAP/BTP tool integrations, remote execution fabric.

---

## 2. Claude Code Architecture Reverse-Engineering

### 2.1 High-Level Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                        Claude Code CLI                           │
│                    (TypeScript / Node.js)                        │
│                                                                  │
│  ┌─────────────┐  ┌──────────────┐  ┌──────────────────────┐  │
│  │  React/Ink  │  │  Agent Loop  │  │   Session Manager    │  │
│  │    TUI      │  │  (REPL core) │  │  ~/.claude/projects/ │  │
│  └─────────────┘  └──────────────┘  └──────────────────────┘  │
│         │                │                      │               │
│  ┌──────▼──────────────────▼────────────────────▼────────────┐ │
│  │                    Core Runtime                             │ │
│  │  ┌──────────┐ ┌─────────────┐ ┌──────────┐ ┌──────────┐ │ │
│  │  │  Tool    │ │  Permission │ │ Context  │ │  Prompt  │ │ │
│  │  │ Registry │ │    Gate     │ │ Manager  │ │ Composer │ │ │
│  │  └──────────┘ └─────────────┘ └──────────┘ └──────────┘ │ │
│  └─────────────────────────────────────────────────────────────┘ │
│                              │                                   │
│  ┌───────────────────────────▼───────────────────────────────┐  │
│  │              @anthropic-ai/sdk (HTTP/SSE)                  │  │
│  └───────────────────────────────────────────────────────────┘  │
│                              │                                   │
└──────────────────────────────┼───────────────────────────────────┘
                               ▼
                  api.anthropic.com (Claude API)
```

### 2.2 Agent Loop Architecture

Claude Code implements a **ReAct loop** (Reason + Act) with refinements:

```
┌─────────────────────────────────────────────────────┐
│                    Agent Loop                        │
│                                                      │
│  1. THINK    LLM receives context + tools schema     │
│              LLM produces: text | tool_use           │
│                                 │                    │
│  2. ACT      For each tool_use:                      │
│              ├─ Check permission gate                │
│              ├─ Show pending UI                      │
│              ├─ Execute tool                         │
│              └─ Collect result                       │
│                                 │                    │
│  3. OBSERVE  Append tool_result to context           │
│              Continue if more tool_use               │
│              Stop if pure text response              │
│                                 │                    │
│  4. REPAIR   On tool error:                         │
│              ├─ Append error as tool_result          │
│              └─ LLM self-corrects on next turn       │
│                                 │                    │
│  5. REFLECT  Check stop conditions:                  │
│              ├─ No more tool calls                   │
│              ├─ Max turns reached                    │
│              ├─ Token budget exhausted               │
│              └─ User interrupt (Ctrl+C)              │
└─────────────────────────────────────────────────────┘
```

**Key implementation detail:** Claude Code does NOT run a backend loop. The Anthropic API streams back content with embedded `tool_use` blocks. The CLI processes each block, executes the tool, and makes a new API call with the accumulated `tool_result` list. This is the same design `bai` has already implemented. The difference is in the depth and robustness of execution machinery.

### 2.3 Tool System

Claude Code defines ~15 built-in tools, each as a class with:

```typescript
interface Tool {
  name: string                              // Unique identifier
  description: string                       // LLM-facing description
  inputSchema: JSONSchema                   // Parameter validation
  userFacing: boolean                       // Show in /tools list?
  isReadOnly(): boolean                     // Permission classification
  shouldConfirmUse(input): boolean          // Per-call approval check
  execute(input, context): ToolResult       // Implementation
  renderResult(result): React.ReactNode     // TUI rendering
}
```

**Built-in tools (confirmed via npm package inspection):**

| Tool | Category | Read-only | Notes |
|------|----------|-----------|-------|
| `Bash` | Shell | No | Timeout 120s, persistent shell session |
| `Read` | Filesystem | Yes | With line range support |
| `Edit` | Filesystem | No | Exact string replacement with uniqueness check |
| `Write` | Filesystem | No | Full file write; requires prior Read in session |
| `Glob` | Search | Yes | Pattern matching, gitignore-aware |
| `Grep` | Search | Yes | Regex/literal search with context lines |
| `LS` | Filesystem | Yes | Directory listing |
| `TodoCreate` | Task | No | Creates structured todo list |
| `TodoRead` | Task | Yes | Reads current todo list |
| `TodoUpdate` | Task | No | Updates todo items |
| `WebFetch` | Network | Yes | Fetch URL content |
| `WebSearch` | Network | Yes | Search the web |
| `Agent` | Meta | No | Spawn subagent in new context window |
| `NotebookEdit` | Jupyter | No | Edit notebook cells |
| `NotebookRead` | Jupyter | Yes | Read notebook |

**Critical design pattern — Edit vs Write:**
- `Write` writes the entire file; requires Read first in same session
- `Edit` does targeted string replacement with uniqueness enforcement
- This prevents accidental overwrites and forces the LLM to understand existing content

### 2.4 Permission System

Claude Code's permission system has four layers:

```
Layer 1: Tool classification (built-in)
  isReadOnly() → auto-allow read-only tools
  !isReadOnly() → may need confirmation

Layer 2: Project settings (.claude/settings.json)
  {
    "allowedTools": ["Bash", "Edit"],
    "deniedTools": [],
    "permissions": {
      "allow": ["Bash(git *)"],     // Pattern-match allow
      "deny": ["Bash(rm -rf *)"]    // Pattern-match deny
    }
  }

Layer 3: Per-call approval (shouldConfirmUse)
  Tool-specific logic: Bash checks for destructive patterns
  Edit checks if file was recently Read

Layer 4: Global user settings (~/.claude/settings.json)
  Per-user overrides, applies across all projects
```

**Approval flow:**
1. Check deny list → reject immediately
2. Check allow list → execute immediately
3. Check tool classification → ask for unclassified
4. Check per-call logic → ask when shouldConfirmUse returns true
5. Show confirmation prompt → user approves/denies/always-allow

### 2.5 Context Management

**Problem:** Claude's context window is finite. Long agentic sessions exhaust it.

**Claude Code's solution — /compact:**
```
/compact [instructions]
  │
  ├─ Take full conversation history
  ├─ Send to Claude with summarization prompt:
  │   "Summarize this conversation, preserving:
  │    - All files read and their content
  │    - All changes made
  │    - Current task state
  │    - Pending decisions"
  ├─ Replace history with summary + recent turns
  └─ Resume from compact state
```

**Automatic compaction:** When context approaches limit, Claude Code auto-triggers compaction with a system note to the user.

**Token budget management:**
- Tracks input/output tokens per turn
- Shows running cost in footer
- Warns at 80% context window
- Hard-stops at limit (with graceful degradation)

### 2.6 Session Persistence

```
~/.claude/
  projects/
    <base64-encoded-cwd>/
      <session-id>.jsonl    ← Newline-delimited JSON (one event per line)
  settings.json             ← Global user settings
  CLAUDE.md                 ← Global project context
  
Working directory:
  .claude/
    settings.json           ← Project settings
    settings.local.json     ← Local overrides (gitignored)
  CLAUDE.md                 ← Project context injected into every session
```

**Session file format (JSONL):**
```jsonl
{"type":"user","message":"Fix the authentication bug","timestamp":"..."}
{"type":"assistant","message":"I'll start by reading the auth module...","tool_use":[...]}
{"type":"tool_result","tool_use_id":"...","content":"..."}
{"type":"system","content":"Context compacted at turn 45"}
```

### 2.7 Hooks Architecture

Claude Code has a hooks system that executes shell commands at lifecycle events:

```json
// .claude/settings.json
{
  "hooks": {
    "PreToolUse": [
      {
        "matcher": "Bash",
        "hooks": [{
          "type": "command",
          "command": "echo 'About to run bash command'"
        }]
      }
    ],
    "PostToolUse": [...],
    "Stop": [...],
    "SubagentStop": [...]
  }
}
```

**Hook lifecycle events:**
- `PreToolUse` — Before any tool execution; can block execution
- `PostToolUse` — After tool execution; receives result
- `Stop` — Agent loop completed
- `SubagentStop` — Spawned subagent completed

### 2.8 CLAUDE.md Project Context

CLAUDE.md files provide persistent project-specific context injected into every session:

```markdown
# Project: MyApp

## Architecture
- Backend: Go gRPC at internal/server/
- Frontend: React at web/src/
- Tests: Run with `make test`

## Conventions
- Always run `make fmt` before committing
- Use functional options pattern for constructors

## Current Focus
- Implementing the payment integration
```

**Loading hierarchy:**
1. `~/.claude/CLAUDE.md` (global)
2. Project root `CLAUDE.md`
3. `.claude/CLAUDE.md`
4. Any `CLAUDE.md` in visited subdirectories

All are concatenated and injected as system context.

### 2.9 Subagent Architecture

The `Agent` tool spawns a subagent in a fresh context window:

```typescript
// Parent agent invokes:
Agent({
  description: "Analyze the auth module",
  prompt: "Read all files in internal/auth/ and produce a security analysis",
  subagent_type: "general-purpose"  // or specialized
})

// Implementation:
// 1. Create new context window (separate API call sequence)
// 2. Inject parent context via prompt parameter
// 3. Run full agent loop in subagent
// 4. Return subagent's final response to parent
// 5. Parent continues with subagent result
```

**Isolation modes:**
- `isolation: "worktree"` — Creates a git worktree for isolated file operations
- Default — Shares the working directory

### 2.10 Streaming Architecture

Claude Code uses SSE streaming from the Anthropic API:

```
API streams:
  content_block_start (type: text)
  content_block_delta (text delta: "I'll start by...")
  content_block_delta (text delta: " reading the file")
  content_block_stop
  content_block_start (type: tool_use, id: "tu_123", name: "Read")
  content_block_delta (input delta: {...})
  content_block_stop
  message_stop (stop_reason: "tool_use")
```

CLI processes this in a streaming loop, rendering text in real-time and accumulating tool_use blocks for execution.

### 2.11 MCP Integration

```
~/.claude/settings.json:
{
  "mcpServers": {
    "github": {
      "type": "stdio",
      "command": "npx",
      "args": ["@modelcontextprotocol/server-github"],
      "env": {"GITHUB_TOKEN": "${GITHUB_TOKEN}"}
    },
    "database": {
      "type": "sse",
      "url": "http://localhost:3000/mcp"
    }
  }
}
```

MCP tools appear alongside built-in tools in the tool registry. The permission system applies equally to MCP tools.

### 2.12 Git Integration

Claude Code reads git state to provide context:
- Current branch, uncommitted changes shown in status
- Git blame for context on code history
- Commit creation helper (via Bash tool + prompt engineering)
- Worktree creation for isolation

No native git library — uses Bash tool with git commands.

### 2.13 Autonomous Loop (/loop)

```
/loop 5m "check for failing tests and fix them"
  │
  ├─ Spawns background agent
  ├─ Agent runs on 5-minute interval
  ├─ Each iteration: full agent loop with the prompt
  ├─ Results pushed to TUI as notifications
  └─ User can interrupt with Ctrl+C
```

Background agents run in headless mode (no TUI); output stored in session file.

---

## 3. Existing bai Architecture Inventory

### 3.1 Current Capabilities (Production-Ready)

```
┌─────────────────────────────────────────────────────────────────┐
│                     bai CLI (Go 1.25)                           │
│                                                                  │
│  ┌──────────────────────────────────────────────────────────┐  │
│  │                   Bubble Tea TUI                          │  │
│  │  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐   │  │
│  │  │ Viewport │ │Textarea  │ │ Markdown │ │Chroma SH │   │  │
│  │  │ Scroll   │ │ Input    │ │ Glamour  │ │ Highlt   │   │  │
│  │  └──────────┘ └──────────┘ └──────────┘ └──────────┘   │  │
│  └──────────────────────────────────────────────────────────┘  │
│                              │                                   │
│  ┌───────────────────────────▼──────────────────────────────┐  │
│  │              Agentic Loop (code.go)                       │  │
│  │  • 20-iteration max • Tool approval gate                  │  │
│  │  • History management • Think-block filtering             │  │
│  └───────────────────────────────────────────────────────────┘  │
│                              │                                   │
│  ┌───────────────────────────▼──────────────────────────────┐  │
│  │              Tool System (tools/)                         │  │
│  │  read_file  write_file  list_dir  search_files  bash      │  │
│  └───────────────────────────────────────────────────────────┘  │
│                              │                                   │
│  ┌───────────────────────────▼──────────────────────────────┐  │
│  │              gRPC Transport (grpc/conn.go)                │  │
│  │  • TLS auto-detect • JWT metadata • Auth interceptors     │  │
│  │  • Auto-retry on Unauthenticated • Token refresh          │  │
│  └───────────────────────────────────────────────────────────┘  │
│                              │                                   │
│  ┌───────────────────────────▼──────────────────────────────┐  │
│  │             Backend Stack (BFF + LLM Router)              │  │
│  │  cli.bluefunda.com:443                                    │  │
│  │  • Chat streaming • MCP management • User/billing         │  │
│  │  • Model routing • Rate limiting • Session storage        │  │
│  └───────────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────────┘
```

### 3.2 Strengths Over Claude Code

| Capability | bai | Claude Code |
|-----------|-----|-------------|
| Enterprise auth | Keycloak OAuth2 + multi-realm | None (API key) |
| Multi-tenant | Full realm isolation | Single user |
| Backend | gRPC with streaming | Direct Anthropic API |
| Model routing | Backend abstraction | Anthropic-only |
| Billing | Stripe integration | N/A (usage-based) |
| MCP management | List/add/subscribe API | Config file only |
| Output formats | table/json/quiet | Interactive only |
| CI/scripting | --auto-apply, headless | Limited |
| Session ops | List/resume/stop/title via API | Local JSONL only |
| Rate limiting | Per-user quotas via API | API rate limits only |
| Diagnostics | bai doctor (6-point) | None |

### 3.3 Current Tool Inventory

| Tool | Approval | Status | Missing vs Claude Code |
|------|----------|--------|----------------------|
| `read_file` | No | ✓ | Line ranges, binary detection |
| `write_file` | Yes | ✓ | Edit (patch) mode is missing |
| `list_dir` | No | ✓ | Recursive mode, gitignore-aware |
| `search_files` | No | ✓ | Glob only; no grep/regex |
| `bash` | Yes | ✓ | Persistent shell session |
| `edit_file` | Yes | ✗ MISSING | Targeted patch; prevents full rewrites |
| `grep` | No | ✗ MISSING | Regex/ripgrep integration |
| `todo_*` | No | ✗ MISSING | Structured task tracking |
| `web_fetch` | No | ✗ MISSING | URL content retrieval |
| `git_*` | No | ✗ MISSING | Native git operations |
| `agent` | No | ✗ MISSING | Subagent spawning |

---

## 4. Comparative Feature Matrix

| Feature | Claude Code Implementation | Existing bai | Gap | Complexity | Recommended Approach | Priority | Reuse Potential |
|---------|--------------------------|--------------|-----|------------|---------------------|----------|-----------------|
| **Agent loop** | ReAct loop, TypeScript, max turns configurable | Basic 20-iteration loop | Depth: no reflection, no repair strategies | Medium | Extend existing loop with structured phases | P0 | High — extend code.go |
| **Edit tool (patch)** | Exact string replacement, uniqueness check, requires prior Read | write_file only (full overwrite) | Critical: no targeted edits | Low | Add edit_file tool with old/new string | P0 | None — implement fresh |
| **Grep tool** | Regex search with context lines, ripgrep integration | search_files (glob only) | Regex search missing | Low | Add grep tool using ripgrep or stdlib | P0 | Partial — extend tools/ |
| **Todo system** | TodoCreate/Read/Update, XML persistence | None | Structured task tracking absent | Low | JSON-backed todo in ~/.bai/todos/ | P1 | None — implement fresh |
| **Context compaction** | /compact command, LLM-summarized history | None | Context will exhaust on long sessions | High | /compact with backend summarization endpoint | P0 | None — design required |
| **CLAUDE.md equivalent** | Hierarchical project context injection | None | Project context must be re-stated every session | Medium | BAI.md loading with same hierarchy | P0 | None — implement fresh |
| **Permission system** | 4-layer: classification, settings.json, per-call, global | Binary: NeedsApproval() bool | No pattern matching, no project-level config | Medium | Extend with settings.json + pattern matching | P0 | Partial — extend NeedsApproval |
| **Session persistence** | JSONL per-session in ~/.claude/projects/ | Server-side via BFF API | Client has no local session cache | Medium | Hybrid: local JSONL cache + BFF sync | P1 | Partial — use BFF API + add local |
| **Hooks system** | PreToolUse/PostToolUse shell commands | None | No extensibility for tool lifecycle | Medium | Hook runner in settings.json | P1 | None — implement fresh |
| **Subagent spawning** | Agent tool with isolation modes | None | No parallel task delegation | High | Add Agent tool + context isolation | P2 | None — design required |
| **Cost tracking** | Per-turn token count, running cost | Rate limit API (server-side) | No client-visible token/cost display | Medium | Add token tracking to stream pump | P1 | Partial — add to stream events |
| **Web fetch** | WebFetch tool with URL sanitization | None | Cannot retrieve web content | Low | Add web_fetch tool with go-http | P1 | None — implement fresh |
| **Git integration** | Via Bash tool + prompt | gitRepoName() only | No native git operations | Low | Add git tool package | P1 | Partial — extend helpers.go |
| **Background agents** | /loop with interval, headless mode | None | No autonomous background execution | High | Headless mode + OS-level persistence | P2 | None — design required |
| **Worktree isolation** | git worktree per subagent | None | No isolation for parallel ops | High | Git worktree + temp dir isolation | P2 | None — implement fresh |
| **MCP tool integration** | All MCP tools appear in tool registry | MCP server management (API) | MCP tools don't flow into local loop | High | Bridge MCP into local tool registry | P1 | High — extend mcp.go |
| **Slash commands** | /help /clear /compact /model /memory etc. | /help /clear /new /model /sessions etc. | /compact, /memory missing | Low | Add missing commands | P0 | High — extend slashcmd.go |
| **Multiline input** | Shift+Enter, vim-mode optional | Shift+Enter supported | vim-mode missing | Low | Optional vim-mode in textarea | P2 | Partial |
| **Global CLAUDE.md** | ~/.claude/CLAUDE.md | None | No global user context file | Low | ~/.bai/BAI.md | P1 | None — trivial |
| **Streaming tokens** | Real-time character streaming | Streaming (chunk events) | ✓ Already implemented | None | N/A | — | High |
| **Think block filtering** | Via ExportedThinkFilter | ✓ Already implemented | N/A | None | N/A | — | High |
| **OAuth2 auth** | API key only | ✓ Keycloak device flow | bai is superior | None | N/A | — | N/A |
| **Multi-tenant** | None | ✓ Multi-realm Keycloak | bai is superior | None | N/A | — | N/A |
| **Output modes** | Interactive TUI only | ✓ table/json/quiet | bai is superior | None | N/A | — | N/A |
| **Diagnostics** | None | ✓ bai doctor | bai is superior | None | N/A | — | N/A |
| **Model routing** | Anthropic models only | ✓ Backend-abstracted | bai is superior | None | N/A | — | N/A |

---

## 5. Gap Analysis

### 5.1 Critical Gaps (P0 — blocks world-class agent)

**1. Edit/Patch Tool**
Current `write_file` does full overwrites. On a 2000-line file, the LLM must reproduce the entire content — expensive, error-prone, and slow. Claude Code's `Edit` tool does targeted string replacement: find exact match, replace, verify uniqueness. This single tool reduces token cost by 10x for common editing tasks.

**2. Context Compaction**  
The agentic loop leaks context on every turn. After ~30 iterations on a large codebase, the context window is exhausted. Without compaction, sessions fail mid-task. This is a session-reliability blocker.

**3. BAI.md Project Context**
Without persistent project context, every session starts blind. The LLM must rediscover architecture, conventions, and current focus every time. CLAUDE.md is among Claude Code's most impactful features for experienced users.

**4. Permission System Depth**
Binary `NeedsApproval()` is insufficient. Users need to say "always allow `git status`" and "never allow `rm -rf`". Pattern-based rules + settings.json project config unlock this.

**5. Grep Tool**
`search_files` only does glob pattern matching. Real coding work requires regex search across file contents. Without grep, the LLM blind-navigates codebases — making many unnecessary read_file calls.

### 5.2 High-Impact Gaps (P1 — significantly improves UX)

**6. Token Cost Visibility**
Users have no idea how many tokens a session consumes. The rate limit API provides quota info but not real-time cost. Showing "~2,400 tokens used this turn" in the footer builds trust and helps users optimize.

**7. Todo/Planning System**
Structured task tracking enables the LLM to maintain multi-step plans reliably. Without it, the LLM loses track of completed/pending steps in long tasks.

**8. Web Fetch Tool**
Reading documentation, checking error messages online, fetching API specs — essential for real-world coding tasks.

**9. MCP Tool Bridge**
MCP servers are managed but their tools don't flow into the local agentic loop. Users with GitHub, database, or Slack MCP servers can't use those capabilities in code mode.

**10. Git Tool Package**
Common git operations (status, diff, log, commit, branch) should be first-class tools with structured output, not raw bash commands with string parsing.

### 5.3 Architecture Gaps (P2 — enables advanced use cases)

**11. Subagent Spawning**
Delegating subtasks to specialized agents enables parallelism and prevents context exhaustion. A "research agent" can explore the codebase while the "coding agent" focuses on the fix.

**12. Background Agent Loop**
Autonomous background agents for CI/monitoring/PR review require headless mode, OS-level process management, and result delivery (notifications, file writes).

**13. Worktree Isolation**
Safe parallel agent execution requires filesystem isolation. Git worktrees provide this without full VM overhead.

---

## 6. bai Agent Runtime Architecture

### 6.1 Target Architecture

```
┌────────────────────────────────────────────────────────────────────┐
│                         bai CLI                                     │
│                                                                      │
│  ┌──────────────────────────────────────────────────────────────┐  │
│  │                      TUI Layer                                │  │
│  │  Bubble Tea + Glamour + Chroma (existing, extend only)        │  │
│  └──────────────────────────────────────────────────────────────┘  │
│                              │                                       │
│  ┌───────────────────────────▼──────────────────────────────────┐  │
│  │                   Agent Runtime                               │  │
│  │                                                               │  │
│  │  ┌─────────────┐  ┌──────────────┐  ┌───────────────────┐   │  │
│  │  │   Planner   │  │   Executor   │  │  Context Manager  │   │  │
│  │  │  (BAI.md +  │  │  (loop +     │  │  (compact +       │   │  │
│  │  │   todo)     │  │   recovery)  │  │   budget)         │   │  │
│  │  └─────────────┘  └──────────────┘  └───────────────────┘   │  │
│  │                              │                                │   │
│  │  ┌───────────────────────────▼──────────────────────────┐   │  │
│  │  │                   Tool Router                         │   │  │
│  │  │                                                       │   │  │
│  │  │  ┌────────────┐ ┌───────────┐ ┌───────────────────┐ │   │  │
│  │  │  │ Permission │ │  Local    │ │    MCP Bridge     │ │   │  │
│  │  │  │   Gate     │ │  Tools    │ │  (local+remote)   │ │   │  │
│  │  │  └────────────┘ └───────────┘ └───────────────────┘ │   │  │
│  │  │                                                       │   │  │
│  │  │  ┌────────────┐ ┌───────────┐ ┌───────────────────┐ │   │  │
│  │  │  │ Filesystem │ │   Shell   │ │    Git Runtime    │ │   │  │
│  │  │  │  Runtime   │ │  Runtime  │ │                   │ │   │  │
│  │  │  └────────────┘ └───────────┘ └───────────────────┘ │   │  │
│  │  └───────────────────────────────────────────────────────┘   │  │
│  │                                                               │  │
│  │  ┌──────────────┐  ┌──────────────┐  ┌──────────────────┐   │  │
│  │  │  Session     │  │  Event Bus   │  │   Telemetry      │   │  │
│  │  │  Manager     │  │  (in-proc)   │  │   (hooks)        │   │  │
│  │  └──────────────┘  └──────────────┘  └──────────────────┘   │  │
│  └──────────────────────────────────────────────────────────────┘  │
│                              │                                       │
│  ┌───────────────────────────▼──────────────────────────────────┐  │
│  │                  gRPC Transport (existing)                     │  │
│  └───────────────────────────────────────────────────────────────┘  │
│                              │                                       │
│                   cli.bluefunda.com:443                              │
└────────────────────────────────────────────────────────────────────┘
```

### 6.2 Agent Runtime Modules

**Planner** (`internal/agent/planner/`)
- Load BAI.md hierarchy and inject into system prompt
- Maintain structured todo list (create/read/update)
- Provide task decomposition primitives
- Track completion percentage for long tasks

**Executor** (`internal/agent/executor/`)
- Run ReAct loop with configurable max iterations
- Structured phases: think → act → observe → repair → reflect
- Recovery strategies: retry on transient errors, self-correction prompts
- User interrupt handling (Ctrl+C with graceful stop + checkpoint)
- Checkpoint creation before destructive operations

**Context Manager** (`internal/agent/context/`)
- Token counting per turn (use tiktoken-compatible Go library)
- Budget tracking: warn at 70%, compact at 85%
- /compact implementation: send to backend summarization endpoint
- Context window display in TUI footer

**Tool Router** (`internal/agent/tools/`)
- Unified registry: local tools + MCP tools
- Permission gate with pattern matching
- Hook invocation (PreToolUse/PostToolUse)
- Result caching for expensive read-only tools

**Session Manager** (`internal/agent/session/`)
- Local JSONL event log (compatible with BFF API)
- Resume from local log + BFF sync
- Checkpoint snapshots before destructive operations
- Undo support via checkpoint rollback

**Event Bus** (`internal/agent/events/`)
- In-process pub/sub for agent lifecycle events
- Telemetry hooks for audit logging
- Replay capability from event log

---

## 7. Tool Abstraction Design

### 7.1 Core Tool Interface

```go
// Tool is the fundamental unit of agent capability.
type Tool interface {
    // Schema returns the JSON schema for LLM consumption.
    Schema() ToolSchema
    
    // IsReadOnly classifies this tool for permission purposes.
    // Read-only tools are auto-approved unless explicitly denied.
    IsReadOnly() bool
    
    // ShouldConfirm returns true if this specific invocation needs approval.
    // Called after permission gate; allows per-call logic beyond IsReadOnly.
    ShouldConfirm(input json.RawMessage) bool
    
    // Execute runs the tool with the given input.
    // Must be context-aware for cancellation and timeout.
    Execute(ctx context.Context, input json.RawMessage) (ToolResult, error)
    
    // RenderResult formats the result for TUI display.
    RenderResult(result ToolResult) string
}

type ToolSchema struct {
    Type     string          `json:"type"`
    Function ToolFunctionDef `json:"function"`
}

type ToolFunctionDef struct {
    Name        string          `json:"name"`
    Description string          `json:"description"`
    Parameters  json.RawMessage `json:"parameters"`
}

type ToolResult struct {
    Content    string        // Text output
    Error      string        // Error message (LLM sees this)
    DurationMs int64         // Execution time
    Metadata   map[string]any // Additional structured data
}
```

### 7.2 Tool Registry

```go
type Registry struct {
    tools    map[string]Tool
    mu       sync.RWMutex
}

func (r *Registry) Register(t Tool) {
    r.mu.Lock()
    defer r.mu.Unlock()
    r.tools[t.Schema().Function.Name] = t
}

func (r *Registry) Dispatch(ctx context.Context, call ToolCall, gate PermissionGate) (ToolResult, error) {
    tool, ok := r.tools[call.Name]
    if !ok {
        return ToolResult{}, fmt.Errorf("unknown tool: %s", call.Name)
    }
    
    decision, err := gate.Check(call)
    if err != nil {
        return ToolResult{}, err
    }
    if decision == Deny {
        return ToolResult{Error: "tool execution denied by policy"}, nil
    }
    if decision == RequireApproval {
        if !gate.RequestApproval(call) {
            return ToolResult{Content: "User declined."}, nil
        }
    }
    
    return tool.Execute(ctx, json.RawMessage(call.Arguments))
}

// Schemas returns all tool schemas for LLM context.
func (r *Registry) Schemas() []ToolSchema {
    r.mu.RLock()
    defer r.mu.RUnlock()
    var schemas []ToolSchema
    for _, t := range r.tools {
        schemas = append(schemas, t.Schema())
    }
    return schemas
}
```

### 7.3 Built-in Tools (Extended Set)

**ReadFile** — Extended
```go
type ReadFileTool struct{}

// Parameters:
// - path: string (required)
// - start_line: int (optional, 1-indexed)
// - end_line: int (optional)
// Returns file contents with line numbers (cat -n format)
// Binary files: return hex dump header + error
// Large files: auto-truncate at 2000 lines with notice
```

**EditFile** — New (critical)
```go
type EditFileTool struct {
    readCache map[string]string // path → content hash
}

// Parameters:
// - path: string (required)
// - old_string: string (required) — must be unique in file
// - new_string: string (required)
// - replace_all: bool (optional, default false)
// 
// Validation:
// - old_string must appear exactly once (unless replace_all)
// - If not found: error "old_string not found in file"
// - If ambiguous: error "old_string matches N occurrences; be more specific"
// 
// Side effect: updates readCache entry for path
```

**WriteFile** — Extended
```go
type WriteFileTool struct {
    readCache map[string]string
}

// Parameters:
// - path: string (required)
// - content: string (required)
// 
// Requires: file must have been read in current session
//           (enforced via readCache to prevent blind writes)
// Creates parent directories automatically
```

**Bash** — Extended
```go
type BashTool struct {
    shell   *persistentShell // Maintains shell session state
    workDir string
}

// Parameters:
// - command: string (required)
// - timeout: int (optional, default 120, max 600)
// - description: string (optional, shown in TUI before approval)
// 
// Features:
// - Persistent shell session (cd, export persist across calls)
// - Combined stdout+stderr
// - Non-zero exit included in output (not as error)
// - Timeout enforced via context
// - Background process detection and warning
```

**Grep** — New
```go
type GrepTool struct{}

// Parameters:
// - pattern: string (required) — regex or literal
// - path: string (required) — file or directory
// - include: string (optional) — glob to filter files, e.g. "*.go"
// - case_sensitive: bool (optional, default true)
// - context_lines: int (optional, default 2)
// - max_results: int (optional, default 100)
// 
// Implementation: exec ripgrep if available, fall back to stdlib
// Output: file:line:content format
```

**TodoWrite** — New
```go
type TodoWriteTool struct {
    store *todoStore
}

// Parameters:
// - todos: []TodoItem (required)
//   - id: string
//   - content: string
//   - status: "pending" | "in_progress" | "completed"
//   - priority: "high" | "medium" | "low"
// 
// Replaces entire todo list (idempotent, LLM-safe)
// Persisted to ~/.bai/todos/<session-id>.json
```

**GitStatus** — New
```go
type GitTool struct{}

// Sub-tools: git_status, git_diff, git_log, git_commit, git_branch
// All read-only by default except git_commit
// Output is structured JSON for reliable LLM parsing
```

**WebFetch** — New
```go
type WebFetchTool struct {
    client *http.Client
    // URL allowlist for enterprise compliance
    allowlist []string
}

// Parameters:
// - url: string (required)
// - selector: string (optional) — CSS selector to extract
// - format: "text" | "markdown" | "json" (default "text")
// 
// Security: URL allowlist configurable in settings.json
// Rate limit: 10 req/session
// Max size: 512KB response
```

---

## 8. Agent Loop Design

### 8.1 Structured Loop Phases

```go
type AgentLoop struct {
    registry    *tools.Registry
    contextMgr  *context.Manager
    sessionMgr  *session.Manager
    planner     *planner.Planner
    eventBus    *events.Bus
    maxIter     int
    cfg         *AgentConfig
}

// Phase represents a named stage in the agent loop.
type Phase string

const (
    PhaseThink   Phase = "think"    // LLM produces response
    PhaseAct     Phase = "act"      // Tools dispatched
    PhaseObserve Phase = "observe"  // Results collected
    PhaseRepair  Phase = "repair"   // Error recovery
    PhaseReflect Phase = "reflect"  // Check stop conditions
)
```

### 8.2 Loop Execution Flow

```
func (l *AgentLoop) Run(ctx context.Context, history []Message, ch chan<- StreamEvent) ([]Message, error) {

  for iteration := 0; iteration < l.maxIter; iteration++ {
    
    // ── PHASE: THINK ─────────────────────────────────────────────
    // Check context budget before each LLM call
    if l.contextMgr.NeedsCompaction(history) {
      history, err = l.contextMgr.Compact(ctx, history)
      ch <- StreamEvent{Kind: "compacted", Summary: "Context compacted"}
    }
    
    // Inject project context (BAI.md)
    systemPrompt := l.planner.BuildSystemPrompt()
    
    // Send to backend
    stream, err := l.backend.Chat(ctx, BuildRequest(history, l.registry.Schemas(), systemPrompt))
    
    // Stream response to TUI
    response, toolCalls, err := l.pumpStream(stream, ch)
    
    // Checkpoint before destructive operations
    if hasDestructiveTools(toolCalls) {
      l.sessionMgr.Checkpoint(history)
    }
    
    history = append(history, Message{Role: "assistant", Content: response, ToolCalls: toolCalls})
    l.sessionMgr.Append(TurnEvent{Phase: PhaseThink, Message: history[len(history)-1]})
    
    // ── PHASE: REFLECT ────────────────────────────────────────────
    if len(toolCalls) == 0 {
      return history, nil  // Pure text response = done
    }
    
    // ── PHASE: ACT ────────────────────────────────────────────────
    for _, tc := range toolCalls {
      l.eventBus.Publish(events.PreToolUse{Call: tc})
      
      result, err := l.registry.Dispatch(ctx, tc, l.permGate)
      
      l.eventBus.Publish(events.PostToolUse{Call: tc, Result: result, Err: err})
      
      // ── PHASE: OBSERVE ──────────────────────────────────────────
      // Error → tool_result with error (LLM self-corrects)
      // Success → tool_result with content
      toolMsg := Message{
        Role:       "tool",
        Content:    formatToolResult(result, err),
        ToolCallID: tc.ID,
      }
      history = append(history, toolMsg)
      l.sessionMgr.Append(TurnEvent{Phase: PhaseObserve, Message: toolMsg})
      
      // Forward result summary to TUI
      ch <- StreamEvent{
        Kind:       "tool_exec",
        ToolName:   tc.Name,
        Status:     resultStatus(err),
        DurationMs: result.DurationMs,
        Summary:    truncate(result.Content, 80),
      }
    }
    
    // ── PHASE: REPAIR ────────────────────────────────────────────
    // Handled implicitly: tool errors become tool_result content
    // LLM sees the error and self-corrects on next iteration
  }
  
  ch <- StreamEvent{Kind: "chunk", Chunk: "\n⚠ Maximum iterations reached.\n"}
  return history, nil
}
```

### 8.3 Checkpoint and Undo

```go
type Checkpoint struct {
    ID        string
    Timestamp time.Time
    History   []Message
    Files     map[string]string  // path → content snapshot
}

func (s *SessionManager) Checkpoint(history []Message) error {
    // Snapshot current state of all files touched in this session
    touched := extractTouchedFiles(history)
    files := make(map[string]string, len(touched))
    for _, path := range touched {
        content, _ := os.ReadFile(path)
        files[path] = string(content)
    }
    
    cp := Checkpoint{
        ID:        uuid.New().String(),
        Timestamp: time.Now(),
        History:   copyHistory(history),
        Files:     files,
    }
    return s.writeCheckpoint(cp)
}

func (s *SessionManager) Rollback(checkpointID string) error {
    cp, err := s.loadCheckpoint(checkpointID)
    if err != nil {
        return err
    }
    // Restore file snapshots
    for path, content := range cp.Files {
        os.WriteFile(path, []byte(content), 0644)
    }
    return nil
}
```

### 8.4 User Interrupt Handling

```go
// Context propagation for graceful cancellation
func (l *AgentLoop) runWithInterrupt(ctx context.Context, ...) {
    // Ctrl+C sends SIGINT → context cancelled
    // We catch it mid-stream in pumpStream:
    
    select {
    case <-ctx.Done():
        // Stream cancelled mid-response
        // Create checkpoint before stopping
        l.sessionMgr.Checkpoint(history)
        ch <- StreamEvent{Kind: "interrupted", Msg: "Stopped. Session saved."}
        return history, context.Canceled
    default:
        // Continue normal processing
    }
}
```

### 8.5 Sequence Diagram: Full Coding Task

```
User        TUI           AgentLoop      ToolRouter      Backend
  │           │               │               │              │
  │ "Fix auth bug" │           │               │              │
  ├──────────►│               │               │              │
  │           │ submitFn()    │               │              │
  │           ├──────────────►│               │              │
  │           │               │ BuildRequest()│              │
  │           │               ├───────────────────────────►  │
  │           │               │               │  stream      │
  │           │               │◄───────────────────────────  │
  │           │ chunk events  │               │              │
  │           │◄──────────────│               │              │
  │           │               │  tool_call: read_file         │
  │           │               ├──────────────►│              │
  │           │ tool_call ev  │               │ ReadFile()   │
  │           │◄──────────────┤               ├─────────────►│
  │           │               │               │◄────────────┤
  │           │               │◄──────────────┤              │
  │           │               │ append tool_result            │
  │           │               │               │              │
  │           │               │  tool_call: edit_file (WRITE)│
  │           │               ├──────────────►│              │
  │           │ approval ev   │               │ PermGate.Check()
  │           │◄──────────────┤               ├─────────────►│
  │ y/n       │               │               │              │
  ├──────────►│               │               │              │
  │           │ reply: true   │               │              │
  │           ├──────────────►│               │              │
  │           │               │               │ EditFile()   │
  │           │               │◄──────────────┤              │
  │           │ tool_exec ev  │               │              │
  │           │◄──────────────┤               │              │
  │           │               │ next iteration → Backend     │
  │           │               ├───────────────────────────►  │
  │           │               │  final text response         │
  │           │               │◄───────────────────────────  │
  │           │ chunk events  │               │              │
  │           │◄──────────────┤               │              │
  │           │  done event   │               │              │
  │           │◄──────────────┤               │              │
```

---

## 9. Context & Memory Architecture

### 9.1 Token Budget Management

```go
type ContextManager struct {
    tokenizer    Tokenizer       // tiktoken-compatible Go impl
    maxTokens    int             // from model metadata
    warnAt       float64         // 0.70 = 70%
    compactAt    float64         // 0.85 = 85%
    backend      BackendClient   // for summarization RPC
}

type TokenStats struct {
    Used       int
    Max        int
    Percentage float64
    Messages   int
}

func (m *ContextManager) Stats(history []Message) TokenStats {
    used := 0
    for _, msg := range history {
        used += m.tokenizer.Count(msg.Content)
        for _, tc := range msg.ToolCalls {
            used += m.tokenizer.Count(tc.Arguments)
        }
    }
    return TokenStats{
        Used:       used,
        Max:        m.maxTokens,
        Percentage: float64(used) / float64(m.maxTokens),
        Messages:   len(history),
    }
}

func (m *ContextManager) NeedsCompaction(history []Message) bool {
    return m.Stats(history).Percentage >= m.compactAt
}
```

### 9.2 Context Compaction

```go
func (m *ContextManager) Compact(ctx context.Context, history []Message) ([]Message, error) {
    // Send history to backend summarization endpoint
    // Backend calls LLM with summarization prompt
    summary, err := m.backend.SummarizeHistory(ctx, &pb.SummarizeRequest{
        History:      marshalHistory(history),
        Preserve:     "all file changes, current task state, pending decisions",
        MaxSummaryLen: 2000,
    })
    if err != nil {
        return history, fmt.Errorf("compact: %w", err)
    }
    
    // Replace history with summary + last N turns
    keepLast := 5
    compacted := []Message{
        {
            Role: "system",
            Content: fmt.Sprintf("[Context compacted at turn %d]\n\n%s", len(history), summary.Text),
        },
    }
    if len(history) > keepLast {
        compacted = append(compacted, history[len(history)-keepLast:]...)
    }
    return compacted, nil
}
```

### 9.3 BAI.md Hierarchy

```go
type ProjectContext struct {
    files []contextFile
}

type contextFile struct {
    path    string
    content string
    priority int  // Higher = later in context (overrides earlier)
}

func LoadProjectContext(workDir string) (*ProjectContext, error) {
    // Load in priority order (later overrides earlier)
    candidates := []string{
        filepath.Join(os.Getenv("HOME"), ".bai", "BAI.md"),  // global
        filepath.Join(workDir, "BAI.md"),                     // project root
        filepath.Join(workDir, ".bai", "BAI.md"),             // project .bai/
        filepath.Join(workDir, "CLAUDE.md"),                  // Claude Code compat
    }
    
    pc := &ProjectContext{}
    for i, path := range candidates {
        content, err := os.ReadFile(path)
        if err != nil {
            continue  // Missing files are fine
        }
        pc.files = append(pc.files, contextFile{
            path:     path,
            content:  string(content),
            priority: i,
        })
    }
    return pc, nil
}

func (pc *ProjectContext) SystemPrompt() string {
    if len(pc.files) == 0 {
        return ""
    }
    var b strings.Builder
    b.WriteString("# Project Context\n\n")
    for _, f := range pc.files {
        fmt.Fprintf(&b, "## From %s\n\n%s\n\n", f.path, f.content)
    }
    return b.String()
}
```

### 9.4 Long-Term Memory (Future)

For v2, implement semantic memory:

```go
type MemoryStore interface {
    // Store a fact or observation
    Remember(ctx context.Context, key string, content string, tags []string) error
    
    // Retrieve relevant memories for a prompt
    Recall(ctx context.Context, query string, limit int) ([]Memory, error)
    
    // Forget specific memory
    Forget(ctx context.Context, key string) error
}

// Implementation options:
// - SQLite with FTS5 for local semantic search
// - Embedded vector store (e.g., sqlite-vec) for embedding-based retrieval
// - BFF API endpoint for server-side memory (cross-device sync)
```

---

## 10. Permission & Security Model

### 10.1 Permission Architecture

```
Layer 1: Tool Classification (immutable)
  Built into each Tool implementation
  IsReadOnly() → auto-allow unless explicitly denied
  
Layer 2: Global User Settings (~/.bai/settings.json)
  Personal preferences across all projects
  
Layer 3: Project Settings (.bai/settings.json)
  Project-specific rules
  Committed to git for team sharing
  
Layer 4: Local Overrides (.bai/settings.local.json)
  Machine-specific overrides (gitignored)
  
Layer 5: Per-call Logic (ShouldConfirm)
  Tool-specific approval for dangerous patterns
```

### 10.2 Settings Schema

```json
// .bai/settings.json
{
  "version": "1",
  "permissions": {
    "allow": [
      "Bash(git status)",
      "Bash(git diff *)",
      "Bash(go test *)",
      "Bash(make *)",
      "ReadFile",
      "ListDir",
      "SearchFiles",
      "Grep"
    ],
    "deny": [
      "Bash(rm -rf *)",
      "Bash(sudo *)",
      "Bash(curl * | *)",
      "WebFetch(*.internal.*)"
    ]
  },
  "tools": {
    "bash": {
      "timeout": 120,
      "persistentShell": true
    },
    "web_fetch": {
      "allowedDomains": ["docs.go.dev", "pkg.go.dev", "stackoverflow.com"]
    }
  },
  "hooks": {
    "PreToolUse": [
      {
        "matcher": "Bash",
        "hooks": [{"type": "command", "command": "echo 'Running: $BAI_TOOL_INPUT'"}]
      }
    ],
    "PostToolUse": [],
    "Stop": [{"type": "command", "command": "notify-send 'bai task completed'"}]
  },
  "context": {
    "baimd": true,
    "claudemd": true,
    "autoLoad": true
  }
}
```

### 10.3 Permission Gate Implementation

```go
type PermissionGate struct {
    globalSettings  *Settings
    projectSettings *Settings
    localSettings   *Settings
    approvalFn      func(call ToolCall) bool  // TUI callback
}

type Decision int

const (
    Allow          Decision = iota
    Deny
    RequireApproval
)

func (g *PermissionGate) Check(call ToolCall) (Decision, error) {
    // 1. Check explicit deny (global > project > local)
    if g.matchesDeny(call) {
        return Deny, nil
    }
    
    // 2. Read-only tools are auto-allowed unless denied
    if call.Tool.IsReadOnly() {
        return Allow, nil
    }
    
    // 3. Check explicit allow list
    if g.matchesAllow(call) {
        return Allow, nil
    }
    
    // 4. Per-call logic
    if call.Tool.ShouldConfirm(call.Input) {
        return RequireApproval, nil
    }
    
    // 5. Default: require approval for write operations
    return RequireApproval, nil
}

func (g *PermissionGate) matchesAllow(call ToolCall) bool {
    for _, pattern := range g.allAllowPatterns() {
        if matchPattern(pattern, call) {
            return true
        }
    }
    return false
}

// Pattern syntax: "ToolName(shell glob for arguments)"
// Examples:
//   "Bash"             → all bash commands
//   "Bash(git *)"      → bash commands starting with "git"
//   "ReadFile"         → all read_file calls
//   "ReadFile(*.go)"   → only .go files
func matchPattern(pattern string, call ToolCall) bool {
    // Parse: "ToolName" or "ToolName(argPattern)"
    name, argPattern := parsePattern(pattern)
    if !strings.EqualFold(name, call.Name) {
        return false
    }
    if argPattern == "" {
        return true
    }
    // Match argPattern against primary argument
    primaryArg := extractPrimaryArg(call.Input)
    matched, _ := filepath.Match(argPattern, primaryArg)
    return matched
}
```

### 10.4 Enterprise Compliance Features

```go
// AuditLogger writes all tool invocations to a structured audit log.
type AuditLogger struct {
    writer io.Writer  // can be file, syslog, or OTLP exporter
}

type AuditEvent struct {
    Timestamp    time.Time
    SessionID    string
    UserID       string      // from JWT sub claim
    Realm        string      // Keycloak realm
    ToolName     string
    ToolInput    string      // sanitized (no secrets)
    Decision     string      // "allow" | "deny" | "user-approved" | "user-denied"
    Result       string      // "ok" | "error"
    DurationMs   int64
}

// Compliance hook: inject into event bus
eventBus.Subscribe(events.PostToolUse{}, func(e events.PostToolUse) {
    auditLogger.Log(AuditEvent{
        Timestamp:  time.Now(),
        SessionID:  sessionID,
        UserID:     cfg.Auth.UserID,
        Realm:      cfg.Realm,
        ToolName:   e.Call.Name,
        ToolInput:  sanitize(e.Call.Arguments),
        Decision:   e.Decision.String(),
        Result:     resultStatus(e.Err),
        DurationMs: e.Result.DurationMs,
    })
})
```

---

## 11. Enterprise Integration Design

### 11.1 Integration Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                    bai Enterprise Integrations                   │
│                                                                  │
│  ┌──────────────────────────────────────────────────────────┐  │
│  │                   Tool Registry                           │  │
│  │                                                           │  │
│  │  Built-in Tools    Enterprise Tools      MCP Bridge       │  │
│  │  ┌──────────┐     ┌──────────────────┐  ┌─────────────┐ │  │
│  │  │ bash     │     │ sap_btp_deploy   │  │ github MCP  │ │  │
│  │  │ read_file│     │ btp_service_bind │  │ jira MCP    │ │  │
│  │  │ edit_file│     │ cf_push          │  │ slack MCP   │ │  │
│  │  │ grep     │     │ temporal_trigger │  │ custom MCPs │ │  │
│  │  │ git_*    │     │ k8s_deploy       │  └─────────────┘ │  │
│  │  └──────────┘     └──────────────────┘                   │  │
│  └──────────────────────────────────────────────────────────┘  │
│                              │                                   │
│  ┌───────────────────────────▼──────────────────────────────┐  │
│  │                  Enterprise Backend                        │  │
│  │  ┌──────────────┐  ┌───────────────┐  ┌──────────────┐  │  │
│  │  │ Keycloak Auth│  │ Temporal WF   │  │ BTP Services │  │  │
│  │  │ (RBAC claims)│  │ (long tasks)  │  │ (CF, XSUAA)  │  │  │
│  │  └──────────────┘  └───────────────┘  └──────────────┘  │  │
│  └──────────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────────┘
```

### 11.2 Temporal Workflow Integration

```go
// WorkflowTool triggers a Temporal workflow and returns its result.
type WorkflowTool struct {
    client temporalclient.Client
    namespace string
}

// The LLM can trigger, query, and signal workflows:
// - trigger_workflow: start a new workflow execution
// - query_workflow: get workflow status/result
// - signal_workflow: send a signal to a running workflow
// - list_workflows: list recent executions

// Example usage by LLM:
// trigger_workflow({"workflow_type": "DeploymentWorkflow", 
//                   "args": {"service": "payment-svc", "env": "staging"}})
// → Returns: {"run_id": "abc123", "status": "started"}

// query_workflow({"run_id": "abc123"})
// → Returns: {"status": "completed", "result": {"deployed_version": "1.2.3"}}
```

### 11.3 RBAC-Aware Tool Permissions

```go
// EnterprisePermissionGate extends PermissionGate with RBAC claims from JWT
type EnterprisePermissionGate struct {
    PermissionGate
    claims JWTClaims
}

func (g *EnterprisePermissionGate) Check(call ToolCall) (Decision, error) {
    // First apply standard permission logic
    decision, err := g.PermissionGate.Check(call)
    if err != nil || decision == Deny {
        return decision, err
    }
    
    // Apply RBAC: check if user's realm roles allow this tool
    if !g.claims.HasRole(toolRequiredRole(call.Name)) {
        return Deny, fmt.Errorf("role '%s' required for tool '%s'", 
            toolRequiredRole(call.Name), call.Name)
    }
    
    return decision, nil
}

// Enterprise tool roles:
// - basic: read_file, grep, list_dir, bash (read-only)
// - developer: + write_file, edit_file, bash (all)
// - deployer: + sap_btp_deploy, cf_push, k8s_deploy
// - admin: + temporal_trigger, workflow_*
```

### 11.4 BTP/SAP Tool Examples

```go
// BTPDeployTool wraps CF push for SAP BTP Cloud Foundry
type BTPDeployTool struct {
    cfClient *cfclient.Client
    realm    string // from token — determines org/space
}

// Parameters:
// - app_name: string
// - manifest: string (path to manifest.yml)
// - space: string (optional, defaults from RBAC claims)
// - org: string (optional, defaults from RBAC claims)

// Auto-populates org/space from Keycloak claims → zero-config deployment
```

---

## 12. Package Structure

```
bai/
├── cmd/bai/main.go                    # entry point (unchanged)
│
├── internal/
│   ├── cmd/                           # CLI command handlers (existing, extend)
│   │   ├── root.go
│   │   ├── chat.go
│   │   ├── code.go                    # ← migrate to use agent/executor
│   │   ├── login.go
│   │   ├── mcp.go
│   │   ├── model.go
│   │   ├── config_cmd.go
│   │   ├── helpers.go
│   │   └── ...
│   │
│   ├── agent/                         # ← NEW: Agent runtime
│   │   ├── executor/
│   │   │   ├── executor.go            # AgentLoop: Run(), phases
│   │   │   ├── checkpoint.go          # Checkpoint/rollback logic
│   │   │   └── recovery.go            # Error recovery strategies
│   │   │
│   │   ├── planner/
│   │   │   ├── planner.go             # BuildSystemPrompt(), task decomp
│   │   │   ├── baimd.go              # BAI.md loading hierarchy
│   │   │   └── todo.go               # TodoStore: CRUD on todos
│   │   │
│   │   ├── context/
│   │   │   ├── manager.go             # TokenStats, NeedsCompaction
│   │   │   ├── compact.go             # Compact(): call backend summarizer
│   │   │   └── tokenizer.go           # Token counting (tiktoken-go or custom)
│   │   │
│   │   └── session/
│   │       ├── session.go             # SessionManager: Append, Checkpoint
│   │       ├── jsonl.go               # JSONL event log read/write
│   │       └── sync.go               # BFF sync (upload/download session)
│   │
│   ├── tools/                         # ← EXTEND: Tool system
│   │   ├── registry.go                # Registry: Register, Dispatch, Schemas
│   │   ├── permission.go              # PermissionGate, pattern matching
│   │   ├── settings.go                # Settings: load/merge hierarchy
│   │   ├── hooks.go                   # Hook runner (PreToolUse/PostToolUse)
│   │   │
│   │   ├── fs/
│   │   │   ├── read.go               # ReadFileTool (with line ranges)
│   │   │   ├── write.go              # WriteFileTool (requires prior read)
│   │   │   ├── edit.go               # EditFileTool (patch: old→new string)
│   │   │   ├── list.go               # ListDirTool (gitignore-aware)
│   │   │   └── search.go             # SearchFilesTool (glob)
│   │   │
│   │   ├── shell/
│   │   │   ├── bash.go               # BashTool (persistent shell, timeout)
│   │   │   └── shell.go              # persistentShell implementation
│   │   │
│   │   ├── search/
│   │   │   └── grep.go               # GrepTool (ripgrep/stdlib fallback)
│   │   │
│   │   ├── git/
│   │   │   ├── git.go                # GitTool router
│   │   │   ├── status.go             # git_status → structured JSON
│   │   │   ├── diff.go               # git_diff
│   │   │   ├── log.go                # git_log
│   │   │   └── commit.go             # git_commit (requires approval)
│   │   │
│   │   ├── web/
│   │   │   └── fetch.go              # WebFetchTool
│   │   │
│   │   ├── tasks/
│   │   │   └── todo.go               # TodoWriteTool, TodoReadTool
│   │   │
│   │   ├── mcp/
│   │   │   ├── bridge.go             # MCPBridge: list servers, invoke tools
│   │   │   └── client.go             # MCP protocol client
│   │   │
│   │   └── enterprise/               # Enterprise-specific tools
│   │       ├── btp.go                # BTP/CF deployment tools
│   │       ├── temporal.go           # Temporal workflow tools
│   │       └── k8s.go                # Kubernetes tools
│   │
│   ├── events/                        # ← NEW: Event bus
│   │   ├── bus.go                    # In-process pub/sub
│   │   ├── events.go                 # Event type definitions
│   │   └── audit.go                  # AuditLogger
│   │
│   ├── ui/                           # (existing, extend)
│   │   ├── output.go
│   │   ├── stream.go
│   │   └── tui/
│   │       ├── model.go              # ← Add token footer, /compact
│   │       ├── view.go               # ← Add token stats rendering
│   │       ├── slashcmd.go          # ← Add /compact, /memory, /undo
│   │       └── ...
│   │
│   ├── grpc/                         # (existing, extend)
│   │   └── conn.go                   # ← Add SummarizeHistory RPC support
│   │
│   ├── config/                       # (existing, extend)
│   │   └── config.go                 # ← Add agent config fields
│   │
│   └── auth/                         # (existing, unchanged)
│       └── auth.go
│
└── api/proto/bff/                    # (existing, extend proto)
    ├── bff.pb.go
    └── bff_grpc.pb.go
```

---

## 13. Key Interfaces & Abstractions

```go
// Tool is the fundamental execution primitive.
type Tool interface {
    Schema() ToolSchema
    IsReadOnly() bool
    ShouldConfirm(input json.RawMessage) bool
    Execute(ctx context.Context, input json.RawMessage) (ToolResult, error)
}

// AgentLoop orchestrates multi-turn execution.
type AgentLoop interface {
    Run(ctx context.Context, history []Message, events chan<- StreamEvent) ([]Message, error)
    Stop()                  // Graceful stop with checkpoint
    Checkpoint() error      // Manual checkpoint
    Rollback(id string) error // Roll back to checkpoint
}

// Planner provides context enrichment and task structure.
type Planner interface {
    BuildSystemPrompt() string
    GetTodos() []Todo
    SetTodos([]Todo) error
}

// ContextManager controls token usage and compaction.
type ContextManager interface {
    Stats(history []Message) TokenStats
    NeedsCompaction(history []Message) bool
    Compact(ctx context.Context, history []Message) ([]Message, error)
}

// PermissionGate controls tool execution access.
type PermissionGate interface {
    Check(call ToolCall) (Decision, error)
    RequestApproval(call ToolCall) bool  // blocking: shows TUI prompt
}

// SessionManager persists agent state.
type SessionManager interface {
    Append(event SessionEvent) error
    Checkpoint(history []Message) (checkpointID string, err error)
    Rollback(checkpointID string) error
    List() ([]SessionInfo, error)
    Resume(sessionID string) ([]Message, error)
}

// Backend abstracts the gRPC/BFF transport.
type Backend interface {
    Chat(ctx context.Context, req ChatRequest) (ChatStream, error)
    SummarizeHistory(ctx context.Context, history []Message) (string, error)
}

// EventBus provides in-process pub/sub for lifecycle events.
type EventBus interface {
    Publish(event any)
    Subscribe(eventType any, handler func(event any))
    Unsubscribe(eventType any, handler func(event any))
}

// ToolRegistry manages tool lifecycle.
type ToolRegistry interface {
    Register(tool Tool)
    RegisterMCP(server string, tools []MCPTool)
    Dispatch(ctx context.Context, call ToolCall, gate PermissionGate) (ToolResult, error)
    Schemas() []ToolSchema
    List() []ToolInfo
}
```

---

## 14. Event-Driven Architecture

### 14.1 Event Bus Design

```go
// Events are strongly typed; bus dispatches by type.
type Bus struct {
    handlers map[reflect.Type][]reflect.Value
    mu       sync.RWMutex
}

// Defined events:
type PreToolUse struct {
    SessionID string
    Iteration int
    Call      ToolCall
}

type PostToolUse struct {
    SessionID string
    Iteration int
    Call      ToolCall
    Result    ToolResult
    Err       error
    Decision  Decision
}

type AgentStarted struct {
    SessionID string
    UserID    string
    Prompt    string
}

type AgentStopped struct {
    SessionID  string
    Iterations int
    Reason     string  // "done" | "max-iter" | "user-interrupt" | "error"
}

type ContextCompacted struct {
    SessionID  string
    Before     int    // token count
    After      int
    Iterations int
}

type CheckpointCreated struct {
    SessionID    string
    CheckpointID string
    Reason       string
}
```

### 14.2 Telemetry Integration

```go
// TelemetryHook subscribes to all events and emits to OTLP/Prometheus
type TelemetryHook struct {
    tracer  trace.Tracer
    meter   metric.Meter
    logger  *slog.Logger
}

// Metrics:
// bai_agent_iterations_total{session_id, user_id, realm}
// bai_tool_calls_total{tool_name, status, session_id}
// bai_tool_duration_ms{tool_name}
// bai_context_tokens{session_id}
// bai_compactions_total{session_id}
// bai_checkpoints_total{session_id}

// Traces:
// bai.agent.loop (span per iteration)
//   ├─ bai.tool.execute (span per tool call)
//   └─ bai.context.compact (span on compaction)
```

### 14.3 Execution Replay

```go
// SessionLog enables deterministic replay for debugging and testing
type SessionLog struct {
    Events []SessionEvent `json:"events"`
}

type SessionEvent struct {
    Seq       int          `json:"seq"`
    Timestamp time.Time    `json:"ts"`
    Type      string       `json:"type"` // "user" | "assistant" | "tool_call" | "tool_result" | "checkpoint"
    Data      json.RawMessage `json:"data"`
}

// Replay: re-execute a session with a mock backend
func Replay(log SessionLog, mockBackend Backend) ([]Message, error) {
    // Recreate exact execution from event log
    // Useful for: debugging agent failures, golden tests, regression testing
}
```

---

## 15. Testing Strategy

### 15.1 Unit Tests

```go
// Each tool has deterministic unit tests
func TestEditFileTool_UniqueMatch(t *testing.T) {
    // Setup: temp file with known content
    // Execute: edit tool with exact match
    // Assert: file modified correctly
}

func TestEditFileTool_AmbiguousMatch(t *testing.T) {
    // Assert: error when old_string appears multiple times
}

func TestPermissionGate_PatternAllow(t *testing.T) {
    // "Bash(git *)" allows "git status" but not "rm -rf"
}
```

### 15.2 Integration Tests (existing pattern extended)

```go
// Extend existing bufconn pattern to test agent loop end-to-end
func TestAgentLoop_ReadAndEdit(t *testing.T) {
    server := startTestServer(t)  // existing bufconn pattern
    // Inject mock LLM responses: [tool_call: read_file] → [tool_call: edit_file] → [text: done]
    // Run agentLoop
    // Assert file was modified correctly
}
```

### 15.3 Golden Tests (Deterministic Replay)

```go
func TestGolden_AuthBugFix(t *testing.T) {
    // Load golden session log from testdata/
    log, _ := LoadSessionLog("testdata/auth_bug_fix.jsonl")
    
    // Replay with mock backend
    result, err := Replay(log, &mockBackend{})
    
    // Assert outcome matches golden
    assert.Equal(t, goldenOutcome, result)
}
```

### 15.4 Simulation Harness

```go
// MockBackend simulates LLM responses for scripted test scenarios
type MockBackend struct {
    Turns []MockTurn  // Pre-scripted responses
    idx   int
}

type MockTurn struct {
    Response  string       // Text content
    ToolCalls []ToolCall   // Tool calls to emit
}

// Enables fully deterministic agent loop testing without real LLM
```

### 15.5 Benchmark Framework

```go
// Measure agent loop performance
func BenchmarkAgentLoop_LargeRepo(b *testing.B) {
    // Setup: 1000-file repo
    // Scenario: "find all TODO comments and create issues"
    // Measure: iterations, tokens, wall time, tool execution time
}

// Eval framework: assess agent quality on standardized tasks
type EvalTask struct {
    Name        string
    Prompt      string
    SetupFn     func(dir string) error  // Create test repo state
    AssertFn    func(dir string) error  // Assert expected outcome
    MaxIter     int
    MaxTokens   int
}
```

---

## 16. Token Efficiency & Performance

### 16.1 Token Optimization Strategies

**1. Edit over Write**
`EditFileTool` sends only the diff (~50 tokens) vs `WriteFileTool` sending full file content (hundreds to thousands of tokens). For a typical 200-line file edit, this is a 20-40x token reduction.

**2. Targeted Read with Line Ranges**
`ReadFileTool` with `start_line`/`end_line` reads only the relevant portion. For a 1000-line file where the bug is on lines 45-60, read 10 lines instead of 1000.

**3. Grep Before Read**
Use `GrepTool` to locate relevant lines, then `ReadFileTool` with line ranges. Avoids reading entire files to find one pattern.

**4. Context Compaction**
Replace verbose tool results with concise summaries. A 200-line file content becomes "Read src/auth/token.go (200 lines): JWT validation with HMAC-256" in the compact summary.

**5. Tool Result Truncation**
Bash output is truncated at 10KB with a note. File content truncated at 2000 lines. This prevents single tool results from consuming the entire context.

**6. Streaming Token Counting**
Count tokens as they stream in; update the footer in real-time. Users see cost accumulate and can `/compact` proactively.

### 16.2 Repo Indexing (v2)

```
For large codebases (10k+ files), add lightweight indexing:

bai index                        # Build .bai/index.json
  ├─ File tree with sizes
  ├─ Symbol index (go/analysis for Go, tree-sitter for others)
  └─ Recent changes (git log --since 30days)

SearchSymbol tool:               # Uses index for fast lookup
  search_symbol("func HandleAuth")
  → Returns file + line without scanning all files
```

### 16.3 Cache Strategy

```go
// Read-only tool results can be cached within a session
type CachedRegistry struct {
    Registry
    cache map[string]ToolResult  // key: tool_name + json.Marshal(input)
    mu    sync.RWMutex
}

// Cache invalidation: on any write operation to that path
// TTL: session lifetime (no cross-session caching for file reads)
// Benefit: repeated read_file calls on same path use cached result
```

---

## 17. Licensing Analysis

### 17.1 Claude Code License Status

Claude Code is **proprietary software** with a commercial license. It is NOT open source. Key points:

- The npm package `@anthropic-ai/claude-code` is distributed as minified/obfuscated TypeScript
- Source code is NOT publicly available on GitHub (the repo contains only documentation)
- License: Anthropic's commercial terms of service
- There is NO Apache/MIT/GPL license on the implementation

### 17.2 What is Safe

| Category | Safe | Reasoning |
|----------|------|-----------|
| **Architectural concepts** | ✓ | Ideas and patterns cannot be copyrighted |
| **Interface designs** | ✓ | Type signatures are abstractions, not expression |
| **Agent loop pattern** | ✓ | ReAct is published academic research (Yao et al., 2022) |
| **Tool abstraction pattern** | ✓ | Tool use is in the Anthropic API spec (public) |
| **Permission system design** | ✓ | Security patterns are general engineering knowledge |
| **CLAUDE.md concept** | ✓ | The idea of a context file is not copyrightable |
| **Context compaction concept** | ✓ | Summarization strategy is general knowledge |
| **Hook system design** | ✓ | Pre/post hooks are a universal pattern |
| **Settings.json schema** | ✓ with care | Don't copy exact field names/structure verbatim |

### 17.3 What is Prohibited

| Category | Prohibited | Reasoning |
|----------|-----------|-----------|
| **Copying source code** | ✗ NEVER | Direct infringement; TypeScript is not public |
| **Deobfuscating/reverse-engineering binary** | ✗ NEVER | Violates ToS + potentially DMCA |
| **Copying exact prompt text** | ✗ NEVER | Specific prompts are creative expression |
| **Copying exact setting names** | ✓ caution | Close to potential "look and feel" risk; use own names |
| **"Compatible" format claims** | ✓ caution | Don't claim CLAUDE.md compatibility without testing |

### 17.4 Safe Reuse Patterns

1. **Use `BAI.md` not `CLAUDE.md`** — Your own name for the context file concept
2. **Reference academic papers** for algorithm attribution (ReAct, chain-of-thought, etc.)
3. **Use your own prompt text** — Never copy system prompt wording
4. **Design your own permission DSL** — The pattern is safe; exact syntax is yours to define
5. **Cite inspiration, not copying** — "Inspired by Claude Code's architectural approach" is fine in docs

### 17.5 Attribution Recommendations

In your documentation:
> "bai's agent runtime draws architectural inspiration from published research on ReAct agents, and from patterns observed in the agentic coding tool ecosystem. All implementation is original."

---

## 18. Phased Implementation Roadmap

### Phase 0: Foundation (Weeks 1–2) — "Fill the Critical Gaps"

**Objective:** Close the most impactful gaps with minimal scope.

```
Week 1:
  [ ] EditFileTool (old_string → new_string, uniqueness check)
  [ ] GrepTool (ripgrep/stdlib, regex + context lines)
  [ ] ReadFileTool enhancement (line ranges, truncation)
  [ ] BAI.md loading (hierarchy: global → project → .bai/)
  
Week 2:
  [ ] Permission settings.json (allow/deny patterns)
  [ ] Token counter in stream pump (update footer)
  [ ] /compact slash command (call backend summarizer)
  [ ] Add /undo, /checkpoint slash commands
```

**Deliverable:** `bai code` is significantly more capable. Edit operations don't require full rewrites. Project context persists across sessions. Users can see token cost.

**Effort:** ~2 engineer-weeks  
**Risk:** Low — all additive changes to existing patterns

### Phase 1: Agent Runtime v1 (Weeks 3–6) — "Production-Grade Loop"

**Objective:** Build the `internal/agent/` package with full loop discipline.

```
Week 3:
  [ ] agent/executor: structured phases (think/act/observe/repair/reflect)
  [ ] agent/executor: checkpoint/rollback
  [ ] agent/executor: user interrupt handling
  [ ] agent/session: JSONL event log

Week 4:
  [ ] agent/context: TokenManager, auto-compact
  [ ] agent/planner: TodoStore, BAI.md integration
  [ ] agent/events: EventBus with PreToolUse/PostToolUse
  [ ] Migrate code.go to use agent/executor

Week 5:
  [ ] tools/registry: unified Registry with Dispatch
  [ ] tools/hooks: Hook runner
  [ ] GitTool package (status, diff, log, commit)
  [ ] WebFetchTool

Week 6:
  [ ] TodoWriteTool, TodoReadTool
  [ ] AuditLogger
  [ ] Extended TUI: token footer, checkpoint indicator
  [ ] Slash commands: /compact, /undo, /checkpoint, /todos
```

**Deliverable:** `bai code` has structured loop with checkpoints, git tools, todo tracking, context compaction, and hooks. Ready for real-world coding tasks.

**Effort:** ~4 engineer-weeks  
**Risk:** Medium — new packages require careful interface design

### Phase 2: Enterprise Features (Weeks 7–10) — "Enterprise-Ready"

**Objective:** Enterprise RBAC, MCP bridge, audit trail, Temporal integration.

```
Week 7–8:
  [ ] EnterprisePermissionGate (RBAC claims from JWT)
  [ ] AuditLogger → OTLP/structured log export
  [ ] MCP Bridge (local loop + remote MCP tools)
  [ ] Session resume (local JSONL + BFF sync)

Week 9–10:
  [ ] Temporal workflow tools
  [ ] BTP/CF deployment tools
  [ ] k8s tools (if needed)
  [ ] RBAC-aware tool roles
  [ ] Compliance mode (no auto-execute, all approvals logged)
```

**Deliverable:** Enterprise teams can use bai with full audit trails, RBAC enforcement, Temporal workflow triggers, and BTP deployments.

**Effort:** ~4 engineer-weeks  
**Risk:** Medium-High — depends on Temporal/BTP client availability

### Phase 3: Multi-Agent (Weeks 11–16) — "Advanced Capabilities"

**Objective:** Subagent spawning, background loops, worktree isolation.

```
Week 11–12:
  [ ] AgentTool (spawn subagent in fresh context)
  [ ] Context isolation (separate history, shared filesystem)
  [ ] Result aggregation (parent receives subagent output)

Week 13–14:
  [ ] Worktree isolation (git worktree per subagent)
  [ ] Background agent mode (headless execution)
  [ ] /loop command with interval scheduling
  [ ] Notification delivery (desktop notification on completion)

Week 15–16:
  [ ] Multi-agent orchestration patterns
  [ ] Research agent + coding agent specializations
  [ ] Parallel tool execution within a turn
  [ ] Agent DAG execution (multiple subagents)
```

**Effort:** ~6 engineer-weeks  
**Risk:** High — multi-agent coordination is complex

### Phase 4: Intelligence Layer (Weeks 17–24) — "Long-Term Differentiators"

**Objective:** Repo indexing, semantic search, long-term memory.

```
[ ] Symbol indexing (go/analysis, tree-sitter)
[ ] SearchSymbol tool
[ ] SQLite-backed long-term memory store
[ ] Cross-session memory (bai remembers past decisions)
[ ] Embeddings-based context retrieval
[ ] Repo summarization for onboarding new sessions
[ ] IDE integration (LSP bridge for real-time context)
[ ] VSCode extension (webview over bai TUI)
```

**Effort:** ~8 engineer-weeks  
**Risk:** High — requires ML infrastructure and careful UX design

---

## 19. Concrete Implementation Examples

### 19.1 Autonomous Repair Loop

```go
// Scenario: bai code --auto "make all tests pass"
func autonomousRepairLoop(ctx context.Context, goal string) error {
    // Phase 1: Understand current state
    history := []Message{{Role: "user", Content: goal}}
    history = append(history, {Role: "user", Content: systemContext.String()})
    
    maxRepairCycles := 5
    for cycle := 0; cycle < maxRepairCycles; cycle++ {
        // Run agent: LLM plans → tools execute → results observed
        history, err = agentLoop.Run(ctx, history, ch)
        
        // Phase 2: Verify outcome
        testResult := bash("go test ./...")
        if testResult.ExitCode == 0 {
            return nil  // Goal achieved
        }
        
        // Phase 3: Feed failure back for self-correction
        history = append(history, Message{
            Role: "user",
            Content: fmt.Sprintf("Tests still failing:\n%s\nPlease fix.", testResult.Output),
        })
    }
    return fmt.Errorf("could not achieve goal in %d repair cycles", maxRepairCycles)
}
```

### 19.2 PR Generation Flow

```
User: "bai code --auto 'create a PR for the authentication refactor'"

Agent Loop:
  Turn 1: Read git status → see uncommitted changes
  Turn 2: Read changed files → understand scope
  Turn 3: Run git diff → see exact changes
  Turn 4: TodoCreate → [draft PR title, draft PR description, run tests, commit, push, create PR]
  Turn 5: Bash("go test ./...") → tests pass
  Turn 6: Bash("git add internal/auth/ ...") → stage changes
  Turn 7: Bash("git commit -m 'refactor: extract JWT validation'") → committed
  Turn 8: Bash("git push -u origin feat/auth-refactor") → pushed
  Turn 9: Bash("gh pr create --title '...' --body '...'") → PR created
  Turn 10: Return PR URL to user
```

### 19.3 Checkpoint Rollback Flow

```
User: "refactor the database connection pool"

Agent Loop Turn 1: Reads connection.go (200 lines)
Agent Loop Turn 2: EditFile → rewrites connection pool (complex change)

CHECKPOINT CREATED: cp_abc123 (snapshot of connection.go before edit)

Agent Loop Turn 3: Bash("go build ./...") → compile error
Agent Loop Turn 4: Tries to fix compile error → makes it worse
Agent Loop Turn 5: Another bash → still broken

User sees: "Task failing, type /undo to restore last checkpoint"

User: /undo cp_abc123
→ connection.go restored to pre-edit state
→ History rolled back to Turn 2
→ User can try a different approach
```

### 19.4 Multi-Step Coding Workflow

```
User: "bai code 'implement OAuth2 login for the REST API'"

Turn 1: List project structure
Turn 2: Read go.mod (check if golang.org/x/oauth2 is available)
Turn 3: WebFetch("https://pkg.go.dev/golang.org/x/oauth2") → understand API
Turn 4: Read existing auth middleware
Turn 5: TodoWrite([
  {id:1, "Add oauth2 dependency to go.mod", pending},
  {id:2, "Create internal/oauth/ package", pending},
  {id:3, "Implement OAuthHandler struct", pending},
  {id:4, "Add /auth/login and /auth/callback routes", pending},
  {id:5, "Write unit tests", pending},
  {id:6, "Update README", pending}
])
Turn 6: Bash("go get golang.org/x/oauth2")
Turn 7: TodoWrite([...id:1 → completed...])
Turn 8-15: Implement each todo item
Turn 16: Bash("go test ./internal/oauth/...")
Turn 17: TodoWrite([all completed])
Turn 18: "I've implemented OAuth2 login. Here's a summary: ..."
```

---

## 20. Final Recommendations

### 20.1 What to Port (Architectural Concepts)

| Concept | Why | How |
|---------|-----|-----|
| EditFileTool (patch semantics) | 20-40x token reduction for edits | Implement `tools/fs/edit.go` as P0 |
| CLAUDE.md → BAI.md hierarchy | Session continuity, team conventions | Implement `agent/planner/baimd.go` as P0 |
| Pattern-based permissions | `Bash(git *)` granularity | Implement `tools/permission.go` as P0 |
| Context compaction (/compact) | Session reliability on long tasks | Add backend RPC + `/compact` command as P0 |
| Tool lifecycle hooks | Extensibility, audit, enterprise | Implement `tools/hooks.go` as P1 |
| Structured loop phases | Reliable recovery, debuggability | Implement `agent/executor.go` as P1 |
| Checkpoint/undo | Reversibility for complex tasks | Implement `agent/session/` as P1 |
| TodoWrite/TodoRead | Task tracking in complex sessions | Implement `tools/tasks/` as P1 |
| Subagent spawning | Parallelism, context isolation | Implement `tools/agent.go` as P2 |
| Background /loop | Autonomous monitoring/CI | Implement headless mode as P2 |

### 20.2 What to Redesign (bai-native)

| Feature | Why Redesign | bai's Advantage |
|---------|-------------|-----------------|
| Permission system | Integrate with Keycloak RBAC claims | Tool access tied to realm roles |
| Session persistence | Hybrid local + BFF API (cross-device) | Claude Code is single-machine |
| MCP management | Already has subscription/listing API | Claude Code is config-file only |
| Audit logging | Enterprise compliance requirements | Structured log → OTLP/SIEM |
| Multi-tenant | Realm-per-org isolation | Claude Code has no concept of tenancy |
| Tool permissions | RBAC-aware (deployer vs developer roles) | Claude Code is single-user |

### 20.3 What to Ignore

| Feature | Why Ignore |
|---------|-----------|
| React/Ink TUI | Bubble Tea is superior for Go; already excellent |
| Direct Anthropic SDK | gRPC backend is a competitive moat; keep it |
| TypeScript tooling | Wrong language for Go ecosystem |
| npm packaging | Already have Go binary distribution |
| API key auth | Keycloak is vastly superior for enterprise |
| Single-user design | Multi-tenant is a bai differentiator |

### 20.4 What to Build Natively (bai Differentiators)

| Capability | Description | Timeline |
|-----------|-------------|----------|
| **Temporal integration** | Trigger/query/signal workflows from agent | Phase 2 |
| **BTP/CF tools** | SAP BTP Cloud Foundry deployments | Phase 2 |
| **RBAC tool permissions** | Role-based tool access from Keycloak claims | Phase 2 |
| **Enterprise audit trail** | OTLP-exportable tool execution log | Phase 2 |
| **Multi-tenant isolation** | Realm-scoped sessions, tools, memory | Phase 2 |
| **Cross-device sessions** | BFF-synced session state | Phase 2 |
| **Repo semantic index** | Symbol lookup, fast navigation | Phase 4 |
| **Long-term memory** | Cross-session knowledge base | Phase 4 |
| **IDE integration** | LSP bridge for real-time context | Phase 4 |

### 20.5 Short-Term Wins (30 days)

1. **EditFileTool** — Immediate 20-40x token reduction for editing tasks
2. **GrepTool** — Dramatically faster codebase navigation
3. **BAI.md** — Persistent project context; no more re-explaining architecture
4. **Token footer** — Users can see cost; builds trust
5. **Permission patterns** — `Bash(git *)` auto-allow unlocks faster workflows
6. **Line-range ReadFile** — Targeted reads reduce context waste
7. **/compact command** — Long sessions become reliable

**Combined impact:** These changes make `bai code` feel qualitatively different — faster, smarter, more aware. Achievable in 2 weeks.

### 20.6 Long-Term Differentiators (12+ months)

1. **Enterprise-native agent** — The only agentic coding tool with Keycloak RBAC, audit trail, and Temporal integration
2. **SAP/BTP ecosystem** — Native integrations that Claude Code will never have
3. **Multi-tenant by default** — Org-level isolation, team sharing, realm-scoped MCP servers
4. **Cross-device sessions** — Agent state synced via BFF; continue on any machine
5. **Semantic repo memory** — Agent learns your codebase across sessions; no re-exploration overhead
6. **Workflow agent** — `bai` as the orchestrator for long-running enterprise processes, not just interactive coding

### 20.7 Architecture Principles to Maintain

1. **Local execution substrate** — State-modifying ops happen on the user's machine
2. **Backend as LLM proxy** — Backend is stateless per request; loop runs in CLI
3. **gRPC transport** — Keep it; it's a moat for enterprise features
4. **Minimal dependencies** — Don't bloat go.mod; new tools should use stdlib where possible
5. **Tool isolation** — Each tool is a struct with clear interface; no cross-tool dependencies
6. **Approval before mutation** — Never mutate state without user consent or explicit auto-apply
7. **Auditability by default** — Every tool invocation logged; every session replayable

---

## Appendix A: Component Interaction Diagram

```
┌────────┐     ┌──────────────────────────────────────────────────┐
│  User  │     │                  bai CLI                          │
│        │     │                                                    │
│  Input ├────►│  TUI (Bubble Tea)                                 │
│        │     │    │                                               │
│        │     │    ▼                                               │
│        │     │  AgentLoop.Run()                                  │
│        │     │    │                                               │
│        │     │    ├─ Planner.BuildSystemPrompt() ──── BAI.md     │
│        │     │    │                                               │
│        │     │    ├─ ContextManager.NeedsCompaction()            │
│        │     │    │   └─ [if yes] Compact() ──── Backend.Summarize │
│        │     │    │                                               │
│        │     │    ├─ Backend.Chat() ──────────────► BFF gRPC     │
│        │     │    │   └─ StreamEvent channel                      │
│        │     │    │                                               │
│        │     │    ├─ [tool_calls] ToolRegistry.Dispatch()        │
│        │     │    │   ├─ PermissionGate.Check()                  │
│        │     │    │   │   ├─ GlobalSettings                       │
│        │     │    │   │   ├─ ProjectSettings (.bai/settings.json)│
│        │     │    │   │   └─ RBAC claims (JWT)                    │
│        │     │    │   ├─ [RequireApproval] → TUI prompt           │
│        │     │    │   ├─ EventBus.Publish(PreToolUse)             │
│        │     │    │   ├─ tool.Execute()                           │
│        │     │    │   └─ EventBus.Publish(PostToolUse)            │
│        │     │    │       ├─ AuditLogger.Log()                    │
│        │     │    │       └─ TelemetryHook.Record()               │
│        │     │    │                                               │
│        │     │    ├─ SessionManager.Append()                     │
│        │     │    │   ├─ Local JSONL                              │
│        │     │    │   └─ [async] BFF.SyncSession()               │
│        │     │    │                                               │
│        │     │    └─ [stop conditions] return history             │
│        │     │                                                    │
│ Output │◄────│  TUI renders response                             │
└────────┘     └──────────────────────────────────────────────────┘
```

---

*Document version: 1.1 · Architecture study for bai agent platform evolution*  
*Next review: After Phase 0 implementation complete*

---

## Appendix B: Verified Claude Code Implementation Details

*From deep research confirming exact implementation patterns (sources: arxiv 2604.14228, leaked source map analysis, official docs)*

### B.1 Five-Layer Compaction Pipeline (Precise)

Claude Code runs these 5 transformations **in order** before every API call in `query.ts`:

```
Layer 1: applyToolResultBudget()
  → Caps byte size of individual tool results inline in history
  → Cheapest operation; prevents single oversized Bash output consuming context

Layer 2: snipCompact()  [HISTORY_SNIP feature flag]
  → Removes provably-unnecessary middle-of-history messages
  → Keeps opening context + recent turns; uses recency heuristics

Layer 3: microcompact() / cached microcompact
  → Merges consecutive tool_result/user pairs into condensed summaries
  → Lightweight, incremental; cached variant skips unchanged pairs

Layer 4: contextCollapse()  [CONTEXT_COLLAPSE feature flag]
  → Read-time projection over full REPL history
  → Rewrites message array to hide old content; more aggressive than microcompact

Layer 5: autoCompact()
  → Nuclear option; triggers at ~85% context (CLAUDE_AUTOCOMPACT_PCT_OVERRIDE)
  → Forks a second Claude agent to produce comprehensive summary
  → Main loop pauses, waits for summary, replaces all history with compact block
  → Also triggers reactively if API call fails with context-overflow error
```

**bai implication:** Implement this as a middleware chain before each backend call. Start with layers 1 and 5 (cheapest and most important), add 2–4 incrementally.

### B.2 Tool Concurrency — Input-Dependent (Critical Detail)

`isConcurrencySafe(input)` is a **method on each tool that receives the specific call input**, not a static property. This enables per-call safety decisions:

- `Bash("cat file.txt")` → may be safe
- `Bash("rm -rf /tmp")` → not safe (same tool, different safety)

In `toolOrchestration.ts`, the orchestrator partitions the tool_use list from each model response:
1. Walk the list sequentially
2. While `isConcurrencySafe(input) == true`, batch into `Promise.all()`
3. Any unsafe call starts a new serial batch
4. Result: maximum parallelism without sacrificing safety

**bai implication:** Change the `Tool` interface from `IsReadOnly() bool` to `IsConcurrencySafe(input json.RawMessage) bool`. This unlocks parallel execution of safe tool calls (Grep + Read + Glob all at once).

### B.3 Full Tool Inventory (27 Built-in)

Confirmed built-in tools as of v2.1.145:

| Tool | Read-only | Concurrency-safe | Key notes |
|------|-----------|-----------------|-----------|
| `Read` | Yes | Yes | Line ranges, PDFs, images, .ipynb |
| `Write` | No | No | Requires prior Read in session |
| `Edit` | No | No | Exact string replace, uniqueness enforced |
| `MultiEdit` | No | No | Batch edits across files in one call |
| `Glob` | Yes | Yes | Does NOT respect .gitignore |
| `Grep` | Yes | Yes | Uses ripgrep; respects .gitignore |
| `LS` | Yes | Yes | Directory listing |
| `Bash` | No | Input-dep | Persistent shell, sandboxed (bwrap/seatbelt) |
| `TodoRead` | Yes | Yes | Reads session task list |
| `TodoWrite` | No | No | Replaces entire task list |
| `Task` | No | No | Spawns subagent; supports worktree isolation |
| `WebFetch` | Yes | Yes | HTML→Markdown, 15min cache, sub-model process |
| `WebSearch` | Yes | Yes | Returns titles+URLs only (no page fetch) |
| `NotebookRead` | Yes | Yes | Reads .ipynb cells+outputs |
| `NotebookEdit` | No | No | Insert/replace/delete cells |
| `exit_plan_mode` | No | No | Control flow: exit plan mode |
| ... + MCP bridged tools (uniform interface) |

**bai implication:** `MultiEdit` is an important efficiency tool — batch multiple edits to multiple files in one LLM round-trip. Add as P0 alongside `EditFileTool`.

### B.4 Exact Session Storage Layout

```
~/.claude/
├── settings.json                            ← global user settings
├── CLAUDE.md                                ← global memory (human-authored)
└── projects/
    └── <url-encoded-project-path>/          ← e.g. -Users-phani-src-myproject
        ├── sessions/
        │   └── <uuid>.jsonl                 ← append-only, one event per line
        └── memory/
            └── MEMORY.md                   ← auto-memory index (LLM-authored)
```

**JSONL event types:** `user`, `assistant`, `tool_use`, `tool_result`, `system`

**bai equivalent:** Store at `~/.bai/projects/<encoded-path>/sessions/<uuid>.jsonl`  
With BFF sync layer on top for cross-device resumption (bai advantage).

### B.5 Six Permission Modes

| Mode | Behavior |
|------|----------|
| `default` | Ask before destructive actions; read-only auto-approved |
| `acceptEdits` | Auto-approve file edits; ask before Bash |
| `plan` | No execution; plan-only mode |
| `auto` | ML classifier decides; interactive fallback |
| `dontAsk` | All tool calls auto-approved (dangerous) |
| `bypassPermissions` | CI/headless mode; skip all checks |

**bai implication:** Add `acceptEdits` mode (most useful for daily use — auto-approve safe edits) and `plan` mode (high-value for review workflows).

### B.6 Prompt Caching Architecture (Critical for Token Efficiency)

System prompt is **identical for all users on the same version** — this is intentional. It creates a shared cache prefix across the entire Claude Code user base, dramatically reducing API costs.

CLAUDE.md is injected as `<system-reminder>` in messages, NOT in the system prompt. This preserves system prompt cache validity while allowing per-project customization.

**bai implication:** Keep the bai system prompt static. Use a `<system-reminder>` XML tag injection pattern for BAI.md content. This alone could reduce LLM costs by 50%+ on multi-turn sessions.

### B.7 Hooks — Exit Code Semantics

| Hook event | Exit 0 | Exit 1 | Exit 2 |
|-----------|--------|--------|--------|
| `PreToolUse` | proceed | non-blocking error | **block tool call** |
| `Stop` | stop normally | error, stop | **force Claude to continue** |
| `PostToolUse` | proceed | non-blocking error | N/A |

Exit 2 on `Stop` is a powerful pattern: the hook can examine the outcome and tell Claude to keep working. Useful for CI-style validation loops: "if tests fail, keep going".

**bai implication:** Implement the same exit code semantics. This is a simple pattern but enables powerful automation.

### B.8 Multi-Model Strategy

Claude Code uses three model slots:

| Slot | Config key | Default | Used for |
|------|-----------|---------|---------|
| Main | `model` | `claude-sonnet-4-6` | Primary conversation |
| Small | `smallModel` | `claude-haiku-4-5` | Sub-tasks, cheap ops |
| Large | `largeModel` | `claude-opus-4-7` | Complex reasoning |

Internal models (not user-configurable): WebFetch content extraction, auto-compact summarization, permission ML classifier.

**bai implication:** bai's backend already abstracts models. Expose `small_model` and `large_model` config keys. Use small model for compaction summarization to reduce cost.

### B.9 KAIROS — Background Daemon (Unreleased, Important Signal)

KAIROS is a persistent background agent found in the leaked source:
- Runs as a daemon across sessions
- Receives periodic tick prompts (5-minute cron cycles)
- Subscribes to GitHub webhooks in real-time
- Maintains append-only daily log files
- Sends push notifications on action completion

**bai implication:** This confirms the background agent pattern is the major next frontier. bai should build this natively with Temporal for reliability (advantage: Temporal handles durability, retries, and observability that KAIROS would need to build from scratch).

### B.10 Tool Search for MCP Servers

When many MCP servers are configured, tool definitions consume significant context tokens. Claude Code implements **deferred tool loading**: tool schemas are withheld from context by default and loaded on-demand per turn based on model intent.

This is the same mechanism that powers Claude Code's own `ToolSearch` built-in (the tool that lets Claude discover other tools).

**bai implication:** As MCP adoption grows, this becomes critical. Implement deferred schema loading in the tool registry: send only tool names initially, load full schemas when the model requests a tool from a specific domain.

### B.11 Sandboxing Implementation

- **Linux:** `bubblewrap` (bwrap) — user namespaces, filesystem restriction to CWD
- **macOS:** Apple `seatbelt` / `sandbox-exec` with custom profile
- **Network:** Not blocked by default (filesystem-focused)
- **Configuration:** `.claude/settings.json` → `sandbox.enabled`, `sandbox.allowedPaths`

**bai implication:** Phase 3 feature. For Phase 0-1, rely on the approval gate. For Phase 3+, add OS-level sandboxing using the same primitives.

---

## Appendix C: Feature Matrix Addendum

*Additional rows based on confirmed Claude Code implementation details*

| Feature | Claude Code | bai | Gap | Priority |
|---------|-------------|-----|-----|----------|
| MultiEdit (batch edits) | ✓ MultiEdit tool | ✗ | 1 LLM call for N file changes | P0 |
| Parallel tool execution | ✓ isConcurrencySafe(input) | ✗ | Serial only; slow on multi-file reads | P0 |
| 5-layer compaction | ✓ progressive pipeline | ✗ | Single autoCompact only planned | P1 |
| Deferred MCP schemas | ✓ ToolSearch | ✗ | All schemas loaded upfront | P2 |
| OS-level sandboxing | ✓ bwrap/seatbelt | ✗ | Approval gate only | P3 |
| acceptEdits mode | ✓ | ✗ | Only binary approve-all/ask-all | P0 |
| plan mode | ✓ | ✗ | No read-only planning mode | P1 |
| Auto-memory (MEMORY.md) | ✓ LLM-authored | ✗ | No cross-session LLM memory | P2 |
| Session --continue | ✓ | ✗ (server-side only) | No local auto-resume of last session | P1 |
| System prompt caching | ✓ stable system prompt | Unknown | Verify backend caches bai system prompt | P0 |

---

*Sources: arxiv:2604.14228 "Dive into Claude Code", Claude Code official documentation, Piebald-AI system prompts repository, developer community analysis of leaked source maps (v2.1.88, March 31 2026)*
