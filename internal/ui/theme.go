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

	// panel caches the normal border's styled runes, and legend caches the
	// rendered colour key, each for this style set. Both are functions of the
	// theme and the terminal's colour depth and of nothing else, and both used
	// to be rebuilt on every frame; setTheme swaps the whole struct, so a theme
	// change cannot leave anything stale behind. See panelRuleFor and
	// legendText.
	panel  *panelRule
	legend legendCache

	// cells caches the rendered atoms a board and keyboard are made of: one
	// letter on one tile, one keycap, one empty slot. Each is a function of the
	// style set and its text, height and tile-outline flag, and nothing else,
	// and a frame draws seventy or more of them — through lipgloss's Render,
	// which costs about two microseconds and thirty allocations apiece. The
	// cache is keyed by the face, the height and the text, and is built lazily:
	// a theme is not paid for until a cell of it is drawn.
	//
	// It lives on the style set so setTheme invalidates it with the styles it
	// was rendered from. Meaningless after a theme change, it goes with the
	// styles.
	cells map[cellKey]string

	// emptyFrame is the box colour for an unscored tile when tile outlines are
	// on. It used to be built afresh for every empty row of every frame; it is
	// a function of the palette, so it belongs here. tileRejected is the typed
	// tile flashing a refusal, derived from tileActive after the theme's
	// overrides have been applied.
	emptyFrame   lipgloss.Style
	tileRejected lipgloss.Style
	// statValue is the profile's figure style: the accent, bold. Forcing bold
	// is deliberate (it always has been), so it is built after the overrides
	// exactly as the call site used to.
	statValue lipgloss.Style

	// hist caches the histogram's ramp and its rendered whole cells. A bar is
	// up to 24 styled cells and the profile draws one per attempt count, so
	// rendering them per frame cost more than the rest of the screen together.
	hist histCache

	// cal caches the calendar's five shades and the squares drawn in them. The
	// year is up to 378 squares, each drawn through one of ten styles.
	cal calCache

	// preview is the theme picker's tile-and-keycap sample, a pure function of
	// this style set. See renderThemePreview.
	preview      string
	previewReady bool

	// shimmer is the splash sweep's material: the ramp and each art rune
	// already rendered in each of its colours. See styles.shimmerCell.
	shimmer shimmerCache

	// standing is a live room's standings square in each of the three marks:
	// the tile's own background as the character's colour. A room draws one per
	// mark of every other player's latest row, every frame.
	standing      [3]string
	standingReady bool
}

// shimmerCache is the splash's sweep for one style set. A frame of the sweep
// draws the whole art, one cell at a time, and each cell's colour depends only
// on its column and the ramp; none of it changes while the style set is live.
type shimmerCache struct {
	ramp  []color.Color
	cells map[shimmerKey]string
}

type shimmerKey struct {
	step int
	r    rune
}

// calCache is the calendar's material for one style set.
type calCache struct {
	ramp   [5]color.Color
	plain  [5]string
	picked [5]string
	ready  bool
}

// histCache is the profile histogram's material for one style set: the colour
// ramp and the glyph already rendered in each of its colours.
type histCache struct {
	ramp  []color.Color
	whole [distributionWidth]string
	ready bool
}

// panelRule is the panel's border material for one content width: every cell of
// the top and bottom rules already rendered, and the two side glyphs.
//
// It exists because the rule is a gradient — blend works in Lab space, and each
// cell used to be its own Style.Render — and none of it depends on what is on
// the screen. The width is the cache's whole key: a panel only ever has one, and
// a terminal resize re-renders once.
type panelRule struct {
	width  int
	top    []string
	bottom []string
	left   string
	right  string
}

// legendCache is the colour key for one style set, rendered on first use.
type legendCache struct {
	text  string
	width int
	ready bool
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
		// Bold: the art is drawn in line characters, and a bold weight is what
		// makes them read as strokes rather than as punctuation.
		splash:  plain(c(theme.Accent)).Bold(true),
		helpBar: lipgloss.NewStyle().MarginTop(1),

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

	// Derived cell styles, built after the overrides so a theme that recolours
	// tile.active or tile.empty moves them with it. Both used to be constructed
	// afresh on every frame that drew a typed or empty row.
	s.emptyFrame = lipgloss.NewStyle().Foreground(s.tileEmpty.GetForeground())
	s.tileRejected = s.tileActive.Foreground(s.err.GetForeground())
	s.statValue = s.accent.Bold(true)
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
//
// Each line is measured once. Measuring is the expensive half — lipgloss.Width
// runs the ANSI parser and a grapheme iterator over the line — and the old
// version measured every line twice, once to find the widest and again to pad
// it. A block that is already square is returned as it is, which is the common
// case for anything that has just come out of JoinVertical.
func block(s string) string {
	lines := strings.Split(s, "\n")
	widths := make([]int, len(lines))
	width := 0
	for i, l := range lines {
		w := lipgloss.Width(l)
		widths[i] = w
		if w > width {
			width = w
		}
	}
	square := true
	for _, w := range widths {
		if w != width {
			square = false
			break
		}
	}
	if square {
		return s
	}

	var b strings.Builder
	b.Grow(len(s) + len(lines)*(width+1))
	for i, l := range lines {
		if i > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(l)
		if pad := width - widths[i]; pad > 0 {
			b.WriteString(strings.Repeat(" ", pad))
		}
	}
	return b.String()
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

// bodyBudget is how many lines a screen's body may take before the panel
// outgrows the terminal.
//
// Overflowing is not merely ugly: nothing here truncates, so the renderer drops
// the excess off the top of the screen (see hitMap.clip), taking the title, the
// close box and whatever else was up there with it. A screen that can shed or
// scroll should therefore know its budget.
//
// The subtraction is the chrome around the body: the help bar and the blank
// above it, and the panel's padding and border. There is no heading to take off:
// the panel's rule names the screen, and a body that repeated it cost two rows
// for nothing. A height of zero — the size before the first
// WindowSizeMsg — means unbounded, and is reported as 0 for callers to skip.
func bodyBudget(height int) int {
	if height <= 0 {
		return 0
	}
	const (
		help  = 2 // the help bar and the blank line above it
		frame = 2 // the panel's top and bottom border
	)
	return height - help - frame - 2*st.metric.PanelPadY
}

// affordableSections returns the leading run of extras that fits in the budget,
// given the sections already committed to and a blank line spacing each extra
// off. A budget of zero — an unmeasured terminal — is unbounded, which is what
// keeps the headless tests drawing whole screens.
//
// It measures by height rather than by row count because a section here is a
// block: a table, a paragraph or a row of tiles. aboutScreen sheds by counting
// rows instead, since every one of its rows is exactly one line.
func affordableSections(committed, optional []string, budget int) []string {
	if budget <= 0 {
		return optional
	}
	used := 0
	for _, s := range committed {
		used += lipgloss.Height(s)
	}
	for i, extra := range optional {
		if extra == "" {
			continue
		}
		if used+lipgloss.Height(extra)+1 > budget {
			return optional[:i]
		}
		used += lipgloss.Height(extra) + 1
	}
	return optional
}

// panelSteps are the widths a panel's content is drawn at, narrowest first.
// Sized to its own content, every screen had a different panel, and moving
// between them made the frame grow and shrink under the pointer. Two widths
// cover all of them at the default metrics: the narrow one holds the menu and
// the list-like screens and about (the widest of them, settings and how to
// play, are 49),
// and the wide one is the board's own width, which the result, the sprint and
// challenge setups and custom fit inside. About is narrow too, unless a long
// data path widens it. Content wider than the last step — the profile with its
// stats, or a theme with bigger tiles — keeps its own width.
var panelSteps = []int{49, 63}

// panelWidth is the content width a frame is drawn at: the smallest step that
// holds the content, but never more than the terminal has room for (room, from
// bodyWidth; zero is unmeasured, which is unbounded) and never less than the
// content itself. A narrow terminal therefore gets the panels it always had.
func panelWidth(content, room int) int {
	for _, step := range panelSteps {
		if step >= content {
			if room > 0 {
				step = min(step, room)
			}
			return max(step, content)
		}
	}
	return content
}

// centreLine centres one single-line string in width cells, splitting an odd
// gap the way lipgloss's Align(Center) does: the extra column goes right.
//
// It is the one-line case of lipgloss.NewStyle().Width(width).Align(Center)
// .Render(s), which re-wraps the whole string — parsing every escape code and
// grapheme in it — just to put spaces around it. The keyboard centres three
// already-rendered rows a frame, and each of those rows is a full line of
// styled keycaps; this pads them without touching what is inside.
//
// s must be one line. A multi-line string would need each line centred against
// the block's widest, which is what JoinVertical does and why callers use that
// for blocks.
func centreLine(s string, width int) string {
	w := lipgloss.Width(s)
	gap := width - w
	if gap <= 0 {
		return s
	}
	left := gap / 2
	return strings.Repeat(" ", left) + s + strings.Repeat(" ", gap-left)
}

// padLine right-pads one line to width cells, the one-line case of
// lipgloss.NewStyle().Width(width).Render(s) with its default left alignment.
// It is for a row of labels that are known to fit; a line wider than width is
// returned whole rather than wrapped, so it is not a general replacement.
func padLine(s string, width int) string {
	if w := lipgloss.Width(s); w < width {
		return s + strings.Repeat(" ", width-w)
	}
	return s
}

// joinVerticalCentre and joinVerticalLeft are lipgloss.JoinVertical's two
// alignments, measuring each line once instead of twice and without its
// per-line padding of blank lines through an extra style.
//
// Both produce exactly the bytes lipgloss does for content this UI writes (no
// tabs and no CRLF; anything containing either falls back to lipgloss so the
// helper stays total). JoinVertical measures every line twice — once in
// getLines and once to compute the padding — and it is called on whole screens,
// so the second pass is worth not paying. TestJoinVerticalMatchesLipgloss holds
// the two to the same answer, including the odd-column rule: lipgloss's Join
// puts the extra space on the left, unlike Align(Center), which puts it on the
// right.
func joinVerticalCentre(parts ...string) string {
	return joinVertical(parts, true)
}

func joinVerticalLeft(parts ...string) string {
	return joinVertical(parts, false)
}

func joinVertical(parts []string, centre bool) string {
	if len(parts) == 0 {
		return ""
	}
	if len(parts) == 1 {
		return parts[0]
	}

	var (
		lines  []string
		widths []int
		maxW   int
	)
	for _, part := range parts {
		if strings.IndexByte(part, '\t') >= 0 || strings.Contains(part, "\r\n") {
			if centre {
				return lipgloss.JoinVertical(lipgloss.Center, parts...)
			}
			return lipgloss.JoinVertical(lipgloss.Left, parts...)
		}
		for line := range strings.SplitSeq(part, "\n") {
			w := lipgloss.Width(line)
			lines = append(lines, line)
			widths = append(widths, w)
			if w > maxW {
				maxW = w
			}
		}
	}

	var b strings.Builder
	size := len(lines) - 1
	for i := range lines {
		size += len(lines[i])
		if pad := maxW - widths[i]; pad > 0 {
			size += pad
		}
	}
	b.Grow(size)
	for i, line := range lines {
		if i > 0 {
			b.WriteByte('\n')
		}
		pad := maxW - widths[i]
		if pad <= 0 {
			b.WriteString(line)
			continue
		}
		if centre {
			// JoinVertical's Center: round(w/2) on the left, the rest right.
			left := (pad + 1) / 2
			b.WriteString(strings.Repeat(" ", left))
			b.WriteString(line)
			b.WriteString(strings.Repeat(" ", pad-left))
			continue
		}
		b.WriteString(line)
		b.WriteString(strings.Repeat(" ", pad))
	}
	return b.String()
}

// frameContent joins the body and the help bar the way joinVerticalCentre +
// frameLines did, in one measurement pass, and returns the finished lines at
// the panel's content width.
//
// The frame is what every screen ends in, and the two joins used to measure
// every line of it twice: once to centre the body against the help bar, and
// once more to centre the result in the panel's width. The roundings are the
// two lipgloss steps applied in order (JoinVertical's extra column goes left,
// widen's extra column goes right of the enlarged block), so the bytes are the
// same; there is simply one fewer walk over the frame.
func frameContent(body, help string, room int) ([]string, int) {
	parts := [2]string{body, help}
	var (
		lines  []string
		widths []int
		maxW   int
	)
	for _, part := range parts {
		if strings.IndexByte(part, '\t') >= 0 || strings.Contains(part, "\r\n") {
			joined := joinVerticalCentre(body, help)
			return frameLines(joined, room)
		}
		for line := range strings.SplitSeq(part, "\n") {
			w := lipgloss.Width(line)
			lines = append(lines, line)
			widths = append(widths, w)
			if w > maxW {
				maxW = w
			}
		}
	}

	target := panelWidth(maxW, room)
	panelLeft := (target - maxW) / 2
	panelRight := target - maxW - panelLeft
	for i, line := range lines {
		gap := maxW - widths[i]
		// JoinVertical's Centre: the odd column goes left.
		left := (gap + 1) / 2
		right := gap - left
		lines[i] = strings.Repeat(" ", panelLeft+left) + line +
			strings.Repeat(" ", right+panelRight)
	}
	return lines, target
}

// frameLines squares the frame's lines and centres the block at the panel's
// width: every line is padded to the widest, then the block is centred in the
// step panelWidth chooses. It returns the lines and the width they share.
//
// It exists so the frame is measured once. The old path measured the composed
// frame to choose the width, measured it again in widen, and a third time in
// the panel's padding, then split the joined string apart again. The frame is
// the widest thing the app draws — the calendar's year is a whole screen of
// squares — so those passes are the largest single cost left in a render.
//
// room is bodyWidth's answer: zero is unmeasured, which is unbounded.
func frameLines(s string, room int) ([]string, int) {
	lines := strings.Split(s, "\n")
	widths := make([]int, len(lines))
	w := 0
	for i, line := range lines {
		widths[i] = lipgloss.Width(line)
		if widths[i] > w {
			w = widths[i]
		}
	}
	target := panelWidth(w, room)
	left := (target - w) / 2
	right := target - w - left
	l, r := strings.Repeat(" ", left), strings.Repeat(" ", right)
	for i, line := range lines {
		lines[i] = l + line + strings.Repeat(" ", w-widths[i]) + r
	}
	return lines, target
}

// bodyWidth is bodyBudget's other axis: how many columns a screen's body
// may take before the panel outgrows the terminal. A screen that can lay itself
// out more than one way — stacking what will not sit side by side — uses it to
// choose. Zero is unbounded, as it is there.
func bodyWidth(width int) int {
	if width <= 0 {
		return 0
	}
	const frame = 2 // the panel's left and right border
	return width - frame - 2*st.metric.PanelPadX
}

// renderPanel draws a rounded border around content with a title inlaid in the
// top edge, btop-style, in the theme's own border colour. The border is built by
// hand rather than via lipgloss's Border() so the title can sit inside the top
// rule. corner is an optional segment inlaid at the right end of that rule —
// the close box — and may be empty.
//
// It draws through the style set's cached rule material: the gradient and the
// per-cell styling are a function of the width and the palette, and the panel is
// on every frame. See panelRuleFor.
func renderPanel(title, status, corner, content string) string {
	if hasTabsOrCRLF(content) {
		lines, width := lipglossPanelBody(content)
		return drawPanel(title, status, corner, lines, width, st.panelRuleFor(width))
	}
	lines, contentWidth := squareLines(content)
	lines, width := panelBodyLines(lines, contentWidth)
	return drawPanel(title, status, corner, lines, width, st.panelRuleFor(width))
}

// renderPanelLit is renderPanel's win-accent twin: the frame is drawn in a
// colour that changes from frame to frame, so its rule material cannot be
// cached. Callers with nothing to say pass st.border to renderPanel instead.
func renderPanelLit(title, status, corner, content string, border lipgloss.Style) string {
	if hasTabsOrCRLF(content) {
		lines, width := lipglossPanelBody(content)
		return drawPanel(title, status, corner, lines, width,
			st.litPanelRule(width, border, st.accent.GetForeground()))
	}
	lines, contentWidth := squareLines(content)
	lines, width := panelBodyLines(lines, contentWidth)
	return drawPanel(title, status, corner, lines, width,
		st.litPanelRule(width, border, st.accent.GetForeground()))
}

// hasTabsOrCRLF reports content the hand-written padding does not interpret.
// Tabs are widened and CRLF is folded by lipgloss's reader, and nothing this UI
// writes contains either; a direct caller (a test, a future screen) that does
// gets the library's own answer rather than a near miss.
func hasTabsOrCRLF(s string) bool {
	return strings.IndexByte(s, '\t') >= 0 || strings.Contains(s, "\r\n")
}

// lipglossPanelBody is the padding path renderPanel used before the fast one:
// a style's Padding, which also squares the lines. It is kept for content that
// needs the library's own interpretation of whitespace.
func lipglossPanelBody(content string) ([]string, int) {
	inner := lipgloss.NewStyle().
		Padding(st.metric.PanelPadY, st.metric.PanelPadX).
		Render(content)
	lines := strings.Split(inner, "\n")
	return lines, lipgloss.Width(lines[0])
}

// renderPanelLines draws the frame around lines that have already been squared
// and centred to contentWidth cells — which is what frameLines produces for the
// composed frame. It is the frame's own path: the panel is handed the
// measurement rather than repeating it.
func renderPanelLines(title, status, corner string, lines []string, contentWidth int) string {
	lines, width := panelBodyLines(lines, contentWidth)
	return drawPanel(title, status, corner, lines, width, st.panelRuleFor(width))
}

// renderPanelLitLines is renderPanelLines for the accent. See renderPanelLit.
func renderPanelLitLines(title, status, corner string, lines []string, contentWidth int, border lipgloss.Style) string {
	lines, width := panelBodyLines(lines, contentWidth)
	return drawPanel(title, status, corner, lines, width,
		st.litPanelRule(width, border, st.accent.GetForeground()))
}

// squareLines splits content and pads every line to the widest, reporting the
// width they share. It is block's work without the join, for a caller that is
// about to handle the lines one at a time anyway.
func squareLines(content string) ([]string, int) {
	lines := strings.Split(content, "\n")
	widths := make([]int, len(lines))
	width := 0
	for i, line := range lines {
		widths[i] = lipgloss.Width(line)
		if widths[i] > width {
			width = widths[i]
		}
	}
	square := true
	for _, w := range widths {
		if w != width {
			square = false
			break
		}
	}
	if square {
		return lines, width
	}
	for i := range lines {
		if pad := width - widths[i]; pad > 0 {
			lines[i] += strings.Repeat(" ", pad)
		}
	}
	return lines, width
}

// panelBodyLines pads square content lines for the panel and reports the width
// the rule has to span. Every input line must already be contentWidth cells
// wide; squareLines and frameLines are the two ways to guarantee it.
//
// The padding is written here rather than through a lipgloss style because a
// style's Render re-parses every line of the content — ANSI state machine,
// grapheme widths, wrapping and alignment — to put a few spaces around it.
// The frame is padded once per message, and on a wide screen (the calendar's
// year, a long profile) that pass cost more than the rest of the frame. The
// output is the bytes Style.Padding would have produced: `padX` spaces on each
// side of every line and `padY` blank lines above and below, themselves as wide
// as the block.
//
// TestPanelBodyMatchesLipgloss holds the two to the same answer, so a change in
// lipgloss's padding shows up as a failing test rather than as a moved frame.
func panelBodyLines(lines []string, contentWidth int) ([]string, int) {
	padX, padY := st.metric.PanelPadX, st.metric.PanelPadY
	width := contentWidth + 2*padX

	out := make([]string, 0, len(lines)+2*padY)
	blank := strings.Repeat(" ", width)
	for range padY {
		out = append(out, blank)
	}
	left := strings.Repeat(" ", padX)
	for _, line := range lines {
		out = append(out, left+line+left)
	}
	for range padY {
		out = append(out, blank)
	}
	return out, width
}

// panelRuleFor returns the border material for a content width, rendering it
// once per width for this style set. A theme change builds a new set, and the
// colour-profile setter drops what is cached here, so nothing outlives the
// palette or the depth it was drawn for.
func (s *styles) panelRuleFor(width int) *panelRule {
	if s.panel != nil && s.panel.width == width {
		return s.panel
	}
	s.panel = s.litPanelRule(width, s.border, s.accent.GetForeground())
	return s.panel
}

// litPanelRule renders the rule material for one width in a given border style,
// uncached. The rules are drawn along a gradient rather than in one colour: lit
// at the corners, easing to the border's own colour across the middle. Both
// stops come from the palette, and on a terminal without the depth for it blend
// returns the border colour flat — exactly the frame this app drew before.
//
// Only the horizontals take it. Running it down the sides as well would light
// the whole frame at once, which is what a win does and what a win should keep
// to itself.
func (s *styles) litPanelRule(width int, border lipgloss.Style, accent color.Color) *panelRule {
	b := s.borderRunes()
	rule := blend(width+2, accent, border.GetForeground(), accent)
	r := &panelRule{
		width:  width,
		top:    make([]string, width+2),
		bottom: make([]string, width+2),
	}
	cell := func(i int, rune string) string {
		return border.Foreground(colorAt(rule, i)).Render(rune)
	}
	r.top[0] = cell(0, b.TopLeft)
	for i := range width {
		r.top[i+1] = cell(i+1, b.Top)
	}
	r.top[width+1] = cell(width+1, b.TopRight)
	r.bottom[0] = cell(0, b.BottomLeft)
	for i := range width {
		r.bottom[i+1] = cell(i+1, b.Bottom)
	}
	r.bottom[width+1] = cell(width+1, b.BottomRight)
	r.left = border.Render(b.Left)
	r.right = border.Render(b.Right)
	return r
}

// drawPanel assembles the frame from prepared border material.
func drawPanel(title, status, corner string, lines []string, width int, rule *panelRule) string {
	label := " " + title + " "
	// The status is inlaid the way the close box is, so it has to be measured
	// the same way — and given up rather than allowed to eat the rule. A frame
	// with no fill left has nothing to hang a status on.
	if status != "" {
		status = " " + status + " "
	}
	fill := width - 1 - lipgloss.Width(label) - lipgloss.Width(corner) - lipgloss.Width(status)
	if fill < minPanelFill {
		fill += lipgloss.Width(status)
		status = ""
	}
	if fill < 0 {
		fill = 0
	}

	var sb strings.Builder
	sb.Grow(width*(len(lines)+2) + len(title) + len(status) + len(corner))
	sb.WriteString(rule.top[0])
	sb.WriteString(rule.top[1])
	sb.WriteString(st.panelTitle.Render(label))
	// Where the fill starts, in rule coordinates: the two runes above plus the
	// label. Colouring each rune by its real column is what makes the gradient
	// continue through the inlays instead of restarting after them.
	start := 2 + lipgloss.Width(label)
	for i := range fill {
		sb.WriteString(rule.top[start+i])
	}
	// Muted rather than an element of its own: the status is a label on the
	// frame, and a new themeable name would cost a theme.Elements entry and a
	// docs/THEMES.md row for something every theme already colours.
	sb.WriteString(st.muted.Render(status))
	sb.WriteString(corner)
	sb.WriteString(rule.top[width+1])

	sb.WriteByte('\n')
	for _, l := range lines {
		sb.WriteString(rule.left)
		sb.WriteString(l)
		sb.WriteString(rule.right)
		sb.WriteByte('\n')
	}
	// The bottom takes the same colour per column as the top, so the two rules
	// read as one frame rather than as two that happen to match.
	sb.WriteString(rule.bottom[0])
	for i := range width {
		sb.WriteString(rule.bottom[i+1])
	}
	sb.WriteString(rule.bottom[width+1])
	return sb.String()
}

// minPanelFill is how much plain rule has to be left before the top border will
// carry a status. Below it the rule reads as a row of labels rather than as a
// frame, so the status is the part that goes.
const minPanelFill = 4
