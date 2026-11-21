package tui

import (
	"fmt"
	"strings"
)

// SlashCommand is a command triggered by typing / in the input.
type SlashCommand struct {
	Name        string
	Description string
	Alias       string
	// Prompt is non-empty for custom commands loaded from .bai/commands/*.md.
	// When set, submitting the command sends Prompt as the user message.
	Prompt string
	// Run is set for commands a product registers in Go (e.g. abaper's
	// /system). It runs off the UI goroutine with the text typed after the
	// command; the returned text is shown as a system message.
	Run func(args string) (string, error)
	// Instant runs Run on the UI goroutine and shows only its output: for
	// quick local commands (no network) that read UI-owned state.
	Instant bool
}

// builtinCommands is the canonical list shown in the autocomplete menu.
var builtinCommands = []SlashCommand{
	{Name: "/help", Description: "Show keyboard shortcuts and commands"},
	{Name: "/clear", Description: "Clear the conversation display"},
	{Name: "/new", Description: "Start a fresh session with a new ID"},
	{Name: "/reset", Description: "Clear messages (keep session ID)"},
	{Name: "/model", Description: "Show or switch the active model  /model gpt-4"},
	{Name: "/copy", Description: "Copy the last response, /copy <n> for an earlier one, /copy code [n] for a code block"},
	{Name: "/retry", Description: "Resend the last message, replacing the response  /retry --model gpt-4"},
	{Name: "/sessions", Description: "List recent sessions"},
	{Name: "/resume", Description: "Resume a session by ID or number  /resume 2"},
	{Name: "/code", Description: "Switch to code mode and load file system tools"},
	{Name: "/chat", Description: "Switch to chat mode and unload file tools"},
	{Name: "/auto", Description: "Toggle auto-apply for code tools (code sessions only)"},
	{Name: "/max-turns", Description: "Show or set the agentic step limit for this session  /max-turns 100"},
	{Name: "/plan", Description: "Toggle plan mode — read-only tools until you exit it"},
	{Name: "/mcp", Description: "List or activate MCP servers  /mcp github"},
	{Name: "/memory", Description: "List, show, or delete memory entries  /memory <key> | /memory delete <key>"},
	{Name: "/account", Description: "Show account info (name, email)"},
	{Name: "/usage", Description: "Show token usage and rate limits"},
	{Name: "/tools", Description: "List available tools"},
	{Name: "/context", Description: "Show current context info"},
	{Name: "/status", Description: "Show version, model, mode, session and settings"},
	{Name: "/rewind", Description: "Restore code and/or conversation to before an earlier message (Esc Esc)"},
	{Name: "/update", Description: "Check for a newer version and upgrade"},
	{Name: "/exit", Description: "Quit the session", Alias: "quit"},
}

// expandArguments substitutes every "$ARGUMENTS" placeholder in a custom
// command's prompt with the text typed after the command name. A prompt with
// no placeholder simply doesn't accept an argument — args are dropped.
func expandArguments(prompt, args string) string {
	return strings.ReplaceAll(prompt, "$ARGUMENTS", args)
}

// matchSlashCommands returns commands matching filter from both builtins and
// custom commands. If filter is empty or just "/" all commands are returned.
func matchSlashCommands(filter string, custom []SlashCommand) []SlashCommand {
	all := append(builtinCommands, custom...)
	filter = strings.TrimPrefix(filter, "/")
	filter = strings.ToLower(filter)
	if filter == "" {
		return all
	}
	var out []SlashCommand
	for _, c := range all {
		if strings.Contains(strings.ToLower(c.Name), filter) ||
			strings.Contains(strings.ToLower(c.Description), filter) {
			out = append(out, c)
		}
	}
	return out
}

// statusText is /status: what this session is running with, like Claude
// Code's /status.
func (m Model) statusText() string {
	mode := m.mode().describe()
	turns := "no limit"
	if m.cfg.MaxTurns > 0 {
		turns = fmt.Sprintf("%d steps", m.cfg.MaxTurns)
	}
	model := m.modelName()
	if m.cfg.Model == "" {
		model += " (chosen by the server)"
	}
	lines := []string{
		"Version:    " + m.cfg.Version,
		"Model:      " + model,
		"Session:    " + m.cfg.ChatID,
		"Directory:  " + m.cfg.WorkDir,
	}
	if m.cfg.IsCode {
		lines = append(lines, "Mode:       "+mode, "Max turns:  "+turns)
	}
	if m.cfg.StatusFn != nil {
		lines = append(lines, m.cfg.StatusFn()...)
	}
	return strings.Join(lines, "\n")
}

// modelName is the session's model for display; with none chosen, the
// server picks one.
func (m Model) modelName() string {
	if m.cfg.Model == "" {
		return "default"
	}
	return m.cfg.Model
}
