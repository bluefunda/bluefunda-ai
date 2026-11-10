package cmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "github.com/bluefunda/bluerequests/api/proto/bff"
	"github.com/bluefunda/bluerequests/internal/ui"
)

func rowMap(rows [][]string) map[string]string {
	m := map[string]string{}
	for _, r := range rows {
		m[r[0]] = r[1]
	}
	return m
}

func TestSessionRowsForAReadySession(t *testing.T) {
	now := time.Date(2026, 10, 26, 12, 0, 0, 0, time.UTC)
	rows := rowMap(sessionRows(&pb.A4HSession{
		State: "sap_running", InstanceName: "a4h-x", PublicIp: "203.0.113.5",
		Deadline: "2026-10-26T14:41:00Z",
	}, now))

	for field, want := range map[string]string{
		"Status":         "Ready",
		"System":         "a4h-x",
		"Host":           "203.0.113.5",
		"SAP GUI":        "instance 00 (port 3200)",
		"Time remaining": "2h 41m",
		"Ends at":        "2026-10-26T14:41:00Z",
	} {
		if rows[field] != want {
			t.Errorf("%s = %q, want %q", field, rows[field], want)
		}
	}
	if !strings.Contains(rows["ADT (Eclipse)"], "http://203.0.113.5:50000") {
		t.Errorf("ADT = %q", rows["ADT (Eclipse)"])
	}
	if _, ok := rows["Note"]; ok {
		t.Error("a ready session must not say SAP is still starting")
	}
}

func TestSessionRowsWarnWhileSAPIsStarting(t *testing.T) {
	rows := rowMap(sessionRows(&pb.A4HSession{State: "waiting_for_sap", PublicIp: "203.0.113.5"}, time.Now()))

	if rows["Host"] != "203.0.113.5" || !strings.Contains(rows["Note"], "still starting") {
		t.Fatalf("rows = %v", rows)
	}
	if _, ok := rows["Time remaining"]; ok {
		t.Error("no countdown before the session is running")
	}
}

func TestSessionRowsForNoSessionAreJustTheStatus(t *testing.T) {
	rows := sessionRows(&pb.A4HSession{State: "none"}, time.Now())

	if len(rows) != 1 || rows[0][1] != "No active session" {
		t.Fatalf("rows = %v", rows)
	}
}

func TestPrintSessionInEachOutputFormat(t *testing.T) {
	sess := &pb.A4HSession{State: "sap_running", PublicIp: "203.0.113.5", Deadline: "2026-10-26T14:41:00Z"}
	for format, check := range map[ui.OutputFormat]func(string) bool{
		ui.FormatQuiet: func(s string) bool { return s == "sap_running\n" },
		// protojson randomizes its whitespace on purpose, so parse rather than compare text.
		ui.FormatJSON: func(s string) bool {
			var got map[string]string
			return json.Unmarshal([]byte(s), &got) == nil &&
				got["state"] == "sap_running" && got["publicIp"] == "203.0.113.5" && got["deadline"] == "2026-10-26T14:41:00Z"
		},
		ui.FormatTable: func(s string) bool { return strings.Contains(s, "FIELD") && strings.Contains(s, "203.0.113.5") },
	} {
		var out, errOut bytes.Buffer
		printSession(&ui.Printer{Out: &out, Err: &errOut, Format: format}, sess)

		if !check(out.String()) {
			t.Errorf("%s output = %q", format, out.String())
		}
		if errOut.Len() != 0 {
			t.Errorf("%s: data command wrote to stderr: %q", format, errOut.String())
		}
	}
}

func TestA4HErrorExplainsTheStatusCodes(t *testing.T) {
	if got := a4hError(status.Error(codes.NotFound, "no running session")); got.Error() != "no running session" {
		t.Errorf("NotFound -> %q", got)
	}
	got := a4hError(status.Error(codes.Unavailable, "session service is not responding"))
	if !strings.Contains(got.Error(), "try again in a moment") {
		t.Errorf("Unavailable -> %q", got)
	}
	other := status.Error(codes.PermissionDenied, "nope")
	if a4hError(other) != other {
		t.Error("other errors must pass through unchanged")
	}
}

func TestConfirmOnlyAcceptsYes(t *testing.T) {
	for answer, want := range map[string]bool{
		"y\n": true, "Y\n": true, "yes\n": true, " YES \n": true, "y": true,
		"n\n": false, "\n": false, "no\n": false, "yep\n": false, "maybe\n": false,
	} {
		var prompt bytes.Buffer
		got, err := confirm(strings.NewReader(answer), &prompt, "End it? ")
		if err != nil {
			t.Fatalf("%q: %v", answer, err)
		}
		if got != want {
			t.Errorf("answer %q = %v, want %v", answer, got, want)
		}
		if prompt.String() != "End it? " {
			t.Errorf("prompt = %q", prompt.String())
		}
	}
	if _, err := confirm(strings.NewReader(""), &bytes.Buffer{}, "?"); !errors.Is(err, errNoAnswer) {
		t.Errorf("end of input must not count as an answer, got %v", err)
	}
}

// waiterFor builds a waiter over a scripted sequence of states with a fake clock.
func waiterFor(states []string, getErr error) (*a4hWaiter, *[]time.Duration, *bytes.Buffer) {
	clock := time.Date(2026, 10, 26, 12, 0, 0, 0, time.UTC)
	var slept []time.Duration
	var info bytes.Buffer
	i := 0
	return &a4hWaiter{
		get: func() (*pb.A4HSession, error) {
			if getErr != nil {
				return nil, getErr
			}
			s := states[i]
			if i < len(states)-1 {
				i++
			}
			return &pb.A4HSession{State: s}, nil
		},
		sleep: func(d time.Duration) { slept = append(slept, d); clock = clock.Add(d) },
		now:   func() time.Time { return clock },
		p:     &ui.Printer{Out: &bytes.Buffer{}, Err: &info, Format: ui.FormatTable},
	}, &slept, &info
}

func TestWaitFollowsStartupUntilReady(t *testing.T) {
	w, slept, info := waiterFor([]string{"vm_running", "waiting_for_sap", "waiting_for_sap", "sap_running"}, nil)

	s, err := w.untilReady(&pb.A4HSession{State: "provisioning_vm"}, 15*time.Minute)

	if err != nil || s.State != "sap_running" {
		t.Fatalf("s = %v err = %v", s, err)
	}
	if len(*slept) != 4 || (*slept)[0] != 5*time.Second {
		t.Errorf("slept %v, want four 5s waits", *slept)
	}
	out := info.String()
	for _, want := range []string{"Creating your system", "System created", "Starting SAP", "Ready"} {
		if strings.Count(out, want) != 1 {
			t.Errorf("%q reported %d times, want once (only on change):\n%s", want, strings.Count(out, want), out)
		}
	}
}

func TestWaitFailsWhenTheSessionEndsInstead(t *testing.T) {
	w, _, _ := waiterFor([]string{"terminating"}, nil)

	_, err := w.untilReady(&pb.A4HSession{State: "waiting_for_sap"}, 15*time.Minute)

	if err == nil || !strings.Contains(err.Error(), "ended before SAP was ready") {
		t.Fatalf("err = %v", err)
	}
}

func TestWaitGivesUpAfterTheTimeout(t *testing.T) {
	w, slept, _ := waiterFor([]string{"waiting_for_sap"}, nil)

	_, err := w.untilReady(&pb.A4HSession{State: "waiting_for_sap"}, 20*time.Second)

	if err == nil || !strings.Contains(err.Error(), "not ready after 20s") {
		t.Fatalf("err = %v", err)
	}
	if len(*slept) != 4 {
		t.Errorf("slept %d times, want 4 (5s each up to 20s)", len(*slept))
	}
}

func TestWaitStopsOnAPollError(t *testing.T) {
	boom := errors.New("connection lost")
	w, _, _ := waiterFor(nil, boom)

	_, err := w.untilReady(&pb.A4HSession{State: "provisioning_vm"}, 15*time.Minute)

	if !errors.Is(err, boom) {
		t.Fatalf("err = %v", err)
	}
}
