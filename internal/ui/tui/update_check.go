package tui

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// UpdateAvailableMsg is sent when a newer bai version is found on GitHub.
type UpdateAvailableMsg struct{ Version string }

// checkForUpdateCmd fires a background goroutine that hits the GitHub releases
// API. It sends UpdateAvailableMsg if a newer version is found, or returns nil
// (silently ignored by BubbleTea) on any error or when already up-to-date.
// Only runs when version is a real semver tag (not "dev").
func checkForUpdateCmd(current string) tea.Cmd {
	if current == "" || current == "dev" {
		return nil
	}
	return func() tea.Msg {
		tag, err := fetchLatestGitHubRelease("bluefunda", "bluefunda-ai")
		if err != nil || tag == "" {
			return nil
		}
		if tuiSemverNewer(tag, current) {
			return UpdateAvailableMsg{Version: tag}
		}
		return nil
	}
}

type githubReleasePayload struct {
	TagName string `json:"tag_name"`
}

func fetchLatestGitHubRelease(owner, repo string) (string, error) {
	client := &http.Client{Timeout: 5 * time.Second}
	url := "https://api.github.com/repos/" + owner + "/" + repo + "/releases/latest"
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "bai/tui (+github.com/bluefunda/bluefunda-ai)")
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

// tuiSemverNewer reports whether candidate is strictly newer than base.
func tuiSemverNewer(candidate, base string) bool {
	c := tuiParseSemver(candidate)
	b := tuiParseSemver(base)
	for i := range c {
		if c[i] != b[i] {
			return c[i] > b[i]
		}
	}
	return false
}

func tuiParseSemver(v string) [3]int {
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
