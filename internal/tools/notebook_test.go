package tools

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const sampleNotebook = `{
 "cells": [
  {
   "cell_type": "markdown",
   "id": "abc12345",
   "metadata": {},
   "source": ["# Demo Notebook\n", "Some intro text."]
  },
  {
   "cell_type": "code",
   "execution_count": 3,
   "id": "def67890",
   "metadata": {"tags": ["keep-me"]},
   "outputs": [
    {"name": "stdout", "output_type": "stream", "text": ["hello\n", "world\n"]},
    {"data": {"text/plain": ["42"]}, "execution_count": 3, "metadata": {}, "output_type": "execute_result"}
   ],
   "source": ["print('hello')\n", "print('world')\n", "40 + 2"]
  }
 ],
 "metadata": {
  "kernelspec": {"display_name": "Python 3", "language": "python", "name": "python3"},
  "language_info": {"name": "python", "version": "3.11.0"}
 },
 "nbformat": 4,
 "nbformat_minor": 5
}`

func writeSampleNotebook(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "sample.ipynb")
	if err := os.WriteFile(path, []byte(sampleNotebook), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestReadNotebook_RendersCellsAndOutputSummary(t *testing.T) {
	path := writeSampleNotebook(t)
	got, err := ReadNotebook(path)
	if err != nil {
		t.Fatalf("ReadNotebook: %v", err)
	}
	for _, want := range []string{
		"language: python",
		"[0] markdown",
		"# Demo Notebook",
		"[1] code (executed [3])",
		"print('hello')",
		"--- output ---",
		"hello",
		"world",
		"42",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("ReadNotebook output missing %q:\n%s", want, got)
		}
	}
}

func TestReadNotebook_NotExecutedCellHasNoExecutionCount(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nb.ipynb")
	raw := `{"cells":[{"cell_type":"code","id":"x","metadata":{},"outputs":[],"execution_count":null,"source":["1+1"]}],"metadata":{},"nbformat":4,"nbformat_minor":5}`
	if err := os.WriteFile(path, []byte(raw), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := ReadNotebook(path)
	if err != nil {
		t.Fatalf("ReadNotebook: %v", err)
	}
	if !strings.Contains(got, "[0] code (not executed)") {
		t.Errorf("ReadNotebook = %q, want 'not executed' state", got)
	}
}

func TestReadNotebook_NotFound(t *testing.T) {
	if _, err := ReadNotebook(filepath.Join(t.TempDir(), "missing.ipynb")); err == nil {
		t.Error("expected error for missing file, got nil")
	}
}

func TestReadNotebook_NotJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad.ipynb")
	if err := os.WriteFile(path, []byte("not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadNotebook(path); err == nil {
		t.Error("expected error for non-JSON file, got nil")
	}
}

func TestEditNotebook_ReplaceClearsOutputsAndPreservesUnknownFields(t *testing.T) {
	path := writeSampleNotebook(t)
	if _, err := EditNotebook(path, 1, "replace", "print('changed')", ""); err != nil {
		t.Fatalf("EditNotebook: %v", err)
	}

	got, err := ReadNotebook(path)
	if err != nil {
		t.Fatalf("ReadNotebook: %v", err)
	}
	if !strings.Contains(got, "print('changed')") {
		t.Errorf("ReadNotebook = %q, want replaced source", got)
	}
	if strings.Contains(got, "--- output ---") {
		t.Errorf("ReadNotebook = %q, want outputs cleared after replace", got)
	}
	if !strings.Contains(got, "[1] code (not executed)") {
		t.Errorf("ReadNotebook = %q, want execution_count cleared after replace", got)
	}

	// Unknown fields (id, tags) must survive the edit untouched.
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]json.RawMessage
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	var cells []map[string]json.RawMessage
	if err := json.Unmarshal(doc["cells"], &cells); err != nil {
		t.Fatal(err)
	}
	if string(cells[1]["id"]) != `"def67890"` {
		t.Errorf("cell id = %s, want it preserved across the edit", cells[1]["id"])
	}
	if !strings.Contains(string(cells[1]["metadata"]), "keep-me") {
		t.Errorf("cell metadata = %s, want tags preserved across the edit", cells[1]["metadata"])
	}
}

func TestEditNotebook_ReplaceOutOfRange(t *testing.T) {
	path := writeSampleNotebook(t)
	if _, err := EditNotebook(path, 5, "replace", "x", ""); err == nil {
		t.Error("expected out-of-range error, got nil")
	}
}

func TestEditNotebook_InsertAtBeginning(t *testing.T) {
	path := writeSampleNotebook(t)
	if _, err := EditNotebook(path, 0, "insert", "# new first cell", "markdown"); err != nil {
		t.Fatalf("EditNotebook: %v", err)
	}
	got, err := ReadNotebook(path)
	if err != nil {
		t.Fatalf("ReadNotebook: %v", err)
	}
	if !strings.Contains(got, "(3 cells") {
		t.Errorf("ReadNotebook = %q, want 3 cells after insert", got)
	}
	if !strings.Contains(got, "[0] markdown\n    # new first cell") {
		t.Errorf("ReadNotebook = %q, want new cell inserted at index 0", got)
	}
	if !strings.Contains(got, "[1] markdown\n    # Demo Notebook") {
		t.Errorf("ReadNotebook = %q, want original cell 0 shifted to index 1", got)
	}
}

func TestEditNotebook_InsertAtEndDefaultsToCode(t *testing.T) {
	path := writeSampleNotebook(t)
	if _, err := EditNotebook(path, 2, "insert", "x = 1", ""); err != nil {
		t.Fatalf("EditNotebook: %v", err)
	}
	got, err := ReadNotebook(path)
	if err != nil {
		t.Fatalf("ReadNotebook: %v", err)
	}
	if !strings.Contains(got, "[2] code (not executed)\n    x = 1") {
		t.Errorf("ReadNotebook = %q, want new code cell appended at index 2", got)
	}
}

func TestEditNotebook_InsertOutOfRange(t *testing.T) {
	path := writeSampleNotebook(t)
	if _, err := EditNotebook(path, 99, "insert", "x", ""); err == nil {
		t.Error("expected out-of-range error, got nil")
	}
}

func TestEditNotebook_Delete(t *testing.T) {
	path := writeSampleNotebook(t)
	if _, err := EditNotebook(path, 0, "delete", "", ""); err != nil {
		t.Fatalf("EditNotebook: %v", err)
	}
	got, err := ReadNotebook(path)
	if err != nil {
		t.Fatalf("ReadNotebook: %v", err)
	}
	if !strings.Contains(got, "(1 cells") {
		t.Errorf("ReadNotebook = %q, want 1 cell remaining after delete", got)
	}
	if strings.Contains(got, "Demo Notebook") {
		t.Errorf("ReadNotebook = %q, want deleted cell gone", got)
	}
}

func TestEditNotebook_ClearOutputs(t *testing.T) {
	path := writeSampleNotebook(t)
	if _, err := EditNotebook(path, 1, "clear_outputs", "", ""); err != nil {
		t.Fatalf("EditNotebook: %v", err)
	}
	got, err := ReadNotebook(path)
	if err != nil {
		t.Fatalf("ReadNotebook: %v", err)
	}
	if strings.Contains(got, "--- output ---") {
		t.Errorf("ReadNotebook = %q, want outputs cleared", got)
	}
	if !strings.Contains(got, "print('hello')") {
		t.Errorf("ReadNotebook = %q, want source untouched by clear_outputs", got)
	}
}

func TestEditNotebook_UnknownEditMode(t *testing.T) {
	path := writeSampleNotebook(t)
	if _, err := EditNotebook(path, 0, "frobnicate", "x", ""); err == nil {
		t.Error("expected error for unknown edit_mode, got nil")
	}
}

func TestEditNotebook_NotFound(t *testing.T) {
	if _, err := EditNotebook(filepath.Join(t.TempDir(), "missing.ipynb"), 0, "replace", "x", ""); err == nil {
		t.Error("expected error for missing file, got nil")
	}
}

func TestExecute_NotebookTools(t *testing.T) {
	path := writeSampleNotebook(t)

	readArgs, _ := json.Marshal(map[string]any{"path": path})
	got, err := Execute("read_notebook", string(readArgs))
	if err != nil {
		t.Fatalf("Execute read_notebook: %v", err)
	}
	if !strings.Contains(got, "Demo Notebook") {
		t.Errorf("Execute read_notebook = %q", got)
	}

	editArgs, _ := json.Marshal(map[string]any{
		"path": path, "cell_index": 1, "edit_mode": "clear_outputs",
	})
	if _, err := Execute("edit_notebook", string(editArgs)); err != nil {
		t.Fatalf("Execute edit_notebook: %v", err)
	}
	got, err = Execute("read_notebook", string(readArgs))
	if err != nil {
		t.Fatalf("Execute read_notebook: %v", err)
	}
	if strings.Contains(got, "--- output ---") {
		t.Errorf("Execute read_notebook after clear_outputs = %q, want outputs cleared", got)
	}
}

func TestNeedsApproval_NotebookTools(t *testing.T) {
	if NeedsApproval("read_notebook") {
		t.Error("read_notebook should not need approval")
	}
	if NeedsApproval("edit_notebook") {
		t.Error("edit_notebook should not need approval, matching edit_file/patch_file")
	}
}

func TestLocalToolSchemas_IncludesNotebookTools(t *testing.T) {
	schemasJSON, err := LocalToolSchemas()
	if err != nil {
		t.Fatalf("LocalToolSchemas: %v", err)
	}
	for _, name := range []string{"read_notebook", "edit_notebook"} {
		if !strings.Contains(schemasJSON, `"`+name+`"`) {
			t.Errorf("LocalToolSchemas missing tool %q", name)
		}
	}
}
