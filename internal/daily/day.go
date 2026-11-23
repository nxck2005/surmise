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
//
// It is a hand-written reader of the one fixed layout rather than time.Parse:
// the daily stores carry a date string each, and the profile's daily-streak
// walk parses every one of them on every recomputation. The accept set is
// time.Parse's for this layout — four-digit years, a real calendar day — and
// TestParseDayMatchesTimeParse holds the two to it.
func ParseDay(s string) (Day, error) {
	if len(s) != 10 || s[4] != '-' || s[7] != '-' {
		return Day{}, badDay(s)
	}
	digit := func(i int) (int, bool) {
		c := s[i]
		if c < '0' || c > '9' {
			return 0, false
		}
		return int(c - '0'), true
	}
	var v [8]int // year's four digits, then month's and day's two each
	for i := range v {
		d, ok := digit(offsetOf(i))
		if !ok {
			return Day{}, badDay(s)
		}
		v[i] = d
	}
	year := v[0]*1000 + v[1]*100 + v[2]*10 + v[3]
	month := v[4]*10 + v[5]
	day := v[6]*10 + v[7]
	if month < 1 || month > 12 || day < 1 || day > daysInMonth(year, time.Month(month)) {
		return Day{}, badDay(s)
	}
	return Day{year: year, month: time.Month(month), day: day}, nil
}

// badDay is the one refusal wording. It is built only on failure: a successful
// parse must not pay for an error's formatting, and ParseDay is called once per
// daily record in every profile recomputation.
func badDay(s string) error {
	return fmt.Errorf("daily: %.40q is not a date (want %s)", s, dayLayout)
}

// offsetOf maps the i'th parsed digit to its position in "YYYY-MM-DD".
func offsetOf(i int) int {
	switch {
	case i < 4:
		return i
	case i < 6:
		return i + 1 // past the first dash
	default:
		return i + 2 // past the second
	}
}

// daysInMonth is the Gregorian month length. Time's own rule: a year divisible
// by 4 is a leap year unless it is a century not divisible by 400.
func daysInMonth(year int, month time.Month) int {
	switch month {
	case time.February:
		if year%4 == 0 && (year%100 != 0 || year%400 == 0) {
			return 29
		}
		return 28
	case time.April, time.June, time.September, time.November:
		return 30
	default:
		return 31
	}
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
// needs no method; ordering does. The fields are already normalised, so the
// tuple compares as a date without building two time.Time values — this runs
// once per comparison in every streak walk and calendar sort.
func (d Day) Before(e Day) bool {
	if d.year != e.year {
		return d.year < e.year
	}
	if d.month != e.month {
		return d.month < e.month
	}
	return d.day < e.day
}

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
//
// It counts in whole days rather than through a time.Duration, which overflows
// past 292 years, and a hand-edited date can be in the year 1. The count comes
// from the civil calendar directly rather than from two time.Time values: the
// calendar's click indices and the daily-streak walk ask this for every cell
// and every gap, and the conversion is a handful of integer operations.
func (d Day) DaysSince(e Day) int {
	return civilDays(d.year, d.month, d.day) - civilDays(e.year, e.month, e.day)
}

// civilDays is the day number of a Gregorian date, counting from 1970-01-01.
// It is Howard Hinnant's days_from_civil: exact for every year the count can
// hold, with February pushed to the end of the year so leap days land in one
// place. Both ends of DaysSince use it, so the epoch cancels.
func civilDays(year int, month time.Month, day int) int {
	y := year
	m := int(month)
	if m <= 2 {
		y--
	}
	era := y / 400
	if y < 0 && y%400 != 0 {
		era-- // floor division, so year 1 is in the era before year 0
	}
	yoe := y - era*400
	mp := (m + 9) % 12
	doy := (153*mp+2)/5 + day - 1
	doe := yoe*365 + yoe/4 - yoe/100 + doy
	return era*146097 + doe - 719468
}
