package cmd

import (
	"strings"
	"testing"

	"github.com/bluefunda/bluefunda-ai/internal/config"
	"github.com/bluefunda/bluefunda-ai/internal/ui"
)

func TestUseProfile_SetsDefaultProfile(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	cfg := &config.Config{
		Profiles: map[string]config.Profile{"dev": {Endpoint: "dev.internal:443"}},
	}
	p, buf := testPrinter(ui.FormatTable)

	if err := useProfile(cfg, "dev", p); err != nil {
		t.Fatalf("useProfile: %v", err)
	}
	if cfg.DefaultProfile != "dev" {
		t.Errorf("cfg.DefaultProfile = %q, want %q", cfg.DefaultProfile, "dev")
	}
	if !strings.Contains(buf.String(), "dev") {
		t.Errorf("expected success message to mention the profile, got: %s", buf.String())
	}
}

func TestUseProfile_UnknownNameListsAvailable(t *testing.T) {
	cfg := &config.Config{
		Profiles: map[string]config.Profile{"dev": {}, "staging": {}},
	}
	p, _ := testPrinter(ui.FormatTable)

	err := useProfile(cfg, "nonexistent", p)
	if err == nil {
		t.Fatal("expected an error for an unknown profile, got nil")
	}
	if !strings.Contains(err.Error(), "dev") || !strings.Contains(err.Error(), "staging") {
		t.Errorf("error = %q, want it to list available profile names", err.Error())
	}
	if cfg.DefaultProfile != "" {
		t.Errorf("cfg.DefaultProfile = %q, want unchanged after a failed use-profile", cfg.DefaultProfile)
	}
}

func TestUseProfile_NoneConfigured(t *testing.T) {
	cfg := &config.Config{}
	p, _ := testPrinter(ui.FormatTable)

	err := useProfile(cfg, "dev", p)
	if err == nil {
		t.Fatal("expected an error when no profiles are configured, got nil")
	}
	if !strings.Contains(err.Error(), "no profiles are configured") {
		t.Errorf("error = %q, want a clear \"no profiles configured\" message", err.Error())
	}
}
