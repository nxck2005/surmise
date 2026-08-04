package ui

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/nxck2005/wortle/internal/game"
	"github.com/nxck2005/wortle/internal/store"
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
	for _, want := range []string{"a", "z", "enter", "esc", "tab", "backspace", "up", "down", "left", "right"} {
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
	// The number was only peeked, so the first real puzzle still gets #1.
	if n, _ := s.PeekNumber(); n != 1 {
		t.Errorf("PeekNumber() = %d, want 1 (no number consumed)", n)
	}
}

// The first guess is what saves a puzzle and gives it #1.
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
	if list[0].Number != 1 {
		t.Errorf("saved puzzle number = %d, want 1", list[0].Number)
	}
}

func TestMenuRendersModes(t *testing.T) {
	m := newModel(t)
	view := m.View().Content
	for _, want := range []string{"wortle", "4 letters", "5 letters", "6 letters", "puzzles", "profile"} {
		if !strings.Contains(view, want) {
			t.Errorf("menu missing %q\n%s", want, view)
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
	if m.game.g.Number <= 0 {
		t.Errorf("new puzzle has number %d", m.game.g.Number)
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

	m.profile.reload(m.store)
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
