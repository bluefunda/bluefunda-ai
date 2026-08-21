package cmd

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

func TestReadDaemonPID_MissingFile(t *testing.T) {
	pid := readDaemonPID(filepath.Join(t.TempDir(), "daemon.pid"))
	if pid != 0 {
		t.Errorf("readDaemonPID(missing) = %d, want 0", pid)
	}
}

func TestReadDaemonPID_LiveProcess(t *testing.T) {
	pidFile := filepath.Join(t.TempDir(), "daemon.pid")
	self := os.Getpid()
	if err := os.WriteFile(pidFile, []byte(strconv.Itoa(self)), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := readDaemonPID(pidFile); got != self {
		t.Errorf("readDaemonPID(live) = %d, want %d", got, self)
	}
}

func TestReadDaemonPID_GarbageContent(t *testing.T) {
	pidFile := filepath.Join(t.TempDir(), "daemon.pid")
	if err := os.WriteFile(pidFile, []byte("not-a-pid"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := readDaemonPID(pidFile); got != 0 {
		t.Errorf("readDaemonPID(garbage) = %d, want 0", got)
	}
}

func TestReadDaemonPID_StaleProcess(t *testing.T) {
	// A PID astronomically unlikely to be a live process on any real system.
	pidFile := filepath.Join(t.TempDir(), "daemon.pid")
	if err := os.WriteFile(pidFile, []byte("999999999"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := readDaemonPID(pidFile); got != 0 {
		t.Errorf("readDaemonPID(stale) = %d, want 0", got)
	}
}
