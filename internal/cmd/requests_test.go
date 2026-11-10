package cmd

import (
	"strings"
	"testing"

	"github.com/bluefunda/bluefunda-ai/internal/app"
)

func TestBaiRequests_Mounted(t *testing.T) {
	for _, path := range [][]string{
		{"requests", "login"}, {"requests", "logout"}, {"requests", "cr", "list"},
		{"requests", "a4h", "status"}, {"requests", "user", "info"}, {"breq", "cr", "get"},
	} {
		if c, _, err := rootCmd.Find(path); err != nil || c == rootCmd {
			t.Errorf("bai %v not found", path)
		}
	}
}

func TestBaiRequests_SwitchesToBreqIdentity(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Cleanup(func() { app.Set(app.Bai) })
	if err := requestsCmd.PersistentPreRunE(requestsCmd, nil); err != nil {
		t.Fatal(err)
	}
	got := app.Current()
	if got.Dir != Breq.Dir || got.KeychainService != Breq.KeychainService || got.Realm != "trm" || got.BFFURL != Breq.BFFURL {
		t.Errorf("bai requests must use breq's login and backend, got %+v", got)
	}
	if got.Name != "bai requests" {
		t.Errorf("hints must name `bai requests`, got %q", got.Name)
	}
}

// A failing requests command skips cobra's post-run hook; Execute must still
// hand the process back to bai (its tips signal is recorded under ~/.bai).
func TestBaiRequests_FailureRestoresBaiIdentity(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("BREQ_NO_KEYCHAIN", "1")
	t.Cleanup(func() { app.Set(app.Bai); rootCmd.SetArgs(nil) })
	rootCmd.SetArgs([]string{"requests", "cr", "list"})
	err := Execute()
	if err == nil || !strings.Contains(err.Error(), "bai requests login") {
		t.Errorf("err = %v, want a hint to run `bai requests login`", err)
	}
	if app.Current().Name != "bai" {
		t.Errorf("identity after Execute = %q, want bai", app.Current().Name)
	}
}
