package ui

import (
	"strings"
	"testing"
	"time"

	"github.com/nxck2005/surmise/internal/game"
)

// openListByMenu opens the puzzle list from the menu, the way a player does.
func openListByMenu(t *testing.T, m *Model) {
	t.Helper()
	m.menu.cursor = menuIndex(t, m, choiceList, 0)
	press(t, m, "enter")
}

// hasLine reports whether the frame holds a line that is exactly want once the
// panel's border and padding are trimmed off.
func hasLine(frame, want string) bool {
	for _, line := range strings.Split(plain(frame), "\n") {
		if strings.Trim(line, " │") == want {
			return true
		}
	}
	return false
}

// listHelpLine is the panel's last content line, which is the help bar.
func listHelpLine(t *testing.T, frame string) string {
	t.Helper()
	lines := strings.Split(plain(frame), "\n")
	for i, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "╰") {
			for j := i - 1; j >= 0; j-- {
				if s := strings.Trim(lines[j], " │"); s != "" {
					return s
				}
			}
		}
	}
	t.Fatalf("no panel bottom in the frame:\n%s", plain(frame))
	return ""
}

// listAt is a fixed instant for the fixtures; none of these tests read a time.
var listAt = time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)

func TestListFilterBarIsHiddenUntilAsked(t *testing.T) {
	m := newModel(t)
	savedGame(t, m.store, fixedID(1), 5, game.Won, listAt, listAt)
	openListByMenu(t, m)
	if strings.Contains(plain(draw(t, m)), "m mode") {
		t.Errorf("the filter bar shows before f was pressed:\n%s", plain(draw(t, m)))
	}
}

func TestListFilterBarShowsAndHides(t *testing.T) {
	m := newModel(t)
	savedGame(t, m.store, fixedID(1), 5, game.Won, listAt, listAt)
	openListByMenu(t, m)

	press(t, m, "f")
	frame := draw(t, m)
	if !hasLine(frame, "m mode all · t kind all · s status all") {
		t.Errorf("no filter bar line in:\n%s", plain(frame))
	}
	if got, want := listHelpLine(t, frame), "enter open · d delete · f close · esc menu"; got != want {
		t.Errorf("help with the bar open = %q, want %q", got, want)
	}

	press(t, m, "m")
	if m.list.filter.length != 4 {
		t.Fatalf("filter.length = %d after m, want 4", m.list.filter.length)
	}
	press(t, m, "f")
	frame = draw(t, m)
	if strings.Contains(plain(frame), "m mode") {
		t.Errorf("the bar is still showing after f:\n%s", plain(frame))
	}
	if m.list.filter != (listFilter{}) {
		t.Errorf("filter = %+v after hiding the bar, want the zero filter", m.list.filter)
	}
	if len(m.list.items) != 1 {
		t.Errorf("%d items after hiding the bar, want 1", len(m.list.items))
	}
	if got, want := listHelpLine(t, frame), "enter open · d delete · f filter · esc menu"; got != want {
		t.Errorf("help with the bar closed = %q, want %q", got, want)
	}
}

func TestListFilterStepsTheMode(t *testing.T) {
	m := newModel(t)
	for i, length := range []int{4, 5, 6} {
		savedGame(t, m.store, fixedID(i+1), length, game.Won, listAt, listAt)
	}
	openListByMenu(t, m)
	press(t, m, "f")

	for _, length := range []int{4, 5, 6} {
		press(t, m, "m")
		if len(m.list.items) != 1 || m.list.items[0].Length != length {
			t.Fatalf("mode filter %d: items = %+v, want the one %d-letter puzzle", length, m.list.items, length)
		}
	}
	press(t, m, "m")
	if len(m.list.items) != 3 {
		t.Errorf("after wrapping to all, %d items, want 3", len(m.list.items))
	}
}

func TestListFilterStepsTheKind(t *testing.T) {
	m := newModel(t)
	ids := saveOneOfEachKind(t, m.store)
	openListByMenu(t, m)
	press(t, m, "f")

	for _, kind := range []puzzleKind{kindRandom, kindDaily, kindChallenge, kindCustom, kindRoom} {
		press(t, m, "t")
		if len(m.list.items) != 1 || m.list.items[0].ID != ids[kind] {
			t.Fatalf("kind filter %d: items = %+v, want only %s", kind, m.list.items, ids[kind])
		}
	}
	press(t, m, "t")
	if len(m.list.items) != 5 {
		t.Errorf("after wrapping to all, %d items, want 5", len(m.list.items))
	}
}

func TestListFilterStepsTheStatus(t *testing.T) {
	m := newModel(t)
	savedGame(t, m.store, fixedID(1), 5, game.InProgress, listAt, listAt)
	savedGame(t, m.store, fixedID(2), 5, game.Won, listAt, listAt)
	savedGame(t, m.store, fixedID(3), 5, game.Lost, listAt, listAt)
	openListByMenu(t, m)
	press(t, m, "f")

	for _, status := range []game.Status{game.InProgress, game.Won, game.Lost} {
		press(t, m, "s")
		if len(m.list.items) != 1 || m.list.items[0].Status != status {
			t.Fatalf("status filter %v: items = %+v, want the one puzzle with that status", status, m.list.items)
		}
	}
	press(t, m, "s")
	if len(m.list.items) != 3 {
		t.Errorf("after wrapping to all, %d items, want 3", len(m.list.items))
	}
}

func TestListFilterKeysNeedTheBar(t *testing.T) {
	m := newModel(t)
	savedGame(t, m.store, fixedID(1), 4, game.Won, listAt, listAt)
	savedGame(t, m.store, fixedID(2), 5, game.Won, listAt, listAt)
	openListByMenu(t, m)

	for _, k := range []string{"m", "t", "s"} {
		press(t, m, k)
	}
	if m.list.filter != (listFilter{}) {
		t.Errorf("filter = %+v after m, t, s with the bar hidden, want the zero filter", m.list.filter)
	}
	if len(m.list.items) != 2 {
		t.Errorf("%d items, want 2", len(m.list.items))
	}
}

func TestListFilterWithNoMatch(t *testing.T) {
	m := newModel(t)
	savedGame(t, m.store, fixedID(1), 5, game.Won, listAt, listAt)
	openListByMenu(t, m)
	press(t, m, "f")
	press(t, m, "s")

	frame := draw(t, m)
	if !hasLine(frame, "no puzzles match") {
		t.Errorf("no \"no puzzles match\" line in:\n%s", plain(frame))
	}
	if got, want := listHelpLine(t, frame), "f close · esc menu"; got != want {
		t.Errorf("help with nothing matching = %q, want %q", got, want)
	}
}

func TestListFilterByClick(t *testing.T) {
	m := newModel(t)
	savedGame(t, m.store, fixedID(1), 4, game.Won, listAt, listAt)
	savedGame(t, m.store, fixedID(2), 5, game.Won, listAt, listAt)
	openListByMenu(t, m)

	click(t, m, action{kind: actListFilters})
	if !m.list.filtersOpen {
		t.Fatal("clicking the filter button did not open the bar")
	}
	click(t, m, action{kind: actListFilter, index: filterMode})
	if len(m.list.items) != 1 || m.list.items[0].Length != 4 {
		t.Fatalf("after clicking the mode field, items = %+v, want the one 4-letter puzzle", m.list.items)
	}
	click(t, m, action{kind: actListFilters})
	if m.list.filtersOpen {
		t.Error("clicking the close button left the bar open")
	}
	if len(m.list.items) != 2 {
		t.Errorf("%d items after closing the bar, want 2", len(m.list.items))
	}
}

func TestListFilterStatusInlay(t *testing.T) {
	m := newModel(t)
	for i, length := range []int{4, 5, 5, 6, 6} {
		savedGame(t, m.store, fixedID(i+1), length, game.Won, listAt, listAt)
	}
	openListByMenu(t, m)
	if !strings.Contains(plain(draw(t, m)), "5 saved") {
		t.Errorf("no \"5 saved\" in:\n%s", plain(draw(t, m)))
	}

	press(t, m, "f")
	press(t, m, "m")
	press(t, m, "m")
	frame := plain(draw(t, m))
	if !strings.Contains(frame, "2 of 5") {
		t.Errorf("no \"2 of 5\" in:\n%s", frame)
	}
	if strings.Contains(frame, "5 saved") {
		t.Errorf("\"5 saved\" is still showing while filtered:\n%s", frame)
	}
}

func TestListFilterSurvivesADelete(t *testing.T) {
	m := newModel(t)
	savedGame(t, m.store, fixedID(1), 5, game.Won, listAt, listAt)
	savedGame(t, m.store, fixedID(2), 5, game.Won, listAt, listAt)
	savedGame(t, m.store, fixedID(3), 6, game.Won, listAt, listAt)
	openListByMenu(t, m)
	press(t, m, "f")
	press(t, m, "m")
	press(t, m, "m")
	press(t, m, "d")
	press(t, m, "d")

	if !m.list.filtersOpen {
		t.Error("the bar closed after a delete")
	}
	if m.list.filter.length != 5 {
		t.Errorf("filter.length = %d after a delete, want 5", m.list.filter.length)
	}
	if len(m.list.items) != 1 || m.list.items[0].Length != 5 {
		t.Errorf("items = %+v, want the one remaining 5-letter puzzle", m.list.items)
	}
}

func TestListFilterResetsOnReopen(t *testing.T) {
	m := newModel(t)
	savedGame(t, m.store, fixedID(1), 4, game.Won, listAt, listAt)
	savedGame(t, m.store, fixedID(2), 5, game.Won, listAt, listAt)
	openListByMenu(t, m)
	press(t, m, "f")
	press(t, m, "m")
	press(t, m, "esc")
	if m.screen != screenMenu {
		t.Fatalf("esc left the screen at %v, want the menu", m.screen)
	}
	openListByMenu(t, m)

	if m.list.filtersOpen {
		t.Error("the bar was still open after reopening the list")
	}
	if m.list.filter != (listFilter{}) {
		t.Errorf("filter = %+v after reopening, want the zero filter", m.list.filter)
	}
	if len(m.list.items) != 2 {
		t.Errorf("%d items after reopening, want 2", len(m.list.items))
	}
}

func TestListHelpWhenNothingIsSaved(t *testing.T) {
	m := newModel(t)
	openListByMenu(t, m)
	if got, want := listHelpLine(t, draw(t, m)), "esc menu"; got != want {
		t.Errorf("help with nothing saved = %q, want %q", got, want)
	}
}

// The filter bar and the blank under it cost the list two rows, so a long
// list with the bar open still fits a short terminal: an over-tall frame
// loses its top, the panel's title and close box with it.
func TestListFilterBarFitsAShortTerminal(t *testing.T) {
	m := newModel(t)
	for i := range 30 {
		savedGame(t, m.store, fixedID(i+1), 5, game.Won, listAt, listAt)
	}
	openListByMenu(t, m)
	press(t, m, "f")

	const height = 24
	frame := drawAt(t, m, height)
	if n := strings.Count(frame, "\n") + 1; n > height {
		t.Errorf("the frame is %d lines on a %d-line terminal:\n%s", n, height, plain(frame))
	}
	if !strings.Contains(plain(frame), "╭") {
		t.Errorf("the panel's top rule is missing:\n%s", plain(frame))
	}
}
