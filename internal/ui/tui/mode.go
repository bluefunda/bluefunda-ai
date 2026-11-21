package tui

// permMode is the session's permission mode. Shift+Tab cycles default →
// accept edits → plan → default, as in Claude Code; auto (/auto or --auto)
// is left by the next Shift+Tab.
type permMode int

const (
	modeDefault permMode = iota
	modeAcceptEdits
	modePlan
	modeAuto
)

func (m Model) mode() permMode {
	switch {
	case m.cfg.PlanMode:
		return modePlan
	case m.cfg.AutoApply:
		return modeAuto
	case m.cfg.AcceptEdits:
		return modeAcceptEdits
	}
	return modeDefault
}

func (p permMode) next() permMode {
	switch p {
	case modeDefault:
		return modeAcceptEdits
	case modeAcceptEdits:
		return modePlan
	}
	return modeDefault
}

// label is the indicator under the input; "" for the default mode.
func (p permMode) label() string {
	switch p {
	case modeAcceptEdits:
		return "⏵⏵ accept edits on"
	case modePlan:
		return "⏸ plan mode on"
	case modeAuto:
		return "⏵⏵⏵ auto: tools run without asking"
	}
	return ""
}

func (p permMode) describe() string {
	switch p {
	case modeAcceptEdits:
		return "accept edits (file edits run without asking)"
	case modePlan:
		return "plan (read-only)"
	case modeAuto:
		return "auto (runs tools without asking)"
	}
	return "default (asks before changes)"
}

// setMode switches to p, telling the session about every flag that changed.
func (m *Model) setMode(p permMode) {
	set := func(cur *bool, want bool, apply func(bool)) {
		if *cur != want {
			*cur = want
			if apply != nil {
				apply(want)
			}
		}
	}
	set(&m.cfg.PlanMode, p == modePlan, m.cfg.SetPlanModeFn)
	set(&m.cfg.AutoApply, p == modeAuto, m.cfg.SetAutoApplyFn)
	set(&m.cfg.AcceptEdits, p == modeAcceptEdits, m.cfg.SetAcceptEditsFn)
}
