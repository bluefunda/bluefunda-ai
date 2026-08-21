package cmd

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseFrontmatter_DescriptionAndBody(t *testing.T) {
	content := "---\ndescription: Code review this file\n---\nReview $ARGUMENTS for correctness."
	desc, body := parseFrontmatter(content)
	if desc != "Code review this file" {
		t.Errorf("description = %q", desc)
	}
	if body != "Review $ARGUMENTS for correctness." {
		t.Errorf("body = %q", body)
	}
}

func TestParseFrontmatter_NoFrontmatter(t *testing.T) {
	content := "Just a plain prompt, no frontmatter."
	desc, body := parseFrontmatter(content)
	if desc != "" {
		t.Errorf("description = %q, want empty", desc)
	}
	if body != content {
		t.Errorf("body = %q, want the whole content unchanged", body)
	}
}

func TestParseFrontmatter_UnterminatedBlockTreatedAsBody(t *testing.T) {
	content := "---\ndescription: oops, no closing delimiter\nstill going"
	desc, body := parseFrontmatter(content)
	if desc != "" {
		t.Errorf("description = %q, want empty for unterminated frontmatter", desc)
	}
	if body != content {
		t.Errorf("body = %q, want the whole content unchanged", body)
	}
}

func writeCommandFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestLoadCustomSlashCommands_ProjectOnly(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", t.TempDir())
	writeCommandFile(t, filepath.Join(dir, ".bai", "commands"), "review.md",
		"---\ndescription: Code review this file\n---\nReview $ARGUMENTS for correctness.")

	cmds := loadCustomSlashCommands(dir)
	if len(cmds) != 1 {
		t.Fatalf("loadCustomSlashCommands returned %d commands, want 1: %+v", len(cmds), cmds)
	}
	if cmds[0].Name != "/review" || cmds[0].Description != "Code review this file" {
		t.Errorf("cmds[0] = %+v", cmds[0])
	}
	if cmds[0].Prompt != "Review $ARGUMENTS for correctness." {
		t.Errorf("cmds[0].Prompt = %q", cmds[0].Prompt)
	}
}

func TestLoadCustomSlashCommands_UserOnly(t *testing.T) {
	dir := t.TempDir()
	home := t.TempDir()
	t.Setenv("HOME", home)
	writeCommandFile(t, filepath.Join(home, ".bai", "commands"), "standup.md", "Summarize yesterday's commits.")

	cmds := loadCustomSlashCommands(dir)
	if len(cmds) != 1 {
		t.Fatalf("loadCustomSlashCommands returned %d commands, want 1: %+v", len(cmds), cmds)
	}
	if cmds[0].Name != "/standup" {
		t.Errorf("cmds[0].Name = %q", cmds[0].Name)
	}
	if cmds[0].Description != "custom command" {
		t.Errorf("cmds[0].Description = %q, want default when no frontmatter", cmds[0].Description)
	}
}

func TestLoadCustomSlashCommands_ProjectOverridesUserOnCollision(t *testing.T) {
	dir := t.TempDir()
	home := t.TempDir()
	t.Setenv("HOME", home)
	writeCommandFile(t, filepath.Join(home, ".bai", "commands"), "review.md", "user-level review prompt")
	writeCommandFile(t, filepath.Join(dir, ".bai", "commands"), "review.md", "project-level review prompt")

	cmds := loadCustomSlashCommands(dir)
	if len(cmds) != 1 {
		t.Fatalf("loadCustomSlashCommands returned %d commands, want 1 (deduped): %+v", len(cmds), cmds)
	}
	if cmds[0].Prompt != "project-level review prompt" {
		t.Errorf("cmds[0].Prompt = %q, want project entry to win", cmds[0].Prompt)
	}
}

func TestLoadCustomSlashCommands_MergesDistinctNames(t *testing.T) {
	dir := t.TempDir()
	home := t.TempDir()
	t.Setenv("HOME", home)
	writeCommandFile(t, filepath.Join(home, ".bai", "commands"), "standup.md", "user command")
	writeCommandFile(t, filepath.Join(dir, ".bai", "commands"), "review.md", "project command")

	cmds := loadCustomSlashCommands(dir)
	if len(cmds) != 2 {
		t.Fatalf("loadCustomSlashCommands returned %d commands, want 2: %+v", len(cmds), cmds)
	}
	// Sorted by name: /review before /standup.
	if cmds[0].Name != "/review" || cmds[1].Name != "/standup" {
		t.Errorf("cmds = %+v, want sorted [/review, /standup]", cmds)
	}
}

func TestLoadCustomSlashCommands_None(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", t.TempDir())
	if cmds := loadCustomSlashCommands(dir); cmds != nil {
		t.Errorf("loadCustomSlashCommands = %+v, want nil with no command files", cmds)
	}
}

func TestLoadCustomSlashCommands_IgnoresNonMarkdownAndEmptyBody(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", t.TempDir())
	commandsDir := filepath.Join(dir, ".bai", "commands")
	writeCommandFile(t, commandsDir, "notes.txt", "not markdown, should be ignored")
	writeCommandFile(t, commandsDir, "empty.md", "---\ndescription: has no body\n---\n")

	cmds := loadCustomSlashCommands(dir)
	if len(cmds) != 0 {
		t.Errorf("loadCustomSlashCommands = %+v, want empty (non-.md skipped, empty body skipped)", cmds)
	}
}

func TestLoadCustomSlashCommands_WalksUpToGitRoot(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", t.TempDir())
	if err := os.MkdirAll(filepath.Join(dir, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeCommandFile(t, filepath.Join(dir, ".bai", "commands"), "review.md", "review prompt")

	sub := filepath.Join(dir, "internal", "cmd")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}

	cmds := loadCustomSlashCommands(sub)
	if len(cmds) != 1 || cmds[0].Name != "/review" {
		t.Errorf("loadCustomSlashCommands(subdir) = %+v, want /review found by walking up to git root", cmds)
	}
}
