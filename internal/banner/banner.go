// Package banner provides the ASCII art the splash screen draws, compiled into
// the binary.
//
// Art is data, the way themes and word lists are data: one .txt file per
// banner under art/, embedded at build time and read by name. Adding a banner
// is dropping in a file — no Go changes, no list to keep in step — which is the
// whole point, since the set is expected to grow and a player picks between
// them in the settings screen.
//
// The one caveat: a banner spells the product's name out in glyphs, so this is
// the single place the name is *drawn* rather than read from internal/brand.
// A rename means drawing new art; brand.Name only decides which file is the
// default. (internal/daily must not import this package, for the same reason it
// must not import brand.)
package banner

import (
	"embed"
	"io/fs"
	"math/rand/v2"
	"path"
	"sort"
	"strings"
	"sync"
	"unicode/utf8"

	"github.com/nxck2005/surmise/internal/brand"
)

//go:embed art/*.txt
var data embed.FS

const artDir = "art"

// Banner is one piece of art, measured when it is loaded.
//
// Width is a rune count, which is honest only because the art is ASCII (a test
// enforces that). Anything wider than one cell per rune would make the measured
// width disagree with the drawn one, and the splash centres itself on it.
type Banner struct {
	Name   string   // the file name without its extension
	Lines  []string // trailing whitespace stripped, leading space preserved
	Width  int      // the longest line, in cells
	Height int      // len(Lines)
}

// Empty reports whether b is the zero Banner, which is what a caller gets when
// there is no art to give it. Bad or missing art is never fatal — it just means
// no splash — so callers check this rather than an error.
func (b Banner) Empty() bool { return b.Height == 0 }

var (
	once   sync.Once
	loaded []Banner // sorted by name
)

// load parses the embedded art on first use. A file that reads as nothing is
// skipped rather than reported: decoration must never cost anyone a launch.
func load() {
	entries, err := fs.ReadDir(data, artDir)
	if err != nil {
		return
	}
	for _, e := range entries {
		b, err := read(e.Name())
		if err != nil || b.Empty() {
			continue
		}
		loaded = append(loaded, b)
	}
	sort.Slice(loaded, func(i, j int) bool { return loaded[i].Name < loaded[j].Name })
}

func read(file string) (Banner, error) {
	raw, err := data.ReadFile(path.Join(artDir, file))
	if err != nil {
		return Banner{}, err
	}

	b := Banner{Name: strings.TrimSuffix(file, path.Ext(file))}
	for _, line := range strings.Split(strings.ReplaceAll(string(raw), "\r\n", "\n"), "\n") {
		// Only the right-hand side is trimmed: leading space is what positions
		// a glyph, while trailing space is invisible padding that would make
		// the art measure wider than it draws.
		line = strings.TrimRight(line, " \t")
		b.Lines = append(b.Lines, line)
		b.Width = max(b.Width, utf8.RuneCountInString(line))
	}

	// Blank lines at either end are an artefact of how the file was saved, not
	// part of the drawing, and each one costs a row on a short terminal.
	for len(b.Lines) > 0 && b.Lines[0] == "" {
		b.Lines = b.Lines[1:]
	}
	for len(b.Lines) > 0 && b.Lines[len(b.Lines)-1] == "" {
		b.Lines = b.Lines[:len(b.Lines)-1]
	}
	b.Height = len(b.Lines)
	return b, nil
}

// List returns every bundled banner, ordered by name. The order is stable
// because it is what the settings screen cycles through: a player's saved
// choice is a name, but the sequence they step past should not shuffle between
// builds.
func List() []Banner {
	once.Do(load)
	out := make([]Banner, len(loaded))
	copy(out, loaded)
	return out
}

// Names returns the bundled banners' names, in the same order as List.
func Names() []string {
	once.Do(load)
	out := make([]string, len(loaded))
	for i, b := range loaded {
		out[i] = b.Name
	}
	return out
}

// Get returns the named banner. A name that no longer exists — art dropped
// between releases, with the choice still in settings.json — reports false, and
// callers fall back to Default rather than treating it as an error.
func Get(name string) (Banner, bool) {
	once.Do(load)
	for _, b := range loaded {
		if b.Name == name {
			return b, true
		}
	}
	return Banner{}, false
}

// Default is the art shown when nothing has been chosen: the one named after
// the product, or failing that the first there is. Resolving it through
// brand.Name rather than a literal keeps the default pointing at the right file
// if the product is ever renamed and the art redrawn under the new name.
func Default() Banner {
	if b, ok := Get(brand.Name); ok {
		return b
	}
	if all := List(); len(all) > 0 {
		return all[0]
	}
	return Banner{}
}

// Random returns one of the bundled banners. A nil source uses the package
// global, which is what the app does; tests pass their own to pin the choice.
func Random(r *rand.Rand) Banner {
	all := List()
	if len(all) == 0 {
		return Banner{}
	}
	if r != nil {
		return all[r.IntN(len(all))]
	}
	return all[rand.IntN(len(all))]
}
