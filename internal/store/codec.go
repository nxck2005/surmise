package store

import (
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/nxck2005/surmise/internal/game"
)

// The bytes a store writes, and the rules about what a deleted puzzle turns
// into, live here rather than in any one implementation.
//
// There are two stores — JSON on a filesystem, KVStore on a browser's
// localStorage — and the tombstone rules are the subtle part of both. A deleted
// *finished* puzzle becomes a marker rather than nothing, because that is what
// stops deleting a loss from merging the win runs either side of it (see
// stats.Compute). A second copy of that logic would drift, and drift there is
// silently wrong streaks rather than a crash.
//
// Sharing the codec has a second effect worth keeping: both stores hold the
// same bytes under a different key, so moving a history between them is a copy.

// schemaVersion is the save-format version stamped into every record and into
// settings as they are written. A reader accepts its own version and 0 — 0 is
// every file written before the tag existed, and it stays valid forever — and
// refuses anything else: a number it does not know means either a file from a
// newer app or a corrupt one, and misreading either silently is exactly what
// the tag exists to prevent. Bump it only for a breaking change to the format;
// adding a field never is one. The rule lives in docs/UPGRADING.md.
const schemaVersion = 1

// encodeRecord renders whatever a store was handed: a puzzle, or the marker a
// deleted one leaves behind. Routing here rather than at the call site is what
// makes Save total — a caller that holds a tombstone (importing a backup is the
// first) writes the same bytes Delete would, instead of a Game spelled out in
// full, which codec's own rule below says reads as corruption.
func encodeRecord(g *game.Game) ([]byte, error) {
	if g.Deleted {
		return encodeTombstone(g)
	}
	return encodeGame(g)
}

// encodeGame renders a puzzle for storage. It is the live-puzzle half of
// encodeRecord; anything that might hold a tombstone wants that instead.
//
// Stamping happens here rather than at every constructor so no call site can
// forget: a record leaves this package saying what format it is in. Callers may
// hold a Schema they read off an older file; overwriting only a zero keeps such
// a value honest through a load-modify-save cycle.
func encodeGame(g *game.Game) ([]byte, error) {
	if err := g.Validate(); err != nil {
		return nil, err
	}
	if g.Schema == 0 {
		g.Schema = schemaVersion
	}
	b, err := json.MarshalIndent(g, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("store: encode puzzle %s: %w", g.ID, err)
	}
	return b, nil
}

// decodeGame reads whatever was stored, tombstones included. Callers that must
// not resume a deletion check Deleted themselves; only Delete and All see one.
func decodeGame(id string, b []byte) (*game.Game, error) {
	return decodeRecord("puzzle "+id, b)
}

// decodeRecord is decodeGame with the caller's own name for what it is reading,
// so a record that came out of a backup file reports as a record rather than as
// a puzzle the store was asked for.
//
// The schema check is the whole promise: 0 is every pre-tag file and this
// version's own, anything else is refused rather than half-understood.
func decodeRecord(label string, b []byte) (*game.Game, error) {
	var g game.Game
	if err := json.Unmarshal(b, &g); err != nil {
		return nil, fmt.Errorf("store: decode %s: %w", label, err)
	}
	if g.Schema != 0 && g.Schema != schemaVersion {
		return nil, fmt.Errorf("store: %s: schema version mismatch", label)
	}
	if err := g.Validate(); err != nil {
		return nil, fmt.Errorf("store: %s: %w", label, err)
	}
	return &g, nil
}

// EncodeRecord and DecodeRecord are the codec, for a caller outside this
// package that has to hold the same bytes a store does. internal/backup is the
// one: an archive carries records, tombstones included, and the whole claim
// that a backup moves between the two stores rests on there being exactly one
// set of rules about what a record looks like. Do not add a second.
func EncodeRecord(g *game.Game) ([]byte, error) { return encodeRecord(g) }

// DecodeRecord reads one back. label names what is being read, for the error.
func DecodeRecord(label string, b []byte) (*game.Game, error) { return decodeRecord(label, b) }

// tombstoneRecord is how a deleted puzzle is written: the fields game.Tombstone
// keeps, and no others. Encoding the *game.Game itself would spell out every
// field it no longer has ("answer": "", "guesses": null, a zero startedAt),
// which reads as a corrupt puzzle rather than as a deliberate marker — and
// Game's tags carry no omitempty on purpose, so that an ordinary save is
// written exactly as it always was. The keys match Game's, so reading a
// tombstone is just decoding a Game.
type tombstoneRecord struct {
	// Schema carries the same format tag every record does; a tombstone is a
	// record, not an exception to the rule.
	Schema    int         `json:"schema"`
	ID        string      `json:"id"`
	Length    int         `json:"length"`
	Status    game.Status `json:"status"`
	UpdatedAt time.Time   `json:"updatedAt"`
	// Daily carries omitempty, unlike its neighbours, so a casual puzzle's
	// tombstone is written exactly as it always was and only a deleted daily
	// gains a key. See game.Tombstone for why a deleted day has to remember
	// which day it was.
	Daily string `json:"daily,omitempty"`
	// Custom carries omitempty for the same reason as Daily, and is kept for the
	// reason game.Tombstone gives: a custom puzzle counts towards nothing, so a
	// tombstone that forgot it was custom would read as an ordinary loss and
	// break a streak the puzzle itself never touched.
	Custom  bool `json:"custom,omitempty"`
	Deleted bool `json:"deleted"`
}

// encodeTombstone renders the marker a deleted finished puzzle leaves behind.
func encodeTombstone(g *game.Game) ([]byte, error) {
	if err := g.Validate(); err != nil {
		return nil, err
	}
	b, err := json.MarshalIndent(tombstoneRecord{
		Schema:    schemaVersion,
		ID:        g.ID,
		Length:    g.Length,
		Status:    g.Status,
		UpdatedAt: g.UpdatedAt,
		Daily:     g.Daily,
		Custom:    g.Custom,
		Deleted:   g.Deleted,
	}, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("store: encode tombstone %s: %w", g.ID, err)
	}
	return b, nil
}

// encodeSettings and decodeSettings keep preferences in one shape too. A
// missing or damaged blob yields the defaults rather than an error: every field
// has a working zero value, and a bad settings file must never cost a puzzle.
//
// Settings carry the same schema tag records do, stamped on write and checked
// on read — but a mismatch here degrades to the defaults rather than an error,
// because losing a preference is acceptable and losing a puzzle is not.
func encodeSettings(v Settings) ([]byte, error) {
	if v.Schema == 0 {
		v.Schema = schemaVersion
	}
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("store: encode settings: %w", err)
	}
	return b, nil
}

func decodeSettings(b []byte) Settings {
	var out Settings
	if err := json.Unmarshal(b, &out); err != nil || (out.Schema != 0 && out.Schema != schemaVersion) {
		return Settings{}
	}
	return out
}

// summarise turns records into the browse list every store's List returns:
// tombstones dropped, most recently updated first. Tombstones are history, and
// history belongs to the streak walk, not to a list of puzzles to open.
func summaries(games []*game.Game) []Summary {
	out := make([]Summary, 0, len(games))
	for _, g := range games {
		if g.Deleted {
			continue
		}
		out = append(out, summarize(g))
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].UpdatedAt.After(out[j].UpdatedAt)
	})
	return out
}
