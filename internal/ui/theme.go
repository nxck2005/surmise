package ui

import (
	"image/color"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/nxck2005/wortle/internal/theme"
)

// This file turns a theme.Theme — plain colours, glyphs and numbers — into the
// lipgloss styles the screens render with. Nothing else in the package builds a
// style from a colour: adding a themeable element means adding a field here and
// an element name in internal/theme, so the two stay in step.
//
// The active styles live in one package-level pointer rather than thirty
// separate style variables. Styles are derived from the palette, so a palette
// change has to rebuild all of them at once; swapping a single pointer does
// that atomically, which is what makes live preview in the theme picker
// possible.

// st is the style set every screen renders through.
var st = newStyles(theme.Default())

// setTheme swaps the active look. Called at startup and on every move of the
// theme picker's cursor.
func setTheme(t *theme.Theme) {
	if t == nil {
		t = theme.Default()
	}
	st = newStyles(t)
}

// styles holds one built style per themeable element, plus the glyphs and
// metrics the render code needs directly.
type styles struct {
	theme *theme.Theme

	// Text.
	title  lipgloss.Style
	text   lipgloss.Style
	muted  lipgloss.Style
	accent lipgloss.Style
	err    lipgloss.Style

	// Chrome.
	border     lipgloss.Style
	panelTitle lipgloss.Style
	menuPick   lipgloss.Style
	cursor     lipgloss.Style
	help       lipgloss.Style
	helpHover  lipgloss.Style
	bar        lipgloss.Style

	// helpBar spaces the keybind hints off the content above them. It sets no
	// foreground on purpose: the hints are coloured per segment (they change on
	// hover), and an outer colour would be cancelled by the inner resets.
	helpBar lipgloss.Style

	// Board.
	tileCorrect lipgloss.Style
	tilePresent lipgloss.Style
	tileAbsent  lipgloss.Style
	tileActive  lipgloss.Style
	tileEmpty   lipgloss.Style
	caret       lipgloss.Style

	// Keyboard.
	keyUnused  lipgloss.Style
	keyCorrect lipgloss.Style
	keyPresent lipgloss.Style
	keyAbsent  lipgloss.Style

	// Puzzle-list status words.
	statusWon     color.Color
	statusLost    color.Color
	statusPlaying color.Color

	// Terminal-level colours, set on the tea.View.
	bg color.Color
	fg color.Color

	glyph  theme.Glyphs
	metric theme.Metrics

	// hoverAttrs is the "the pointer is on this" cue, applied on top of whatever
	// style the atom already has. Keeping it as attributes rather than a
	// finished style is what lets it compose with a filled tile, a keycap and a
	// help hint alike.
	hoverAttrs theme.Override
}

func newStyles(t *theme.Theme) *styles {
	c := t.Color
	tile := func(bg, fg color.Color) lipgloss.Style {
		s := lipgloss.NewStyle().
			Width(t.Metrics.TileWidth).
			Align(lipgloss.Center).
			Bold(true).
			Foreground(fg)
		if bg != nil {
			s = s.Background(bg)
		}
		return s
	}
	// Keycaps are filled — background, not just coloured text — so each letter's
	// state reads at a glance against the ground.
	keycap := func(bg, fg color.Color) lipgloss.Style {
		return lipgloss.NewStyle().
			Padding(0, t.Metrics.KeyPadX).
			Bold(true).
			Background(bg).
			Foreground(fg)
	}
	plain := func(fg color.Color) lipgloss.Style {
		return lipgloss.NewStyle().Foreground(fg)
	}

	s := &styles{
		theme: t,

		title:  plain(c(theme.Accent)).Bold(true),
		text:   plain(c(theme.Text)),
		muted:  plain(c(theme.Muted)),
		accent: plain(c(theme.Accent)),
		err:    plain(c(theme.Error)),

		border:     plain(c(theme.Muted)),
		panelTitle: plain(c(theme.Accent)),
		menuPick:   plain(c(theme.Accent)).Bold(true),
		cursor:     plain(c(theme.Accent)),
		help:       plain(c(theme.Muted)),
		helpHover:  plain(c(theme.Accent)),
		bar:        plain(c(theme.Correct)),
		helpBar:    lipgloss.NewStyle().MarginTop(1),

		tileCorrect: tile(c(theme.Correct), c("correct_text")),
		tilePresent: tile(c(theme.Present), c("present_text")),
		tileAbsent:  tile(c(theme.Absent), c("absent_text")),
		tileActive:  tile(nil, c(theme.Text)),
		tileEmpty:   tile(nil, c(theme.Slot)).Bold(false),
		caret:       tile(nil, c(theme.Accent)).Bold(false),

		keyUnused:  keycap(c(theme.KeyFace), c("key_unused_text")),
		keyCorrect: keycap(c(theme.Correct), c("key_correct_text")),
		keyPresent: keycap(c(theme.Present), c("key_present_text")),
		keyAbsent:  keycap(c(theme.KeySpent), c("key_absent_text")),

		statusWon:     c(theme.Correct),
		statusLost:    c(theme.Error),
		statusPlaying: c(theme.Accent),

		bg: c(theme.Bg),
		fg: c(theme.Text),

		glyph:      t.Glyphs,
		metric:     t.Metrics,
		hoverAttrs: hoverDefault(t),
	}

	// Per-element overrides land last, so a theme can bold a label or recolour
	// one tile without restating anything the palette already decided.
	for element, target := range map[string]*lipgloss.Style{
		"title": &s.title, "text": &s.text, "muted": &s.muted,
		"accent": &s.accent, "error": &s.err,
		"help": &s.help, "help_hover": &s.helpHover,
		"border": &s.border, "panel_title": &s.panelTitle,
		"menu_selected": &s.menuPick, "cursor": &s.cursor, "caret": &s.caret,
		"bar":          &s.bar,
		"tile.correct": &s.tileCorrect, "tile.present": &s.tilePresent,
		"tile.absent": &s.tileAbsent, "tile.active": &s.tileActive,
		"tile.empty": &s.tileEmpty,
		"key.unused": &s.keyUnused, "key.correct": &s.keyCorrect,
		"key.present": &s.keyPresent, "key.absent": &s.keyAbsent,
	} {
		*target = apply(*target, t.Override(element))
	}

	// The three status words are colours rather than styles, since the list
	// composes them next to already-styled columns.
	for element, target := range map[string]*color.Color{
		"status.won": &s.statusWon, "status.lost": &s.statusLost,
		"status.playing": &s.statusPlaying,
	} {
		if fg := t.Override(element).FG; fg != nil {
			*target = fg
		}
	}
	return s
}

// hoverDefault is the built-in hover cue: underline, which composes with every
// filled tile and keycap and reads as a link on the help bar. A theme can
// replace it wholesale under [style.hover].
func hoverDefault(t *theme.Theme) theme.Override {
	o := t.Override("hover")
	if o.Bold == nil && o.Italic == nil && o.Underline == nil && o.Faint == nil && o.FG == nil && o.BG == nil {
		on := true
		return theme.Override{Underline: &on}
	}
	return o
}

// apply layers a theme's override onto a built style. A nil field leaves the
// built-in value alone.
func apply(s lipgloss.Style, o theme.Override) lipgloss.Style {
	if o.FG != nil {
		s = s.Foreground(o.FG)
	}
	if o.BG != nil {
		s = s.Background(o.BG)
	}
	if o.Bold != nil {
		s = s.Bold(*o.Bold)
	}
	if o.Italic != nil {
		s = s.Italic(*o.Italic)
	}
	if o.Underline != nil {
		s = s.Underline(*o.Underline)
	}
	if o.Faint != nil {
		s = s.Faint(*o.Faint)
	}
	return s
}

// hover marks a style as the one the pointer is over. It is the single cue, in
// a single place, for every screen.
func (s *styles) hover(base lipgloss.Style) lipgloss.Style {
	return apply(base, s.hoverAttrs)
}

// border returns the frame runes the panel is drawn with.
func (s *styles) borderRunes() lipgloss.Border {
	switch s.glyph.Border {
	case "normal":
		return lipgloss.NormalBorder()
	case "thick":
		return lipgloss.ThickBorder()
	case "double":
		return lipgloss.DoubleBorder()
	case "hidden":
		return lipgloss.HiddenBorder()
	case "block":
		return lipgloss.BlockBorder()
	default:
		return lipgloss.RoundedBorder()
	}
}

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

		style := st.help
		if h.hovered(item.act) {
			style = st.hover(st.helpHover)
		}
		segments[i] = h.mark(item.act, style.Render(text))
	}
	return st.helpBar.Render(strings.Join(segments, st.help.Render(st.glyph.Separator)))
}

// renderPanel draws a rounded border around content with a title inlaid in the
// top edge, btop-style. The border is built by hand rather than via lipgloss's
// Border() so the title can sit inside the top rule. corner is an optional
// segment inlaid at the right end of that rule — the close box — and may be
// empty.
func renderPanel(title, corner, content string) string {
	inner := lipgloss.NewStyle().
		Padding(st.metric.PanelPadY, st.metric.PanelPadX).
		Render(content)
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

	b := st.borderRunes()

	label := " " + title + " "
	fill := width - 1 - lipgloss.Width(label) - lipgloss.Width(corner)
	if fill < 0 {
		fill = 0
	}
	top := st.border.Render(b.TopLeft+b.Top) +
		st.panelTitle.Render(label) +
		st.border.Render(strings.Repeat(b.Top, fill)) +
		corner +
		st.border.Render(b.TopRight)

	var sb strings.Builder
	sb.WriteString(top)
	sb.WriteByte('\n')
	for _, l := range lines {
		sb.WriteString(st.border.Render(b.Left))
		sb.WriteString(l)
		sb.WriteString(st.border.Render(b.Right))
		sb.WriteByte('\n')
	}
	sb.WriteString(st.border.Render(b.BottomLeft + strings.Repeat(b.Bottom, width) + b.BottomRight))
	return sb.String()
}
