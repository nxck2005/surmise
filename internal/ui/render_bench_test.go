package ui

import (
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/nxck2005/surmise/internal/game"
	"github.com/nxck2005/surmise/internal/store"
)

// Benchmarks for the frame pipeline. They are not thresholds — CI never fails on
// wall-clock numbers — but the render code is the heaviest thing the app does
// per message, so it is worth being able to say what a frame costs and what a
// change did to it. Everything here renders a fixed screen at a fixed size with
// motion off, so two runs of the same build agree.

// benchGame builds the puzzle the board benchmarks draw: two guesses on it and
// a partly typed third, which puts marks, letter states and a caret on screen.
func benchGame(b *testing.B) *game.Game {
	b.Helper()
	g, err := game.New(5)
	if err != nil {
		b.Fatal(err)
	}
	g.Answer = "crane"
	for _, w := range []string{"about", "adieu"} {
		if err := g.Guess(w); err != nil {
			b.Fatal(err)
		}
	}
	return g
}

// benchBoardModel is a model sitting on that board, with the clock held still so
// two frames are comparable.
func benchBoardModel(b *testing.B) *Model {
	b.Helper()
	s, err := store.NewJSON(b.TempDir())
	if err != nil {
		b.Fatal(err)
	}
	m := New(s, nil, Options{Motion: motionOffName})
	m.Update(tea.WindowSizeMsg{Width: testWidth, Height: testHeight})
	m.openGame(benchGame(b), false)
	m.game.typing = "s"
	m.game.sessionStart = time.Time{}
	m.Update(tea.WindowSizeMsg{Width: testWidth, Height: testHeight})
	m.View()
	return m
}

func BenchmarkBoardFrame(b *testing.B) {
	m := benchBoardModel(b)
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		m.View()
	}
}

// BenchmarkNoopMouseMotion is the message path a moving pointer walks: an Update
// that changes nothing, then the View the framework calls after every message.
// The pointer is parked in the middle of a keycap, so every iteration but the
// first is the case the reuse handshake exists for.
func BenchmarkNoopMouseMotion(b *testing.B) {
	m := benchBoardModel(b)
	r, ok := m.hits.find(action{kind: actLetter, letter: 'q'})
	if !ok {
		b.Fatal("no keycap on the frame")
	}
	x, y := r.x+r.w/2, r.y+r.h/2
	m.Update(tea.MouseMotionMsg{X: x, Y: y}) // the motion that parks the pointer
	m.View()

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		m.Update(tea.MouseMotionMsg{X: x, Y: y})
		m.View()
	}
}

// BenchmarkMenuFrame renders the menu, which is the screen a player sits on
// while a backup or a restore is running.
func BenchmarkMenuFrame(b *testing.B) {
	s, err := store.NewJSON(b.TempDir())
	if err != nil {
		b.Fatal(err)
	}
	m := New(s, nil, Options{Motion: motionOffName})
	m.screen = screenMenu
	m.Update(tea.WindowSizeMsg{Width: testWidth, Height: testHeight})
	m.View()
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		m.View()
	}
}

// BenchmarkProfileFrame renders the heaviest body: a full histogram, the
// per-mode tables and the daily table over a large history.
func BenchmarkProfileFrame(b *testing.B) {
	s, err := store.NewJSON(b.TempDir())
	if err != nil {
		b.Fatal(err)
	}
	m := New(s, nil, Options{Motion: motionOffName})
	m.Update(tea.WindowSizeMsg{Width: testWidth, Height: testHeight})
	m.profile.reload(benchHistory(b, 500), m.day, "", 0)
	m.screen = screenProfile
	m.View()
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		m.View()
	}
}

// benchHistory builds n finished puzzles spread across the attempt counts, which
// is what gives the histogram every bar it can draw.
func benchHistory(b *testing.B, n int) []*game.Game {
	b.Helper()
	words := []string{"about", "adieu", "slate"}
	out := make([]*game.Game, 0, n)
	for i := range n {
		g, err := game.New(5)
		if err != nil {
			b.Fatal(err)
		}
		g.Answer = "crane"
		for j := range i % (len(words) + 1) {
			if err := g.Guess(words[j%len(words)]); err != nil {
				b.Fatal(err)
			}
		}
		if err := g.Guess("crane"); err != nil {
			b.Fatal(err)
		}
		out = append(out, g)
	}
	return out
}
