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
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/nxck2005/surmise/internal/backup"
	"github.com/nxck2005/surmise/internal/banner"
	"github.com/nxck2005/surmise/internal/brand"
	"github.com/nxck2005/surmise/internal/build"
	"github.com/nxck2005/surmise/internal/daily"
	"github.com/nxck2005/surmise/internal/game"
	"github.com/nxck2005/surmise/internal/stats"
	"github.com/nxck2005/surmise/internal/store"
	"github.com/nxck2005/surmise/internal/theme"
	"github.com/nxck2005/surmise/internal/words"
)

type screen int

const (
	screenMenu screen = iota
	screenGame
	screenResult
	screenList
	screenProfile
	screenThemes
	screenSettings
	screenDaily
	screenCustom
	screenHowTo
	screenAbout
	screenBackup
	screenSplash
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

// backupFileMsg carries a file back from the platform's file picker. Empty
// bytes with no error mean the player chose nothing, which the screen reports
// as such rather than as a failure.
type backupFileMsg struct {
	b    []byte
	from string
	err  error
}

func tick() tea.Cmd {
	return tea.Tick(tickInterval, func(t time.Time) tea.Msg { return tickMsg(t) })
}

// splashDoneMsg ends a timed splash. It is its own one-shot timer rather than a
// deadline checked against the one-second tick, which would let a 1.2s splash
// sit for up to 2.2s — the whole point of the duration is that it is brief.
type splashDoneMsg struct{}

func splashTimer(d time.Duration) tea.Cmd {
	return tea.Tick(d, func(time.Time) tea.Msg { return splashDoneMsg{} })
}

// animMsg asks for a repaint part-way through an effect. It is its own message
// rather than a faster tickMsg because the two have different lifetimes: the
// one-second clock runs forever and re-arms unconditionally, this one stops the
// moment nothing is animating. Neither can starve the other — they are separate
// timers, and each re-arms only itself.
type animMsg time.Time

func animFrame() tea.Cmd {
	return tea.Tick(frameInterval, func(t time.Time) tea.Msg { return animMsg(t) })
}

// themeWatchInterval is how often the themes directory is looked at. The
// unchanged path reads one directory and opens no file, so a second buys a
// live edit-and-look loop for very little.
const themeWatchInterval = time.Second

// themesMsg carries a reloaded theme library. A nil lib means the directory was
// looked at and had not moved on — the message still arrives, because receiving
// it is what arms the next look.
type themesMsg struct{ lib *theme.Library }

// watchThemes re-reads the themes directory when it changes, so editing a theme
// file shows up without a restart. It is its own timer rather than a check
// hung off the one-second clock because the clock is a redraw, and this is disk
// work that has to be able to stop: a library with no directory — the bundled
// set, which is what the headless tests get — arms nothing at all.
//
// Nothing here closes over the model: commands run on another goroutine.
func watchThemes(lib *theme.Library) tea.Cmd {
	if lib.Dir() == "" {
		return nil
	}
	return tea.Tick(themeWatchInterval, func(time.Time) tea.Msg {
		if !lib.Changed() {
			return themesMsg{}
		}
		return themesMsg{lib: lib.Reopen()}
	})
}

// tagline is the one-line description under the product's name, on the menu and
// on the splash.
const tagline = "a word game for the terminal"

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

	// dataDir is where the files live, for the about screen to show. Nothing
	// reads or writes through it; that is the store's job.
	dataDir string

	screen   screen
	splash   splashScreen
	menu     menuScreen
	game     *gameScreen
	result   resultScreen
	list     listScreen
	profile  profileScreen
	themes   themeScreen
	settings settingsScreen
	daily    dailyScreen
	custom   customScreen
	howTo    howToScreen
	about    aboutScreen
	backup   backupScreen

	// transfer is how a backup file leaves and re-enters this build. Nil means
	// the platform cannot move files, and the menu then offers no backup row —
	// which is what the headless tests see.
	transfer Transfer

	// hits is where the last frame drew its clickable regions; hover is what the
	// pointer was last over, which the next frame highlights. Both are written
	// by View, which the framework calls on the same goroutine as Update
	// immediately afterwards, so there is no race between drawing and clicking.
	hits  *hitMap
	hover action

	// anim is what the board is animating, and how strongly. It lives on the
	// root because two screens read it — the board draws the reveal, the panel
	// draws the win accent — and because a screen change has to be able to
	// settle everything at once.
	anim anims

	// pendingResult holds the debrief back while the guess that finished the
	// puzzle is still revealing. The puzzle is already banked and saved by then;
	// this only defers which screen is showing. Any input gives up the wait.
	pendingResult bool

	width, height int
	err           error
	quitting      bool
}

// defaultLength is the word length the app opens on when nothing else says
// otherwise, matching the five letters the genre settled on.
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
	// Splash forces the startup splash for one run without saving it: "off" for
	// none, "random" for any bundled art, or a banner's name. Empty means the
	// saved choice, and it is what the headless tests pass to keep the first
	// keystroke going where they expect.
	Splash string
	// Motion forces the board's feedback animations for one run without saving
	// the choice: "off", "restrained" or "pronounced". Empty means the saved
	// choice, and it is what the headless tests pass to hold the board still.
	Motion string
	// Transfer is how the platform moves a backup file in and out: a directory
	// natively, a download and a file picker in a browser. Nil — which is what
	// the headless tests pass — means this build cannot, and the backup row is
	// then not offered at all rather than offered and broken.
	Transfer Transfer
	// DataDir is where saves, settings and themes live. It is display data —
	// the about screen shows it, and the UI's own file access still goes
	// through the store — so empty simply means "not known", which is what the
	// headless tests pass.
	DataDir string
}

// settingsStore is the part of a store that remembers preferences. It is a
// separate interface rather than part of store.Store because puzzle history
// and local presentation choices are different concerns; a store that does not
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
	m := &Model{
		store:    s,
		themeLib: lib,
		// The menu is built knowing whether this build can move files, because
		// a backup row that cannot do anything is worse than no row.
		menu:     newMenuScreen(opts.Transfer != nil),
		dailySrc: opts.DailySeeds,
		dataDir:  opts.DataDir,
		transfer: opts.Transfer,
	}
	if m.dailySrc == nil {
		m.dailySrc = daily.Local()
	}
	m.applyStartupTheme(opts.Theme)
	m.applyStartupLength(opts.Length)
	m.applyStartupDay(opts.Day)
	m.applyStartupSplash(opts.Splash)
	m.applyStartupMotion(opts.Motion)

	g, err := newPuzzle(s, m.length)
	if err != nil {
		m.err = err
		m.openMenu()
	} else {
		m.openGame(g, false)
	}
	// The splash goes up last, in front of a screen that is already live. That
	// is what makes dismissing it a screen swap and nothing else — and what
	// sends a player whose puzzle could not be built to the menu rather than to
	// a board that does not exist.
	m.raiseSplash()
	return m
}

// openGame installs a puzzle as the active screen, handing it the current
// terminal size: the board is the one screen that trims itself to fit.
func (m *Model) openGame(g *game.Game, saved bool) {
	// Nothing animates across a change of board. The reveal is keyed to a
	// puzzle id as well, which is what covers startNew swapping the game in
	// place without coming through here.
	m.anim.clear()
	m.pendingResult = false
	m.game = newGameScreen(m.store, g, saved)
	m.game.anim = &m.anim
	m.game.playtime = m.bankPlaytime
	m.game.resize(m.width, m.height)
	m.screen = screenGame
}

// openResult raises the debrief for the finished board. A save error is carried
// with it: completion still has a useful result, but must not hide that the
// durable copy failed.
func (m *Model) openResult() {
	if m.game == nil || !m.game.g.Status.Done() {
		return
	}
	notice := ""
	if m.game.message != "" && time.Now().Before(m.game.msgUntil) {
		notice = m.game.message
	}
	m.result.anim = &m.anim
	m.result.open(m.game.g, notice)
	m.screen = screenResult

	// The board hurried through a loss to get here; this is what it hurried
	// for. A win has already had its accent on the way in.
	if m.game.g.Status == game.Lost {
		m.anim.beginAnswer()
	}
}

// submitGame is the one submit path for both Enter and the on-screen keycap.
// The root owns it because a finishing guess changes the active screen.
func (m *Model) submitGame() tea.Cmd {
	if m.screen != screenGame || m.game == nil {
		return nil
	}
	if m.game.g.Status.Done() {
		m.openResult()
		return nil
	}

	cmd := m.game.submit()
	if m.game.g.Status.Done() {
		// The puzzle is already banked and saved by now — submit does both
		// before returning, and no animation is ever allowed to sit between an
		// event and its durable copy. Only which screen is showing waits, and
		// only for as long as the finishing row takes to reveal.
		if m.anim.busy(timeNow()) {
			m.pendingResult = true
			return cmd
		}
		m.openResult()
	}
	return cmd
}

// settlePendingResult raises the debrief once the finishing guess has finished
// revealing. With motion off nothing is ever busy, so submitGame opens the
// result in the same frame it always did and this is never reached.
func (m *Model) settlePendingResult() {
	if !m.pendingResult || m.anim.busy(timeNow()) {
		return
	}
	m.pendingResult = false
	m.openResult()
}

// skipToResult gives up the wait above. Any keystroke or click while the board
// is finishing settles the animation and shows the result at once, so the pause
// is never something a fast player has to sit through.
//
// The input that skips is consumed rather than acted on, the way the splash
// swallows the key that dismisses it: a stray "n" should not start a puzzle
// nobody asked for.
func (m *Model) skipToResult() bool {
	if !m.pendingResult {
		return false
	}
	m.anim.clear()
	m.pendingResult = false
	m.openResult()
	return true
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

// applyStartupSplash resolves the startup art and how it is dismissed, the same
// shape as the theme and the mode: an override first, then what was saved, then
// the default. A name that resolves to nothing is reported rather than
// swallowed, and never fatal — art that stopped shipping between releases must
// cost a note on the error line, not a launch.
//
// "No splash" is carried as no art rather than as a flag, so there is one thing
// to check: raiseSplash puts up whatever art there is, and an empty banner
// simply never fits.
func (m *Model) applyStartupSplash(override string) {
	s := m.settingsOf()

	mode, ok := parseSplashMode(s.SplashDismiss)
	if !ok {
		m.err = fmt.Errorf("no splash setting %q — using %s", s.SplashDismiss, mode.setting())
	}
	m.splash.mode = mode

	d, ok := parseSplashDuration(s.SplashMillis)
	if !ok {
		m.err = fmt.Errorf("splash length %dms is out of range — using %s",
			s.SplashMillis, splashDurationLabel(splashDuration))
	}
	m.splash.duration = d

	want := override
	if want == splashOn {
		// -splash on turns it back on for a run without saying which art, so it
		// falls through to the saved choice.
		want = ""
	}
	if want == "" {
		if s.Splash == splashOff {
			return
		}
		want = s.SplashArt
	}

	switch want {
	case splashOff:
		return
	case "":
		m.splash.art = banner.Default()
	case splashRandom:
		// Rolled once here, not once per frame: the splash must not flicker
		// between banners as it redraws.
		m.splash.art = banner.Random(nil)
	default:
		art, ok := banner.Get(want)
		if !ok {
			art = banner.Default()
			m.err = fmt.Errorf("no splash art named %q — using %s", want, art.Name)
		}
		m.splash.art = art
	}
}

// applyStartupMotion resolves how much the board animates: an override first,
// then what was saved, then the environment, then restrained. The same shape as
// the theme, the mode and the splash — and, like them, an unreadable value is
// reported on the error line rather than refused.
//
// The environment is consulted only when nobody has chosen: a player who has
// been to the settings screen has said what they want, and a $NO_MOTION left in
// a shell profile must not overrule them.
func (m *Model) applyStartupMotion(override string) {
	if override != "" {
		want, ok := parseMotion(override)
		if !ok {
			m.err = fmt.Errorf("no motion setting %q — using %s", override, want.setting())
		}
		m.anim.motion = want
		return
	}

	saved := m.settingsOf().Motion
	if saved != "" {
		want, ok := parseMotion(saved)
		if !ok {
			m.err = fmt.Errorf("no motion setting %q — using %s", saved, want.setting())
		}
		m.anim.motion = want
		return
	}

	if prefersReducedMotion() {
		m.anim.motion = motionOff
		return
	}
	m.anim.motion = motionPronounced
}

// raiseSplash puts the splash in front of whatever screen is already live,
// remembering that screen as where dismissing goes. It does nothing when there
// is no art, or when the terminal is too small to draw it.
func (m *Model) raiseSplash() {
	m.splash.resize(m.width, m.height)
	if !m.splash.fits() {
		return
	}
	m.splash.next = m.screen
	m.splash.anim = &m.anim
	m.screen = screenSplash
	// The sweep starts with the screen. Nothing waits on it: the splash is
	// dismissible from the first frame, and a dismissal simply leaves the effect
	// to expire behind whatever came next.
	m.anim.beginSplash()
}

// dismissSplash reveals the screen underneath. Both the key path and the click
// path call it, and so does the timer, which is why it checks the screen: a
// timer that fires after a manual skip must do nothing rather than send an
// already-playing person somewhere.
func (m *Model) dismissSplash() tea.Cmd {
	if m.screen != screenSplash {
		return nil
	}
	m.screen = m.splash.next
	return nil
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

// bankPlaytime adds a played session to the lifetime counter. It is the board's
// playtime hook, and the only thing that ever increases the figure.
//
// Read-modify-write like every other settings write, so a session banked while
// another preference is being changed cannot lose either one. A store that keeps
// no settings simply counts nothing.
func (m *Model) bankPlaytime(d time.Duration) {
	if d <= 0 {
		return
	}
	s := m.settingsOf()
	s.PlaytimeMS += d.Milliseconds()
	m.saveSettings(s)
}

// playtime is the lifetime total as it should be displayed: the counter, floored
// by what the saved puzzles can still prove. The floor is what an install whose
// history predates the counter shows, and it is written back the first time it
// wins, which is the whole of the migration.
func (m *Model) playtime() time.Duration {
	saved := time.Duration(m.settingsOf().PlaytimeMS) * time.Millisecond
	games, err := m.store.All()
	if err != nil {
		return saved
	}
	total := stats.Playtime(saved, games)
	if total > saved {
		s := m.settingsOf()
		s.PlaytimeMS = total.Milliseconds()
		m.saveSettings(s)
	}
	return total
}

func (m *Model) Init() tea.Cmd {
	// Batch drops the nils, so neither the splash timer nor the theme watch
	// needs a condition here: each decides for itself whether it exists.
	//
	// animCmd is here as well as in Update's wrapper because the splash sweep is
	// the one effect that starts before any message arrives: without it the art
	// would sit still until the first tick a second later.
	return tea.Batch(tick(), watchThemes(m.themeLib), m.splashCmd(), m.animCmd())
}

// splashCmd is the timer a timed splash runs on, or nil when there is nothing
// to time — no splash showing, or a mode that waits for input instead. It is a
// method rather than an inline condition in Init so a test can ask whether the
// timer was armed without waiting out its duration.
func (m *Model) splashCmd() tea.Cmd {
	if m.screen != screenSplash || !m.splash.mode.timed() {
		return nil
	}
	return splashTimer(m.splash.duration)
}

// pushSize hands the terminal's size to the screens that lay out against it.
// They shed or scroll rather than let the panel outgrow the terminal, which
// would take the top of the frame — title, close box and all — off the screen.
func (m *Model) pushSize() {
	if m.game != nil {
		m.game.resize(m.width, m.height)
	}
	m.profile.resize(m.width, m.height)
	m.howTo.resize(m.width, m.height)
	m.about.resize(m.width, m.height)
	m.list.resize(m.height)
	m.themes.resize(m.height)

	// The splash is measured too late to be checked at startup — there is no
	// size until the first WindowSizeMsg — so this is where art too big for the
	// terminal gives up its turn rather than overflowing the frame.
	m.splash.resize(m.width, m.height)
	if m.screen == screenSplash && !m.splash.fits() {
		m.dismissSplash()
	}
}

// Update is a thin wrapper so the animation chain is armed in exactly one
// place. Any handler below may start an effect, and animCmd is idempotent, so
// no branch can forget to arm one and none can arm a second.
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	model, cmd := m.update(msg)
	// Batch drops nils, the same property Init relies on.
	return model, tea.Batch(cmd, m.animCmd())
}

// animCmd arms the next repaint, or nothing when nothing is animating. This is
// what makes the loop self-cancelling: the chain lives exactly as long as the
// effects do, and an idle board holds no timer at all.
func (m *Model) animCmd() tea.Cmd {
	if m.anim.live || !m.anim.busy(timeNow()) {
		return nil
	}
	m.anim.live = true
	return animFrame()
}

func (m *Model) update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.pushSize()
		return m, nil

	case tickMsg:
		// Redraw so the clock advances and expired messages disappear.
		return m, tick()

	case animMsg:
		// The chain is re-armed by Update's wrapper, not here: clearing the flag
		// is all this branch owes it. A frame that arrives after everything has
		// settled — because a keystroke ended an effect, or the screen changed,
		// or motion was turned off — therefore re-arms nothing, the same way a
		// splashDoneMsg for a splash that is no longer timed does nothing.
		m.anim.live = false
		m.settlePendingResult()
		return m, nil

	case themesMsg:
		if msg.lib != nil {
			m.applyThemes(msg.lib)
		}
		// Re-armed from the model's library rather than the message's, so the
		// watch follows the swap instead of stamping the superseded one.
		return m, watchThemes(m.themeLib)

	case splashDoneMsg:
		// The mode is checked as well as the screen: only a timed splash arms a
		// timer, so one arriving for a splash that waits for input belongs to a
		// mode that has since changed, and must not cut it short.
		if m.splash.mode.timed() {
			return m, m.dismissSplash()
		}
		return m, nil

	case dailyMsg:
		if msg.err != nil {
			m.err = msg.err // stay where we are; nothing has been created
			return m, nil
		}
		// Transient until the first guess, like every other new puzzle: opening
		// the daily and walking away saves nothing.
		m.openGame(msg.g, false)
		return m, nil

	case backupFileMsg:
		// The screen may have been left while the picker was open. Applying it
		// anyway would write a history nobody is looking at into the store and
		// report it to a screen that is not showing, so a file that arrives
		// late is dropped: the player asked from a screen they have since left.
		if m.screen != screenBackup {
			return m, nil
		}
		m.applyBackup(msg)
		return m, nil

	case tea.ColorProfileMsg:
		// What the terminal can actually show, reported once at startup. The
		// gradients are the only thing that reads it, and they fall back to a
		// flat palette colour when there is not the depth for them.
		setColorProfile(msg.Profile)
		return m, nil

	case tea.KeyPressMsg:
		return m.handleKey(msg)

	case tea.MouseClickMsg:
		// Only the left button acts; a click on nothing is a no-op. Release is
		// deliberately ignored, so a target fires the moment it is pressed.
		if msg.Button != tea.MouseLeft {
			return m, nil
		}
		// Same rule as a keystroke: a click while the board is finishing spends
		// itself on the skip rather than on whatever it landed on.
		if m.skipToResult() {
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
	case actBackupSave:
		m.backup.point(backupRowSave)
	case actBackupLoad:
		m.backup.point(backupRowLoad)
	case actThemeRow:
		// Hovering a theme previews it, the same as arrowing onto it.
		m.themes.point(a.index)
	case actFieldEdit, actFieldDone:
		m.pointField(a.index)
	case actSettingNext, actSettingPrev:
		m.settings.point(a.index)
	case actCustomNext, actCustomPrev:
		m.custom.point(a.index)
	}
}

// activeField is the text field the active screen is editing, or nil. It is
// what lets esc and the help bar treat text input the same way on every screen
// that has any, rather than each one being special-cased at the root.
func (m *Model) activeField() *textField {
	switch m.screen {
	case screenSettings:
		if m.settings.name.editing {
			return &m.settings.name
		}
	case screenCustom:
		if m.custom.secret.editing {
			return &m.custom.secret
		}
	}
	return nil
}

// fieldAt is the text field on the active screen's given row, or nil. Clicks
// carry the row, and the screen is what says which field sits there — so the
// four field actions serve every screen without one kind each.
func (m *Model) fieldAt(row int) *textField {
	switch m.screen {
	case screenSettings:
		if row == rowProfileName {
			return &m.settings.name
		}
	case screenCustom:
		if row == customRowSecret {
			return &m.custom.secret
		}
	}
	return nil
}

// pointField moves the active screen's cursor to a field's row, so that
// clicking a field selects it exactly as arrowing onto it would.
func (m *Model) pointField(row int) {
	switch m.screen {
	case screenSettings:
		m.settings.point(row)
	case screenCustom:
		m.custom.point(row)
	}
}

// fieldCommitted is what a screen does when one of its fields has actually
// changed. A setting is written; a value that lives only for the next screen is
// not.
func (m *Model) fieldCommitted(row int) {
	switch m.screen {
	case screenSettings:
		m.commitSettings(row)
	}
}

// dispatch performs a clicked action. Every branch routes into the same methods
// the key handlers call, so clicking and typing cannot drift apart.
func (m *Model) dispatch(a action) tea.Cmd {
	switch a.kind {
	case actQuit:
		return m.quit()

	case actBack:
		// esc belongs to the editor while one is open: it abandons the draft
		// rather than the screen.
		if f := m.activeField(); f != nil {
			f.finish(false)
			return nil
		}
		return m.back()

	case actBackupSave, actBackupLoad:
		if m.screen != screenBackup {
			return nil
		}
		row := backupRowSave
		if a.kind == actBackupLoad {
			row = backupRowLoad
		}
		// Clicking a row also selects it, so the help bar's "enter" keeps
		// talking about the thing that was last acted on.
		m.backup.point(row)
		return m.doBackup(row)

	case actSplashDismiss:
		// The same method the key path calls; dismissSplash itself checks that
		// the splash is what is showing.
		return m.dismissSplash()

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

	case actDailyCopy:
		// The same method the c key calls; copyTrio itself checks there is a
		// finished trio to copy.
		return m.copyTrio()

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

	case actJumpTop, actJumpBottom:
		// The ends of a scrolling list, on whichever list is showing. Both go
		// through the same methods home and end do.
		top := a.kind == actJumpTop
		switch m.screen {
		case screenList:
			if top {
				m.list.jumpTop()
			} else {
				m.list.jumpBottom()
			}
		case screenThemes:
			if top {
				m.themes.jumpTop()
			} else {
				m.themes.jumpBottom()
			}
			// Moving the cursor previews, exactly as arrowing onto a row does.
			m.themes.preview()
		}
		return nil

	case actHowToPage:
		if m.screen != screenHowTo {
			return nil
		}
		// A dot and the help bar's arrows both carry the page they turn to, so
		// they land in the same method the keys step through.
		m.howTo.show(a.index)
		return nil

	case actThemeRow:
		if m.screen != screenThemes {
			return nil
		}
		m.themes.point(a.index)
		return m.commitTheme()

	case actThemeReload:
		if m.screen != screenThemes {
			return nil
		}
		// The same method the r key calls, so the two cannot drift.
		return m.reloadThemes()

	// The four field actions route through fieldAt, so a screen gets a working
	// text editor by naming its field rather than by growing four action kinds
	// and four arms of its own.
	case actFieldEdit:
		if f := m.fieldAt(a.index); f != nil {
			m.pointField(a.index)
			f.begin()
		}
		return nil

	case actFieldDone:
		if f := m.fieldAt(a.index); f != nil && f.finish(true) {
			m.fieldCommitted(a.index)
		}
		return nil

	case actFieldCancel:
		if f := m.fieldAt(a.index); f != nil {
			f.finish(false)
		}
		return nil

	case actFieldBackspace:
		if f := m.fieldAt(a.index); f != nil {
			f.deleteRune()
		}
		return nil

	case actCustomNext, actCustomPrev:
		if m.screen != screenCustom || m.custom.secret.editing {
			return nil
		}
		m.custom.point(a.index)
		// The same cycle method the arrow keys call.
		if a.kind == actCustomNext {
			m.custom.cycle(1)
		} else {
			m.custom.cycle(-1)
		}
		return nil

	case actCustomStart:
		if m.screen != screenCustom {
			return nil
		}
		return m.startCustom()

	case actSettingNext, actSettingPrev:
		if m.screen != screenSettings {
			return nil
		}
		if m.settings.name.editing {
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
	case actResultReview:
		return m.reviewResult()

	case actResultNext:
		return m.nextResult()

	case actResultCopy:
		return m.copyResult()
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
		return m.submitGame()
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
	// Whatever was animating belongs to the screen being left.
	m.anim.clear()
	m.pendingResult = false

	switch {
	case m.screen == screenResult && m.game != nil:
		if err := m.game.leave(); err != nil {
			m.result.notice = fmt.Sprintf("could not save: %v", err)
			return nil
		}
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
	m.openMenu()
	return nil
}

// openMenu raises the menu, deriving the status line first. What it says is
// read off the saves, so it has to be recomputed every time the menu comes up
// — a puzzle finished, a mode of the day played, a puzzle deleted since the
// last visit would all otherwise be missed.
func (m *Model) openMenu() {
	m.menu.reload(m.store, m.day)
	m.screen = screenMenu
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

	m.openMenu()
	return nil
}

// applyThemes takes a re-read theme library. It is the single place a reload
// lands, whether the watch noticed the change or the player asked for one.
func (m *Model) applyThemes(lib *theme.Library) {
	m.themeLib = lib

	if m.screen == screenThemes {
		// Keep the player's place in the list, then re-apply what the cursor is
		// on: editing the highlighted theme is the whole point of the loop, and
		// the preview is what shows the edit.
		m.themes.refresh(lib)
		m.themes.preview()
		return
	}

	// Anywhere else the committed theme is what is on screen, so re-apply that.
	m.restoreTheme()
}

// reloadThemes re-reads the directory on demand. Synchronous, the way startup
// reads it: the player asked, so the answer belongs in this frame.
func (m *Model) reloadThemes() tea.Cmd {
	m.applyThemes(m.themeLib.Reopen())
	return nil
}

// restoreTheme puts back the committed theme after an abandoned preview, and is
// also the fallback when a reload takes the committed theme away — Resolve
// already means "this name, else Tokyo Night, else the built-in default", so a
// theme file deleted or made unreadable under us lands somewhere valid rather
// than leaving whatever was previewed on screen.
//
// m.themeName is deliberately left alone: the settings file still names that
// theme, so putting the file back brings it back.
func (m *Model) restoreTheme() {
	t, _ := m.themeLib.Resolve(m.themeName)
	setTheme(t)
}

func (m *Model) handleKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	// ctrl+c always exits, and must not lose an in-progress puzzle.
	if msg.String() == "ctrl+c" {
		return m, m.quit()
	}

	// A board still finishing its last guess gives way to the first key pressed,
	// which is spent on the skip itself.
	if m.skipToResult() {
		return m, nil
	}

	switch m.screen {
	case screenSplash:
		// Any key at all, which is why this arm reads no key: the splash offers
		// exactly one thing to do. In the fixed mode it offers nothing, and the
		// keystroke is swallowed rather than reaching the board behind it.
		if m.splash.mode.dismissible() {
			return m, m.dismissSplash()
		}
		return m, nil
	case screenMenu:
		return m.updateMenu(msg)
	case screenGame:
		// Enter may finish the puzzle and raise another screen, so the root's
		// shared keyboard/mouse submit path owns it. An armed tab prompt keeps
		// Enter inside gameScreen, where it confirms the replacement.
		if msg.String() == "enter" && !m.game.confirmNew {
			return m, m.submitGame()
		}
		cmd, back := m.game.update(msg)
		if back {
			m.openMenu()
		}
		return m, cmd
	case screenResult:
		return m.updateResult(msg)
	case screenList:
		return m.updateList(msg)
	case screenDaily:
		return m.updateDaily(msg)
	case screenThemes:
		return m.updateThemes(msg)
	case screenSettings:
		return m.updateSettings(msg)
	case screenCustom:
		return m.updateCustom(msg)
	case screenHowTo:
		if m.howTo.update(msg) {
			m.openMenu()
		}
		return m, nil
	case screenBackup:
		return m.updateBackup(msg)
	// Both are read-only screens with no cursor, so their whole key handling is
	// "get me out of here".
	case screenProfile, screenAbout:
		if key := msg.String(); key == "esc" || key == "q" {
			m.openMenu()
		}
		return m, nil
	}
	return m, nil
}

func (m *Model) updateResult(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter", "r":
		return m, m.reviewResult()
	case "n":
		return m, m.nextResult()
	case "c":
		return m, m.copyResult()
	case "esc", "q":
		return m, m.back()
	default:
		return m, nil
	}
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

	case choiceCustom:
		// Opened fresh every time: a word left over from the last hand-over is
		// the one thing this screen must never show.
		m.custom = newCustomScreen(m.length)
		m.screen = screenCustom

	case choiceList:
		m.list.reload(m.store)
		m.screen = screenList

	case choiceProfile:
		s := m.settingsOf()
		m.profile.reload(m.store, m.day, s.DisplayName, m.playtime())
		m.screen = screenProfile

	case choiceThemes:
		m.themes.reload(m.themeLib, m.themeName)
		m.screen = screenThemes

	case choiceSettings:
		m.settings.reload(m.settingsOf())
		m.screen = screenSettings

	case choiceHowTo:
		// Opened at the front every time: the pages are a sequence, and picking
		// the screen up where it was last left would start a first-time reader
		// halfway through it.
		m.howTo.reset()
		m.screen = screenHowTo

	case choiceBackup:
		// Opened fresh every time: a report of what an import did an hour ago
		// says nothing about what is on the machine now — the same reason the
		// how-to screen opens on its first page.
		m.backup.reset()
		m.screen = screenBackup

	case choiceAbout:
		m.about.reload(m.dataDir)
		m.screen = screenAbout

	case choiceQuit:
		return m.quit()
	}
	return nil
}

func (m *Model) updateBackup(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if key := msg.String(); key == "esc" || key == "q" {
		return m, m.back()
	}
	row, act := m.backup.update(msg)
	if !act {
		return m, nil
	}
	return m, m.doBackup(row)
}

// doBackup runs one of the screen's two actions. Both the key path and the
// click path land here, so a row and its button cannot drift apart.
func (m *Model) doBackup(row int) tea.Cmd {
	switch row {
	case backupRowSave:
		m.saveBackup()
		return nil
	case backupRowLoad:
		return m.loadBackup()
	}
	return nil
}

// saveBackup writes the whole install through the platform's Transfer.
//
// The themes come from the library's own directory rather than from a path this
// package worked out; an empty directory — the browser, which has none — yields
// nothing to carry, which is exactly right.
func (m *Model) saveBackup() {
	if m.transfer == nil {
		return
	}
	themes, err := theme.Files(m.themeLib.Dir())
	if err != nil {
		// One unreadable theme directory is not worth losing the puzzles over;
		// the archive is still worth writing without it.
		themes = nil
	}
	b, err := backup.Build(m.store, m.settingsOf(), themes, build.Get().String(), time.Now())
	if err != nil {
		m.backup.refused(err)
		return
	}
	where, err := m.transfer.Save(b)
	if err != nil {
		m.backup.refused(err)
		return
	}
	m.backup.saved(where)
}

// loadBackup asks the platform for a file. Transfer.Load may block for as long
// as a file picker is open, which is why this is a command and not a call: the
// app keeps drawing, and the screen says what it is waiting for.
func (m *Model) loadBackup() tea.Cmd {
	if m.transfer == nil {
		return nil
	}
	m.backup.waiting = true
	transfer := m.transfer
	return func() tea.Msg {
		b, from, err := transfer.Load()
		return backupFileMsg{b: b, from: from, err: err}
	}
}

// applyBackup merges a file the platform handed back.
//
// Everything about the merge itself belongs to internal/backup; what is here is
// only what the UI owns — writing the preferences back, putting the themes on
// disk, and applying a theme the archive filled in so the player sees it now
// rather than after a restart.
func (m *Model) applyBackup(msg backupFileMsg) {
	switch {
	case msg.err != nil:
		m.backup.refused(msg.err)
		return
	case len(msg.b) == 0:
		// A picker closed without choosing. Not a failure.
		m.backup.cancelled()
		return
	}

	res, err := backup.Apply(msg.b, m.store, m.settingsOf())
	if err != nil {
		m.backup.refused(err)
		return
	}

	if len(res.SettingsFilled) > 0 || res.PlaytimeAdded > 0 {
		m.saveSettings(res.Settings)
	}
	// A theme the archive named is applied through the picker's own path, so
	// the restore looks the way the player left it rather than the way this
	// install happened to be set up.
	if res.Settings.Theme != "" && res.Settings.Theme != m.themeName {
		m.themeName = res.Settings.Theme
		m.restoreTheme()
	}

	// Themes are written here rather than by Apply, which does no file I/O so
	// that the browser — with no theme directory at all — can use every other
	// part of it. An empty directory means there is nowhere to put them.
	added := 0
	if dir := m.themeLib.Dir(); dir != "" && len(res.Themes) > 0 {
		n, _, err := theme.WriteNew(dir, res.Themes)
		added = n
		if err != nil {
			// A refused theme name must not make a restore that moved a whole
			// history look like it failed, so it goes on the error line while
			// the report below still reports what landed.
			m.err = err
		}
	}
	m.backup.loaded(res, added)
}

func (m *Model) updateDaily(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	// Copying belongs to the day rather than to the highlighted mode, so it is
	// answered here rather than in the screen's own key handling — the same
	// reason the board's enter is answered by the root.
	if msg.String() == "c" {
		return m, m.copyTrio()
	}
	open, back := m.daily.update(msg)
	switch {
	case back:
		m.openMenu()
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

// copyTrio puts the day's three boards on the clipboard, once all three are
// finished. It is the one guard both the key and the button pass through, so
// neither can offer a copy of a day that is not done.
func (m *Model) copyTrio() tea.Cmd {
	if m.screen != screenDaily || !m.daily.trio().complete() {
		return nil
	}
	m.daily.copyRequested = true
	return tea.SetClipboard(shareTrio(m.daily.day.String(), m.daily.rows))
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
		m.openMenu()
	case open:
		return m, m.openSelected()
	case del:
		return m, m.deleteSelected()
	}
	return m, nil
}

func (m *Model) updateThemes(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	commit, back, reload := m.themes.update(msg)
	switch {
	case back:
		return m, m.back()
	case commit:
		return m, m.commitTheme()
	case reload:
		return m, m.reloadThemes()
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

func (m *Model) updateCustom(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	start, back := m.custom.update(msg)
	if back {
		// Leaving forgets the word: it must not be waiting here if the screen is
		// opened again by somebody else.
		m.custom.clear()
		return m, m.back()
	}
	if start {
		return m, m.startCustom()
	}
	return m, nil
}

// startCustom hands the terminal over: it turns the typed word into a board
// and shows it, having first forgotten the word.
//
// The order matters. The screen is cleared before openGame swaps to the board,
// so no later frame — a resize, a stale hover, coming back for a second round —
// can redraw what was typed. And because a new puzzle is transient until its
// first guess, a hand-over nobody plays leaves nothing on disk at all.
func (m *Model) startCustom() tea.Cmd {
	if msg := m.custom.check(); msg != "" {
		m.custom.msg = msg
		return nil
	}

	g, err := game.NewCustom(m.custom.secret.value, m.custom.length)
	if err != nil {
		m.err = err
		return nil
	}
	m.custom.clear()
	m.openGame(g, false)
	return nil
}

// commitSettings writes what the settings screen now holds. Cycling rows save
// immediately; the profile-name editor calls this only when its draft is kept,
// so escape can discard text without making settings generally transactional.
//
// row is what was just changed: only a change to the mode moves the length this
// run is playing, so toggling the other setting cannot quietly discard a
// -length override.
func (m *Model) commitSettings(row int) {
	s := m.settingsOf()
	s.Length, s.RememberLast = m.settings.length, m.settings.rememberLast
	s.DisplayName = m.settings.name.value
	// Written out rather than left empty for the defaults: a preference someone
	// has actually visited should read back the same way it looks on screen.
	s.Splash = splashOff
	if m.settings.splash {
		s.Splash = splashOn
	}
	s.SplashArt = m.settings.splashArt
	s.SplashDismiss = m.settings.splashMode.setting()
	s.SplashMillis = int(m.settings.splashTime / time.Millisecond)
	s.Motion = m.settings.motion.setting()
	m.saveSettings(s)

	if row == rowLength {
		// Take effect now rather than at the next launch.
		m.length = m.settings.length
	}
	if row == rowMotion {
		// Likewise immediate — and turning motion off settles whatever was
		// running, so the choice is visible in the frame that follows it rather
		// than after the current effect finishes.
		m.anim.motion = m.settings.motion
		if m.anim.motion == motionOff {
			m.anim.clear()
		}
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
	if (m.screen == screenGame || m.screen == screenResult) && m.game != nil {
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
	v.WindowTitle = brand.Name
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
	// A frame taller than the terminal is not shown from the top, so the
	// coordinates just recorded are not the ones clicks arrive in. See clip.
	// An unmeasured height counts as unbounded, as it does everywhere else.
	if m.height > 0 {
		h.clip(lipgloss.Height(v.Content)-m.height, m.height)
	}
	m.hits = h
	return v
}

// How the win accent rises and falls: the number of colours it is mixed from,
// and the fractions of the moment spent arriving and leaving. The rest is spent
// at full accent, which is where the effect reads.
const (
	winSteps   = 24
	winRampIn  = 0.25
	winRampOut = 0.35
)

// frame composes the whole screen. It is separate from View so a test can
// compose the same screen with no hit map and prove that marking changes not one
// cell of the result (TestMarkersDoNotAffectLayout).
func (m *Model) frame(h *hitMap) string {
	body, help := m.activeScreen(h)
	if m.err != nil {
		// Centred, and the body squared off first: joined left, an error wider
		// than the screen it is reporting on dragged that screen to the left.
		body = lipgloss.JoinVertical(lipgloss.Center,
			block(body), "", st.err.Render(fmt.Sprintf("error: %v", m.err)))
	}

	content := lipgloss.JoinVertical(lipgloss.Center, body, help)

	// Box the content in a rounded, titled panel (btop-style) and centre that
	// panel in the terminal. The border hugs the content, not the terminal
	// edges. Before the first WindowSizeMsg the dimensions are zero, so the
	// panel is emitted on its own.
	// A solved board accents the whole frame for a moment: the same runes at the
	// same width, in the colour the theme already uses for emphasis. The accent
	// rises and falls across that moment rather than switching on and off, so
	// the frame answers a win the way the tiles do — by turning, not by
	// blinking — and it lands back on the border colour it started from, which
	// is the settled frame. On a terminal that cannot blend, the run is flat in
	// the accent and this is the hard swap it always was.
	border := st.border
	if p, winning := m.anim.winning(timeNow()); winning {
		lit := blend(winSteps, st.border.GetForeground(), st.accent.GetForeground())
		strength := min(p/winRampIn, (1-p)/winRampOut, 1)
		border = st.border.Foreground(colorAt(lit, int(max(strength, 0)*float64(winSteps-1))))
	}
	panel := renderPanel(m.screenTitle(), m.screenStatus(), m.closeBox(h), content, border)
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
	case screenResult:
		return "result"
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
	case screenCustom:
		return "custom"
	case screenHowTo:
		return "how to play"
	case screenAbout:
		return "about"
	default:
		return brand.Name
	}
}

// screenStatus is what the panel's top rule carries at its right end: the one
// fact about the screen that is worth reading without looking away from what
// you are doing. It is a label, never a control, and the panel drops it rather
// than crowd the rule on a narrow terminal.
//
// Nothing here may repeat what the screen already says in its own body — the
// board's mode and score live here *instead* of in its header.
func (m *Model) screenStatus() string {
	switch m.screen {
	case screenGame:
		if m.game == nil {
			return ""
		}
		g := m.game.g
		what := fmt.Sprintf("%d letters", g.Length)
		switch {
		case g.Daily != "":
			what = fmt.Sprintf("daily %s · %s", g.Daily, what)
		case g.Custom:
			what = fmt.Sprintf("custom · %s", what)
		}
		return fmt.Sprintf("%s · %d/%d", what, g.Attempts(), g.MaxAttempts)

	case screenList:
		if n := len(m.list.items); n > 0 {
			return fmt.Sprintf("%d saved", n)
		}

	case screenDaily:
		// The day's progress, which is the question this screen exists to
		// answer; the trio line under the rows carries the rest of it.
		if t := m.daily.trio(); t.done > 0 {
			return fmt.Sprintf("%d/%d done", t.done, t.of)
		}
	}
	return ""
}

func (m *Model) activeScreen(h *hitMap) (body, help string) {
	switch m.screen {
	case screenSplash:
		return m.splash.view(h), m.splash.help(h)
	case screenGame:
		return m.game.view(h), m.game.help(h)
	case screenResult:
		return m.result.view(h), m.result.help(h)
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
	case screenCustom:
		return m.custom.view(h), m.custom.help(h)
	case screenHowTo:
		return m.howTo.view(h), m.howTo.help(h)
	case screenAbout:
		return m.about.view(h), m.about.help(h)
	case screenBackup:
		return m.backup.view(h), m.backup.help(h)
	default:
		return m.menu.view(h), m.menu.help(h)
	}
}

// --- menu ---

type choiceKind int

const (
	choiceNewGame choiceKind = iota
	choiceDaily
	choiceCustom
	choiceList
	choiceProfile
	choiceThemes
	choiceSettings
	choiceBackup
	choiceHowTo
	choiceAbout
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

	// What the status line under the title is made of: how many of the day's
	// modes are finished, and the player's current win streak. Derived by
	// reload from the saved games every time the menu is raised — never
	// stored, never carried over from the last visit.
	modesDone int
	modesOf   int
	streak    int
}

// reload derives the status line from what is on disk. One pass over the saves
// feeds both halves, the same bargain the daily screen's trio makes; a read
// error leaves it empty rather than half-said.
func (m *menuScreen) reload(s store.Store, day daily.Day) {
	m.modesDone, m.modesOf, m.streak = 0, len(words.Lengths), 0

	games, err := s.All()
	if err != nil {
		return
	}
	byID := make(map[string]*game.Game, len(games))
	for _, g := range games {
		byID[g.ID] = g
	}
	for _, n := range words.Lengths {
		if g, ok := byID[daily.ID(day, n)]; ok && !g.Deleted && g.Status.Done() {
			m.modesDone++
		}
	}
	m.streak = stats.ComputeAt(games, day).CurrentStreak
}

// status is the line itself, or "" when there is nothing to say: no mode of
// the day finished and no streak running renders the menu exactly as it was
// before the line existed.
func (m *menuScreen) status() string {
	var parts []string
	if m.modesDone > 0 {
		parts = append(parts, fmt.Sprintf("daily %d/%d", m.modesDone, m.modesOf))
	}
	if m.streak > 0 {
		parts = append(parts, fmt.Sprintf("streak %d", m.streak))
	}
	return strings.Join(parts, " · ")
}

// newMenuScreen builds the menu. transfers says whether this build can move a
// backup file; a build that cannot is offered no backup row at all.
func newMenuScreen(transfers bool) menuScreen {
	// Word lengths lead the menu; they are the game's difficulty modes.
	choices := make([]choice, 0, len(words.Lengths)+10)
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
		// Under the daily for the same reason: another way to get a board,
		// rather than another difficulty.
		choice{kind: choiceCustom, label: "custom"},
		choice{kind: choiceList, label: "puzzles"},
		choice{kind: choiceProfile, label: "profile"},
		choice{kind: choiceThemes, label: "themes"},
		choice{kind: choiceSettings, label: "settings"},
	)
	// Under settings, above the reference screens: it acts on the whole install
	// rather than on a puzzle, which is what the rows around it do.
	if transfers {
		choices = append(choices, choice{kind: choiceBackup, label: "backup"})
	}
	choices = append(choices,
		// The two reference screens sit at the foot, together: one explains the
		// game, the other explains the build.
		choice{kind: choiceHowTo, label: "how to play"},
		choice{kind: choiceAbout, label: "about"},
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
	// The tagline sits under the title rather than beside it. On one line it was
	// the widest thing on the screen, and centring the list against it put the
	// choices well to the right of the word they belong under.
	//
	// Once there is something to say, the day's progress says it instead: a
	// returning player reads their status where a new one reads the tagline.
	// The two swap places rather than stack, so the menu never grows a row —
	// it has no height budget, and a short terminal loses its top.
	under := tagline
	if s := m.status(); s != "" {
		under = s
	}
	heading := lipgloss.JoinVertical(lipgloss.Center,
		st.title.Render(brand.Name),
		st.muted.Render(under),
	)

	// Labels are centred inside a column as wide as the longest, with the
	// selection markers held in fixed-width gutters either side. The gutters are
	// what this screen got wrong before: it centred marker-plus-label as one
	// unit, so selecting a row shifted its label sideways. Kept out of the
	// centring, the markers appear beside a label that has not moved.
	labelWidth := 0
	for _, c := range m.choices {
		if w := lipgloss.Width(c.label); w > labelWidth {
			labelWidth = w
		}
	}
	// Padding goes on before styling: padding an already-styled string counts
	// its escape codes as characters. An odd gap cannot be halved, so the odd
	// column goes right, as lipgloss's own Align(Center) does — consistently, so
	// the rows lean the same way rather than alternating.
	pad := func(label string) string {
		gap := labelWidth - lipgloss.Width(label)
		return strings.Repeat(" ", gap/2) + label + strings.Repeat(" ", gap-gap/2)
	}
	blank := strings.Repeat(" ", lipgloss.Width(st.glyph.Cursor))
	trail := strings.Repeat(" ", lipgloss.Width(st.glyph.CursorRight))

	rows := make([]string, len(m.choices))
	for i, c := range m.choices {
		var row string
		if i == m.cursor {
			row = st.cursor.Render(st.glyph.Cursor) +
				st.menuPick.Render(pad(c.label)) +
				st.cursor.Render(st.glyph.CursorRight)
		} else {
			// Two weights, not one: the ways to get a board read brighter than
			// the ways to get somewhere else. It is the cheapest hierarchy there
			// is — no rule, no blank row, nothing that costs a line on a
			// terminal that has none to spare.
			row = blank + m.weight(c).Render(pad(c.label)) + trail
		}
		// Every row is the same width now, so the whole row is the click target
		// and aiming at it is forgiving. Hovering it moves the cursor, which is
		// highlight enough.
		rows[i] = h.mark(action{kind: actMenuChoice, index: i}, row)
	}

	// block, not JoinVertical, holds the list together: the rows are already the
	// same width, and squaring them off is what makes the outer join slide the
	// whole list under the heading rather than re-centring each row.
	list := block(strings.Join(rows, "\n"))
	return lipgloss.JoinVertical(lipgloss.Center, heading, "", list)
}

// weight is how an unselected row is drawn. Playing is what the menu is for, so
// the modes, the daily and a custom board carry the text colour; everything
// below them is navigation and stays muted.
func (m *menuScreen) weight(c choice) lipgloss.Style {
	switch c.kind {
	case choiceNewGame, choiceDaily, choiceCustom:
		return st.text
	default:
		return st.muted
	}
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
