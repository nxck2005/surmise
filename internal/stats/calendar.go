package stats

import (
	"time"

	"github.com/nxck2005/surmise/internal/daily"
	"github.com/nxck2005/surmise/internal/game"
)

// PlayDay is the calendar day a puzzle is filed under: a daily's own date,
// and for any other puzzle the date, in loc, that it was started on. A game
// started before midnight and finished after it stays on the day it started.
//
// It takes the two fields rather than a game so the puzzle list, which holds
// summaries, files a puzzle by exactly the same rule as the calendar. ok is
// false when there is no day to give: a daily label that will not parse, or
// no start time at all. loc must not be nil.
func PlayDay(dailyLabel string, startedAt time.Time, loc *time.Location) (daily.Day, bool) {
	if dailyLabel != "" {
		d, err := daily.ParseDay(dailyLabel)
		return d, err == nil
	}
	if startedAt.IsZero() {
		return daily.Day{}, false
	}
	return daily.DayIn(startedAt, loc), true
}

// DayTally is what one calendar day holds.
type DayTally struct {
	Games, Won, Lost, InPlay int
}

// Activity counts the puzzles on each calendar day, by PlayDay.
//
// It is a record of play, not a figure: custom puzzles and room rounds are
// counted, because they were played. Deleted puzzles are not — a tombstone
// keeps no start time, and the calendar shows what is still there.
func Activity(games []*game.Game, loc *time.Location) map[daily.Day]DayTally {
	out := make(map[daily.Day]DayTally)
	for _, g := range games {
		if g.Deleted {
			continue
		}
		d, ok := PlayDay(g.Daily, g.StartedAt, loc)
		if !ok {
			continue
		}
		t := out[d]
		t.Games++
		switch g.Status {
		case game.Won:
			t.Won++
		case game.Lost:
			t.Lost++
		default:
			t.InPlay++
		}
		out[d] = t
	}
	return out
}
