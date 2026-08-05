package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/nxck2005/wortle/internal/game"
)

// JSON stores one file per puzzle under a directory.
//
// One file per puzzle keeps writes small (the game is saved after every guess)
// and means a single corrupt file costs one puzzle rather than the whole
// history. Writes go to a temp file and are renamed into place, so a crash
// mid-write cannot leave a half-written save.
//
// There is deliberately no index and no counter: a puzzle's displayed code is
// derived from its own id (see game.Code), so the store allocates nothing that
// deleting a puzzle could leave a hole in. An older install may still have a
// meta.json holding the retired puzzle counter; it is simply never read.
type JSON struct {
	dir string
}

const puzzleDir = "puzzles"

// DefaultDir is where puzzles live: ~/.config/wortle on Linux, and the
// platform equivalent elsewhere.
func DefaultDir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("store: locate config dir: %w", err)
	}
	return filepath.Join(base, "wortle"), nil
}

// NewJSON opens (and creates if needed) a store rooted at dir.
func NewJSON(dir string) (*JSON, error) {
	s := &JSON{dir: dir}
	if err := os.MkdirAll(filepath.Join(dir, puzzleDir), 0o755); err != nil {
		return nil, fmt.Errorf("store: create %s: %w", dir, err)
	}
	return s, nil
}

func (s *JSON) pathFor(id string) string {
	return filepath.Join(s.dir, puzzleDir, id+".json")
}

func (s *JSON) Save(g *game.Game) error {
	if err := g.Validate(); err != nil {
		return err
	}
	b, err := json.MarshalIndent(g, "", "  ")
	if err != nil {
		return fmt.Errorf("store: encode puzzle %s: %w", g.ID, err)
	}
	return writeFileAtomic(s.pathFor(g.ID), b)
}

// Load returns a playable puzzle. A tombstone is reported as ErrNotFound: it is
// a record of the sequence of play, not a puzzle, and nothing may resume one.
func (s *JSON) Load(id string) (*game.Game, error) {
	g, err := s.load(id)
	if err != nil {
		return nil, err
	}
	if g.Deleted {
		return nil, ErrNotFound
	}
	return g, nil
}

// load reads whatever is on disk, tombstones included. Only Delete and All,
// which have to see deletions, use it directly.
func (s *JSON) load(id string) (*game.Game, error) {
	b, err := os.ReadFile(s.pathFor(id))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("store: read puzzle %s: %w", id, err)
	}

	var g game.Game
	if err := json.Unmarshal(b, &g); err != nil {
		return nil, fmt.Errorf("store: decode puzzle %s: %w", id, err)
	}
	if err := g.Validate(); err != nil {
		return nil, fmt.Errorf("store: puzzle %s: %w", id, err)
	}
	return &g, nil
}

// Delete removes a puzzle.
//
// A *finished* puzzle is not unlinked but overwritten with its Tombstone: the
// answer and the guesses go, and a five-field marker stays in their place. That
// is what stops deleting a loss from merging the win runs either side of it and
// inflating the longest streak (see stats.Compute). The rewrite goes through
// writeFileAtomic like every other write, so a crash mid-delete leaves either
// the puzzle or the tombstone, never a half-written file.
//
// An unfinished puzzle is unlinked outright: streaks ignore in-progress
// puzzles, so a tombstone for one would record nothing.
func (s *JSON) Delete(id string) error {
	g, err := s.load(id)
	if err != nil {
		return err
	}
	if g.Deleted {
		return ErrNotFound
	}

	if g.Status.Done() {
		return s.saveTombstone(g.Tombstone())
	}

	if err := os.Remove(s.pathFor(id)); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return ErrNotFound
		}
		return fmt.Errorf("store: delete puzzle %s: %w", id, err)
	}
	return nil
}

// tombstoneRecord is how a deleted puzzle is written: the fields game.Tombstone
// keeps, and no others. Encoding the *game.Game itself would spell out every
// field it no longer has ("answer": "", "guesses": null, a zero startedAt),
// which reads as a corrupt puzzle rather than as a deliberate marker — and
// Game's tags carry no omitempty on purpose, so that an ordinary save is
// written exactly as it always was. The keys match Game's, so reading a
// tombstone is just decoding a Game.
type tombstoneRecord struct {
	ID        string      `json:"id"`
	Length    int         `json:"length"`
	Status    game.Status `json:"status"`
	UpdatedAt time.Time   `json:"updatedAt"`
	Deleted   bool        `json:"deleted"`
}

func (s *JSON) saveTombstone(g *game.Game) error {
	if err := g.Validate(); err != nil {
		return err
	}
	b, err := json.MarshalIndent(tombstoneRecord{
		ID:        g.ID,
		Length:    g.Length,
		Status:    g.Status,
		UpdatedAt: g.UpdatedAt,
		Deleted:   g.Deleted,
	}, "", "  ")
	if err != nil {
		return fmt.Errorf("store: encode tombstone %s: %w", g.ID, err)
	}
	return writeFileAtomic(s.pathFor(g.ID), b)
}

// All returns every readable record, tombstones included — stats need them to
// see where a deleted puzzle broke a streak, and they are the one caller that
// does. Unreadable files are skipped rather than failing the whole call, so one
// bad save cannot lock the player out of their history.
func (s *JSON) All() ([]*game.Game, error) {
	entries, err := os.ReadDir(filepath.Join(s.dir, puzzleDir))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("store: list puzzles: %w", err)
	}

	games := make([]*game.Game, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		g, err := s.load(strings.TrimSuffix(e.Name(), ".json"))
		if err != nil {
			continue
		}
		games = append(games, g)
	}
	return games, nil
}

func (s *JSON) List() ([]Summary, error) {
	games, err := s.All()
	if err != nil {
		return nil, err
	}
	// Tombstones are history, not puzzles: they belong to the streak walk, not
	// to the browse list or to anything else built on List.
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
	return out, nil
}

// writeFileAtomic writes via a temp file in the same directory, then renames.
// Same-directory matters: rename is only atomic within a filesystem.
func writeFileAtomic(path string, b []byte) error {
	dir := filepath.Dir(path)
	f, err := os.CreateTemp(dir, ".tmp-*")
	if err != nil {
		return fmt.Errorf("store: create temp in %s: %w", dir, err)
	}
	tmp := f.Name()
	defer os.Remove(tmp) // no-op once the rename succeeds

	if _, err := f.Write(b); err != nil {
		f.Close()
		return fmt.Errorf("store: write %s: %w", tmp, err)
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return fmt.Errorf("store: sync %s: %w", tmp, err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("store: close %s: %w", tmp, err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("store: rename into %s: %w", path, err)
	}
	return nil
}
