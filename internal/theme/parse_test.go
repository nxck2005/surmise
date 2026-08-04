package theme

import (
	"image/color"
	"strings"
	"testing"
)

// rgb flattens a colour so tests can compare values rather than
// implementations: lipgloss returns several concrete types depending on how a
// colour was written, and a theme only cares what it looks like.
func rgb(c color.Color) [3]uint32 {
	r, g, b, _ := c.RGBA()
	return [3]uint32{r >> 8, g >> 8, b >> 8}
}

func parse(t *testing.T, body string) *Theme {
	t.Helper()
	th, warns := Parse("test", []byte(body))
	if len(warns) > 0 {
		t.Fatalf("unexpected warnings: %v", warns)
	}
	return th
}

// A theme file only has to say what it changes; everything else comes from the
// default. This is what keeps sharing a theme a matter of a few lines.
func TestPartialThemeInheritsDefaults(t *testing.T) {
	th := parse(t, `
name = "tiny"
accent = "#ff0000"
`)

	if th.Name != "tiny" {
		t.Errorf("Name = %q", th.Name)
	}
	if got, want := rgb(th.Color(Accent)), rgb(mustColor("#ff0000")); got != want {
		t.Errorf("accent = %v, want %v", got, want)
	}
	if got, want := rgb(th.Color(Bg)), rgb(Default().Color(Bg)); got != want {
		t.Errorf("bg = %v, want the default %v", got, want)
	}
	if th.Glyphs.Caret != Default().Glyphs.Caret {
		t.Errorf("caret glyph = %q, want the default", th.Glyphs.Caret)
	}
}

// Parsing must not scribble on the built-in theme, which every later load is
// overlaid onto.
func TestParseDoesNotMutateDefault(t *testing.T) {
	before := rgb(Default().Color(Accent))
	parse(t, `accent = "#123456"`)
	if after := rgb(Default().Color(Accent)); after != before {
		t.Errorf("Default() accent changed to %v", after)
	}
}

// The text-on-tile colours follow the background unless the theme says
// otherwise, so a theme that only sets `bg` still reads correctly.
func TestDerivedColoursFollowTheirSource(t *testing.T) {
	th := parse(t, `bg = "#ffffff"`)
	if got, want := rgb(th.Color("correct_text")), rgb(mustColor("#ffffff")); got != want {
		t.Errorf("correct_text = %v, want to follow bg %v", got, want)
	}

	th = parse(t, `
bg = "#ffffff"
correct_text = "#010203"
`)
	if got, want := rgb(th.Color("correct_text")), rgb(mustColor("#010203")); got != want {
		t.Errorf("explicit correct_text = %v, want %v", got, want)
	}
}

// Sections are the readable form; the flat form is what people type. Both must
// mean the same thing.
func TestSectionAndFlatFormsAgree(t *testing.T) {
	flat := parse(t, `accent = "#00ff00"`)
	sectioned := parse(t, "[colors]\naccent = \"#00ff00\"\n")
	if rgb(flat.Color(Accent)) != rgb(sectioned.Color(Accent)) {
		t.Error("flat and [colors] forms disagree")
	}
}

func TestColourValueForms(t *testing.T) {
	th := parse(t, `
accent = "#fff"
muted = 8
error = "accent"
`)
	if got, want := rgb(th.Color(Accent)), [3]uint32{255, 255, 255}; got != want {
		t.Errorf("#fff = %v, want %v", got, want)
	}
	if rgb(th.Color(Error)) != rgb(th.Color(Accent)) {
		t.Error("palette reference did not resolve to accent")
	}
	// An ANSI number has no fixed RGB — it is whatever the terminal says — so
	// the assertion is only that it was accepted.
	if th.Color(Muted) == nil {
		t.Error("ANSI colour was dropped")
	}
}

func TestStyleOverrides(t *testing.T) {
	th := parse(t, `
[style.tile.correct]
bold = false
fg = "accent"

[style.title]
italic = true
`)

	o := th.Override("tile.correct")
	if o.Bold == nil || *o.Bold {
		t.Errorf("tile.correct bold = %v, want false", o.Bold)
	}
	if o.FG == nil || rgb(o.FG) != rgb(th.Color(Accent)) {
		t.Error("tile.correct fg did not resolve to accent")
	}
	if it := th.Override("title").Italic; it == nil || !*it {
		t.Error("title italic was not set")
	}
	// Untouched elements stay untouched, which is what makes overrides additive.
	if th.Override("muted") != (Override{}) {
		t.Error("muted picked up an override it never asked for")
	}
}

func TestGlyphsAndMetrics(t *testing.T) {
	th := parse(t, `
[glyphs]
border = "thick"
empty = "-"

[metrics]
tile_width = 5
`)
	if th.Glyphs.Border != "thick" || th.Glyphs.Empty != "-" {
		t.Errorf("glyphs = %+v", th.Glyphs)
	}
	if th.Metrics.TileWidth != 5 {
		t.Errorf("tile_width = %d, want 5", th.Metrics.TileWidth)
	}
	if th.Metrics.KeyPadX != Default().Metrics.KeyPadX {
		t.Error("an untouched metric changed")
	}
}

// Bad input is reported, never fatal: a theme with one broken line still loads,
// so a typo costs one colour instead of the whole file.
func TestBadLinesWarnAndAreSkipped(t *testing.T) {
	th, warns := Parse("test", []byte(`
accent = "#00ff00"
bg = "#zzz"
nonsense = "x"
[style.nope]
bold = true
[metrics]
tile_width = "wide"
this line has no equals sign
`))

	if len(warns) != 5 {
		t.Fatalf("got %d warnings, want 5: %v", len(warns), warns)
	}
	for _, w := range warns {
		if w.Line == 0 {
			t.Errorf("warning without a line number: %v", w)
		}
	}
	// The good line either side of the bad ones still took effect.
	if got, want := rgb(th.Color(Accent)), rgb(mustColor("#00ff00")); got != want {
		t.Errorf("accent = %v, want %v; a bad line stopped the parse", got, want)
	}
	if rgb(th.Color(Bg)) != rgb(Default().Color(Bg)) {
		t.Error("a rejected colour was applied anyway")
	}
}

func TestWarningNamesTheLine(t *testing.T) {
	_, warns := Parse("test", []byte("\n\nbg = \"#gg0000\"\n"))
	if len(warns) != 1 {
		t.Fatalf("warnings = %v", warns)
	}
	if warns[0].Line != 3 {
		t.Errorf("line = %d, want 3", warns[0].Line)
	}
	if !strings.Contains(warns[0].String(), "line 3") {
		t.Errorf("String() = %q, want it to name the line", warns[0].String())
	}
}

// A '#' opens a comment, but every colour starts with one. The parser has to
// tell them apart or no theme would load at all.
func TestCommentsDoNotEatColours(t *testing.T) {
	th := parse(t, `
# a leading comment
accent = "#abcdef"   # trailing comment
`)
	if got, want := rgb(th.Color(Accent)), rgb(mustColor("#abcdef")); got != want {
		t.Errorf("accent = %v, want %v", got, want)
	}
}

// Metrics feed straight into lipgloss widths, where a negative would panic.
func TestMetricsAreRangeChecked(t *testing.T) {
	_, warns := Parse("test", []byte("[metrics]\ntile_width = -3\n"))
	if len(warns) != 1 {
		t.Fatalf("warnings = %v, want one range complaint", warns)
	}
}
