package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bluefunda/bluefunda-ai/internal/ui/tui"
)

const testSchema = `{"type":"object","properties":{"n":{"type":"integer"}},"required":["n"]}`

// fakeLoop answers each turn with the next scripted reply.
func fakeLoop(replies ...string) (func([]codeMessage, chan<- tui.StreamEvent) ([]codeMessage, error), *[]codeMessage) {
	var last []codeMessage
	i := 0
	return func(h []codeMessage, _ chan<- tui.StreamEvent) ([]codeMessage, error) {
		h = append(h, codeMessage{Role: "assistant", Content: replies[i]})
		i++
		last = h
		return h, nil
	}, &last
}

func TestRunPrintTurn_Schema(t *testing.T) {
	schema, err := loadJSONSchema(testSchema)
	if err != nil {
		t.Fatal(err)
	}
	ch := make(chan tui.StreamEvent, 8)

	// Valid on the first answer, even inside a code fence.
	loop, _ := fakeLoop("```json\n{\"n\": 3}\n```")
	h, v, err := runPrintTurn(nil, "count", schema, ch, loop)
	if err != nil || v.(map[string]any)["n"] != float64(3) {
		t.Fatalf("v = %v, err = %v", v, err)
	}
	if !strings.Contains(h[0].Content, "JSON Schema") || !strings.Contains(h[0].Content, `"required":["n"]`) {
		t.Errorf("the prompt should ask for the schema: %q", h[0].Content)
	}

	// Wrong, then corrected: the model is told exactly what was wrong.
	loop, last := fakeLoop(`{"n": "three"}`, `{"n": 3}`)
	_, v, err = runPrintTurn(nil, "count", schema, ch, loop)
	if err != nil || v == nil {
		t.Fatalf("retry: v = %v, err = %v", v, err)
	}
	if fix := (*last)[2].Content; !strings.Contains(fix, "$.n: expected integer, got string") {
		t.Errorf("correction prompt = %q", fix)
	}

	// Still wrong after the retries: an error.
	loop, _ = fakeLoop("no json", "still none", "nope")
	if _, v, err = runPrintTurn(nil, "count", schema, ch, loop); err == nil || v != nil {
		t.Errorf("want an error after %d retries, got v = %v", schemaRetries, v)
	}

	// No schema: the prompt is unchanged and nothing is checked.
	loop, _ = fakeLoop("plain text")
	h, v, err = runPrintTurn(nil, "hi", nil, ch, loop)
	if err != nil || v != nil || h[0].Content != "hi" {
		t.Errorf("no schema: %+v %v %v", h, v, err)
	}
}

func TestLoadJSONSchema(t *testing.T) {
	if s, err := loadJSONSchema(""); s != nil || err != nil {
		t.Error("empty means no schema")
	}
	f := filepath.Join(t.TempDir(), "s.json")
	if err := os.WriteFile(f, []byte(testSchema), 0o644); err != nil {
		t.Fatal(err)
	}
	if s, err := loadJSONSchema(f); err != nil || s == nil {
		t.Errorf("from a file: %v", err)
	}
	if _, err := loadJSONSchema(`{"$ref":"x"}`); err == nil {
		t.Error("unsupported schema accepted")
	}
	if _, err := loadJSONSchema("/no/such/file.json"); err == nil {
		t.Error("missing file accepted")
	}
}

func TestCheckPrintFormats(t *testing.T) {
	ok := [][3]string{{"text", "text", "p"}, {"", "json", "p"}, {"stream-json", "stream-json", ""}}
	for _, c := range ok {
		if err := checkPrintFormats(c[0], c[1], c[2]); err != nil {
			t.Errorf("%q: %v", c, err)
		}
	}
	bad := [][3]string{{"stream-json", "text", ""}, {"stream-json", "stream-json", "a prompt"}, {"xml", "text", "p"}}
	for _, c := range bad {
		if err := checkPrintFormats(c[0], c[1], c[2]); err == nil {
			t.Errorf("%q accepted", c)
		}
	}
}

func TestParseStreamJSONLine(t *testing.T) {
	good := map[string]string{
		`{"type":"user","message":{"role":"user","content":"hi"}}`:                                                    "hi",
		`{"type":"user","message":{"role":"user","content":[{"type":"text","text":"a"},{"type":"text","text":"b"}]}}`: "a\nb",
	}
	for in, want := range good {
		if got, err := parseStreamJSONLine([]byte(in)); err != nil || got != want {
			t.Errorf("%s: %q, %v", in, got, err)
		}
	}
	for _, in := range []string{`nope`, `{"type":"assistant"}`, `{"type":"user","message":{"content":[{"type":"image"}]}}`, `{"type":"user","message":{"content":5}}`} {
		if _, err := parseStreamJSONLine([]byte(in)); err == nil {
			t.Errorf("%s accepted", in)
		}
	}
}

// Each good line is a turn, in order; a bad one is reported and skipped.
func TestReadStreamJSONInput(t *testing.T) {
	in := strings.NewReader(`{"type":"user","message":{"role":"user","content":"one"}}

garbage
{"type":"user","message":{"role":"user","content":"two"}}
`)
	var out bytes.Buffer
	var got []string
	readStreamJSONInput(in, &out, func(p string) { got = append(got, p) })
	if strings.Join(got, ",") != "one,two" {
		t.Errorf("turns = %q", got)
	}
	if !strings.Contains(out.String(), `"type":"error"`) {
		t.Errorf("bad line not reported: %s", out.String())
	}
}

// With a schema, text output is only the validated JSON; json and
// stream-json report it as structured_output.
func TestDrainPrintStreamWith_Structured(t *testing.T) {
	run := func(format string, ok bool) string {
		ch := make(chan tui.StreamEvent, 3)
		ch <- tui.StreamEvent{Kind: "chunk", Chunk: "thinking out loud"}
		ch <- tui.StreamEvent{Kind: "done"}
		close(ch)
		var w, errW bytes.Buffer
		drainPrintStreamWith(ch, format, &w, &errW, func() (any, bool) {
			if !ok {
				return nil, false
			}
			return map[string]any{"n": 3}, true
		})
		return w.String()
	}
	if got := run("text", true); got != "{\n  \"n\": 3\n}\n" {
		t.Errorf("text = %q", got)
	}
	var arr []map[string]any
	if err := json.Unmarshal([]byte(run("json", true)), &arr); err != nil || arr[0]["structured_output"] == nil {
		t.Errorf("json = %v %v", arr, err)
	}
	if got := run("stream-json", false); !strings.Contains(got, `"stop_reason":"error"`) {
		t.Errorf("stream-json without a valid answer = %s", got)
	}
}
