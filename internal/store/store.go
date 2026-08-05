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
	Length    int
	Status    game.Status
	Attempts  int
	Elapsed   time.Duration
	UpdatedAt time.Time
	// Daily is the date this puzzle is the daily for, or empty. Carried here so
	// the browse list can label one without loading the whole game.
	Daily string
}

// Store reads and writes puzzles.
type Store interface {
	// Save writes a puzzle, creating or replacing it.
	Save(g *game.Game) error
	// Load returns a puzzle by id, or ErrNotFound.
	Load(id string) (*game.Game, error)
	// Delete removes a puzzle by id, or returns ErrNotFound.
	Delete(id string) error
	// List returns summaries of all puzzles, most recently updated first.
	List() ([]Summary, error)
	// All returns every puzzle in full. Stats need the guess distribution,
	// which Summary deliberately omits.
	All() ([]*game.Game, error)
}

func summarize(g *game.Game) Summary {
	return Summary{
		ID:        g.ID,
		Length:    g.Length,
		Status:    g.Status,
		Attempts:  g.Attempts(),
		Elapsed:   g.Elapsed(),
		UpdatedAt: g.UpdatedAt,
		Daily:     g.Daily,
	}
}
