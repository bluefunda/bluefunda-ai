package abap

import (
	"fmt"
	"strings"
	"sync"
)

// Object is an SAP object the agent deployed, with the outcome of its most
// recent attempt.
type Object struct {
	Name     string
	Type     string
	Source   string // source sent on the last attempt; re-checked by Verify
	Attempts int
	OK       bool
	Detail   string // failure reason when !OK
}

func (o Object) key() string { return o.Type + "/" + o.Name }

// Tracker records every deploy the agent makes through the abap_deploy tool.
// It is fed by the tool handler itself, so the summary reflects what was
// actually sent to SAP rather than the model's narration. Safe for
// concurrent use: the agent may run tools in parallel.
type Tracker struct {
	mu      sync.Mutex
	order   []string
	objects map[string]Object
}

// NewTracker returns an empty Tracker.
func NewTracker() *Tracker { return &Tracker{objects: map[string]Object{}} }

// Record stores the outcome of one deploy attempt.
func (t *Tracker) Record(name, objectType, source string, err error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	o := Object{Name: name, Type: objectType}
	if prev, ok := t.objects[o.key()]; ok {
		o.Attempts = prev.Attempts
	} else {
		t.order = append(t.order, o.key())
	}
	o.Attempts++
	o.Source = source
	o.OK = err == nil
	if err != nil {
		o.Detail = err.Error()
	}
	t.objects[o.key()] = o
}

// Objects returns the touched objects in first-touched order.
func (t *Tracker) Objects() []Object {
	t.mu.Lock()
	defer t.mu.Unlock()
	out := make([]Object, 0, len(t.order))
	for _, k := range t.order {
		out = append(out, t.objects[k])
	}
	return out
}

// Reset forgets all recorded objects (e.g. at the start of a new prompt).
func (t *Tracker) Reset() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.order = nil
	t.objects = map[string]Object{}
}

// Summary renders one line per touched object.
func Summary(objs []Object) string {
	if len(objs) == 0 {
		return "No SAP objects were deployed."
	}
	var b strings.Builder
	fmt.Fprintf(&b, "SAP objects touched (%d):\n", len(objs))
	for _, o := range objs {
		status := "deployed"
		if !o.OK {
			status = "FAILED: " + o.Detail
		}
		tries := ""
		if o.Attempts > 1 {
			tries = fmt.Sprintf(" (%d attempts)", o.Attempts)
		}
		fmt.Fprintf(&b, "  %-9s %-30s %s%s\n", o.Type, o.Name, status, tries)
	}
	return strings.TrimRight(b.String(), "\n")
}

// Checker is the part of Client the verification gate uses.
type Checker interface {
	SyntaxCheck(name, objectType, source string) (*SyntaxCheckResult, error)
	Activate(name, objectType string) (*ActivateResult, error)
	UnitTests(name, objectType string) (*UnitTestResult, error)
}

// Verification is the gate outcome for one object. Each step is "ok",
// "failed", "skipped", or (unit tests only) "none" when there are no tests.
type Verification struct {
	Object   Object
	Syntax   string
	Active   string
	Tests    string
	Problems []string
}

// Passed reports whether the object cleared every step.
func (v Verification) Passed() bool { return len(v.Problems) == 0 }

// Verify runs syntax check → activation → unit tests for every object the
// agent deployed. The product runs this itself after the loop; it does not
// trust the model to have done it.
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
		v.Problems = append(v.Problems, "last deploy failed: "+o.Detail)
		return v
	}

	if o.Source != "" {
		res, err := c.SyntaxCheck(o.Name, o.Type, o.Source)
		if err != nil {
			v.Syntax = "failed"
			v.Problems = append(v.Problems, "syntax check: "+err.Error())
		} else {
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
		v.Problems = append(v.Problems, "not active: "+ActivationSummary(act.Messages))
	default:
		v.Active = "ok"
	}

	if o.Type != "class" && o.Type != "program" {
		return v
	}
	tests, err := c.UnitTests(o.Name, o.Type)
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

// RepairPrompt asks the agent to fix the failed objects under the same names and types.
func RepairPrompt(vs []Verification) string {
	var b strings.Builder
	b.WriteString("Verification of the objects you changed found problems. Fix them under the SAME names and object types, then redeploy. If a problem is an \"already exists\" conflict, do not retry — just report it:\n")
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
