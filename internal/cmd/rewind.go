package cmd

import (
	"fmt"
	"slices"
	"strings"

	"github.com/bluefunda/bluefunda-ai/internal/checkpoint"
	"github.com/bluefunda/bluefunda-ai/internal/tools"
)

// sessionCheckpoints is the interactive session's checkpoint store, which
// the edit tools save files to before changing them. nil outside an
// interactive session (headless runs have no /rewind).
var sessionCheckpoints *checkpoint.Store

// saveBeforeEdit saves the file an edit tool is about to change, so /rewind
// can restore it.
func saveBeforeEdit(toolName, argsJSON string) {
	if !slices.Contains(editTools, toolName) {
		return
	}
	if path := tools.ExtractPrimaryArg(toolName, argsJSON); path != "" {
		sessionCheckpoints.BeforeEdit(path)
	}
}

// conversationAt reports whether the conversation can still be cut back to
// before p: its message is still where it was added.
func conversationAt(history []codeMessage, p checkpoint.Point) bool {
	return p.HistoryLen >= 0 && p.HistoryLen < len(history) &&
		history[p.HistoryLen].Role == "user" && history[p.HistoryLen].Content == p.Message
}

// rewind restores the code and/or the conversation to before the message
// of the given turn, and says what it did. save persists the conversation.
func rewind(cp *checkpoint.Store, history *[]codeMessage, turn int, code, chat bool, save func()) (string, error) {
	var point *checkpoint.Point
	for _, p := range cp.Points() {
		if p.Turn == turn {
			point = &p
			break
		}
	}
	if point == nil {
		return "", fmt.Errorf("that message is no longer in the conversation")
	}
	if chat && !conversationAt(*history, *point) {
		return "", fmt.Errorf("the conversation was compacted or changed since that message; only code can be restored")
	}
	var parts []string
	if code {
		restored, err := cp.RestoreCode(turn)
		if err != nil {
			return "", fmt.Errorf("restored %d files, but: %w", len(restored), err)
		}
		parts = append(parts, fmt.Sprintf("Restored %d %s", len(restored), pluralWord(len(restored), "file")))
		for _, f := range restored {
			parts[0] += "\n   " + f
		}
	}
	if chat {
		*history = (*history)[:point.HistoryLen]
		save()
		cp.DropPointsFrom(turn)
		parts = append(parts, "Conversation rewound to before: "+oneLine(point.Prompt, 80))
	}
	return strings.Join(parts, "\n"), nil
}

func pluralWord(n int, w string) string {
	if n == 1 {
		return w
	}
	return w + "s"
}

func oneLine(s string, n int) string {
	s = strings.Join(strings.Fields(s), " ")
	if r := []rune(s); len(r) > n {
		return string(r[:n-1]) + "…"
	}
	return s
}
