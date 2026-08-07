package ui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

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
	cursor       int
}

// The rows, in display order. Their order is the index carried by a click.
const (
	rowLength = iota
	rowRememberLast
	settingRows
)

// The two columns, in display cells. Fixed rather than measured because there
// are two rows: a widest-of helper would be more machinery than the numbers.
const (
	labelWidth = 16
	valueWidth = 13
)

// reload takes the saved preferences. A zero length means nothing was ever
// chosen, which shows as the default the app actually opens on.
func (m *settingsScreen) reload(s store.Settings) {
	m.length = s.Length
	if !words.SupportedLength(m.length) {
		m.length = defaultLength
	}
	m.rememberLast = s.RememberLast
	m.cursor = 0
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

func (m *settingsScreen) move(delta int) {
	m.cursor = min(max(m.cursor+delta, 0), settingRows-1)
}

// point selects the row under the pointer, so hovering moves the cursor the way
// it does on the menu and the puzzle list.
func (m *settingsScreen) point(row int) {
	if row >= 0 && row < settingRows {
		m.cursor = row
	}
}

// cycle steps the highlighted row's value. Both the arrow keys and the clicked
// ‹ › targets land here, so the two inputs cannot drift apart.
func (m *settingsScreen) cycle(delta int) {
	switch m.cursor {
	case rowLength:
		m.length = stepLength(m.length, delta)
	case rowRememberLast:
		// Two values, so either direction is a toggle.
		m.rememberLast = !m.rememberLast
	}
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
	next := (at + delta) % len(words.Lengths)
	if next < 0 {
		next += len(words.Lengths)
	}
	return words.Lengths[next]
}

func (m *settingsScreen) view(h *hitMap) string {
	rows := []string{
		m.renderRow(h, rowLength, "default mode",
			fmt.Sprintf("%d letters", m.length)),
		m.renderRow(h, rowRememberLast, "remember last",
			onOff(m.rememberLast)),
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

	arrow := func(a action, glyph string) string {
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
	return prefix +
		labelStyle.Render(fmt.Sprintf("%-*s", labelWidth, label)) +
		arrow(prev, st.glyph.ValuePrev) +
		h.mark(next, valueBox.Render(valueStyle.Render(value))) +
		arrow(next, st.glyph.ValueNext)
}

// notes are the one-line explanations, since neither setting says what it does
// and one of them quietly overrides the other. They are listed together so the
// screen can reserve the width of the longest.
var notes = struct{ length, remembering, notRemembering string }{
	length:         "the mode new puzzles start in",
	remembering:    "playing a mode makes it the default",
	notRemembering: "the default stays whatever is set here",
}

func (m *settingsScreen) note() string {
	switch {
	case m.cursor != rowRememberLast:
		return notes.length
	case m.rememberLast:
		return notes.remembering
	default:
		return notes.notRemembering
	}
}

func noteWidth() int {
	return max(lipgloss.Width(notes.length),
		lipgloss.Width(notes.remembering),
		lipgloss.Width(notes.notRemembering))
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
