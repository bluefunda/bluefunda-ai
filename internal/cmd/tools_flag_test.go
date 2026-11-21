package cmd

import (
	"slices"
	"strings"
	"testing"

	"github.com/bluefunda/bluefunda-ai/internal/tools"
)

func TestRestrictTools(t *testing.T) {
	all, err := tools.LocalToolSchemas()
	if err != nil {
		t.Fatal(err)
	}
	if got, dropped, err := restrictTools(all, nil); err != nil || got != all || dropped != nil {
		t.Errorf("no --tools should leave schemas unchanged")
	}
	got, dropped, err := restrictTools(all, []string{" read_file", "search_content", ""})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(got, `"bash"`) || !strings.Contains(got, `"read_file"`) || !strings.Contains(got, `"search_content"`) {
		t.Errorf("filtered schemas = %s", got)
	}
	// Hidden tools come back so the caller can deny them too.
	if !slices.Contains(dropped, "bash") || !slices.Contains(dropped, "write_file") {
		t.Errorf("dropped = %q", dropped)
	}
	if got, _, _ := restrictTools("", []string{"read_file"}); got != "" {
		t.Errorf("chat mode (no schemas) should stay empty")
	}
}

func TestToolsFlagRegistered(t *testing.T) {
	for _, c := range []string{"root", "code"} {
		fs := rootCmd.Flags()
		if c == "code" {
			fs = codeCmd.Flags()
		}
		if fs.Lookup("tools") == nil {
			t.Errorf("%s command has no --tools flag", c)
		}
	}
}
