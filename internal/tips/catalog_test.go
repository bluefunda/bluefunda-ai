package tips

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	tipcatalog "github.com/bluefunda/tipcatalog"
)

// failNetworkFetch fails the test if the catalog client ever tries to hit
// the network — CLITips must work entirely off the embedded fallback when
// there's no valid cache.
func failNetworkFetch(t *testing.T) {
	t.Helper()
	orig := fetchLatestManifestFn
	fetchLatestManifestFn = func() ([]byte, []byte, error) {
		t.Fatal("fetchLatestManifestFn called — CLITips must not hit the network")
		return nil, nil, nil
	}
	t.Cleanup(func() { fetchLatestManifestFn = orig })
}

func hasSurface(tp tipcatalog.Tip, surface string) bool {
	for _, s := range tp.Surfaces {
		if s == surface {
			return true
		}
	}
	return false
}

func TestCLITips_FallsBackToEmbeddedWithNoCache(t *testing.T) {
	withHome(t)
	failNetworkFetch(t)

	tips, err := CLITips()
	if err != nil {
		t.Fatalf("CLITips: %v", err)
	}
	if len(tips) == 0 {
		t.Fatal("expected at least one embedded CLI tip")
	}
	for _, tp := range tips {
		if !hasSurface(tp, tipcatalog.SurfaceCLI) {
			t.Fatalf("tip %q returned by CLITips does not declare the cli surface", tp.ID)
		}
	}
}

func writeCache(t *testing.T, home string, catalog, sig []byte) {
	t.Helper()
	dir := filepath.Join(home, ".bai", "tips")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, catalogFileName), catalog, 0o600); err != nil {
		t.Fatalf("write catalog: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, catalogSigFileName), sig, 0o600); err != nil {
		t.Fatalf("write sig: %v", err)
	}
}

func TestCLITips_FallsBackOnGarbageSignature(t *testing.T) {
	home := withHome(t)
	failNetworkFetch(t)

	garbageSig := base64.StdEncoding.EncodeToString([]byte("not-a-real-signature-not-a-real-sig!!"))
	writeCache(t, home, []byte(`[]`), []byte(garbageSig))

	tips, err := CLITips()
	if err != nil {
		t.Fatalf("CLITips: %v", err)
	}
	if len(tips) == 0 {
		t.Fatal("expected fallback to embedded tips on bad signature, got none")
	}
}

func TestCLITips_FallsBackWhenSignedByForeignKey(t *testing.T) {
	home := withHome(t)
	failNetworkFetch(t)

	// A well-formed, validly-signed manifest — but signed by a key that
	// isn't tipcatalog.PublicKey. Proves verification is genuinely enforced
	// against the real embedded key, not just checking "is this well-formed".
	_, foreignPriv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	tip := tipcatalog.Tip{
		ID:             "foreign-signed-tip",
		Family:         "fam",
		Surfaces:       []string{tipcatalog.SurfaceCLI},
		Render:         tipcatalog.Render{CLI: &tipcatalog.RenderContent{Title: "T", Body: "B"}},
		Embedding:      make([]float64, 32),
		CatalogVersion: "1",
	}
	compiled, err := tipcatalog.Compile([]tipcatalog.Tip{tip})
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	sig := tipcatalog.Sign(compiled, foreignPriv)
	writeCache(t, home, compiled, []byte(base64.StdEncoding.EncodeToString(sig)))

	tips, err := CLITips()
	if err != nil {
		t.Fatalf("CLITips: %v", err)
	}
	for _, tp := range tips {
		if tp.ID == "foreign-signed-tip" {
			t.Fatal("a manifest signed by a foreign key must not be trusted")
		}
	}
}

func TestEnsureFresh_DoesNotBlock(t *testing.T) {
	withHome(t)
	proceed := make(chan struct{})
	fetchDone := make(chan struct{})
	orig := fetchLatestManifestFn
	fetchLatestManifestFn = func() ([]byte, []byte, error) {
		<-proceed // would hang forever if EnsureFresh waited on us
		close(fetchDone)
		return nil, nil, errors.New("simulated fetch failure")
	}
	t.Cleanup(func() { fetchLatestManifestFn = orig })

	done := make(chan struct{})
	go func() {
		EnsureFresh()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("EnsureFresh blocked for over 1s — it must return immediately")
	}

	// Let the background goroutine finish (and restore
	// fetchLatestManifestFn) before this test returns, so it can't race
	// with the next test's use of the same package-level var.
	close(proceed)
	<-fetchDone
}

func TestRefreshIfStale_ThrottledWithin24h(t *testing.T) {
	withHome(t)

	calls := 0
	orig := fetchLatestManifestFn
	fetchLatestManifestFn = func() ([]byte, []byte, error) {
		calls++
		return nil, nil, errors.New("simulated fetch failure")
	}
	t.Cleanup(func() { fetchLatestManifestFn = orig })

	refreshIfStale()
	refreshIfStale()
	refreshIfStale()

	if calls != 1 {
		t.Fatalf("expected exactly 1 fetch within the 24h throttle window, got %d", calls)
	}
}

func TestRefreshIfStale_RefetchesAfterWindowExpires(t *testing.T) {
	withHome(t)

	calls := 0
	orig := fetchLatestManifestFn
	fetchLatestManifestFn = func() ([]byte, []byte, error) {
		calls++
		return nil, nil, errors.New("simulated fetch failure")
	}
	t.Cleanup(func() { fetchLatestManifestFn = orig })

	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	origNow := now
	now = func() time.Time { return base }
	t.Cleanup(func() { now = origNow })

	refreshIfStale()
	now = func() time.Time { return base.Add(catalogRefreshInterval + time.Minute) }
	refreshIfStale()

	if calls != 2 {
		t.Fatalf("expected 2 fetches after the throttle window elapsed, got %d", calls)
	}
}
