package abap

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/bluefunda/bluefunda-ai/internal/tools"
)

// ObjectTypes are the object types the gateway accepts.
var ObjectTypes = []string{"program", "class", "interface", "function", "include", "structure", "table"}

// Instructions are the ABAP rules given to the agent alongside the tools.
const Instructions = `You are an ABAP expert developer. You write, review, and deploy SAP ABAP code.

You can read and edit local ABAP source files, and you work with the connected SAP system through the abap_* tools:
  abap_search          find objects by name pattern (SAP wildcards, e.g. ZCL_*)
  abap_get_source      read an object's source from SAP
  abap_list_package    list the objects in a package
  abap_syntax_check    check source without saving anything
  abap_deploy          save + activate an object (from a local file or inline source)
  abap_activate        activate an already-saved object
  abap_run_unit_tests  run an object's ABAP Unit tests

Rules:
- Always uppercase ABAP object names (ZCL_MY_CLASS, ZFM_VALIDATE, etc.)
- Valid types: program, class, interface, function, include, structure, table
- Prefer reading existing source before writing new code
- Keep source in local files and deploy with source_file, so the user can review it
- Run abap_syntax_check before abap_deploy, then abap_run_unit_tests for objects with unit tests
- DO NOT explain what you are about to do — act immediately with tool calls
- Keep text responses to one line
- Never rename an object or change its type to get around an error. If deploy returns
  "already exists", STOP and report the conflict — do not append _V2 or any other
  suffix, and do not retry under a different name or object type.
- If you cannot build exactly what the user asked for (name and object type), stop and
  say why instead of switching to a different approach.
- Interfaces MUST declare "INTERFACE <name> PUBLIC." — without PUBLIC the object shell
  activates, then fails with a misleading "A class already exists with the name X".
  If you see that error on an interface, fix the source and redeploy under the same name.
- DDIC structures and tables MUST use the curly-brace form: "define structure NAME { ... }"
  / "define table NAME { ... }" — NOT the dot-terminated "end structure." form.
- DDIC structures need BOTH "@EndUserText.label : '...'" AND "@AbapCatalog.enhancement.category"
  or save fails with a generic "errors in source" the syntax check does NOT catch.
- DDIC tables need "@AbapCatalog.tableCategory", "@AbapCatalog.deliveryClass",
  "@AbapCatalog.dataMaintenance", and a "key client : abap.clnt not null;" first field.`

// Env is what the pack's tools need at call time. NewClient is called per
// tool call so login refreshes and `system use` take effect immediately.
type Env struct {
	NewClient func() (*Client, error)
	Tracker   *Tracker // may be nil
}

type objectArgs struct {
	Name       string `json:"name"`
	Type       string `json:"type"`
	Source     string `json:"source"`
	SourceFile string `json:"source_file"`
	Pattern    string `json:"pattern"`
	Package    string `json:"package"`
}

func parseArgs(raw string) (objectArgs, error) {
	var a objectArgs
	if err := json.Unmarshal([]byte(raw), &a); err != nil {
		return a, fmt.Errorf("parse arguments: %w", err)
	}
	a.Name = strings.ToUpper(strings.TrimSpace(a.Name))
	a.Package = strings.ToUpper(strings.TrimSpace(a.Package))
	a.Type = strings.ToLower(strings.TrimSpace(a.Type))
	return a, nil
}

func (a objectArgs) requireObject() error {
	if a.Name == "" {
		return fmt.Errorf("name is required")
	}
	for _, t := range ObjectTypes {
		if a.Type == t {
			return nil
		}
	}
	return fmt.Errorf("type must be one of %s", strings.Join(ObjectTypes, ", "))
}

// source returns inline source or the contents of source_file (confined to the workspace).
func (a objectArgs) source() (string, error) {
	if a.Source != "" && a.SourceFile != "" {
		return "", fmt.Errorf("pass either source or source_file, not both")
	}
	if a.SourceFile == "" {
		if a.Source == "" {
			return "", fmt.Errorf("source or source_file is required")
		}
		return a.Source, nil
	}
	path, err := tools.ConfineToWorkspace(a.SourceFile)
	if err != nil {
		return "", err
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read source file: %w", err)
	}
	return string(b), nil
}

const (
	objectSchema = `{
		"type": "object",
		"properties": {
			"name": {"type": "string", "description": "Object name, uppercase (e.g. ZCL_ORDER_API)"},
			"type": {"type": "string", "enum": ["program","class","interface","function","include","structure","table"]}
		},
		"required": ["name", "type"]
	}`
	sourceSchema = `{
		"type": "object",
		"properties": {
			"name":        {"type": "string", "description": "Object name, uppercase"},
			"type":        {"type": "string", "enum": ["program","class","interface","function","include","structure","table"]},
			"source_file": {"type": "string", "description": "Local file holding the full source (preferred)"},
			"source":      {"type": "string", "description": "Full source inline, when there is no local file"}
		},
		"required": ["name", "type"]
	}`
)

// Pack returns the ABAP tool pack bound to env.
func Pack(env Env) tools.Pack {
	withClient := func(fn func(*Client, objectArgs) (string, error)) func(context.Context, string) (string, error) {
		return func(_ context.Context, raw string) (string, error) {
			a, err := parseArgs(raw)
			if err != nil {
				return "", err
			}
			c, err := env.NewClient()
			if err != nil {
				return "", err
			}
			return fn(c, a)
		}
	}

	return tools.Pack{
		Name:         "abap",
		Instructions: Instructions,
		Tools: []tools.PackTool{
			{
				Name:        "abap_search",
				Description: "Search SAP repository objects by name pattern (SAP wildcards, e.g. ZCL_ORDER*). Optionally filter by object type.",
				Parameters: json.RawMessage(`{
					"type": "object",
					"properties": {
						"pattern": {"type": "string", "description": "Name pattern, e.g. Z*ORDER*"},
						"type":    {"type": "string", "description": "Optional ADT type filter, e.g. CLAS/OC, PROG/P"}
					},
					"required": ["pattern"]
				}`),
				ReadOnly:   true,
				PrimaryArg: "pattern",
				Handler: withClient(func(c *Client, a objectArgs) (string, error) {
					if a.Pattern == "" {
						return "", fmt.Errorf("pattern is required")
					}
					objs, err := c.Search(strings.ToUpper(a.Pattern), a.Type)
					if err != nil {
						return "", err
					}
					return formatRows(objs, 100), nil
				}),
			},
			{
				Name:        "abap_get_source",
				Description: "Read the current source of an object from the SAP system.",
				Parameters:  json.RawMessage(objectSchema),
				ReadOnly:    true,
				PrimaryArg:  "name",
				Handler: withClient(func(c *Client, a objectArgs) (string, error) {
					if err := a.requireObject(); err != nil {
						return "", err
					}
					src, err := c.Source(a.Name, a.Type)
					if err != nil {
						return "", err
					}
					if src == "" {
						return fmt.Sprintf("%s %s has no source (or does not exist).", a.Type, a.Name), nil
					}
					return src, nil
				}),
			},
			{
				Name:        "abap_list_package",
				Description: "List the objects in an SAP package, optionally filtered by object type.",
				Parameters: json.RawMessage(`{
					"type": "object",
					"properties": {
						"package": {"type": "string", "description": "Package name, e.g. ZORDERS or $TMP"},
						"type":    {"type": "string", "description": "Optional object type filter"}
					},
					"required": ["package"]
				}`),
				ReadOnly:   true,
				PrimaryArg: "package",
				Handler: withClient(func(c *Client, a objectArgs) (string, error) {
					if a.Package == "" {
						return "", fmt.Errorf("package is required")
					}
					objs, err := c.ListPackage(a.Package, a.Type)
					if err != nil {
						return "", err
					}
					return formatRows(objs, 200), nil
				}),
			},
			{
				Name:        "abap_syntax_check",
				Description: "Syntax-check ABAP source against the SAP system without saving anything.",
				Parameters:  json.RawMessage(sourceSchema),
				ReadOnly:    true,
				PrimaryArg:  "name",
				Handler: withClient(func(c *Client, a objectArgs) (string, error) {
					if err := a.requireObject(); err != nil {
						return "", err
					}
					src, err := a.source()
					if err != nil {
						return "", err
					}
					res, err := c.SyntaxCheck(a.Name, a.Type, src)
					if err != nil {
						return "", err
					}
					return syntaxReport(res), nil
				}),
			},
			{
				Name:        "abap_deploy",
				Description: "Save an object's full source to SAP and activate it. Creates the object if it does not exist. Fails (does not rename) if the name is taken by another object type.",
				Parameters:  json.RawMessage(sourceSchema),
				PrimaryArg:  "name",
				Handler: withClient(func(c *Client, a objectArgs) (string, error) {
					if err := a.requireObject(); err != nil {
						return "", err
					}
					src, err := a.source()
					if err != nil {
						return "", err
					}
					err = deploy(c, a.Name, a.Type, src)
					if env.Tracker != nil {
						env.Tracker.Record(a.Name, a.Type, src, err)
					}
					if err != nil {
						return "", err
					}
					return fmt.Sprintf("Deployed and activated %s %s.", a.Type, a.Name), nil
				}),
			},
			{
				Name:        "abap_activate",
				Description: "Activate an object that is already saved on SAP.",
				Parameters:  json.RawMessage(objectSchema),
				PrimaryArg:  "name",
				Handler: withClient(func(c *Client, a objectArgs) (string, error) {
					if err := a.requireObject(); err != nil {
						return "", err
					}
					res, err := c.Activate(a.Name, a.Type)
					if err != nil {
						return "", err
					}
					if !res.Activated {
						return "", fmt.Errorf("activation failed: %s", ActivationSummary(res.Messages))
					}
					return fmt.Sprintf("Activated %s %s.", a.Type, a.Name), nil
				}),
			},
			{
				Name:        "abap_run_unit_tests",
				Description: "Run the ABAP Unit tests of a class or program and report each failing method.",
				Parameters:  json.RawMessage(objectSchema),
				ReadOnly:    true,
				PrimaryArg:  "name",
				Handler: withClient(func(c *Client, a objectArgs) (string, error) {
					if err := a.requireObject(); err != nil {
						return "", err
					}
					res, err := c.UnitTests(a.Name, a.Type)
					if err != nil {
						return "", err
					}
					return unitTestReport(res), nil
				}),
			},
		},
	}
}

// deploy saves and activates one object; an inactive result is an error.
func deploy(c *Client, name, objectType, source string) error {
	if err := c.Save(name, objectType, source); err != nil {
		return fmt.Errorf("save failed: %w", err)
	}
	res, err := c.Activate(name, objectType)
	if err != nil {
		return fmt.Errorf("activation failed: %w", err)
	}
	if !res.Activated {
		return fmt.Errorf("activation failed: %s", ActivationSummary(res.Messages))
	}
	return nil
}

// Deploy saves and activates one object (name is uppercased); an inactive
// result is an error.
func Deploy(c *Client, name, objectType, source string) error {
	return deploy(c, strings.ToUpper(name), objectType, source)
}

func syntaxReport(res *SyntaxCheckResult) string {
	var errs, other []string
	for _, m := range res.Messages {
		line := fmt.Sprintf("line %d:%d %s: %s", m.Line, m.Column, m.Severity, m.Text)
		if m.Severity == "error" {
			errs = append(errs, line)
		} else {
			other = append(other, line)
		}
	}
	if len(errs) == 0 && len(other) == 0 {
		return "Syntax OK — no findings."
	}
	head := "Syntax OK"
	if len(errs) > 0 {
		head = fmt.Sprintf("Syntax check FAILED with %d error(s)", len(errs))
	}
	return head + "\n" + strings.Join(append(errs, other...), "\n")
}

func unitTestReport(res *UnitTestResult) string {
	if res.TotalTests == 0 {
		return "No unit tests found."
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%d tests: %d passed, %d failed", res.TotalTests, res.Passed, res.Failed)
	for _, tc := range res.TestClasses {
		for _, m := range tc.Methods {
			if m.Status != "passed" {
				fmt.Fprintf(&b, "\n%s->%s %s: %s", tc.Name, m.Name, m.Status, m.Message)
			}
		}
	}
	return b.String()
}

// jsonList renders up to limit rows as compact JSON lines with stable key order.
func formatRows(rows []map[string]any, limit int) string {
	if len(rows) == 0 {
		return "No objects found."
	}
	var b strings.Builder
	for i, r := range rows {
		if i == limit {
			fmt.Fprintf(&b, "… %d more not shown — narrow the pattern\n", len(rows)-limit)
			break
		}
		keys := make([]string, 0, len(r))
		for k := range r {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		parts := make([]string, 0, len(keys))
		for _, k := range keys {
			parts = append(parts, fmt.Sprintf("%s=%v", k, r[k]))
		}
		b.WriteString(strings.Join(parts, " "))
		b.WriteByte('\n')
	}
	return strings.TrimRight(b.String(), "\n")
}
