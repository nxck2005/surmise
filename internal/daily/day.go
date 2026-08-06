package daily

import (
	"fmt"
	"time"
)

// dayLayout is the one textual form of a Day: the date, and nothing else.
const dayLayout = "2006-01-02"

// Day is a calendar date in UTC.
//
// UTC rather than local time so that every player's daily changes at the same
// instant — which is what a shared board, and eventually a leaderboard, needs.
// The cost is that "today" and the daily's date disagree for part of the day
// well east or west of Greenwich, so the date is always shown rather than
// implied.
type Day struct {
	year  int
	month time.Month
	day   int
}

// Today is the day currently being played.
func Today() Day { return DayOf(time.Now()) }

// DayOf is the day an instant falls in, wherever the clock it came from was.
func DayOf(t time.Time) Day {
	u := t.UTC()
	return Day{year: u.Year(), month: u.Month(), day: u.Day()}
}

// ParseDay reads a date in the form Day prints, for the -day override.
func ParseDay(s string) (Day, error) {
	t, err := time.Parse(dayLayout, s)
	if err != nil {
		return Day{}, fmt.Errorf("daily: %q is not a date (want %s)", s, dayLayout)
	}
	return DayOf(t), nil
}

func (d Day) String() string {
	return d.startsAt().Format(dayLayout)
}

// IsZero reports whether d is the zero Day, which no real date is.
func (d Day) IsZero() bool { return d == Day{} }

// startsAt is the instant this day begins.
func (d Day) startsAt() time.Time {
	return time.Date(d.year, d.month, d.day, 0, 0, 0, 0, time.UTC)
}

// ResetsAt is the instant this day's puzzles are replaced by the next day's.
func (d Day) ResetsAt() time.Time { return d.startsAt().AddDate(0, 0, 1) }

// AddDays is the day n days after d, and n days before it when n is negative.
//
// It exists for walking a run of consecutive dates — a daily streak is indexed
// by calendar day, so it has to be able to ask for the day before this one
// rather than for the record before this one. Going through time.AddDate keeps
// month and year ends right, and UTC means there is no hour to lose to a
// daylight-saving shift.
func (d Day) AddDays(n int) Day { return DayOf(d.startsAt().AddDate(0, 0, n)) }

// Before reports whether d falls earlier than e. Day is comparable, so equality
// needs no method; ordering does.
func (d Day) Before(e Day) bool { return d.startsAt().Before(e.startsAt()) }
