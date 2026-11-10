// Package a4h holds what the `breq a4h` commands and the watch screen share:
// the session states reported by the A4H session API, the startup steps, how
// often to poll, and how to present a session.
package a4h

import (
	"fmt"
	"time"
)

// Session states, as reported by trm-bff (A4HSession.state).
const (
	StateNone           = "none"
	StateProvisioningVM = "provisioning_vm"
	StateVMRunning      = "vm_running"
	StateWaitingForSAP  = "waiting_for_sap"
	StateSAPRunning     = "sap_running"
	StateTerminating    = "terminating"
	StateTerminated     = "terminated"
)

// Steps are the startup stages shown to the user, in order. StateSAPRunning
// completes all of them.
var Steps = []string{"Create system", "System running", "Start SAP", "Ready"}

var starting = []string{StateProvisioningVM, StateVMRunning, StateWaitingForSAP}

var labels = map[string]string{
	StateNone:           "No active session",
	StateProvisioningVM: "Creating your system…",
	StateVMRunning:      "System created, waiting for it to boot…",
	StateWaitingForSAP:  "Starting SAP…",
	StateSAPRunning:     "Ready",
	StateTerminating:    "Shutting down…",
	StateTerminated:     "Session ended",
}

// Label is the human-readable form of a state.
func Label(state string) string {
	if l, ok := labels[state]; ok {
		return l
	}
	return "Unknown"
}

// IsStarting reports whether the system is still coming up.
func IsStarting(state string) bool {
	for _, s := range starting {
		if s == state {
			return true
		}
	}
	return false
}

// IsActive reports whether a system exists behind the session, so it can be
// ended.
func IsActive(state string) bool {
	return IsStarting(state) || state == StateSAPRunning
}

// CanStart reports whether a new session can be started.
func CanStart(state string) bool {
	return state == StateNone || state == StateTerminated
}

// Step is the index into Steps of the stage in progress, len(Steps) once ready,
// or -1 when there is no startup to show.
func Step(state string) int {
	for i, s := range starting {
		if s == state {
			return i
		}
	}
	if state == StateSAPRunning {
		return len(Steps)
	}
	return -1
}

// PollInterval is how long to wait before asking again; 0 means don't.
func PollInterval(state string) time.Duration {
	switch {
	case IsStarting(state) || state == StateTerminating:
		return 5 * time.Second
	case state == StateSAPRunning:
		return 30 * time.Second
	default:
		return 0
	}
}

// Remaining formats the time left until the RFC 3339 deadline: "2h 41m",
// "12m", or "ending soon" once it has passed or can't be read.
func Remaining(deadline string, now time.Time) string {
	d, err := time.Parse(time.RFC3339, deadline)
	if err != nil || !d.After(now) {
		return "ending soon"
	}
	minutes := int((d.Sub(now) + time.Minute - 1) / time.Minute)
	if h := minutes / 60; h > 0 {
		return fmt.Sprintf("%dh %dm", h, minutes%60)
	}
	return fmt.Sprintf("%dm", minutes)
}

// Connection describes how to reach a running system.
type Connection struct {
	Host    string
	SAPGUI  string
	ADTHTTP string
	ADTTLS  string
}

// ConnectionFor returns the connection details for host (the session's public
// IP): SAP GUI on instance 00 (port 3200) and ABAP Development Tools over HTTP
// (50000) or HTTPS (50001).
func ConnectionFor(host string) Connection {
	return Connection{
		Host:    host,
		SAPGUI:  "instance 00 (port 3200)",
		ADTHTTP: fmt.Sprintf("http://%s:50000", host),
		ADTTLS:  fmt.Sprintf("https://%s:50001", host),
	}
}
