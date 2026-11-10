package tui

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "github.com/bluefunda/bluerequests/api/proto/bff"
)

// fakeA4HAPI answers every call with next and counts them.
type fakeA4HAPI struct {
	next                           *pb.A4HSession
	err                            error
	gets, starts, extends, endings int
}

func (f *fakeA4HAPI) reply() (*pb.A4HSessionResponse, error) {
	if f.err != nil {
		return nil, f.err
	}
	return &pb.A4HSessionResponse{Session: f.next}, nil
}

func (f *fakeA4HAPI) GetA4HSession(context.Context, *pb.GetA4HSessionRequest, ...grpc.CallOption) (*pb.A4HSessionResponse, error) {
	f.gets++
	return f.reply()
}
func (f *fakeA4HAPI) StartA4HSession(context.Context, *pb.StartA4HSessionRequest, ...grpc.CallOption) (*pb.A4HSessionResponse, error) {
	f.starts++
	return f.reply()
}
func (f *fakeA4HAPI) ExtendA4HSession(context.Context, *pb.ExtendA4HSessionRequest, ...grpc.CallOption) (*pb.A4HSessionResponse, error) {
	f.extends++
	return f.reply()
}
func (f *fakeA4HAPI) TerminateA4HSession(context.Context, *pb.TerminateA4HSessionRequest, ...grpc.CallOption) (*pb.A4HSessionResponse, error) {
	f.endings++
	return f.reply()
}

func session(state string) *pb.A4HSession { return &pb.A4HSession{State: state} }

func ready() *pb.A4HSession {
	return &pb.A4HSession{
		State:        "sap_running",
		InstanceName: "a4h-x",
		PublicIp:     "203.0.113.5",
		Deadline:     time.Now().Add(150 * time.Minute).UTC().Format(time.RFC3339),
	}
}

// loaded returns a model that has received sess from its first status call.
func loaded(t *testing.T, api *fakeA4HAPI, sess *pb.A4HSession) a4hModel {
	t.Helper()
	api.next = sess
	m := newA4HModel(api, 100, 30)
	reply := m.fetch()()
	next, _ := m.Update(reply)
	return next.(a4hModel)
}

// press sends a key and returns the model and the command it produced.
func press(m a4hModel, k string) (a4hModel, tea.Cmd) {
	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(k)})
	return next.(a4hModel), cmd
}

func TestOffersToStartWhenThereIsNoSession(t *testing.T) {
	m := loaded(t, &fakeA4HAPI{}, session("none"))

	view := m.View()

	if !strings.Contains(view, "No active session") || !strings.Contains(view, "start") {
		t.Fatalf("view = %q", view)
	}
	if strings.Contains(view, "end session") {
		t.Fatal("offers to end a session that does not exist")
	}
}

func TestStartingASessionCallsStartAndShowsProgress(t *testing.T) {
	api := &fakeA4HAPI{}
	m := loaded(t, api, session("none"))

	m, cmd := press(m, "s")
	if cmd == nil || !m.busy {
		t.Fatal("start did not run")
	}
	api.next = session("provisioning_vm")
	next, poll := m.Update(cmd())
	m = next.(a4hModel)

	if api.starts != 1 {
		t.Fatalf("StartA4HSession called %d times", api.starts)
	}
	view := m.View()
	for _, want := range []string{"Creating your system", "Create system", "System running", "Start SAP", "Ready"} {
		if !strings.Contains(view, want) {
			t.Errorf("view lacks %q:\n%s", want, view)
		}
	}
	if poll == nil {
		t.Fatal("no poll scheduled while the system is starting")
	}
}

func TestShowsConnectionDetailsAndRemainingTimeWhenReady(t *testing.T) {
	m := loaded(t, &fakeA4HAPI{}, ready())

	view := m.View()

	for _, want := range []string{"203.0.113.5", "http://203.0.113.5:50000", "https://203.0.113.5:50001", "instance 00 (port 3200)", "2h 30m", "extend", "end session"} {
		if !strings.Contains(view, want) {
			t.Errorf("view lacks %q:\n%s", want, view)
		}
	}
	if strings.Contains(view, "still starting") {
		t.Fatal("says SAP is still starting when it is ready")
	}
}

func TestWarnsThatSAPIsStillStartingWhenOnlyTheAddressIsKnown(t *testing.T) {
	sess := &pb.A4HSession{State: "waiting_for_sap", PublicIp: "203.0.113.5"}
	m := loaded(t, &fakeA4HAPI{}, sess)

	view := m.View()

	if !strings.Contains(view, "203.0.113.5") || !strings.Contains(view, "still starting") {
		t.Fatalf("view = %q", view)
	}
}

func TestEndingASessionNeedsConfirmation(t *testing.T) {
	api := &fakeA4HAPI{}
	m := loaded(t, api, ready())

	m, cmd := press(m, "x")
	if cmd != nil || !m.confirmEnd {
		t.Fatal("x must only ask for confirmation")
	}
	if !strings.Contains(m.View(), "It will be deleted") {
		t.Fatalf("no warning shown:\n%s", m.View())
	}

	m, cmd = press(m, "n")
	if cmd != nil || m.confirmEnd || api.endings != 0 {
		t.Fatal("declining must not end the session")
	}

	m, _ = press(m, "x")
	m, cmd = press(m, "y")
	if cmd == nil {
		t.Fatal("confirming did not end the session")
	}
	api.next = session("terminating")
	cmd()
	if api.endings != 1 {
		t.Fatalf("TerminateA4HSession called %d times", api.endings)
	}
}

func TestOnlyOffersActionsThatApplyToTheState(t *testing.T) {
	for _, tc := range []struct {
		state         string
		start, extend bool
		end           bool
	}{
		{"none", true, false, false},
		{"terminated", true, false, false},
		{"provisioning_vm", false, false, true},
		{"waiting_for_sap", false, false, true},
		{"sap_running", false, true, true},
		{"terminating", false, false, false},
	} {
		m := loaded(t, &fakeA4HAPI{}, session(tc.state))
		if _, cmd := press(m, "s"); (cmd != nil) != tc.start {
			t.Errorf("%s: start available = %v, want %v", tc.state, cmd != nil, tc.start)
		}
		if _, cmd := press(m, "e"); (cmd != nil) != tc.extend {
			t.Errorf("%s: extend available = %v, want %v", tc.state, cmd != nil, tc.extend)
		}
		if mm, _ := press(m, "x"); mm.confirmEnd != tc.end {
			t.Errorf("%s: end available = %v, want %v", tc.state, mm.confirmEnd, tc.end)
		}
	}
}

func TestPollsWhileInFlightAndStopsWhenIdle(t *testing.T) {
	api := &fakeA4HAPI{next: session("provisioning_vm")}
	m := newA4HModel(api, 100, 30)
	_, poll := m.Update(m.fetch()())
	if poll == nil {
		t.Fatal("no poll while provisioning")
	}

	api.next = session("none")
	_, poll = m.Update(m.fetch()())
	if poll != nil {
		t.Fatal("polling an idle session")
	}
}

func TestOnlyTheNewestScheduledPollCounts(t *testing.T) {
	m := loaded(t, &fakeA4HAPI{}, session("provisioning_vm"))
	stale := a4hPollMsg{seq: m.pollSeq - 1}
	current := a4hPollMsg{seq: m.pollSeq}

	if _, cmd := m.Update(stale); cmd != nil {
		t.Fatal("a stale poll triggered a call, so keypresses would speed polling up")
	}
	if next, cmd := m.Update(current); cmd == nil || !next.(a4hModel).busy {
		t.Fatal("the current poll did not fetch")
	}
}

func TestKeepsPollingAfterAFailedPoll(t *testing.T) {
	api := &fakeA4HAPI{}
	m := loaded(t, api, session("waiting_for_sap"))

	api.err = status.Error(codes.Unavailable, "no responders")
	next, poll := m.Update(m.fetch()())
	m = next.(a4hModel)

	if poll == nil {
		t.Fatal("polling stopped after one failed poll")
	}
	if !strings.Contains(m.View(), "not responding") || !strings.Contains(m.View(), "Starting SAP") {
		t.Fatalf("expected the error and the last known state:\n%s", m.View())
	}
}

func TestExpiredSignInTellsTheUserToLogInAgain(t *testing.T) {
	m := newA4HModel(&fakeA4HAPI{err: status.Error(codes.Unauthenticated, "unauthorized")}, 100, 30)

	next, _ := m.Update(m.fetch()())

	if !strings.Contains(next.(a4hModel).View(), "breq login") {
		t.Fatalf("view = %q", next.(a4hModel).View())
	}
}

func TestShowsLoadErrorsInsteadOfLoadingForever(t *testing.T) {
	m := newA4HModel(&fakeA4HAPI{err: errors.New("boom")}, 100, 30)

	next, poll := m.Update(m.fetch()())

	if !strings.Contains(next.(a4hModel).View(), "Request failed") {
		t.Fatalf("view = %q", next.(a4hModel).View())
	}
	if poll != nil {
		t.Fatal("polling with no session to poll")
	}
}

func TestQuitKeys(t *testing.T) {
	m := loaded(t, &fakeA4HAPI{}, ready())
	for _, k := range []string{"q"} {
		if _, cmd := press(m, k); cmd == nil || cmd() != tea.Quit() {
			t.Errorf("%s did not quit", k)
		}
	}
	if _, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC}); cmd == nil || cmd() != tea.Quit() {
		t.Error("ctrl+c did not quit")
	}
}
