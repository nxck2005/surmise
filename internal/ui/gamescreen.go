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

	// playtime hands a banked session to the lifetime counter, which the root
	// owns because it lives in the settings and this screen only writes puzzles.
	// Nil is a board that counts nothing, which is what a screen built outside
	// the root — every layout test — gets.
	playtime func(time.Duration)

	// anim is the root's animation state, shared rather than owned: the board
	// starts the effects, and the panel around it draws the win accent. Nil is
	// a board that does not animate, which is every nil-safe method's job to
	// handle and what the layout tests render through.
	anim *anims

	// sprint is the timed run this board belongs to, or nil. The root owns the
	// session; the pointer is all the board needs, to show the clock in its
	// status line and to stand down its own restart affordances — a run deals
	// its boards itself.
	sprint *sprintSession

	// width and height are the terminal's, pushed down by the root. The board is
	// the tallest screen in the app, so it is the one that has to decide how
	// much of itself it can afford to draw — see boardLayout. Zero means "not
	// measured yet", which counts as unbounded.
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

// bank records a finished session in both places time is kept: on the puzzle,
// where it becomes the solve the profile averages, and in the lifetime counter.
// Every site that banks goes through here, so the two can never learn about a
// session separately.
//
// It is the only caller of AddElapsed outside the tests. Nothing that merely
// reads or aggregates may call that method: it bumps UpdatedAt, which is the
// order the streaks are walked in.
func (m *gameScreen) bank(d time.Duration) {
	m.g.AddElapsed(d)
	m.countPlaytime(d)
}

// countPlaytime adds to the lifetime counter alone. Only the discarded-puzzle
// path in leave wants this without the other half of bank.
func (m *gameScreen) countPlaytime(d time.Duration) {
	if m.playtime != nil {
		m.playtime(d)
	}
}

// leave banks the current session's time and saves. Called on every exit path
// so a puzzle abandoned with ctrl+c still records its time. An unplayed puzzle
// (no guesses, never persisted) is discarded — but its minutes still reach the
// lifetime counter, which counts time in the app rather than time on a record.
//
// A finished puzzle banks nothing: its time was banked by the guess that ended
// it, and the seconds after that are spent reviewing, not solving. Counting them
// would inflate the recorded solve — and, because AddElapsed bumps UpdatedAt,
// would also reorder the completion sequence the streaks are read from. This
// mirrors elapsed(), which stops the clock on the same condition.
func (m *gameScreen) leave() error {
	if !m.sessionStart.IsZero() && !m.g.Status.Done() {
		d := time.Since(m.sessionStart)
		if m.persisted {
			m.bank(d)
		} else {
			m.countPlaytime(d)
		}
	}
	m.sessionStart = time.Time{}

	if !m.persisted {
		return nil
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

// update handles board-local key presses, returning a command and whether the
// screen wants to hand control back to the menu. The root owns an unarmed Enter
// because an accepted final guess raises the result screen.
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
		// During a run, restarting is not on offer: the run deals its own
		// boards, and a confirm prompt for one it is about to deal anyway
		// would be noise. The help bar drops the hint to match.
		if m.sprint == nil {
			m.confirmNew = true
		}
		return nil, false

	case "backspace":
		m.deleteLetter()

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
	if c < 'a' || c > 'z' || m.refusing() {
		return
	}
	if !m.g.Status.Done() && len(m.typing) < m.g.Length {
		// The clock starts here, and only here, so both a keystroke and a click
		// on the on-screen keyboard start it — a letter is the first thing
		// either can produce. The cap lights for the same reason and in the same
		// place: one seam, so a click feels exactly like a keypress.
		m.startClock()
		m.typing += string(c)
		m.anim.beginKey(c, actLetter)
	}
}

// refusing reports whether the terminal is too short to draw the board at all.
// Input is turned away while it is, so a board nobody can see is never played
// blind — the refusal says what to do about it, and esc still leaves.
func (m *gameScreen) refusing() bool { return m.layout().refuse }

func (m *gameScreen) deleteLetter() {
	if m.refusing() {
		return
	}
	if len(m.typing) > 0 {
		m.typing = m.typing[:len(m.typing)-1]
		m.anim.beginKey(0, actBackspace)
	}
}

// reject flashes the row that was refused. It is the whole of the feedback for
// a guess that cannot be played: the board does not move, so nothing the player
// is pointing at goes anywhere.
func (m *gameScreen) reject() {
	m.anim.beginBoard(animInvalid, m.g.ID, len(m.g.Guesses), m.g.Length)
}

// trimTo erases the row being typed back to slot i, so clicking the third tile
// leaves the first two letters standing.
func (m *gameScreen) trimTo(i int) {
	if i >= 0 && i < len(m.typing) {
		m.typing = m.typing[:i]
	}
}

func (m *gameScreen) submit() tea.Cmd {
	if m.refusing() {
		return nil
	}
	// The two refusals below flash the row; the third branch does not. A guess
	// the player can fix is worth a cue, but a save that failed is a sentence,
	// and notify already writes it.
	if len(m.typing) < m.g.Length {
		m.notify("needs %d letters", m.g.Length)
		m.reject()
		return nil
	}

	switch err := m.g.Guess(m.typing); {
	case errors.Is(err, game.ErrNotAWord):
		m.notify("not in word list")
		m.reject()
		return nil
	case err != nil:
		m.notify("%v", err)
		return nil
	}

	// The row that was just accepted is the one that reveals. A win reveals and
	// then accents; a loss reveals faster, because the answer waiting on the
	// result screen is what the player is actually after.
	kind := animReveal
	switch m.g.Status {
	case game.Won:
		kind = animWin
	case game.Lost:
		kind = animLoss
	}
	m.anim.beginBoard(kind, m.g.ID, len(m.g.Guesses)-1, m.g.Length)

	m.typing = ""
	m.message = ""
	m.msgUntil = time.Time{}

	// Bank time immediately on the winning or losing guess, so the recorded
	// duration is the time actually spent solving. The clock can legitimately
	// not be running — a guess typed entirely before this screen banked its
	// last session, say — and a zero sessionStart would otherwise bank every
	// second since the epoch.
	if m.g.Status.Done() && !m.sessionStart.IsZero() {
		m.bank(time.Since(m.sessionStart))
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
//
// It refuses on a custom board for a nearer reason: somebody else chose
// that word and handed the terminal over, and tab is close enough to the keys
// being typed that discarding their puzzle must not be one keystroke away.
func (m *gameScreen) startNew() tea.Cmd {
	if m.g.Daily != "" {
		m.notify("the daily is one puzzle a day — pick a mode from the menu for another")
		return nil
	}
	if m.g.Custom {
		m.notify("this word was set by hand — start another from the menu")
		return nil
	}

	if err := m.leave(); err != nil {
		m.notify("could not save: %v", err)
		return nil
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

	// One decision for the whole frame. Everything below reads it rather than
	// asking the terminal again, so no two sections can disagree about what has
	// been given up.
	l := m.layout()
	if l.refuse {
		return m.refusal(l)
	}

	// One clock for the whole frame: two reads a microsecond apart could put the
	// board and the keyboard on different sides of the same instant.
	now := timeNow()

	sections := make([]string, 0, 9)
	// Which mode this is, whether it is a daily and how many guesses are gone
	// are all on the panel's top rule now (Model.screenStatus), so the header
	// keeps what is the board's own: which puzzle, and how long you have been
	// at it. It is the first thing given up after the legend — the code is in
	// the puzzle list and the clock reappears in the status line on a win.
	if l.header {
		sections = append(sections,
			lipgloss.JoinHorizontal(lipgloss.Top,
				st.title.Render(fmt.Sprintf("%s #%s", brand.Name, game.Code(g.ID))),
				st.muted.Render("   "+formatDuration(m.elapsed())),
			),
			"",
		)
	}

	sections = append(sections,
		renderBoard(g, m.typing, h, m.anim, now, l.tiles, l.boardGap),
		"",
		renderKeyboard(g.LetterStates(), h, m.anim, now, l.kbdGap),
		"",
		m.statusLine(h),
	)
	// Last, under the status line and spaced off it like every other section: a
	// reference the player consults, not something to read past on the way to
	// the board.
	if l.legend {
		sections = append(sections, "", renderLegend())
	}
	// Centre the sections relative to each other so the header and status line
	// sit under the middle of the board and keyboard rather than hugging the
	// left edge.
	return lipgloss.JoinVertical(lipgloss.Center, sections...)
}

// refusal is what a terminal too short for even the tightest board gets. The
// alternative is drawing anyway and losing the panel's title rule and its close
// box off the top of the frame, which is the bug this whole ladder exists for:
// a board nobody can leave with a mouse is worse than no board.
//
// The numbers are measured, not written out, so they cannot drift from the
// arithmetic that produced them.
func (m *gameScreen) refusal(l boardLayout) string {
	return lipgloss.JoinVertical(lipgloss.Center,
		st.err.Render("this terminal is too short"),
		"",
		st.muted.Render(fmt.Sprintf("%d letters needs %d rows · %d here",
			m.g.Length, l.rows(m.g.MaxAttempts), m.height)),
	)
}

// boardLayout is how much of the board screen the terminal can afford. The
// board is the tallest screen in the app, and nothing in the pipeline
// truncates: an over-tall frame loses its *top* rows, which are the panel's
// title rule and its close box. So the screen has smaller forms, and this is
// which one is being drawn.
//
// The rungs are given up in a fixed order, cheapest harm first: the legend, the
// header, the keyboard's gutters, then the board's. The ladder is only ever
// descended — a rung is never climbed back to pay for a lower one, or a
// terminal that had just given up its header would regain the legend.
//
// The status line is deliberately not a rung. It carries the transient error,
// the win and loss result, and the tab-then-enter prompt whose two halves are
// click targets; none of that is anywhere else, and the panel's rule duplicates
// only what the header used to say.
type boardLayout struct {
	tiles    int  // tile height: flatTile, or tallTile when there is room to spare
	header   bool // the puzzle code and the running clock
	legend   bool // the colour key under the board
	kbdGap   int  // blank rows between the keyboard's three rows
	boardGap int  // blank rows between the guess rows
	refuse   bool // even the tightest form overflows; say so instead
}

// rows is what this form takes, top to bottom: the header and its blank, the
// board rows with their gutters, a blank, three keyboard rows with theirs, a
// blank, the status line, then the legend and the blank that spaces it off —
// plus the help bar and its margin, the panel's padding and its border.
func (l boardLayout) rows(attempts int) int {
	body := attempts*l.tiles + (attempts-1)*l.boardGap +
		1 + (3 + 2*l.kbdGap) + 1 + 1
	if l.header {
		body += 2
	}
	if l.legend {
		body += 2
	}
	chrome := 2 + 2*st.metric.PanelPadY + 2
	return body + chrome
}

// tallSpare is how many rows a tall board keeps in hand: a board that grew
// until it exactly filled the terminal would leave the player nowhere to put an
// error line.
const tallSpare = 1

// layout walks the ladder and returns the first form that fits. An unmeasured
// size — before the first WindowSizeMsg — counts as unbounded, the same rule
// affordableSections follows, and it is what keeps the headless tests drawing
// whole screens.
func (m *gameScreen) layout() boardLayout {
	l := boardLayout{tiles: flatTile, header: true, legend: m.legendFitsWidth(), kbdGap: 1, boardGap: 1}
	if m.width <= 0 || m.height <= 0 {
		return l
	}

	attempts := m.g.MaxAttempts

	// The tall board is asked for first, and only from the untightened form: a
	// window with room to grow is by definition a window with nothing to give
	// up, so growing and shedding can never combine.
	if l.legend {
		if tall := l.withTiles(tallTile); tall.rows(attempts)+tallSpare <= m.height {
			return tall
		}
	}

	for _, give := range []func(*boardLayout){
		func(l *boardLayout) { l.legend = false },
		func(l *boardLayout) { l.header = false },
		func(l *boardLayout) { l.kbdGap = 0 },
		func(l *boardLayout) { l.boardGap = 0 },
	} {
		if l.rows(attempts) <= m.height {
			return l
		}
		give(&l)
	}
	if l.rows(attempts) > m.height {
		l.refuse = true
	}
	return l
}

func (l boardLayout) withTiles(tiles int) boardLayout {
	l.tiles = tiles
	return l
}

// legendFitsWidth is the legend's other axis. tile_width is themeable, so a
// wide theme can price the legend out of a terminal with rows to spare — and a
// legend already gone for width buys no rows, so this is a precondition on the
// top of the ladder rather than a rung of it.
func (m *gameScreen) legendFitsWidth() bool {
	if m.width <= 0 {
		return true
	}
	return lipgloss.Width(renderLegend())+2*st.metric.PanelPadX+2 <= m.width
}

// tileRows is how tall a board tile is drawn. The board is one row tall by
// default and always has been — the tallest mode plus the keyboard already
// nears a 24-row terminal — but a window with room to spare gets the fuller
// board, where a letter sits in the middle of its tile instead of being the
// whole of it.
func (m *gameScreen) tileRows() int { return m.layout().tiles }

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
		line := st.muted.Render(fmt.Sprintf("%d guesses left", m.g.Remaining()))
		if m.sprint != nil && m.sprint.running() {
			// The run's clock leads the line: it is the one thing a sprint
			// player reads mid-board, and the solved count rides with it.
			line = st.accent.Render(sprintClock(m.sprint.left())) +
				st.muted.Render(fmt.Sprintf(" · %d solved · ", m.sprint.solved)) +
				line
		}
		return line
	}
}

func (m *gameScreen) help(h *hitMap) string {
	// A refused board takes no input, so it offers no key that would do
	// nothing: a hint for a dead key is a promise the screen does not keep.
	if m.layout().refuse {
		return renderHelp(h, helpItem{keys: "esc", label: "menu", act: action{kind: actBack}})
	}
	if m.sprint != nil {
		// A timed run owns its board supply, so there is no restart to offer;
		// and esc ends the run rather than leaving for the menu.
		items := []helpItem{{
			keys: "enter", label: "submit", act: action{kind: actSubmit}},
		}
		if !m.g.Status.Done() {
			items = append([]helpItem{{label: "type a word"}}, items...)
		}
		return renderHelp(h, append(items,
			helpItem{keys: "esc", label: "end sprint", act: action{kind: actBack}})...)
	}
	if m.g.Status.Done() {
		return renderHelp(h,
			helpItem{keys: "enter", label: "result", act: action{kind: actSubmit}},
			helpItem{keys: "tab+enter", label: "new puzzle", act: action{kind: actNewPuzzle}},
			helpItem{keys: "esc", label: "menu", act: action{kind: actBack}},
		)
	}
	return renderHelp(h,
		helpItem{label: "type a word"},
		helpItem{keys: "enter", label: "submit", act: action{kind: actSubmit}},
		helpItem{keys: "tab+enter", label: "new puzzle", act: action{kind: actNewPuzzle}},
		helpItem{keys: "esc", label: "menu", act: action{kind: actBack}},
	)
}
