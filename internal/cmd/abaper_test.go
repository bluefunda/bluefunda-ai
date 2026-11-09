package cmd

import (
	"testing"

	"github.com/bluefunda/bluefunda-ai/internal/app"
)

func TestABAPerRoot_CommandSurface(t *testing.T) {
	root := newABAPerRoot()
	for _, path := range [][]string{
		{"login"}, {"logout"}, {"version"}, {"update"},
		{"status"}, {"system", "add"}, {"search"}, {"get"}, {"list"},
		{"check"}, {"deploy"}, {"test"}, {"code"}, {"generate"},
		{"ai", "code"}, {"ai", "chat"}, // compat with the standalone abaper CLI
	} {
		if c, _, err := root.Find(path); err != nil || c == root {
			t.Errorf("abaper %v not found", path)
		}
	}
	if c, _, _ := root.Find([]string{"chat"}); c != root {
		t.Error("bai-only commands must not leak into abaper")
	}
}

func TestABAPerIdentity(t *testing.T) {
	if ABAPer.Dir != ".abaper" || ABAPer.EnvPrefix != "ABAPER_" || ABAPer.ClientID != "cai-cli" || ABAPer.HomebrewCask != "abaper" {
		t.Errorf("abaper identity changed: %+v — existing installs depend on these", ABAPer)
	}
	if app.Current().Name != "bai" {
		t.Error("building the abaper root must not switch the process identity; only ExecuteABAPer does")
	}
}
