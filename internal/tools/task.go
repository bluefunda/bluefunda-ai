package tools

import (
	"bufio"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

const taskTimeout = 10 * time.Minute

// taskDisallowed holds the parent session's deny rules, forwarded to every
// task child as --disallowed-tools (#406). Set once at session start.
var (
	taskDisallowedMu sync.RWMutex
	taskDisallowed   []string
)

// SetTaskDisallowedTools records the deny rules task children must inherit.
func SetTaskDisallowedTools(rules []string) {
	taskDisallowedMu.Lock()
	defer taskDisallowedMu.Unlock()
	taskDisallowed = append([]string(nil), rules...)
}

// taskDisallowedArgs renders the inherited deny rules as child CLI flags:
// one --disallowed-tools=<rule> per rule. The child parses the flag as a
// pflag StringSlice, which splits on commas CSV-style, so each rule is
// CSV-encoded to reach the child as exactly one rule.
func taskDisallowedArgs() []string {
	taskDisallowedMu.RLock()
	defer taskDisallowedMu.RUnlock()
	args := make([]string, 0, len(taskDisallowed))
	for _, r := range taskDisallowed {
		args = append(args, "--disallowed-tools="+csvField(r))
	}
	return args
}

// csvField encodes s as a single CSV field (quoted only when needed).
func csvField(s string) string {
	var b strings.Builder
	w := csv.NewWriter(&b)
	_ = w.Write([]string{s})
	w.Flush()
	return strings.TrimSuffix(b.String(), "\n")
}

// taskArgs builds the child's command line: headless, auto-approved, with the
// parent's deny rules, and the prompt last after "--" so a prompt starting
// with "-" isn't read as a flag.
func taskArgs(prompt string, maxTurns int, useWorktree bool) []string {
	args := []string{"--print", "--output-format", "stream-json", "--auto"}
	args = append(args, taskDisallowedArgs()...)
	if maxTurns > 0 {
		args = append(args, "--max-turns", fmt.Sprintf("%d", maxTurns))
	}
	if useWorktree {
		args = append(args, "--worktree")
	}
	return append(args, "--", prompt)
}

// RunTask spawns a headless bai subprocess with the given prompt and returns
// its final text output as the tool result. The child runs with --auto so it
// never blocks waiting for approval. Budget controls are NOT inherited from the
// parent to avoid double-counting costs.
func RunTask(ctx context.Context, prompt, workDir string, maxTurns int, useWorktree bool) (string, error) {
	self, err := os.Executable()
	if err != nil {
		self, err = exec.LookPath("bai")
		if err != nil {
			return "", fmt.Errorf("cannot locate bai binary: %w", err)
		}
	}

	args := taskArgs(prompt, maxTurns, useWorktree)

	taskCtx, cancel := context.WithTimeout(ctx, taskTimeout)
	defer cancel()

	cmd := exec.CommandContext(taskCtx, self, args...)
	if workDir != "" {
		cmd.Dir = workDir
	}
	cmd.Stderr = os.Stderr

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", fmt.Errorf("task: stdout pipe: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("task: start: %w", err)
	}

	var textParts []string
	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 4*1024*1024), 4*1024*1024)
	for scanner.Scan() {
		var ev map[string]any
		if err := json.Unmarshal(scanner.Bytes(), &ev); err != nil {
			continue
		}
		if t, _ := ev["type"].(string); t == "text" {
			if text, _ := ev["text"].(string); text != "" {
				textParts = append(textParts, text)
			}
		}
	}

	if err := cmd.Wait(); err != nil && taskCtx.Err() != nil {
		return "", fmt.Errorf("task timed out after %s", taskTimeout)
	}

	if len(textParts) == 0 {
		return "(no output)", nil
	}
	return strings.Join(textParts, ""), nil
}
