package tips

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

// withHome points $HOME (and thus statePath) at a temp dir for the duration
// of the test.
func withHome(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	return dir
}

func TestDecay_HalvesAtOneHalfLife(t *testing.T) {
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	s := State{LastDecay: start}
	s.InterestVector[0] = 100

	s.decay(start.Add(halfLife))

	got := s.InterestVector[0]
	if got < 49.9 || got > 50.1 {
		t.Fatalf("expected ~50 after one half-life, got %v", got)
	}
}

func TestDecay_QuartersAtTwoHalfLives(t *testing.T) {
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	s := State{LastDecay: start}
	s.InterestVector[0] = 100

	s.decay(start.Add(2 * halfLife))

	got := s.InterestVector[0]
	if got < 24.9 || got > 25.1 {
		t.Fatalf("expected ~25 after two half-lives, got %v", got)
	}
}

func TestDecay_NoOpOnFreshState(t *testing.T) {
	s := State{}
	s.InterestVector[0] = 42
	s.decay(time.Now())
	if s.InterestVector[0] != 42 {
		t.Fatalf("expected no decay on zero LastDecay, got %v", s.InterestVector[0])
	}
}

func TestDecay_NoOpOnNonPositiveElapsed(t *testing.T) {
	at := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	s := State{LastDecay: at}
	s.InterestVector[0] = 42
	s.decay(at) // elapsed == 0
	if s.InterestVector[0] != 42 {
		t.Fatalf("expected no decay when elapsed <= 0, got %v", s.InterestVector[0])
	}
}

func TestRecord_CapsHistoryAt500(t *testing.T) {
	withHome(t)

	for i := 0; i < 510; i++ {
		if err := Record(Invocation{Command: "code", DurationBucket: "<1s"}); err != nil {
			t.Fatalf("Record: %v", err)
		}
	}

	state, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(state.History) != maxHistory {
		t.Fatalf("history length = %d, want %d", len(state.History), maxHistory)
	}
}

func TestRecord_NonZeroExitWeightsMore(t *testing.T) {
	withHome(t)

	if err := Record(Invocation{Command: "ok-cmd", ExitCode: 0}); err != nil {
		t.Fatalf("Record: %v", err)
	}
	okState, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	okWeight := okState.InterestVector[bucket("cmd:ok-cmd")]

	withHome(t)
	if err := Record(Invocation{Command: "fail-cmd", ExitCode: 1}); err != nil {
		t.Fatalf("Record: %v", err)
	}
	failState, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	failWeight := failState.InterestVector[bucket("cmd:fail-cmd")]

	if failWeight != nonZeroExitWeight*okWeight {
		t.Fatalf("failWeight = %v, want %v", failWeight, nonZeroExitWeight*okWeight)
	}
}

func TestRecord_ConcurrentWritesNoCorruption(t *testing.T) {
	withHome(t)

	const n = 20
	var wg sync.WaitGroup
	wg.Add(n)
	errs := make(chan error, n)
	for i := 0; i < n; i++ {
		go func(i int) {
			defer wg.Done()
			errs <- Record(Invocation{Command: "concurrent", ExitCode: 0})
		}(i)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("Record: %v", err)
		}
	}

	state, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(state.History) != n {
		t.Fatalf("history length = %d, want %d (lost writes = corruption)", len(state.History), n)
	}

	// Confirm the persisted file is valid, uncorrupted JSON.
	path, err := statePath()
	if err != nil {
		t.Fatalf("statePath: %v", err)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read state file: %v", err)
	}
	var raw map[string]any
	if err := json.Unmarshal(b, &raw); err != nil {
		t.Fatalf("state file is not valid JSON: %v", err)
	}
}

func TestFingerprintRepo(t *testing.T) {
	if FingerprintRepo("") != "" {
		t.Fatal("expected empty fingerprint for empty remote URL")
	}
	a := FingerprintRepo("git@github.com:bluefunda/bluefunda-ai.git")
	b := FingerprintRepo("git@github.com:bluefunda/bluefunda-ai.git")
	c := FingerprintRepo("git@github.com:bluefunda/other.git")
	if a != b {
		t.Fatal("expected stable fingerprint for the same remote URL")
	}
	if a == c {
		t.Fatal("expected different fingerprints for different remote URLs")
	}
	if len(a) != 16 {
		t.Fatalf("expected 16-char hex fingerprint, got %d chars", len(a))
	}
}

func TestDurationBucket(t *testing.T) {
	cases := []struct {
		d    time.Duration
		want string
	}{
		{500 * time.Millisecond, "<1s"},
		{2 * time.Second, "1-5s"},
		{10 * time.Second, "5-30s"},
		{time.Minute, ">30s"},
	}
	for _, c := range cases {
		if got := DurationBucket(c.d); got != c.want {
			t.Errorf("DurationBucket(%v) = %q, want %q", c.d, got, c.want)
		}
	}
}

func TestRecord_AtomicWriteLeavesNoTempFiles(t *testing.T) {
	dir := withHome(t)

	if err := Record(Invocation{Command: "code"}); err != nil {
		t.Fatalf("Record: %v", err)
	}

	entries, err := os.ReadDir(filepath.Join(dir, ".bai", "tips"))
	if err != nil {
		t.Fatalf("read tips dir: %v", err)
	}
	for _, e := range entries {
		if e.Name() != "signal.json" && e.Name() != "signal.json.lock" {
			t.Fatalf("unexpected leftover file: %s", e.Name())
		}
	}
}
