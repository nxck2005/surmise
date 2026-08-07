// Package stats aggregates finished puzzles into the numbers shown on the
// profile screen.
//
// Everything is recomputed from the saved puzzles rather than incrementally
// maintained. At a few thousand games that costs nothing, and it means the
// numbers can never drift out of sync with the record of play.
package stats

import (
	"sort"
	"time"

	"github.com/nxck2005/surmise/internal/daily"
	"github.com/nxck2005/surmise/internal/game"
)

// Summary is the whole-profile view. Fields are additive by design; the metric
// set is expected to grow.
type Summary struct {
	Played  int
	Won     int
	Lost    int
	InPlay  int
	WinRate float64 // 0..1 over finished puzzles only

	// AvgAttempts and AvgTime cover won puzzles only. Averaging in losses
	// would make both numbers meaningless: a loss always uses every attempt,
	// and an abandoned puzzle can sit open for hours.
	AvgAttempts float64
	AvgTime     time.Duration

	CurrentStreak int
	MaxStreak     int

	// Distribution maps attempt count to number of wins in that many guesses.
	Distribution map[int]int

	// ByLength breaks the same figures down per game mode.
	ByLength map[int]ModeSummary

	// Daily covers the daily puzzles alone, per game mode. A mode whose daily
	// has never been played is absent, as in ByLength.
	Daily map[int]DailySummary
}

// ModeSummary is the per-word-length slice of a Summary.
type ModeSummary struct {
	Played      int
	Won         int
	WinRate     float64
	AvgAttempts float64
	AvgTime     time.Duration
}

// DailySummary is one mode's daily puzzles. Its streaks count consecutive
// calendar days rather than consecutive wins, which is the whole difference
// between it and the figures above: missing a day breaks a daily streak, and
// no amount of casual play repairs it.
//
// Like Summary the fields are additive — a daily distribution and daily
// averages belong here if they are ever wanted.
type DailySummary struct {
	Played  int
	Won     int
	WinRate float64

	CurrentStreak int
	MaxStreak     int
}

// Compute aggregates a set of puzzles against today's date. Order does not
// matter; puzzles are sorted internally where sequence is significant.
func Compute(games []*game.Game) Summary { return ComputeAt(games, daily.Today()) }

// ComputeAt is Compute with the current day supplied rather than read from the
// clock.
//
// Only the daily streak needs it, and it needs it for one distinction: today
// with no daily played is a day not played *yet*, while any earlier day with
// nothing on it is a day missed. Taking the day as an argument keeps the clock
// out of the package, lets the tests state the date they mean, and lets the UI
// pass the day it already resolved at startup — so -day moves the profile as
// well as the board.
func ComputeAt(games []*game.Game, today daily.Day) Summary {
	s := Summary{
		Distribution: make(map[int]int),
		ByLength:     make(map[int]ModeSummary),
		Daily:        make(map[int]DailySummary),
	}

	var (
		totalAttempts int
		totalTime     time.Duration
		perLength     = make(map[int]*accumulator)
		perDaily      = make(map[int]*accumulator)
	)

	for _, g := range games {
		// A deleted puzzle is gone from every figure here; it survives only in
		// the streak walk below, and only as a break in the sequence.
		if g.Deleted {
			continue
		}

		acc, ok := perLength[g.Length]
		if !ok {
			acc = &accumulator{}
			perLength[g.Length] = acc
		}

		// A daily is counted twice on purpose: it is one of the player's
		// puzzles like any other, and it is also one of their dailies. Only
		// the second tally is restricted.
		var dacc *accumulator
		if g.Daily != "" {
			dacc, ok = perDaily[g.Length]
			if !ok {
				dacc = &accumulator{}
				perDaily[g.Length] = dacc
			}
		}

		switch g.Status {
		case game.Won:
			s.Played++
			s.Won++
			acc.played++
			acc.won++

			s.Distribution[g.Attempts()]++
			totalAttempts += g.Attempts()
			totalTime += g.Elapsed()
			acc.attempts += g.Attempts()
			acc.time += g.Elapsed()

			if dacc != nil {
				dacc.played++
				dacc.won++
				dacc.attempts += g.Attempts()
				dacc.time += g.Elapsed()
			}

		case game.Lost:
			s.Played++
			s.Lost++
			acc.played++
			if dacc != nil {
				dacc.played++
			}

		default:
			s.InPlay++
		}
	}

	if finished := s.Won + s.Lost; finished > 0 {
		s.WinRate = float64(s.Won) / float64(finished)
	}
	if s.Won > 0 {
		s.AvgAttempts = float64(totalAttempts) / float64(s.Won)
		s.AvgTime = totalTime / time.Duration(s.Won)
	}

	for length, acc := range perLength {
		m := ModeSummary{Played: acc.played, Won: acc.won}
		if acc.played > 0 {
			m.WinRate = float64(acc.won) / float64(acc.played)
		}
		if acc.won > 0 {
			m.AvgAttempts = float64(acc.attempts) / float64(acc.won)
			m.AvgTime = acc.time / time.Duration(acc.won)
		}
		s.ByLength[length] = m
	}

	s.CurrentStreak, s.MaxStreak = streaks(games)

	runs := dailyStreaks(games, today)
	for length, acc := range perDaily {
		d := DailySummary{Played: acc.played, Won: acc.won}
		if acc.played > 0 {
			d.WinRate = float64(acc.won) / float64(acc.played)
		}
		d.CurrentStreak, d.MaxStreak = runs[length].current, runs[length].longest
		s.Daily[length] = d
	}
	// A mode present only as tombstones is left out: the counters skip deleted
	// records, and a run made only of deleted days is zero by the rules below,
	// so there would be nothing to show.

	return s
}

type accumulator struct {
	played, won, attempts int
	time                  time.Duration
}

// streaks walks finished puzzles in completion order. Unfinished puzzles are
// ignored rather than treated as breaks, so having a game open does not reset
// a streak.
//
// Deleted puzzles stay in the walk, which is the whole point of keeping a
// tombstone (game.Tombstone): a deleted loss still broke the streak, and
// dropping it would merge the runs either side and make the longest streak go
// *up* when a loss is deleted. Because the tombstone remembers the status, the
// rule can be exact rather than pessimistic:
//
//   - a win still on disk extends the run;
//   - a deleted win neither extends nor breaks it — it is no longer one of the
//     wins on the profile, so it must not lengthen a run either, which keeps
//     MaxStreak from ever exceeding Won;
//   - any loss, deleted or not, resets the run.
func streaks(games []*game.Game) (current, longest int) {
	finished := make([]*game.Game, 0, len(games))
	for _, g := range games {
		if g.Status.Done() {
			finished = append(finished, g)
		}
	}
	sort.Slice(finished, func(i, j int) bool {
		return finished[i].UpdatedAt.Before(finished[j].UpdatedAt)
	})

	run := 0
	for _, g := range finished {
		switch {
		case g.Status != game.Won:
			run = 0
		case g.Deleted:
			// leave the run as it stands
		default:
			run++
			if run > longest {
				longest = run
			}
		}
	}
	return run, longest
}

// dailyRun is one mode's pair of daily streaks.
type dailyRun struct{ current, longest int }

// dailyStreaks walks the calendar, per mode, rather than the record.
//
// That is the whole difference from streaks: a daily streak is a run of
// consecutive *days won*, so the gap between two records matters as much as the
// records themselves, and a day nobody played is a break rather than a
// non-event. Modes are independent — the daily is one puzzle per mode per day,
// and missing the six-letter board should not cost a five-letter run.
//
// Walking each day between the first record and today, the rules are:
//
//   - a win still on disk extends the run;
//   - any loss, deleted or not, resets it — as in streaks, which is what the
//     date on a tombstone is kept for;
//   - a deleted win neither extends nor breaks it, so a day removed from the
//     profile cannot lengthen a run either;
//   - an unfinished daily is neutral, matching streaks ignoring a game still in
//     play;
//   - a day with no record at all resets it, unless it is today or later: today
//     is not missed until it is over, so an unplayed today leaves yesterday's
//     streak standing.
func dailyStreaks(games []*game.Game, today daily.Day) map[int]dailyRun {
	byMode := make(map[int]map[daily.Day]*game.Game)
	for _, g := range games {
		if g.Daily == "" {
			continue
		}
		d, err := daily.ParseDay(g.Daily)
		if err != nil {
			// A date nothing can read places the puzzle nowhere on the
			// calendar; drop it rather than guess a day and invent a break.
			continue
		}
		days, ok := byMode[g.Length]
		if !ok {
			days = make(map[daily.Day]*game.Game)
			byMode[g.Length] = days
		}
		days[d] = g
	}

	runs := make(map[int]dailyRun, len(byMode))
	for length, days := range byMode {
		runs[length] = walkDays(days, today)
	}
	return runs
}

// walkDays applies the rules in dailyStreaks' comment to one mode's days.
func walkDays(days map[daily.Day]*game.Game, today daily.Day) dailyRun {
	var first, last daily.Day
	for d := range days {
		if first.IsZero() || d.Before(first) {
			first = d
		}
		if last.IsZero() || last.Before(d) {
			last = d
		}
	}
	if first.IsZero() {
		return dailyRun{}
	}

	// The walk ends at today, or at the last record when that is later — which
	// -day makes possible, since it can put "today" behind days already played.
	end := today
	if end.Before(last) {
		end = last
	}

	var r dailyRun
	for d := first; !end.Before(d); d = d.AddDays(1) {
		g, played := days[d]
		switch {
		case !played:
			if d.Before(today) {
				r.current = 0
			}
		case !g.Status.Done():
			// still in play: neutral, as an open game is to streaks
		case g.Status != game.Won:
			r.current = 0
		case g.Deleted:
			// leave the run as it stands
		default:
			r.current++
			if r.current > r.longest {
				r.longest = r.current
			}
		}
	}
	return r
}
