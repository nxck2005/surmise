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

	"github.com/nxck2005/wortle/internal/game"
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
}

// ModeSummary is the per-word-length slice of a Summary.
type ModeSummary struct {
	Played      int
	Won         int
	WinRate     float64
	AvgAttempts float64
	AvgTime     time.Duration
}

// Compute aggregates a set of puzzles. Order does not matter; puzzles are
// sorted internally where sequence is significant.
func Compute(games []*game.Game) Summary {
	s := Summary{
		Distribution: make(map[int]int),
		ByLength:     make(map[int]ModeSummary),
	}

	var (
		totalAttempts int
		totalTime     time.Duration
		perLength     = make(map[int]*accumulator)
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

		case game.Lost:
			s.Played++
			s.Lost++
			acc.played++

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
