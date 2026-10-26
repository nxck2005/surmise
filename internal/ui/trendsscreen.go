package ui

import (
	"fmt"
	"slices"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/nxck2005/surmise/internal/game"
	"github.com/nxck2005/surmise/internal/stats"
	"github.com/nxck2005/surmise/internal/words"
)

// trendsScreen is one mode's recent form, personal bests and opening words.
type trendsScreen struct {
	games  []*game.Game // the shared history: read, never changed
	length int
	trend  stats.Trend
	height int
}

func (m *trendsScreen) resize(h int) { m.height = h }

// reload shows the trends for a mode, from a history the caller has read.
func (m *trendsScreen) reload(games []*game.Game, length int) {
	m.games = games
	m.length = length
	m.trend = stats.Trends(games, length)
}

// step moves to the previous or next mode. The ends do not wrap.
func (m *trendsScreen) step(delta int) {
	i := slices.Index(words.Lengths, m.length) + delta
	if i < 0 || i >= len(words.Lengths) {
		return
	}
	m.reload(m.games, words.Lengths[i])
}

func (m *trendsScreen) update(msg tea.KeyPressMsg) (back bool) {
	switch msg.String() {
	case "esc", "q":
		return true
	case "left", "h":
		m.step(-1)
	case "right", "l":
		m.step(1)
	}
	return false
}

func (m *trendsScreen) view(h *hitMap) string {
	// The mode line. An end of the range loses its arrow and its target, but
	// keeps the width, so the label does not move.
	arrow := func(a action, glyph string, present bool) string {
		if !present {
			return strings.Repeat(" ", lipgloss.Width(glyph))
		}
		style := st.accent
		if h.hovered(a) {
			style = st.hover(st.accent)
		}
		return h.mark(a, style.Render(glyph))
	}
	modeLine := arrow(action{kind: actTrendsPrev}, st.glyph.ValuePrev, m.length != words.Lengths[0]) +
		" " + st.text.Render(fmt.Sprintf("%d letters", m.length)) + " " +
		arrow(action{kind: actTrendsNext}, st.glyph.ValueNext, m.length != words.Lengths[len(words.Lengths)-1])

	if m.trend.AllTime.Games == 0 {
		return block(lipgloss.JoinVertical(lipgloss.Left, modeLine, "",
			st.muted.Render(fmt.Sprintf("no finished %d-letter games yet", m.length))))
	}

	row := func(label, recent, all string) string {
		return st.muted.Render(fmt.Sprintf("%-14s", label)) +
			st.text.Render(fmt.Sprintf("%10s%10s", recent, all))
	}
	f := strings.Join([]string{
		st.muted.Render(fmt.Sprintf("%-14s%10s%10s", "", fmt.Sprintf("last %d", m.trend.Recent.Games), "all time")),
		row("win rate", formatPercent(m.trend.Recent.WinRate), formatPercent(m.trend.AllTime.WinRate)),
		row("avg guesses", formatFloat(m.trend.Recent.AvgAttempts), formatFloat(m.trend.AllTime.AvgAttempts)),
		row("avg time", formatDuration(m.trend.Recent.AvgTime), formatDuration(m.trend.AllTime.AvgTime)),
	}, "\n")

	var bests string
	if m.trend.Fewest != nil {
		lines := []string{st.muted.Render("personal bests")}
		if b := m.trend.Fastest; b != nil {
			lines = append(lines, st.muted.Render(fmt.Sprintf("%-14s", "fastest win"))+
				st.text.Render(fmt.Sprintf("%10s", formatDuration(b.Elapsed)))+
				st.muted.Render(fmt.Sprintf("%10s", "#"+game.Code(b.ID))))
		}
		b := m.trend.Fewest
		lines = append(lines, st.muted.Render(fmt.Sprintf("%-14s", "fewest guesses"))+
			st.text.Render(fmt.Sprintf("%10s", fmt.Sprint(b.Attempts)))+
			st.muted.Render(fmt.Sprintf("%10s", "#"+game.Code(b.ID))))
		bests = strings.Join(lines, "\n")
	}

	var openers string
	if len(m.trend.Openers) > 0 {
		lines := []string{st.muted.Render(fmt.Sprintf("%-14s%10s%10s%10s", "top openers", "games", "won", "avg"))}
		for _, o := range m.trend.Openers {
			lines = append(lines, st.text.Render(fmt.Sprintf("%-14s%10d", strings.ToUpper(o.Word), o.Games))+
				st.muted.Render(fmt.Sprintf("%10s%10s", formatPercent(o.WinRate), formatFloat(o.AvgAttempts))))
		}
		openers = strings.Join(lines, "\n")
	}

	// Openers are given up before bests: the list is a prefix of what fits.
	committed := []string{modeLine, "", f}
	optional := []string{bests, openers}
	sections := committed
	for _, extra := range affordableSections(committed, optional, bodyBudget(m.height)) {
		if extra != "" {
			sections = append(sections, "", extra)
		}
	}
	return block(lipgloss.JoinVertical(lipgloss.Left, sections...))
}

func (m *trendsScreen) help(h *hitMap) string {
	return renderHelp(h,
		helpItem{keys: "←/→", label: "mode"},
		helpItem{keys: "esc", label: "profile", act: action{kind: actBack}},
	)
}
