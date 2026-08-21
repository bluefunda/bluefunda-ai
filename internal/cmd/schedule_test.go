package cmd

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bluefunda/bluefunda-ai/internal/schedule"
	"github.com/bluefunda/bluefunda-ai/internal/ui"
)

func testScheduleStore(t *testing.T) *schedule.Store {
	t.Helper()
	return schedule.New(filepath.Join(t.TempDir(), "schedules.yaml"))
}

func TestCreateSchedule_Success(t *testing.T) {
	store := testScheduleStore(t)
	p, buf := testPrinter(ui.FormatTable)

	err := createSchedule(store, p, schedule.Entry{Cron: "0 9 * * 1-5", Prompt: "summarize CI", Enabled: true})
	if err != nil {
		t.Fatalf("createSchedule: %v", err)
	}
	if !strings.Contains(buf.String(), "Created schedule") {
		t.Errorf("expected success message, got: %s", buf.String())
	}

	entries, err := store.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Prompt != "summarize CI" {
		t.Errorf("store.List() = %+v", entries)
	}
}

func TestCreateSchedule_InvalidCron(t *testing.T) {
	store := testScheduleStore(t)
	p, _ := testPrinter(ui.FormatTable)

	err := createSchedule(store, p, schedule.Entry{Cron: "garbage", Prompt: "x"})
	if err == nil {
		t.Error("expected an error for an invalid cron expression, got nil")
	}
}

func TestListSchedules_Empty(t *testing.T) {
	store := testScheduleStore(t)
	p, buf := testPrinter(ui.FormatTable)

	if err := listSchedules(store, p); err != nil {
		t.Fatalf("listSchedules: %v", err)
	}
	if !strings.Contains(buf.String(), "no scheduled runs") {
		t.Errorf("expected empty-state message, got: %s", buf.String())
	}
}

func TestListSchedules_Table(t *testing.T) {
	store := testScheduleStore(t)
	if _, err := store.Create(schedule.Entry{Name: "ci-summary", Cron: "@daily", Prompt: "x", Enabled: true}); err != nil {
		t.Fatal(err)
	}
	p, buf := testPrinter(ui.FormatTable)

	if err := listSchedules(store, p); err != nil {
		t.Fatalf("listSchedules: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "ci-summary") || !strings.Contains(out, "@daily") {
		t.Errorf("expected name and cron in table output, got: %s", out)
	}
}

func TestShowSchedule_Found(t *testing.T) {
	store := testScheduleStore(t)
	e, err := store.Create(schedule.Entry{Name: "ci-summary", Cron: "@daily", Prompt: "do the thing", Dir: "/tmp/proj", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer

	if err := showSchedule(store, &buf, e.ID); err != nil {
		t.Fatalf("showSchedule: %v", err)
	}
	out := buf.String()
	for _, want := range []string{e.ID, "ci-summary", "@daily", "/tmp/proj", "do the thing"} {
		if !strings.Contains(out, want) {
			t.Errorf("showSchedule output missing %q:\n%s", want, out)
		}
	}
}

func TestShowSchedule_NotFound(t *testing.T) {
	store := testScheduleStore(t)
	var buf bytes.Buffer
	if err := showSchedule(store, &buf, "nonexistent"); err == nil {
		t.Error("expected an error for a nonexistent id, got nil")
	}
}

func TestDeleteSchedule_Success(t *testing.T) {
	store := testScheduleStore(t)
	e, err := store.Create(schedule.Entry{Cron: "@daily", Prompt: "x"})
	if err != nil {
		t.Fatal(err)
	}
	p, buf := testPrinter(ui.FormatTable)

	if err := deleteSchedule(store, p, e.ID); err != nil {
		t.Fatalf("deleteSchedule: %v", err)
	}
	if !strings.Contains(buf.String(), "Deleted") {
		t.Errorf("expected success message, got: %s", buf.String())
	}
	entries, _ := store.List()
	if len(entries) != 0 {
		t.Errorf("store.List() after delete = %+v, want empty", entries)
	}
}

func TestDeleteSchedule_NotFound(t *testing.T) {
	store := testScheduleStore(t)
	p, _ := testPrinter(ui.FormatTable)
	if err := deleteSchedule(store, p, "nonexistent"); err == nil {
		t.Error("expected an error for a nonexistent id, got nil")
	}
}

func TestFormatScheduleTime_Nil(t *testing.T) {
	if got := formatScheduleTime(nil); got != "-" {
		t.Errorf("formatScheduleTime(nil) = %q, want %q", got, "-")
	}
}
