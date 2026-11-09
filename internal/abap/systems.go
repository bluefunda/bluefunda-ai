package abap

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/bluefunda/bluefunda-ai/internal/app"
)

// Command is the CLI prefix for ABAP commands in user-facing hints:
// "bai abap" under bai, "abaper" in the abaper binary.
var Command = "bai abap"

// SystemAddHint is the command that registers an SAP system.
func SystemAddHint() string { return Command + " system add" }

const systemsFile = "systems.json"

// SAPSystem holds the connection details for one SAP system.
type SAPSystem struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Host     string `json:"host"`
	Client   string `json:"client"`
	Username string `json:"username"`
	Password string `json:"password"`
}

// Systems is the registry stored in <home>/systems.json (same format the
// standalone abaper CLI wrote to ~/.abaper/systems.json).
type Systems struct {
	Systems []SAPSystem `json:"systems"`
	Active  string      `json:"active"`
}

func systemsPath() (string, error) {
	return app.HomePath(systemsFile)
}

// LoadSystems reads the registry; a missing file is an empty registry.
func LoadSystems() (*Systems, error) {
	path, err := systemsPath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return &Systems{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read systems: %w", err)
	}
	var s Systems
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	return &s, nil
}

// Save writes the registry with 0600 permissions; it holds SAP passwords.
func (s *Systems) Save() error {
	path, err := systemsPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal systems: %w", err)
	}
	return os.WriteFile(path, data, 0o600)
}

// ActiveSystem returns the active system, falling back to the first one.
func (s *Systems) ActiveSystem() *SAPSystem {
	for i := range s.Systems {
		if s.Systems[i].ID == s.Active {
			return &s.Systems[i]
		}
	}
	if len(s.Systems) > 0 {
		return &s.Systems[0]
	}
	return nil
}

// Add appends sys, makes it active if it is the first, and returns its ID.
func (s *Systems) Add(sys SAPSystem) string {
	// Millisecond IDs collide when systems are added back to back; bump
	// until unique so Find/Active never confuse two systems.
	ms := time.Now().UnixMilli()
	sys.ID = fmt.Sprintf("sys-%d", ms)
	for s.hasID(sys.ID) {
		ms++
		sys.ID = fmt.Sprintf("sys-%d", ms)
	}
	if sys.Name == "" {
		sys.Name = sys.Host
	}
	if sys.Client == "" {
		sys.Client = "100"
	}
	s.Systems = append(s.Systems, sys)
	if s.Active == "" {
		s.Active = sys.ID
	}
	return sys.ID
}

func (s *Systems) hasID(id string) bool {
	for _, sys := range s.Systems {
		if sys.ID == id {
			return true
		}
	}
	return false
}

// Remove deletes the system with the given ID and moves the active pointer if needed.
func (s *Systems) Remove(id string) bool {
	for i, sys := range s.Systems {
		if sys.ID != id {
			continue
		}
		s.Systems = append(s.Systems[:i], s.Systems[i+1:]...)
		if s.Active == id {
			s.Active = ""
			if len(s.Systems) > 0 {
				s.Active = s.Systems[0].ID
			}
		}
		return true
	}
	return false
}

// Find looks a system up by ID, name or host.
func (s *Systems) Find(q string) *SAPSystem {
	for i := range s.Systems {
		if sys := &s.Systems[i]; sys.ID == q || sys.Name == q || sys.Host == q {
			return sys
		}
	}
	return nil
}
