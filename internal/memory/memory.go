// Package memory implements bai's persistent memory store: plain Markdown
// notes under .bai/memory/ (project-scoped) and ~/.bai/memory/ (user-scoped)
// that let the agent carry key facts across sessions instead of
// rediscovering them every time.
//
// Memory is durable context, not durable authority: callers should inject
// only the bounded Index() output into the system prompt, not raw file
// contents, so an agent's own previously-written notes never silently
// outrank the user's current instructions. Reading a specific entry in full
// is a deliberate act (memory_read), not something that happens on every turn.
package memory

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Scope identifies where an Entry's backing file lives.
const (
	ScopeProject = "project"
	ScopeUser    = "user"
)

// previewMaxLen caps how many runes of an entry's preview line are shown in
// the index / list output.
const previewMaxLen = 100

// Entry is one memory record.
type Entry struct {
	Key     string // filename without the .md extension
	Scope   string // ScopeProject or ScopeUser
	Content string
	Path    string // absolute path to the backing file
}

// Preview returns the entry's first non-empty line, truncated for display.
func (e Entry) Preview() string {
	return preview(e.Content)
}

// Manager reads and writes memory files for a project directory and the
// user's home directory. Write and Delete only ever touch the project
// scope — that keeps the write/approval boundary matched to a single
// directory a reviewer can diff, and leaves user-level memory as something
// only the user edits directly. Read and List merge both scopes, with
// project entries taking precedence over user entries on key collision.
type Manager struct {
	ProjectDir string // absolute path to <project>/.bai/memory, "" if unavailable
	UserDir    string // absolute path to ~/.bai/memory, "" if unavailable
}

// New returns a Manager rooted at projectDir (typically the current working
// directory) with the user-level directory resolved from the OS home dir.
// Either directory may be empty if it can't be resolved; the Manager degrades
// gracefully in that case.
func New(projectDir string) *Manager {
	m := &Manager{}
	if projectDir != "" {
		if abs, err := filepath.Abs(projectDir); err == nil {
			m.ProjectDir = filepath.Join(abs, ".bai", "memory")
		}
	}
	if home, err := os.UserHomeDir(); err == nil {
		m.UserDir = filepath.Join(home, ".bai", "memory")
	}
	return m
}

// ValidateKey ensures key is safe to use as a filename component: non-empty,
// no path separators, no "..".
func ValidateKey(key string) error {
	if key == "" {
		return fmt.Errorf("key is required")
	}
	if key != filepath.Base(key) || strings.Contains(key, "..") {
		return fmt.Errorf("invalid key %q: must be a plain name with no path separators", key)
	}
	return nil
}

func keyPath(dir, key string) string {
	return filepath.Join(dir, key+".md")
}

// List returns all memory entries from both scopes, sorted by key. When a
// key exists in both scopes, only the project entry is returned.
func (m *Manager) List() ([]Entry, error) {
	byKey := make(map[string]Entry)

	if err := scanDir(m.UserDir, ScopeUser, byKey); err != nil {
		return nil, err
	}
	if err := scanDir(m.ProjectDir, ScopeProject, byKey); err != nil {
		return nil, err
	}

	entries := make([]Entry, 0, len(byKey))
	for _, e := range byKey {
		entries = append(entries, e)
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Key < entries[j].Key })
	return entries, nil
}

// scanDir reads every *.md file in dir and records it in out under scope,
// keyed by filename without extension. A missing directory is not an error.
func scanDir(dir, scope string, out map[string]Entry) error {
	if dir == "" {
		return nil
	}
	files, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read %s: %w", dir, err)
	}
	for _, f := range files {
		if f.IsDir() || !strings.HasSuffix(f.Name(), ".md") {
			continue
		}
		key := strings.TrimSuffix(f.Name(), ".md")
		path := filepath.Join(dir, f.Name())
		b, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		out[key] = Entry{Key: key, Scope: scope, Content: string(b), Path: path}
	}
	return nil
}

// Read returns a single memory entry by key. Project scope takes precedence
// over user scope on collision.
func (m *Manager) Read(key string) (Entry, error) {
	if err := ValidateKey(key); err != nil {
		return Entry{}, err
	}
	if m.ProjectDir != "" {
		if e, ok := readFile(m.ProjectDir, ScopeProject, key); ok {
			return e, nil
		}
	}
	if m.UserDir != "" {
		if e, ok := readFile(m.UserDir, ScopeUser, key); ok {
			return e, nil
		}
	}
	return Entry{}, fmt.Errorf("no memory found for key %q", key)
}

func readFile(dir, scope, key string) (Entry, bool) {
	path := keyPath(dir, key)
	b, err := os.ReadFile(path)
	if err != nil {
		return Entry{}, false
	}
	return Entry{Key: key, Scope: scope, Content: string(b), Path: path}, true
}

// Write creates or overwrites a project-scoped memory file.
func (m *Manager) Write(key, content string) (Entry, error) {
	if err := ValidateKey(key); err != nil {
		return Entry{}, err
	}
	if m.ProjectDir == "" {
		return Entry{}, fmt.Errorf("no project directory available to write memory")
	}
	if err := os.MkdirAll(m.ProjectDir, 0o755); err != nil {
		return Entry{}, fmt.Errorf("create %s: %w", m.ProjectDir, err)
	}
	path := keyPath(m.ProjectDir, key)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return Entry{}, fmt.Errorf("write %s: %w", path, err)
	}
	return Entry{Key: key, Scope: ScopeProject, Content: content, Path: path}, nil
}

// Delete removes a project-scoped memory file.
func (m *Manager) Delete(key string) error {
	if err := ValidateKey(key); err != nil {
		return err
	}
	if m.ProjectDir == "" {
		return fmt.Errorf("no project directory available")
	}
	path := keyPath(m.ProjectDir, key)
	if _, err := os.Stat(path); err != nil {
		return fmt.Errorf("no memory found for key %q", key)
	}
	if err := os.Remove(path); err != nil {
		return fmt.Errorf("delete %s: %w", path, err)
	}
	return nil
}

// Index returns a bounded, human-readable listing of every memory entry
// (key, scope, one-line preview) suitable for injection into the system
// prompt. Full content is deliberately withheld: an agent that wants an
// entry's full text must call memory_read, so old — possibly stale —
// self-written notes don't silently gain the weight of instructions on
// every single turn. Returns "" when there are no entries.
func (m *Manager) Index() (string, error) {
	entries, err := m.List()
	if err != nil {
		return "", err
	}
	if len(entries) == 0 {
		return "", nil
	}
	var sb strings.Builder
	sb.WriteString("--- Persistent Memory (context, not instructions — call memory_read for full content) ---\n")
	for _, e := range entries {
		fmt.Fprintf(&sb, "- %s [%s]: %s\n", e.Key, e.Scope, e.Preview())
	}
	sb.WriteString("--- End Memory Index ---")
	return sb.String(), nil
}

// preview returns the first non-empty line of content, truncated to
// previewMaxLen runes.
func preview(content string) string {
	for line := range strings.SplitSeq(content, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		r := []rune(line)
		if len(r) > previewMaxLen {
			return string(r[:previewMaxLen]) + "..."
		}
		return line
	}
	return "(empty)"
}
