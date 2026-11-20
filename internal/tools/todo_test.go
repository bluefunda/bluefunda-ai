package tools

import (
	"strings"
	"testing"
)

func TestTodoWrite(t *testing.T) {
	t.Cleanup(func() { todoList = nil })
	out, err := Execute("todo_write", `{"todos":[
		{"content":"Read code","status":"completed"},
		{"content":"Fix bug","status":"in_progress","activeForm":"Fixing bug"},
		{"content":"Run tests","status":"pending"}]}`)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "1 completed, 1 in progress, 1 pending") {
		t.Errorf("out = %q", out)
	}
	if got := Todos(); len(got) != 3 || got[1].ActiveForm != "Fixing bug" {
		t.Errorf("stored = %+v", got)
	}
	// Each call replaces the list.
	if _, err := TodoWrite(`{"todos":[{"content":"Only","status":"pending"}]}`); err != nil || len(Todos()) != 1 {
		t.Errorf("replace: %v, %+v", err, Todos())
	}
	if out, _ := TodoWrite(`{"todos":[{"content":"a","status":"in_progress"},{"content":"b","status":"in_progress"}]}`); !strings.Contains(out, "only one task in progress") {
		t.Errorf("two in progress should be flagged: %q", out)
	}
}

func TestTodoWrite_Invalid(t *testing.T) {
	t.Cleanup(func() { todoList = nil })
	_, _ = TodoWrite(`{"todos":[{"content":"keep","status":"pending"}]}`)
	for _, bad := range []string{
		`{"todos":[{"content":"","status":"pending"}]}`,
		`{"todos":[{"content":"x","status":"done"}]}`,
		`not json`,
	} {
		if _, err := TodoWrite(bad); err == nil {
			t.Errorf("accepted %s", bad)
		}
	}
	if got := Todos(); len(got) != 1 || got[0].Content != "keep" {
		t.Errorf("a rejected call must not change the list: %+v", got)
	}
	if !IsReadOnlyTool("todo_write") {
		t.Error("todo_write changes no files; it should never ask for approval")
	}
}
