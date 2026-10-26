package ui

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/nxck2005/surmise/internal/challenge"
	"github.com/nxck2005/surmise/internal/daily"
	"github.com/nxck2005/surmise/internal/game"
	"github.com/nxck2005/surmise/internal/store"
	"github.com/nxck2005/surmise/internal/words"
)

// savedGame saves a finished or unfinished puzzle of the given length with
// the given status, started and last updated at the given instants, and
// returns it. Won takes one guess; lost takes every guess; in play takes one
// wrong guess. The answer is the first in the list, the wrong guess the second.
func savedGame(t *testing.T, s store.Store, id string, length int, status game.Status, started, updated time.Time) *game.Game {
	t.Helper()
	answer, err := words.AnswerAt(length, 0)
	if err != nil {
		t.Fatal(err)
	}
	wrong, err := words.AnswerAt(length, 1)
	if err != nil {
		t.Fatal(err)
	}
	g, err := game.NewFrom(id, answer, length)
	if err != nil {
		t.Fatal(err)
	}
	playOut(t, g, status, wrong)
	g.StartedAt, g.UpdatedAt = started.UTC(), updated.UTC()
	if err := s.Save(g); err != nil {
		t.Fatal(err)
	}
	return g
}

// playOut plays g to the status asked for, with wrong as every losing guess.
func playOut(t *testing.T, g *game.Game, status game.Status, wrong string) {
	t.Helper()
	switch status {
	case game.Won:
		if err := g.Guess(g.Answer); err != nil {
			t.Fatal(err)
		}
	case game.Lost:
		for g.Status == game.InProgress {
			if err := g.Guess(wrong); err != nil {
				t.Fatal(err)
			}
		}
	default:
		if err := g.Guess(wrong); err != nil {
			t.Fatal(err)
		}
	}
}

// fixedID is a valid 16-hex puzzle id built from n.
func fixedID(n int) string { return fmt.Sprintf("%016x", n) }

// saveOneOfEachKind saves a won 5-letter puzzle of every kind: random, daily
// (2026-10-01), challenge, custom and room round. It returns their ids by kind.
func saveOneOfEachKind(t *testing.T, s store.Store) map[puzzleKind]string {
	t.Helper()
	at := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	ids := map[puzzleKind]string{}

	ids[kindRandom] = savedGame(t, s, fixedID(1), 5, game.Won, at, at).ID

	day, err := daily.ParseDay("2026-10-01")
	if err != nil {
		t.Fatal(err)
	}
	d, err := daily.New(context.Background(), daily.Local(), day, 5)
	if err != nil {
		t.Fatal(err)
	}
	playOut(t, d, game.Won, "")
	if err := s.Save(d); err != nil {
		t.Fatal(err)
	}
	ids[kindDaily] = d.ID

	c, err := challenge.Parse("4500-820C-20A1-G73J")
	if err != nil {
		t.Fatal(err)
	}
	cg, err := c.NewGame()
	if err != nil {
		t.Fatal(err)
	}
	playOut(t, cg, game.Won, "")
	if err := s.Save(cg); err != nil {
		t.Fatal(err)
	}
	ids[kindChallenge] = cg.ID

	cu, err := game.NewCustom("crane", 5)
	if err != nil {
		t.Fatal(err)
	}
	playOut(t, cu, game.Won, "")
	if err := s.Save(cu); err != nil {
		t.Fatal(err)
	}
	ids[kindCustom] = cu.ID

	r, err := game.NewRoomRound(game.RoomInfo{
		Code: "K7M2QX", Mode: "live", Round: 1, Seat: 1,
		Ranking: []game.RoomRank{{Seat: 1, Name: "nick", Solved: true, Guesses: 1,
			ElapsedMS: 81_000, Rows: [][]game.Mark{game.Score("crane", "crane")}}},
	}, "crane", []string{"crane"}, 81_000, at)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Save(r); err != nil {
		t.Fatal(err)
	}
	ids[kindRoom] = r.ID
	return ids
}
