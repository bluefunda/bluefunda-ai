package tips

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	tipcatalog "github.com/bluefunda/tipcatalog"
)

const (
	catalogOwner           = "bluefunda"
	catalogRepo            = "tipcatalog"
	catalogRefreshInterval = 24 * time.Hour

	catalogFileName    = "catalog.json"
	catalogSigFileName = "catalog.json.sig"
	catalogMarkerName  = "catalog.checked_at"
)

// fetchLatestManifestFn is overridable in tests so they never hit the
// network. Returns the compiled catalog bytes and its raw (non-base64)
// signature.
var fetchLatestManifestFn = fetchLatestManifest

// CLITips returns the current CLI-surfaced tip set. It prefers the cached
// signed manifest fetched by EnsureFresh; if that's missing, unreadable, or
// fails signature verification, it falls back to the catalog embedded in
// this binary. Either way, the result is filtered to tips whose Surfaces
// includes "cli".
func CLITips() ([]tipcatalog.Tip, error) {
	if tips, ok := loadCachedManifest(); ok {
		return filterSurface(tips, tipcatalog.SurfaceCLI), nil
	}
	tips, err := tipcatalog.Embedded()
	if err != nil {
		return nil, err
	}
	return filterSurface(tips, tipcatalog.SurfaceCLI), nil
}

// EnsureFresh spawns a detached goroutine that refreshes the cached manifest
// if it's missing or older than 24h, and returns immediately without
// waiting for it — callers must never block command exit on this. Because
// the goroutine keeps running only for as long as the process does, very
// short-lived commands may exit before it completes; that's an accepted
// trade-off of the "opportunistic, zero-latency" design (see the Contextual
// Tip Engine spec) — the next invocation (or any longer-running command,
// e.g. an interactive chat session) will pick it up.
func EnsureFresh() {
	go refreshIfStale()
}

// refreshIfStale is the synchronous body of EnsureFresh, split out so tests
// can call it directly instead of racing a goroutine.
func refreshIfStale() {
	dir, err := tipsDir()
	if err != nil {
		return
	}

	marker := filepath.Join(dir, catalogMarkerName)
	if data, err := os.ReadFile(marker); err == nil {
		if last, err := time.Parse(time.RFC3339, strings.TrimSpace(string(data))); err == nil {
			if now().Sub(last) < catalogRefreshInterval {
				return
			}
		}
	}
	// Touch the marker before fetching so a slow or failing fetch doesn't
	// get retried on every single invocation until it succeeds.
	_ = os.WriteFile(marker, []byte(now().Format(time.RFC3339)), 0o600)

	data, sig, err := fetchLatestManifestFn()
	if err != nil {
		return
	}
	if !tipcatalog.Verify(data, sig, tipcatalog.PublicKey) {
		return
	}
	_ = atomicWriteFile(filepath.Join(dir, catalogFileName), data)
	_ = atomicWriteFile(filepath.Join(dir, catalogSigFileName), []byte(base64.StdEncoding.EncodeToString(sig)))
}

// loadCachedManifest reads and verifies the cached manifest. ok is false on
// any failure (missing files, bad signature, malformed JSON) — callers
// should treat that as "no cache" and fall back to the embedded catalog.
func loadCachedManifest() (tips []tipcatalog.Tip, ok bool) {
	dir, err := tipsDir()
	if err != nil {
		return nil, false
	}

	data, err := os.ReadFile(filepath.Join(dir, catalogFileName))
	if err != nil {
		return nil, false
	}
	sigB64, err := os.ReadFile(filepath.Join(dir, catalogSigFileName))
	if err != nil {
		return nil, false
	}
	sig, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(sigB64)))
	if err != nil {
		return nil, false
	}
	if !tipcatalog.Verify(data, sig, tipcatalog.PublicKey) {
		return nil, false
	}
	if err := json.Unmarshal(data, &tips); err != nil {
		return nil, false
	}
	return tips, true
}

// filterSurface returns the subset of tips whose Surfaces includes surface.
func filterSurface(tips []tipcatalog.Tip, surface string) []tipcatalog.Tip {
	var out []tipcatalog.Tip
	for _, t := range tips {
		for _, s := range t.Surfaces {
			if s == surface {
				out = append(out, t)
				break
			}
		}
	}
	return out
}

type githubAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

type githubReleasePayload struct {
	Assets []githubAsset `json:"assets"`
}

// fetchLatestManifest downloads catalog.json and catalog.json.sig from the
// tip-catalog repo's latest GitHub Release. sig is decoded from its
// base64 asset content.
func fetchLatestManifest() (data, sig []byte, err error) {
	client := &http.Client{Timeout: 5 * time.Second}

	req, err := http.NewRequest(http.MethodGet,
		fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/latest", catalogOwner, catalogRepo), nil)
	if err != nil {
		return nil, nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "bai/tips (+github.com/bluefunda/bluefunda-ai)")

	resp, err := client.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, nil, fmt.Errorf("github releases api: unexpected status %d", resp.StatusCode)
	}

	var rel githubReleasePayload
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return nil, nil, err
	}

	var catalogURL, sigURL string
	for _, a := range rel.Assets {
		switch a.Name {
		case catalogFileName:
			catalogURL = a.BrowserDownloadURL
		case catalogSigFileName:
			sigURL = a.BrowserDownloadURL
		}
	}
	if catalogURL == "" || sigURL == "" {
		return nil, nil, errors.New("latest release is missing catalog.json or catalog.json.sig asset")
	}

	data, err = fetchAsset(client, catalogURL)
	if err != nil {
		return nil, nil, err
	}
	sigB64, err := fetchAsset(client, sigURL)
	if err != nil {
		return nil, nil, err
	}
	sig, err = base64.StdEncoding.DecodeString(strings.TrimSpace(string(sigB64)))
	if err != nil {
		return nil, nil, err
	}
	return data, sig, nil
}

func fetchAsset(client *http.Client, url string) ([]byte, error) {
	resp, err := client.Get(url) //nolint:noctx
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch asset: unexpected status %d", resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}
