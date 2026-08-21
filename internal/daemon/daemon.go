// Package daemon implements the polling loop behind `bai daemon start`: on
// each tick, fire any enabled schedule.Entry whose NextRun has passed. The
// tick logic here is process-lifecycle-agnostic (no signals, no PID files —
// that's internal/cmd/daemon.go) so it's plain, fast to test.
package daemon

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/bluefunda/bluefunda-ai/internal/schedule"
)

// Executor runs one scheduled entry to completion and returns a transcript
// of what happened (for the log file), or an error if the run failed.
type Executor func(ctx context.Context, e schedule.Entry) (transcript string, err error)

// FireResult is one entry's outcome from a Tick.
type FireResult struct {
	Entry Entry
	Err   error
}

// Entry is re-exported for callers that only import daemon.
type Entry = schedule.Entry

// Runner holds everything one Tick needs. Store and Execute are required;
// LogDir and Now default to ~/.bai/daemon/logs and time.Now if unset.
type Runner struct {
	Store   *schedule.Store
	Execute Executor
	LogDir  string
	Now     func() time.Time
}

func (r *Runner) now() time.Time {
	if r.Now != nil {
		return r.Now()
	}
	return time.Now()
}

// Tick checks every enabled schedule and fires the ones whose NextRun has
// passed, one at a time (sequential — Executor implementations built on
// sdk/agent change the process's working directory per run, so concurrent
// execution would race). Returns the entries that were fired.
func (r *Runner) Tick(ctx context.Context) ([]FireResult, error) {
	now := r.now()
	entries, err := r.Store.List()
	if err != nil {
		return nil, fmt.Errorf("list schedules: %w", err)
	}

	var results []FireResult
	for _, e := range entries {
		if !e.Enabled || e.NextRun == nil || now.Before(*e.NextRun) {
			continue
		}

		sched, cronErr := schedule.ParseCron(e.Cron)
		if cronErr != nil {
			// A schedule that was valid at creation but somehow isn't
			// anymore (hand-edited file) — record the error and leave
			// NextRun alone so it surfaces on every tick until fixed.
			e.LastStatus = "error"
			e.LastError = cronErr.Error()
			_ = r.Store.Update(e)
			results = append(results, FireResult{Entry: e, Err: cronErr})
			continue
		}

		transcript, runErr := r.Execute(ctx, e)
		fired := now
		e.LastRun = &fired
		if runErr != nil {
			e.LastStatus = "error"
			e.LastError = runErr.Error()
		} else {
			e.LastStatus = "ok"
			e.LastError = ""
		}
		next := sched.Next(now)
		e.NextRun = &next

		if err := r.Store.Update(e); err != nil {
			return results, fmt.Errorf("update schedule %s after firing: %w", e.ID, err)
		}
		if r.LogDir != "" {
			_ = writeLog(r.LogDir, e, transcript, runErr)
		}
		results = append(results, FireResult{Entry: e, Err: runErr})
	}
	return results, nil
}

// writeLog appends one run's transcript to
// <logDir>/<entry-id>/<RFC3339-ish timestamp>.log.
func writeLog(logDir string, e Entry, transcript string, runErr error) error {
	dir := filepath.Join(logDir, e.ID)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	ts := time.Now().UTC().Format("20060102T150405Z")
	path := filepath.Join(dir, ts+".log")

	var sb strings.Builder
	fmt.Fprintf(&sb, "schedule: %s\nprompt: %s\ndir: %s\n\n", e.ID, e.Prompt, e.Dir)
	sb.WriteString(transcript)
	if runErr != nil {
		fmt.Fprintf(&sb, "\n\n[error] %v\n", runErr)
	}
	return os.WriteFile(path, []byte(sb.String()), 0o600)
}
