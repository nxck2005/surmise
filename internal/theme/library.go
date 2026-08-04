package theme

import (
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
)

// The bundled themes are ordinary theme files, embedded rather than special-
// cased, so every one of them doubles as a worked example a user can copy into
// their own themes directory and edit.
//
//go:embed themes/*.toml
var bundled embed.FS

// DefaultName is the theme used when nothing else is chosen.
const DefaultName = "serika dark"

// dirName is the themes directory inside the data dir, beside `puzzles`.
const dirName = "themes"

// Entry is one theme offered to the player. A file that failed to load is still
// an entry: the picker shows it with its error, which is how a typo gets
// noticed instead of the theme silently vanishing.
type Entry struct {
	Name     string
	Source   string // "built-in", or the path it was read from
	Theme    *Theme // nil when Err is set
	Warnings []Warning
	Err      error
}

// Builtin reports whether the entry came from the binary rather than a file.
func (e Entry) Builtin() bool { return e.Source == "built-in" }

// Library is the set of themes available this run.
type Library struct {
	entries []Entry
}

// Dir returns the themes directory for a data dir. It follows -data, so a
// scratch profile gets a scratch set of themes.
func Dir(dataDir string) string { return filepath.Join(dataDir, dirName) }

// Open loads the bundled themes plus every *.toml in dir. A user theme whose
// name matches a bundled one replaces it, so a theme can be adjusted by copying
// it out and editing the copy.
func Open(dir string) *Library {
	l := &Library{}
	l.loadFS(bundled, "themes", "built-in")
	if dir != "" {
		l.loadDir(dir)
	}
	sort.Slice(l.entries, func(i, j int) bool { return l.entries[i].Name < l.entries[j].Name })
	return l
}

// Bundled returns only the themes compiled into the binary. Tests and the
// no-config path use it.
func Bundled() *Library { return Open("") }

func (l *Library) loadFS(fsys fs.FS, dir, source string) {
	entries, err := fs.ReadDir(fsys, dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		// embed.FS paths are always slash-separated, whatever the host OS.
		b, err := fs.ReadFile(fsys, path.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		l.add(themeFile(e.Name(), b, source))
	}
}

func (l *Library) loadDir(dir string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		// A missing themes directory is the normal case, not a problem.
		if !errors.Is(err, fs.ErrNotExist) {
			l.add(Entry{Name: dir, Source: dir, Err: err})
		}
		return
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".toml") {
			continue
		}
		path := filepath.Join(dir, e.Name())
		b, err := os.ReadFile(path)
		if err != nil {
			l.add(Entry{Name: fallbackName(e.Name()), Source: path, Err: err})
			continue
		}
		l.add(themeFile(e.Name(), b, path))
	}
}

func themeFile(filename string, data []byte, source string) Entry {
	t, warns := Parse(fallbackName(filename), data)
	return Entry{Name: t.Name, Source: source, Theme: t, Warnings: warns}
}

// fallbackName turns serika-dark.toml into "serika dark", so a theme file that
// omits `name` is still presentable.
func fallbackName(filename string) string {
	base := strings.TrimSuffix(filename, ".toml")
	return strings.ReplaceAll(base, "-", " ")
}

// add appends an entry, replacing any existing one with the same name so user
// files shadow bundled ones.
func (l *Library) add(e Entry) {
	for i, existing := range l.entries {
		if existing.Name == e.Name {
			l.entries[i] = e
			return
		}
	}
	l.entries = append(l.entries, e)
}

// Entries returns every theme, sorted by name.
func (l *Library) Entries() []Entry {
	if l == nil {
		return nil
	}
	return l.entries
}

// Get returns a theme by name.
func (l *Library) Get(name string) (*Theme, bool) {
	if l == nil {
		return nil, false
	}
	for _, e := range l.entries {
		if e.Name == name && e.Theme != nil {
			return e.Theme, true
		}
	}
	return nil, false
}

// Resolve picks the theme to start with, preferring name and falling back to
// the default. It reports whether name was honoured, so a mistyped -theme can
// be surfaced instead of silently ignored.
func (l *Library) Resolve(name string) (*Theme, bool) {
	if name != "" {
		if t, ok := l.Get(name); ok {
			return t, true
		}
	}
	if t, ok := l.Get(DefaultName); ok {
		return t, name == ""
	}
	return Default(), name == ""
}

// exampleName is dropped into an empty themes directory so the first thing a
// player finds there is a file they can copy.
const exampleName = "example.toml"

// EnsureDir creates the themes directory and, if it is empty, seeds it with a
// copy of the default theme. Writing your own theme then starts with a file
// that already exists rather than a blank page.
func EnsureDir(dir string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("theme: create %s: %w", dir, err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) > 0 {
		return nil
	}
	b, err := bundled.ReadFile("themes/serika-dark.toml")
	if err != nil {
		return nil
	}
	return os.WriteFile(filepath.Join(dir, exampleName), seedExample(b), 0o644)
}

// seedExample turns a bundled theme into a starting point. The name and author
// lines are dropped so the copy is called after its file — otherwise the
// example would shadow the built-in theme it was copied from, and editing it
// would look like the built-in theme had changed.
func seedExample(bundledTheme []byte) []byte {
	var b strings.Builder
	b.WriteString(exampleHeader)
	for _, line := range strings.Split(string(bundledTheme), "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "name") || strings.HasPrefix(trimmed, "author") {
			continue
		}
		b.WriteString(line)
		b.WriteByte('\n')
	}
	return []byte(b.String())
}

const exampleHeader = `# A wortle theme, named after its file. Change the colours below — or copy this
# file under a new name — and it shows up in the theme picker (esc → themes).
# Every key is optional: anything you leave out keeps its built-in value.
#
# Colours take #rrggbb, #rgb, an ANSI number 0-255, or the name of another
# palette entry. Add a ` + "`" + `name = "..."` + "`" + ` line to title it something other than the
# filename. See docs/THEMES.md for the full list of keys.

`
