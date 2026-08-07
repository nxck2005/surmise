package ui

import (
	"errors"
	"fmt"
	"image/color"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/nxck2005/surmise/internal/daily"
	"github.com/nxck2005/surmise/internal/game"
	"github.com/nxck2005/surmise/internal/store"
	"github.com/nxck2005/surmise/internal/words"
)

// dailyScreen offers the day's puzzle in each mode. There is one daily per
// length, so this is a short, fixed list rather than something that scrolls.
//
// It reports the state of each mode's puzzle — untouched, in play, finished, or
// spent — because "have I done today's?" is the question the screen exists to
// answer, and because a finished one opens for review rather than for play.
type dailyScreen struct {
	day  daily.Day
	rows []dailyRow

	cursor int
	err    error
}

type dailyRow struct {
	length int
	id     string

	// status is the zero Status when the puzzle has not been started. spent
	// means it was played and then deleted: the record is a tombstone, and
	// rebuilding the puzzle would write over it (see openDaily).
	status   game.Status
	attempts int
	spent    bool
}

// reload reads what has been played of a day.
//
// It reads through All rather than Load because a deleted daily has to be
// visible here: Load reports a tombstone as ErrNotFound, which would make a
// spent day look untouched. One pass over the directory covers every mode.
func (m *dailyScreen) reload(s store.Store, d daily.Day) {
	m.day = d
	m.err = nil
	m.rows = make([]dailyRow, 0, len(words.Lengths))

	saved, err := s.All()
	if err != nil {
		m.err = err
	}
	byID := make(map[string]*game.Game, len(saved))
	for _, g := range saved {
		byID[g.ID] = g
	}

	for _, n := range words.Lengths {
		row := dailyRow{length: n, id: daily.ID(d, n)}
		if g, ok := byID[row.id]; ok {
			row.status, row.attempts, row.spent = g.Status, g.Attempts(), g.Deleted
		}
		m.rows = append(m.rows, row)
	}
	m.cursor = min(max(m.cursor, 0), len(m.rows)-1)
}

// selected returns the highlighted mode's row.
func (m *dailyScreen) selected() (dailyRow, bool) {
	if m.cursor < 0 || m.cursor >= len(m.rows) {
		return dailyRow{}, false
	}
	return m.rows[m.cursor], true
}

// update moves the cursor, and reports whether a mode was chosen or the player
// asked to go back.
func (m *dailyScreen) update(msg tea.KeyPressMsg) (open, back bool) {
	switch msg.String() {
	case "esc", "q":
		return false, true
	case "up", "k":
		m.move(-1)
	case "down", "j":
		m.move(1)
	case "enter", " ":
		if _, ok := m.selected(); ok {
			return true, false
		}
	}
	return false, false
}

func (m *dailyScreen) move(delta int) {
	if len(m.rows) == 0 {
		return
	}
	m.cursor = min(max(m.cursor+delta, 0), len(m.rows)-1)
}

// point selects the row the pointer is over, so one click is enough to play.
func (m *dailyScreen) point(row int) {
	if row >= 0 && row < len(m.rows) {
		m.cursor = row
	}
}

func (m *dailyScreen) view(h *hitMap) string {
	// The date is always shown rather than implied: the daily turns over at UTC
	// midnight, so for much of the world it is not the date on the wall clock.
	// The countdown is only true of the live day — under -day it would be
	// counting down to a board this screen is not showing — so it is dropped.
	when := m.day.String()
	if m.day == daily.Today() {
		when += " · resets in " + until(m.day.ResetsAt())
	}

	// The date belongs to the title, tight under it, so this screen composes its
	// own heading rather than going through titled.
	heading := lipgloss.JoinVertical(lipgloss.Center,
		st.title.Render("daily"),
		st.muted.Render(when),
	)

	if m.err != nil {
		return lipgloss.JoinVertical(lipgloss.Center, heading, "",
			st.err.Render(fmt.Sprintf("could not read puzzles: %v", m.err)))
	}

	rows := make([]string, len(m.rows))
	for i, row := range m.rows {
		rows[i] = h.mark(action{kind: actDailyRow, index: i},
			m.renderRow(row, i == m.cursor))
	}
	// Squared off first, so the join slides the rows under the heading as one
	// block instead of centring each on its own.
	return lipgloss.JoinVertical(lipgloss.Center, heading, "",
		block(strings.Join(rows, "\n")))
}

// renderRow lays out one mode, in the same column shape as the puzzle list: the
// code, what it is, then the only coloured part, its state.
func (m *dailyScreen) renderRow(row dailyRow, selected bool) string {
	statusText, statusColor := row.describe()

	left := fmt.Sprintf("#%s %-10s ", game.Code(row.id), fmt.Sprintf("%d letters", row.length))
	status := fmt.Sprintf("%-14s", statusText)

	rowStyle := st.muted
	prefix := strings.Repeat(" ", lipgloss.Width(st.glyph.Cursor))
	if selected {
		rowStyle = st.text
		prefix = st.cursor.Render(st.glyph.Cursor)
	}
	// Styles are composed side by side, never nested: wrapping an already-styled
	// string in another style corrupts its escape codes.
	return prefix + rowStyle.Render(left) +
		lipgloss.NewStyle().Foreground(statusColor).Render(status)
}

// describe is the row's state in words, mirroring the puzzle list's vocabulary
// so the same puzzle reads the same way in both places.
func (row dailyRow) describe() (text string, c color.Color) {
	switch {
	case row.spent:
		return "deleted", st.statusLost
	case row.status == game.Won:
		return fmt.Sprintf("solved %d/%d", row.attempts, row.length+1), st.statusWon
	case row.status == game.Lost:
		return "failed", st.statusLost
	case row.status == game.InProgress:
		return fmt.Sprintf("in play %d/%d", row.attempts, row.length+1), st.statusPlaying
	default:
		return "not started", st.statusPlaying
	}
}

func (m *dailyScreen) help(h *hitMap) string {
	act := action{kind: actDailyRow, index: m.cursor}
	return renderHelp(h,
		helpItem{keys: "↑/↓", label: "mode"},
		helpItem{keys: "enter", label: "play", act: act},
		helpItem{keys: "esc", label: "back", act: action{kind: actBack}},
	)
}

// until is how long is left, rounded to something worth reading. It never
// counts below a minute: the exact second the board turns over is not
// actionable, and a ticking countdown would only draw the eye.
func until(t time.Time) string {
	d := time.Until(t)
	switch {
	case d <= 0:
		return "moments"
	case d < time.Hour:
		return fmt.Sprintf("%dm", max(int(d.Minutes()), 1))
	default:
		return fmt.Sprintf("%dh %dm", int(d.Hours()), int(d.Minutes())%60)
	}
}

// errDailySpent is what opening a deleted daily reports. Deleting a finished
// puzzle leaves a tombstone, and the daily's id is derived, so rebuilding the
// puzzle would save straight over that tombstone — destroying the record that a
// win or a loss happened, which is exactly what tombstones exist to keep.
var errDailySpent = errors.New("this daily was deleted — it cannot be played again")
