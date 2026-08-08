package ui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/nxck2005/surmise/internal/banner"
	"github.com/nxck2005/surmise/internal/store"
	"github.com/nxck2005/surmise/internal/words"
)

// settingsScreen edits the persisted preferences that are not the theme: which
// mode the app opens on, and whether playing a mode changes that.
//
// Unlike the theme picker there is nothing to preview here, so there is no
// commit step — the root saves after every change and esc has nothing to undo.
// The screen holds the values it is editing rather than reading the store each
// frame, so rendering stays free of I/O.
type settingsScreen struct {
	length       int
	rememberLast bool

	// The splash's three: whether it appears, which art it draws, and how it
	// goes away. The last two are meaningless with the first off, and the screen
	// disables them rather than letting someone set a value that does nothing.
	splash     bool
	splashArt  string // a banner's name, or "random"
	splashMode splashMode

	cursor int
}

// The rows, in display order. Their order is the index carried by a click.
const (
	rowLength = iota
	rowRememberLast
	rowSplash
	rowSplashArt
	rowSplashDismiss
	settingRows
)

// The two columns, in display cells. Fixed rather than measured because the
// rows are few and their values are short: a widest-of helper would be more
// machinery than the numbers. A banner named wider than valueWidth is what
// would change that.
const (
	labelWidth = 16
	// Wide enough for the longest value there is with a space either side of it:
	// "timed + skip" filled the old 13 cells edge to edge and read as cramped
	// against the step arrow. A banner named wider than this is what would move
	// it again.
	valueWidth = 15
)

// reload takes the saved preferences. A zero length means nothing was ever
// chosen, which shows as the default the app actually opens on.
func (m *settingsScreen) reload(s store.Settings) {
	m.length = s.Length
	if !words.SupportedLength(m.length) {
		m.length = defaultLength
	}
	m.rememberLast = s.RememberLast

	m.splash = s.Splash != splashOff
	m.splashArt = s.SplashArt
	if _, ok := banner.Get(m.splashArt); !ok && m.splashArt != splashRandom {
		// Nothing chosen, or art that no longer ships. Either way the screen
		// shows what the app would actually draw, not a name with nothing behind
		// it — the same reasoning as a zero length showing as the default mode.
		m.splashArt = banner.Default().Name
	}
	m.splashMode, _ = parseSplashMode(s.SplashDismiss)

	m.cursor = 0
}

// enabled reports whether a row can be changed. The art and the dismissal are
// only meaningful while the splash is on, so with it off they are shown greyed
// out: no arrows, no click targets, and the cursor passes over them.
func (m *settingsScreen) enabled(row int) bool {
	switch row {
	case rowSplashArt, rowSplashDismiss:
		return m.splash
	default:
		return true
	}
}

// update moves between rows and steps the highlighted value, reporting whether
// anything changed (so the root can save) and whether to go back.
func (m *settingsScreen) update(msg tea.KeyPressMsg) (changed, back bool) {
	switch msg.String() {
	case "esc", "q":
		return false, true
	case "up", "k":
		m.move(-1)
	case "down", "j":
		m.move(1)
	case "left", "h":
		m.cycle(-1)
		return true, false
	case "right", "l", "enter", " ":
		m.cycle(1)
		return true, false
	}
	return false, false
}

// move steps the cursor to the next row it can do anything with, so a disabled
// row is passed over rather than landed on. Running out of enabled rows in that
// direction leaves the cursor where it was, which is what clamping did before.
func (m *settingsScreen) move(delta int) {
	for row := m.cursor + delta; row >= 0 && row < settingRows; row += delta {
		if m.enabled(row) {
			m.cursor = row
			return
		}
	}
}

// point selects the row under the pointer, so hovering moves the cursor the way
// it does on the menu and the puzzle list. A disabled row marks no target, so
// this is never called with one — the guard is here because point is the seam
// the mouse comes through, and a stale hit map from the frame before the splash
// was switched off would otherwise land on it.
func (m *settingsScreen) point(row int) {
	if row >= 0 && row < settingRows && m.enabled(row) {
		m.cursor = row
	}
}

// cycle steps the highlighted row's value. Both the arrow keys and the clicked
// ‹ › targets land here, so the two inputs cannot drift apart.
func (m *settingsScreen) cycle(delta int) {
	if !m.enabled(m.cursor) {
		return
	}
	switch m.cursor {
	case rowLength:
		m.length = stepLength(m.length, delta)
	case rowRememberLast:
		// Two values, so either direction is a toggle.
		m.rememberLast = !m.rememberLast
	case rowSplash:
		m.splash = !m.splash
	case rowSplashArt:
		m.splashArt = stepArt(m.splashArt, delta)
	case rowSplashDismiss:
		m.splashMode = stepMode(m.splashMode, delta)
	}
}

// stepArt walks the bundled banners and then "random", wrapping like the modes
// do. Random sits at the end rather than the start so stepping forward from the
// default shows the art itself first — the names are the point, and random is
// the answer for someone who has seen them all.
func stepArt(current string, delta int) string {
	choices := append(banner.Names(), splashRandom)
	return choices[wrap(indexOf(choices, current), delta, len(choices))]
}

func stepMode(current splashMode, delta int) splashMode {
	at := 0
	for i, mode := range splashModes {
		if mode == current {
			at = i
		}
	}
	return splashModes[wrap(at, delta, len(splashModes))]
}

func indexOf(all []string, want string) int {
	for i, s := range all {
		if s == want {
			return i
		}
	}
	return 0
}

// wrap steps an index around a list of n, in either direction.
func wrap(at, delta, n int) int {
	next := (at + delta) % n
	if next < 0 {
		next += n
	}
	return next
}

// stepLength walks the supported modes, wrapping at both ends: the list is
// short enough that wrapping is quicker than reversing direction.
func stepLength(current, delta int) int {
	at := 0
	for i, n := range words.Lengths {
		if n == current {
			at = i
		}
	}
	return words.Lengths[wrap(at, delta, len(words.Lengths))]
}

func (m *settingsScreen) view(h *hitMap) string {
	rows := []string{
		m.renderRow(h, rowLength, "default mode",
			fmt.Sprintf("%d letters", m.length)),
		m.renderRow(h, rowRememberLast, "remember last",
			onOff(m.rememberLast)),
		m.renderRow(h, rowSplash, "splash", onOff(m.splash)),
		m.renderRow(h, rowSplashArt, "splash art", m.splashArt),
		m.renderRow(h, rowSplashDismiss, "splash dismiss", m.splashMode.label()),
	}

	// The note is padded to the widest one there is, so moving the cursor does
	// not resize the panel around it.
	note := lipgloss.NewStyle().Width(noteWidth()).Align(lipgloss.Center).
		Render(st.muted.Render(m.note()))

	return lipgloss.JoinVertical(lipgloss.Center,
		st.title.Render("settings"),
		"",
		lipgloss.JoinVertical(lipgloss.Left, rows...),
		"",
		note,
	)
}

// renderRow lays out one setting: label on the left, the value flanked by the
// step arrows on the right. The value and the › both step forward — the value
// is the bigger target, and stepping backwards through three modes is rare
// enough to leave to the ‹.
func (m *settingsScreen) renderRow(h *hitMap, row int, label, value string) string {
	prev := action{kind: actSettingPrev, index: row}
	next := action{kind: actSettingNext, index: row}

	prefix := strings.Repeat(" ", lipgloss.Width(st.glyph.Cursor))
	labelStyle, valueStyle := st.muted, st.muted
	if row == m.cursor {
		prefix = st.cursor.Render(st.glyph.Cursor)
		labelStyle, valueStyle = st.text, st.accent
	}

	// A disabled row keeps its label and its value — it still says what it is
	// set to — but loses the arrows, which is the whole of the greying out. The
	// space they occupied is kept, so switching the splash off does not resize
	// the panel around the rows that are left.
	arrow := func(a action, glyph string) string {
		if !m.enabled(row) {
			return strings.Repeat(" ", lipgloss.Width(glyph))
		}
		style := st.muted
		if h.hovered(a) {
			style = st.hover(st.accent)
		}
		return h.mark(a, style.Render(glyph))
	}
	if h.hovered(next) {
		valueStyle = st.hover(valueStyle)
	}

	// Both columns are fixed-width so the rows line up under each other: the
	// label left-aligned, the value centred between its arrows.
	valueBox := lipgloss.NewStyle().Width(valueWidth).Align(lipgloss.Center)
	cell := valueBox.Render(valueStyle.Render(value))
	if m.enabled(row) {
		cell = h.mark(next, cell)
	}
	return prefix +
		labelStyle.Render(fmt.Sprintf("%-*s", labelWidth, label)) +
		arrow(prev, st.glyph.ValuePrev) +
		cell +
		arrow(next, st.glyph.ValueNext)
}

// notes are the one-line explanations, since neither setting says what it does
// and one of them quietly overrides the other. They are listed together so the
// screen can reserve the width of the longest.
var notes = struct {
	length, remembering, notRemembering string
	splashOn, splashOff                 string
	art, randomArt, dismiss             string
}{
	length:         "the mode new puzzles start in",
	remembering:    "playing a mode makes it the default",
	notRemembering: "the default stays whatever is set here",
	splashOn:       "the art drawn while the app starts",
	splashOff:      "turn it on to choose art and dismissal",
	art:            "which art the splash draws",
	randomArt:      "a different banner each launch",
	dismiss:        "how the splash gets out of the way",
}

func (m *settingsScreen) note() string {
	switch m.cursor {
	case rowRememberLast:
		if m.rememberLast {
			return notes.remembering
		}
		return notes.notRemembering
	case rowSplash:
		// With the splash off, the note explains the two dead rows below it —
		// they are the only thing on screen that has just changed.
		if !m.splash {
			return notes.splashOff
		}
		return notes.splashOn
	case rowSplashArt:
		if m.splashArt == splashRandom {
			return notes.randomArt
		}
		return notes.art
	case rowSplashDismiss:
		return notes.dismiss
	default:
		return notes.length
	}
}

func noteWidth() int {
	return max(lipgloss.Width(notes.length),
		lipgloss.Width(notes.remembering),
		lipgloss.Width(notes.notRemembering),
		lipgloss.Width(notes.splashOn),
		lipgloss.Width(notes.splashOff),
		lipgloss.Width(notes.art),
		lipgloss.Width(notes.randomArt),
		lipgloss.Width(notes.dismiss))
}

func onOff(b bool) string {
	if b {
		return "on"
	}
	return "off"
}

func (m *settingsScreen) help(h *hitMap) string {
	return renderHelp(h,
		helpItem{keys: "↑/↓", label: "move"},
		// Two buttons rather than one: the hint said ←/→ while the target only
		// ever stepped forward, so the bar promised something it would not do.
		helpItem{keys: "←", label: "back", act: action{kind: actSettingPrev, index: m.cursor}},
		helpItem{keys: "→", label: "change", act: action{kind: actSettingNext, index: m.cursor}},
		helpItem{keys: "esc", label: "menu", act: action{kind: actBack}},
	)
}
