package schedule

import (
	"path/filepath"
	"testing"
	"time"
)

func testStore(t *testing.T) *Store {
	t.Helper()
	return New(filepath.Join(t.TempDir(), "schedules.yaml"))
}

func TestParseCron_Valid(t *testing.T) {
	sched, err := ParseCron("0 9 * * 1-5")
	if err != nil {
		t.Fatalf("ParseCron: %v", err)
	}
	// A Monday at 08:00 should next fire at 09:00 the same day.
	from := time.Date(2026, 1, 5, 8, 0, 0, 0, time.UTC) // Monday
	next := sched.Next(from)
	want := time.Date(2026, 1, 5, 9, 0, 0, 0, time.UTC)
	if !next.Equal(want) {
		t.Errorf("Next(%v) = %v, want %v", from, next, want)
	}
}

func TestParseCron_Invalid(t *testing.T) {
	if _, err := ParseCron("not a cron expression"); err == nil {
		t.Error("expected an error for an invalid cron expression, got nil")
	}
}

func TestCreate_AssignsIDAndNextRun(t *testing.T) {
	s := testStore(t)
	e, err := s.Create(Entry{Cron: "0 9 * * 1-5", Prompt: "summarize CI failures", Dir: "/tmp/proj"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if e.ID == "" {
		t.Error("Create did not assign an ID")
	}
	if e.NextRun == nil {
		t.Error("Create did not compute NextRun")
	}
	if e.CreatedAt.IsZero() {
		t.Error("Create did not set CreatedAt")
	}
}

func TestCreate_InvalidCronRejected(t *testing.T) {
	s := testStore(t)
	if _, err := s.Create(Entry{Cron: "garbage", Prompt: "x"}); err == nil {
		t.Error("expected an error for an invalid cron expression, got nil")
	}
	entries, err := s.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("List() = %+v, want no entry persisted after a rejected Create", entries)
	}
}

func TestList_Empty(t *testing.T) {
	s := testStore(t)
	entries, err := s.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("List() = %+v, want empty", entries)
	}
}

func TestList_SortedByCreatedAt(t *testing.T) {
	s := testStore(t)
	older := Entry{Cron: "@daily", Prompt: "a", CreatedAt: time.Now().Add(-time.Hour)}
	newer := Entry{Cron: "@daily", Prompt: "b", CreatedAt: time.Now()}
	if _, err := s.Create(newer); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Create(older); err != nil {
		t.Fatal(err)
	}

	entries, err := s.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(entries) != 2 || entries[0].Prompt != "a" || entries[1].Prompt != "b" {
		t.Errorf("List() = %+v, want [a, b] sorted by CreatedAt", entries)
	}
}

func TestGet_Found(t *testing.T) {
	s := testStore(t)
	created, err := s.Create(Entry{Cron: "@daily", Prompt: "x"})
	if err != nil {
		t.Fatal(err)
	}
	got, ok, err := s.Get(created.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !ok || got.Prompt != "x" {
		t.Errorf("Get(%q) = %+v, %v", created.ID, got, ok)
	}
}

func TestGet_NotFound(t *testing.T) {
	s := testStore(t)
	_, ok, err := s.Get("nonexistent")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if ok {
		t.Error("Get(nonexistent) = true, want false")
	}
}

func TestUpdate_ReplacesMatchingEntry(t *testing.T) {
	s := testStore(t)
	e, err := s.Create(Entry{Cron: "@daily", Prompt: "x", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	e.LastStatus = "ok"
	if err := s.Update(e); err != nil {
		t.Fatalf("Update: %v", err)
	}
	got, ok, err := s.Get(e.ID)
	if err != nil || !ok {
		t.Fatalf("Get after Update: %v, ok=%v", err, ok)
	}
	if got.LastStatus != "ok" {
		t.Errorf("LastStatus = %q, want %q", got.LastStatus, "ok")
	}
}

func TestUpdate_NotFound(t *testing.T) {
	s := testStore(t)
	if err := s.Update(Entry{ID: "nonexistent"}); err == nil {
		t.Error("expected an error updating a nonexistent entry, got nil")
	}
}

func TestDelete_RemovesEntry(t *testing.T) {
	s := testStore(t)
	e, err := s.Create(Entry{Cron: "@daily", Prompt: "x"})
	if err != nil {
		t.Fatal(err)
	}
	ok, err := s.Delete(e.ID)
	if err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if !ok {
		t.Error("Delete returned false, want true")
	}
	entries, err := s.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Errorf("List() after Delete = %+v, want empty", entries)
	}
}

func TestDelete_NotFound(t *testing.T) {
	s := testStore(t)
	ok, err := s.Delete("nonexistent")
	if err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if ok {
		t.Error("Delete(nonexistent) returned true, want false")
	}
}

func TestList_MissingFileIsNotAnError(t *testing.T) {
	s := New(filepath.Join(t.TempDir(), "does-not-exist", "schedules.yaml"))
	entries, err := s.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if entries != nil {
		t.Errorf("List() = %+v, want nil for a missing file", entries)
	}
}
