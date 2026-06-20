// Package hooks discovers and runs shell scripts that intercept tool calls.
//
// Scripts are placed in .bai/hooks/pre-tool/ or .bai/hooks/post-tool/ relative
// to the git root. Each script receives a JSON object on stdin and may write a
// JSON object to stdout. Exit code 2 blocks the tool call; any other non-zero
// code logs a warning and continues.
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
package hooks

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

const hookTimeout = 10 * time.Second

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
