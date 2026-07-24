package ui

import (
	"image/color"
	"strings"

	"charm.land/lipgloss/v2"
)

// The palette follows monkeytype's default "serika dark": a flat dark ground,
// one amber accent carrying all emphasis, and muted grey for everything
// secondary. Tile colours are Wordle's, desaturated slightly to sit inside it.
//
// Every colour in the app comes from here, so restyling is a one-file change.
var (
	colorBg      = lipgloss.Color("#0a0a0a")
	colorAccent  = lipgloss.Color("#e2b714")
	colorMuted   = lipgloss.Color("#646669")
	colorText    = lipgloss.Color("#d1d0c5")
	colorError   = lipgloss.Color("#ca4754")
	colorCorrect = lipgloss.Color("#6aaa64")
	colorPresent = lipgloss.Color("#c9b458")
	colorAbsent  = lipgloss.Color("#3a3a3c")
	// colorSlot is for empty board cells: dim, but light enough to read against
	// the background, which colorAbsent is not.
	colorSlot = lipgloss.Color("#565758")
)

var (
	// Text styles.
	titleStyle  = lipgloss.NewStyle().Foreground(colorAccent).Bold(true)
	textStyle   = lipgloss.NewStyle().Foreground(colorText)
	mutedStyle  = lipgloss.NewStyle().Foreground(colorMuted)
	accentStyle = lipgloss.NewStyle().Foreground(colorAccent)
	errorStyle  = lipgloss.NewStyle().Foreground(colorError)

	// helpBar spaces the keybind hints off the content above them. It sets no
	// foreground on purpose: the hints are coloured per segment (they change on
	// hover), and an outer colour would be cancelled by the inner resets.
	helpBar = lipgloss.NewStyle().MarginTop(1)
)

// hoverStyle is the one cue for "the pointer is on this". Underline composes
// with every filled tile and keycap style in this file, and reads as a link on
// the help bar; changing the cue is a one-line change here.
func hoverStyle(s lipgloss.Style) lipgloss.Style { return s.Underline(true) }

// helpItem is one hint on the bottom bar: the keys, what they do, and — for
// everything a mouse needs to reach — the action a click on it performs.
type helpItem struct {
	keys  string
	label string
	act   action
}

// renderHelp draws the bottom hint line. Every hint that carries an action is
// also a button, so the bar doubles as the mouse-only control strip without
// looking any different from the plain hint line it replaced.
func renderHelp(h *hitMap, items ...helpItem) string {
	segments := make([]string, len(items))
	for i, item := range items {
		text := item.label
		if item.keys != "" {
			text = item.keys + " " + item.label
		}

		style := mutedStyle
		if h.hovered(item.act) {
			style = hoverStyle(accentStyle)
		}
		segments[i] = h.mark(item.act, style.Render(text))
	}
	return helpBar.Render(strings.Join(segments, mutedStyle.Render(" · ")))
}

// tileStyle renders one letter of the board. Played letters get a filled
// block; the row being typed is outlined in accent so the eye tracks it.
func tileStyle(bg color.Color, fg color.Color) lipgloss.Style {
	return lipgloss.NewStyle().
		Width(7).
		Align(lipgloss.Center).
		Bold(true).
		Background(bg).
		Foreground(fg)
}

var (
	tileCorrect = tileStyle(colorCorrect, colorBg)
	tilePresent = tileStyle(colorPresent, colorBg)
	tileAbsent  = tileStyle(colorAbsent, colorMuted)

	// tileActive is a letter the player has typed but not submitted.
	tileActive = lipgloss.NewStyle().
			Width(7).
			Align(lipgloss.Center).
			Bold(true).
			Foreground(colorText)

	// tileEmpty is an unfilled slot.
	tileEmpty = lipgloss.NewStyle().
			Width(7).
			Align(lipgloss.Center).
			Foreground(colorSlot)
)

// Key faces for the on-screen keyboard. The keys are rendered as filled caps —
// background, not just coloured text — so each letter's state reads at a glance
// against the near-black ground. An untouched key is a solid mid-grey; a
// letter known absent drops to a dim, obviously "spent" cap.
var (
	colorKeyFace  = lipgloss.Color("#565758") // untouched key
	colorKeySpent = lipgloss.Color("#1c1c1e") // letter guessed and absent
)

// keyStyle renders one keycap: a padded, bold letter on a filled background.
func keyStyle(bg, fg color.Color) lipgloss.Style {
	return lipgloss.NewStyle().Padding(0, 2).Bold(true).Background(bg).Foreground(fg)
}

// borderStyle colours the panel frame.
var borderStyle = lipgloss.NewStyle().Foreground(colorMuted)

// renderPanel draws a rounded border around content with a title inlaid in the
// top edge, btop-style. The border is built by hand rather than via lipgloss's
// Border() so the title can sit inside the top rule. corner is an optional
// segment inlaid at the right end of that rule — the close box — and may be
// empty.
func renderPanel(title, corner, content string) string {
	const padX = 3
	inner := lipgloss.NewStyle().Padding(1, padX).Render(content)
	lines := strings.Split(inner, "\n")

	// Widest line sets the inner width; pad the rest so the right edge aligns.
	width := 0
	for _, l := range lines {
		if w := lipgloss.Width(l); w > width {
			width = w
		}
	}
	for i, l := range lines {
		if pad := width - lipgloss.Width(l); pad > 0 {
			lines[i] = l + strings.Repeat(" ", pad)
		}
	}

	b := lipgloss.RoundedBorder()

	label := " " + title + " "
	fill := width - 1 - lipgloss.Width(label) - lipgloss.Width(corner)
	if fill < 0 {
		fill = 0
	}
	top := borderStyle.Render(b.TopLeft+b.Top) +
		accentStyle.Render(label) +
		borderStyle.Render(strings.Repeat(b.Top, fill)) +
		corner +
		borderStyle.Render(b.TopRight)

	var sb strings.Builder
	sb.WriteString(top)
	sb.WriteByte('\n')
	for _, l := range lines {
		sb.WriteString(borderStyle.Render(b.Left))
		sb.WriteString(l)
		sb.WriteString(borderStyle.Render(b.Right))
		sb.WriteByte('\n')
	}
	sb.WriteString(borderStyle.Render(b.BottomLeft + strings.Repeat(b.Bottom, width) + b.BottomRight))
	return sb.String()
}

var (
	keyUnused  = keyStyle(colorKeyFace, colorText)
	keyCorrect = keyStyle(colorCorrect, colorBg)
	keyPresent = keyStyle(colorPresent, colorBg)
	keyAbsent  = keyStyle(colorKeySpent, colorMuted)
)
