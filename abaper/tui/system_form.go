package tui

import (
	"context"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/bluefunda/abaper/internal/client"
	"github.com/bluefunda/abaper/internal/config"
	"github.com/bluefunda/abaper/styles"
)

// ── messages ─────────────────────────────────────────────────────────────────

// SystemFormSavedMsg is emitted when the user saves a system.
type SystemFormSavedMsg struct{ System config.SAPSystem }

// SystemFormCancelledMsg is emitted when the user cancels.
type SystemFormCancelledMsg struct{}

type systemTestResultMsg struct{ err error }

// ── field indices ─────────────────────────────────────────────────────────────

const (
	sfName = iota
	sfHost
	sfClient
	sfUsername
	sfPassword
	sfCount
)

var sfLabels = [sfCount]string{
	"Display Name",
	"Host URL",
	"Client",
	"Username",
	"Password",
}

var sfPlaceholders = [sfCount]string{
	"My DEV (optional)",
	"https://sap-host:44300",
	"100",
	"SAPUSER",
	"",
}

// ── styles ────────────────────────────────────────────────────────────────────

var (
	sfFormBox  = styles.StyleAccentBorder.Padding(1, 3)
	sfTitle    = lipgloss.NewStyle().Bold(true).Foreground(styles.ColorAccent)
	sfLabel    = lipgloss.NewStyle().Foreground(styles.ColorMuted).Width(16)
	sfInputOn  = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(styles.ColorAccent).Padding(0, 1)
	sfInputOff = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(styles.ColorBorder).Padding(0, 1)
	sfHint     = lipgloss.NewStyle().Foreground(styles.ColorMuted).Italic(true)
	sfSuccess  = styles.StyleSuccess
	sfErr      = styles.StyleError
)

// ── model ─────────────────────────────────────────────────────────────────────

type systemFormModel struct {
	inputs  [sfCount]textinput.Model
	focused int
	editID  string
	testing bool
	spinner spinner.Model
	testMsg string
	testOK  bool
	saveErr string
	width   int
	height  int
}

func newSystemForm(sys *config.SAPSystem) *systemFormModel {
	m := &systemFormModel{}

	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(styles.ColorAccent)
	m.spinner = sp

	for i := range m.inputs {
		t := textinput.New()
		t.Prompt = ""
		t.Width = 46
		t.CharLimit = 512
		t.Placeholder = sfPlaceholders[i]
		m.inputs[i] = t
	}
	m.inputs[sfClient].SetValue("100")
	m.inputs[sfPassword].EchoMode = textinput.EchoPassword
	m.inputs[sfPassword].EchoCharacter = '•'

	if sys != nil {
		m.editID = sys.ID
		m.inputs[sfName].SetValue(sys.Name)
		m.inputs[sfHost].SetValue(sys.Host)
		m.inputs[sfClient].SetValue(sys.Client)
		m.inputs[sfUsername].SetValue(sys.Username)
		m.inputs[sfPassword].SetValue(sys.Password)
	}

	m.inputs[m.focused].Focus()
	return m
}

func (m *systemFormModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m *systemFormModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case spinner.TickMsg:
		if m.testing {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			return m, cmd
		}
		return m, nil

	case systemTestResultMsg:
		m.testing = false
		if msg.err != nil {
			m.testMsg = "Connection failed: " + msg.err.Error()
			m.testOK = false
		} else {
			m.testMsg = "Connection successful!"
			m.testOK = true
		}
		return m, nil

	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyEsc:
			return m, func() tea.Msg { return SystemFormCancelledMsg{} }

		case tea.KeyTab, tea.KeyDown:
			m.inputs[m.focused].Blur()
			m.focused = (m.focused + 1) % sfCount
			m.inputs[m.focused].Focus()
			return m, textinput.Blink

		case tea.KeyShiftTab, tea.KeyUp:
			m.inputs[m.focused].Blur()
			m.focused = (m.focused - 1 + sfCount) % sfCount
			m.inputs[m.focused].Focus()
			return m, textinput.Blink

		case tea.KeyCtrlT:
			return m, m.runTest()

		case tea.KeyCtrlS:
			return m, m.trySave()

		case tea.KeyEnter:
			if m.focused == sfPassword {
				return m, m.trySave()
			}
			m.inputs[m.focused].Blur()
			m.focused = (m.focused + 1) % sfCount
			m.inputs[m.focused].Focus()
			return m, textinput.Blink
		}
	}

	var cmd tea.Cmd
	m.inputs[m.focused], cmd = m.inputs[m.focused].Update(msg)
	return m, cmd
}

func (m *systemFormModel) runTest() tea.Cmd {
	if m.testing {
		return nil
	}
	host := strings.TrimSpace(m.inputs[sfHost].Value())
	username := strings.TrimSpace(m.inputs[sfUsername].Value())
	password := m.inputs[sfPassword].Value()
	if host == "" || username == "" || password == "" {
		m.testMsg = "Host, Username, and Password are required to test"
		m.testOK = false
		return nil
	}
	m.testing = true
	m.testMsg = ""
	m.saveErr = ""
	sapClient := m.inputs[sfClient].Value()
	if sapClient == "" {
		sapClient = "100"
	}
	return tea.Batch(
		m.spinner.Tick,
		func() tea.Msg {
			c, err := client.NewClient()
			if err != nil {
				return systemTestResultMsg{err: err}
			}
			return systemTestResultMsg{err: c.SystemConnect(context.Background(), host, sapClient, username, password)}
		},
	)
}

func (m *systemFormModel) trySave() tea.Cmd {
	host := strings.TrimSpace(m.inputs[sfHost].Value())
	username := strings.TrimSpace(m.inputs[sfUsername].Value())
	password := m.inputs[sfPassword].Value()
	if host == "" || username == "" || password == "" {
		m.saveErr = "Host, Username, and Password are required"
		return nil
	}
	m.saveErr = ""
	sapClient := m.inputs[sfClient].Value()
	if sapClient == "" {
		sapClient = "100"
	}
	sys := config.SAPSystem{
		ID:       m.editID,
		Name:     strings.TrimSpace(m.inputs[sfName].Value()),
		Host:     host,
		Client:   sapClient,
		Username: username,
		Password: password,
	}
	return func() tea.Msg { return SystemFormSavedMsg{System: sys} }
}

func (m *systemFormModel) View() string {
	title := "Add SAP System"
	if m.editID != "" {
		title = "Edit SAP System"
	}

	var b strings.Builder
	b.WriteString(sfTitle.Render(title) + "\n\n")

	for i := range m.inputs {
		label := sfLabel.Render(sfLabels[i])
		var box string
		if i == m.focused {
			box = sfInputOn.Render(m.inputs[i].View())
		} else {
			box = sfInputOff.Render(m.inputs[i].View())
		}
		b.WriteString(lipgloss.JoinHorizontal(lipgloss.Center, label, box))
		b.WriteString("\n")
	}

	b.WriteString("\n")

	switch {
	case m.testing:
		b.WriteString(sfHint.Render(m.spinner.View() + " Testing connection..."))
	case m.testOK && m.testMsg != "":
		b.WriteString(sfSuccess.Render("✓ " + m.testMsg))
	case !m.testOK && m.testMsg != "":
		b.WriteString(sfErr.Render("✗ " + m.testMsg))
	}
	if m.saveErr != "" {
		if m.testMsg != "" {
			b.WriteString("\n")
		}
		b.WriteString(sfErr.Render("✗ " + m.saveErr))
	}

	b.WriteString("\n\n")
	b.WriteString(sfHint.Render("Tab/↑↓  navigate   Ctrl+T  test   Ctrl+S  save   Esc  cancel"))

	form := sfFormBox.Render(b.String())

	// Center the form in the terminal
	if m.width > 0 && m.height > 0 {
		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, form)
	}
	return form
}
