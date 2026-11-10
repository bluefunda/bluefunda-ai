package tui

import (
	"context"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "github.com/bluefunda/bluerequests/api/proto/bff"
	"github.com/bluefunda/bluerequests/internal/a4h"
	trmgrpc "github.com/bluefunda/bluerequests/internal/grpc"
)

// a4hAPI is the part of pb.BFFServiceClient the session screen uses.
type a4hAPI interface {
	GetA4HSession(ctx context.Context, in *pb.GetA4HSessionRequest, opts ...grpc.CallOption) (*pb.A4HSessionResponse, error)
	StartA4HSession(ctx context.Context, in *pb.StartA4HSessionRequest, opts ...grpc.CallOption) (*pb.A4HSessionResponse, error)
	ExtendA4HSession(ctx context.Context, in *pb.ExtendA4HSessionRequest, opts ...grpc.CallOption) (*pb.A4HSessionResponse, error)
	TerminateA4HSession(ctx context.Context, in *pb.TerminateA4HSessionRequest, opts ...grpc.CallOption) (*pb.A4HSessionResponse, error)
}

// clockInterval is how often the time remaining is redrawn.
const clockInterval = 30 * time.Second

// a4hReplyMsg carries the outcome of any session call.
type a4hReplyMsg struct {
	session *pb.A4HSession
	err     error
}

// a4hPollMsg asks for a status refresh. Only the newest scheduled poll counts
// (seq), so keypresses that trigger calls can't make polling accelerate.
type a4hPollMsg struct{ seq int }

type a4hClockMsg time.Time

type a4hModel struct {
	api    a4hAPI
	width  int
	height int

	session    *pb.A4HSession // nil until the first reply
	errMsg     string
	busy       bool
	confirmEnd bool
	pollSeq    int
	now        time.Time
	quitting   bool
}

func newA4HModel(api a4hAPI, width, height int) a4hModel {
	return a4hModel{api: api, width: width, height: height, now: time.Now()}
}

// RunA4H shows the live session screen until the user quits.
func RunA4H(conn *trmgrpc.Conn) error {
	p := tea.NewProgram(newA4HModel(conn.Client, 0, 0), tea.WithAltScreen())
	_, err := p.Run()
	return err
}

// ── commands ──────────────────────────────────────────────────────────────────

func (m a4hModel) call(f func(ctx context.Context, api a4hAPI) (*pb.A4HSessionResponse, error)) tea.Cmd {
	api := m.api
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), fetchTimeout)
		defer cancel()
		resp, err := f(ctx, api)
		return a4hReplyMsg{session: resp.GetSession(), err: err}
	}
}

func (m a4hModel) fetch() tea.Cmd {
	return m.call(func(ctx context.Context, api a4hAPI) (*pb.A4HSessionResponse, error) {
		return api.GetA4HSession(ctx, &pb.GetA4HSessionRequest{})
	})
}

func (m a4hModel) start() tea.Cmd {
	return m.call(func(ctx context.Context, api a4hAPI) (*pb.A4HSessionResponse, error) {
		return api.StartA4HSession(ctx, &pb.StartA4HSessionRequest{})
	})
}

func (m a4hModel) extend() tea.Cmd {
	return m.call(func(ctx context.Context, api a4hAPI) (*pb.A4HSessionResponse, error) {
		return api.ExtendA4HSession(ctx, &pb.ExtendA4HSessionRequest{})
	})
}

func (m a4hModel) terminate() tea.Cmd {
	return m.call(func(ctx context.Context, api a4hAPI) (*pb.A4HSessionResponse, error) {
		return api.TerminateA4HSession(ctx, &pb.TerminateA4HSessionRequest{})
	})
}

func clockTick() tea.Cmd {
	return tea.Tick(clockInterval, func(t time.Time) tea.Msg { return a4hClockMsg(t) })
}

// schedulePoll arms the next status refresh for the current state, if any, and
// invalidates every earlier one.
func (m a4hModel) schedulePoll() (a4hModel, tea.Cmd) {
	m.pollSeq++
	if m.session == nil {
		return m, nil
	}
	d := a4h.PollInterval(m.session.GetState())
	if d == 0 {
		return m, nil
	}
	seq := m.pollSeq
	return m, tea.Tick(d, func(time.Time) tea.Msg { return a4hPollMsg{seq: seq} })
}

// ── tea.Model ─────────────────────────────────────────────────────────────────

func (m a4hModel) Init() tea.Cmd {
	return tea.Batch(m.fetch(), clockTick())
}

func (m a4hModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		return m, nil

	case a4hClockMsg:
		m.now = time.Time(msg)
		return m, clockTick()

	case a4hReplyMsg:
		m.busy = false
		if msg.err != nil {
			m.errMsg = a4hErrText(msg.err)
		} else {
			m.errMsg = ""
			m.session = msg.session
		}
		// A failed poll must not stop polling; keep asking while a session is in flight.
		return m.schedulePoll()

	case a4hPollMsg:
		if msg.seq != m.pollSeq || m.busy {
			return m, nil
		}
		m.busy = true
		return m, m.fetch()

	case tea.KeyMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

func (m a4hModel) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	if key == "ctrl+c" || (key == "q" && !m.confirmEnd) {
		m.quitting = true
		return m, tea.Quit
	}

	if m.confirmEnd {
		switch key {
		case "y":
			m.confirmEnd, m.busy = false, true
			return m, m.terminate()
		case "n", "esc":
			m.confirmEnd = false
		}
		return m, nil
	}

	if m.busy || m.session == nil {
		if key == "r" && !m.busy {
			m.busy = true
			return m, m.fetch()
		}
		return m, nil
	}
	state := m.session.GetState()
	switch key {
	case "r":
		m.busy = true
		return m, m.fetch()
	case "s":
		if a4h.CanStart(state) {
			m.busy, m.errMsg = true, ""
			return m, m.start()
		}
	case "e":
		if state == a4h.StateSAPRunning {
			m.busy, m.errMsg = true, ""
			return m, m.extend()
		}
	case "x":
		if a4h.IsActive(state) {
			m.confirmEnd = true
		}
	}
	return m, nil
}

// a4hErrText is what the screen says when a call failed.
func a4hErrText(err error) string {
	if trmgrpc.IsAuthError(err) {
		return "Your sign-in has expired. Quit and run 'breq login', then reopen this screen."
	}
	switch status.Code(err) {
	case codes.NotFound:
		return "There is no running session."
	case codes.Unavailable:
		return "The session service is not responding; trying again."
	}
	return "Request failed: " + status.Convert(err).Message()
}

// ── view ──────────────────────────────────────────────────────────────────────

func (m a4hModel) View() string {
	if m.quitting {
		return ""
	}
	width := m.width
	if width <= 0 {
		width = 80
	}

	var b strings.Builder
	b.WriteString(renderAppHeader(width, "SAP ABAP Trial"))
	b.WriteString("\n\n")
	b.WriteString(m.body())
	b.WriteString("\n\n")
	b.WriteString(sFooter.Width(width).Render(m.footer()))
	return b.String()
}

func (m a4hModel) body() string {
	if m.session == nil {
		if m.errMsg != "" {
			return "  " + sError.Render(m.errMsg)
		}
		return "  " + sMuted.Render("Loading…")
	}

	state := m.session.GetState()
	lines := []string{"  " + sDetailLabel.Render("Status  ") + m.statusText(state)}

	if step := a4h.Step(state); step >= 0 {
		lines = append(lines, "", "  "+renderA4HSteps(step))
	}

	if ip := m.session.GetPublicIp(); ip != "" && a4h.IsActive(state) {
		c := a4h.ConnectionFor(ip)
		lines = append(lines, "",
			"  "+sDetailLabel.Render("Host    ")+sDetailValue.Render(c.Host),
			"  "+sDetailLabel.Render("SAP GUI ")+sDetailValue.Render(c.SAPGUI),
			"  "+sDetailLabel.Render("ADT     ")+sDetailValue.Render(c.ADTHTTP+"  "+c.ADTTLS),
		)
		if state != a4h.StateSAPRunning {
			lines = append(lines, "  "+sMuted.Render("SAP is still starting; connect once the status is Ready."))
		}
	}
	if state == a4h.StateSAPRunning && m.session.GetDeadline() != "" {
		lines = append(lines, "",
			"  "+sDetailLabel.Render("Remaining ")+sDetailValue.Render(a4h.Remaining(m.session.GetDeadline(), m.now)))
	}

	if m.confirmEnd {
		lines = append(lines, "", "  "+sError.Render("End your SAP trial system? It will be deleted.")+"  "+
			sKeyName.Render("y")+sMuted.Render(" yes  ")+sKeyName.Render("n")+sMuted.Render(" keep it"))
	}
	if m.errMsg != "" {
		lines = append(lines, "", "  "+sError.Render(m.errMsg))
	}
	if m.busy {
		lines = append(lines, "", "  "+sMuted.Render("Working…"))
	}
	return strings.Join(lines, "\n")
}

func (m a4hModel) statusText(state string) string {
	label := a4h.Label(state)
	switch {
	case state == a4h.StateSAPRunning:
		return sStageDone.Render(label)
	case a4h.IsStarting(state):
		return sStageCurrent.Render(label)
	default:
		return sMuted.Render(label)
	}
}

// renderA4HSteps draws the startup stages: done, in progress, still to come.
func renderA4HSteps(step int) string {
	parts := make([]string, len(a4h.Steps))
	for i, name := range a4h.Steps {
		switch {
		case i < step:
			parts[i] = sStageDone.Render("✓ " + name)
		case i == step:
			parts[i] = sStageCurrent.Render("● " + name)
		default:
			parts[i] = sStagePending.Render("○ " + name)
		}
	}
	return strings.Join(parts, sMuted.Render("  ──  "))
}

func (m a4hModel) footer() string {
	hint := func(key, desc string) string { return sKeyName.Render(key) + " " + desc }
	var items []string
	if m.session != nil && !m.confirmEnd {
		state := m.session.GetState()
		switch {
		case a4h.CanStart(state):
			items = append(items, hint("s", "start"))
		case state == a4h.StateSAPRunning:
			items = append(items, hint("e", "extend +1h"), hint("x", "end session"))
		case a4h.IsStarting(state):
			items = append(items, hint("x", "cancel"))
		}
	}
	items = append(items, hint("r", "refresh"), hint("q", "quit"))
	return lipgloss.NewStyle().Render(strings.Join(items, sMuted.Render("  •  ")))
}
