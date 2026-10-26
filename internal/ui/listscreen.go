package ui

import (
	"fmt"
	"image/color"
	"slices"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/nxck2005/surmise/internal/daily"
	"github.com/nxck2005/surmise/internal/game"
	"github.com/nxck2005/surmise/internal/stats"
	"github.com/nxck2005/surmise/internal/store"
	"github.com/nxck2005/surmise/internal/words"
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

// puzzleKind is where a puzzle came from, as the list labels it and as its
// kind filter picks it. kindAll is the filter's "any".
type puzzleKind int

const (
	kindAll puzzleKind = iota
	kindRandom
	kindDaily
	kindChallenge
	kindCustom
	kindRoom
)

// kindOf classifies a puzzle. The order of the cases is renderRow's order, and
// renderRow now reads it from here, so a label and a filter cannot disagree.
func kindOf(s store.Summary) puzzleKind {
	switch {
	case s.Daily != "":
		return kindDaily
	case s.Custom:
		return kindCustom
	case s.Challenge:
		return kindChallenge
	case s.Room:
		return kindRoom
	}
	return kindRandom
}

// statusFilter is the status choice on the filter bar. statusAll is "any".
type statusFilter int

const (
	statusAll statusFilter = iota
	statusPlaying
	statusWon
	statusLost
)

// listFilter is what the filter bar has chosen. Its zero value lets
// everything through.
type listFilter struct {
	length int // 0 is any mode
	kind   puzzleKind
	status statusFilter
}

// The filter bar's three fields, in the order they are drawn. They are the
// index of an actListFilter click.
const (
	filterMode = iota
	filterKind
	filterStatus
)

// filterKinds and filterStatuses are the order each field steps through,
// starting from "all" and wrapping back to it.
var (
	filterKinds    = []puzzleKind{kindAll, kindRandom, kindDaily, kindChallenge, kindCustom, kindRoom}
	filterStatuses = []statusFilter{statusAll, statusPlaying, statusWon, statusLost}
)

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

	// all is everything List returned; items is the part of it the filters
	// let through, and is what every row, cursor and click indexes.
	all []store.Summary

	// filtersOpen shows the filter bar. Closing it clears the filter, so the
	// list a player comes back to is always the whole one.
	filtersOpen bool
	filter      listFilter

	// day narrows the list to one calendar day; zero is every day. Only the
	// calendar sets it, and esc then goes back to the calendar.
	day  daily.Day
	zone *time.Location
}

func (m *listScreen) resize(h int) {
	m.height = h
	m.clampOffset()
}

// matches reports whether a puzzle passes the filter and, when the
// calendar asked for one day, is on that day.
func (m *listScreen) matches(s store.Summary) bool {
	f := m.filter
	if f.length != 0 && s.Length != f.length {
		return false
	}
	if f.kind != kindAll && kindOf(s) != f.kind {
		return false
	}
	switch {
	case f.status == statusPlaying && s.Status != game.InProgress,
		f.status == statusWon && s.Status != game.Won,
		f.status == statusLost && s.Status != game.Lost:
		return false
	}
	if !m.day.IsZero() {
		d, ok := stats.PlayDay(s.Daily, s.StartedAt, m.zone)
		if !ok || d != m.day {
			return false
		}
	}
	return true
}

// apply rebuilds items from all.
func (m *listScreen) apply() {
	m.items = m.items[:0:0]
	for _, s := range m.all {
		if m.matches(s) {
			m.items = append(m.items, s)
		}
	}
}

// filtered reports whether the rows are fewer than everything by choice.
func (m *listScreen) filtered() bool { return m.filter != listFilter{} || !m.day.IsZero() }

// toggleFilters shows or hides the filter bar. Hiding it clears the filter.
// The keys and the help bar's button both come here.
func (m *listScreen) toggleFilters() {
	m.filtersOpen = !m.filtersOpen
	if !m.filtersOpen {
		m.filter = listFilter{}
	}
	m.apply()
	m.cursor, m.offset = 0, 0
	m.confirmDelete = false
}

// cycle steps one field of the filter bar to its next value, wrapping round
// to "all". The keys and a click on the field both come here. It does
// nothing while the bar is hidden.
func (m *listScreen) cycle(field int) {
	if !m.filtersOpen {
		return
	}
	switch field {
	case filterMode:
		modes := append([]int{0}, words.Lengths...)
		m.filter.length = modes[(slices.Index(modes, m.filter.length)+1)%len(modes)]
	case filterKind:
		m.filter.kind = filterKinds[(slices.Index(filterKinds, m.filter.kind)+1)%len(filterKinds)]
	case filterStatus:
		m.filter.status = filterStatuses[(slices.Index(filterStatuses, m.filter.status)+1)%len(filterStatuses)]
	default:
		return
	}
	m.apply()
	m.cursor, m.offset = 0, 0
	m.confirmDelete = false
}

// headerLines is how many lines the body spends above the rows: the filter
// bar and the blank under it.
func (m *listScreen) headerLines() int {
	n := 0
	if !m.day.IsZero() {
		n += 2 // the day and the blank under it
	}
	if m.filtersOpen {
		n += 2
	}
	return n
}

// rows is the size of the visible window, after the lines above the rows.
func (m *listScreen) rows() int {
	h := m.height
	if h > 0 {
		h = max(h-m.headerLines(), 1)
	}
	return windowRows(h, len(m.items))
}

// reload reads the puzzles afresh for a list opened from the menu: the whole
// list, no filter, cursor at the top.
func (m *listScreen) reload(s store.Store) {
	m.all, m.err = s.List()
	m.filtersOpen = false
	m.filter = listFilter{}
	m.day = daily.Day{}
	m.apply()
	m.cursor, m.offset = 0, 0
	m.confirmDelete = false
}

// openDay shows one calendar day's puzzles, for the calendar.
func (m *listScreen) openDay(s store.Store, day daily.Day, zone *time.Location) {
	m.reload(s)
	m.day, m.zone = day, zone
	m.apply()
}

// refresh re-reads the puzzles while holding the filter and the cursor where
// the player left them, clamped to whatever is still there. reload resets
// everything, which is right on entering the screen and wrong after deleting
// a row.
func (m *listScreen) refresh(s store.Store) {
	cursor := m.cursor
	m.all, m.err = s.List()
	m.apply()
	m.confirmDelete = false
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
	case "f":
		m.toggleFilters()
	case "m":
		m.cycle(filterMode)
	case "t":
		m.cycle(filterKind)
	case "s":
		m.cycle(filterStatus)
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
// way it does on a web page, so a click has no separate "select" step. It
// reports whether the rows changed — a pointer already on the selected row does
// nothing, and saying so lets the root keep the frame it already drew.
func (m *listScreen) point(row int) bool {
	if row < 0 || row >= len(m.items) || row == m.cursor {
		return false
	}
	// Moving to a different row disarms: a prompt asking about the puzzle
	// the pointer has just left would be answering for the wrong one.
	m.confirmDelete = false
	m.cursor = row
	return true
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

// filterBar is the three filters on one line. Each field is a click target
// that steps it, like its key.
func (m *listScreen) filterBar(h *hitMap) string {
	mode := "all"
	if m.filter.length != 0 {
		mode = fmt.Sprint(m.filter.length)
	}
	kinds := map[puzzleKind]string{
		kindAll: "all", kindRandom: "random", kindDaily: "daily",
		kindChallenge: "challenge", kindCustom: "custom", kindRoom: "room",
	}
	statuses := map[statusFilter]string{
		statusAll: "all", statusPlaying: "in play", statusWon: "won", statusLost: "lost",
	}
	field := func(i int, key, name, value string) string {
		a := action{kind: actListFilter, index: i}
		label, val := st.muted, st.text
		if h.hovered(a) {
			label, val = st.hover(label), st.hover(val)
		}
		return h.mark(a, label.Render(key+" "+name+" ")+val.Render(value))
	}
	sep := st.muted.Render(" · ")
	return field(filterMode, "m", "mode", mode) + sep +
		field(filterKind, "t", "kind", kinds[m.filter.kind]) + sep +
		field(filterStatus, "s", "status", statuses[m.filter.status])
}

func (m *listScreen) view(h *hitMap) string {
	switch {
	case m.err != nil:
		return block(st.err.Render(safeText(fmt.Sprintf("could not read puzzles: %v", m.err))))
	case len(m.all) == 0:
		return block(st.muted.Render("no puzzles yet — start one from the menu"))
	}

	// The filter bar, the counter and the delete prompt join the rows in one
	// block, so they centre with the list rather than drifting against it.
	var lines []string
	if !m.day.IsZero() {
		y, mon, d := m.day.Date()
		lines = append(lines, st.text.Render(fmt.Sprintf("%s %d %s %d",
			calendarWeekdays[m.day.Weekday()], d, calendarMonths[mon-1], y)), "")
	}
	if m.filtersOpen {
		lines = append(lines, m.filterBar(h), "")
	}
	if len(m.items) == 0 {
		lines = append(lines, st.muted.Render("no puzzles match"))
		return block(strings.Join(lines, "\n"))
	}

	end := min(m.offset+m.rows(), len(m.items))
	for i := m.offset; i < end; i++ {
		// One click opens a puzzle; esc comes straight back, so there is no
		// need to make selecting a separate step.
		lines = append(lines, h.mark(action{kind: actListRow, index: i},
			m.renderRow(m.items[i], i == m.cursor)))
	}

	if len(m.items) > m.rows() {
		lines = append(lines, "", scrollCounter(h, m.offset+1, end, len(m.items)))
	}
	if prompt := m.deletePrompt(h); prompt != "" {
		lines = append(lines, "", prompt)
	}
	return block(strings.Join(lines, "\n"))
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
	} else if s.Challenge {
		question = "delete this challenge? this code cannot be played again here "
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
	switch kindOf(s) {
	case kindDaily:
		what = fmt.Sprintf("daily %s", s.Daily)
	case kindCustom:
		what = "custom"
	case kindChallenge:
		what = "challenge"
	case kindRoom:
		what = "room"
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
	back := helpItem{keys: "esc", label: "menu", act: action{kind: actBack}}
	if !m.day.IsZero() {
		back.label = "back"
	}
	if len(m.all) == 0 {
		// Nothing saved: nothing to open, delete or filter.
		return renderHelp(h, back)
	}
	var items []helpItem
	if len(m.items) > 0 {
		items = append(items,
			helpItem{keys: "enter", label: "open", act: action{kind: actListRow, index: m.cursor}},
			helpItem{keys: "d", label: "delete", act: action{kind: actDeletePuzzle, index: m.cursor}},
		)
	}
	toggle := "filter"
	if m.filtersOpen {
		toggle = "close"
	}
	items = append(items, helpItem{keys: "f", label: toggle, act: action{kind: actListFilters}}, back)
	return renderHelp(h, items...)
}
