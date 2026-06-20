// Package audit writes structured session logs to ~/.bai/audit/<date>/<session>.jsonl.
// Each line is a JSON event record. Files are created 0600; the directory is 0700.
// Entries older than retentionDays are pruned on session start.
package audit

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const retentionDays = 90

// Logger appends structured audit events to a per-session JSONL file.
type Logger struct {
	file      *os.File
	sessionID string
}

// NewLogger opens (or creates) the audit log file for sessionID.
// Returns a no-op logger and a non-nil error if the file cannot be opened.
func NewLogger(sessionID string) (*Logger, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return &Logger{}, err
	}
	dir := filepath.Join(home, ".bai", "audit", time.Now().Format("2006-01-02"))
	if err := os.MkdirAll(dir, 0700); err != nil {
		return &Logger{}, err
	}
	path := filepath.Join(dir, sessionID+".jsonl")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return &Logger{}, err
	}
	l := &Logger{file: f, sessionID: sessionID}
	pruneOld(home)
	return l, nil
}

// Close flushes and closes the underlying file.
func (l *Logger) Close() {
	if l.file != nil {
		_ = l.file.Close()
	}
}

// LogSessionStart records the beginning of a bai code session.
func (l *Logger) LogSessionStart(model, cwd, version string) {
	l.write(map[string]any{
		"event":   "session_start",
		"model":   model,
		"cwd":     cwd,
		"version": version,
	})
}

// LogSessionEnd records the end of a session.
func (l *Logger) LogSessionEnd(turns int, stopReason string) {
	l.write(map[string]any{
		"event":       "session_end",
		"turns":       turns,
		"stop_reason": stopReason,
	})
}

// LogToolCall records a tool invocation before execution.
func (l *Logger) LogToolCall(toolName, argsJSON string, approved bool, autoApproved bool) {
	l.write(map[string]any{
		"event":         "tool_call",
		"tool":          toolName,
		"input":         argsJSON,
		"approved":      approved,
		"auto_approved": autoApproved,
	})
}

// LogToolResult records the outcome of a tool execution.
func (l *Logger) LogToolResult(toolName string, durationMs int64, success bool) {
	l.write(map[string]any{
		"event":       "tool_result",
		"tool":        toolName,
		"duration_ms": durationMs,
		"success":     success,
	})
}

func (l *Logger) write(fields map[string]any) {
	if l.file == nil {
		return
	}
	fields["ts"] = time.Now().UTC().Format(time.RFC3339)
	fields["session_id"] = l.sessionID
	line, err := json.Marshal(fields)
	if err != nil {
		return
	}
	fmt.Fprintf(l.file, "%s\n", line)
}

// pruneOld removes audit directories older than retentionDays.
func pruneOld(home string) {
	auditRoot := filepath.Join(home, ".bai", "audit")
	entries, err := os.ReadDir(auditRoot)
	if err != nil {
		return
	}
	cutoff := time.Now().AddDate(0, 0, -retentionDays)
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		t, err := time.Parse("2006-01-02", e.Name())
		if err != nil {
			continue
		}
		if t.Before(cutoff) {
			_ = os.RemoveAll(filepath.Join(auditRoot, e.Name()))
		}
	}
}
