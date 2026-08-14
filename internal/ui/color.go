package ui

import (
	"image/color"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/colorprofile"
)

// This file is the app's only colour arithmetic. Everywhere else reads a
// finished style off st.
//
// The rule that keeps themes in charge: a derived colour is always derived
// **from palette colours**. A gradient blends between two colours the theme
// already set, so every bundled theme gets the effect without an edit and
// nothing here needs a new themeable element — the same bargain the board's
// animations make, which compose existing tokens rather than naming their own.

// colors is how much colour the terminal can show. It is package state, like
// st: the profile belongs to the program, and the render code that needs it is
// reached from too many places to thread it through.
//
// TrueColor is the assumption until the terminal says otherwise. Bubble Tea
// reports the real profile at startup (tea.ColorProfileMsg), and the browser
// build forces TrueColor, so this default is only what the headless tests and
// the frames before that message see.
var colors = colorprofile.TrueColor

// setColorProfile records what the terminal told us. A test that changes it
// must restore it, as one that calls setTheme must.
func setColorProfile(p colorprofile.Profile) { colors = p }

// rich reports whether a gradient is worth drawing. Below 256 colours a blend
// quantises to a handful of steps and reads as banding rather than as a
// gradient, so everything derived collapses to a flat colour instead — one
// branch, in blend, rather than a condition at every call site.
func rich() bool { return colors >= colorprofile.ANSI256 }

// blend returns exactly steps colours easing through stops, left to right.
//
// On a terminal without the depth for it — and when there is nothing to blend —
// the run is flat in the **last** stop, so a caller writes its gradient as
// "from the emphasis, back to the ordinary colour" and gets the ordinary colour
// on a poor terminal.
func blend(steps int, stops ...color.Color) []color.Color {
	if steps <= 0 {
		return nil
	}
	stops = compact(stops)
	if len(stops) == 0 {
		return make([]color.Color, steps)
	}
	if len(stops) == 1 || !rich() {
		return flat(steps, stops[len(stops)-1])
	}

	// Blend1D returns the stops themselves when there are more of them than
	// steps, so a run narrower than the stop list would otherwise come back
	// short.
	out := lipgloss.Blend1D(steps, stops...)
	if len(out) < steps {
		return flat(steps, stops[len(stops)-1])
	}
	return out
}

// colorAt reads a palette safely, so a caller may index by a position it
// computed without checking the run's length first.
func colorAt(pal []color.Color, i int) color.Color {
	if len(pal) == 0 {
		return nil
	}
	return pal[min(max(i, 0), len(pal)-1)]
}

// lift and dim move a colour toward white and toward black. They are for
// emphasis within one palette colour — a tile brightening as it is celebrated —
// where introducing a second colour would say something the theme did not.
func lift(c color.Color, percent float64) color.Color {
	if c == nil || !rich() {
		return c
	}
	return lipgloss.Lighten(c, percent)
}

func dim(c color.Color, percent float64) color.Color {
	if c == nil || !rich() {
		return c
	}
	return lipgloss.Darken(c, percent)
}

func flat(steps int, c color.Color) []color.Color {
	out := make([]color.Color, steps)
	for i := range out {
		out[i] = c
	}
	return out
}

func compact(stops []color.Color) []color.Color {
	out := stops[:0:0]
	for _, c := range stops {
		if c != nil {
			out = append(out, c)
		}
	}
	return out
}
