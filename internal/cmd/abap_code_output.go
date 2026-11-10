package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"

	"github.com/bluefunda/bluefunda-ai/internal/abap"
	"github.com/bluefunda/bluefunda-ai/internal/tools"
	"github.com/bluefunda/bluefunda-ai/sdk/agent"
)

// outputStreamJSON is the `code` output mode for editor integrations: one
// JSON event per line on stdout. The schema is in abaper/docs/stream-json.md;
// field changes there are breaking. The agent events share their names and
// keys with `bai --print --output-format stream-json`.
const outputStreamJSON = "stream-json"

// codeReporter renders a headless `code` run.
type codeReporter interface {
	Start(model, system, cwd string)
	Event(agent.Event)
	Objects([]abap.Object)
	Verifying()
	Verified(round int, results []abap.Verification)
	Repairing(round, of int)
	Done(err error)
}

// textReporter is the human output: agent text on stdout, everything else on stderr.
type textReporter struct {
	out     io.Writer
	verbose bool
}

func (r textReporter) Start(model, system, cwd string) {}

func (r textReporter) Event(ev agent.Event) {
	switch ev.Type {
	case "text":
		_, _ = fmt.Fprint(r.out, ev.Text)
	case "tool_use":
		fmt.Fprintf(os.Stderr, "\n▸ %s %s\n", ev.ToolName, tools.ExtractPrimaryArg(ev.ToolName, ev.ToolInput))
	case "tool_result":
		if r.verbose || strings.HasPrefix(ev.ToolOutput, "Error:") {
			fmt.Fprintf(os.Stderr, "  %s\n", truncate(ev.ToolOutput, 400))
		}
	case "error":
		fmt.Fprintf(os.Stderr, "\nError: %v\n", ev.Err)
	}
}

func (r textReporter) Objects(objs []abap.Object) {
	fmt.Fprintln(os.Stderr, "\n"+abap.Summary(objs))
}

func (r textReporter) Verifying() { fmt.Fprintln(os.Stderr, "\nVerifying SAP objects…") }

func (r textReporter) Verified(_ int, results []abap.Verification) {
	fmt.Fprintln(os.Stderr, abap.Report(results))
}

func (r textReporter) Repairing(round, of int) {
	fmt.Fprintf(os.Stderr, "\nRepair round %d of %d…\n", round, of)
}

func (r textReporter) Done(error) {}

// streamReporter writes newline-delimited JSON events. Agent events can come
// from tool goroutines, so writes are serialized.
type streamReporter struct {
	mu  sync.Mutex
	enc *json.Encoder
}

func newStreamReporter(w io.Writer) *streamReporter {
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	return &streamReporter{enc: enc}
}

func (r *streamReporter) emit(ev any) {
	r.mu.Lock()
	defer r.mu.Unlock()
	_ = r.enc.Encode(ev)
}

type streamObject struct {
	Type     string `json:"type"`
	Name     string `json:"name"`
	OK       bool   `json:"ok"`
	Attempts int    `json:"attempts"`
	Detail   string `json:"detail,omitempty"`
}

type streamVerification struct {
	Type     string   `json:"type"`
	Name     string   `json:"name"`
	Syntax   string   `json:"syntax"`
	Active   string   `json:"active"`
	Tests    string   `json:"tests"`
	Problems []string `json:"problems,omitempty"`
}

func (r *streamReporter) Start(model, system, cwd string) {
	r.emit(struct {
		Type   string `json:"type"`
		Model  string `json:"model"`
		System string `json:"system"`
		Cwd    string `json:"cwd"`
	}{"session", model, system, cwd})
}

func (r *streamReporter) Event(ev agent.Event) {
	switch ev.Type {
	case "text":
		r.emit(struct {
			Type string `json:"type"`
			Text string `json:"text"`
		}{"text", ev.Text})
	case "tool_use":
		var input any = ev.ToolInput
		if json.Valid([]byte(ev.ToolInput)) {
			input = json.RawMessage(ev.ToolInput)
		}
		r.emit(struct {
			Type  string `json:"type"`
			ID    string `json:"id"`
			Name  string `json:"name"`
			Input any    `json:"input"`
		}{"tool_use", ev.ToolID, ev.ToolName, input})
	case "tool_result":
		r.emit(struct {
			Type   string `json:"type"`
			ID     string `json:"id"`
			Name   string `json:"name"`
			OK     bool   `json:"ok"`
			Output string `json:"output"`
		}{"tool_result", ev.ToolID, ev.ToolName, !strings.HasPrefix(ev.ToolOutput, "Error:"), ev.ToolOutput})
	case "result":
		r.emit(struct {
			Type        string `json:"type"`
			StopReason  string `json:"stop_reason"`
			InputTokens int32  `json:"input_tokens"`
		}{"result", ev.StopReason, ev.InputToks})
	case "error":
		r.emit(struct {
			Type  string `json:"type"`
			Error string `json:"error"`
		}{"error", fmt.Sprint(ev.Err)})
	}
}

func (r *streamReporter) Objects(objs []abap.Object) {
	items := make([]streamObject, len(objs))
	for i, o := range objs {
		items[i] = streamObject{Type: o.Type, Name: o.Name, OK: o.OK, Attempts: o.Attempts, Detail: o.Detail}
	}
	r.emit(struct {
		Type  string         `json:"type"`
		Items []streamObject `json:"items"`
	}{"objects", items})
}

func (r *streamReporter) Verifying() {}

func (r *streamReporter) Verified(round int, results []abap.Verification) {
	report := make([]streamVerification, len(results))
	for i, v := range results {
		report[i] = streamVerification{Type: v.Object.Type, Name: v.Object.Name,
			Syntax: v.Syntax, Active: v.Active, Tests: v.Tests, Problems: v.Problems}
	}
	r.emit(struct {
		Type   string               `json:"type"`
		Round  int                  `json:"round"`
		Passed bool                 `json:"passed"`
		Report []streamVerification `json:"report"`
	}{"verify", round, abap.AllPassed(results), report})
}

func (r *streamReporter) Repairing(round, of int) {
	r.emit(struct {
		Type  string `json:"type"`
		Round int    `json:"round"`
		Of    int    `json:"of"`
	}{"repair", round, of})
}

// Done is always the last event; exit matches the process exit code.
func (r *streamReporter) Done(err error) {
	ev := struct {
		Type  string `json:"type"`
		Exit  int    `json:"exit"`
		Error string `json:"error,omitempty"`
	}{Type: "done"}
	if err != nil {
		ev.Exit, ev.Error = 1, err.Error()
	}
	r.emit(ev)
}
