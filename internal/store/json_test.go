package store

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/nxck2005/wortle/internal/game"
)

func newStore(t *testing.T) *JSON {
	t.Helper()
	s, err := NewJSON(t.TempDir())
	if err != nil {
		t.Fatalf("NewJSON: %v", err)
	}
	return s
}

func newGame(t *testing.T, s *JSON, length int) *game.Game {
	t.Helper()
	n, err := s.NextNumber()
	if err != nil {
		t.Fatalf("NextNumber: %v", err)
	}
	g, err := game.New(length, n)
	if err != nil {
		t.Fatalf("game.New: %v", err)
	}
	return g
}

// The core persistence guarantee: a puzzle interrupted mid-game comes back
// with its board intact.
func TestSaveLoadRoundTrip(t *testing.T) {
	s := newStore(t)
	g := newGame(t, s, 5)
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
	if got.Answer != g.Answer || got.Number != g.Number || got.Length != g.Length {
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

func TestNextNumberIncrementsAndSurvivesReopen(t *testing.T) {
	dir := t.TempDir()
	s, err := NewJSON(dir)
	if err != nil {
		t.Fatal(err)
	}
	for want := 1; want <= 3; want++ {
		got, err := s.NextNumber()
		if err != nil {
			t.Fatal(err)
		}
		if got != want {
			t.Fatalf("NextNumber = %d, want %d", got, want)
		}
	}

	// A fresh process must not reissue numbers.
	reopened, err := NewJSON(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := reopened.NextNumber(); got != 4 {
		t.Errorf("after reopen NextNumber = %d, want 4", got)
	}
}

func TestListIsNewestFirst(t *testing.T) {
	s := newStore(t)
	for _, n := range []int{4, 5, 6} {
		if err := s.Save(newGame(t, s, n)); err != nil {
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
	good := newGame(t, s, 5)
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
	g := newGame(t, s, 5)
	g.Answer = "toolongforfive"
	if err := s.Save(g); err == nil {
		t.Error("Save accepted an invalid game")
	}
}

// A damaged meta file must not prevent play; the counter rebuilds from disk.
func TestCorruptMetaRebuildsFromPuzzles(t *testing.T) {
	dir := t.TempDir()
	s, err := NewJSON(dir)
	if err != nil {
		t.Fatal(err)
	}
	g := newGame(t, s, 5) // takes number 1
	g.Number = 9
	if err := s.Save(g); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, metaName), []byte("garbage"), 0o644); err != nil {
		t.Fatal(err)
	}

	reopened, err := NewJSON(dir)
	if err != nil {
		t.Fatalf("NewJSON with corrupt meta: %v", err)
	}
	got, err := reopened.NextNumber()
	if err != nil {
		t.Fatal(err)
	}
	if got != 10 {
		t.Errorf("NextNumber after corrupt meta = %d, want 10", got)
	}
}
