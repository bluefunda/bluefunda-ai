package agentrun

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"sync"

	"github.com/bluefunda/bluefunda-ai/sdk/agent"
)

// Object is an SAP object the agent tried to create or deploy, with the
// outcome of the most recent attempt.
type Object struct {
	Name       string
	Type       string
	SourceFile string
	Action     string // "deploy" or "generate"
	Attempts   int
	OK         bool
	Detail     string // failure reason when !OK
}

func (o Object) key() string { return o.Type + "/" + o.Name }

// Tracker records every `abaper deploy` / `abaper generate` the agent runs
// through its bash tool. It works from the tool calls themselves rather than
// the model's narration, so the summary can't be talked around. Safe for
// concurrent use: the agent loop may execute tools in parallel.
type Tracker struct {
	mu      sync.Mutex
	pending map[string][]Object // tool ID -> invocations awaiting a result
	order   []string
	objects map[string]Object
}

func NewTracker() *Tracker {
	return &Tracker{pending: map[string][]Object{}, objects: map[string]Object{}}
}

// Observe feeds one agent event into the tracker. Pass every event from
// agent.Options.OnEvent.
func (t *Tracker) Observe(ev agent.Event) {
	if ev.ToolName != "bash" {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()

	switch ev.Type {
	case "tool_use":
		var args struct {
			Command string `json:"command"`
		}
		if err := json.Unmarshal([]byte(ev.ToolInput), &args); err != nil {
			return
		}
		if invs := parseInvocations(args.Command); len(invs) > 0 {
			t.pending[ev.ToolID] = invs
		}
	case "tool_result":
		invs, ok := t.pending[ev.ToolID]
		if !ok {
			return
		}
		delete(t.pending, ev.ToolID)
		for _, inv := range invs {
			inv.OK, inv.Detail = outcome(inv, ev.ToolOutput)
			if prev, seen := t.objects[inv.key()]; seen {
				inv.Attempts = prev.Attempts
			} else {
				t.order = append(t.order, inv.key())
			}
			inv.Attempts++
			t.objects[inv.key()] = inv
		}
	}
}

// Objects returns every object touched, in first-seen order, with the
// outcome of its latest attempt.
func (t *Tracker) Objects() []Object {
	t.mu.Lock()
	defer t.mu.Unlock()
	out := make([]Object, 0, len(t.order))
	for _, k := range t.order {
		out = append(out, t.objects[k])
	}
	return out
}

// Summary renders the list of SAP objects the agent changed.
func Summary(objs []Object) string {
	if len(objs) == 0 {
		return "No SAP objects were changed."
	}
	var b strings.Builder
	b.WriteString("SAP objects touched by this run:\n")
	for _, o := range objs {
		mark, status := "✓", pastTense(o.Action)
		if !o.OK {
			mark, status = "✗", o.Action+" failed"
			if o.Detail != "" {
				status += ": " + o.Detail
			}
		}
		attempts := ""
		if o.Attempts > 1 {
			attempts = fmt.Sprintf(" (%d attempts)", o.Attempts)
		}
		fmt.Fprintf(&b, "  %s %-9s %-30s %s%s\n", mark, o.Type, o.Name, status, attempts)
	}
	return strings.TrimRight(b.String(), "\n")
}

func pastTense(action string) string {
	if action == "generate" {
		return "created"
	}
	return "deployed"
}

// outcome decides whether an invocation succeeded from the bash output. The
// bash tool doesn't report exit codes, so this keys off the success line the
// deploy/generate commands print.
func outcome(inv Object, output string) (bool, string) {
	var want string
	if inv.Action == "generate" {
		want = fmt.Sprintf("Successfully created %s %s", inv.Type, inv.Name)
	} else {
		want = fmt.Sprintf("Successfully deployed and activated %s %s", inv.Type, inv.Name)
	}
	if strings.Contains(strings.ToLower(output), strings.ToLower(want)) {
		return true, ""
	}
	return false, failureLine(output)
}

func failureLine(output string) string {
	lines := strings.Split(strings.TrimSpace(output), "\n")
	for _, l := range lines {
		if strings.HasPrefix(strings.TrimSpace(l), "Error") {
			return truncate(strings.TrimSpace(l), 160)
		}
	}
	if len(lines) > 0 {
		return truncate(strings.TrimSpace(lines[len(lines)-1]), 160)
	}
	return ""
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "…"
}

// parseInvocations finds every `abaper deploy` / `abaper generate` in a shell
// command line, including ones chained with &&, ||, ; or |.
func parseInvocations(command string) []Object {
	var out []Object
	for _, seg := range splitSegments(command) {
		words := shellWords(seg)
		i := 0
		for i < len(words) && filepath.Base(words[i]) != "abaper" {
			i++
		}
		if i+1 >= len(words) {
			continue
		}
		action := words[i+1]
		if action != "deploy" && action != "generate" {
			continue
		}
		obj := Object{Action: action, Type: "program"}
		flags := words[i+2:]
		for j := 0; j < len(flags); j++ {
			name, val, hasVal := strings.Cut(flags[j], "=")
			if !hasVal && j+1 < len(flags) {
				val = flags[j+1]
			}
			switch name {
			case "--name":
				obj.Name = strings.ToUpper(val)
			case "--type":
				obj.Type = strings.ToLower(val)
			case "--source-file":
				obj.SourceFile = val
			default:
				continue
			}
			if !hasVal {
				j++
			}
		}
		if obj.Name != "" {
			out = append(out, obj)
		}
	}
	return out
}

func splitSegments(command string) []string {
	r := strings.NewReplacer("&&", "\n", "||", "\n", ";", "\n", "|", "\n")
	return strings.Split(r.Replace(command), "\n")
}

// shellWords splits on whitespace, honouring single and double quotes.
func shellWords(s string) []string {
	var words []string
	var cur strings.Builder
	var quote rune
	inWord := false
	for _, r := range s {
		switch {
		case quote != 0:
			if r == quote {
				quote = 0
			} else {
				cur.WriteRune(r)
			}
		case r == '\'' || r == '"':
			quote, inWord = r, true
		case r == ' ' || r == '\t':
			if inWord {
				words = append(words, cur.String())
				cur.Reset()
				inWord = false
			}
		default:
			cur.WriteRune(r)
			inWord = true
		}
	}
	if inWord {
		words = append(words, cur.String())
	}
	return words
}
