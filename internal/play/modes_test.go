package play

import (
	"errors"
	"testing"
	"time"

	"github.com/nxck2005/surmise/internal/game"
	"github.com/nxck2005/surmise/internal/store"
)

// miss is a real five-letter guess that is not the board's answer.
func miss(b *Board) string {
	if b.Game().Answer == "about" {
		return "crane"
	}
	return "about"
}

func TestChallengeOpensResumesAndStaysSpent(t *testing.T) {
	a, s, _ := newApp(t)
	code, err := NewChallenge(5)
	if err != nil {
		t.Fatal(err)
	}
	b, err := a.OpenChallenge(code)
	if err != nil {
		t.Fatal(err)
	}
	if v := b.View(); v.Kind != KindChallenge || v.ChallengeCode == "" || v.CanRestart {
		t.Errorf("challenge view: kind %q code %q restart %v", v.Kind, v.ChallengeCode, v.CanRestart)
	}
	if ids := onDisk(t, s); len(ids) != 0 {
		t.Errorf("opening a challenge saved %v", ids)
	}
	guess(t, b, miss(b))

	again, err := a.OpenChallenge(code)
	if err != nil || len(again.Game().Guesses) != 1 {
		t.Fatalf("reopen: %v, %d guesses; want the saved board", err, len(again.Game().Guesses))
	}
	solve(t, again)
	if err := s.Delete(again.Game().ID); err != nil {
		t.Fatal(err)
	}
	if _, err := a.OpenChallenge(code); !errors.Is(err, ErrChallengeSpent) {
		t.Errorf("a deleted challenge: %v, want ErrChallengeSpent", err)
	}
}

func TestChallengeBadCode(t *testing.T) {
	a, _, _ := newApp(t)
	for _, code := range []string{"", "!!", "zzzzzzzzzzzz"} {
		_, err := a.OpenChallenge(code)
		if CodeOf(err) != ErrBadChallenge.Code {
			t.Errorf("OpenChallenge(%q): %v, want bad_challenge", code, err)
		}
	}
}

func TestCustom(t *testing.T) {
	a, s, _ := newApp(t)
	cases := []struct {
		word    string
		anyWord bool
		want    error
	}{
		{"cran", false, ErrTooShort},
		{"cranes", false, ErrTooShort},
		{"qzxvw", false, ErrNotAWord},
		{"qzxvw", true, nil},
		{"Crane", false, nil},
	}
	for _, c := range cases {
		if err := CheckCustom(c.word, 5, c.anyWord); !errors.Is(err, c.want) && !(err == nil && c.want == nil) {
			t.Errorf("CheckCustom(%q, any %v) = %v, want %v", c.word, c.anyWord, err, c.want)
		}
	}
	if _, err := a.StartCustom("qzxvw", 5, false); !errors.Is(err, ErrNotAWord) {
		t.Errorf("StartCustom of a non-word: %v", err)
	}
	b, err := a.StartCustom("crane", 5, false)
	if err != nil {
		t.Fatal(err)
	}
	if v := b.View(); v.Kind != KindCustom || v.Answer != "" {
		t.Errorf("custom view: kind %q answer %q", v.Kind, v.Answer)
	}
	if ids := onDisk(t, s); len(ids) != 0 {
		t.Errorf("a hand-over nobody played saved %v", ids)
	}
	guess(t, b, "crane")
	if b.Game().Status != game.Won {
		t.Errorf("status %v after the answer", b.Game().Status)
	}
}

func TestSprintRun(t *testing.T) {
	a, s, c := newApp(t)
	sp, err := a.StartSprint(5, 30*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if v := sp.View(); v.LeftMS != 30000 || v.Over || v.Board.Kind != KindSprint {
		t.Fatalf("start: left %d over %v kind %q", v.LeftMS, v.Over, v.Board.Kind)
	}

	// Board one: solved in two after ten seconds.
	b := sp.Board()
	for _, r := range miss(b) {
		sp.Type(r)
	}
	if err := sp.Submit(); err != nil {
		t.Fatal(err)
	}
	c.advance(10 * time.Second)
	for _, r := range b.Game().Answer {
		sp.Type(r)
	}
	if err := sp.Submit(); err != nil {
		t.Fatal(err)
	}
	sp.Submit() // a stray submit on the finished board records nothing twice
	if v := sp.View(); v.Dealt != 1 || v.Solved != 1 || v.AvgGuesses != 2 || v.Accuracy != 100 || v.SolveTimeMS != 10000 {
		t.Errorf("after one win: %+v", v)
	}

	// Board two: one miss, then the clock runs out.
	if err := sp.Next(); err != nil {
		t.Fatal(err)
	}
	two := sp.Board()
	if two == b {
		t.Fatal("Next kept the finished board")
	}
	for _, r := range miss(two) {
		sp.Type(r)
	}
	sp.Submit()
	c.advance(25 * time.Second)
	if err := sp.Type('a'); !errors.Is(err, ErrSprintOver) {
		t.Errorf("typing after the deadline: %v, want ErrSprintOver", err)
	}
	if err := sp.Next(); !errors.Is(err, ErrSprintOver) {
		t.Errorf("Next after the deadline: %v", err)
	}
	v := sp.View()
	if !v.Over || v.LeftMS != 0 || v.Dealt != 1 || v.Missed != 0 {
		t.Errorf("summary: %+v", v)
	}
	// The open board left like any other: saved with its time banked.
	if ids := onDisk(t, s); len(ids) != 2 {
		t.Errorf("on disk %v, want both boards", ids)
	}
	g, err := s.Load(two.Game().ID)
	if err != nil || g.Elapsed() == 0 || g.Status.Done() {
		t.Errorf("caught board: %v, elapsed %v, status %v", err, g.Elapsed(), g.Status)
	}
	if err := sp.End(); err != nil {
		t.Errorf("ending twice: %v", err)
	}
}

func TestSprintLossCountsAsMissed(t *testing.T) {
	a, _, _ := newApp(t)
	sp, err := a.StartSprint(5, 0) // not on the list: the default
	if err != nil {
		t.Fatal(err)
	}
	if sp.View().DurationMS != DefaultSprintDuration.Milliseconds() {
		t.Errorf("duration %d, want the default", sp.View().DurationMS)
	}
	b := sp.Board()
	for range b.Game().MaxAttempts {
		for _, r := range miss(b) {
			sp.Type(r)
		}
		sp.Submit()
	}
	if v := sp.View(); v.Dealt != 1 || v.Missed != 1 || v.Solved != 0 || v.AvgGuesses != 0 || v.Accuracy != 0 {
		t.Errorf("after one loss: %+v", v)
	}
}

func TestProfileFloorsPlaytime(t *testing.T) {
	a, _, c := newApp(t)
	b := fixed(t, a)
	b.Type('a')
	c.advance(40 * time.Second)
	b.Trim(0)
	guess(t, b, "crane")
	// An install from before the counter: the record holds time the
	// counter never saw.
	if err := a.UpdateSettings(func(s *store.Settings) { s.PlaytimeMS = 0 }); err != nil {
		t.Fatal(err)
	}
	p, err := a.Profile()
	if err != nil {
		t.Fatal(err)
	}
	if p.PlaytimeMS != 40000 || a.Settings().PlaytimeMS != 40000 {
		t.Errorf("playtime %d, saved %d; want the 40s floor written back", p.PlaytimeMS, a.Settings().PlaytimeMS)
	}
	if p.Played != 1 || p.Won != 1 || len(p.Distribution) != 1 || p.Distribution[0] != 1 {
		t.Errorf("profile %+v", p)
	}
	if len(p.Modes) != 1 || p.Modes[0].Length != 5 || p.Modes[0].Daily != nil {
		t.Errorf("modes %+v", p.Modes)
	}
}

func TestCalendarAndTrends(t *testing.T) {
	a, _, _ := newApp(t)
	guess(t, fixed(t, a), "crane")
	solve(t, openDaily(t, a, 4))
	cal, err := a.Calendar(time.UTC)
	if err != nil {
		t.Fatal(err)
	}
	if cal.Today != "2026-11-01" || len(cal.Days) != 1 || cal.Days[0].Games != 2 || cal.Days[0].Won != 2 {
		t.Errorf("calendar %+v", cal)
	}
	tr, err := a.Trends(5)
	if err != nil {
		t.Fatal(err)
	}
	if tr.AllTime.Games != 1 || tr.Fewest == nil || tr.Fewest.Attempts != 1 || len(tr.Openers) != 1 || tr.Openers[0].Word != "crane" {
		t.Errorf("trends %+v", tr)
	}
	if empty, _ := a.Trends(6); empty.Fastest != nil || empty.Openers == nil {
		t.Errorf("an unplayed length: %+v", empty)
	}
}
