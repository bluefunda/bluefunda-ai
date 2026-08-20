package tui

import "testing"

func TestExpandArguments_SubstitutesPlaceholder(t *testing.T) {
	got := expandArguments("Review $ARGUMENTS for correctness.", "internal/cmd/code.go")
	want := "Review internal/cmd/code.go for correctness."
	if got != want {
		t.Errorf("expandArguments = %q, want %q", got, want)
	}
}

func TestExpandArguments_NoPlaceholderLeavesPromptUnchanged(t *testing.T) {
	got := expandArguments("Summarize yesterday's commits.", "some args")
	want := "Summarize yesterday's commits."
	if got != want {
		t.Errorf("expandArguments = %q, want %q", got, want)
	}
}

func TestExpandArguments_EmptyArgsClearsPlaceholder(t *testing.T) {
	got := expandArguments("Review $ARGUMENTS.", "")
	want := "Review ."
	if got != want {
		t.Errorf("expandArguments = %q, want %q", got, want)
	}
}

func TestMatchSlashCommands_IncludesCustomCommands(t *testing.T) {
	custom := []SlashCommand{{Name: "/review", Description: "Code review this file"}}
	matches := matchSlashCommands("/rev", custom)
	found := false
	for _, m := range matches {
		if m.Name == "/review" {
			found = true
		}
	}
	if !found {
		t.Errorf("matchSlashCommands(%q) = %+v, want /review included", "/rev", matches)
	}
}
