package tools

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

const taskTimeout = 10 * time.Minute

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

	args := []string{"--print", "--output-format", "stream-json", "--auto", prompt}
	if maxTurns > 0 {
		args = append(args, "--max-turns", fmt.Sprintf("%d", maxTurns))
	}
	if useWorktree {
		args = append(args, "--worktree")
	}

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
