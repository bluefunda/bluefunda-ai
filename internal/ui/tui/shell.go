package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// shellDoneMsg carries the output of a "!" command.
type shellDoneMsg struct {
	output string
	err    error
}

// shellCommand reports whether input is a "!<command>" shell command and
// returns the command.
func shellCommand(input string) (string, bool) {
	rest, ok := strings.CutPrefix(input, "!")
	rest = strings.TrimSpace(rest)
	return rest, ok && rest != ""
}

// runShell shows the command as typed and runs it off the UI goroutine.
func (m Model) runShell(input, command string) (tea.Model, tea.Cmd) {
	m.textarea.Reset()
	m.showSlash = false
	m.historyIdx = -1
	if len(m.inputHistory) == 0 || m.inputHistory[len(m.inputHistory)-1] != input {
		m.inputHistory = append(m.inputHistory, input)
	}
	m.messages = append(m.messages, newUserMessage("! "+command))
	fn := m.cfg.ShellFn
	return m, func() tea.Msg {
		out, err := fn(command)
		return shellDoneMsg{output: out, err: err}
	}
}

// shellDisplayLines caps how much of a command's output is shown; the model
// still gets all of it (up to the tool output cap).
const shellDisplayLines = 40

func formatShellOutput(out string, err error) string {
	if err != nil {
		return "  ⎿ " + err.Error()
	}
	out = strings.TrimRight(out, "\n")
	if out == "" {
		return "  ⎿ (no output)"
	}
	lines := strings.Split(out, "\n")
	more := 0
	if len(lines) > shellDisplayLines {
		more = len(lines) - shellDisplayLines
		lines = lines[:shellDisplayLines]
	}
	for i, l := range lines {
		prefix := "    "
		if i == 0 {
			prefix = "  ⎿ "
		}
		lines[i] = prefix + l
	}
	if more > 0 {
		lines = append(lines, fmt.Sprintf("    … %d more lines", more))
	}
	return strings.Join(lines, "\n")
}
