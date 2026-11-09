package commands

import (
	"bytes"
	"strings"
	"testing"
)

func TestPrintObjectList_UsesTypeAndNameKeys(t *testing.T) {
	// Regression test: the API returns "type"/"name" keys (see search.go and
	// rest/server ADTObject json tags), not "object_type"/"object_name".
	var buf bytes.Buffer
	objects := []map[string]any{
		{"type": "PROG/P", "name": "ZHELLO_WORLD", "description": "Hello World"},
	}

	if err := printObjectList(&buf, objects, "text", "No objects found."); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := buf.String()
	if !strings.Contains(got, "PROG/P") || !strings.Contains(got, "ZHELLO_WORLD") {
		t.Errorf("expected type and name in output, got %q", got)
	}
}

func TestPrintObjectList_IgnoresLegacyKeys(t *testing.T) {
	var buf bytes.Buffer
	objects := []map[string]any{
		{"object_type": "PROG/P", "object_name": "ZHELLO_WORLD"},
	}

	if err := printObjectList(&buf, objects, "text", "No objects found."); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got := buf.String(); got != "\n" {
		t.Errorf("expected blank line for unrecognized keys (guards against reintroducing the old bug), got %q", got)
	}
}

func TestPrintObjectList_EmptyShowsMessage(t *testing.T) {
	var buf bytes.Buffer
	if err := printObjectList(&buf, nil, "text", "No objects found."); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := strings.TrimSpace(buf.String()); got != "No objects found." {
		t.Errorf("expected empty message, got %q", got)
	}
}
