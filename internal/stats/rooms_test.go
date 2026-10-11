package stats

// The room-round rules: a finished round from an online room is history like
// a custom puzzle — listed and reviewable, and never a figure.

import (
	"testing"
	"time"

	"github.com/nxck2005/surmise/internal/game"
)

// roomGame builds one finished round of a 5-letter room through the same
// constructor the UI saves with. Our seat is always 2 ("nick"), the answer
// always "crane"; whether we solved decides our entry, and the other seat
// ("sam") finishes at the cap either way.
func roomGame(t *testing.T, code string, guesses []string, elapsedMS int64, at time.Time) *game.Game {
	t.Helper()

	rows := make([][]game.Mark, len(guesses))
	for i, w := range guesses {
		rows[i] = game.Score(w, "crane")
	}
	solved := len(guesses) > 0 && guesses[len(guesses)-1] == "crane"

	// Seat 1 never solves it and spends the whole board: six rows of five
	// marks, none of them a match.
	samRows := make([][]game.Mark, 6)
	for i := range samRows {
		samRows[i] = make([]game.Mark, 5)
	}

	info := game.RoomInfo{
		Code:  code,
		Mode:  "live",
		Round: 1,
		Seat:  2,
		Ranking: []game.RoomRank{
			// Seat 1 never solves it and spends the whole board.
			{Seat: 1, Name: "sam", Solved: false, Guesses: 6, ElapsedMS: 600_000,
				Rows: samRows},
			{Seat: 2, Name: "nick", Solved: solved, Guesses: len(guesses),
				ElapsedMS: elapsedMS, Rows: rows},
		},
	}
	g, err := game.NewRoomRound(info, "crane", guesses, elapsedMS, at)
	if err != nil {
		t.Fatalf("NewRoomRound(%s): %v", code, err)
	}
	return g
}

func wonRoomRound(t *testing.T, code string, at time.Time) *game.Game {
	t.Helper()
	return roomGame(t, code, []string{"slate", "crane"}, 81_000, at)
}

func lostRoomRound(t *testing.T, code string, at time.Time) *game.Game {
	t.Helper()
	return roomGame(t, code, []string{"slate", "adieu", "pious", "trope", "lymph", "ghost"}, 600_000, at)
}

func TestRoomRoundsMoveNoFigure(t *testing.T) {
	base := time.Date(2026, 10, 11, 10, 0, 0, 0, time.UTC)
	ordinary := []*game.Game{
		mk(5, game.Won, 3, 30*time.Second, base),
		mk(5, game.Won, 4, time.Minute, base.Add(time.Minute)),
		mk(5, game.Won, 2, 20*time.Second, base.Add(2*time.Minute)),
	}

	alone := ordinary
	mixed := []*game.Game{
		ordinary[0],
		wonRoomRound(t, "K7M2QX", base.Add(30*time.Second)),
		ordinary[1],
		lostRoomRound(t, "N4XYRZ", base.Add(90*time.Second)),
		ordinary[2],
	}

	same := func(t *testing.T, got, want Summary) {
		t.Helper()
		if got.Played != want.Played || got.Won != want.Won || got.Lost != want.Lost {
			t.Errorf("counts = %d/%d/%d, want %d/%d/%d",
				got.Played, got.Won, got.Lost, want.Played, want.Won, want.Lost)
		}
		if got.CurrentStreak != want.CurrentStreak || got.MaxStreak != want.MaxStreak {
			t.Errorf("streaks = %d/%d, want %d/%d",
				got.CurrentStreak, got.MaxStreak, want.CurrentStreak, want.MaxStreak)
		}
		if got.Distribution[1] != want.Distribution[1] || got.Distribution[2] != want.Distribution[2] ||
			got.Distribution[3] != want.Distribution[3] || got.Distribution[4] != want.Distribution[4] {
			t.Errorf("distribution = %v, want %v", got.Distribution, want.Distribution)
		}
		if got.AvgAttempts != want.AvgAttempts || got.AvgTime != want.AvgTime {
			t.Errorf("averages = %v/%v, want %v/%v", got.AvgAttempts, got.AvgTime,
				want.AvgAttempts, want.AvgTime)
		}
	}

	t.Run("live rounds", func(t *testing.T) {
		same(t, Compute(mixed), Compute(alone))
	})

	t.Run("a deleted loss is a tombstone", func(t *testing.T) {
		deleted := mixed[3].Tombstone()
		after := []*game.Game{mixed[0], mixed[1], mixed[2], deleted, mixed[4]}
		same(t, Compute(after), Compute(alone))
	})
}
