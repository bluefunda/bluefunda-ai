package memory

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// testManager builds a Manager with independent, isolated project and user
// directories under t.TempDir() — no dependency on the real $HOME.
func testManager(t *testing.T) (*Manager, string, string) {
	t.Helper()
	root := t.TempDir()
	projectDir := filepath.Join(root, "project")
	userDir := filepath.Join(root, "user")
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(userDir, 0o755); err != nil {
		t.Fatal(err)
	}
	m := &Manager{
		ProjectDir: filepath.Join(projectDir, ".bai", "memory"),
		UserDir:    filepath.Join(userDir, ".bai", "memory"),
	}
	return m, projectDir, userDir
}

func TestValidateKey(t *testing.T) {
	cases := []struct {
		key     string
		wantErr bool
	}{
		{"conventions", false},
		{"known-bugs", false},
		{"", true},
		{"../escape", true},
		{"a/b", true},
		{"..", true},
	}
	for _, c := range cases {
		err := ValidateKey(c.key)
		if (err != nil) != c.wantErr {
			t.Errorf("ValidateKey(%q) error = %v, wantErr %v", c.key, err, c.wantErr)
		}
	}
}

func TestWriteThenRead(t *testing.T) {
	m, _, _ := testManager(t)

	e, err := m.Write("conventions", "always run golangci-lint before committing")
	if err != nil {
		t.Fatalf("Write: %v", err)
	}
	if e.Scope != ScopeProject {
		t.Errorf("Write scope = %q, want %q", e.Scope, ScopeProject)
	}
	if _, err := os.Stat(e.Path); err != nil {
		t.Errorf("expected file at %s: %v", e.Path, err)
	}

	got, err := m.Read("conventions")
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if got.Content != "always run golangci-lint before committing" {
		t.Errorf("Read content = %q", got.Content)
	}
	if got.Scope != ScopeProject {
		t.Errorf("Read scope = %q, want %q", got.Scope, ScopeProject)
	}
}

func TestWrite_Overwrite(t *testing.T) {
	m, _, _ := testManager(t)

	if _, err := m.Write("known-bugs", "bug 1"); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if _, err := m.Write("known-bugs", "bug 1, bug 2"); err != nil {
		t.Fatalf("Write (overwrite): %v", err)
	}

	got, err := m.Read("known-bugs")
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if got.Content != "bug 1, bug 2" {
		t.Errorf("Read content = %q, want overwritten content", got.Content)
	}
}

func TestWrite_InvalidKey(t *testing.T) {
	m, _, _ := testManager(t)
	if _, err := m.Write("../escape", "content"); err == nil {
		t.Error("expected error writing invalid key, got nil")
	}
}

func TestWrite_NoProjectDir(t *testing.T) {
	m := &Manager{}
	if _, err := m.Write("key", "content"); err == nil {
		t.Error("expected error writing with no project dir, got nil")
	}
}

func TestRead_NotFound(t *testing.T) {
	m, _, _ := testManager(t)
	if _, err := m.Read("nonexistent"); err == nil {
		t.Error("expected error reading nonexistent key, got nil")
	}
}

func TestRead_UserScopeFallback(t *testing.T) {
	m, _, _ := testManager(t)
	if err := os.MkdirAll(m.UserDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(m.UserDir, "prefs.md"), []byte("user prefs"), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := m.Read("prefs")
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if got.Content != "user prefs" || got.Scope != ScopeUser {
		t.Errorf("Read = %+v, want user-scoped 'user prefs'", got)
	}
}

func TestRead_ProjectTakesPrecedenceOverUser(t *testing.T) {
	m, _, _ := testManager(t)
	if err := os.MkdirAll(m.UserDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(m.UserDir, "stack.md"), []byte("user version"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := m.Write("stack", "project version"); err != nil {
		t.Fatalf("Write: %v", err)
	}

	got, err := m.Read("stack")
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if got.Content != "project version" || got.Scope != ScopeProject {
		t.Errorf("Read = %+v, want project-scoped 'project version'", got)
	}
}

func TestDelete(t *testing.T) {
	m, _, _ := testManager(t)
	e, err := m.Write("temp", "temporary note")
	if err != nil {
		t.Fatalf("Write: %v", err)
	}

	if err := m.Delete("temp"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := os.Stat(e.Path); !os.IsNotExist(err) {
		t.Errorf("expected file removed, stat err = %v", err)
	}
	if _, err := m.Read("temp"); err == nil {
		t.Error("expected error reading deleted key, got nil")
	}
}

func TestDelete_NotFound(t *testing.T) {
	m, _, _ := testManager(t)
	if err := m.Delete("nonexistent"); err == nil {
		t.Error("expected error deleting nonexistent key, got nil")
	}
}

func TestDelete_DoesNotTouchUserScope(t *testing.T) {
	m, _, _ := testManager(t)
	if err := os.MkdirAll(m.UserDir, 0o755); err != nil {
		t.Fatal(err)
	}
	userFile := filepath.Join(m.UserDir, "shared-key.md")
	if err := os.WriteFile(userFile, []byte("user content"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := m.Write("shared-key", "project content"); err != nil {
		t.Fatalf("Write: %v", err)
	}

	if err := m.Delete("shared-key"); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	// The project entry is gone, but the user-scoped file must survive.
	if _, err := os.Stat(userFile); err != nil {
		t.Errorf("expected user-scope file to survive delete, stat err = %v", err)
	}
	got, err := m.Read("shared-key")
	if err != nil {
		t.Fatalf("Read after delete: %v", err)
	}
	if got.Scope != ScopeUser || got.Content != "user content" {
		t.Errorf("Read after delete = %+v, want fallback to user scope", got)
	}
}

func TestList_MergesAndSorts(t *testing.T) {
	m, _, _ := testManager(t)
	if err := os.MkdirAll(m.UserDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(m.UserDir, "zeta.md"), []byte("z"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := m.Write("alpha", "a"); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if _, err := m.Write("beta", "b"); err != nil {
		t.Fatalf("Write: %v", err)
	}

	entries, err := m.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(entries) != 3 {
		t.Fatalf("List returned %d entries, want 3", len(entries))
	}
	keys := []string{entries[0].Key, entries[1].Key, entries[2].Key}
	want := []string{"alpha", "beta", "zeta"}
	for i := range want {
		if keys[i] != want[i] {
			t.Errorf("List()[%d].Key = %q, want %q (full: %v)", i, keys[i], want[i], keys)
		}
	}
}

func TestList_KeyCollisionPrefersProject(t *testing.T) {
	m, _, _ := testManager(t)
	if err := os.MkdirAll(m.UserDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(m.UserDir, "dup.md"), []byte("user"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := m.Write("dup", "project"); err != nil {
		t.Fatalf("Write: %v", err)
	}

	entries, err := m.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("List returned %d entries, want 1 (deduped)", len(entries))
	}
	if entries[0].Scope != ScopeProject || entries[0].Content != "project" {
		t.Errorf("List()[0] = %+v, want project entry to win", entries[0])
	}
}

func TestList_Empty(t *testing.T) {
	m, _, _ := testManager(t)
	entries, err := m.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("List() = %v, want empty", entries)
	}
}

func TestList_MissingDirsAreNotErrors(t *testing.T) {
	m := &Manager{ProjectDir: "/nonexistent/does/not/exist", UserDir: "/also/missing"}
	entries, err := m.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("List() = %v, want empty", entries)
	}
}

func TestIndex_Empty(t *testing.T) {
	m, _, _ := testManager(t)
	idx, err := m.Index()
	if err != nil {
		t.Fatalf("Index: %v", err)
	}
	if idx != "" {
		t.Errorf("Index() = %q, want empty string when no entries", idx)
	}
}

func TestIndex_ListsKeysAndPreviewsNotFullContent(t *testing.T) {
	m, _, _ := testManager(t)
	longSecret := "this line should not appear verbatim in the bounded index output at all"
	if _, err := m.Write("conventions", "Always run golangci-lint before committing.\n"+longSecret); err != nil {
		t.Fatalf("Write: %v", err)
	}

	idx, err := m.Index()
	if err != nil {
		t.Fatalf("Index: %v", err)
	}
	if !strings.Contains(idx, "conventions") {
		t.Errorf("Index() = %q, want it to mention key 'conventions'", idx)
	}
	if !strings.Contains(idx, "Always run golangci-lint before committing.") {
		t.Errorf("Index() = %q, want first-line preview present", idx)
	}
	if strings.Contains(idx, longSecret) {
		t.Errorf("Index() = %q, want second line withheld from bounded index", idx)
	}
}

func TestPreview_TruncatesLongLines(t *testing.T) {
	long := strings.Repeat("x", 200)
	got := preview(long)
	if len(got) <= previewMaxLen || got[len(got)-3:] != "..." {
		t.Errorf("preview(long) = %q, want truncated with ellipsis", got)
	}
}

func TestPreview_SkipsLeadingBlankLines(t *testing.T) {
	got := preview("\n\n  \nfirst real line\nsecond line")
	if got != "first real line" {
		t.Errorf("preview() = %q, want %q", got, "first real line")
	}
}

func TestPreview_Empty(t *testing.T) {
	if got := preview(""); got != "(empty)" {
		t.Errorf("preview(\"\") = %q, want \"(empty)\"", got)
	}
}

func TestWrite_SetsDefaultFrontmatter(t *testing.T) {
	m, _, _ := testManager(t)
	e, err := m.Write("conventions", "run lint first")
	if err != nil {
		t.Fatalf("Write: %v", err)
	}
	if e.Source != SourceAgent {
		t.Errorf("Write Source = %q, want %q", e.Source, SourceAgent)
	}
	if e.Status != StatusActive {
		t.Errorf("Write Status = %q, want %q", e.Status, StatusActive)
	}
	if e.CreatedAt == "" {
		t.Error("Write CreatedAt is empty, want an RFC3339 timestamp")
	}

	raw, err := os.ReadFile(e.Path)
	if err != nil {
		t.Fatalf("read backing file: %v", err)
	}
	if !strings.HasPrefix(string(raw), "---\n") {
		t.Errorf("backing file = %q, want a leading frontmatter block", raw)
	}
}

func TestWrite_RefusesSecretLikeContent(t *testing.T) {
	cases := []struct {
		name    string
		content string
	}{
		{"AWS key", "our staging key is AKIAABCDEFGHIJKLMNOP, do not share it"},
		{"private key block", "-----BEGIN RSA PRIVATE KEY-----\nMIIBogIBAAJ...\n-----END RSA PRIVATE KEY-----"},
		{"GitHub token", "auth with ghp_abcdefghijklmnopqrstuvwxyz0123456789"},
		{"credential assignment", "api_key: sk-live-abcdefghijklmnopqrstuvwx"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			m, _, _ := testManager(t)
			if _, err := m.Write("leak", c.content); err == nil {
				t.Errorf("Write(%q) = nil error, want refusal", c.content)
			}
			if _, err := m.Read("leak"); err == nil {
				t.Error("expected no file written after refused secret-like content")
			}
		})
	}
}

func TestWrite_OrdinaryContentNotFlaggedAsSecret(t *testing.T) {
	m, _, _ := testManager(t)
	content := "The access token is refreshed automatically; see auth.go for the retry policy."
	if _, err := m.Write("notes", content); err != nil {
		t.Fatalf("Write: %v", err)
	}
}

func TestReadEntry_PreFrontmatterFileDefaultsToActiveAgent(t *testing.T) {
	m, _, _ := testManager(t)
	if err := os.MkdirAll(m.ProjectDir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(m.ProjectDir, "legacy.md")
	if err := os.WriteFile(path, []byte("written before frontmatter existed"), 0o644); err != nil {
		t.Fatal(err)
	}

	e, err := m.Read("legacy")
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if e.Content != "written before frontmatter existed" {
		t.Errorf("Read Content = %q, want the raw body unchanged", e.Content)
	}
	if e.Source != SourceAgent || e.Status != StatusActive {
		t.Errorf("Read = %+v, want default Source=agent Status=active", e)
	}
}

func TestReadEntry_IncidentalLeadingDashesNotMistakenForFrontmatter(t *testing.T) {
	m, _, _ := testManager(t)
	if err := os.MkdirAll(m.ProjectDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// A Markdown horizontal-rule-style note that happens to start with "---"
	// but isn't a YAML mapping with any known frontmatter field.
	body := "---\nnote: just a title line, not real frontmatter\n---\nactual content"
	path := filepath.Join(m.ProjectDir, "odd.md")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}

	e, err := m.Read("odd")
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if e.Content != body {
		t.Errorf("Read Content = %q, want the whole file treated as body (no known frontmatter fields)", e.Content)
	}
}

func TestSupersede_ExcludedFromListAndIndexButReadable(t *testing.T) {
	m, _, _ := testManager(t)
	if _, err := m.Write("old-stack", "Go 1.20, gRPC"); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if _, err := m.Write("new-stack", "Go 1.26, gRPC"); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if err := m.Supersede("old-stack"); err != nil {
		t.Fatalf("Supersede: %v", err)
	}

	entries, err := m.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	for _, e := range entries {
		if e.Key == "old-stack" {
			t.Errorf("List() includes superseded key %q, want it excluded", e.Key)
		}
	}

	idx, err := m.Index()
	if err != nil {
		t.Fatalf("Index: %v", err)
	}
	if strings.Contains(idx, "old-stack") {
		t.Errorf("Index() = %q, want superseded key excluded", idx)
	}

	// Still readable directly, and still present in ListAll.
	e, err := m.Read("old-stack")
	if err != nil {
		t.Fatalf("Read superseded entry: %v", err)
	}
	if e.Status != StatusSuperseded || e.Content != "Go 1.20, gRPC" {
		t.Errorf("Read(old-stack) = %+v, want Status=superseded with content preserved", e)
	}

	all, err := m.ListAll()
	if err != nil {
		t.Fatalf("ListAll: %v", err)
	}
	found := false
	for _, e := range all {
		if e.Key == "old-stack" {
			found = true
		}
	}
	if !found {
		t.Error("ListAll() does not include superseded key 'old-stack'")
	}
}

func TestSupersede_NotFound(t *testing.T) {
	m, _, _ := testManager(t)
	if err := m.Supersede("nonexistent"); err == nil {
		t.Error("expected error superseding nonexistent key, got nil")
	}
}
