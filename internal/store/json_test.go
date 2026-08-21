package store

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/nxck2005/surmise/internal/game"
)

func newStore(t *testing.T) *JSON {
	t.Helper()
	s, err := NewJSON(t.TempDir())
	if err != nil {
		t.Fatalf("NewJSON: %v", err)
	}
	return s
}

func newGame(t *testing.T, length int) *game.Game {
	t.Helper()
	g, err := game.New(length)
	if err != nil {
		t.Fatalf("game.New: %v", err)
	}
	return g
}

// The core persistence guarantee: a puzzle interrupted mid-game comes back
// with its board intact.
func TestSaveLoadRoundTrip(t *testing.T) {
	s := newStore(t)
	g := newGame(t, 5)
	g.Answer = "crane"
	if err := g.Guess("about"); err != nil {
		t.Fatal(err)
	}
	g.AddElapsed(1500 * 1e6) // 1.5s in nanoseconds

	if err := s.Save(g); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, err := s.Load(g.ID)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.Answer != g.Answer || got.ID != g.ID || got.Length != g.Length {
		t.Errorf("Load returned %+v, want %+v", got, g)
	}
	if len(got.Guesses) != 1 || got.Guesses[0] != "about" {
		t.Errorf("guesses = %v, want [about]", got.Guesses)
	}
	if len(got.Marks) != 1 || len(got.Marks[0]) != 5 {
		t.Errorf("marks = %v, want one row of 5", got.Marks)
	}
	if got.Elapsed() != g.Elapsed() {
		t.Errorf("elapsed = %v, want %v", got.Elapsed(), g.Elapsed())
	}
	if got.Status != game.InProgress {
		t.Errorf("status = %v, want in progress", got.Status)
	}
}

func TestLoadMissingReturnsNotFound(t *testing.T) {
	s := newStore(t)
	if _, err := s.Load("nope"); !errors.Is(err, ErrNotFound) {
		t.Errorf("Load(missing) = %v, want ErrNotFound", err)
	}
}

func TestListIsNewestFirst(t *testing.T) {
	s := newStore(t)
	for _, n := range []int{4, 5, 6} {
		if err := s.Save(newGame(t, n)); err != nil {
			t.Fatal(err)
		}
	}

	list, err := s.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 3 {
		t.Fatalf("List returned %d puzzles, want 3", len(list))
	}
	for i := 1; i < len(list); i++ {
		if list[i-1].UpdatedAt.Before(list[i].UpdatedAt) {
			t.Errorf("List not sorted newest first: %v", list)
		}
	}
}

// One unreadable file must not hide the rest of the player's history.
func TestAllSkipsCorruptFiles(t *testing.T) {
	s := newStore(t)
	good := newGame(t, 5)
	if err := s.Save(good); err != nil {
		t.Fatal(err)
	}
	bad := filepath.Join(s.dir, puzzleDir, "broken.json")
	if err := os.WriteFile(bad, []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}

	games, err := s.All()
	if err != nil {
		t.Fatalf("All: %v", err)
	}
	if len(games) != 1 || games[0].ID != good.ID {
		t.Errorf("All() = %v, want just the good puzzle", games)
	}
}

func TestSaveRejectsInvalidGame(t *testing.T) {
	s := newStore(t)
	g := newGame(t, 5)
	g.Answer = "toolongforfive"
	if err := s.Save(g); err == nil {
		t.Error("Save accepted an invalid game")
	}
}

// An unfinished puzzle is unlinked outright: streaks never look at one, so
// there is nothing about it worth remembering.
func TestDeleteRemovesOnlyItsPuzzle(t *testing.T) {
	s := newStore(t)
	doomed, keep := newGame(t, 5), newGame(t, 4)
	for _, g := range []*game.Game{doomed, keep} {
		if err := s.Save(g); err != nil {
			t.Fatal(err)
		}
	}

	if err := s.Delete(doomed.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := s.Load(doomed.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("Load after Delete = %v, want ErrNotFound", err)
	}
	if _, err := os.Stat(s.pathFor(doomed.ID)); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("file still on disk after Delete: %v", err)
	}

	list, err := s.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].ID != keep.ID {
		t.Errorf("List after Delete = %v, want just the kept puzzle", list)
	}
}

// Deleting a finished puzzle leaves a tombstone in its place, so the streak
// walk still knows a finished puzzle sat at that moment. Everything about how
// it was played is destroyed.
func TestDeleteFinishedLeavesTombstone(t *testing.T) {
	s := newStore(t)
	g := newGame(t, 5)
	g.Answer = "crane"
	if err := g.Guess("crane"); err != nil {
		t.Fatal(err)
	}
	if g.Status != game.Won {
		t.Fatalf("status = %v, want won", g.Status)
	}
	if err := s.Save(g); err != nil {
		t.Fatal(err)
	}

	if err := s.Delete(g.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := os.Stat(s.pathFor(g.ID)); err != nil {
		t.Fatalf("tombstone missing from disk: %v", err)
	}

	// Nothing that reads puzzles may hand a tombstone back as one.
	if _, err := s.Load(g.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("Load of a tombstone = %v, want ErrNotFound", err)
	}
	list, err := s.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 0 {
		t.Errorf("List = %v, want no puzzles", list)
	}

	// All is the one reader that sees them, because stats need them.
	games, err := s.All()
	if err != nil {
		t.Fatal(err)
	}
	if len(games) != 1 {
		t.Fatalf("All returned %d records, want the tombstone", len(games))
	}
	got := games[0]
	if !got.Deleted || got.ID != g.ID || got.Status != game.Won || got.Length != g.Length {
		t.Errorf("tombstone = %+v, want deleted %s/%d/%v", got, g.ID, g.Length, game.Won)
	}
	if !got.UpdatedAt.Equal(g.UpdatedAt) {
		t.Errorf("UpdatedAt = %v, want %v (the streak walk sorts on it)", got.UpdatedAt, g.UpdatedAt)
	}
	if got.Answer != "" || len(got.Guesses) != 0 || len(got.Marks) != 0 {
		t.Errorf("tombstone kept the play record: %+v", got)
	}

	// On disk it must say only what it means — a file full of empty answers and
	// null guesses reads as corruption, not as a marker.
	b, err := os.ReadFile(s.pathFor(g.ID))
	if err != nil {
		t.Fatal(err)
	}
	var raw map[string]any
	if err := json.Unmarshal(b, &raw); err != nil {
		t.Fatal(err)
	}
	want := []string{"id", "length", "status", "updatedAt", "deleted", "schema"}
	if len(raw) != len(want) {
		t.Errorf("tombstone file has %d fields (%v), want exactly %v", len(raw), raw, want)
	}
	for _, k := range want {
		if _, ok := raw[k]; !ok {
			t.Errorf("tombstone file is missing %q: %v", k, raw)
		}
	}
}

// A deleted daily keeps its date, because the daily streak walks the calendar
// and cannot otherwise tell the day from one never played. A casual puzzle's
// tombstone is unchanged by that — the key is omitempty (see the test above,
// which pins the field list).
func TestDeleteDailyKeepsItsDate(t *testing.T) {
	s := newStore(t)
	g := newGame(t, 5)
	g.Daily = "2026-08-06"
	g.Status = game.Lost
	if err := s.Save(g); err != nil {
		t.Fatal(err)
	}
	if err := s.Delete(g.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	games, err := s.All()
	if err != nil {
		t.Fatal(err)
	}
	if len(games) != 1 {
		t.Fatalf("All returned %d records, want the tombstone", len(games))
	}
	if got := games[0]; !got.Deleted || got.Daily != "2026-08-06" {
		t.Errorf("tombstone = %+v, want deleted with daily 2026-08-06", got)
	}

	b, err := os.ReadFile(s.pathFor(g.ID))
	if err != nil {
		t.Fatal(err)
	}
	var raw map[string]any
	if err := json.Unmarshal(b, &raw); err != nil {
		t.Fatal(err)
	}
	if raw["daily"] != "2026-08-06" {
		t.Errorf("tombstone file = %v, want a daily key", raw)
	}
}

func TestDeleteTombstoneReturnsNotFound(t *testing.T) {
	s := newStore(t)
	g := newGame(t, 5)
	g.Status = game.Lost
	if err := s.Save(g); err != nil {
		t.Fatal(err)
	}
	if err := s.Delete(g.ID); err != nil {
		t.Fatalf("first Delete: %v", err)
	}
	if err := s.Delete(g.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("second Delete = %v, want ErrNotFound", err)
	}
}

func TestDeleteMissingReturnsNotFound(t *testing.T) {
	s := newStore(t)
	if err := s.Delete("nope"); !errors.Is(err, ErrNotFound) {
		t.Errorf("Delete(missing) = %v, want ErrNotFound", err)
	}
}

// Installs made before puzzles carried their own codes still have a meta.json
// holding the retired counter. It must be ignored, not choked on.
func TestStaleMetaFileIsIgnored(t *testing.T) {
	dir := t.TempDir()
	s, err := NewJSON(dir)
	if err != nil {
		t.Fatal(err)
	}
	g := newGame(t, 5)
	if err := s.Save(g); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "meta.json"), []byte("garbage"), 0o644); err != nil {
		t.Fatal(err)
	}

	reopened, err := NewJSON(dir)
	if err != nil {
		t.Fatalf("NewJSON with a stale meta file: %v", err)
	}
	list, err := reopened.List()
	if err != nil || len(list) != 1 || list[0].ID != g.ID {
		t.Errorf("List = %v (err %v), want the one saved puzzle", list, err)
	}
}

// Old saves predate both the UUID and the code, and carry a "number" field
// that no longer exists. They must still load.
func TestLoadsPreCodeSaveFormat(t *testing.T) {
	s := newStore(t)
	const id = "7f3a1c0b9d2e4f56" // a pre-UUID id
	old := `{
		"id": "` + id + `",
		"number": 42,
		"length": 5,
		"answer": "crane",
		"guesses": [],
		"marks": [],
		"maxAttempts": 6,
		"status": "in_progress",
		"startedAt": "2025-01-01T00:00:00Z",
		"updatedAt": "2025-01-01T00:00:00Z",
		"elapsedMs": 0
	}`
	if err := os.WriteFile(s.pathFor(id), []byte(old), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := s.Load(id)
	if err != nil {
		t.Fatalf("Load of a pre-code save: %v", err)
	}
	if got.ID != id || got.Answer != "crane" {
		t.Errorf("Load returned %+v", got)
	}
	if code := game.Code(got.ID); len(code) != 6 {
		t.Errorf("Code(%q) = %q, want six digits", id, code)
	}
}
