package tui

import (
	"strings"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/formatters"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/alecthomas/chroma/v2/styles"
	"github.com/charmbracelet/x/ansi"
)

// renderCodeBlock renders a fenced code block the way Claude Code does:
// syntax-highlighted and indented like the prose around it, with no border
// or label, so selecting it copies the code and nothing else. Lines longer
// than the width wrap. Streaming-safe (works on partial/unclosed fences).
func renderCodeBlock(lang, code string, width int, _ Theme) string {
	code = strings.TrimRight(code, "\n")
	// A tab is one column to the wrapper but up to eight on screen.
	code = strings.ReplaceAll(code, "\t", "    ")
	highlighted := highlight(lang, code)
	if inner := width - len(codeIndent); inner >= 10 {
		highlighted = ansi.Hardwrap(highlighted, inner, true)
	}
	return indentLines(highlighted, codeIndent)
}

// codeIndent lines code up with glamour's paragraph margin.
const codeIndent = "  "

// highlight applies chroma syntax highlighting and returns the result with
// ANSI color codes. Falls back to plain text if the language is unknown.
func highlight(lang, code string) string {
	lexer := lexers.Get(lang)
	if lexer == nil {
		lexer = lexers.Fallback
	}
	lexer = chroma.Coalesce(lexer)

	// Use the "monokai" style — dark, readable, matches our palette.
	// Other good options: "dracula", "nord", "onedark", "github-dark"
	style := styles.Get("monokai")
	if style == nil {
		style = styles.Fallback
	}

	formatter := formatters.Get("terminal256")
	if formatter == nil {
		return code
	}

	iter, err := lexer.Tokenise(nil, code)
	if err != nil {
		return code
	}

	var buf strings.Builder
	if err := formatter.Format(&buf, style, iter); err != nil {
		return code
	}

	result := buf.String()
	// chroma terminal256 appends a trailing reset + newline; trim it
	result = strings.TrimRight(result, "\n")
	return result
}

// ─── Markdown splitter ───────────────────────────────────────────────────────

// segment is one piece of a markdown document: either prose or a code block.
type segment struct {
	isCode bool
	lang   string
	body   string
}

// splitMarkdown splits markdown text into alternating prose and fenced code
// block segments. Handles streaming: an unclosed fence is treated as an
// in-progress code block (so it renders rather than leaking raw markdown).
func splitMarkdown(md string) []segment {
	var segs []segment
	lines := strings.Split(md, "\n")

	var proseBuf, codeBuf strings.Builder
	inCode := false
	lang := ""

	flushProse := func() {
		if proseBuf.Len() > 0 {
			segs = append(segs, segment{body: proseBuf.String()})
			proseBuf.Reset()
		}
	}
	flushCode := func() {
		segs = append(segs, segment{isCode: true, lang: lang, body: codeBuf.String()})
		codeBuf.Reset()
		lang = ""
	}

	for _, line := range lines {
		if !inCode {
			if strings.HasPrefix(line, "```") {
				flushProse()
				lang = strings.TrimSpace(strings.TrimPrefix(line, "```"))
				inCode = true
			} else {
				proseBuf.WriteString(line)
				proseBuf.WriteByte('\n')
			}
		} else {
			if strings.TrimSpace(line) == "```" {
				flushCode()
				inCode = false
			} else {
				codeBuf.WriteString(line)
				codeBuf.WriteByte('\n')
			}
		}
	}

	// Flush remaining — an open fence is an in-progress streaming block
	flushProse()
	if codeBuf.Len() > 0 {
		flushCode()
	}

	return segs
}

// codeBlocksIn returns the body of every fenced code block in content, in
// document order, trimmed of the trailing newline splitMarkdown's line
// accumulation leaves (same trim renderCodeBlock already applies). Used by
// /copy code [n] (#288).
func codeBlocksIn(content string) []string {
	segs := splitMarkdown(content)
	var blocks []string
	for _, s := range segs {
		if s.isCode {
			blocks = append(blocks, strings.TrimRight(s.body, "\n"))
		}
	}
	return blocks
}
