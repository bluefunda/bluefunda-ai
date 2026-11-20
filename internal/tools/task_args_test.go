package tools

import (
	"reflect"
	"testing"

	"github.com/spf13/pflag"
)

// The child parses --disallowed-tools as a pflag StringSlice. Every forwarded
// rule must reach it intact, including ones with commas or quotes, and the
// prompt must stay positional even when it starts with "-".
func TestTaskArgs_ForwardsDenyRulesIntact(t *testing.T) {
	rules := []string{"write_file", "bash:echo a,b", `bash:grep "x"`}
	SetTaskDisallowedTools(rules)
	t.Cleanup(func() { SetTaskDisallowedTools(nil) })

	args := taskArgs("-v looks like a flag", 7, true)

	fs := pflag.NewFlagSet("child", pflag.ContinueOnError)
	var deny []string
	fs.StringSliceVar(&deny, "disallowed-tools", nil, "")
	fs.Bool("print", false, "")
	fs.String("output-format", "", "")
	fs.Bool("auto", false, "")
	fs.Int("max-turns", 0, "")
	fs.Bool("worktree", false, "")
	if err := fs.Parse(args); err != nil {
		t.Fatalf("child parse %q: %v", args, err)
	}
	if !reflect.DeepEqual(deny, rules) {
		t.Errorf("child deny = %q, want %q", deny, rules)
	}
	if got := fs.Args(); !reflect.DeepEqual(got, []string{"-v looks like a flag"}) {
		t.Errorf("positional = %q, want the prompt only", got)
	}
	if n, _ := fs.GetInt("max-turns"); n != 7 {
		t.Errorf("max-turns = %d, want 7", n)
	}
	if w, _ := fs.GetBool("worktree"); !w {
		t.Error("worktree flag lost")
	}
}

func TestTaskArgs_NoDenyRules(t *testing.T) {
	SetTaskDisallowedTools(nil)
	want := []string{"--print", "--output-format", "stream-json", "--auto", "--", "do it"}
	if got := taskArgs("do it", 0, false); !reflect.DeepEqual(got, want) {
		t.Errorf("args = %q, want %q", got, want)
	}
}
