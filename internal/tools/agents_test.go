package tools

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/spf13/pflag"
)

func TestParseAgentFile(t *testing.T) {
	cases := []struct {
		name, src string
		wantTools []string
		wantErr   string
	}{
		{"comma tools", "---\nname: reviewer\ndescription: Reviews diffs\ntools: read_file, search_content\nmodel: think\n---\nBe strict.\n",
			[]string{"read_file", "search_content"}, ""},
		{"list tools", "---\nname: reviewer\ndescription: Reviews diffs\ntools:\n  - read_file\n  - bash\n---\nBe strict.",
			[]string{"read_file", "bash"}, ""},
		{"no tools", "---\nname: reviewer\ndescription: Reviews diffs\n---\nBe strict.", nil, ""},
		{"crlf", "---\r\nname: reviewer\r\ndescription: Reviews diffs\r\n---\r\nBe strict.\r\n", nil, ""},
		{"no frontmatter", "name: x\n", nil, "missing --- frontmatter"},
		{"unterminated", "---\nname: x\ndescription: y\n", nil, "unterminated"},
		{"bad name", "---\nname: Bad Name\ndescription: y\n---\n", nil, "must be lowercase"},
		{"no description", "---\nname: x\n---\n", nil, "description is required"},
		{"bad yaml", "---\nname: [x\n---\n", nil, "frontmatter"},
		{"bad tools", "---\nname: x\ndescription: y\ntools: {a: b}\n---\n", nil, "tools"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := ParseAgentFile("a.md", []byte(c.src))
			if c.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), c.wantErr) {
					t.Fatalf("err = %v, want containing %q", err, c.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got.Name != "reviewer" || got.Description != "Reviews diffs" || got.Instructions != "Be strict." {
				t.Errorf("got %+v", got)
			}
			if !reflect.DeepEqual(got.Tools, c.wantTools) {
				t.Errorf("tools = %q, want %q", got.Tools, c.wantTools)
			}
		})
	}
}

func TestLoadAgentTypes_ProjectOverridesUserAndBadFilesSkipped(t *testing.T) {
	user, proj := t.TempDir(), t.TempDir()
	write := func(dir, file, body string) {
		if err := os.WriteFile(filepath.Join(dir, file), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(user, "rev.md", "---\nname: reviewer\ndescription: user version\n---\n")
	write(proj, "rev.md", "---\nname: reviewer\ndescription: project version\n---\n")
	write(proj, "broken.md", "no frontmatter")
	write(proj, "explore.md", "---\nname: explore\ndescription: custom explore\n---\n")

	types, errs := LoadAgentTypes(user, proj, "")
	if len(errs) != 1 || !strings.Contains(errs[0].Error(), "broken.md") {
		t.Errorf("errs = %v, want one for broken.md", errs)
	}
	by := map[string]AgentType{}
	var names []string
	for _, a := range types {
		by[a.Name] = a
		names = append(names, a.Name)
	}
	if want := []string{"explore", "general", "plan", "reviewer"}; !reflect.DeepEqual(names, want) {
		t.Errorf("names = %q, want %q (sorted)", names, want)
	}
	if by["reviewer"].Description != "project version" {
		t.Errorf("reviewer = %q, want the project file to win", by["reviewer"].Description)
	}
	if by["explore"].Description != "custom explore" {
		t.Errorf("a project file should be able to replace a built-in")
	}
}

func TestLookupAgentType(t *testing.T) {
	SetAgentTypes(nil)
	if a, err := lookupAgentType(""); err != nil || a.Name != DefaultAgentType {
		t.Errorf("empty = %+v, %v; want the default type", a, err)
	}
	_, err := lookupAgentType("nope")
	if err == nil || !strings.Contains(err.Error(), "explore") || !strings.Contains(err.Error(), "general") {
		t.Errorf("err = %v, want it to list the valid types", err)
	}
	if _, err := Execute("task", `{"prompt":"x","subagent_type":"nope"}`); err == nil {
		t.Error("task with an unknown subagent_type should fail before spawning")
	}
}

// The explore type runs its child with only read-only tools, its model, and
// its instructions ahead of the task.
func TestTaskArgs_AgentType(t *testing.T) {
	SetTaskDisallowedTools(nil)
	agent := AgentType{Name: "x", Tools: []string{"read_file", "search_content"}, Model: "think", Instructions: "Read only."}
	args := taskArgs(agent, "find foo", 0, false)

	fs := pflag.NewFlagSet("child", pflag.ContinueOnError)
	var only []string
	fs.StringSliceVar(&only, "tools", nil, "")
	model := fs.String("model", "", "")
	fs.Bool("print", false, "")
	fs.String("output-format", "", "")
	fs.Bool("auto", false, "")
	if err := fs.Parse(args); err != nil {
		t.Fatalf("child parse %q: %v", args, err)
	}
	if !reflect.DeepEqual(only, agent.Tools) {
		t.Errorf("--tools = %q, want %q", only, agent.Tools)
	}
	if *model != "think" {
		t.Errorf("--model = %q", *model)
	}
	if p := fs.Args(); len(p) != 1 || !strings.HasPrefix(p[0], "Read only.") || !strings.HasSuffix(p[0], "find foo") {
		t.Errorf("prompt = %q, want instructions then the task", p)
	}
}

func TestTaskSchemaListsAgentTypes(t *testing.T) {
	SetAgentTypes(append(BuiltinAgentTypes(), AgentType{Name: "reviewer", Description: "Reviews diffs"}))
	t.Cleanup(func() { SetAgentTypes(nil) })
	s := taskSchema()
	if !strings.Contains(s.Function.Description, "- reviewer: Reviews diffs") {
		t.Errorf("description does not list custom types:\n%s", s.Function.Description)
	}
	var params struct {
		Properties map[string]struct {
			Enum []string `json:"enum"`
		} `json:"properties"`
	}
	if err := json.Unmarshal(s.Function.Parameters, &params); err != nil {
		t.Fatalf("parameters are not valid JSON: %v", err)
	}
	if got := params.Properties["subagent_type"].Enum; len(got) != 4 || got[3] != "reviewer" {
		t.Errorf("enum = %q", got)
	}
}

func TestFilterSchemas(t *testing.T) {
	all, err := LocalToolSchemas()
	if err != nil {
		t.Fatal(err)
	}
	kept, dropped, err := FilterSchemas(all, func(n string) bool { return n == "read_file" })
	if err != nil {
		t.Fatal(err)
	}
	var s []ToolSchema
	_ = json.Unmarshal([]byte(kept), &s)
	if len(s) != 1 || s[0].Function.Name != "read_file" {
		t.Errorf("kept = %v", s)
	}
	if !slices.Contains(dropped, "bash") || slices.Contains(dropped, "read_file") {
		t.Errorf("dropped = %q", dropped)
	}
}
