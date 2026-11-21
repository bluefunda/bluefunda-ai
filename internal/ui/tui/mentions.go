package tui

import (
	"path"
	"sort"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// The @ picker: while the word being typed starts with "@", matching files
// and directories are listed above the input, as in Claude Code. Tab or
// Enter puts the chosen path in; the session attaches it on send.

// maxMentionMatches caps the picker's list.
const maxMentionMatches = 50

// currentMention returns the query after "@" when the last word of the
// input is a mention being typed.
func currentMention(val string) (string, bool) {
	i := strings.LastIndexAny(val, " \t\n")
	word := val[i+1:]
	if q, ok := strings.CutPrefix(word, "@"); ok {
		return q, true
	}
	return "", false
}

// matchMentions ranks paths for query q: file name starts with q, then path
// starts with q, then name contains q, then path contains q; shorter first.
func matchMentions(paths []string, q string) []string {
	q = strings.ToLower(q)
	type hit struct {
		p    string
		rank int
	}
	var hits []hit
	for _, p := range paths {
		lp := strings.ToLower(p)
		base := path.Base(strings.TrimSuffix(lp, "/"))
		rank := -1
		switch {
		case q == "" || strings.HasPrefix(base, q):
			rank = 0
		case strings.HasPrefix(lp, q):
			rank = 1
		case strings.Contains(base, q):
			rank = 2
		case strings.Contains(lp, q):
			rank = 3
		}
		if rank >= 0 {
			hits = append(hits, hit{p, rank})
		}
	}
	sort.SliceStable(hits, func(i, j int) bool {
		if hits[i].rank != hits[j].rank {
			return hits[i].rank < hits[j].rank
		}
		return len(hits[i].p) < len(hits[j].p)
	})
	out := make([]string, 0, min(len(hits), maxMentionMatches))
	for _, h := range hits[:min(len(hits), maxMentionMatches)] {
		out = append(out, h.p)
	}
	return out
}

// updateMentionMenu refreshes the picker after the input changes. The file
// list is loaded on the first "@" and kept for the session.
func (m *Model) updateMentionMenu() {
	q, ok := currentMention(m.textarea.Value())
	if !ok || m.cfg.FilesFn == nil {
		m.showMention = false
		return
	}
	if m.mentionFiles == nil {
		m.mentionFiles = m.cfg.FilesFn()
		if m.mentionFiles == nil {
			m.mentionFiles = []string{}
		}
	}
	m.mentionMatches = matchMentions(m.mentionFiles, q)
	m.showMention = len(m.mentionMatches) > 0
	m.showSlash = m.showSlash && !m.showMention
	if m.mentionIdx >= len(m.mentionMatches) {
		m.mentionIdx = 0
	}
}

// acceptMention replaces the mention being typed with the chosen path. A
// directory keeps the picker open on its contents.
func (m *Model) acceptMention() {
	if len(m.mentionMatches) == 0 {
		return
	}
	choice := m.mentionMatches[m.mentionIdx]
	val := m.textarea.Value()
	i := strings.LastIndexAny(val, " \t\n")
	val = val[:i+1] + "@" + choice
	if !strings.HasSuffix(choice, "/") {
		val += " "
	}
	m.textarea.SetValue(val)
	m.textarea.CursorEnd()
	m.mentionIdx = 0
	m.updateMentionMenu()
}

func (m Model) renderMentionMenu() string {
	th := m.theme
	const visible = 8
	start := max(0, min(m.mentionIdx-2, len(m.mentionMatches)-visible))
	end := min(start+visible, len(m.mentionMatches))
	var rows []string
	for i := start; i < end; i++ {
		line := "  " + lipgloss.NewStyle().Foreground(th.AccentBold).Render("@"+m.mentionMatches[i])
		if i == m.mentionIdx {
			line = th.SlashSelected.Width(m.width - 6).Render(line)
		}
		rows = append(rows, line)
	}
	return th.SlashMenu.Width(m.width - 4).Render(strings.Join(rows, "\n"))
}
