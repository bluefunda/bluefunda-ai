package agentrun

import (
	"fmt"
	"os"
	"strings"

	"github.com/bluefunda/abaper/internal/client"
	"github.com/bluefunda/abaper/types"
)

// Checker is the subset of the ABAPer API client the verification gate uses.
type Checker interface {
	SyntaxCheck(objectName, objectType, source string) (*types.SyntaxCheckResult, error)
	Activate(objectName, objectType string) (*client.ActivateResult, error)
	RunUnitTests(objectName, objectType string) (*types.UnitTestResult, error)
}

// Verification is the gate outcome for one object. Each step is "ok",
// "failed", "skipped" or "none" (unit tests only: object has no tests).
type Verification struct {
	Object   Object
	Syntax   string
	Active   string
	Tests    string
	Problems []string
}

func (v Verification) Passed() bool { return len(v.Problems) == 0 }

// Verify runs syntax check → activation → unit tests for every object the
// agent touched. abaper runs this itself after the loop ends; it does not
// rely on the model having done it.
func Verify(c Checker, objs []Object) []Verification {
	out := make([]Verification, 0, len(objs))
	for _, o := range objs {
		out = append(out, verifyOne(c, o))
	}
	return out
}

func verifyOne(c Checker, o Object) Verification {
	v := Verification{Object: o, Syntax: "skipped", Active: "skipped", Tests: "skipped"}
	if !o.OK {
		v.Problems = append(v.Problems, fmt.Sprintf("last %s failed: %s", o.Action, o.Detail))
		return v
	}

	if src, err := os.ReadFile(o.SourceFile); err == nil {
		res, err := c.SyntaxCheck(o.Name, o.Type, string(src))
		switch {
		case err != nil:
			v.Syntax = "failed"
			v.Problems = append(v.Problems, "syntax check: "+err.Error())
		default:
			v.Syntax = "ok"
			for _, m := range res.Messages {
				if m.Severity == "error" {
					v.Syntax = "failed"
					v.Problems = append(v.Problems, fmt.Sprintf("syntax error line %d: %s", m.Line, m.Text))
				}
			}
		}
	}

	act, err := c.Activate(o.Name, o.Type)
	switch {
	case err != nil:
		v.Active = "failed"
		v.Problems = append(v.Problems, "activation: "+err.Error())
	case !act.Activated:
		v.Active = "failed"
		msgs := make([]string, 0, len(act.Messages))
		for _, m := range act.Messages {
			msgs = append(msgs, m.Text)
		}
		v.Problems = append(v.Problems, "not active: "+strings.Join(msgs, "; "))
	default:
		v.Active = "ok"
	}

	if o.Type != "class" && o.Type != "program" {
		return v
	}
	tests, err := c.RunUnitTests(o.Name, o.Type)
	switch {
	case err != nil:
		v.Tests = "failed"
		v.Problems = append(v.Problems, "unit tests: "+err.Error())
	case tests.TotalTests == 0:
		v.Tests = "none"
	case tests.Failed > 0:
		v.Tests = "failed"
		for _, tc := range tests.TestClasses {
			for _, m := range tc.Methods {
				if m.Status != "passed" {
					v.Problems = append(v.Problems, fmt.Sprintf("unit test %s->%s %s: %s", tc.Name, m.Name, m.Status, m.Message))
				}
			}
		}
	default:
		v.Tests = "ok"
	}
	return v
}

// Report renders the per-object gate table.
func Report(vs []Verification) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%-9s %-30s %-8s %-8s %-8s\n", "TYPE", "NAME", "SYNTAX", "ACTIVE", "TESTS")
	for _, v := range vs {
		fmt.Fprintf(&b, "%-9s %-30s %-8s %-8s %-8s\n", v.Object.Type, v.Object.Name, v.Syntax, v.Active, v.Tests)
		for _, p := range v.Problems {
			fmt.Fprintf(&b, "    - %s\n", p)
		}
	}
	return strings.TrimRight(b.String(), "\n")
}

// RepairPrompt asks the agent to fix the objects that failed verification,
// under the same names and types.
func RepairPrompt(vs []Verification) string {
	var b strings.Builder
	b.WriteString("abaper verified the objects you changed and found problems. Fix them under the SAME names and object types, then redeploy. If a problem is an \"already exists\" conflict, do not retry — just report it:\n")
	for _, v := range vs {
		if v.Passed() {
			continue
		}
		fmt.Fprintf(&b, "\n%s %s:\n", v.Object.Type, v.Object.Name)
		for _, p := range v.Problems {
			fmt.Fprintf(&b, "- %s\n", p)
		}
	}
	return b.String()
}

// AllPassed reports whether every verification passed.
func AllPassed(vs []Verification) bool {
	for _, v := range vs {
		if !v.Passed() {
			return false
		}
	}
	return true
}
