package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/bluefunda/abaper/internal/client"
	"github.com/bluefunda/abaper/internal/config"
	"github.com/bluefunda/abaper/internal/health"
	"github.com/bluefunda/abaper/tui/slash"
)

// Model is the root Bubble Tea model.
type Model struct {
	view       appView
	chat       *chatModel
	systemForm *systemFormModel
	width      int
	height     int
}

type appView int

const (
	viewChat appView = iota
	viewSystemForm
)

// New creates the root TUI model.
func New(version string) *Model {
	return &Model{
		chat: newChatModel(version),
	}
}

func (m *Model) Init() tea.Cmd {
	return m.chat.Init()
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// System form is active — route messages there first
	if m.view == viewSystemForm && m.systemForm != nil {
		switch msg := msg.(type) {
		case tea.WindowSizeMsg:
			m.width = msg.Width
			m.height = msg.Height
			m.systemForm.width = msg.Width
			m.systemForm.height = msg.Height
			return m, nil

		case SystemFormSavedMsg:
			return m, m.handleSystemSaved(msg.System)

		case SystemFormCancelledMsg:
			m.view = viewChat
			m.systemForm = nil
			return m, nil
		}

		newForm, cmd := m.systemForm.Update(msg)
		m.systemForm = newForm.(*systemFormModel)
		return m, cmd
	}

	// Chat view
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.chat.SetSize(msg.Width, msg.Height)
		return m, nil

	case tea.KeyMsg:
		if !m.chat.streaming {
			switch msg.Type {
			case tea.KeyCtrlC, tea.KeyCtrlD:
				return m, tea.Quit
			}
		}

	case slash.QuitMsg:
		return m, tea.Quit

	case slash.ClearMsg:
		m.chat.messages = nil
		m.chat.rebuildViewport()
		return m, nil

	case slash.HelpMsg:
		m.chat.messages = append(m.chat.messages, chatMessage{
			kind:    kindSystem,
			content: helpText,
		})
		m.chat.rebuildViewport()
		return m, nil

	case slash.SystemOpenMsg:
		if msg.EditID == "list" {
			return m, m.showSystemList()
		}
		var existing *config.SAPSystem
		if msg.EditID != "" {
			if sysCfg, err := config.LoadSystems(); err == nil {
				existing = sysCfg.FindByNameOrID(msg.EditID)
			}
		}
		m.systemForm = newSystemForm(existing)
		m.systemForm.width = m.width
		m.systemForm.height = m.height
		m.view = viewSystemForm
		return m, m.systemForm.Init()

	case slash.SourcePreviewMsg:
		if msg.Name == "" || msg.ObjectType == "" {
			m.chat.messages = append(m.chat.messages, chatMessage{
				kind:    kindSystem,
				content: "Usage: `/source <name> <type>` — e.g. `/source ZHELLO_PROGRAM PROG/P`",
			})
			m.chat.rebuildViewport()
			return m, nil
		}
		m.chat.messages = append(m.chat.messages, chatMessage{
			kind:    kindSystem,
			content: fmt.Sprintf("Fetching **%s** (%s)…", msg.Name, msg.ObjectType),
		})
		m.chat.rebuildViewport()
		return m, doGetSource(msg.Name, msg.ObjectType)

	case sourceResultMsg:
		m.chat.messages = append(m.chat.messages, chatMessage{
			kind:    kindSystem,
			content: msg.content,
		})
		m.chat.rebuildViewport()
		return m, nil

	case slash.ObjectSearchMsg:
		if msg.Pattern == "" {
			m.chat.messages = append(m.chat.messages, chatMessage{
				kind:    kindSystem,
				content: "Usage: `/object <pattern> [type]` — e.g. `/object ZMY*` or `/object ZCL* CLAS/OC`",
			})
			m.chat.rebuildViewport()
			return m, nil
		}
		m.chat.messages = append(m.chat.messages, chatMessage{
			kind:    kindSystem,
			content: fmt.Sprintf("Searching for **%s**…", msg.Pattern),
		})
		m.chat.rebuildViewport()
		return m, doSearch(msg.Pattern, msg.ObjectType)

	case searchResultMsg:
		m.chat.messages = append(m.chat.messages, chatMessage{
			kind:    kindSystem,
			content: msg.content,
		})
		m.chat.rebuildViewport()
		return m, nil

	case slash.StatusMsg:
		m.chat.messages = append(m.chat.messages, chatMessage{
			kind:    kindSystem,
			content: "Checking status…",
		})
		m.chat.rebuildViewport()
		return m, doStatusCheck()

	case statusResultMsg:
		m.chat.messages = append(m.chat.messages, chatMessage{
			kind:    kindSystem,
			content: msg.content,
		})
		m.chat.rebuildViewport()
		return m, nil

	case slash.UnknownCmdMsg:
		return m, nil
	}

	newChat, cmd := m.chat.Update(msg)
	m.chat = newChat
	return m, cmd
}

func (m *Model) showSystemList() tea.Cmd {
	sysCfg, err := config.LoadSystems()
	if err != nil || len(sysCfg.Systems) == 0 {
		m.chat.messages = append(m.chat.messages, chatMessage{
			kind:    kindSystem,
			content: "No SAP systems configured. Use `/system add` or `abaper system add --help`.",
		})
		m.chat.rebuildViewport()
		return nil
	}

	var sb strings.Builder
	sb.WriteString("**SAP Systems**\n\n")
	for _, s := range sysCfg.Systems {
		marker := "  "
		if s.ID == sysCfg.Active {
			marker = "● "
		}
		fmt.Fprintf(&sb, "%s**%s** — %s (client %s, user %s)\n", marker, s.Name, s.Host, s.Client, s.Username)
	}
	sb.WriteString("\nUse `/system add` to add or `/system edit <name>` to edit.")

	m.chat.messages = append(m.chat.messages, chatMessage{
		kind:    kindSystem,
		content: sb.String(),
	})
	m.chat.rebuildViewport()
	return nil
}

func (m *Model) handleSystemSaved(sys config.SAPSystem) tea.Cmd {
	m.view = viewChat
	m.systemForm = nil

	sysCfg, err := config.LoadSystems()
	if err != nil {
		sysCfg = &config.SystemsConfig{}
	}

	var action string
	if sys.ID != "" {
		sysCfg.UpdateSystem(sys.ID, sys)
		action = fmt.Sprintf("Updated SAP system **%s** (%s)", sys.Name, sys.Host)
	} else {
		id := sysCfg.AddSystem(sys)
		sysCfg.Active = id
		action = fmt.Sprintf("Added SAP system **%s** (%s) — set as active", sys.Name, sys.Host)
	}

	_ = config.SaveSystems(sysCfg)

	m.chat.messages = append(m.chat.messages, chatMessage{
		kind:    kindSystem,
		content: "✓ " + action,
	})
	m.chat.rebuildViewport()
	return nil
}

func (m *Model) View() string {
	if m.width == 0 {
		return ""
	}
	if m.view == viewSystemForm && m.systemForm != nil {
		return m.systemForm.View()
	}
	return m.chat.View()
}

const helpText = `Commands: /help /clear /status /source /object /system /system add /system list /quit
Keys: Enter submit · Shift+Enter newline · Ctrl+C/Esc cancel stream · Tab navigate form · Ctrl+T test connection · Ctrl+S save`

type searchResultMsg struct{ content string }
type sourceResultMsg struct{ content string }
type statusResultMsg struct{ content string }

// doStatusCheck runs the full (every configured system) health check for the
// /status slash command — this is deliberately more thorough than the fast
// active-system-only check that drives the status bar.
func doStatusCheck() tea.Cmd {
	return func() tea.Msg {
		return statusResultMsg{content: formatStatusReport(health.Check(context.Background(), true))}
	}
}

func formatStatusReport(r health.Report) string {
	var sb strings.Builder
	sb.WriteString("**Status**\n\n")

	switch {
	case !r.TokenPresent:
		sb.WriteString("Auth: not logged in — run `abaper login`\n")
	case !r.TokenValid:
		sb.WriteString("Auth: session expired — run `abaper login`\n")
	default:
		fmt.Fprintf(&sb, "Auth: authenticated (expires in %s)\n", r.TokenExpiresIn.Round(time.Minute))
	}

	if r.GatewayReachable {
		fmt.Fprintf(&sb, "Gateway: reachable (%s)\n", r.GatewayLatency.Round(time.Millisecond))
	} else {
		sb.WriteString("Gateway: unreachable\n")
	}

	sb.WriteString("\n**SAP Systems**\n\n")
	if len(r.Systems) == 0 {
		sb.WriteString("No SAP systems configured. Use `/system add`.\n")
		return sb.String()
	}
	for _, s := range r.Systems {
		marker := "  "
		if s.Active {
			marker = "● "
		}
		state := "✗ unreachable"
		if s.Reachable {
			state = fmt.Sprintf("✓ reachable (%s)", s.Latency.Round(time.Millisecond))
		}
		fmt.Fprintf(&sb, "%s**%s** — %s — %s\n", marker, s.Name, s.Host, state)
	}
	return sb.String()
}

func doGetSource(name, objectType string) tea.Cmd {
	return func() tea.Msg {
		c, err := client.NewClient()
		if err != nil {
			return sourceResultMsg{content: "Error: " + err.Error()}
		}
		obj, err := c.GetObject(objectType, name)
		if err != nil {
			return sourceResultMsg{content: "Error: " + err.Error()}
		}
		source, _ := (*obj)["source"].(string)
		if source == "" {
			return sourceResultMsg{content: fmt.Sprintf("No source found for **%s** (%s).", name, objectType)}
		}
		lines := strings.Count(source, "\n") + 1
		var sb strings.Builder
		fmt.Fprintf(&sb, "**%s** (%s) — %d lines  ↑↓ PgUp PgDn to scroll\n\n```abap\n%s\n```", name, objectType, lines, source)
		return sourceResultMsg{content: sb.String()}
	}
}

func doSearch(pattern, objectType string) tea.Cmd {
	return func() tea.Msg {
		c, err := client.NewClient()
		if err != nil {
			return searchResultMsg{content: "Search error: " + err.Error()}
		}
		objects, err := c.SearchObjects(pattern, objectType)
		if err != nil {
			return searchResultMsg{content: "Search error: " + err.Error()}
		}
		if len(objects) == 0 {
			return searchResultMsg{content: fmt.Sprintf("No objects found matching **%s**.", pattern)}
		}

		var sb strings.Builder
		fmt.Fprintf(&sb, "**Search results for %s** (%d)\n\n", pattern, len(objects))
		sb.WriteString("| TYPE | NAME | DESCRIPTION |\n")
		sb.WriteString("|------|------|-------------|\n")
		for _, obj := range objects {
			objType, _ := obj["type"].(string)
			objName, _ := obj["name"].(string)
			desc, _ := obj["description"].(string)
			fmt.Fprintf(&sb, "| %s | %s | %s |\n", objType, objName, desc)
		}
		return searchResultMsg{content: sb.String()}
	}
}
