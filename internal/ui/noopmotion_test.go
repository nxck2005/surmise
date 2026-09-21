package ui

// The frame-reuse handshake: a mouse motion that changes nothing the frame is
// made of asks the next View to hand back the frame already on the terminal.
// View is composed for every message the framework processes, so a pointer
// resting on a target used to repaint at the motion rate for bytes nobody sees —
// the renderer drops an identical frame, but only after View has built it.
//
// "Changes nothing" is the delicate part, and these tests are mostly about how
// it is decided: the hover identity, the selection a pointer-following screen
// keeps, and the theme a hovered row previews. Getting it wrong would either
// freeze a frame that should have moved or serve a stale one, so every case here
// checks both the reuse request and what is actually on screen.

import (
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/nxck2005/surmise/internal/theme"
)

// motionAt sends a bare pointer motion to the middle of a rect.
func motionAt(m *Model, r rect) {
	m.Update(tea.MouseMotionMsg{X: r.x + r.w/2, Y: r.y + r.h/2})
}

// motionTo sends a bare pointer motion to a cell.
func motionTo(m *Model, x, y int) {
	m.Update(tea.MouseMotionMsg{X: x, Y: y})
}

// centre is the middle of a rect, in terminal cells.
func centre(r rect) (int, int) { return r.x + r.w/2, r.y + r.h/2 }

// parkedWithHover puts the pointer in the middle of a target and renders, so the
// frame on screen has that target hovered, and returns the target's rect. A
// second motion to the same cell is then the no-op case.
func parkedWithHover(t *testing.T, m *Model, a action) rect {
	t.Helper()
	draw(t, m)
	r, ok := m.hits.find(a)
	if !ok {
		t.Fatalf("nothing on screen for %+v", a)
	}
	motionAt(m, r)
	if m.hover != a {
		t.Fatalf("hover = %+v, want %+v", m.hover, a)
	}
	m.View() // the frame the hover is part of
	return r
}

// emptyCell finds a cell no target covers, for the tests that need the pointer
// to leave everything.
func emptyCell(t *testing.T, m *Model) (int, int) {
	t.Helper()
	for y := range testHeight {
		for x := range testWidth {
			if _, ok := m.hits.at(x, y); !ok {
				return x, y
			}
		}
	}
	t.Fatal("every cell of the frame is a target")
	return 0, 0
}

// Repeated motion inside one target is the case the whole mechanism exists for.
// The frame served must be the frame that was on screen — bytes and hit map
// alike — and the request must not outlive the View that consumed it.
func TestNoopMotionReusesTheFrame(t *testing.T) {
	m := gameModel(t)
	r := parkedWithHover(t, m, action{kind: actLetter, letter: 'q'})

	hits, frame := m.hits, m.lastView.Content
	motionAt(m, r)
	if !m.reuse {
		t.Fatal("a motion with nothing to change did not ask for the previous frame")
	}

	if got := m.View(); got.Content != frame {
		t.Error("the reused frame is not the frame that was on screen")
	}
	if m.hits != hits {
		t.Error("a no-op motion composed the frame again")
	}
	if m.reuse {
		t.Error("the reuse request outlived the View that consumed it")
	}

	// A later View, with no message in between, composes: the request is
	// one-shot, and nothing may serve bytes from an unknown moment.
	if m.View().Content != frame {
		t.Error("the recomposed frame differs from the settled one")
	}
	if m.hits == hits {
		t.Error("a second View served the same hit map without composing")
	}
}

// A motion that lands on a different target is not a no-op: the highlight has to
// move, and the hover has to record where the pointer really is.
func TestMovingBetweenTargetsComposesAndUpdatesHover(t *testing.T) {
	m := gameModel(t)
	parkedWithHover(t, m, action{kind: actLetter, letter: 'q'})

	next := action{kind: actLetter, letter: 'w'}
	r, ok := m.hits.find(next)
	if !ok {
		t.Fatalf("nothing on screen for %+v", next)
	}
	hits := m.hits

	motionAt(m, r)
	if m.reuse {
		t.Error("a motion onto another target asked for the previous frame")
	}
	if m.hover != next {
		t.Errorf("hover = %+v, want %+v", m.hover, next)
	}
	m.View()
	if m.hits == hits {
		t.Error("the frame was not composed")
	}
}

// Leaving a target changes the highlight, so it composes once; moving on with
// nothing under the pointer is then a no-op like any other.
func TestMovingOffATargetComposesOnceThenReuses(t *testing.T) {
	m := gameModel(t)
	parkedWithHover(t, m, action{kind: actLetter, letter: 'q'})

	x, y := emptyCell(t, m)
	hits := m.hits
	motionTo(m, x, y)
	if m.reuse {
		t.Error("leaving a target asked for the previous frame")
	}
	if m.hover != (action{}) {
		t.Errorf("hover = %+v, want nothing under a cell with no target", m.hover)
	}
	m.View()
	if m.hits == hits {
		t.Error("leaving a target did not compose")
	}

	// The next cell along is also empty, and the pointer is already nowhere.
	offHits := m.hits
	motionTo(m, x+1, y)
	if !m.reuse {
		t.Error("further off-target motion did not ask for the previous frame")
	}
	m.View()
	if m.hits != offHits {
		t.Error("further off-target motion composed the frame again")
	}
}

// The counterexample that rules out "same hover means no-op": the keyboard can
// move the selection away while the pointer stays inside the row it was last
// over, and a motion there has to put the selection back.
func TestMotionRestoresSelectionAfterAKeyboardMove(t *testing.T) {
	m := newModel(t) // the menu, where the selection follows the pointer
	draw(t, m)

	row := action{kind: actMenuChoice, index: 0}
	r, ok := m.hits.find(row)
	if !ok {
		t.Fatalf("nothing on screen for %+v", row)
	}
	motionAt(m, r)
	if m.menu.cursor != 0 {
		t.Fatal("the pointer did not move the selection")
	}
	m.View()

	send(t, m, "down") // the keyboard moves the selection away
	if m.menu.cursor != 1 {
		t.Fatalf("cursor = %d after moving down, want 1", m.menu.cursor)
	}
	if m.hover != row {
		t.Fatalf("the keyboard moved the hover as well: %+v", m.hover)
	}

	hits := m.hits
	motionAt(m, r)
	if m.menu.cursor != 0 {
		t.Errorf("cursor = %d, want the pointer to have put the selection back", m.menu.cursor)
	}
	if m.reuse {
		t.Error("a motion that moved the selection asked for the previous frame")
	}
	m.View()
	if m.hits == hits {
		t.Error("the frame was not composed")
	}
}

// Hovering a theme previews it, and moving on to another row previews that one.
// Moving inside the row already previewed must not re-apply it: applying a theme
// rebuilds every style, and there is nothing new for those styles to draw.
func TestThemePreviewFollowsThePointerWithoutRebuilding(t *testing.T) {
	withTheme(t, theme.Default())

	m := newModel(t)
	m.themes.reload(m.themeLib, m.themeName)
	m.screen = screenThemes
	draw(t, m)
	// The list opens on the committed theme, which on a 40-row terminal can be
	// scrolled past the first rows; start it at the top so row 1 is on screen.
	send(t, m, "home")
	draw(t, m)

	cursor := m.themes.cursor
	other := 0
	if other == cursor {
		other = 1
	}
	row := action{kind: actThemeRow, index: other}
	r, ok := m.hits.find(row)
	if !ok {
		t.Fatalf("nothing on screen for %+v", row)
	}
	want := m.themes.entries[other].Name

	motionAt(m, r)
	if got := st.theme.Name; got != want {
		t.Fatalf("hover previewed %q, want %q", got, want)
	}
	m.View()

	// The frame the preview drew, and the styles it drew with.
	r, ok = m.hits.find(row)
	if !ok {
		t.Fatalf("the previewed frame has no target for %+v", row)
	}
	applied, hits := st, m.hits

	motionAt(m, r)
	if st != applied {
		t.Error("motion inside the row already previewed re-applied the theme")
	}
	if !m.reuse {
		t.Error("motion inside the row already previewed asked for a new frame")
	}
	m.View()
	if m.hits != hits {
		t.Error("motion inside the row already previewed composed the frame again")
	}
}

// A reused frame leaves its hit map in place, which is what makes clicks that
// arrive between frames resolve to what is really under the pointer.
func TestClickAfterAReusedFrameStillActs(t *testing.T) {
	m := gameModel(t)
	r := parkedWithHover(t, m, action{kind: actLetter, letter: 'q'})
	x, y := centre(r)

	hits := m.hits
	motionTo(m, x, y)
	if !m.reuse {
		t.Fatal("a motion with nothing to change did not ask for the previous frame")
	}
	m.View()
	if m.hits != hits {
		t.Fatal("the frame was composed again for a no-op motion")
	}

	m.Update(tea.MouseClickMsg{X: x, Y: y, Button: tea.MouseLeft})
	if m.game.typing != "q" {
		t.Errorf("typing = %q after a click on a reused frame, want q", m.game.typing)
	}
}

// No-op mouse motion is not a clock. The one-second tick and the animation
// chain must still compose frames while the pointer rests, or an animating board
// would freeze under a moving pointer.
func TestTicksAndAnimationsStillRepaintThroughAReuse(t *testing.T) {
	now := time.Now()
	m := animModel(t)
	// The menu is where newModel puts it; a board is what "q" should type into.
	send(t, m, "down", "enter")
	withClock(t, &now)
	m.Update(tea.WindowSizeMsg{Width: testWidth, Height: testHeight})
	send(t, m, "q") // types, and starts a keycap pulse

	r := parkedWithHover(t, m, action{kind: actSubmit})
	hits, frame := m.hits, m.lastView.Content
	motionAt(m, r)
	if !m.reuse {
		t.Fatal("a motion with nothing to change did not ask for the previous frame")
	}
	m.View()

	// The animation's own tick is a message like any other, and the framework
	// calls View after each one. The pulse lasts a fixed hold rather than
	// marching, so the step here is past its end — the frame it settles into is
	// the un-highlighted one.
	advance(t, m, &now, 200*time.Millisecond)
	if m.reuse {
		t.Error("the animation tick left the previous frame on request")
	}
	if got := m.View().Content; got == frame {
		t.Error("the frame did not advance with the animation")
	}
	if m.hits == hits {
		t.Error("the animation tick served the reused frame")
	}

	// And so is the one-second clock.
	clockHits := m.hits
	m.Update(tickMsg(time.Now()))
	if m.reuse {
		t.Error("the clock tick left the previous frame on request")
	}
	m.View()
	if m.hits == clockHits {
		t.Error("the clock tick served the reused frame")
	}
}
