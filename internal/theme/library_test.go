package theme

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// parseValue refuses control characters inside a theme file, but a theme's
// display name falls back to its *filename* and its Source is the path — neither
// of which the parser ever sees. A file named with an escape in it would
// otherwise reach the picker and -themes unfiltered.
//
// Driven through the helpers rather than through a real file on disk on purpose:
// Windows will not create a filename containing an escape, and CI runs there.
func TestFilenamesCannotCarryEscapes(t *testing.T) {
	got := fallbackName("evil\x1b]0;pwned\x07.toml")
	if strings.ContainsAny(got, "\x1b\x07") {
		t.Errorf("fallbackName kept a control character: %q", got)
	}
	if want := "evil�]0;pwned�"; got != want {
		t.Errorf("fallbackName = %q, want %q", got, want)
	}

	// An ordinary name is untouched, escapes being the whole of what changes.
	if got, want := fallbackName("rose-pine.toml"), "rose pine"; got != want {
		t.Errorf("fallbackName = %q, want %q", got, want)
	}
	// Including the non-ASCII a glyph or a name is legitimately made of.
	if got, want := safeText("café ▓ 👩‍🚀"), "café ▓ 👩‍🚀"; got != want {
		t.Errorf("safeText mangled ordinary text: %q, want %q", got, want)
	}

	// The path and the failure both quote the filename back, so both are
	// repaired too.
	if got := safeText("/themes/evil\x1b[2J.toml"); strings.ContainsRune(got, 0x1b) {
		t.Errorf("safeText kept an escape in a path: %q", got)
	}
	if got := safeErr(errors.New("open /themes/evil\x1b[2J.toml: no such file")); strings.ContainsRune(got.Error(), 0x1b) {
		t.Errorf("safeErr kept an escape: %q", got)
	}
	if safeErr(nil) != nil {
		t.Error("safeErr(nil) should stay nil")
	}
}

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

// Default() hardcodes the same palette that ember-dark.toml declares: one has
// to work with no files at all, the other has to be a copyable example. Nothing
// stops the two drifting except this test, and drift would mean the app looked
// different depending on whether the bundled file happened to load.
func TestDefaultMatchesItsBundledFile(t *testing.T) {
	raw, err := bundled.ReadFile(defaultFile)
	if err != nil {
		t.Fatalf("read %s: %v", defaultFile, err)
	}

	// Parse would overlay the file onto Default() and hide exactly the drift
	// this test is looking for, so read the declarations out of the text.
	declared := map[string]string{}
	for line := range strings.Lines(string(raw)) {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "[") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		declared[strings.TrimSpace(key)] = strings.Trim(strings.TrimSpace(value), `"`)
	}
	if len(declared) < 11 {
		t.Fatalf("%s declared only %d keys; the palette should be complete", defaultFile, len(declared))
	}

	code := Default()
	for key, hex := range declared {
		if key == "name" {
			if hex != code.Name {
				t.Errorf("name: file has %q, Default() has %q", hex, code.Name)
			}
			continue
		}
		if got, want := code.Color(key), mustColor(hex); got != want {
			t.Errorf("%s: file has %s, Default() has %v", key, hex, got)
		}
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
