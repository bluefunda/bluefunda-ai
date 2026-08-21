package daemon

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/bluefunda/bluefunda-ai/internal/schedule"
)

func testRunner(t *testing.T, exec Executor, now time.Time) (*Runner, *schedule.Store) {
	t.Helper()
	store := schedule.New(filepath.Join(t.TempDir(), "schedules.yaml"))
	r := &Runner{
		Store:   store,
		Execute: exec,
		Now:     func() time.Time { return now },
	}
	return r, store
}

func TestTick_FiresDueEntry(t *testing.T) {
	now := time.Date(2026, 1, 5, 9, 0, 0, 0, time.UTC)
	var firedPrompt string
	exec := func(ctx context.Context, e schedule.Entry) (string, error) {
		firedPrompt = e.Prompt
		return "did the thing", nil
	}
	r, store := testRunner(t, exec, now)

	past := now.Add(-time.Minute)
	e, err := store.Create(schedule.Entry{Cron: "@daily", Prompt: "summarize CI", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	e.NextRun = &past
	if err := store.Update(e); err != nil {
		t.Fatal(err)
	}

	results, err := r.Tick(context.Background())
	if err != nil {
		t.Fatalf("Tick: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("Tick fired %d entries, want 1", len(results))
	}
	if firedPrompt != "summarize CI" {
		t.Errorf("Executor received prompt %q, want %q", firedPrompt, "summarize CI")
	}

	got, ok, err := store.Get(e.ID)
	if err != nil || !ok {
		t.Fatalf("Get after Tick: %v, ok=%v", err, ok)
	}
	if got.LastStatus != "ok" {
		t.Errorf("LastStatus = %q, want %q", got.LastStatus, "ok")
	}
	if got.LastRun == nil || !got.LastRun.Equal(now) {
		t.Errorf("LastRun = %v, want %v", got.LastRun, now)
	}
	if got.NextRun == nil || !got.NextRun.After(now) {
		t.Errorf("NextRun = %v, want a time after %v", got.NextRun, now)
	}
}

func TestTick_SkipsNotYetDueEntry(t *testing.T) {
	now := time.Date(2026, 1, 5, 9, 0, 0, 0, time.UTC)
	fired := false
	exec := func(ctx context.Context, e schedule.Entry) (string, error) {
		fired = true
		return "", nil
	}
	r, store := testRunner(t, exec, now)

	future := now.Add(time.Hour)
	e, err := store.Create(schedule.Entry{Cron: "@daily", Prompt: "x", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	e.NextRun = &future
	if err := store.Update(e); err != nil {
		t.Fatal(err)
	}

	results, err := r.Tick(context.Background())
	if err != nil {
		t.Fatalf("Tick: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("Tick fired %d entries, want 0 (not yet due)", len(results))
	}
	if fired {
		t.Error("Executor was called for a not-yet-due entry")
	}
}

func TestTick_SkipsDisabledEntry(t *testing.T) {
	now := time.Date(2026, 1, 5, 9, 0, 0, 0, time.UTC)
	fired := false
	exec := func(ctx context.Context, e schedule.Entry) (string, error) {
		fired = true
		return "", nil
	}
	r, store := testRunner(t, exec, now)

	past := now.Add(-time.Minute)
	e, err := store.Create(schedule.Entry{Cron: "@daily", Prompt: "x", Enabled: false})
	if err != nil {
		t.Fatal(err)
	}
	e.NextRun = &past
	if err := store.Update(e); err != nil {
		t.Fatal(err)
	}

	results, err := r.Tick(context.Background())
	if err != nil {
		t.Fatalf("Tick: %v", err)
	}
	if len(results) != 0 || fired {
		t.Error("Tick fired a disabled entry")
	}
}

func TestTick_RecordsExecutorError(t *testing.T) {
	now := time.Date(2026, 1, 5, 9, 0, 0, 0, time.UTC)
	exec := func(ctx context.Context, e schedule.Entry) (string, error) {
		return "partial output", errors.New("agent run failed")
	}
	r, store := testRunner(t, exec, now)

	past := now.Add(-time.Minute)
	e, err := store.Create(schedule.Entry{Cron: "@daily", Prompt: "x", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	e.NextRun = &past
	if err := store.Update(e); err != nil {
		t.Fatal(err)
	}

	results, err := r.Tick(context.Background())
	if err != nil {
		t.Fatalf("Tick: %v", err)
	}
	if len(results) != 1 || results[0].Err == nil {
		t.Fatalf("Tick results = %+v, want one result with an error", results)
	}

	got, _, _ := store.Get(e.ID)
	if got.LastStatus != "error" || got.LastError != "agent run failed" {
		t.Errorf("got = %+v, want LastStatus=error LastError=%q", got, "agent run failed")
	}
	// NextRun must still advance even on failure — a broken schedule
	// shouldn't fire on every tick forever.
	if got.NextRun == nil || !got.NextRun.After(now) {
		t.Errorf("NextRun = %v, want advanced past %v despite the error", got.NextRun, now)
	}
}

func TestTick_MultipleEntriesOnlyDueOnesFire(t *testing.T) {
	now := time.Date(2026, 1, 5, 9, 0, 0, 0, time.UTC)
	var firedIDs []string
	exec := func(ctx context.Context, e schedule.Entry) (string, error) {
		firedIDs = append(firedIDs, e.ID)
		return "", nil
	}
	r, store := testRunner(t, exec, now)

	past := now.Add(-time.Minute)
	future := now.Add(time.Hour)

	due, err := store.Create(schedule.Entry{Cron: "@daily", Prompt: "due", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	due.NextRun = &past
	if err := store.Update(due); err != nil {
		t.Fatal(err)
	}

	notDue, err := store.Create(schedule.Entry{Cron: "@daily", Prompt: "not due", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	notDue.NextRun = &future
	if err := store.Update(notDue); err != nil {
		t.Fatal(err)
	}

	results, err := r.Tick(context.Background())
	if err != nil {
		t.Fatalf("Tick: %v", err)
	}
	if len(results) != 1 || len(firedIDs) != 1 || firedIDs[0] != due.ID {
		t.Errorf("Tick fired %v, want only %q", firedIDs, due.ID)
	}
}

func TestTick_EmptyStoreNoOp(t *testing.T) {
	r, _ := testRunner(t, func(ctx context.Context, e schedule.Entry) (string, error) {
		t.Fatal("Executor should not be called on an empty store")
		return "", nil
	}, time.Now())

	results, err := r.Tick(context.Background())
	if err != nil {
		t.Fatalf("Tick: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("Tick() = %+v, want empty", results)
	}
}
