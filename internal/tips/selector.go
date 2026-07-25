package tips

import (
	"math"
	"sort"
	"strings"
	"time"

	tipcatalog "github.com/bluefunda/tipcatalog"
)

// defaultCooldown applies when a tip declares no cooldown (or an
// unparseable one), as a safety floor against showing the same tip on
// consecutive invocations.
const defaultCooldown = time.Hour

// personaPriorBoost is added to a candidate's similarity score when its
// PersonaGate matches the caller's persona — a cold-start prior so a new
// user sees persona-appropriate tips even before enough signal has
// accumulated to differentiate by embedding similarity alone.
const personaPriorBoost = 0.25

// tierRank orders account tiers for MinTier gating. Provisional: this repo
// has no cached local notion of the user's actual subscription tier yet
// (billing.go's plan data requires a network call, which the tip hot path
// must not make) — callers should pass EligibilityContext.Tier "" until
// that's wired up, which this ranks the same as "free".
var tierRank = map[string]int{
	"":           0,
	"free":       0,
	"pro":        1,
	"enterprise": 2,
}

// TipState is the per-tip bookkeeping Eligibility reads. Phase 4 owns
// writing real cooldown/dismissal data; until then callers pass the zero
// value (never shown, never dismissed), which is always eligible on those
// two axes.
type TipState struct {
	LastShownAt    time.Time
	DismissedUntil time.Time
}

// EligibilityContext is caller-supplied context Eligibility checks each tip
// against. Zero values (Tier="", Persona="") are the safe defaults: only
// ungated tips are eligible until real tier/persona sourcing exists.
type EligibilityContext struct {
	Tier    string
	Persona string
	Surface string
	Now     time.Time
}

// Eligible reports whether tip may be shown given state and ctx. Pure
// function, no I/O: tier, cooldown, dismissal, persona gate, and surface
// match are all evaluated from the passed-in values only.
func Eligible(tip tipcatalog.Tip, state TipState, ctx EligibilityContext) bool {
	if !tierMeets(tip.MinTier, ctx.Tier) {
		return false
	}
	if tip.PersonaGate != "" && tip.PersonaGate != ctx.Persona {
		return false
	}
	if !hasSurface(tip, ctx.Surface) {
		return false
	}
	if ctx.Now.Before(state.DismissedUntil) {
		return false
	}
	if !state.LastShownAt.IsZero() && ctx.Now.Sub(state.LastShownAt) < cooldown(tip) {
		return false
	}
	return true
}

func tierMeets(minTier, tier string) bool {
	if minTier == "" {
		return true
	}
	return tierRank[strings.ToLower(tier)] >= tierRank[strings.ToLower(minTier)]
}

func cooldown(tip tipcatalog.Tip) time.Duration {
	if tip.Cooldown == "" {
		return defaultCooldown
	}
	d, err := time.ParseDuration(tip.Cooldown)
	if err != nil || d <= 0 {
		return defaultCooldown
	}
	return d
}

// Candidate is an eligible tip scored against the caller's interest vector.
type Candidate struct {
	Tip        tipcatalog.Tip
	Similarity float64
}

// Ranker orders scored candidates, most-preferred first. RulesRanker is the
// only implementation today; the interface exists so a learned ranker
// (e.g. Thompson sampling) can drop in later without callers changing.
type Ranker interface {
	Rank(candidates []Candidate, ctx EligibilityContext) []Candidate
}

// RulesRanker sorts by cosine similarity, applying a cold-start persona
// prior boost.
type RulesRanker struct{}

func (RulesRanker) Rank(candidates []Candidate, ctx EligibilityContext) []Candidate {
	ranked := make([]Candidate, len(candidates))
	copy(ranked, candidates)

	score := func(c Candidate) float64 {
		s := c.Similarity
		if ctx.Persona != "" && c.Tip.PersonaGate == ctx.Persona {
			s += personaPriorBoost
		}
		return s
	}

	sort.SliceStable(ranked, func(i, j int) bool {
		return score(ranked[i]) > score(ranked[j])
	})
	return ranked
}

// Select runs the full eligibility -> candidates -> rank pipeline and
// returns the top tip, if any is eligible.
func Select(tips []tipcatalog.Tip, interest [interestVectorDim]float64, states map[string]TipState, ctx EligibilityContext, ranker Ranker) (tipcatalog.Tip, bool) {
	var eligible []tipcatalog.Tip
	for _, t := range tips {
		if Eligible(t, states[t.ID], ctx) {
			eligible = append(eligible, t)
		}
	}
	if len(eligible) == 0 {
		return tipcatalog.Tip{}, false
	}

	candidates := make([]Candidate, len(eligible))
	for i, t := range eligible {
		candidates[i] = Candidate{Tip: t, Similarity: cosineSimilarity(interest, t.Embedding)}
	}

	ranked := ranker.Rank(candidates, ctx)
	if len(ranked) == 0 {
		return tipcatalog.Tip{}, false
	}
	return ranked[0].Tip, true
}

func hasSurface(tip tipcatalog.Tip, surface string) bool {
	for _, s := range tip.Surfaces {
		if s == surface {
			return true
		}
	}
	return false
}

// cosineSimilarity returns the cosine similarity between a and b, or 0 if
// their lengths differ or either is a zero vector.
func cosineSimilarity(a [interestVectorDim]float64, b []float64) float64 {
	if len(b) != len(a) {
		return 0
	}
	var dot, na, nb float64
	for i := range a {
		dot += a[i] * b[i]
		na += a[i] * a[i]
		nb += b[i] * b[i]
	}
	if na == 0 || nb == 0 {
		return 0
	}
	return dot / (math.Sqrt(na) * math.Sqrt(nb))
}
