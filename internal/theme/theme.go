// Package theme is the styleable surface of wortle, expressed as data.
//
// A theme is one file of `key = value` lines (see parse.go). Everything the UI
// can colour, letter or size is named here, so adding a themeable element means
// adding a key to this file and reading it in internal/ui — not scattering a
// new constant through the render code.
//
// The package is deliberately free of Bubble Tea: it yields colours, glyphs and
// numbers, and internal/ui turns those into lipgloss styles. lipgloss itself is
// imported only for its colour parsing.
package theme

import (
	"image/color"
)

// Palette keys. These are the colours a theme actually chooses; everything else
// in the UI is drawn from them.
const (
	Bg       = "bg"       // terminal background
	Text     = "text"     // primary foreground
	Accent   = "accent"   // the one emphasis colour
	Muted    = "muted"    // secondary text, borders
	Error    = "error"    // failures and the close box
	Correct  = "correct"  // right letter, right place
	Present  = "present"  // right letter, wrong place
	Absent   = "absent"   // letter not in the word
	Slot     = "slot"     // empty board cell
	KeyFace  = "key_face" // untouched keycap
	KeySpent = "key_spent"
)

// baseKeys is every palette entry a theme sets directly, in the order the
// documentation and the bundled themes list them.
var baseKeys = []string{
	Bg, Text, Accent, Muted, Error,
	Correct, Present, Absent, Slot, KeyFace, KeySpent,
}

// derivedKeys are the text-on-a-filled-background colours. They exist so light
// themes work: printing on a green tile wants the background colour on a dark
// theme and something else entirely on a light one. A theme that says nothing
// about them gets the sensible follow-on, so most themes never mention them.
var derivedKeys = map[string]string{
	"correct_text":     Bg,
	"present_text":     Bg,
	"absent_text":      Muted,
	"key_correct_text": Bg,
	"key_present_text": Bg,
	"key_absent_text":  Muted,
	"key_unused_text":  Text,
}

// Elements are the styleable pieces of the UI. A theme may override any of them
// under a [style.<element>] section; unnamed ones keep the built-in look, which
// is itself derived from the palette.
var Elements = []string{
	"title", "text", "muted", "accent", "error",
	"help", "help_hover", "hover",
	"border", "panel_title", "menu_selected", "cursor", "caret", "bar",
	"tile.correct", "tile.present", "tile.absent", "tile.active", "tile.empty",
	"key.unused", "key.correct", "key.present", "key.absent",
	"status.won", "status.lost", "status.playing",
}

// Override is a per-element tweak. A nil field means "leave the built-in
// value alone", which is what lets a theme bold one label without having to
// restate its colours.
type Override struct {
	FG, BG    color.Color
	Bold      *bool
	Italic    *bool
	Underline *bool
	Faint     *bool
}

// Glyphs are the characters the UI draws. Changing these changes layout widths,
// so themes that touch them should be checked in a real terminal.
type Glyphs struct {
	Caret       string // where the next letter will land
	Empty       string // an unfilled board cell
	Cursor      string // selection marker, left side
	CursorRight string // selection marker, right side (menu rows are flanked)
	Separator   string // between hints on the help bar
	ValuePrev   string // settings row: step to the previous value
	ValueNext   string // settings row: step to the next value
	JumpFirst   string // scrolling list: jump to the first row
	JumpLast    string // scrolling list: jump to the last row
	Enter       string // submit keycap
	Delete      string // backspace keycap
	Bar         string // profile histogram
	Close       string // the panel's close box
	Border      string // rounded | normal | thick | double | hidden | block
}

// Metrics are the size knobs. Like glyphs, these move the layout.
type Metrics struct {
	TileWidth int // display cells per board tile
	KeyPadX   int // horizontal padding inside a keycap
	PanelPadX int // horizontal padding inside the panel border
	PanelPadY int // vertical padding inside the panel border
}

// Theme is one complete look.
type Theme struct {
	Name   string
	Author string

	palette map[string]color.Color
	Glyphs  Glyphs
	Metrics Metrics
	Styles  map[string]Override
}

// Color returns a palette entry. Derived entries are resolved here rather than
// filled in at load time: a theme that sets only `bg` must still move the
// text-on-tile colours with it. Unknown keys yield the primary foreground, so a
// typo shows up as flat text instead of an invisible element.
func (t *Theme) Color(key string) color.Color {
	if c, ok := t.palette[key]; ok {
		return c
	}
	if from, ok := derivedKeys[key]; ok {
		return t.palette[from]
	}
	return t.palette[Text]
}

// Override returns the tweak for an element, or the zero value.
func (t *Theme) Override(element string) Override {
	return t.Styles[element]
}

// Default is the built-in look: "serika dark", with Wordle's tile
// colours desaturated to sit inside it. It is the base every loaded theme is
// overlaid onto, so a four-line theme file is valid.
func Default() *Theme {
	t := &Theme{
		Name: "serika dark",
		palette: map[string]color.Color{
			Bg:       mustColor("#0a0a0a"),
			Text:     mustColor("#d1d0c5"),
			Accent:   mustColor("#e2b714"),
			Muted:    mustColor("#646669"),
			Error:    mustColor("#ca4754"),
			Correct:  mustColor("#6aaa64"),
			Present:  mustColor("#c9b458"),
			Absent:   mustColor("#3a3a3c"),
			Slot:     mustColor("#565758"),
			KeyFace:  mustColor("#565758"),
			KeySpent: mustColor("#1c1c1e"),
		},
		Glyphs: Glyphs{
			Caret:       "_",
			Empty:       "·",
			Cursor:      "› ",
			CursorRight: " ‹",
			Separator:   " · ",
			ValuePrev:   "‹",
			ValueNext:   "›",
			JumpFirst:   "⇱",
			JumpLast:    "⇲",
			Enter:       "⏎",
			Delete:      "⌫",
			Bar:         "█",
			Close:       "×",
			Border:      "rounded",
		},
		Metrics: Metrics{TileWidth: 7, KeyPadX: 2, PanelPadX: 3, PanelPadY: 1},
		Styles:  map[string]Override{},
	}
	return t
}

// clone deep-copies a theme so overlaying a file never mutates Default's maps.
func (t *Theme) clone() *Theme {
	out := *t
	out.palette = make(map[string]color.Color, len(t.palette))
	for k, v := range t.palette {
		out.palette[k] = v
	}
	out.Styles = make(map[string]Override, len(t.Styles))
	for k, v := range t.Styles {
		out.Styles[k] = v
	}
	return &out
}
