// Package store persists puzzles between runs.
//
// Everything is expressed against the Store interface so the local JSON
// implementation can later sit beside, or behind, a networked one without the
// UI changing. That is the seam for the eventual global leaderboard.
package store

import (
	"errors"
	"time"

	"github.com/nxck2005/wortle/internal/game"
)

// ErrNotFound is returned by Load for an unknown id.
var ErrNotFound = errors.New("store: puzzle not found")

// Summary is the cheap view of a puzzle, for the browse list. It avoids
// loading and decoding every saved game just to render a menu.
type Summary struct {
	ID        string
	Number    int
	Length    int
	Status    game.Status
	Attempts  int
	Elapsed   time.Duration
	UpdatedAt time.Time
}

// Store reads and writes puzzles.
type Store interface {
	// Save writes a puzzle, creating or replacing it.
	Save(g *game.Game) error
	// Load returns a puzzle by id, or ErrNotFound.
	Load(id string) (*game.Game, error)
	// List returns summaries of all puzzles, most recently updated first.
	List() ([]Summary, error)
	// All returns every puzzle in full. Stats need the guess distribution,
	// which Summary deliberately omits.
	All() ([]*game.Game, error)
	// NextNumber reserves and returns the next display number.
	NextNumber() (int, error)
	// PeekNumber returns what NextNumber would return, without reserving it.
	// Puzzles show a prospective "#N" before they are saved; the number is
	// only committed once the puzzle is worth keeping.
	PeekNumber() (int, error)
}

func summarize(g *game.Game) Summary {
	return Summary{
		ID:        g.ID,
		Number:    g.Number,
		Length:    g.Length,
		Status:    g.Status,
		Attempts:  g.Attempts(),
		Elapsed:   g.Elapsed(),
		UpdatedAt: g.UpdatedAt,
	}
}
