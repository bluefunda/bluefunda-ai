package tips

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"time"

	tipcatalog "github.com/bluefunda/tipcatalog"
)

const (
	// invocationBudget is the minimum number of invocations required
	// between two shown tips.
	invocationBudget = 20

	// dailyBudget is the maximum number of tips shown per calendar day
	// (UTC).
	dailyBudget = 2

	// retireAfterShows: a tip shown this many times with no detected
	// engagement is retired regardless of score. This repo has no
	// engagement-detection signal yet (that needs Phase 5's reward events
	// plus a structured "suggested command" field tips don't have today),
	// so every show currently counts as "no engagement" — the safe
	// direction to err in, since losing a tip's visibility early is much
	// cheaper than showing it forever.
	retireAfterShows = 3
)

// backoffSchedule is the dismissal backoff ladder for a tip family: 24h,
// then 72h, then 14d. A dismissal beyond the schedule's length is
// permanent (see farFuture).
var backoffSchedule = []time.Duration{24 * time.Hour, 72 * time.Hour, 14 * 24 * time.Hour}

// farFuture stands in for "permanent" so a dismissal past the backoff
// schedule reuses the same time-comparison logic as every other stage.
var farFuture = time.Date(9999, 1, 1, 0, 0, 0, 0, time.UTC)

type familyState struct {
	BackoffStage   int       `json:"backoff_stage"`
	DismissedUntil time.Time `json:"dismissed_until"`
}

type tipStat struct {
	ShownCount  int       `json:"shown_count"`
	LastShownAt time.Time `json:"last_shown_at"`
}

// AnnoyanceState is the persisted anti-annoyance bookkeeping: the on/off
// switch, global show-budget counters, per-family dismissal backoff, and
// per-tip show counts (for retirement).
type AnnoyanceState struct {
	Disabled    bool                   `json:"disabled"`
	Invocations int                    `json:"invocations_since_shown"`
	ShownToday  int                    `json:"shown_today"`
	ShownDay    string                 `json:"shown_day"` // YYYY-MM-DD, UTC
	Families    map[string]familyState `json:"families"`
	Tips        map[string]tipStat     `json:"tips"`
}

func annoyancePath() (string, error) {
	dir, err := tipsDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "annoyance.json"), nil
}

func loadAnnoyance() (AnnoyanceState, error) {
	path, err := annoyancePath()
	if err != nil {
		return AnnoyanceState{}, err
	}
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return AnnoyanceState{Families: map[string]familyState{}, Tips: map[string]tipStat{}}, nil
	}
	if err != nil {
		return AnnoyanceState{}, err
	}
	defer func() { _ = f.Close() }()

	var s AnnoyanceState
	if err := json.NewDecoder(f).Decode(&s); err != nil {
		return AnnoyanceState{}, err
	}
	if s.Families == nil {
		s.Families = map[string]familyState{}
	}
	if s.Tips == nil {
		s.Tips = map[string]tipStat{}
	}
	return s, nil
}

func saveAnnoyance(s AnnoyanceState) error {
	path, err := annoyancePath()
	if err != nil {
		return err
	}
	b, err := json.Marshal(s)
	if err != nil {
		return err
	}
	return atomicWriteFile(path, b)
}

// withAnnoyanceLock loads the persisted state, runs fn against it while
// holding a file lock, and atomically persists the result. fn should do no
// I/O of its own — keep the lock window short.
func withAnnoyanceLock(fn func(*AnnoyanceState)) error {
	path, err := annoyancePath()
	if err != nil {
		return err
	}
	lock, err := lockFile(path + ".lock")
	if err != nil {
		return err
	}
	defer func() { _ = lock.Unlock() }()

	s, err := loadAnnoyance()
	if err != nil {
		return err
	}
	fn(&s)
	return saveAnnoyance(s)
}

// Disable permanently turns off tips. Requires no further flags or
// confirmation.
func Disable() error {
	return withAnnoyanceLock(func(s *AnnoyanceState) { s.Disabled = true })
}

// Enable turns tips back on after Disable.
func Enable() error {
	return withAnnoyanceLock(func(s *AnnoyanceState) { s.Disabled = false })
}

// DismissFamily advances family's dismissal backoff stage and returns the
// window the family is now suppressed until.
func DismissFamily(family string) (time.Time, error) {
	var until time.Time
	err := withAnnoyanceLock(func(s *AnnoyanceState) {
		fs := s.Families[family]
		if fs.BackoffStage >= len(backoffSchedule) {
			until = farFuture
		} else {
			until = now().Add(backoffSchedule[fs.BackoffStage])
			fs.BackoffStage++
		}
		fs.DismissedUntil = until
		s.Families[family] = fs
	})
	return until, err
}

// dayKey returns t's UTC calendar day as "2006-01-02", for the daily
// shown-count rollover.
func dayKey(t time.Time) string {
	return t.UTC().Format("2006-01-02")
}

// budgetAllows reports whether the global anti-annoyance budget permits
// showing a tip right now.
func budgetAllows(s AnnoyanceState, at time.Time) bool {
	if s.Invocations < invocationBudget {
		return false
	}
	if s.ShownDay == dayKey(at) && s.ShownToday >= dailyBudget {
		return false
	}
	return true
}

// bumpInvocationCounter increments the invocation-since-last-shown counter
// and persists it immediately — every invocation counts toward the budget
// even when a tip ultimately isn't shown.
func bumpInvocationCounter() (AnnoyanceState, error) {
	var result AnnoyanceState
	err := withAnnoyanceLock(func(s *AnnoyanceState) {
		s.Invocations++
		result = *s
	})
	return result, err
}

// recordShown marks tipID as shown now: increments its show count, resets
// the invocation budget counter, and increments today's shown count
// (rolling over on day change).
func recordShown(tipID string) error {
	return withAnnoyanceLock(func(s *AnnoyanceState) {
		ts := s.Tips[tipID]
		ts.ShownCount++
		ts.LastShownAt = now()
		s.Tips[tipID] = ts

		s.Invocations = 0

		day := dayKey(now())
		if s.ShownDay != day {
			s.ShownDay = day
			s.ShownToday = 0
		}
		s.ShownToday++
	})
}

// isRetired reports whether tipID has been shown retireAfterShows times.
func isRetired(s AnnoyanceState, tipID string) bool {
	return s.Tips[tipID].ShownCount >= retireAfterShows
}

// filterRetired removes retired tips from tips.
func filterRetired(tips []tipcatalog.Tip, s AnnoyanceState) []tipcatalog.Tip {
	out := make([]tipcatalog.Tip, 0, len(tips))
	for _, t := range tips {
		if !isRetired(s, t.ID) {
			out = append(out, t)
		}
	}
	return out
}

// tipStatesFrom builds the per-tip TipState map Eligible reads: cooldown
// from the tip's own last-shown time, dismissal from its family's backoff
// window.
func tipStatesFrom(s AnnoyanceState, tips []tipcatalog.Tip) map[string]TipState {
	states := make(map[string]TipState, len(tips))
	for _, t := range tips {
		var st TipState
		if ts, ok := s.Tips[t.ID]; ok {
			st.LastShownAt = ts.LastShownAt
		}
		if fs, ok := s.Families[t.Family]; ok {
			st.DismissedUntil = fs.DismissedUntil
		}
		states[t.ID] = st
	}
	return states
}

// ListedTip is one candidate as `bai tips list` reports it.
type ListedTip struct {
	ID         string
	Family     string
	Similarity float64
	Eligible   bool
}

// ListEligible returns every CLI tip ranked by similarity, each flagged
// with whether it's currently eligible — ignoring the global
// invocation/daily budget, but respecting per-tip cooldown, family
// dismissal, persona/tier/surface gates, and retirement. This is "what
// would have been shown" for `bai tips list`.
func ListEligible() ([]ListedTip, error) {
	ann, err := loadAnnoyance()
	if err != nil {
		return nil, err
	}
	catalog, err := CLITips()
	if err != nil {
		return nil, err
	}
	state, err := Load()
	if err != nil {
		return nil, err
	}

	ctx := EligibilityContext{Surface: tipcatalog.SurfaceCLI, Persona: personaFromHistory(state), Now: now()}
	states := tipStatesFrom(ann, catalog)

	out := make([]ListedTip, 0, len(catalog))
	for _, t := range catalog {
		out = append(out, ListedTip{
			ID:         t.ID,
			Family:     t.Family,
			Similarity: cosineSimilarity(state.InterestVector, t.Embedding),
			Eligible:   !isRetired(ann, t.ID) && Eligible(t, states[t.ID], ctx),
		})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Similarity > out[j].Similarity })
	return out, nil
}
