package cmd

import (
	"path/filepath"
	"reflect"
	"testing"

	"github.com/bluefunda/bluefunda-ai/internal/config"
)

func bashArgs(cmd string) string { return `{"command":` + jsonString(cmd) + `}` }

func jsonString(s string) string {
	b := []byte{'"'}
	for _, r := range s {
		switch r {
		case '"', '\\':
			b = append(b, '\\', byte(r))
		default:
			b = append(b, string(r)...)
		}
	}
	return string(append(b, '"'))
}

// "Don't ask again" on one python3 call covers later, different python3
// calls — the case that made a real session ask 16 times — but not a chained
// command it never saw.
func TestSessionAllow_BashPrefixGeneralises(t *testing.T) {
	s := newSessionAllowSet()
	s.add("bash", bashArgs(`cd /tmp && python3 -c "import json; print(1)"`))
	if !s.contains("bash", bashArgs(`cd /tmp && python3 -c "print(2)" | head`)) {
		t.Error("a later python3 command should be covered")
	}
	if s.contains("bash", bashArgs(`python3 -c "x"; rm -rf ~`)) {
		t.Error("a chained rm must not be covered")
	}
	if s.contains("bash", bashArgs("node x.js")) {
		t.Error("a different program must not be covered")
	}
}

func TestSessionAllow_EditToolsAllowedTogether(t *testing.T) {
	s := newSessionAllowSet()
	s.add("edit_file", `{"path":"a.go","old_string":"x","new_string":"y"}`)
	for _, tool := range []string{"edit_file", "write_file", "patch_file", "edit_notebook"} {
		if !s.contains(tool, `{"path":"other.go"}`) {
			t.Errorf("%s should be allowed after allowing file edits", tool)
		}
	}
	if s.contains("bash", bashArgs("rm x")) {
		t.Error("allowing edits must not allow bash")
	}
}

func TestSessionAllow_OtherToolsByName(t *testing.T) {
	s := newSessionAllowSet()
	s.add("mcp__github__create_issue", `{"title":"a"}`)
	if !s.contains("mcp__github__create_issue", `{"title":"b"}`) {
		t.Error("the same MCP tool with other args should be covered")
	}
	if s.contains("mcp__github__delete_repo", `{}`) {
		t.Error("another MCP tool must not be covered")
	}
}

func TestSessionAllow_UngeneralisableFallsBackToExact(t *testing.T) {
	s := newSessionAllowSet()
	cmd := "echo $(date) > stamp.txt"
	s.add("bash", bashArgs(cmd))
	if !s.contains("bash", bashArgs(cmd)) {
		t.Error("the exact command should be remembered")
	}
	if s.contains("bash", bashArgs("echo $(id) > other.txt")) {
		t.Error("only the exact command should be remembered")
	}
}

func TestAlwaysScopeLabels(t *testing.T) {
	cases := map[[2]string]string{
		{"bash", bashArgs("gh pr create && rm -rf build")}: "`gh pr`, `rm` commands",
		{"bash", bashArgs("echo $(id)")}:                   "",
		{"write_file", `{"path":"a"}`}:                     "file edits",
		{"mcp__jira__create", `{}`}:                        "mcp__jira__create",
	}
	for in, want := range cases {
		if _, _, got := alwaysScope(in[0], in[1]); got != want {
			t.Errorf("alwaysScope(%s, %s) label = %q, want %q", in[0], in[1], got, want)
		}
	}
}

func TestSessionAllow_PersistsGeneralisedRules(t *testing.T) {
	dir := t.TempDir()
	s := newSessionAllowSet()
	s.persistPath = filepath.Join(dir, ".bai", "settings.local.yaml")
	s.add("bash", bashArgs("gh pr create --title x"))
	s.add("edit_file", `{"path":"a.go"}`)

	lc, _ := config.FindLocalConfig(dir)
	if lc == nil {
		t.Fatal("expected local config to be created")
	}
	want := []string{"bash:gh pr *", "write_file", "edit_file", "patch_file", "edit_notebook"}
	if !reflect.DeepEqual(lc.Permissions.Allow, want) {
		t.Errorf("Permissions.Allow = %q, want %q", lc.Permissions.Allow, want)
	}
}
