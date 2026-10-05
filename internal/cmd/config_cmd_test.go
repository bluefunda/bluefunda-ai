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

func TestConfigKeys_UpdateCheck_DefaultsToTrue(t *testing.T) {
	cfg := &config.Config{}
	k := configKeys["update.check"]
	if got := k.get(cfg); got != "true" {
		t.Errorf("update.check default = %q, want %q (unset = enabled)", got, "true")
	}
}

func TestConfigKeys_UpdateCheck_SetRoundTrip(t *testing.T) {
	cfg := &config.Config{}
	k := configKeys["update.check"]
	if err := k.set(cfg, "false"); err != nil {
		t.Fatalf("set(false): %v", err)
	}
	if got := k.get(cfg); got != "false" {
		t.Errorf("after set(false), get() = %q, want %q", got, "false")
	}
	if cfg.UpdateCheckEnabled() {
		t.Error("expected UpdateCheckEnabled() to be false after set(false)")
	}

	if err := k.set(cfg, "true"); err != nil {
		t.Fatalf("set(true): %v", err)
	}
	if got := k.get(cfg); got != "true" {
		t.Errorf("after set(true), get() = %q, want %q", got, "true")
	}
}

func TestConfigKeys_UpdateCheck_InvalidValueErrors(t *testing.T) {
	cfg := &config.Config{}
	k := configKeys["update.check"]
	err := k.set(cfg, "maybe")
	if err == nil {
		t.Fatal("expected an error for an invalid bool value, got nil")
	}
	if !strings.Contains(err.Error(), "maybe") {
		t.Errorf("error = %q, want it to mention the invalid value", err.Error())
	}
	if cfg.UpdateCheck != nil {
		t.Error("expected cfg.UpdateCheck to remain unset after a rejected value")
	}
}

func TestConfigKeys_ExistingKeysStillSetWithoutError(t *testing.T) {
	cfg := &config.Config{}
	for _, key := range []string{"model", "output", "endpoint"} {
		k := configKeys[key]
		if err := k.set(cfg, "x"); err != nil {
			t.Errorf("set(%q, \"x\") returned an error after the signature change: %v", key, err)
		}
		if got := k.get(cfg); got != "x" {
			t.Errorf("get(%q) = %q, want %q", key, got, "x")
		}
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
