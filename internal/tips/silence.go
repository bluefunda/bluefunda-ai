package tips

import "os"

// ShouldSilence reports whether the tip path must print nothing. quiet
// should reflect the caller's own quiet/output-format flag (e.g. `-o
// quiet`); everything else here is detected directly from the process
// environment since none of it existed before this package (repo-wide,
// only stdout isatty was checked anywhere, in internal/cmd/code.go).
func ShouldSilence(quiet bool) bool {
	if quiet {
		return true
	}
	if os.Getenv("NO_COLOR") != "" {
		return true
	}
	if os.Getenv("CI") != "" || os.Getenv("GITHUB_ACTIONS") != "" {
		return true
	}
	if !stderrIsTerminal() {
		return true
	}
	return false
}

// stderrIsTerminal reports whether stderr is an interactive terminal
// (false when piped, redirected to a file, or otherwise non-tty). A var so
// tests can simulate an interactive terminal — test runners never attach a
// real tty to stderr, so the real check is always false under `go test`.
var stderrIsTerminal = func() bool {
	fi, err := os.Stderr.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}
