package theme

import (
	"fmt"
	"image/color"
	"math"
	"strings"
)

// The phone app draws with its own set of colours, the AppTokens below, and
// takes them from the same theme files the terminal reads. A theme file may
// say what they are under [app.colors]; anything it leaves out is derived from
// the [colors] palette, so every terminal theme is also a phone theme with no
// extra work. The terminal never reads any of this.
//
// Spacing, sizes, layout and motion are not tokens: a theme can recolour the
// app but never move anything in it, the same rule the terminal keeps.

// AppSpec is a file's [app] and [app.colors] sections, as written.
type AppSpec struct {
	Pair   string                 // the light or dark partner, shown with this theme as one
	Fonts  string                 // a bundled font set; a theme never loads a font
	Colors map[string]color.Color // [app.colors], by key
}

// Font sets the app bundles. A theme names one; it cannot ship its own.
const (
	FontsGeist   = "geist"
	DefaultFonts = FontsGeist
)

var fontSets = []string{FontsGeist}

// App colour keys, as written in [app.colors]. Each is one AppTokens field.
var appColorKeys = []string{
	"canvas", "card", "sheet", "highlight",
	"ink", "ink_dim", "ink_muted",
	"accent", "accent_text", "action", "on_action", "danger",
	"chart_highlight", "chart_neutral",
	"key_face", "key_special", "key_spent", "key_text", "key_spent_text",
	"tile_empty", "tile_typed", "cursor",
	"correct", "correct_letter", "present", "present_letter", "absent", "absent_letter",
	"glow_from", "glow_to", "tab_tint",
}

// setApp applies one key under [app] or [app.colors].
func (t *Theme) setApp(key, value string) error {
	switch key {
	case "pair":
		if len(value) > maxFieldBytes {
			return fmt.Errorf("pair is longer than %d bytes", maxFieldBytes)
		}
		t.App.Pair = value
		return nil
	case "fonts":
		for _, f := range fontSets {
			if f == value {
				t.App.Fonts = value
				return nil
			}
		}
		return fmt.Errorf("unknown font set %q: want %s", value, strings.Join(fontSets, ", "))
	}
	name, ok := strings.CutPrefix(key, "colors.")
	if !ok || !isAppColorKey(name) {
		return fmt.Errorf("unknown key %q", "app."+key)
	}
	c, err := t.parseColor(value)
	if err != nil {
		return err
	}
	// A phone has no terminal palette for an ANSI number to mean.
	if t.valueIsANSI(value) {
		return fmt.Errorf("app colour %s: want #rrggbb or a palette name, not an ANSI number", name)
	}
	if t.App.Colors == nil {
		t.App.Colors = map[string]color.Color{}
	}
	t.App.Colors[name] = c
	return nil
}

func isAppColorKey(name string) bool {
	for _, k := range appColorKeys {
		if k == name {
			return true
		}
	}
	return false
}

// TilePalette is the player's tile colour choice in the app. The accessibility
// palettes replace the theme's tiles outright, so whether a player can read
// the board never depends on which theme they picked.
type TilePalette string

const (
	TilesStandard     TilePalette = ""
	TilesColorblind   TilePalette = "colorblind"
	TilesHighContrast TilePalette = "high-contrast"
)

// TilePalettes lists every choice, standard first.
var TilePalettes = []TilePalette{TilesStandard, TilesColorblind, TilesHighContrast}

// Tile is one mark's fill and the letter drawn on it.
type Tile struct {
	Fill   string `json:"fill"`
	Letter string `json:"letter"`
}

// Tiles are the three marks.
type Tiles struct {
	Correct Tile `json:"correct"`
	Present Tile `json:"present"`
	Absent  Tile `json:"absent"`
}

// AppTokens is every colour the app draws with, as #rrggbb, plus the few
// non-colour choices a theme makes. It is the whole styleable surface of the
// app; the JSON form is what the engine hands to Kotlin.
type AppTokens struct {
	Name  string `json:"name"`
	Pair  string `json:"pair,omitempty"`
	Fonts string `json:"fonts"`
	Dark  bool   `json:"dark"`

	Canvas    string `json:"canvas"`    // the ground under everything
	Card      string `json:"card"`      // cards, a step up from the canvas
	Sheet     string `json:"sheet"`     // sheets and dialogs, a step above cards
	Highlight string `json:"highlight"` // the hairline at a card's top edge

	Ink      string `json:"ink"`      // primary text
	InkDim   string `json:"inkDim"`   // supporting text
	InkMuted string `json:"inkMuted"` // captions, dates, codes

	Accent     string `json:"accent"`     // selection, charts, links
	AccentText string `json:"accentText"` // the accent as text on the canvas
	Action     string `json:"action"`     // primary buttons, switches
	OnAction   string `json:"onAction"`   // text on Action
	Danger     string `json:"danger"`     // errors, delete

	ChartHighlight string `json:"chartHighlight"` // the bar or line that matters
	ChartNeutral   string `json:"chartNeutral"`   // every other bar

	KeyFace      string `json:"keyFace"`      // an untouched key
	KeySpecial   string `json:"keySpecial"`   // enter and delete
	KeySpent     string `json:"keySpent"`     // a key known absent
	KeyText      string `json:"keyText"`      // the letter on KeyFace and KeySpecial
	KeySpentText string `json:"keySpentText"` // the letter on KeySpent

	TileEmpty string `json:"tileEmpty"` // an empty tile's outline
	TileTyped string `json:"tileTyped"` // a typed, unscored tile's outline
	Cursor    string `json:"cursor"`    // the outline on the next empty tile

	Tiles Tiles `json:"tiles"`

	GlowFrom string `json:"glowFrom"` // the daily card's glow, top-left
	GlowTo   string `json:"glowTo"`   // and bottom-right
	TabTint  string `json:"tabTint"`  // the tab bar's glass
}

// Derivation steps, as fractions mixed from the first colour toward the
// second. Surfaces climb from the background toward the text in small steps,
// the way the mockups' cards sit just above the canvas; dim ink sits halfway
// between the text and muted colours.
const (
	cardStep      = 0.07 // card = bg → text
	sheetStep     = 0.10 // sheet = bg → text
	highlightStep = 0.16 // highlight = bg → text
	inkDimStep    = 0.50 // ink_dim = text → muted
	specialStep   = 0.60 // key_special = bg → key_face
	neutralStep   = 0.25 // chart_neutral = bg → text
	repairStep    = 0.05 // contrast repair moves this far toward black or white per step
)

// Contrast floors (WCAG ratios) the tokens are held to. Text is the WCAG AA
// floor for body text; tile and key letters are large and bold, so they get
// the large-text floor.
const (
	floorText   = 4.5
	floorLetter = 3.0
)

// App resolves a theme into app tokens. The order is: the palette, then the
// file's [app.colors] values, then derived values for the rest, then contrast
// repair of derived text colours, then the tile palette, which always wins.
func App(t *Theme, tiles TilePalette) AppTokens {
	set := t.App.Colors
	bg := t.Color(Bg)
	text := t.Color(Text)

	// pick takes the file's value for key, or the derived one.
	explicit := map[string]bool{}
	pick := func(key string, derived color.Color) color.Color {
		if c, ok := set[key]; ok {
			explicit[key] = true
			return c
		}
		return derived
	}

	canvas := pick("canvas", bg)
	card := pick("card", mix(bg, text, cardStep))
	sheet := pick("sheet", mix(bg, text, sheetStep))
	highlight := pick("highlight", mix(bg, text, highlightStep))
	ink := pick("ink", text)
	inkDim := pick("ink_dim", mix(text, t.Color(Muted), inkDimStep))
	inkMuted := pick("ink_muted", t.Color(Muted))
	accent := pick("accent", t.Color(Accent))
	accentText := pick("accent_text", accent)
	action := pick("action", accent)
	onAction := pick("on_action", moreContrast(action, bg, text))
	keyFace := pick("key_face", t.Color(KeyFace))
	keySpent := pick("key_spent", t.Color(KeySpent))
	correct := pick("correct", t.Color(Correct))
	present := pick("present", t.Color(Present))
	absent := pick("absent", t.Color(Absent))

	// repair moves a derived text colour away from its grounds until it meets
	// floor on every one of them. A value the file set is left alone: the
	// contrast test reports it instead, and the file gets fixed.
	repair := func(key string, c color.Color, floor float64, grounds ...color.Color) color.Color {
		if explicit[key] {
			return c
		}
		return repairContrast(c, floor, grounds...)
	}
	ink = repair("ink", ink, floorText, canvas, card, sheet)
	inkDim = repair("ink_dim", inkDim, floorText, canvas, card, sheet)
	inkMuted = repair("ink_muted", inkMuted, floorText, canvas, card)
	accentText = repair("accent_text", accentText, floorText, canvas, card)
	onAction = repair("on_action", onAction, floorText, action)

	out := AppTokens{
		Name:  t.Name,
		Pair:  t.App.Pair,
		Fonts: t.App.Fonts,
		Dark:  isDark(canvas),

		Canvas:    hex(canvas),
		Card:      hex(card),
		Sheet:     hex(sheet),
		Highlight: hex(highlight),

		Ink:      hex(ink),
		InkDim:   hex(inkDim),
		InkMuted: hex(inkMuted),

		Accent:     hex(accent),
		AccentText: hex(accentText),
		Action:     hex(action),
		OnAction:   hex(onAction),
		Danger:     hex(pick("danger", t.Color(Error))),

		ChartHighlight: hex(pick("chart_highlight", accent)),
		ChartNeutral:   hex(pick("chart_neutral", mix(bg, text, neutralStep))),

		KeyFace:      hex(keyFace),
		KeySpecial:   hex(pick("key_special", mix(bg, keyFace, specialStep))),
		KeySpent:     hex(keySpent),
		KeyText:      hex(repair("key_text", pick("key_text", t.Color("key_unused_text")), floorLetter, keyFace)),
		KeySpentText: hex(pick("key_spent_text", t.Color("key_absent_text"))),

		TileEmpty: hex(pick("tile_empty", t.Color(Slot))),
		TileTyped: hex(pick("tile_typed", t.Color(Muted))),
		Cursor:    hex(pick("cursor", accent)),

		Tiles: Tiles{
			Correct: Tile{hex(correct), hex(repair("correct_letter", pick("correct_letter", t.Color("correct_text")), floorLetter, correct))},
			Present: Tile{hex(present), hex(repair("present_letter", pick("present_letter", t.Color("present_text")), floorLetter, present))},
			Absent:  Tile{hex(absent), hex(repair("absent_letter", pick("absent_letter", t.Color("absent_text")), floorLetter, absent))},
		},

		GlowFrom: hex(pick("glow_from", accent)),
		GlowTo:   hex(pick("glow_to", t.Color(Present))),
		TabTint:  hex(pick("tab_tint", card)),
	}
	if out.Fonts == "" {
		out.Fonts = DefaultFonts
	}
	if p, ok := accessibleTiles[tiles]; ok {
		if out.Dark {
			out.Tiles = p.dark
		} else {
			out.Tiles = p.light
		}
	}
	return out
}

// accessibleTiles are the fixed tile palettes that replace a theme's own. Each
// has a dark and a light side, chosen by the canvas.
var accessibleTiles = map[TilePalette]struct{ dark, light Tiles }{
	// Blue and orange: told apart by hue and by lightness, for red-green
	// colour blindness.
	TilesColorblind: {
		dark: Tiles{
			Correct: Tile{"#377bff", "#ffffff"},
			Present: Tile{"#ff8a3d", "#060709"},
			Absent:  Tile{"#2a2f38", "#d5d9e0"},
		},
		light: Tiles{
			Correct: Tile{"#2f6bf0", "#ffffff"},
			Present: Tile{"#ff8a3d", "#060709"},
			Absent:  Tile{"#d9dce3", "#2f3237"},
		},
	},
	// Brighter fills with dark letters, far apart in lightness.
	TilesHighContrast: {
		dark: Tiles{
			Correct: Tile{"#3cc46f", "#060709"},
			Present: Tile{"#ffd34d", "#060709"},
			Absent:  Tile{"#3a3f4a", "#ffffff"},
		},
		light: Tiles{
			Correct: Tile{"#1b7a3f", "#ffffff"},
			Present: Tile{"#f5b800", "#060709"},
			Absent:  Tile{"#8a909c", "#060709"},
		},
	},
}

// rgbf is a colour as three 0–1 channels.
type rgbf struct{ r, g, b float64 }

func toRGB(c color.Color) rgbf {
	r, g, b, _ := c.RGBA()
	return rgbf{float64(r) / 0xffff, float64(g) / 0xffff, float64(b) / 0xffff}
}

func (c rgbf) RGBA() (r, g, b, a uint32) {
	q := func(v float64) uint32 { return uint32(math.Round(clamp01(v)*0xff)) * 0x101 }
	return q(c.r), q(c.g), q(c.b), 0xffff
}

func clamp01(v float64) float64 { return math.Max(0, math.Min(1, v)) }

// mix moves from a toward b by f, channel by channel in sRGB, the same blend
// the terminal's gradients use.
func mix(a, b color.Color, f float64) color.Color {
	x, y := toRGB(a), toRGB(b)
	return rgbf{x.r + (y.r-x.r)*f, x.g + (y.g-x.g)*f, x.b + (y.b-x.b)*f}
}

// hex writes a colour as #rrggbb.
func hex(c color.Color) string {
	r, g, b, _ := c.RGBA()
	return fmt.Sprintf("#%02x%02x%02x", r>>8, g>>8, b>>8)
}

// Luminance is the WCAG relative luminance of a colour.
func Luminance(c color.Color) float64 {
	v := toRGB(c)
	lin := func(x float64) float64 {
		if x <= 0.03928 {
			return x / 12.92
		}
		return math.Pow((x+0.055)/1.055, 2.4)
	}
	return 0.2126*lin(v.r) + 0.7152*lin(v.g) + 0.0722*lin(v.b)
}

// Contrast is the WCAG contrast ratio between two colours, from 1 to 21.
func Contrast(a, b color.Color) float64 {
	la, lb := Luminance(a), Luminance(b)
	return (max(la, lb) + 0.05) / (min(la, lb) + 0.05)
}

var (
	black = rgbf{0, 0, 0}
	white = rgbf{1, 1, 1}
)

// isDark reports whether a ground is nearer black than white.
func isDark(c color.Color) bool { return Contrast(c, white) > Contrast(c, black) }

// moreContrast returns whichever of a and b stands out more on ground.
func moreContrast(ground, a, b color.Color) color.Color {
	if Contrast(a, ground) >= Contrast(b, ground) {
		return a
	}
	return b
}

// repairContrast moves c toward black or white, whichever the first ground is
// further from, in repairStep steps, until it meets floor on every ground. It
// stops at black or white, which is as far as a colour can go.
func repairContrast(c color.Color, floor float64, grounds ...color.Color) color.Color {
	meets := func(c color.Color) bool {
		for _, g := range grounds {
			if Contrast(c, g) < floor {
				return false
			}
		}
		return true
	}
	if meets(c) {
		return c
	}
	var toward color.Color = white
	if !isDark(grounds[0]) {
		toward = black
	}
	from := c
	for f := repairStep; f < 1; f += repairStep {
		if m := mix(from, toward, f); meets(m) {
			return m
		}
	}
	return toward
}
