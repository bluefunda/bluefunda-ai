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

// mergeSlashCommands concatenates command lists, keeping the first command
// of each name.
func mergeSlashCommands(lists ...[]tui.SlashCommand) []tui.SlashCommand {
	seen := map[string]bool{}
	var out []tui.SlashCommand
	for _, l := range lists {
		for _, c := range l {
			if !seen[c.Name] {
				seen[c.Name] = true
				out = append(out, c)
			}
		}
	}
	return out
}

// promptCommands are built-in commands that send a prompt, like Claude
// Code's /init. A file of the same name in .bai/commands replaces one.
func promptCommands() []tui.SlashCommand {
	return []tui.SlashCommand{
		{
			Name:        "/init",
			Description: "Write or improve AGENTS.md with what an agent needs to work in this repo",
			Prompt:      initPrompt,
		},
	}
}

// initPrompt asks for the project instructions file bai loads at session
// start (findContextFile: .bai/context.md, else AGENTS.md).
const initPrompt = `Set up this repository's agent instructions file.

If .bai/context.md exists, that is the file to update; otherwise AGENTS.md at the repository root. If the file exists, read it first and improve it: keep what is still correct, fix what is wrong, add what is missing.

Investigate before writing. Read the build files (Makefile, package.json, go.mod, pyproject.toml, Cargo.toml and the like), the CI configuration, the README and the top-level layout, then write down what a coding agent needs to work here:

- the exact commands to build, test (including a single test), lint and format, taken from the repo, not guessed
- how the code is organised: the main directories and what lives in each
- conventions the code follows that aren't obvious from one file: error handling, naming, test style, commit message format
- traps: generated files not to edit, required environment, slow or flaky steps, anything that surprised you

Keep it short and specific to this repo. Skip generic advice that applies to every project, and never invent a command or rule you did not find evidence for. When you are done, say which file you wrote and summarise what is in it.

$ARGUMENTS`
