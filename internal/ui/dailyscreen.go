package ui

import (
	"errors"
	"fmt"
	"image/color"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/nxck2005/surmise/internal/brand"
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

	// copyRequested acknowledges a copy of the trio result. Like the debrief's,
	// it says requested rather than copied, because OSC 52 is never answered.
	copyRequested bool
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

	// What the trio card and its shared text are made of. The marks are the
	// scored rows, never the guesses that earned them and never the answer: the
	// screen holds nothing it would be a spoiler to paste.
	maxAttempts int
	elapsed     time.Duration
	marks       [][]game.Mark
}

// done reports whether this mode's daily is finished and still has a record. A
// deleted one is not: game.Tombstone keeps the status and drops the marks, so a
// spent day is a day with no board left to show.
func (row dailyRow) done() bool { return !row.spent && row.status.Done() }

// reload reads what has been played of a day.
//
// It reads through All rather than Load because a deleted daily has to be
// visible here: Load reports a tombstone as ErrNotFound, which would make a
// spent day look untouched. One pass over the directory covers every mode.
func (m *dailyScreen) reload(s store.Store, d daily.Day) {
	m.day = d
	m.err = nil
	m.copyRequested = false
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
			row.maxAttempts, row.elapsed, row.marks = g.MaxAttempts, g.Elapsed(), g.Marks
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
	sections := []string{heading, "", block(strings.Join(rows, "\n"))}
	if card := m.renderTrio(); card != "" {
		sections = append(sections, "", card)
	}
	if m.copyRequested {
		sections = append(sections, "", st.muted.Render("copy requested"))
	}
	return lipgloss.JoinVertical(lipgloss.Center, sections...)
}

// renderTrio is the day as one event: how many of its modes are done, and — once
// they all are — what the day cost altogether. It is empty until the first mode
// is finished, because a counter of nothing is not progress.
//
// The completed line is accented and nothing more. That is the whole of the
// feedback: no timer, no reveal, and deliberately not the board's win accent,
// because a day is not a puzzle.
func (m *dailyScreen) renderTrio() string {
	t := m.trio()
	switch {
	case t.done == 0:
		return ""
	case !t.complete():
		return st.muted.Render(fmt.Sprintf("trio · %d/%d", t.done, t.of))
	}
	return st.accent.Render(fmt.Sprintf("the trio · %d/%d · %d guesses · %s",
		t.done, t.of, t.guesses, formatDuration(t.elapsed)))
}

// trioSummary is a day across its modes. It is derived from the rows every time
// rather than stored: the daily screen already reads every saved game to build
// them, and a day's set of three is not a thing the store knows about.
//
// It is deliberately not a stats figure. internal/stats keeps the modes
// independent on purpose — missing the six-letter board must not cost a
// five-letter run — and the trio is a day's presentation, not a fourth streak.
type trioSummary struct {
	done, of int
	guesses  int
	elapsed  time.Duration
}

func (t trioSummary) complete() bool { return t.of > 0 && t.done == t.of }

func (m *dailyScreen) trio() trioSummary {
	t := trioSummary{of: len(m.rows)}
	for _, row := range m.rows {
		if !row.done() {
			continue
		}
		t.done++
		t.guesses += row.attempts
		t.elapsed += row.elapsed
	}
	return t
}

// shareTrio is the day's result as public text: the three boards under one
// heading. Like shareResult it carries marks and figures only — no answer, no
// guess — and the screen it is built from holds nothing else anyway.
//
// The date names the day rather than three puzzle codes: everyone playing that
// date has the same three boards, so the codes would say nothing the date does
// not.
func shareTrio(day string, rows []dailyRow) string {
	var b strings.Builder
	var guesses int
	var elapsed time.Duration
	for _, row := range rows {
		guesses += row.attempts
		elapsed += row.elapsed
	}

	fmt.Fprintf(&b, "%s daily %s · %d/%d\n", brand.Name, day, len(rows), len(rows))
	fmt.Fprintf(&b, "%d guesses · %s\n", guesses, formatDuration(elapsed))
	for _, row := range rows {
		fmt.Fprintf(&b, "\n%d letters %s\n", row.length, row.shareAttempts())
		b.WriteString(shareGrid(row.marks))
	}
	b.WriteString("\n" + shareLegend)
	return b.String()
}

// shareAttempts is the row's score, in the debrief's vocabulary: X/max for a
// loss, so a day with one lost mode reads the same way a lost puzzle does.
func (row dailyRow) shareAttempts() string {
	if row.status == game.Lost {
		return fmt.Sprintf("X/%d", row.maxAttempts)
	}
	return fmt.Sprintf("%d/%d", row.attempts, row.maxAttempts)
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
	items := []helpItem{
		{keys: "↑/↓", label: "mode"},
		{keys: "enter", label: "play", act: action{kind: actDailyRow, index: m.cursor}},
	}
	// Offered only once there is a trio to copy — a hint for a key that would do
	// nothing is a promise the screen does not keep.
	if m.trio().complete() {
		items = append(items, helpItem{
			keys: "c", label: "copy", act: action{kind: actDailyCopy},
		})
	}
	items = append(items, helpItem{keys: "esc", label: "back", act: action{kind: actBack}})
	return renderHelp(h, items...)
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
