package ui

import (
	"strings"
	"testing"

	"github.com/nxck2005/surmise/internal/daily"
)

// showMenu raises the menu the way the app does — through openMenu, so the
// status line is derived from what is on disk — and returns the frame.
func showMenu(t *testing.T, m *Model) string {
	t.Helper()
	m.openMenu()
	return m.View().Content
}

// A fresh install has nothing to say: no mode of the day is finished and no
// streak is running, so the menu renders exactly as it did before the status
// line existed — tagline, not an empty or half-said line.
func TestMenuStatusIsQuietOnAFreshInstall(t *testing.T) {
	m := dailyModel(t, Options{})

	frame := showMenu(t, m)
	if strings.Contains(frame, "daily 0/3") {
		t.Errorf("an untouched day still counts:\n%s", frame)
	}
	if strings.Contains(frame, "streak") {
		t.Errorf("a fresh install claims a streak:\n%s", frame)
	}
	if !strings.Contains(frame, tagline) {
		t.Errorf("with nothing to say, the tagline is gone:\n%s", frame)
	}
}

// The line grows with the day: one mode finished reads 1/3, all three read 3/3,
// and a win in the same visit starts a streak.
func TestMenuStatusCountsTheDaysModes(t *testing.T) {
	m := dailyModel(t, Options{})

	finishDaily(t, m, 4, true)
	frame := showMenu(t, m)
	if !strings.Contains(frame, "daily 1/3 · streak 1") {
		t.Errorf("one won mode does not read 1/3 with a streak:\n%s", frame)
	}
	if strings.Contains(frame, tagline) {
		t.Errorf("the tagline stayed beside the status:\n%s", frame)
	}

	finishDaily(t, m, 5, true)
	finishDaily(t, m, 6, false)
	frame = showMenu(t, m)
	if !strings.Contains(frame, "daily 3/3") {
		t.Errorf("a lost mode still finishes the day:\n%s", frame)
	}
}

// Deleting a finished mode ends the day short. A tombstone keeps the status but
// drops the board, so a spent mode is not one the menu can count.
func TestMenuStatusSkipsASpentMode(t *testing.T) {
	m := dailyModel(t, Options{})
	finishDaily(t, m, 4, true)
	finishDaily(t, m, 5, true)

	var spent string
	for _, row := range m.daily.rows {
		if row.length == 5 {
			spent = row.id
		}
	}
	if err := m.store.Delete(spent); err != nil {
		t.Fatal(err)
	}

	if frame := showMenu(t, m); strings.Contains(frame, "daily 2/3") {
		t.Errorf("a spent mode counts toward the day:\n%s", frame)
	}
}

// The line follows the root's day, not the wall clock — the same rule the
// profile and the daily screen follow, and what makes -day move it too.
func TestMenuStatusFollowsTheRootsDay(t *testing.T) {
	m := dailyModel(t, Options{})
	finishDaily(t, m, 4, true)

	pinned, err := daily.ParseDay(testDay)
	if err != nil {
		t.Fatal(err)
	}
	m.day = pinned.AddDays(-1)
	if frame := showMenu(t, m); strings.Contains(frame, "daily 1/3") {
		t.Errorf("another day's menu counts today's modes:\n%s", frame)
	}

	m.day = pinned
	if frame := showMenu(t, m); !strings.Contains(frame, "daily 1/3") {
		t.Errorf("the day's own menu lost its count:\n%s", frame)
	}
}

// The streak half follows the profile's figure: custom puzzles move nothing,
// and a loss ends it.
func TestMenuStatusStreakFollowsTheProfile(t *testing.T) {
	m := dailyModel(t, Options{})
	finishDaily(t, m, 4, true)

	// A custom puzzle is played and saved, but counts for nothing.
	openCustom(t, m)
	typeSecret(t, m, "zzzz")
	for _, c := range "zzzz" {
		send(t, m, string(c))
	}
	send(t, m, "enter")
	send(t, m, "esc")

	if frame := showMenu(t, m); !strings.Contains(frame, "streak 1") {
		t.Errorf("a custom puzzle moved the streak:\n%s", frame)
	}
}
