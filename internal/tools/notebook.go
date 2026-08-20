package tools

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
)

// nbSource represents a Jupyter cell's "source" field. nbformat allows it to
// be encoded as either a single string or an array of line strings; this
// type parses both and always re-encodes as an array, matching how Jupyter
// itself writes notebooks.
type nbSource []string

func (s *nbSource) UnmarshalJSON(b []byte) error {
	var lines []string
	if err := json.Unmarshal(b, &lines); err == nil {
		*s = lines
		return nil
	}
	var whole string
	if err := json.Unmarshal(b, &whole); err != nil {
		return fmt.Errorf("cell source must be a string or array of strings: %w", err)
	}
	*s = splitSourceLines(whole)
	return nil
}

func (s nbSource) MarshalJSON() ([]byte, error) {
	lines := []string(s)
	if lines == nil {
		lines = []string{}
	}
	return json.Marshal(lines)
}

func (s nbSource) String() string {
	return strings.Join(s, "")
}

// splitSourceLines splits content into nbformat-style lines: every line
// except a trailing blank one keeps its "\n" terminator.
func splitSourceLines(content string) nbSource {
	if content == "" {
		return nbSource{}
	}
	parts := strings.SplitAfter(content, "\n")
	if len(parts) > 0 && parts[len(parts)-1] == "" {
		parts = parts[:len(parts)-1]
	}
	return nbSource(parts)
}

// nbOutput is one Jupyter cell output. Fields are a superset across
// output_type values (stream, execute_result, display_data, error); only the
// ones relevant to a given type are populated when read.
type nbOutput struct {
	OutputType string                     `json:"output_type"`
	Name       string                     `json:"name,omitempty"`
	Text       nbSource                   `json:"text,omitempty"`
	Data       map[string]json.RawMessage `json:"data,omitempty"`
	EName      string                     `json:"ename,omitempty"`
	EValue     string                     `json:"evalue,omitempty"`
}

// cellRaw is a notebook cell kept as a generic field map so editing one
// aspect (source, outputs, cell_type) never drops fields this package
// doesn't know about — cell ids, tags, widget metadata, attachments, and
// anything added by newer nbformat versions all round-trip unchanged.
type cellRaw map[string]json.RawMessage

func (c cellRaw) cellType() string {
	var s string
	_ = json.Unmarshal(c["cell_type"], &s)
	return s
}

func (c cellRaw) source() nbSource {
	var s nbSource
	_ = json.Unmarshal(c["source"], &s)
	return s
}

func (c cellRaw) outputs() []nbOutput {
	var out []nbOutput
	_ = json.Unmarshal(c["outputs"], &out)
	return out
}

func (c cellRaw) executionCount() (int, bool) {
	raw, ok := c["execution_count"]
	if !ok || string(raw) == "null" {
		return 0, false
	}
	var n int
	if json.Unmarshal(raw, &n) != nil {
		return 0, false
	}
	return n, true
}

func (c cellRaw) setSource(s nbSource) {
	b, _ := json.Marshal(s)
	c["source"] = b
}

func (c cellRaw) setCellType(t string) {
	b, _ := json.Marshal(t)
	c["cell_type"] = b
}

// clearOutputs removes execution outputs. Non-code cells (markdown, raw)
// don't carry these fields at all under nbformat, so they're deleted rather
// than nulled when the cell isn't code.
func (c cellRaw) clearOutputs() {
	if c.cellType() == "code" {
		c["outputs"] = json.RawMessage("[]")
		c["execution_count"] = json.RawMessage("null")
		return
	}
	delete(c, "outputs")
	delete(c, "execution_count")
}

func newCellRaw(cellType string, src nbSource) cellRaw {
	c := cellRaw{}
	c.setCellType(cellType)
	c.setSource(src)
	c["metadata"] = json.RawMessage("{}")
	c["id"] = json.RawMessage(fmt.Sprintf("%q", newCellID()))
	if cellType == "code" {
		c["outputs"] = json.RawMessage("[]")
		c["execution_count"] = json.RawMessage("null")
	}
	return c
}

// newCellID returns a short id in the style Jupyter itself generates for
// nbformat >= 4.5, which requires every cell to have a unique id.
func newCellID() string {
	return strings.ReplaceAll(uuid.New().String(), "-", "")[:8]
}

// notebookDoc is a Jupyter notebook file kept as a generic field map for the
// same reason as cellRaw: only "cells" is ever rewritten, everything else
// (top-level metadata, nbformat version) passes through untouched.
type notebookDoc map[string]json.RawMessage

func readNotebookDoc(path string) (notebookDoc, []cellRaw, error) {
	if path == "" {
		return nil, nil, fmt.Errorf("path is required")
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, fmt.Errorf("read %s: %w", path, err)
	}
	var doc notebookDoc
	if err := json.Unmarshal(b, &doc); err != nil {
		return nil, nil, fmt.Errorf("parse %s as a Jupyter notebook: %w", path, err)
	}
	var cells []cellRaw
	if raw, ok := doc["cells"]; ok {
		if err := json.Unmarshal(raw, &cells); err != nil {
			return nil, nil, fmt.Errorf("parse cells in %s: %w", path, err)
		}
	}
	return doc, cells, nil
}

func writeNotebookDoc(path string, doc notebookDoc, cells []cellRaw) error {
	if cells == nil {
		cells = []cellRaw{}
	}
	b, err := json.Marshal(cells)
	if err != nil {
		return fmt.Errorf("encode cells: %w", err)
	}
	doc["cells"] = b

	out, err := json.MarshalIndent(doc, "", " ")
	if err != nil {
		return fmt.Errorf("encode notebook: %w", err)
	}

	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".bai-notebook-*")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(out); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return fmt.Errorf("write temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("close temp file: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("rename to %s: %w", path, err)
	}
	return nil
}

// notebookLanguage returns the kernel language name from notebook metadata
// (e.g. "python"), or "" if not present.
func notebookLanguage(doc notebookDoc) string {
	var meta struct {
		KernelSpec struct {
			Language string `json:"language"`
		} `json:"kernelspec"`
		LanguageInfo struct {
			Name string `json:"name"`
		} `json:"language_info"`
	}
	raw, ok := doc["metadata"]
	if !ok {
		return ""
	}
	if json.Unmarshal(raw, &meta) != nil {
		return ""
	}
	if meta.LanguageInfo.Name != "" {
		return meta.LanguageInfo.Name
	}
	return meta.KernelSpec.Language
}

const outputSummaryMaxLen = 2000

// summarizeOutputs renders a code cell's outputs as short, readable text:
// stream text and text/plain results verbatim (capped), other mime types by
// name only, and errors as "ename: evalue".
func summarizeOutputs(outputs []nbOutput) string {
	var parts []string
	for _, o := range outputs {
		switch o.OutputType {
		case "stream":
			parts = append(parts, o.Text.String())
		case "execute_result", "display_data":
			if text, ok := o.Data["text/plain"]; ok {
				var s nbSource
				if json.Unmarshal(text, &s) == nil {
					parts = append(parts, s.String())
					continue
				}
			}
			for mime := range o.Data {
				if mime != "text/plain" {
					parts = append(parts, fmt.Sprintf("[%s output]", mime))
					break
				}
			}
		case "error":
			parts = append(parts, fmt.Sprintf("%s: %s", o.EName, o.EValue))
		}
	}
	summary := strings.Join(parts, "\n")
	if len(summary) > outputSummaryMaxLen {
		summary = summary[:outputSummaryMaxLen] + "... (truncated)"
	}
	return summary
}

// indentLines splits content into lines prefixed with 4 spaces. Returns nil
// for empty content.
func indentLines(content string) []string {
	if content == "" {
		return nil
	}
	lines := strings.Split(strings.TrimSuffix(content, "\n"), "\n")
	out := make([]string, len(lines))
	for i, l := range lines {
		out[i] = "    " + l
	}
	return out
}

// ReadNotebook renders a Jupyter notebook's cells as structured text: index,
// cell type, execution state, source, and a short output summary for code
// cells. This is a read-only rendering — non-text outputs (images, HTML,
// widgets) are named, not reproduced.
func ReadNotebook(path string) (string, error) {
	doc, cells, err := readNotebookDoc(path)
	if err != nil {
		return "", err
	}
	lang := notebookLanguage(doc)

	var sb strings.Builder
	fmt.Fprintf(&sb, "Notebook: %s (%d cells", path, len(cells))
	if lang != "" {
		fmt.Fprintf(&sb, ", language: %s", lang)
	}
	sb.WriteString(")\n")

	for i, c := range cells {
		sb.WriteString("\n")
		ct := c.cellType()
		if ct == "code" {
			state := "not executed"
			if n, ok := c.executionCount(); ok {
				state = fmt.Sprintf("executed [%d]", n)
			}
			fmt.Fprintf(&sb, "[%d] code (%s)\n", i, state)
		} else {
			fmt.Fprintf(&sb, "[%d] %s\n", i, ct)
		}
		for _, line := range indentLines(c.source().String()) {
			sb.WriteString(line)
			sb.WriteString("\n")
		}
		if summary := summarizeOutputs(c.outputs()); summary != "" {
			sb.WriteString("    --- output ---\n")
			for _, line := range indentLines(summary) {
				sb.WriteString(line)
				sb.WriteString("\n")
			}
		}
	}
	return sb.String(), nil
}

// EditNotebook applies one edit to a Jupyter notebook by cell index:
//   - "replace" (default): overwrite cellIndex's source (and cellType, if
//     non-empty), and clear that cell's outputs/execution_count since they
//     described the now-replaced source.
//   - "insert": insert a new cell of cellType (default "code") with
//     newSource before cellIndex (cellIndex == len(cells) appends at the end).
//   - "delete": remove cellIndex.
//   - "clear_outputs": clear cellIndex's outputs/execution_count without
//     touching its source.
func EditNotebook(path string, cellIndex int, editMode, newSource, cellType string) (string, error) {
	doc, cells, err := readNotebookDoc(path)
	if err != nil {
		return "", err
	}

	switch editMode {
	case "", "replace":
		if err := checkCellIndex(cells, cellIndex); err != nil {
			return "", err
		}
		c := cells[cellIndex]
		c.setSource(splitSourceLines(newSource))
		if cellType != "" {
			c.setCellType(cellType)
		}
		c.clearOutputs()
	case "insert":
		if cellIndex < 0 || cellIndex > len(cells) {
			return "", fmt.Errorf("cell_index %d out of range for insert (notebook has %d cells)", cellIndex, len(cells))
		}
		ct := cellType
		if ct == "" {
			ct = "code"
		}
		newCell := newCellRaw(ct, splitSourceLines(newSource))
		cells = append(cells, nil)
		copy(cells[cellIndex+1:], cells[cellIndex:])
		cells[cellIndex] = newCell
	case "delete":
		if err := checkCellIndex(cells, cellIndex); err != nil {
			return "", err
		}
		cells = append(cells[:cellIndex], cells[cellIndex+1:]...)
	case "clear_outputs":
		if err := checkCellIndex(cells, cellIndex); err != nil {
			return "", err
		}
		cells[cellIndex].clearOutputs()
	default:
		return "", fmt.Errorf("unknown edit_mode %q: want replace, insert, delete, or clear_outputs", editMode)
	}

	if err := writeNotebookDoc(path, doc, cells); err != nil {
		return "", err
	}
	return fmt.Sprintf("%s cell %d in %s (%d cells total)", editModeVerb(editMode), cellIndex, path, len(cells)), nil
}

func checkCellIndex(cells []cellRaw, i int) error {
	if i < 0 || i >= len(cells) {
		return fmt.Errorf("cell_index %d out of range (notebook has %d cells)", i, len(cells))
	}
	return nil
}

func editModeVerb(mode string) string {
	switch mode {
	case "insert":
		return "inserted"
	case "delete":
		return "deleted"
	case "clear_outputs":
		return "cleared outputs of"
	default:
		return "replaced"
	}
}
