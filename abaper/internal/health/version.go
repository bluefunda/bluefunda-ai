package health

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// LatestRelease returns the latest released version tag (without a leading
// "v") for bluefunda/abaper from GitHub. Shared by `abaper doctor` and the
// bare TUI's update-available footer badge.
func LatestRelease(ctx context.Context) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.github.com/repos/bluefunda/abaper/releases/latest", nil)
	if err != nil {
		return "", err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()

	var release struct {
		TagName string `json:"tag_name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return "", err
	}
	if release.TagName == "" {
		return "", fmt.Errorf("no release tag returned")
	}
	return strings.TrimPrefix(release.TagName, "v"), nil
}
