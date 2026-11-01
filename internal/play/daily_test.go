package play

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/nxck2005/surmise/internal/daily"
	"github.com/nxck2005/surmise/internal/game"
)

// failingSource is a seed source that must not be asked.
type failingSource struct{ asked int }

func (f *failingSource) Seed(context.Context, daily.Day, int) (daily.Seed, error) {
	f.asked++
	return daily.Seed{}, errors.New("offline")
}

var testDay = daily.DayFrom(2026, time.November, 1)

func openDaily(t *testing.T, a *App, n int) *Board {
	t.Helper()
	b, err := a.OpenDaily(context.Background(), testDay, n)
	if err != nil {
		t.Fatalf("OpenDaily(%d): %v", n, err)
	}
	return b
}

// solve wins a board in one guess.
func solve(t *testing.T, b *Board) {
	t.Helper()
	guess(t, b, b.Game().Answer)
}

func TestOpenDailyMakesTheDaysPuzzleUnsaved(t *testing.T) {
	a, s, _ := newApp(t)
	b := openDaily(t, a, 5)
	g := b.Game()
	if g.ID != daily.ID(testDay, 5) || g.Daily != testDay.String() {
		t.Errorf("daily id %s day %q, want %s %s", g.ID, g.Daily, daily.ID(testDay, 5), testDay)
	}
	if ids := onDisk(t, s); len(ids) != 0 {
		t.Errorf("opening a daily saved %v", ids)
	}
	if b.View().CanRestart {
		t.Error("a daily offers a restart")
	}
}

// What is on disk is read first, so a started daily resumes with no seed:
// the source is never asked.
func TestOpenDailyResumesWithoutTheSeed(t *testing.T) {
	a, _, _ := newApp(t)
	guess(t, openDaily(t, a, 5), "about")
	src := &failingSource{}
	a.daily = src
	b := openDaily(t, a, 5)
	if len(b.Game().Guesses) != 1 || src.asked != 0 {
		t.Errorf("resume: %d guesses, source asked %d times; want 1 and 0", len(b.Game().Guesses), src.asked)
	}
	if _, err := a.OpenDaily(context.Background(), testDay, 6); CodeOf(err) != ErrNewPuzzle.Code {
		t.Errorf("an unplayed day with no seed: %v, want new_puzzle_failed", err)
	}
}

// A deleted daily stays deleted: its tombstone is the record of the day.
func TestOpenDailyRefusesASpentDay(t *testing.T) {
	a, s, _ := newApp(t)
	b := openDaily(t, a, 5)
	solve(t, b)
	if err := s.Delete(b.Game().ID); err != nil {
		t.Fatal(err)
	}
	if _, err := a.OpenDaily(context.Background(), testDay, 5); !errors.Is(err, ErrDailySpent) {
		t.Errorf("OpenDaily on a deleted daily: %v, want ErrDailySpent", err)
	}
}

func TestDailyViewStates(t *testing.T) {
	a, s, _ := newApp(t)
	guess(t, openDaily(t, a, 4), "abet")
	won := openDaily(t, a, 5)
	solve(t, won)
	six := openDaily(t, a, 6)
	solve(t, six)
	if err := s.Delete(six.Game().ID); err != nil {
		t.Fatal(err)
	}

	v, err := a.DailyView(testDay)
	if err != nil {
		t.Fatal(err)
	}
	if v.Day != "2026-11-01" || v.ResetsAtMS != testDay.ResetsAt().UnixMilli() {
		t.Errorf("day %q resets %d", v.Day, v.ResetsAtMS)
	}
	want := []string{DailyPlaying, DailyWon, DailySpent}
	for i, r := range v.Rows {
		if r.State != want[i] {
			t.Errorf("row %d (%d letters) is %q, want %q", i, r.Length, r.State, want[i])
		}
	}
	if r := v.Rows[2]; len(r.Marks) != 0 || r.Attempts != 0 {
		t.Errorf("a spent row kept its board: %+v", r)
	}
	if v.Trio.Done != 1 || v.Trio.Complete || v.Share != "" {
		t.Errorf("trio %+v share %q, want 1 done and no share", v.Trio, v.Share)
	}
}

func TestDailyViewNeverCarriesGuessesOrAnswers(t *testing.T) {
	a, _, _ := newApp(t)
	b := openDaily(t, a, 5)
	guess(t, b, "about")
	answer := b.Game().Answer
	v, err := a.DailyView(testDay)
	if err != nil {
		t.Fatal(err)
	}
	dump := strings.ToLower(strings.Join([]string{v.Share, v.Rows[1].Code, v.Rows[1].ID}, " "))
	for _, r := range v.Rows {
		for _, m := range r.Marks {
			dump += strings.Join(m, " ")
		}
	}
	for _, secret := range []string{"about", answer} {
		if strings.Contains(dump, secret) {
			t.Errorf("the daily view carries %q", secret)
		}
	}
}

func TestDailyViewTrioAndShare(t *testing.T) {
	a, _, c := newApp(t)
	for _, n := range []int{4, 5, 6} {
		b := openDaily(t, a, n)
		b.Type('a')
		c.advance(10 * time.Second)
		b.Trim(0)
		solve(t, b)
	}
	v, err := a.DailyView(testDay)
	if err != nil {
		t.Fatal(err)
	}
	if !v.Trio.Complete || v.Trio.Done != 3 || v.Trio.Guesses != 3 || v.Trio.ElapsedMS != 30000 {
		t.Errorf("trio = %+v", v.Trio)
	}
	want := "surmise daily 2026-11-01 · 3/3\n3 guesses · 30s\n" +
		"\n4 letters 1/5\n■■■■\n" +
		"\n5 letters 1/6\n■■■■■\n" +
		"\n6 letters 1/7\n■■■■■■\n" +
		"\n■ correct  □ present  · absent"
	if v.Share != want {
		t.Errorf("share =\n%s\nwant\n%s", v.Share, want)
	}
}

func TestResult(t *testing.T) {
	a, _, c := newApp(t)
	b := fixed(t, a)
	if _, err := a.Result(b); !errors.Is(err, ErrNotDone) {
		t.Errorf("Result in play: %v, want ErrNotDone", err)
	}
	b.Type('a')
	c.advance(65 * time.Second)
	b.Trim(0)
	guess(t, b, "react")
	guess(t, b, "crane")
	r, err := a.Result(b)
	if err != nil {
		t.Fatal(err)
	}
	want := "surmise #" + game.Code(fixedID) + " 2/6\n5 letters · 1:05\n□□■□·\n■■■■■\n■ correct  □ present  · absent"
	if r.Share != want || r.Attempts != "2/6" || r.Next != KindRandom {
		t.Errorf("result = %+v\nshare want\n%s", r, want)
	}
	if strings.Contains(r.Share, "crane") || strings.Contains(r.Share, "react") {
		t.Error("the share carries a word")
	}
	if r.Fastest || r.Fewest {
		t.Error("a first win is reported as a best")
	}
}

func TestResultBestNoteAndNext(t *testing.T) {
	a, _, c := newApp(t)
	slow := fixed(t, a)
	slow.Type('a')
	c.advance(time.Minute)
	slow.Trim(0)
	guess(t, slow, "about")
	guess(t, slow, "crane")

	quick := openDaily(t, a, 5)
	quick.Type('a')
	c.advance(time.Second)
	quick.Trim(0)
	solve(t, quick)
	r, err := a.Result(quick)
	if err != nil {
		t.Fatal(err)
	}
	if !r.Fastest || !r.Fewest {
		t.Errorf("second win, faster and fewer: fastest %v fewest %v", r.Fastest, r.Fewest)
	}
	if r.Next != KindDaily {
		t.Errorf("next after a daily = %q, want daily", r.Next)
	}
}

func TestFormatDuration(t *testing.T) {
	cases := map[time.Duration]string{
		0:                               "–",
		400 * time.Millisecond:          "0s",
		42 * time.Second:                "42s",
		3*time.Minute + 7*time.Second:   "3:07",
		time.Hour + 2*time.Minute + 3e9: "1:02:03",
	}
	for d, want := range cases {
		if got := FormatDuration(d); got != want {
			t.Errorf("FormatDuration(%v) = %q, want %q", d, got, want)
		}
	}
}
