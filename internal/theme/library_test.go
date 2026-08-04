package theme

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The bundled themes are the worked examples users copy, so they have to be
// exemplary: no warnings, a name, and a full palette. This is the test that
// makes "copy a built-in theme and edit it" safe advice.
func TestBundledThemesAreClean(t *testing.T) {
	entries := Bundled().Entries()
	if len(entries) < 10 {
		t.Fatalf("only %d bundled themes; the picker is meant to ship a library", len(entries))
	}

	for _, e := range entries {
		if e.Err != nil {
			t.Errorf("%s: %v", e.Source, e.Err)
			continue
		}
		if len(e.Warnings) > 0 {
			t.Errorf("%s: %v", e.Name, e.Warnings)
		}
		if e.Name == "" || strings.Contains(e.Name, ".toml") {
			t.Errorf("%s: unnamed theme", e.Source)
		}
		for _, key := range baseKeys {
			if _, ok := e.Theme.palette[key]; !ok {
				t.Errorf("%s: does not set %s; a bundled theme should be a complete example", e.Name, key)
			}
		}
	}
}

func TestDefaultThemeIsBundled(t *testing.T) {
	if _, ok := Bundled().Get(DefaultName); !ok {
		t.Fatalf("no bundled theme named %q", DefaultName)
	}
}

func TestUserThemesShadowBundled(t *testing.T) {
	dir := t.TempDir()
	body := "name = \"" + DefaultName + "\"\naccent = \"#ff00ff\"\n"
	if err := os.WriteFile(filepath.Join(dir, "mine.toml"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}

	lib := Open(dir)
	th, ok := lib.Get(DefaultName)
	if !ok {
		t.Fatal("theme disappeared")
	}
	if got, want := rgb(th.Color(Accent)), rgb(mustColor("#ff00ff")); got != want {
		t.Errorf("accent = %v, want the user's %v", got, want)
	}

	// Shadowing replaces rather than duplicates, or the picker would show the
	// same name twice.
	count := 0
	for _, e := range lib.Entries() {
		if e.Name == DefaultName {
			count++
		}
	}
	if count != 1 {
		t.Errorf("%q appears %d times", DefaultName, count)
	}
}

// A broken theme file is still listed, with its warnings, so the author can see
// what went wrong instead of watching it vanish.
func TestBrokenUserThemeIsListedWithWarnings(t *testing.T) {
	dir := t.TempDir()
	body := "name = \"broken\"\nbg = \"#nothex\"\n"
	if err := os.WriteFile(filepath.Join(dir, "broken.toml"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}

	for _, e := range Open(dir).Entries() {
		if e.Name != "broken" {
			continue
		}
		if len(e.Warnings) == 0 {
			t.Fatal("broken theme reported no warnings")
		}
		if e.Theme == nil {
			t.Fatal("broken theme should still be usable, minus the bad line")
		}
		return
	}
	t.Fatal("broken theme was not listed")
}

func TestOpenIgnoresNonThemeFiles(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	if len(Open(dir).Entries()) != len(Bundled().Entries()) {
		t.Error("a non-.toml file was loaded as a theme")
	}
}

func TestResolveFallsBackAndReports(t *testing.T) {
	lib := Bundled()

	if _, ok := lib.Resolve(""); !ok {
		t.Error("empty name should resolve quietly to the default")
	}
	th, ok := lib.Resolve("no such theme")
	if ok {
		t.Error("a missing theme should be reported, not silently swapped")
	}
	if th.Name != DefaultName {
		t.Errorf("fell back to %q, want %q", th.Name, DefaultName)
	}
}

// EnsureDir seeds an empty themes directory, so the first thing a would-be
// theme author finds is a file to copy rather than nothing at all.
func TestEnsureDirSeedsAnExample(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "themes")
	if err := EnsureDir(dir); err != nil {
		t.Fatal(err)
	}

	b, err := os.ReadFile(filepath.Join(dir, exampleName))
	if err != nil {
		t.Fatalf("no example written: %v", err)
	}
	if _, warns := Parse("example", b); len(warns) > 0 {
		t.Errorf("the seeded example does not parse cleanly: %v", warns)
	}

	// A second run must not overwrite whatever the player has since put there.
	if err := os.WriteFile(filepath.Join(dir, exampleName), []byte("name = \"edited\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := EnsureDir(dir); err != nil {
		t.Fatal(err)
	}
	b, _ = os.ReadFile(filepath.Join(dir, exampleName))
	if !strings.Contains(string(b), "edited") {
		t.Error("EnsureDir clobbered an existing file")
	}
}
