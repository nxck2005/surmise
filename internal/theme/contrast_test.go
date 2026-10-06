package theme

import (
	"image/color"
	"math"
	"testing"
)

// luminance is the WCAG relative luminance of a colour.
func luminance(c color.Color) float64 {
	r, g, b, _ := c.RGBA()
	lin := func(v uint32) float64 {
		x := float64(v) / 0xffff
		if x <= 0.03928 {
			return x / 12.92
		}
		return math.Pow((x+0.055)/1.055, 2.4)
	}
	return 0.2126*lin(r) + 0.7152*lin(g) + 0.0722*lin(b)
}

// contrast is the WCAG contrast ratio between two colours, from 1 to 21.
func contrast(a, b color.Color) float64 {
	la, lb := luminance(a), luminance(b)
	return (max(la, lb) + 0.05) / (min(la, lb) + 0.05)
}

// Floors on the pairs a player has to tell apart. An untouched keycap and an
// absent tile used to be one colour in dracula and solarized and nearly one in
// the rest, so a fresh keyboard read as every letter ruled out — the legend
// shows the absent tile. The letters are held to 3:1, the WCAG floor for bold
// and large text, which every bundled keycap and tile letter is.
//
// The terminal theme is checked too: its ANSI numbers resolve to the standard
// palette here, which is not what any one terminal shows, but two numbers that
// are the same colour everywhere still fail.
func TestBundledThemesKeepKeysAndTilesApart(t *testing.T) {
	const (
		apart    = 1.45 // above every pair that read as one colour: 1.00–1.38
		readable = 3.0  // a letter against its fill
	)
	for _, e := range Bundled().Entries() {
		th := e.Theme
		pairs := []struct {
			what  string
			a, b  string
			floor float64
		}{
			{"key_face against absent", KeyFace, Absent, apart},
			{"key_unused_text on key_face", "key_unused_text", KeyFace, readable},
			{"absent_text on absent", "absent_text", Absent, readable},
		}
		for _, p := range pairs {
			if got := contrast(th.Color(p.a), th.Color(p.b)); got < p.floor {
				t.Errorf("%s: %s is %.2f:1, want at least %.1f:1", e.Name, p.what, got, p.floor)
			}
		}
	}
}
