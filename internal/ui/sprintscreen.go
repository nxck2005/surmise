package ui

import (
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/nxck2005/surmise/internal/game"
	"github.com/nxck2005/surmise/internal/words"
)

// Sprint mode: solve as many boards as the clock allows. The session is a
// wrapper around the ordinary board — every puzzle dealt is a plain random
// game that saves and counts like any other, and the wrapper owns exactly two
// things the board does not: the deadline, and the tally the summary reads.
//
// Nothing here is persisted. A session's tally dies with its summary screen;
// the boards themselves are already on disk. Personal bests wait for the
// trends work, which is where a stored figure belongs.

// sprintDurations is what the clock can be set to, shortest first. The short
// ends are deliberate: they are the Monkeytype shapes, where the run is over
// before it can outstay its welcome.
var sprintDurations = []time.Duration{
	10 * time.Second,
	15 * time.Second,
	30 * time.Second,
	time.Minute,
	2 * time.Minute,
	5 * time.Minute,
	10 * time.Minute,
}

// defaultSprintDuration is the offered default. It has to be one of
// sprintDurations, or the row would snap somewhere on first touch.
const defaultSprintDuration = 5 * time.Minute

func sprintDurationLabel(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d/time.Second))
	}
	return fmt.Sprintf("%dm", int(d/time.Minute))
}

func sprintDurationIndex(d time.Duration) int {
	for i, want := range sprintDurations {
		if want == d {
			return i
		}
	}
	return 0
}

// sprintClock renders a remaining time as m:ss. A session still running is
// never negative, so no sign is carried.
func sprintClock(d time.Duration) string {
	d = max(d.Round(time.Second), 0)
	s := int(d / time.Second)
	return fmt.Sprintf("%d:%02d", s/60, s%60)
}

// sprintSession is the live state of one run: the deadline, and how the boards
// it dealt went. It lives on the root while the session runs and on the screen
// afterwards, for the summary; the open board holds the same pointer, which is
// how the countdown reaches the status line without the root rendering the
// board's body.
type sprintSession struct {
	length   int
	duration time.Duration

	// deadline is zero until begin. Zero is "not started", not "already over":
	// a setup screen holds an unstarted session, and nothing checks expiry
	// against an unstarted clock.
	deadline time.Time

	dealt     int
	solved    int
	missed    int
	attempts  int             // guesses spent, wins only — averages cover wins only
	solveTime time.Duration   // across every finished board
	lastID    string          // guard: each board is recorded once
}

func (s *sprintSession) begin(now time.Time) { s.deadline = now.Add(s.duration) }

// running reports whether the clock is live. An unstarted session is not
// running, and neither is one the deadline has passed.
func (s *sprintSession) running() bool {
	return !s.deadline.IsZero() && timeNow().Before(s.deadline)
}

// left is the time on the clock. Only meaningful while running.
func (s *sprintSession) left() time.Duration { return s.deadline.Sub(timeNow()) }

// record banks one finished board into the tally. It is called from
// submitGame, after submit has banked the time and saved — so the summary can
// never disagree with what is on disk.
//
// lastID makes this idempotent: the finished board sits on screen for the
// length of its reveal, and any number of submits in that window must not
// count it twice.
func (s *sprintSession) record(g *game.Game) {
	if g.ID == s.lastID {
		return
	}
	s.lastID = g.ID
	s.dealt++
	if g.Status == game.Won {
		s.solved++
		s.attempts += g.Attempts()
	} else {
		s.missed++
	}
	s.solveTime += g.Elapsed()
}

// --- the screen ---

type sprintPhase int

const (
	sprintSetup sprintPhase = iota
	sprintSummary
)

const (
	sprintRowLength = iota
	sprintRowTime
	sprintRows
)

// sprintScreen is the setup before a run and the summary after it. Like the
// custom screen it is rebuilt when raised from the menu, so nothing of the
// last visit leaks into the next one.
type sprintScreen struct {
	phase    sprintPhase
	length   int
	duration time.Duration
	cursor   int

	// session is the run this screen most recently ended, kept only for the
	// summary to read. Nil until one has been begun.
	session *sprintSession
}

func newSprintScreen(length int) sprintScreen {
	s := sprintScreen{length: length, duration: defaultSprintDuration}
	if !words.SupportedLength(s.length) {
		s.length = defaultLength
	}
	return s
}

func (m *sprintScreen) update(msg tea.KeyPressMsg) (start, again, back bool) {
	switch msg.String() {
	case "esc", "q":
		return false, false, true
	}

	if m.phase == sprintSummary {
		switch msg.String() {
		case "enter", " ", "n":
			return false, true, false
		}
		return false, false, false
	}

	switch msg.String() {
	case "up", "k":
		m.move(-1)
	case "down", "j":
		m.move(1)
	case "left", "h":
		m.cycle(-1)
	case "right", "l":
		m.cycle(1)
	case "enter", " ":
		start = true
	}
	return start, false, false
}

func (m *sprintScreen) move(delta int) {
	m.cursor = min(max(m.cursor+delta, 0), sprintRows-1)
}

func (m *sprintScreen) point(row int) {
	if row >= 0 && row < sprintRows {
		m.cursor = row
	}
}

func (m *sprintScreen) cycle(delta int) {
	switch m.cursor {
	case sprintRowLength:
		m.length = stepLength(m.length, delta)
	case sprintRowTime:
		m.duration = sprintDurations[wrap(sprintDurationIndex(m.duration), delta, len(sprintDurations))]
	}
}

// sprintNotes are the one-line explanations under the rows, sized like the
// custom screen's so moving the cursor does not resize the panel.
var sprintNotes = struct{ length, time string }{
	length: "every board in the run is this long",
	time:   "how long the clock runs",
}

func (m *sprintScreen) note() string {
	if m.cursor == sprintRowTime {
		return sprintNotes.time
	}
	return sprintNotes.length
}

func (m *sprintScreen) renderRow(h *hitMap, row int, label, value string) string {
	prev := action{kind: actSprintPrev, index: row}
	next := action{kind: actSprintNext, index: row}

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

	cell := lipgloss.NewStyle().Width(valueWidth).Align(lipgloss.Center).
		Render(valueStyle.Render(value))
	cell = h.mark(next, cell)
	return prefix +
		labelStyle.Render(fmt.Sprintf("%-*s", labelWidth, label)) +
		arrow(prev, st.glyph.ValuePrev) +
		cell +
		arrow(next, st.glyph.ValueNext)
}

func (m *sprintScreen) view(h *hitMap) string {
	if m.phase == sprintSummary && m.session != nil {
		return m.summaryView()
	}

	rows := []string{
		m.renderRow(h, sprintRowLength, "length", fmt.Sprintf("%d letters", m.length)),
		m.renderRow(h, sprintRowTime, "clock", sprintDurationLabel(m.duration)),
	}
	note := lipgloss.NewStyle().Width(noteWidth()).Align(lipgloss.Center).
		Render(st.muted.Render(m.note()))

	return lipgloss.JoinVertical(lipgloss.Center,
		st.title.Render("sprint"),
		"",
		lipgloss.JoinVertical(lipgloss.Left, rows...),
		"",
		note,
	)
}

// summaryView is the end-of-run card. Every line is derived from the session;
// nothing was written down anywhere else.
func (m *sprintScreen) summaryView() string {
	s := m.session

	outcome := st.accent.Render("time")
	if s.solved == 0 {
		outcome = st.err.Render("nothing solved")
	}

	facts := []string{
		fmt.Sprintf("%d solved", s.solved),
	}
	if s.missed > 0 {
		facts = append(facts, fmt.Sprintf("%d missed", s.missed))
	}
	body := st.text.Render(strings.Join(facts, " · "))

	extra := make([]string, 0, 3)
	if s.solved > 0 {
		extra = append(extra, fmt.Sprintf("%.1f guesses avg",
			float64(s.attempts)/float64(s.solved)))
	}
	if s.dealt > 0 {
		extra = append(extra, fmt.Sprintf("%d%% accuracy", s.solved*100/s.dealt))
	}
	if s.solveTime > 0 {
		extra = append(extra, formatDuration(s.solveTime)+" solving")
	}

	lines := []string{
		st.title.Render("sprint over"),
		st.muted.Render(fmt.Sprintf("%s · %d letters",
			sprintDurationLabel(s.duration), s.length)),
		"",
		outcome,
		body,
	}
	if len(extra) > 0 {
		lines = append(lines, "", st.muted.Render(strings.Join(extra, " · ")))
	}
	return lipgloss.JoinVertical(lipgloss.Center, lines...)
}

func (m *sprintScreen) help(h *hitMap) string {
	if m.phase == sprintSummary {
		return renderHelp(h,
			helpItem{keys: "enter", label: "again", act: action{kind: actSprintAgain}},
			helpItem{keys: "esc", label: "menu", act: action{kind: actBack}},
		)
	}
	return renderHelp(h,
		helpItem{keys: "↑/↓", label: "move"},
		helpItem{keys: "←/→", label: "change",
			act: action{kind: actSprintNext, index: m.cursor}},
		helpItem{keys: "enter", label: "start", act: action{kind: actSprintStart}},
		helpItem{keys: "esc", label: "back", act: action{kind: actBack}},
	)
}
