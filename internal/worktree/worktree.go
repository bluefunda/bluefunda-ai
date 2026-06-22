// Package worktree manages temporary git worktrees for isolated agent sessions.
//
// When --worktree is set, the agentic loop runs inside a detached worktree so
// that all file writes are isolated from the user's main working tree. At end of
// session the user chooses to apply, discard, or keep the worktree.
package worktree

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// GitRoot returns the absolute path to the git repository root from the
// current working directory.
func GitRoot() (string, error) {
	out, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return "", fmt.Errorf("not inside a git repository")
	}
	return strings.TrimSpace(string(out)), nil
}

// Create adds a detached worktree at <gitRoot>/.bai/worktrees/<sessionID>.
// Returns the absolute path to the created worktree.
func Create(gitRoot, sessionID string) (string, error) {
	path := filepath.Join(gitRoot, ".bai", "worktrees", sessionID)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return "", fmt.Errorf("create worktrees dir: %w", err)
	}
	cmd := exec.Command("git", "-C", gitRoot, "worktree", "add", "--detach", path)
	if out, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("git worktree add: %s", strings.TrimSpace(string(out)))
	}
	return path, nil
}

// Remove removes a worktree. When force is true, uncommitted changes are discarded.
// gitRoot is the main repository root; it is required so that git resolves the
// correct worktree list regardless of the process working directory.
func Remove(gitRoot, path string, force bool) error {
	args := []string{"-C", gitRoot, "worktree", "remove"}
	if force {
		args = append(args, "--force")
	}
	args = append(args, path)
	cmd := exec.Command("git", args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		// If the path is already gone, treat as success.
		if _, statErr := os.Stat(path); os.IsNotExist(statErr) {
			return nil
		}
		return fmt.Errorf("git worktree remove: %s", strings.TrimSpace(string(out)))
	}
	return nil
}

// ChangedFiles returns the names of files modified or added in the worktree
// relative to HEAD (tracked changes) plus untracked files.
func ChangedFiles(worktreePath string) ([]string, error) {
	var all []string

	// Tracked modifications / deletions.
	out, err := exec.Command("git", "-C", worktreePath, "diff", "--name-only", "HEAD").Output()
	if err == nil {
		for _, f := range splitLines(string(out)) {
			if f != "" {
				all = append(all, f)
			}
		}
	}

	// Untracked new files (created by the agent but not committed).
	out, err = exec.Command("git", "-C", worktreePath,
		"ls-files", "--others", "--exclude-standard").Output()
	if err == nil {
		for _, f := range splitLines(string(out)) {
			if f != "" {
				all = append(all, f)
			}
		}
	}

	return all, nil
}

// Apply patches changes from worktreePath into the main tree at gitRoot.
// Tracked modifications are applied via `git diff HEAD | git apply`.
// New untracked files are copied directly into the main tree.
func Apply(worktreePath, gitRoot string) error {
	// 1. Apply tracked changes via unified diff.
	diff, err := gitDiff(worktreePath)
	if err != nil {
		return err
	}
	if len(bytes.TrimSpace(diff)) > 0 {
		applyCmd := exec.Command("git", "-C", gitRoot, "apply", "--whitespace=fix")
		applyCmd.Stdin = bytes.NewReader(diff)
		if out, err := applyCmd.CombinedOutput(); err != nil {
			return fmt.Errorf("git apply: %s", strings.TrimSpace(string(out)))
		}
	}

	// 2. Copy untracked new files.
	out, _ := exec.Command("git", "-C", worktreePath,
		"ls-files", "--others", "--exclude-standard").Output()
	for _, relPath := range splitLines(string(out)) {
		if relPath == "" {
			continue
		}
		src := filepath.Join(worktreePath, relPath)
		dst := filepath.Join(gitRoot, relPath)
		if err := copyFile(src, dst); err != nil {
			return fmt.Errorf("copy %s: %w", relPath, err)
		}
	}

	return nil
}

// gitDiff returns the binary-safe unified diff of tracked changes in the worktree.
func gitDiff(worktreePath string) ([]byte, error) {
	out, err := exec.Command("git", "-C", worktreePath, "diff", "--binary", "HEAD").Output()
	if err != nil {
		// exit 1 from git diff means differences exist — that's fine.
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return out, nil
		}
		return nil, fmt.Errorf("git diff: %w", err)
	}
	return out, nil
}

// copyFile copies src to dst, creating parent directories as needed.
func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}
	info, _ := os.Stat(src)
	mode := os.FileMode(0644)
	if info != nil {
		mode = info.Mode()
	}
	return os.WriteFile(dst, data, mode)
}

func splitLines(s string) []string {
	return strings.Split(strings.TrimRight(s, "\n"), "\n")
}
