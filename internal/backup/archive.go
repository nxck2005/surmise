// Package backup is a player's whole local install in one file.
//
// surmise keeps everything on the machine it is played on, in one copy. In the
// browser that copy is localStorage, which clearing site data destroys; on a
// desktop it is a directory nothing tells the player about. This package is
// what lets them take it with them.
//
// It does no I/O. Build takes a store and returns bytes; Apply takes bytes and
// a store and writes through it. Where those bytes come from and go to is the
// platform's business — a file on a desktop, a download in a browser — and that
// split is what keeps the whole format testable without either.
//
// The one rule everything else follows from: an archive may only ever ADD. It
// never overwrites a record, never deletes one, and never lowers a counter. So
// importing the wrong file costs the player nothing, and importing the same
// file twice changes nothing the second time.
package backup

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/nxck2005/surmise/internal/game"
	"github.com/nxck2005/surmise/internal/store"
	"github.com/nxck2005/surmise/internal/theme"
)

// Format is what every archive says it is, and it is a frozen literal.
//
// It is deliberately NOT built from brand.Name, which is the rule everything
// else user-facing follows. An archive is the one thing this app produces that
// leaves the install and outlives it: a reader that compared against the
// current name would refuse every file ever written before a rename, and the
// player would lose exactly the history the file exists to protect. So the tag
// is written down once and never changes, for the same reason internal/daily
// freezes its derivation tags. TestFormatTagIsFrozen holds it to that.
const Format = "surmise.backup"

// Version is the shape of the file, not the version of the app. A reader
// refuses anything higher: a file from a newer release may hold records this
// one would silently drop, and dropping a player's history quietly is the one
// failure this package exists to prevent.
const Version = 1

// Archive is the file. Records are held as raw JSON rather than decoded games
// so that they are exactly the bytes a store holds — see store.EncodeRecord.
// That is what makes an archive portable between the two stores by
// construction rather than by a conversion somebody has to keep correct.
type Archive struct {
	Format    string            `json:"format"`
	Version   int               `json:"version"`
	CreatedAt time.Time         `json:"createdAt"`
	App       string            `json:"app,omitempty"`
	Puzzles   []json.RawMessage `json:"puzzles"`
	Settings  *store.Settings   `json:"settings,omitempty"`
	Themes    []theme.File      `json:"themes,omitempty"`
}

// Build reads a whole install into an archive.
//
// settings and themes may be zero and nil: a platform without preferences or
// without a theme directory — the browser has no theme directory at all — puts
// nothing in those sections rather than inventing one.
func Build(s store.Store, settings store.Settings, themes []theme.File, app string, now time.Time) ([]byte, error) {
	// All, not List: tombstones are records, and internal/stats reads them to
	// tell a deleted day from a day never played. A backup that dropped them
	// would restore a history whose streaks were wrong.
	games, err := s.All()
	if err != nil {
		return nil, fmt.Errorf("backup: read puzzles: %w", err)
	}

	// Sorted by id so two exports of an unchanged history are byte-identical.
	// A backup somebody diffs or checksums is worth more than one they cannot.
	sortByID(games)

	a := Archive{
		Format:    Format,
		Version:   Version,
		CreatedAt: now.UTC().Truncate(time.Second),
		App:       app,
		Puzzles:   make([]json.RawMessage, 0, len(games)),
		Themes:    themes,
	}
	for _, g := range games {
		b, err := store.EncodeRecord(g)
		if err != nil {
			return nil, fmt.Errorf("backup: encode %s: %w", g.ID, err)
		}
		a.Puzzles = append(a.Puzzles, b)
	}
	if settings != (store.Settings{}) {
		a.Settings = &settings
	}

	out, err := json.MarshalIndent(a, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("backup: encode archive: %w", err)
	}
	return out, nil
}

// Read parses an archive and checks every record in it before the caller is
// allowed to do anything with it.
//
// One bad record refuses the whole file, which is the opposite of what
// store.JSON.All does with a corrupt save — and deliberately so. A save that
// will not read is already the player's, and skipping it keeps the rest of
// their history reachable. An archive is data being invited in from outside,
// and half of one is not something to write into a working install.
func Read(b []byte) (*Archive, []*game.Game, error) {
	var a Archive
	if err := json.Unmarshal(b, &a); err != nil {
		return nil, nil, fmt.Errorf("backup: this is not a %s file: %w", Format, err)
	}
	if a.Format != Format {
		if a.Format == "" {
			return nil, nil, fmt.Errorf("backup: this is not a %s file", Format)
		}
		return nil, nil, fmt.Errorf("backup: this is a %q file, not %s", a.Format, Format)
	}
	if a.Version <= 0 {
		return nil, nil, fmt.Errorf("backup: no version in this file")
	}
	if a.Version > Version {
		return nil, nil, fmt.Errorf("backup: this file is version %d and this build reads %d — update the game and try again",
			a.Version, Version)
	}

	games := make([]*game.Game, 0, len(a.Puzzles))
	seen := make(map[string]bool, len(a.Puzzles))
	for i, raw := range a.Puzzles {
		g, err := store.DecodeRecord(fmt.Sprintf("record %d of this backup", i+1), raw)
		if err != nil {
			return nil, nil, fmt.Errorf("backup: %w", err)
		}
		if seen[g.ID] {
			return nil, nil, fmt.Errorf("backup: record %d repeats puzzle %s; this file is not consistent",
				i+1, game.Code(g.ID))
		}
		seen[g.ID] = true
		games = append(games, g)
	}
	return &a, games, nil
}

func sortByID(games []*game.Game) {
	for i := 1; i < len(games); i++ {
		for j := i; j > 0 && games[j].ID < games[j-1].ID; j-- {
			games[j], games[j-1] = games[j-1], games[j]
		}
	}
}
