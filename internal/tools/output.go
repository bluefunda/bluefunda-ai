package tools

import (
	"fmt"
	"strings"
)

// MaxOutputChars caps the tool output sent back to the model. A single
// command can print megabytes (cat-ing logs, a broad find), and one such
// result fills the context window. Claude Code caps Bash output at 30 000
// characters by default; the same limit applies here to every tool.
const MaxOutputChars = 30_000

// CapOutput returns s unchanged when it fits in limit characters. Otherwise
// it keeps the start and the end — where commands print what they are doing
// and how it ended — and replaces the middle with a note telling the model
// how much was dropped and how to see less.
func CapOutput(s string, limit int) string {
	if len(s) <= limit {
		return s
	}
	note := fmt.Sprintf("\n\n[... %d characters omitted: output too large. "+
		"Narrow the command (grep, head, tail, a smaller path) or read the file in parts with offset/limit ...]\n\n",
		len(s)-limit)
	head, tail := limit*2/3, limit/3
	return strings.ToValidUTF8(s[:head], "") + note + strings.ToValidUTF8(s[len(s)-tail:], "")
}
