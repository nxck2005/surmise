// Package ui is the Bubble Tea front end.
//
// A single root Model owns the screen stack and routes key presses to whichever
// screen is active. The screens are plain structs rather than nested
// tea.Models: they render to strings and report back what the root should do,
// which keeps message plumbing to one place.
package ui

import (
	"fmt"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/nxck2005/wortle/internal/store"
	"github.com/nxck2005/wortle/internal/words"
)

type screen int

const (
	screenMenu screen = iota
	screenGame
	screenList
	screenProfile
)

// tickInterval drives the on-screen clock and expires transient messages.
const tickInterval = time.Second

type tickMsg time.Time

func tick() tea.Cmd {
	return tea.Tick(tickInterval, func(t time.Time) tea.Msg { return tickMsg(t) })
}

// Model is the root of the application.
type Model struct {
	store store.Store

	screen  screen
	menu    menuScreen
	game    *gameScreen
	list    listScreen
	profile profileScreen

	width, height int
	err           error
	quitting      bool
}

// defaultLength is the word length the app opens on, matching classic Wordle.
const defaultLength = 5

// New builds the root model over a store and opens straight into a puzzle, the
// way monkeytype drops you onto a test. The menu is one esc away. This first
// puzzle is transient until played, so launching and quitting saves nothing.
func New(s store.Store) *Model {
	m := &Model{store: s, menu: newMenuScreen()}

	g, err := newPuzzle(s, defaultLength)
	if err != nil {
		m.err = err
		m.screen = screenMenu
		return m
	}
	m.game = newGameScreen(s, g, false)
	m.screen = screenGame
	return m
}

func (m *Model) Init() tea.Cmd { return tick() }

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		return m, nil

	case tickMsg:
		// Redraw so the clock advances and expired messages disappear.
		return m, tick()

	case tea.KeyPressMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

func (m *Model) handleKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	// ctrl+c always exits, and must not lose an in-progress puzzle.
	if msg.String() == "ctrl+c" {
		return m, m.quit()
	}

	switch m.screen {
	case screenMenu:
		return m.updateMenu(msg)
	case screenGame:
		cmd, back := m.game.update(msg)
		if back {
			m.screen = screenMenu
		}
		return m, cmd
	case screenList:
		return m.updateList(msg)
	case screenProfile:
		if key := msg.String(); key == "esc" || key == "q" {
			m.screen = screenMenu
		}
		return m, nil
	}
	return m, nil
}

func (m *Model) updateMenu(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	choice, chosen := m.menu.update(msg)
	if !chosen {
		return m, nil
	}

	switch choice.kind {
	case choiceNewGame:
		g, err := newPuzzle(m.store, choice.length)
		if err != nil {
			m.err = err
			return m, nil
		}
		m.game = newGameScreen(m.store, g, false)
		m.screen = screenGame

	case choiceList:
		m.list.reload(m.store)
		m.screen = screenList

	case choiceProfile:
		m.profile.reload(m.store)
		m.screen = screenProfile

	case choiceQuit:
		return m, m.quit()
	}
	return m, nil
}

func (m *Model) updateList(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	open, back := m.list.update(msg)
	switch {
	case back:
		m.screen = screenMenu
	case open:
		summary, ok := m.list.selected()
		if !ok {
			return m, nil
		}
		g, err := m.store.Load(summary.ID)
		if err != nil {
			m.err = err
			return m, nil
		}
		m.game = newGameScreen(m.store, g, true)
		m.screen = screenGame
	}
	return m, nil
}

// quit saves any open puzzle before exiting, so quitting mid-game is a pause
// rather than a loss.
func (m *Model) quit() tea.Cmd {
	m.quitting = true
	if m.screen == screenGame && m.game != nil {
		if err := m.game.leave(); err != nil {
			m.err = err
		}
	}
	return tea.Quit
}

func (m *Model) View() tea.View {
	var v tea.View
	v.AltScreen = true
	v.BackgroundColor = colorBg
	v.ForegroundColor = colorText
	v.WindowTitle = "wortle"

	if m.quitting {
		return v
	}

	body, help := m.activeScreen()
	if m.err != nil {
		body = lipgloss.JoinVertical(lipgloss.Left,
			body, "", errorStyle.Render(fmt.Sprintf("error: %v", m.err)))
	}

	content := lipgloss.JoinVertical(lipgloss.Center, body, help)

	// Box the content in a rounded, titled panel (btop-style) and centre that
	// panel in the terminal. The border hugs the content, not the terminal
	// edges. Before the first WindowSizeMsg the dimensions are zero, so the
	// panel is emitted on its own.
	panel := renderPanel(m.screenTitle(), content)
	if m.width > 0 && m.height > 0 {
		v.Content = lipgloss.Place(m.width, m.height,
			lipgloss.Center, lipgloss.Center, panel)
	} else {
		v.Content = panel
	}
	return v
}

// screenTitle is the label shown in the panel's top border.
func (m *Model) screenTitle() string {
	switch m.screen {
	case screenList:
		return "puzzles"
	case screenProfile:
		return "profile"
	default:
		return "wortle"
	}
}

func (m *Model) activeScreen() (body, help string) {
	switch m.screen {
	case screenGame:
		return m.game.view(), m.game.help()
	case screenList:
		return m.list.view(), m.list.help()
	case screenProfile:
		return m.profile.view(), m.profile.help()
	default:
		return m.menu.view(), m.menu.help()
	}
}

// --- menu ---

type choiceKind int

const (
	choiceNewGame choiceKind = iota
	choiceList
	choiceProfile
	choiceQuit
)

type choice struct {
	kind   choiceKind
	label  string
	length int // for choiceNewGame
}

type menuScreen struct {
	choices []choice
	cursor  int
}

func newMenuScreen() menuScreen {
	// Word lengths lead the menu; they are the game's difficulty modes, the
	// way 15/30/60 lead monkeytype.
	choices := make([]choice, 0, len(words.Lengths)+3)
	for _, n := range words.Lengths {
		choices = append(choices, choice{
			kind:   choiceNewGame,
			label:  fmt.Sprintf("%d letters", n),
			length: n,
		})
	}
	choices = append(choices,
		choice{kind: choiceList, label: "puzzles"},
		choice{kind: choiceProfile, label: "profile"},
		choice{kind: choiceQuit, label: "quit"},
	)
	return menuScreen{choices: choices}
}

func (m *menuScreen) update(msg tea.KeyPressMsg) (choice, bool) {
	switch msg.String() {
	case "up", "k":
		m.cursor = max(m.cursor-1, 0)
	case "down", "j":
		m.cursor = min(m.cursor+1, len(m.choices)-1)
	case "q":
		return choice{kind: choiceQuit}, true
	case "enter", " ":
		return m.choices[m.cursor], true
	}
	return choice{}, false
}

func (m *menuScreen) view() string {
	heading := titleStyle.Render("wortle") + mutedStyle.Render("  wordle for the terminal")

	// Every row is centred to a common width so the list sits under the middle
	// of the heading. The selected item is flanked symmetrically so its marker
	// does not throw off the centring.
	width := 0
	for _, c := range m.choices {
		if w := lipgloss.Width(c.label) + 4; w > width {
			width = w
		}
	}
	center := lipgloss.NewStyle().Width(width).Align(lipgloss.Center)

	rows := make([]string, len(m.choices))
	for i, c := range m.choices {
		if i == m.cursor {
			rows[i] = center.Render(accentStyle.Bold(true).Render("› " + c.label + " ‹"))
		} else {
			rows[i] = center.Render(mutedStyle.Render(c.label))
		}
	}

	list := lipgloss.JoinVertical(lipgloss.Center, rows...)
	return lipgloss.JoinVertical(lipgloss.Center, heading, "", list)
}

func (m *menuScreen) help() string {
	return helpStyle.Render("↑/↓ move · enter select · q quit")
}

// compile-time check that the root model satisfies the framework contract.
var _ tea.Model = (*Model)(nil)
