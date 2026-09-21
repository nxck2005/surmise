package store

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/nxck2005/surmise/internal/brand"
	"github.com/nxck2005/surmise/internal/game"
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

// MaxRecordBytes bounds one stored puzzle or settings file. A real record is a
// few kilobytes at most — attempts are length+1, so even the longest board
// holds seven short words — and the cap is what stops a planted or hand-edited
// file from being read whole into memory before the codec can refuse it.
// internal/backup holds individual archive records to the same figure, so one
// constant describes a record everywhere.
const MaxRecordBytes = 64 << 10

// DefaultDir is where puzzles live: ~/.config/surmise on Linux, and the
// platform equivalent elsewhere.
func DefaultDir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("store: locate config dir: %w", err)
	}
	return filepath.Join(base, brand.Name), nil
}

// NewJSON opens (and creates if needed) a store rooted at dir.
func NewJSON(dir string) (*JSON, error) {
	s := &JSON{dir: dir}
	// 0700: the data directory belongs to the player alone. The files inside
	// carry 0600; a fresh install should not open the directory wider than the
	// records it holds. Directories that already exist keep whatever mode they
	// had — tightening those is a policy a user chooses, not a silent
	// side effect of opening the app.
	if err := os.MkdirAll(filepath.Join(dir, puzzleDir), 0o700); err != nil {
		return nil, fmt.Errorf("store: create %s: %w", dir, err)
	}
	return s, nil
}

// pathFor names a puzzle's file. It is checked because an id is about to be
// joined into a path, and an id can arrive from outside — a crafted backup, a
// hand-edited save. Without this, an id like "../settings" writes
// settings.json next door to the puzzle directory. The codec refuses such an
// id on its own read and write paths too; this is the layer that makes the
// refusal hold even for a caller that reaches the store directly.
func (s *JSON) pathFor(id string) (string, error) {
	if !game.ValidID(id) {
		return "", fmt.Errorf("store: invalid puzzle id %q", id)
	}
	return filepath.Join(s.dir, puzzleDir, id+".json"), nil
}

func (s *JSON) Save(g *game.Game) error {
	b, err := encodeRecord(g)
	if err != nil {
		return err
	}
	path, err := s.pathFor(g.ID)
	if err != nil {
		return err
	}
	return writeFileAtomic(path, b)
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
	path, err := s.pathFor(id)
	if err != nil {
		return nil, err
	}
	b, err := readLimited(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("store: read puzzle %s: %w", id, err)
	}

	return decodeGame(id, b)
}

// readLimited reads a file that is supposed to hold one record, refusing
// anything over MaxRecordBytes rather than pulling it all into memory first.
// The one-byte overshoot is how "exactly at the limit" is told from "over it"
// without a second read. A file that does not exist comes back as the open
// error, so callers can still recognise fs.ErrNotExist.
//
// A file that is not a plain one is refused before it is opened. Opening a
// FIFO for reading blocks until a writer appears, and a planted one would hang
// every scan that read it — startup, the menu, the profile, the list — so the
// mode has to be the warning. The check is repeated on the descriptor because
// a file swapped between the stat and the open is the one thing the first
// check cannot see; on a platform where that swap can block, closing it would
// need an O_NONBLOCK open, which is deliberately not done here. The theme
// reader carries the same accepted window (see theme.readThemeFile).
func readLimited(path string) ([]byte, error) {
	before, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if !before.Mode().IsRegular() {
		return nil, fmt.Errorf("%s is not a regular file", filepath.Base(path))
	}

	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("%s is not a regular file", filepath.Base(path))
	}

	b, err := io.ReadAll(io.LimitReader(f, MaxRecordBytes+1))
	if err != nil {
		return nil, err
	}
	if len(b) > MaxRecordBytes {
		return nil, fmt.Errorf("%s is larger than %d bytes", filepath.Base(path), MaxRecordBytes)
	}
	return b, nil
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

	path, err := s.pathFor(id)
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return ErrNotFound
		}
		return fmt.Errorf("store: delete puzzle %s: %w", id, err)
	}
	return nil
}

// saveTombstone writes the marker in the puzzle's place. The encoding is shared
// with the browser store; see codec.go.
func (s *JSON) saveTombstone(g *game.Game) error {
	b, err := encodeTombstone(g)
	if err != nil {
		return err
	}
	path, err := s.pathFor(g.ID)
	if err != nil {
		return err
	}
	return writeFileAtomic(path, b)
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
	return summaries(games), nil
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
