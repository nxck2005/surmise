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

	// anim is the root's animation state, shared the way the board shares it.
	// Nil draws the settled debrief, which is what every frame test compares.
	anim *anims
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
	switch {
	case g.Daily != "":
		what = fmt.Sprintf("daily %s · %s", g.Daily, what)
	case g.Custom:
		what = fmt.Sprintf("custom · %s", what)
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
		stackSpaced(rows, 1),
	}
	if g.Status == game.Lost {
		// The word is what a lost puzzle is opened for, so it is briefly worth
		// more than the label beside it. The emphasis settles back to st.text,
		// which is the frame a still debrief has always drawn.
		word := st.text
		if m.anim.answering(timeNow()) {
			word = st.accent
		}
		sections = append(sections, "",
			st.muted.Render("answer ")+word.Render(strings.ToUpper(g.Answer)))
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
	switch {
	case m.g.Daily != "":
		return "daily"
	case m.g.Custom:
		// Another custom board needs somebody to choose a word, so "next"
		// goes back to the screen that asks for one rather than dealing a random
		// puzzle the pair did not ask for.
		return "again"
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
	if g.Custom {
		fmt.Fprintf(&b, "custom · %d letters · %s\n",
			g.Length, formatDuration(g.Elapsed()))
	} else if g.Daily != "" {
		fmt.Fprintf(&b, "daily %s · %d letters · %s\n",
			g.Daily, g.Length, formatDuration(g.Elapsed()))
	} else {
		fmt.Fprintf(&b, "%d letters · %s\n", g.Length, formatDuration(g.Elapsed()))
	}

	b.WriteString(shareGrid(g.Marks))
	b.WriteString(shareLegend)
	return b.String()
}

// shareLegend explains the glyphs shareGrid writes, for a reader whose client
// renders them as bare squares.
const shareLegend = "■ correct  □ present  · absent"

// shareGrid is a scored board as public text: one line per played guess, one
// glyph per mark, with a trailing newline. Every share goes through it, so the
// glyphs are written once and a board reads the same wherever it is pasted.
//
// The glyphs are deliberately fixed rather than themed. A theme dresses this
// terminal; a shared result is read in somebody else's, and a paste that
// depended on the sender's palette would arrive as squares nobody could match to
// a legend.
func shareGrid(marks [][]game.Mark) string {
	var b strings.Builder
	for _, row := range marks {
		for _, mark := range row {
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
	if m.game.g.Daily != "" || m.game.g.Custom {
		if err := m.game.leave(); err != nil {
			m.result.notice = fmt.Sprintf("could not save: %v", err)
			return nil
		}
		if m.game.g.Custom {
			m.custom = newCustomScreen(m.game.g.Length)
			m.screen = screenCustom
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
