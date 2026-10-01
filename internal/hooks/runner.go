// Package hooks discovers and runs shell scripts for both tool-level and
// session-lifecycle events.
//
// Scripts are placed in .bai/hooks/<phase>/ relative to the git root, where
// phase is one of the entries in Phases. Each script receives a JSON object
// on stdin and may write a JSON object to stdout. Exit code 2 blocks the
// associated action (tool call, stop, or compaction); any other non-zero
// code logs a warning and continues.
//
// Tool-level phases match a script's filename (minus extension) against the
// tool name, or "*" for any tool:
//
// Pre-tool stdin:
//
//	{"hook":"pre-tool","session_id":"…","tool_name":"bash","tool_input":{…},"cwd":"…"}
//
// Pre-tool stdout (optional):
//
//	{"modified_input":{…},"system_message":"reason shown to LLM if blocked"}
//
// Post-tool stdin:
//
//	{"hook":"post-tool","session_id":"…","tool_name":"bash","tool_input":{…},"tool_result":"…","cwd":"…"}
//
// Lifecycle phases run every script in the phase directory (no filename
// matching, since these events have no associated tool):
//
// Session-start stdin:
//
//	{"hook":"session-start","session_id":"…","cwd":"…","model":"…","version":"…"}
//
// Session-start stdout (optional, non-blocking):
//
//	{"additional_context":"text injected into the session's history"}
//
// Session-end stdin (fire-and-forget):
//
//	{"hook":"session-end","session_id":"…","cwd":"…","turns":3,"stop_reason":"end_turn"}
//
// Stop stdin:
//
//	{"hook":"stop","session_id":"…","cwd":"…"}
//
// Stop stdout on block (exit 2, optional):
//
//	{"system_message":"reason fed back to the LLM to keep going"}
//
// Pre-compact stdin:
//
//	{"hook":"pre-compact","session_id":"…","cwd":"…","estimated_tokens":12345}
//
// Pre-compact exit 2 skips compaction for that round.
package hooks

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const hookTimeout = 10 * time.Second

// Phases lists every hook phase directory name under .bai/hooks/. Exported so
// callers (e.g. `bai doctor`) can enumerate configured hooks without
// duplicating this list.
var Phases = []string{"pre-tool", "post-tool", "session-start", "session-end", "stop", "pre-compact"}

// Result is the outcome of running pre-tool hooks.
type Result struct {
	// ModifiedInput replaces the original tool input when non-nil.
	ModifiedInput map[string]any
	// SystemMessage is fed back to the LLM when Block is true.
	SystemMessage string
	// Block prevents the tool from executing.
	Block bool
}

// Runner discovers and executes hook scripts for a project.
type Runner struct {
	hooksDir  string
	sessionID string
	cwd       string
}

// New returns a Runner for the given hooks directory. hooksDir is typically
// <git-root>/.bai/hooks. Returns a no-op Runner if hooksDir does not exist.
func New(hooksDir, sessionID, cwd string) *Runner {
	return &Runner{hooksDir: hooksDir, sessionID: sessionID, cwd: cwd}
}

// FindHooksDir walks upward from cwd to the git root looking for .bai/hooks.
// Returns "" if not found.
func FindHooksDir(cwd string) string {
	abs, err := filepath.Abs(cwd)
	if err != nil {
		return ""
	}
	for {
		candidate := filepath.Join(abs, ".bai", "hooks")
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
		if _, err := os.Stat(filepath.Join(abs, ".git")); err == nil {
			break
		}
		parent := filepath.Dir(abs)
		if parent == abs {
			break
		}
		abs = parent
	}
	return ""
}

// PreToolUse runs pre-tool hooks for toolName with inputJSON.
// Returns Result indicating whether to block and any modified input.
func (r *Runner) PreToolUse(toolName, inputJSON string) Result {
	scripts := r.findScripts("pre-tool", toolName)
	if len(scripts) == 0 {
		return Result{}
	}

	payload := map[string]any{
		"hook":       "pre-tool",
		"session_id": r.sessionID,
		"tool_name":  toolName,
		"cwd":        r.cwd,
	}
	var input map[string]any
	if json.Unmarshal([]byte(inputJSON), &input) == nil {
		payload["tool_input"] = input
	}

	current := payload
	for _, script := range scripts {
		res, block, msg := runScript(script, current)
		if block {
			return Result{Block: true, SystemMessage: msg}
		}
		if res != nil {
			if mod, ok := res["modified_input"].(map[string]any); ok {
				current["tool_input"] = mod
			}
		}
	}

	var modInput map[string]any
	if ti, ok := current["tool_input"].(map[string]any); ok {
		modInput = ti
	}
	return Result{ModifiedInput: modInput}
}

// PostToolUse runs post-tool hooks for toolName (fire-and-forget; errors logged only).
func (r *Runner) PostToolUse(toolName, inputJSON, result string) {
	scripts := r.findScripts("post-tool", toolName)
	if len(scripts) == 0 {
		return
	}
	payload := map[string]any{
		"hook":        "post-tool",
		"session_id":  r.sessionID,
		"tool_name":   toolName,
		"tool_result": result,
		"cwd":         r.cwd,
	}
	var input map[string]any
	if json.Unmarshal([]byte(inputJSON), &input) == nil {
		payload["tool_input"] = input
	}
	for _, script := range scripts {
		runScript(script, payload) //nolint:errcheck
	}
}

// SessionStart runs session-start hooks and returns any additional context
// they emit, to be injected into the session's history. Non-blocking: a
// session cannot be "blocked" from starting.
func (r *Runner) SessionStart(model, version string) string {
	scripts := r.findLifecycleScripts("session-start")
	if len(scripts) == 0 {
		return ""
	}
	payload := map[string]any{
		"hook":       "session-start",
		"session_id": r.sessionID,
		"cwd":        r.cwd,
		"model":      model,
		"version":    version,
	}
	var parts []string
	for _, script := range scripts {
		out, _, _ := runScript(script, payload)
		if ctx, ok := out["additional_context"].(string); ok && ctx != "" {
			parts = append(parts, ctx)
		}
	}
	return strings.Join(parts, "\n\n")
}

// SessionEnd runs session-end hooks (fire-and-forget; errors logged only).
func (r *Runner) SessionEnd(turns int, stopReason string) {
	scripts := r.findLifecycleScripts("session-end")
	if len(scripts) == 0 {
		return
	}
	payload := map[string]any{
		"hook":        "session-end",
		"session_id":  r.sessionID,
		"cwd":         r.cwd,
		"turns":       turns,
		"stop_reason": stopReason,
	}
	for _, script := range scripts {
		runScript(script, payload) //nolint:errcheck
	}
}

// Stop runs stop hooks. A script exiting 2 blocks the agent from ending its
// turn; the returned Result's SystemMessage should be fed back to the LLM so
// it knows why it's being asked to continue.
func (r *Runner) Stop() Result {
	scripts := r.findLifecycleScripts("stop")
	if len(scripts) == 0 {
		return Result{}
	}
	payload := map[string]any{
		"hook":       "stop",
		"session_id": r.sessionID,
		"cwd":        r.cwd,
	}
	for _, script := range scripts {
		_, block, msg := runScript(script, payload)
		if block {
			return Result{Block: true, SystemMessage: msg}
		}
	}
	return Result{}
}

// PreCompact runs pre-compact hooks before context compaction. A script
// exiting 2 blocks compaction for this round.
func (r *Runner) PreCompact(estimatedTokens int) Result {
	scripts := r.findLifecycleScripts("pre-compact")
	if len(scripts) == 0 {
		return Result{}
	}
	payload := map[string]any{
		"hook":             "pre-compact",
		"session_id":       r.sessionID,
		"cwd":              r.cwd,
		"estimated_tokens": estimatedTokens,
	}
	for _, script := range scripts {
		_, block, msg := runScript(script, payload)
		if block {
			return Result{Block: true, SystemMessage: msg}
		}
	}
	return Result{}
}

// findLifecycleScripts returns every script in .bai/hooks/<phase>/, with no
// filename matching (lifecycle events have no associated tool name).
func (r *Runner) findLifecycleScripts(phase string) []string {
	if r.hooksDir == "" {
		return nil
	}
	dir := filepath.Join(r.hooksDir, phase)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var scripts []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		scripts = append(scripts, filepath.Join(dir, e.Name()))
	}
	return scripts
}

// findScripts returns hook scripts matching toolName or the wildcard (*).
func (r *Runner) findScripts(phase, toolName string) []string {
	if r.hooksDir == "" {
		return nil
	}
	dir := filepath.Join(r.hooksDir, phase)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var scripts []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		base := name
		// strip extension for matching
		for i := len(name) - 1; i >= 0; i-- {
			if name[i] == '.' {
				base = name[:i]
				break
			}
		}
		if base == toolName || base == "*" {
			scripts = append(scripts, filepath.Join(dir, name))
		}
	}
	return scripts
}

// runScript executes a single hook script, returning parsed stdout, whether to
// block (exit code 2), and an optional system message.
func runScript(script string, payload map[string]any) (map[string]any, bool, string) {
	input, err := json.Marshal(payload)
	if err != nil {
		return nil, false, ""
	}

	ctx, cancel := context.WithTimeout(context.Background(), hookTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, script)
	cmd.Stdin = bytes.NewReader(input)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err = cmd.Run()
	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		}
	}

	if exitCode == 2 {
		var out map[string]any
		msg := ""
		if json.Unmarshal(stdout.Bytes(), &out) == nil {
			if s, ok := out["system_message"].(string); ok {
				msg = s
			}
		}
		if msg == "" {
			msg = fmt.Sprintf("hook %s blocked tool execution", filepath.Base(script))
		}
		return nil, true, msg
	}

	if exitCode != 0 {
		// Non-2 failure: log to stderr and continue.
		fmt.Fprintf(os.Stderr, "[bai] hook %s exited %d: %s\n", filepath.Base(script), exitCode, stderr.String())
		return nil, false, ""
	}

	var out map[string]any
	json.Unmarshal(stdout.Bytes(), &out) //nolint:errcheck
	return out, false, ""
}
