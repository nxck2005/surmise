package backup

import (
	"bytes"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/nxck2005/surmise/internal/daily"
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

// wonDaily is a finished daily for a date, with the derived id the codec now
// requires of anything labeled as a daily.
func wonDaily(t *testing.T, date string) *game.Game {
	t.Helper()
	d, err := daily.ParseDay(date)
	if err != nil {
		t.Fatalf("ParseDay(%q): %v", date, err)
	}
	g, err := game.NewFrom(daily.ID(d, 5), "crane", 5)
	if err != nil {
		t.Fatalf("NewFrom: %v", err)
	}
	g.Daily = date
	if err := g.Guess("crane"); err != nil {
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

// staticStore hands Build a fixed history, for the tests that are about Build's
// own bounds rather than about a real store.
type staticStore struct{ games []*game.Game }

func (s staticStore) All() ([]*game.Game, error)      { return s.games, nil }
func (s staticStore) Load(string) (*game.Game, error) { return nil, store.ErrNotFound }
func (s staticStore) Save(*game.Game) error           { return nil }
func (s staticStore) Delete(string) error             { return store.ErrNotFound }
func (s staticStore) List() ([]store.Summary, error)  { return nil, nil }

func (s staticStore) IDs() ([]string, error) {
	ids := make([]string, 0, len(s.games))
	for _, g := range s.games {
		ids = append(ids, g.ID)
	}
	sort.Strings(ids)
	return ids, nil
}

// A backup this build writes has to be one it can read back. Writing a file
// whose own import refuses it is the failure the format exists to prevent, and
// the player finds out only when they need the file. Every export path — the
// flags and the screen — now gets a refusal naming the limit instead.
func TestBuildRefusesWhatReadWouldRefuse(t *testing.T) {
	at := time.Date(2026, 9, 21, 12, 0, 0, 0, time.UTC)

	t.Run("too many records", func(t *testing.T) {
		g := wonGame(t, "crane")
		many := make([]*game.Game, maxPuzzles+1)
		for i := range many {
			many[i] = g
		}
		_, err := Build(staticStore{games: many}, store.Settings{}, nil, "test", at)
		if err == nil || !strings.Contains(err.Error(), "more than a backup may hold (10000)") {
			t.Fatalf("Build = %v, want the record limit named", err)
		}
	})

	t.Run("too many themes", func(t *testing.T) {
		themes := make([]theme.File, maxThemes+1)
		_, err := Build(newStore(t), store.Settings{}, themes, "test", at)
		if err == nil || !strings.Contains(err.Error(), "more than a backup may hold (256)") {
			t.Fatalf("Build = %v, want the theme limit named", err)
		}
	})

	t.Run("oversized theme", func(t *testing.T) {
		themes := []theme.File{{Name: "big.toml", Body: strings.Repeat("a", theme.MaxFileBytes+1)}}
		_, err := Build(newStore(t), store.Settings{}, themes, "test", at)
		if err == nil || !strings.Contains(err.Error(), "larger than") {
			t.Fatalf("Build = %v, want the theme size named", err)
		}
	})

	t.Run("oversized settings", func(t *testing.T) {
		// Every free-text field is bounded now, so a settings section cannot
		// reach MaxRecordBytes by any route and the total-size cap has nothing
		// left to catch. What Build still owes the reader is the field bound,
		// and the refusal names the field rather than the blob.
		settings := store.Settings{DisplayName: strings.Repeat("a", store.MaxRecordBytes)}
		_, err := Build(newStore(t), settings, nil, "test", at)
		if err == nil || !strings.Contains(err.Error(), "display name is longer than") {
			t.Fatalf("Build = %v, want the field named", err)
		}
	})

	// A settings file already on disk is read, not validated. A player who
	// already has an over-long name is not refused their preferences and does
	// not lose their other settings; the UI's text field shortens the one field
	// when it draws. This is the same reason decodeSettings degrades to the
	// defaults instead of erroring, and the reason the bound is a refusal in
	// ValidateSettings — the import path — and nowhere else.
	//
	// The file is written by hand rather than through SaveSettings, because a
	// field over 128 bytes but under MaxRecordBytes is exactly the shape a
	// file can be in and the writer cannot produce: SaveSettings takes the
	// caller's value as it stands and encodeSettings holds the whole blob to
	// MaxRecordBytes, not a field to anything.
	t.Run("an over-long field on disk is read, not refused", func(t *testing.T) {
		dir := t.TempDir()
		js, err := store.NewJSON(dir)
		if err != nil {
			t.Fatal(err)
		}
		// Well past the field cap, well inside the blob cap: a hand-edited
		// settings file, and the one shape that matters here.
		long := store.Settings{DisplayName: strings.Repeat("d", 8*store.MaxSettingFieldBytes)}
		if err := store.ValidateSettings(long); err == nil {
			t.Fatal("the fixture is not actually out of bounds")
		}
		blob, err := json.Marshal(long)
		if err != nil {
			t.Fatal(err)
		}
		if len(blob) > store.MaxRecordBytes {
			t.Fatalf("the fixture is %d bytes, over the cap it is meant to sit under", len(blob))
		}
		if err := os.WriteFile(filepath.Join(dir, "settings.json"), blob, 0o600); err != nil {
			t.Fatal(err)
		}

		// The read path hands the value over as it stands; the repair is the
		// UI's, because the store does not know which cell a field is drawn in.
		// What matters here is that the player keeps their settings.
		got := js.Settings()
		if got.DisplayName == "" {
			t.Fatal("an over-long name cost the player their settings")
		}
		if got.Length != long.Length {
			t.Fatalf("the rest of the settings were lost: %+v", got)
		}
	})

	t.Run("a maximum settings section is nowhere near the cap", func(t *testing.T) {
		// The durable form of what the subtest above used to assert. There is
		// no longer any route to an oversized section, so the cap is only worth
		// keeping if the largest section this build can write is comfortably
		// inside it — which is what makes a backup Build writes readable.
		full := store.Settings{
			Theme:         strings.Repeat("t", store.MaxSettingFieldBytes),
			DisplayName:   strings.Repeat("d", store.MaxSettingFieldBytes),
			Splash:        strings.Repeat("s", store.MaxSettingFieldBytes),
			SplashArt:     strings.Repeat("a", store.MaxSettingFieldBytes),
			SplashDismiss: strings.Repeat("x", store.MaxSettingFieldBytes),
			Motion:        strings.Repeat("m", store.MaxSettingFieldBytes),
			Length:        6,
			RememberLast:  true,
			SplashMillis:  store.MaxSettingFieldBytes,
			PlaytimeMS:    store.MaxPlaytimeMS,
		}
		if err := store.ValidateSettings(full); err != nil {
			t.Fatalf("a maximum settings section = %v, want it accepted", err)
		}
		b, err := json.Marshal(full)
		if err != nil {
			t.Fatal(err)
		}
		// Six fields at their cap and the largest counter there is, and it is
		// well under a fiftieth of the blob cap. That headroom is the point:
		// nothing a player or an archive can put in these fields closes it.
		if len(b)*50 > store.MaxRecordBytes {
			t.Fatalf("a maximum settings section is %d bytes against a cap of %d; the blob cap is nearly reachable again",
				len(b), store.MaxRecordBytes)
		}
	})

	t.Run("oversized record", func(t *testing.T) {
		// A hand-edited pre-schema record can be valid on disk and grow when
		// the shared encoder stamps its schema. The board does not need to be
		// reachable through normal play for Build to owe the reader this cap.
		g := wonGame(t, "crane")
		g.Schema = 0
		for range 2_000 {
			g.Guesses = append(g.Guesses, "about")
			g.Marks = append(g.Marks, game.Score("about", g.Answer))
		}
		_, err := Build(staticStore{games: []*game.Game{g}}, store.Settings{}, nil, "test", at)
		if err == nil || !strings.Contains(err.Error(), "larger than") {
			t.Fatalf("Build = %v, want the record size named", err)
		}
	})
}

// The output cap is the reader's, so its boundary is inclusive exactly as
// Read's is: a file of exactly MaxArchiveBytes is read, one byte more is not.
func TestCheckSizeBoundary(t *testing.T) {
	if err := checkSize(MaxArchiveBytes); err != nil {
		t.Errorf("checkSize(at the limit) = %v, want nil", err)
	}
	if err := checkSize(MaxArchiveBytes + 1); err == nil {
		t.Error("checkSize(one past the limit) = nil, want a refusal")
	}
}

// Tombstones are records. internal/stats reads them to tell a deleted day from
// a day never played, so an archive that dropped them would restore a history
// whose streaks were wrong.
func TestArchiveCarriesTombstones(t *testing.T) {
	s := newStore(t)
	g := wonDaily(t, "2026-08-18")
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

// An archive is the one untrusted file this app invites in, so its shape is
// bounded before anything inside it is decoded.
func TestReadRefusesAnArchiveOverItsLimits(t *testing.T) {
	base := Archive{Format: Format, Version: Version}
	mk := func(a Archive) []byte {
		t.Helper()
		b, err := json.Marshal(a)
		if err != nil {
			t.Fatal(err)
		}
		return b
	}

	t.Run("archive bytes", func(t *testing.T) {
		if _, _, err := Read(make([]byte, MaxArchiveBytes+1)); err == nil {
			t.Error("Read accepted a file larger than MaxArchiveBytes")
		}
	})
	t.Run("too many records", func(t *testing.T) {
		a := base
		a.Puzzles = make([]json.RawMessage, maxPuzzles+1)
		_, _, err := Read(mk(a))
		if err == nil || !strings.Contains(err.Error(), "more than a backup may hold (10000)") {
			t.Errorf("Read of too many records = %v, want the record limit named", err)
		}
	})
	t.Run("oversized record", func(t *testing.T) {
		a := base
		a.Puzzles = []json.RawMessage{json.RawMessage(`"` + strings.Repeat("a", store.MaxRecordBytes) + `"`)}
		if _, _, err := Read(mk(a)); err == nil {
			t.Error("Read accepted a record larger than store.MaxRecordBytes")
		}
	})
	t.Run("too many themes", func(t *testing.T) {
		a := base
		a.Themes = make([]theme.File, maxThemes+1)
		_, _, err := Read(mk(a))
		if err == nil || !strings.Contains(err.Error(), "more than a backup may hold (256)") {
			t.Errorf("Read of too many themes = %v, want the theme limit named", err)
		}
	})
	t.Run("oversized theme", func(t *testing.T) {
		a := base
		a.Themes = []theme.File{{Name: "big.toml", Body: strings.Repeat("a", theme.MaxFileBytes+1)}}
		if _, _, err := Read(mk(a)); err == nil {
			t.Error("Read accepted a theme larger than theme.MaxFileBytes")
		}
	})
	t.Run("oversized settings", func(t *testing.T) {
		a := base
		a.Settings = &store.Settings{DisplayName: strings.Repeat("a", store.MaxRecordBytes)}
		if _, _, err := Read(mk(a)); err == nil || !strings.Contains(err.Error(), "settings") {
			t.Errorf("Read of oversized settings = %v, want a settings refusal", err)
		}
	})
	t.Run("playtime out of range", func(t *testing.T) {
		a := base
		a.Settings = &store.Settings{PlaytimeMS: math.MaxInt64}
		if _, _, err := Read(mk(a)); err == nil || !strings.Contains(err.Error(), "playtime") {
			t.Errorf("Read of an overflowing play counter = %v, want a settings refusal", err)
		}
	})
}

// The refusal above has to come from the element count, not from the array
// having been materialized first. An archive naming two hundred thousand empty
// puzzles is a megabyte of input and, decoded whole, allocates once per record
// — hundreds of times the size of the refusal, which stops at the cap. The
// assertion is a ratio rather than a constant so it does not depend on how
// many allocations the decoder happens to spend per element: doubling the
// array must not double the work.
func TestReadDoesNotAmplifyAHostileArray(t *testing.T) {
	body := func(n int) []byte {
		return []byte(`{"format":"surmise.backup","version":1,"puzzles":[` +
			strings.Repeat("null,", n-1) + `null]}`)
	}

	var err error
	allocs := func(n int) float64 {
		b := body(n)
		return testing.AllocsPerRun(3, func() {
			_, _, err = Read(b)
		})
	}
	small, large := allocs(100_000), allocs(200_000)
	if err == nil {
		t.Fatal("Read accepted an archive naming more records than a backup may hold")
	}
	if !strings.Contains(err.Error(), "more than a backup may hold") {
		t.Fatalf("error = %v, want the record limit named", err)
	}
	if large > small*1.5 {
		t.Errorf("%.0f allocations for 200k records against %.0f for 100k; the refusal must not scale with the array",
			large, small)
	}
}

// An id becomes a filename when an archive lands, so a record carrying
// anything but a plain token is refused with the rest of an untrustworthy
// file — before Apply is given the chance to write it.
func TestReadRefusesAnUnsafePuzzleID(t *testing.T) {
	g := wonGame(t, "crane")
	raw, err := store.EncodeRecord(g)
	if err != nil {
		t.Fatal(err)
	}
	id := []byte(`"id": "` + g.ID + `"`)
	for _, unsafe := range []string{"../settings", "../../outside", "./alias", "a/b", "/etc/settings", `c:\settings`, "with space", ".."} {
		quoted, err := json.Marshal(unsafe)
		if err != nil {
			t.Fatal(err)
		}
		patched := bytes.Replace(raw, id, append([]byte(`"id": `), quoted...), 1)
		if bytes.Equal(patched, raw) {
			t.Fatal("the record does not carry the id it was encoded with")
		}
		body, err := json.Marshal(Archive{
			Format:  Format,
			Version: Version,
			Puzzles: []json.RawMessage{patched},
		})
		if err != nil {
			t.Fatal(err)
		}
		if _, _, err := Read(body); err == nil {
			t.Errorf("Read accepted a record with id %q", unsafe)
		}
	}
}

// A word is what the board draws, tile by tile, so control bytes patched into
// an answer or a guess are refused with the rest of an untrustworthy archive —
// before Apply is given the chance to save one.
func TestReadRefusesWordsWithControlCharacters(t *testing.T) {
	g := wonGame(t, "crane")
	// A wrong guess, so the word being patched below is not also the answer:
	// each case has to reach its own check.
	g.Guesses[0] = "about"
	g.Marks = [][]game.Mark{game.Score("about", g.Answer)}
	raw, err := store.EncodeRecord(g)
	if err != nil {
		t.Fatal(err)
	}
	// \u001b is how JSON spells a control byte; the escape is the point, so
	// the patch has to stay valid JSON for the decoder to reach Validate.
	cases := []struct {
		name     string
		old, new []byte
	}{
		{"answer", []byte(`"answer": "crane"`), []byte(`"answer": "cr\u001bne"`)},
		{"guess", []byte(`"about"`), []byte(`"\u001b]0;x"`)},
	}
	for _, c := range cases {
		patched := bytes.Replace(raw, c.old, c.new, 1)
		if bytes.Equal(patched, raw) {
			t.Fatalf("the record does not carry %q", c.old)
		}
		body, err := json.Marshal(Archive{
			Format:  Format,
			Version: Version,
			Puzzles: []json.RawMessage{patched},
		})
		if err != nil {
			t.Fatal(err)
		}
		if _, _, err := Read(body); err == nil {
			t.Errorf("Read accepted a record with a control character in its %s", c.name)
		}
	}
}

// A daily's date and id are one fact; an imported record that carries a real
// daily's identity under a chosen date is refused with the rest of an
// untrustworthy archive. The store codec enforces it; this pins the import
// path the audit reached it through.
func TestReadRefusesADailyThatIsNotItsDate(t *testing.T) {
	d := daily.DayOf(time.Date(2026, 8, 6, 12, 0, 0, 0, time.UTC))
	g, err := game.NewFrom(daily.ID(d, 5), "crane", 5)
	if err != nil {
		t.Fatal(err)
	}
	g.Daily = d.String()
	raw, err := store.EncodeRecord(g)
	if err != nil {
		t.Fatal(err)
	}

	// Move the record to another valid id, leaving the date it claims behind.
	other, err := game.New(5)
	if err != nil {
		t.Fatal(err)
	}
	patched := bytes.Replace(raw, []byte(`"id": "`+g.ID+`"`), []byte(`"id": "`+other.ID+`"`), 1)
	if bytes.Equal(patched, raw) {
		t.Fatal("the record does not carry the id it was encoded with")
	}
	body, err := json.Marshal(Archive{
		Format:  Format,
		Version: Version,
		Puzzles: []json.RawMessage{patched},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := Read(body); err == nil || !strings.Contains(err.Error(), "does not match puzzle id") {
		t.Errorf("Read of a daily under another id = %v, want a refusal", err)
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
