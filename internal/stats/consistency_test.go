package stats

// The single-pass answer must be the walk's answer.
//
// CurrentStreak and IsBest exist to keep the menu and the result screen from
// sorting a history they only need one figure out of. Both are shortcuts past
// ComputeAt and Trends, so each is held to the longer walk's result here —
// including the ties, the deleted records and the puzzles that count for
// nothing, which are the cases a shortcut is most likely to get wrong.

import (
	"testing"
	"time"

	"github.com/nxck2005/surmise/internal/daily"
	"github.com/nxck2005/surmise/internal/game"
)

// tieGames is a history built to make order matter: equal completion clocks,
// equal times and equal guess counts, interleaved wins, losses and tombstones.
func tieGames() []*game.Game {
	base := time.Date(2026, 8, 6, 12, 0, 0, 0, time.UTC)
	mkAt := func(id string, status game.Status, attempts int, elapsed time.Duration, at time.Time) *game.Game {
		g := mk(5, status, attempts, elapsed, at)
		g.ID = id
		return g
	}
	return []*game.Game{
		// Two wins written in the same instant: the id decides which came first.
		mkAt("0000000000000002", game.Won, 3, 30*time.Second, base),
		mkAt("0000000000000001", game.Won, 3, 30*time.Second, base),
		mkAt("0000000000000003", game.Lost, 6, 99*time.Second, base.Add(time.Minute)),
		mkAt("0000000000000004", game.Won, 4, 20*time.Second, base.Add(2*time.Minute)),
		// A deleted win neither extends nor breaks the run.
		func() *game.Game {
			g := mkAt("0000000000000005", game.Won, 2, 10*time.Second, base.Add(3*time.Minute))
			return g.Tombstone()
		}(),
		mkAt("0000000000000006", game.Won, 5, 40*time.Second, base.Add(4*time.Minute)),
		// A custom puzzle counts for nothing, win or lose.
		func() *game.Game {
			g := mkAt("0000000000000007", game.Won, 1, time.Second, base.Add(5*time.Minute))
			g.Custom = true
			return g
		}(),
		// An unfinished puzzle is not part of any run.
		mkAt("0000000000000008", game.InProgress, 1, 5*time.Second, base.Add(6*time.Minute)),
	}
}

func TestCurrentStreakAgreesWithCompute(t *testing.T) {
	for name, games := range map[string][]*game.Game{
		"nil":     nil,
		"fixture": tieGames(),
	} {
		want := ComputeAt(games, daily.Today()).CurrentStreak
		if got := CurrentStreak(games); got != want {
			t.Errorf("%s: CurrentStreak = %d, ComputeAt says %d", name, got, want)
		}
	}
}

func TestIsBestAgreesWithTrends(t *testing.T) {
	games := tieGames()
	// A length with only one win reports neither: the first win is trivially
	// both, and saying so would mean nothing.
	one := []*game.Game{games[3]}
	for _, fixture := range [][]*game.Game{games, one} {
		for _, g := range fixture {
			if g.Status != game.Won || g.Deleted || !g.CountsForStats() {
				continue
			}
			trend := Trends(fixture, g.Length)
			var wantFast, wantFew bool
			if trend.AllTime.Won >= 2 {
				wantFast = trend.Fastest != nil && trend.Fastest.ID == g.ID
				wantFew = trend.Fewest != nil && trend.Fewest.ID == g.ID
			}
			gotFast, gotFew := IsBest(fixture, g)
			if gotFast != wantFast || gotFew != wantFew {
				t.Errorf("IsBest(%s) = %v/%v, Trends says %v/%v",
					g.ID, gotFast, gotFew, wantFast, wantFew)
			}
		}
	}
}
