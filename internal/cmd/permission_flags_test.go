package cmd

import (
	"bytes"
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/spf13/pflag"

	"github.com/bluefunda/bluefunda-ai/internal/tools"
	"github.com/bluefunda/bluefunda-ai/internal/ui/tui"
)

func parsePermissionFlags(t *testing.T, argv ...string) (allowed, disallowed []string) {
	t.Helper()
	fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
	registerPermissionFlags(fs, &allowed, &disallowed)
	if err := fs.Parse(argv); err != nil {
		t.Fatalf("parse %v: %v", argv, err)
	}
	return allowed, disallowed
}

func TestPermissionFlags_KebabCommaAndRepeat(t *testing.T) {
	allowed, disallowed := parsePermissionFlags(t,
		"--allowed-tools", "bash:go test *,read_file",
		"--allowed-tools", "list_dir",
		"--disallowed-tools", "write_file",
	)
	if want := []string{"bash:go test *", "read_file", "list_dir"}; !reflect.DeepEqual(allowed, want) {
		t.Errorf("allowed = %q, want %q", allowed, want)
	}
	if want := []string{"write_file"}; !reflect.DeepEqual(disallowed, want) {
		t.Errorf("disallowed = %q, want %q", disallowed, want)
	}
}

func TestPermissionFlags_CamelCaseAliases(t *testing.T) {
	allowed, disallowed := parsePermissionFlags(t,
		"--allowedTools", "edit_file",
		"--disallowedTools", "bash:rm *",
	)
	if want := []string{"edit_file"}; !reflect.DeepEqual(allowed, want) {
		t.Errorf("allowed = %q, want %q", allowed, want)
	}
	if want := []string{"bash:rm *"}; !reflect.DeepEqual(disallowed, want) {
		t.Errorf("disallowed = %q, want %q", disallowed, want)
	}
}

func TestPermissionFlags_RegisteredOnRootAndCodeAlias(t *testing.T) {
	for _, name := range []string{"allowed-tools", "disallowed-tools"} {
		if rootCmd.Flags().Lookup(name) == nil {
			t.Errorf("root command missing --%s", name)
		}
		if codeCmd.Flags().Lookup(name) == nil {
			t.Errorf("code command missing --%s", name)
		}
	}
}

func TestMergePermissionRules(t *testing.T) {
	base := make([]string, 1, 4) // spare capacity: merge must not write into it
	base[0] = "web_search"
	got := mergePermissionRules(base, []string{" bash:ls* ", "", "  "})
	if want := []string{"web_search", "bash:ls*"}; !reflect.DeepEqual(got, want) {
		t.Errorf("merged = %q, want %q", got, want)
	}
	if spare := base[:2][1]; spare != "" {
		t.Errorf("merge wrote into the base slice's backing array: %q", spare)
	}
}

// A flag deny rule must win over a flag allow rule and over settings allows,
// since tools.CheckPermissions evaluates deny first.
func TestFlagDenyBeatsAllow(t *testing.T) {
	allow := mergePermissionRules([]string{"bash"}, []string{"bash:go test *"})
	deny := mergePermissionRules(nil, []string{"bash:go test ./secret/*"})
	if got := tools.CheckPermissions(allow, deny, "bash", `{"command":"go test ./secret/x"}`); got != tools.PermitDeny {
		t.Errorf("deny rule should win, got %v", got)
	}
	if got := tools.CheckPermissions(allow, deny, "bash", `{"command":"go test ./..."}`); got != tools.PermitAuto {
		t.Errorf("allowed command should auto-approve, got %v", got)
	}
}

// Headless mode must decline approval requests instead of hanging (#406).
func TestDrainPrintStream_ApprovalIsDeclined(t *testing.T) {
	for _, format := range []string{"text", "stream-json"} {
		t.Run(format, func(t *testing.T) {
			reply := make(chan tui.ApprovalDecision, 1)
			ch := feedEvents([]tui.StreamEvent{
				{Kind: "approval", ToolName: "write_file", ToolArgs: `{"path":"x"}`, ReplyCh: reply},
				{Kind: "done"},
			})
			var out, errOut bytes.Buffer
			if code := drainPrintStream(ch, format, &out, &errOut); code != 0 {
				t.Errorf("exit code = %d, want 0", code)
			}
			select {
			case d := <-reply:
				if d.Approved {
					t.Error("headless approval must be declined")
				}
			default:
				t.Fatal("no reply sent: the agentic loop would block forever")
			}
			if format == "text" {
				if !strings.Contains(errOut.String(), "denied: write_file") {
					t.Errorf("stderr = %q, want a denial notice", errOut.String())
				}
				return
			}
			first := strings.SplitN(out.String(), "\n", 2)[0]
			var ev map[string]any
			if err := json.Unmarshal([]byte(first), &ev); err != nil {
				t.Fatalf("stream-json line %q: %v", first, err)
			}
			if ev["type"] != "permission_denied" || ev["name"] != "write_file" {
				t.Errorf("event = %v, want permission_denied for write_file", ev)
			}
		})
	}
}
