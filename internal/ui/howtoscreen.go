package ui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/nxck2005/surmise/internal/game"
	"github.com/nxck2005/surmise/internal/words"
)

// howToScreen teaches the game: the rules, how a scored row is read, what is
// saved, and every control. It is read-only like the about screen, but its
// content does not fit one terminal, so it is paged rather than scrolled —
// prose is read a section at a time, and a page short enough to fit is a page
// nobody has to scroll back up in.
//
// It is deliberately not a first-launch modal. A player who dismissed the
// rules once still has to be able to look up the duplicate-letter rule in a
// year's time, so this lives in the menu and nowhere else.
type howToScreen struct {
	page int

	// width and height are the terminal's, pushed down by the root. Zero means
	// unmeasured, which counts as unbounded.
	width, height int
}

// howToPage is one section. render returns the sections the page must show and
// the ones it gives up on a short terminal, in the order they are given up.
//
// It is a function rather than a built string because the styles live in the
// package-level st, which is swapped wholesale when the theme changes: a page
// built at init would keep the colours of the theme that was live then, and the
// scoring page draws real tiles.
type howToPage struct {
	name   string
	render func() (required, optional []string)
}

// howToPages is the whole content of the screen, in order. Adding, removing or
// reordering a page is an edit here and in the function it names, and nowhere
// else — the view below only knows how to draw a heading, some sections and a
// row of dots.
var howToPages = []howToPage{
	{name: "the game", render: howToRules},
	{name: "scoring", render: howToScoring},
	{name: "saving", render: howToSaving},
	{name: "controls", render: howToControls},
}

// The page dots. Plain text rather than theme glyphs: a themeable element means
// a theme.Elements entry and a docs/THEMES.md row, and a position indicator on
// one screen has not earned one.
const (
	howToDotOn  = "●"
	howToDotOff = "○"
)

func (s *howToScreen) resize(w, h int) { s.width, s.height = w, h }

// reset returns to the first page, keeping the measured size. Opening the
// screen goes through it, so a second visit starts where the first did.
func (s *howToScreen) reset() { s.page = 0 }

// show turns to a page. It is the one place the page changes, so the keys, the
// dots and the help bar's buttons cannot drift apart.
func (s *howToScreen) show(page int) {
	if page >= 0 && page < len(howToPages) {
		s.page = page
	}
}

// step moves by one, clamped at the ends. There is no wrap: the dots say where
// you are, and a "next" that jumped back to the start would disagree with them.
func (s *howToScreen) step(delta int) {
	s.show(min(max(s.page+delta, 0), len(howToPages)-1))
}

// update handles the screen's keys and reports whether to leave.
func (s *howToScreen) update(msg tea.KeyPressMsg) (back bool) {
	switch msg.String() {
	case "esc", "q":
		return true
	case "left", "h":
		s.step(-1)
	case "right", "l":
		s.step(1)
	}
	return false
}

func (s *howToScreen) view(h *hitMap) string {
	page := howToPages[s.page]
	required, optional := page.render()

	// The page name and its dots lead the body, with a blank under them.
	budget := bodyBudget(s.height)
	if budget > 0 {
		budget = max(budget-2, 1)
	}
	// Spaced out first, so the blank lines between the required sections are
	// part of what the optional ones are measured against.
	sections := make([]string, 0, 2*(len(required)+len(optional)))
	for i, s := range required {
		if i > 0 {
			sections = append(sections, "")
		}
		sections = append(sections, s)
	}
	for _, extra := range affordableSections(sections, optional, budget) {
		if extra != "" {
			sections = append(sections, "", extra)
		}
	}

	// The panel's rule names the screen; the page and its dots lead the body.
	heading := st.muted.Render(page.name) + st.help.Render(st.glyph.Separator) + s.dots(h)
	// Squared off first, so the join slides the body under the heading as one
	// block instead of centring every line of it on its own.
	return lipgloss.JoinVertical(lipgloss.Center, heading, "",
		block(lipgloss.JoinVertical(lipgloss.Left, sections...)))
}

// dots are the position indicator, and each one is also a click target: a
// reader who can see there are four pages should be able to reach the fourth
// without stepping through the ones between.
func (s *howToScreen) dots(h *hitMap) string {
	cells := make([]string, len(howToPages))
	for i := range howToPages {
		a := action{kind: actHowToPage, index: i}
		glyph, style := howToDotOff, st.muted
		if i == s.page {
			glyph, style = howToDotOn, st.accent
		}
		if h.hovered(a) {
			style = st.hover(style)
		}
		cells[i] = h.mark(a, style.Render(glyph))
	}
	return strings.Join(cells, " ")
}

// help drops the hint for whichever end the reader is at, rather than offering
// a key that would do nothing — the same rule the splash's fixed mode follows.
func (s *howToScreen) help(h *hitMap) string {
	items := make([]helpItem, 0, 3)
	if s.page > 0 {
		items = append(items, helpItem{
			keys: "←", label: "prev",
			act: action{kind: actHowToPage, index: s.page - 1},
		})
	}
	if s.page < len(howToPages)-1 {
		items = append(items, helpItem{
			keys: "→", label: "next",
			act: action{kind: actHowToPage, index: s.page + 1},
		})
	}
	items = append(items, helpItem{keys: "esc", label: "menu", act: action{kind: actBack}})
	return renderHelp(h, items...)
}

// --- the pages ---

func howToRules() (required, optional []string) {
	intro := prose(
		"guess the hidden word. type a word, press enter,",
		"and the tiles show how close you got.",
	)

	// Built from the modes that actually ship, and from the same n+1 the game
	// gives a puzzle, so a fourth mode would appear here on its own.
	rows := make([]string, 0, len(words.Lengths)+1)
	rows = append(rows, st.muted.Render("mode         tries"))
	for _, n := range words.Lengths {
		rows = append(rows, st.text.Render(
			fmt.Sprintf("%-12s %d", fmt.Sprintf("%d letters", n), n+1)))
	}

	return []string{intro, strings.Join(rows, "\n")}, nil
}

func howToScoring() (required, optional []string) {
	// The marks are computed rather than written out, so the lesson moves with
	// the rules: a change to Score, or a [style.tile.*] override, changes the
	// example the same way it changes the board.
	example := func(caption, guess, answer string) string {
		return caption + "\n" + renderScoredRow(guess, game.Score(guess, answer))
	}

	// The board's legend, stacked rather than in its one wide line: this screen
	// has the height for it and not the width, and every entry still renders
	// through the very styles the tiles do.
	key := lines(
		legendEntry(st.tileCorrect, "right letter, right spot"),
		legendEntry(st.tilePresent, "in the word, wrong spot"),
		legendEntry(st.tileAbsent, "not in the word"),
	)

	return []string{
			key,
			example(prose("say the answer is CARGO and you guess CRANE:"), "crane", "cargo"),
		}, []string{
			aside(
				"C is in the right spot. R and A are in the",
				"word, but in other spots. N and E aren't in it.",
			),
			example(prose(
				"a letter only lights up as many times as it's",
				"in the answer. if the answer is ABIDE:",
			), "geese", "abide"),
			aside(
				"ABIDE has one E. the last E in GEESE is in the",
				"right spot, so the other E's don't light up.",
			),
		}
}

func howToSaving() (required, optional []string) {
	return []string{
			prose(
				"the game saves after every guess, so you can",
				"quit whenever you like. everything you've",
				"played is under puzzles in the menu.",
			),
			prose("press tab, then enter, for a new puzzle."),
			prose(
				"there's one daily for each length, and it's",
				"the same for everyone. a new one comes out at",
				"midnight UTC.",
			),
		}, []string{
			aside(
				"custom puzzles are saved too, but they don't",
				"count toward your stats.",
			),
			aside(
				"a challenge code gives everyone the same board.",
				"it's saved and counted like any other puzzle.",
			),
		}
}

func howToControls() (required, optional []string) {
	keys := [][2]string{
		{"letters", "type a guess"},
		{"enter", "submit"},
		{"backspace", "delete a letter"},
		{"tab, enter", "a new puzzle"},
		{"←/→", "these pages"},
		{"esc", "back to the menu"},
		{"q", "quit"},
	}
	rows := make([]string, len(keys))
	for i, k := range keys {
		rows[i] = st.text.Render(fmt.Sprintf("%-12s", k[0])) + st.muted.Render(k[1])
	}

	return []string{strings.Join(rows, "\n")}, []string{
		aside(
			"the mouse works too. click the keyboard to",
			"type, click a typed letter to erase back to it,",
			"and click the hints at the bottom. × quits.",
		),
		aside(
			"in a sprint, tab does nothing. esc ends the",
			"run and shows your results.",
		),
		aside(
			"daily, challenge and custom boards can't be",
			"swapped with tab. open another from its menu.",
		),
	}
}

// prose and aside are one section of text each, at the two weights this screen
// uses: what it is saying, and the note beside it. Both take their lines
// already broken, because nothing in this UI reflows — a line too wide for the
// terminal widens the panel rather than wrapping.
//
// They style plain text, so neither may be handed something already rendered:
// nesting one style inside another corrupts its escape codes. The pages compose
// styled pieces with lines instead.
func prose(l ...string) string { return st.text.Render(strings.Join(l, "\n")) }

func aside(l ...string) string { return st.muted.Render(strings.Join(l, "\n")) }

// lines joins already-styled pieces into one section.
func lines(l ...string) string { return strings.Join(l, "\n") }
