// Package game holds the rules of Wortle: puzzle state, guess validation and
// scoring. It has no knowledge of the terminal or of persistence, so the same
// state can be driven by the TUI today and by a server later.
package game

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/nxck2005/wortle/internal/words"
)

// Status is the lifecycle of a puzzle.
type Status string

const (
	InProgress Status = "in_progress"
	Won        Status = "won"
	Lost       Status = "lost"
)

// Done reports whether the puzzle is finished, either way.
func (s Status) Done() bool { return s == Won || s == Lost }

// Errors returned by Guess. The UI matches on these to choose a message.
var (
	ErrFinished    = errors.New("puzzle is already finished")
	ErrWrongLength = errors.New("wrong length")
	ErrNotAWord    = errors.New("not in word list")
)

// Game is a single puzzle. It is the unit of persistence, so every field
// needed to resume play is exported and JSON-tagged.
type Game struct {
	ID     string `json:"id"`     // stable identity, unique across devices
	Number int    `json:"number"` // display number, "Wortle #42"
	Length int    `json:"length"` // 4, 5 or 6

	Answer  string   `json:"answer"`
	Guesses []string `json:"guesses"`
	Marks   [][]Mark `json:"marks"` // parallel to Guesses

	MaxAttempts int    `json:"maxAttempts"`
	Status      Status `json:"status"`

	StartedAt time.Time `json:"startedAt"`
	UpdatedAt time.Time `json:"updatedAt"`
	// ElapsedMS accumulates play time across sessions, so a puzzle resumed
	// tomorrow does not report a 20-hour solve.
	ElapsedMS int64 `json:"elapsedMs"`
}

// attemptsFor returns how many guesses a word of length n allows. Classic
// Wordle gives six for five letters; the shorter and longer modes scale with
// it so difficulty stays roughly even.
func attemptsFor(n int) int { return n + 1 }

// New starts a puzzle of the given length with a randomly chosen answer.
// number is the display number, supplied by the caller since only the store
// knows how many puzzles came before.
func New(length, number int) (*Game, error) {
	answer, err := words.Random(length)
	if err != nil {
		return nil, err
	}
	id, err := newID()
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	return &Game{
		ID:          id,
		Number:      number,
		Length:      length,
		Answer:      answer,
		MaxAttempts: attemptsFor(length),
		Status:      InProgress,
		StartedAt:   now,
		UpdatedAt:   now,
	}, nil
}

func newID() (string, error) {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("game: generate id: %w", err)
	}
	return hex.EncodeToString(b[:]), nil
}

// Guess plays a word. It returns ErrNotAWord or ErrWrongLength for input the
// player should correct, leaving the game untouched, so a rejected guess never
// costs an attempt.
func (g *Game) Guess(word string) error {
	if g.Status.Done() {
		return ErrFinished
	}

	w := words.Normalize(word)
	if len(w) != g.Length {
		return ErrWrongLength
	}
	if !words.IsValidGuess(g.Length, w) {
		return ErrNotAWord
	}

	g.Guesses = append(g.Guesses, w)
	g.Marks = append(g.Marks, Score(w, g.Answer))
	g.UpdatedAt = time.Now().UTC()

	switch {
	case w == g.Answer:
		g.Status = Won
	case len(g.Guesses) >= g.MaxAttempts:
		g.Status = Lost
	}
	return nil
}

// Attempts is the number of guesses played so far.
func (g *Game) Attempts() int { return len(g.Guesses) }

// Remaining is the number of guesses left.
func (g *Game) Remaining() int { return max(g.MaxAttempts-len(g.Guesses), 0) }

// Elapsed is total play time, including the session currently in progress if
// one has been started with AddElapsed.
func (g *Game) Elapsed() time.Duration {
	return time.Duration(g.ElapsedMS) * time.Millisecond
}

// AddElapsed records time spent on this puzzle. The UI calls it when leaving
// the board so that idle time between sessions is not counted.
func (g *Game) AddElapsed(d time.Duration) {
	if d <= 0 {
		return
	}
	g.ElapsedMS += d.Milliseconds()
	g.UpdatedAt = time.Now().UTC()
}

// LetterStates returns the best mark seen so far for each letter guessed,
// which is what the on-screen keyboard displays. Correct beats Present beats
// Absent, so a letter never appears to downgrade as the game goes on.
func (g *Game) LetterStates() map[byte]Mark {
	states := make(map[byte]Mark, 26)
	for i, guess := range g.Guesses {
		for j := range guess {
			c := guess[j]
			if prev, ok := states[c]; !ok || g.Marks[i][j] > prev {
				states[c] = g.Marks[i][j]
			}
		}
	}
	return states
}

// Validate checks that a decoded Game is internally consistent. The store uses
// it to reject corrupt or hand-edited save files rather than crashing later.
func (g *Game) Validate() error {
	switch {
	case g.ID == "":
		return errors.New("game: missing id")
	case !words.SupportedLength(g.Length):
		return fmt.Errorf("game: unsupported length %d", g.Length)
	case len(g.Answer) != g.Length:
		return fmt.Errorf("game: answer %q does not match length %d", g.Answer, g.Length)
	case len(g.Guesses) != len(g.Marks):
		return fmt.Errorf("game: %d guesses but %d marks", len(g.Guesses), len(g.Marks))
	case g.MaxAttempts <= 0:
		return errors.New("game: maxAttempts must be positive")
	case len(g.Guesses) > g.MaxAttempts:
		return fmt.Errorf("game: %d guesses exceeds max %d", len(g.Guesses), g.MaxAttempts)
	}
	for i, guess := range g.Guesses {
		if len(guess) != g.Length || len(g.Marks[i]) != g.Length {
			return fmt.Errorf("game: guess %d has wrong length", i)
		}
	}
	return nil
}
