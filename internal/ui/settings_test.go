package ui

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/nxck2005/surmise/internal/banner"
	"github.com/nxck2005/surmise/internal/store"
)

// openSettings walks the menu to the settings screen the way a player would,
// rather than assigning the screen directly.
func openSettings(t *testing.T, m *Model) {
	t.Helper()
	for i, c := range m.menu.choices {
		if c.kind == choiceSettings {
			m.menu.cursor = i
			send(t, m, "enter")
			if m.screen != screenSettings {
				t.Fatal("menu entry did not open settings")
			}
			return
		}
	}
	t.Fatal("no settings entry on the menu")
}

// newStore is a store over a fresh directory, plus the directory, for the tests
// that reopen it as a second process would.
func newStore(t *testing.T) (*store.JSON, string) {
	t.Helper()
	dir := t.TempDir()
	s, err := store.NewJSON(dir)
	if err != nil {
		t.Fatalf("NewJSON: %v", err)
	}
	return s, dir
}

// reopen is a second model over the same directory: what the next launch sees.
func reopen(t *testing.T, dir string, opts Options) *Model {
	t.Helper()
	s, err := store.NewJSON(dir)
	if err != nil {
		t.Fatalf("NewJSON: %v", err)
	}
	return New(s, nil, opts)
}

// --- resolving the starting mode ---

func TestSavedLengthOpensThatMode(t *testing.T) {
	s, dir := newStore(t)
	if err := s.SaveSettings(store.Settings{Length: 6}); err != nil {
		t.Fatal(err)
	}

	m := reopen(t, dir, Options{})
	if m.game == nil || m.game.g.Length != 6 {
		t.Fatalf("opened on a %d-letter puzzle, want 6", m.game.g.Length)
	}
	if m.err != nil {
		t.Errorf("unexpected error: %v", m.err)
	}
}

// -length wins over the saved choice, and must not overwrite it: an override
// is for one run, exactly like -theme.
func TestLengthOverrideDoesNotPersist(t *testing.T) {
	s, dir := newStore(t)
	if err := s.SaveSettings(store.Settings{Length: 6}); err != nil {
		t.Fatal(err)
	}

	m := reopen(t, dir, Options{Length: 4})
	if m.game.g.Length != 4 {
		t.Fatalf("opened on a %d-letter puzzle, want 4", m.game.g.Length)
	}
	if got := s.Settings().Length; got != 6 {
		t.Errorf("saved length = %d, want the override not to have written 6 away", got)
	}
}

// A length the game has no words for is reported and fallen back from, never
// fatal — the same treatment an unknown theme name gets.
func TestUnsupportedLengthFallsBackAndReports(t *testing.T) {
	_, dir := newStore(t)

	m := reopen(t, dir, Options{Length: 9})
	if m.game == nil || m.game.g.Length != defaultLength {
		t.Fatalf("opened on a %d-letter puzzle, want %d", m.game.g.Length, defaultLength)
	}
	if m.err == nil {
		t.Fatal("no error reported for an unsupported length")
	}
	m.Update(tea.WindowSizeMsg{Width: testWidth, Height: testHeight})
	if !strings.Contains(m.View().Content, "no 9-letter mode") {
		t.Error("the error line does not mention the bad length")
	}
}

// --- the screen ---

func TestSettingsScreenPersistsChoices(t *testing.T) {
	s, dir := newStore(t)
	m := New(s, nil, Options{})
	m.screen = screenMenu

	openSettings(t, m)
	// Two steps forward from 5 wraps to 4, which also proves the wrap.
	send(t, m, "right", "right")
	send(t, m, "down", "right")

	// The splash's three are written out as well: every save is the whole file,
	// and a visited preference reads back as what the screen shows.
	want := store.Settings{
		Length: 4, RememberLast: true,
		Splash: splashOn, SplashArt: banner.Default().Name, SplashDismiss: splashSkip.setting(),
		SplashMillis: int(splashDuration / time.Millisecond),
	}
	if got := s.Settings(); got != want {
		t.Fatalf("saved settings = %+v, want %+v", got, want)
	}

	// The next launch opens on it.
	if m2 := reopen(t, dir, Options{}); m2.game.g.Length != 4 {
		t.Errorf("reopened on a %d-letter puzzle, want 4", m2.game.g.Length)
	}
}

// Changing the mode takes effect without a relaunch: the next puzzle started
// from the menu uses it.
func TestSettingsScreenChangesTheNextPuzzle(t *testing.T) {
	s, _ := newStore(t)
	m := New(s, nil, Options{})
	m.screen = screenMenu

	openSettings(t, m)
	send(t, m, "right") // 5 → 6
	send(t, m, "esc")

	if m.length != 6 {
		t.Fatalf("default length = %d, want 6", m.length)
	}
}

// Toggling the other setting must not discard a -length override: only a
// change to the mode itself moves what this run is playing.
func TestTogglingRememberKeepsTheOverride(t *testing.T) {
	s, dir := newStore(t)
	if err := s.SaveSettings(store.Settings{Length: 5}); err != nil {
		t.Fatal(err)
	}

	m := reopen(t, dir, Options{Length: 4})
	m.screen = screenMenu
	openSettings(t, m)
	send(t, m, "down", "right") // remember last: off → on

	if m.length != 4 {
		t.Errorf("length = %d, want the -length override of 4 to survive", m.length)
	}
	if got := s.Settings(); !got.RememberLast || got.Length != 5 {
		t.Errorf("saved settings = %+v, want the override kept out of the file", got)
	}
}

// esc is not a cancel here — there is nothing being previewed, so a change is
// already saved by the time you leave.
func TestSettingsHaveNoCancel(t *testing.T) {
	s, _ := newStore(t)
	m := New(s, nil, Options{})
	m.screen = screenMenu

	openSettings(t, m)
	send(t, m, "right", "esc")

	if m.screen != screenMenu {
		t.Fatalf("screen = %v after esc, want the menu", m.screen)
	}
	if got := s.Settings().Length; got != 6 {
		t.Errorf("saved length = %d, want 6 kept after esc", got)
	}
}

// --- remember-last ---

func TestRememberLastFollowsThePlayedMode(t *testing.T) {
	s, dir := newStore(t)
	if err := s.SaveSettings(store.Settings{Length: 5, RememberLast: true}); err != nil {
		t.Fatal(err)
	}

	m := reopen(t, dir, Options{})
	m.screen = screenMenu
	chooseMode(t, m, 6)

	if got := s.Settings().Length; got != 6 {
		t.Fatalf("saved length = %d, want playing 6 to have set it", got)
	}
	if m2 := reopen(t, dir, Options{}); m2.game.g.Length != 6 {
		t.Errorf("reopened on a %d-letter puzzle, want 6", m2.game.g.Length)
	}
}

// With it off, playing another mode is a one-off: the default is only what the
// settings screen says.
func TestPlayedModeIsNotRememberedWhenOff(t *testing.T) {
	s, dir := newStore(t)
	if err := s.SaveSettings(store.Settings{Length: 5}); err != nil {
		t.Fatal(err)
	}

	m := reopen(t, dir, Options{})
	m.screen = screenMenu
	chooseMode(t, m, 6)

	if got := s.Settings().Length; got != 5 {
		t.Fatalf("saved length = %d, want it left at 5", got)
	}
}

// Remembering a mode must not cost the theme, since both live in one file.
func TestRememberLastKeepsTheSavedTheme(t *testing.T) {
	s, dir := newStore(t)
	if err := s.SaveSettings(store.Settings{Theme: "dracula", RememberLast: true}); err != nil {
		t.Fatal(err)
	}

	m := reopen(t, dir, Options{})
	m.screen = screenMenu
	chooseMode(t, m, 4)

	want := store.Settings{Theme: "dracula", Length: 4, RememberLast: true}
	if got := s.Settings(); got != want {
		t.Errorf("saved settings = %+v, want %+v", got, want)
	}
}

// chooseMode picks a word-length entry off the menu.
func chooseMode(t *testing.T, m *Model, length int) {
	t.Helper()
	for i, c := range m.menu.choices {
		if c.kind == choiceNewGame && c.length == length {
			m.menu.cursor = i
			send(t, m, "enter")
			if m.game == nil || m.game.g.Length != length {
				t.Fatalf("menu did not start a %d-letter puzzle", length)
			}
			return
		}
	}
	t.Fatalf("no %d-letter entry on the menu", length)
}
