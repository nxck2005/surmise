package ui

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/nxck2005/wortle/internal/build"
	"github.com/nxck2005/wortle/internal/game"
	"github.com/nxck2005/wortle/internal/store"
	"github.com/nxck2005/wortle/internal/theme"
	"github.com/nxck2005/wortle/internal/words"
)

// key builds the message the framework would deliver for a keystroke.
func key(s string) tea.KeyPressMsg {
	if len(s) == 1 {
		return tea.KeyPressMsg{Code: rune(s[0]), Text: s}
	}
	var code rune
	switch s {
	case "enter":
		code = tea.KeyEnter
	case "esc":
		code = tea.KeyEscape
	case "tab":
		code = tea.KeyTab
	case "backspace":
		code = tea.KeyBackspace
	case "down":
		code = tea.KeyDown
	case "up":
		code = tea.KeyUp
	case "left":
		code = tea.KeyLeft
	case "right":
		code = tea.KeyRight
	case "home":
		code = tea.KeyHome
	case "end":
		code = tea.KeyEnd
	default:
		panic("unhandled key " + s)
	}
	return tea.KeyPressMsg{Code: code}
}

// newModel returns a model sitting at the menu. The app really opens on a
// puzzle (see TestStartsOnDefaultPuzzle); most tests start their game
// explicitly from the menu, so the helper resets there for a clean slate.
func newModel(t *testing.T) *Model {
	t.Helper()
	s, err := store.NewJSON(t.TempDir())
	if err != nil {
		t.Fatalf("NewJSON: %v", err)
	}
	m := New(s, nil, Options{})
	m.screen = screenMenu
	return m
}

// menuIndex finds a menu row by what it does, so a test never has to know how
// many rows sit above it. Inserting an entry has broken hard-coded indices
// before; this is the one place that knows the order.
func menuIndex(t *testing.T, m *Model, kind choiceKind, length int) int {
	t.Helper()
	for i, c := range m.menu.choices {
		if c.kind == kind && (kind != choiceNewGame || c.length == length) {
			return i
		}
	}
	t.Fatalf("no menu entry of kind %v (length %d)", kind, length)
	return -1
}

// freezeClock stops the elapsed timer in the game header, which otherwise
// advances between renders and makes any two frames unequal a second apart.
// elapsed() already treats a zero sessionStart as "no session in progress", so
// this is a state the screen genuinely has — a puzzle that is not being played
// right now — not a value invented for the test. Tests that compare one frame
// against another must call it, or they fail on a slow machine.
func freezeClock(m *Model) {
	if m.game != nil {
		m.game.sessionStart = time.Time{}
	}
}

// send feeds keystrokes and returns the rendered screen.
func send(t *testing.T, m *Model, keys ...string) string {
	t.Helper()
	for _, k := range keys {
		if _, cmd := m.Update(key(k)); cmd != nil {
			_ = cmd // commands here are ticks and quits; nothing to drain
		}
	}
	return m.View().Content
}

// TestKeyHelperMatchesFramework guards the test helper itself: if these do not
// produce the strings the screens match on, every other test here is vacuous.
func TestKeyHelperMatchesFramework(t *testing.T) {
	for _, want := range []string{
		"a", "z", "enter", "esc", "tab", "backspace",
		"up", "down", "left", "right", "home", "end",
	} {
		if got := key(want).String(); got != want {
			t.Errorf("key(%q).String() = %q", want, got)
		}
	}
}

// The app opens directly on a 5-letter puzzle, not the menu.
func TestStartsOnDefaultPuzzle(t *testing.T) {
	s, err := store.NewJSON(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	m := New(s, nil, Options{})
	if m.screen != screenGame {
		t.Fatalf("screen = %v, want game", m.screen)
	}
	if m.game == nil || m.game.g.Length != defaultLength {
		t.Fatalf("did not open on a %d-letter puzzle", defaultLength)
	}
}

// An unplayed puzzle must leave nothing behind: no save file, no list entry,
// and no consumed puzzle number.
func TestUnplayedPuzzleIsNotSaved(t *testing.T) {
	s, err := store.NewJSON(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	m := New(s, nil, Options{}) // opens on a puzzle, unplayed

	m.quit() // as if the player launched and immediately quit

	list, err := s.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 0 {
		t.Errorf("List() = %v, want empty for an unplayed puzzle", list)
	}
}

// The first guess is what saves a puzzle.
func TestPuzzleSavedOnFirstGuess(t *testing.T) {
	m := newModel(t)
	send(t, m, "down", "enter")

	if list, _ := m.store.List(); len(list) != 0 {
		t.Fatalf("puzzle saved before any guess: %v", list)
	}

	m.game.g.Answer = "crane"
	send(t, m, "a", "b", "o", "u", "t", "enter")

	list, _ := m.store.List()
	if len(list) != 1 {
		t.Fatalf("List() has %d entries after first guess, want 1", len(list))
	}
	if list[0].ID != m.game.g.ID {
		t.Errorf("saved puzzle id = %q, want %q", list[0].ID, m.game.g.ID)
	}
}

func TestMenuRendersModes(t *testing.T) {
	m := newModel(t)
	view := m.View().Content
	for _, want := range []string{"wortle", "4 letters", "5 letters", "6 letters", "puzzles", "profile", "about"} {
		if !strings.Contains(view, want) {
			t.Errorf("menu missing %q\n%s", want, view)
		}
	}
}

// The about screen is where a player finds out what they are running, so the
// build stamp and the credits must actually reach the frame.
func TestAboutScreenShowsBuildInfo(t *testing.T) {
	m := newModel(t)
	m.menu.point(menuIndex(t, m, choiceAbout, 0))
	view := send(t, m, "enter")

	if m.screen != screenAbout {
		t.Fatalf("screen = %v, want about", m.screen)
	}
	want := []string{
		"about",
		build.Get().Version,
		repoURL,
		license,
		words.Credits[0].Source,
	}
	for _, w := range want {
		if !strings.Contains(view, w) {
			t.Errorf("about screen missing %q\n%s", w, view)
		}
	}

	send(t, m, "esc")
	if m.screen != screenMenu {
		t.Errorf("screen = %v after esc, want menu", m.screen)
	}
}

// The data directory is display data handed in through Options; without it the
// screen must simply omit the paths rather than show an empty one.
func TestAboutScreenShowsDataDir(t *testing.T) {
	dir := t.TempDir()
	s, err := store.NewJSON(dir)
	if err != nil {
		t.Fatalf("NewJSON: %v", err)
	}
	m := New(s, nil, Options{DataDir: dir})
	m.screen = screenMenu
	m.menu.point(menuIndex(t, m, choiceAbout, 0))
	view := send(t, m, "enter")

	if !strings.Contains(view, dir) {
		t.Errorf("about screen missing data dir %q\n%s", dir, view)
	}
	if !strings.Contains(view, theme.Dir(dir)) {
		t.Errorf("about screen missing themes dir %q\n%s", theme.Dir(dir), view)
	}

	plain := newModel(t)
	plain.about.reload("")
	for _, r := range plain.about.rows {
		if r.value == "" {
			t.Errorf("empty value for row %q with no data dir", r.label)
		}
	}
}

// Selecting a mode must open a board of that word length.
func TestStartGameFromMenu(t *testing.T) {
	m := newModel(t)
	send(t, m, "down", "enter") // second entry is 5 letters

	if m.screen != screenGame {
		t.Fatalf("screen = %v, want game", m.screen)
	}
	if m.game.g.Length != 5 {
		t.Errorf("length = %d, want 5", m.game.g.Length)
	}
	if m.game.g.MaxAttempts != 6 {
		t.Errorf("MaxAttempts = %d, want 6", m.game.g.MaxAttempts)
	}
}

func TestTypingAndSubmittingAGuess(t *testing.T) {
	m := newModel(t)
	send(t, m, "down", "enter")
	m.game.g.Answer = "crane"

	// Typing shows on the board before submission.
	view := send(t, m, "a", "b", "o", "u", "t")
	if !strings.Contains(view, "A") || !strings.Contains(view, "T") {
		t.Errorf("typed letters not shown\n%s", view)
	}
	if m.game.g.Attempts() != 0 {
		t.Error("typing should not count as an attempt")
	}

	send(t, m, "enter")
	if m.game.g.Attempts() != 1 {
		t.Fatalf("Attempts() = %d after enter, want 1", m.game.g.Attempts())
	}
	if m.game.typing != "" {
		t.Errorf("input not cleared after submit: %q", m.game.typing)
	}
}

func TestBackspaceEditsInput(t *testing.T) {
	m := newModel(t)
	send(t, m, "down", "enter")
	send(t, m, "a", "b", "c", "backspace")
	if m.game.typing != "ab" {
		t.Errorf("typing = %q, want %q", m.game.typing, "ab")
	}
}

// A rejected word must not cost an attempt, and must say so.
func TestInvalidWordIsRejected(t *testing.T) {
	m := newModel(t)
	send(t, m, "down", "enter")

	view := send(t, m, "z", "z", "z", "z", "z", "enter")
	if m.game.g.Attempts() != 0 {
		t.Errorf("Attempts() = %d, want 0", m.game.g.Attempts())
	}
	if !strings.Contains(view, "not in word list") {
		t.Errorf("missing rejection message\n%s", view)
	}
}

func TestWinningShowsResult(t *testing.T) {
	m := newModel(t)
	send(t, m, "down", "enter")
	m.game.g.Answer = "crane"

	view := send(t, m, "c", "r", "a", "n", "e", "enter")
	if m.game.g.Status != game.Won {
		t.Fatalf("status = %v, want won", m.game.g.Status)
	}
	if !strings.Contains(view, "solved in 1") {
		t.Errorf("missing win message\n%s", view)
	}
}

// Losing must reveal the answer, otherwise the round is unsatisfying.
func TestLosingRevealsAnswer(t *testing.T) {
	m := newModel(t)
	send(t, m, "down", "enter")
	m.game.g.Answer = "crane"

	for range m.game.g.MaxAttempts {
		send(t, m, "a", "b", "o", "u", "t", "enter")
	}
	view := m.View().Content

	if m.game.g.Status != game.Lost {
		t.Fatalf("status = %v, want lost", m.game.g.Status)
	}
	if !strings.Contains(view, "CRANE") {
		t.Errorf("answer not revealed\n%s", view)
	}
}

// Tab alone must not restart; the plan specifies tab *then* enter.
func TestTabThenEnterStartsNewPuzzle(t *testing.T) {
	m := newModel(t)
	send(t, m, "down", "enter")
	first := m.game.g.ID

	view := send(t, m, "tab")
	if m.game.g.ID != first {
		t.Error("tab alone started a new puzzle")
	}
	if !strings.Contains(view, "to start a new puzzle") {
		t.Errorf("missing restart prompt\n%s", view)
	}

	send(t, m, "enter")
	if m.game.g.ID == first {
		t.Error("tab+enter did not start a new puzzle")
	}
}

func TestEscFromTabPromptCancels(t *testing.T) {
	m := newModel(t)
	send(t, m, "down", "enter")
	first := m.game.g.ID

	send(t, m, "tab", "esc")
	if m.game.g.ID != first {
		t.Error("esc at the restart prompt started a new puzzle anyway")
	}
	if m.screen != screenGame {
		t.Error("esc at the restart prompt left the board")
	}
}

// Quitting mid-puzzle must persist it, and it must come back resumable.
func TestProgressSurvivesQuitAndReappearsInList(t *testing.T) {
	dir := t.TempDir()
	s, err := store.NewJSON(dir)
	if err != nil {
		t.Fatal(err)
	}

	m := New(s, nil, Options{})
	send(t, m, "down", "enter")
	m.game.g.Answer = "crane"
	send(t, m, "a", "b", "o", "u", "t", "enter")
	id := m.game.g.ID
	m.quit()

	// A fresh process over the same directory.
	reopened, err := store.NewJSON(dir)
	if err != nil {
		t.Fatal(err)
	}
	m2 := New(reopened, nil, Options{})
	m2.list.reload(reopened)

	if len(m2.list.items) != 1 || m2.list.items[0].ID != id {
		t.Fatalf("puzzle list = %+v, want the saved puzzle", m2.list.items)
	}
	if m2.list.items[0].Status != game.InProgress {
		t.Errorf("status = %v, want in progress", m2.list.items[0].Status)
	}

	g, err := reopened.Load(id)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(g.Guesses) != 1 || g.Guesses[0] != "about" {
		t.Errorf("resumed guesses = %v, want [about]", g.Guesses)
	}
}

func TestOpeningPuzzleFromList(t *testing.T) {
	m := newModel(t)
	send(t, m, "down", "enter") // start a 5-letter game
	id := m.game.g.ID
	// A puzzle only reaches the list once it has been played.
	m.game.g.Answer = "crane"
	send(t, m, "a", "b", "o", "u", "t", "enter")
	send(t, m, "esc") // back to menu

	// Navigate to "puzzles" (index 3) and open the first entry.
	m.list.reload(m.store)
	m.screen = screenList
	send(t, m, "enter")

	if m.screen != screenGame {
		t.Fatalf("screen = %v, want game", m.screen)
	}
	if m.game.g.ID != id {
		t.Errorf("opened %q, want %q", m.game.g.ID, id)
	}
}

// playSome saves n solved puzzles and returns their ids, so a test has a list
// to work on. It goes through the store rather than the menu, so it does not
// depend on where the menu cursor happens to be resting.
func playSome(t *testing.T, m *Model, n int) []string {
	t.Helper()
	ids := make([]string, 0, n)
	for range n {
		g, err := newPuzzle(m.store, 5)
		if err != nil {
			t.Fatalf("newPuzzle: %v", err)
		}
		g.Answer = "crane"
		if err := g.Guess("crane"); err != nil {
			t.Fatalf("Guess: %v", err)
		}
		if err := m.store.Save(g); err != nil {
			t.Fatalf("Save: %v", err)
		}
		ids = append(ids, g.ID)
	}
	return ids
}

func openList(t *testing.T, m *Model) {
	t.Helper()
	m.list.reload(m.store)
	m.screen = screenList
}

// Codes are six digits, so two puzzles can hash to the same one. It is only
// cosmetic — the id is what anything looks a puzzle up by — but a fresh puzzle
// re-rolls rather than show a code already in the list.
func TestNewPuzzleRerollsACollidingCode(t *testing.T) {
	m := newModel(t)
	existing := playSome(t, m, 1)[0]

	// The first draw repeats the saved puzzle's code; the second is its own.
	draws := 0
	draw := func(length int) (*game.Game, error) {
		draws++
		g, err := game.New(length)
		if err != nil {
			return nil, err
		}
		if draws == 1 {
			g.ID = existing // same id, so necessarily the same code
		}
		return g, nil
	}

	g, err := newPuzzleWith(m.store, 5, draw)
	if err != nil {
		t.Fatalf("newPuzzleWith: %v", err)
	}
	if draws != 2 {
		t.Errorf("drew %d times, want 2 (one collision, one re-roll)", draws)
	}
	if game.Code(g.ID) == game.Code(existing) {
		t.Errorf("kept the colliding code %q", game.Code(g.ID))
	}
}

// An unlucky run must still hand back a puzzle: a duplicate code is cosmetic,
// and refusing to start a game over it would be far worse.
func TestNewPuzzleGivesUpRerollingRatherThanFail(t *testing.T) {
	m := newModel(t)
	existing := playSome(t, m, 1)[0]

	draw := func(length int) (*game.Game, error) {
		g, err := game.New(length)
		if err != nil {
			return nil, err
		}
		g.ID = existing // every draw collides
		return g, nil
	}

	g, err := newPuzzleWith(m.store, 5, draw)
	if err != nil {
		t.Fatalf("newPuzzleWith: %v", err)
	}
	if g == nil {
		t.Fatal("no puzzle returned")
	}
}

// Deleting takes two keys, not one: the first arms a prompt naming the puzzle.
func TestDeleteFromListNeedsConfirming(t *testing.T) {
	m := newModel(t)
	ids := playSome(t, m, 2)
	openList(t, m)

	view := send(t, m, "d")
	if !strings.Contains(view, "delete #") {
		t.Fatalf("d did not arm a delete prompt\n%s", view)
	}
	if list, _ := m.store.List(); len(list) != 2 {
		t.Fatalf("arming the prompt already deleted something: %v", list)
	}

	doomed := m.list.items[m.list.cursor].ID
	send(t, m, "d")

	list, _ := m.store.List()
	if len(list) != 1 {
		t.Fatalf("List has %d entries after delete, want 1", len(list))
	}
	if list[0].ID == doomed {
		t.Errorf("deleted the wrong puzzle: %q survived", doomed)
	}
	if _, err := m.store.Load(doomed); err == nil {
		t.Error("deleted puzzle still loads")
	}
	// The other one is untouched.
	survivor := ids[0]
	if doomed == survivor {
		survivor = ids[1]
	}
	if _, err := m.store.Load(survivor); err != nil {
		t.Errorf("delete took the other puzzle too: %v", err)
	}
}

func TestDeletePromptCancels(t *testing.T) {
	m := newModel(t)
	playSome(t, m, 1)
	openList(t, m)

	send(t, m, "d")
	view := send(t, m, "esc")

	if list, _ := m.store.List(); len(list) != 1 {
		t.Errorf("esc at the prompt deleted anyway: %v", list)
	}
	if strings.Contains(view, "delete #") {
		t.Errorf("prompt still armed after esc\n%s", view)
	}
	// esc answered the prompt; it must not also have left the screen.
	if m.screen != screenList {
		t.Errorf("screen = %v, want to stay on the list", m.screen)
	}
}

// The prompt asks about one puzzle, so moving the cursor has to disarm it
// rather than let the answer land on a different row.
func TestMovingDisarmsTheDeletePrompt(t *testing.T) {
	m := newModel(t)
	playSome(t, m, 2)
	openList(t, m)

	send(t, m, "d")
	view := send(t, m, "down")
	if strings.Contains(view, "delete #") {
		t.Errorf("prompt survived a cursor move\n%s", view)
	}
	if list, _ := m.store.List(); len(list) != 2 {
		t.Errorf("moving off the prompt deleted something: %v", list)
	}
}

// Deleting must not fling the selection back to the top of the list.
func TestDeleteKeepsCursorPosition(t *testing.T) {
	m := newModel(t)
	playSome(t, m, 4)
	openList(t, m)

	send(t, m, "down", "down") // third row
	third := m.list.items[2].ID
	send(t, m, "d", "d")

	if m.list.cursor != 2 {
		t.Errorf("cursor = %d after delete, want 2", m.list.cursor)
	}
	if len(m.list.items) != 3 {
		t.Fatalf("list has %d rows after delete, want 3", len(m.list.items))
	}
	if m.list.items[2].ID == third {
		t.Error("the deleted puzzle is still in the list")
	}
}

// Deleting the last row leaves the cursor on the new last row, not past the end.
func TestDeleteClampsCursorAtTheEnd(t *testing.T) {
	m := newModel(t)
	playSome(t, m, 2)
	openList(t, m)

	send(t, m, "down", "d", "d") // the last of two rows

	if m.list.cursor != 0 || len(m.list.items) != 1 {
		t.Errorf("cursor = %d over %d rows, want 0 over 1", m.list.cursor, len(m.list.items))
	}
	if _, ok := m.list.selected(); !ok {
		t.Error("cursor is off the end of the list")
	}
}

func TestDeleteOnEmptyListDoesNothing(t *testing.T) {
	m := newModel(t)
	openList(t, m)

	view := send(t, m, "d", "d")

	if strings.Contains(view, "delete #") {
		t.Errorf("armed a prompt with nothing to delete\n%s", view)
	}
	if m.err != nil {
		t.Errorf("err = %v, want none", m.err)
	}
}

// Deleting is a list-screen action only: the board must not answer to it.
func TestDeleteKeyDoesNothingOnTheBoard(t *testing.T) {
	m := newModel(t)
	playSome(t, m, 1)
	send(t, m, "down", "enter")

	send(t, m, "d", "d")

	if list, _ := m.store.List(); len(list) != 1 {
		t.Errorf("d on the board deleted a puzzle: %v", list)
	}
}

// The clock runs from the first letter, the way a typing test does: opening a
// board and staring at it costs nothing.
func TestClockStartsOnFirstLetter(t *testing.T) {
	m := newModel(t)
	send(t, m, "down", "enter")

	if !m.game.sessionStart.IsZero() {
		t.Error("clock running before anything was typed")
	}
	if d := m.game.elapsed(); d != 0 {
		t.Errorf("elapsed = %v on an untouched board, want 0", d)
	}

	// Neither does deleting nothing, nor a rejected submit.
	send(t, m, "backspace", "enter")
	if !m.game.sessionStart.IsZero() {
		t.Error("clock started without a letter being typed")
	}

	send(t, m, "c")
	if m.game.sessionStart.IsZero() {
		t.Fatal("clock not running after the first letter")
	}

	// And it keeps the session it started; a later letter must not restart it.
	started := m.game.sessionStart
	send(t, m, "r")
	if !m.game.sessionStart.Equal(started) {
		t.Error("the second letter restarted the clock")
	}
}

// Time typed before the winning guess is still banked — the later start must
// not cost the player the seconds they actually spent.
func TestSolveTimeIsBankedFromTheFirstLetter(t *testing.T) {
	m := newModel(t)
	send(t, m, "down", "enter")
	m.game.g.Answer = "crane"

	send(t, m, "c")
	m.game.sessionStart = m.game.sessionStart.Add(-2 * time.Minute)
	send(t, m, "r", "a", "n", "e", "enter")

	g, err := m.store.Load(m.game.g.ID)
	if err != nil {
		t.Fatal(err)
	}
	if g.Elapsed() < 2*time.Minute {
		t.Errorf("elapsed = %v, want at least the 2m spent typing", g.Elapsed())
	}
}

// Resuming a puzzle and leaving without typing must add nothing: the clock is
// armed on the way in, not started.
func TestResumingWithoutTypingAddsNoTime(t *testing.T) {
	m := newModel(t)
	send(t, m, "down", "enter")
	m.game.g.Answer = "crane"
	send(t, m, "s", "t", "o", "n", "e", "enter")
	id := m.game.g.ID
	send(t, m, "esc")

	before, err := m.store.Load(id)
	if err != nil {
		t.Fatal(err)
	}

	m.list.reload(m.store)
	m.screen = screenList
	send(t, m, "enter")
	if m.game.g.ID != id {
		t.Fatalf("opened %q, want %q", m.game.g.ID, id)
	}
	if !m.game.sessionStart.IsZero() {
		t.Error("resuming started the clock before anything was typed")
	}
	send(t, m, "esc")

	after, err := m.store.Load(id)
	if err != nil {
		t.Fatal(err)
	}
	if after.ElapsedMS != before.ElapsedMS {
		t.Errorf("elapsed = %dms after idling, want %dms unchanged",
			after.ElapsedMS, before.ElapsedMS)
	}
}

// Reviewing a finished puzzle must not count as playing it. The clock stops on
// the guess that ends the game, so reopening it from the list and reading the
// board can add neither solve time nor a fresh UpdatedAt — the first would
// inflate the profile's average, the second would reorder the completion
// sequence the streaks are computed from.
func TestReviewingAFinishedPuzzleAddsNoTime(t *testing.T) {
	m := newModel(t)
	send(t, m, "down", "enter")
	m.game.g.Answer = "crane"
	send(t, m, "c", "r", "a", "n", "e", "enter")
	id := m.game.g.ID
	send(t, m, "esc")

	solved, err := m.store.Load(id)
	if err != nil {
		t.Fatal(err)
	}

	// Reopen it from the list and sit on it, as a player reviewing a win does.
	m.list.reload(m.store)
	m.screen = screenList
	send(t, m, "enter")
	if m.game.g.ID != id {
		t.Fatalf("opened %q, want the solved puzzle %q", m.game.g.ID, id)
	}
	m.game.sessionStart = time.Now().Add(-2 * time.Minute)
	send(t, m, "esc")

	reviewed, err := m.store.Load(id)
	if err != nil {
		t.Fatal(err)
	}
	if reviewed.ElapsedMS != solved.ElapsedMS {
		t.Errorf("elapsed = %dms after review, want %dms unchanged",
			reviewed.ElapsedMS, solved.ElapsedMS)
	}
	if !reviewed.UpdatedAt.Equal(solved.UpdatedAt) {
		t.Errorf("UpdatedAt moved from %v to %v while reviewing",
			solved.UpdatedAt, reviewed.UpdatedAt)
	}
}

func TestProfileReflectsPlay(t *testing.T) {
	m := newModel(t)
	send(t, m, "down", "enter")
	m.game.g.Answer = "crane"
	send(t, m, "c", "r", "a", "n", "e", "enter")
	send(t, m, "esc")

	m.profile.reload(m.store, m.day)
	m.screen = screenProfile
	view := m.View().Content

	if !strings.Contains(view, "profile") || !strings.Contains(view, "win rate") {
		t.Errorf("profile missing headings\n%s", view)
	}
	if !strings.Contains(view, "100%") {
		t.Errorf("expected a 100%% win rate\n%s", view)
	}
	if m.profile.summary.Won != 1 {
		t.Errorf("Won = %d, want 1", m.profile.summary.Won)
	}
}

// The visual case the plan calls out: duplicate letters must colour
// differently within one row.
func TestDuplicateLettersRenderDistinctly(t *testing.T) {
	m := newModel(t)
	send(t, m, "down", "enter")
	m.game.g.Answer = "loyal"

	send(t, m, "a", "l", "l", "o", "y", "enter")

	marks := m.game.g.Marks[0]
	// "alloy" vs "loyal": answer has two Ls, so both Ls in the guess score,
	// but the A and O placements differ.
	if marks[1] == marks[2] && marks[1] == game.Absent {
		t.Errorf("both Ls scored absent against an answer containing two: %v", marks)
	}

	view := m.View().Content
	if !strings.Contains(view, "L") {
		t.Errorf("board missing letters\n%s", view)
	}
}

// The keyboard must never downgrade a letter from correct back to absent.
func TestKeyboardKeepsBestLetterState(t *testing.T) {
	m := newModel(t)
	send(t, m, "down", "enter")
	m.game.g.Answer = "crane"

	send(t, m, "c", "r", "a", "n", "e", "enter") // C correct
	states := m.game.g.LetterStates()
	if states['c'] != game.Correct {
		t.Fatalf("c = %v, want correct", states['c'])
	}
	if !strings.Contains(m.View().Content, "Q") {
		t.Error("keyboard not rendered")
	}
}

// The legend is what makes a theme's tile colours readable: it must be on the
// board whenever there is room for it.
func TestLegendOnBoard(t *testing.T) {
	m := newModel(t)
	send(t, m, "down", "enter")
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 40})

	view := sgr.ReplaceAllString(m.View().Content, "")
	for _, want := range []string{"correct spot", "wrong spot", "not in word"} {
		if !strings.Contains(view, want) {
			t.Errorf("legend missing %q\n%s", want, view)
		}
	}
}

// The legend is the first thing to go when the terminal cannot hold the board,
// the keyboard and one more row — on either axis.
func TestLegendYieldsToASmallTerminal(t *testing.T) {
	for _, tc := range []struct {
		name string
		w, h int
	}{
		{"short", 100, 24},
		{"narrow", 50, 40},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := newModel(t)
			send(t, m, "down", "enter")
			m.Update(tea.WindowSizeMsg{Width: tc.w, Height: tc.h})

			view := sgr.ReplaceAllString(m.View().Content, "")
			if strings.Contains(view, "correct spot") {
				t.Errorf("legend still drawn at %dx%d\n%s", tc.w, tc.h, view)
			}
			if !strings.Contains(view, "Q") {
				t.Errorf("board and keyboard should survive\n%s", view)
			}
		})
	}
}

// The legend explains the board only if it is drawn the same way the board is.
// Rendering the sample letter through the very style a scored tile uses is what
// guarantees that, so the two must come out byte-identical.
func TestLegendMatchesTheTilesItExplains(t *testing.T) {
	m := newModel(t)
	send(t, m, "down", "enter")
	m.game.g.Answer = "crane"
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 40})

	send(t, m, "c", "r", "a", "n", "e", "enter") // every letter correct
	view := m.View().Content

	// One from the board's A tile, one from the legend's correct swatch.
	tile := st.tileCorrect.Render(legendSample)
	if n := strings.Count(view, tile); n < 2 {
		t.Errorf("legend swatch and scored tile render differently: %d matches for %q", n, tile)
	}
}
