package tips

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	tipcatalog "github.com/bluefunda/tipcatalog"
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
	s.ensureVectorSized()
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
	s.ensureVectorSized()
	s.InterestVector[0] = 100

	s.decay(start.Add(2 * halfLife))

	got := s.InterestVector[0]
	if got < 24.9 || got > 25.1 {
		t.Fatalf("expected ~25 after two half-lives, got %v", got)
	}
}

func TestDecay_NoOpOnFreshState(t *testing.T) {
	s := State{}
	s.ensureVectorSized()
	s.InterestVector[0] = 42
	s.decay(time.Now())
	if s.InterestVector[0] != 42 {
		t.Fatalf("expected no decay on zero LastDecay, got %v", s.InterestVector[0])
	}
}

func TestDecay_NoOpOnNonPositiveElapsed(t *testing.T) {
	at := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	s := State{LastDecay: at}
	s.ensureVectorSized()
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

	if err := Record(Invocation{Command: "login", ExitCode: 0}); err != nil {
		t.Fatalf("Record: %v", err)
	}
	okState, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	authIdx := topicIndex("auth")
	if authIdx < 0 {
		t.Fatal("expected 'auth' to be a known topic")
	}
	okWeight := okState.InterestVector[authIdx]

	withHome(t)
	if err := Record(Invocation{Command: "login", ExitCode: 1}); err != nil {
		t.Fatalf("Record: %v", err)
	}
	failState, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	failWeight := failState.InterestVector[authIdx]

	if failWeight != nonZeroExitWeight*okWeight {
		t.Fatalf("failWeight = %v, want %v", failWeight, nonZeroExitWeight*okWeight)
	}
}

func TestApply_MapsCommandAndFlagsToTopics(t *testing.T) {
	withHome(t)
	if err := Record(Invocation{Command: "bai mcp list", Flags: []string{"auto"}, ExitCode: 0}); err != nil {
		t.Fatalf("Record: %v", err)
	}
	state, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if state.InterestVector[topicIndex("mcp")] != 1 {
		t.Fatalf("expected 'mcp' topic incremented, vector = %v", state.InterestVector)
	}
	if state.InterestVector[topicIndex("automation")] != 1 {
		t.Fatalf("expected 'automation' topic incremented, vector = %v", state.InterestVector)
	}
}

func TestApply_NonZeroExitAlwaysCountsErrors(t *testing.T) {
	withHome(t)
	// "totally-unmapped-command" matches nothing in commandTopics, but a
	// failing exit must still count toward "errors" regardless.
	if err := Record(Invocation{Command: "totally-unmapped-command", ExitCode: 1}); err != nil {
		t.Fatalf("Record: %v", err)
	}
	state, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if state.InterestVector[topicIndex("errors")] != nonZeroExitWeight {
		t.Fatalf("expected 'errors' topic at weight %v, vector = %v", nonZeroExitWeight, state.InterestVector)
	}
}

func TestEnsureVectorSized_ResetsOnMismatch(t *testing.T) {
	s := State{InterestVector: []float64{1, 2, 3}} // wrong length vs. tipcatalog.Topics
	s.ensureVectorSized()
	if len(s.InterestVector) != len(tipcatalog.Topics) {
		t.Fatalf("len = %d, want %d", len(s.InterestVector), len(tipcatalog.Topics))
	}
	for i, v := range s.InterestVector {
		if v != 0 {
			t.Fatalf("expected a reset zero vector, got %v at %d", v, i)
		}
	}
}

func TestEnsureVectorSized_NoOpWhenAlreadyCorrect(t *testing.T) {
	want := make([]float64, len(tipcatalog.Topics))
	want[0] = 42
	s := State{InterestVector: want}
	s.ensureVectorSized()
	if s.InterestVector[0] != 42 {
		t.Fatal("expected ensureVectorSized to leave a correctly-sized vector untouched")
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
