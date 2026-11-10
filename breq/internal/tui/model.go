// Package tui provides an interactive TUI for the bluerequests platform.
package tui

import (
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	pb "github.com/bluefunda/bluerequests/api/proto/bff"
	trmgrpc "github.com/bluefunda/bluerequests/internal/grpc"
)

// fetchTimeout is the gRPC deadline for all TUI background calls.
const fetchTimeout = 30 * time.Second

// view tracks which screen is active.
type view int

const (
	viewList   view = iota
	viewDetail view = iota
)

// ── root model ────────────────────────────────────────────────────────────────

type Model struct {
	conn   *trmgrpc.Conn
	client pb.BFFServiceClient
	width  int
	height int

	active   view
	list     listModel
	detail   detailModel
	quitting bool
}

// New creates the root TUI model. conn must already be authenticated.
func New(conn *trmgrpc.Conn, width, height int) Model {
	client := conn.Client
	return Model{
		conn:   conn,
		client: client,
		width:  width,
		height: height,
		active: viewList,
		list:   newListModel(client, width, height),
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(
		m.list.Init(),
		tea.EnterAltScreen,
	)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		var lCmd, dCmd tea.Cmd
		m.list, lCmd = m.list.Update(msg)
		if m.active == viewDetail {
			m.detail, dCmd = m.detail.Update(msg)
		}
		cmds = append(cmds, lCmd, dCmd)
		return m, tea.Batch(cmds...)

	case tea.KeyMsg:
		// Global quit
		if msg.String() == "ctrl+c" {
			m.quitting = true
			return m, tea.Quit
		}
		if m.active == viewList && msg.String() == "q" && !m.list.filtering {
			m.quitting = true
			return m, tea.Quit
		}
		if m.active == viewDetail && msg.String() == "q" {
			m.quitting = true
			return m, tea.Quit
		}

		// Route Enter (open detail) from list view
		if m.active == viewList && msg.String() == "enter" && !m.list.filtering {
			if cr := m.list.selectedCR(); cr != nil {
				m.active = viewDetail
				dm := newDetailModel(m.client, cr, m.width, bodyHeight(m.height))
				m.detail = dm
				return m, dm.Init()
			}
			return m, nil
		}

		// Route Esc (back to list) from detail view
		if m.active == viewDetail && (msg.String() == "esc" || msg.String() == "backspace") {
			m.active = viewList
			return m, nil
		}
	}

	// Delegate to active view
	var cmd tea.Cmd
	switch m.active {
	case viewList:
		m.list, cmd = m.list.Update(msg)
	case viewDetail:
		m.detail, cmd = m.detail.Update(msg)
	}
	cmds = append(cmds, cmd)
	return m, tea.Batch(cmds...)
}

func (m Model) View() string {
	if m.quitting {
		return ""
	}

	var header, body, footer string

	switch m.active {
	case viewList:
		header = renderAppHeader(m.width, "Change Requests")
		body = m.list.View(m.width, bodyHeight(m.height))
		footer = listFooter(m.width)
	case viewDetail:
		header = "" // detail has its own header
		body = m.detail.View(m.width, bodyHeight(m.height))
		footer = detailFooter(m.width)
	}

	sections := []string{}
	if header != "" {
		sections = append(sections, header)
	}
	sections = append(sections, body)
	sections = append(sections, footer)
	return strings.Join(sections, "\n")
}

// bodyHeight returns usable height for the body region.
func bodyHeight(total int) int {
	// app header(1) + footer(1) + newlines(2)
	h := total - 4
	if h < 4 {
		h = 4
	}
	return h
}

// renderAppHeader renders the top bar with the platform name and section title.
func renderAppHeader(width int, section string) string {
	brand := sHeaderTitle.Render("bluerequests") + sMuted.Render("  •  ") + sMuted.Render(section)
	right := sMuted.Render("? help  q quit")
	gap := width - lipgloss.Width(brand) - lipgloss.Width(right)
	if gap < 0 {
		gap = 0
	}
	return sHeader.Width(width).Render(brand + strings.Repeat(" ", gap) + right)
}

// Run starts the TUI program. It blocks until the user quits.
func Run(conn *trmgrpc.Conn) error {
	m := New(conn, 0, 0)
	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err := p.Run()
	return err
}
