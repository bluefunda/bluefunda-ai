package slash

import tea "github.com/charmbracelet/bubbletea"

// ClearMsg tells the chat model to clear conversation history.
type ClearMsg struct{}

// QuitMsg tells the app to exit.
type QuitMsg struct{}

// HelpMsg tells the chat model to show help text.
type HelpMsg struct{}

// SystemOpenMsg tells the app to open the SAP system form.
// EditID is empty for "add", or a system ID/name for "edit".
type SystemOpenMsg struct{ EditID string }

// ObjectSearchMsg tells the app to search for ABAP objects.
// Pattern is the search pattern (SAP wildcards, e.g. ZMY*).
// ObjectType is optional, e.g. "PROG/P", "CLAS/OC".
type ObjectSearchMsg struct {
	Pattern    string
	ObjectType string
}

// SourcePreviewMsg tells the app to fetch and display an object's source.
type SourcePreviewMsg struct {
	Name       string
	ObjectType string
}

// StatusMsg tells the app to show login/auth and SAP system connectivity status.
type StatusMsg struct{}

// UnknownCmdMsg is returned when a slash command is not found.
type UnknownCmdMsg struct{ Name string }

// Command is a slash command entry.
type Command struct {
	Name        string
	Description string
	Aliases     []string
	Handler     func(args []string) tea.Cmd
}

// Registry is the list of all registered slash commands.
var Registry = []Command{
	{
		Name:        "help",
		Description: "Show all commands",
		Handler:     func(_ []string) tea.Cmd { return func() tea.Msg { return HelpMsg{} } },
	},
	{
		Name:        "clear",
		Description: "Clear conversation history",
		Handler:     func(_ []string) tea.Cmd { return func() tea.Msg { return ClearMsg{} } },
	},
	{
		Name:        "system",
		Description: "Manage SAP systems: add, list, edit <name>",
		Handler: func(args []string) tea.Cmd {
			// /system list → show list in chat (handled in app.go via SystemOpenMsg with special ID)
			// /system edit <name> → open form pre-filled
			// /system or /system add → open blank form
			sub := ""
			if len(args) > 0 {
				sub = args[0]
			}
			switch sub {
			case "edit":
				id := ""
				if len(args) > 1 {
					id = args[1]
				}
				return func() tea.Msg { return SystemOpenMsg{EditID: id} }
			case "list":
				return func() tea.Msg { return SystemOpenMsg{EditID: "list"} }
			default: // "add" or bare /system
				return func() tea.Msg { return SystemOpenMsg{EditID: ""} }
			}
		},
	},
	{
		Name:        "status",
		Description: "Show login and SAP system connection status",
		Handler:     func(_ []string) tea.Cmd { return func() tea.Msg { return StatusMsg{} } },
	},
	{
		Name:        "compact",
		Description: "Summarize & compress context",
		Handler:     func(_ []string) tea.Cmd { return nil },
	},
	{
		Name:        "source",
		Description: "Preview object source: /source <name> <type>, e.g. /source ZHELLO PROG/P",
		Handler: func(args []string) tea.Cmd {
			name := ""
			objType := ""
			if len(args) > 0 {
				name = args[0]
			}
			if len(args) > 1 {
				objType = args[1]
			}
			return func() tea.Msg { return SourcePreviewMsg{Name: name, ObjectType: objType} }
		},
	},
	{
		Name:        "object",
		Description: "Search ABAP objects: /object <pattern> [type], e.g. /object ZMY* PROG/P",
		Handler: func(args []string) tea.Cmd {
			pattern := ""
			objType := ""
			if len(args) > 0 {
				pattern = args[0]
			}
			if len(args) > 1 {
				objType = args[1]
			}
			return func() tea.Msg { return ObjectSearchMsg{Pattern: pattern, ObjectType: objType} }
		},
	},
	{
		Name:        "activate",
		Description: "Activate current object",
		Handler:     func(_ []string) tea.Cmd { return nil },
	},
	{
		Name:        "transport",
		Description: "Add to transport request",
		Handler:     func(_ []string) tea.Cmd { return nil },
	},
	{
		Name:        "diff",
		Description: "Show full diff view",
		Handler:     func(_ []string) tea.Cmd { return nil },
	},
	{
		Name:        "review",
		Description: "Code review mode",
		Handler:     func(_ []string) tea.Cmd { return nil },
	},
	{
		Name:        "test",
		Description: "Generate ABAP unit tests",
		Handler:     func(_ []string) tea.Cmd { return nil },
	},
	{
		Name:        "model",
		Description: "Switch LLM model",
		Handler:     func(_ []string) tea.Cmd { return nil },
	},
	{
		Name:        "cost",
		Description: "Show token usage + cost",
		Handler:     func(_ []string) tea.Cmd { return nil },
	},
	{
		Name:        "settings",
		Description: "Open settings",
		Handler:     func(_ []string) tea.Cmd { return nil },
	},
	{
		Name:        "logout",
		Description: "Log out",
		Handler:     func(_ []string) tea.Cmd { return nil },
	},
	{
		Name:        "quit",
		Description: "Exit ABAPer",
		Aliases:     []string{"q", "exit"},
		Handler:     func(_ []string) tea.Cmd { return func() tea.Msg { return QuitMsg{} } },
	},
}

// Lookup finds a command by name or alias. Returns nil if not found.
func Lookup(name string) *Command {
	for i := range Registry {
		c := &Registry[i]
		if c.Name == name {
			return c
		}
		for _, alias := range c.Aliases {
			if alias == name {
				return c
			}
		}
	}
	return nil
}
