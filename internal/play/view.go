package play

import (
	"github.com/nxck2005/surmise/internal/game"
)

// BoardView is one snapshot of a board, everything a front end needs to draw
// it and nothing it could use to decide a rule itself. It is plain data with
// JSON tags because the mobile engine returns it as is.
//
// The answer is only in it once the puzzle is over.
type BoardView struct {
	ID          string `json:"id"`
	Code        string `json:"code"`
	Length      int    `json:"length"`
	MaxAttempts int    `json:"maxAttempts"`

	Rows   []Row  `json:"rows"`
	Typing string `json:"typing"`
	// Keys is the best mark seen for each guessed letter; a letter not in it
	// is untouched.
	Keys map[string]string `json:"keys"`

	Status string `json:"status"`
	Answer string `json:"answer,omitempty"`

	// ElapsedMS is the play time at the moment of the snapshot. Running says
	// whether it is still going, so the front end can tick it on its own
	// between snapshots.
	ElapsedMS int64 `json:"elapsedMs"`
	Running   bool  `json:"running"`

	Kind          string `json:"kind"`
	Daily         string `json:"daily,omitempty"`
	ChallengeCode string `json:"challengeCode,omitempty"`

	// CanRestart says whether Restart would be refused. The front end hides
	// the action rather than offering one that fails.
	CanRestart bool `json:"canRestart"`
}

// Row is one played guess and its marks, one per letter.
type Row struct {
	Guess string   `json:"guess"`
	Marks []string `json:"marks"`
}

// The kinds of board, as BoardView.Kind and the history labels name them.
const (
	KindRandom    = "random"
	KindDaily     = "daily"
	KindCustom    = "custom"
	KindChallenge = "challenge"
	KindRoom      = "room"
	KindSprint    = "sprint"
)

// kindOf names where a puzzle came from, in the order the TUI's history list
// checks (internal/ui/listscreen.go), so the two never label a record apart.
func kindOf(g *game.Game) string {
	switch {
	case g.Daily != "":
		return KindDaily
	case g.Custom:
		return KindCustom
	case g.Challenge != nil:
		return KindChallenge
	case g.Room != nil:
		return KindRoom
	}
	return KindRandom
}

// Keys is the on-screen keyboard's state for a puzzle: the best mark seen per
// guessed letter, so a letter never appears to downgrade. It is rebuilt from
// the guesses every time, never cached, so a fresh board cannot inherit the
// marks of the one it replaced.
func Keys(g *game.Game) map[string]string {
	states := g.LetterStates()
	out := make(map[string]string, len(states))
	for c, m := range states {
		out[string(c)] = m.String()
	}
	return out
}

// View is the board's snapshot now.
func (b *Board) View() BoardView {
	g := b.g
	v := BoardView{
		ID:          g.ID,
		Code:        game.Code(g.ID),
		Length:      g.Length,
		MaxAttempts: g.MaxAttempts,
		Rows:        make([]Row, len(g.Guesses)),
		Typing:      b.typing,
		Keys:        Keys(g),
		Status:      string(g.Status),
		ElapsedMS:   b.Elapsed().Milliseconds(),
		Running:     !b.sessionStart.IsZero() && !g.Status.Done(),
		Kind:        kindOf(g),
		Daily:       g.Daily,
		CanRestart:  b.restartRefusal() == nil,
	}
	if b.sprint && v.Kind == KindRandom {
		v.Kind = KindSprint
	}
	if g.Challenge != nil {
		v.ChallengeCode = g.Challenge.Code
	}
	for i, guess := range g.Guesses {
		marks := make([]string, len(g.Marks[i]))
		for j, m := range g.Marks[i] {
			marks[j] = m.String()
		}
		v.Rows[i] = Row{Guess: guess, Marks: marks}
	}
	if g.Status.Done() {
		v.Answer = g.Answer
	}
	return v
}
