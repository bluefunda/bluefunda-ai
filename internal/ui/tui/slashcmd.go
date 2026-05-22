package tui

import "strings"

// SlashCommand is a command triggered by typing / in the input.
type SlashCommand struct {
	Name        string
	Description string
	Alias       string
}

// builtinCommands is the canonical list shown in the autocomplete menu.
var builtinCommands = []SlashCommand{
	{"/help", "Show keyboard shortcuts and commands", ""},
	{"/clear", "Clear the conversation display", ""},
	{"/reset", "Start a new session", ""},
	{"/model", "Show or set the active model", ""},
	{"/cost", "Show estimated token usage", ""},
	{"/tools", "List available tools", ""},
	{"/context", "Show current context info", ""},
	{"/exit", "Quit the session", "quit"},
}

// matchSlashCommands returns commands whose Name contains filter (case-insensitive).
// If filter is empty or just "/" all commands are returned.
func matchSlashCommands(filter string) []SlashCommand {
	filter = strings.TrimPrefix(filter, "/")
	filter = strings.ToLower(filter)
	if filter == "" {
		return builtinCommands
	}
	var out []SlashCommand
	for _, c := range builtinCommands {
		if strings.Contains(strings.ToLower(c.Name), filter) ||
			strings.Contains(strings.ToLower(c.Description), filter) {
			out = append(out, c)
		}
	}
	return out
}
