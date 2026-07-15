package tui

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/google/uuid"
)

// ──────────────────────────────────────────────
//  Tea messages (events sent to the BubbleTea loop)
// ──────────────────────────────────────────────

type StreamChunkMsg struct{ Chunk string }
type StreamToolCallMsg struct {
	ID   string
	Name string
	Args string
}
type StreamToolExecMsg struct {
	Name       string
	Status     string
	DurationMs int64
	Summary    string
}
type StreamProgressMsg struct {
	Iteration int
	Tools     []string
}
type StreamHeartbeatMsg struct{}
type StreamDoneMsg struct{ Err error }

// InterruptedMsg is sent when the user presses Ctrl+C during a streaming turn.
type InterruptedMsg struct{}
type StreamErrorMsg struct{ Msg string }
type ApprovalRequestMsg struct {
	ToolName string
	Args     string
	ReplyCh  chan bool
}
type ApprovalResponseMsg struct{ Approved bool }

// SessionsLoadedMsg is the async result of listing sessions for /sessions.
type SessionsLoadedMsg struct {
	Sessions []SessionInfo
	Err      error
}

// AccountLoadedMsg is the async result of /account.
type AccountLoadedMsg struct {
	Info *AccountInfo
	Err  error
}

// UsageLoadedMsg is the async result of /usage.
type UsageLoadedMsg struct {
	Info *UsageInfo
	Err  error
}

// UsageAutoCheckMsg is the async result of the post-response background usage check.
type UsageAutoCheckMsg struct {
	Info *UsageInfo
	Err  error
}

// MCPListLoadedMsg is the async result of listing MCP servers for /mcp.
type MCPListLoadedMsg struct {
	Items []MCPInfo
	Err   error
}

// MCPActivatedMsg is the async result of activating an MCP server via /mcp <name>.
type MCPActivatedMsg struct {
	Name string
	Err  error
}

// ModelsLoadedMsg is the async result of listing models for /model.
type ModelsLoadedMsg struct {
	Items []ModelInfo
	Err   error
}

// tickMsg drives the spinner animation.
type tickMsg time.Time

// updateExecDoneMsg is sent when the bai update subprocess exits.
type updateExecDoneMsg struct{ err error }

// ──────────────────────────────────────────────
//  SessionConfig is passed from cmd to the TUI
// ──────────────────────────────────────────────

type SessionConfig struct {
	ChatID         string
	Model          string
	IsCode         bool   // code session (has local tools)
	WorkDir        string // working directory shown in header
	Version        string // bai version shown in header
	AutoApply      bool
	InitialPrompt  string                        // auto-submitted as the first message
	RepoName       string                        // git repo name shown in header
	ResumeTitle    string                        // set when auto-resuming a past session
	IsResume       bool                          // true = resuming; isNewChat starts false
	ListSessionsFn func() ([]SessionInfo, error) // nil = listing not available
	AccountFn      func() (*AccountInfo, error)  // nil = account info not available
	UsageFn        func() (*UsageInfo, error)    // nil = usage not available
	MCPListFn      func() ([]MCPInfo, error)     // nil = MCP listing not available
	MCPActivateFn  func(name string) error       // nil = MCP activation not available
	ListModelsFn   func() ([]ModelInfo, error)   // nil = model listing not available
	SetAutoApplyFn func(enabled bool)            // nil = auto-apply not available (non-code sessions)
	SetCodeModeFn  func(enabled bool)            // nil = mode switch not supported in this session
	CustomCommands []SlashCommand                // loaded from .bai/commands/*.md
}

// SessionInfo is one entry returned by ListSessionsFn for /sessions display.
type SessionInfo struct {
	ID    string
	Title string
	Model string
}

// AccountInfo holds the data shown by /account.
type AccountInfo struct {
	Name     string
	Email    string
	Username string
}

// ModelInfo is one model entry shown by /model.
type ModelInfo struct {
	Name    string
	OwnedBy string
}

// MCPInfo is one MCP server entry shown by /mcp.
type MCPInfo struct {
	Name        string
	Type        string
	Available   bool
	Description string
}

// UsageInfo holds the data shown by /usage.
type UsageInfo struct {
	PlanType       string
	RPMUsed        int32
	RPMLimit       int32
	RPMPercent     float64
	DailyPercent   float64
	MonthlyPercent float64
	InputTokens    int64
	OutputTokens   int64
	TotalTokens    int64
	RetryAfter     int // seconds to wait when rate limited (future: populated by cai-bff)
}

// StreamEvent is a discriminated union of all events that can arrive from a
// gRPC stream. The BubbleTea cmd reads one event from the channel per tick.
type StreamEvent struct {
	Kind string // "chunk"|"tool_call"|"tool_exec"|"progress"|"heartbeat"|"approval"|"done"|"error"|"rate_limited"|"usage_warning"|"live_usage_pct"
	// chunk
	Chunk string
	// tool_call / tool_exec / approval
	ToolID   string
	ToolName string
	ToolArgs string
	// tool_exec
	Status     string
	DurationMs int64
	Summary    string
	// progress
	Iteration int
	Tools     []string
	// approval: reply channel (true=approved)
	ReplyCh chan bool
	// done / error
	Err    error
	ErrMsg string
	// token usage — set on "done" events when the backend provides usage data
	UsagePromptTokens     int32
	UsageCompletionTokens int32
	// live_usage_pct — live usage percentage during streaming (0-100)
	LivePct float64
}

// SubmitFn opens a gRPC stream for the given input and pumps events into the
// returned channel. The channel is closed when the stream ends.
// model may be updated at runtime by /model — submitInput always passes m.cfg.Model.
type SubmitFn func(chatID, model, input string, isNew bool) <-chan StreamEvent

// ──────────────────────────────────────────────
//  Model
// ──────────────────────────────────────────────

const (
	headerHeight   = 2
	footerHeight   = 1
	inputMinLines  = 1
	inputMaxLines  = 5  // max textarea rows before internal scroll
	countThreshold = 80 // show char/line count when input exceeds this many runes
)

type Model struct {
	// Layout
	width, height int

	// Session
	cfg       SessionConfig
	isNewChat bool
	submitFn  SubmitFn
	theme     Theme

	// Messages
	messages []ChatMessage

	// Viewport (conversation scroll area)
	viewport viewport.Model
	vpReady  bool
	atBottom bool

	// Input area
	textarea     textarea.Model
	inputHistory []string
	historyIdx   int
	historyDraft string

	// Streaming
	streaming    bool
	streamCh     <-chan StreamEvent
	streamStop   chan struct{} // closed to interrupt the current turn without quitting
	spinnerIdx   int
	liveUsagePct float64 // live usage % updated every ~25 tokens during streaming (0 = inactive)

	// Approval prompt (tool confirmation)
	pendingApproval *ApprovalRequestMsg

	// Slash command menu
	showSlash    bool
	slashMatches []SlashCommand
	slashIdx     int

	// Model picker (shown when /model is typed with no argument)
	showModelPicker  bool
	modelPickerItems []ModelInfo
	modelPickerIdx   int

	// Token usage (cumulative across turns, updated on each "done" event)
	totalPromptTokens     int32
	totalCompletionTokens int32

	// Update notification
	updateAvailable string // non-empty when a newer version is found on startup

	// Misc
	quit              bool
	initialPromptSent bool
	recentSessions    []SessionInfo
}

// New creates a new TUI model.
func New(cfg SessionConfig, submit SubmitFn) Model {
	th := DefaultTheme()

	ta := textarea.New()
	ta.Placeholder = "Ask BlueFunda AI..."
	ta.ShowLineNumbers = false
	ta.CharLimit = 0
	ta.SetWidth(80)
	ta.SetHeight(inputMinLines)
	ta.Focus()
	// Style the textarea
	ta.FocusedStyle.Base = lipgloss.NewStyle().Foreground(th.Foreground)
	ta.FocusedStyle.Placeholder = lipgloss.NewStyle().Foreground(th.Muted)
	ta.FocusedStyle.CursorLine = lipgloss.NewStyle()
	ta.BlurredStyle = ta.FocusedStyle
	ta.KeyMap.InsertNewline.SetKeys("shift+enter", "ctrl+j")

	vp := viewport.New(80, 20)
	vp.SetContent("")

	m := Model{
		cfg:        cfg,
		isNewChat:  !cfg.IsResume,
		submitFn:   submit,
		theme:      th,
		textarea:   ta,
		viewport:   vp,
		atBottom:   true,
		historyIdx: -1,
	}

	// Show the welcome system message
	var welcome string
	if cfg.IsResume {
		welcome = "Resuming session " + cfg.ChatID[:8]
		if cfg.ResumeTitle != "" {
			welcome += "  ·  " + cfg.ResumeTitle
		}
	} else {
		welcome = fmt.Sprintf("Session %s  ·  model: %s", cfg.ChatID[:8], cfg.Model)
	}
	if cfg.RepoName != "" {
		welcome += "  ·  " + cfg.RepoName
	}
	if cfg.IsCode {
		welcome += fmt.Sprintf("  ·  code  ·  dir: %s", cfg.WorkDir)
	}
	m.messages = append(m.messages, newSystemMessage(welcome))
	return m
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(
		textarea.Blink,
		tickCmd(),
		tea.EnableBracketedPaste,
		checkForUpdateCmd(m.cfg.Version),
	)
}

func tickCmd() tea.Cmd {
	return tea.Tick(100*time.Millisecond, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

// ──────────────────────────────────────────────
//  Update
// ──────────────────────────────────────────────

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		newM := m.handleResize()
		if newM.cfg.InitialPrompt != "" && !newM.initialPromptSent {
			newM.initialPromptSent = true
			newM.textarea.SetValue(newM.cfg.InitialPrompt)
			return newM.submitInput()
		}
		return newM, nil

	case tickMsg:
		if m.streaming {
			m.spinnerIdx++
		}
		cmds = append(cmds, tickCmd())

	// ── Stream events (channel-based) ──────────

	case StreamEvent:
		cmds = append(cmds, m.handleStreamEvent(msg)...)

	case InterruptedMsg:
		// Ctrl+C already handled in handleKey; nothing more to do here.
		return m, nil

	case StreamDoneMsg:
		m.streaming = false
		finalizedIdx := -1
		for i := len(m.messages) - 1; i >= 0; i-- {
			if m.messages[i].Role == RoleAssistant && m.messages[i].Streaming {
				m.messages[i].finishStreaming()
				finalizedIdx = i
				break
			}
		}
		if msg.Err != nil {
			m.messages = append(m.messages, newSystemMessage("Error: "+msg.Err.Error()))
		}
		m.refreshViewport()
		if finalizedIdx >= 0 && m.messages[finalizedIdx].Content != "" {
			m.scrollToMessageStart(finalizedIdx)
		} else {
			m.viewport.GotoBottom()
		}
		m.atBottom = m.viewport.AtBottom()
		m.textarea.Focus()
		cmds = append(cmds, textarea.Blink)

	// ── Approval ──────────────────────────────

	case ApprovalRequestMsg:
		m.pendingApproval = &msg
		m.refreshViewport()

	case SessionsLoadedMsg:
		// Remove the "Loading sessions…" placeholder
		if len(m.messages) > 0 && m.messages[len(m.messages)-1].Role == RoleSystem {
			m.messages = m.messages[:len(m.messages)-1]
		}
		if msg.Err != nil {
			m.messages = append(m.messages, newSystemMessage("Error: "+msg.Err.Error()))
		} else {
			m.recentSessions = msg.Sessions
			m.messages = append(m.messages, newSystemMessage(m.formatSessions(msg.Sessions)))
		}
		m.refreshViewport()
		m.viewport.GotoBottom()

	case AccountLoadedMsg:
		if len(m.messages) > 0 && m.messages[len(m.messages)-1].Role == RoleSystem {
			m.messages = m.messages[:len(m.messages)-1]
		}
		if msg.Err != nil {
			m.messages = append(m.messages, newSystemMessage("Error: "+msg.Err.Error()))
		} else {
			m.messages = append(m.messages, newSystemMessage(formatAccount(msg.Info)))
		}
		m.refreshViewport()
		m.viewport.GotoBottom()

	case UsageLoadedMsg:
		if len(m.messages) > 0 && m.messages[len(m.messages)-1].Role == RoleSystem {
			m.messages = m.messages[:len(m.messages)-1]
		}
		if msg.Err != nil {
			m.messages = append(m.messages, newSystemMessage("Error: "+msg.Err.Error()))
		} else {
			m.messages = append(m.messages, newSystemMessage(formatUsage(msg.Info)))
		}
		m.refreshViewport()
		m.viewport.GotoBottom()

	case UsageAutoCheckMsg:
		if msg.Err == nil && msg.Info != nil {
			if line := formatUsageCompact(msg.Info); line != "" {
				m.messages = append(m.messages, newSystemMessage(line))
				wasAtBottom := m.atBottom
				m.refreshViewport()
				if wasAtBottom {
					m.viewport.GotoBottom()
				}
			}
		}

	case MCPListLoadedMsg:
		if len(m.messages) > 0 && m.messages[len(m.messages)-1].Role == RoleSystem {
			m.messages = m.messages[:len(m.messages)-1]
		}
		if msg.Err != nil {
			m.messages = append(m.messages, newSystemMessage("Error: "+msg.Err.Error()))
		} else {
			m.messages = append(m.messages, newSystemMessage(formatMCPList(msg.Items)))
		}
		m.refreshViewport()
		m.viewport.GotoBottom()

	case MCPActivatedMsg:
		if len(m.messages) > 0 && m.messages[len(m.messages)-1].Role == RoleSystem {
			m.messages = m.messages[:len(m.messages)-1]
		}
		if msg.Err != nil {
			m.messages = append(m.messages, newSystemMessage("Error activating "+msg.Name+": "+msg.Err.Error()))
		} else {
			m.messages = append(m.messages, newSystemMessage("Activated MCP server: "+msg.Name))
		}
		m.refreshViewport()
		m.viewport.GotoBottom()

	case ModelsLoadedMsg:
		if len(m.messages) > 0 && m.messages[len(m.messages)-1].Role == RoleSystem {
			m.messages = m.messages[:len(m.messages)-1]
		}
		if msg.Err != nil {
			m.messages = append(m.messages, newSystemMessage("Error loading models: "+msg.Err.Error()))
			m.refreshViewport()
			m.viewport.GotoBottom()
		} else {
			m.modelPickerItems = msg.Items
			m.modelPickerIdx = 0
			for i, item := range msg.Items {
				if item.Name == m.cfg.Model {
					m.modelPickerIdx = i
					break
				}
			}
			m.showModelPicker = true
		}

	case UpdateAvailableMsg:
		m.updateAvailable = msg.Version

	case updateExecDoneMsg:
		// After `bai update` exits (success or failure), quit rather than
		// resuming — the binary on disk may have changed.
		return m, tea.Quit

	// ── Keyboard ──────────────────────────────

	case tea.KeyMsg:
		if m.pendingApproval != nil {
			return m.handleApprovalKey(msg)
		}
		if m.showModelPicker {
			return m.handleModelPickerKey(msg)
		}
		return m.handleKey(msg)
	}

	// Route all remaining events to textarea and viewport.
	// Textarea stays active even while streaming so the user can type ahead.
	{
		prevRows := m.effectiveInputRows()
		prevCount := m.showInputCount()

		var taCmd tea.Cmd
		m.textarea, taCmd = m.textarea.Update(msg)
		cmds = append(cmds, taCmd)

		// Relayout when the textarea grows/shrinks or the count indicator
		// appears/disappears, so the viewport adjusts correctly.
		if m.effectiveInputRows() != prevRows || m.showInputCount() != prevCount {
			m = m.handleResize()
		}

		// Update slash menu whenever text changes
		m.updateSlashMenu()
	}

	var vpCmd tea.Cmd
	m.viewport, vpCmd = m.viewport.Update(msg)
	cmds = append(cmds, vpCmd)

	return m, tea.Batch(cmds...)
}

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg.String() {

	case "ctrl+c":
		if m.streaming && m.streamStop != nil {
			// Cancel current turn without quitting; drain remaining events in background.
			close(m.streamStop)
			m.streamStop = nil
			m.streaming = false
			m.messages = append(m.messages, newSystemMessage("Interrupted."))
			go func(ch <-chan StreamEvent) { for range ch {} }(m.streamCh)
			m.textarea.Focus()
			m.refreshViewport()
			return m, nil
		}
		m.quit = true
		return m, tea.Quit

	case "ctrl+d":
		if m.textarea.Value() == "" {
			m.quit = true
			return m, tea.Quit
		}

	case "ctrl+l":
		// Clear conversation display
		m.messages = m.messages[:0]
		m.messages = append(m.messages, newSystemMessage("Screen cleared."))
		m.refreshViewport()
		return m, nil

	case "enter":
		if m.showSlash && len(m.slashMatches) > 0 {
			cmd := m.slashMatches[m.slashIdx]
			if m.textarea.Value() == cmd.Name {
				// Already typed the exact command name — skip the autocomplete
				// step and submit directly.
				m.showSlash = false
				return m.submitInput()
			}
			return m.acceptSlashCommand()
		}
		if m.streaming {
			return m, nil
		}
		return m.submitInput()

	case "up":
		if m.showSlash {
			if m.slashIdx > 0 {
				m.slashIdx--
			}
			return m, nil
		}
		// History navigation
		if !m.textarea.Focused() {
			break
		}
		if m.historyIdx == -1 && len(m.inputHistory) > 0 {
			m.historyDraft = m.textarea.Value()
			m.historyIdx = len(m.inputHistory) - 1
			m.textarea.SetValue(m.inputHistory[m.historyIdx])
			m.textarea.CursorEnd()
			return m, nil
		} else if m.historyIdx > 0 {
			m.historyIdx--
			m.textarea.SetValue(m.inputHistory[m.historyIdx])
			m.textarea.CursorEnd()
			return m, nil
		}

	case "down":
		if m.showSlash {
			if m.slashIdx < len(m.slashMatches)-1 {
				m.slashIdx++
			}
			return m, nil
		}
		// History navigation
		if m.historyIdx != -1 {
			if m.historyIdx < len(m.inputHistory)-1 {
				m.historyIdx++
				m.textarea.SetValue(m.inputHistory[m.historyIdx])
			} else {
				m.historyIdx = -1
				m.textarea.SetValue(m.historyDraft)
			}
			m.textarea.CursorEnd()
			return m, nil
		}

	case "esc":
		if m.showSlash {
			m.showSlash = false
			return m, nil
		}

	case "tab":
		if m.showSlash && len(m.slashMatches) > 0 {
			return m.acceptSlashCommand()
		}

	case "pgup":
		m.viewport.PageUp()
		m.atBottom = false
		return m, nil

	case "pgdown":
		m.viewport.PageDown()
		m.atBottom = m.viewport.AtBottom()
		return m, nil
	}

	var taCmd tea.Cmd
	m.textarea, taCmd = m.textarea.Update(msg)
	cmds = append(cmds, taCmd)
	m.updateSlashMenu()

	return m, tea.Batch(cmds...)
}

func (m Model) handleApprovalKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch strings.ToLower(msg.String()) {
	case "y":
		ch := m.streamCh
		replyCh := m.pendingApproval.ReplyCh
		m.pendingApproval = nil
		m.messages = append(m.messages, newSystemMessage("  Applied."))
		m.refreshViewport()
		// Send approval answer (unblocks code goroutine) then resume stream pump.
		stop := m.streamStop
		return m, func() tea.Msg {
			replyCh <- true
			return waitForStreamEvent(ch, stop)()
		}
	case "n", "esc":
		ch := m.streamCh
		stop := m.streamStop
		replyCh := m.pendingApproval.ReplyCh
		m.pendingApproval = nil
		m.messages = append(m.messages, newSystemMessage("  Skipped."))
		m.refreshViewport()
		return m, func() tea.Msg {
			replyCh <- false
			return waitForStreamEvent(ch, stop)()
		}
	case "ctrl+c":
		m.quit = true
		if m.pendingApproval != nil {
			m.pendingApproval.ReplyCh <- false
			m.pendingApproval = nil
		}
		return m, tea.Quit
	}
	return m, nil
}

func (m Model) handleModelPickerKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "ctrl+p":
		if m.modelPickerIdx > 0 {
			m.modelPickerIdx--
		}
	case "down", "ctrl+n":
		if m.modelPickerIdx < len(m.modelPickerItems)-1 {
			m.modelPickerIdx++
		}
	case "enter":
		if len(m.modelPickerItems) > 0 {
			selected := m.modelPickerItems[m.modelPickerIdx]
			m.cfg.Model = selected.Name
			m.showModelPicker = false
			m.messages = append(m.messages, newSystemMessage("Switched to model: "+selected.Name))
			m.refreshViewport()
			m.viewport.GotoBottom()
		}
	case "esc", "ctrl+c":
		m.showModelPicker = false
	}
	return m, nil
}

func (m Model) submitInput() (tea.Model, tea.Cmd) {
	input := strings.TrimSpace(m.textarea.Value())
	if input == "" {
		return m, nil
	}

	// Handle slash commands
	if strings.HasPrefix(input, "/") {
		return m.handleSlashCommand(input)
	}

	m.textarea.Reset()
	m.showSlash = false
	m.historyIdx = -1

	// Record in history (dedup consecutive identical)
	if len(m.inputHistory) == 0 || m.inputHistory[len(m.inputHistory)-1] != input {
		m.inputHistory = append(m.inputHistory, input)
		if len(m.inputHistory) > 200 {
			m.inputHistory = m.inputHistory[1:]
		}
	}

	// Append user message to viewport — stays visible in conversation history.
	m.messages = append(m.messages, newUserMessage(input))
	m.streaming = true
	m.atBottom = true
	m.refreshViewport()
	m.viewport.GotoBottom()

	isNew := m.isNewChat
	m.isNewChat = false

	// Open the stream and start pumping events via cmd chaining.
	m.streamStop = make(chan struct{})
	m.streamCh = m.submitFn(m.cfg.ChatID, m.cfg.Model, input, isNew)
	return m, waitForStreamEvent(m.streamCh, m.streamStop)
}

func (m Model) handleSlashCommand(input string) (tea.Model, tea.Cmd) {
	m.textarea.Reset()
	m.showSlash = false

	switch {
	case input == "/update":
		exe, err := os.Executable()
		if err != nil {
			m.messages = append(m.messages, newSystemMessage("Could not locate bai binary: "+err.Error()))
			m.refreshViewport()
			break
		}
		cmd := exec.Command(exe, "update")
		return m, tea.ExecProcess(cmd, func(err error) tea.Msg {
			return updateExecDoneMsg{err: err}
		})

	case input == "/exit" || input == "/quit":
		m.quit = true
		return m, tea.Quit

	case input == "/clear":
		m.messages = m.messages[:0]
		m.messages = append(m.messages, newSystemMessage("Conversation cleared."))
		m.refreshViewport()

	case input == "/reset":
		m.messages = m.messages[:0]
		m.isNewChat = true
		m.messages = append(m.messages, newSystemMessage(
			fmt.Sprintf("New session started  ·  model: %s", m.cfg.Model)))
		m.refreshViewport()

	case input == "/help":
		m.messages = append(m.messages, newSystemMessage(helpText()))
		m.refreshViewport()

	case input == "/new":
		m.cfg.ChatID = uuid.New().String()
		m.isNewChat = true
		m.messages = m.messages[:0]
		m.messages = append(m.messages, newSystemMessage(
			fmt.Sprintf("New session  ·  %s  ·  model: %s", m.cfg.ChatID[:8], m.cfg.Model)))
		m.refreshViewport()

	case input == "/model" || strings.HasPrefix(input, "/model "):
		arg := strings.TrimSpace(strings.TrimPrefix(input, "/model"))
		if arg == "" {
			if m.cfg.ListModelsFn != nil {
				m.messages = append(m.messages, newSystemMessage("Loading models…"))
				m.refreshViewport()
				fn := m.cfg.ListModelsFn
				m.viewport.GotoBottom()
				return m, func() tea.Msg {
					items, err := fn()
					return ModelsLoadedMsg{Items: items, Err: err}
				}
			}
			m.messages = append(m.messages, newSystemMessage("Model: "+m.cfg.Model))
		} else {
			m.cfg.Model = strings.TrimSpace(arg)
			m.messages = append(m.messages, newSystemMessage("Switched to model: "+m.cfg.Model))
		}
		m.refreshViewport()

	case input == "/sessions":
		if m.cfg.ListSessionsFn != nil {
			m.messages = append(m.messages, newSystemMessage("Loading sessions…"))
			m.refreshViewport()
			fn := m.cfg.ListSessionsFn
			m.viewport.GotoBottom()
			return m, func() tea.Msg {
				sessions, err := fn()
				return SessionsLoadedMsg{Sessions: sessions, Err: err}
			}
		}
		m.messages = append(m.messages, newSystemMessage("Session listing not available"))
		m.refreshViewport()

	case strings.HasPrefix(input, "/resume "):
		id := strings.TrimSpace(strings.TrimPrefix(input, "/resume "))
		if n, err := strconv.Atoi(id); err == nil && n >= 1 && n <= len(m.recentSessions) {
			s := m.recentSessions[n-1]
			m.cfg.ChatID = s.ID
			m.isNewChat = false
			label := s.ID
			if len(label) > 8 {
				label = label[:8]
			}
			if s.Title != "" {
				label += "  " + s.Title
			}
			m.messages = append(m.messages, newSystemMessage("Resumed: "+label))
		} else {
			m.cfg.ChatID = id
			m.isNewChat = false
			m.messages = append(m.messages, newSystemMessage("Resumed: "+id))
		}
		m.refreshViewport()

	case input == "/mcp" || strings.HasPrefix(input, "/mcp "):
		arg := strings.TrimSpace(strings.TrimPrefix(input, "/mcp"))
		if arg == "" {
			if m.cfg.MCPListFn != nil {
				m.messages = append(m.messages, newSystemMessage("Loading MCP servers…"))
				m.refreshViewport()
				fn := m.cfg.MCPListFn
				m.viewport.GotoBottom()
				return m, func() tea.Msg {
					items, err := fn()
					return MCPListLoadedMsg{Items: items, Err: err}
				}
			}
			m.messages = append(m.messages, newSystemMessage("MCP listing not available"))
			m.refreshViewport()
		} else {
			if m.cfg.MCPActivateFn != nil {
				name := strings.TrimSpace(arg)
				m.messages = append(m.messages, newSystemMessage("Activating "+name+"…"))
				m.refreshViewport()
				fn := m.cfg.MCPActivateFn
				m.viewport.GotoBottom()
				return m, func() tea.Msg {
					err := fn(name)
					return MCPActivatedMsg{Name: name, Err: err}
				}
			}
			m.messages = append(m.messages, newSystemMessage("MCP activation not available"))
			m.refreshViewport()
		}

	case input == "/chat":
		if !m.cfg.IsCode {
			m.messages = append(m.messages, newSystemMessage("Already in chat mode"))
			m.refreshViewport()
			break
		}
		m.cfg.IsCode = false
		if m.cfg.SetCodeModeFn != nil {
			m.cfg.SetCodeModeFn(false)
		}
		m.messages = append(m.messages, newSystemMessage("Switched to chat mode — file tools unloaded"))
		m.refreshViewport()

	case input == "/code":
		if m.cfg.IsCode {
			m.messages = append(m.messages, newSystemMessage("Already in code mode — use /auto to toggle tool approval"))
			m.refreshViewport()
			break
		}
		if m.cfg.SetCodeModeFn != nil {
			m.cfg.IsCode = true
			m.cfg.SetCodeModeFn(true)
			m.messages = append(m.messages, newSystemMessage("Switched to code mode — file tools loaded"))
		} else {
			m.messages = append(m.messages, newSystemMessage(
				"File tools are not available in this session.\nStart with `bai code [prompt]` for the full agentic experience."))
		}
		m.refreshViewport()

	case input == "/auto":
		if !m.cfg.IsCode {
			m.messages = append(m.messages, newSystemMessage("/auto is only available in code sessions (bai code)"))
			m.refreshViewport()
			break
		}
		m.cfg.AutoApply = !m.cfg.AutoApply
		if m.cfg.SetAutoApplyFn != nil {
			m.cfg.SetAutoApplyFn(m.cfg.AutoApply)
		}
		state := "disabled — tools will prompt for approval"
		if m.cfg.AutoApply {
			state = "enabled — tools will execute without prompting"
		}
		m.messages = append(m.messages, newSystemMessage("Auto-apply "+state))
		m.refreshViewport()

	case input == "/account":
		if m.cfg.AccountFn != nil {
			m.messages = append(m.messages, newSystemMessage("Loading account…"))
			m.refreshViewport()
			fn := m.cfg.AccountFn
			m.viewport.GotoBottom()
			return m, func() tea.Msg {
				info, err := fn()
				return AccountLoadedMsg{Info: info, Err: err}
			}
		}
		m.messages = append(m.messages, newSystemMessage("Account info not available"))
		m.refreshViewport()

	case input == "/usage":
		if m.cfg.UsageFn != nil {
			m.messages = append(m.messages, newSystemMessage("Loading usage…"))
			m.refreshViewport()
			fn := m.cfg.UsageFn
			m.viewport.GotoBottom()
			return m, func() tea.Msg {
				info, err := fn()
				return UsageLoadedMsg{Info: info, Err: err}
			}
		}
		m.messages = append(m.messages, newSystemMessage("Usage info not available"))
		m.refreshViewport()

	case input == "/tools" && m.cfg.IsCode:
		m.messages = append(m.messages, newSystemMessage(
			"Available tools: read_file · edit_file · write_file · list_dir · search_files · search_content · bash · web_fetch · web_search\n"+
				"MCP tools: mcp__<server>__<tool>  (configure in .bai/settings.yaml)"))
		m.refreshViewport()

	case input == "/context":
		m.messages = append(m.messages, newSystemMessage(
			fmt.Sprintf("Chat ID: %s  ·  Messages: %d", m.cfg.ChatID, len(m.messages))))
		m.refreshViewport()

	default:
		// Check custom commands loaded from .bai/commands/*.md.
		cmdName := strings.Fields(input)[0] // e.g. "/review"
		for _, c := range m.cfg.CustomCommands {
			if c.Name == cmdName && c.Prompt != "" {
				m.messages = append(m.messages, newUserMessage(c.Prompt))
				m.streaming = true
				m.atBottom = true
				m.refreshViewport()
				m.viewport.GotoBottom()
				isNew := m.isNewChat
				m.isNewChat = false
				m.streamCh = m.submitFn(m.cfg.ChatID, m.cfg.Model, c.Prompt, isNew)
				return m, waitForStreamEvent(m.streamCh, m.streamStop)
			}
		}
		m.messages = append(m.messages, newSystemMessage("Unknown command: "+input))
		m.refreshViewport()
	}

	m.viewport.GotoBottom()
	return m, nil
}

// slashCommandNeedsArg lists commands that require an argument typed after them.
// These are filled into the textarea but not immediately submitted so the user
// can append the argument before pressing Enter again.
var slashCommandNeedsArg = map[string]bool{
	"/model":  true,
	"/resume": true,
	"/mcp":    true,
}

func (m *Model) acceptSlashCommand() (tea.Model, tea.Cmd) {
	if len(m.slashMatches) == 0 {
		return m, nil
	}
	cmd := m.slashMatches[m.slashIdx]
	m.textarea.SetValue(cmd.Name)
	m.textarea.CursorEnd()
	m.showSlash = false
	// Submit immediately for commands that don't need an argument.
	if !slashCommandNeedsArg[cmd.Name] {
		return m.submitInput()
	}
	return m, nil
}

func (m *Model) updateSlashMenu() {
	val := m.textarea.Value()
	if strings.HasPrefix(val, "/") && !strings.Contains(val, "\n") {
		m.slashMatches = matchSlashCommands(val, m.cfg.CustomCommands)
		m.showSlash = len(m.slashMatches) > 0
		if m.slashIdx >= len(m.slashMatches) {
			m.slashIdx = 0
		}
	} else {
		m.showSlash = false
	}
}

func (m *Model) ensureAssistantMsg() {
	if len(m.messages) == 0 || m.messages[len(m.messages)-1].Role != RoleAssistant ||
		!m.messages[len(m.messages)-1].Streaming {
		m.messages = append(m.messages, newAssistantMessage())
	}
}

// effectiveInputRows computes the visual row count for the current textarea
// value, accounting for both explicit newlines and word-wrap at the current
// terminal width. The result is clamped to [inputMinLines, inputMaxLines].
func (m Model) effectiveInputRows() int {
	w := m.width - 4 // textarea inner width (border + padding)
	if w <= 0 {
		w = 80
	}
	total := 0
	for _, line := range strings.Split(m.textarea.Value(), "\n") {
		runeLen := len([]rune(line))
		if runeLen == 0 {
			total++
		} else {
			total += (runeLen + w - 1) / w // ceiling division
		}
	}
	if total < inputMinLines {
		return inputMinLines
	}
	if total > inputMaxLines {
		return inputMaxLines
	}
	return total
}

// showInputCount returns true when the char/line indicator should be shown.
func (m Model) showInputCount() bool {
	val := m.textarea.Value()
	return len([]rune(val)) > countThreshold || strings.ContainsRune(val, '\n')
}

func (m Model) handleResize() Model {
	m.width = max(m.width, 40)
	m.height = max(m.height, 10)

	rows := m.effectiveInputRows()
	countH := 0
	if m.showInputCount() {
		countH = 1
	}
	inputH := rows + 2 + countH // content rows + top/bottom border + optional count
	vpH := m.height - headerHeight - footerHeight - inputH - 2
	if vpH < 4 {
		vpH = 4
	}

	m.viewport.Width = m.width
	m.viewport.Height = vpH
	m.textarea.SetWidth(m.width - 4)
	m.textarea.SetHeight(rows)

	m.vpReady = true
	m.refreshViewport()
	if m.atBottom {
		m.viewport.GotoBottom()
	}
	return m
}

func (m *Model) refreshViewport() {
	if !m.vpReady {
		return
	}
	m.viewport.SetContent(m.renderConversation())
}

// renderConversation renders the full conversation for the viewport.
func (m *Model) renderConversation() string {
	innerWidth := m.width - 4
	if innerWidth < 20 {
		innerWidth = 20
	}
	var sb strings.Builder
	for i := range m.messages {
		if sb.Len() > 0 {
			sb.WriteByte('\n')
		}
		sb.WriteString(m.renderMessageAt(i, innerWidth))
	}
	return sb.String()
}

func helpText() string {
	return strings.Join([]string{
		"",
		"  Keyboard shortcuts",
		"  ─────────────────────────────────",
		"  Enter          Send message",
		"  Shift+Enter    New line",
		"  Up/Down        Navigate input history",
		"  Ctrl+L         Clear screen",
		"  Ctrl+C          Interrupt turn (or quit when idle)",
	"  Ctrl+D          Quit",
		"  PgUp/PgDn      Scroll conversation",
		"",
		"  Slash commands",
		"  ─────────────────────────────────",
		"  /new             Start a fresh session",
		"  /model [name]    Show or switch model",
		"  /sessions        List recent sessions",
		"  /resume <id|n>   Resume a session",
		"  /code            Switch to code mode (file tools)",
		"  /chat            Switch to chat mode (no tools)",
		"  /auto            Toggle auto-apply for code tools",
		"  /mcp [name]      List or activate MCP servers",
		"  /account         Show account info",
		"  /usage           Show token usage",
		"  /update          Check for a newer version and upgrade",
		"  /clear  /reset  /context  /exit",
		"",
	}, "\n")
}

func (m *Model) formatSessions(sessions []SessionInfo) string {
	if len(sessions) == 0 {
		return "No sessions found. Use /new to start one."
	}
	var sb strings.Builder
	sb.WriteString("\n  Recent sessions:\n")
	for i, s := range sessions {
		id8 := s.ID
		if len(id8) > 8 {
			id8 = id8[:8]
		}
		title := s.Title
		if title == "" {
			title = "(untitled)"
		}
		if len(title) > 40 {
			title = title[:37] + "..."
		}
		fmt.Fprintf(&sb, "  %2d.  %s  %s\n", i+1, id8, title)
	}
	sb.WriteString("\n  /resume <number> to switch")
	return sb.String()
}

func formatAccount(info *AccountInfo) string {
	if info == nil {
		return "No account info available."
	}
	return strings.Join([]string{
		"",
		"  Account",
		"  ─────────────────────────────────",
		"  Name:      " + info.Name,
		"  Email:     " + info.Email,
		"  Username:  " + info.Username,
		"",
	}, "\n")
}

// usageBar renders a fixed-width block progress bar, e.g. "████░░░░░░".
func usageBar(pct float64, width int) string {
	if pct < 0 {
		pct = 0
	}
	if pct > 100 {
		pct = 100
	}
	filled := int(pct / 100 * float64(width))
	return strings.Repeat("█", filled) + strings.Repeat("░", width-filled)
}

// usageAlert returns an inline suffix based on consumption level.
func usageAlert(pct float64) string {
	switch {
	case pct >= 100:
		return "  ✗ limit reached"
	case pct >= 95:
		return "  ⚠ almost out"
	case pct >= 90:
		return "  ⚠ running low"
	case pct >= 80:
		return "  ⚠ warning"
	default:
		return ""
	}
}

// formatUsageCompact returns a one-liner shown after each response.
// Returns "" when consumption is below 70% (quiet path).
func formatUsageCompact(info *UsageInfo) string {
	if info == nil {
		return ""
	}
	// Pick the highest-usage window that is actively enforced.
	type window struct {
		pct    float64
		period string
	}
	windows := []window{
		{info.RPMPercent, "rpm"},
		{info.DailyPercent, "daily"},
		{info.MonthlyPercent, "monthly"},
	}
	best := window{}
	for _, w := range windows {
		if w.pct > best.pct {
			best = w
		}
	}
	if best.pct < 80 {
		return ""
	}
	displayPct := best.pct
	if displayPct > 100 {
		displayPct = 100
	}
	suffix := usageAlert(best.pct)
	if best.pct >= 100 && info.RetryAfter > 0 {
		s := info.RetryAfter
		d := s / 86400
		h := (s % 86400) / 3600
		m := (s % 3600) / 60
		switch {
		case d > 0 && h > 0:
			suffix = fmt.Sprintf("  ✗ resets in %dd %dh", d, h)
		case d > 0:
			suffix = fmt.Sprintf("  ✗ resets in %dd", d)
		case h > 0 && m > 0:
			suffix = fmt.Sprintf("  ✗ resets in %dh %dm", h, m)
		case h > 0:
			suffix = fmt.Sprintf("  ✗ resets in %dh", h)
		case m > 0:
			suffix = fmt.Sprintf("  ✗ resets in %dm", m)
		default:
			suffix = fmt.Sprintf("  ✗ resets in %ds", s)
		}
	}
	return fmt.Sprintf("  [%s] %.0f%% %s%s", usageBar(best.pct, 10), displayPct, best.period, suffix)
}

func formatUsage(info *UsageInfo) string {
	if info == nil {
		return "No usage info available."
	}

	lines := []string{
		"",
		"  Usage  ·  plan: " + info.PlanType,
		"  ─────────────────────────────────",
	}

	addRow := func(label string, pct float64) {
		displayPct := pct
		if displayPct > 100 {
			displayPct = 100
		}
		lines = append(lines, fmt.Sprintf("  %-8s [%s] %5.1f%%%s",
			label, usageBar(pct, 10), displayPct, usageAlert(pct)))
	}

	rpmDisplayPct := info.RPMPercent
	if rpmDisplayPct > 100 {
		rpmDisplayPct = 100
	}
	lines = append(lines, fmt.Sprintf("  RPM      %d/%d [%s] %5.1f%%%s",
		info.RPMUsed, info.RPMLimit, usageBar(info.RPMPercent, 10), rpmDisplayPct, usageAlert(info.RPMPercent)))
	addRow("Daily", info.DailyPercent)
	addRow("Monthly", info.MonthlyPercent)

	if info.RetryAfter > 0 {
		s := info.RetryAfter
		d := s / 86400
		h := (s % 86400) / 3600
		m := (s % 3600) / 60
		var retryStr string
		switch {
		case d > 0 && h > 0:
			retryStr = fmt.Sprintf("%dd %dh", d, h)
		case d > 0:
			retryStr = fmt.Sprintf("%dd", d)
		case h > 0 && m > 0:
			retryStr = fmt.Sprintf("%dh %dm", h, m)
		case h > 0:
			retryStr = fmt.Sprintf("%dh", h)
		case m > 0:
			retryStr = fmt.Sprintf("%dm", m)
		default:
			retryStr = fmt.Sprintf("%ds", s)
		}
		lines = append(lines, fmt.Sprintf("  ✗ Rate limited — retry in %s", retryStr))
	}

	if info.TotalTokens > 0 {
		lines = append(lines, "")
		lines = append(lines, fmt.Sprintf("  Input:   %d tokens", info.InputTokens))
		lines = append(lines, fmt.Sprintf("  Output:  %d tokens", info.OutputTokens))
		lines = append(lines, fmt.Sprintf("  Total:   %d tokens", info.TotalTokens))
	}
	lines = append(lines, "")
	return strings.Join(lines, "\n")
}

func formatMCPList(items []MCPInfo) string {
	if len(items) == 0 {
		return "No MCP servers available. Visit your account to configure integrations."
	}
	var sb strings.Builder
	sb.WriteString("\n  MCP servers:\n")
	for _, s := range items {
		status := "○"
		if s.Available {
			status = "●"
		}
		desc := s.Description
		if len(desc) > 45 {
			desc = desc[:42] + "..."
		}
		line := fmt.Sprintf("  %s  %-20s  %s", status, s.Name, desc)
		sb.WriteString(line + "\n")
	}
	sb.WriteString("\n  /mcp <name> to activate  ·  ● = available")
	return sb.String()
}

// handleStreamEvent processes one StreamEvent from the channel and returns any
// follow-on cmds (always includes the next read unless the stream is done).
func (m *Model) handleStreamEvent(ev StreamEvent) []tea.Cmd {
	var cmds []tea.Cmd

	switch ev.Kind {
	case "chunk":
		m.ensureAssistantMsg()
		last := len(m.messages) - 1
		m.messages[last].appendChunk(ev.Chunk)
		m.refreshViewport()
		if m.atBottom {
			m.viewport.GotoBottom()
		}
		cmds = append(cmds, waitForStreamEvent(m.streamCh, m.streamStop))

	case "tool_call":
		m.ensureAssistantMsg()
		last := len(m.messages) - 1
		m.messages[last].addToolEvent(ToolEvent{
			Kind: ToolCall,
			Name: ev.ToolName,
			Args: ev.ToolArgs,
		})
		m.refreshViewport()
		cmds = append(cmds, waitForStreamEvent(m.streamCh, m.streamStop))

	case "tool_exec":
		m.ensureAssistantMsg()
		last := len(m.messages) - 1
		m.messages[last].addToolEvent(ToolEvent{
			Kind:       ToolExec,
			Name:       ev.ToolName,
			Status:     ev.Status,
			DurationMs: ev.DurationMs,
			Summary:    ev.Summary,
		})
		m.refreshViewport()
		cmds = append(cmds, waitForStreamEvent(m.streamCh, m.streamStop))

	case "progress":
		m.ensureAssistantMsg()
		last := len(m.messages) - 1
		m.messages[last].addToolEvent(ToolEvent{
			Kind:      ToolProgress,
			Iteration: ev.Iteration,
			Tools:     ev.Tools,
		})
		m.refreshViewport()
		cmds = append(cmds, waitForStreamEvent(m.streamCh, m.streamStop))

	case "heartbeat":
		cmds = append(cmds, waitForStreamEvent(m.streamCh, m.streamStop))

	case "approval":
		// The stream goroutine already created a reply channel; store it.
		// The stream pump is suspended until handleApprovalKey sends the answer.
		ap := ApprovalRequestMsg{ToolName: ev.ToolName, Args: ev.ToolArgs, ReplyCh: ev.ReplyCh}
		m.pendingApproval = &ap
		m.refreshViewport()
		// No next-read cmd here — handleApprovalKey will resume the stream pump.

	case "rate_limited":
		// Rate limit hit — shown as a styled system notice, not an AI chat bubble.
		// Finish any in-flight assistant message; if it has no content (pre-block),
		// remove it so there's no blank bubble.
		m.streaming = false
		if last := len(m.messages) - 1; last >= 0 && m.messages[last].Role == RoleAssistant {
			m.messages[last].finishStreaming()
			if m.messages[last].Content == "" && len(m.messages[last].ToolEvents) == 0 {
				m.messages = m.messages[:last]
			}
		}
		m.messages = append(m.messages, newSystemMessage("⚠ "+ev.ErrMsg))
		m.refreshViewport()
		m.viewport.GotoBottom()
		m.textarea.Focus()
		cmds = append(cmds, waitForStreamEvent(m.streamCh, m.streamStop))

	case "live_usage_pct":
		// Live usage tick during streaming — update the header bar and continue.
		// Do NOT drain the stream; the response is still in progress.
		m.liveUsagePct = ev.LivePct
		cmds = append(cmds, waitForStreamEvent(m.streamCh, m.streamStop))

	case "usage_warning":
		// Show a non-blocking warning and continue reading the stream.
		// The response is still in progress; the done event follows shortly.
		m.messages = append(m.messages, newSystemMessage("⚠ "+ev.ErrMsg))
		m.refreshViewport()
		cmds = append(cmds, waitForStreamEvent(m.streamCh, m.streamStop))

	case "error":
		m.streaming = false
		m.messages = append(m.messages, newSystemMessage("Error: "+ev.ErrMsg))
		m.refreshViewport()
		m.viewport.GotoBottom()
		m.textarea.Focus()

	case "done":
		m.streaming = false
		m.liveUsagePct = 0
		// Search backwards: a usage_warning system message may have been appended
		// after the assistant message, so the last entry might not be the assistant.
		newlyFinalizedIdx := -1
		for i := len(m.messages) - 1; i >= 0; i-- {
			if m.messages[i].Role == RoleAssistant && m.messages[i].Streaming {
				m.messages[i].finishStreaming()
				newlyFinalizedIdx = i
				break
			}
		}
		if ev.UsagePromptTokens > 0 || ev.UsageCompletionTokens > 0 {
			m.totalPromptTokens += ev.UsagePromptTokens
			m.totalCompletionTokens += ev.UsageCompletionTokens
		}
		m.refreshViewport()
		if newlyFinalizedIdx >= 0 && m.messages[newlyFinalizedIdx].Content != "" {
			// Scroll viewport to the top of the response so it's immediately visible.
			m.scrollToMessageStart(newlyFinalizedIdx)
		} else {
			m.viewport.GotoBottom()
		}
		// Sync atBottom so UsageAutoCheckMsg doesn't force-scroll past the response.
		m.atBottom = m.viewport.AtBottom()
		m.textarea.Focus()
		cmds = append(cmds, textarea.Blink)
		// Silently check usage after each response; shows alert only when >= 80%.
		// Wait 600ms so cai-mcp-go has time to record stream_end tokens to KV
		// before we query — avoids showing stale pre-response percentage.
		if m.cfg.UsageFn != nil {
			fn := m.cfg.UsageFn
			cmds = append(cmds, func() tea.Msg {
				time.Sleep(600 * time.Millisecond)
				info, err := fn()
				return UsageAutoCheckMsg{Info: info, Err: err}
			})
		}
	}

	return cmds
}

// scrollToMessageStart positions the viewport so the user sees the beginning
// of the message at msgIdx. The viewport clamps YOffset automatically so this
// is equivalent to GotoBottom when the remaining content fits on screen.
func (m *Model) scrollToMessageStart(msgIdx int) {
	innerWidth := m.width - 4
	if innerWidth < 20 {
		innerWidth = 20
	}
	var sb strings.Builder
	for i := 0; i < msgIdx; i++ {
		if sb.Len() > 0 {
			sb.WriteByte('\n')
		}
		sb.WriteString(m.renderMessageAt(i, innerWidth))
	}
	linesBefore := 0
	if s := sb.String(); s != "" {
		linesBefore = strings.Count(s, "\n") + 1
	}
	m.viewport.YOffset = linesBefore
	// viewport clamps to maxYOffset internally, so short content auto-bottoms
}

// waitForStreamEvent returns a tea.Cmd that blocks until the next StreamEvent
// arrives on ch or the stop channel is closed (Ctrl+C interrupt).
func waitForStreamEvent(ch <-chan StreamEvent, stop <-chan struct{}) tea.Cmd {
	return func() tea.Msg {
		select {
		case ev, ok := <-ch:
			if !ok {
				return StreamDoneMsg{}
			}
			return ev
		case <-stop:
			return InterruptedMsg{}
		}
	}
}
