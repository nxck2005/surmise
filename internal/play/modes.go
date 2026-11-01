package play

import (
	"errors"
	"strings"
	"time"

	"github.com/nxck2005/surmise/internal/challenge"
	"github.com/nxck2005/surmise/internal/game"
	"github.com/nxck2005/surmise/internal/store"
	"github.com/nxck2005/surmise/internal/words"
)

// --- challenge ---

// NewChallenge makes a fresh shareable code for a length. Nothing is saved:
// the board appears only when somebody opens the code.
func NewChallenge(length int) (string, error) {
	c, err := challenge.Generate(length)
	if err != nil {
		return "", &Error{Code: ErrBadChallenge.Code, Err: err}
	}
	return c.String(), nil
}

// OpenChallenge opens the board a code stands for. It keeps the daily's
// order: a saved board wins, a deleted one (a tombstone) is refused rather
// than rebuilt, and only a code nobody here has played makes a new board.
// A code that does not parse is ErrBadChallenge, its detail the reason.
func (a *App) OpenChallenge(code string) (*Board, error) {
	c, err := challenge.Parse(code)
	if err != nil {
		return nil, &Error{Code: ErrBadChallenge.Code, Err: errors.New(strings.TrimPrefix(err.Error(), "challenge: "))}
	}
	id := c.ID()
	switch g, err := a.store.Load(id); {
	case err == nil:
		return a.Open(g, true), nil
	case !errors.Is(err, store.ErrNotFound):
		return nil, err
	}
	if spent, err := a.exists(id); err != nil {
		return nil, err
	} else if spent {
		return nil, ErrChallengeSpent
	}
	g, err := c.NewGame()
	if err != nil {
		return nil, &Error{Code: ErrBadChallenge.Code, Err: err}
	}
	return a.Open(g, false), nil
}

// --- custom ---

// CheckCustom says why a hand-set word cannot be used yet, or nil. anyWord
// lets a word outside the guess list be the answer; the guesses are still
// held to the list (game.Guess). The refusal never quotes the word: the
// screen is looked at by whoever is about to guess.
func CheckCustom(word string, length int, anyWord bool) error {
	w := words.Normalize(word)
	switch {
	case len(w) != length:
		return ErrTooShort
	case !anyWord && !words.IsValidGuess(length, w):
		return ErrNotAWord
	}
	return nil
}

// StartCustom hands the phone over: a board whose answer someone just
// typed. The caller must forget the word as this returns, before the board
// is shown, so no later frame can draw it. The board is not saved until its
// first guess, so a hand-over nobody plays leaves nothing on disk.
func (a *App) StartCustom(word string, length int, anyWord bool) (*Board, error) {
	if err := CheckCustom(word, length, anyWord); err != nil {
		return nil, err
	}
	g, err := game.NewCustom(word, length)
	if err != nil {
		return nil, &Error{Code: ErrNewPuzzle.Code, Err: err}
	}
	return a.Open(g, false), nil
}

// --- sprint ---

// SprintDurations is what a run's clock can be set to, shortest first.
var SprintDurations = []time.Duration{
	10 * time.Second, 15 * time.Second, 30 * time.Second,
	time.Minute, 2 * time.Minute, 5 * time.Minute, 10 * time.Minute,
}

// DefaultSprintDuration is the offered default, one of SprintDurations.
const DefaultSprintDuration = 5 * time.Minute

// Sprint is one timed run: solve as many boards as the clock allows. Every
// board it deals is a plain random puzzle that saves and counts like any
// other; the run owns only the deadline and the tally. Nothing about the run
// itself is saved.
//
// Expiry is checked on every input, so a keystroke after the deadline ends
// the run instead of reaching a board that is no longer part of it.
type Sprint struct {
	app      *App
	length   int
	duration time.Duration
	deadline time.Time
	board    *Board
	over     bool

	dealt, solved, missed int
	attempts              int           // guesses spent, wins only
	solveTime             time.Duration // across every finished board
	lastID                string        // each board is recorded once
}

// StartSprint deals the first board and starts the clock. The clock runs
// from here, not from the first letter: the run is short, and staring at
// board one is time it buys nothing else with.
func (a *App) StartSprint(length int, d time.Duration) (*Sprint, error) {
	if !validSprint(d) {
		d = DefaultSprintDuration
	}
	b, err := a.NewRandom(length)
	if err != nil {
		return nil, err
	}
	b.SetSprint(true)
	return &Sprint{app: a, length: length, duration: d, deadline: a.clock.Now().Add(d), board: b}, nil
}

func validSprint(d time.Duration) bool {
	for _, want := range SprintDurations {
		if want == d {
			return true
		}
	}
	return false
}

// Board is the board the run is on.
func (s *Sprint) Board() *Board { return s.board }

// Over reports whether the run has ended. A run whose deadline has passed is
// ended by this call, which banks and saves its open board.
func (s *Sprint) Over() bool {
	if !s.over && !s.app.clock.Now().Before(s.deadline) {
		s.End()
	}
	return s.over
}

// Type, Delete and Trim are the board's, refused with ErrSprintOver once the
// clock has run out.
func (s *Sprint) Type(c rune) error {
	if s.Over() {
		return ErrSprintOver
	}
	s.board.Type(c)
	return nil
}

func (s *Sprint) Delete() error {
	if s.Over() {
		return ErrSprintOver
	}
	s.board.Delete()
	return nil
}

func (s *Sprint) Trim(i int) error {
	if s.Over() {
		return ErrSprintOver
	}
	s.board.Trim(i)
	return nil
}

// Submit plays the typed row. A finishing guess is recorded in the tally at
// once, after the board banked and saved it, so the summary can never
// disagree with what is on disk. The next board is dealt by Next, which the
// front end calls when the reveal is over.
func (s *Sprint) Submit() error {
	if s.Over() {
		return ErrSprintOver
	}
	err := s.board.Submit()
	if s.board.g.Status.Done() {
		s.record(s.board.g)
	}
	return err
}

// record banks one finished board into the tally, once per board however
// often it is asked.
func (s *Sprint) record(g *game.Game) {
	if g.ID == s.lastID {
		return
	}
	s.lastID = g.ID
	s.dealt++
	if g.Status == game.Won {
		s.solved++
		s.attempts += g.Attempts()
	} else {
		s.missed++
	}
	s.solveTime += g.Elapsed()
}

// Next deals a fresh board in place of the finished one. On a run that is
// over it does nothing and reports ErrSprintOver; on a board still in play it
// does nothing. A run that cannot deal ends.
func (s *Sprint) Next() error {
	if s.Over() {
		return ErrSprintOver
	}
	if !s.board.g.Status.Done() {
		return nil
	}
	b, err := s.app.NewRandom(s.length)
	if err != nil {
		s.End()
		return err
	}
	b.SetSprint(true)
	s.board = b
	return nil
}

// End stops the run. The open board leaves the way any abandoned board does,
// banked and saved, so a puzzle the clock caught mid-flight stays resumable
// from the history. Ending twice is ending once.
func (s *Sprint) End() error {
	if s.over {
		return nil
	}
	s.over = true
	return s.board.Leave()
}

// SprintView is the run now: its board, the clock and the tally.
type SprintView struct {
	Board      BoardView `json:"board"`
	Length     int       `json:"length"`
	DurationMS int64     `json:"durationMs"`
	// LeftMS is the time on the clock, never below zero.
	LeftMS int64 `json:"leftMs"`
	Over   bool  `json:"over"`

	Dealt  int `json:"dealt"`
	Solved int `json:"solved"`
	Missed int `json:"missed"`
	// AvgGuesses is over wins only; zero with no win.
	AvgGuesses float64 `json:"avgGuesses"`
	// Accuracy is solved out of dealt, as a whole percent.
	Accuracy    int   `json:"accuracy"`
	SolveTimeMS int64 `json:"solveTimeMs"`
}

// View is the run's snapshot. It checks the clock first, so a view asked for
// after the deadline is the summary.
func (s *Sprint) View() SprintView {
	over := s.Over()
	v := SprintView{
		Board:       s.board.View(),
		Length:      s.length,
		DurationMS:  s.duration.Milliseconds(),
		LeftMS:      max(s.deadline.Sub(s.app.clock.Now()), 0).Milliseconds(),
		Over:        over,
		Dealt:       s.dealt,
		Solved:      s.solved,
		Missed:      s.missed,
		SolveTimeMS: s.solveTime.Milliseconds(),
	}
	if s.solved > 0 {
		v.AvgGuesses = float64(s.attempts) / float64(s.solved)
	}
	if s.dealt > 0 {
		v.Accuracy = s.solved * 100 / s.dealt
	}
	return v
}
