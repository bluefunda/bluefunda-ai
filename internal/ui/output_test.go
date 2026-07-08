package ui

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestPrinter_Table_DefaultFormat(t *testing.T) {
	var out bytes.Buffer
	p := &Printer{Out: &out, Err: &bytes.Buffer{}, Format: FormatTable}

	headers := []string{"NAME", "AGE"}
	rows := [][]string{
		{"Alice", "30"},
		{"Bob", "25"},
	}
	p.Table(headers, rows)

	result := out.String()
	if !strings.Contains(result, "NAME") {
		t.Errorf("expected header NAME in output, got: %s", result)
	}
	if !strings.Contains(result, "Alice") {
		t.Errorf("expected Alice in output, got: %s", result)
	}
	if !strings.Contains(result, "Bob") {
		t.Errorf("expected Bob in output, got: %s", result)
	}
	// Verify separator exists
	if !strings.Contains(result, "---") {
		t.Errorf("expected separator in output, got: %s", result)
	}
}

func TestPrinter_Table_JSONFormat(t *testing.T) {
	var out bytes.Buffer
	p := &Printer{Out: &out, Err: &bytes.Buffer{}, Format: FormatJSON}

	headers := []string{"name", "age"}
	rows := [][]string{
		{"Alice", "30"},
		{"Bob", "25"},
	}
	p.Table(headers, rows)

	var records []map[string]string
	if err := json.Unmarshal(out.Bytes(), &records); err != nil {
		t.Fatalf("expected valid JSON, got error: %v\noutput: %s", err, out.String())
	}
	if len(records) != 2 {
		t.Fatalf("expected 2 records, got %d", len(records))
	}
	if records[0]["name"] != "Alice" {
		t.Errorf("expected Alice, got %q", records[0]["name"])
	}
	if records[1]["age"] != "25" {
		t.Errorf("expected 25, got %q", records[1]["age"])
	}
}

func TestPrinter_Table_QuietFormat(t *testing.T) {
	var out bytes.Buffer
	p := &Printer{Out: &out, Err: &bytes.Buffer{}, Format: FormatQuiet}

	headers := []string{"ID", "NAME"}
	rows := [][]string{
		{"123", "Alice"},
		{"456", "Bob"},
	}
	p.Table(headers, rows)

	result := out.String()
	lines := strings.Split(strings.TrimSpace(result), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d: %v", len(lines), lines)
	}
	if lines[0] != "123" {
		t.Errorf("expected '123', got %q", lines[0])
	}
	if lines[1] != "456" {
		t.Errorf("expected '456', got %q", lines[1])
	}
}

func TestPrinter_Success_Quiet(t *testing.T) {
	var errBuf bytes.Buffer
	p := &Printer{Out: &bytes.Buffer{}, Err: &errBuf, Format: FormatQuiet}
	p.Success("done")
	if errBuf.Len() != 0 {
		t.Errorf("expected no output in quiet mode, got: %s", errBuf.String())
	}
}

func TestPrinter_Error_AlwaysShown(t *testing.T) {
	var errBuf bytes.Buffer
	p := &Printer{Out: &bytes.Buffer{}, Err: &errBuf, Format: FormatQuiet}
	p.Error("something broke")
	if !strings.Contains(errBuf.String(), "something broke") {
		t.Errorf("expected error message even in quiet mode, got: %s", errBuf.String())
	}
}

func TestPrinter_Info_Quiet(t *testing.T) {
	var errBuf bytes.Buffer
	p := &Printer{Out: &bytes.Buffer{}, Err: &errBuf, Format: FormatQuiet}
	p.Info("info msg")
	if errBuf.Len() != 0 {
		t.Errorf("expected no info in quiet mode, got: %s", errBuf.String())
	}
}

func TestPrinter_JSON(t *testing.T) {
	var out bytes.Buffer
	p := &Printer{Out: &out, Err: &bytes.Buffer{}, Format: FormatJSON}
	p.JSON(map[string]string{"key": "val"})

	var parsed map[string]string
	if err := json.Unmarshal(out.Bytes(), &parsed); err != nil {
		t.Fatalf("expected valid JSON: %v", err)
	}
	if parsed["key"] != "val" {
		t.Errorf("expected val, got %q", parsed["key"])
	}
}

func TestPrinter_Table_EmptyRows(t *testing.T) {
	var out bytes.Buffer
	p := &Printer{Out: &out, Err: &bytes.Buffer{}, Format: FormatTable}
	p.Table([]string{"A", "B"}, nil)

	result := out.String()
	if !strings.Contains(result, "A") {
		t.Errorf("expected header even with no rows, got: %s", result)
	}
}

func TestPrinter_Table_JSONEmpty(t *testing.T) {
	var out bytes.Buffer
	p := &Printer{Out: &out, Err: &bytes.Buffer{}, Format: FormatJSON}
	p.Table([]string{"A"}, nil)

	var records []map[string]string
	if err := json.Unmarshal(out.Bytes(), &records); err != nil {
		t.Fatalf("expected valid JSON: %v", err)
	}
	if len(records) != 0 {
		t.Errorf("expected empty array, got %d records", len(records))
	}
}

func TestPrinter_Warn_Quiet(t *testing.T) {
	var errBuf bytes.Buffer
	p := &Printer{Out: &bytes.Buffer{}, Err: &errBuf, Format: FormatQuiet}
	p.Warn("warning msg")
	if errBuf.Len() != 0 {
		t.Errorf("expected no warn in quiet mode, got: %s", errBuf.String())
	}
}

func TestPrinter_Warn_Visible(t *testing.T) {
	var errBuf bytes.Buffer
	p := &Printer{Out: &bytes.Buffer{}, Err: &errBuf, Format: FormatTable}
	p.Warn("watch out")
	if !strings.Contains(errBuf.String(), "watch out") {
		t.Errorf("expected warn message, got: %s", errBuf.String())
	}
}

func TestPrinter_Success_Visible(t *testing.T) {
	var errBuf bytes.Buffer
	p := &Printer{Out: &bytes.Buffer{}, Err: &errBuf, Format: FormatTable}
	p.Success("all good")
	if !strings.Contains(errBuf.String(), "all good") {
		t.Errorf("expected success message, got: %s", errBuf.String())
	}
}

func TestPrinter_Info_Visible(t *testing.T) {
	var errBuf bytes.Buffer
	p := &Printer{Out: &bytes.Buffer{}, Err: &errBuf, Format: FormatTable}
	p.Info("useful info")
	if !strings.Contains(errBuf.String(), "useful info") {
		t.Errorf("expected info message, got: %s", errBuf.String())
	}
}

func TestPrinter_ToolCall_Table(t *testing.T) {
	var errBuf bytes.Buffer
	p := &Printer{Out: &bytes.Buffer{}, Err: &errBuf, Format: FormatTable}
	p.ToolCall("read_file", `{"path":"main.go"}`)
	if !strings.Contains(errBuf.String(), "read_file") {
		t.Errorf("expected tool name in output, got: %s", errBuf.String())
	}
}

func TestPrinter_ToolCall_Quiet(t *testing.T) {
	var errBuf bytes.Buffer
	p := &Printer{Out: &bytes.Buffer{}, Err: &errBuf, Format: FormatQuiet}
	p.ToolCall("read_file", `{}`)
	if errBuf.Len() != 0 {
		t.Errorf("expected no output in quiet mode, got: %s", errBuf.String())
	}
}

func TestPrinter_ToolExec_OK(t *testing.T) {
	var errBuf bytes.Buffer
	p := &Printer{Out: &bytes.Buffer{}, Err: &errBuf, Format: FormatTable}
	p.ToolExec("bash", "ok", 150, "exit 0")
	out := errBuf.String()
	if !strings.Contains(out, "bash") {
		t.Errorf("expected 'bash' in output, got: %s", out)
	}
	if !strings.Contains(out, "exit 0") {
		t.Errorf("expected summary in output, got: %s", out)
	}
}

func TestPrinter_ToolExec_Error(t *testing.T) {
	var errBuf bytes.Buffer
	p := &Printer{Out: &bytes.Buffer{}, Err: &errBuf, Format: FormatTable}
	p.ToolExec("bash", "error", 200, "")
	if !strings.Contains(errBuf.String(), "bash") {
		t.Errorf("expected 'bash' in error output, got: %s", errBuf.String())
	}
}

func TestPrinter_ToolExec_Quiet(t *testing.T) {
	var errBuf bytes.Buffer
	p := &Printer{Out: &bytes.Buffer{}, Err: &errBuf, Format: FormatQuiet}
	p.ToolExec("bash", "ok", 100, "done")
	if errBuf.Len() != 0 {
		t.Errorf("expected no output in quiet mode, got: %s", errBuf.String())
	}
}

func TestFormatToolArgs_WithArgs(t *testing.T) {
	got := formatToolArgs(`{"path":"src/main.go"}`)
	if !strings.Contains(got, "path") {
		t.Errorf("expected 'path' in formatted args, got: %s", got)
	}
	if !strings.Contains(got, "src/main.go") {
		t.Errorf("expected value in formatted args, got: %s", got)
	}
}

func TestFormatToolArgs_Empty(t *testing.T) {
	if got := formatToolArgs(""); got != "" {
		t.Errorf("expected empty string for empty args, got: %s", got)
	}
	if got := formatToolArgs("{}"); got != "" {
		t.Errorf("expected empty string for empty object, got: %s", got)
	}
}

func TestFormatToolArgs_LongValue(t *testing.T) {
	long := strings.Repeat("x", 50)
	got := formatToolArgs(`{"key":"` + long + `"}`)
	if len(got) > 90 {
		t.Errorf("expected truncated output, got len=%d: %s", len(got), got)
	}
}
