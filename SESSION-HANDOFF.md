# Session Handoff — TUI scrollback fix + agentic tool-calling regression

_Date: 2026-07-04 · repo: `bluefunda-ai` (also references `cai-bff`, `cai-llm-router`)_

Two separate threads in this session:
1. **DONE + committed to working tree** — TUI scrollback fix.
2. **DIAGNOSED, issues drafted (not filed)** — local file tools / agentic loop broken; root cause is in `cai-llm-router`.

---

## Thread 1 — TUI scroll fix (COMPLETE, in working tree, not committed)

### Problem
TUI couldn't scroll; responses scrolled off and were unrecoverable.

### Root cause
Commit #201 ("persistent viewport") set up the data structures but never wired the scroll path:
- `view.go` never rendered `m.viewport.View()` — it showed only the **last N lines** of the whole conversation (manual tail clamp), truncating everything above.
- PgUp/PgDn drove an invisible viewport → no effect.
- The `tea.Println` scrollback-commit that comments *claimed* existed was **never called**; `printed` was never set true. So history lived in the ephemeral inline block and was lost.
- `program.go` ran `tea.NewProgram(m)` with no options (no alt-screen, no mouse).

### Decision
User chose **inline + native scrollback** (Claude Code model) over alt-screen viewport — finished turns go to the terminal's real scrollback via `tea.Println`; only the live turn stays inline. Native trackpad/wheel scroll; native copy/paste keeps working (doesn't worsen open bug #59).

### Changes made (all in `internal/ui/tui/`)
- **`model.go`**:
  - Added `commitScrollback()` — renders each finished (non-live) unprinted message, joins into a single `tea.Println` (single call per Update cycle to preserve order), marks them `printed`. Skips the live streaming assistant turn until done.
  - Added `withCommit(cmd)` wrapper.
  - Wired `commitScrollback`/`withCommit` into the 3 `Update` exit points: `WindowSizeMsg` path, `KeyMsg` path, and the fall-through bottom return (for stream/async events).
  - Removed dead `pgup`/`pgdown` key handlers.
  - Updated `/help` text: "Scroll — Use your terminal's scrollback (mouse / trackpad)".
- **`scrollback_test.go`** (new): 3 tests — finished msgs get committed; live streaming turn stays inline until `finishStreaming()`; `renderActiveMessages` excludes printed msgs. All pass.

### Status
`go build ./...` OK, `go vet` OK, tests pass. **Not committed.** Interactive smoke test still pending (needs TTY + auth): long response + scroll mid-stream, `/help`, Ctrl+C interrupt, mouse text-selection/copy.

### Known limitation (inherent to this model)
Committed scrollback does **not** reflow on terminal resize (same as Claude Code). Worth a docs note.

### Binary
Built at `/Users/phani/Downloads/src/bluefunda-ai/bai` via `make build` (v1.34.0-dirty). Run in a real terminal: `./bai` (agentic) or `./bai chat` (no tools).

---

## Thread 2 — Local file tools / agentic loop broken (ROOT-CAUSED; fix is in cai-llm-router)

### Symptom
Asking `bai` to access local files does nothing; the model claims it has no `list_dir`/file tools and only offers `web_search` / `fetch_url`.

### Command clarification
`bai` and `bai code` are the **same** now — `bai code` is a deprecated alias; both call `runAgenticSession` and load local tools. **`bai chat`** is the only tools-less mode. `--no-tools` disables tools. So the user does NOT need `bai code`.

### How the contract works (fragile "smuggling")
Because the load balancer strips proto fields 8+ (`local_tools`, `code_messages`), the CLI:
- Sets `Model: "cli/<model>"` and packs tools+history as JSON into the `Prompt` field.
- `buildCodeRequest` in `internal/cmd/code.go:1315` builds `cliCodePayload{V:1, History, Tools}`.
The router (`cai-llm-router`) is supposed to detect the `cli/` prefix and rehydrate `req.LocalTools` from `Prompt`.

### Evidence gathered (verified end-to-end)
- **CLI is correct** (proved via throwaway test): `LocalToolSchemas()` = 6136 bytes incl. `list_dir`; request `Model="cli/anthropic"`, `Prompt`=6759-byte JSON, `V=1`, tools present. CLI smuggling code unchanged for months (only #84 added the version field).
- **cai-bff is correct**: `internal/transport/grpc/handler.go:303` preserves the `cli/` prefix (only strips `:think` suffix) and forwards `Prompt` + `LocalTools` unchanged.
- **cai-llm-router version guard is fine**: `internal/handler/chat.go:180` `expectedVersion=1`, tolerates `V=1`; a mismatch would emit an explicit `StreamError` — NOT observed.
- **Downstream assembly is fine**: `handleLocalToolsRequest` → `buildLLMRequest` (`chat.go:1042-1046`) attaches tools correctly — but that path is only reached when `req.LocalTools != ""` (`chat.go:214`, `405`).

### Root cause
At runtime the model exposed **server-side** tools (`web_search`/`fetch_url` — the fetch-mcp swap from cai-llm-router #123), which exist **only on the normal agent path**. Therefore `req.LocalTools` was **empty** at the router → the `cli/` decode did not populate it → the request **silently fell through to the normal agent**. Most consistent trigger: the `cli/` prefix not reaching the decode check at `chat.go:169` (decode skipped). Whatever the trigger, the router's own defect is that it **silently degrades** — no detection or error path for "a `cli/`-originated request that produced zero local tools." Success is only a `Debug` log; bad JSON is only a `Warn` (`chat.go:200`) and still falls through.

### Repro
`bai --print -m anthropic "Use the list_dir tool on the current directory. You must call the tool."`
→ model says it has no such tool, lists only web tools.

### Key file references (cai-llm-router)
- `internal/handler/chat.go:169-203` — `cli/` prefix detect + `cliCodePayload` decode + version guard.
- `internal/handler/chat.go:214`, `405-406` — routing decision (`if req.LocalTools != ""`).
- `internal/handler/chat.go:862-921` — `handleLocalToolsRequest`.
- `internal/handler/chat.go:1042-1049` — `buildLLMRequest` parses `local_tools` into `[]llmrouter.Tool`; **silently ignores on parse failure**.

---

## NEXT ACTIONS (per user: only plan + create issues in cai-llm-router; user will execute from console)

### Plan for cai-llm-router
1. **Fail loud, don't degrade.** `cli/` prefix present but decode yields empty `LocalTools` → emit `StreamError` + `DoneEvent`, do NOT route to normal agent.
2. **Diagnose prefix-vs-payload.** Add `Info`/`Error` log at routing decision (`chat.go:214`): was request CLI-originated (prefix seen) and were tools recovered? Distinguishes "prefix lost upstream" from "prompt decode failed."
3. **Harden tool-schema parse.** `chat.go:1042-1049`: make `[]llmrouter.Tool` unmarshal failure a hard error for `cli/` requests; add a test unmarshalling the CLI's current `LocalToolSchemas()` into `llmrouter.Tool` to catch drift from the llmrouter dep bump (#76).

### Issue 1 (ready to file)
Title: `fix(handler): CLI agentic requests silently degrade to tool-less agent when cli/ payload yields no local_tools`
Repo: `bluefunda/cai-llm-router` · label `bug`
Body: see full text below.

### Issue 2 (ready to file)
Title: `fix(handler): don't silently drop local_tools on schema parse failure; verify llmrouter.Tool shape`
Repo: `bluefunda/cai-llm-router` · label `bug`
Body: see full text below.

### gh commands
```bash
# Issue 1
gh issue create --repo bluefunda/cai-llm-router \
  --title "fix(handler): CLI agentic requests silently degrade to tool-less agent when cli/ payload yields no local_tools" \
  --label bug \
  --body "$(cat <<'EOF'
## Problem
`bai` (CLI-side agentic loop) can no longer use local file tools (`list_dir`, `read_file`, `edit_file`, `bash`, …). The model responds as if those tools don't exist and instead reports only server-side tools (`web_search`, `fetch_url`).

## Root cause
The CLI smuggles tool schemas into `Prompt` and marks the request with `Model: "cli/<model>"`. The router rehydrates `req.LocalTools` in `internal/handler/chat.go:169-203` only when the `cli/` prefix is present and `Prompt` decodes as a valid `cliCodePayload`.

Verified across the stack:
- CLI sends `Model: cli/<model>` + valid payload (`V=1`, tools present). OK
- cai-bff preserves the `cli/` prefix and `Prompt`. OK
- Router version guard tolerates `V=1` (a mismatch would emit an explicit error — not observed). OK
- `handleLocalToolsRequest`/`buildLLMRequest` attach tools correctly — but only when `req.LocalTools != ""` (`chat.go:214`, `405`). OK

Decisive: at runtime the model exposed **server-side** tools (`web_search`/`fetch_url`), which exist only on the **normal agent path**. So `req.LocalTools` was **empty** at the router and the request silently fell through to the normal agent. The `cli/` decode did not populate `LocalTools`.

## The router defect
Regardless of the upstream trigger, the router **silently degrades**: there is no detection or error path for "a `cli/`-originated request that ended up with zero local tools." Success is only a `Debug` log; a bad-JSON `Prompt` is only a `Warn` (`chat.go:200`) and still falls through; a stripped prefix has no logging at all.

## Expected behaviour
- A request whose `Model` has the `cli/` prefix but resolves to empty `LocalTools` after decode is a **hard error**: publish `StreamError` (+ `DoneEvent`), do not route to the normal agent.
- Add `Info`/`Error` logging at the routing decision (`chat.go:214`) recording: was the request CLI-originated (prefix seen), and were local tools recovered?

## Repro
`bai --print -m anthropic "Use the list_dir tool on the current directory. You must call the tool."`
→ model replies it has no such tool and lists only web tools.

## Affected code
- `internal/handler/chat.go:169-203` (decode + guards)
- `internal/handler/chat.go:214` / `405-406` (routing decision)
EOF
)"

# Issue 2
gh issue create --repo bluefunda/cai-llm-router \
  --title "fix(handler): don't silently drop local_tools on schema parse failure; verify llmrouter.Tool shape" \
  --label bug \
  --body "$(cat <<'EOF'
## Problem
`buildLLMRequest` (`internal/handler/chat.go:1042-1049`) parses `req.LocalTools` into `[]llmrouter.Tool`. On `json.Unmarshal` failure it logs `Warn("Failed to parse local_tools, ignoring")` and proceeds **with no tools attached**. This silently strips the entire CLI toolset if the schema shape drifts.

## Why now
The `llmrouter` dependency was bumped (#76, StreamResult API). If `llmrouter.Tool` changed shape relative to the CLI's `ToolSchema` JSON (`{type, function:{name, description, parameters}}`), every CLI agentic request loses its tools with only a `Warn`.

## Expected behaviour
- For `cli/`-originated requests, a `local_tools` parse failure must be a **hard error** (`StreamError` + `DoneEvent`), not a silent ignore.
- Add a test that unmarshals the CLI's current `LocalToolSchemas()` output into `[]llmrouter.Tool` to lock the contract and catch future drift.

## Affected code
- `internal/handler/chat.go:1042-1049`
EOF
)"
```

### Optional / deferred
Cross-repo hardening (out of scope per user): kill the `cli/`-prefix + Prompt-smuggling hack in favor of the real `local_tools` proto field once the LB stops stripping fields 8+. Touches CLI (`internal/cmd/code.go:buildCodeRequest`) + cai-bff. File as a separate tracking issue only if desired.

---

## Open pre-existing issues noted (bluefunda-ai)
- #59 (OPEN) copy/paste in TUI on macOS — the scrollback fix keeps native selection working but doesn't add OSC 52.
- #200 (CLOSED) was the TUI UX issue whose `tea.Println` plan #201 left half-implemented (Thread 1 finishes it).
