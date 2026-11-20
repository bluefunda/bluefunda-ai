// Package checkpoint keeps the file contents an agent session's edit tools
// overwrite, so /rewind can put files back as they were before an earlier
// message, like Claude Code's checkpoints.
//
// Each message the user sends starts a turn. Before an edit tool first
// changes a file during a turn, the file's previous contents (or the fact
// that it did not exist) are saved. Restoring to turn N gives every file
// changed in turn N or later the contents it had when it was first changed
// at or after N. Only edit tools are tracked: changes made by shell
// commands are not, as in Claude Code.
package checkpoint

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

// Limits on what is kept in memory. A file over MaxFileBytes is not saved;
// once MaxTotalBytes is used, no more files are saved this session.
const (
	MaxFileBytes  = 5 << 20
	MaxTotalBytes = 100 << 20
)

// Point is one message the user sent, as listed by /rewind.
type Point struct {
	Turn   int
	Prompt string    // as the user typed it
	At     time.Time // when it was sent
	// HistoryLen is the conversation's length before this message, and
	// Message the exact message added there. The conversation can be cut
	// back to it only while that message is still at that position:
	// compaction or a retry rewrites history and invalidates it.
	HistoryLen int
	Message    string
	// Files is how many files were changed in this turn or later.
	Files int
}

type backup struct {
	turn    int
	path    string // absolute
	existed bool
	data    []byte
	mode    os.FileMode
}

// Store holds a session's checkpoints. It is safe for concurrent use.
type Store struct {
	mu      sync.Mutex
	turn    int
	points  []Point
	backups []backup // in the order taken; turn is non-decreasing
	total   int
	skipped map[string]string // path → why it could not be saved
}

// New returns an empty store.
func New() *Store { return &Store{skipped: map[string]string{}} }

// Begin starts a new turn for a message the user sent: prompt as typed,
// message as added to the conversation at position historyLen.
func (s *Store) Begin(prompt, message string, historyLen int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.turn++
	// Messages at or after this position are gone from the conversation (a
	// retry or rewind replaced them); their backups stay.
	kept := s.points[:0]
	for _, p := range s.points {
		if p.HistoryLen < historyLen {
			kept = append(kept, p)
		}
	}
	s.points = append(kept, Point{Turn: s.turn, Prompt: prompt, At: time.Now(), HistoryLen: historyLen, Message: message})
}

// BeforeEdit saves path's current contents if this is the first change to
// it in the current turn. Call it before the edit runs.
func (s *Store) BeforeEdit(path string) {
	if s == nil {
		return
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.turn == 0 {
		return
	}
	for i := len(s.backups) - 1; i >= 0 && s.backups[i].turn == s.turn; i-- {
		if s.backups[i].path == abs {
			return
		}
	}
	b := backup{turn: s.turn, path: abs}
	info, err := os.Stat(abs)
	switch {
	case errors.Is(err, os.ErrNotExist):
		// Restoring removes it.
	case err != nil:
		s.skipped[abs] = err.Error()
		return
	case info.Size() > MaxFileBytes:
		s.skipped[abs] = fmt.Sprintf("larger than %d MB", MaxFileBytes>>20)
		return
	case s.total+int(info.Size()) > MaxTotalBytes:
		s.skipped[abs] = "checkpoint memory limit reached"
		return
	default:
		data, err := os.ReadFile(abs)
		if err != nil {
			s.skipped[abs] = err.Error()
			return
		}
		b.existed, b.data, b.mode = true, data, info.Mode().Perm()
		s.total += len(data)
	}
	s.backups = append(s.backups, b)
}

// Points lists the messages that can be rewound to, oldest first, with how
// many files each would restore.
func (s *Store) Points() []Point {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Point, len(s.points))
	for i, p := range s.points {
		p.Files = len(s.earliestFrom(p.Turn))
		out[i] = p
	}
	return out
}

// earliestFrom returns, per path, the first backup taken in turn or later.
func (s *Store) earliestFrom(turn int) map[string]backup {
	first := map[string]backup{}
	for _, b := range s.backups {
		if b.turn < turn {
			continue
		}
		if _, ok := first[b.path]; !ok {
			first[b.path] = b
		}
	}
	return first
}

// RestoreCode puts every file changed in turn or later back as it was
// before that turn and returns the paths restored (sorted). Backups from
// turn on are dropped: the files are now in that earlier state.
func (s *Store) RestoreCode(turn int) ([]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var restored []string
	var errs []error
	for path, b := range s.earliestFrom(turn) {
		var err error
		if b.existed {
			if err = os.MkdirAll(filepath.Dir(path), 0o755); err == nil {
				err = os.WriteFile(path, b.data, b.mode)
			}
		} else if err = os.Remove(path); errors.Is(err, os.ErrNotExist) {
			err = nil
		}
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", path, err))
			continue
		}
		restored = append(restored, path)
	}
	kept := s.backups[:0]
	s.total = 0
	for _, b := range s.backups {
		if b.turn < turn {
			kept = append(kept, b)
			s.total += len(b.data)
		}
	}
	s.backups = kept
	sort.Strings(restored)
	return restored, errors.Join(errs...)
}

// DropPointsFrom removes the message at turn and later ones from the list,
// after the conversation is cut back to before it. Their backups stay, so
// restoring code to an earlier message still covers their files.
func (s *Store) DropPointsFrom(turn int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	kept := s.points[:0]
	for _, p := range s.points {
		if p.Turn < turn {
			kept = append(kept, p)
		}
	}
	s.points = kept
}

// Skipped returns files that changed but could not be saved, with why.
func (s *Store) Skipped() map[string]string {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make(map[string]string, len(s.skipped))
	for k, v := range s.skipped {
		out[k] = v
	}
	return out
}
