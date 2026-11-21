package cmd

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/pflag"

	"github.com/bluefunda/bluefunda-ai/internal/jsonschema"
	"github.com/bluefunda/bluefunda-ai/internal/ui/tui"
)

// registerHeadlessFlags adds Claude Code's --input-format and --json-schema.
func registerHeadlessFlags(fs *pflag.FlagSet, inputFormat, jsonSchema *string) {
	fs.StringVar(inputFormat, "input-format", "text",
		"Input format for --print: text, or stream-json (one JSON user message per line on stdin; needs --output-format stream-json)")
	fs.StringVar(jsonSchema, "json-schema", "",
		"For --print: the final answer must be JSON matching this JSON Schema (inline JSON or a file path)")
}

// checkPrintFormats rejects flag combinations that can't work.
func checkPrintFormats(inputFormat, outputFormat, prompt string) error {
	switch inputFormat {
	case "", "text":
		return nil
	case "stream-json":
		if outputFormat != "stream-json" {
			return fmt.Errorf("--input-format stream-json needs --output-format stream-json")
		}
		if prompt != "" {
			return fmt.Errorf("--input-format stream-json reads messages from stdin; drop the prompt argument")
		}
		return nil
	}
	return fmt.Errorf("--input-format must be text or stream-json, not %q", inputFormat)
}

// loadJSONSchema compiles --json-schema: inline JSON, or a path to a file.
// "" means no schema.
func loadJSONSchema(arg string) (*jsonschema.Schema, error) {
	arg = strings.TrimSpace(arg)
	if arg == "" {
		return nil, nil
	}
	data := []byte(arg)
	if !strings.HasPrefix(arg, "{") {
		b, err := os.ReadFile(arg)
		if err != nil {
			return nil, fmt.Errorf("--json-schema: %w", err)
		}
		data = b
	}
	s, err := jsonschema.Compile(data)
	if err != nil {
		return nil, fmt.Errorf("--json-schema: %w", err)
	}
	return s, nil
}

// schemaRetries is how many times the model may correct an answer that
// doesn't match --json-schema.
const schemaRetries = 2

// runPrintTurn adds userMsg to history and runs the agentic loop. With a
// schema it asks for a JSON answer, checks it, and gives the model up to
// schemaRetries chances to fix a mismatch; structured is the valid value.
func runPrintTurn(
	history []codeMessage,
	userMsg string,
	schema *jsonschema.Schema,
	ch chan<- tui.StreamEvent,
	loop func([]codeMessage, chan<- tui.StreamEvent) ([]codeMessage, error),
) (_ []codeMessage, structured any, _ error) {
	if schema != nil {
		userMsg += "\n\nWhen you are done, reply with only a JSON value that matches this JSON Schema: no other text and no code fence.\n" + schema.String()
	}
	history = append(history, codeMessage{Role: "user", Content: userMsg})
	history, err := loop(history, ch)
	if err != nil || schema == nil {
		return history, nil, err
	}
	for attempt := 0; ; attempt++ {
		problems := schemaProblems(schema, lastAssistantText(history))
		if len(problems) == 0 {
			v, _ := jsonschema.Extract(lastAssistantText(history))
			return history, v, nil
		}
		if attempt == schemaRetries {
			return history, nil, fmt.Errorf("the answer did not match --json-schema: %s", strings.Join(problems, "; "))
		}
		history = append(history, codeMessage{Role: "user", Content: "Your answer did not match the JSON Schema:\n- " +
			strings.Join(problems, "\n- ") + "\nReply with only the corrected JSON value."})
		if history, err = loop(history, ch); err != nil {
			return history, nil, err
		}
	}
}

// schemaProblems checks an answer against the schema; nil means valid.
func schemaProblems(schema *jsonschema.Schema, answer string) []string {
	v, err := jsonschema.Extract(answer)
	if err != nil {
		return []string{err.Error()}
	}
	return schema.Validate(v)
}

// lastAssistantText is the final answer: the last assistant message with text.
func lastAssistantText(history []codeMessage) string {
	for i := len(history) - 1; i >= 0; i-- {
		if history[i].Role == "assistant" && strings.TrimSpace(history[i].Content) != "" {
			return history[i].Content
		}
	}
	return ""
}

// streamJSONMessage is one line of --input-format stream-json, as Claude
// Code reads it: {"type":"user","message":{"role":"user","content":...}},
// where content is a string or a list of {"type":"text","text":...} blocks.
type streamJSONMessage struct {
	Type    string `json:"type"`
	Message struct {
		Role    string          `json:"role"`
		Content json.RawMessage `json:"content"`
	} `json:"message"`
}

// parseStreamJSONLine returns the user text of one input line.
func parseStreamJSONLine(line []byte) (string, error) {
	var m streamJSONMessage
	if err := json.Unmarshal(line, &m); err != nil {
		return "", fmt.Errorf("invalid JSON: %w", err)
	}
	if m.Type != "user" {
		return "", fmt.Errorf("unsupported message type %q (only \"user\")", m.Type)
	}
	var text string
	if err := json.Unmarshal(m.Message.Content, &text); err == nil {
		return text, nil
	}
	var blocks []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if err := json.Unmarshal(m.Message.Content, &blocks); err != nil {
		return "", fmt.Errorf("message.content must be a string or a list of text blocks")
	}
	var parts []string
	for _, b := range blocks {
		if b.Type == "text" {
			parts = append(parts, b.Text)
		}
	}
	if len(parts) == 0 {
		return "", fmt.Errorf("message has no text content")
	}
	return strings.Join(parts, "\n"), nil
}

// readStreamJSONInput runs a turn for each user message on r, in order,
// until EOF. A bad line is reported on w as an error event and skipped.
func readStreamJSONInput(r io.Reader, w io.Writer, turn func(prompt string)) {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 1<<20), 16<<20)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		text, err := parseStreamJSONLine([]byte(line))
		if err != nil {
			json.NewEncoder(w).Encode(map[string]any{"type": "error", "error": "input: " + err.Error()}) //nolint:errcheck
			continue
		}
		turn(text)
	}
	if err := sc.Err(); err != nil {
		json.NewEncoder(w).Encode(map[string]any{"type": "error", "error": "input: " + err.Error()}) //nolint:errcheck
	}
}
