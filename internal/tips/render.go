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

// MaybeShowTip prints one tip to stderr if conditions allow it (not
// silenced, at least one tip eligible) and kicks off a non-blocking catalog
// refresh. quiet should reflect the caller's own quiet/output-format flag.
func MaybeShowTip(quiet bool) {
	EnsureFresh()

	if ShouldSilence(quiet) {
		return
	}

	catalog, err := CLITips()
	if err != nil || len(catalog) == 0 {
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

	// Phase 4 owns persisted per-tip cooldown/dismissal bookkeeping; a nil
	// states map reads as the zero TipState for every tip (never shown,
	// never dismissed), which is the correct default until that lands.
	selected, ok := Select(catalog, state.InterestVector, nil, ctx, RulesRanker{})
	if !ok {
		return
	}

	render := selected.Render.CLI
	if render == nil {
		return
	}
	fmt.Fprintln(os.Stderr)
	fmt.Fprintln(os.Stderr, "Tip: "+render.Body)
}

func personaFromHistory(state State) string {
	if len(state.History) < newUserHistoryThreshold {
		return "new_user"
	}
	return ""
}
