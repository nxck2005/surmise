package ui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/nxck2005/surmise/internal/brand"
	"github.com/nxck2005/surmise/internal/game"
)

// trioAnswers are answers of each mode's length, forced onto the day's boards so
// a test can win or lose one deliberately.
var trioAnswers = map[int]string{4: "cool", 5: "crane", 6: "batter"}

// finishDaily plays one mode's daily to the end, winning in one guess or losing
// every attempt. It starts from the menu, as a player would.
func finishDaily(t *testing.T, m *Model, length int, win bool) {
	t.Helper()
	m.screen = screenMenu
	playDaily(t, m, length)

	answer := trioAnswers[length]
	m.game.g.Answer = answer
	if !win {
		// A word of the right length that is not the answer, guessed until the
		// board runs out of attempts.
		wrong := map[int]string{4: "epic", 5: "mount", 6: "sounds"}[length]
		for range m.game.g.MaxAttempts {
			for _, c := range wrong {
				send(t, m, string(c))
			}
			send(t, m, "enter")
		}
	} else {
		for _, c := range answer {
			send(t, m, string(c))
		}
		send(t, m, "enter")
	}
	if !m.game.g.Status.Done() {
		t.Fatalf("the %d-letter daily is still %v", length, m.game.g.Status)
	}
	send(t, m, "esc")
}

// showTrio returns the daily screen's frame, from the menu.
func showTrio(t *testing.T, m *Model) string {
	t.Helper()
	m.screen = screenMenu
	showDaily(t, m)
	return m.View().Content
}

// The day is one event, so the screen counts how much of it is done. It says
// nothing until there is something to count.
func TestTrioCountsTheDaysFinishedModes(t *testing.T) {
	m := dailyModel(t, Options{})

	if frame := showTrio(t, m); strings.Contains(frame, "trio") {
		t.Errorf("an untouched day already counts a trio:\n%s", frame)
	}

	finishDaily(t, m, 4, true)
	if frame := showTrio(t, m); !strings.Contains(frame, "trio · 1/3") {
		t.Errorf("one finished mode does not read 1/3:\n%s", frame)
	}

	finishDaily(t, m, 5, true)
	frame := showTrio(t, m)
	if !strings.Contains(frame, "trio · 2/3") {
		t.Errorf("two finished modes do not read 2/3:\n%s", frame)
	}
	// Not done yet, so there is nothing to copy and nothing offering it.
	if strings.Contains(frame, "copy") {
		t.Errorf("an unfinished trio offers a copy:\n%s", frame)
	}
	if cmd := m.copyTrio(); cmd != nil {
		t.Error("an unfinished trio produced a clipboard command")
	}
}

// A lost mode still completes the day: the card is a record of it, not a prize.
func TestTrioCompletesWithALoss(t *testing.T) {
	m := dailyModel(t, Options{})
	finishDaily(t, m, 4, true)
	finishDaily(t, m, 5, true)
	finishDaily(t, m, 6, false)

	frame := showTrio(t, m)
	if !strings.Contains(frame, "the trio · 3/3") {
		t.Fatalf("the day did not complete:\n%s", frame)
	}

	// The totals are the three saved games added up, so they cannot drift from
	// the boards the screen is listing.
	var guesses int
	var elapsed time.Duration
	for _, row := range m.daily.rows {
		guesses += row.attempts
		elapsed += row.elapsed
	}
	if want := fmt.Sprintf("%d guesses", guesses); !strings.Contains(frame, want) {
		t.Errorf("the card does not say %q:\n%s", want, frame)
	}
	if want := formatDuration(elapsed); !strings.Contains(frame, want) {
		t.Errorf("the card does not say %q:\n%s", want, frame)
	}
	if !strings.Contains(frame, "c copy") {
		t.Errorf("a finished trio offers no copy:\n%s", frame)
	}
}

// Deleting one of the three ends the day short. A tombstone keeps the status and
// drops the marks, so a spent mode has no board left to show or share.
func TestDeletedDailyHoldsTheTrioIncomplete(t *testing.T) {
	m := dailyModel(t, Options{})
	finishDaily(t, m, 4, true)
	finishDaily(t, m, 5, true)
	finishDaily(t, m, 6, true)

	var spent string
	for _, row := range m.daily.rows {
		if row.length == 6 {
			spent = row.id
		}
	}
	if err := m.store.Delete(spent); err != nil {
		t.Fatal(err)
	}

	frame := showTrio(t, m)
	if !strings.Contains(frame, "trio · 2/3") {
		t.Errorf("a spent mode still counts toward the day:\n%s", frame)
	}
	if strings.Contains(frame, "the trio") || strings.Contains(frame, "c copy") {
		t.Errorf("a spent day still offers its card:\n%s", frame)
	}
	if cmd := m.copyTrio(); cmd != nil {
		t.Error("a spent day produced a clipboard command")
	}
}

// Anything the keys can do, a click can do: both go through copyTrio.
func TestTrioCopiesByKeyAndByClick(t *testing.T) {
	m := dailyModel(t, Options{})
	for _, n := range []int{4, 5, 6} {
		finishDaily(t, m, n, true)
	}
	showTrio(t, m)

	_, cmd := m.Update(key("c"))
	if cmd == nil {
		t.Error("c produced no clipboard command")
	}
	if !m.daily.copyRequested {
		t.Error("c did not acknowledge the copy")
	}
	if frame := m.View().Content; !strings.Contains(frame, "copy requested") {
		t.Errorf("the screen does not acknowledge the copy:\n%s", frame)
	}

	// Reloading the screen clears the acknowledgement, so it belongs to the copy
	// and not to the day.
	m.daily.reload(m.store, m.day)
	if m.daily.copyRequested {
		t.Error("the acknowledgement survived a reload")
	}

	click(t, m, action{kind: actDailyCopy})
	if !m.daily.copyRequested {
		t.Error("the copy button did not acknowledge the copy")
	}
}

func TestShareTrioIsSpoilerSafe(t *testing.T) {
	rows := []dailyRow{
		{
			length: 4, status: game.Won, attempts: 1, maxAttempts: 5,
			elapsed: 30 * time.Second,
			marks:   [][]game.Mark{game.Score("cool", "cool")},
		},
		{
			length: 5, status: game.Won, attempts: 2, maxAttempts: 6,
			elapsed: 45 * time.Second,
			marks: [][]game.Mark{
				game.Score("about", "crane"),
				game.Score("crane", "crane"),
			},
		},
		{
			length: 6, status: game.Lost, attempts: 7, maxAttempts: 7,
			elapsed: 8 * time.Second,
			marks:   [][]game.Mark{game.Score("sounds", "batter")},
		},
	}

	want := fmt.Sprintf("%s daily 2026-08-06 · 3/3\n", brand.Name) +
		"10 guesses · 1:23\n" +
		"\n4 letters 1/5\n" +
		"■■■■\n" +
		"\n5 letters 2/6\n" +
		"□····\n" +
		"■■■■■\n" +
		"\n6 letters X/7\n" +
		"······\n" +
		"\n■ correct  □ present  · absent"
	got := shareTrio("2026-08-06", rows)
	if got != want {
		t.Errorf("shareTrio() =\n%q\nwant\n%q", got, want)
	}
	for _, spoiler := range []string{"cool", "crane", "batter", "about", "sounds"} {
		if strings.Contains(got, spoiler) {
			t.Errorf("the shared trio contains spoiler %q", spoiler)
		}
	}
}
