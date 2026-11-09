package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAddSystem_SetsDefaultsAndActive(t *testing.T) {
	cfg := &SystemsConfig{}
	id := cfg.AddSystem(SAPSystem{Host: "https://abap.example.com", Username: "dev"})

	if id == "" {
		t.Fatal("expected non-empty ID")
	}
	if cfg.Active != id {
		t.Errorf("expected first system to become active, got active=%q id=%q", cfg.Active, id)
	}
	sys := cfg.GetActive()
	if sys == nil {
		t.Fatal("expected active system")
	}
	if sys.Name != sys.Host {
		t.Errorf("expected name to default to host, got %q", sys.Name)
	}
	if sys.Client != "100" {
		t.Errorf("expected default client 100, got %q", sys.Client)
	}
}

func TestAddSystem_SecondSystemDoesNotStealActive(t *testing.T) {
	cfg := &SystemsConfig{}
	first := cfg.AddSystem(SAPSystem{Host: "https://a.example.com"})
	cfg.AddSystem(SAPSystem{Host: "https://b.example.com"})

	if cfg.Active != first {
		t.Errorf("expected active to remain the first system, got %q", cfg.Active)
	}
}

func TestUpdateSystem(t *testing.T) {
	cfg := &SystemsConfig{}
	id := cfg.AddSystem(SAPSystem{Host: "https://a.example.com", Username: "dev"})

	ok := cfg.UpdateSystem(id, SAPSystem{Host: "https://a.example.com", Username: "developer", Client: "001"})
	if !ok {
		t.Fatal("expected update to succeed")
	}
	sys := cfg.FindByNameOrID(id)
	if sys == nil || sys.Username != "developer" || sys.Client != "001" {
		t.Errorf("update did not apply: %+v", sys)
	}

	if cfg.UpdateSystem("does-not-exist", SAPSystem{}) {
		t.Error("expected update of unknown ID to fail")
	}
}

func TestRemoveSystem_ReassignsActive(t *testing.T) {
	cfg := &SystemsConfig{}
	first := cfg.AddSystem(SAPSystem{Host: "https://a.example.com"})
	second := cfg.AddSystem(SAPSystem{Host: "https://b.example.com"})

	if !cfg.RemoveSystem(first) {
		t.Fatal("expected removal to succeed")
	}
	if cfg.Active != second {
		t.Errorf("expected active to move to remaining system, got %q", cfg.Active)
	}
	if len(cfg.Systems) != 1 {
		t.Errorf("expected 1 system remaining, got %d", len(cfg.Systems))
	}

	if !cfg.RemoveSystem(second) {
		t.Fatal("expected removal of last system to succeed")
	}
	if cfg.Active != "" {
		t.Errorf("expected active to clear when no systems remain, got %q", cfg.Active)
	}

	if cfg.RemoveSystem("does-not-exist") {
		t.Error("expected removal of unknown ID to fail")
	}
}

func TestFindByNameOrID(t *testing.T) {
	cfg := &SystemsConfig{}
	id := cfg.AddSystem(SAPSystem{Name: "a4h", Host: "https://a4h.example.com"})

	for _, query := range []string{id, "a4h", "https://a4h.example.com"} {
		if cfg.FindByNameOrID(query) == nil {
			t.Errorf("expected to find system by %q", query)
		}
	}
	if cfg.FindByNameOrID("missing") != nil {
		t.Error("expected nil for unknown query")
	}
}

func TestGetActive_FallsBackToFirst(t *testing.T) {
	cfg := &SystemsConfig{
		Systems: []SAPSystem{{ID: "sys-1", Host: "https://a.example.com"}},
		Active:  "sys-does-not-exist",
	}
	active := cfg.GetActive()
	if active == nil || active.ID != "sys-1" {
		t.Errorf("expected fallback to first system, got %+v", active)
	}
}

func TestGetActive_EmptyConfig(t *testing.T) {
	cfg := &SystemsConfig{}
	if cfg.GetActive() != nil {
		t.Error("expected nil active system for empty config")
	}
}

func TestLoadSaveSystems_RoundTrip(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	loaded, err := LoadSystems()
	if err != nil {
		t.Fatalf("expected no error loading missing systems file, got %v", err)
	}
	if len(loaded.Systems) != 0 {
		t.Fatalf("expected empty systems for missing file, got %+v", loaded)
	}

	cfg := &SystemsConfig{}
	cfg.AddSystem(SAPSystem{Host: "https://abap.example.com", Client: "001", Username: "developer", Password: "secret"})

	if err := SaveSystems(cfg); err != nil {
		t.Fatalf("save systems: %v", err)
	}

	reloaded, err := LoadSystems()
	if err != nil {
		t.Fatalf("reload systems: %v", err)
	}
	if len(reloaded.Systems) != 1 || reloaded.Systems[0].Host != "https://abap.example.com" {
		t.Errorf("unexpected reloaded systems: %+v", reloaded.Systems)
	}

	path := filepath.Join(ConfigDirPath(), systemsFile)
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat systems file: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Errorf("expected systems.json to be 0600, got %o", info.Mode().Perm())
	}
}
