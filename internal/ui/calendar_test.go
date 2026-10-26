package ui

import (
	"context"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/nxck2005/surmise/internal/daily"
	"github.com/nxck2005/surmise/internal/game"
	"github.com/nxck2005/surmise/internal/stats"
	"github.com/nxck2005/surmise/internal/words"
)

// calendarNow is the fixture's today. It is a package variable so the clock
// pointer withClock holds stays alive for the whole test.
var calendarNow = time.Date(2026, 10, 26, 12, 0, 0, 0, time.UTC)

// calDay is a calendar date, for the fixtures and the assertions.
func calDay(year int, month time.Month, day int) daily.Day { return daily.DayFrom(year, month, day) }

// calendarIDs are the two puzzles of 2026-10-20 that the day list must show.
type calendarIDs struct{ custom, room string }

// calendarBase is a model whose zone and clock are pinned to the fixture's.
func calendarBase(t *testing.T) *Model {
	t.Helper()
	m := newModel(t)
	m.zone = time.UTC
	withClock(t, &calendarNow)
	return m
}

// openProfileByMenu opens the profile from the menu, the way a player does.
func openProfileByMenu(t *testing.T, m *Model) {
	t.Helper()
	m.menu.cursor = menuIndex(t, m, choiceProfile, 0)
	press(t, m, "enter")
	if m.screen != screenProfile {
		t.Fatalf("screen = %v, want the profile", m.screen)
	}
}

// calendarSaved is the plan's fixture: the saves, with the profile open.
func calendarSaved(t *testing.T) (*Model, calendarIDs) {
	t.Helper()
	m := calendarBase(t)
	at := func(y int, mo time.Month, d int) time.Time { return time.Date(y, mo, d, 12, 0, 0, 0, time.UTC) }
	var ids calendarIDs

	// 2026-10-26: two won, one lost.
	savedGame(t, m.store, fixedID(10), 5, game.Won, at(2026, 10, 26), at(2026, 10, 26))
	savedGame(t, m.store, fixedID(11), 5, game.Won, at(2026, 10, 26), at(2026, 10, 26))
	savedGame(t, m.store, fixedID(12), 5, game.Lost, at(2026, 10, 26), at(2026, 10, 26))

	// A won puzzle that was deleted: it leaves a tombstone, which never shows.
	savedGame(t, m.store, fixedID(13), 5, game.Won, at(2026, 10, 26), at(2026, 10, 26))
	if err := m.store.Delete(fixedID(13)); err != nil {
		t.Fatal(err)
	}

	// 2026-10-20: a custom puzzle and a room round.
	cu, err := game.NewCustom("crane", 5)
	if err != nil {
		t.Fatal(err)
	}
	playOut(t, cu, game.Won, "")
	cu.StartedAt = at(2026, 10, 20)
	if err := m.store.Save(cu); err != nil {
		t.Fatal(err)
	}
	ids.custom = cu.ID

	r, err := game.NewRoomRound(game.RoomInfo{
		Code: "K7M2QX", Mode: "live", Round: 1, Seat: 1,
		Ranking: []game.RoomRank{{Seat: 1, Name: "nick", Solved: true, Guesses: 1,
			ElapsedMS: 81_000, Rows: [][]game.Mark{game.Score("crane", "crane")}}},
	}, "crane", []string{"crane"}, 81_000, time.Date(2026, 10, 20, 12, 1, 21, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if err := m.store.Save(r); err != nil {
		t.Fatal(err)
	}
	ids.room = r.ID

	// The daily of 2026-01-01, lost, started the day after: it is filed on its
	// own date.
	d, err := daily.New(context.Background(), daily.Local(), calDay(2026, time.January, 1), 5)
	if err != nil {
		t.Fatal(err)
	}
	wrong, err := words.AnswerAt(5, 0)
	if err != nil {
		t.Fatal(err)
	}
	if wrong == d.Answer {
		if wrong, err = words.AnswerAt(5, 1); err != nil {
			t.Fatal(err)
		}
	}
	playOut(t, d, game.Lost, wrong)
	d.StartedAt = at(2026, 1, 2)
	if err := m.store.Save(d); err != nil {
		t.Fatal(err)
	}

	savedGame(t, m.store, fixedID(14), 5, game.Won, at(2025, 12, 31), at(2025, 12, 31))
	savedGame(t, m.store, fixedID(15), 5, game.Won, at(2025, 12, 31), at(2025, 12, 31))
	savedGame(t, m.store, fixedID(16), 5, game.InProgress, at(2025, 10, 26), at(2025, 10, 26))
	savedGame(t, m.store, fixedID(17), 5, game.Won, at(2025, 10, 25), at(2025, 10, 25))
	savedGame(t, m.store, fixedID(18), 5, game.Won, at(2026, 11, 30), at(2026, 11, 30))

	openProfileByMenu(t, m)
	return m, ids
}

// calendarModel is the fixture with the calendar open.
func calendarModel(t *testing.T) *Model {
	t.Helper()
	m, _ := calendarSaved(t)
	press(t, m, "c")
	if m.screen != screenCalendar {
		t.Fatalf("screen = %v, want the calendar", m.screen)
	}
	return m
}

// calendarSized sets the terminal size and returns the calendar's body as
// plain lines with their trailing spaces gone.
func calendarSized(t *testing.T, m *Model, w, h int) []string {
	t.Helper()
	m.Update(tea.WindowSizeMsg{Width: w, Height: h})
	return calendarLines(m)
}

func calendarLines(m *Model) []string {
	lines := strings.Split(plain(m.calendar.view(nil)), "\n")
	for i, l := range lines {
		lines[i] = strings.TrimRight(l, " ")
	}
	return lines
}

// wantCalendar checks a body against a golden text line by line. The golden's
// lines sit between two | marks, which are not part of the text.
func wantCalendar(t *testing.T, got []string, golden string) {
	t.Helper()
	var want []string
	for _, l := range strings.Split(golden, "\n") {
		if len(l) < 2 || l[0] != '|' || l[len(l)-1] != '|' {
			t.Fatalf("golden line %q is not between | marks", l)
		}
		want = append(want, l[1:len(l)-1])
	}
	if len(got) != len(want) {
		t.Errorf("got %d lines, want %d\n got:\n%s\nwant:\n%s",
			len(got), len(want), strings.Join(got, "\n"), strings.Join(want, "\n"))
		return
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("line %d\n got: %q\nwant: %q", i+1, got[i], want[i])
		}
	}
}

func TestCalendarDrawsTheLastYear(t *testing.T) {
	m := calendarModel(t)
	wantCalendar(t, calendarSized(t, m, 100, 40), calendarGoldenB1)
}

func TestCalendarYears(t *testing.T) {
	m := calendarModel(t)
	calendarSized(t, m, 100, 40)
	today := calDay(2026, time.October, 26)

	press(t, m, "]")
	wantCalendar(t, calendarLines(m), calendarGoldenB2)
	if m.calendar.cursor != today {
		t.Errorf("cursor = %v in 2026, want %v", m.calendar.cursor, today)
	}

	press(t, m, "]")
	wantCalendar(t, calendarLines(m), calendarGoldenB3)
	if want := calDay(2025, time.December, 31); m.calendar.cursor != want {
		t.Errorf("cursor = %v in 2025, want %v", m.calendar.cursor, want)
	}

	press(t, m, "]")
	if m.calendar.shown != 2 {
		t.Errorf("shown = %d after the last year, want 2", m.calendar.shown)
	}

	press(t, m, "[")
	if m.calendar.shown != 1 || m.calendar.cursor != today {
		t.Errorf("after [: shown = %d, cursor = %v; want 1, %v", m.calendar.shown, m.calendar.cursor, today)
	}
	press(t, m, "[")
	if m.calendar.shown != 0 || m.calendar.cursor != today {
		t.Errorf("after [ again: shown = %d, cursor = %v; want 0, %v", m.calendar.shown, m.calendar.cursor, today)
	}
}

func TestCalendarShowsTheMostRecentWeeksThatFit(t *testing.T) {
	m := calendarModel(t)
	wantCalendar(t, calendarSized(t, m, 52, 40), calendarGoldenB4)

	for range 39 {
		press(t, m, "left")
	}
	if want := calDay(2026, time.January, 26); m.calendar.cursor != want {
		t.Fatalf("cursor = %v after 39 steps, want %v", m.calendar.cursor, want)
	}
	press(t, m, "left")
	if want := calDay(2026, time.January, 26); m.calendar.cursor != want {
		t.Errorf("cursor = %v after one step too many, want it to stay on %v", m.calendar.cursor, want)
	}
}

func TestCalendarShedsOnAShortTerminal(t *testing.T) {
	m := calendarModel(t)
	wantCalendar(t, calendarSized(t, m, 100, 18), calendarGoldenB5)
}

func TestCalendarLevels(t *testing.T) {
	for _, tc := range []struct{ n, peak, want int }{
		{3, 3, 4}, {2, 3, 3}, {1, 3, 2}, {0, 3, 0}, {1, 1, 4}, {1, 0, 0},
	} {
		if got := calendarLevel(tc.n, tc.peak); got != tc.want {
			t.Errorf("calendarLevel(%d, %d) = %d, want %d", tc.n, tc.peak, got, tc.want)
		}
	}

	m := calendarModel(t)
	r := m.calendar.rng()
	peak := 0
	for d := r.from; !r.to.Before(d); d = d.AddDays(1) {
		peak = max(peak, m.calendar.tally(d).Games)
	}
	if peak != 3 {
		t.Fatalf("peak = %d, want 3", peak)
	}
	for _, tc := range []struct {
		day  daily.Day
		want int
	}{
		{calDay(2026, time.October, 26), 4},
		{calDay(2026, time.October, 20), 3},
		{calDay(2026, time.January, 1), 2},
		{calDay(2025, time.October, 26), 2},
		{calDay(2025, time.October, 27), 0},
	} {
		if got := calendarLevel(m.calendar.tally(tc.day).Games, peak); got != tc.want {
			t.Errorf("level of %v = %d, want %d", tc.day, got, tc.want)
		}
	}
}

func TestCalendarCursorStaysInAWeek(t *testing.T) {
	m := calendarModel(t)
	calendarSized(t, m, 100, 40)

	for _, step := range []struct {
		key  string
		want daily.Day
	}{
		{"right", calDay(2026, time.October, 26)}, // the next week is the future
		{"down", calDay(2026, time.October, 26)},  // 2026-10-27 is in the future
		{"up", calDay(2026, time.October, 25)},
		{"up", calDay(2026, time.October, 25)}, // Sunday: the top of the column
		{"left", calDay(2026, time.October, 18)},
		{"down", calDay(2026, time.October, 19)},
	} {
		press(t, m, step.key)
		if m.calendar.cursor != step.want {
			t.Fatalf("after %q the cursor is %v, want %v", step.key, m.calendar.cursor, step.want)
		}
	}
}

func TestCalendarDetailFollowsTheDay(t *testing.T) {
	m := calendarModel(t)
	calendarSized(t, m, 100, 40)

	detail := func() string {
		lines := calendarLines(m)
		return lines[len(lines)-3]
	}
	from := m.calendar.rng().from
	m.calendar.point(calDay(2026, time.January, 1).DaysSince(from))
	if got, want := detail(), "1 game on thu 1 jan 2026 · 1 lost"; got != want {
		t.Errorf("detail = %q, want %q", got, want)
	}
	m.calendar.point(calDay(2026, time.January, 2).DaysSince(from))
	if got, want := detail(), "no games on fri 2 jan 2026"; got != want {
		t.Errorf("detail = %q, want %q", got, want)
	}
}

func TestCalendarListsADay(t *testing.T) {
	m, ids := calendarSaved(t)
	press(t, m, "c")
	draw(t, m)

	m.calendar.point(calDay(2026, time.October, 20).DaysSince(m.calendar.rng().from))
	press(t, m, "enter")
	if m.screen != screenList {
		t.Fatalf("screen = %v, want the list", m.screen)
	}
	if len(m.list.items) != 2 {
		t.Fatalf("%d items, want 2", len(m.list.items))
	}
	got := map[string]bool{m.list.items[0].ID: true, m.list.items[1].ID: true}
	if !got[ids.custom] || !got[ids.room] {
		t.Errorf("items are %v, want the custom puzzle %s and the room round %s", got, ids.custom, ids.room)
	}

	frame := draw(t, m)
	if !hasLine(frame, "tue 20 oct 2026") {
		t.Errorf("no line equal to the day:\n%s", plain(frame))
	}
	if !strings.Contains(plain(frame), "2 of 11") {
		t.Errorf("no \"2 of 11\" in the frame:\n%s", plain(frame))
	}
	if got, want := listHelpLine(t, frame), "enter open · d delete · f filter · esc back"; got != want {
		t.Errorf("help = %q, want %q", got, want)
	}

	press(t, m, "esc")
	if m.screen != screenCalendar {
		t.Fatalf("screen = %v after esc, want the calendar", m.screen)
	}
	if want := calDay(2026, time.October, 20); m.calendar.cursor != want {
		t.Errorf("cursor = %v after coming back, want %v", m.calendar.cursor, want)
	}
	press(t, m, "esc")
	if m.screen != screenProfile {
		t.Errorf("screen = %v after a second esc, want the profile", m.screen)
	}
}

func TestCalendarEnterOnAnEmptyDayDoesNothing(t *testing.T) {
	m := calendarModel(t)
	calendarSized(t, m, 100, 40)

	m.calendar.point(calDay(2026, time.January, 2).DaysSince(m.calendar.rng().from))
	press(t, m, "enter")
	if m.screen != screenCalendar {
		t.Errorf("screen = %v after enter on an empty day, want the calendar", m.screen)
	}
	if help := listHelpLine(t, draw(t, m)); strings.Contains(help, "enter puzzles") {
		t.Errorf("the help bar offers enter on an empty day: %q", help)
	}
}

func TestCalendarByMouse(t *testing.T) {
	m := calendarModel(t)
	draw(t, m)
	from := m.calendar.rng().from

	jan1 := calDay(2026, time.January, 1)
	point(t, m, action{kind: actCalendarDay, index: jan1.DaysSince(from)})
	if m.calendar.cursor != jan1 {
		t.Errorf("cursor = %v after pointing at %v", m.calendar.cursor, jan1)
	}

	click(t, m, action{kind: actCalendarRange, index: 2})
	if m.calendar.shown != 2 || !strings.Contains(plain(m.calendar.view(nil)), "4 games in 2025") {
		t.Errorf("shown = %d after clicking the third range, want 2 (2025)", m.calendar.shown)
	}
	click(t, m, action{kind: actCalendarRange, index: 0})
	if m.calendar.shown != 0 {
		t.Fatalf("shown = %d after clicking the first range", m.calendar.shown)
	}

	oct20 := calDay(2026, time.October, 20)
	click(t, m, action{kind: actCalendarDay, index: oct20.DaysSince(from)})
	if m.screen != screenList || m.list.day != oct20 {
		t.Errorf("screen = %v, list day = %v; want the list of %v", m.screen, m.list.day, oct20)
	}
	if len(m.list.items) != 2 {
		t.Errorf("%d items, want 2", len(m.list.items))
	}
}

func TestProfileOpensTheCalendar(t *testing.T) {
	m, _ := calendarSaved(t)

	if got, want := listHelpLine(t, draw(t, m)), "c calendar · esc menu"; got != want {
		t.Errorf("profile help = %q, want %q", got, want)
	}
	click(t, m, action{kind: actOpenCalendar})
	if m.screen != screenCalendar {
		t.Fatalf("screen = %v after clicking the calendar button, want the calendar", m.screen)
	}
	if got, want := listHelpLine(t, draw(t, m)), "↑/↓/←/→ move · enter puzzles · [/] year · esc profile"; got != want {
		t.Errorf("calendar help = %q, want %q", got, want)
	}
	press(t, m, "esc")
	if m.screen != screenProfile {
		t.Errorf("screen = %v after esc, want the profile", m.screen)
	}
}

func TestCalendarGeometry(t *testing.T) {
	r := calRange{from: calDay(2028, time.January, 1), to: calDay(2028, time.December, 31)}
	start, cols := r.grid()
	if want := calDay(2027, time.December, 26); start != want || cols != 54 {
		t.Errorf("2028: start %v, %d columns; want %v, 54", start, cols, want)
	}

	m := calendarModel(t)
	start, cols = m.calendar.rng().grid()
	if want := calDay(2025, time.October, 26); start != want || cols != 53 {
		t.Errorf("last year: start %v, %d columns; want %v, 53", start, cols, want)
	}
}

func TestCalendarWideGlyph(t *testing.T) {
	m := calendarModel(t)
	// After the model: New applies the saved theme, which would undo this one.
	withTheme(t, themed(t, "[glyphs]\nday = \"██\"\n"))
	wantCalendar(t, calendarSized(t, m, 120, 40), calendarGoldenB6)
}

// A day with many puzzles, listed with the filter bar open, must still fit a
// 24-row terminal: the day line and its blank are rows the window pays for.
func TestDayListFitsAShortTerminal(t *testing.T) {
	m := calendarBase(t)
	for i := range 30 {
		savedGame(t, m.store, fixedID(100+i), 5, game.Won, calendarNow, calendarNow)
	}
	openProfileByMenu(t, m)
	press(t, m, "c")
	press(t, m, "enter")
	if m.screen != screenList {
		t.Fatalf("screen = %v, want the list", m.screen)
	}
	press(t, m, "f")

	frame := drawAt(t, m, 24)
	if n := strings.Count(frame, "\n") + 1; n > 24 {
		t.Errorf("the frame is %d lines tall on a 24-row terminal:\n%s", n, plain(frame))
	}
	if !strings.Contains(plain(frame), "╭") {
		t.Errorf("the panel's top rule is off the screen:\n%s", plain(frame))
	}
}

func TestCalendarRangesAreBounded(t *testing.T) {
	tallies := map[daily.Day]stats.DayTally{}
	for _, y := range []int{1, 2019, 2020, 2021, 2022, 2023, 2024, 2025} {
		tallies[calDay(y, time.March, 3)] = stats.DayTally{Games: 1}
	}
	today := calDay(2026, time.October, 26)
	tallies[today] = stats.DayTally{Games: 1}

	labels := func() []string {
		var out []string
		for _, r := range calendarRanges(tallies, today) {
			out = append(out, r.label)
		}
		return out
	}
	want := "last year,2026,2025,2024,2023,2022,2021"
	if got := strings.Join(labels(), ","); got != want {
		t.Errorf("ranges = %s, want %s", got, want)
	}

	tallies[calDay(2027, time.January, 5)] = stats.DayTally{Games: 1}
	if got := strings.Join(labels(), ","); got != want {
		t.Errorf("with a game in 2027: ranges = %s, want %s", got, want)
	}
}

// The selected day is the one square drawn reversed, so the player can see
// which day the detail line is about. The plain text cannot show it.
func TestCalendarShowsTheSelectedDay(t *testing.T) {
	m := calendarModel(t)
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 40})
	view := m.calendar.view(nil)
	l := m.calendar.layout()
	peak := 3 // the fixture's busiest day in the last year
	ramp := calendarRamp()
	level := calendarLevel(m.calendar.tally(m.calendar.cursor).Games, peak)
	picked := lipgloss.NewStyle().Foreground(ramp[level]).Reverse(true).Render(l.glyph)
	if n := strings.Count(view, picked); n != 1 {
		t.Errorf("the reversed square appears %d times, want once", n)
	}
	m.calendar.move(-1)
	level = calendarLevel(m.calendar.tally(m.calendar.cursor).Games, peak)
	moved := lipgloss.NewStyle().Foreground(ramp[level]).Reverse(true).Render(l.glyph)
	if n := strings.Count(m.calendar.view(nil), moved); n != 1 {
		t.Errorf("after moving, the reversed square appears %d times, want once", n)
	}
}
