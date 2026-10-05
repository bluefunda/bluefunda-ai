// Package updatecheck implements a passive, 24h-cached background check for
// newer bai releases (#287): a one-line notice, never a prompt. The two
// surfaces (the TUI footer in internal/ui/tui and the non-interactive stderr
// line wired in internal/cmd/root.go) both call into this package so the
// fetch/cache/compare logic exists in exactly one place.
package updatecheck

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/bluefunda/bluefunda-ai/internal/tips"
)

const (
	githubOwner   = "bluefunda"
	githubRepo    = "bluefunda-ai"
	checkInterval = 24 * time.Hour
	fetchTimeout  = 5 * time.Second
)

// cacheState is persisted at ~/.bai/update-check.json.
type cacheState struct {
	LastChecked   time.Time `json:"last_checked"`
	LatestVersion string    `json:"latest_version"`
}

// Check returns the latest known release tag and whether it's newer than
// current, consulting a 24h cache before hitting GitHub. current == "" or
// "dev" always reports unavailable — dev builds have no sensible baseline to
// compare against. A network failure still bumps the cache's timestamp (so a
// persistently offline or rate-limited machine backs off for 24h instead of
// retrying on every invocation) and falls back to whatever was last cached.
func Check(current string) (latest string, available bool) {
	if current == "" || current == "dev" {
		return "", false
	}

	path, err := cachePath()
	if err != nil {
		// No home dir resolvable — nothing to cache against; check once,
		// fail silent, don't persist.
		tag, fetchErr := fetchLatestRelease(githubOwner, githubRepo)
		if fetchErr != nil || tag == "" {
			return "", false
		}
		return tag, newer(tag, current)
	}

	state := loadCache(path)
	if time.Since(state.LastChecked) >= checkInterval {
		tag, fetchErr := fetchLatestRelease(githubOwner, githubRepo)
		state.LastChecked = time.Now()
		if fetchErr == nil && tag != "" {
			state.LatestVersion = tag
		}
		_ = saveCache(path, state) // best-effort; a failed write just means we check again next time
	}

	if state.LatestVersion == "" {
		return "", false
	}
	return state.LatestVersion, newer(state.LatestVersion, current)
}

// MaybeNotify is the one-call orchestrator for the non-interactive stderr
// surface, mirroring tips.MaybeShowTip(quiet)'s calling convention. enabled
// is the resolved `update.check` config value — checked first so a disabled
// config does zero work, not even a cache read. quiet and the rest of
// tips.ShouldSilence's checks (CI, NO_COLOR, non-TTY stderr) are reused
// directly rather than re-implemented a second time in this package.
func MaybeNotify(current string, quiet bool, enabled bool) {
	if !enabled || tips.ShouldSilence(quiet) {
		return
	}
	latest, available := Check(current)
	if !available {
		return
	}
	notify(os.Stderr, current, latest)
}

func notify(w io.Writer, current, latest string) {
	fmt.Fprintf(w, "bai %s → %s available · run `bai update`\n", current, latest)
}

func cachePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, ".bai")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	return filepath.Join(dir, "update-check.json"), nil
}

func loadCache(path string) cacheState {
	data, err := os.ReadFile(path)
	if err != nil {
		return cacheState{}
	}
	var state cacheState
	if json.Unmarshal(data, &state) != nil {
		return cacheState{}
	}
	return state
}

// saveCache writes state to path via a temp-file-then-rename so a concurrent
// reader never observes a partially-written file. No cross-process locking:
// a same-second race between two bai invocations is "last writer wins," an
// acceptable outcome for a cache that's wrong for at most 24h either way.
func saveCache(path string, state cacheState) error {
	data, err := json.Marshal(state)
	if err != nil {
		return err
	}
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".bai-update-check-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return err
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return err
	}
	return os.Rename(tmpName, path)
}

type githubReleasePayload struct {
	TagName string `json:"tag_name"`
}

// fetchLatestRelease hits the GitHub releases API. A non-200 response or any
// transport error returns ("", err-or-nil) — callers treat both the same way
// (no update info available this round), matching the issue's "a network
// failure or GitHub rate limit is silent" requirement.
func fetchLatestRelease(owner, repo string) (string, error) {
	client := &http.Client{Timeout: fetchTimeout}
	url := "https://api.github.com/repos/" + owner + "/" + repo + "/releases/latest"
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "bai/updatecheck (+github.com/bluefunda/bluefunda-ai)")
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close() //nolint:errcheck
	if resp.StatusCode != http.StatusOK {
		return "", nil
	}
	var rel githubReleasePayload
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return "", err
	}
	return rel.TagName, nil
}

// newer reports whether candidate is strictly newer than base under semver
// ordering.
func newer(candidate, base string) bool {
	c := parseSemver(candidate)
	b := parseSemver(base)
	for i := range c {
		if c[i] != b[i] {
			return c[i] > b[i]
		}
	}
	return false
}

func parseSemver(v string) [3]int {
	v = strings.TrimPrefix(v, "v")
	parts := strings.SplitN(v, ".", 3)
	var out [3]int
	for i, p := range parts {
		if i >= 3 {
			break
		}
		p, _, _ = strings.Cut(p, "-")
		n := 0
		for _, ch := range p {
			if ch < '0' || ch > '9' {
				break
			}
			n = n*10 + int(ch-'0')
		}
		out[i] = n
	}
	return out
}
