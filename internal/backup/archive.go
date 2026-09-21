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
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
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

// MaxArchiveBytes bounds an archive before it is parsed. An archive is the one
// file this app invites in from outside, and a real history is a few megabytes
// at most — ten thousand records of a few kilobytes each — so the cap is far
// above any honest file while keeping a hostile one from being read and
// unmarshalled whole. The three native and browser entry points all hold
// themselves to this figure, so one number describes "too big to be a backup".
const MaxArchiveBytes = 64 << 20

// What an archive may hold, enforced by recordList and themeList as the arrays
// are decoded: the cap is checked before the element past it is read, so a
// file can never make the decoder allocate more than the limits allow. The
// limits are two orders of magnitude above a long history; their job is to
// bound the shapes a parser walks, not to ration anyone's play.
const (
	maxPuzzles = 10_000
	maxThemes  = 256
)

// Archive is the file. Records are held as raw JSON rather than decoded games
// so that they are exactly the bytes a store holds — see store.EncodeRecord.
// That is what makes an archive portable between the two stores by
// construction rather than by a conversion somebody has to keep correct.
//
// The two arrays are named slice types rather than plain slices because they
// have to count themselves as they are decoded; see decodeBounded. The JSON
// shape is unchanged and Build still writes them the way it always did.
type Archive struct {
	Format    string          `json:"format"`
	Version   int             `json:"version"`
	CreatedAt time.Time       `json:"createdAt"`
	App       string          `json:"app,omitempty"`
	Puzzles   recordList      `json:"puzzles"`
	Settings  *store.Settings `json:"settings,omitempty"`
	Themes    themeList       `json:"themes,omitempty"`
}

// recordList is the puzzles array. It exists so the record cap is applied to
// the array while it is read: json.Unmarshal into a plain []json.RawMessage
// materializes every element first and only then can Read count them, which is
// how a few megabytes naming two million empty puzzles once spent hundreds of
// times their size before the refusal. Each element is decoded and counted one
// at a time instead, and the array is refused at the element past the cap.
type recordList []json.RawMessage

func (l *recordList) UnmarshalJSON(b []byte) error {
	elems, err := decodeBounded[json.RawMessage](b, maxPuzzles, "records")
	if err != nil {
		return err
	}
	*l = elems
	return nil
}

// themeList is the themes array, bounded the same way and for the same reason.
type themeList []theme.File

func (l *themeList) UnmarshalJSON(b []byte) error {
	elems, err := decodeBounded[theme.File](b, maxThemes, "themes")
	if err != nil {
		return err
	}
	*l = elems
	return nil
}

// tooManyError is the refusal a count limit produces. Build raises it for an
// archive it was about to write and the decoder raises it for one it was about
// to read, so both say the same sentence about the same limit.
type tooManyError struct {
	what string // "records" or "themes"
	n    int
	max  int
}

func (e tooManyError) Error() string {
	return fmt.Sprintf("backup: %d %s is more than a backup may hold (%d)", e.n, e.what, e.max)
}

// decodeBounded reads one JSON array without ever holding more than max
// elements. The cap is checked before each element is decoded, so a hostile
// array is refused at the limit rather than after it has been allocated.
func decodeBounded[T any](b []byte, max int, what string) ([]T, error) {
	dec := json.NewDecoder(bytes.NewReader(b))
	tok, err := dec.Token()
	if err != nil {
		return nil, err
	}
	if tok == nil {
		// A JSON null is a section that is not there, the same as no elements.
		return nil, nil
	}
	if d, ok := tok.(json.Delim); !ok || d != '[' {
		return nil, fmt.Errorf("backup: want an array of %s", what)
	}

	var out []T
	for dec.More() {
		if len(out) >= max {
			return nil, tooManyError{what: what, n: max + 1, max: max}
		}
		var elem T
		if err := dec.Decode(&elem); err != nil {
			return nil, err
		}
		out = append(out, elem)
	}
	if _, err := dec.Token(); err != nil { // the closing ']'
		return nil, err
	}
	return out, nil
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
	if len(b) > MaxArchiveBytes {
		return nil, nil, fmt.Errorf("backup: this file is larger than %d bytes", MaxArchiveBytes)
	}

	var a Archive
	if err := json.Unmarshal(b, &a); err != nil {
		// A count limit speaks for itself: "10001 records is more than a
		// backup may hold" is the whole answer, not a sign that this is some
		// other file. See recordList.
		var tooMany tooManyError
		if errors.As(err, &tooMany) {
			return nil, nil, tooMany
		}
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
	for i, t := range a.Themes {
		if len(t.Body) > theme.MaxFileBytes {
			return nil, nil, fmt.Errorf("backup: theme %d is larger than %d bytes", i+1, theme.MaxFileBytes)
		}
	}
	// The settings section is the one part of the file with no per-field bound
	// of its own, and an archive may devote most of its 64 MiB to one string.
	// The same rules the settings store writes and reads under are applied
	// here, before Apply can fill any of them in — see store.ValidateSettings.
	if a.Settings != nil {
		if err := store.ValidateSettings(*a.Settings); err != nil {
			return nil, nil, fmt.Errorf("backup: settings: %w", err)
		}
	}

	games := make([]*game.Game, 0, len(a.Puzzles))
	seen := make(map[string]bool, len(a.Puzzles))
	for i, raw := range a.Puzzles {
		if len(raw) > store.MaxRecordBytes {
			return nil, nil, fmt.Errorf("backup: record %d is larger than %d bytes", i+1, store.MaxRecordBytes)
		}
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
	sort.Slice(games, func(i, j int) bool { return games[i].ID < games[j].ID })
}
