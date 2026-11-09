package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"
)

// Pack is a bundle of domain tools plus agent instructions that a product
// adds to the platform agent — e.g. the ABAP pack behind `bai abap` and
// abaper. Once registered, pack tools show up in LocalToolSchemas, are
// dispatched by Execute, and take part in approval and plan-mode checks
// like any built-in, so every agent surface (bai code, the TUI, sdk/agent)
// picks them up without extra wiring.
type Pack struct {
	Name string
	// Instructions is prepended to the conversation as a system message.
	Instructions string
	Tools        []PackTool
}

// PackTool is one tool contributed by a Pack.
type PackTool struct {
	Name        string
	Description string
	// Parameters is the JSON Schema for the arguments object. nil → no arguments.
	Parameters json.RawMessage
	// ReadOnly tools never prompt for approval and stay available in plan mode.
	ReadOnly bool
	// PrimaryArg names the argument matched by permission globs
	// ("adt_deploy:Z*") and shown in approval prompts.
	PrimaryArg string
	Handler    func(ctx context.Context, argumentsJSON string) (string, error)
}

var (
	packsMu sync.RWMutex
	packs   = map[string]Pack{}
)

// RegisterPack adds p to the running process and returns a function that
// removes it again. Tool names must not collide with built-ins or with
// tools of other registered packs.
func RegisterPack(p Pack) (unregister func(), err error) {
	builtins := builtinToolNames()
	packsMu.Lock()
	defer packsMu.Unlock()
	if _, ok := packs[p.Name]; ok {
		return nil, fmt.Errorf("tool pack %q already registered", p.Name)
	}
	seen := map[string]bool{}
	for _, other := range packs {
		for _, t := range other.Tools {
			seen[t.Name] = true
		}
	}
	for _, t := range p.Tools {
		if t.Name == "" || t.Handler == nil {
			return nil, fmt.Errorf("tool pack %q: every tool needs a Name and a Handler", p.Name)
		}
		if builtins[t.Name] || seen[t.Name] {
			return nil, fmt.Errorf("tool pack %q: duplicate tool name %q", p.Name, t.Name)
		}
		seen[t.Name] = true
	}
	packs[p.Name] = p
	return func() {
		packsMu.Lock()
		defer packsMu.Unlock()
		delete(packs, p.Name)
	}, nil
}

// PackInstructions returns the instructions of all registered packs, in
// name order, joined by blank lines. Empty when no pack is registered.
func PackInstructions() string {
	packsMu.RLock()
	defer packsMu.RUnlock()
	var out []string
	for _, name := range sortedPackNames() {
		if s := strings.TrimSpace(packs[name].Instructions); s != "" {
			out = append(out, s)
		}
	}
	return strings.Join(out, "\n\n")
}

// packSchemas returns the schemas of all registered pack tools, in pack-name order.
func packSchemas() []ToolSchema {
	packsMu.RLock()
	defer packsMu.RUnlock()
	var out []ToolSchema
	for _, name := range sortedPackNames() {
		for _, t := range packs[name].Tools {
			params := t.Parameters
			if len(params) == 0 {
				params = json.RawMessage(`{"type":"object","properties":{}}`)
			}
			out = append(out, ToolSchema{
				Type:     "function",
				Function: FunctionDef{Name: t.Name, Description: t.Description, Parameters: params},
			})
		}
	}
	return out
}

// lookupPackTool finds a registered pack tool by name.
func lookupPackTool(name string) (PackTool, bool) {
	packsMu.RLock()
	defer packsMu.RUnlock()
	for _, p := range packs {
		for _, t := range p.Tools {
			if t.Name == name {
				return t, true
			}
		}
	}
	return PackTool{}, false
}

// sortedPackNames must be called with packsMu held.
func sortedPackNames() []string {
	names := make([]string, 0, len(packs))
	for n := range packs {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

func builtinToolNames() map[string]bool {
	schemas := builtinSchemas()
	names := make(map[string]bool, len(schemas))
	for _, s := range schemas {
		names[s.Function.Name] = true
	}
	return names
}
