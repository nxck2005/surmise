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
	"sync"

	"github.com/nxck2005/wortle/internal/game"
)

// JSON stores one file per puzzle under a directory, plus a small meta file
// holding the puzzle counter.
//
// One file per puzzle keeps writes small (the game is saved after every guess)
// and means a single corrupt file costs one puzzle rather than the whole
// history. Writes go to a temp file and are renamed into place, so a crash
// mid-write cannot leave a half-written save.
type JSON struct {
	dir string

	mu   sync.Mutex // serializes counter read-modify-write
	meta metaFile
}

type metaFile struct {
	LastNumber int `json:"lastNumber"`
}

const (
	puzzleDir = "puzzles"
	metaName  = "meta.json"
)

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
	if err := s.readMeta(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *JSON) metaPath() string { return filepath.Join(s.dir, metaName) }
func (s *JSON) pathFor(id string) string {
	return filepath.Join(s.dir, puzzleDir, id+".json")
}

func (s *JSON) readMeta() error {
	b, err := os.ReadFile(s.metaPath())
	if errors.Is(err, fs.ErrNotExist) {
		s.meta = metaFile{}
		return nil
	}
	if err != nil {
		return fmt.Errorf("store: read meta: %w", err)
	}
	if err := json.Unmarshal(b, &s.meta); err != nil {
		// A damaged counter should not make the game unstartable; recover by
		// rebuilding it from the puzzles on disk.
		s.meta = metaFile{LastNumber: s.highestNumberOnDisk()}
	}
	return nil
}

func (s *JSON) highestNumberOnDisk() int {
	games, err := s.All()
	if err != nil {
		return 0
	}
	highest := 0
	for _, g := range games {
		if g.Number > highest {
			highest = g.Number
		}
	}
	return highest
}

// NextNumber reserves the next display number and persists the counter
// immediately, so two puzzles can never share a number even if the process
// dies before the first is saved.
func (s *JSON) NextNumber() (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.meta.LastNumber++
	b, err := json.MarshalIndent(s.meta, "", "  ")
	if err != nil {
		return 0, fmt.Errorf("store: encode meta: %w", err)
	}
	if err := writeFileAtomic(s.metaPath(), b); err != nil {
		s.meta.LastNumber-- // roll back so memory matches disk
		return 0, err
	}
	return s.meta.LastNumber, nil
}

// PeekNumber reports the next number without reserving it. In this single-user,
// single-process app nothing else advances the counter between a peek and the
// matching NextNumber, so the two agree.
func (s *JSON) PeekNumber() (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.meta.LastNumber + 1, nil
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

func (s *JSON) Load(id string) (*game.Game, error) {
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

// All returns every readable puzzle. Unreadable files are skipped rather than
// failing the whole call, so one bad save cannot lock the player out of their
// history.
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
		g, err := s.Load(strings.TrimSuffix(e.Name(), ".json"))
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
	out := make([]Summary, len(games))
	for i, g := range games {
		out[i] = summarize(g)
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
