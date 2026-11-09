package tools

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func testPack() Pack {
	return Pack{
		Name:         "abap",
		Instructions: "You are an ABAP expert.",
		Tools: []PackTool{
			{
				Name:       "adt_get_source",
				ReadOnly:   true,
				PrimaryArg: "name",
				Parameters: json.RawMessage(`{"type":"object","properties":{"name":{"type":"string"}}}`),
				Handler: func(_ context.Context, args string) (string, error) {
					return "source:" + args, nil
				},
			},
			{
				Name:       "adt_deploy",
				PrimaryArg: "name",
				Handler:    func(context.Context, string) (string, error) { return "deployed", nil },
			},
		},
	}
}

func registerTestPack(t *testing.T) {
	t.Helper()
	unregister, err := RegisterPack(testPack())
	if err != nil {
		t.Fatalf("RegisterPack: %v", err)
	}
	t.Cleanup(unregister)
}

func TestRegisterPack_ToolsJoinSchemasAndDispatch(t *testing.T) {
	registerTestPack(t)

	schemas, err := LocalToolSchemas()
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{`"bash"`, `"adt_get_source"`, `"adt_deploy"`} {
		if !strings.Contains(schemas, name) {
			t.Errorf("schemas missing %s", name)
		}
	}

	out, err := Execute("adt_get_source", `{"name":"ZCL_FOO"}`)
	if err != nil || out != `source:{"name":"ZCL_FOO"}` {
		t.Errorf("Execute = %q, %v", out, err)
	}
	if got := PackInstructions(); got != "You are an ABAP expert." {
		t.Errorf("PackInstructions = %q", got)
	}
}

func TestRegisterPack_ApprovalAndPlanMode(t *testing.T) {
	registerTestPack(t)

	if NeedsApproval("adt_get_source") || !IsReadOnlyTool("adt_get_source") {
		t.Error("read-only pack tool should skip approval and be allowed in plan mode")
	}
	if !NeedsApproval("adt_deploy") || IsReadOnlyTool("adt_deploy") {
		t.Error("mutating pack tool should need approval and be blocked in plan mode")
	}
	if CheckPermissions(nil, []string{"adt_deploy:ZTMP*"}, "adt_deploy", `{"name":"ZTMP_X"}`) != PermitDeny {
		t.Error("deny glob should match the pack tool's PrimaryArg")
	}
}

func TestRegisterPack_Unregister(t *testing.T) {
	unregister, err := RegisterPack(testPack())
	if err != nil {
		t.Fatal(err)
	}
	unregister()
	if _, err := Execute("adt_deploy", `{}`); err == nil {
		t.Error("tool should be gone after unregister")
	}
	if PackInstructions() != "" {
		t.Error("instructions should be gone after unregister")
	}
}

func TestRegisterPack_Rejects(t *testing.T) {
	registerTestPack(t)
	cases := map[string]Pack{
		"same pack twice":    testPack(),
		"shadows a built-in": {Name: "x", Tools: []PackTool{{Name: "bash", Handler: testPack().Tools[1].Handler}}},
		"clashes with pack":  {Name: "y", Tools: []PackTool{{Name: "adt_deploy", Handler: testPack().Tools[1].Handler}}},
		"missing handler":    {Name: "z", Tools: []PackTool{{Name: "adt_other"}}},
	}
	for name, p := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := RegisterPack(p); err == nil {
				t.Error("expected error")
			}
		})
	}
}
