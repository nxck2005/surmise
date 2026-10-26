package ui

import (
	"fmt"
	"image/color"
	"sort"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/nxck2005/surmise/internal/daily"
	"github.com/nxck2005/surmise/internal/game"
	"github.com/nxck2005/surmise/internal/stats"
)

// calendarGutter is the weekday-label column left of the grid: "mon ".
const calendarGutter = 4

// calendarYears is the most calendar years the range row offers, the current
// one included. A hand-edited start time in the year 1 must not give the row
// two thousand entries.
const calendarYears = 6

// calendarGlyph is the square drawn when a theme's day glyph is empty.
const calendarGlyph = "■"

var (
	calendarMonths   = [...]string{"jan", "feb", "mar", "apr", "may", "jun", "jul", "aug", "sep", "oct", "nov", "dec"}
	calendarWeekdays = [...]string{"sun", "mon", "tue", "wed", "thu", "fri", "sat"}
)

// calRange is one entry of the range row: the days it draws, both ends
// included.
type calRange struct {
	label    string
	from, to daily.Day
}

// sundayOnOrBefore is the Sunday that starts d's week, as a column starts.
func sundayOnOrBefore(d daily.Day) daily.Day { return d.AddDays(-int(d.Weekday())) }

// grid is the range's columns: the Sunday at the top of the first, and how
// many there are. A year can take 54 (2028 starts on a Saturday and ends on
// a Sunday).
func (r calRange) grid() (start daily.Day, cols int) {
	start = sundayOnOrBefore(r.from)
	return start, sundayOnOrBefore(r.to).DaysSince(start)/7 + 1
}

// contains reports whether d is one of the range's days.
func (r calRange) contains(d daily.Day) bool { return !d.Before(r.from) && !r.to.Before(d) }

// calendarRanges is the range row: the last year, then the current calendar
// year, then each earlier year that holds a game, newest first, at most
// calendarYears years in all. Years after today's are never offered.
func calendarRanges(tallies map[daily.Day]stats.DayTally, today daily.Day) []calRange {
	ty, _, _ := today.Date()
	ranges := []calRange{{label: "last year", from: sundayOnOrBefore(today).AddDays(-52 * 7), to: today}}

	seen := map[int]bool{ty: true}
	years := []int{ty}
	for d := range tallies {
		if y, _, _ := d.Date(); y < ty && !seen[y] {
			seen[y] = true
			years = append(years, y)
		}
	}
	sort.Sort(sort.Reverse(sort.IntSlice(years)))
	if len(years) > calendarYears {
		years = years[:calendarYears]
	}
	for _, y := range years {
		r := calRange{label: fmt.Sprint(y), from: daily.DayFrom(y, time.January, 1), to: daily.DayFrom(y, time.December, 31)}
		if y == ty {
			r.to = today
		}
		ranges = append(ranges, r)
	}
	return ranges
}

// calendarLevel is how dark a day is drawn, 0 (nothing played) to 4 (the
// busiest day in view): the share of the busiest day's count, rounded up to
// a quarter.
func calendarLevel(n, peak int) int {
	if n <= 0 || peak <= 0 {
		return 0
	}
	return min((4*n+peak-1)/peak, 4)
}

// calendarRamp is the five shades, from the empty-tile colour up to the
// correct tile's. On a terminal that cannot blend, every played day is the
// correct colour and only "played or not" shows, which is still the point.
func calendarRamp() [5]color.Color {
	empty := st.tileEmpty.GetForeground()
	full := st.tileCorrect.GetBackground()
	r := blend(5, empty, full)
	return [5]color.Color{empty, r[1], r[2], r[3], r[4]}
}

// calendarScreen is a year of play, one square a day, like a GitHub profile's
// contribution graph.
type calendarScreen struct {
	tallies map[daily.Day]stats.DayTally
	today   daily.Day
	ranges  []calRange
	shown   int       // index into ranges
	cursor  daily.Day // the selected day; always drawn and visible

	width, height int
}

func (m *calendarScreen) resize(w, h int) { m.width, m.height = w, h }

// reload opens the calendar fresh: the last year, today selected.
func (m *calendarScreen) reload(games []*game.Game, today daily.Day, zone *time.Location) {
	m.tallies = stats.Activity(games, zone)
	m.today = today
	m.ranges = calendarRanges(m.tallies, today)
	m.shown = 0
	m.cursor = today
}

// refresh re-reads the counts after coming back from a day's puzzle list,
// keeping the range and the day the player was on wherever they still exist.
func (m *calendarScreen) refresh(games []*game.Game, today daily.Day, zone *time.Location) {
	shown, cursor := m.shown, m.cursor
	m.tallies = stats.Activity(games, zone)
	m.today = today
	m.ranges = calendarRanges(m.tallies, today)
	m.shown = min(shown, len(m.ranges)-1)
	m.cursor = cursor
	m.keepCursor()
}

func (m *calendarScreen) rng() calRange { return m.ranges[m.shown] }

func (m *calendarScreen) tally(d daily.Day) stats.DayTally { return m.tallies[d] }

// calLayout is one frame's geometry.
type calLayout struct {
	glyph string
	w     int       // cells per day
	start daily.Day // the Sunday at the top of the range's first column
	cols  int       // the range's columns
	first int       // the first column drawn; the newest columns always show
}

// layout decides the frame's geometry. A terminal too narrow for every week
// draws the most recent weeks that fit; nothing scrolls.
func (m *calendarScreen) layout() calLayout {
	glyph := st.glyph.Day
	w := lipgloss.Width(glyph)
	if w == 0 {
		glyph, w = calendarGlyph, 1
	}
	start, cols := m.rng().grid()
	first := 0
	if bw := bodyWidth(m.width); bw > 0 {
		if fit := max((bw-calendarGutter)/w, 1); cols > fit {
			first = cols - fit
		}
	}
	return calLayout{glyph: glyph, w: w, start: start, cols: cols, first: first}
}

// visibleFrom is the earliest day drawn.
func (m *calendarScreen) visibleFrom(l calLayout) daily.Day {
	d := l.start.AddDays(7 * l.first)
	if d.Before(m.rng().from) {
		return m.rng().from
	}
	return d
}

// selectable reports whether d is drawn: in the range and in a visible week.
func (m *calendarScreen) selectable(d daily.Day) bool {
	return m.rng().contains(d) && !d.Before(m.visibleFrom(m.layout()))
}

// keepCursor puts the cursor on the range's last day when it is not drawn.
func (m *calendarScreen) keepCursor() {
	if !m.selectable(m.cursor) {
		m.cursor = m.rng().to
	}
}

// move steps the cursor by days. Up and down stay inside a week (a column),
// and a step that would land on a day not drawn does nothing.
func (m *calendarScreen) move(days int) bool {
	wd := m.cursor.Weekday()
	if (days == -1 && wd == time.Sunday) || (days == 1 && wd == time.Saturday) {
		return false
	}
	d := m.cursor.AddDays(days)
	if !m.selectable(d) {
		return false
	}
	m.cursor = d
	return true
}

// point selects the day a click or the pointer is on: index is days since
// the range's first day. It reports whether the cursor moved.
func (m *calendarScreen) point(index int) bool {
	d := m.rng().from.AddDays(index)
	if index < 0 || !m.selectable(d) || d == m.cursor {
		return false
	}
	m.cursor = d
	return true
}

// showRange switches the range row's entry. The cursor keeps its day when
// the new range draws it, and goes to the range's last day when not.
func (m *calendarScreen) showRange(i int) {
	if i < 0 || i >= len(m.ranges) || i == m.shown {
		return
	}
	m.shown = i
	m.keepCursor()
}

// update reads a key. It reports whether the player asked to list the
// selected day's puzzles, or to go back.
func (m *calendarScreen) update(msg tea.KeyPressMsg) (open, back bool) {
	switch msg.String() {
	case "esc", "q":
		return false, true
	case "left", "h":
		m.move(-7)
	case "right", "l":
		m.move(7)
	case "up", "k":
		m.move(-1)
	case "down", "j":
		m.move(1)
	case "[":
		m.showRange(m.shown - 1)
	case "]":
		m.showRange(m.shown + 1)
	case "enter":
		return m.tally(m.cursor).Games > 0, false
	}
	return false, false
}

// monthRow is the month names over the grid, rendered whole in st.muted.
func (m *calendarScreen) monthRow(l calLayout) string {
	width := (l.cols - l.first) * l.w
	slice := []byte(strings.Repeat(" ", width))
	type label struct {
		x   int
		mon time.Month
	}
	var placed []label
	prev := time.Month(0)
	for c := l.first; c < l.cols; c++ {
		top := l.start.AddDays(7 * c)
		if top.Before(m.rng().from) {
			top = m.rng().from
		}
		_, mon, _ := top.Date()
		if c > l.first && mon == prev {
			continue
		}
		prev = mon
		x := (c - l.first) * l.w
		if x+3 > width {
			continue
		}
		if len(placed) > 0 && x < placed[len(placed)-1].x+4 {
			if len(placed) != 1 {
				continue
			}
			placed = placed[:0]
		}
		placed = append(placed, label{x, mon})
	}
	for _, p := range placed {
		copy(slice[p.x:], calendarMonths[p.mon-1])
	}
	return st.muted.Render(strings.Repeat(" ", calendarGutter) + string(slice))
}

// gridRows is the seven lines of days, Sunday first.
func (m *calendarScreen) gridRows(h *hitMap, l calLayout, peak int) []string {
	ramp := calendarRamp()
	var plain, picked [5]string
	for i := range ramp {
		plain[i] = lipgloss.NewStyle().Foreground(ramp[i]).Render(l.glyph)
		picked[i] = lipgloss.NewStyle().Foreground(ramp[i]).Reverse(true).Render(l.glyph)
	}
	lines := make([]string, 0, 7)
	for r := 0; r < 7; r++ {
		label := strings.Repeat(" ", calendarGutter)
		switch r {
		case 1:
			label = "mon "
		case 3:
			label = "wed "
		case 5:
			label = "fri "
		}
		var b strings.Builder
		b.WriteString(st.muted.Render(label))
		for c := l.first; c < l.cols; c++ {
			d := l.start.AddDays(7*c + r)
			if !m.rng().contains(d) {
				b.WriteString(strings.Repeat(" ", l.w))
				continue
			}
			level := calendarLevel(m.tally(d).Games, peak)
			cell := plain[level]
			if d == m.cursor {
				cell = picked[level]
			}
			b.WriteString(h.mark(action{kind: actCalendarDay, index: d.DaysSince(m.rng().from)}, cell))
		}
		lines = append(lines, b.String())
	}
	return lines
}

// detail is the selected day, rendered whole in st.text.
func (m *calendarScreen) detail() string {
	t := m.tally(m.cursor)
	count := fmt.Sprintf("%d games", t.Games)
	switch t.Games {
	case 0:
		count = "no games"
	case 1:
		count = "1 game"
	}
	y, mon, d := m.cursor.Date()
	s := fmt.Sprintf("%s on %s %d %s %d", count, calendarWeekdays[m.cursor.Weekday()], d, calendarMonths[mon-1], y)
	for _, part := range []struct {
		n    int
		name string
	}{{t.Won, "won"}, {t.Lost, "lost"}, {t.InPlay, "in play"}} {
		if part.n > 0 {
			s += fmt.Sprintf(" · %d %s", part.n, part.name)
		}
	}
	return st.text.Render(s)
}

// rangeRow is the range switcher; each entry is a click target.
func (m *calendarScreen) rangeRow(h *hitMap) string {
	parts := make([]string, len(m.ranges))
	for i, r := range m.ranges {
		a := action{kind: actCalendarRange, index: i}
		style := st.muted
		if i == m.shown {
			style = st.accent
		}
		if h.hovered(a) {
			style = st.hover(style)
		}
		parts[i] = h.mark(a, style.Render(r.label))
	}
	return strings.Join(parts, st.muted.Render(" · "))
}

func (m *calendarScreen) view(h *hitMap) string {
	l := m.layout()
	r := m.rng()

	total, peak := 0, 0
	for d := r.from; !r.to.Before(d); d = d.AddDays(1) {
		n := m.tally(d).Games
		total += n
		peak = max(peak, n)
	}

	heading := fmt.Sprintf("%d games", total)
	switch total {
	case 0:
		heading = "no games"
	case 1:
		heading = "1 game"
	}
	if m.shown == 0 {
		heading += " in the last year"
	} else {
		heading += " in " + r.label
	}

	ramp := calendarRamp()
	legend := st.muted.Render("less ")
	for _, c := range ramp {
		legend += lipgloss.NewStyle().Foreground(c).Render(l.glyph)
	}
	legend += st.muted.Render(" more")
	if pad := calendarGutter + (l.cols-l.first)*l.w - lipgloss.Width(legend); pad > 0 {
		legend = strings.Repeat(" ", pad) + legend
	}

	// The grid and the selected day never shed: they are the screen. The
	// rest is given up in a fixed order when the terminal is short — the
	// heading first, then the legend, then the range row — and, like the
	// board's ladder, a later one is never kept after an earlier one went.
	showYears, showLegend, showHeading := true, true, true
	if budget := bodyBudget(m.height); budget > 0 {
		used := 9 // the month row, seven rows of days, the detail line
		showYears = used+2 <= budget
		if showYears {
			used += 2
		}
		showLegend = showYears && used+1 <= budget
		if showLegend {
			used++
		}
		showHeading = showLegend && used+2 <= budget
	}

	var lines []string
	if showHeading {
		lines = append(lines, st.text.Render(heading), "")
	}
	lines = append(lines, m.monthRow(l))
	lines = append(lines, m.gridRows(h, l, peak)...)
	if showLegend {
		lines = append(lines, legend)
	}
	lines = append(lines, m.detail())
	if showYears {
		lines = append(lines, "", m.rangeRow(h))
	}
	return block(lipgloss.JoinVertical(lipgloss.Left, lines...))
}

func (m *calendarScreen) help(h *hitMap) string {
	items := []helpItem{{keys: "↑/↓/←/→", label: "move"}}
	if m.tally(m.cursor).Games > 0 {
		items = append(items, helpItem{keys: "enter", label: "puzzles",
			act: action{kind: actCalendarDay, index: m.cursor.DaysSince(m.rng().from)}})
	}
	items = append(items,
		helpItem{keys: "[/]", label: "year"},
		helpItem{keys: "esc", label: "profile", act: action{kind: actBack}},
	)
	return renderHelp(h, items...)
}
