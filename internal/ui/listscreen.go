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

	// confirmDelete arms the delete prompt for the row under the cursor. It is
	// the same arm-then-confirm shape as the board's confirmNew, and it must
	// never outlive the row it was armed on, so anything that moves the cursor
	// or leaves the screen clears it.
	confirmDelete bool
}

func (m *listScreen) reload(s store.Store) {
	m.items, m.err = s.List()
	m.cursor, m.offset = 0, 0
	m.confirmDelete = false
}

// refresh re-reads the puzzles while holding the cursor where the player left
// it, clamped to whatever is still there. reload resets to the top, which is
// right on entering the screen and wrong after deleting a row.
func (m *listScreen) refresh(s store.Store) {
	cursor := m.cursor
	m.reload(s)
	m.cursor = min(max(cursor, 0), max(len(m.items)-1, 0))
	m.clampOffset()
}

// selected returns the highlighted puzzle, if any.
func (m *listScreen) selected() (store.Summary, bool) {
	if m.cursor < 0 || m.cursor >= len(m.items) {
		return store.Summary{}, false
	}
	return m.items[m.cursor], true
}

// update moves the cursor. It reports whether the player chose a puzzle to
// open, asked to delete one, or asked to go back.
func (m *listScreen) update(msg tea.KeyPressMsg) (open, del, back bool) {
	key := msg.String()

	// An armed delete prompt consumes the next key, so the confirmation cannot
	// be walked into by a keystroke meant for the list underneath.
	if m.confirmDelete {
		m.confirmDelete = false
		switch key {
		case "d", "y", "enter":
			return false, true, false
		default:
			// Anything else cancels, and is not also acted on: the player was
			// answering a question, not driving the list.
			return false, false, false
		}
	}

	switch key {
	case "esc", "q":
		return false, false, true
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
			return true, false, false
		}
	case "d":
		if _, ok := m.selected(); ok {
			m.confirmDelete = true
		}
	}
	m.clampOffset()
	return false, false, false
}

func (m *listScreen) move(delta int) {
	if len(m.items) == 0 {
		return
	}
	m.cursor = min(max(m.cursor+delta, 0), len(m.items)-1)
	m.confirmDelete = false
}

// scroll pans the visible window without moving the selection, which is what a
// mouse wheel should do. The selection follows the pointer instead (see the
// hover handling in app.go), so the two never fight.
func (m *listScreen) scroll(delta int) {
	m.offset = min(max(m.offset+delta, 0), max(len(m.items)-visibleRows, 0))
}

// point selects the row the pointer is over. Hovering moves the selection, the
// way it does on a web page, so a click has no separate "select" step.
func (m *listScreen) point(row int) {
	if row >= 0 && row < len(m.items) {
		// Moving to a different row disarms: a prompt asking about the puzzle
		// the pointer has just left would be answering for the wrong one.
		if row != m.cursor {
			m.confirmDelete = false
		}
		m.cursor = row
	}
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

func (m *listScreen) view(h *hitMap) string {
	var b strings.Builder
	b.WriteString(st.title.Render("puzzles"))
	b.WriteString("\n\n")

	switch {
	case m.err != nil:
		b.WriteString(st.err.Render(fmt.Sprintf("could not read puzzles: %v", m.err)))
		return b.String()
	case len(m.items) == 0:
		b.WriteString(st.muted.Render("no puzzles yet — start one from the menu"))
		return b.String()
	}

	end := min(m.offset+visibleRows, len(m.items))
	for i := m.offset; i < end; i++ {
		// One click opens a puzzle; esc comes straight back, so there is no
		// need to make selecting a separate step.
		b.WriteString(h.mark(action{kind: actListRow, index: i},
			m.renderRow(m.items[i], i == m.cursor)))
		b.WriteString("\n")
	}

	if len(m.items) > visibleRows {
		b.WriteString(st.muted.Render(fmt.Sprintf("\n  %d–%d of %d",
			m.offset+1, end, len(m.items))))
	}
	if prompt := m.deletePrompt(h); prompt != "" {
		b.WriteString("\n\n" + prompt)
	}
	return b.String()
}

// deletePrompt is the armed confirmation, drawn under the rows. Both halves are
// click targets, so the mouse can answer it as well as the keyboard.
func (m *listScreen) deletePrompt(h *hitMap) string {
	if !m.confirmDelete {
		return ""
	}
	s, ok := m.selected()
	if !ok {
		return ""
	}
	confirm := action{kind: actDeletePuzzle, index: m.cursor}
	cancel := action{kind: actCancelDelete}
	return st.muted.Render(fmt.Sprintf("delete #%s? ", game.Code(s.ID))) +
		h.mark(confirm, st.accent.Render("d")) +
		st.muted.Render(" to delete · ") +
		h.mark(cancel, st.accent.Render("esc")) +
		st.muted.Render(" to keep it")
}

// renderRow lays out one puzzle. The status word is the only coloured part, so
// styles are composed side by side rather than nested — wrapping an
// already-styled string in another style corrupts its escape codes.
func (m *listScreen) renderRow(s store.Summary, selected bool) string {
	statusText, statusColor := describeStatus(s)

	// Columns are padded before styling, since padding a styled string by byte
	// width counts invisible escape codes as characters.
	// Codes are fixed-width, so this column needs no padding to stay aligned.
	left := fmt.Sprintf("#%s %-10s ", game.Code(s.ID), fmt.Sprintf("%d letters", s.Length))
	status := fmt.Sprintf("%-14s", statusText)
	right := " " + formatDuration(s.Elapsed)

	rowStyle := st.muted
	prefix := strings.Repeat(" ", lipgloss.Width(st.glyph.Cursor))
	if selected {
		rowStyle = st.text
		prefix = st.cursor.Render(st.glyph.Cursor)
	}

	return prefix +
		rowStyle.Render(left) +
		lipgloss.NewStyle().Foreground(statusColor).Render(status) +
		rowStyle.Render(right)
}

func describeStatus(s store.Summary) (text string, c color.Color) {
	switch s.Status {
	case game.Won:
		return fmt.Sprintf("solved %d/%d", s.Attempts, s.Length+1), st.statusWon
	case game.Lost:
		return "failed", st.statusLost
	default:
		return fmt.Sprintf("in play %d/%d", s.Attempts, s.Length+1), st.statusPlaying
	}
}

func (m *listScreen) help(h *hitMap) string {
	if m.confirmDelete {
		// The prompt is the only thing worth answering while it is up, so the
		// button bar stops offering anything that would answer it by accident.
		return renderHelp(h,
			helpItem{keys: "d", label: "delete", act: action{kind: actDeletePuzzle, index: m.cursor}},
			helpItem{keys: "esc", label: "keep", act: action{kind: actCancelDelete}},
		)
	}
	return renderHelp(h,
		helpItem{keys: "↑/↓", label: "move"},
		helpItem{keys: "enter", label: "open", act: action{kind: actListRow, index: m.cursor}},
		helpItem{keys: "d", label: "delete", act: action{kind: actDeletePuzzle, index: m.cursor}},
		helpItem{keys: "esc", label: "menu", act: action{kind: actBack}},
	)
}
