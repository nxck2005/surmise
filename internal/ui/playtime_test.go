package ui

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/nxck2005/surmise/internal/game"
)

// playFor solves a puzzle whose session started d ago, so the banked time is a
// figure a test can assert on: a real session here lasts microseconds and would
// round to nothing.
func playFor(t *testing.T, m *Model, d time.Duration) {
	t.Helper()
	m.menu.point(menuIndex(t, m, choiceNewGame, 5))
	send(t, m, "enter")
	m.game.g.Answer = "crane"

	send(t, m, "c") // the first letter starts the clock
	m.game.sessionStart = time.Now().Add(-d)
	send(t, m, "r", "a", "n", "e", "enter")

	if m.game.g.Status != game.Won {
		t.Fatalf("status = %q, want won", m.game.g.Status)
	}
}

func TestPlaytimeIsBankedOnTheFinishingGuess(t *testing.T) {
	m := newModel(t)
	playFor(t, m, 90*time.Second)

	// The counter and the puzzle learn about the same session together, which is
	// what gameScreen.bank exists to guarantee.
	banked := time.Duration(m.settingsOf().PlaytimeMS) * time.Millisecond
	if banked < 89*time.Second || banked > 2*time.Minute {
		t.Errorf("counter = %v after a 90s session, want about 90s", banked)
	}
	if solve := m.game.g.Elapsed(); solve < 89*time.Second {
		t.Errorf("the puzzle recorded %v, want about 90s", solve)
	}
}

// The point of a counter rather than a figure summed from the records: time
// played is permanent. A deleted puzzle keeps a tombstone with no elapsed time,
// so anything derived from the records alone would fall here.
func TestDeletingAPuzzleKeepsItsPlaytime(t *testing.T) {
	m := newModel(t)
	playFor(t, m, 90*time.Second)
	before := m.playtime()

	send(t, m, "esc")
	openList(t, m)
	send(t, m, "d", "d")

	if list, _ := m.store.List(); len(list) != 0 {
		t.Fatalf("puzzle still listed after deletion: %v", list)
	}
	if got := m.playtime(); got != before {
		t.Errorf("playtime = %v after deleting the puzzle, want %v", got, before)
	}
}

// An install whose history predates the counter reads its total from the saved
// puzzles once, and keeps it from then on.
func TestPlaytimeSeedsItselfFromExistingPuzzles(t *testing.T) {
	m := newModel(t)
	playFor(t, m, 90*time.Second)

	// Wind the counter back to what an older settings file says: nothing.
	s := m.settingsOf()
	s.PlaytimeMS = 0
	m.saveSettings(s)

	seeded := m.playtime()
	if seeded < 89*time.Second {
		t.Fatalf("playtime = %v with no counter, want the saved puzzle's time", seeded)
	}
	if got := time.Duration(m.settingsOf().PlaytimeMS) * time.Millisecond; got != seeded {
		t.Errorf("counter = %v after seeding, want it written back as %v", got, seeded)
	}
}

func TestProfileShowsPlaytime(t *testing.T) {
	m := newModel(t)
	playFor(t, m, 90*time.Second)
	send(t, m, "esc")

	m.menu.point(menuIndex(t, m, choiceProfile, 0))
	send(t, m, "enter")
	if m.screen != screenProfile {
		t.Fatalf("screen = %v, want profile", m.screen)
	}

	view := plain(m.View().Content)
	if !strings.Contains(view, "playtime") || !strings.Contains(view, "1m") {
		t.Errorf("profile does not show the playtime:\n%s", view)
	}
}

// The figure never sheds, so it has to fit either way: beside the time row in a
// wide terminal, stacked under it in a narrow one. A fourth cell in a narrow
// terminal would widen the whole panel past the screen, which nothing clips.
func TestProfileStacksPlaytimeInANarrowTerminal(t *testing.T) {
	for _, tc := range []struct {
		name          string
		width         int
		wantSameLine  bool
		wantOwnLine   bool
		neighbourCell string
	}{
		{name: "wide", width: 100, wantSameLine: true, neighbourCell: "streak"},
		{name: "narrow", width: 70, wantOwnLine: true, neighbourCell: "streak"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := newModel(t)
			playFor(t, m, 90*time.Second)
			send(t, m, "esc")
			m.Update(tea.WindowSizeMsg{Width: tc.width, Height: 40})

			m.menu.point(menuIndex(t, m, choiceProfile, 0))
			send(t, m, "enter")

			view := plain(m.View().Content)
			var shared bool
			for _, line := range strings.Split(view, "\n") {
				if strings.Contains(line, tc.neighbourCell) && strings.Contains(line, "playtime") {
					shared = true
				}
			}
			if shared != tc.wantSameLine {
				t.Errorf("playtime shares the %s row = %v, want %v:\n%s",
					tc.neighbourCell, shared, tc.wantSameLine, view)
			}
			if tc.wantOwnLine && !strings.Contains(view, "playtime") {
				t.Errorf("playtime is missing from a %d-column profile:\n%s", tc.width, view)
			}
			// Either way the panel has to stay inside the terminal.
			if w := lipgloss.Width(m.View().Content); w > tc.width {
				t.Errorf("frame is %d columns wide, want no more than %d", w, tc.width)
			}
		})
	}
}
