package ui

import (
	"fmt"
	"image/color"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/nxck2005/surmise/internal/theme"
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

	// splash is the startup art. It is its own element rather than the title's
	// style because it is the one thing on screen drawn in glyphs rather than
	// letters, and a theme may well want it quieter — or louder — than a heading.
	splash lipgloss.Style

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
		splash:     plain(c(theme.Accent)),
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
		"bar": &s.bar, "splash": &s.splash,
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

		// The bar's copy of an action is marked as such, so hovering a button
		// lights that button and nothing else — several of these repeat the
		// action of the row the cursor is on, and the action is the hover key.
		act := item.act
		act.help = true

		style := st.help
		if h.hovered(act) {
			style = st.hover(st.helpHover)
		}
		segments[i] = h.mark(act, style.Render(text))
	}
	return st.helpBar.Render(strings.Join(segments, st.help.Render(st.glyph.Separator)))
}

// block squares a multi-line string off, padding every line to the width of the
// widest.
//
// It exists because lipgloss.JoinVertical(Center, …) centres line by line, not
// block by block: hand it a table and it centres each row on its own, so rows
// whose widths differ in parity land a column apart and the left edge goes
// ragged. Squaring the table first makes every line the same width, so the one
// centring offset applies to all of them and the block moves as a unit with its
// own alignment intact.
//
// Safe on marked content: hitMap's markers are zero-width, so lipgloss.Width
// ignores them and the padding lands where it would have anyway
// (TestMarkersDoNotAffectLayout).
func block(s string) string {
	lines := strings.Split(s, "\n")
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
	return strings.Join(lines, "\n")
}

// titled is the standard shape of a list-style screen: its title centred over
// its body, with the body moved as a block so its rows keep the shared left edge
// that makes their columns line up. Screens that lay out around a board
// (gameScreen, themeScreen) centre their own sections instead.
func titled(title, body string) string {
	return lipgloss.JoinVertical(lipgloss.Center,
		st.title.Render(title),
		"",
		block(body),
	)
}

// scrollCounter is the "3–14 of 27" line under a scrolling list, with its two
// ends as click targets.
//
// The counter is where the jump targets belong because it is the one thing on
// screen that is already about position in the list — and because home and end
// had no clickable equivalent at all before it carried them, which is the only
// hard parity gap the mouse sweep turned up. Both ends go through the screen's
// jumpTop/jumpBottom, the same methods the keys call.
func scrollCounter(h *hitMap, first, last, total int) string {
	end := func(a action, glyph string) string {
		style := st.muted
		if h.hovered(a) {
			style = st.hover(style)
		}
		return h.mark(a, style.Render(glyph))
	}
	return "  " +
		end(action{kind: actJumpTop}, st.glyph.JumpFirst) +
		st.muted.Render(fmt.Sprintf(" %d–%d of %d ", first, last, total)) +
		end(action{kind: actJumpBottom}, st.glyph.JumpLast)
}

// bodyBudget is how many lines a titled screen's body may take before the panel
// outgrows the terminal.
//
// Overflowing is not merely ugly: nothing here truncates, so the renderer drops
// the excess off the top of the screen (see hitMap.clip), taking the title, the
// close box and whatever else was up there with it. A screen that can shed or
// scroll should therefore know its budget.
//
// The subtraction is the chrome around the body: the title and the blank under
// it that titled adds, the help bar and the blank above it, and the panel's
// padding and border. A height of zero — the size before the first
// WindowSizeMsg — means unbounded, and is reported as 0 for callers to skip.
func bodyBudget(height int) int {
	if height <= 0 {
		return 0
	}
	const (
		title = 2 // the title and the blank line under it
		help  = 2 // the help bar and the blank line above it
		frame = 2 // the panel's top and bottom border
	)
	return height - title - help - frame - 2*st.metric.PanelPadY
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
	// Squaring the content off gives the border a straight right edge to follow.
	lines := strings.Split(block(inner), "\n")
	width := lipgloss.Width(lines[0])

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
