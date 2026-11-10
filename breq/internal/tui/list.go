package tui

import (
	"context"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	pb "github.com/bluefunda/bluerequests/api/proto/bff"
)

// ── messages ─────────────────────────────────────────────────────────────────

type crsLoadedMsg struct{ crs []*pb.ChangeRequest }
type crsErrMsg struct{ err error }

// ── model ─────────────────────────────────────────────────────────────────────

type listModel struct {
	client    pb.BFFServiceClient
	items     []*pb.ChangeRequest
	filtered  []*pb.ChangeRequest
	cursor    int
	width     int
	height    int
	loading   bool
	err       error
	filter    textinput.Model
	filtering bool
	spinner   spinner.Model
}

func newListModel(client pb.BFFServiceClient, width, height int) listModel {
	ti := textinput.New()
	ti.Placeholder = "filter…"
	ti.CharLimit = 60

	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = sInfo

	return listModel{
		client:  client,
		width:   width,
		height:  height,
		loading: true,
		filter:  ti,
		spinner: sp,
	}
}

func (m listModel) fetchCmd() tea.Cmd {
	client := m.client
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), fetchTimeout)
		defer cancel()
		resp, err := client.ListChangeRequests(ctx, &pb.ListChangeRequestsRequest{})
		if err != nil {
			return crsErrMsg{err}
		}
		out := make([]*pb.ChangeRequest, 0, len(resp.ChangeRequests))
		for _, cr := range resp.ChangeRequests {
			if cr != nil && cr.Id != "" && cr.Id != "<nil>" {
				out = append(out, cr)
			}
		}
		return crsLoadedMsg{out}
	}
}

func (m listModel) Init() tea.Cmd {
	return tea.Batch(m.fetchCmd(), m.spinner.Tick)
}

func (m listModel) Update(msg tea.Msg) (listModel, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case crsLoadedMsg:
		m.loading = false
		m.items = msg.crs
		m.applyFilter()

	case crsErrMsg:
		m.loading = false
		m.err = msg.err

	case spinner.TickMsg:
		if m.loading {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			cmds = append(cmds, cmd)
		}

	case tea.KeyMsg:
		if m.filtering {
			switch msg.String() {
			case "enter", "esc":
				m.filtering = false
				m.filter.Blur()
				m.cursor = 0
			default:
				var cmd tea.Cmd
				m.filter, cmd = m.filter.Update(msg)
				cmds = append(cmds, cmd)
				m.applyFilter()
				m.cursor = 0
			}
			return m, tea.Batch(cmds...)
		}

		switch {
		case key.Matches(msg, listKeys.Filter):
			m.filtering = true
			m.filter.Focus()
			cmds = append(cmds, textinput.Blink)

		case key.Matches(msg, listKeys.Up):
			if m.cursor > 0 {
				m.cursor--
			}

		case key.Matches(msg, listKeys.Down):
			if m.cursor < len(m.filtered)-1 {
				m.cursor++
			}

		case key.Matches(msg, listKeys.PageUp):
			rows := m.visibleRows()
			m.cursor -= rows
			if m.cursor < 0 {
				m.cursor = 0
			}

		case key.Matches(msg, listKeys.PageDown):
			rows := m.visibleRows()
			m.cursor += rows
			if m.cursor >= len(m.filtered) {
				m.cursor = len(m.filtered) - 1
			}

		case key.Matches(msg, listKeys.Refresh):
			m.loading = true
			m.err = nil
			cmds = append(cmds, m.fetchCmd(), m.spinner.Tick)
		}
	}

	return m, tea.Batch(cmds...)
}

func (m *listModel) applyFilter() {
	term := strings.ToLower(strings.TrimSpace(m.filter.Value()))
	if term == "" {
		m.filtered = m.items
		return
	}
	out := m.filtered[:0:0]
	for _, cr := range m.items {
		if strings.Contains(strings.ToLower(cr.Description), term) ||
			strings.Contains(strings.ToLower(cr.Id), term) ||
			strings.Contains(strings.ToLower(cr.Status), term) ||
			strings.Contains(strings.ToLower(cr.RequestOwner), term) ||
			strings.Contains(strings.ToLower(cr.Severity), term) {
			out = append(out, cr)
		}
	}
	m.filtered = out
}

// selectedCR returns the CR at the cursor, or nil.
func (m listModel) selectedCR() *pb.ChangeRequest {
	if len(m.filtered) == 0 || m.cursor >= len(m.filtered) {
		return nil
	}
	return m.filtered[m.cursor]
}

func (m listModel) visibleRows() int {
	// header(1) + colheader(1) + divider(1) + footer(1) + filter(1) + padding(2)
	reserved := 7
	n := m.height - reserved
	if n < 1 {
		n = 1
	}
	return n
}

// ── View ──────────────────────────────────────────────────────────────────────

func (m listModel) View(width, height int) string {
	if m.loading {
		return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center,
			m.spinner.View()+" "+sMuted.Render("Loading change requests…"))
	}
	if m.err != nil {
		return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center,
			sError.Render("Error: "+m.err.Error()))
	}

	var b strings.Builder

	// Filter bar
	filterLabel := sFilterInactive.Render("filter:")
	if m.filtering {
		filterLabel = sFilterActive.Render("filter:")
	}
	filterLine := filterLabel + " " + m.filter.View()
	count := sMuted.Render(fmt.Sprintf("  %d result(s)", len(m.filtered)))
	b.WriteString(filterLine + count + "\n")
	b.WriteString(sDivider.Render(strings.Repeat("─", width)) + "\n")

	// Column widths
	idW := 14
	sevW := 10
	statusW := 22
	ownerW := 16
	descW := width - idW - sevW - statusW - ownerW - 8
	if descW < 10 {
		descW = 10
	}

	// Column headers
	hdr := fmt.Sprintf("%-*s  %-*s  %-*s  %-*s  %-*s",
		idW, "ID",
		descW, "DESCRIPTION",
		statusW, "STATUS",
		sevW, "SEVERITY",
		ownerW, "OWNER",
	)
	b.WriteString(sColHeader.Render(hdr) + "\n")
	b.WriteString(sDivider.Render(strings.Repeat("─", width)) + "\n")

	// Rows
	rows := m.visibleRows()
	offset := 0
	if m.cursor >= rows {
		offset = m.cursor - rows + 1
	}

	for i := offset; i < len(m.filtered) && i < offset+rows; i++ {
		cr := m.filtered[i]
		id := truncate(cr.Id, idW)
		desc := truncate(cr.Description, descW)
		status := truncate(cr.Status, statusW)
		sev := truncate(cr.Severity, sevW)
		owner := truncate(cr.RequestOwner, ownerW)

		line := fmt.Sprintf("%-*s  %-*s  %-*s  %-*s  %-*s",
			idW, id,
			descW, desc,
			statusW, StatusBadge(status),
			sevW, SeverityBadge(sev),
			ownerW, owner,
		)

		if i == m.cursor {
			b.WriteString(sRowSelected.Render(line) + "\n")
		} else {
			b.WriteString(sRowNormal.Render(line) + "\n")
		}
	}

	return b.String()
}

// listFooter returns the key hint bar for the list view.
func listFooter(width int) string {
	hints := strings.Join([]string{
		footerHint("↑↓/jk", "navigate"),
		footerHint("/", "filter"),
		footerHint("enter", "open"),
		footerHint("r", "refresh"),
		footerHint("q", "quit"),
	}, "  ")
	return sFooter.Width(width).Render(hints)
}

// truncate shortens s to max n runes, appending "…" if truncated.
func truncate(s string, n int) string {
	if n <= 0 {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	if n <= 1 {
		return "…"
	}
	return string(runes[:n-1]) + "…"
}
