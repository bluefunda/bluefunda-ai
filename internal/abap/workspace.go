package abap

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// stateFile records, per object, the SAP source the workspace file is based on.
const stateFile = ".abaper-state.json"

// ErrConflict means the object changed on SAP after it was checked out, or
// exists on SAP although it was never checked out. Applying would overwrite
// someone else's work.
var ErrConflict = errors.New("conflict")

// fileExt maps an object type to its workspace file extension (abapGit
// names where abapGit has a source file).
var fileExt = map[string]string{
	"class":     ".clas.abap",
	"interface": ".intf.abap",
	"program":   ".prog.abap",
	"include":   ".prog.abap",
	"ddls":      ".ddls.asddls",
	"srvd":      ".srvd.srvdsrv",
	"table":     ".tabl.astabl",
	"structure": ".tabl.astabl",
}

// Base is what the workspace knows about one object's SAP version.
type Base struct {
	Name string    `json:"name"`
	Type string    `json:"type"`
	File string    `json:"file"`           // relative to the workspace root
	Hash string    `json:"hash,omitempty"` // SHA-256 of the SAP source; empty = new object
	At   time.Time `json:"at"`
}

// Workspace keeps ABAP objects as files on disk. A nil *Workspace means the
// feature is off and the pack behaves as it always has.
type Workspace struct {
	Root           string
	InlineMaxLines int // sources longer than this are not returned inline

	mu sync.Mutex
}

// NewWorkspace returns a workspace rooted at dir, creating it if needed.
func NewWorkspace(dir string, inlineMaxLines int) (*Workspace, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, fmt.Errorf("workspace dir: %w", err)
	}
	if err := os.MkdirAll(abs, 0o755); err != nil {
		return nil, fmt.Errorf("create workspace: %w", err)
	}
	if inlineMaxLines <= 0 {
		inlineMaxLines = 200
	}
	return &Workspace{Root: abs, InlineMaxLines: inlineMaxLines}, nil
}

// Supports reports whether objects of this type can live in the workspace.
func (w *Workspace) Supports(objectType string) bool {
	_, ok := fileExt[objectType]
	return ok
}

// FileName is the object's file name inside the workspace.
func FileName(name, objectType string) (string, error) {
	ext, ok := fileExt[objectType]
	if !ok {
		return "", fmt.Errorf("type %s has no workspace file", objectType)
	}
	return strings.ToLower(name) + ext, nil
}

// Path is the absolute path of the object's file.
func (w *Workspace) Path(name, objectType string) (string, error) {
	f, err := FileName(name, objectType)
	if err != nil {
		return "", err
	}
	return filepath.Join(w.Root, f), nil
}

// HashSource returns the hash used for conflict checks. Line endings are
// normalized so a CRLF round trip is not a conflict.
func HashSource(src string) string {
	sum := sha256.Sum256([]byte(strings.ReplaceAll(src, "\r\n", "\n")))
	return hex.EncodeToString(sum[:])
}

func key(name, objectType string) string { return objectType + "/" + strings.ToUpper(name) }

func (w *Workspace) load() (map[string]Base, error) {
	data, err := os.ReadFile(filepath.Join(w.Root, stateFile))
	if errors.Is(err, os.ErrNotExist) {
		return map[string]Base{}, nil
	}
	if err != nil {
		return nil, err
	}
	m := map[string]Base{}
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("%s: %w", stateFile, err)
	}
	return m, nil
}

func (w *Workspace) save(m map[string]Base) error {
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	tmp := filepath.Join(w.Root, stateFile+".tmp")
	if err := os.WriteFile(tmp, append(data, '\n'), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, filepath.Join(w.Root, stateFile))
}

// Base returns the recorded base of an object, if any.
func (w *Workspace) Base(name, objectType string) (Base, bool, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	m, err := w.load()
	if err != nil {
		return Base{}, false, err
	}
	b, ok := m[key(name, objectType)]
	return b, ok, nil
}

// Bases lists every recorded object, sorted by file.
func (w *Workspace) Bases() ([]Base, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	m, err := w.load()
	if err != nil {
		return nil, err
	}
	out := make([]Base, 0, len(m))
	for _, b := range m {
		out = append(out, b)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].File < out[j].File })
	return out, nil
}

func (w *Workspace) setBase(name, objectType, hash string) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	m, err := w.load()
	if err != nil {
		return err
	}
	f, err := FileName(name, objectType)
	if err != nil {
		return err
	}
	m[key(name, objectType)] = Base{Name: strings.ToUpper(name), Type: objectType, File: f, Hash: hash, At: time.Now().UTC()}
	return w.save(m)
}

// Read returns the object's file contents.
func (w *Workspace) Read(name, objectType string) (string, error) {
	p, err := w.Path(name, objectType)
	if err != nil {
		return "", err
	}
	b, err := os.ReadFile(p)
	if err != nil {
		return "", fmt.Errorf("read %s: %w", filepath.Base(p), err)
	}
	return string(b), nil
}

// SourceReader is what checkout and the conflict check need from SAP.
// Client satisfies it; "" with a nil error means the object has no source
// or does not exist.
type SourceReader interface {
	Source(name, objectType string) (string, error)
}

// Checkout writes the SAP source to the workspace and records it as the base.
// It refuses to overwrite local edits (a file that differs from its base)
// unless force is set.
func (w *Workspace) Checkout(r SourceReader, name, objectType string, force bool) (path string, lines int, err error) {
	name = strings.ToUpper(name)
	path, err = w.Path(name, objectType)
	if err != nil {
		return "", 0, err
	}
	if !force {
		if local, rerr := os.ReadFile(path); rerr == nil {
			b, ok, berr := w.Base(name, objectType)
			if berr != nil {
				return "", 0, berr
			}
			if !ok || HashSource(string(local)) != b.Hash {
				return "", 0, fmt.Errorf("%s has local changes; apply them first or check out with force", filepath.Base(path))
			}
		}
	}
	src, err := r.Source(name, objectType)
	if err != nil {
		return "", 0, err
	}
	if src == "" {
		return "", 0, fmt.Errorf("%s %s has no source on SAP (or does not exist)", objectType, name)
	}
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		return "", 0, fmt.Errorf("write %s: %w", filepath.Base(path), err)
	}
	if err := w.setBase(name, objectType, HashSource(src)); err != nil {
		return "", 0, err
	}
	return path, CountLines(src), nil
}

// CheckConflict re-reads SAP and returns ErrConflict when SAP no longer holds
// the base version, or holds an object that was never checked out.
func (w *Workspace) CheckConflict(r SourceReader, name, objectType string) error {
	b, ok, err := w.Base(name, objectType)
	if err != nil {
		return err
	}
	current, err := r.Source(name, objectType)
	if err != nil {
		return fmt.Errorf("re-read %s before apply: %w", name, err)
	}
	switch {
	case !ok || b.Hash == "":
		if current != "" {
			return fmt.Errorf("%w: %s %s already exists on SAP but was not checked out; check it out first", ErrConflict, objectType, name)
		}
	case current == "":
		return fmt.Errorf("%w: %s %s was deleted on SAP after checkout", ErrConflict, objectType, name)
	case HashSource(current) != b.Hash:
		return fmt.Errorf("%w: %s %s changed on SAP after checkout; check it out again and redo the edit", ErrConflict, objectType, name)
	}
	return nil
}

// Applied records a successful deploy of src as the new base.
func (w *Workspace) Applied(name, objectType, src string) error {
	return w.setBase(name, objectType, HashSource(src))
}

// CountLines counts lines the way an editor shows them.
func CountLines(s string) int {
	if s == "" {
		return 0
	}
	n := strings.Count(s, "\n")
	if !strings.HasSuffix(s, "\n") {
		n++
	}
	return n
}
