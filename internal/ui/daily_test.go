package ui

import (
	"context"
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/nxck2005/surmise/internal/daily"
	"github.com/nxck2005/surmise/internal/game"
	"github.com/nxck2005/surmise/internal/store"
)

// testDay is the date the daily tests pin, so none of them depend on when they
// are run. The -day override exists for exactly this.
const testDay = "2026-08-06"

// pump runs a command and feeds what it produces back into the model, which is
// what send deliberately does not do. Building a daily is asynchronous — it has
// to be, since the source it will one day use can block — so a test that stops
// at the command has not opened anything.
//
// Ticks are dropped rather than run: tea.Tick really sleeps, and the clock is
// not what any of this is about.
func pump(t *testing.T, m *Model, cmd tea.Cmd) {
	t.Helper()
	for range 8 {
		if cmd == nil {
			return
		}
		msg := cmd()
		if _, isTick := msg.(tickMsg); msg == nil || isTick {
			return
		}
		_, cmd = m.Update(msg)
	}
	t.Fatal("commands did not settle")
}

// dailyModel is a model whose daily is pinned to a known date.
func dailyModel(t *testing.T, opts Options) *Model {
	t.Helper()
	s, err := store.NewJSON(t.TempDir())
	if err != nil {
		t.Fatalf("NewJSON: %v", err)
	}
	if opts.Day == "" {
		opts.Day = testDay
	}
	m := New(s, nil, opts)
	m.screen = screenMenu
	return m
}

// showDaily walks the menu to the daily screen the way a player would.
func showDaily(t *testing.T, m *Model) {
	t.Helper()
	m.menu.cursor = menuIndex(t, m, choiceDaily, 0)
	send(t, m, "enter")
	if m.screen != screenDaily {
		t.Fatalf("screen = %v, want the daily screen", m.screen)
	}
}

// playDaily opens a mode's daily from the daily screen, running the command
// that builds it.
func playDaily(t *testing.T, m *Model, length int) {
	t.Helper()
	showDaily(t, m)
	for i, row := range m.daily.rows {
		if row.length == length {
			m.daily.cursor = i
		}
	}
	_, cmd := m.Update(key("enter"))
	pump(t, m, cmd)
}

func TestDailyOpensTheDaysPuzzle(t *testing.T) {
	m := dailyModel(t, Options{})
	playDaily(t, m, 5)

	if m.screen != screenGame {
		t.Fatalf("screen = %v, want the board", m.screen)
	}
	want := daily.ID(m.day, 5)
	if m.game.g.ID != want {
		t.Errorf("id = %q, want the derived %q", m.game.g.ID, want)
	}
	if m.game.g.Daily != testDay {
		t.Errorf("Daily = %q, want %q", m.game.g.Daily, testDay)
	}
	// The board says which day it is, since the daily turns over in UTC.
	if frame := m.View().Content; !strings.Contains(frame, "daily "+testDay) {
		t.Error("the board does not name the day it is the daily for")
	}
}

// Every mode has its own daily, and they are genuinely different puzzles.
func TestEachModeHasItsOwnDaily(t *testing.T) {
	seen := make(map[string]bool)
	for _, n := range []int{4, 5, 6} {
		m := dailyModel(t, Options{})
		playDaily(t, m, n)
		g := m.game.g
		if g.Length != n {
			t.Fatalf("asked for %d letters, got %d", n, g.Length)
		}
		if seen[g.ID] || seen[g.Answer] {
			t.Errorf("length %d repeats another mode's puzzle", n)
		}
		seen[g.ID], seen[g.Answer] = true, true
	}
}

// The counterpart to TestNewPuzzleRerollsACollidingCode: the random path
// re-rolls its id away from a code already in use, and the daily must not —
// its id is what makes it the same puzzle for everybody.
func TestDailyKeepsItsIDWhenTheCodeCollides(t *testing.T) {
	m := dailyModel(t, Options{})
	want := daily.ID(m.day, 5)

	// Park a puzzle already wearing the daily's code. Codes are six digits, so
	// a hit turns up in about a million tries.
	code := game.Code(want)
	var clash *game.Game
	for i := range 4_000_000 {
		id := fmt.Sprintf("collide-%d", i)
		if game.Code(id) != code {
			continue
		}
		g, err := game.NewFrom(id, "about", 5)
		if err != nil {
			t.Fatal(err)
		}
		clash = g
		break
	}
	if clash == nil {
		t.Fatal("no id collided with the daily's code, which is implausible")
	}
	if err := m.store.Save(clash); err != nil {
		t.Fatal(err)
	}

	playDaily(t, m, 5)
	if m.game.g.ID != want {
		t.Errorf("id = %q, want %q — the daily re-rolled away from a collision", m.game.g.ID, want)
	}
}

// Winning a daily gives the profile a section the casual figures do not: a
// streak counted in days. The profile takes its day from the root, so under
// -day the streak and the board agree on what today is.
func TestProfileShowsTheDailyStreak(t *testing.T) {
	m := dailyModel(t, Options{})
	playDaily(t, m, 5)
	m.game.g.Answer = "crane"
	send(t, m, "c", "r", "a", "n", "e", "enter")
	if m.game.g.Status != game.Won {
		t.Fatalf("status = %v, want won", m.game.g.Status)
	}
	send(t, m, "esc")

	m.profile.reload(m.store, m.day, "")
	m.screen = screenProfile
	view := m.View().Content

	if !strings.Contains(view, "daily") {
		t.Errorf("profile has no daily section\n%s", view)
	}
	if !strings.Contains(view, "streak 1 (max 1)") {
		t.Errorf("profile does not show the daily streak\n%s", view)
	}

	got := m.profile.summary.Daily[5]
	if got.Played != 1 || got.Won != 1 || got.CurrentStreak != 1 {
		t.Errorf("Daily[5] = %+v, want 1 played, 1 won, streak 1", got)
	}
	if _, ok := m.profile.summary.Daily[4]; ok {
		t.Error("a mode whose daily was never played should be absent")
	}
}

// A player who has never opened the daily screen sees the profile they always
// did, which is what keeps the section from being noise.
func TestProfileHasNoDailySectionWithoutADaily(t *testing.T) {
	m := dailyModel(t, Options{})
	send(t, m, "down", "enter") // the five-letter mode, an ordinary puzzle
	m.game.g.Answer = "crane"
	send(t, m, "c", "r", "a", "n", "e", "enter")
	if m.game.g.Status != game.Won || m.game.g.Daily != "" {
		t.Fatalf("wanted a won casual puzzle, got status %v daily %q",
			m.game.g.Status, m.game.g.Daily)
	}
	send(t, m, "esc")

	m.profile.reload(m.store, m.day, "")
	m.screen = screenProfile
	if view := m.View().Content; strings.Contains(view, "streak 1 (max 1)") {
		t.Errorf("casual play produced a daily row\n%s", view)
	}
	if len(m.profile.summary.Daily) != 0 {
		t.Errorf("Daily = %+v, want empty", m.profile.summary.Daily)
	}
}

// Opening the daily and walking away must save nothing, like any other new
// puzzle: 0/6 entries do not belong in the list.
func TestDailyIsTransientUntilTheFirstGuess(t *testing.T) {
	m := dailyModel(t, Options{})
	playDaily(t, m, 5)
	send(t, m, "esc")

	list, err := m.store.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 0 {
		t.Errorf("List() has %d entries after opening and leaving the daily, want 0", len(list))
	}
}

// Coming back to the daily resumes it rather than dealing a second board.
func TestDailyResumesRatherThanRestarting(t *testing.T) {
	m := dailyModel(t, Options{})
	playDaily(t, m, 5)
	m.game.g.Answer = "crane"
	send(t, m, "a", "b", "o", "u", "t", "enter")
	id := m.game.g.ID
	send(t, m, "esc")

	playDaily(t, m, 5)
	switch g := m.game.g; {
	case g.ID != id:
		t.Errorf("id = %q on reopening, want %q", g.ID, id)
	case g.Attempts() != 1:
		t.Errorf("Attempts() = %d on reopening, want the guess to still be there", g.Attempts())
	}
	if list, _ := m.store.List(); len(list) != 1 {
		t.Errorf("List() has %d entries, want the one daily", len(list))
	}
}

// Deleting a finished daily leaves a tombstone. Because the id is derived, a
// recreated daily would save straight over it — destroying the record of how
// the day went and handing back a puzzle that was already played. It is refused.
func TestDeletedDailyIsNotRecreated(t *testing.T) {
	m := dailyModel(t, Options{})
	playDaily(t, m, 5)
	id := m.game.g.ID
	m.game.g.Answer = "crane"
	send(t, m, "c", "r", "a", "n", "e", "enter") // win it
	send(t, m, "esc")

	if err := m.store.Delete(id); err != nil {
		t.Fatal(err)
	}

	playDaily(t, m, 5)
	if m.screen == screenGame && m.game.g.ID == id {
		t.Fatal("the deleted daily was handed back to be played again")
	}
	if m.err == nil {
		t.Error("nothing was reported when the deleted daily was reopened")
	}

	// The tombstone must survive, or deleting a daily loss would still be a way
	// to mend a streak.
	saved, err := m.store.All()
	if err != nil {
		t.Fatal(err)
	}
	var found bool
	for _, g := range saved {
		if g.ID == id {
			found = g.Deleted
		}
	}
	if !found {
		t.Error("the tombstone was overwritten")
	}
}

// The daily screen reports what has been played without opening anything.
func TestDailyScreenShowsWhatIsPlayed(t *testing.T) {
	m := dailyModel(t, Options{})
	playDaily(t, m, 5)
	m.game.g.Answer = "crane"
	send(t, m, "c", "r", "a", "n", "e", "enter")
	send(t, m, "esc")

	showDaily(t, m)
	frame := m.View().Content
	if !strings.Contains(frame, "solved 1/6") {
		t.Error("the daily screen does not show the solved mode")
	}
	if !strings.Contains(frame, "not started") {
		t.Error("the daily screen does not show the untouched modes")
	}
	if !strings.Contains(frame, testDay) {
		t.Error("the daily screen does not name the day")
	}
}

// A source that cannot supply a seed — which is what an offline remote source
// will be — reports and changes nothing.
type unavailableSeeds struct{}

func (unavailableSeeds) Seed(context.Context, daily.Day, int) (daily.Seed, error) {
	return daily.Seed{}, daily.ErrUnavailable
}

func TestUnavailableDailyIsReported(t *testing.T) {
	m := dailyModel(t, Options{DailySeeds: unavailableSeeds{}})
	playDaily(t, m, 5)

	if m.screen != screenDaily {
		t.Errorf("screen = %v, want to have stayed on the daily screen", m.screen)
	}
	if m.err == nil {
		t.Error("an unavailable seed was not reported")
	}
	if list, _ := m.store.List(); len(list) != 0 {
		t.Error("something was saved despite the seed failing")
	}
}

func TestDayOverride(t *testing.T) {
	const other = "2027-03-09"
	a := dailyModel(t, Options{})
	b := dailyModel(t, Options{Day: other})
	playDaily(t, a, 5)
	playDaily(t, b, 5)

	if a.game.g.ID == b.game.g.ID {
		t.Error("two dates produced the same puzzle")
	}
	if b.game.g.Daily != other {
		t.Errorf("Daily = %q, want %q", b.game.g.Daily, other)
	}
}

// A date that will not parse is reported, not fatal, the same as an unknown
// theme or an unsupported length.
func TestBadDayOverrideFallsBackToToday(t *testing.T) {
	m := dailyModel(t, Options{Day: "yesterday"})
	if m.err == nil {
		t.Error("a bad -day was swallowed")
	}
	if m.day != daily.Today() {
		t.Errorf("day = %s, want today", m.day)
	}
}

// The daily is one puzzle a day, so the board's restart must not quietly deal a
// random one in its place.
func TestDailyBoardRefusesToRestart(t *testing.T) {
	m := dailyModel(t, Options{})
	playDaily(t, m, 5)
	id := m.game.g.ID

	send(t, m, "tab", "enter")
	if m.game.g.ID != id {
		t.Error("tab-then-enter replaced the daily with another puzzle")
	}
	if m.game.message == "" {
		t.Error("nothing was said about why the daily did not restart")
	}
}

// The house rule: anything the keys can do, a click can do.
func TestPlayingTheDailyByClickingOnly(t *testing.T) {
	m := dailyModel(t, Options{})

	click(t, m, action{kind: actMenuChoice, index: menuIndex(t, m, choiceDaily, 0)})
	if m.screen != screenDaily {
		t.Fatalf("screen = %v after clicking the daily row, want the daily screen", m.screen)
	}

	// Clicking a mode's row runs the same command the keys do.
	draw(t, m)
	r, ok := m.hits.find(action{kind: actDailyRow, index: 1})
	if !ok {
		t.Fatal("no clickable row for the 5-letter daily")
	}
	_, cmd := m.Update(tea.MouseClickMsg{X: r.x + r.w/2, Y: r.y + r.h/2, Button: tea.MouseLeft})
	pump(t, m, cmd)

	if m.screen != screenGame {
		t.Fatalf("screen = %v after clicking a mode, want the board", m.screen)
	}
	if m.game.g.ID != daily.ID(m.day, 5) {
		t.Errorf("clicking opened %q, want the day's 5-letter puzzle", m.game.g.ID)
	}

	// And it really is playable with the mouse alone.
	m.game.g.Answer = "crane"
	clickWord(t, m, "crane")
	click(t, m, action{kind: actSubmit})
	if m.game.g.Status != game.Won {
		t.Errorf("status = %v after clicking the answer, want won", m.game.g.Status)
	}
}

// A saved daily is labelled as one in the puzzle list, so it is not mistaken
// for a casual puzzle when deciding what to delete.
func TestPuzzleListLabelsTheDaily(t *testing.T) {
	m := dailyModel(t, Options{})
	playDaily(t, m, 5)
	m.game.g.Answer = "crane"
	send(t, m, "a", "b", "o", "u", "t", "enter")
	send(t, m, "esc")

	m.menu.cursor = menuIndex(t, m, choiceList, 0)
	frame := send(t, m, "enter")
	if m.screen != screenList {
		t.Fatalf("screen = %v, want the puzzle list", m.screen)
	}
	if !strings.Contains(frame, "daily "+testDay) {
		t.Error("the puzzle list does not label the daily")
	}

	// And deleting one warns that the day cannot be played again.
	frame = send(t, m, "d")
	if !strings.Contains(frame, "cannot be played again") {
		t.Error("the delete prompt does not warn that a daily is spent for good")
	}
}
