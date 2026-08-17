package backup

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/nxck2005/surmise/internal/game"
	"github.com/nxck2005/surmise/internal/store"
	"github.com/nxck2005/surmise/internal/theme"
)

// The claim this package makes is that a file written on one machine restores
// on another without costing the player anything they already had. These tests
// hold it to the three halves of that: the file says what it is and reads back
// whole, a restore only ever adds, and the same bytes work in either store.

func newStore(t *testing.T) store.Store {
	t.Helper()
	return store.NewKV(store.NewMemoryKV())
}

// wonGame is a finished puzzle, which is the state a deletion turns into a
// tombstone rather than removing outright.
func wonGame(t *testing.T, answer string) *game.Game {
	t.Helper()
	g, err := game.New(len(answer))
	if err != nil {
		t.Fatalf("game.New: %v", err)
	}
	g.Answer = answer
	if err := g.Guess(answer); err != nil {
		t.Fatalf("Guess: %v", err)
	}
	if g.Status != game.Won {
		t.Fatalf("status = %v, want won", g.Status)
	}
	return g
}

func inProgress(t *testing.T, answer, guess string) *game.Game {
	t.Helper()
	g, err := game.New(len(answer))
	if err != nil {
		t.Fatalf("game.New: %v", err)
	}
	g.Answer = answer
	if err := g.Guess(guess); err != nil {
		t.Fatalf("Guess: %v", err)
	}
	return g
}

func buildFrom(t *testing.T, s store.Store, settings store.Settings, themes []theme.File) []byte {
	t.Helper()
	b, err := Build(s, settings, themes, "test", time.Date(2026, 8, 18, 12, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	return b
}

// The tag is a promise to every file already written. A rename that changed it
// would make this build refuse backups it wrote itself, which is precisely the
// history the package exists to protect. See the constant's comment.
func TestFormatTagIsFrozen(t *testing.T) {
	if Format != "surmise.backup" {
		t.Errorf("Format = %q; it is frozen, and changing it orphans every archive already written", Format)
	}
}

func TestBuildAndReadRoundTrip(t *testing.T) {
	s := newStore(t)
	won := wonGame(t, "crane")
	open := inProgress(t, "slate", "about")
	for _, g := range []*game.Game{won, open} {
		if err := s.Save(g); err != nil {
			t.Fatal(err)
		}
	}

	b := buildFrom(t, s, store.Settings{Theme: "dracula"}, []theme.File{{Name: "mine.toml", Body: "# mine\n"}})

	a, games, err := Read(b)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if a.Format != Format || a.Version != Version {
		t.Errorf("header = %s v%d, want %s v%d", a.Format, a.Version, Format, Version)
	}
	if a.App != "test" {
		t.Errorf("app = %q, want test", a.App)
	}
	if len(games) != 2 {
		t.Fatalf("read %d puzzles, want 2", len(games))
	}
	if a.Settings == nil || a.Settings.Theme != "dracula" {
		t.Errorf("settings = %+v, want the theme carried", a.Settings)
	}
	if len(a.Themes) != 1 || a.Themes[0].Body != "# mine\n" {
		t.Errorf("themes = %+v, want the one file carried verbatim", a.Themes)
	}

	byID := map[string]*game.Game{}
	for _, g := range games {
		byID[g.ID] = g
	}
	got, ok := byID[won.ID]
	if !ok {
		t.Fatalf("the won puzzle is not in the archive")
	}
	if got.Answer != won.Answer || len(got.Guesses) != len(won.Guesses) || got.Status != won.Status {
		t.Errorf("won puzzle came back as %+v, want the board intact", got)
	}
}

// Settings nobody has touched are left out rather than written as a block of
// empty strings, so an archive from a fresh install says "nothing chosen"
// instead of "everything chosen to be blank".
func TestBuildOmitsUntouchedSettings(t *testing.T) {
	b := buildFrom(t, newStore(t), store.Settings{}, nil)
	if strings.Contains(string(b), `"settings"`) {
		t.Errorf("an archive of an install with no preferences writes a settings section:\n%s", b)
	}
}

// Two exports of an unchanged history are the same bytes, which is what makes a
// backup something a player can diff or checksum.
func TestBuildIsDeterministic(t *testing.T) {
	s := newStore(t)
	for _, w := range []string{"crane", "slate", "adieu"} {
		if err := s.Save(wonGame(t, w)); err != nil {
			t.Fatal(err)
		}
	}

	first := buildFrom(t, s, store.Settings{}, nil)
	second := buildFrom(t, s, store.Settings{}, nil)
	if string(first) != string(second) {
		t.Errorf("two exports of the same history differ:\n%s\n---\n%s", first, second)
	}
}

// Tombstones are records. internal/stats reads them to tell a deleted day from
// a day never played, so an archive that dropped them would restore a history
// whose streaks were wrong.
func TestArchiveCarriesTombstones(t *testing.T) {
	s := newStore(t)
	g := wonGame(t, "crane")
	g.Daily = "2026-08-18"
	if err := s.Save(g); err != nil {
		t.Fatal(err)
	}
	if err := s.Delete(g.ID); err != nil {
		t.Fatal(err)
	}

	_, games, err := Read(buildFrom(t, s, store.Settings{}, nil))
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if len(games) != 1 {
		t.Fatalf("read %d records, want the tombstone", len(games))
	}
	if !games[0].Deleted {
		t.Errorf("the tombstone came back as a live puzzle: %+v", games[0])
	}
	if games[0].Daily != "2026-08-18" {
		t.Errorf("daily = %q, want the date kept — the streak walk needs it", games[0].Daily)
	}
	if games[0].Answer != "" {
		t.Errorf("the tombstone carries an answer it should have lost: %q", games[0].Answer)
	}
}

// A file that is not ours, or is from a build that knows more than this one, is
// refused whole. Half an archive is not something to write into a working
// install — see Read.
func TestReadRefusesWhatItCannotTrust(t *testing.T) {
	valid := buildFrom(t, newStore(t), store.Settings{}, nil)

	cases := []struct {
		name string
		body string
		want string
	}{
		{"not json", "this is not a backup", "not a surmise.backup file"},
		{"no format", `{"version":1,"puzzles":[]}`, "not a surmise.backup file"},
		{"another app's file", `{"format":"other.backup","version":1,"puzzles":[]}`, `"other.backup"`},
		{"no version", `{"format":"surmise.backup","puzzles":[]}`, "no version"},
		{"a newer format", `{"format":"surmise.backup","version":99,"puzzles":[]}`, "update the game"},
		{"a corrupt record", `{"format":"surmise.backup","version":1,"puzzles":[{"id":"x"}]}`, "record 1"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, _, err := Read([]byte(c.body))
			if err == nil {
				t.Fatalf("Read accepted %s", c.name)
			}
			if !strings.Contains(err.Error(), c.want) {
				t.Errorf("error = %v, want it to mention %q", err, c.want)
			}
		})
	}

	if _, _, err := Read(valid); err != nil {
		t.Errorf("Read refused a file this package wrote: %v", err)
	}
}

// Two records claiming the same puzzle mean the file was assembled by
// something other than Build, and there is no honest way to choose between
// them.
func TestReadRefusesARepeatedPuzzle(t *testing.T) {
	s := newStore(t)
	g := wonGame(t, "crane")
	if err := s.Save(g); err != nil {
		t.Fatal(err)
	}

	var a Archive
	if err := json.Unmarshal(buildFrom(t, s, store.Settings{}, nil), &a); err != nil {
		t.Fatal(err)
	}
	a.Puzzles = append(a.Puzzles, a.Puzzles[0])
	doubled, err := json.Marshal(a)
	if err != nil {
		t.Fatal(err)
	}

	if _, _, err := Read(doubled); err == nil || !strings.Contains(err.Error(), "not consistent") {
		t.Errorf("Read of a file repeating a puzzle = %v, want a refusal", err)
	}
}
