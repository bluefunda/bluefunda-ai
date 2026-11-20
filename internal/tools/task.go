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
func taskArgs(agent AgentType, prompt string, maxTurns int, useWorktree bool) []string {
	args := []string{"--print", "--output-format", "stream-json", "--auto"}
	args = append(args, taskDisallowedArgs()...)
	// The agent type's limits (#408): its tool allow-list and model.
	if len(agent.Tools) > 0 {
		fields := make([]string, len(agent.Tools))
		for i, t := range agent.Tools {
			fields[i] = csvField(t)
		}
		args = append(args, "--tools="+strings.Join(fields, ","))
	}
	if agent.Model != "" {
		args = append(args, "--model", agent.Model)
	}
	if maxTurns > 0 {
		args = append(args, "--max-turns", fmt.Sprintf("%d", maxTurns))
	}
	if useWorktree {
		args = append(args, "--worktree")
	}
	return append(args, "--", taskPrompt(agent, prompt))
}

// RunTask spawns a headless bai subprocess with the given prompt and returns
// its final text output as the tool result. The child runs with --auto so it
// never blocks waiting for approval. Budget controls are NOT inherited from the
// parent to avoid double-counting costs.
func RunTask(ctx context.Context, agent AgentType, prompt, workDir string, maxTurns int, useWorktree bool) (string, error) {
	self, err := os.Executable()
	if err != nil {
		self, err = exec.LookPath("bai")
		if err != nil {
			return "", fmt.Errorf("cannot locate bai binary: %w", err)
		}
	}

	args := taskArgs(agent, prompt, maxTurns, useWorktree)

	taskCtx, cancel := context.WithTimeout(ctx, taskTimeout)
	defer cancel()

	cmd := exec.CommandContext(taskCtx, self, args...)
	if workDir != "" {
		cmd.Dir = workDir
	}
	// Never the terminal: the parent may be drawing a TUI there. Kept for
	// the error if the child fails.
	stderr := &tailBuffer{max: 2048}
	cmd.Stderr = stderr

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", fmt.Errorf("task: stdout pipe: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("task: start: %w", err)
	}

	report := progressFrom(ctx)
	calls := 0
	var textParts []string
	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 4*1024*1024), 4*1024*1024)
	for scanner.Scan() {
		var ev map[string]any
		if err := json.Unmarshal(scanner.Bytes(), &ev); err != nil {
			continue
		}
		switch t, _ := ev["type"].(string); t {
		case "text":
			if text, _ := ev["text"].(string); text != "" {
				textParts = append(textParts, text)
			}
		case "tool_use":
			calls++
			name, _ := ev["name"].(string)
			input, _ := ev["input"].(string)
			report(fmt.Sprintf("%s %s", plural(calls, "tool call"), activityLine(name, input)))
		}
	}

	waitErr := cmd.Wait()
	if waitErr != nil && taskCtx.Err() != nil {
		return "", fmt.Errorf("task timed out after %s", taskTimeout)
	}
	if waitErr != nil && len(textParts) == 0 {
		if msg := strings.TrimSpace(stderr.String()); msg != "" {
			return "", fmt.Errorf("subagent failed: %w: %s", waitErr, msg)
		}
		return "", fmt.Errorf("subagent failed: %w", waitErr)
	}

	if len(textParts) == 0 {
		return "(no output)", nil
	}
	return strings.Join(textParts, ""), nil
}

// ProgressFunc receives one-line updates while a tool runs, such as a
// subagent's latest tool call.
type ProgressFunc func(line string)

type progressKey struct{}

// WithProgress returns a context whose task calls report to f.
func WithProgress(ctx context.Context, f ProgressFunc) context.Context {
	return context.WithValue(ctx, progressKey{}, f)
}

func progressFrom(ctx context.Context) ProgressFunc {
	if f, ok := ctx.Value(progressKey{}).(ProgressFunc); ok && f != nil {
		return f
	}
	return func(string) {}
}

func plural(n int, noun string) string {
	if n == 1 {
		return "1 " + noun
	}
	return fmt.Sprintf("%d %ss", n, noun)
}

// activityLine describes a subagent's tool call: "· bash go test ./...".
func activityLine(name, inputJSON string) string {
	var in map[string]any
	_ = json.Unmarshal([]byte(inputJSON), &in)
	for _, k := range []string{"command", "path", "pattern", "query", "url", "prompt"} {
		if v, ok := in[k].(string); ok && v != "" {
			v = strings.Join(strings.Fields(v), " ")
			if r := []rune(v); len(r) > 60 {
				v = string(r[:57]) + "..."
			}
			return "· " + name + " " + v
		}
	}
	return "· " + name
}

// tailBuffer keeps the last max bytes written to it.
type tailBuffer struct {
	mu  sync.Mutex
	max int
	buf []byte
}

func (b *tailBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.buf = append(b.buf, p...)
	if over := len(b.buf) - b.max; over > 0 {
		b.buf = append([]byte(nil), b.buf[over:]...)
	}
	return len(p), nil
}

func (b *tailBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return strings.ToValidUTF8(string(b.buf), "")
}
