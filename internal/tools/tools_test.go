package tools

import "testing"

func TestIsReadOnlyTool(t *testing.T) {
	cases := map[string]bool{
		"read_file":      true,
		"list_dir":       true,
		"search_files":   true,
		"search_content": true,
		"read_notebook":  true,
		"web_fetch":      true,
		"web_search":     true,
		"memory_read":    true,
		"memory_list":    true,
		"write_file":     false,
		"edit_file":      false,
		"patch_file":     false,
		"edit_notebook":  false,
		"bash":           false,
		"task":           false,
		"memory_write":   false,
		"memory_delete":  false,
		"unknown_tool":   false,
	}
	for tool, want := range cases {
		if got := IsReadOnlyTool(tool); got != want {
			t.Errorf("IsReadOnlyTool(%q) = %v, want %v", tool, got, want)
		}
	}
}
