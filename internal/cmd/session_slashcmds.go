package cmd

import (
	"errors"
	"fmt"
	"sync"

	"github.com/bluefunda/bluefunda-ai/internal/ui/tui"
)

// errTurnRunning is returned by commands that rewrite the session's history
// while a turn is using it.
var errTurnRunning = errors.New("a turn is still running; try again when it finishes")

// compactCommand is /compact [focus]: replace the conversation with a
// summary to free context, as Claude Code's /compact does. turnMu is held by
// every running turn; run does the compaction.
func compactCommand(turnMu *sync.Mutex, run func(focus string) (string, error)) tui.SlashCommand {
	return tui.SlashCommand{
		Name:        "/compact",
		Description: "Summarise the conversation to free context  /compact [what to keep]",
		Run: func(focus string) (string, error) {
			if !turnMu.TryLock() {
				return "", errTurnRunning
			}
			defer turnMu.Unlock()
			return run(focus)
		},
	}
}

// humanTokens formats a token count as "850" or "12.3k".
func humanTokens(n int) string {
	if n < 1000 {
		return fmt.Sprint(n)
	}
	return fmt.Sprintf("%.1fk", float64(n)/1000)
}
