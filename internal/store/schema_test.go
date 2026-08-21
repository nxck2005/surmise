package store

import (
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
		ID: "test-id", Length: 5, Answer: "crane",
		Guesses: []string{"crane"}, Marks: [][]game.Mark{{2, 2, 2, 2, 2}},
		MaxAttempts: 6, Status: game.Won,
		StartedAt: time.Unix(0, 0).UTC(), UpdatedAt: time.Unix(0, 0).UTC(),
	}
	b, err := EncodeRecord(g)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), `"schema": 1`) {
		t.Errorf("puzzle record carries no schema tag: %s", b)
	}

	g.Deleted = true
	b, err = EncodeRecord(g)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), `"schema": 1`) {
		t.Errorf("tombstone carries no schema tag: %s", b)
	}

	b, err = encodeSettings(Settings{Theme: "nord"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), `"schema": 1`) {
		t.Errorf("settings carry no schema tag: %s", b)
	}
}

func TestRecordRoundTripKeepsSchema(t *testing.T) {
	g := &game.Game{
		ID: "test-id", Length: 5, Answer: "crane",
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
