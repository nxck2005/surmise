package ui

import (
	"fmt"
	"sort"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/nxck2005/wortle/internal/stats"
	"github.com/nxck2005/wortle/internal/store"
	"github.com/nxck2005/wortle/internal/words"
)

// distributionWidth is the character width of the longest histogram bar.
const distributionWidth = 24

// profileScreen shows aggregate performance, in the spirit of a typing test's
// account page.
type profileScreen struct {
	summary stats.Summary
	err     error
}

func (m *profileScreen) reload(s store.Store) {
	games, err := s.All()
	if err != nil {
		m.err = err
		return
	}
	m.err = nil
	m.summary = stats.Compute(games)
}

func (m *profileScreen) view(h *hitMap) string {
	if m.err != nil {
		return titled("profile",
			st.err.Render(fmt.Sprintf("could not read puzzles: %v", m.err)))
	}

	s := m.summary
	if s.Played == 0 && s.InPlay == 0 {
		return titled("profile", st.muted.Render("no games played yet"))
	}

	// The sections stay left-joined: these are label-over-value columns and a
	// histogram, which only read if they share a left edge. titled centres the
	// finished block.
	var sections []string
	sections = append(sections,
		renderStatRow([]stat{
			{"played", fmt.Sprint(s.Played)},
			{"won", fmt.Sprint(s.Won)},
			{"win rate", formatPercent(s.WinRate)},
		}),
		"",
		renderStatRow([]stat{
			{"avg attempts", formatFloat(s.AvgAttempts)},
			{"avg time", formatDuration(s.AvgTime)},
			{"streak", fmt.Sprintf("%d (max %d)", s.CurrentStreak, s.MaxStreak)},
		}),
	)

	if s.InPlay > 0 {
		sections = append(sections, "",
			st.muted.Render(fmt.Sprintf("%d puzzle(s) still in play", s.InPlay)))
	}

	sections = append(sections, "", m.renderDistribution())

	if byMode := m.renderByMode(); byMode != "" {
		sections = append(sections, "", byMode)
	}
	return titled("profile", lipgloss.JoinVertical(lipgloss.Left, sections...))
}

type stat struct{ label, value string }

// renderStatRow lays out stats as label-over-value blocks.
func renderStatRow(stats []stat) string {
	blocks := make([]string, len(stats))
	for i, s := range stats {
		blocks[i] = lipgloss.NewStyle().Width(20).Render(
			lipgloss.JoinVertical(lipgloss.Left,
				st.muted.Render(s.label),
				st.accent.Bold(true).Render(s.value),
			),
		)
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, blocks...)
}

// renderDistribution draws the guess histogram: how often the player wins in
// each number of attempts.
func (m *profileScreen) renderDistribution() string {
	dist := m.summary.Distribution
	if len(dist) == 0 {
		return st.muted.Render("no solved puzzles yet")
	}

	attempts := make([]int, 0, len(dist))
	peak := 0
	for a, n := range dist {
		attempts = append(attempts, a)
		if n > peak {
			peak = n
		}
	}
	sort.Ints(attempts)

	var b strings.Builder
	b.WriteString(st.muted.Render("guess distribution"))
	b.WriteString("\n")
	for _, a := range attempts {
		n := dist[a]
		width := max(n*distributionWidth/peak, 1)
		bar := st.bar.Render(strings.Repeat(st.glyph.Bar, width))
		fmt.Fprintf(&b, "%s %s %s\n",
			st.muted.Render(fmt.Sprintf("%2d", a)), bar, st.text.Render(fmt.Sprint(n)))
	}
	return strings.TrimRight(b.String(), "\n")
}

// renderByMode breaks the headline figures down per word length, so the modes
// can be compared.
func (m *profileScreen) renderByMode() string {
	var rows []string
	for _, n := range words.Lengths {
		mode, ok := m.summary.ByLength[n]
		if !ok || mode.Played == 0 {
			continue
		}
		rows = append(rows, fmt.Sprintf("%s  %s  %s  %s",
			st.text.Render(fmt.Sprintf("%d letters", n)),
			st.muted.Render(fmt.Sprintf("%-12s", fmt.Sprintf("%d played", mode.Played))),
			st.muted.Render(fmt.Sprintf("%-10s", formatPercent(mode.WinRate))),
			st.muted.Render(fmt.Sprintf("avg %s in %s",
				formatFloat(mode.AvgAttempts), formatDuration(mode.AvgTime))),
		))
	}
	if len(rows) == 0 {
		return ""
	}
	return st.muted.Render("by mode") + "\n" + strings.Join(rows, "\n")
}

func (m *profileScreen) help(h *hitMap) string {
	return renderHelp(h, helpItem{keys: "esc", label: "menu", act: action{kind: actBack}})
}
