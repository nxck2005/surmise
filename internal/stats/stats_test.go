package stats

import (
	"testing"
	"time"

	"github.com/nxck2005/surmise/internal/game"
)

// mk builds a finished-or-not puzzle directly, bypassing the word lists so
// tests stay independent of the vocabulary.
func mk(length int, status game.Status, attempts int, elapsed time.Duration, at time.Time) *game.Game {
	g := &game.Game{
		Length:      length,
		Status:      status,
		MaxAttempts: length + 1,
		ElapsedMS:   elapsed.Milliseconds(),
		UpdatedAt:   at,
	}
	for range attempts {
		g.Guesses = append(g.Guesses, "xxxxx")
	}
	return g
}

func TestComputeEmpty(t *testing.T) {
	s := Compute(nil)
	if s.Played != 0 || s.WinRate != 0 || s.AvgAttempts != 0 || s.MaxStreak != 0 {
		t.Errorf("empty stats = %+v, want zeroes", s)
	}
	if s.Distribution == nil || s.ByLength == nil {
		t.Error("maps should be non-nil so callers need not check")
	}
}

func TestComputeCounts(t *testing.T) {
	base := time.Now()
	games := []*game.Game{
		mk(5, game.Won, 3, 30*time.Second, base),
		mk(5, game.Won, 5, 50*time.Second, base.Add(time.Minute)),
		mk(5, game.Lost, 6, 90*time.Second, base.Add(2*time.Minute)),
		mk(5, game.InProgress, 1, 10*time.Second, base.Add(3*time.Minute)),
	}
	s := Compute(games)

	if s.Played != 3 {
		t.Errorf("Played = %d, want 3 (in-progress excluded)", s.Played)
	}
	if s.Won != 2 || s.Lost != 1 || s.InPlay != 1 {
		t.Errorf("Won/Lost/InPlay = %d/%d/%d, want 2/1/1", s.Won, s.Lost, s.InPlay)
	}
	if s.WinRate != 2.0/3.0 {
		t.Errorf("WinRate = %v, want 2/3", s.WinRate)
	}
	// Averages cover wins only: (3+5)/2 attempts, (30s+50s)/2 time.
	if s.AvgAttempts != 4 {
		t.Errorf("AvgAttempts = %v, want 4", s.AvgAttempts)
	}
	if s.AvgTime != 40*time.Second {
		t.Errorf("AvgTime = %v, want 40s", s.AvgTime)
	}
	if s.Distribution[3] != 1 || s.Distribution[5] != 1 {
		t.Errorf("Distribution = %v, want one win each at 3 and 5", s.Distribution)
	}
}

func TestStreaks(t *testing.T) {
	base := time.Now()
	at := func(i int) time.Time { return base.Add(time.Duration(i) * time.Minute) }

	tests := []struct {
		name                 string
		statuses             []game.Status
		wantCurrent, wantMax int
	}{
		{"all wins", []game.Status{game.Won, game.Won, game.Won}, 3, 3},
		{"loss resets", []game.Status{game.Won, game.Won, game.Lost}, 0, 2},
		{"recovers after loss", []game.Status{game.Won, game.Lost, game.Won, game.Won}, 2, 2},
		{"max is behind current", []game.Status{game.Won, game.Won, game.Won, game.Lost, game.Won}, 1, 3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var games []*game.Game
			for i, st := range tt.statuses {
				games = append(games, mk(5, st, 3, time.Second, at(i)))
			}
			s := Compute(games)
			if s.CurrentStreak != tt.wantCurrent || s.MaxStreak != tt.wantMax {
				t.Errorf("streaks = %d/%d, want %d/%d",
					s.CurrentStreak, s.MaxStreak, tt.wantCurrent, tt.wantMax)
			}
		})
	}
}

// The regression this whole tombstone mechanism exists for: deleting a loss
// used to merge the win runs either side of it, so the longest streak went up
// when a loss was removed. The tombstone keeps the break in place.
func TestDeletedLossStillBreaksStreak(t *testing.T) {
	base := time.Now()
	at := func(i int) time.Time { return base.Add(time.Duration(i) * time.Minute) }

	// win win LOSS win win win
	statuses := []game.Status{game.Won, game.Won, game.Lost, game.Won, game.Won, game.Won}
	var games []*game.Game
	for i, st := range statuses {
		games = append(games, mk(5, st, 3, time.Second, at(i)))
	}
	loss := games[2]

	if s := Compute(games); s.MaxStreak != 3 {
		t.Fatalf("MaxStreak with the loss = %d, want 3", s.MaxStreak)
	}

	// Dropping the loss outright is the old, wrong behaviour. Pinning it here
	// keeps the test honest about what the tombstone is buying.
	without := append(append([]*game.Game{}, games[:2]...), games[3:]...)
	if s := Compute(without); s.MaxStreak != 5 {
		t.Fatalf("MaxStreak with the loss gone = %d, want 5 (the old bug)", s.MaxStreak)
	}

	games[2] = loss.Tombstone()
	s := Compute(games)
	if s.MaxStreak != 3 {
		t.Errorf("MaxStreak after deleting the loss = %d, want 3", s.MaxStreak)
	}
	if s.CurrentStreak != 3 {
		t.Errorf("CurrentStreak = %d, want 3", s.CurrentStreak)
	}
	// The deleted puzzle is gone from every count.
	if s.Played != 5 || s.Won != 5 || s.Lost != 0 || s.WinRate != 1 {
		t.Errorf("counts = %d played/%d won/%d lost/%v rate, want 5/5/0/1",
			s.Played, s.Won, s.Lost, s.WinRate)
	}
}

// A deleted win must not lengthen a run it is no longer counted in, or the
// longest streak could exceed the number of wins on the profile.
func TestDeletedWinNeitherExtendsNorBreaks(t *testing.T) {
	base := time.Now()
	at := func(i int) time.Time { return base.Add(time.Duration(i) * time.Minute) }

	var games []*game.Game
	for i := range 5 {
		games = append(games, mk(5, game.Won, 3, time.Second, at(i)))
	}
	games[2] = games[2].Tombstone()

	s := Compute(games)
	if s.Won != 4 {
		t.Fatalf("Won = %d, want 4", s.Won)
	}
	if s.MaxStreak != 4 || s.CurrentStreak != 4 {
		t.Errorf("streaks = %d/%d, want 4/4 (runs join, deleted win uncounted)",
			s.CurrentStreak, s.MaxStreak)
	}
}

// Unfinished puzzles are invisible to streaks, so a tombstone for one (which
// the store does not write) would still change nothing.
func TestDeletedInProgressChangesNothing(t *testing.T) {
	base := time.Now()
	games := []*game.Game{
		mk(5, game.Won, 3, time.Second, base),
		mk(5, game.InProgress, 1, time.Second, base.Add(time.Minute)).Tombstone(),
		mk(5, game.Won, 4, time.Second, base.Add(2*time.Minute)),
	}
	s := Compute(games)
	if s.CurrentStreak != 2 || s.MaxStreak != 2 {
		t.Errorf("streaks = %d/%d, want 2/2", s.CurrentStreak, s.MaxStreak)
	}
	if s.InPlay != 0 {
		t.Errorf("InPlay = %d, want 0 (a tombstone is not in play)", s.InPlay)
	}
}

// An open puzzle must not break a winning streak.
func TestInProgressDoesNotBreakStreak(t *testing.T) {
	base := time.Now()
	games := []*game.Game{
		mk(5, game.Won, 3, time.Second, base),
		mk(5, game.InProgress, 1, time.Second, base.Add(time.Minute)),
		mk(5, game.Won, 4, time.Second, base.Add(2*time.Minute)),
	}
	if s := Compute(games); s.CurrentStreak != 2 {
		t.Errorf("CurrentStreak = %d, want 2", s.CurrentStreak)
	}
}

func TestByLength(t *testing.T) {
	base := time.Now()
	games := []*game.Game{
		mk(4, game.Won, 2, 20*time.Second, base),
		mk(5, game.Won, 4, 60*time.Second, base.Add(time.Minute)),
		mk(5, game.Lost, 6, 60*time.Second, base.Add(2*time.Minute)),
	}
	s := Compute(games)

	if got := s.ByLength[4]; got.Played != 1 || got.Won != 1 || got.AvgAttempts != 2 {
		t.Errorf("ByLength[4] = %+v", got)
	}
	if got := s.ByLength[5]; got.Played != 2 || got.Won != 1 || got.WinRate != 0.5 {
		t.Errorf("ByLength[5] = %+v", got)
	}
	if _, ok := s.ByLength[6]; ok {
		t.Error("ByLength should omit unplayed modes")
	}
}

// custom marks a puzzle as custom, which is the whole of being left out.
func custom(g *game.Game) *game.Game {
	g.Custom = true
	return g
}

func TestCustomPuzzlesMoveNoFigure(t *testing.T) {
	base := time.Now()
	drawn := []*game.Game{
		mk(5, game.Won, 3, 30*time.Second, base),
		mk(5, game.Lost, 6, 90*time.Second, base.Add(time.Minute)),
	}
	want := Compute(drawn)

	// The same history with custom puzzles interleaved: a win that would
	// lift the win rate and shorten the average, and a loss that would lower it.
	mixed := []*game.Game{
		drawn[0],
		custom(mk(5, game.Won, 1, time.Second, base.Add(10*time.Second))),
		drawn[1],
		custom(mk(5, game.Lost, 6, time.Hour, base.Add(2*time.Minute))),
	}
	got := Compute(mixed)

	if got.Played != want.Played || got.Won != want.Won || got.Lost != want.Lost {
		t.Errorf("counts = %d/%d/%d, want %d/%d/%d",
			got.Played, got.Won, got.Lost, want.Played, want.Won, want.Lost)
	}
	if got.WinRate != want.WinRate {
		t.Errorf("WinRate = %v, want %v", got.WinRate, want.WinRate)
	}
	if got.AvgAttempts != want.AvgAttempts || got.AvgTime != want.AvgTime {
		t.Errorf("averages = %v/%v, want %v/%v",
			got.AvgAttempts, got.AvgTime, want.AvgAttempts, want.AvgTime)
	}
	if got.Distribution[1] != 0 {
		t.Errorf("a custom win reached the distribution: %v", got.Distribution)
	}
	if got.ByLength[5] != want.ByLength[5] {
		t.Errorf("ByLength[5] = %+v, want %+v", got.ByLength[5], want.ByLength[5])
	}
}

func TestACustomLossDoesNotBreakTheStreak(t *testing.T) {
	base := time.Now()
	games := []*game.Game{
		mk(5, game.Won, 3, time.Minute, base),
		custom(mk(5, game.Lost, 6, time.Minute, base.Add(time.Minute))),
		mk(5, game.Won, 4, time.Minute, base.Add(2*time.Minute)),
	}
	s := Compute(games)
	if s.CurrentStreak != 2 || s.MaxStreak != 2 {
		t.Errorf("CurrentStreak/MaxStreak = %d/%d, want 2/2: a puzzle that counts for "+
			"nothing must not break a run", s.CurrentStreak, s.MaxStreak)
	}
}

func TestDeletingACustomLossLeavesTheStreakAlone(t *testing.T) {
	base := time.Now()
	lost := custom(mk(5, game.Lost, 6, time.Minute, base.Add(time.Minute)))
	games := []*game.Game{
		mk(5, game.Won, 3, time.Minute, base),
		lost.Tombstone(),
		mk(5, game.Won, 4, time.Minute, base.Add(2*time.Minute)),
	}
	s := Compute(games)
	// The live loss did not break the run, so its tombstone must not either —
	// otherwise deleting a puzzle that never counted would shorten a streak.
	if s.CurrentStreak != 2 || s.MaxStreak != 2 {
		t.Errorf("CurrentStreak/MaxStreak = %d/%d, want 2/2", s.CurrentStreak, s.MaxStreak)
	}
}
