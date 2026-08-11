package ui

import (
	"fmt"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/nxck2005/surmise/internal/banner"
	"github.com/nxck2005/surmise/internal/store"
	"github.com/nxck2005/surmise/internal/words"
)

// settingsScreen edits the persisted preferences that are not the theme:
// profile presentation, the opening mode, and the startup splash.
//
// Cycling preferences save on every step. The profile name is the one staged
// value: enter keeps its text draft and esc discards it. The screen holds every
// value it is editing rather than reading the store each frame, so rendering
// stays free of I/O.
type settingsScreen struct {
	length         int
	rememberLast   bool
	displayName    string
	nameBeforeEdit string
	editingName    bool

	// The splash's three: whether it appears, which art it draws, and how it
	// goes away. The last two are meaningless with the first off, and the screen
	// disables them rather than letting someone set a value that does nothing.
	splash     bool
	splashArt  string // a banner's name, or "random"
	splashMode splashMode
	splashTime time.Duration

	// motion is how much the board animates. Three values rather than a switch:
	// off is a real choice, and so is wanting more than the default.
	motion motion

	cursor int
}

// The rows, in display order. Their order is the index carried by a click.
const (
	rowLength = iota
	rowRememberLast
	rowProfileName
	// Before the splash block, so the dependent-row logic that block relies on
	// stays a contiguous run.
	rowMotion
	rowSplash
	rowSplashArt
	rowSplashDismiss
	rowSplashTime
	settingRows
)

// The two columns, in display cells. Fixed so every row shares one alignment.
// valueWidth also bounds the cosmetic profile name; its last cell is reserved
// for the caret while editing.
const (
	labelWidth          = 16
	valueWidth          = 20
	displayNameMaxWidth = valueWidth - 1
)

// reload takes the saved preferences. A zero length means nothing was ever
// chosen, which shows as the default the app actually opens on.
func (m *settingsScreen) reload(s store.Settings) {
	m.length = s.Length
	if !words.SupportedLength(m.length) {
		m.length = defaultLength
	}
	m.rememberLast = s.RememberLast
	m.displayName = sanitizeDisplayName(s.DisplayName)
	m.nameBeforeEdit = ""
	m.editingName = false

	m.splash = s.Splash != splashOff
	m.splashArt = s.SplashArt
	if _, ok := banner.Get(m.splashArt); !ok && m.splashArt != splashRandom {
		// Nothing chosen, or art that no longer ships. Either way the screen
		// shows what the app would actually draw, not a name with nothing behind
		// it — the same reasoning as a zero length showing as the default mode.
		m.splashArt = banner.Default().Name
	}
	m.splashMode, _ = parseSplashMode(s.SplashDismiss)
	m.splashTime, _ = parseSplashDuration(s.SplashMillis)

	m.motion, _ = parseMotion(s.Motion)

	m.cursor = 0
}

// enabled reports whether a row can be changed. A row whose value would do
// nothing is shown greyed out instead: no arrows, no click targets, and the
// cursor passes over it.
//
// The art and dismissal need the splash itself; the time needs a dismissal
// that is actually timed. While the name editor owns text input, every other
// row is temporarily inert so a typed key cannot change an unrelated setting.
func (m *settingsScreen) enabled(row int) bool {
	if m.editingName {
		return row == rowProfileName
	}
	switch row {
	case rowSplashArt, rowSplashDismiss:
		return m.splash
	case rowSplashTime:
		return m.splash && m.splashMode.timed()
	default:
		return true
	}
}

// update moves between rows, edits the profile name, and steps ordinary
// values. It reports whether a completed change must be persisted and whether
// the screen asked to go back.
func (m *settingsScreen) update(msg tea.KeyPressMsg) (changed, back bool) {
	if m.editingName {
		switch msg.String() {
		case "esc":
			m.finishNameEdit(false)
		case "enter":
			return m.finishNameEdit(true), false
		case "backspace":
			m.deleteNameRune()
		default:
			m.typeName(msg.Text)
		}
		return false, false
	}

	switch msg.String() {
	case "esc", "q":
		return false, true
	case "up", "k":
		m.move(-1)
	case "down", "j":
		m.move(1)
	case "left", "h":
		if m.cursor != rowProfileName {
			m.cycle(-1)
			return true, false
		}
	case "right", "l", "enter", " ":
		if m.cursor == rowProfileName {
			m.beginNameEdit()
		} else {
			m.cycle(1)
			return true, false
		}
	}
	return false, false
}

func (m *settingsScreen) beginNameEdit() {
	if m.editingName {
		return
	}
	m.cursor = rowProfileName
	m.nameBeforeEdit = m.displayName
	m.editingName = true
}

func (m *settingsScreen) finishNameEdit(save bool) bool {
	if !m.editingName {
		return false
	}
	m.editingName = false
	if !save {
		m.displayName = m.nameBeforeEdit
		return false
	}
	m.displayName = sanitizeDisplayName(m.displayName)
	return m.displayName != m.nameBeforeEdit
}

func (m *settingsScreen) deleteNameRune() {
	if !m.editingName || m.displayName == "" {
		return
	}
	_, size := utf8.DecodeLastRuneInString(m.displayName)
	m.displayName = m.displayName[:len(m.displayName)-size]
}

func (m *settingsScreen) typeName(text string) {
	if !m.editingName || text == "" {
		return
	}
	var b strings.Builder
	b.Grow(len(m.displayName) + len(text))
	b.WriteString(m.displayName)
	width := lipgloss.Width(m.displayName)
	for _, r := range text {
		r, ok := displayNameRune(r)
		if !ok {
			continue
		}
		runeWidth := lipgloss.Width(string(r))
		if width+runeWidth > displayNameMaxWidth {
			break
		}
		b.WriteRune(r)
		width += runeWidth
	}
	m.displayName = b.String()
}

// sanitizeDisplayName protects the terminal and the fixed-width settings row
// from a hand-edited settings file. The result is presentation only; this does
// not define an account-name or network-identity format.
func sanitizeDisplayName(name string) string {
	var b strings.Builder
	b.Grow(len(name))
	width := 0
	for _, r := range name {
		r, ok := displayNameRune(r)
		if !ok {
			continue
		}
		runeWidth := lipgloss.Width(string(r))
		if width+runeWidth > displayNameMaxWidth {
			break
		}
		b.WriteRune(r)
		width += runeWidth
	}
	return strings.TrimSpace(b.String())
}

func displayNameRune(r rune) (rune, bool) {
	if unicode.IsSpace(r) {
		return ' ', true
	}
	return r, unicode.IsPrint(r)
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
	case rowMotion:
		m.motion = stepMotion(m.motion, delta)
	case rowSplash:
		m.splash = !m.splash
	case rowSplashArt:
		m.splashArt = stepArt(m.splashArt, delta)
	case rowSplashDismiss:
		m.splashMode = stepMode(m.splashMode, delta)
	case rowSplashTime:
		m.splashTime = stepSplashDuration(m.splashTime, delta)
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

func stepMotion(current motion, delta int) motion {
	at := 0
	for i, m := range motionOrder {
		if m == current {
			at = i
		}
	}
	return motionOrder[wrap(at, delta, len(motionOrder))]
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
		m.renderNameRow(h),
		m.renderRow(h, rowMotion, "motion", m.motion.label()),
		m.renderRow(h, rowSplash, "splash", onOff(m.splash)),
		m.renderRow(h, rowSplashArt, "splash art", m.splashArt),
		m.renderRow(h, rowSplashDismiss, "splash dismiss", m.splashMode.label()),
		m.renderRow(h, rowSplashTime, "splash time", splashDurationLabel(m.splashTime)),
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

// renderRow lays out one cycling setting: label on the left, the value flanked
// by step arrows on the right. The value and the › both step forward — the
// value is the bigger target, and stepping backwards is rare enough to leave
// to the ‹.
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

func (m *settingsScreen) renderNameRow(h *hitMap) string {
	kind := actSettingNameEdit
	value := m.displayName
	if m.editingName {
		kind = actSettingNameDone
		value += st.glyph.Caret
	} else if value == "" {
		value = "not set"
	}
	a := action{kind: kind, index: rowProfileName}

	prefix := strings.Repeat(" ", lipgloss.Width(st.glyph.Cursor))
	labelStyle, valueStyle := st.muted, st.muted
	if m.cursor == rowProfileName {
		prefix = st.cursor.Render(st.glyph.Cursor)
		labelStyle, valueStyle = st.text, st.accent
	}
	if h.hovered(a) {
		valueStyle = st.hover(valueStyle)
	}

	valueBox := lipgloss.NewStyle().Width(valueWidth).Align(lipgloss.Center)
	cell := h.mark(a, valueBox.Render(valueStyle.Render(value)))
	return prefix +
		labelStyle.Render(fmt.Sprintf("%-*s", labelWidth, "profile name")) +
		strings.Repeat(" ", lipgloss.Width(st.glyph.ValuePrev)) +
		cell +
		strings.Repeat(" ", lipgloss.Width(st.glyph.ValueNext))
}

// notes are the one-line explanations for the selected preference. They are
// listed together so the screen can reserve the width of the longest.
var notes = struct {
	length, remembering, notRemembering string
	profileName                         string
	splashOn, splashOff                 string
	art, randomArt, dismiss             string
	splashTime, untimed                 string
	motionOff, motionOn, motionLoud     string
}{
	length:         "the mode new puzzles start in",
	remembering:    "playing a mode makes it the default",
	notRemembering: "the default stays whatever is set here",
	profileName:    "a local profile label, not an account or sign-in",
	splashOn:       "the art drawn while the app starts",
	splashOff:      "turn it on to choose art and dismissal",
	art:            "which art the splash draws",
	randomArt:      "a different banner each launch",
	dismiss:        "how the splash gets out of the way",
	splashTime:     "how long a timed splash stays up",
	untimed:        "this dismissal waits, so there is nothing to time",
	motionOff:      "the board changes at once, with no animation",
	motionOn:       "tiles turn one at a time, and a win is marked",
	motionLoud:     "the same feedback, slower and repeated",
}

func (m *settingsScreen) note() string {
	switch m.cursor {
	case rowRememberLast:
		if m.rememberLast {
			return notes.remembering
		}
		return notes.notRemembering
	case rowProfileName:
		return notes.profileName
	case rowMotion:
		switch m.motion {
		case motionOff:
			return notes.motionOff
		case motionPronounced:
			return notes.motionLoud
		default:
			return notes.motionOn
		}
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
		// The dismissal is the row that disables the one under it, so it is
		// where saying so belongs.
		if !m.splashMode.timed() {
			return notes.untimed
		}
		return notes.dismiss
	case rowSplashTime:
		return notes.splashTime
	default:
		return notes.length
	}
}

func noteWidth() int {
	return max(lipgloss.Width(notes.length),
		lipgloss.Width(notes.remembering),
		lipgloss.Width(notes.notRemembering),
		lipgloss.Width(notes.profileName),
		lipgloss.Width(notes.splashOn),
		lipgloss.Width(notes.splashOff),
		lipgloss.Width(notes.art),
		lipgloss.Width(notes.randomArt),
		lipgloss.Width(notes.dismiss),
		lipgloss.Width(notes.splashTime),
		lipgloss.Width(notes.untimed),
		lipgloss.Width(notes.motionOff),
		lipgloss.Width(notes.motionOn),
		lipgloss.Width(notes.motionLoud))
}

func onOff(b bool) string {
	if b {
		return "on"
	}
	return "off"
}

func (m *settingsScreen) help(h *hitMap) string {
	if m.editingName {
		return renderHelp(h,
			helpItem{keys: "⌫", label: "erase", act: action{kind: actSettingNameBackspace}},
			helpItem{keys: "enter", label: "save", act: action{kind: actSettingNameDone}},
			helpItem{keys: "esc", label: "cancel", act: action{kind: actSettingNameCancel}},
		)
	}
	if m.cursor == rowProfileName {
		return renderHelp(h,
			helpItem{keys: "↑/↓", label: "move"},
			helpItem{keys: "enter", label: "edit", act: action{kind: actSettingNameEdit}},
			helpItem{keys: "esc", label: "menu", act: action{kind: actBack}},
		)
	}
	return renderHelp(h,
		helpItem{keys: "↑/↓", label: "move"},
		// Two buttons rather than one: the hint said ←/→ while the target only
		// ever stepped forward, so the bar promised something it would not do.
		helpItem{keys: "←", label: "back", act: action{kind: actSettingPrev, index: m.cursor}},
		helpItem{keys: "→", label: "change", act: action{kind: actSettingNext, index: m.cursor}},
		helpItem{keys: "esc", label: "menu", act: action{kind: actBack}},
	)
}
