package daily

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/nxck2005/wortle/internal/game"
	"github.com/nxck2005/wortle/internal/words"
)

func day(t *testing.T, s string) Day {
	t.Helper()
	d, err := ParseDay(s)
	if err != nil {
		t.Fatalf("ParseDay(%q): %v", s, err)
	}
	return d
}

// stubSource stands in for a source that is not the local one, which is what
// the identity tests need: a different seed must not move a single id.
type stubSource struct {
	seed Seed
	err  error
}

func (s stubSource) Seed(context.Context, Day, int) (Seed, error) { return s.seed, s.err }

func TestParseDayRoundTrips(t *testing.T) {
	const want = "2026-08-06"
	if got := day(t, want).String(); got != want {
		t.Errorf("round trip = %q, want %q", got, want)
	}
	for _, bad := range []string{"", "tomorrow", "2026-8-6", "2026-13-01", "2026-08-06T10:00:00Z"} {
		if _, err := ParseDay(bad); err == nil {
			t.Errorf("ParseDay(%q) succeeded, want error", bad)
		}
	}
}

// The daily rolls over at UTC midnight, so the day an instant belongs to is
// decided in UTC no matter what zone the clock that produced it was in.
func TestDayOfIsUTC(t *testing.T) {
	// Half past midnight in UTC+13 is still the previous day in UTC.
	east := time.FixedZone("UTC+13", 13*60*60)
	if got := DayOf(time.Date(2026, 8, 7, 0, 30, 0, 0, east)).String(); got != "2026-08-06" {
		t.Errorf("DayOf(just past local midnight, UTC+13) = %s, want 2026-08-06", got)
	}
	// And late evening in UTC-11 is already the next day in UTC.
	west := time.FixedZone("UTC-11", -11*60*60)
	if got := DayOf(time.Date(2026, 8, 6, 23, 0, 0, 0, west)).String(); got != "2026-08-07" {
		t.Errorf("DayOf(late evening, UTC-11) = %s, want 2026-08-07", got)
	}
}

func TestResetsAtIsNextUTCMidnight(t *testing.T) {
	got := day(t, "2026-08-06").ResetsAt()
	want := time.Date(2026, 8, 7, 0, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Errorf("ResetsAt = %s, want %s", got, want)
	}
}

// The id is what every player names the same puzzle by and what the store keys
// its file on, so it is pinned to a literal: if this changes, every daily
// already on disk is orphaned and two versions of the app disagree about the
// day's code.
func TestIDIsStable(t *testing.T) {
	const want = "13f0405e-2c98-8e40-ba2c-dce569a50a05"
	if got := ID(day(t, "2026-08-06"), 5); got != want {
		t.Errorf("ID = %q, want %q\n(changing the id derivation orphans every saved daily)", got, want)
	}
}

func TestIDDiffersByDateAndLength(t *testing.T) {
	base := ID(day(t, "2026-08-06"), 5)
	if next := ID(day(t, "2026-08-07"), 5); next == base {
		t.Error("consecutive days share an id")
	}
	for _, n := range []int{4, 6} {
		if other := ID(day(t, "2026-08-06"), n); other == base {
			t.Errorf("length %d shares the 5-letter id", n)
		}
	}
}

// The seam test. Identity comes from public data and the answer from the seed,
// which is what lets the seed source be replaced later without renaming a
// single puzzle. It fails the moment anyone lets a seed reach the id.
func TestIdentityIsIndependentOfTheSource(t *testing.T) {
	d := day(t, "2026-08-06")
	other := stubSource{seed: Seed{1, 2, 3}}

	local, err := New(t.Context(), Local(), d, 5)
	if err != nil {
		t.Fatal(err)
	}
	swapped, err := New(t.Context(), other, d, 5)
	if err != nil {
		t.Fatal(err)
	}

	if local.ID != swapped.ID {
		t.Errorf("id moved with the source: %q vs %q", local.ID, swapped.ID)
	}
	if game.Code(local.ID) != game.Code(swapped.ID) {
		t.Error("code moved with the source")
	}
	if local.Answer == swapped.Answer {
		t.Error("answer did not move with the source — the seed is not reaching it")
	}
}

func TestSeedDependsOnDateAndLength(t *testing.T) {
	src := Local()
	seed := func(date string, n int) Seed {
		t.Helper()
		s, err := src.Seed(t.Context(), day(t, date), n)
		if err != nil {
			t.Fatal(err)
		}
		return s
	}

	base := seed("2026-08-06", 5)
	if again := seed("2026-08-06", 5); again != base {
		t.Error("the same day and length gave two seeds")
	}
	if next := seed("2026-08-07", 5); next == base {
		t.Error("consecutive days share a seed")
	}
	if other := seed("2026-08-06", 6); other == base {
		t.Error("two lengths share a seed")
	}
}

func TestNewBuildsAPlayablePuzzle(t *testing.T) {
	d := day(t, "2026-08-06")
	for _, n := range words.Lengths {
		g, err := New(t.Context(), Local(), d, n)
		if err != nil {
			t.Fatalf("length %d: %v", n, err)
		}
		if err := g.Validate(); err != nil {
			t.Errorf("length %d: %v", n, err)
		}
		switch {
		case g.ID != ID(d, n):
			t.Errorf("length %d: id = %q, want the derived %q", n, g.ID, ID(d, n))
		case g.Daily != d.String():
			t.Errorf("length %d: Daily = %q, want %q", n, g.Daily, d)
		case g.Length != n || g.MaxAttempts != n+1:
			t.Errorf("length %d: got length %d, %d attempts", n, g.Length, g.MaxAttempts)
		case g.Status != game.InProgress:
			t.Errorf("length %d: status = %q", n, g.Status)
		case !words.IsValidGuess(n, g.Answer):
			t.Errorf("length %d: answer %q is not a playable word", n, g.Answer)
		}
	}
}

func TestNewRejectsUnsupportedLength(t *testing.T) {
	if _, err := New(t.Context(), Local(), day(t, "2026-08-06"), 7); err == nil {
		t.Error("New with length 7 succeeded, want error")
	}
}

// A source that cannot speak for a day must stop the puzzle being built rather
// than a fallback word being invented — that is what an offline remote source
// will do every time it has nothing cached.
func TestSourceErrorStopsNew(t *testing.T) {
	g, err := New(t.Context(), stubSource{err: ErrUnavailable}, day(t, "2026-08-06"), 5)
	if !errors.Is(err, ErrUnavailable) {
		t.Errorf("err = %v, want ErrUnavailable", err)
	}
	if g != nil {
		t.Error("a game was returned despite the seed failing")
	}
}

// Answers must not all be the same word, and must not repeat in lockstep: a
// collapsed derivation would pass every other test here.
func TestAnswersSpreadAcrossDays(t *testing.T) {
	seen := make(map[string]int)
	d := day(t, "2026-01-01")
	for i := range 60 {
		g, err := New(t.Context(), Local(), DayOf(d.startsAt().AddDate(0, 0, i)), 5)
		if err != nil {
			t.Fatal(err)
		}
		seen[g.Answer]++
	}
	if len(seen) < 50 {
		t.Errorf("60 days produced only %d distinct answers", len(seen))
	}
}

// The tripwire for regenerating the word lists. The answer is an index into the
// shipped, sorted answer list, so regenerating with tools/genwords moves the
// word for every date not yet played — which would hand two versions of the app
// different boards on the same day. If this fails because the lists were
// regenerated deliberately, that is a coordinated, breaking release: update the
// table then.
func TestDailyAnswersAreStable(t *testing.T) {
	cases := []struct {
		date   string
		length int
		answer string
	}{
		{"2026-08-06", 4, "baby"},
		{"2026-08-06", 5, "broad"},
		{"2026-08-06", 6, "fruits"},
		{"2026-12-25", 5, "merge"},
		{"2027-01-01", 5, "bride"},
	}
	for _, c := range cases {
		g, err := New(t.Context(), Local(), day(t, c.date), c.length)
		if err != nil {
			t.Fatal(err)
		}
		if g.Answer != c.answer {
			t.Errorf("%s/%d: answer = %q, want %q", c.date, c.length, g.Answer, c.answer)
		}
	}
}
