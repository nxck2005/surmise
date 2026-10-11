package store

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/nxck2005/surmise/internal/game"
)

// The save-format promise, pinned: a record written before the schema tag
// existed decodes exactly as it always did, a record written now says so, and
// a record claiming a version this build does not know is refused rather than
// half-understood. The compatibility rule itself lives in docs/UPGRADING.md.

func TestLegacyRecordStillDecodes(t *testing.T) {
	b, err := os.ReadFile("testdata/legacy-puzzle.json")
	if err != nil {
		t.Fatal(err)
	}
	g, err := DecodeRecord("puzzle from v0.4.0", b)
	if err != nil {
		t.Fatalf("a pre-schema record must stay readable forever: %v", err)
	}
	if g.ID != "3f2a7c1e-8b4d-4c6a-9e2f-1d5b8a7c3e90" ||
		g.Length != 5 || g.Answer != "crane" || g.Status != game.Won ||
		g.MaxAttempts != 6 || g.ElapsedMS != 695_000 {
		t.Errorf("legacy record decoded wrong: %+v", g)
	}
	if len(g.Guesses) != 2 || g.Guesses[1] != "crane" ||
		g.Marks[0][4] != game.Correct || g.Marks[1][0] != game.Correct {
		t.Errorf("legacy marks decoded wrong: %v %v", g.Guesses, g.Marks)
	}
	if g.Schema != 0 {
		t.Errorf("legacy record should read as pre-schema, got %d", g.Schema)
	}
}

func TestLegacySettingsStillDecode(t *testing.T) {
	b, err := os.ReadFile("testdata/legacy-settings.json")
	if err != nil {
		t.Fatal(err)
	}
	got := decodeSettings(b)
	want := Settings{
		Theme: "nord", DisplayName: "nick", Length: 6, RememberLast: true,
		SplashDismiss: "key", SplashMillis: 1200, PlaytimeMS: 90_000,
	}
	if got != want {
		t.Errorf("legacy settings = %+v, want %+v", got, want)
	}
}

// Every write leaves this build's tag on the bytes: puzzles, tombstones and
// settings alike. This is the guarantee that a future reader can tell what it
// is looking at.
func TestSchemaStampedOnEveryWrite(t *testing.T) {
	g := &game.Game{
		ID: "5b8d0e2f-1a3c-4d57-8e9f-0a1b2c3d4e5f", Length: 5, Answer: "crane",
		Guesses: []string{"crane"}, Marks: [][]game.Mark{{2, 2, 2, 2, 2}},
		MaxAttempts: 6, Status: game.Won,
		StartedAt: time.Unix(0, 0).UTC(), UpdatedAt: time.Unix(0, 0).UTC(),
	}
	b, err := EncodeRecord(g)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), `"schema":1`) {
		t.Errorf("puzzle record carries no schema tag: %s", b)
	}

	g.Deleted = true
	b, err = EncodeRecord(g)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), `"schema":1`) {
		t.Errorf("tombstone carries no schema tag: %s", b)
	}

	b, err = encodeSettings(Settings{Theme: "nord"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), `"schema":1`) {
		t.Errorf("settings carry no schema tag: %s", b)
	}
}

func TestRecordRoundTripKeepsSchema(t *testing.T) {
	g := &game.Game{
		ID: "7c9e1d3a-2b4f-4a58-9d6e-0f1a2b3c4d5e", Length: 5, Answer: "crane",
		Guesses: []string{"crane"}, Marks: [][]game.Mark{{2, 2, 2, 2, 2}},
		MaxAttempts: 6, Status: game.Won,
		StartedAt: time.Unix(0, 0).UTC(), UpdatedAt: time.Unix(0, 0).UTC(),
	}
	b, err := EncodeRecord(g)
	if err != nil {
		t.Fatal(err)
	}
	back, err := DecodeRecord("round trip", b)
	if err != nil {
		t.Fatal(err)
	}
	if back.Schema != schemaVersion {
		t.Errorf("round-tripped Schema = %d, want %d", back.Schema, schemaVersion)
	}
}

func TestUnknownSchemaIsRefused(t *testing.T) {
	for _, bad := range []int{schemaVersion + 1, -1} {
		raw, err := json.Marshal(map[string]any{
			"id": "x", "length": 5, "answer": "crane",
			"guesses": []string{"crane"}, "marks": [][]int{{2, 2, 2, 2, 2}},
			"maxAttempts": 6, "status": "won",
			"startedAt": "2026-08-15T09:30:00Z", "updatedAt": "2026-08-15T09:41:35Z",
			"elapsedMs": 1000, "schema": bad,
		})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := DecodeRecord("puzzle x", raw); err == nil {
			t.Errorf("schema %d was accepted; want refusal", bad)
		} else if !strings.Contains(err.Error(), "schema version mismatch") {
			t.Errorf("schema %d refused with the wrong words: %v", bad, err)
		}
	}
}

func TestUnknownSettingsSchemaFallsBackToDefaults(t *testing.T) {
	// A puzzle record from an unknown future is refused; settings from one
	// degrade to the defaults, because losing a preference must never cost a
	// puzzle and there is no error path here to spend.
	if got := decodeSettings([]byte(`{"theme":"nord","schema":99}`)); got != (Settings{}) {
		t.Errorf("settings with unknown schema = %+v, want the defaults", got)
	}
}

// Records and settings are written compact, which is what fits a full backup
// into a browser's storage, and an indented record — every file written before
// v0.7.0, and the legacy fixture — still reads. Whitespace is not part of the
// format.
func TestRecordsAreWrittenCompact(t *testing.T) {
	b, err := os.ReadFile("testdata/legacy-puzzle.json")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), "\n  ") {
		t.Fatal("the legacy fixture is no longer indented, so it pins nothing here")
	}
	g, err := DecodeRecord("legacy puzzle", b)
	if err != nil {
		t.Fatalf("an indented record no longer reads: %v", err)
	}

	tomb := *g.Tombstone()
	settings, err := encodeSettings(Settings{Theme: "nord", DisplayName: "nick"})
	if err != nil {
		t.Fatal(err)
	}
	for name, enc := range map[string]func() ([]byte, error){
		"puzzle":    func() ([]byte, error) { return EncodeRecord(g) },
		"tombstone": func() ([]byte, error) { return EncodeRecord(&tomb) },
		"settings":  func() ([]byte, error) { return settings, nil },
	} {
		out, err := enc()
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		var compact bytes.Buffer
		if err := json.Compact(&compact, out); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if compact.String() != string(out) {
			t.Errorf("%s is not written compact:\n%s", name, out)
		}
	}
}

// fixtureRoomRoundRecord is the room-round fixture's record as the code that
// wrote it sees it: a valid finished round written by this build.
func TestRoomRoundFixtureIsACurrentRecord(t *testing.T) {
	b, err := os.ReadFile("testdata/room-round.json")
	if err != nil {
		t.Fatal(err)
	}
	g, err := decodeRecord("puzzle from testdata/room-round.json", b)
	if err != nil {
		t.Fatalf("the room-round fixture does not decode: %v", err)
	}
	if g.Room == nil || g.Room.Code != "K7M2QX" || g.Room.Seat != 2 || !g.Status.Done() {
		t.Errorf("fixture round = %+v, want a finished room round", g)
	}
	if g.ID != "73144140-eaf5-8a2d-bb31-8f8cc2421824" {
		t.Errorf("fixture id = %s, want the pinned RoomRoundID", g.ID)
	}
}

// TestARoomRoundReadsAsAPuzzleWithoutTheKey is what an OLDER version sees when
// it opens today's file: it declares the record shape of Game before the Room
// field existed — every other field, and no room key — and finds a finished
// puzzle with its guesses, because the Room section is simply unknown JSON to
// such a reader.
type gameWithoutRoom struct {
	ID          string              `json:"id"`
	Length      int                 `json:"length"`
	Answer      string              `json:"answer"`
	Guesses     []string            `json:"guesses"`
	Marks       [][]game.Mark       `json:"marks"`
	MaxAttempts int                 `json:"maxAttempts"`
	Status      game.Status         `json:"status"`
	StartedAt   time.Time           `json:"startedAt"`
	UpdatedAt   time.Time           `json:"updatedAt"`
	ElapsedMS   int64               `json:"elapsedMs"`
	Daily       string              `json:"daily,omitempty"`
	Deleted     bool                `json:"deleted,omitempty"`
	Custom      bool                `json:"custom,omitempty"`
	Challenge   *game.ChallengeInfo `json:"challenge,omitempty"`
	Schema      int                 `json:"schema"`
}

func TestARoomRoundReadsAsAPuzzleWithoutTheKey(t *testing.T) {
	b, err := os.ReadFile("testdata/room-round.json")
	if err != nil {
		t.Fatal(err)
	}
	var old gameWithoutRoom
	if err := json.Unmarshal(b, &old); err != nil {
		t.Fatal(err)
	}
	if old.Schema != 1 {
		t.Errorf("schema = %d, want 1", old.Schema)
	}
	if old.Status != game.Won || len(old.Guesses) != 2 || old.Guesses[1] != "crane" {
		t.Errorf("older reader saw %d guesses, status %v", len(old.Guesses), old.Status)
	}
	if old.Challenge != nil || old.Daily != "" || old.Custom {
		t.Errorf("older reader saw an origin that is not on the record: %+v", old)
	}
}
