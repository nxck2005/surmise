// Package game holds the rules of Surmise: puzzle state, guess validation and
// scoring. It has no knowledge of the terminal or of persistence, so the same
// state can be driven by the TUI today and by a server later.
package game

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/nxck2005/surmise/internal/words"
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

	// Daily is the UTC date this puzzle is the daily for, "2006-01-02", and is
	// empty for an ordinary puzzle. Like Deleted it is omitempty, so a casual
	// puzzle's file is unchanged and a save written before dailies existed
	// decodes to "".
	Daily string `json:"daily,omitempty"`

	// Deleted marks a tombstone: the player removed this puzzle, and all that
	// survives is the fact that a finished puzzle sat here (see Tombstone).
	// omitempty keeps it out of every ordinary save, so a real puzzle's file is
	// unchanged and an older save decodes to false.
	Deleted bool `json:"deleted,omitempty"`

	// Custom marks a puzzle whose answer a person chose rather than one drawn
	// from the answer list: a board set by hand, by whoever handed the terminal
	// over. Like Daily and Deleted it is omitempty, so an ordinary puzzle's file
	// is unchanged and a save written before custom puzzles existed decodes to
	// false.
	//
	// It is what CountsForStats reads, and it is why a custom answer may sit
	// outside the guess list — see Guess.
	Custom bool `json:"custom,omitempty"`

	// Schema is the version of the save format this record was written with.
	// The store stamps it on every write and refuses to read a number it does
	// not know, so a breaking change to the format can never silently misread
	// an older or newer file. Zero means "written before the tag existed" and
	// stays valid forever; see docs/UPGRADING.md for the compatibility rule.
	Schema int `json:"schema"`
}

// CountsForStats reports whether a puzzle belongs in the player's figures.
//
// A custom answer was chosen by a person rather than drawn, so its
// attempts, its time and its win or loss say something about the person who set
// it as much as the person who played it. It is kept and listed like any other
// puzzle; it simply does not move the numbers.
//
// This is the only place that judgement is made: including custom puzzles again
// is deleting the one condition below.
func (g *Game) CountsForStats() bool { return !g.Custom }

// Tombstone returns what is left of a puzzle once it is deleted: enough to
// place it in the sequence of play, and nothing about how it was played. The
// answer, the guesses and the marks are dropped, so deleting really does
// destroy the record.
//
// Streaks are runs of wins over finished puzzles in time order, so removing a
// loss outright would silently merge the runs either side of it and inflate the
// longest streak. Keeping the status and the timestamp is what lets
// stats.Compute know a loss happened here without knowing anything else.
//
// Daily is kept for the same reason one step further out: the daily streak is
// indexed by calendar date rather than by completion order, so a deleted day
// that forgot which day it was would leave a hole indistinguishable from a day
// never played — and the runs either side of a deleted daily loss would merge
// exactly as they used to for an ordinary one. It says which day, never how it
// went beyond the status.
//
// Custom is kept for the mirror of that reason. A custom puzzle is left out of
// every figure, so a tombstone that forgot it was custom would read as an
// ordinary loss and break a streak the live puzzle never touched — deleting a
// puzzle that never counted would lower the longest streak.
func (g *Game) Tombstone() *Game {
	return &Game{
		ID:        g.ID,
		Length:    g.Length,
		Status:    g.Status,
		UpdatedAt: g.UpdatedAt,
		Daily:     g.Daily,
		Custom:    g.Custom,
		Deleted:   true,
	}
}

// attemptsFor returns how many guesses a word of length n allows. The genre
// settled on six for five letters; the shorter and longer modes scale with it
// so difficulty stays roughly even.
func attemptsFor(n int) int { return n + 1 }

// New starts a puzzle of the given length with a randomly chosen answer.
func New(length int) (*Game, error) {
	answer, err := words.Random(length)
	if err != nil {
		return nil, err
	}
	id, err := newID()
	if err != nil {
		return nil, err
	}
	return NewFrom(id, answer, length)
}

// NewFrom starts a puzzle from an id and an answer the caller has already
// decided on, for deterministic sources — today, the daily, whose id and answer
// are both derived from its date.
//
// It validates rather than draws: everything New would have chosen is supplied,
// so the only thing left to do is refuse an id, answer or length that could not
// have come from a real puzzle.
func NewFrom(id, answer string, length int) (*Game, error) {
	now := time.Now().UTC()
	g := &Game{
		ID:          id,
		Length:      length,
		Answer:      words.Normalize(answer),
		MaxAttempts: attemptsFor(length),
		Status:      InProgress,
		StartedAt:   now,
		UpdatedAt:   now,
	}
	if err := g.Validate(); err != nil {
		return nil, err
	}
	// Validate is about internal consistency and knows nothing of the word
	// lists; an answer nobody could ever type would make the puzzle unwinnable.
	if !words.IsValidGuess(length, g.Answer) {
		return nil, fmt.Errorf("game: answer %q is not a valid word", answer)
	}
	return g, nil
}

// NewCustom starts a puzzle from an answer a person chose: the secret word of a
// custom puzzle. The id is drawn like any other random puzzle's, because
// there is nothing deterministic to reproduce here — the word came from a human,
// not from a seed.
//
// Unlike NewFrom it does not require the answer to be in the guess list. That is
// the whole point of the "ignore word list" choice on the way in: a name or a
// word the list does not carry is allowed, and Guess accepts it in return (see
// there). What it still refuses is a word no board could hold — the wrong
// length, or anything that is not a plain letter — because the tiles and the
// on-screen keyboard have nowhere to put it.
//
// Whether an off-list word is allowed at all is the caller's decision, not this
// constructor's: the custom screen checks words.IsValidGuess itself unless the
// player has asked it not to.
func NewCustom(answer string, length int) (*Game, error) {
	w := words.Normalize(answer)
	for i := 0; i < len(w); i++ {
		if w[i] < 'a' || w[i] > 'z' {
			return nil, fmt.Errorf("game: answer %q must be letters only", answer)
		}
	}
	id, err := newID()
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	g := &Game{
		ID:          id,
		Length:      length,
		Answer:      w,
		MaxAttempts: attemptsFor(length),
		Status:      InProgress,
		StartedAt:   now,
		UpdatedAt:   now,
		Custom:      true,
	}
	if err := g.Validate(); err != nil {
		return nil, err
	}
	return g, nil
}

// newID returns a random UUID (version 4), formatted canonically. It is
// hand-rolled rather than pulled from a dependency for the same reason the
// theme reader is: sixteen random bytes and six bit-twiddles do not warrant a
// module.
func newID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("game: generate id: %w", err)
	}
	return FormatID(b, 4), nil
}

// FormatID renders sixteen bytes as a canonical UUID string of the given
// version. It is exported because a derived id — a daily's, computed from its
// date — is built elsewhere but must look like every other id on disk, and the
// byte layout is worth having in exactly one place.
//
// The version nibble is the caller's to set (newID sets 4 for random ids;
// derived ids use 8, the "custom" version, which is what a hashed id honestly
// is); this only stamps the variant and formats.
func FormatID(b [16]byte, version byte) string {
	b[6] = b[6]&0x0f | version<<4
	b[8] = b[8]&0x3f | 0x80 // variant 10
	h := hex.EncodeToString(b[:])
	return h[:8] + "-" + h[8:12] + "-" + h[12:16] + "-" + h[16:20] + "-" + h[20:]
}

// codeSpace is how many distinct codes exist: six decimal digits.
const codeSpace = 1_000_000

// Code is the short, human-facing form of a puzzle id: six digits, derived by
// hashing, e.g. "042317".
//
// It is a label, never a key. Code is not injective — six digits is a space of
// a million, so a long history will eventually show two puzzles the same code —
// so nothing may look a puzzle up by it. The id remains the only identity.
//
// Deriving it by hashing rather than by counting means it depends on nothing
// but the puzzle itself. That is what lets a puzzle be deleted without
// disturbing any other puzzle's code, and it is also what will let a daily
// puzzle — whose id is derived deterministically from its date — show every
// player the same code, with nothing to coordinate.
//
// It accepts any id, including the 16-hex ids written before puzzles carried
// UUIDs, so old saves need no migration.
func Code(id string) string {
	sum := sha256.Sum256([]byte(id))
	return fmt.Sprintf("%06d", binary.BigEndian.Uint64(sum[:8])%codeSpace)
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
	// The answer is always typeable, whatever the list says. For every drawn
	// puzzle the second condition is dead weight, because every answer is also a
	// valid guess; for a custom puzzle set with "any word" on it is
	// the difference between a puzzle and an unwinnable one. It reveals nothing
	// that was not already true: an answer has always been a word the player
	// could type.
	if !words.IsValidGuess(g.Length, w) && w != g.Answer {
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
	// A tombstone has been stripped of everything the rest of these checks are
	// about, so only its identity is left to check.
	if g.Deleted {
		if g.ID == "" {
			return errors.New("game: missing id")
		}
		if !words.SupportedLength(g.Length) {
			return fmt.Errorf("game: unsupported length %d", g.Length)
		}
		return nil
	}

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
