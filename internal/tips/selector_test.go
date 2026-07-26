package tips

import (
	"testing"
	"time"

	tipcatalog "github.com/bluefunda/tipcatalog"
)

func baseTip() tipcatalog.Tip {
	return tipcatalog.Tip{
		ID:       "t1",
		Family:   "fam",
		Surfaces: []string{tipcatalog.SurfaceCLI},
		Render:   tipcatalog.Render{CLI: &tipcatalog.RenderContent{Title: "T", Body: "B"}},
	}
}

func baseCtx() EligibilityContext {
	return EligibilityContext{Surface: tipcatalog.SurfaceCLI, Now: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}
}

func TestEligible_OKByDefault(t *testing.T) {
	if !Eligible(baseTip(), TipState{}, baseCtx()) {
		t.Fatal("expected a plain tip with no gates to be eligible")
	}
}

func TestEligible_SurfaceMismatch(t *testing.T) {
	ctx := baseCtx()
	ctx.Surface = tipcatalog.SurfaceIOS
	if Eligible(baseTip(), TipState{}, ctx) {
		t.Fatal("expected ineligible: tip does not declare the ios surface")
	}
}

func TestEligible_PersonaGate(t *testing.T) {
	tip := baseTip()
	tip.PersonaGate = "new_user"

	ctx := baseCtx() // Persona == ""
	if Eligible(tip, TipState{}, ctx) {
		t.Fatal("expected ineligible: persona gate set, context persona empty")
	}

	ctx.Persona = "new_user"
	if !Eligible(tip, TipState{}, ctx) {
		t.Fatal("expected eligible: persona matches gate")
	}

	ctx.Persona = "power_user"
	if Eligible(tip, TipState{}, ctx) {
		t.Fatal("expected ineligible: persona does not match gate")
	}
}

func TestEligible_TierGate(t *testing.T) {
	tip := baseTip()
	tip.MinTier = "pro"

	ctx := baseCtx() // Tier == ""
	if Eligible(tip, TipState{}, ctx) {
		t.Fatal("expected ineligible: pro-gated tip, empty tier context")
	}

	ctx.Tier = "free"
	if Eligible(tip, TipState{}, ctx) {
		t.Fatal("expected ineligible: free tier below pro gate")
	}

	ctx.Tier = "pro"
	if !Eligible(tip, TipState{}, ctx) {
		t.Fatal("expected eligible: tier meets gate")
	}

	ctx.Tier = "enterprise"
	if !Eligible(tip, TipState{}, ctx) {
		t.Fatal("expected eligible: tier exceeds gate")
	}
}

func TestEligible_Dismissal(t *testing.T) {
	ctx := baseCtx()
	state := TipState{DismissedUntil: ctx.Now.Add(time.Hour)}
	if Eligible(baseTip(), state, ctx) {
		t.Fatal("expected ineligible: still within dismissal window")
	}

	state.DismissedUntil = ctx.Now.Add(-time.Hour)
	if !Eligible(baseTip(), state, ctx) {
		t.Fatal("expected eligible: dismissal window has passed")
	}
}

func TestEligible_Cooldown(t *testing.T) {
	tip := baseTip()
	tip.Cooldown = "24h"
	ctx := baseCtx()

	state := TipState{LastShownAt: ctx.Now.Add(-1 * time.Hour)}
	if Eligible(tip, state, ctx) {
		t.Fatal("expected ineligible: shown 1h ago, cooldown is 24h")
	}

	state.LastShownAt = ctx.Now.Add(-25 * time.Hour)
	if !Eligible(tip, state, ctx) {
		t.Fatal("expected eligible: cooldown has elapsed")
	}
}

func TestEligible_CooldownDefaultsWhenUnset(t *testing.T) {
	tip := baseTip() // Cooldown == ""
	ctx := baseCtx()

	state := TipState{LastShownAt: ctx.Now.Add(-time.Minute)}
	if Eligible(tip, state, ctx) {
		t.Fatal("expected ineligible: shown 1 minute ago, default cooldown floor should apply")
	}

	state.LastShownAt = ctx.Now.Add(-2 * defaultCooldown)
	if !Eligible(tip, state, ctx) {
		t.Fatal("expected eligible: past the default cooldown floor")
	}
}

func TestCosineSimilarity_IdenticalVectorsIsOne(t *testing.T) {
	v := make([]float64, tipcatalog.EmbeddingDim)
	b := make([]float64, tipcatalog.EmbeddingDim)
	for i := range v {
		v[i] = float64(i + 1)
		b[i] = float64(i + 1)
	}
	got := cosineSimilarity(v, b)
	if got < 0.999 || got > 1.001 {
		t.Fatalf("expected ~1.0 for identical vectors, got %v", got)
	}
}

func TestCosineSimilarity_ZeroVectorIsZero(t *testing.T) {
	v := make([]float64, tipcatalog.EmbeddingDim)
	b := make([]float64, tipcatalog.EmbeddingDim)
	b[0] = 1
	if got := cosineSimilarity(v, b); got != 0 {
		t.Fatalf("expected 0 for a zero vector, got %v", got)
	}
}

func TestSelect_PicksHighestSimilarityAmongEligible(t *testing.T) {
	ctx := baseCtx()

	interest := make([]float64, tipcatalog.EmbeddingDim)
	interest[0] = 1

	near := baseTip()
	near.ID = "near"
	near.Embedding = make([]float64, tipcatalog.EmbeddingDim)
	near.Embedding[0] = 1

	far := baseTip()
	far.ID = "far"
	far.Embedding = make([]float64, tipcatalog.EmbeddingDim)
	far.Embedding[1] = 1

	gated := baseTip()
	gated.ID = "gated"
	gated.PersonaGate = "power_user"
	gated.Embedding = make([]float64, tipcatalog.EmbeddingDim)
	gated.Embedding[0] = 1

	selected, ok := Select([]tipcatalog.Tip{far, gated, near}, interest, nil, ctx, RulesRanker{})
	if !ok {
		t.Fatal("expected a tip to be selected")
	}
	if selected.ID != "near" {
		t.Fatalf("expected 'near' (highest similarity, ungated) to be selected, got %q", selected.ID)
	}
}

func TestSelect_NoneEligible(t *testing.T) {
	ctx := baseCtx()
	tip := baseTip()
	tip.PersonaGate = "power_user" // ctx.Persona is ""
	_, ok := Select([]tipcatalog.Tip{tip}, make([]float64, tipcatalog.EmbeddingDim), nil, ctx, RulesRanker{})
	if ok {
		t.Fatal("expected no tip selected when none are eligible")
	}
}

func TestRulesRanker_PersonaPriorBreaksTies(t *testing.T) {
	ctx := baseCtx()
	ctx.Persona = "new_user"

	matched := Candidate{Tip: tipcatalog.Tip{ID: "matched", PersonaGate: "new_user"}, Similarity: 0.1}
	unmatched := Candidate{Tip: tipcatalog.Tip{ID: "unmatched"}, Similarity: 0.2}

	ranked := RulesRanker{}.Rank([]Candidate{unmatched, matched}, ctx)
	if ranked[0].Tip.ID != "matched" {
		t.Fatalf("expected persona-matched candidate to rank first despite lower similarity, got %q", ranked[0].Tip.ID)
	}
}
