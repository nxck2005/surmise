package store

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/nxck2005/surmise/internal/game"
)

// The behaviour every Store owes its callers, run against every implementation.
//
// JSON's own tests stay where they are: they reach for paths, file contents and
// a stale meta.json, which are facts about a filesystem rather than about a
// store. What is here is the contract the UI and internal/stats actually depend
// on, and the tombstone rules are most of it — a browser store that got those
// wrong would not crash, it would quietly report the wrong streaks.

// settingsStore mirrors the interface internal/ui looks for by type assertion.
type settingsCapable interface {
	Store
	Settings() Settings
	SaveSettings(Settings) error
}

type impl struct {
	name string
	open func(t *testing.T) settingsCapable
}

func implementations() []impl {
	return []impl{
		{"JSON", func(t *testing.T) settingsCapable {
			t.Helper()
			s, err := NewJSON(t.TempDir())
			if err != nil {
				t.Fatalf("NewJSON: %v", err)
			}
			return s
		}},
		{"KV", func(t *testing.T) settingsCapable {
			t.Helper()
			return NewKV(NewMemoryKV())
		}},
	}
}

// eachStore runs one case against every implementation.
func eachStore(t *testing.T, name string, fn func(t *testing.T, s settingsCapable)) {
	t.Helper()
	for _, im := range implementations() {
		t.Run(name+"/"+im.name, func(t *testing.T) { fn(t, im.open(t)) })
	}
}

// wonGame is a finished puzzle, which is the state that earns a tombstone.
func wonGame(t *testing.T) *game.Game {
	t.Helper()
	g := newGame(t, 5)
	g.Answer = "crane"
	if err := g.Guess("crane"); err != nil {
		t.Fatal(err)
	}
	if g.Status != game.Won {
		t.Fatalf("status = %v, want won", g.Status)
	}
	return g
}

func TestStoreRoundTrip(t *testing.T) {
	eachStore(t, "roundtrip", func(t *testing.T, s settingsCapable) {
		g := newGame(t, 5)
		g.Answer = "crane"
		if err := g.Guess("about"); err != nil {
			t.Fatal(err)
		}
		if err := s.Save(g); err != nil {
			t.Fatalf("Save: %v", err)
		}

		got, err := s.Load(g.ID)
		if err != nil {
			t.Fatalf("Load: %v", err)
		}
		if got.ID != g.ID || got.Answer != g.Answer || got.Length != g.Length {
			t.Errorf("Load = %+v, want %+v", got, g)
		}
		if len(got.Guesses) != len(g.Guesses) {
			t.Errorf("guesses = %v, want %v", got.Guesses, g.Guesses)
		}
	})
}

func TestStoreLoadMissing(t *testing.T) {
	eachStore(t, "missing", func(t *testing.T, s settingsCapable) {
		if _, err := s.Load("nope"); !errors.Is(err, ErrNotFound) {
			t.Errorf("Load of an unknown id = %v, want ErrNotFound", err)
		}
		if err := s.Delete("nope"); !errors.Is(err, ErrNotFound) {
			t.Errorf("Delete of an unknown id = %v, want ErrNotFound", err)
		}
	})
}

// An empty store is empty, not an error. A first run has nothing saved.
func TestStoreStartsEmpty(t *testing.T) {
	eachStore(t, "empty", func(t *testing.T, s settingsCapable) {
		list, err := s.List()
		if err != nil {
			t.Fatalf("List: %v", err)
		}
		if len(list) != 0 {
			t.Errorf("List = %v, want nothing", list)
		}
		games, err := s.All()
		if err != nil {
			t.Fatalf("All: %v", err)
		}
		if len(games) != 0 {
			t.Errorf("All = %v, want nothing", games)
		}
	})
}

func TestStoreListIsNewestFirst(t *testing.T) {
	eachStore(t, "order", func(t *testing.T, s settingsCapable) {
		var ids []string
		for i := range 3 {
			g := newGame(t, 5)
			// Space the timestamps so the order is a fact, not a coincidence.
			g.UpdatedAt = g.UpdatedAt.Add(time.Duration(i) * time.Second)
			if err := s.Save(g); err != nil {
				t.Fatal(err)
			}
			ids = append(ids, g.ID)
		}

		list, err := s.List()
		if err != nil {
			t.Fatal(err)
		}
		if len(list) != 3 {
			t.Fatalf("List returned %d, want 3", len(list))
		}
		for i, want := range []string{ids[2], ids[1], ids[0]} {
			if list[i].ID != want {
				t.Errorf("List[%d] = %s, want %s (newest first)", i, list[i].ID, want)
			}
		}
	})
}

// Deleting an unfinished puzzle leaves nothing at all: streaks ignore puzzles in
// progress, so a marker for one would record nothing.
func TestStoreDeleteUnfinishedLeavesNothing(t *testing.T) {
	eachStore(t, "delete-unfinished", func(t *testing.T, s settingsCapable) {
		g := newGame(t, 5)
		if err := s.Save(g); err != nil {
			t.Fatal(err)
		}
		if err := s.Delete(g.ID); err != nil {
			t.Fatalf("Delete: %v", err)
		}

		if _, err := s.Load(g.ID); !errors.Is(err, ErrNotFound) {
			t.Errorf("Load after Delete = %v, want ErrNotFound", err)
		}
		games, err := s.All()
		if err != nil {
			t.Fatal(err)
		}
		if len(games) != 0 {
			t.Errorf("All = %+v, want nothing: an unfinished delete leaves no marker", games)
		}
	})
}

// The rule the whole streak calculation rests on. Deleting a *finished* puzzle
// leaves a tombstone, so a deleted loss still breaks a run of wins.
func TestStoreDeleteFinishedLeavesTombstone(t *testing.T) {
	eachStore(t, "delete-finished", func(t *testing.T, s settingsCapable) {
		g := wonGame(t)
		if err := s.Save(g); err != nil {
			t.Fatal(err)
		}
		if err := s.Delete(g.ID); err != nil {
			t.Fatalf("Delete: %v", err)
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
			t.Errorf("tombstone = %+v, want deleted %s/%d/won", got, g.ID, g.Length)
		}
		if !got.UpdatedAt.Equal(g.UpdatedAt) {
			t.Errorf("UpdatedAt = %v, want %v (the streak walk sorts on it)", got.UpdatedAt, g.UpdatedAt)
		}
		if got.Answer != "" || len(got.Guesses) != 0 || len(got.Marks) != 0 {
			t.Errorf("tombstone kept the play record: %+v", got)
		}

		// Deleting it again is not a second deletion.
		if err := s.Delete(g.ID); !errors.Is(err, ErrNotFound) {
			t.Errorf("Delete of a tombstone = %v, want ErrNotFound", err)
		}
	})
}

// A deleted daily keeps its date. The daily streak walks the calendar and
// cannot otherwise tell a deleted day from one never played.
func TestStoreDeletedDailyKeepsItsDate(t *testing.T) {
	eachStore(t, "delete-daily", func(t *testing.T, s settingsCapable) {
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
			t.Fatalf("All returned %d, want the tombstone", len(games))
		}
		if games[0].Daily != "2026-08-06" {
			t.Errorf("deleted daily lost its date: %+v", games[0])
		}
	})
}

func TestStoreSettingsRoundTrip(t *testing.T) {
	eachStore(t, "settings", func(t *testing.T, s settingsCapable) {
		// Nothing saved yet is the zero value, not an error.
		if got := s.Settings(); got != (Settings{}) {
			t.Errorf("Settings before any save = %+v, want the zero value", got)
		}

		want := Settings{
			Theme: "nord", Length: 6, DisplayName: "nick",
			SplashDismiss: "key", SplashMillis: 1200,
		}
		if err := s.SaveSettings(want); err != nil {
			t.Fatalf("SaveSettings: %v", err)
		}
		if got := s.Settings(); got != want {
			t.Errorf("Settings = %+v, want %+v", got, want)
		}
	})
}

// Preferences and puzzles share a key space in the browser store. Neither may
// show up in the other's listing.
func TestStoreSettingsAreNotAPuzzle(t *testing.T) {
	eachStore(t, "settings-isolation", func(t *testing.T, s settingsCapable) {
		if err := s.SaveSettings(Settings{Theme: "dracula"}); err != nil {
			t.Fatal(err)
		}
		games, err := s.All()
		if err != nil {
			t.Fatal(err)
		}
		if len(games) != 0 {
			t.Errorf("All = %+v, want nothing: settings are not a puzzle", games)
		}
	})
}

// A write failure must surface rather than panic. A browser refuses a write
// when the origin's storage allowance is full, and that is ordinary.
func TestKVStoreReportsWriteFailure(t *testing.T) {
	s := NewKV(failingKV{NewMemoryKV()})
	if err := s.Save(newGame(t, 5)); err == nil {
		t.Error("Save on a full store returned no error")
	}
	if err := s.SaveSettings(Settings{Theme: "nord"}); err == nil {
		t.Error("SaveSettings on a full store returned no error")
	}
}

type failingKV struct{ KV }

func (failingKV) Set(string, string) error { return fmt.Errorf("quota exceeded") }

// The exact keys are part of the format. Pinning them here is what makes a
// later migration a decision rather than an accident.
func TestKVStoreKeyLayout(t *testing.T) {
	kv := NewMemoryKV()
	s := NewKV(kv)
	g := newGame(t, 5)
	if err := s.Save(g); err != nil {
		t.Fatal(err)
	}
	if err := s.SaveSettings(Settings{Theme: "nord"}); err != nil {
		t.Fatal(err)
	}

	for _, want := range []string{"surmise/v1/puzzle/" + g.ID, "surmise/v1/settings"} {
		if _, ok := kv.Get(want); !ok {
			t.Errorf("no value at %q; keys are %v", want, kv.Keys())
		}
	}
}

// Both stores hold the same bytes, so a history can move between them. That is
// what sharing one codec buys, and it is worth a test because the next store
// (a server) will want the same guarantee.
func TestStoresAgreeOnTheirBytes(t *testing.T) {
	g := wonGame(t)

	kv := NewMemoryKV()
	if err := NewKV(kv).Save(g); err != nil {
		t.Fatal(err)
	}
	fromKV, _ := kv.Get("surmise/v1/puzzle/" + g.ID)

	onDisk, err := encodeGame(g)
	if err != nil {
		t.Fatal(err)
	}
	if fromKV != string(onDisk) {
		t.Errorf("the two stores encode a puzzle differently:\nkv:   %s\nfile: %s", fromKV, onDisk)
	}
}

// TestStoreTombstoneKeepsTheCustomMarker holds both stores to the rule the
// streak walk depends on. A custom puzzle counts towards nothing, so its
// tombstone must still say so: one that read as an ordinary loss would break a
// run the live puzzle never touched, and deleting a puzzle that never counted
// would move the profile.
func TestStoreTombstoneKeepsTheCustomMarker(t *testing.T) {
	eachStore(t, "custom-tombstone", func(t *testing.T, s settingsCapable) {
		g := wonGame(t)
		g.Custom = true
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
		if got := games[0]; !got.Custom || got.CountsForStats() {
			t.Errorf("tombstone = %+v, want one that still counts for nothing", got)
		}
	})
}

// TestStoreSummaryCarriesTheCustomMarker: the browse list only ever sees a
// Summary, so without this it could not say what a puzzle is.
func TestStoreSummaryCarriesTheCustomMarker(t *testing.T) {
	eachStore(t, "custom-summary", func(t *testing.T, s settingsCapable) {
		g := wonGame(t)
		g.Custom = true
		if err := s.Save(g); err != nil {
			t.Fatal(err)
		}
		list, err := s.List()
		if err != nil {
			t.Fatal(err)
		}
		if len(list) != 1 || !list[0].Custom {
			t.Errorf("List = %+v, want one custom summary", list)
		}
	})
}
