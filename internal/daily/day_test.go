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
