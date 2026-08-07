package stats

import (
	"testing"
	"time"

	"github.com/nxck2005/surmise/internal/daily"
	"github.com/nxck2005/surmise/internal/game"
)

// day is the reference date the daily tests are written around. Fixed rather
// than derived from the clock so a test never straddles UTC midnight.
var day = daily.DayOf(time.Date(2026, 8, 7, 12, 0, 0, 0, time.UTC))

// ago is the day n days before the reference one.
func ago(n int) daily.Day { return day.AddDays(-n) }

// mkDaily builds a daily puzzle for a given day, the way daily.New would label
// one. UpdatedAt tracks the date so the ordinary streak walk still sees a
// sensible sequence.
func mkDaily(length int, status game.Status, d daily.Day) *game.Game {
	g := mk(length, status, 3, 30*time.Second, d.ResetsAt().Add(-time.Hour))
	g.Daily = d.String()
	return g
}

func TestDailyStreakCountsConsecutiveDays(t *testing.T) {
	games := []*game.Game{
		mkDaily(5, game.Won, ago(3)),
		mkDaily(5, game.Won, ago(2)),
		mkDaily(5, game.Won, ago(1)),
		mkDaily(5, game.Won, day),
	}
	got := ComputeAt(games, day).Daily[5]
	if got.CurrentStreak != 4 || got.MaxStreak != 4 {
		t.Errorf("streaks = %d/%d, want 4/4", got.CurrentStreak, got.MaxStreak)
	}
	if got.Played != 4 || got.Won != 4 || got.WinRate != 1 {
		t.Errorf("counts = %+v, want 4 played, 4 won, rate 1", got)
	}
}

// The point of a daily streak: a day nobody played is a break, even though
// nothing on disk records it.
func TestMissedDayBreaksTheDailyStreak(t *testing.T) {
	games := []*game.Game{
		mkDaily(5, game.Won, ago(4)),
		mkDaily(5, game.Won, ago(3)),
		// ago(2) never played.
		mkDaily(5, game.Won, ago(1)),
		mkDaily(5, game.Won, day),
	}
	got := ComputeAt(games, day).Daily[5]
	if got.CurrentStreak != 2 {
		t.Errorf("CurrentStreak = %d, want 2 (the gap resets it)", got.CurrentStreak)
	}
	if got.MaxStreak != 2 {
		t.Errorf("MaxStreak = %d, want 2", got.MaxStreak)
	}
	if got.Played != 4 {
		t.Errorf("Played = %d, want 4 (a missed day is not a played one)", got.Played)
	}
}

// Today is not missed until it is over, so an unplayed today leaves yesterday's
// streak standing — and an unplayed yesterday does not.
func TestTodayUnplayedKeepsTheStreak(t *testing.T) {
	games := []*game.Game{
		mkDaily(5, game.Won, ago(2)),
		mkDaily(5, game.Won, ago(1)),
	}
	if got := ComputeAt(games, day).Daily[5]; got.CurrentStreak != 2 {
		t.Errorf("CurrentStreak with today unplayed = %d, want 2", got.CurrentStreak)
	}
	// The same record a day later: yesterday is now the missed day.
	if got := ComputeAt(games, day.AddDays(1)).Daily[5]; got.CurrentStreak != 0 {
		t.Errorf("CurrentStreak a day later = %d, want 0", got.CurrentStreak)
	}
}

func TestLostDayBreaksTheDailyStreak(t *testing.T) {
	games := []*game.Game{
		mkDaily(5, game.Won, ago(3)),
		mkDaily(5, game.Won, ago(2)),
		mkDaily(5, game.Lost, ago(1)),
		mkDaily(5, game.Won, day),
	}
	got := ComputeAt(games, day).Daily[5]
	if got.CurrentStreak != 1 || got.MaxStreak != 2 {
		t.Errorf("streaks = %d/%d, want 1/2", got.CurrentStreak, got.MaxStreak)
	}
	if got.WinRate != 0.75 {
		t.Errorf("WinRate = %v, want 0.75", got.WinRate)
	}
}

// The daily analogue of TestDeletedLossStillBreaksStreak, and what the date on
// a tombstone is kept for: without it the deleted day would read as a day never
// played, which breaks the run too — so the test pins the *stronger* claim that
// the loss is still known to have been a loss, by checking the day is not
// treated as absent from the record.
func TestDeletedDailyLossStillBreaksTheStreak(t *testing.T) {
	loss := mkDaily(5, game.Lost, ago(1))
	games := []*game.Game{
		mkDaily(5, game.Won, ago(3)),
		mkDaily(5, game.Won, ago(2)),
		loss.Tombstone(),
		mkDaily(5, game.Won, day),
	}
	if loss.Tombstone().Daily != ago(1).String() {
		t.Fatalf("a tombstone dropped its date: %q", loss.Tombstone().Daily)
	}
	got := ComputeAt(games, day).Daily[5]
	if got.CurrentStreak != 1 || got.MaxStreak != 2 {
		t.Errorf("streaks = %d/%d, want 1/2", got.CurrentStreak, got.MaxStreak)
	}
	// The deleted day is gone from the counters, as everywhere else.
	if got.Played != 3 || got.Won != 3 {
		t.Errorf("counts = %d played/%d won, want 3/3", got.Played, got.Won)
	}
}

// A deleted win is not on the profile, so it must not lengthen a run either.
// The days either side of it join rather than break, exactly as they do in the
// ordinary walk.
func TestDeletedDailyWinNeitherExtendsNorBreaks(t *testing.T) {
	games := []*game.Game{
		mkDaily(5, game.Won, ago(3)),
		mkDaily(5, game.Won, ago(2)).Tombstone(),
		mkDaily(5, game.Won, ago(1)),
		mkDaily(5, game.Won, day),
	}
	got := ComputeAt(games, day).Daily[5]
	if got.Won != 3 {
		t.Fatalf("Won = %d, want 3", got.Won)
	}
	if got.CurrentStreak != 3 || got.MaxStreak != 3 {
		t.Errorf("streaks = %d/%d, want 3/3 (never more than Won)",
			got.CurrentStreak, got.MaxStreak)
	}
}

// One daily per mode per day, so the modes keep their own streaks: missing the
// six-letter board must not cost a five-letter run.
func TestDailyStreaksAreIndependentPerMode(t *testing.T) {
	games := []*game.Game{
		mkDaily(5, game.Won, ago(2)),
		mkDaily(5, game.Won, ago(1)),
		mkDaily(5, game.Won, day),
		mkDaily(6, game.Won, ago(2)),
		// The six-letter daily was missed yesterday.
		mkDaily(6, game.Won, day),
	}
	s := ComputeAt(games, day)
	if got := s.Daily[5]; got.CurrentStreak != 3 {
		t.Errorf("5-letter CurrentStreak = %d, want 3", got.CurrentStreak)
	}
	if got := s.Daily[6]; got.CurrentStreak != 1 {
		t.Errorf("6-letter CurrentStreak = %d, want 1", got.CurrentStreak)
	}
	if _, ok := s.Daily[4]; ok {
		t.Error("Daily should omit modes whose daily was never played")
	}
}

// An unfinished daily is neutral, matching the ordinary walk's treatment of a
// game still in play — but it is not a win, so it does not extend a run.
func TestUnfinishedDailyIsNeutral(t *testing.T) {
	games := []*game.Game{
		mkDaily(5, game.Won, ago(2)),
		mkDaily(5, game.InProgress, ago(1)),
		mkDaily(5, game.Won, day),
	}
	got := ComputeAt(games, day).Daily[5]
	if got.CurrentStreak != 2 || got.MaxStreak != 2 {
		t.Errorf("streaks = %d/%d, want 2/2", got.CurrentStreak, got.MaxStreak)
	}
}

// The two tallies are separate: casual play neither feeds the daily figures nor
// repairs a daily streak, and a daily still counts as an ordinary puzzle.
func TestDailyAndCasualCountsAreSeparate(t *testing.T) {
	games := []*game.Game{
		mkDaily(5, game.Won, ago(2)),
		mk(5, game.Won, 3, time.Second, ago(1).ResetsAt().Add(-time.Hour)),
		mk(5, game.Lost, 6, time.Second, day.ResetsAt().Add(-time.Hour)),
	}
	s := ComputeAt(games, day)

	if s.Played != 3 || s.Won != 2 {
		t.Errorf("whole profile = %d played/%d won, want 3/2 (a daily counts too)",
			s.Played, s.Won)
	}
	got := s.Daily[5]
	if got.Played != 1 || got.Won != 1 {
		t.Errorf("daily = %d played/%d won, want 1/1 (casual games excluded)",
			got.Played, got.Won)
	}
	// The casual win on the intervening day does not fill the daily gap.
	if got.CurrentStreak != 0 {
		t.Errorf("CurrentStreak = %d, want 0 (casual play does not repair it)",
			got.CurrentStreak)
	}
}

// -day can put "today" behind days already played; the walk must still reach
// them rather than stopping short and reporting a stale run.
func TestDailyStreakWalksPastAPinnedToday(t *testing.T) {
	games := []*game.Game{
		mkDaily(5, game.Won, day),
		mkDaily(5, game.Won, day.AddDays(1)),
	}
	if got := ComputeAt(games, day).Daily[5]; got.CurrentStreak != 2 {
		t.Errorf("CurrentStreak = %d, want 2", got.CurrentStreak)
	}
}

// A date the package cannot read places the puzzle nowhere on the calendar, so
// it is dropped rather than allowed to invent a break.
func TestUnreadableDailyDateIsIgnored(t *testing.T) {
	bad := mkDaily(5, game.Won, ago(1))
	bad.Daily = "not-a-date"
	games := []*game.Game{
		mkDaily(5, game.Won, ago(2)),
		bad,
		mkDaily(5, game.Won, day),
	}
	got := ComputeAt(games, day).Daily[5]
	if got.CurrentStreak != 1 {
		t.Errorf("CurrentStreak = %d, want 1 (the unreadable day is not a win)",
			got.CurrentStreak)
	}
	if got.Played != 3 {
		t.Errorf("Played = %d, want 3 (it is still a daily that was played)", got.Played)
	}
}

func TestComputeDailyEmpty(t *testing.T) {
	s := Compute(nil)
	if s.Daily == nil {
		t.Error("Daily should be non-nil so callers need not check")
	}
}
