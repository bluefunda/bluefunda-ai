package app

import (
	"path/filepath"
	"testing"
)

func TestDefaultIsBai(t *testing.T) {
	if got := Current(); got.Name != "bai" || got.Dir != ".bai" || got.EnvPrefix != "BAI_" {
		t.Errorf("default identity = %+v, want bai", got)
	}
}

func TestSetChangesPathsAndEnv(t *testing.T) {
	t.Cleanup(func() { Set(Bai) })
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("ABAPER_MODEL", "fast")
	t.Setenv("BAI_MODEL", "think")

	Set(Identity{Name: "abaper", Dir: ".abaper", EnvPrefix: "ABAPER_"})

	if got := Getenv("MODEL"); got != "fast" {
		t.Errorf("Getenv(MODEL) = %q, want the ABAPER_ value", got)
	}
	got, err := HomePath("sessions")
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(home, ".abaper", "sessions"); got != want {
		t.Errorf("HomePath = %q, want %q", got, want)
	}
}
