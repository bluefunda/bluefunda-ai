package tips

import "testing"

// clearSilenceEnv unsets every env var ShouldSilence checks, so each test
// starts from a known "nothing set" baseline regardless of the ambient
// shell/CI this test itself runs under.
func clearSilenceEnv(t *testing.T) {
	t.Helper()
	t.Setenv("NO_COLOR", "")
	t.Setenv("CI", "")
	t.Setenv("GITHUB_ACTIONS", "")
}

// simulateInteractiveTerminal overrides the tty check so ShouldSilence's
// other conditions can be exercised in isolation — go test never attaches a
// real tty to stderr, so without this every case would trivially silence.
func simulateInteractiveTerminal(t *testing.T) {
	t.Helper()
	orig := stderrIsTerminal
	stderrIsTerminal = func() bool { return true }
	t.Cleanup(func() { stderrIsTerminal = orig })
}

func TestShouldSilence_QuietFlag(t *testing.T) {
	clearSilenceEnv(t)
	simulateInteractiveTerminal(t)
	if !ShouldSilence(true) {
		t.Fatal("expected silence when quiet is true")
	}
}

func TestShouldSilence_NoColor(t *testing.T) {
	clearSilenceEnv(t)
	simulateInteractiveTerminal(t)
	t.Setenv("NO_COLOR", "1")
	if !ShouldSilence(false) {
		t.Fatal("expected silence when NO_COLOR is set")
	}
}

func TestShouldSilence_CI(t *testing.T) {
	clearSilenceEnv(t)
	simulateInteractiveTerminal(t)
	t.Setenv("CI", "true")
	if !ShouldSilence(false) {
		t.Fatal("expected silence when CI is set")
	}
}

func TestShouldSilence_GithubActions(t *testing.T) {
	clearSilenceEnv(t)
	simulateInteractiveTerminal(t)
	t.Setenv("GITHUB_ACTIONS", "true")
	if !ShouldSilence(false) {
		t.Fatal("expected silence when GITHUB_ACTIONS is set")
	}
}

func TestShouldSilence_PipedStdoutStderr(t *testing.T) {
	clearSilenceEnv(t)
	// No simulateInteractiveTerminal here: this is the real, unmocked tty
	// check, and go test always pipes/redirects stderr — exactly the
	// "piped stdout+stderr" non-interactive condition this proves silent.
	if !ShouldSilence(false) {
		t.Fatal("expected silence when stderr is not an interactive terminal")
	}
}

func TestShouldSilence_NotSilencedWhenInteractiveAndClean(t *testing.T) {
	clearSilenceEnv(t)
	simulateInteractiveTerminal(t)
	if ShouldSilence(false) {
		t.Fatal("expected not silenced: interactive terminal, no quiet/CI/NO_COLOR")
	}
}
