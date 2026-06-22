package worktree

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// initRepo creates a temporary git repo with one committed file.
func initRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%v: %s", args, out)
		}
	}

	run("git", "init")
	run("git", "config", "user.email", "test@example.com")
	run("git", "config", "user.name", "Test")
	run("git", "commit", "--allow-empty", "-m", "init")

	// Commit a tracked file.
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n"), 0644); err != nil {
		t.Fatal(err)
	}
	run("git", "add", "main.go")
	run("git", "commit", "-m", "add main.go")

	return dir
}

func TestCreate(t *testing.T) {
	repo := initRepo(t)
	wt, err := Create(repo, "test-session-123")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	defer func() { _ = Remove(repo, wt, true) }()

	// Worktree should exist and contain the tracked file.
	if _, err := os.Stat(filepath.Join(wt, "main.go")); err != nil {
		t.Errorf("main.go missing in worktree: %v", err)
	}
	// Should be under .bai/worktrees/
	if !strings.Contains(wt, filepath.Join(".bai", "worktrees", "test-session-123")) {
		t.Errorf("unexpected worktree path: %s", wt)
	}
}

func TestRemove(t *testing.T) {
	repo := initRepo(t)
	wt, err := Create(repo, "remove-test")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := Remove(repo, wt, true); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if _, err := os.Stat(wt); !os.IsNotExist(err) {
		t.Errorf("worktree directory still exists after Remove")
	}
}

func TestRemove_AlreadyGone(t *testing.T) {
	repo := initRepo(t)
	// Removing a path that doesn't exist should not error.
	if err := Remove(repo, "/tmp/nonexistent-bai-wt-xyz", false); err != nil {
		t.Errorf("Remove missing path: %v", err)
	}
}

func TestChangedFiles_NoChanges(t *testing.T) {
	repo := initRepo(t)
	wt, err := Create(repo, "nochange-test")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	defer func() { _ = Remove(repo, wt, true) }()

	files, err := ChangedFiles(wt)
	if err != nil {
		t.Fatalf("ChangedFiles: %v", err)
	}
	if len(files) != 0 {
		t.Errorf("expected 0 changed files, got %v", files)
	}
}

func TestChangedFiles_WithModification(t *testing.T) {
	repo := initRepo(t)
	wt, err := Create(repo, "modify-test")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	defer func() { _ = Remove(repo, wt, true) }()

	// Modify the tracked file in the worktree.
	if err := os.WriteFile(filepath.Join(wt, "main.go"), []byte("package main\n\nfunc main() {}\n"), 0644); err != nil {
		t.Fatal(err)
	}

	files, err := ChangedFiles(wt)
	if err != nil {
		t.Fatalf("ChangedFiles: %v", err)
	}
	if len(files) == 0 {
		t.Error("expected at least one changed file, got none")
	}
	found := false
	for _, f := range files {
		if f == "main.go" {
			found = true
		}
	}
	if !found {
		t.Errorf("main.go not in changed files: %v", files)
	}
}

func TestChangedFiles_WithNewFile(t *testing.T) {
	repo := initRepo(t)
	wt, err := Create(repo, "newfile-test")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	defer func() { _ = Remove(repo, wt, true) }()

	// Write a new untracked file in the worktree.
	if err := os.WriteFile(filepath.Join(wt, "new.go"), []byte("package main\n"), 0644); err != nil {
		t.Fatal(err)
	}

	files, err := ChangedFiles(wt)
	if err != nil {
		t.Fatalf("ChangedFiles: %v", err)
	}
	found := false
	for _, f := range files {
		if f == "new.go" {
			found = true
		}
	}
	if !found {
		t.Errorf("new.go not in changed files: %v", files)
	}
}

func TestApply_TrackedModification(t *testing.T) {
	repo := initRepo(t)
	wt, err := Create(repo, "apply-tracked")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	defer func() { _ = Remove(repo, wt, true) }()

	// Modify the tracked file in the worktree.
	newContent := "package main\n\nfunc main() {}\n"
	if err := os.WriteFile(filepath.Join(wt, "main.go"), []byte(newContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Apply to main tree.
	if err := Apply(wt, repo); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	// Main tree should have the new content.
	data, err := os.ReadFile(filepath.Join(repo, "main.go"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != newContent {
		t.Errorf("main.go content = %q, want %q", string(data), newContent)
	}
}

func TestApply_NewFile(t *testing.T) {
	repo := initRepo(t)
	wt, err := Create(repo, "apply-new")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	defer func() { _ = Remove(repo, wt, true) }()

	// Create a new untracked file in the worktree.
	if err := os.WriteFile(filepath.Join(wt, "extra.go"), []byte("package main\n"), 0644); err != nil {
		t.Fatal(err)
	}

	if err := Apply(wt, repo); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	// New file must appear in main tree.
	if _, err := os.Stat(filepath.Join(repo, "extra.go")); err != nil {
		t.Errorf("extra.go not copied to main tree: %v", err)
	}
}

func TestGitRoot_InsideRepo(t *testing.T) {
	repo := initRepo(t)
	orig, _ := os.Getwd()
	defer func() { _ = os.Chdir(orig) }()

	_ = os.Chdir(repo)
	root, err := GitRoot()
	if err != nil {
		t.Fatalf("GitRoot: %v", err)
	}
	// Resolve symlinks for comparison (macOS /var → /private/var).
	got, _ := filepath.EvalSymlinks(root)
	want, _ := filepath.EvalSymlinks(repo)
	if got != want {
		t.Errorf("GitRoot = %q, want %q", got, want)
	}
}

func TestGitRoot_OutsideRepo(t *testing.T) {
	orig, _ := os.Getwd()
	defer func() { _ = os.Chdir(orig) }()

	_ = os.Chdir(t.TempDir())
	_, err := GitRoot()
	if err == nil {
		t.Error("expected error outside a git repo")
	}
}
