package a4h

import (
	"testing"
	"time"
)

func TestStepsFollowTheStartupStates(t *testing.T) {
	for state, want := range map[string]int{
		StateProvisioningVM: 0,
		StateVMRunning:      1,
		StateWaitingForSAP:  2,
		StateSAPRunning:     len(Steps),
		StateNone:           -1,
		StateTerminating:    -1,
		StateTerminated:     -1,
		"something_new":     -1,
	} {
		if got := Step(state); got != want {
			t.Errorf("Step(%q) = %d, want %d", state, got, want)
		}
	}
}

func TestWhichStatesCanStartOrEnd(t *testing.T) {
	for _, tc := range []struct {
		state               string
		starting, active, s bool
	}{
		{StateNone, false, false, true},
		{StateProvisioningVM, true, true, false},
		{StateVMRunning, true, true, false},
		{StateWaitingForSAP, true, true, false},
		{StateSAPRunning, false, true, false},
		{StateTerminating, false, false, false},
		{StateTerminated, false, false, true},
	} {
		if IsStarting(tc.state) != tc.starting || IsActive(tc.state) != tc.active || CanStart(tc.state) != tc.s {
			t.Errorf("%s: starting=%v active=%v canStart=%v", tc.state, IsStarting(tc.state), IsActive(tc.state), CanStart(tc.state))
		}
	}
}

func TestPollsFastInFlightSlowlyWhenReadyNeverWhenIdle(t *testing.T) {
	for state, want := range map[string]time.Duration{
		StateProvisioningVM: 5 * time.Second,
		StateWaitingForSAP:  5 * time.Second,
		StateTerminating:    5 * time.Second,
		StateSAPRunning:     30 * time.Second,
		StateNone:           0,
		StateTerminated:     0,
		"":                  0,
	} {
		if got := PollInterval(state); got != want {
			t.Errorf("PollInterval(%q) = %v, want %v", state, got, want)
		}
	}
}

func TestEveryStateHasALabel(t *testing.T) {
	for _, s := range []string{StateNone, StateProvisioningVM, StateVMRunning, StateWaitingForSAP, StateSAPRunning, StateTerminating, StateTerminated} {
		if Label(s) == "Unknown" {
			t.Errorf("no label for %s", s)
		}
	}
	if Label("something_new") != "Unknown" {
		t.Error("unknown states must not pretend to have a label")
	}
}

func TestRemaining(t *testing.T) {
	now := time.Date(2026, 10, 26, 12, 0, 0, 0, time.UTC)
	for deadline, want := range map[string]string{
		"2026-10-26T14:41:00Z": "2h 41m",
		"2026-10-26T12:12:00Z": "12m",
		"2026-10-26T12:00:10Z": "1m",
		"2026-10-26T11:59:00Z": "ending soon",
		"2026-10-26T12:00:00Z": "ending soon",
		"":                     "ending soon",
		"garbage":              "ending soon",
	} {
		if got := Remaining(deadline, now); got != want {
			t.Errorf("Remaining(%q) = %q, want %q", deadline, got, want)
		}
	}
}

func TestConnectionFor(t *testing.T) {
	c := ConnectionFor("203.0.113.5")
	if c.Host != "203.0.113.5" || c.ADTHTTP != "http://203.0.113.5:50000" || c.ADTTLS != "https://203.0.113.5:50001" {
		t.Fatalf("connection = %+v", c)
	}
	if c.SAPGUI != "instance 00 (port 3200)" {
		t.Fatalf("SAP GUI = %q", c.SAPGUI)
	}
}
