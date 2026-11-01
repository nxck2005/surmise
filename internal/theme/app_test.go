package theme

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func TestAppSectionsParseWithoutWarnings(t *testing.T) {
	th, warns := Parse("x", []byte(`
[app]
pair  = "other"
fonts = "geist"

[app.colors]
card   = "#112233"
action = "accent"
`))
	if len(warns) > 0 {
		t.Fatalf("warnings: %v", warns)
	}
	if th.App.Pair != "other" || th.App.Fonts != FontsGeist {
		t.Errorf("app = %+v", th.App)
	}
	if got := hex(th.App.Colors["card"]); got != "#112233" {
		t.Errorf("card = %s", got)
	}
	if got, want := hex(th.App.Colors["action"]), hex(th.Color(Accent)); got != want {
		t.Errorf("action = %s, want the accent %s", got, want)
	}
}

func TestBadAppKeysWarn(t *testing.T) {
	for _, line := range []string{
		"[app]\nwut = 1",
		"[app.colors]\nwut = \"#000000\"",
		"[app]\nfonts = \"comic\"",
		"[app.colors]\ncard = 3",
		"[app.colors]\ncard = \"#nothex\"",
		"[app]\npair = \"" + strings.Repeat("x", maxFieldBytes+1) + "\"",
	} {
		_, warns := Parse("x", []byte(line))
		if len(warns) != 1 {
			t.Errorf("%q: %d warnings, want 1: %v", line, len(warns), warns)
		}
	}
}

// The terminal must draw a theme with [app] sections exactly as it draws the
// same theme without them.
func TestAppSectionsDoNotChangeTheTerminalTheme(t *testing.T) {
	plain := "[colors]\nbg = \"#101010\"\naccent = \"#ff0000\"\n"
	a, _ := Parse("x", []byte(plain))
	b, _ := Parse("x", []byte(plain+"[app]\npair = \"y\"\n[app.colors]\nbg = \"#ffffff\"\ncanvas = \"#ffffff\"\n"))
	for _, k := range append(baseKeys, "correct_text", "absent_text") {
		if hex(a.Color(k)) != hex(b.Color(k)) {
			t.Errorf("%s changed: %s vs %s", k, hex(a.Color(k)), hex(b.Color(k)))
		}
	}
	if !reflect.DeepEqual(a.Glyphs, b.Glyphs) || a.Metrics != b.Metrics || len(a.Styles) != len(b.Styles) {
		t.Error("glyphs, metrics or styles changed")
	}
}

func TestAppDerivesFromThePalette(t *testing.T) {
	th, _ := Parse("x", []byte(`
bg      = "#000000"
text    = "#ffffff"
accent  = "#4c8dff"
muted   = "#808080"
error   = "#ff0000"
correct = "#00aa00"
present = "#ddbb00"
absent  = "#333333"
`))
	a := App(th, TilesStandard)
	checks := []struct{ name, got, want string }{
		{"canvas", a.Canvas, "#000000"},
		{"card", a.Card, hex(mix(th.Color(Bg), th.Color(Text), cardStep))},
		{"sheet", a.Sheet, hex(mix(th.Color(Bg), th.Color(Text), sheetStep))},
		{"ink", a.Ink, "#ffffff"},
		{"inkDim", a.InkDim, "#c0c0c0"},
		{"accent", a.Accent, "#4c8dff"},
		{"action", a.Action, "#4c8dff"},
		{"onAction", a.OnAction, "#000000"},
		{"danger", a.Danger, "#ff0000"},
		{"cursor", a.Cursor, "#4c8dff"},
		{"correct", a.Tiles.Correct.Fill, "#00aa00"},
		{"correct letter", a.Tiles.Correct.Letter, "#000000"},
		{"absent letter", a.Tiles.Absent.Letter, "#ffffff"},
		{"glowFrom", a.GlowFrom, "#4c8dff"},
		{"glowTo", a.GlowTo, "#ddbb00"},
		{"tabTint", a.TabTint, a.Card},
		{"fonts", a.Fonts, DefaultFonts},
	}
	for _, c := range checks {
		if c.got != c.want {
			t.Errorf("%s = %s, want %s", c.name, c.got, c.want)
		}
	}
	if !a.Dark {
		t.Error("a black ground is not dark")
	}
}

func TestAppValuesInTheFileWin(t *testing.T) {
	th, _ := Parse("x", []byte(`
bg = "#000000"
[app.colors]
card = "#123456"
action = "#ff8a3d"
on_action = "#060709"
`))
	a := App(th, TilesStandard)
	if a.Card != "#123456" || a.Action != "#ff8a3d" || a.OnAction != "#060709" {
		t.Errorf("card %s action %s onAction %s", a.Card, a.Action, a.OnAction)
	}
}

// Derived text colours are repaired up to the floor; a colour the file set is
// left as written, so the contrast test can name the file to fix.
func TestAppRepairsOnlyDerivedColours(t *testing.T) {
	low := `
bg    = "#202020"
text  = "#303030"
muted = "#282828"
`
	th, _ := Parse("x", []byte(low))
	a := App(th, TilesStandard)
	for name, c := range map[string]string{"ink": a.Ink, "inkDim": a.InkDim, "inkMuted": a.InkMuted} {
		for _, g := range []string{a.Canvas, a.Card} {
			if r := Contrast(mustColor(c), mustColor(g)); r < floorText {
				t.Errorf("%s %s on %s is %.2f:1 after repair", name, c, g, r)
			}
		}
	}

	th, _ = Parse("x", []byte(low+"[app.colors]\nink = \"#303030\"\n"))
	if got := App(th, TilesStandard).Ink; got != "#303030" {
		t.Errorf("an explicit ink was repaired to %s", got)
	}
}

func TestANSIOnly(t *testing.T) {
	cases := map[string]bool{
		`bg = "#000000"`:              false,
		`accent = 3`:                  true,
		"bg = 0\ntext = \"bg\"":       true,
		"bg = 0\nbg = \"#000000\"":    false,
		`correct_text = 0`:            false, // not a base key
		"[colors]\nkey_spent = 255\n": true,
	}
	for src, want := range cases {
		th, warns := Parse("x", []byte(src))
		if len(warns) > 0 {
			t.Fatalf("%q: %v", src, warns)
		}
		if got := th.ANSIOnly(); got != want {
			t.Errorf("%q: ANSIOnly = %v, want %v", src, got, want)
		}
	}
	if th, _ := Bundled().Get("terminal"); !th.ANSIOnly() {
		t.Error("the terminal theme is not ANSI-only")
	}
	if Default().ANSIOnly() {
		t.Error("the default theme is ANSI-only")
	}
}

// appThemes is every bundled theme the app lists.
func appThemes(t *testing.T) []Entry {
	var out []Entry
	for _, e := range Bundled().Entries() {
		if e.Theme != nil && !e.Theme.ANSIOnly() {
			out = append(out, e)
		}
	}
	if len(out) < 14 {
		t.Fatalf("only %d app themes", len(out))
	}
	return out
}

// Every theme the app lists, in every tile palette, has to be readable. A theme
// that fails gets [app.colors] values in its file, never an exception here.
func TestBundledAppThemesMeetTheFloors(t *testing.T) {
	const apart = 1.15 // the three tile fills must differ in lightness
	for _, e := range appThemes(t) {
		for _, p := range TilePalettes {
			a := App(e.Theme, p)
			c := mustColor
			pairs := []struct {
				what   string
				fg, bg string
				floor  float64
			}{
				{"ink on canvas", a.Ink, a.Canvas, floorText},
				{"ink on card", a.Ink, a.Card, floorText},
				{"ink on sheet", a.Ink, a.Sheet, floorText},
				{"inkDim on canvas", a.InkDim, a.Canvas, floorText},
				{"inkDim on card", a.InkDim, a.Card, floorText},
				{"inkDim on sheet", a.InkDim, a.Sheet, floorText},
				{"inkMuted on canvas", a.InkMuted, a.Canvas, floorText},
				{"inkMuted on card", a.InkMuted, a.Card, floorText},
				{"accentText on canvas", a.AccentText, a.Canvas, floorText},
				{"onAction on action", a.OnAction, a.Action, floorText},
				{"keyText on keyFace", a.KeyText, a.KeyFace, floorLetter},
				{"correct letter", a.Tiles.Correct.Letter, a.Tiles.Correct.Fill, floorLetter},
				{"present letter", a.Tiles.Present.Letter, a.Tiles.Present.Fill, floorLetter},
				{"absent letter", a.Tiles.Absent.Letter, a.Tiles.Absent.Fill, floorLetter},
				{"correct vs present", a.Tiles.Correct.Fill, a.Tiles.Present.Fill, apart},
				{"correct vs absent", a.Tiles.Correct.Fill, a.Tiles.Absent.Fill, apart},
				{"present vs absent", a.Tiles.Present.Fill, a.Tiles.Absent.Fill, apart},
			}
			for _, x := range pairs {
				if r := Contrast(c(x.fg), c(x.bg)); r < x.floor {
					t.Errorf("%s / tiles %q: %s is %.2f:1 (%s on %s), want %.1f:1", e.Name, p, x.what, r, x.fg, x.bg, x.floor)
				}
			}
		}
	}
}

func TestAccessibleTilesFollowTheGround(t *testing.T) {
	lib := Bundled()
	night, _ := lib.Get("night")
	day, _ := lib.Get("day")
	for _, p := range []TilePalette{TilesColorblind, TilesHighContrast} {
		if got := App(night, p).Tiles; got != accessibleTiles[p].dark {
			t.Errorf("%s on night = %+v, want the dark side", p, got)
		}
		if got := App(day, p).Tiles; got != accessibleTiles[p].light {
			t.Errorf("%s on day = %+v, want the light side", p, got)
		}
	}
	if App(night, TilesStandard).Tiles.Correct.Fill != "#2f9e5e" {
		t.Error("standard tiles are not the theme's own")
	}
}

func TestBundledPairsPointBothWays(t *testing.T) {
	lib := Bundled()
	for _, name := range []string{"night", "day", "ember dark", "ember light"} {
		th, ok := lib.Get(name)
		if !ok {
			t.Fatalf("no bundled %q", name)
		}
		other, ok := lib.Get(th.App.Pair)
		if !ok {
			t.Errorf("%s pairs with %q, which is not bundled", name, th.App.Pair)
			continue
		}
		if other.App.Pair != name {
			t.Errorf("%s pairs with %s, which pairs with %q", name, other.Name, other.App.Pair)
		}
		if App(th, TilesStandard).Dark == App(other, TilesStandard).Dark {
			t.Errorf("%s and %s are both the same side", name, other.Name)
		}
	}
}

// The night and day files are the mockups' colours; these are the ones the
// design was approved on.
func TestNightAndDayKeepTheApprovedColours(t *testing.T) {
	lib := Bundled()
	night, _ := lib.Get("night")
	day, _ := lib.Get("day")
	n, d := App(night, TilesStandard), App(day, TilesStandard)
	for _, c := range []struct{ what, got, want string }{
		{"night canvas", n.Canvas, "#000000"},
		{"night card", n.Card, "#16181e"},
		{"night sheet", n.Sheet, "#1a1c22"},
		{"night ink", n.Ink, "#f2f4f8"},
		{"night action", n.Action, "#ff8a3d"},
		{"night onAction", n.OnAction, "#060709"},
		{"night correct", n.Tiles.Correct.Fill, "#2f9e5e"},
		{"night present", n.Tiles.Present.Fill, "#e2b53e"},
		{"night absent", n.Tiles.Absent.Fill, "#2a2f38"},
		{"day canvas", d.Canvas, "#f2f2f7"},
		{"day correct", d.Tiles.Correct.Fill, "#2b8f52"},
	} {
		if c.got != c.want {
			t.Errorf("%s = %s, want %s", c.what, c.got, c.want)
		}
	}
}

func TestAppTokensJSONRoundTrip(t *testing.T) {
	night, _ := Bundled().Get("night")
	a := App(night, TilesStandard)
	b, err := json.Marshal(a)
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{`"inkDim"`, `"onAction"`, `"tiles":{"correct":{"fill"`, `"pair":"day"`} {
		if !strings.Contains(string(b), key) {
			t.Errorf("JSON has no %s: %s", key, b)
		}
	}
	var back AppTokens
	if err := json.Unmarshal(b, &back); err != nil {
		t.Fatal(err)
	}
	if back != a {
		t.Errorf("round trip changed the tokens:\n%+v\n%+v", a, back)
	}
}
