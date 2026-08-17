package backup

import (
	"testing"
	"time"

	"github.com/nxck2005/surmise/internal/game"
	"github.com/nxck2005/surmise/internal/store"
	"github.com/nxck2005/surmise/internal/theme"
)

// ids returns what a store holds, tombstones included, as a set.
func ids(t *testing.T, s store.Store) map[string]*game.Game {
	t.Helper()
	games, err := s.All()
	if err != nil {
		t.Fatal(err)
	}
	out := make(map[string]*game.Game, len(games))
	for _, g := range games {
		out[g.ID] = g
	}
	return out
}

// The whole point: a history restores into an empty install intact.
func TestApplyRestoresIntoAnEmptyInstall(t *testing.T) {
	from := newStore(t)
	won := wonGame(t, "crane")
	open := inProgress(t, "slate", "about")
	for _, g := range []*game.Game{won, open} {
		if err := from.Save(g); err != nil {
			t.Fatal(err)
		}
	}
	b := buildFrom(t, from, store.Settings{Theme: "dracula", PlaytimeMS: 60_000}, nil)

	to := newStore(t)
	res, err := Apply(b, to, store.Settings{})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if res.PuzzlesAdded != 2 || res.PuzzlesKept != 0 {
		t.Errorf("added %d and kept %d, want 2 and 0", res.PuzzlesAdded, res.PuzzlesKept)
	}
	if res.Settings.Theme != "dracula" {
		t.Errorf("theme = %q, want the archive's answer to a question nobody had answered", res.Settings.Theme)
	}
	if res.PlaytimeAdded != time.Minute {
		t.Errorf("playtime gained = %v, want a minute", res.PlaytimeAdded)
	}

	got := ids(t, to)
	if len(got) != 2 {
		t.Fatalf("restored %d puzzles, want 2", len(got))
	}
	if g := got[won.ID]; g == nil || g.Answer != "crane" || g.Status != game.Won {
		t.Errorf("the won puzzle restored as %+v, want the board intact", g)
	}
}

// The rule the package is built on: an archive may only ever add. A puzzle the
// player already has is theirs, whatever the file says about it.
func TestApplyNeverOverwritesAPuzzle(t *testing.T) {
	g := wonGame(t, "crane")

	from := newStore(t)
	stale := *g
	stale.Answer = "slate"
	if err := from.Save(&stale); err != nil {
		t.Fatal(err)
	}
	b := buildFrom(t, from, store.Settings{}, nil)

	to := newStore(t)
	if err := to.Save(g); err != nil {
		t.Fatal(err)
	}

	res, err := Apply(b, to, store.Settings{})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if res.PuzzlesAdded != 0 || res.PuzzlesKept != 1 {
		t.Errorf("added %d and kept %d, want 0 and 1", res.PuzzlesAdded, res.PuzzlesKept)
	}
	if got := ids(t, to)[g.ID]; got.Answer != "crane" {
		t.Errorf("answer = %q, want the local copy left alone", got.Answer)
	}
}

// A tombstone in an archive must not delete a puzzle the player still holds.
// This is the sharpest edge of "only ever adds": the record is a deletion, and
// applying it as one would destroy live history.
func TestApplyDoesNotDeleteWithATombstone(t *testing.T) {
	g := wonGame(t, "crane")

	from := newStore(t)
	if err := from.Save(g); err != nil {
		t.Fatal(err)
	}
	if err := from.Delete(g.ID); err != nil {
		t.Fatal(err)
	}
	b := buildFrom(t, from, store.Settings{}, nil)

	to := newStore(t)
	if err := to.Save(g); err != nil {
		t.Fatal(err)
	}

	if _, err := Apply(b, to, store.Settings{}); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	got := ids(t, to)[g.ID]
	if got == nil || got.Deleted {
		t.Errorf("a tombstone in the archive deleted a puzzle the player still had: %+v", got)
	}
	if got.Answer != "crane" {
		t.Errorf("answer = %q, want the live puzzle untouched", got.Answer)
	}
}

// A tombstone the install does not have is still a record worth restoring, and
// it has to land as the marker rather than as a puzzle spelled out empty.
func TestApplyRestoresATombstoneAsAMarker(t *testing.T) {
	from := newStore(t)
	g := wonGame(t, "crane")
	if err := from.Save(g); err != nil {
		t.Fatal(err)
	}
	if err := from.Delete(g.ID); err != nil {
		t.Fatal(err)
	}
	b := buildFrom(t, from, store.Settings{}, nil)

	to := newStore(t)
	res, err := Apply(b, to, store.Settings{})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if res.PuzzlesAdded != 1 {
		t.Errorf("added %d, want the tombstone restored", res.PuzzlesAdded)
	}
	got := ids(t, to)[g.ID]
	if got == nil || !got.Deleted {
		t.Fatalf("the tombstone did not restore as a tombstone: %+v", got)
	}
	// A restored tombstone must also stay unopenable.
	if _, err := to.Load(g.ID); err == nil {
		t.Error("a restored tombstone can be resumed")
	}
}

// Importing the same file twice changes nothing the second time, which is what
// makes a mistaken import free.
func TestApplyIsIdempotent(t *testing.T) {
	from := newStore(t)
	for _, w := range []string{"crane", "slate"} {
		if err := from.Save(wonGame(t, w)); err != nil {
			t.Fatal(err)
		}
	}
	b := buildFrom(t, from, store.Settings{Theme: "dracula", PlaytimeMS: 1000}, nil)

	to := newStore(t)
	first, err := Apply(b, to, store.Settings{})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	second, err := Apply(b, to, first.Settings)
	if err != nil {
		t.Fatalf("second Apply: %v", err)
	}
	if second.PuzzlesAdded != 0 || second.PuzzlesKept != first.PuzzlesAdded {
		t.Errorf("second import added %d and kept %d, want 0 and %d",
			second.PuzzlesAdded, second.PuzzlesKept, first.PuzzlesAdded)
	}
	if len(second.SettingsFilled) != 0 || second.PlaytimeAdded != 0 {
		t.Errorf("second import changed preferences: %v, +%v", second.SettingsFilled, second.PlaytimeAdded)
	}
	if second.Any() {
		t.Errorf("second import reports that it did something: %+v", second)
	}
}

func TestApplyMergesSettings(t *testing.T) {
	archived := store.Settings{
		Theme:        "dracula",
		DisplayName:  "them",
		Length:       6,
		Motion:       "off",
		RememberLast: true,
		PlaytimeMS:   90_000,
	}
	b := buildFrom(t, newStore(t), archived, nil)

	// A player who has chosen a theme and a length keeps both, and gains only
	// what they had not chosen.
	current := store.Settings{Theme: "gruvbox", Length: 4, PlaytimeMS: 30_000}
	res, err := Apply(b, newStore(t), current)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}

	got := res.Settings
	if got.Theme != "gruvbox" || got.Length != 4 {
		t.Errorf("chosen preferences were overwritten: theme %q, length %d", got.Theme, got.Length)
	}
	if got.DisplayName != "them" || got.Motion != "off" || !got.RememberLast {
		t.Errorf("unset preferences were not filled in: %+v", got)
	}
	if got.PlaytimeMS != 90_000 || res.PlaytimeAdded != 60*time.Second {
		t.Errorf("playtime = %dms (+%v), want it raised to 90000ms", got.PlaytimeMS, res.PlaytimeAdded)
	}
	if len(res.SettingsFilled) != 3 {
		t.Errorf("filled = %v, want the three unset preferences named", res.SettingsFilled)
	}
}

// The counter never falls. An archive from a machine that has played less than
// this one must not lower the lifetime total — see store.Settings.PlaytimeMS.
func TestApplyNeverLowersThePlayCounter(t *testing.T) {
	b := buildFrom(t, newStore(t), store.Settings{PlaytimeMS: 1_000}, nil)

	res, err := Apply(b, newStore(t), store.Settings{PlaytimeMS: 500_000})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if res.Settings.PlaytimeMS != 500_000 || res.PlaytimeAdded != 0 {
		t.Errorf("playtime = %dms (+%v), want the larger figure kept",
			res.Settings.PlaytimeMS, res.PlaytimeAdded)
	}
}

// Themes are handed back rather than written: Apply does no file I/O, because
// the browser has no theme directory to write into.
func TestApplyHandsBackThemesRatherThanWritingThem(t *testing.T) {
	files := []theme.File{{Name: "mine.toml", Body: "# mine\n"}}
	b := buildFrom(t, newStore(t), store.Settings{}, files)

	res, err := Apply(b, newStore(t), store.Settings{})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if len(res.Themes) != 1 || res.Themes[0].Name != "mine.toml" || res.Themes[0].Body != "# mine\n" {
		t.Errorf("themes = %+v, want the file carried through verbatim", res.Themes)
	}
}

// The claim that makes this worth building: the same file moves a history
// between a desktop install and a browser one. Both stores share the codec, so
// an archive is a copy rather than a conversion.
func TestArchiveMovesBetweenStores(t *testing.T) {
	native, err := store.NewJSON(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	won := wonGame(t, "crane")
	gone := wonGame(t, "slate")
	for _, g := range []*game.Game{won, gone} {
		if err := native.Save(g); err != nil {
			t.Fatal(err)
		}
	}
	if err := native.Delete(gone.ID); err != nil {
		t.Fatal(err)
	}

	browser := store.NewKV(store.NewMemoryKV())
	if _, err := Apply(buildFrom(t, native, store.Settings{}, nil), browser, store.Settings{}); err != nil {
		t.Fatalf("into the browser store: %v", err)
	}

	got := ids(t, browser)
	if len(got) != 2 {
		t.Fatalf("the browser store holds %d records, want the puzzle and the tombstone", len(got))
	}
	if g := got[won.ID]; g == nil || g.Answer != "crane" {
		t.Errorf("the puzzle did not survive the move: %+v", g)
	}
	if g := got[gone.ID]; g == nil || !g.Deleted {
		t.Errorf("the tombstone did not survive the move: %+v", g)
	}

	// And back again, into a fresh native install.
	back, err := store.NewJSON(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Apply(buildFrom(t, browser, store.Settings{}, nil), back, store.Settings{}); err != nil {
		t.Fatalf("back into a native store: %v", err)
	}
	if len(ids(t, back)) != 2 {
		t.Errorf("the round trip lost records: %+v", ids(t, back))
	}
}
