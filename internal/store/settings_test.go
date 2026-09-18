package store

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSettingsRoundTrip(t *testing.T) {
	dir := t.TempDir()
	s, err := NewJSON(dir)
	if err != nil {
		t.Fatal(err)
	}

	if got := s.Settings(); got != (Settings{}) {
		t.Errorf("fresh store has settings %+v, want the zero value", got)
	}
	want := Settings{Theme: "nord", Length: 6, RememberLast: true, DisplayName: "nick", PlaytimeMS: 90_000}
	if err := s.SaveSettings(want); err != nil {
		t.Fatal(err)
	}
	// SaveSettings stamps the format tag on the way out.
	want.Schema = schemaVersion

	// Read through a new store, so the test proves it reached the disk.
	reopened, err := NewJSON(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got := reopened.Settings(); got != want {
		t.Errorf("Settings() = %+v, want %+v", got, want)
	}
}

// Every field's zero value has to be a working "nothing chosen": a settings
// file that predates a field must not read as an invalid choice.
func TestPartialSettingsKeepZeroValues(t *testing.T) {
	dir := t.TempDir()
	s, err := NewJSON(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, settingsName), []byte(`{"theme":"nord"}`), 0o644); err != nil {
		t.Fatal(err)
	}

	want := Settings{Theme: "nord"}
	if got := s.Settings(); got != want {
		t.Errorf("Settings() = %+v, want %+v", got, want)
	}
}

// Preferences are conveniences; losing them must never cost a puzzle, so a
// corrupt file reads as the defaults rather than an error.
func TestCorruptSettingsFallBackToDefaults(t *testing.T) {
	dir := t.TempDir()
	s, err := NewJSON(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, settingsName), []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}

	if got := s.Settings(); got != (Settings{}) {
		t.Errorf("Settings() = %+v, want the zero value", got)
	}
	// And it is recoverable: writing over it works.
	if err := s.SaveSettings(Settings{Theme: "dracula"}); err != nil {
		t.Fatal(err)
	}
	if got := s.Settings().Theme; got != "dracula" {
		t.Errorf("Theme = %q, want dracula", got)
	}
}

// Settings sit beside the puzzles, not among them, or they would show up as a
// corrupt game in the list.
func TestSettingsAreNotAPuzzle(t *testing.T) {
	dir := t.TempDir()
	s, err := NewJSON(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.SaveSettings(Settings{Theme: "nord"}); err != nil {
		t.Fatal(err)
	}

	list, err := s.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 0 {
		t.Errorf("List() = %d entries, want 0", len(list))
	}
}

// The store may not write a settings blob its own reader would refuse: the
// reader is bounded by MaxRecordBytes, so the writer is too. Without this an
// imported archive could make the app write a settings file it then cannot
// read, which is the same state-integrity failure the archive's own bounds
// exist to prevent.
func TestSaveSettingsRefusesAnOversizedBlob(t *testing.T) {
	dir := t.TempDir()
	s, err := NewJSON(dir)
	if err != nil {
		t.Fatal(err)
	}
	err = s.SaveSettings(Settings{DisplayName: strings.Repeat("a", MaxRecordBytes)})
	if err == nil {
		t.Fatal("SaveSettings wrote settings larger than MaxRecordBytes")
	}
	if !strings.Contains(err.Error(), "larger than") {
		t.Errorf("error = %v, want it to name the size bound", err)
	}
}

// A hand-edited counter past the duration's range is healed on read and
// saturated on write, so the conversion on the profile never overflows — and
// a value that wrapped negative cannot take the lifetime total with it.
func TestPlaytimeIsClampedToItsBound(t *testing.T) {
	dir := t.TempDir()
	s, err := NewJSON(dir)
	if err != nil {
		t.Fatal(err)
	}

	raw := fmt.Sprintf(`{"schema":%d,"playtime_ms":%d}`, schemaVersion, math.MaxInt64)
	if err := os.WriteFile(filepath.Join(dir, settingsName), []byte(raw), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := s.Settings().PlaytimeMS; got != MaxPlaytimeMS {
		t.Errorf("read playtime = %d, want it clamped to %d", got, MaxPlaytimeMS)
	}

	if err := s.SaveSettings(Settings{PlaytimeMS: MaxPlaytimeMS + 10}); err != nil {
		t.Fatal(err)
	}
	if got := s.Settings().PlaytimeMS; got != MaxPlaytimeMS {
		t.Errorf("written playtime = %d, want it saturated at %d", got, MaxPlaytimeMS)
	}

	if err := os.WriteFile(filepath.Join(dir, settingsName), []byte(`{"playtime_ms":-1}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := s.Settings().PlaytimeMS; got != 0 {
		t.Errorf("negative playtime = %d, want 0", got)
	}
}

// ValidateSettings is the import side of the same rules: it refuses what the
// store would not write, before an archive's preferences can be applied.
func TestValidateSettings(t *testing.T) {
	good := Settings{Theme: "nord", DisplayName: "nick", PlaytimeMS: 90_000}
	if err := ValidateSettings(good); err != nil {
		t.Errorf("ValidateSettings(%+v) = %v, want nil", good, err)
	}

	cases := []struct {
		name string
		v    Settings
	}{
		{"unknown schema", Settings{Schema: schemaVersion + 1}},
		{"negative playtime", Settings{PlaytimeMS: -1}},
		{"overflowing playtime", Settings{PlaytimeMS: math.MaxInt64}},
		{"oversized", Settings{DisplayName: strings.Repeat("a", MaxRecordBytes)}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if err := ValidateSettings(c.v); err == nil {
				t.Errorf("ValidateSettings accepted %s", c.name)
			}
		})
	}
}
