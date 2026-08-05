package ui

import (
	"regexp"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/nxck2005/wortle/internal/game"
	"github.com/nxck2005/wortle/internal/store"
)

// The mouse tests need a terminal size: clickable regions only exist once a
// frame has been composed and placed.
const (
	testWidth  = 100
	testHeight = 40
)

// draw sizes the terminal and renders, which is what records the frame's
// clickable regions.
func draw(t *testing.T, m *Model) string {
	t.Helper()
	m.Update(tea.WindowSizeMsg{Width: testWidth, Height: testHeight})
	return m.View().Content
}

// target looks up where an action was last drawn.
func target(t *testing.T, m *Model, a action) rect {
	t.Helper()
	draw(t, m)
	r, ok := m.hits.find(a)
	if !ok {
		t.Fatalf("nothing on screen for %+v", a)
	}
	return r
}

// click presses the left button in the middle of wherever an action is drawn.
// Coordinates come from the frame itself, so these tests do not bake in a
// layout that every styling tweak would invalidate.
func click(t *testing.T, m *Model, a action) {
	t.Helper()
	r := target(t, m, a)
	m.Update(tea.MouseClickMsg{X: r.x + r.w/2, Y: r.y + r.h/2, Button: tea.MouseLeft})
}

// point moves the pointer over an action without pressing anything.
func point(t *testing.T, m *Model, a action) {
	t.Helper()
	r := target(t, m, a)
	m.Update(tea.MouseMotionMsg{X: r.x + r.w/2, Y: r.y + r.h/2})
}

// clickWord spells a word on the on-screen keyboard.
func clickWord(t *testing.T, m *Model, word string) {
	t.Helper()
	for i := range word {
		click(t, m, action{kind: actLetter, letter: word[i]})
	}
}

var sgr = regexp.MustCompile("\x1b\\[[0-9;]*m")

// cells strips the styling from a frame and returns it as a grid of display
// cells, so a recorded rect can be checked against the glyphs really on screen.
// Every non-ASCII glyph in this UI is one cell wide, so runes map to cells.
func cells(frame string) [][]rune {
	lines := strings.Split(sgr.ReplaceAllString(frame, ""), "\n")
	grid := make([][]rune, len(lines))
	for i, l := range lines {
		grid[i] = []rune(l)
	}
	return grid
}

// at returns the text a rect covers in the frame.
func at(t *testing.T, frame string, r rect) string {
	t.Helper()
	grid := cells(frame)
	if r.y >= len(grid) || r.x+r.w > len(grid[r.y]) {
		t.Fatalf("rect %+v falls outside the %d-line frame", r, len(grid))
	}
	return string(grid[r.y][r.x : r.x+r.w])
}

// gameModel returns a model on a 5-letter board with a known answer.
func gameModel(t *testing.T) *Model {
	t.Helper()
	m := newModel(t)
	send(t, m, "down", "enter")
	m.game.g.Answer = "crane"
	return m
}

// TestMarkersDoNotAffectLayout is the load-bearing test for the whole approach:
// composing a screen with hit regions must produce, once the markers are
// stripped, exactly the bytes composing it without them does. If this passes,
// mouse support cannot have shifted a single cell of the UI.
func TestMarkersDoNotAffectLayout(t *testing.T) {
	m := newModel(t)
	m.Update(tea.WindowSizeMsg{Width: testWidth, Height: testHeight})

	// A puzzle worth listing, some typing on the board, and the restart prompt
	// armed, so every kind of marked atom is on screen at some point.
	send(t, m, "down", "enter")
	m.game.g.Answer = "crane"
	send(t, m, "a", "b", "o", "u", "t", "enter")
	send(t, m, "c", "r")
	// The marked and unmarked frames are rendered one after the other, so a
	// second boundary between them would read as marking having moved things.
	freezeClock(m)

	for _, tc := range []struct {
		name  string
		setup func()
	}{
		{"game", func() { m.screen = screenGame }},
		{"game with restart prompt", func() { m.screen, m.game.confirmNew = screenGame, true }},
		{"menu", func() { m.screen, m.game.confirmNew = screenMenu, false }},
		{"list", func() { m.list.reload(m.store); m.screen = screenList }},
		{"list with delete prompt", func() {
			m.list.reload(m.store)
			m.screen, m.list.confirmDelete = screenList, true
		}},
		{"profile", func() { m.profile.reload(m.store); m.screen = screenProfile }},
		{"themes", func() { m.themes.reload(m.themeLib, m.themeName); m.screen = screenThemes }},
		{"settings", func() { m.settings.reload(m.settingsOf()); m.screen = screenSettings }},
		{"about", func() { m.about.reload(m.dataDir); m.screen = screenAbout }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tc.setup()

			plain := m.frame(nil)
			h := &hitMap{}
			marked := h.scan(m.frame(h))

			if marked != plain {
				t.Errorf("marking changed the frame\n plain: %q\nmarked: %q", plain, marked)
			}
			if len(h.zones) == 0 {
				t.Error("no clickable regions on this screen")
			}
			if strings.Contains(marked, markerStart) {
				t.Error("markers left in the frame handed to the renderer")
			}
		})
	}
}

// TestClickTargetsMatchGlyphs is the geometry guard: the recorded rects must
// cover the elements they claim to. Every other test here looks its coordinates
// up through find(), so this is what catches a broken scan.
func TestClickTargetsMatchGlyphs(t *testing.T) {
	m := gameModel(t)
	send(t, m, "c", "r")
	frame := draw(t, m)

	for _, tc := range []struct {
		name string
		act  action
		want string
	}{
		{"keycap", action{kind: actLetter, letter: 'q'}, "  Q  "},
		{"enter key", action{kind: actSubmit}, "  ⏎  "},
		{"backspace key", action{kind: actBackspace}, "  ⌫  "},
		{"typed tile", action{kind: actTrim, index: 1}, "   R   "},
		{"close box", action{kind: actQuit}, "×"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r, ok := m.hits.find(tc.act)
			if !ok {
				t.Fatalf("nothing on screen for %+v", tc.act)
			}
			if got := at(t, frame, r); got != tc.want {
				t.Errorf("rect %+v covers %q, want %q", r, got, tc.want)
			}
		})
	}

	// Menu and list rows are wide targets, so check they contain their label
	// rather than matching it exactly.
	m.screen = screenMenu
	frame = draw(t, m)
	r, _ := m.hits.find(action{kind: actMenuChoice, index: 1})
	if got := at(t, frame, r); !strings.Contains(got, "5 letters") {
		t.Errorf("menu row 1 covers %q, want it to contain %q", got, "5 letters")
	}

	// The settings step arrows are single glyphs, so they pin scan down the way
	// the keycaps do on the board.
	m.settings.reload(m.settingsOf())
	m.screen = screenSettings
	frame = draw(t, m)
	if r, ok := m.hits.find(action{kind: actSettingPrev}); !ok {
		t.Error("no step-back target on the settings screen")
	} else if got := at(t, frame, r); got != st.glyph.ValuePrev {
		t.Errorf("step-back rect %+v covers %q, want %q", r, got, st.glyph.ValuePrev)
	}

	// The value and the › share one action — both step forward — so the value
	// is the first zone marked with it and the arrow is the last.
	next := action{kind: actSettingNext, index: rowRememberLast}
	first, _ := m.hits.find(next)
	if got := at(t, frame, first); !strings.Contains(got, "off") {
		t.Errorf("remember-last value covers %q, want it to contain %q", got, "off")
	}
	if got := at(t, frame, lastZone(t, m, next)); got != st.glyph.ValueNext {
		t.Errorf("step-forward rect covers %q, want %q", got, st.glyph.ValueNext)
	}
}

// lastZone is the final region recorded for an action, for the cases where one
// action is deliberately drawn twice.
func lastZone(t *testing.T, m *Model, a action) rect {
	t.Helper()
	for i := len(m.hits.zones) - 1; i >= 0; i-- {
		if m.hits.zones[i].act == a {
			return m.hits.zones[i].rect
		}
	}
	t.Fatalf("nothing on screen for %+v", a)
	return rect{}
}

// Both settings are changeable with the mouse alone, in both directions.
func TestSettingsByClickingOnly(t *testing.T) {
	m := newModel(t)
	m.settings.reload(m.settingsOf())
	m.screen = screenSettings

	// Clicking the value steps forward; the ‹ steps back to where it started.
	click(t, m, action{kind: actSettingNext, index: rowLength})
	if m.settings.length != 6 {
		t.Fatalf("length = %d after clicking ›, want 6", m.settings.length)
	}
	click(t, m, action{kind: actSettingPrev, index: rowLength})
	if m.settings.length != defaultLength {
		t.Errorf("length = %d after clicking ‹, want %d", m.settings.length, defaultLength)
	}

	click(t, m, action{kind: actSettingNext, index: rowRememberLast})
	if !m.settings.rememberLast {
		t.Error("clicking the remember-last row did not turn it on")
	}

	// And a click writes through, not just into the screen struct.
	s, ok := m.store.(settingsStore)
	if !ok {
		t.Fatal("test store does not keep settings")
	}
	if got := s.Settings(); !got.RememberLast || got.Length != defaultLength {
		t.Errorf("saved settings = %+v, want the clicked values", got)
	}
}

// Hovering a settings row moves the cursor onto it, so one click is enough to
// change the row you are pointing at.
func TestHoveringSettingsMovesTheCursor(t *testing.T) {
	m := newModel(t)
	m.settings.reload(m.settingsOf())
	m.screen = screenSettings

	point(t, m, action{kind: actSettingNext, index: rowRememberLast})
	if m.settings.cursor != rowRememberLast {
		t.Errorf("cursor = %d after hovering the second row, want %d", m.settings.cursor, rowRememberLast)
	}
}

// A full puzzle must be playable with nothing but the mouse.
func TestPlayingByClickingOnly(t *testing.T) {
	m := gameModel(t)

	clickWord(t, m, "about")
	if m.game.typing != "about" {
		t.Fatalf("typing = %q after clicking keys, want %q", m.game.typing, "about")
	}

	click(t, m, action{kind: actSubmit})
	if m.game.g.Attempts() != 1 {
		t.Fatalf("Attempts() = %d after clicking ⏎, want 1", m.game.g.Attempts())
	}
	if m.game.typing != "" {
		t.Errorf("input not cleared after submit: %q", m.game.typing)
	}

	clickWord(t, m, "crane")
	click(t, m, action{kind: actSubmit})
	if m.game.g.Status != game.Won {
		t.Fatalf("status = %v, want won", m.game.g.Status)
	}
	if !strings.Contains(m.View().Content, "solved in 2") {
		t.Error("missing win message after a mouse-only game")
	}
}

func TestClickingBackspaceAndTilesEdits(t *testing.T) {
	m := gameModel(t)
	clickWord(t, m, "crane")

	click(t, m, action{kind: actBackspace})
	if m.game.typing != "cran" {
		t.Errorf("typing = %q after clicking ⌫, want %q", m.game.typing, "cran")
	}

	// Clicking a typed tile erases the row back to that slot.
	click(t, m, action{kind: actTrim, index: 1})
	if m.game.typing != "c" {
		t.Errorf("typing = %q after clicking tile 1, want %q", m.game.typing, "c")
	}

	// An empty slot is not a target at all.
	draw(t, m)
	if _, ok := m.hits.find(action{kind: actTrim, index: 3}); ok {
		t.Error("an untyped slot should not be clickable")
	}
}

// The help bar doubles as the button bar: new puzzle and menu must work from it.
func TestHelpBarButtons(t *testing.T) {
	m := gameModel(t)
	send(t, m, "a", "b", "o", "u", "t", "enter") // make the puzzle worth saving
	first := m.game.g.ID

	click(t, m, action{kind: actNewPuzzle})
	if m.game.g.ID == first {
		t.Error("clicking new puzzle did not start one")
	}

	// The fresh puzzle is unplayed, so leaving must not save it: the peek/commit
	// split has to survive the click path too.
	click(t, m, action{kind: actBack})
	if m.screen != screenMenu {
		t.Fatalf("screen = %v after clicking menu, want menu", m.screen)
	}
	list, _ := m.store.List()
	if len(list) != 1 {
		t.Errorf("List() has %d entries, want 1 — the unplayed puzzle was saved", len(list))
	}
}

// The armed tab+enter prompt is clickable in both directions.
func TestClickingRestartPrompt(t *testing.T) {
	m := gameModel(t)
	send(t, m, "tab")
	first := m.game.g.ID

	click(t, m, action{kind: actCancelNew})
	if m.game.confirmNew {
		t.Error("clicking esc did not dismiss the prompt")
	}
	if m.game.g.ID != first {
		t.Error("clicking esc started a new puzzle anyway")
	}

	send(t, m, "tab")
	click(t, m, action{kind: actNewPuzzle})
	if m.game.g.ID == first {
		t.Error("clicking enter at the prompt did not start a new puzzle")
	}
}

func TestMenuAndListClicks(t *testing.T) {
	m := newModel(t)

	// Start a 6-letter game by clicking its menu row.
	click(t, m, action{kind: actMenuChoice, index: menuIndex(t, m, choiceNewGame, 6)})
	if m.screen != screenGame || m.game.g.Length != 6 {
		t.Fatalf("screen = %v, length = %d; want a 6-letter game", m.screen, m.game.g.Length)
	}

	// Play it enough to be listed, then come back to the menu.
	m.game.g.Answer = "sample"
	send(t, m, "b", "o", "t", "t", "l", "e", "enter")
	id := m.game.g.ID
	click(t, m, action{kind: actBack})

	// Open the puzzle list, then the puzzle, both with one click each.
	click(t, m, action{kind: actMenuChoice, index: menuIndex(t, m, choiceList, 0)})
	if m.screen != screenList {
		t.Fatalf("screen = %v, want list", m.screen)
	}
	click(t, m, action{kind: actListRow, index: 0})
	if m.screen != screenGame {
		t.Fatalf("screen = %v after clicking a row, want game", m.screen)
	}
	if m.game.g.ID != id {
		t.Errorf("opened %q, want %q", m.game.g.ID, id)
	}
}

// Deleting has to be reachable with a mouse alone, like every other keybind.
// Unlike the restart prompt, a click does not skip the confirmation: the first
// click arms it, and the prompt's own target carries it out.
func TestDeletingByClickingOnly(t *testing.T) {
	m := newModel(t)
	playSome(t, m, 2)
	m.list.reload(m.store)
	m.screen = screenList
	draw(t, m)

	doomed := m.list.items[0].ID
	click(t, m, action{kind: actDeletePuzzle, index: 0})
	if list, _ := m.store.List(); len(list) != 2 {
		t.Fatalf("one click deleted a puzzle without confirming: %v", list)
	}
	if !m.list.confirmDelete {
		t.Fatal("clicking delete did not arm the prompt")
	}

	// The confirm target is the prompt, which is somewhere else on screen.
	click(t, m, action{kind: actDeletePuzzle, index: 0})

	list, _ := m.store.List()
	if len(list) != 1 {
		t.Fatalf("List has %d entries after confirming, want 1", len(list))
	}
	if list[0].ID == doomed {
		t.Errorf("deleted the wrong puzzle: %q survived", doomed)
	}
	if m.list.confirmDelete {
		t.Error("prompt still armed after the delete went through")
	}
}

func TestClickingCancelKeepsThePuzzle(t *testing.T) {
	m := newModel(t)
	playSome(t, m, 1)
	m.list.reload(m.store)
	m.screen = screenList
	draw(t, m)

	click(t, m, action{kind: actDeletePuzzle, index: 0})
	click(t, m, action{kind: actCancelDelete})

	if list, _ := m.store.List(); len(list) != 1 {
		t.Errorf("cancelling deleted anyway: %v", list)
	}
	if m.list.confirmDelete {
		t.Error("prompt still armed after cancelling")
	}
}

func TestProfileReachableAndDismissableByMouse(t *testing.T) {
	m := newModel(t)
	click(t, m, action{kind: actMenuChoice, index: menuIndex(t, m, choiceProfile, 0)})
	if m.screen != screenProfile {
		t.Fatalf("screen = %v, want profile", m.screen)
	}
	click(t, m, action{kind: actBack})
	if m.screen != screenMenu {
		t.Errorf("screen = %v after clicking menu, want menu", m.screen)
	}
}

// The clock starts on the first letter whichever way it was typed, since both
// paths go through typeLetter.
func TestKeycapClickStartsTheClock(t *testing.T) {
	m := gameModel(t)
	draw(t, m)
	if !m.game.sessionStart.IsZero() {
		t.Fatal("clock running before anything was typed")
	}

	clickWord(t, m, "c")
	if m.game.sessionStart.IsZero() {
		t.Error("clicking a keycap did not start the clock")
	}
}

func TestAboutReachableAndDismissableByMouse(t *testing.T) {
	m := newModel(t)
	click(t, m, action{kind: actMenuChoice, index: menuIndex(t, m, choiceAbout, 0)})
	if m.screen != screenAbout {
		t.Fatalf("screen = %v, want about", m.screen)
	}
	click(t, m, action{kind: actBack})
	if m.screen != screenMenu {
		t.Errorf("screen = %v after clicking menu, want menu", m.screen)
	}
}

// The close box quits, and quitting mid-puzzle still saves.
func TestCloseBoxQuits(t *testing.T) {
	m := gameModel(t)
	send(t, m, "a", "b", "o", "u", "t", "enter")

	click(t, m, action{kind: actQuit})
	if !m.quitting {
		t.Fatal("clicking × did not quit")
	}
	if list, _ := m.store.List(); len(list) != 1 {
		t.Errorf("List() has %d entries after quitting mid-puzzle, want 1", len(list))
	}
}

func TestHoverFollowsPointer(t *testing.T) {
	m := newModel(t)

	point(t, m, action{kind: actMenuChoice, index: 2})
	if m.menu.cursor != 2 {
		t.Errorf("menu cursor = %d after hovering row 2, want 2", m.menu.cursor)
	}
	if m.hover != (action{kind: actMenuChoice, index: 2}) {
		t.Errorf("hover = %+v, want menu row 2", m.hover)
	}

	// Hovering nothing clears the highlight but leaves the selection alone.
	m.Update(tea.MouseMotionMsg{X: 0, Y: 0})
	if m.hover.kind != actNone {
		t.Errorf("hover = %+v over empty space, want none", m.hover)
	}
	if m.menu.cursor != 2 {
		t.Errorf("menu cursor = %d, want it to stay at 2", m.menu.cursor)
	}

	// A hovered keycap renders differently from an idle one.
	g := gameModel(t)
	idle := draw(t, g)
	point(t, g, action{kind: actLetter, letter: 'q'})
	if hovered := draw(t, g); hovered == idle {
		t.Error("hovering a keycap changed nothing on screen")
	}
}

func TestWheelScrollsTheList(t *testing.T) {
	s, err := store.NewJSON(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	// More puzzles than fit, so there is something to scroll.
	for range visibleRows + 3 {
		g, err := newPuzzle(s, 5)
		if err != nil {
			t.Fatal(err)
		}
		if err := s.Save(g); err != nil {
			t.Fatal(err)
		}
	}

	m := New(s, nil, Options{})
	m.list.reload(s)
	m.screen = screenList
	draw(t, m)

	m.Update(tea.MouseWheelMsg{Button: tea.MouseWheelDown})
	m.Update(tea.MouseWheelMsg{Button: tea.MouseWheelDown})
	if m.list.offset != 2 {
		t.Errorf("offset = %d after two wheel-downs, want 2", m.list.offset)
	}
	if m.list.cursor != 0 {
		t.Errorf("cursor = %d, want the wheel to scroll without moving it", m.list.cursor)
	}

	for range 5 {
		m.Update(tea.MouseWheelMsg{Button: tea.MouseWheelUp})
	}
	if m.list.offset != 0 {
		t.Errorf("offset = %d after scrolling back up, want 0", m.list.offset)
	}
}

// Stray mouse input must never act: only a left press on a target does.
func TestNonActingMouseInput(t *testing.T) {
	m := gameModel(t)
	r := target(t, m, action{kind: actLetter, letter: 'q'})

	m.Update(tea.MouseClickMsg{X: r.x, Y: r.y, Button: tea.MouseRight})
	m.Update(tea.MouseReleaseMsg{X: r.x, Y: r.y, Button: tea.MouseLeft})
	m.Update(tea.MouseClickMsg{X: 0, Y: 0, Button: tea.MouseLeft})

	if m.game.typing != "" {
		t.Errorf("typing = %q; right-click, release and empty space must all do nothing", m.game.typing)
	}
}
