package agentrun

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bluefunda/abaper/internal/client"
	"github.com/bluefunda/abaper/types"
	"github.com/bluefunda/bluefunda-ai/sdk/agent"
)

func bashUse(id, command string) agent.Event {
	return agent.Event{Type: "tool_use", ToolID: id, ToolName: "bash", ToolInput: `{"command":` + quoteJSON(command) + `}`}
}

func bashResult(id, output string) agent.Event {
	return agent.Event{Type: "tool_result", ToolID: id, ToolName: "bash", ToolOutput: output}
}

func quoteJSON(s string) string {
	r := strings.NewReplacer(`\`, `\\`, `"`, `\"`, "\n", `\n`)
	return `"` + r.Replace(s) + `"`
}

func TestPrompt_NoRenameRule(t *testing.T) {
	if strings.Contains(SystemPrompt, "_V2 (then") {
		t.Error("system prompt still tells the agent to rename to _V2 on conflict")
	}
	if !strings.Contains(SystemPrompt, "abaper check") {
		t.Error("system prompt should document abaper check")
	}
	if strings.Contains(SystemPrompt, "abaper test --name <NAME> --type <type>                          # syntax check") {
		t.Error("system prompt still calls abaper test a syntax check")
	}
}

// Replays the #126 shape: a failed class deploy, a switch to a renamed
// program, and a later success. The summary must list every object.
func TestTracker_RecordsEveryObjectFromToolCalls(t *testing.T) {
	tr := NewTracker()
	events := []agent.Event{
		{Type: "tool_use", ToolID: "w1", ToolName: "write_file", ToolInput: `{"path":"zcl_alv.clas.abap"}`},
		bashUse("1", "abaper deploy --name zcl_alv_soh --type class --source-file zcl_alv.clas.abap"),
		bashResult("1", "Uploading class ZCL_ALV_SOH...\nError: upload failed: API error: A class already exists with the name ZCL_ALV_SOH"),
		bashUse("2", "cd /tmp/work && abaper deploy --type=program --name=ZALV_SOH_V2 --source-file 'z alv.prog.abap'"),
		bashResult("2", "Uploading program ZALV_SOH_V2...\nActivating program ZALV_SOH_V2...\nSuccessfully deployed and activated program ZALV_SOH_V2."),
		bashUse("3", "abaper search 'ZALV*'"),
		bashResult("3", "ZALV_SOH_V2 PROG/P"),
	}
	for _, ev := range events {
		tr.Observe(ev)
	}

	objs := tr.Objects()
	if len(objs) != 2 {
		t.Fatalf("expected 2 objects, got %+v", objs)
	}
	if objs[0].Name != "ZCL_ALV_SOH" || objs[0].Type != "class" || objs[0].OK {
		t.Errorf("unexpected first object: %+v", objs[0])
	}
	if !strings.Contains(objs[0].Detail, "already exists") {
		t.Errorf("expected failure detail to carry the conflict, got %q", objs[0].Detail)
	}
	if objs[1].Name != "ZALV_SOH_V2" || objs[1].Type != "program" || !objs[1].OK || objs[1].SourceFile != "z alv.prog.abap" {
		t.Errorf("unexpected second object: %+v", objs[1])
	}

	sum := Summary(objs)
	for _, want := range []string{"✗ class", "ZCL_ALV_SOH", "deploy failed", "✓ program", "ZALV_SOH_V2", "deployed"} {
		if !strings.Contains(sum, want) {
			t.Errorf("summary missing %q:\n%s", want, sum)
		}
	}
}

func TestTracker_LatestAttemptWins(t *testing.T) {
	tr := NewTracker()
	tr.Observe(bashUse("1", "abaper deploy --name ZFOO --source-file zfoo.abap"))
	tr.Observe(bashResult("1", "Error: activation failed: [E] line 3: Field X is unknown"))
	tr.Observe(bashUse("2", "abaper deploy --name ZFOO --source-file zfoo.abap"))
	tr.Observe(bashResult("2", "Successfully deployed and activated program ZFOO."))

	objs := tr.Objects()
	if len(objs) != 1 || !objs[0].OK || objs[0].Attempts != 2 {
		t.Fatalf("unexpected objects: %+v", objs)
	}
	if !strings.Contains(Summary(objs), "(2 attempts)") {
		t.Errorf("summary should show attempts: %s", Summary(objs))
	}
}

func TestTracker_GenerateAndCaseInsensitiveType(t *testing.T) {
	tr := NewTracker()
	tr.Observe(bashUse("1", "abaper generate --type CLASS --name zcl_x --source-file x.abap"))
	tr.Observe(bashResult("1", "Successfully created CLASS ZCL_X."))
	objs := tr.Objects()
	if len(objs) != 1 || !objs[0].OK || objs[0].Action != "generate" {
		t.Fatalf("unexpected objects: %+v", objs)
	}
}

func TestSummary_Empty(t *testing.T) {
	if got := Summary(nil); got != "No SAP objects were changed." {
		t.Errorf("unexpected summary: %q", got)
	}
}

type fakeChecker struct {
	syntax    *types.SyntaxCheckResult
	activated bool
	tests     *types.UnitTestResult
	testsErr  error
	calls     []string
}

func (f *fakeChecker) SyntaxCheck(name, typ, _ string) (*types.SyntaxCheckResult, error) {
	f.calls = append(f.calls, "check "+name)
	return f.syntax, nil
}

func (f *fakeChecker) Activate(name, typ string) (*client.ActivateResult, error) {
	f.calls = append(f.calls, "activate "+name)
	r := &client.ActivateResult{Activated: f.activated}
	if !f.activated {
		r.Messages = []client.ActivateMessage{{Severity: "E", Text: "inactive version exists"}}
	}
	return r, nil
}

func (f *fakeChecker) RunUnitTests(name, typ string) (*types.UnitTestResult, error) {
	f.calls = append(f.calls, "test "+name)
	return f.tests, f.testsErr
}

func writeSource(t *testing.T) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "src.abap")
	if err := os.WriteFile(p, []byte("REPORT zfoo."), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestVerify_AllGreen(t *testing.T) {
	f := &fakeChecker{syntax: &types.SyntaxCheckResult{}, activated: true, tests: &types.UnitTestResult{TotalTests: 2, Passed: 2}}
	vs := Verify(f, []Object{{Name: "ZCL_FOO", Type: "class", OK: true, SourceFile: writeSource(t), Action: "deploy"}})
	if !AllPassed(vs) {
		t.Fatalf("expected pass, got %+v", vs)
	}
	if got := strings.Join(f.calls, ","); got != "check ZCL_FOO,activate ZCL_FOO,test ZCL_FOO" {
		t.Errorf("unexpected call order: %s", got)
	}
	if vs[0].Syntax != "ok" || vs[0].Active != "ok" || vs[0].Tests != "ok" {
		t.Errorf("unexpected statuses: %+v", vs[0])
	}
}

func TestVerify_ReportsEachFailure(t *testing.T) {
	f := &fakeChecker{
		syntax:    &types.SyntaxCheckResult{Messages: []types.SyntaxCheckMessage{{Severity: "error", Text: "Field X is unknown", Line: 3}, {Severity: "warning", Text: "unused"}}},
		activated: false,
		tests: &types.UnitTestResult{TotalTests: 1, Failed: 1, TestClasses: []types.TestClassResult{{
			Name: "LTCL_MAIN", Methods: []types.TestMethodResult{{Name: "ADDS", Status: "failed", Message: "Expected 2"}},
		}}},
	}
	vs := Verify(f, []Object{{Name: "ZFOO", Type: "program", OK: true, SourceFile: writeSource(t), Action: "deploy"}})
	if AllPassed(vs) {
		t.Fatal("expected failure")
	}
	if len(vs[0].Problems) != 3 {
		t.Errorf("expected syntax, activation and test problems, got %v", vs[0].Problems)
	}
	rp := RepairPrompt(vs)
	for _, want := range []string{"SAME names", "program ZFOO", "Field X is unknown", "inactive version exists", "LTCL_MAIN->ADDS"} {
		if !strings.Contains(rp, want) {
			t.Errorf("repair prompt missing %q:\n%s", want, rp)
		}
	}
	if !strings.Contains(Report(vs), "failed") {
		t.Errorf("report should show failures:\n%s", Report(vs))
	}
}

func TestVerify_FailedDeploySkipsRemoteCalls(t *testing.T) {
	f := &fakeChecker{}
	vs := Verify(f, []Object{{Name: "ZCL_X", Type: "class", OK: false, Action: "deploy", Detail: "already exists"}})
	if AllPassed(vs) || len(f.calls) != 0 {
		t.Fatalf("expected failure without remote calls, got %+v calls=%v", vs, f.calls)
	}
}

func TestVerify_NoTestsAndInterfaces(t *testing.T) {
	f := &fakeChecker{syntax: &types.SyntaxCheckResult{}, activated: true, tests: &types.UnitTestResult{}, testsErr: nil}
	vs := Verify(f, []Object{
		{Name: "ZFOO", Type: "program", OK: true, SourceFile: writeSource(t), Action: "deploy"},
		{Name: "ZIF_FOO", Type: "interface", OK: true, SourceFile: writeSource(t), Action: "deploy"},
	})
	if !AllPassed(vs) {
		t.Fatalf("expected pass, got %+v", vs)
	}
	if vs[0].Tests != "none" || vs[1].Tests != "skipped" {
		t.Errorf("unexpected test statuses: %q %q", vs[0].Tests, vs[1].Tests)
	}
}

func TestVerify_UnitTestErrorIsAProblem(t *testing.T) {
	f := &fakeChecker{syntax: &types.SyntaxCheckResult{}, activated: true, testsErr: errors.New("API error 500")}
	vs := Verify(f, []Object{{Name: "ZCL_FOO", Type: "class", OK: true, SourceFile: writeSource(t), Action: "deploy"}})
	if AllPassed(vs) || vs[0].Tests != "failed" {
		t.Fatalf("expected test failure, got %+v", vs)
	}
}
