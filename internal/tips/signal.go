// Package tips collects local interest signal from bai invocations so later
// phases of the Contextual Tip Engine can rank which tips to show. See
// github.com/bluefunda/tipcatalog for the shared tip content schema this
// eventually feeds into (Phase 2/3 wire the two together).
package tips

import (
	"bufio"
	"encoding/json"
	"hash/fnv"
	"math"
	"os"
	"path/filepath"
	"time"
)

// now is overridable in tests, matching this repo's execCommand/httpClient
// test-injection pattern (see go-update's upgrader_test.go).
var now = time.Now

const (
	// interestVectorDim must match tipcatalog.EmbeddingDim. Kept as a local
	// constant rather than an import of github.com/bluefunda/tipcatalog
	// because this phase is scoped to stay independent of the catalog client
	// (Phase 2); Phase 2/3 assert the two stay equal.
	interestVectorDim = 32

	// maxHistory caps how many invocations are retained; older entries are
	// dropped FIFO.
	maxHistory = 500

	// halfLife is the EWMA decay half-life for the interest vector.
	halfLife = 14 * 24 * time.Hour

	// nonZeroExitWeight is how much more an invocation that exited non-zero
	// weighs versus a clean exit — a failed command is the strongest
	// interest signal available.
	nonZeroExitWeight = 3.0
)

// Invocation is one recorded bai command run.
type Invocation struct {
	Command         string    `json:"command"`
	Flags           []string  `json:"flags"`
	ExitCode        int       `json:"exit_code"`
	RepoFingerprint string    `json:"repo_fingerprint,omitempty"`
	DurationBucket  string    `json:"duration_bucket"`
	Timestamp       time.Time `json:"timestamp"`
}

// State is the persisted signal store: recent invocation history plus the
// decayed interest vector derived from it.
type State struct {
	History        []Invocation               `json:"history"`
	InterestVector [interestVectorDim]float64 `json:"interest_vector"`
	LastDecay      time.Time                  `json:"last_decay"`
}

// DurationBucket classifies d into one of a small number of coarse buckets.
func DurationBucket(d time.Duration) string {
	switch {
	case d < time.Second:
		return "<1s"
	case d < 5*time.Second:
		return "1-5s"
	case d < 30*time.Second:
		return "5-30s"
	default:
		return ">30s"
	}
}

// FingerprintRepo hashes a git remote URL into a stable, non-reversible
// fingerprint. Callers look up the remote URL themselves (this package does
// not shell out) and pass the result to Record via Invocation.RepoFingerprint.
// Returns "" for an empty remoteURL.
func FingerprintRepo(remoteURL string) string {
	if remoteURL == "" {
		return ""
	}
	h := fnv.New64a()
	_, _ = h.Write([]byte(remoteURL))
	return fnvHex(h.Sum64())
}

func fnvHex(v uint64) string {
	const hex = "0123456789abcdef"
	b := make([]byte, 16)
	for i := 15; i >= 0; i-- {
		b[i] = hex[v&0xf]
		v >>= 4
	}
	return string(b)
}

// bucket deterministically maps key into a [0, interestVectorDim) index via
// feature hashing — a stand-in for a real embedding model (see the
// EmbeddingDim placeholder comment in tipcatalog).
func bucket(key string) int {
	h := fnv.New32a()
	_, _ = h.Write([]byte(key))
	return int(h.Sum32() % interestVectorDim)
}

// tipsDir returns ~/.bai/tips, creating it if needed. Shared by signal.go
// (signal.json) and catalog.go (catalog.json / catalog.json.sig).
func tipsDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, ".bai", "tips")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	return dir, nil
}

// statePath returns ~/.bai/tips/signal.json, creating the containing
// directory if needed.
func statePath() (string, error) {
	dir, err := tipsDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "signal.json"), nil
}

// Record loads the persisted state, applies decay, folds in inv, trims
// history to maxHistory, and atomically persists the result. Safe for
// concurrent use across processes via a file lock.
func Record(inv Invocation) error {
	path, err := statePath()
	if err != nil {
		return err
	}

	lock, err := lockFile(path + ".lock")
	if err != nil {
		return err
	}
	defer func() { _ = lock.Unlock() }()

	state, err := loadState(path)
	if err != nil {
		return err
	}

	t := now()
	state.decay(t)
	state.apply(inv)
	if len(state.History) > maxHistory {
		state.History = state.History[len(state.History)-maxHistory:]
	}
	state.LastDecay = t

	return saveState(path, state)
}

// Load returns the current persisted state without modifying it (no decay
// applied — callers that need an up-to-date interest vector should Record
// first, or apply decay themselves via the same formula).
func Load() (State, error) {
	path, err := statePath()
	if err != nil {
		return State{}, err
	}
	return loadState(path)
}

// decay applies EWMA half-life decay to the interest vector based on elapsed
// time since s.LastDecay. A zero LastDecay (fresh state) is treated as "no
// decay owed".
func (s *State) decay(at time.Time) {
	if s.LastDecay.IsZero() {
		return
	}
	elapsed := at.Sub(s.LastDecay)
	if elapsed <= 0 {
		return
	}
	factor := math.Pow(2, -float64(elapsed)/float64(halfLife))
	for i := range s.InterestVector {
		s.InterestVector[i] *= factor
	}
}

// apply folds inv into the interest vector (weighted 3x on non-zero exit)
// and appends it to history.
func (s *State) apply(inv Invocation) {
	weight := 1.0
	if inv.ExitCode != 0 {
		weight = nonZeroExitWeight
	}

	s.InterestVector[bucket("cmd:"+inv.Command)] += weight
	for _, f := range inv.Flags {
		s.InterestVector[bucket("flag:"+f)] += weight
	}

	s.History = append(s.History, inv)
}

func loadState(path string) (State, error) {
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return State{}, nil
	}
	if err != nil {
		return State{}, err
	}
	defer func() { _ = f.Close() }()

	var s State
	if err := json.NewDecoder(f).Decode(&s); err != nil {
		return State{}, err
	}
	return s, nil
}

// saveState writes state to path atomically, mirroring
// internal/session/store.go's Save.
func saveState(path string, state State) error {
	b, err := json.Marshal(state)
	if err != nil {
		return err
	}
	return atomicWriteFile(path, b)
}

// atomicWriteFile writes data to path via temp file + rename, so readers
// never observe a partial write. Shared by signal.go and catalog.go.
func atomicWriteFile(path string, data []byte) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".bai-tips-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()

	w := bufio.NewWriter(tmp)
	if _, err := w.Write(data); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return err
	}
	if err := w.Flush(); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return err
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return err
	}
	return os.Rename(tmpName, path)
}
