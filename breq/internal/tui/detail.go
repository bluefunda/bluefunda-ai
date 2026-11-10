package tui

import (
	"context"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	pb "github.com/bluefunda/bluerequests/api/proto/bff"
)

// ── messages ─────────────────────────────────────────────────────────────────

type commentsLoadedMsg struct{ comments []*pb.Comment }
type commentsErrMsg struct{ err error }

// ── workflow stages ───────────────────────────────────────────────────────────

// crWorkflowStages is the ordered stage list used for the timeline.
// The BFF currently surfaces simplified status strings; the full SAP
// stage constants are kept as aliases so the display works regardless
// of which representation the BFF returns.
var crWorkflowStages = []string{
	"planned",
	"inprogress",
	"completed",
}

// stageAliases maps alternate/SAP-style status strings to the canonical
// simplified form used in crWorkflowStages.
var stageAliases = map[string]string{
	// BFF canonical
	"planned":    "planned",
	"inprogress": "inprogress",
	"completed":  "completed",
	// SAP-style variants
	"PLANNED":                    "planned",
	"IN_PROGRESS":                "inprogress",
	"QA":                         "inprogress",
	"TOC_RELEASE_INITIATED":      "inprogress",
	"TOC_RELEASED":               "inprogress",
	"TOC_IMPORTING_TO_QA":        "inprogress",
	"TOC_IMPORTED_TO_QA":         "inprogress",
	"UAT":                        "inprogress",
	"UAT_APPROVED":               "inprogress",
	"RELEASE_MANAGEMENT":         "inprogress",
	"TRANSPORT_RELEASE_FROM_DEV": "inprogress",
	"TRANSPORT_RELEASED":         "inprogress",
	"IMPORT_TO_PROD":             "inprogress",
	"SCHEDULE_IMPORT":            "inprogress",
	"IMPORT_IS_SCHEDULED":        "inprogress",
	"IMPORTED":                   "completed",
}

// stageIndex returns the position of s in crWorkflowStages, or -1.
func stageIndex(s string) int {
	canonical, ok := stageAliases[s]
	if !ok {
		canonical = strings.ToLower(s)
	}
	for i, stage := range crWorkflowStages {
		if stage == canonical {
			return i
		}
	}
	return -1
}

// ── tabs ─────────────────────────────────────────────────────────────────────

type detailTab int

const (
	tabDetails  detailTab = 0
	tabComments detailTab = 1
)

// ── model ─────────────────────────────────────────────────────────────────────

type detailModel struct {
	client   pb.BFFServiceClient
	cr       *pb.ChangeRequest
	comments []*pb.Comment
	loading  bool
	err      error
	tab      detailTab
	vp       viewport.Model
	width    int
	height   int
}

func newDetailModel(client pb.BFFServiceClient, cr *pb.ChangeRequest, width, height int) detailModel {
	vp := viewport.New(width, contentHeight(height))
	vp.Style = sDetailValue
	return detailModel{
		client:  client,
		cr:      cr,
		loading: true,
		tab:     tabDetails,
		vp:      vp,
		width:   width,
		height:  height,
	}
}

func contentHeight(h int) int {
	// header(1) + tabs(1) + divider(1) + footer(1) + padding(2)
	reserved := 6
	n := h - reserved
	if n < 4 {
		n = 4
	}
	return n
}

func (m detailModel) fetchCommentsCmd() tea.Cmd {
	client := m.client
	id := m.cr.Id
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), fetchTimeout)
		defer cancel()
		resp, err := client.ListComments(ctx, &pb.ListCommentsRequest{ChangeRequestId: id})
		if err != nil {
			return commentsErrMsg{err}
		}
		return commentsLoadedMsg{resp.Comments}
	}
}

func (m detailModel) Init() tea.Cmd {
	return m.fetchCommentsCmd()
}

func (m detailModel) Update(msg tea.Msg) (detailModel, tea.Cmd) {
	var cmds []tea.Cmd
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.vp.Width = msg.Width
		m.vp.Height = contentHeight(msg.Height)
		m.refreshViewport()

	case commentsLoadedMsg:
		m.loading = false
		m.comments = msg.comments
		m.refreshViewport()

	case commentsErrMsg:
		m.loading = false
		m.err = msg.err

	case tea.KeyMsg:
		switch {
		case key.Matches(msg, detailKeys.TabNext):
			if m.tab == tabDetails {
				m.tab = tabComments
			} else {
				m.tab = tabDetails
			}
			m.refreshViewport()

		default:
			var cmd tea.Cmd
			m.vp, cmd = m.vp.Update(msg)
			cmds = append(cmds, cmd)
		}
	}
	return m, tea.Batch(cmds...)
}

func (m *detailModel) refreshViewport() {
	m.vp.SetContent(m.renderContent())
}

func (m detailModel) renderContent() string {
	switch m.tab {
	case tabDetails:
		return m.renderDetails()
	case tabComments:
		return m.renderComments()
	}
	return ""
}

func (m detailModel) renderDetails() string {
	cr := m.cr
	var b strings.Builder

	field := func(label, value string) {
		b.WriteString(sDetailLabel.Render(label) + "  " + sDetailValue.Render(value) + "\n")
	}

	b.WriteString(sPanelTitle.Width(m.width-4).Render("Details") + "\n\n")
	field("ID", cr.Id)
	field("Description", cr.Description)
	field("Status", StatusBadge(cr.Status))
	field("Severity", SeverityBadge(cr.Severity))
	field("Type", cr.RequestType)
	field("Owner", cr.RequestOwner)
	field("Assignee", cr.Assignee)
	field("Project", cr.ProjectId)
	field("Created", cr.CreatedAt)
	field("Updated", cr.UpdatedAt)

	b.WriteString("\n")
	b.WriteString(sPanelTitle.Width(m.width-4).Render("Workflow Stage") + "\n\n")
	b.WriteString(m.renderTimeline())

	return b.String()
}

func (m detailModel) renderTimeline() string {
	current := stageIndex(m.cr.Status)
	var b strings.Builder
	for i, stage := range crWorkflowStages {
		var icon, line string
		switch {
		case i < current:
			icon = "●"
			line = sStageDone.Render(icon + " " + stage)
		case i == current:
			icon = "▶"
			line = sStageCurrent.Render(icon + " " + stage + "  ← current")
		default:
			icon = "○"
			line = sStagePending.Render(icon + " " + stage)
		}
		b.WriteString("  " + line + "\n")
	}
	return b.String()
}

func (m detailModel) renderComments() string {
	var b strings.Builder
	b.WriteString(sPanelTitle.Width(m.width-4).Render(fmt.Sprintf("Comments (%d)", len(m.comments))) + "\n\n")

	if m.loading {
		b.WriteString(sMuted.Render("  Loading…") + "\n")
		return b.String()
	}
	if m.err != nil {
		b.WriteString(sError.Render("  Error: "+m.err.Error()) + "\n")
		return b.String()
	}
	if len(m.comments) == 0 {
		b.WriteString(sMuted.Render("  No comments yet.") + "\n")
		return b.String()
	}

	sep := sDivider.Render(strings.Repeat("─", m.width-4))
	for i, c := range m.comments {
		if i > 0 {
			b.WriteString("  " + sep + "\n")
		}
		b.WriteString("  " + sCommentAuthor.Render(c.CreatedBy) + "  " + sCommentMeta.Render(c.CreatedAt) + "\n")
		// Indent message body
		for _, ln := range strings.Split(c.Message, "\n") {
			b.WriteString("  " + sDetailValue.Render(ln) + "\n")
		}
		b.WriteString("\n")
	}
	return b.String()
}

// View renders the full detail pane.
func (m detailModel) View(width, height int) string {
	cr := m.cr
	var b strings.Builder

	// Breadcrumb / title
	title := sHeaderTitle.Render(cr.Id) + "  " +
		sMuted.Render(truncate(cr.Description, 60))
	badges := StatusBadge(cr.Status) + "  " + SeverityBadge(cr.Severity)
	titleLine := lipgloss.JoinHorizontal(lipgloss.Top, title, lipgloss.NewStyle().Width(width-lipgloss.Width(title)-lipgloss.Width(badges)-2).Render(""), badges)
	b.WriteString(sHeader.Width(width).Render(titleLine) + "\n")

	// Tabs
	det := sTabInactive.Render("Details")
	cmt := sTabInactive.Render("Comments")
	if m.tab == tabDetails {
		det = sTabActive.Render("Details")
	} else {
		cmt = sTabActive.Render("Comments")
	}
	b.WriteString("  " + det + "   " + cmt + "\n")
	b.WriteString(sDivider.Render(strings.Repeat("─", width)) + "\n")

	// Viewport
	b.WriteString(m.vp.View() + "\n")

	return b.String()
}

// detailFooter returns the key hint bar for the detail view.
func detailFooter(width int) string {
	hints := strings.Join([]string{
		footerHint("esc", "back"),
		footerHint("tab", "switch panel"),
		footerHint("↑↓/jk", "scroll"),
		footerHint("q", "quit"),
	}, "  ")
	return sFooter.Width(width).Render(hints)
}
