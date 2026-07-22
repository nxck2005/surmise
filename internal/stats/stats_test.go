package stats

import (
	"testing"
	"time"

	"github.com/nxck2005/wortle/internal/game"
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
