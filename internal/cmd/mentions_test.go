package cmd

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// chdirTemp runs the test in a fresh directory, which is also the workspace
// root the mention paths are confined to.
func chdirTemp(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Chdir(dir)
	return dir
}

func TestExpandMentions(t *testing.T) {
	chdirTemp(t)
	must := func(err error) {
		if err != nil {
			t.Fatal(err)
		}
	}
	must(os.MkdirAll("pkg/sub", 0o755))
	must(os.WriteFile("pkg/a.go", []byte("package pkg\n```\n"), 0o644))
	must(os.WriteFile("bin.dat", []byte{1, 0, 2}, 0o644))

	got := expandMentions("explain @pkg/a.go, and @pkg/ and @bin.dat and @missing.go and email me@host")
	if !strings.HasPrefix(got, "explain @pkg/a.go, and") {
		t.Errorf("the message itself must come first, unchanged:\n%s", got)
	}
	for _, want := range []string{
		"Contents of @pkg/a.go:\n````\npackage pkg\n```\n````", // fence outlasts the file's own
		"Contents of directory @pkg/:\n```\na.go\nsub/\n```",
		"@bin.dat is a binary file",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("lacks %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "missing.go:") || strings.Contains(got, "host:") {
		t.Errorf("non-paths must stay plain text:\n%s", got)
	}
	if expandMentions("no mentions here") != "no mentions here" {
		t.Error("a message without mentions must be unchanged")
	}
	// The same file twice is attached once.
	if n := strings.Count(expandMentions("@pkg/a.go @pkg/a.go"), "Contents of"); n != 1 {
		t.Errorf("attached %d times", n)
	}
}

// A mention can't read outside the workspace.
func TestExpandMentions_ConfinedToWorkspace(t *testing.T) {
	outside := t.TempDir()
	secret := filepath.Join(outside, "secret.txt")
	if err := os.WriteFile(secret, []byte("s3cret"), 0o644); err != nil {
		t.Fatal(err)
	}
	chdirTemp(t)
	if got := expandMentions("@" + secret + " @../" + filepath.Base(outside) + "/secret.txt"); strings.Contains(got, "s3cret") {
		t.Errorf("read a file outside the workspace:\n%s", got)
	}
}

func TestMentionCandidates_Walk(t *testing.T) {
	chdirTemp(t) // not a git repository: the walk is used
	for _, p := range []string{"a/b.go", ".git/x", "node_modules/m.js", "c.md"} {
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	got := mentionCandidates()
	if !slices.Equal(got, []string{"a/", "a/b.go", "c.md"}) {
		t.Errorf("candidates = %q", got)
	}
}
