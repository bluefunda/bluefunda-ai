package tui

import "strings"

// SlashCommand is a command triggered by typing / in the input.
type SlashCommand struct {
	Name        string
	Description string
	Alias       string
	// Prompt is non-empty for custom commands loaded from .bai/commands/*.md.
	// When set, submitting the command sends Prompt as the user message.
	Prompt string
}

// builtinCommands is the canonical list shown in the autocomplete menu.
var builtinCommands = []SlashCommand{
	{"/help", "Show keyboard shortcuts and commands", "", ""},
	{"/clear", "Clear the conversation display", "", ""},
	{"/new", "Start a fresh session with a new ID", "", ""},
	{"/reset", "Clear messages (keep session ID)", "", ""},
	{"/model", "Show or switch the active model  /model gpt-4", "", ""},
	{"/sessions", "List recent sessions", "", ""},
	{"/resume", "Resume a session by ID or number  /resume 2", "", ""},
	{"/code", "Switch to code mode and load file system tools", "", ""},
	{"/chat", "Switch to chat mode and unload file tools", "", ""},
	{"/auto", "Toggle auto-apply for code tools (code sessions only)", "", ""},
	{"/mcp", "List or activate MCP servers  /mcp github", "", ""},
	{"/account", "Show account info (name, email)", "", ""},
	{"/usage", "Show token usage and rate limits", "", ""},
	{"/tools", "List available tools", "", ""},
	{"/context", "Show current context info", "", ""},
	{"/update", "Check for a newer bai version and upgrade", "", ""},
	{"/exit", "Quit the session", "quit", ""},
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
