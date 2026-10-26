package daily

import (
	"fmt"
	"time"
)

// dayLayout is the one textual form of a Day: the date, and nothing else.
const dayLayout = "2006-01-02"

// Day is a calendar date. For the daily it is always the date in UTC; the
// profile's calendar also uses it for dates in the player's own zone (DayIn).
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
		return Day{}, fmt.Errorf("daily: %.40q is not a date (want %s)", s, dayLayout)
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

// DayIn is the date an instant falls on in loc. The daily itself always
// lives in UTC (DayOf); this is for filing a player's own history by their
// own calendar. loc must not be nil.
func DayIn(t time.Time, loc *time.Location) Day {
	l := t.In(loc)
	return Day{year: l.Year(), month: l.Month(), day: l.Day()}
}

// DayFrom is the date with this year, month and day, normalised the way
// time.Date normalises, so day 0 is the last day of the month before.
func DayFrom(year int, month time.Month, day int) Day {
	return DayOf(time.Date(year, month, day, 0, 0, 0, 0, time.UTC))
}

// Date returns the year, month and day.
func (d Day) Date() (year int, month time.Month, day int) { return d.year, d.month, d.day }

// Weekday is the day of the week d falls on.
func (d Day) Weekday() time.Weekday { return d.startsAt().Weekday() }

// DaysSince is how many days after e d falls, negative when d is earlier.
// It counts in Unix seconds rather than through a time.Duration, which
// overflows past 292 years, and a hand-edited date can be in the year 1.
func (d Day) DaysSince(e Day) int {
	return int((d.startsAt().Unix() - e.startsAt().Unix()) / 86400)
}
