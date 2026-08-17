package ui

import (
	"regexp"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/nxck2005/surmise/internal/game"
	"github.com/nxck2005/surmise/internal/store"
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

	result, err := game.NewFrom("result-layout", "crane", 5)
	if err != nil {
		t.Fatal(err)
	}
	if err := result.Guess("crane"); err != nil {
		t.Fatal(err)
	}

	for _, tc := range []struct {
		name  string
		setup func()
	}{
		{"game", func() { m.screen = screenGame }},
		{"game with restart prompt", func() { m.screen, m.game.confirmNew = screenGame, true }},
		{"result", func() { m.result.open(result, ""); m.screen = screenResult }},
		{"menu", func() { m.screen, m.game.confirmNew = screenMenu, false }},
		{"list", func() { m.list.reload(m.store); m.screen = screenList }},
		{"list with delete prompt", func() {
			m.list.reload(m.store)
			m.screen, m.list.confirmDelete = screenList, true
		}},
		{"daily", func() { m.daily.reload(m.store, m.day); m.screen = screenDaily }},
		{"daily with a finished trio", func() {
			m.daily.reload(m.store, m.day)
			// A day whose modes are all done, so the card and its copy button
			// are on screen. The rows are what the screen renders from.
			for i := range m.daily.rows {
				m.daily.rows[i].status = game.Won
				m.daily.rows[i].attempts, m.daily.rows[i].maxAttempts = 3, 6
				m.daily.rows[i].spent = false
			}
			m.screen = screenDaily
		}},
		{"profile", func() { m.profile.reload(m.store, m.day, "", 0); m.screen = screenProfile }},
		{"themes", func() { m.themes.reload(m.themeLib, m.themeName); m.screen = screenThemes }},
		{"settings", func() { m.settings.reload(m.settingsOf()); m.screen = screenSettings }},
		{"how to play", func() { m.howTo.reset(); m.screen = screenHowTo }},
		{"how to play, a middle page", func() { m.howTo.show(1); m.screen = screenHowTo }},
		{"about", func() { m.about.reload(m.dataDir); m.screen = screenAbout }},
		{"backup", func() { m.backup.reset(); m.screen = screenBackup }},
		{"splash", func() { m.raiseSplash(); m.screen = screenSplash }},
		{"splash waiting for a key", func() {
			m.splash.mode = splashKey
			m.raiseSplash()
			m.screen = screenSplash
		}},
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

func TestProfileDisplayNameEditorHasMouseControls(t *testing.T) {
	m := newModel(t)
	m.settings.reload(m.settingsOf())
	m.screen = screenSettings
	m.settings.cursor = rowProfileName

	click(t, m, action{kind: actFieldEdit, index: rowProfileName})
	if !m.settings.name.editing {
		t.Fatal("clicking the profile name did not start editing")
	}
	send(t, m, "n", "i", "x")
	click(t, m, action{kind: actFieldBackspace, index: rowProfileName})
	click(t, m, action{kind: actFieldDone, index: rowProfileName})

	if m.settings.name.editing {
		t.Fatal("clicking save left the name editor open")
	}
	s := m.store.(settingsStore)
	if got := s.Settings().DisplayName; got != "ni" {
		t.Errorf("saved display name = %q, want ni", got)
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
	if m.screen != screenResult {
		t.Fatalf("screen after mouse-only win = %v, want result", m.screen)
	}
	if !strings.Contains(m.View().Content, "solved in 2") {
		t.Error("missing win message after a mouse-only game")
	}

	first := m.game.g.ID
	click(t, m, action{kind: actResultReview})
	if m.screen != screenGame {
		t.Fatalf("screen after clicking review = %v, want game", m.screen)
	}
	click(t, m, action{kind: actSubmit})
	if m.screen != screenResult {
		t.Fatalf("screen after clicking result on the board = %v, want result", m.screen)
	}
	click(t, m, action{kind: actResultCopy})
	if !m.result.copyRequested {
		t.Fatal("clicking copy did not record its acknowledgement")
	}
	click(t, m, action{kind: actResultNext})
	if m.screen != screenGame || m.game.g.ID == first {
		t.Errorf("clicking next left screen %v with puzzle %q", m.screen, m.game.g.ID)
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
	m := New(s, nil, Options{})
	m.screen = screenList
	draw(t, m)

	// More puzzles than the window holds at this terminal size, so there is
	// something to scroll. The window follows the height now, so ask it.
	for range m.list.rows() + 3 {
		g, err := newPuzzle(s, 5)
		if err != nil {
			t.Fatal(err)
		}
		if err := s.Save(g); err != nil {
			t.Fatal(err)
		}
	}
	m.list.reload(s)
	draw(t, m)
	if len(m.list.items) <= m.list.rows() {
		t.Fatalf("%d puzzles fit in a %d-row window; nothing to scroll",
			len(m.list.items), m.list.rows())
	}

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

// drawAt sizes the terminal to a specific height and renders, for the cases
// where the frame does not fit. The width stays roomy so only the height is
// under test.
func drawAt(t *testing.T, m *Model, height int) string {
	t.Helper()
	m.Update(tea.WindowSizeMsg{Width: testWidth, Height: height})
	return m.View().Content
}

// A frame taller than the terminal is not shown from the top: Bubble Tea's
// renderer drops the excess lines off the top of the buffer. Nothing in the
// pipeline truncates before then — lipgloss.PlaceVertical returns an over-tall
// block unchanged — so without clip every recorded region is wrong by exactly
// the overflow, and every click on a short terminal lands somewhere else.
//
// This is checked against the glyphs the *terminal* shows, which is the last
// `height` lines of the frame.
func TestClickTargetsSurviveAnOverflowingFrame(t *testing.T) {
	m := newModel(t)
	m.screen = screenMenu

	// A height the menu cannot fit in, so the frame overflows.
	const height = 12
	frame := drawAt(t, m, height)
	lines := strings.Split(frame, "\n")
	if len(lines) <= height {
		t.Fatalf("frame is %d lines at height %d; it must overflow for this test",
			len(lines), height)
	}
	visible := strings.Join(lines[len(lines)-height:], "\n")

	// The quit row is near the bottom, so it survives the clipping.
	quit := action{kind: actMenuChoice, index: menuIndex(t, m, choiceQuit, 0)}
	r, ok := m.hits.find(quit)
	if !ok {
		t.Fatal("the quit row has no click target")
	}
	if got := at(t, visible, r); !strings.Contains(got, "quit") {
		t.Errorf("the quit target covers %q on the visible screen, want the quit row", got)
	}

	// And a click there really does reach it, rather than whatever the
	// unclipped coordinates pointed at.
	m.Update(tea.MouseClickMsg{X: r.x + r.w/2, Y: r.y + r.h/2, Button: tea.MouseLeft})
	if !m.quitting {
		t.Error("clicking the quit row on a short terminal did not quit")
	}
}

// What the terminal cuts off, the hit map must forget: a region scrolled above
// the first visible row is not somewhere the player can click.
func TestClippedTargetsAreDropped(t *testing.T) {
	m := newModel(t)
	m.screen = screenMenu

	drawAt(t, m, testHeight)
	roomy := len(m.hits.zones)
	if roomy == 0 {
		t.Fatal("the menu has no targets at full height")
	}

	// Squeezed, the top of the frame is gone and so are the targets that were
	// drawn there — the close box in the panel's border, and the first rows.
	const height = 8
	drawAt(t, m, height)
	if len(m.hits.zones) >= roomy {
		t.Errorf("%d targets at height %d, %d at height %d: nothing was clipped",
			len(m.hits.zones), height, roomy, testHeight)
	}
	// Whatever survives has to be somewhere the player can actually reach.
	for _, z := range m.hits.zones {
		if z.rect.y < 0 || z.rect.y >= height {
			t.Errorf("zone %+v lies outside the %d visible rows", z, height)
		}
	}
	// Nothing may answer for the top-left corner just because it was never
	// scanned: an unpositioned zone is not a target.
	if a, ok := m.hits.at(0, 0); ok {
		t.Errorf("cell (0,0) resolves to %+v; the corner must not be a phantom target", a)
	}
}

// A screen that outgrows the terminal loses its top rows to the renderer, so
// the tall screens shed their extras first. The headline figures never go: they
// are what the profile is for.
func TestProfileShedsExtrasOnAShortTerminal(t *testing.T) {
	m := newModel(t)
	send(t, m, "down", "enter")
	m.game.g.Answer = "crane"
	send(t, m, "c", "r", "a", "n", "e", "enter")
	send(t, m, "esc")
	m.profile.reload(m.store, m.day, "", 0)
	m.screen = screenProfile

	roomy := drawAt(t, m, testHeight)
	for _, want := range []string{"win rate", "guess distribution", "by mode"} {
		if !strings.Contains(roomy, want) {
			t.Fatalf("a roomy terminal should show %q\n%s", want, roomy)
		}
	}

	short := drawAt(t, m, 14)
	if !strings.Contains(short, "win rate") {
		t.Errorf("the headline figures must survive\n%s", short)
	}
	if strings.Contains(short, "by mode") {
		t.Errorf("the per-mode table should have been shed\n%s", short)
	}
	if h := lipgloss.Height(short); h > lipgloss.Height(roomy) {
		t.Errorf("the short frame is %d lines, taller than the roomy one", h)
	}
}

// The about screen gives up its credits before anything a bug report needs.
func TestAboutShedsCreditsOnAShortTerminal(t *testing.T) {
	m := newModel(t)
	m.about.reload("")
	m.screen = screenAbout

	if roomy := drawAt(t, m, testHeight); !strings.Contains(roomy, "version") {
		t.Fatalf("the about screen lost its version row\n%s", roomy)
	}
	short := drawAt(t, m, 12)
	if !strings.Contains(short, "version") {
		t.Errorf("version must survive a short terminal\n%s", short)
	}
}

// The window follows the terminal instead of being a fixed twelve rows, so a
// tall terminal shows more and a short one does not overflow.
func TestListWindowFollowsTheTerminal(t *testing.T) {
	s, err := store.NewJSON(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	for range 30 {
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

	tall := drawAt(t, m, 40)
	short := drawAt(t, m, 16)
	if m.list.rows() >= 40 {
		t.Errorf("a 16-row terminal still asks for %d rows", m.list.rows())
	}
	if lipgloss.Height(short) >= lipgloss.Height(tall) {
		t.Errorf("the short frame (%d lines) is no shorter than the tall one (%d)",
			lipgloss.Height(short), lipgloss.Height(tall))
	}
	// Whatever it draws has to fit the terminal it was told about, or the
	// renderer eats the top of it.
	if h := lipgloss.Height(short); h > 16 {
		t.Errorf("the frame is %d lines on a 16-row terminal", h)
	}
}

// A wheel scroll must survive the next keystroke. clampOffset belongs to
// whatever moved the cursor; running it after every key dragged the window
// straight back to the selection.
func TestKeypressDoesNotUndoAWheelScroll(t *testing.T) {
	s, err := store.NewJSON(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	m := New(s, nil, Options{})
	m.screen = screenList
	draw(t, m)
	for range m.list.rows() + 5 {
		g, err := newPuzzle(s, 5)
		if err != nil {
			t.Fatal(err)
		}
		if err := s.Save(g); err != nil {
			t.Fatal(err)
		}
	}
	m.list.reload(s)
	draw(t, m)

	m.Update(tea.MouseWheelMsg{Button: tea.MouseWheelDown})
	m.Update(tea.MouseWheelMsg{Button: tea.MouseWheelDown})
	if m.list.offset != 2 {
		t.Fatalf("offset = %d after two wheel-downs, want 2", m.list.offset)
	}

	// A key that does not move the cursor must leave the window where it is.
	send(t, m, "x")
	if m.list.offset != 2 {
		t.Errorf("offset = %d after an unrelated key, want the scroll kept", m.list.offset)
	}
	// One that does move it may of course pull the window back.
	send(t, m, "down")
	if m.list.cursor != 1 {
		t.Errorf("cursor = %d, want the key to have moved it", m.list.cursor)
	}
	// clampOffset scrolls just far enough to show the cursor, so the window
	// stops at it rather than snapping back to the top.
	if m.list.offset != 1 {
		t.Errorf("offset = %d; moving the cursor should scroll just enough to show it",
			m.list.offset)
	}
}

// home and end were the one hard parity gap: keys with nothing on screen to
// click. The counter under a scrolling list carries them now.
func TestJumpTargetsMatchHomeAndEnd(t *testing.T) {
	s, err := store.NewJSON(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	m := New(s, nil, Options{})
	m.screen = screenList
	draw(t, m)
	for range m.list.rows() + 5 {
		g, err := newPuzzle(s, 5)
		if err != nil {
			t.Fatal(err)
		}
		if err := s.Save(g); err != nil {
			t.Fatal(err)
		}
	}
	m.list.reload(s)
	last := len(m.list.items) - 1

	// The keys, for the behaviour the clicks have to match.
	send(t, m, "end")
	if m.list.cursor != last {
		t.Fatalf("end put the cursor at %d, want %d", m.list.cursor, last)
	}
	send(t, m, "home")
	if m.list.cursor != 0 {
		t.Fatalf("home put the cursor at %d, want 0", m.list.cursor)
	}

	click(t, m, action{kind: actJumpBottom})
	if m.list.cursor != last {
		t.Errorf("clicking the jump-to-end target put the cursor at %d, want %d",
			m.list.cursor, last)
	}
	click(t, m, action{kind: actJumpTop})
	if m.list.cursor != 0 {
		t.Errorf("clicking the jump-to-start target put the cursor at %d, want 0",
			m.list.cursor)
	}

	// And they cover the glyphs they claim to.
	frame := draw(t, m)
	r, ok := m.hits.find(action{kind: actJumpTop})
	if !ok {
		t.Fatal("no jump-to-start target")
	}
	if got := at(t, frame, r); got != st.glyph.JumpFirst {
		t.Errorf("the jump-to-start rect covers %q, want %q", got, st.glyph.JumpFirst)
	}
}

// A list short enough to fit has no counter, so it has no jump targets either —
// there is nowhere to jump to.
func TestNoJumpTargetsWithoutScrolling(t *testing.T) {
	m := newModel(t)
	send(t, m, "down", "enter")
	m.game.g.Answer = "crane"
	send(t, m, "c", "r", "a", "n", "e", "enter")
	send(t, m, "esc")
	m.list.reload(m.store)
	m.screen = screenList
	draw(t, m)

	if _, ok := m.hits.find(action{kind: actJumpTop}); ok {
		t.Error("a one-row list offers a jump-to-start target")
	}
}

// The settings help bar advertises ← and →, so both have to be buttons.
func TestSettingsHelpBarStepsBothWays(t *testing.T) {
	m := newModel(t)
	m.settings.reload(m.settingsOf())
	m.screen = screenSettings
	draw(t, m)

	before := m.settings.length
	click(t, m, action{kind: actSettingNext, index: rowLength})
	if m.settings.length == before {
		t.Fatalf("stepping forward left the mode at %d", before)
	}
	// The help bar's back button is the last zone carrying the action, since the
	// row's own ‹ glyph is marked first.
	r := lastZone(t, m, action{kind: actSettingPrev, index: rowLength})
	m.Update(tea.MouseClickMsg{X: r.x + r.w/2, Y: r.y + r.h/2, Button: tea.MouseLeft})
	if got := m.settings.length; got != before {
		t.Errorf("mode = %d after stepping back, want %d", got, before)
	}
}

// The help bar repeats what the screen already offers — "enter select" carries
// the very action the selected row does — and the action doubles as the hover
// key. Pointing at a row used to light the button up too, which reads as the
// pointer being in two places at once.
func TestHoveringARowLeavesTheHelpBarAlone(t *testing.T) {
	m := newModel(t)
	m.screen = screenMenu

	row := action{kind: actMenuChoice, index: 2}
	helpButton := action{kind: actMenuChoice, index: 2, help: true}

	point(t, m, row)
	if m.hover != row {
		t.Fatalf("hover = %+v after pointing at row 2, want the row", m.hover)
	}

	// The frame the pointer produced: the row is lit, the button is not.
	draw(t, m)
	h := m.hits
	if !h.hovered(row) {
		t.Error("the row under the pointer is not hovered")
	}
	if h.hovered(helpButton) {
		t.Error("the help bar's button lit up while the pointer was on a row")
	}

	// And in the bytes, not just the predicate: the help bar has to come out
	// of a hover exactly as it went in. The idle snapshot has to be taken with
	// the pointer off everything, or it is not idle.
	m.Update(tea.MouseMotionMsg{X: 0, Y: 0})
	idle := helpLine(t, drawn(t, m))
	point(t, m, row)
	if got := helpLine(t, drawn(t, m)); got != idle {
		t.Errorf("the help bar changed while the pointer was on a row\n idle: %q\nhover: %q",
			idle, got)
	}

	// Both are still targets, and both still do the same thing.
	if _, ok := h.find(row); !ok {
		t.Error("the row lost its click target")
	}
	click(t, m, action{kind: actMenuChoice, index: menuIndex(t, m, choiceProfile, 0)})
	if m.screen != screenProfile {
		t.Error("clicking a menu row stopped working")
	}
}

// And the other way round: a pointer on the button lights the button, not the
// row it happens to name.
func TestHoveringTheHelpBarLeavesTheRowAlone(t *testing.T) {
	m := newModel(t)
	m.screen = screenMenu
	draw(t, m)

	// find ignores which copy of an action it returns, and the row is marked
	// first, so ask for the bar's copy exactly.
	r := lastZone(t, m, action{kind: actMenuChoice, index: m.menu.cursor, help: true})
	m.Update(tea.MouseMotionMsg{X: r.x + r.w/2, Y: r.y + r.h/2})

	draw(t, m)
	if !m.hits.hovered(action{kind: actMenuChoice, index: m.menu.cursor, help: true}) {
		t.Error("the help-bar button under the pointer is not hovered")
	}
	if m.hits.hovered(action{kind: actMenuChoice, index: m.menu.cursor}) {
		t.Error("the menu row lit up while the pointer was on the help bar")
	}
}

// drawn renders and returns the model, so a frame can be taken inline.
func drawn(t *testing.T, m *Model) *Model {
	t.Helper()
	draw(t, m)
	return m
}

// helpLine is the bottom hint line of the current frame, styling included.
func helpLine(t *testing.T, m *Model) string {
	t.Helper()
	lines := strings.Split(m.View().Content, "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		if strings.Contains(sgr.ReplaceAllString(lines[i], ""), "select") {
			return lines[i]
		}
	}
	t.Fatal("no help bar in the frame")
	return ""
}
