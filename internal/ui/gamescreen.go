package ui

import (
	"errors"
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/nxck2005/surmise/internal/brand"
	"github.com/nxck2005/surmise/internal/game"
	"github.com/nxck2005/surmise/internal/store"
)

// messageTTL is how long a rejected-guess message stays on screen.
const messageTTL = 2 * time.Second

// gameScreen is the board. It owns the puzzle currently being played and is
// responsible for saving it; nothing else writes to the store while a game is
// open.
type gameScreen struct {
	store store.Store
	g     *game.Game

	typing   string
	message  string
	msgUntil time.Time

	// confirmNew is set by Tab and cleared by Enter or Esc, giving the
	// deliberate two-key restart.
	confirmNew bool

	// persisted is false for a fresh puzzle until its first guess. A puzzle
	// nobody has played is not written to disk, so it never clutters the list
	// with a 0/6 entry.
	persisted bool

	// sessionStart is when the player last started typing on this board, not
	// when they opened it: the clock runs from the first keystroke, the way a
	// typing test does, so staring at a fresh board costs nothing. Zero means no
	// session is in progress — either nothing has been typed yet, or the time
	// has been banked into the game, which happens on the way out and on the
	// guess that ends the puzzle so idle time is never counted.
	sessionStart time.Time

	// width and height are the terminal's, pushed down by the root. The board is
	// the tallest screen in the app, so it is the one that has to decide whether
	// an optional row — the colour legend — fits. Zero means "not measured yet".
	width, height int
}

// resize records the terminal size. The root calls it on every WindowSizeMsg
// and when a board is opened.
func (m *gameScreen) resize(w, h int) { m.width, m.height = w, h }

// newGameScreen wraps a puzzle. saved reports whether it is already on disk:
// true for a puzzle loaded from the list, false for a freshly created one.
//
// The clock is not started here — startClock does that on the first letter.
func newGameScreen(s store.Store, g *game.Game, saved bool) *gameScreen {
	return &gameScreen{store: s, g: g, persisted: saved}
}

// enter arms the session clock for a board the player has just opened or
// returned to: it does not run until the next letter is typed.
func (m *gameScreen) enter() { m.sessionStart = time.Time{} }

// startClock begins a session on the first letter typed into it. Every later
// letter finds a session already running and leaves it alone, so the clock
// measures from the first keystroke to the last guess, not from the moment the
// board appeared. Time is banked and the clock zeroed by leave and by the guess
// that finishes the puzzle; typing again after that starts a fresh session.
func (m *gameScreen) startClock() {
	if m.sessionStart.IsZero() {
		m.sessionStart = time.Now()
	}
}

// leave banks the current session's time and saves. Called on every exit path
// so a puzzle abandoned with ctrl+c still records its time. An unplayed puzzle
// (no guesses, never persisted) is simply discarded.
//
// A finished puzzle banks nothing: its time was banked by the guess that ended
// it, and the seconds after that are spent reviewing, not solving. Counting them
// would inflate the recorded solve — and, because AddElapsed bumps UpdatedAt,
// would also reorder the completion sequence the streaks are read from. This
// mirrors elapsed(), which stops the clock on the same condition.
func (m *gameScreen) leave() error {
	if !m.persisted {
		m.sessionStart = time.Time{}
		return nil
	}
	if !m.sessionStart.IsZero() {
		if !m.g.Status.Done() {
			m.g.AddElapsed(time.Since(m.sessionStart))
		}
		m.sessionStart = time.Time{}
	}
	return m.store.Save(m.g)
}

// exit leaves the board for the menu, banking time and saving on the way out.
// Both esc and a click on the menu button come through here.
func (m *gameScreen) exit() {
	if err := m.leave(); err != nil {
		m.notify("could not save: %v", err)
	}
}

// elapsed is play time including the session in progress.
func (m *gameScreen) elapsed() time.Duration {
	d := m.g.Elapsed()
	if !m.sessionStart.IsZero() && !m.g.Status.Done() {
		d += time.Since(m.sessionStart)
	}
	return d
}

func (m *gameScreen) notify(format string, args ...any) {
	m.message = fmt.Sprintf(format, args...)
	m.msgUntil = time.Now().Add(messageTTL)
}

// update handles a key press, returning a command and whether the screen wants
// to hand control back to the menu.
func (m *gameScreen) update(msg tea.KeyPressMsg) (tea.Cmd, bool) {
	key := msg.String()

	// Tab arms a new puzzle; the next Enter confirms it.
	if m.confirmNew {
		switch key {
		case "enter":
			m.confirmNew = false
			return m.startNew(), false
		case "tab", "esc":
			m.confirmNew = false
			return nil, false
		}
	}

	switch key {
	case "esc":
		m.exit()
		return nil, true

	case "tab":
		m.confirmNew = true
		return nil, false

	case "backspace":
		m.deleteLetter()

	case "enter":
		return m.submit(), false
	}

	// Letter input. Single printable ASCII letters only; anything else is a
	// keybind we do not handle.
	if len(key) == 1 && key[0] >= 'a' && key[0] <= 'z' {
		m.typeLetter(key[0])
	}
	return nil, false
}

// typeLetter, deleteLetter and trimTo are the editing intents, kept separate
// from key matching so a click on the on-screen keyboard or on a typed tile
// reaches exactly the same code a keystroke does.
func (m *gameScreen) typeLetter(c byte) {
	if c < 'a' || c > 'z' {
		return
	}
	if !m.g.Status.Done() && len(m.typing) < m.g.Length {
		// The clock starts here, and only here, so both a keystroke and a click
		// on the on-screen keyboard start it — a letter is the first thing
		// either can produce.
		m.startClock()
		m.typing += string(c)
	}
}

func (m *gameScreen) deleteLetter() {
	if len(m.typing) > 0 {
		m.typing = m.typing[:len(m.typing)-1]
	}
}

// trimTo erases the row being typed back to slot i, so clicking the third tile
// leaves the first two letters standing.
func (m *gameScreen) trimTo(i int) {
	if i >= 0 && i < len(m.typing) {
		m.typing = m.typing[:i]
	}
}

func (m *gameScreen) submit() tea.Cmd {
	if m.g.Status.Done() {
		m.notify("puzzle finished — tab then enter for a new one")
		return nil
	}
	if len(m.typing) < m.g.Length {
		m.notify("needs %d letters", m.g.Length)
		return nil
	}

	switch err := m.g.Guess(m.typing); {
	case errors.Is(err, game.ErrNotAWord):
		m.notify("not in word list")
		return nil
	case err != nil:
		m.notify("%v", err)
		return nil
	}

	m.typing = ""

	// Bank time immediately on the winning or losing guess, so the recorded
	// duration is the time actually spent solving. The clock can legitimately
	// not be running — a guess typed entirely before this screen banked its
	// last session, say — and a zero sessionStart would otherwise bank every
	// second since the epoch.
	if m.g.Status.Done() && !m.sessionStart.IsZero() {
		m.g.AddElapsed(time.Since(m.sessionStart))
		m.sessionStart = time.Time{}
	}

	// The first guess is what makes a puzzle worth keeping. Nothing has to be
	// reserved — the puzzle's code comes from its own id — so this only marks
	// the puzzle as worth writing from here on.
	m.persisted = true

	// Save after every guess: a kill -9 should cost nothing.
	if err := m.store.Save(m.g); err != nil {
		m.notify("could not save: %v", err)
	}
	return nil
}

// startNew replaces the board with a fresh puzzle of the same length.
//
// It refuses on a daily. Everywhere else "start another" is the obvious thing
// tab means, but there is only one daily a day: replacing it with a random
// puzzle would look like a reroll of a board that is supposed to be shared, and
// silently leave the day unplayed.
func (m *gameScreen) startNew() tea.Cmd {
	if m.g.Daily != "" {
		m.notify("the daily is one puzzle a day — pick a mode from the menu for another")
		return nil
	}

	if err := m.leave(); err != nil {
		m.notify("could not save: %v", err)
	}

	g, err := newPuzzle(m.store, m.g.Length)
	if err != nil {
		m.notify("could not start puzzle: %v", err)
		m.enter() // stay on the old board; leave() banked its time
		return nil
	}

	m.g = g
	m.persisted = false
	m.typing = ""
	m.message = ""
	m.enter()
	return nil
}

// newPuzzle creates a fresh puzzle in memory. It is not saved until the first
// guess, but its code is final from the start, since the code is derived from
// the id. Shared with the menu, which also starts games.
//
// Codes are six digits, so a long history will eventually produce two puzzles
// wearing the same one. That is harmless — the id is what anything looks a
// puzzle up by — but it reads as a glitch, so a fresh puzzle re-rolls its id a
// few times to avoid clashing with one already saved. Only this random path
// re-rolls: a seeded puzzle (a daily) must keep the id its seed determines,
// even if it happens to collide.
func newPuzzle(s store.Store, length int) (*game.Game, error) {
	return newPuzzleWith(s, length, game.New)
}

// newPuzzleWith is newPuzzle with the draw injected, since random ids cannot be
// made to collide on demand.
func newPuzzleWith(s store.Store, length int, draw func(int) (*game.Game, error)) (*game.Game, error) {
	const attempts = 8

	taken, err := takenCodes(s)
	if err != nil {
		return nil, err
	}
	var g *game.Game
	for range attempts {
		if g, err = draw(length); err != nil {
			return nil, err
		}
		if !taken[game.Code(g.ID)] {
			break
		}
	}
	// Falling out of the loop keeps the last draw. A duplicate code is only
	// cosmetic, so an unlucky run still gets a puzzle rather than an error.
	return g, nil
}

// takenCodes is the set of codes already on disk. A read failure is reported,
// since it means the list the player is about to see is unreliable too.
func takenCodes(s store.Store) (map[string]bool, error) {
	items, err := s.List()
	if err != nil {
		return nil, err
	}
	taken := make(map[string]bool, len(items))
	for _, it := range items {
		taken[game.Code(it.ID)] = true
	}
	return taken, nil
}

func (m *gameScreen) view(h *hitMap) string {
	g := m.g

	// A daily says which day it is, since the board turns over at UTC midnight
	// and so is not always the date on the player's wall clock.
	what := fmt.Sprintf("%d letters", g.Length)
	if g.Daily != "" {
		what = fmt.Sprintf("daily %s · %s", g.Daily, what)
	}
	header := lipgloss.JoinHorizontal(lipgloss.Top,
		st.title.Render(fmt.Sprintf("%s #%s", brand.Name, game.Code(g.ID))),
		st.muted.Render(fmt.Sprintf("   %s   %s   %d/%d",
			what, formatDuration(m.elapsed()), g.Attempts(), g.MaxAttempts)),
	)

	sections := []string{
		header,
		"",
		renderBoard(g, m.typing, h),
		"",
		renderKeyboard(g.LetterStates(), h),
		"",
		m.statusLine(h),
	}
	// Last, under the status line and spaced off it like every other section: a
	// reference the player consults, not something to read past on the way to
	// the board.
	if legend := renderLegend(); m.fits(legend) {
		sections = append(sections, "", legend)
	}
	// Centre the sections relative to each other so the header and status line
	// sit under the middle of the board and keyboard rather than hugging the
	// left edge.
	return lipgloss.JoinVertical(lipgloss.Center, sections...)
}

// fits reports whether the terminal can afford the legend row. The board plus
// keyboard already nears a 24-row terminal, and tile_width is themeable, so the
// legend is the first thing to go when either axis runs short. An unmeasured
// size (before the first WindowSizeMsg) counts as unbounded.
func (m *gameScreen) fits(legend string) bool {
	if m.width <= 0 || m.height <= 0 {
		return true
	}

	// What the screen costs without the legend: header, blank, board rows with a
	// blank between each, blank, three keyboard rows likewise, blank, status.
	body := 1 + 1 + (2*m.g.MaxAttempts - 1) + 1 + 5 + 1 + 1
	// Plus the help bar and its margin, and the panel's padding and border.
	chrome := 2 + 2*st.metric.PanelPadY + 2
	// The legend costs two rows: itself and the blank that spaces it off.
	if body+chrome+2 > m.height {
		return false
	}
	return lipgloss.Width(legend)+2*st.metric.PanelPadX+2 <= m.width
}

// statusLine carries whichever of the transient message, the end-of-game
// result, or the restart prompt is most important right now.
func (m *gameScreen) statusLine(h *hitMap) string {
	if m.confirmNew {
		// Both halves of the prompt are clickable, so a player who armed the
		// prompt with tab is not stranded without a keyboard.
		confirm := action{kind: actNewPuzzle}
		cancel := action{kind: actCancelNew}
		return h.mark(confirm, st.accent.Render("enter")) +
			st.muted.Render(" to start a new puzzle · ") +
			h.mark(cancel, st.accent.Render("esc")) +
			st.muted.Render(" to cancel")
	}
	if m.message != "" && time.Now().Before(m.msgUntil) {
		return st.err.Render(m.message)
	}

	switch m.g.Status {
	case game.Won:
		return st.accent.Render(fmt.Sprintf("solved in %d — %s",
			m.g.Attempts(), formatDuration(m.g.Elapsed())))
	case game.Lost:
		return st.err.Render("out of guesses — ") +
			st.text.Render(strings.ToUpper(m.g.Answer))
	default:
		return st.muted.Render(fmt.Sprintf("%d guesses left", m.g.Remaining()))
	}
}

func (m *gameScreen) help(h *hitMap) string {
	return renderHelp(h,
		helpItem{label: "type a word"},
		helpItem{keys: "enter", label: "submit", act: action{kind: actSubmit}},
		helpItem{keys: "tab+enter", label: "new puzzle", act: action{kind: actNewPuzzle}},
		helpItem{keys: "esc", label: "menu", act: action{kind: actBack}},
	)
}
