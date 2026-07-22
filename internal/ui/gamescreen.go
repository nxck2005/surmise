package ui

import (
	"errors"
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/nxck2005/wortle/internal/game"
	"github.com/nxck2005/wortle/internal/store"
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
	// monkeytype-style two-key restart from IDEA.md.
	confirmNew bool

	// persisted is false for a fresh puzzle until its first guess. A puzzle
	// nobody has played is not written to disk, so it never clutters the list
	// with a 0/6 entry.
	persisted bool

	// sessionStart is when the player last entered this board. Elapsed time is
	// banked into the game on the way out so idle time between sessions is not
	// counted.
	sessionStart time.Time
}

// newGameScreen wraps a puzzle. saved reports whether it is already on disk:
// true for a puzzle loaded from the list, false for a freshly created one.
func newGameScreen(s store.Store, g *game.Game, saved bool) *gameScreen {
	return &gameScreen{store: s, g: g, persisted: saved, sessionStart: time.Now()}
}

// enter restarts the session clock, for when the player returns to a puzzle.
func (m *gameScreen) enter() { m.sessionStart = time.Now() }

// leave banks the current session's time and saves. Called on every exit path
// so a puzzle abandoned with ctrl+c still records its time. An unplayed puzzle
// (no guesses, never persisted) is simply discarded.
func (m *gameScreen) leave() error {
	if !m.persisted {
		m.sessionStart = time.Time{}
		return nil
	}
	if !m.sessionStart.IsZero() {
		m.g.AddElapsed(time.Since(m.sessionStart))
		m.sessionStart = time.Time{}
	}
	return m.store.Save(m.g)
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
		if err := m.leave(); err != nil {
			m.notify("could not save: %v", err)
		}
		return nil, true

	case "tab":
		m.confirmNew = true
		return nil, false

	case "backspace":
		if len(m.typing) > 0 {
			m.typing = m.typing[:len(m.typing)-1]
		}

	case "enter":
		return m.submit(), false
	}

	// Letter input. Single printable ASCII letters only; anything else is a
	// keybind we do not handle.
	if len(key) == 1 && key[0] >= 'a' && key[0] <= 'z' {
		if !m.g.Status.Done() && len(m.typing) < m.g.Length {
			m.typing += key
		}
	}
	return nil, false
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
	// duration is the time actually spent solving.
	if m.g.Status.Done() {
		m.g.AddElapsed(time.Since(m.sessionStart))
		m.sessionStart = time.Time{}
	}

	// The first guess is what makes a puzzle worth keeping: reserve its number
	// now, so abandoned-before-playing puzzles never consume one.
	if !m.persisted {
		n, err := m.store.NextNumber()
		if err != nil {
			m.notify("could not save: %v", err)
			return nil
		}
		m.g.Number = n
		m.persisted = true
	}

	// Save after every guess: a kill -9 should cost nothing.
	if err := m.store.Save(m.g); err != nil {
		m.notify("could not save: %v", err)
	}
	return nil
}

// startNew replaces the board with a fresh puzzle of the same length.
func (m *gameScreen) startNew() tea.Cmd {
	if err := m.leave(); err != nil {
		m.notify("could not save: %v", err)
	}

	g, err := newPuzzle(m.store, m.g.Length)
	if err != nil {
		m.notify("could not start puzzle: %v", err)
		m.sessionStart = time.Now() // stay on the old board
		return nil
	}

	m.g = g
	m.persisted = false
	m.typing = ""
	m.message = ""
	m.enter()
	return nil
}

// newPuzzle creates a fresh puzzle in memory. It is not saved and its number is
// only prospective (see PeekNumber) until the first guess persists it. Shared
// with the menu, which also starts games.
func newPuzzle(s store.Store, length int) (*game.Game, error) {
	number, err := s.PeekNumber()
	if err != nil {
		return nil, err
	}
	return game.New(length, number)
}

func (m *gameScreen) view() string {
	g := m.g

	header := lipgloss.JoinHorizontal(lipgloss.Top,
		titleStyle.Render(fmt.Sprintf("wortle #%d", g.Number)),
		mutedStyle.Render(fmt.Sprintf("   %d letters   %s   %d/%d",
			g.Length, formatDuration(m.elapsed()), g.Attempts(), g.MaxAttempts)),
	)

	sections := []string{
		header,
		"",
		renderBoard(g, m.typing),
		"",
		renderKeyboard(g.LetterStates()),
		"",
		m.statusLine(),
	}
	// Centre the sections relative to each other so the header and status line
	// sit under the middle of the board and keyboard rather than hugging the
	// left edge.
	return lipgloss.JoinVertical(lipgloss.Center, sections...)
}

// statusLine carries whichever of the transient message, the end-of-game
// result, or the restart prompt is most important right now.
func (m *gameScreen) statusLine() string {
	if m.confirmNew {
		return accentStyle.Render("enter") + mutedStyle.Render(" to start a new puzzle · ") +
			accentStyle.Render("esc") + mutedStyle.Render(" to cancel")
	}
	if m.message != "" && time.Now().Before(m.msgUntil) {
		return errorStyle.Render(m.message)
	}

	switch m.g.Status {
	case game.Won:
		return accentStyle.Render(fmt.Sprintf("solved in %d — %s",
			m.g.Attempts(), formatDuration(m.g.Elapsed())))
	case game.Lost:
		return errorStyle.Render("out of guesses — ") +
			textStyle.Render(strings.ToUpper(m.g.Answer))
	default:
		return mutedStyle.Render(fmt.Sprintf("%d guesses left", m.g.Remaining()))
	}
}

func (m *gameScreen) help() string {
	return helpStyle.Render("type a word · enter submit · tab+enter new puzzle · esc menu")
}
