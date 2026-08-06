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

// defaultRows is how many rows a list shows when it has not been told the
// terminal's height — before the first WindowSizeMsg, and in the headless
// tests. It is the old fixed window, kept so those keep rendering as they did.
const defaultRows = 12

// minRows is the shortest useful window. Below this the list is not worth
// scrolling and the screen is going to be cramped whatever we do.
const minRows = 3

// windowRows is how many rows a list of n items may draw, given the terminal's
// height. The list is the whole body of its screen, so its budget is the
// screen's, less the counter line and the blank above it once there is
// something to count.
func windowRows(height, n int) int {
	budget := bodyBudget(height)
	if budget <= 0 {
		return defaultRows
	}
	if n > budget {
		budget -= 2 // the "3–14 of 27" counter and its spacing
	}
	return max(budget, minRows)
}

// listScreen browses saved puzzles. Unfinished ones can be resumed, finished
// ones reviewed.
type listScreen struct {
	items  []store.Summary
	cursor int
	offset int // index of the first visible row
	err    error

	// height is the terminal's, pushed down by the root so the window can be
	// as tall as there is room for. Zero means unmeasured.
	height int

	// confirmDelete arms the delete prompt for the row under the cursor. It is
	// the same arm-then-confirm shape as the board's confirmNew, and it must
	// never outlive the row it was armed on, so anything that moves the cursor
	// or leaves the screen clears it.
	confirmDelete bool
}

func (m *listScreen) resize(h int) {
	m.height = h
	m.clampOffset()
}

// rows is the size of the visible window.
func (m *listScreen) rows() int { return windowRows(m.height, len(m.items)) }

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
		m.jumpTop()
	case "end", "G":
		m.jumpBottom()
	case "enter":
		if _, ok := m.selected(); ok {
			return true, false, false
		}
	case "d":
		if _, ok := m.selected(); ok {
			m.confirmDelete = true
		}
	}
	return false, false, false
}

// jumpTop and jumpBottom are the ends of the list. They are methods because the
// keys and the counter's click targets both go through them.
func (m *listScreen) jumpTop() {
	m.cursor = 0
	m.clampOffset()
}

func (m *listScreen) jumpBottom() {
	m.cursor = max(len(m.items)-1, 0)
	m.clampOffset()
}

func (m *listScreen) move(delta int) {
	if len(m.items) == 0 {
		return
	}
	m.cursor = min(max(m.cursor+delta, 0), len(m.items)-1)
	m.confirmDelete = false
	m.clampOffset()
}

// scroll pans the visible window without moving the selection, which is what a
// mouse wheel should do. The selection follows the pointer instead (see the
// hover handling in app.go), so the two never fight.
//
// Nothing clamps afterwards, deliberately: clampOffset belongs to whatever
// moved the *cursor*. Running it after every key — as this screen used to —
// dragged the window back the moment the player touched the keyboard, so a
// wheel scroll could not survive a keystroke.
func (m *listScreen) scroll(delta int) {
	m.offset = min(max(m.offset+delta, 0), max(len(m.items)-m.rows(), 0))
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
	if rows := m.rows(); m.cursor >= m.offset+rows {
		m.offset = m.cursor - rows + 1
	}
	m.offset = max(m.offset, 0)
}

func (m *listScreen) view(h *hitMap) string {
	switch {
	case m.err != nil:
		return titled("puzzles",
			st.err.Render(fmt.Sprintf("could not read puzzles: %v", m.err)))
	case len(m.items) == 0:
		return titled("puzzles",
			st.muted.Render("no puzzles yet — start one from the menu"))
	}

	// The counter and the delete prompt join the rows in one block, so they
	// centre with the list rather than drifting against it.
	var lines []string
	end := min(m.offset+m.rows(), len(m.items))
	for i := m.offset; i < end; i++ {
		// One click opens a puzzle; esc comes straight back, so there is no
		// need to make selecting a separate step.
		lines = append(lines, h.mark(action{kind: actListRow, index: i},
			m.renderRow(m.items[i], i == m.cursor)))
	}

	if len(m.items) > m.rows() {
		lines = append(lines, "", st.muted.Render(fmt.Sprintf("  %d–%d of %d",
			m.offset+1, end, len(m.items))))
	}
	if prompt := m.deletePrompt(h); prompt != "" {
		lines = append(lines, "", prompt)
	}
	return titled("puzzles", strings.Join(lines, "\n"))
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
	// A daily is the one delete with a consequence past the record itself: its
	// id is derived, so the day cannot be played again once its tombstone is
	// there. Say so while there is still a chance to answer no.
	question := fmt.Sprintf("delete #%s? ", game.Code(s.ID))
	if s.Daily != "" {
		question = fmt.Sprintf("delete the %s daily? it cannot be played again ", s.Daily)
	}
	return st.muted.Render(question) +
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
	what := fmt.Sprintf("%d letters", s.Length)
	if s.Daily != "" {
		what = fmt.Sprintf("daily %s", s.Daily)
	}
	left := fmt.Sprintf("#%s %-16s ", game.Code(s.ID), what)
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
