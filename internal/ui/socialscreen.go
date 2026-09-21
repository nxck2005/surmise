package ui

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

const (
	socialNewChallenge = iota
	socialEnterChallenge
	socialCustom
	socialRows
)

var socialLabels = []string{"new challenge", "enter code", "custom puzzle"}

// socialScreen groups the offline ways two people can share a board without
// growing the already-full main menu.
type socialScreen struct{ cursor int }

func (m *socialScreen) update(msg tea.KeyPressMsg) (choice int, selected, back bool) {
	switch msg.String() {
	case "esc", "q":
		return 0, false, true
	case "up", "k":
		m.point(m.cursor - 1)
	case "down", "j":
		m.point(m.cursor + 1)
	case "enter", " ":
		return m.cursor, true, false
	}
	return 0, false, false
}

func (m *socialScreen) point(row int) bool {
	row = min(max(row, 0), socialRows-1)
	if m.cursor == row {
		return false
	}
	m.cursor = row
	return true
}

func (m *socialScreen) view(h *hitMap) string {
	width := 0
	for _, label := range socialLabels {
		width = max(width, lipgloss.Width(label))
	}
	blank := strings.Repeat(" ", lipgloss.Width(st.glyph.Cursor))
	trail := strings.Repeat(" ", lipgloss.Width(st.glyph.CursorRight))
	rows := make([]string, socialRows)
	for i, label := range socialLabels {
		prefix, suffix, style := blank, trail, st.muted
		if i == m.cursor {
			prefix = st.cursor.Render(st.glyph.Cursor)
			suffix = st.cursor.Render(st.glyph.CursorRight)
			style = st.text
		}
		cell := lipgloss.NewStyle().Width(width).Align(lipgloss.Center).Render(style.Render(label))
		rows[i] = h.mark(action{kind: actSocialChoice, index: i}, prefix+cell+suffix)
	}
	return titled("social play", strings.Join(rows, "\n"))
}

func (m *socialScreen) help(h *hitMap) string {
	return renderHelp(h,
		helpItem{keys: "↑/↓", label: "move"},
		helpItem{keys: "enter", label: "select", act: action{kind: actSocialChoice, index: m.cursor}},
		helpItem{keys: "esc", label: "menu", act: action{kind: actBack}},
	)
}
