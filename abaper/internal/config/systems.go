package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const systemsFile = "systems.json"

// SAPSystem holds credentials and metadata for one SAP system.
type SAPSystem struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Host     string `json:"host"`
	Client   string `json:"client"`
	Username string `json:"username"`
	Password string `json:"password"`
}

// SystemsConfig is the root of ~/.abaper/systems.json.
type SystemsConfig struct {
	Systems []SAPSystem `json:"systems"`
	Active  string      `json:"active"`
}

func systemsFilePath() string {
	return filepath.Join(ConfigDirPath(), systemsFile)
}

// LoadSystems reads ~/.abaper/systems.json; returns an empty config if missing.
func LoadSystems() (*SystemsConfig, error) {
	data, err := os.ReadFile(systemsFilePath())
	if os.IsNotExist(err) {
		return &SystemsConfig{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read systems: %w", err)
	}
	var cfg SystemsConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse systems: %w", err)
	}
	return &cfg, nil
}

// SaveSystems writes cfg to ~/.abaper/systems.json (0600 perms).
func SaveSystems(cfg *SystemsConfig) error {
	if err := EnsureConfigDir(); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal systems: %w", err)
	}
	return os.WriteFile(systemsFilePath(), data, 0600)
}

// GetActive returns the active system, falling back to the first one.
func (c *SystemsConfig) GetActive() *SAPSystem {
	for i := range c.Systems {
		if c.Systems[i].ID == c.Active {
			return &c.Systems[i]
		}
	}
	if len(c.Systems) > 0 {
		return &c.Systems[0]
	}
	return nil
}

// AddSystem appends a new system, sets it active if none exists, and returns its ID.
func (c *SystemsConfig) AddSystem(sys SAPSystem) string {
	sys.ID = fmt.Sprintf("sys-%d", time.Now().UnixMilli())
	if sys.Name == "" {
		sys.Name = sys.Host
	}
	if sys.Client == "" {
		sys.Client = "100"
	}
	c.Systems = append(c.Systems, sys)
	if c.Active == "" {
		c.Active = sys.ID
	}
	return sys.ID
}

// UpdateSystem replaces an existing system by ID.
func (c *SystemsConfig) UpdateSystem(id string, updated SAPSystem) bool {
	for i := range c.Systems {
		if c.Systems[i].ID == id {
			updated.ID = id
			if updated.Name == "" {
				updated.Name = updated.Host
			}
			if updated.Client == "" {
				updated.Client = "100"
			}
			c.Systems[i] = updated
			return true
		}
	}
	return false
}

// RemoveSystem removes a system by ID and adjusts the active pointer.
func (c *SystemsConfig) RemoveSystem(id string) bool {
	for i, s := range c.Systems {
		if s.ID == id {
			c.Systems = append(c.Systems[:i], c.Systems[i+1:]...)
			if c.Active == id {
				if len(c.Systems) > 0 {
					c.Active = c.Systems[0].ID
				} else {
					c.Active = ""
				}
			}
			return true
		}
	}
	return false
}

// FindByNameOrID looks up a system by ID, display name, or host.
func (c *SystemsConfig) FindByNameOrID(q string) *SAPSystem {
	for i := range c.Systems {
		s := &c.Systems[i]
		if s.ID == q || s.Name == q || s.Host == q {
			return s
		}
	}
	return nil
}
