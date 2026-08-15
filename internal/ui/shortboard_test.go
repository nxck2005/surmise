package ui

import (
	"strings"
	"testing"

	"github.com/nxck2005/surmise/internal/brand"
)

// boardModel starts a board in one mode, the way every height sweep below needs
// it: through the menu, so the screen is the one a player reaches.
func boardModel(t *testing.T, length int) *Model {
	t.Helper()
	m := newModel(t)
	m.menu.point(menuIndex(t, m, choiceNewGame, length))
	send(t, m, "enter")
	return m
}

// floor is the shortest terminal a mode can be played in: the tightest form the
// ladder has, which is boardLayout's zero value plus a flat tile. Deriving it
// rather than writing it down is what keeps these tests honest when a section
// is added to the screen.
func floor(m *Model) int {
	return boardLayout{tiles: flatTile}.rows(m.game.g.MaxAttempts)
}

// The bug this whole ladder exists for. Nothing in the pipeline truncates, and
// Bubble Tea's renderer drops an over-tall frame's excess off the *top* — so an
// overflowing board loses the panel's title rule and its close box, which are
// the only things saying which screen this is and the only way out with a
// mouse.
//
// This is the test the flat case was excused from before: see the note in
// TestATallBoardAlwaysFits.
func TestTheBoardAlwaysFitsTheTerminal(t *testing.T) {
	for _, length := range []int{4, 5, 6} {
		m := boardModel(t, length)
		for height := floor(m); height <= 80; height++ {
			frame := drawAt(t, m, height)
			if got := strings.Count(frame, "\n") + 1; got > height {
				t.Fatalf("a %d-letter board is %d rows in a %d-row terminal",
					length, got, height)
			}
			if m.game.layout().refuse {
				t.Fatalf("a %d-letter board refuses at %d rows, which is at or above its floor of %d",
					length, height, floor(m))
			}
		}
	}
}

// 24 rows is the classic terminal, and the size the app's own guidance has
// always implied fits. Row count alone would not catch what the player actually
// lost, so this asserts on the two things an overflow takes.
func TestEveryModeFitsAClassicTerminal(t *testing.T) {
	const height = 24
	for _, length := range []int{4, 5, 6} {
		m := boardModel(t, length)
		frame := drawAt(t, m, height)

		if got := strings.Count(frame, "\n") + 1; got > height {
			t.Errorf("a %d-letter board is %d rows in a %d-row terminal", length, got, height)
		}
		// What the terminal really shows is the last `height` lines: the renderer
		// drops an over-tall frame's excess off the top, which is exactly how the
		// title rule and the close box used to disappear.
		lines := strings.Split(sgr.ReplaceAllString(frame, ""), "\n")
		if len(lines) > height {
			lines = lines[len(lines)-height:]
		}
		visible := strings.Join(lines, "\n")
		if !strings.Contains(visible, brand.Name) {
			t.Errorf("the panel's title rule is off the top of a %d-letter board:\n%s", length, visible)
		}
		if !strings.Contains(visible, "×") {
			t.Errorf("the close box is off the top of a %d-letter board:\n%s", length, visible)
		}
		if _, ok := m.hits.find(action{kind: actQuit}); !ok {
			t.Errorf("a %d-letter board at %d rows has no close target", length, height)
		}
	}
}

// The order is the whole design: cheapest harm first, and only ever descended.
// A ladder that climbed back would let a terminal that had just given up its
// header regain the legend.
func TestTheShedLadderGivesUpInOrder(t *testing.T) {
	for _, tc := range []struct {
		length int
		height int
		want   boardLayout
	}{
		// 5 letters, from a roomy terminal down to the floor.
		{5, 40, boardLayout{tiles: flatTile, header: true, legend: true, kbdGap: 1, boardGap: 1}},
		{5, 28, boardLayout{tiles: flatTile, header: true, kbdGap: 1, boardGap: 1}},
		{5, 26, boardLayout{tiles: flatTile, kbdGap: 1, boardGap: 1}},
		{5, 24, boardLayout{tiles: flatTile, boardGap: 1}},
		{5, 22, boardLayout{tiles: flatTile}},
		{5, 17, boardLayout{tiles: flatTile, refuse: true}},

		// Where each mode lands on a classic terminal.
		{4, 24, boardLayout{tiles: flatTile, kbdGap: 1, boardGap: 1}},
		{6, 24, boardLayout{tiles: flatTile}},
	} {
		m := boardModel(t, tc.length)
		drawAt(t, m, tc.height)
		if got := m.game.layout(); got != tc.want {
			t.Errorf("%d letters at %d rows: layout %+v, want %+v",
				tc.length, tc.height, got, tc.want)
		}
	}
}

// Each rung has to be the *first* form that fits, or the screen gives up more
// than the terminal asked it to. Checking the rung above always overflows is
// what pins the arithmetic in rows() to the frame the renderer really draws.
func TestTheLadderGivesUpNoMoreThanItHasTo(t *testing.T) {
	for _, length := range []int{4, 5, 6} {
		m := boardModel(t, length)
		attempts := m.game.g.MaxAttempts
		for height := floor(m); height <= 48; height++ {
			drawAt(t, m, height)
			l := m.game.layout()
			if l.rows(attempts) > height {
				t.Fatalf("%d letters at %d rows: the chosen form needs %d",
					length, height, l.rows(attempts))
			}
			if up, ok := rungAbove(l); ok && up.rows(attempts) <= height {
				t.Errorf("%d letters at %d rows: gave up to %+v, but %+v fits in %d",
					length, height, l, up, up.rows(attempts))
			}
		}
	}
}

// rungAbove is the form the ladder would have drawn one rung earlier, and
// reports false at the top. It is the ladder's order written backwards, so a
// change to one without the other fails the test above.
func rungAbove(l boardLayout) (boardLayout, bool) {
	switch {
	case l.tiles == tallTile:
		return l, false
	case l.boardGap == 0:
		l.boardGap = 1
	case l.kbdGap == 0:
		l.kbdGap = 1
	case !l.header:
		l.header = true
	case !l.legend:
		l.legend = true
	default:
		return l, false
	}
	l.refuse = false
	return l, true
}

// The status line is not a rung. It carries the transient error, the win and
// loss result, and the tab-then-enter prompt — and both halves of that prompt
// are click targets, so a player without a keyboard is stranded without it.
func TestTheStatusLineNeverSheds(t *testing.T) {
	m := boardModel(t, 6)
	for height := floor(m); height <= 40; height++ {
		frame := sgr.ReplaceAllString(drawAt(t, m, height), "")
		if !strings.Contains(frame, "guesses left") {
			t.Fatalf("the status line is gone at %d rows:\n%s", height, frame)
		}
	}

	// And the prompt it carries keeps its two targets on a classic terminal.
	drawAt(t, m, 24)
	send(t, m, "tab")
	drawAt(t, m, 24)
	for _, want := range []action{{kind: actNewPuzzle}, {kind: actCancelNew}} {
		if _, ok := m.hits.find(want); !ok {
			t.Errorf("the restart prompt has no %v target at 24 rows", want.kind)
		}
	}
}

// Below the floor there is no form left, and drawing anyway is the bug. The
// screen says so instead, and stays reachable and leavable.
func TestATerminalBelowTheFloorIsRefused(t *testing.T) {
	m := boardModel(t, 6)
	height := floor(m) - 1
	frame := sgr.ReplaceAllString(drawAt(t, m, height), "")

	if !m.game.layout().refuse {
		t.Fatalf("a %d-row terminal is below the floor of %d but was not refused", height, floor(m))
	}
	if got := strings.Count(frame, "\n") + 1; got > height {
		t.Errorf("the refusal is %d rows in a %d-row terminal:\n%s", got, height, frame)
	}
	if !strings.Contains(frame, "too short") {
		t.Errorf("the refusal does not say what is wrong:\n%s", frame)
	}
	if !strings.Contains(frame, "esc") {
		t.Errorf("the refusal offers no way out:\n%s", frame)
	}

	// A board nobody can see is never played blind.
	send(t, m, "c")
	if m.game.typing != "" {
		t.Errorf("a refused board took the letter %q", m.game.typing)
	}

	send(t, m, "esc")
	if m.screen != screenMenu {
		t.Errorf("esc did not leave a refused board; screen is %v", m.screen)
	}
}

// A tightened board and keyboard still have to be clickable: hit regions are
// measured from the frame, so giving up a gutter must move the targets with the
// glyphs rather than leave them where the spaced frame put them.
func TestATightBoardKeepsItsClickTargets(t *testing.T) {
	m := boardModel(t, 6)
	drawAt(t, m, 24)
	if l := m.game.layout(); l.kbdGap != 0 || l.boardGap != 0 {
		t.Fatalf("a 6-letter board at 24 rows should be tight; layout is %+v", l)
	}

	click(t, m, action{kind: actLetter, letter: 'q'})
	if m.game.typing != "q" {
		t.Errorf("clicking Q on a tight keyboard typed %q", m.game.typing)
	}
}

// Growing and shedding must never combine: a window with room to grow is by
// definition a window with nothing to give up.
func TestATallBoardIsNeverATightenedOne(t *testing.T) {
	for _, length := range []int{4, 5, 6} {
		m := boardModel(t, length)
		for height := floor(m); height <= 80; height++ {
			drawAt(t, m, height)
			l := m.game.layout()
			if l.tiles != tallTile {
				continue
			}
			if want := (boardLayout{tiles: tallTile, header: true, legend: true, kbdGap: 1, boardGap: 1}); l != want {
				t.Fatalf("a tall %d-letter board at %d rows has given something up: %+v",
					length, height, l)
			}
		}
	}
}

// The arithmetic reads the theme's metrics rather than the numbers that
// happened to be true when it was written. A theme with no panel padding hands
// the ladder two rows it never has to ask for.
func TestTheLadderReadsTheThemeMetrics(t *testing.T) {
	withTheme(t, themed(t, "[metrics]\npanel_pad_y = 0\n"))

	for _, length := range []int{4, 5, 6} {
		m := boardModel(t, length)
		frame := drawAt(t, m, 24)
		if got := strings.Count(frame, "\n") + 1; got > 24 {
			t.Errorf("a %d-letter board is %d rows in a 24-row terminal with no panel padding",
				length, got)
		}
		// Two rows cheaper than the default theme, so the ladder stops higher up.
		if l := m.game.layout(); l.refuse {
			t.Errorf("a %d-letter board refuses at 24 rows with no panel padding", length)
		}
	}
}

// An unmeasured terminal is unbounded — the same rule affordableSections
// follows, and what keeps every headless test drawing a whole screen.
func TestAnUnmeasuredTerminalGivesUpNothing(t *testing.T) {
	m := boardModel(t, 6)
	m.game.resize(0, 0)
	want := boardLayout{tiles: flatTile, header: true, legend: true, kbdGap: 1, boardGap: 1}
	if got := m.game.layout(); got != want {
		t.Errorf("an unmeasured board draws %+v, want %+v", got, want)
	}
}
