package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"

	"github.com/bluefunda/abaper/internal/config"
	"github.com/bluefunda/abaper/internal/health"
	"github.com/bluefunda/abaper/styles"
	"github.com/bluefunda/abaper/tui/slash"
	"github.com/bluefunda/abaper/internal/agentrun"
	"github.com/bluefunda/bluefunda-ai/sdk/agent"
)

// healthPollInterval controls how often the TUI re-checks login/system
// status in the background so the header self-heals without user action.
const healthPollInterval = 60 * time.Second

// tokenExpiringSoon is the threshold below which the auth dot turns amber
// instead of red/green.
const tokenExpiringSoon = 10 * time.Minute

// ── message types ──────────────────────────────────────────────────────────

type msgKind int

const (
	kindLogo      msgKind = iota // startup logo block
	kindUser                     // human turn
	kindAssistant                // content built incrementally during streaming
	kindTool                     // tool execution status line
	kindSystem                   // dim informational line
	kindError
)

type chatMessage struct {
	kind       msgKind
	content    string
	toolName   string // for kindTool
	toolStatus string // "running" | "completed" | "failed"
	durationMs int
}

// ── tea messages ───────────────────────────────────────────────────────────

type streamChunkMsg struct{ content string }
type streamToolMsg struct{ name string }
type streamDoneMsg struct{ err error }
type healthCheckedMsg struct{ report health.Report }
type healthTickMsg struct{}
type updateCheckedMsg struct{ latest string }

// ── layout constants ───────────────────────────────────────────────────────
//
// Layout mirrors bai's: a top header (content + bottom border), the
// conversation viewport, an optional slash menu, the input in a rounded
// border box, and a bottom footer hint line.

const (
	headerHeight   = 2 // content line + bottom border
	footerHeight   = 1
	inputBorderPad = 2 // rounded border top + bottom
	inputHeight    = 3
	normalOverhead = headerHeight + footerHeight + inputBorderPad + inputHeight
)

// ── chat model ─────────────────────────────────────────────────────────────

type chatModel struct {
	version  string
	messages []chatMessage

	viewport viewport.Model
	textarea textarea.Model
	spinner  spinner.Model
	renderer *glamour.TermRenderer

	streaming    bool
	cancelStream context.CancelFunc
	streamCh     chan agent.Event
	currentCh    chan agent.Event // written by runner's OnEvent; swapped per message
	runner       *agent.Runner
	tracker      *agentrun.Tracker // SAP objects touched by the current message; swapped per message

	model string

	health          health.Report
	healthChecked   bool
	updateAvailable string // latest version string, empty if up to date/unknown

	slashOpen bool
	slashMenu *slash.MenuModel

	width  int
	height int
}

func newChatModel(version string) *chatModel {
	ta := textarea.New()
	ta.Placeholder = "Type your ABAP request or / for commands"
	ta.ShowLineNumbers = false
	ta.SetHeight(inputHeight)
	ta.CharLimit = 0
	// Shift+Enter for newline; plain Enter is intercepted for submit
	ta.KeyMap.InsertNewline.SetKeys("shift+enter", "ctrl+j")
	ta.Focus()

	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(styles.ColorAccent)

	m := &chatModel{
		version:  version,
		spinner:  sp,
		textarea: ta,
		model:    "fast",
	}

	// Runner is created once per TUI session. OnEvent writes to m.currentCh,
	// which sendMessage replaces before each Run() call — no race because
	// BubbleTea's update loop is single-threaded and streaming must finish
	// before the next message can be submitted.
	m.runner = agent.New(agent.Options{
		Model:    m.model,
		MaxTurns: 20,
		OnEvent: func(ev agent.Event) {
			if tr := m.tracker; tr != nil {
				tr.Observe(ev)
			}
			if ch := m.currentCh; ch != nil {
				select {
				case ch <- ev:
				default:
				}
			}
		},
	})
	m.runner.WithSystemPrompt(agentrun.SystemPrompt)

	m.messages = []chatMessage{{kind: kindLogo}}
	if _, err := config.LoadTokens(); err != nil {
		m.messages = append(m.messages, chatMessage{
			kind:    kindSystem,
			content: "Not logged in — run `abaper login` to authenticate.",
		})
	}

	return m
}

func (m *chatModel) Init() tea.Cmd {
	return tea.Batch(textarea.Blink, doHealthCheck(), healthTick(), doUpdateCheck(m.version))
}

// doHealthCheck runs the fast (active-system-only) health check used by the
// status bar; the full multi-system breakdown is only computed on demand via
// the /status slash command.
func doHealthCheck() tea.Cmd {
	return func() tea.Msg {
		return healthCheckedMsg{report: health.Check(context.Background(), false)}
	}
}

func healthTick() tea.Cmd {
	return tea.Tick(healthPollInterval, func(time.Time) tea.Msg { return healthTickMsg{} })
}

// doUpdateCheck runs once at startup (unlike the health check, it isn't
// polled) and surfaces a footer badge when a newer abaper release exists.
func doUpdateCheck(currentVersion string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()

		latest, err := health.LatestRelease(ctx)
		current := strings.TrimPrefix(currentVersion, "v")
		if err != nil || current == "dev" || latest == current {
			return updateCheckedMsg{}
		}
		return updateCheckedMsg{latest: latest}
	}
}

// SetSize recalculates component sizes when the terminal is resized.
func (m *chatModel) SetSize(w, h int) {
	m.width = w
	m.height = h
	m.textarea.SetWidth(w - 2)
	m.rebuildViewport()

	var err error
	m.renderer, err = glamour.NewTermRenderer(
		glamour.WithStandardStyle("dark"),
		glamour.WithWordWrap(w-6),
	)
	if err != nil {
		m.renderer = nil
	}
}

func (m *chatModel) viewportHeight() int {
	extra := 0
	if m.slashOpen && m.slashMenu != nil {
		extra = m.slashMenu.Height()
	}
	h := m.height - normalOverhead - extra
	if h < 3 {
		h = 3
	}
	return h
}

func (m *chatModel) rebuildViewport() {
	atBottom := m.viewport.AtBottom()
	m.viewport.Width = m.width
	m.viewport.Height = m.viewportHeight()
	m.viewport.SetContent(m.renderMessages())
	if atBottom {
		m.viewport.GotoBottom()
	}
}

// ── Update ─────────────────────────────────────────────────────────────────

func (m *chatModel) Update(msg tea.Msg) (*chatModel, tea.Cmd) {
	var cmds []tea.Cmd

	// ── slash menu is open: route events into it ──────────────────────────
	if m.slashOpen {
		switch ev := msg.(type) {
		case slash.SelectedMsg:
			m.slashOpen = false
			m.rebuildViewport()
			m.textarea.Focus()
			return m, tea.Batch(textarea.Blink, m.runSlashCmd(ev.Cmd, ev.Args))

		case slash.CancelMsg:
			m.slashOpen = false
			m.rebuildViewport()
			m.textarea.Focus()
			return m, textarea.Blink

		default:
			newMenu, cmd := m.slashMenu.Update(msg)
			m.slashMenu = newMenu
			cmds = append(cmds, cmd)
			// Rebuild in case filter changed command list height
			m.rebuildViewport()
			return m, tea.Batch(cmds...)
		}
	}

	// ── key events ────────────────────────────────────────────────────────
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch {
		// Cancel active stream
		case (keyMsg.Type == tea.KeyCtrlC || keyMsg.Type == tea.KeyEsc) && m.streaming:
			if m.cancelStream != nil {
				m.cancelStream()
			}
			return m, nil

		// Show help on `?`
		case keyMsg.Type == tea.KeyRunes && len(keyMsg.Runes) == 1 &&
			keyMsg.Runes[0] == '?' && !m.streaming && !m.slashOpen:
			return m, func() tea.Msg { return slash.HelpMsg{} }

		// Open slash menu when `/` is typed into an empty input
		case keyMsg.Type == tea.KeyRunes &&
			len(keyMsg.Runes) == 1 && keyMsg.Runes[0] == '/' &&
			strings.TrimSpace(m.textarea.Value()) == "" &&
			!m.streaming:
			m.slashOpen = true
			menuW := m.width - 4
			if menuW < 44 {
				menuW = 44
			}
			m.slashMenu = slash.NewMenu(menuW)
			m.rebuildViewport()
			return m, nil // do NOT pass '/' to textarea

		// Submit on Enter (shift+enter handled by textarea for newlines)
		case keyMsg.Type == tea.KeyEnter && !m.streaming:
			input := strings.TrimSpace(m.textarea.Value())
			if input == "" {
				return m, nil
			}
			// Typed slash command (e.g. `/quit`)
			if strings.HasPrefix(input, "/") {
				parts := strings.Fields(strings.TrimPrefix(input, "/"))
				if len(parts) > 0 {
					if cmd := slash.Lookup(parts[0]); cmd != nil {
						m.textarea.Reset()
						return m, m.runSlashCmd(*cmd, parts[1:])
					}
				}
			}
			m.textarea.Reset()
			return m, m.sendMessage(input)
		}
	}

	// ── streaming events ──────────────────────────────────────────────────
	switch ev := msg.(type) {
	case streamChunkMsg:
		m.appendToLastAssistant(ev.content)
		m.rebuildViewport()
		return m, m.waitForStream()

	case streamToolMsg:
		m.upsertTool(ev.name, "running", 0)
		m.rebuildViewport()
		return m, m.waitForStream()

	case streamDoneMsg:
		m.streaming = false
		m.cancelStream = nil
		if ev.err != nil && ev.err != context.Canceled {
			m.messages = append(m.messages, chatMessage{kind: kindError, content: ev.err.Error()})
		}
		if m.tracker != nil {
			if objs := m.tracker.Objects(); len(objs) > 0 {
				m.messages = append(m.messages, chatMessage{kind: kindSystem, content: agentrun.Summary(objs)})
			}
		}
		m.rebuildViewport()
		return m, textarea.Blink

	case spinner.TickMsg:
		if m.streaming {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			cmds = append(cmds, cmd)
			// Re-render viewport so spinner frame updates in place
			m.viewport.SetContent(m.renderMessages())
		}

	case healthCheckedMsg:
		m.health = ev.report
		m.healthChecked = true
		return m, nil

	case healthTickMsg:
		return m, tea.Batch(doHealthCheck(), healthTick())

	case updateCheckedMsg:
		m.updateAvailable = ev.latest
		return m, nil
	}

	// ── pass through to textarea and viewport ─────────────────────────────
	var taCmd, vpCmd tea.Cmd
	m.textarea, taCmd = m.textarea.Update(msg)
	m.viewport, vpCmd = m.viewport.Update(msg)
	cmds = append(cmds, taCmd, vpCmd)

	return m, tea.Batch(cmds...)
}

// ── slash command execution ────────────────────────────────────────────────

func (m *chatModel) runSlashCmd(cmd slash.Command, args []string) tea.Cmd {
	if cmd.Handler == nil {
		m.messages = append(m.messages, chatMessage{
			kind:    kindSystem,
			content: fmt.Sprintf("/%s — not yet implemented", cmd.Name),
		})
		m.rebuildViewport()
		return nil
	}
	return func() tea.Msg {
		innerCmd := cmd.Handler(args)
		if innerCmd == nil {
			return slash.UnknownCmdMsg{Name: cmd.Name}
		}
		return innerCmd()
	}
}

// ── streaming ─────────────────────────────────────────────────────────────

func (m *chatModel) sendMessage(input string) tea.Cmd {
	m.messages = append(m.messages, chatMessage{kind: kindUser, content: input})
	m.messages = append(m.messages, chatMessage{kind: kindAssistant, content: ""})
	m.streaming = true
	m.rebuildViewport()

	ch := make(chan agent.Event, 64)
	m.tracker = agentrun.NewTracker()
	m.currentCh = ch // runner's OnEvent closure writes here
	m.streamCh = ch

	ctx, cancel := context.WithCancel(context.Background())
	m.cancelStream = cancel

	go func() {
		defer func() {
			m.currentCh = nil
			close(ch)
		}()

		if err := m.runner.Run(ctx, input); err != nil && ctx.Err() == nil {
			ch <- agent.Event{Type: "error", Err: err}
		}
	}()

	return tea.Batch(m.spinner.Tick, m.waitForStream())
}

// waitForStream is a recursive Cmd that drains one meaningful event from streamCh.
func (m *chatModel) waitForStream() tea.Cmd {
	return func() tea.Msg {
		for {
			ev, ok := <-m.streamCh
			if !ok {
				return streamDoneMsg{}
			}
			switch ev.Type {
			case "text":
				if ev.Text != "" {
					return streamChunkMsg{content: ev.Text}
				}
			case "tool_use":
				return streamToolMsg{name: ev.ToolName}
			case "result":
				return streamDoneMsg{}
			case "error":
				var msg string
				if ev.Err != nil {
					msg = ev.Err.Error()
				}
				return streamDoneMsg{err: fmt.Errorf("%s", msg)}
			}
			// unknown event types: keep looping
		}
	}
}

// ── message helpers ────────────────────────────────────────────────────────

func (m *chatModel) appendToLastAssistant(content string) {
	for i := len(m.messages) - 1; i >= 0; i-- {
		if m.messages[i].kind == kindAssistant {
			m.messages[i].content += content
			return
		}
	}
}

func (m *chatModel) upsertTool(name, status string, durationMs int) {
	// Update existing "running" entry for this tool
	for i := len(m.messages) - 1; i >= 0; i-- {
		if m.messages[i].kind == kindTool && m.messages[i].toolName == name && m.messages[i].toolStatus == "running" {
			m.messages[i].toolStatus = status
			m.messages[i].durationMs = durationMs
			return
		}
	}
	m.messages = append(m.messages, chatMessage{
		kind:       kindTool,
		toolName:   name,
		toolStatus: "running",
	})
}

// ── rendering ─────────────────────────────────────────────────────────────

func (m *chatModel) renderMessages() string {
	var parts []string
	for _, msg := range m.messages {
		parts = append(parts, m.renderMessage(msg))
	}
	if m.streaming {
		parts = append(parts, m.spinner.View()+" "+
			lipgloss.NewStyle().Foreground(styles.ColorMuted).Render("generating..."))
	}
	return strings.Join(parts, "\n\n")
}

func (m *chatModel) renderMessage(msg chatMessage) string {
	switch msg.kind {
	case kindLogo:
		return renderLogo(m.version, m.model)

	case kindUser:
		return styles.StyleUserMsg.Render("You") + "  " + msg.content

	case kindAssistant:
		label := styles.StyleAssistantLabel.Render("◆ ABAPer")
		if msg.content == "" {
			return label
		}
		rendered := msg.content
		if m.renderer != nil {
			if r, err := m.renderer.Render(msg.content); err == nil {
				rendered = strings.TrimRight(r, "\n")
			}
		}
		return label + "\n" + rendered

	case kindTool:
		icon := styles.StyleTool.Render("●")
		statusStr := lipgloss.NewStyle().Foreground(styles.ColorMuted).Render("● running")
		switch msg.toolStatus {
		case "completed":
			icon = lipgloss.NewStyle().Foreground(styles.ColorSuccess).Render("✓")
			statusStr = lipgloss.NewStyle().Foreground(styles.ColorSuccess).
				Render(fmt.Sprintf("✓ %.1fs", float64(msg.durationMs)/1000))
		case "failed":
			icon = lipgloss.NewStyle().Foreground(styles.ColorError).Render("✗")
			statusStr = lipgloss.NewStyle().Foreground(styles.ColorError).Render("✗ failed")
		}
		name := lipgloss.NewStyle().Foreground(styles.ColorMuted).Render(msg.toolName)
		return "  " + icon + "  " + name + "  " + statusStr

	case kindSystem:
		return styles.StyleMuted.Render("  " + msg.content)

	case kindError:
		return styles.StyleError.Render("  ✗ " + msg.content)
	}
	return ""
}

// renderHeader is the top bar: "◆ ABAPer  vX.Y.Z  ·  <model>" on the left,
// live auth/system status (see renderAuthStatus) on the right — mirrors
// bai's header layout and bottom-border-only style.
func (m *chatModel) renderHeader() string {
	left := styles.StyleAssistantLabel.Render("◆ ABAPer")
	if m.version != "" {
		left += styles.StyleMuted.Render("  " + m.version)
	}
	left += styles.StyleMuted.Render("  ·  " + m.model)

	right := m.renderAuthStatus()

	gap := m.width - lipgloss.Width(left) - lipgloss.Width(right) - 2
	if gap < 1 {
		gap = 1
	}
	line := " " + left + strings.Repeat(" ", gap) + right
	return styles.StyleHeader.Width(m.width).Render(line)
}

// renderFooter is the bottom hint line: key hints (swapped while streaming)
// plus an update-available badge when a newer release exists.
func (m *chatModel) renderFooter() string {
	hint := "[↑] history  ·  /help commands  ·  ? help"
	if m.streaming {
		hint = "Ctrl+C / Esc cancel  ·  Ctrl+D quit"
	}
	left := "  " + styles.StyleMuted.Render(hint)
	if m.updateAvailable == "" {
		return left
	}
	badge := styles.StyleWarning.Render("↑ " + m.updateAvailable + " available  ·  run: abaper update")
	gap := m.width - lipgloss.Width(left) - lipgloss.Width(badge) - 2
	if gap < 1 {
		gap = 1
	}
	return left + strings.Repeat(" ", gap) + badge
}

// renderAuthStatus renders the auth dot plus the active SAP system's live
// reachability, e.g. "● authenticated  ● A4H". Real state comes from the
// background health check (see doHealthCheck/healthTick); before the first
// check completes it shows a neutral "checking..." placeholder.
func (m *chatModel) renderAuthStatus() string {
	if !m.healthChecked {
		return styles.StyleMuted.Render("◌ checking...")
	}

	var dotColor lipgloss.TerminalColor
	var label string
	switch {
	case !m.health.TokenPresent:
		dotColor, label = styles.ColorError, "not logged in"
	case !m.health.TokenValid:
		dotColor, label = styles.ColorError, "session expired"
	case m.health.TokenExpiresIn < tokenExpiringSoon:
		dotColor, label = styles.ColorWarning, "session expiring"
	default:
		dotColor, label = styles.ColorSuccess, "authenticated"
	}
	auth := lipgloss.NewStyle().Foreground(dotColor).Render("●") + styles.StyleMuted.Render(" "+label)

	if len(m.health.Systems) == 0 {
		return auth + styles.StyleMuted.Render("  no SAP system")
	}
	sys := m.health.Systems[0] // fast check only queries the active system
	sysColor := styles.ColorError
	if sys.Reachable {
		sysColor = styles.ColorSuccess
	}
	sysPart := lipgloss.NewStyle().Foreground(sysColor).Render("●") + styles.StyleMuted.Render(" "+sys.Name)

	return auth + "  " + sysPart
}

func (m *chatModel) renderInput() string {
	if m.streaming {
		return styles.StyleMuted.Render(m.spinner.View() + " thinking...  (Ctrl+C to cancel)")
	}
	prompt := lipgloss.NewStyle().Foreground(styles.ColorAccent).Render("❯ ")
	return prompt + m.textarea.View()
}

// renderInputBox wraps the input in a rounded border, like bai's input area.
func (m *chatModel) renderInputBox() string {
	return styles.StyleInputBorder.Width(m.width - 2).Render(m.renderInput())
}

// ── View ───────────────────────────────────────────────────────────────────

func (m *chatModel) View() string {
	if m.slashOpen && m.slashMenu != nil {
		return lipgloss.JoinVertical(lipgloss.Left,
			m.renderHeader(),
			m.viewport.View(),
			m.slashMenu.View(),
			m.renderInputBox(),
			m.renderFooter(),
		)
	}

	return lipgloss.JoinVertical(lipgloss.Left,
		m.renderHeader(),
		m.viewport.View(),
		m.renderInputBox(),
		m.renderFooter(),
	)
}
