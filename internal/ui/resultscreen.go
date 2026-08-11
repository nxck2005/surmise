package ui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/nxck2005/surmise/internal/brand"
	"github.com/nxck2005/surmise/internal/game"
)

// resultScreen is the payoff after a puzzle ends. It keeps the finished game by
// pointer so review and result are two views of exactly the same saved state.
type resultScreen struct {
	g             *game.Game
	notice        string
	copyRequested bool
}

func (m *resultScreen) open(g *game.Game, notice string) {
	m.g = g
	m.notice = notice
	m.copyRequested = false
}

func (m *resultScreen) view(_ *hitMap) string {
	g := m.g

	outcome := st.accent.Render(fmt.Sprintf("solved in %d", g.Attempts()))
	if g.Status == game.Lost {
		outcome = st.err.Render("not this time")
	}

	what := fmt.Sprintf("%d letters", g.Length)
	if g.Daily != "" {
		what = fmt.Sprintf("daily %s · %s", g.Daily, what)
	}
	meta := st.muted.Render(fmt.Sprintf("%s · %s · %s",
		what, resultAttempts(g), formatDuration(g.Elapsed())))

	rows := make([]string, len(g.Guesses))
	for i, guess := range g.Guesses {
		rows[i] = renderScoredRow(guess, g.Marks[i])
	}

	sections := []string{
		outcome,
		st.title.Render(fmt.Sprintf("%s #%s", brand.Name, game.Code(g.ID))),
		meta,
		"",
		stackSpaced(rows),
	}
	if g.Status == game.Lost {
		sections = append(sections, "",
			st.muted.Render("answer ")+st.text.Render(strings.ToUpper(g.Answer)))
	}
	sections = append(sections, "", renderLegend())

	switch {
	case m.notice != "":
		sections = append(sections, "", st.err.Render(m.notice))
	case m.copyRequested:
		sections = append(sections, "", st.muted.Render("copy requested"))
	}
	return lipgloss.JoinVertical(lipgloss.Center, sections...)
}

func (m *resultScreen) help(h *hitMap) string {
	return renderHelp(h,
		helpItem{keys: "enter/r", label: "review", act: action{kind: actResultReview}},
		helpItem{keys: "n", label: m.nextLabel(), act: action{kind: actResultNext}},
		helpItem{keys: "c", label: "copy", act: action{kind: actResultCopy}},
		helpItem{keys: "esc", label: "menu", act: action{kind: actBack}},
	)
}

func (m *resultScreen) nextLabel() string {
	if m.g.Daily != "" {
		return "daily"
	}
	return "next"
}

func resultAttempts(g *game.Game) string {
	if g.Status == game.Lost {
		return fmt.Sprintf("X/%d", g.MaxAttempts)
	}
	return fmt.Sprintf("%d/%d", g.Attempts(), g.MaxAttempts)
}

// shareResult renders only public outcome data and scoring marks. In particular
// it carries neither the answer nor the player's guesses, so it is safe to paste
// where somebody else may still want to play the puzzle.
func shareResult(g *game.Game) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s #%s %s\n", brand.Name, game.Code(g.ID), resultAttempts(g))
	if g.Daily != "" {
		fmt.Fprintf(&b, "daily %s · %d letters · %s\n",
			g.Daily, g.Length, formatDuration(g.Elapsed()))
	} else {
		fmt.Fprintf(&b, "%d letters · %s\n", g.Length, formatDuration(g.Elapsed()))
	}

	for _, marks := range g.Marks {
		for _, mark := range marks {
			switch mark {
			case game.Correct:
				b.WriteRune('■')
			case game.Present:
				b.WriteRune('□')
			default:
				b.WriteRune('·')
			}
		}
		b.WriteByte('\n')
	}
	b.WriteString("■ correct  □ present  · absent")
	return b.String()
}

func (m *Model) reviewResult() tea.Cmd {
	if m.screen != screenResult || m.game == nil {
		return nil
	}
	m.screen = screenGame
	return nil
}

func (m *Model) nextResult() tea.Cmd {
	if m.screen != screenResult || m.game == nil {
		return nil
	}
	if m.game.g.Daily != "" {
		if err := m.game.leave(); err != nil {
			m.result.notice = fmt.Sprintf("could not save: %v", err)
			return nil
		}
		m.openDailyScreen()
		return nil
	}

	m.screen = screenGame
	return m.game.startNew()
}

func (m *Model) copyResult() tea.Cmd {
	if m.screen != screenResult || m.result.g == nil {
		return nil
	}
	m.result.copyRequested = true
	return tea.SetClipboard(shareResult(m.result.g))
}
