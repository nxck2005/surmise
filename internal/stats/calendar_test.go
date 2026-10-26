package stats

import (
	"testing"
	"time"

	"github.com/nxck2005/surmise/internal/daily"
	"github.com/nxck2005/surmise/internal/game"
)

func calDay(t *testing.T, s string) daily.Day {
	t.Helper()
	d, err := daily.ParseDay(s)
	if err != nil {
		t.Fatalf("ParseDay(%q): %v", s, err)
	}
	return d
}

func TestPlayDayDailyUsesItsDate(t *testing.T) {
	at := time.Date(2026, 1, 2, 12, 0, 0, 0, time.UTC)
	d, ok := PlayDay("2026-01-01", at, time.UTC)
	if !ok || d != calDay(t, "2026-01-01") {
		t.Errorf("PlayDay = %v, %v, want 2026-01-01, true", d, ok)
	}
}

func TestPlayDayUsesTheStartInTheZone(t *testing.T) {
	at := time.Date(2026, 10, 25, 23, 30, 0, 0, time.UTC)
	d, ok := PlayDay("", at, time.FixedZone("ist", 19800))
	if !ok || d != calDay(t, "2026-10-26") {
		t.Errorf("PlayDay = %v, %v, want 2026-10-26, true", d, ok)
	}
}

func TestPlayDayNeedsSomething(t *testing.T) {
	if _, ok := PlayDay("", time.Time{}, time.UTC); ok {
		t.Error("no label and no start time gave a day")
	}
	if _, ok := PlayDay("not-a-day", time.Now(), time.UTC); ok {
		t.Error("a label that is not a date gave a day")
	}
}

func TestActivityCounts(t *testing.T) {
	at := func(day, hour int) time.Time {
		return time.Date(2026, 10, day, hour, 0, 0, 0, time.UTC)
	}
	// startedAt builds a game the way mk does, then gives it a start time.
	startedAt := func(status game.Status, when time.Time, edit func(*game.Game)) *game.Game {
		g := mk(5, status, 3, time.Minute, when)
		g.StartedAt = when
		if edit != nil {
			edit(g)
		}
		return g
	}
	games := []*game.Game{
		startedAt(game.Won, at(26, 9), nil),
		startedAt(game.Won, at(26, 9), nil),
		startedAt(game.Lost, at(26, 9), nil),
		startedAt(game.InProgress, at(26, 20), nil),
		startedAt(game.Won, at(20, 12), func(g *game.Game) { g.Custom = true }),
		startedAt(game.Won, at(20, 12), func(g *game.Game) { g.Room = &game.RoomInfo{} }),
		startedAt(game.Won, at(26, 10), func(g *game.Game) { g.Deleted = true }),
		mk(5, game.Won, 3, time.Minute, at(26, 10)), // no StartedAt, no label
	}
	got := Activity(games, time.UTC)
	if len(got) != 2 {
		t.Errorf("Activity has %d days, want 2: %v", len(got), got)
	}
	if want := (DayTally{Games: 4, Won: 2, Lost: 1, InPlay: 1}); got[calDay(t, "2026-10-26")] != want {
		t.Errorf("2026-10-26 = %+v, want %+v", got[calDay(t, "2026-10-26")], want)
	}
	if want := (DayTally{Games: 2, Won: 2}); got[calDay(t, "2026-10-20")] != want {
		t.Errorf("2026-10-20 = %+v, want %+v", got[calDay(t, "2026-10-20")], want)
	}
}
