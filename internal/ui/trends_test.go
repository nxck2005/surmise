package ui

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/nxck2005/surmise/internal/game"
	"github.com/nxck2005/surmise/internal/store"
)

// trendsDay is 12:00 UTC on a day of October 2026, the fixtures' only clock.
func trendsDay(day int) time.Time {
	return time.Date(2026, 10, day, 12, 0, 0, 0, time.UTC)
}

// trendsGame builds a puzzle with a known answer, plays the guesses, and
// stamps the time it took and the day it was started and last updated.
func trendsGame(t *testing.T, id, answer string, guesses []string, elapsedMS int64, at time.Time) *game.Game {
	t.Helper()
	g, err := game.NewFrom(id, answer, 5)
	if err != nil {
		t.Fatal(err)
	}
	for _, w := range guesses {
		if err := g.Guess(w); err != nil {
			t.Fatal(err)
		}
	}
	g.ElapsedMS = elapsedMS
	g.StartedAt, g.UpdatedAt = at, at
	return g
}

func trendsSave(t *testing.T, s store.Store, g *game.Game) {
	t.Helper()
	if err := s.Save(g); err != nil {
		t.Fatal(err)
	}
}

// trendsCustomWin saves a one-guess custom win, which moves no figure.
func trendsCustomWin(t *testing.T, s store.Store, at time.Time) *game.Game {
	t.Helper()
	cu, err := game.NewCustom("crane", 5)
	if err != nil {
		t.Fatal(err)
	}
	if err := cu.Guess("crane"); err != nil {
		t.Fatal(err)
	}
	cu.ElapsedMS = 5_000
	cu.StartedAt, cu.UpdatedAt = at, at
	trendsSave(t, s, cu)
	return cu
}

// trendsSaved is the plan's fixture: the saves, with the profile open on a
// 5-letter mode.
func trendsSaved(t *testing.T) *Model {
	t.Helper()
	m := newModel(t)
	s := m.store
	trendsSave(t, s, trendsGame(t, fixedID(30), "crane", []string{"slate", "trace", "crane"}, 60_000, trendsDay(1)))
	trendsSave(t, s, trendsGame(t, fixedID(31), "slate", []string{"crane", "slate"}, 41_000, trendsDay(2)))
	trendsSave(t, s, trendsGame(t, fixedID(32), "crane",
		[]string{"slate", "slate", "slate", "slate", "slate", "slate"}, 120_000, trendsDay(3)))
	trendsSave(t, s, trendsGame(t, fixedID(33), "crane", []string{"slate", "trace", "grace", "crane"}, 90_000, trendsDay(4)))
	// Game 5 is custom and must not appear anywhere on the screen.
	trendsCustomWin(t, s, trendsDay(5))

	m.length = 5
	openProfileByMenu(t, m)
	return m
}

// trendsModel is the fixture with the trends screen open.
func trendsModel(t *testing.T) *Model {
	t.Helper()
	m := trendsSaved(t)
	press(t, m, "t")
	if m.screen != screenTrends {
		t.Fatalf("screen = %v, want the trends screen", m.screen)
	}
	return m
}

// trendsLines is the trends body as plain lines with their trailing spaces
// gone.
func trendsLines(m *Model) []string {
	lines := strings.Split(plain(m.trends.view(nil)), "\n")
	for i, l := range lines {
		lines[i] = strings.TrimRight(l, " ")
	}
	return lines
}

func TestTrendsScreen(t *testing.T) {
	m := trendsModel(t)
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 40})

	// CODE2 is game 2's code, right-aligned in ten columns with its #.
	code := "#" + game.Code(fixedID(31))
	golden := strings.ReplaceAll(`|‹ 5 letters ›|
||
|                  last 4  all time|
|win rate             75%       75%|
|avg guesses            3         3|
|avg time            1:04      1:04|
||
|personal bests|
|fastest win          41s   #CODE2|
|fewest guesses         2   #CODE2|
||
|top openers        games       won       avg|
|SLATE                  3       67%       3.5|
|CRANE                  1      100%         2|`, "#CODE2", code)
	wantCalendar(t, trendsLines(m), golden)
}

func TestTrendsModeEnds(t *testing.T) {
	m := trendsModel(t)

	press(t, m, "left")
	if m.trends.length != 4 {
		t.Fatalf("mode = %d after left, want 4", m.trends.length)
	}
	want := []string{"  4 letters ›", "", "no finished 4-letter games yet"}
	got := trendsLines(m)
	if len(got) != len(want) {
		t.Fatalf("body = %q, want %q", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("line %d = %q, want %q", i+1, got[i], want[i])
		}
	}

	// (prove) The first mode does not wrap.
	press(t, m, "left")
	if m.trends.length != 4 {
		t.Errorf("mode = %d after left on the first mode, want 4", m.trends.length)
	}

	press(t, m, "right")
	press(t, m, "right")
	if m.trends.length != 6 {
		t.Fatalf("mode = %d after two rights, want 6", m.trends.length)
	}
	press(t, m, "right")
	if m.trends.length != 6 {
		t.Errorf("mode = %d after right on the last mode, want 6", m.trends.length)
	}
}

func TestTrendsByMouse(t *testing.T) {
	m := trendsModel(t)

	click(t, m, action{kind: actTrendsPrev})
	if m.trends.length != 4 {
		t.Errorf("mode = %d after clicking the left arrow, want 4", m.trends.length)
	}
	click(t, m, action{kind: actTrendsNext})
	if m.trends.length != 5 {
		t.Errorf("mode = %d after clicking the right arrow, want 5", m.trends.length)
	}
}

func TestTrendsShedOpenersFirst(t *testing.T) {
	m := trendsModel(t)
	form := []string{"win rate", "avg guesses", "avg time", "all time"}

	for _, tc := range []struct {
		height, budget int
		bests, openers bool
	}{
		{19, 13, true, false},
		{15, 9, false, false},
	} {
		m.Update(tea.WindowSizeMsg{Width: 100, Height: tc.height})
		if got := bodyBudget(tc.height); got != tc.budget {
			t.Fatalf("bodyBudget(%d) = %d, the plan says %d", tc.height, got, tc.budget)
		}
		body := plain(m.trends.view(nil))
		if has := strings.Contains(body, "personal bests"); has != tc.bests {
			t.Errorf("height %d: personal bests shown = %v, want %v\n%s", tc.height, has, tc.bests, body)
		}
		if has := strings.Contains(body, "top openers"); has != tc.openers {
			t.Errorf("height %d: top openers shown = %v, want %v\n%s", tc.height, has, tc.openers, body)
		}
		if !strings.Contains(body, "5 letters") {
			t.Errorf("height %d: no mode line\n%s", tc.height, body)
		}
		for _, f := range form {
			if !strings.Contains(body, f) {
				t.Errorf("height %d: the form table lost %q\n%s", tc.height, f, body)
			}
		}
	}
}

func TestProfileHelpOffersBoth(t *testing.T) {
	m := trendsSaved(t)

	if got, want := listHelpLine(t, draw(t, m)), "c calendar · t trends · esc menu"; got != want {
		t.Errorf("profile help = %q, want %q", got, want)
	}
	click(t, m, action{kind: actOpenTrends})
	if m.screen != screenTrends {
		t.Fatalf("screen = %v after clicking the trends button, want the trends screen", m.screen)
	}
	if got, want := listHelpLine(t, draw(t, m)), "←/→ mode · esc profile"; got != want {
		t.Errorf("trends help = %q, want %q", got, want)
	}
	press(t, m, "esc")
	if m.screen != screenProfile {
		t.Errorf("screen = %v after esc, want the profile", m.screen)
	}
}

// A board played in a test banks almost no time, so it could never be the
// fastest. The tests below save finished games directly and raise the result
// the way the finish path does.

// slowWin saves a six-guess win that took an hour.
func slowWin(t *testing.T, s store.Store) {
	t.Helper()
	trendsSave(t, s, trendsGame(t, fixedID(40), "crane",
		[]string{"slate", "slate", "slate", "slate", "slate", "crane"}, 3_600_000, trendsDay(1)))
}

// resultOf opens a saved game's result screen and returns the frame.
func resultOf(t *testing.T, m *Model, g *game.Game) string {
	t.Helper()
	m.openGame(g, true)
	m.openResult()
	if m.screen != screenResult {
		t.Fatalf("screen = %v, want the result", m.screen)
	}
	return draw(t, m)
}

func TestResultSaysANewBest(t *testing.T) {
	m := newModel(t)
	slowWin(t, m.store)

	one := trendsGame(t, fixedID(41), "crane", []string{"crane"}, 5_000, trendsDay(2))
	trendsSave(t, m.store, one)
	frame := resultOf(t, m, one)
	if !hasLine(frame, "your fastest win and fewest guesses at 5 letters") {
		t.Errorf("no best-yet note in the result:\n%s", plain(frame))
	}

	two := trendsGame(t, fixedID(42), "crane", []string{"slate", "crane"}, 3_000, trendsDay(3))
	trendsSave(t, m.store, two)
	frame = resultOf(t, m, two)
	if !hasLine(frame, "your fastest win at 5 letters") {
		t.Errorf("no fastest-win note in the result:\n%s", plain(frame))
	}
	if strings.Contains(frame, "fewest") {
		t.Errorf("the second win is not the fewest guesses, but the result says so:\n%s", plain(frame))
	}
}

func TestResultSaysNothingForAFirstWin(t *testing.T) {
	m := newModel(t)
	one := trendsGame(t, fixedID(41), "crane", []string{"crane"}, 5_000, trendsDay(2))
	trendsSave(t, m.store, one)

	frame := resultOf(t, m, one)
	for _, s := range []string{"your fastest", "your fewest"} {
		if strings.Contains(frame, s) {
			t.Errorf("a first win in a mode says %q:\n%s", s, plain(frame))
		}
	}
}

func TestResultSaysNothingForACustomWin(t *testing.T) {
	m := newModel(t)
	slowWin(t, m.store)
	cu := trendsCustomWin(t, m.store, trendsDay(2))

	frame := resultOf(t, m, cu)
	for _, s := range []string{"your fastest", "your fewest"} {
		if strings.Contains(frame, s) {
			t.Errorf("a custom win says %q:\n%s", s, plain(frame))
		}
	}
}
