package commands

import (
	"bytes"
	"strings"
	"testing"

	"github.com/bluefunda/abaper/types"
)

func TestCountSyntaxErrors_IgnoresWarnings(t *testing.T) {
	result := &types.SyntaxCheckResult{Messages: []types.SyntaxCheckMessage{
		{Severity: "error", Text: "Field X is unknown", Line: 3},
		{Severity: "warning", Text: "Unused variable", Line: 5},
		{Severity: "error", Text: "Missing period", Line: 7},
	}}
	if got := countSyntaxErrors(result); got != 2 {
		t.Errorf("expected 2 errors, got %d", got)
	}
}

func TestPrintSyntaxCheck_ShowsLineNumbers(t *testing.T) {
	var buf bytes.Buffer
	printSyntaxCheck(&buf, "program", "ZFOO", &types.SyntaxCheckResult{Messages: []types.SyntaxCheckMessage{
		{Severity: "error", Text: "Field X is unknown", Line: 3, Column: 7},
	}})
	if got := buf.String(); got != "ZFOO:3:7: error: Field X is unknown\n" {
		t.Errorf("unexpected output: %q", got)
	}
}

func TestPrintSyntaxCheck_Clean(t *testing.T) {
	var buf bytes.Buffer
	printSyntaxCheck(&buf, "class", "ZCL_FOO", &types.SyntaxCheckResult{})
	if !strings.Contains(buf.String(), "No syntax errors") {
		t.Errorf("unexpected output: %q", buf.String())
	}
}

func TestPrintUnitTests_ListsMethodsAndTotals(t *testing.T) {
	var buf bytes.Buffer
	printUnitTests(&buf, &types.UnitTestResult{
		TotalTests: 2, Passed: 1, Failed: 1,
		TestClasses: []types.TestClassResult{{
			Name: "LTCL_MAIN",
			Methods: []types.TestMethodResult{
				{Name: "ADDS", Status: "passed"},
				{Name: "DIVIDES", Status: "failed", Message: "Expected 2, got 0"},
			},
		}},
	})
	got := buf.String()
	for _, want := range []string{"PASSED LTCL_MAIN->ADDS", "FAILED LTCL_MAIN->DIVIDES: Expected 2, got 0", "1 passed, 1 failed, 2 total"} {
		if !strings.Contains(got, want) {
			t.Errorf("expected %q in output, got %q", want, got)
		}
	}
}

func TestPrintUnitTests_NoTests(t *testing.T) {
	var buf bytes.Buffer
	printUnitTests(&buf, &types.UnitTestResult{})
	if !strings.Contains(buf.String(), "No unit tests found") {
		t.Errorf("unexpected output: %q", buf.String())
	}
}
