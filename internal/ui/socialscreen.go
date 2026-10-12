package ui

// Social play: the ways to start a board with somebody else. With network on
// and a room store present, rooms the install is still in lead the list, then
// making and joining one; the three offline rows are always below them.

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/nxck2005/surmise/internal/store"
)

var socialLabels = []string{"new challenge", "enter code", "custom puzzle"}

type socialKind int

const (
	socialRejoin socialKind = iota
	socialMakeRoom
	socialJoinRoom
	socialNewChallenge
	socialEnterChallenge
	socialCustom
)

// socialRow is one row of the screen. Rejoin rows carry the room they would
// open; the rest are the fixed set.
type socialRow struct {
	kind  socialKind
	label string

	// entry is the active room a rejoin row opens.
	entry store.RoomEntry
}

// socialScreen groups the offline ways two people can share a board without
// growing the already-full main menu.
type socialScreen struct {
	rows   []socialRow
	cursor int

	// note is the screen's own message, under the rows. An error shows in the
	// error style; a notice does not.
	note    string
	noteErr bool

	// confirmLeave is armed by x on a rejoin row and carried out by x again.
	// Leaving a room is not undoable, so it confirms like deleting does.
	confirmLeave bool
}

// openSocialScreen builds the rows: the rooms this install is in (network
// permitting), then make and join, then the three offline rows.
func (m *Model) openSocialScreen() {
	s := socialScreen{}
	if m.roomsOnline() {
		for _, entry := range m.activeRooms() {
			word := "any time"
			if entry.Mode == "live" {
				word = "together"
			}
			s.rows = append(s.rows, socialRow{
				kind:  socialRejoin,
				label: fmt.Sprintf("room %s · %s", entry.Code, word),
				entry: entry,
			})
		}
		s.rows = append(s.rows,
			socialRow{kind: socialMakeRoom, label: "make room"},
			socialRow{kind: socialJoinRoom, label: "join room"},
		)
	}
	for i, label := range socialLabels {
		kind := socialNewChallenge
		switch i {
		case 1:
			kind = socialEnterChallenge
		case 2:
			kind = socialCustom
		}
		s.rows = append(s.rows, socialRow{kind: kind, label: label})
	}
	m.social = s
	m.screen = screenSocial
}

// socialAct is what a key asked the screen to do.
type socialAct int

const (
	socialStay socialAct = iota
	socialSelect
	socialLeave
)

func (m *socialScreen) update(msg tea.KeyPressMsg) (choice int, act socialAct, back bool) {
	if m.confirmLeave {
		// While the leave prompt is up, everything but its own confirm and
		// cancel is swallowed: a prompt an unattended key could carry out is
		// how rooms get left by accident.
		switch msg.String() {
		case "x":
			return m.cursor, socialLeave, false
		case "esc":
			m.confirmLeave = false
		}
		return 0, socialStay, false
	}
	switch msg.String() {
	case "esc", "q":
		return 0, socialStay, true
	case "up", "k":
		m.point(m.cursor - 1)
	case "down", "j":
		m.point(m.cursor + 1)
	case "x":
		// x on a room row asks to leave it; the prompt below is what x again
		// (or click) carries out, and everything else waits until it is done.
		if m.cursor < len(m.rows) && m.rows[m.cursor].kind == socialRejoin {
			m.confirmLeave = true
		}
	case "enter", " ":
		return m.cursor, socialSelect, false
	}
	return 0, socialStay, false
}

func (m *socialScreen) point(row int) bool {
	row = min(max(row, 0), len(m.rows)-1)
	if m.cursor == row {
		return false
	}
	m.cursor = row
	m.confirmLeave = false
	m.note, m.noteErr = "", false
	return true
}

func (m *socialScreen) view(h *hitMap) string {
	width := 0
	for _, row := range m.rows {
		width = max(width, lipgloss.Width(row.label))
	}
	blank := strings.Repeat(" ", lipgloss.Width(st.glyph.Cursor))
	trail := strings.Repeat(" ", lipgloss.Width(st.glyph.CursorRight))
	rows := make([]string, len(m.rows))
	for i, row := range m.rows {
		prefix, suffix, style := blank, trail, st.muted
		if i == m.cursor {
			prefix = st.cursor.Render(st.glyph.Cursor)
			suffix = st.cursor.Render(st.glyph.CursorRight)
			style = st.text
		}
		cell := lipgloss.NewStyle().Width(width).Align(lipgloss.Center).Render(style.Render(row.label))
		atom := action{kind: actSocialChoice, index: i}
		if row.kind == socialRejoin && !m.confirmLeave {
			atom = action{kind: actSocialChoice, index: i}
		}
		rows[i] = h.mark(atom, prefix+cell+suffix)
	}

	sections := []string{block(strings.Join(rows, "\n"))}
	if len(m.rows) == 0 {
		return lipgloss.JoinVertical(lipgloss.Center, sections...)
	}
	if m.confirmLeave {
		prompt := st.muted.Render(fmt.Sprintf("leave room %s? ", m.rows[m.cursor].entry.Code)) +
			m.markLeave(h, st.accent.Render("x")) +
			st.muted.Render(" to leave · ") +
			m.markStay(h, st.accent.Render("esc")) +
			st.muted.Render(" to stay")
		sections = append(sections, "", prompt)
	} else if m.note != "" {
		style := st.muted
		if m.noteErr {
			style = st.err
		}
		sections = append(sections, "", style.Render(safeText(m.note)))
	}
	return lipgloss.JoinVertical(lipgloss.Center, sections...)
}

func (m *socialScreen) markLeave(h *hitMap, s string) string {
	return h.mark(action{kind: actSocialLeaveConfirm}, s)
}

func (m *socialScreen) markStay(h *hitMap, s string) string {
	return h.mark(action{kind: actSocialLeaveCancel}, s)
}

func (m *socialScreen) help(h *hitMap) string {
	if len(m.rows) == 0 {
		// The screen has not been opened yet — a frame nobody drew a row for.
		return renderHelp(h, helpItem{keys: "esc", label: "menu", act: action{kind: actBack}})
	}
	if m.confirmLeave {
		return renderHelp(h,
			helpItem{keys: "x", label: "leave", act: action{kind: actSocialLeaveConfirm}},
			helpItem{keys: "esc", label: "stay", act: action{kind: actSocialLeaveCancel}},
		)
	}
	row := m.rows[m.cursor]
	if row.kind == socialRejoin {
		return renderHelp(h,
			helpItem{keys: "↑/↓", label: "move"},
			helpItem{keys: "enter", label: "open", act: action{kind: actSocialChoice, index: m.cursor}},
			helpItem{keys: "x", label: "leave", act: action{kind: actSocialLeave, index: m.cursor}},
			helpItem{keys: "esc", label: "menu", act: action{kind: actBack}},
		)
	}
	return renderHelp(h,
		helpItem{keys: "↑/↓", label: "move"},
		helpItem{keys: "enter", label: "select", act: action{kind: actSocialChoice, index: m.cursor}},
		helpItem{keys: "esc", label: "menu", act: action{kind: actBack}},
	)
}
