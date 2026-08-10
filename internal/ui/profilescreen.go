package ui

import (
	"fmt"
	"sort"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/nxck2005/surmise/internal/daily"
	"github.com/nxck2005/surmise/internal/stats"
	"github.com/nxck2005/surmise/internal/store"
	"github.com/nxck2005/surmise/internal/words"
)

// distributionWidth is the character width of the longest histogram bar.
const distributionWidth = 24

// profileScreen shows aggregate performance, in the spirit of a typing test's
// account page.
type profileScreen struct {
	summary     stats.Summary
	err         error
	displayName string

	// width and height are the terminal's, pushed down by the root so the
	// screen can shed what it cannot afford to draw. Zero means unmeasured,
	// which counts as unbounded.
	width, height int
}

func (m *profileScreen) resize(w, h int) { m.width, m.height = w, h }

// reload recomputes the profile. The day comes from the root rather than from
// the clock so that -day moves the daily streak the same way it moves the
// board: the screen and the puzzle it describes agree on what today is. The
// display name is a local setting supplied by the root, not identity attached
// to any game.
func (m *profileScreen) reload(s store.Store, today daily.Day, displayName string) {
	m.displayName = sanitizeDisplayName(displayName)
	games, err := s.All()
	if err != nil {
		m.err = err
		return
	}
	m.err = nil
	m.summary = stats.ComputeAt(games, today)
}

func (m *profileScreen) view(h *hitMap) string {
	if m.err != nil {
		return titled(m.title(),
			st.err.Render(fmt.Sprintf("could not read puzzles: %v", m.err)))
	}

	s := m.summary
	if s.Played == 0 && s.InPlay == 0 {
		return titled(m.title(), st.muted.Render("no games played yet"))
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

	// The rest are extras, dropped from the bottom up when the terminal cannot
	// afford them — the same bargain the board's colour legend makes. The
	// headline rows above never drop: they are what the screen is for.
	//
	// The order is the order they are given up in, reversed: the daily table
	// goes first because the daily screen shows the day's state anyway, then
	// the per-mode table, and the histogram last of the three.
	optional := []string{m.renderDistribution(), m.renderByMode(), m.renderDaily()}
	for _, extra := range m.affordable(sections, optional) {
		if extra != "" {
			sections = append(sections, "", extra)
		}
	}
	return titled(m.title(), lipgloss.JoinVertical(lipgloss.Left, sections...))
}

func (m *profileScreen) title() string {
	if m.displayName != "" {
		return m.displayName
	}
	return "profile"
}

// affordable returns the leading run of extras that fits under the terminal's
// budget, given what is already committed to. An unmeasured height is
// unbounded, which is what keeps the headless tests drawing the whole screen.
func (m *profileScreen) affordable(committed, optional []string) []string {
	budget := bodyBudget(m.height)
	if budget <= 0 {
		return optional
	}
	used := 0
	for _, s := range committed {
		used += lipgloss.Height(s)
	}
	for i, extra := range optional {
		if extra == "" {
			continue
		}
		// Each extra costs its own height plus the blank line spacing it off.
		if used+lipgloss.Height(extra)+1 > budget {
			return optional[:i]
		}
		used += lipgloss.Height(extra) + 1
	}
	return optional
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

// renderDaily is the by-mode table for the daily puzzles, whose streak is the
// figure the section exists for: it counts consecutive days rather than
// consecutive wins, so it is not the streak above and must not be read as it.
// The columns line up with renderByMode's on purpose.
func (m *profileScreen) renderDaily() string {
	var rows []string
	for _, n := range words.Lengths {
		mode, ok := m.summary.Daily[n]
		if !ok || mode.Played == 0 {
			continue
		}
		rows = append(rows, fmt.Sprintf("%s  %s  %s  %s",
			st.text.Render(fmt.Sprintf("%d letters", n)),
			st.muted.Render(fmt.Sprintf("%-12s", fmt.Sprintf("%d played", mode.Played))),
			st.muted.Render(fmt.Sprintf("%-10s", formatPercent(mode.WinRate))),
			st.muted.Render(fmt.Sprintf("streak %d (max %d)",
				mode.CurrentStreak, mode.MaxStreak)),
		))
	}
	if len(rows) == 0 {
		return ""
	}
	return st.muted.Render("daily") + "\n" + strings.Join(rows, "\n")
}

func (m *profileScreen) help(h *hitMap) string {
	return renderHelp(h, helpItem{keys: "esc", label: "menu", act: action{kind: actBack}})
}
