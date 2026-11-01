package ui

import (
	"testing"
	"time"

	"github.com/nxck2005/surmise/internal/game"
	"github.com/nxck2005/surmise/internal/play"
)

// internal/play carries a copy of the share texts for the phone (D9 moves the
// terminal onto it later). Until then the two copies must agree to the byte:
// a result pasted from the phone has to read like one pasted from here.
func TestShareTextsMatchPlay(t *testing.T) {
	mk := func(id, answer string, edit func(*game.Game), guesses ...string) *game.Game {
		g, err := game.NewFrom(id, answer, len(answer))
		if err != nil {
			t.Fatal(err)
		}
		edit(g)
		for _, w := range guesses {
			if err := g.Guess(w); err != nil {
				t.Fatal(err)
			}
		}
		g.ElapsedMS = 187_000
		return g
	}
	const id = "3f2a7c1e-8b4d-4c6a-9e2f-1d5b8a7c3e90"
	games := map[string]*game.Game{
		"random won": mk(id, "crane", func(*game.Game) {}, "react", "crane"),
		"random lost": mk(id, "crane", func(*game.Game) {},
			"about", "about", "about", "about", "about", "about"),
		"daily":     mk(id, "crane", func(g *game.Game) { g.Daily = "2026-11-01" }, "crane"),
		"custom":    mk(id, "crane", func(g *game.Game) { g.Custom = true }, "slate", "crane"),
		"challenge": mk(id, "crane", func(g *game.Game) { g.Challenge = &game.ChallengeInfo{Code: "AB12CD"} }, "crane"),
	}
	for name, g := range games {
		if got, want := play.ShareResult(g), shareResult(g); got != want {
			t.Errorf("%s: play.ShareResult =\n%s\nui shareResult =\n%s", name, got, want)
		}
	}

	rows := []dailyRow{
		{length: 4, status: game.Won, attempts: 3, maxAttempts: 5, elapsed: 40 * time.Second,
			marks: [][]game.Mark{{0, 1, 0, 2}, {2, 2, 1, 0}, {2, 2, 2, 2}}},
		{length: 5, status: game.Lost, attempts: 6, maxAttempts: 6, elapsed: 5 * time.Minute,
			marks: [][]game.Mark{{0, 0, 0, 0, 0}, {1, 1, 0, 0, 0}, {2, 0, 0, 0, 0}, {2, 2, 0, 0, 0}, {2, 2, 2, 0, 0}, {2, 2, 2, 2, 0}}},
		{length: 6, status: game.Won, attempts: 2, maxAttempts: 7, elapsed: 2 * time.Hour,
			marks: [][]game.Mark{{1, 0, 0, 0, 0, 2}, {2, 2, 2, 2, 2, 2}}},
	}
	prow := make([]play.DailyRow, len(rows))
	for i, r := range rows {
		state := play.DailyWon
		if r.status == game.Lost {
			state = play.DailyLost
		}
		marks := make([][]string, len(r.marks))
		for j, m := range r.marks {
			for _, x := range m {
				marks[j] = append(marks[j], x.String())
			}
		}
		prow[i] = play.DailyRow{Length: r.length, State: state, Attempts: r.attempts,
			MaxAttempts: r.maxAttempts, ElapsedMS: r.elapsed.Milliseconds(), Marks: marks}
	}
	if got, want := play.ShareTrio("2026-11-01", prow), shareTrio("2026-11-01", rows); got != want {
		t.Errorf("play.ShareTrio =\n%s\nui shareTrio =\n%s", got, want)
	}

	for _, d := range []time.Duration{0, -time.Second, 499 * time.Millisecond, 59 * time.Second,
		61 * time.Second, time.Hour, 25*time.Hour + 61*time.Second} {
		if got, want := play.FormatDuration(d), formatDuration(d); got != want {
			t.Errorf("FormatDuration(%v): play %q, ui %q", d, got, want)
		}
	}
}
