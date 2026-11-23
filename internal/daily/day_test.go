package daily

import (
	"testing"
	"time"
)

func TestDayIn(t *testing.T) {
	at := time.Date(2026, 10, 25, 23, 30, 0, 0, time.UTC)
	cases := []struct {
		name string
		loc  *time.Location
		want string
	}{
		{"ist", time.FixedZone("ist", 5*3600+1800), "2026-10-26"},
		{"utc", time.UTC, "2026-10-25"},
		{"pdt", time.FixedZone("pdt", -7*3600), "2026-10-25"},
	}
	for _, c := range cases {
		if got := DayIn(at, c.loc); got != day(t, c.want) {
			t.Errorf("DayIn in %s = %v, want %s", c.name, got, c.want)
		}
	}
}

func TestDayFrom(t *testing.T) {
	if got, want := DayFrom(2026, 3, 0), day(t, "2026-02-28"); got != want {
		t.Errorf("DayFrom(2026, 3, 0) = %v, want %v", got, want)
	}
	if got, want := DayFrom(2028, 12, 31), day(t, "2028-12-31"); got != want {
		t.Errorf("DayFrom(2028, 12, 31) = %v, want %v", got, want)
	}
}

func TestDayWeekday(t *testing.T) {
	cases := []struct {
		date string
		want time.Weekday
	}{
		{"2026-10-26", time.Monday},
		{"2026-01-01", time.Thursday},
		{"2028-01-01", time.Saturday},
	}
	for _, c := range cases {
		if got := day(t, c.date).Weekday(); got != c.want {
			t.Errorf("%s is %v, want %v", c.date, got, c.want)
		}
	}
}

func TestDaysSince(t *testing.T) {
	a, b := day(t, "2026-10-26"), day(t, "2025-10-26")
	if got := a.DaysSince(b); got != 365 {
		t.Errorf("DaysSince = %d, want 365", got)
	}
	if got := b.DaysSince(a); got != -365 {
		t.Errorf("DaysSince = %d, want -365", got)
	}
	if got := DayFrom(1, 1, 1).DaysSince(DayFrom(2026, 10, 26)); got != -739914 {
		t.Errorf("year 1 DaysSince = %d, want -739914", got)
	}
}

func TestDate(t *testing.T) {
	y, m, d := DayFrom(2026, 10, 26).Date()
	if y != 2026 || m != time.October || d != 26 {
		t.Errorf("Date = %d %v %d, want 2026 October 26", y, m, d)
	}
}

// The hand-written reader must accept exactly what time.Parse accepts for the
// layout — no more (a made-up day must not get an id) and no less (every date
// already on disk must still decode).
func TestParseDayMatchesTimeParse(t *testing.T) {
	try := func(s string) {
		t.Helper()
		_, wantErr := time.Parse(dayLayout, s)
		got, gotErr := ParseDay(s)
		if (gotErr == nil) != (wantErr == nil) {
			t.Errorf("ParseDay(%q) error = %v, time.Parse says %v", s, gotErr, wantErr)
			return
		}
		if gotErr == nil && got.String() != s {
			t.Errorf("ParseDay(%q) = %s", s, got)
		}
	}
	// Every day of a leap year and the year either side.
	for _, year := range []int{2023, 2024, 2025} {
		for month := time.January; month <= time.December; month++ {
			for d := 1; d <= 31; d++ {
				try(time.Date(year, month, d, 0, 0, 0, 0, time.UTC).Format(dayLayout))
			}
		}
	}
	// The century rule, the ends of the format, and the malformed shapes the
	// old parser refused.
	for _, s := range []string{
		"0000-01-01", "0000-02-29", "0001-01-01", "0099-12-31",
		"1900-02-28", "1900-02-29", "2000-02-29", "9999-12-31",
		"2026-1-02", "2026-01-2", "2026-1-2", "2026-01-01T00:00:00Z",
		"2026/01/01", " 2026-01-01", "2026-01-01 ", "+2026-01-01",
		"2026-00-01", "2026-13-01", "2026-01-00", "2026-01-32",
		"2026-04-31", "2026-06-31", "2026-09-31", "2026-11-31",
		"", "-", "2026--1-01", "2026-01--1",
	} {
		try(s)
	}
}

// Time and the civil-day count must agree on every distance the app can ask
// for: a click index on the calendar and a daily-streak gap both use it.
func TestDaysSinceMatchesUnixDays(t *testing.T) {
	days := []Day{
		DayFrom(1, time.January, 1),
		DayFrom(4, time.February, 29),
		DayFrom(100, time.March, 1),
		DayFrom(400, time.February, 29),
		DayFrom(1899, time.December, 31),
		DayFrom(1900, time.February, 28),
		DayFrom(1970, time.January, 1),
		DayFrom(2000, time.February, 29),
		DayFrom(2026, time.October, 26),
		DayFrom(9999, time.December, 31),
	}
	for _, a := range days {
		for _, b := range days {
			want := int((a.startsAt().Unix() - b.startsAt().Unix()) / 86400)
			if got := a.DaysSince(b); got != want {
				t.Errorf("%s.DaysSince(%s) = %d, want %d", a, b, got, want)
			}
			if got, want := a.Before(b), a.startsAt().Before(b.startsAt()); got != want {
				t.Errorf("%s.Before(%s) = %v, want %v", a, b, got, want)
			}
		}
	}
}
