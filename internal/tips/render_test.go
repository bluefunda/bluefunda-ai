package tips

import (
	"errors"
	"io"
	"os"
	"testing"
)

func TestMaybeShowTip_NeverWritesStdout(t *testing.T) {
	withHome(t)
	simulateInteractiveTerminal(t)
	clearSilenceEnv(t)

	fetchDone := make(chan struct{})
	orig := fetchLatestManifestFn
	fetchLatestManifestFn = func() ([]byte, []byte, error) {
		defer close(fetchDone)
		return nil, nil, errors.New("no network in tests")
	}
	t.Cleanup(func() { fetchLatestManifestFn = orig })

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	origStdout := os.Stdout
	os.Stdout = w

	MaybeShowTip(false)

	// Wait for EnsureFresh's background goroutine before restoring
	// os.Stdout, so it can't leak into (and race with) the next test.
	<-fetchDone
	_ = w.Close()
	os.Stdout = origStdout

	out, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if len(out) != 0 {
		t.Fatalf("expected nothing ever written to stdout by the tip path, got: %q", out)
	}
}

// BenchmarkMaybeShowTip measures the steady-state per-invocation cost of
// the tip hook — the acceptance target is <2ms added to a command like
// `bai --version`. The first call primes the 24h refresh marker so
// subsequent iterations reflect the common case (no fetch attempted).
func BenchmarkMaybeShowTip(b *testing.B) {
	home := b.TempDir()
	b.Setenv("HOME", home)
	b.Setenv("NO_COLOR", "")
	b.Setenv("CI", "")
	b.Setenv("GITHUB_ACTIONS", "")

	origTerm := stderrIsTerminal
	stderrIsTerminal = func() bool { return true }
	b.Cleanup(func() { stderrIsTerminal = origTerm })

	origFetch := fetchLatestManifestFn
	fetchLatestManifestFn = func() ([]byte, []byte, error) {
		return nil, nil, errors.New("no network in benchmarks")
	}
	b.Cleanup(func() { fetchLatestManifestFn = origFetch })

	MaybeShowTip(false) // primes the throttle marker

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		MaybeShowTip(false)
	}
}
