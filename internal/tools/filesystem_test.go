package tools

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestWorkspaceRoot_FindsGitAncestor(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	sub := filepath.Join(root, "a", "b", "c")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(sub)

	got := workspaceRoot()
	want, _ := filepath.EvalSymlinks(root)
	gotResolved, _ := filepath.EvalSymlinks(got)
	if gotResolved != want {
		t.Errorf("workspaceRoot() = %q, want %q", got, want)
	}
}

func TestWorkspaceRoot_FallsBackToCwdWithoutGit(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	got := workspaceRoot()
	want, _ := filepath.EvalSymlinks(dir)
	gotResolved, _ := filepath.EvalSymlinks(got)
	if gotResolved != want {
		t.Errorf("workspaceRoot() = %q, want %q (no .git anywhere above a temp dir)", got, want)
	}
}

func TestConfinePath_AllowsInsideRoot(t *testing.T) {
	root := t.TempDir()
	if _, err := confinePath(root, filepath.Join(root, "sub", "file.txt")); err != nil {
		t.Errorf("expected a path inside root to be allowed, got: %v", err)
	}
}

func TestConfinePath_RejectsAbsoluteOutsideRoot(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir() // a sibling temp dir, not under root
	if _, err := confinePath(root, outside); err == nil {
		t.Error("expected an absolute path outside root to be rejected")
	}
}

func TestConfinePath_RejectsTraversalOutsideRoot(t *testing.T) {
	root := t.TempDir()
	sub := filepath.Join(root, "sub")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(sub)

	if _, err := confinePath(root, "../../etc/passwd"); err == nil {
		t.Error("expected ../.. traversal outside root to be rejected")
	}
}

func TestConfinePath_RejectsSymlinkEscapingRoot(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlinks need elevated privileges on windows")
	}
	root := t.TempDir()
	outside := t.TempDir()
	link := filepath.Join(root, "escape")
	if err := os.Symlink(outside, link); err != nil {
		t.Fatal(err)
	}

	if _, err := confinePath(root, link); err == nil {
		t.Error("expected a symlink escaping root to be rejected")
	}
}

func TestConfinePath_AllowsNewNestedFileNotYetCreated(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "newdir", "nested", "newfile.txt")
	if _, err := confinePath(root, target); err != nil {
		t.Errorf("expected a not-yet-created nested path inside root to be allowed, got: %v", err)
	}
}

func TestExecute_WriteFile_RejectsOutsideRoot(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)
	outside := t.TempDir()
	targetPath := filepath.Join(outside, "pwned.txt")

	args, _ := json.Marshal(map[string]any{"path": targetPath, "content": "pwned"})
	_, err := Execute("write_file", string(args))
	if err == nil {
		t.Fatal("expected write_file to reject a path outside the workspace root")
	}
	if _, statErr := os.Stat(targetPath); statErr == nil {
		t.Error("write_file must not have created the file outside the workspace root")
	}
}

func TestExecute_ReadFile_RejectsOutsideRoot(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)
	outside := t.TempDir()
	secret := filepath.Join(outside, "secret.txt")
	if err := os.WriteFile(secret, []byte("top secret"), 0o644); err != nil {
		t.Fatal(err)
	}

	args, _ := json.Marshal(map[string]any{"path": secret})
	if _, err := Execute("read_file", string(args)); err == nil {
		t.Fatal("expected read_file to reject a path outside the workspace root")
	}
}

func TestExecute_Task_RejectsWorkingDirectoryOutsideRoot(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)
	outside := t.TempDir()

	args, _ := json.Marshal(map[string]any{"prompt": "do something", "working_directory": outside})
	_, err := Execute("task", string(args))
	if err == nil {
		t.Fatal("expected task to reject a working_directory outside the workspace root")
	}
}
