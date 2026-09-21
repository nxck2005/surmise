// Package store persists client puzzle history.
//
// Store is shared by the native JSON and browser KV implementations. It is a
// client persistence boundary, not an authoritative leaderboard protocol:
// game.Game contains the plaintext answer and client-authored result data.
// Future network play needs a purpose-built API, though a sync cache may wrap a
// Store without changing the UI.
package store

import (
	"errors"
	"time"

	"github.com/nxck2005/surmise/internal/game"
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
	// Custom reports a custom puzzle, whose answer a person chose. Carried
	// for the same reason as Daily: the browse list labels one, and Summary is
	// all the list ever sees.
	Custom bool
	// Challenge reports a reproducible social challenge. The list needs only
	// the origin label; the full canonical code stays on the Game.
	Challenge bool
}

// Store reads and writes puzzles.
type Store interface {
	// Save writes a puzzle, creating or replacing it.
	Save(g *game.Game) error
	// Load returns a puzzle by id, or ErrNotFound.
	Load(id string) (*game.Game, error)
	// Delete removes a puzzle by id, or returns ErrNotFound.
	Delete(id string) error
	// IDs returns the id of every puzzle record the store holds, tombstones
	// included, in ascending order.
	//
	// It is the identity-only read: no record is opened or decoded, so a caller
	// that only needs to know *which* puzzles exist — a fresh board's code
	// check, a restore's duplicate check, the daily screen's tombstone hunt —
	// does not pay for the history it is asking about. That cost is the whole
	// reason this method exists: All is O(records) of JSON decode, and a
	// browser pays a localStorage crossing per record on top.
	//
	// The semantics are exactly "what would All see, identity only":
	//
	//   - a record that cannot be decoded still occupies storage, so its id is
	//     returned (an unreadable file is not a licence to overwrite it);
	//   - a deleted finished puzzle's tombstone is a record, so its id is
	//     returned even though List and Load hide it;
	//   - settings and anything else that is not a puzzle this app wrote are
	//     not, and an id-shaped file or key that has never been a legal id is
	//     not either.
	//
	// Callers that need to read the puzzles want List or All; anything that
	// turns around and loads every id it was given has gained nothing.
	IDs() ([]string, error)
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
		Custom:    g.Custom,
		Challenge: g.Challenge != nil,
	}
}
