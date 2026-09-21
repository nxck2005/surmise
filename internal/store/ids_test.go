package store

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/nxck2005/surmise/internal/game"
)

// IDs is the read that must not open a record, which puts the rules about what
// a record *is* in one place per store: a name or key this app could not have
// written is not one, while a name it could have written is — even when the
// bytes behind it are unreadable, because the identity is what occupies the
// space.

// A puzzle directory gathers stray files over an install's life: the retired
// meta.json, a settings file, an editor backup, a hand-dropped note. None of
// them is a puzzle, and an id that has never been legal is not one either.
func TestJSONIDsIgnoreNamesThatAreNotRecords(t *testing.T) {
	dir := t.TempDir()
	s, err := NewJSON(dir)
	if err != nil {
		t.Fatal(err)
	}

	// A real record to sit among the strays.
	g := newGame(t, 5)
	if err := s.Save(g); err != nil {
		t.Fatal(err)
	}

	puzzles := filepath.Join(dir, puzzleDir)
	strays := []string{
		"settings.json", // the preferences file, one directory up in a real install
		"meta.json",     // the retired puzzle counter
		"notes.txt",     // not even JSON
		"crane.json",    // a word, not an id
		".json",         // an empty name
		"../escape.json",
		"8E1C4A72-9B3D-4F60-8123-456789ABCDE0.json", // an id in the wrong case
	}
	for _, name := range strays {
		if err := os.WriteFile(filepath.Join(puzzles, name), []byte("{}"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	// A directory whose name looks like a record is not one.
	if err := os.Mkdir(filepath.Join(puzzles, "3f2a7b4c-5d6e-4f70-8123-456789abcdef.json"), 0o700); err != nil {
		t.Fatal(err)
	}

	ids, err := s.IDs()
	if err != nil {
		t.Fatalf("IDs: %v", err)
	}
	if len(ids) != 1 || ids[0] != g.ID {
		t.Errorf("IDs = %v, want only the real record %s", ids, g.ID)
	}
}

// A record whose bytes will not decode still holds its identity: All skips it
// (one bad save must not lock the player out of the rest), but the id is there
// and must not read as absent — a restore that treated it as absent would
// overwrite it, and the code it wears must stay reserved.
func TestJSONIDsKeepACorruptRecord(t *testing.T) {
	dir := t.TempDir()
	s, err := NewJSON(dir)
	if err != nil {
		t.Fatal(err)
	}
	g := newGame(t, 5)
	if err := os.WriteFile(filepath.Join(dir, puzzleDir, g.ID+".json"), []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}

	ids, err := s.IDs()
	if err != nil {
		t.Fatalf("IDs: %v", err)
	}
	if len(ids) != 1 || ids[0] != g.ID {
		t.Errorf("IDs = %v, want the unreadable record's id %s", ids, g.ID)
	}

	games, err := s.All()
	if err != nil {
		t.Fatalf("All: %v", err)
	}
	if len(games) != 0 {
		t.Errorf("All = %+v, want the unreadable record skipped", games)
	}
}

// The key space is shared with preferences and with whatever else an origin has
// stored, and only the puzzle prefix followed by a legal id names a record.
// Keys() has no order, so the answer does not either.
func TestKVIDsIgnoreKeysThatAreNotRecords(t *testing.T) {
	kv := NewMemoryKV()
	s := NewKV(kv)

	g := newGame(t, 5)
	if err := s.Save(g); err != nil {
		t.Fatal(err)
	}
	if err := s.SaveSettings(Settings{Theme: "nord"}); err != nil {
		t.Fatal(err)
	}

	other := newGame(t, 5)
	strays := []string{
		kvSettingsKey,
		"otherapp/v1/puzzle/" + other.ID,
		"surmise/v2/puzzle/" + other.ID,
		kvPuzzlePrefix,
		kvPuzzlePrefix + "crane",
		kvPuzzlePrefix + "8E1C4A72-9B3D-4F60",
		"surmise/v1/puzzles/" + g.ID,
		kvPuzzlePrefix + g.ID + "/extra",
		kvPuzzlePrefix + g.ID + ".json",
	}
	for _, k := range strays {
		if err := kv.Set(k, "{}"); err != nil {
			t.Fatal(err)
		}
	}

	ids, err := s.IDs()
	if err != nil {
		t.Fatalf("IDs: %v", err)
	}
	if len(ids) != 1 || ids[0] != g.ID {
		t.Errorf("IDs = %v, want only the real record %s", ids, g.ID)
	}
}

// A stored key whose value will not decode keeps its id for the same reason a
// file does: it occupies the identity, so the import must not overwrite it and
// the code must not be handed to a new puzzle.
func TestKVIDsKeepACorruptRecord(t *testing.T) {
	kv := NewMemoryKV()
	s := NewKV(kv)

	g := newGame(t, 5)
	if err := kv.Set(kvPuzzleKey(g.ID), "{not json"); err != nil {
		t.Fatal(err)
	}

	ids, err := s.IDs()
	if err != nil {
		t.Fatalf("IDs: %v", err)
	}
	if len(ids) != 1 || ids[0] != g.ID {
		t.Errorf("IDs = %v, want the unreadable record's id %s", ids, g.ID)
	}

	games, err := s.All()
	if err != nil {
		t.Fatalf("All: %v", err)
	}
	if len(games) != 0 {
		t.Errorf("All = %+v, want the unreadable record skipped", games)
	}
}

// BenchmarkIDsAgainstAll is the identity-only read against the full one at the
// sizes a real history reaches. It is here so the difference stays measured
// rather than asserted: All decodes every record, IDs opens none. Benchmarks
// are not part of CI, and no timing here is a threshold.
func BenchmarkIDsAgainstAll(b *testing.B) {
	for _, n := range []int{100, 1000, 10000} {
		b.Run("JSON/IDs/"+strconv.Itoa(n), func(b *testing.B) {
			s := fillRecords(b, n)
			b.ReportAllocs()
			b.ResetTimer()
			for range b.N {
				if _, err := s.IDs(); err != nil {
					b.Fatal(err)
				}
			}
		})
		b.Run("JSON/All/"+strconv.Itoa(n), func(b *testing.B) {
			s := fillRecords(b, n)
			b.ReportAllocs()
			b.ResetTimer()
			for range b.N {
				if _, err := s.All(); err != nil {
					b.Fatal(err)
				}
			}
		})
	}

	for _, n := range []int{100, 1000} {
		b.Run("KV/IDs/"+strconv.Itoa(n), func(b *testing.B) {
			s := kvWith(b, n)
			b.ReportAllocs()
			b.ResetTimer()
			for range b.N {
				if _, err := s.IDs(); err != nil {
					b.Fatal(err)
				}
			}
		})
		b.Run("KV/All/"+strconv.Itoa(n), func(b *testing.B) {
			s := kvWith(b, n)
			b.ReportAllocs()
			b.ResetTimer()
			for range b.N {
				if _, err := s.All(); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// fillRecords writes n real records straight to disk, without Save's fsync: a
// benchmark fixture is not a durability test.
func fillRecords(b *testing.B, n int) *JSON {
	b.Helper()
	dir := b.TempDir()
	s, err := NewJSON(dir)
	if err != nil {
		b.Fatal(err)
	}
	for range n {
		g := newRecord(b)
		raw, err := encodeGame(g)
		if err != nil {
			b.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, puzzleDir, g.ID+".json"), raw, 0o600); err != nil {
			b.Fatal(err)
		}
	}
	return s
}

func kvWith(b *testing.B, n int) *KVStore {
	b.Helper()
	s := NewKV(NewMemoryKV())
	for range n {
		if err := s.Save(newRecord(b)); err != nil {
			b.Fatal(err)
		}
	}
	return s
}

// newRecord is the fixture both benchmarks fill with: a puzzle with a guess on
// it, so the record is the size one really is rather than a fresh board's.
func newRecord(b *testing.B) *game.Game {
	b.Helper()
	g, err := game.New(5)
	if err != nil {
		b.Fatal(err)
	}
	g.Answer = "crane"
	if err := g.Guess("about"); err != nil {
		b.Fatal(err)
	}
	return g
}
