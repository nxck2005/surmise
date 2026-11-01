package play

import (
	"github.com/nxck2005/surmise/internal/game"
	"github.com/nxck2005/surmise/internal/stats"
)

// ResultView is the debrief for a finished board.
type ResultView struct {
	Board BoardView `json:"board"`
	// Attempts is the score as the share text writes it: 3/6, or X/6.
	Attempts string `json:"attempts"`
	Share    string `json:"share"`
	// Fastest and Fewest say the win is its length's fastest or fewest-guess
	// win (stats.IsBest, so never before the length's second win). The words
	// for them are the front end's.
	Fastest bool `json:"fastest"`
	Fewest  bool `json:"fewest"`
	// Next is where "next" goes: a fresh board for a random one, the daily
	// screen for a daily, the word screen for a custom board, social play for
	// a challenge. Named by kind, as BoardView.Kind names it.
	Next string `json:"next"`
}

// Result is the debrief for the board, refused with ErrNotDone while it is
// in play. It reads the shared history, which already holds the board's
// puzzle: Submit saved it before anything could ask.
//
// A history that cannot be read loses the best note and nothing else.
func (a *App) Result(b *Board) (ResultView, error) {
	g := b.g
	if !g.Status.Done() {
		return ResultView{}, ErrNotDone
	}
	v := ResultView{
		Board:    b.View(),
		Attempts: attemptsText(g.Status == game.Lost, g.Attempts(), g.MaxAttempts),
		Share:    ShareResult(g),
		Next:     KindRandom,
	}
	switch v.Board.Kind {
	case KindDaily, KindCustom, KindChallenge, KindSprint:
		v.Next = v.Board.Kind
	}
	if games, err := a.History(); err == nil {
		v.Fastest, v.Fewest = stats.IsBest(games, g)
	}
	return v, nil
}
