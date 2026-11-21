package checkpoint

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func write(t *testing.T, p, s string) {
	t.Helper()
	if err := os.WriteFile(p, []byte(s), 0o644); err != nil {
		t.Fatal(err)
	}
}

func read(t *testing.T, p string) string {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		return "<missing>"
	}
	return string(b)
}

// Three turns edit two files and create a third; restoring to each turn
// gives the files their state from before it.
func TestRestoreCode(t *testing.T) {
	dir := t.TempDir()
	a, b, c := filepath.Join(dir, "a"), filepath.Join(dir, "b"), filepath.Join(dir, "c")
	write(t, a, "a0")
	write(t, b, "b0")
	s := New()

	s.Begin("one", "one", 0)
	s.BeforeEdit(a)
	write(t, a, "a1")
	s.BeforeEdit(a) // second edit in the same turn keeps the first backup
	write(t, a, "a1b")

	s.Begin("two", "two", 2)
	s.BeforeEdit(b)
	write(t, b, "b2")
	s.BeforeEdit(c)
	write(t, c, "c2")

	s.Begin("three", "three", 4)
	s.BeforeEdit(a)
	write(t, a, "a3")

	pts := s.Points()
	if len(pts) != 3 || pts[0].Files != 3 || pts[1].Files != 3 || pts[2].Files != 1 {
		t.Fatalf("points = %+v", pts)
	}

	got, err := s.RestoreCode(2)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(got, []string{a, b, c}) {
		t.Errorf("restored %q", got)
	}
	if read(t, a) != "a1b" || read(t, b) != "b0" || read(t, c) != "<missing>" {
		t.Errorf("after restore to 2: a=%q b=%q c=%q", read(t, a), read(t, b), read(t, c))
	}

	// Backups from turn 2 on are gone; turn 1's still restores a.
	if pts := s.Points(); pts[1].Files != 0 || pts[0].Files != 1 {
		t.Errorf("points after restore = %+v", pts)
	}
	if _, err := s.RestoreCode(1); err != nil || read(t, a) != "a0" {
		t.Errorf("restore to 1: a=%q err=%v", read(t, a), err)
	}
}

func TestBeforeEditOutsideTurnAndNil(t *testing.T) {
	var nilStore *Store
	nilStore.BeforeEdit("x") // must not panic
	s := New()
	s.BeforeEdit(filepath.Join(t.TempDir(), "x")) // no turn yet: ignored
	if len(s.backups) != 0 {
		t.Error("saved outside a turn")
	}
}

func TestLimits(t *testing.T) {
	dir := t.TempDir()
	big := filepath.Join(dir, "big")
	if err := os.WriteFile(big, make([]byte, MaxFileBytes+1), 0o644); err != nil {
		t.Fatal(err)
	}
	s := New()
	s.Begin("x", "x", 0)
	s.BeforeEdit(big)
	if len(s.backups) != 0 || s.Skipped()[big] == "" {
		t.Errorf("a file over the limit should be skipped and reported: %v", s.Skipped())
	}
}

func TestDropPoints(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "a")
	write(t, a, "a0")
	s := New()
	s.Begin("one", "one", 0)
	s.Begin("two", "two", 2)
	s.BeforeEdit(a)
	write(t, a, "a2")

	s.DropPointsFrom(2)
	if pts := s.Points(); len(pts) != 1 || pts[0].Files != 1 {
		t.Errorf("points = %+v; turn 2's backup must still count for turn 1", pts)
	}
	if _, err := s.RestoreCode(1); err != nil || read(t, a) != "a0" {
		t.Errorf("a=%q err=%v", read(t, a), err)
	}
}
