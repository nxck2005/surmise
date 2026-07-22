package ui

import (
	"fmt"
	"image/color"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/nxck2005/wortle/internal/game"
	"github.com/nxck2005/wortle/internal/store"
)

// visibleRows caps how many puzzles are drawn at once, so the list scrolls
// instead of overflowing short terminals.
const visibleRows = 12

// listScreen browses saved puzzles. Unfinished ones can be resumed, finished
// ones reviewed.
type listScreen struct {
	items  []store.Summary
	cursor int
	offset int // index of the first visible row
	err    error
}

func (m *listScreen) reload(s store.Store) {
	m.items, m.err = s.List()
	m.cursor, m.offset = 0, 0
}

// selected returns the highlighted puzzle, if any.
func (m *listScreen) selected() (store.Summary, bool) {
	if m.cursor < 0 || m.cursor >= len(m.items) {
		return store.Summary{}, false
	}
	return m.items[m.cursor], true
}

// update moves the cursor. It reports whether the player chose a puzzle to
// open, and whether they asked to go back.
func (m *listScreen) update(msg tea.KeyPressMsg) (open, back bool) {
	switch msg.String() {
	case "esc", "q":
		return false, true
	case "up", "k":
		m.move(-1)
	case "down", "j":
		m.move(1)
	case "home", "g":
		m.cursor = 0
	case "end", "G":
		m.cursor = len(m.items) - 1
	case "enter":
		if _, ok := m.selected(); ok {
			return true, false
		}
	}
	m.clampOffset()
	return false, false
}

func (m *listScreen) move(delta int) {
	if len(m.items) == 0 {
		return
	}
	m.cursor = min(max(m.cursor+delta, 0), len(m.items)-1)
}

// clampOffset scrolls the window just far enough to keep the cursor visible.
func (m *listScreen) clampOffset() {
	if m.cursor < m.offset {
		m.offset = m.cursor
	}
	if m.cursor >= m.offset+visibleRows {
		m.offset = m.cursor - visibleRows + 1
	}
	m.offset = max(m.offset, 0)
}

func (m *listScreen) view() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("puzzles"))
	b.WriteString("\n\n")

	switch {
	case m.err != nil:
		b.WriteString(errorStyle.Render(fmt.Sprintf("could not read puzzles: %v", m.err)))
		return b.String()
	case len(m.items) == 0:
		b.WriteString(mutedStyle.Render("no puzzles yet — start one from the menu"))
		return b.String()
	}

	end := min(m.offset+visibleRows, len(m.items))
	for i := m.offset; i < end; i++ {
		b.WriteString(m.renderRow(m.items[i], i == m.cursor))
		b.WriteString("\n")
	}

	if len(m.items) > visibleRows {
		b.WriteString(mutedStyle.Render(fmt.Sprintf("\n  %d–%d of %d",
			m.offset+1, end, len(m.items))))
	}
	return b.String()
}

// renderRow lays out one puzzle. The status word is the only coloured part, so
// styles are composed side by side rather than nested — wrapping an
// already-styled string in another style corrupts its escape codes.
func (m *listScreen) renderRow(s store.Summary, selected bool) string {
	statusText, statusColor := describeStatus(s)

	// Columns are padded before styling, since padding a styled string by byte
	// width counts invisible escape codes as characters.
	left := fmt.Sprintf("#%-5d %-10s ", s.Number, fmt.Sprintf("%d letters", s.Length))
	status := fmt.Sprintf("%-14s", statusText)
	right := " " + formatDuration(s.Elapsed)

	rowStyle := mutedStyle
	prefix := "  "
	if selected {
		rowStyle = textStyle
		prefix = accentStyle.Render("› ")
	}

	return prefix +
		rowStyle.Render(left) +
		lipgloss.NewStyle().Foreground(statusColor).Render(status) +
		rowStyle.Render(right)
}

func describeStatus(s store.Summary) (text string, c color.Color) {
	switch s.Status {
	case game.Won:
		return fmt.Sprintf("solved %d/%d", s.Attempts, s.Length+1), colorCorrect
	case game.Lost:
		return "failed", colorError
	default:
		return fmt.Sprintf("in play %d/%d", s.Attempts, s.Length+1), colorAccent
	}
}

func (m *listScreen) help() string {
	return helpStyle.Render("↑/↓ move · enter open · esc menu")
}
