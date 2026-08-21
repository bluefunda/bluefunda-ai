// Package schedule implements bai's local recurring-run store: cron-style
// entries persisted at ~/.bai/schedules.yaml, each firing a headless agent
// run at its own cadence. This is Phase 1 of #177 — client-side only, driven
// by a locally-run `bai daemon start` process. Managed (server-side)
// schedules that survive the laptop being offline are out of scope here.
package schedule

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/robfig/cron/v3"
	"gopkg.in/yaml.v3"
)

// Entry is one scheduled agent run.
type Entry struct {
	ID         string     `yaml:"id"`
	Name       string     `yaml:"name,omitempty"`
	Cron       string     `yaml:"cron"`
	Prompt     string     `yaml:"prompt"`
	Dir        string     `yaml:"dir"`
	Model      string     `yaml:"model,omitempty"`
	Enabled    bool       `yaml:"enabled"`
	CreatedAt  time.Time  `yaml:"created_at"`
	NextRun    *time.Time `yaml:"next_run,omitempty"`
	LastRun    *time.Time `yaml:"last_run,omitempty"`
	LastStatus string     `yaml:"last_status,omitempty"` // "ok" | "error" | ""
	LastError  string     `yaml:"last_error,omitempty"`
}

// ParseCron validates a cron expression and returns its schedule.
func ParseCron(expr string) (cron.Schedule, error) {
	sched, err := cron.ParseStandard(expr)
	if err != nil {
		return nil, fmt.Errorf("invalid cron expression %q: %w", expr, err)
	}
	return sched, nil
}

// Store reads and writes schedule entries at Path (~/.bai/schedules.yaml).
type Store struct {
	Path string
}

// DefaultPath returns ~/.bai/schedules.yaml.
func DefaultPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("home directory: %w", err)
	}
	return filepath.Join(home, ".bai", "schedules.yaml"), nil
}

// New returns a Store backed by path.
func New(path string) *Store {
	return &Store{Path: path}
}

// List returns all entries, sorted by CreatedAt.
func (s *Store) List() ([]Entry, error) {
	entries, err := s.load()
	if err != nil {
		return nil, err
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].CreatedAt.Before(entries[j].CreatedAt) })
	return entries, nil
}

// Get returns the entry with the given id.
func (s *Store) Get(id string) (Entry, bool, error) {
	entries, err := s.load()
	if err != nil {
		return Entry{}, false, err
	}
	for _, e := range entries {
		if e.ID == id {
			return e, true, nil
		}
	}
	return Entry{}, false, nil
}

// Create validates e's cron expression, assigns an ID and NextRun if unset,
// appends it, and saves. e.CreatedAt defaults to now if zero.
func (s *Store) Create(e Entry) (Entry, error) {
	sched, err := ParseCron(e.Cron)
	if err != nil {
		return Entry{}, err
	}
	if e.ID == "" {
		e.ID = newID()
	}
	if e.CreatedAt.IsZero() {
		e.CreatedAt = time.Now()
	}
	next := sched.Next(e.CreatedAt)
	e.NextRun = &next

	entries, err := s.load()
	if err != nil {
		return Entry{}, err
	}
	entries = append(entries, e)
	if err := s.save(entries); err != nil {
		return Entry{}, err
	}
	return e, nil
}

// Update replaces the entry matching e.ID. Returns an error if no entry with
// that ID exists.
func (s *Store) Update(e Entry) error {
	entries, err := s.load()
	if err != nil {
		return err
	}
	for i, existing := range entries {
		if existing.ID == e.ID {
			entries[i] = e
			return s.save(entries)
		}
	}
	return fmt.Errorf("no schedule found with id %q", e.ID)
}

// Delete removes the entry with the given id. Returns false, nil if no such
// entry exists.
func (s *Store) Delete(id string) (bool, error) {
	entries, err := s.load()
	if err != nil {
		return false, err
	}
	out := entries[:0]
	found := false
	for _, e := range entries {
		if e.ID == id {
			found = true
			continue
		}
		out = append(out, e)
	}
	if !found {
		return false, nil
	}
	return true, s.save(out)
}

func (s *Store) load() ([]Entry, error) {
	data, err := os.ReadFile(s.Path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read %s: %w", s.Path, err)
	}
	var doc struct {
		Schedules []Entry `yaml:"schedules"`
	}
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("parse %s: %w", s.Path, err)
	}
	return doc.Schedules, nil
}

func (s *Store) save(entries []Entry) error {
	if err := os.MkdirAll(filepath.Dir(s.Path), 0o700); err != nil {
		return fmt.Errorf("create %s: %w", filepath.Dir(s.Path), err)
	}
	doc := struct {
		Schedules []Entry `yaml:"schedules"`
	}{Schedules: entries}
	data, err := yaml.Marshal(&doc)
	if err != nil {
		return fmt.Errorf("marshal schedules: %w", err)
	}
	if err := os.WriteFile(s.Path, data, 0o600); err != nil {
		return fmt.Errorf("write %s: %w", s.Path, err)
	}
	return nil
}

// newID returns a short unique id in the same style used elsewhere in bai
// for generated identifiers (e.g. notebook cell ids).
func newID() string {
	return strings.ReplaceAll(uuid.New().String(), "-", "")[:8]
}
