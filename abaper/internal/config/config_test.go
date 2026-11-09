package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSaveLoadClearTokens_RoundTrip(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	if _, err := LoadTokens(); err == nil {
		t.Fatal("expected error loading tokens before any are saved")
	}

	tokens := &Tokens{
		AccessToken:  "access-123",
		RefreshToken: "refresh-456",
		ExpiresAt:    1234567890,
	}
	if err := SaveTokens(tokens); err != nil {
		t.Fatalf("save tokens: %v", err)
	}

	loaded, err := LoadTokens()
	if err != nil {
		t.Fatalf("load tokens: %v", err)
	}
	if loaded.AccessToken != tokens.AccessToken || loaded.RefreshToken != tokens.RefreshToken || loaded.ExpiresAt != tokens.ExpiresAt {
		t.Errorf("loaded tokens do not match saved tokens: %+v", loaded)
	}

	path := filepath.Join(ConfigDirPath(), TokenFile)
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat tokens file: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Errorf("expected tokens.yaml to be 0600, got %o", info.Mode().Perm())
	}

	if err := ClearTokens(); err != nil {
		t.Fatalf("clear tokens: %v", err)
	}
	if _, err := LoadTokens(); err == nil {
		t.Fatal("expected error loading tokens after clearing")
	}
}

func TestClearTokens_MissingFileIsNotAnError(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if err := ClearTokens(); err != nil {
		t.Errorf("expected no error clearing nonexistent tokens, got %v", err)
	}
}

func TestConfigDirPath(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	want := filepath.Join(home, ConfigDir)
	if got := ConfigDirPath(); got != want {
		t.Errorf("expected %q, got %q", want, got)
	}
}
