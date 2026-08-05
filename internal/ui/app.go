// Package ui is the Bubble Tea front end.
//
// A single root Model owns the screen stack and routes key presses to whichever
// screen is active. The screens are plain structs rather than nested
// tea.Models: they render to strings and report back what the root should do,
// which keeps message plumbing to one place.
package ui

import (
	"context"
	"errors"
	"fmt"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/nxck2005/wortle/internal/daily"
	"github.com/nxck2005/wortle/internal/game"
	"github.com/nxck2005/wortle/internal/store"
	"github.com/nxck2005/wortle/internal/theme"
	"github.com/nxck2005/wortle/internal/words"
)

type screen int

const (
	screenMenu screen = iota
	screenGame
	screenList
	screenProfile
	screenThemes
	screenSettings
	screenDaily
)

// tickInterval drives the on-screen clock and expires transient messages.
const tickInterval = time.Second

// dailyTimeout bounds building a daily. The local source cannot block, so this
// is entirely for the remote one that replaces it: a daily that cannot be
// fetched must fail and say so, not hang the menu.
const dailyTimeout = 10 * time.Second

// dailyMsg carries a built daily back from the command that made it.
type dailyMsg struct {
	g   *game.Game
	err error
}

type tickMsg time.Time

func tick() tea.Cmd {
	return tea.Tick(tickInterval, func(t time.Time) tea.Msg { return tickMsg(t) })
}

// Model is the root of the application.
type Model struct {
	store store.Store

	// themes is every theme available this run; themeName is the committed
	// choice, which the picker restores when a preview is abandoned.
	themeLib  *theme.Library
	themeName string

	// length is the word length a puzzle started from here gets: the resolved
	// default, not necessarily what the open board is playing.
	length int

	// day is the date the daily plays, resolved once at startup; dailySrc is
	// where its seed comes from. The source is a field rather than a package
	// default so a test — and, later, a remote build — can swap it.
	day      daily.Day
	dailySrc daily.Source

	screen   screen
	menu     menuScreen
	game     *gameScreen
	list     listScreen
	profile  profileScreen
	themes   themeScreen
	settings settingsScreen
	daily    dailyScreen

	// hits is where the last frame drew its clickable regions; hover is what the
	// pointer was last over, which the next frame highlights. Both are written
	// by View, which the framework calls on the same goroutine as Update
	// immediately afterwards, so there is no race between drawing and clicking.
	hits  *hitMap
	hover action

	width, height int
	err           error
	quitting      bool
}

// defaultLength is the word length the app opens on when nothing else says
// otherwise, matching classic Wordle.
const defaultLength = 5

// Options are the one-run overrides, from flags or the environment. Every zero
// value means "use whatever was saved", so a new override is additive here
// rather than another positional argument to New.
type Options struct {
	// Theme forces a theme by name without changing the saved choice.
	Theme string
	// Length forces the starting word length, likewise without saving it.
	Length int
	// Day forces the date the daily plays, "2006-01-02". For looking at another
	// day's board without waiting for it; like the others it is never saved.
	Day string
	// DailySeeds overrides where daily seeds come from. Nil means daily.Local,
	// which is the only source there is today.
	DailySeeds daily.Source
}

// settingsStore is the part of a store that remembers preferences. It is a
// separate interface rather than part of store.Store because a remote backend
// might well serve puzzles without owning the local look; a store that does not
// implement it simply cannot persist a theme choice.
type settingsStore interface {
	Settings() store.Settings
	SaveSettings(store.Settings) error
}

// New builds the root model over a store and opens straight into a puzzle, the
// way a typing test drops you straight into typing. The menu is one esc away. This first
// puzzle is transient until played, so launching and quitting saves nothing.
//
// lib is the available themes; nil means the bundled set. opts carries the
// one-run overrides; its zero value means "use whatever was saved".
func New(s store.Store, lib *theme.Library, opts Options) *Model {
	if lib == nil {
		lib = theme.Bundled()
	}
	m := &Model{store: s, themeLib: lib, menu: newMenuScreen(), dailySrc: opts.DailySeeds}
	if m.dailySrc == nil {
		m.dailySrc = daily.Local()
	}
	m.applyStartupTheme(opts.Theme)
	m.applyStartupLength(opts.Length)
	m.applyStartupDay(opts.Day)

	g, err := newPuzzle(s, m.length)
	if err != nil {
		m.err = err
		m.screen = screenMenu
		return m
	}
	m.openGame(g, false)
	return m
}

// openGame installs a puzzle as the active screen, handing it the current
// terminal size: the board is the one screen that trims itself to fit.
func (m *Model) openGame(g *game.Game, saved bool) {
	m.game = newGameScreen(m.store, g, saved)
	m.game.resize(m.width, m.height)
	m.screen = screenGame
}

// applyStartupTheme resolves which theme to open with: an explicit override
// first, then whatever was saved, then the default. A name that resolves to
// nothing is reported rather than swallowed, so a typo in -theme is visible.
func (m *Model) applyStartupTheme(override string) {
	want := override
	if want == "" {
		if ss, ok := m.store.(settingsStore); ok {
			want = ss.Settings().Theme
		}
	}

	t, ok := m.themeLib.Resolve(want)
	if !ok {
		m.err = fmt.Errorf("no theme named %q — using %s", want, theme.DefaultName)
	}
	m.themeName = t.Name
	setTheme(t)
}

// applyStartupLength resolves the mode the app opens on, the same way
// applyStartupTheme resolves the look: an explicit override first, then the
// saved choice, then the built-in default. An unsupported length is reported
// rather than silently corrected — a typo in -length should be visible — but is
// never fatal, and a saved zero simply means nothing was ever chosen.
func (m *Model) applyStartupLength(override int) {
	m.length = defaultLength

	want := override
	if want == 0 {
		want = m.settingsOf().Length
	}
	switch {
	case want == 0:
	case words.SupportedLength(want):
		m.length = want
	default:
		m.err = fmt.Errorf("no %d-letter mode — using %d", want, defaultLength)
	}
}

// applyStartupDay resolves which date the daily plays. Unlike the theme and the
// mode there is nothing saved to fall back to — a date is not a preference — so
// it is the override or today. A date that will not parse is reported and
// ignored, the same as an unknown theme or an unsupported length.
func (m *Model) applyStartupDay(override string) {
	m.day = daily.Today()
	if override == "" {
		return
	}
	d, err := daily.ParseDay(override)
	if err != nil {
		m.err = err
		return
	}
	m.day = d
}

// settingsOf reads the saved preferences, or their defaults from a store that
// does not keep any.
func (m *Model) settingsOf() store.Settings {
	if ss, ok := m.store.(settingsStore); ok {
		return ss.Settings()
	}
	return store.Settings{}
}

// saveSettings writes preferences back, reporting a failure on the error line.
// Callers read-modify-write through settingsOf so one field never clobbers
// another.
func (m *Model) saveSettings(s store.Settings) {
	ss, ok := m.store.(settingsStore)
	if !ok {
		return
	}
	if err := ss.SaveSettings(s); err != nil {
		m.err = err
	}
}

func (m *Model) Init() tea.Cmd { return tick() }

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		if m.game != nil {
			m.game.resize(m.width, m.height)
		}
		return m, nil

	case tickMsg:
		// Redraw so the clock advances and expired messages disappear.
		return m, tick()

	case dailyMsg:
		if msg.err != nil {
			m.err = msg.err // stay where we are; nothing has been created
			return m, nil
		}
		// Transient until the first guess, like every other new puzzle: opening
		// the daily and walking away saves nothing.
		m.openGame(msg.g, false)
		return m, nil

	case tea.KeyPressMsg:
		return m.handleKey(msg)

	case tea.MouseClickMsg:
		// Only the left button acts; a click on nothing is a no-op. Release is
		// deliberately ignored, so a target fires the moment it is pressed.
		if msg.Button != tea.MouseLeft {
			return m, nil
		}
		if a, ok := m.hits.at(msg.X, msg.Y); ok {
			return m, m.dispatch(a)
		}

	case tea.MouseMotionMsg:
		m.handleMotion(msg.X, msg.Y)

	case tea.MouseWheelMsg:
		// The puzzle list and the theme list are the only things long enough
		// to scroll.
		delta := 0
		switch msg.Button {
		case tea.MouseWheelUp:
			delta = -1
		case tea.MouseWheelDown:
			delta = 1
		}
		switch m.screen {
		case screenList:
			m.list.scroll(delta)
		case screenThemes:
			m.themes.scroll(delta)
		}
	}
	return m, nil
}

// handleMotion records what the pointer is over so the next render can
// highlight it. On the menu and the puzzle list the selection follows the
// pointer as well, which is what makes one click enough to act.
func (m *Model) handleMotion(x, y int) {
	a, _ := m.hits.at(x, y) // off any target, a is the zero action: no hover
	m.hover = a

	switch a.kind {
	case actMenuChoice:
		m.menu.point(a.index)
	case actListRow, actDeletePuzzle:
		// The delete button carries the row it refers to, so hovering it keeps
		// the selection and the prompt talking about the same puzzle.
		m.list.point(a.index)
	case actDailyRow:
		m.daily.point(a.index)
	case actThemeRow:
		// Hovering a theme previews it, the same as arrowing onto it.
		m.themes.point(a.index)
	case actSettingNext, actSettingPrev:
		m.settings.point(a.index)
	}
}

// dispatch performs a clicked action. Every branch routes into the same methods
// the key handlers call, so clicking and typing cannot drift apart.
func (m *Model) dispatch(a action) tea.Cmd {
	switch a.kind {
	case actQuit:
		return m.quit()

	case actBack:
		return m.back()

	case actMenuChoice:
		if m.screen != screenMenu || a.index >= len(m.menu.choices) {
			return nil
		}
		m.menu.point(a.index)
		return m.applyChoice(m.menu.choices[a.index])

	case actListRow:
		if m.screen != screenList {
			return nil
		}
		m.list.point(a.index)
		return m.openSelected()

	case actDailyRow:
		if m.screen != screenDaily {
			return nil
		}
		m.daily.point(a.index)
		return m.openSelectedDaily()

	case actDeletePuzzle:
		if m.screen != screenList {
			return nil
		}
		m.list.point(a.index)
		// Unlike actNewPuzzle, a click does not skip the confirmation. Starting
		// a puzzle is undoable by starting another; this is not, and a row in a
		// list is an easy thing to mis-click. The first click arms the prompt,
		// and the prompt's own target — a different place on screen — is what
		// carries it out.
		if !m.list.confirmDelete {
			if _, ok := m.list.selected(); ok {
				m.list.confirmDelete = true
			}
			return nil
		}
		m.list.confirmDelete = false
		return m.deleteSelected()

	case actCancelDelete:
		if m.screen != screenList {
			return nil
		}
		m.list.confirmDelete = false
		return nil

	case actThemeRow:
		if m.screen != screenThemes {
			return nil
		}
		m.themes.point(a.index)
		return m.commitTheme()

	case actSettingNext, actSettingPrev:
		if m.screen != screenSettings {
			return nil
		}
		m.settings.point(a.index)
		// Same cycle methods the arrow keys call, so the two paths cannot drift.
		if a.kind == actSettingNext {
			m.settings.cycle(1)
		} else {
			m.settings.cycle(-1)
		}
		m.commitSettings(a.index)
		return nil
	}

	// Everything left belongs to the board.
	if m.screen != screenGame || m.game == nil {
		return nil
	}
	switch a.kind {
	case actLetter:
		m.game.typeLetter(a.letter)
	case actBackspace:
		m.game.deleteLetter()
	case actTrim:
		m.game.trimTo(a.index)
	case actSubmit:
		return m.game.submit()
	case actNewPuzzle:
		// A click is already deliberate, so it needs no tab-then-enter confirm.
		m.game.confirmNew = false
		return m.game.startNew()
	case actCancelNew:
		m.game.confirmNew = false
	}
	return nil
}

// back is what esc does: leave the board, saving on the way out, and return to
// the menu.
func (m *Model) back() tea.Cmd {
	switch {
	case m.screen == screenGame && m.game != nil:
		m.game.exit()
	case m.screen == screenThemes:
		// Leaving the picker without choosing puts back the saved theme, so a
		// preview is never accidentally permanent.
		m.restoreTheme()
	case m.screen == screenList:
		// An armed prompt must not be waiting when the list is next opened.
		m.list.confirmDelete = false
	}
	m.screen = screenMenu
	return nil
}

// commitTheme keeps whatever the picker is currently previewing and writes it
// to the settings file. Both enter and a click land here.
func (m *Model) commitTheme() tea.Cmd {
	e, ok := m.themes.selected()
	if !ok || e.Theme == nil {
		return nil
	}
	m.themeName = e.Name
	m.themes.saved = e.Name
	setTheme(e.Theme)

	s := m.settingsOf()
	s.Theme = e.Name
	m.saveSettings(s)

	m.screen = screenMenu
	return nil
}

// restoreTheme puts back the committed theme after an abandoned preview.
func (m *Model) restoreTheme() {
	if t, ok := m.themeLib.Get(m.themeName); ok {
		setTheme(t)
	}
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
	case screenDaily:
		return m.updateDaily(msg)
	case screenThemes:
		return m.updateThemes(msg)
	case screenSettings:
		return m.updateSettings(msg)
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
	return m, m.applyChoice(choice)
}

// applyChoice acts on a menu entry, whether it was chosen with enter or clicked.
func (m *Model) applyChoice(c choice) tea.Cmd {
	switch c.kind {
	case choiceNewGame:
		g, err := newPuzzle(m.store, c.length)
		if err != nil {
			m.err = err
			return nil
		}
		m.length = c.length
		// With "remember last" on, playing a mode is how the default is set —
		// the settings screen is then a display of what you last played.
		if s := m.settingsOf(); s.RememberLast && s.Length != c.length {
			s.Length = c.length
			m.saveSettings(s)
		}
		m.openGame(g, false)

	case choiceDaily:
		m.openDailyScreen()

	case choiceList:
		m.list.reload(m.store)
		m.screen = screenList

	case choiceProfile:
		m.profile.reload(m.store)
		m.screen = screenProfile

	case choiceThemes:
		m.themes.reload(m.themeLib, m.themeName)
		m.screen = screenThemes

	case choiceSettings:
		m.settings.reload(m.settingsOf())
		m.screen = screenSettings

	case choiceQuit:
		return m.quit()
	}
	return nil
}

func (m *Model) updateDaily(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	open, back := m.daily.update(msg)
	switch {
	case back:
		m.screen = screenMenu
	case open:
		return m, m.openSelectedDaily()
	}
	return m, nil
}

// openDailyScreen shows the day's puzzles, re-reading what has been played of
// them. The date is resolved once at startup, so a session left open across UTC
// midnight keeps offering the day it started on until it is restarted — the
// alternative is the board changing under a player mid-puzzle.
func (m *Model) openDailyScreen() {
	m.daily.reload(m.store, m.day)
	m.screen = screenDaily
}

// openSelectedDaily plays — or reviews — the highlighted mode's daily.
func (m *Model) openSelectedDaily() tea.Cmd {
	row, ok := m.daily.selected()
	if !ok {
		return nil
	}
	return m.openDaily(row.length)
}

// openDaily opens a day's puzzle for one mode.
//
// The order matters. What is already on disk is consulted first, so resuming or
// reviewing a daily never needs a seed at all — which is what will keep those
// working offline once seeds come from a network. Only a day with nothing saved
// asks the source, and because that may one day block, it does so as a command
// rather than inline.
func (m *Model) openDaily(length int) tea.Cmd {
	id := daily.ID(m.day, length)

	switch g, err := m.store.Load(id); {
	case err == nil:
		m.openGame(g, true)
		return nil
	case !errors.Is(err, store.ErrNotFound):
		m.err = err
		return nil
	}

	// Load reports a tombstone as ErrNotFound, so a deleted daily looks unplayed
	// here. Rebuilding it would save over that tombstone at the first guess and
	// destroy the record of how the day went, so it is refused instead.
	if spent, err := dailySpent(m.store, id); err != nil {
		m.err = err
		return nil
	} else if spent {
		m.err = errDailySpent
		return nil
	}

	// Nothing may close over m: commands run on another goroutine.
	src, day := m.dailySrc, m.day
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), dailyTimeout)
		defer cancel()

		g, err := daily.New(ctx, src, day, length)
		return dailyMsg{g: g, err: err}
	}
}

// dailySpent reports whether a daily was played and then deleted. It asks the
// store rather than the daily screen's rows, so the guard holds on any path
// that reaches a puzzle, not only the one that has just rendered the list.
func dailySpent(s store.Store, id string) (bool, error) {
	saved, err := s.All()
	if err != nil {
		return false, err
	}
	for _, g := range saved {
		if g.ID == id {
			return g.Deleted, nil
		}
	}
	return false, nil
}

func (m *Model) updateList(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	open, del, back := m.list.update(msg)
	switch {
	case back:
		m.list.confirmDelete = false
		m.screen = screenMenu
	case open:
		return m, m.openSelected()
	case del:
		return m, m.deleteSelected()
	}
	return m, nil
}

func (m *Model) updateThemes(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	commit, back := m.themes.update(msg)
	switch {
	case back:
		return m, m.back()
	case commit:
		return m, m.commitTheme()
	}
	return m, nil
}

func (m *Model) updateSettings(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	changed, back := m.settings.update(msg)
	if back {
		return m, m.back()
	}
	if changed {
		m.commitSettings(m.settings.cursor)
	}
	return m, nil
}

// commitSettings writes what the settings screen now holds. There is nothing
// to preview here, unlike the theme picker, so every change is saved as it is
// made and esc has nothing to undo.
//
// row is what was just changed: only a change to the mode moves the length this
// run is playing, so toggling the other setting cannot quietly discard a
// -length override.
func (m *Model) commitSettings(row int) {
	s := m.settingsOf()
	s.Length, s.RememberLast = m.settings.length, m.settings.rememberLast
	m.saveSettings(s)

	if row == rowLength {
		// Take effect now rather than at the next launch.
		m.length = m.settings.length
	}
}

// openSelected resumes — or reviews — the highlighted puzzle.
func (m *Model) openSelected() tea.Cmd {
	summary, ok := m.list.selected()
	if !ok {
		return nil
	}
	g, err := m.store.Load(summary.ID)
	if err != nil {
		m.err = err
		return nil
	}
	m.openGame(g, true)
	return nil
}

// deleteSelected removes the highlighted puzzle and re-reads the list. It is
// the one destructive thing in the app, so both the key path and the click path
// arm a confirmation before reaching here.
//
// Nothing else has to be reconciled: a puzzle's code comes from its own id, so
// no other puzzle is disturbed, and the profile recomputes from what is left
// the next time it is opened. What is left of a *finished* puzzle is its
// tombstone (store.Delete, game.Tombstone) — the streak walk still sees that a
// win or a loss happened at that moment, so deleting a loss cannot merge the
// win runs either side of it and raise the longest streak.
func (m *Model) deleteSelected() tea.Cmd {
	summary, ok := m.list.selected()
	if !ok {
		return nil
	}
	if err := m.store.Delete(summary.ID); err != nil {
		m.err = err
		return nil
	}
	m.list.refresh(m.store)
	return nil
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
	// Read from the active style set every frame, so switching theme in the
	// picker recolours the terminal itself and not just the panel.
	v.BackgroundColor = st.bg
	v.ForegroundColor = st.fg
	v.WindowTitle = "wortle"
	// Clicking is a first-class input here: every keybind has an on-screen
	// target. All-motion mode is what makes hover highlighting possible, since
	// it reports the pointer with no button held.
	v.MouseMode = tea.MouseModeAllMotion

	if m.quitting {
		return v
	}

	// Clickable regions are collected fresh each frame; only the hover carries
	// over from the last one.
	h := &hitMap{hover: m.hover}

	// Composition finished, so the markers left by mark() now sit at their
	// final coordinates: record them and strip them back out.
	v.Content = h.scan(m.frame(h))
	m.hits = h
	return v
}

// frame composes the whole screen. It is separate from View so a test can
// compose the same screen with no hit map and prove that marking changes not one
// cell of the result (TestMarkersDoNotAffectLayout).
func (m *Model) frame(h *hitMap) string {
	body, help := m.activeScreen(h)
	if m.err != nil {
		body = lipgloss.JoinVertical(lipgloss.Left,
			body, "", st.err.Render(fmt.Sprintf("error: %v", m.err)))
	}

	content := lipgloss.JoinVertical(lipgloss.Center, body, help)

	// Box the content in a rounded, titled panel (btop-style) and centre that
	// panel in the terminal. The border hugs the content, not the terminal
	// edges. Before the first WindowSizeMsg the dimensions are zero, so the
	// panel is emitted on its own.
	panel := renderPanel(m.screenTitle(), m.closeBox(h), content)
	if m.width > 0 && m.height > 0 {
		return lipgloss.Place(m.width, m.height,
			lipgloss.Center, lipgloss.Center, panel)
	}
	return panel
}

// closeBox is the × inlaid at the right end of the panel's top border. Quitting
// has to be reachable with the mouse from every screen, not just the menu.
func (m *Model) closeBox(h *hitMap) string {
	quit := action{kind: actQuit}
	style := st.muted
	if h.hovered(quit) {
		style = st.hover(st.err)
	}
	return " " + h.mark(quit, style.Render(st.glyph.Close)) + " "
}

// screenTitle is the label shown in the panel's top border.
func (m *Model) screenTitle() string {
	switch m.screen {
	case screenList:
		return "puzzles"
	case screenDaily:
		return "daily"
	case screenProfile:
		return "profile"
	case screenThemes:
		return "themes"
	case screenSettings:
		return "settings"
	default:
		return "wortle"
	}
}

func (m *Model) activeScreen(h *hitMap) (body, help string) {
	switch m.screen {
	case screenGame:
		return m.game.view(h), m.game.help(h)
	case screenList:
		return m.list.view(h), m.list.help(h)
	case screenDaily:
		return m.daily.view(h), m.daily.help(h)
	case screenProfile:
		return m.profile.view(h), m.profile.help(h)
	case screenThemes:
		return m.themes.view(h), m.themes.help(h)
	case screenSettings:
		return m.settings.view(h), m.settings.help(h)
	default:
		return m.menu.view(h), m.menu.help(h)
	}
}

// --- menu ---

type choiceKind int

const (
	choiceNewGame choiceKind = iota
	choiceDaily
	choiceList
	choiceProfile
	choiceThemes
	choiceSettings
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
	// Word lengths lead the menu; they are the game's difficulty modes.
	choices := make([]choice, 0, len(words.Lengths)+5)
	for _, n := range words.Lengths {
		choices = append(choices, choice{
			kind:   choiceNewGame,
			label:  fmt.Sprintf("%d letters", n),
			length: n,
		})
	}
	choices = append(choices,
		// The daily sits under the modes rather than among them: it is not a
		// fourth difficulty, it is those same modes on a shared board.
		choice{kind: choiceDaily, label: "daily"},
		choice{kind: choiceList, label: "puzzles"},
		choice{kind: choiceProfile, label: "profile"},
		choice{kind: choiceThemes, label: "themes"},
		choice{kind: choiceSettings, label: "settings"},
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

// point selects a row, for the pointer to move the cursor with.
func (m *menuScreen) point(i int) {
	if i >= 0 && i < len(m.choices) {
		m.cursor = i
	}
}

func (m *menuScreen) view(h *hitMap) string {
	heading := st.title.Render("wortle") + st.muted.Render("  wordle for the terminal")

	// Every row is centred to a common width so the list sits under the middle
	// of the heading. The selected item is flanked symmetrically so its marker
	// does not throw off the centring.
	marks := lipgloss.Width(st.glyph.Cursor) + lipgloss.Width(st.glyph.CursorRight)
	width := 0
	for _, c := range m.choices {
		if w := lipgloss.Width(c.label) + marks; w > width {
			width = w
		}
	}
	center := lipgloss.NewStyle().Width(width).Align(lipgloss.Center)

	rows := make([]string, len(m.choices))
	for i, c := range m.choices {
		var row string
		if i == m.cursor {
			row = center.Render(st.menuPick.Render(st.glyph.Cursor + c.label + st.glyph.CursorRight))
		} else {
			row = center.Render(st.muted.Render(c.label))
		}
		// The whole centred row is the click target, so it is forgiving to aim
		// at. Hovering it moves the cursor, which is highlight enough.
		rows[i] = h.mark(action{kind: actMenuChoice, index: i}, row)
	}

	list := lipgloss.JoinVertical(lipgloss.Center, rows...)
	return lipgloss.JoinVertical(lipgloss.Center, heading, "", list)
}

func (m *menuScreen) help(h *hitMap) string {
	return renderHelp(h,
		helpItem{keys: "↑/↓", label: "move"},
		helpItem{keys: "enter", label: "select", act: action{kind: actMenuChoice, index: m.cursor}},
		helpItem{keys: "q", label: "quit", act: action{kind: actQuit}},
	)
}

// compile-time check that the root model satisfies the framework contract.
var _ tea.Model = (*Model)(nil)
