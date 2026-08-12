package stats

import (
	"testing"
	"time"

	"github.com/nxck2005/surmise/internal/game"
)

func TestPlaytimeCountsEveryPuzzle(t *testing.T) {
	now := time.Now()
	won := mk(5, game.Won, 3, 40*time.Second, now)
	lost := mk(5, game.Lost, 6, 90*time.Second, now)
	inPlay := mk(5, game.InProgress, 2, 20*time.Second, now)
	custom := mk(5, game.Won, 4, 30*time.Second, now)
	custom.Custom = true

	// Everything counts here, unlike every figure on Summary: losing, being
	// halfway through and playing somebody else's word are all playing.
	got := Playtime(0, []*game.Game{won, lost, inPlay, custom})
	if want := 180 * time.Second; got != want {
		t.Errorf("Playtime = %v, want %v", got, want)
	}
}

func TestPlaytimeKeepsTheCounterOverTheRecords(t *testing.T) {
	g := mk(5, game.Won, 3, time.Minute, time.Now())

	// The counter remembers puzzles the records no longer do, so it wins.
	if got, want := Playtime(2*time.Hour, []*game.Game{g}), 2*time.Hour; got != want {
		t.Errorf("Playtime = %v, want the saved counter %v", got, want)
	}
	// And an install whose history predates the counter is seeded from what the
	// records can prove, rather than reading as never having played.
	if got, want := Playtime(0, []*game.Game{g}), time.Minute; got != want {
		t.Errorf("Playtime with no counter = %v, want the floor %v", got, want)
	}
}

// TestPlaytimeSurvivesDeletion is the promise the whole feature rests on: time
// played is permanent. A tombstone carries no ElapsedMS, so a total summed from
// the records alone would fall here — the counter is what stops it.
func TestPlaytimeSurvivesDeletion(t *testing.T) {
	kept := mk(5, game.Won, 3, time.Minute, time.Now())
	doomed := mk(5, game.Lost, 6, 5*time.Minute, time.Now())

	before := Playtime(0, []*game.Game{kept, doomed})
	if want := 6 * time.Minute; before != want {
		t.Fatalf("Playtime before deletion = %v, want %v", before, want)
	}

	// Deleting is what the store does to the record: a tombstone with the time
	// destroyed. The counter carries the same total as before.
	tomb := doomed.Tombstone()
	if got := Playtime(before, []*game.Game{kept, tomb}); got != before {
		t.Errorf("Playtime after deletion = %v, want %v — deleting must not take time back", got, before)
	}
}

func TestFormatPlaytime(t *testing.T) {
	cases := []struct {
		in   time.Duration
		want string
	}{
		{0, "none"},
		{-time.Second, "none"},
		{42 * time.Second, "42s"},
		{90 * time.Second, "1m"},
		{59 * time.Minute, "59m"},
		{time.Hour + 4*time.Minute, "1h 04m"},
		{25*time.Hour + 30*time.Minute, "1d 01h"},
		{50 * time.Hour, "2d 02h"},
	}
	for _, c := range cases {
		if got := FormatPlaytime(c.in); got != c.want {
			t.Errorf("FormatPlaytime(%v) = %q, want %q", c.in, got, c.want)
		}
	}
}
