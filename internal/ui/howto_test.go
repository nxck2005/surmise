package ui

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/nxck2005/surmise/internal/game"
	"github.com/nxck2005/surmise/internal/words"
)

// openHowTo reaches the screen the way a player does.
func openHowTo(t *testing.T, m *Model) string {
	t.Helper()
	m.menu.point(menuIndex(t, m, choiceHowTo, 0))
	return send(t, m, "enter")
}

func TestHowToOpensFromTheMenuAndLeaves(t *testing.T) {
	m := newModel(t)

	frame := openHowTo(t, m)
	if m.screen != screenHowTo {
		t.Fatalf("screen = %v, want the how-to screen", m.screen)
	}
	if !strings.Contains(frame, "how to play") {
		t.Errorf("the screen does not name itself:\n%s", frame)
	}

	send(t, m, "esc")
	if m.screen != screenMenu {
		t.Errorf("esc left the screen on %v, want the menu", m.screen)
	}
}

// The attempt counts are what a new player comes here for, and they are the one
// thing on the screen that has to agree with the rules: a puzzle gets one guess
// more than it has letters, for every mode that ships.
func TestHowToStatesTheAttemptsForEveryMode(t *testing.T) {
	m := newModel(t)
	frame := openHowTo(t, m)

	plain := sgr.ReplaceAllString(frame, "")
	for _, n := range words.Lengths {
		g, err := game.New(n)
		if err != nil {
			t.Fatal(err)
		}
		// The row as the screen draws it, against the attempts a real puzzle of
		// that mode gets — so a change to either side fails here.
		row := fmt.Sprintf("%-12s %d", fmt.Sprintf("%d letters", n), g.MaxAttempts)
		if !strings.Contains(plain, row) {
			t.Errorf("no row %q:\n%s", row, plain)
		}
	}
}

// The worked examples are computed from game.Score, so the lesson cannot drift
// from the rules. This pins the duplicate-letter one, which is the whole reason
// the page exists: ABIDE has one E, the exact match at the end claims it, and
// the earlier ones score absent.
func TestHowToScoresItsExamplesLikeTheGame(t *testing.T) {
	m := newModel(t)
	openHowTo(t, m)
	m.howTo.show(1)
	frame := sgr.ReplaceAllString(m.View().Content, "")

	if !strings.Contains(frame, "ABIDE") || !strings.Contains(frame, "CARGO") {
		t.Fatalf("the scoring page lost an example:\n%s", frame)
	}
	for _, want := range []string{
		renderScoredRow("crane", game.Score("crane", "cargo")),
		renderScoredRow("geese", game.Score("geese", "abide")),
	} {
		if !strings.Contains(m.View().Content, want) {
			t.Errorf("the page does not draw %q as the game scores it:\n%s",
				sgr.ReplaceAllString(want, ""), frame)
		}
	}
}

func TestHowToPagingClampsAtBothEnds(t *testing.T) {
	m := newModel(t)
	openHowTo(t, m)

	if m.howTo.page != 0 {
		t.Fatalf("opened on page %d, want the first", m.howTo.page)
	}
	send(t, m, "left")
	if m.howTo.page != 0 {
		t.Errorf("prev on the first page moved to %d", m.howTo.page)
	}

	last := len(howToPages) - 1
	for range len(howToPages) + 2 {
		send(t, m, "right")
	}
	if m.howTo.page != last {
		t.Errorf("next ran to page %d, want it clamped at %d", m.howTo.page, last)
	}

	// Re-entering starts at the front: the pages are a sequence.
	send(t, m, "esc")
	openHowTo(t, m)
	if m.howTo.page != 0 {
		t.Errorf("reopened on page %d, want the first", m.howTo.page)
	}
}

// A hint for a key that would do nothing is a promise the screen does not keep,
// so each end drops the direction it cannot go.
func TestHowToHelpDropsTheDeadDirection(t *testing.T) {
	m := newModel(t)
	frame := openHowTo(t, m)

	if strings.Contains(frame, "prev") {
		t.Errorf("the first page offers prev:\n%s", frame)
	}
	if !strings.Contains(frame, "next") {
		t.Errorf("the first page offers no next:\n%s", frame)
	}

	m.howTo.show(len(howToPages) - 1)
	frame = m.View().Content
	if strings.Contains(frame, "next") {
		t.Errorf("the last page offers next:\n%s", frame)
	}
	if !strings.Contains(frame, "prev") {
		t.Errorf("the last page offers no prev:\n%s", frame)
	}
}

func TestHowToPagesByMouse(t *testing.T) {
	m := newModel(t)
	openHowTo(t, m)

	// The help bar's arrow and a dot are the two ways to page with a pointer;
	// both carry the page they turn to.
	click(t, m, action{kind: actHowToPage, index: 1})
	if m.howTo.page != 1 {
		t.Fatalf("the next button left the screen on page %d", m.howTo.page)
	}

	last := len(howToPages) - 1
	click(t, m, action{kind: actHowToPage, index: last})
	if m.howTo.page != last {
		t.Errorf("the last dot left the screen on page %d, want %d", m.howTo.page, last)
	}

	click(t, m, action{kind: actBack})
	if m.screen != screenMenu {
		t.Errorf("the menu button left the screen on %v", m.screen)
	}
}

// Nothing here truncates, and an over-tall frame loses its top rows — title,
// close box and all. Every page therefore gives up its extras rather than
// overflowing a terminal that a board would still fit in.
func TestHowToShedsExtrasOnAShortTerminal(t *testing.T) {
	m := newModel(t)
	openHowTo(t, m)

	const short = 20
	for i := range howToPages {
		m.howTo.show(i)
		frame := drawAt(t, m, short)
		if h := len(strings.Split(frame, "\n")); h > short {
			t.Errorf("page %q is %d lines in a %d-line terminal:\n%s",
				howToPages[i].name, h, short, frame)
		}
	}

	// The full-height frame is the one that carries everything, so the shedding
	// really is a response to the terminal and not a page that lost content.
	m.howTo.show(0)
	tall := drawAt(t, m, testHeight)
	if !strings.Contains(sgr.ReplaceAllString(tall, ""), "refused") {
		t.Errorf("a tall terminal dropped an extra anyway:\n%s", tall)
	}
	m.Update(tea.WindowSizeMsg{Width: testWidth, Height: testHeight})
}
