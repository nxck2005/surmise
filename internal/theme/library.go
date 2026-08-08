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
	"unicode"

	"github.com/nxck2005/surmise/internal/brand"
)

// The bundled themes are ordinary theme files, embedded rather than special-
// cased, so every one of them doubles as a worked example a user can copy into
// their own themes directory and edit.
//
//go:embed themes/*.toml
var bundled embed.FS

// DefaultName is the theme used when nothing else is chosen. defaultFile is the
// bundled file it comes from; the two are kept together because the seeded
// example theme is a copy of that file.
const (
	DefaultName = "ember dark"
	defaultFile = "themes/ember-dark.toml"
)

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
//
// It remembers the directory it was opened from so that reloading is something
// the library can be asked for, rather than something a caller rebuilds a path
// to do. stamp is what that directory looked like at the time, for Changed.
type Library struct {
	entries []Entry
	dir     string // "" for the bundled-only set
	stamp   string
}

// Dir returns the themes directory for a data dir. It follows -data, so a
// scratch profile gets a scratch set of themes.
func Dir(dataDir string) string { return filepath.Join(dataDir, dirName) }

// Open loads the bundled themes plus every *.toml in dir. A user theme whose
// name matches a bundled one replaces it, so a theme can be adjusted by copying
// it out and editing the copy.
func Open(dir string) *Library {
	// The stamp is taken before the files are read, not after. A theme edited
	// in between then leaves a stamp that no longer matches the directory, so
	// the next Changed says yes and the edit is picked up on the following
	// reload. Stamping afterwards would record the new state against the old
	// contents, and that edit would never be seen again.
	l := &Library{dir: dir, stamp: Stamp(dir)}
	l.loadFS(bundled, "themes", "built-in")
	if dir != "" {
		l.loadDir(dir)
	}
	sort.Slice(l.entries, func(i, j int) bool { return l.entries[i].Name < l.entries[j].Name })
	return l
}

// Stamp summarises a themes directory without opening a single file: the name,
// size and modification time of every *.toml in it. Two stamps that differ mean
// the directory has moved on.
//
// This is how a change is noticed, rather than a filesystem watcher, because
// the dependency set is deliberately Charm-only and a directory read once a
// second costs nothing. The limit of the cheap version is real: an edit that
// keeps the byte count identical and lands within the modification time's
// resolution is invisible. That is what the picker's manual reload is for.
//
// An empty or unreadable directory stamps as "", which is what makes the
// bundled-only library permanently unchanged.
func Stamp(dir string) string {
	if dir == "" {
		return ""
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}
	var b strings.Builder
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".toml") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			// A file that vanished between the read and the stat is a change in
			// its own right; name it so the next stamp differs from this one.
			fmt.Fprintf(&b, "%s\x00gone\n", e.Name())
			continue
		}
		fmt.Fprintf(&b, "%s\x00%d\x00%d\n", e.Name(), info.Size(), info.ModTime().UnixNano())
	}
	return b.String()
}

// Dir reports the themes directory this library was opened from, "" for the
// bundled-only set.
func (l *Library) Dir() string {
	if l == nil {
		return ""
	}
	return l.dir
}

// Changed reports whether the themes directory has moved on since Open read it.
func (l *Library) Changed() bool {
	if l == nil || l.dir == "" {
		return false
	}
	return Stamp(l.dir) != l.stamp
}

// Reopen reads the same directory again. The result is a new library: the old
// one stays valid, so whatever is still holding it keeps a consistent set.
func (l *Library) Reopen() *Library {
	return Open(l.Dir())
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
			l.add(Entry{Name: safeText(dir), Source: safeText(dir), Err: safeErr(err)})
		}
		return
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".toml") {
			continue
		}
		// path opens the file; safeText(path) is what anyone gets to look at.
		path := filepath.Join(dir, e.Name())
		b, err := os.ReadFile(path)
		if err != nil {
			l.add(Entry{Name: fallbackName(e.Name()), Source: safeText(path), Err: safeErr(err)})
			continue
		}
		l.add(themeFile(e.Name(), b, safeText(path)))
	}
}

// safeErr rebuilds an error around its repaired message. The errors reaching an
// Entry are os.ReadDir's and os.ReadFile's, which quote the path they failed on
// and so carry a hostile filename into the picker with them. Nothing inspects
// Entry.Err beyond `!= nil`, so flattening the type here costs nothing.
func safeErr(err error) error {
	if err == nil {
		return nil
	}
	return errors.New(safeText(err.Error()))
}

func themeFile(filename string, data []byte, source string) Entry {
	t, warns := Parse(fallbackName(filename), data)
	return Entry{Name: t.Name, Source: source, Theme: t, Warnings: warns}
}

// fallbackName turns serika-dark.toml into "serika dark", so a theme file that
// omits `name` is still presentable.
func fallbackName(filename string) string {
	base := strings.TrimSuffix(safeText(filename), ".toml")
	return strings.ReplaceAll(base, "-", " ")
}

// safeText replaces control characters in text that reaches the terminal without
// having passed through parseValue: a theme's filename, the path it was read
// from, and the error from a file that would not open. Those are the way in that
// refusing control characters in *values* leaves open — a name is still rendered
// by the picker and by -themes whether or not the file inside it ever parsed.
//
// It repairs rather than refuses, unlike parseValue, because the two are not the
// same kind of input: a file's contents are the theme author's to correct, while
// a filename is the reader's, and a theme that works should not vanish over the
// name someone else gave it. U+FFFD rather than dropping the rune, so a tampered
// name looks wrong instead of looking fine.
//
// Only Source and the display name are repaired, never the path used to open the
// file — loadDir keeps the real one for that.
func safeText(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return '�'
		}
		return r
	}, s)
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
	b, err := bundled.ReadFile(defaultFile)
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
	b.WriteString(exampleHeader())
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

// exampleHeader is a function rather than a const because it names the product,
// which lives in one place (internal/brand) so that renaming stays cheap.
func exampleHeader() string {
	return `# A ` + brand.Name + ` theme, named after its file. Change the colours below — or copy this
# file under a new name — and it shows up in the theme picker (esc → themes).
# Every key is optional: anything you leave out keeps its built-in value.
#
# Colours take #rrggbb, #rgb, an ANSI number 0-255, or the name of another
# palette entry. Add a ` + "`" + `name = "..."` + "`" + ` line to title it something other than the
# filename. See docs/THEMES.md for the full list of keys.

`
}
