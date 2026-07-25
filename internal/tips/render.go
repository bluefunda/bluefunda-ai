package tips

import (
	"fmt"
	"os"

	tipcatalog "github.com/bluefunda/tipcatalog"
)

// newUserHistoryThreshold: below this many recorded invocations, the
// persona "new_user" is inferred from Phase 1's own history — a lightweight
// substitute for a real persona-detection system, which doesn't exist yet.
const newUserHistoryThreshold = 10

// MaybeShowTip prints one tip to stderr if conditions allow it: tips
// aren't disabled (bai tips off), not silenced, the anti-annoyance budget
// (at most 1 per 20 invocations, 2 per day) permits it, and at least one
// non-retired tip is eligible. Also kicks off a non-blocking catalog
// refresh. quiet should reflect the caller's own quiet/output-format flag.
//
// Every invocation counts toward the budget, shown or not, so the
// bookkeeping is updated (via bumpInvocationCounter) before any of the
// early-return checks below.
func MaybeShowTip(quiet bool) {
	EnsureFresh()

	ann, err := bumpInvocationCounter()
	if err != nil {
		return
	}
	if ann.Disabled || ShouldSilence(quiet) {
		return
	}
	if !budgetAllows(ann, now()) {
		return
	}

	catalog, err := CLITips()
	if err != nil || len(catalog) == 0 {
		return
	}
	catalog = filterRetired(catalog, ann)
	if len(catalog) == 0 {
		return
	}

	state, err := Load()
	if err != nil {
		return
	}

	ctx := EligibilityContext{
		Surface: tipcatalog.SurfaceCLI,
		Persona: personaFromHistory(state),
		Now:     now(),
	}
	states := tipStatesFrom(ann, catalog)

	selected, ok := Select(catalog, state.InterestVector, states, ctx, RulesRanker{})
	if !ok {
		return
	}

	render := selected.Render.CLI
	if render == nil {
		return
	}
	fmt.Fprintln(os.Stderr)
	fmt.Fprintln(os.Stderr, "Tip: "+render.Body)

	_ = recordShown(selected.ID)
}

func personaFromHistory(state State) string {
	if len(state.History) < newUserHistoryThreshold {
		return "new_user"
	}
	return ""
}
