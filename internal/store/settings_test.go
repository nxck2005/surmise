package store

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSettingsRoundTrip(t *testing.T) {
	dir := t.TempDir()
	s, err := NewJSON(dir)
	if err != nil {
		t.Fatal(err)
	}

	if got := s.Settings(); got.Theme != "" {
		t.Errorf("fresh store has settings %+v, want the zero value", got)
	}
	if err := s.SaveSettings(Settings{Theme: "nord"}); err != nil {
		t.Fatal(err)
	}

	// Read through a new store, so the test proves it reached the disk.
	reopened, err := NewJSON(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got := reopened.Settings().Theme; got != "nord" {
		t.Errorf("Theme = %q, want nord", got)
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
