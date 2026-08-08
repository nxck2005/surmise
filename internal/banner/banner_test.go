package banner

import (
	"math/rand/v2"
	"sort"
	"testing"
	"unicode/utf8"

	"github.com/nxck2005/surmise/internal/brand"
)

func TestBundledArtLoads(t *testing.T) {
	all := List()
	if len(all) == 0 {
		t.Fatal("no bundled art")
	}
	for _, b := range all {
		if b.Name == "" {
			t.Error("banner with no name")
		}
		if b.Height == 0 || len(b.Lines) != b.Height {
			t.Errorf("%s: height %d, %d lines", b.Name, b.Height, len(b.Lines))
		}
		if b.Empty() {
			t.Errorf("%s: loaded but reports empty", b.Name)
		}
	}
}

// The splash centres itself on Width, which is a rune count. That is only the
// drawn width while the art stays ASCII — one wide or combining rune and the
// banner would sit off-centre, or overflow the panel it was measured to fit.
func TestArtIsASCII(t *testing.T) {
	for _, b := range List() {
		for i, line := range b.Lines {
			for _, r := range line {
				if r > utf8.RuneSelf || r == '\t' {
					t.Errorf("%s line %d: non-ASCII or tab %q", b.Name, i+1, r)
					break
				}
			}
		}
	}
}

// Width has to be the longest line, and no line may exceed it: those are the
// two halves of the measurement the splash's fits() check relies on.
func TestWidthMatchesTheLongestLine(t *testing.T) {
	for _, b := range List() {
		widest := 0
		for _, line := range b.Lines {
			widest = max(widest, utf8.RuneCountInString(line))
		}
		if b.Width != widest {
			t.Errorf("%s: Width %d, longest line %d", b.Name, b.Width, widest)
		}
	}
}

// Trailing space is invisible, so art carrying it would measure wider than it
// draws and the splash would centre it wrongly.
func TestLinesCarryNoTrailingSpace(t *testing.T) {
	for _, b := range List() {
		for i, line := range b.Lines {
			if line != "" && (line[len(line)-1] == ' ' || line[len(line)-1] == '\t') {
				t.Errorf("%s line %d: trailing whitespace", b.Name, i+1)
			}
		}
		if b.Lines[0] == "" || b.Lines[b.Height-1] == "" {
			t.Errorf("%s: blank line at an end", b.Name)
		}
	}
}

func TestListIsSortedAndStable(t *testing.T) {
	names := Names()
	if !sort.StringsAreSorted(names) {
		t.Errorf("names not sorted: %v", names)
	}
	for i, b := range List() {
		if b.Name != names[i] {
			t.Errorf("List and Names disagree at %d: %q vs %q", i, b.Name, names[i])
		}
	}
}

// The list is copied out, so a caller cannot scribble on the loaded art and
// change what every later caller sees.
func TestListIsACopy(t *testing.T) {
	first := List()
	first[0].Name = "scribbled"
	if List()[0].Name == "scribbled" {
		t.Error("List handed out the package's own slice")
	}
}

func TestGet(t *testing.T) {
	want := List()[0]
	got, ok := Get(want.Name)
	if !ok {
		t.Fatalf("Get(%q): not found", want.Name)
	}
	if got.Width != want.Width || got.Height != want.Height {
		t.Errorf("Get(%q) = %dx%d, want %dx%d", want.Name, got.Width, got.Height, want.Width, want.Height)
	}
	if _, ok := Get("no such banner"); ok {
		t.Error("Get reported a banner that does not exist")
	}
}

// A saved choice naming art that no longer ships must not be an error — the
// caller falls back to Default, which is why Get reports a bool rather than
// failing.
func TestDefaultIsTheProductsOwnArt(t *testing.T) {
	d := Default()
	if d.Empty() {
		t.Fatal("no default banner")
	}
	if _, ok := Get(brand.Name); ok && d.Name != brand.Name {
		t.Errorf("Default() = %q, want %q", d.Name, brand.Name)
	}
}

func TestRandomReturnsABundledBanner(t *testing.T) {
	known := map[string]bool{}
	for _, name := range Names() {
		known[name] = true
	}
	r := rand.New(rand.NewPCG(1, 2))
	for range 20 {
		if b := Random(r); !known[b.Name] {
			t.Fatalf("Random returned %q, which is not bundled", b.Name)
		}
	}
	if b := Random(nil); !known[b.Name] {
		t.Fatalf("Random(nil) returned %q, which is not bundled", b.Name)
	}
}
