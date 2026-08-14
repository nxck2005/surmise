package ui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

// tallHeight is a terminal with room for the fuller board; testHeight (40) is
// not, which is what keeps every other test looking at the flat one.
const tallHeight = 48

// The board grows only when the terminal can afford everything it already drew.
// A window that cannot is left exactly as it was — the flat board is not a
// degraded mode, it is the default.
func TestBoardGrowsOnlyWhenTheTerminalCanAffordIt(t *testing.T) {
	m := gameModel(t)

	for _, tc := range []struct {
		height, want int
	}{
		{0, flatTile},  // unmeasured
		{24, flatTile}, // the classic terminal
		{testHeight, flatTile},
		{tallHeight, tallTile},
	} {
		m.Update(tea.WindowSizeMsg{Width: testWidth, Height: tc.height})
		if got := m.game.tileRows(); got != tc.want {
			t.Errorf("a %d-row terminal draws %d-row tiles, want %d", tc.height, got, tc.want)
		}
	}
}

// Growing must never push the frame past the terminal: nothing here truncates,
// and an over-tall frame loses its top — panel title, close box and all.
//
// Only the grown board is asserted. A board that stayed flat is drawing exactly
// what it drew before this existed, including the overflow a 24-row terminal has
// always had with the longer modes.
func TestATallBoardAlwaysFits(t *testing.T) {
	for _, length := range []int{4, 5, 6} {
		m := newModel(t)
		m.menu.point(menuIndex(t, m, choiceNewGame, length))
		send(t, m, "enter")

		grew := false
		for height := 24; height <= 80; height++ {
			frame := drawAt(t, m, height)
			if m.game.tileRows() == flatTile {
				continue
			}
			grew = true
			if got := strings.Count(frame, "\n") + 1; got > height {
				t.Fatalf("a tall %d-letter board is %d rows in a %d-row terminal",
					length, got, height)
			}
		}
		if !grew {
			t.Errorf("a %d-letter board never grew, even at 80 rows", length)
		}
	}
}

// The legend is the first thing to go on a short terminal, and the last thing a
// tall board is allowed to cost: growing the tiles must not push it off.
func TestATallBoardKeepsTheLegend(t *testing.T) {
	m := gameModel(t)
	frame := drawAt(t, m, tallHeight)

	if m.game.tileRows() != tallTile {
		t.Fatalf("the board did not grow at %d rows", tallHeight)
	}
	if !strings.Contains(sgr.ReplaceAllString(frame, ""), "correct spot") {
		t.Errorf("a tall board dropped the legend:\n%s", frame)
	}
}

// A tall tile is one atom, so its whole height is the click target — and the
// letter it claims to cover really is inside it.
func TestTallTilesKeepTheirClickTargets(t *testing.T) {
	m := gameModel(t)
	send(t, m, "c", "r")
	frame := drawAt(t, m, tallHeight)

	trim := action{kind: actTrim, index: 1}
	r, ok := m.hits.find(trim)
	if !ok {
		t.Fatal("no typed-tile target on a tall board")
	}
	if r.h != tallTile {
		t.Errorf("the target is %d rows tall, want %d", r.h, tallTile)
	}
	// The letter sits in the middle row of the tile, which is where a click at
	// the target's centre lands.
	middle := rect{x: r.x, y: r.y + r.h/2, w: r.w, h: 1}
	if got := strings.TrimSpace(at(t, frame, middle)); got != "R" {
		t.Errorf("the middle of the target covers %q, want %q", got, "R")
	}

	// And it still does what it is for: trimming the row back to that slot.
	m.Update(tea.MouseClickMsg{X: r.x + r.w/2, Y: r.y + r.h/2, Button: tea.MouseLeft})
	if m.game.typing != "c" {
		t.Errorf("typing = %q after trimming to slot 1, want %q", m.game.typing, "c")
	}
}

// Only the board grows. The debrief shows the same word again and has no height
// to spare, and neither has the theme preview or the how-to-play example.
func TestOnlyTheBoardGrows(t *testing.T) {
	m := gameModel(t)
	send(t, m, "c", "r", "a", "n", "e", "enter")
	if m.screen != screenResult {
		t.Fatalf("screen = %v, want the result", m.screen)
	}

	tall := drawAt(t, m, tallHeight)
	flat := drawAt(t, m, testHeight)
	if sgr.ReplaceAllString(tall, "") == "" {
		t.Fatal("no frame")
	}
	// The debrief's grid is the same shape in a big terminal as in a small one.
	tallRows := strings.Count(sgr.ReplaceAllString(tall, ""), "\n")
	flatRows := strings.Count(sgr.ReplaceAllString(flat, ""), "\n")
	if tallRows-tallHeight != flatRows-testHeight {
		t.Errorf("the debrief changed height with the terminal: %d at %d, %d at %d",
			tallRows, tallHeight, flatRows, testHeight)
	}
}
