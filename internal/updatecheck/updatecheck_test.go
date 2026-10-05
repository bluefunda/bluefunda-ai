package updatecheck

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCheck_DevVersionShortCircuits(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	for _, v := range []string{"", "dev"} {
		latest, available := Check(v)
		if latest != "" || available {
			t.Errorf("Check(%q) = (%q, %v), want (\"\", false)", v, latest, available)
		}
	}
}

func TestNewer(t *testing.T) {
	cases := []struct {
		candidate, base string
		want            bool
	}{
		{"v1.5.0", "v1.4.2", true},
		{"1.5.0", "1.4.2", true},
		{"v1.4.2", "v1.4.2", false},
		{"v1.4.1", "v1.4.2", false},
		{"v2.0.0", "v1.99.99", true},
	}
	for _, c := range cases {
		if got := newer(c.candidate, c.base); got != c.want {
			t.Errorf("newer(%q, %q) = %v, want %v", c.candidate, c.base, got, c.want)
		}
	}
}

func TestSaveAndLoadCache_RoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "update-check.json")
	want := cacheState{LastChecked: time.Now().Truncate(time.Second), LatestVersion: "v1.5.0"}
	if err := saveCache(path, want); err != nil {
		t.Fatalf("saveCache: %v", err)
	}
	got := loadCache(path)
	if !got.LastChecked.Equal(want.LastChecked) || got.LatestVersion != want.LatestVersion {
		t.Errorf("loadCache() = %+v, want %+v", got, want)
	}
}

func TestLoadCache_MissingFileReturnsZeroValue(t *testing.T) {
	got := loadCache(filepath.Join(t.TempDir(), "does-not-exist.json"))
	if !got.LastChecked.IsZero() || got.LatestVersion != "" {
		t.Errorf("loadCache() on a missing file = %+v, want zero value", got)
	}
}

func TestLoadCache_CorruptFileReturnsZeroValue(t *testing.T) {
	path := filepath.Join(t.TempDir(), "update-check.json")
	if err := os.WriteFile(path, []byte("not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	got := loadCache(path)
	if !got.LastChecked.IsZero() || got.LatestVersion != "" {
		t.Errorf("loadCache() on a corrupt file = %+v, want zero value", got)
	}
}

func TestCheck_CacheHitAvoidsNetworkCall(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	path := filepath.Join(home, ".bai", "update-check.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	seed := cacheState{LastChecked: time.Now(), LatestVersion: "v9.9.9"}
	data, err := json.Marshal(seed)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}

	// No network stub is installed; if Check tried to hit the network despite
	// a fresh cache, this test would hang/fail against the real GitHub API in
	// CI. A fast, correct result here demonstrates the cache was used.
	latest, available := Check("v1.0.0")
	if latest != "v9.9.9" || !available {
		t.Errorf("Check() = (%q, %v), want (\"v9.9.9\", true) from the cache", latest, available)
	}
}

// MaybeNotify writes to the real os.Stderr, which can't be swapped for a
// fake TTY in a portable unit test (tips.ShouldSilence's stderr-TTY check
// would still report false even behind a redirected pipe, since pipes
// aren't char devices — same limitation noted in internal/tips's own test
// suite). What's deterministically testable instead: both gates below must
// short-circuit before any cache read or network call. HOME points at an
// empty dir with no cache file and no network stub, so a regression that
// skipped the gate would attempt a real GitHub call and fail/hang in CI.

func TestMaybeNotify_DisabledSkipsAllWork(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	MaybeNotify("v1.0.0", false, false)
}

// TestMaybeNotify_QuietSkipsAllWork covers the other deterministic gate:
// tips.ShouldSilence(quiet) returns true unconditionally when quiet is true,
// checked before its TTY-dependent checks — so this is testable regardless
// of whether the test runner's stderr is a terminal.
func TestMaybeNotify_QuietSkipsAllWork(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	MaybeNotify("v1.0.0", true, true)
}

func TestNotify_WritesExpectedLine(t *testing.T) {
	var buf bytes.Buffer
	notify(&buf, "1.4.2", "1.5.0")
	want := "bai 1.4.2 → 1.5.0 available · run `bai update`\n"
	if buf.String() != want {
		t.Errorf("notify() wrote %q, want %q", buf.String(), want)
	}
}
