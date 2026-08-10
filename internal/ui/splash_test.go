package ui

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/nxck2005/surmise/internal/banner"
	"github.com/nxck2005/surmise/internal/store"
)

// mustStore is a store over a fresh directory, for the tests that do not also
// need the directory back.
func mustStore(t *testing.T) *store.JSON {
	t.Helper()
	s, err := store.NewJSON(t.TempDir())
	if err != nil {
		t.Fatalf("NewJSON: %v", err)
	}
	return s
}

// splashModel is a model sitting on the splash, as a launch leaves it.
func splashModel(t *testing.T, opts Options) *Model {
	t.Helper()
	m := New(mustStore(t), nil, opts)
	if m.screen != screenSplash {
		t.Fatalf("screen = %v, want splash", m.screen)
	}
	return m
}

func TestSplashDrawsItsArt(t *testing.T) {
	m := splashModel(t, Options{})
	frame := draw(t, m)

	art := banner.Default()
	// The widest line is the one that would be clipped or wrapped if the art did
	// not fit, so it is the one worth looking for.
	widest := ""
	for _, line := range art.Lines {
		if len(line) > len(widest) {
			widest = line
		}
	}
	if !strings.Contains(sgr.ReplaceAllString(frame, ""), widest) {
		t.Errorf("the art is not in the frame\n%s", frame)
	}
	if !strings.Contains(sgr.ReplaceAllString(frame, ""), tagline) {
		t.Error("the tagline is not under the art")
	}
}

// A timed splash ends on its timer, and a timer that arrives after it has gone
// must not act on the screen underneath.
func TestTimedSplashDismisses(t *testing.T) {
	m := splashModel(t, Options{})
	m.splash.mode = splashSkip

	m.Update(splashDoneMsg{})
	if m.screen != screenGame {
		t.Fatalf("screen after the timer = %v, want game", m.screen)
	}

	// A second one arrives after a manual skip in real use; here it stands for
	// any late timer. It must change nothing.
	send(t, m, "esc") // to the menu, so a wrongly handled timer is visible
	m.Update(splashDoneMsg{})
	if m.screen != screenMenu {
		t.Errorf("a late timer moved the screen to %v", m.screen)
	}
}

func TestDefaultSplashWaitsForAnyKey(t *testing.T) {
	m := splashModel(t, Options{})
	if m.splash.mode != splashKey {
		t.Fatalf("default mode = %v, want any key", m.splash.mode)
	}
	if m.splashCmd() != nil {
		t.Fatal("the default splash armed a timer")
	}

	m.Update(splashDoneMsg{})
	if m.screen != screenSplash {
		t.Fatalf("timer dismissed the default splash to %v", m.screen)
	}
	send(t, m, "a")
	if m.screen != screenGame {
		t.Fatalf("key dismissed the default splash to %v, want game", m.screen)
	}
}

// The three dismissal modes differ only in what ends the splash, so they are
// worth one table.
func TestSplashDismissModes(t *testing.T) {
	for _, tc := range []struct {
		name          string
		mode          splashMode
		key, timer    bool // does this end it?
		clickable     bool
		wantAfterKey  screen
		wantAfterTick screen
	}{
		{name: "skip", mode: splashSkip, key: true, timer: true, clickable: true},
		{name: "key", mode: splashKey, key: true, timer: false, clickable: true},
		{name: "fixed", mode: splashFixed, key: false, timer: true, clickable: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// A key first.
			m := splashModel(t, Options{})
			m.splash.mode = tc.mode
			send(t, m, "a")
			if got := m.screen == screenGame; got != tc.key {
				t.Errorf("dismissed by a key = %v, want %v", got, tc.key)
			}

			// Then the timer, on a fresh splash.
			m = splashModel(t, Options{})
			m.splash.mode = tc.mode
			m.Update(splashDoneMsg{})
			if got := m.screen == screenGame; got != tc.timer {
				t.Errorf("dismissed by the timer = %v, want %v", got, tc.timer)
			}

			// And whether there is anything to click at all.
			m = splashModel(t, Options{})
			m.splash.mode = tc.mode
			draw(t, m)
			if _, ok := m.hits.find(action{kind: actSplashDismiss}); ok != tc.clickable {
				t.Errorf("a click target on screen = %v, want %v", ok, tc.clickable)
			}
		})
	}
}

// Anything the keys can do a click can do too: the art itself is the button.
func TestSplashDismissesByClicking(t *testing.T) {
	m := splashModel(t, Options{})
	click(t, m, action{kind: actSplashDismiss})
	if m.screen != screenGame {
		t.Fatalf("screen after clicking the art = %v, want game", m.screen)
	}
}

// A splash the terminal cannot hold is skipped rather than clipped: an
// overflowing frame loses its top rows, which would take the panel's title and
// close box with it.
func TestSplashYieldsToASmallTerminal(t *testing.T) {
	m := splashModel(t, Options{})
	m.Update(tea.WindowSizeMsg{Width: 24, Height: 10})
	if m.screen != screenGame {
		t.Fatalf("screen on a small terminal = %v, want game", m.screen)
	}

	// And a terminal big enough keeps it.
	m = splashModel(t, Options{})
	m.Update(tea.WindowSizeMsg{Width: testWidth, Height: testHeight})
	if m.screen != screenSplash {
		t.Errorf("screen on a %dx%d terminal = %v, want splash", testWidth, testHeight, m.screen)
	}
}

// The overrides: off, a name, random, and a name that is not there.
func TestSplashOverride(t *testing.T) {
	s, err := store.NewJSON(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	if m := New(s, nil, Options{Splash: splashOff}); m.screen != screenGame {
		t.Errorf("-splash off still showed screen %v", m.screen)
	}

	name := banner.Default().Name
	m := New(s, nil, Options{Splash: name})
	if m.splash.art.Name != name {
		t.Errorf("-splash %s drew %q", name, m.splash.art.Name)
	}

	if m := New(s, nil, Options{Splash: splashRandom}); m.splash.art.Empty() {
		t.Error("-splash random drew nothing")
	}

	// Art that no longer ships costs a note, not a launch.
	m = New(s, nil, Options{Splash: "no such art"})
	if m.err == nil {
		t.Error("an unknown banner was not reported")
	}
	if m.splash.art.Name != banner.Default().Name {
		t.Errorf("fell back to %q, want the default art", m.splash.art.Name)
	}
}

// The splash is switched off by the setting as well as by the flag, and the
// saved dismissal is honoured at the next launch.
func TestSavedSplashPreferences(t *testing.T) {
	s, dir := newStore(t)
	if err := s.SaveSettings(store.Settings{Splash: splashOff}); err != nil {
		t.Fatal(err)
	}
	if m := reopen(t, dir, Options{}); m.screen != screenGame {
		t.Errorf("a saved off setting still showed screen %v", m.screen)
	}

	if err := s.SaveSettings(store.Settings{SplashDismiss: splashKey.setting()}); err != nil {
		t.Fatal(err)
	}
	m := reopen(t, dir, Options{})
	if m.splash.mode != splashKey {
		t.Errorf("mode = %v, want the saved one", m.splash.mode)
	}

	// A dismissal that does not parse is reported and falls back, the same as an
	// unsupported -length.
	if err := s.SaveSettings(store.Settings{SplashDismiss: "sideways"}); err != nil {
		t.Fatal(err)
	}
	m = reopen(t, dir, Options{})
	if m.err == nil {
		t.Error("an unknown dismissal was not reported")
	}
	if m.splash.mode != splashKey {
		t.Errorf("mode = %v, want the default", m.splash.mode)
	}
}

// --- the settings rows ---

func TestSplashSettingsPersist(t *testing.T) {
	s, dir := newStore(t)
	m := New(s, nil, Options{Splash: splashOff})
	m.screen = screenMenu
	openSettings(t, m)

	// Down to the art row, then one step: past the last banner is "random".
	m.settings.cursor = rowSplashArt
	send(t, m, "right")
	// And on to the dismissal.
	send(t, m, "down", "right")

	got := s.Settings()
	if got.SplashArt == banner.Default().Name {
		t.Errorf("the art row did not step: %q", got.SplashArt)
	}
	if got.SplashDismiss == splashKey.setting() {
		t.Errorf("the dismissal row did not step: %q", got.SplashDismiss)
	}

	// The next launch reads them back.
	m2 := reopen(t, dir, Options{})
	if m2.splash.mode.setting() != got.SplashDismiss {
		t.Errorf("reopened with mode %q, want %q", m2.splash.mode.setting(), got.SplashDismiss)
	}
}

// With the splash off, its two dependent rows are dead: the cursor passes over
// them, they offer nothing to click, and cycling them does nothing.
func TestSplashRowsAreDisabledWhenItIsOff(t *testing.T) {
	m := newModel(t)
	openSettings(t, m)

	// Off, from the row that owns it.
	m.settings.cursor = rowSplash
	send(t, m, "right")
	if m.settings.splash {
		t.Fatal("the splash row did not turn off")
	}

	send(t, m, "down")
	if m.settings.cursor != rowSplash {
		t.Errorf("the cursor moved onto a disabled row (%d)", m.settings.cursor)
	}

	before := m.settings.splashArt
	m.settings.cursor = rowSplashArt // as a stale click would leave it
	send(t, m, "right")
	if m.settings.splashArt != before {
		t.Errorf("a disabled row still cycled: %q → %q", before, m.settings.splashArt)
	}

	// Put the cursor back where the real flow leaves it. The help bar carries
	// the current row's own step actions, so a cursor parked on a disabled row
	// would answer for it — which is exactly why nothing can park it there.
	m.settings.cursor = rowSplash
	draw(t, m)
	for _, a := range []action{
		{kind: actSettingNext, index: rowSplashArt},
		{kind: actSettingPrev, index: rowSplashArt},
		{kind: actSettingNext, index: rowSplashDismiss},
	} {
		if _, ok := m.hits.find(a); ok {
			t.Errorf("a disabled row still offers %+v to click", a)
		}
	}

	// Back on, and the rows come alive again.
	m.settings.cursor = rowSplash
	send(t, m, "right")
	draw(t, m)
	if _, ok := m.hits.find(action{kind: actSettingNext, index: rowSplashArt}); !ok {
		t.Error("the art row did not come back")
	}
}

// --- how long it stays up ---

// The saved length is what a timed splash runs on, and it is only armed for a
// mode that has something to time.
func TestSplashDuration(t *testing.T) {
	s, dir := newStore(t)
	if err := s.SaveSettings(store.Settings{
		SplashDismiss: splashSkip.setting(),
		SplashMillis:  2500,
	}); err != nil {
		t.Fatal(err)
	}

	m := reopen(t, dir, Options{})
	if m.splash.duration != 2500*time.Millisecond {
		t.Errorf("duration = %v, want the saved 2.5s", m.splash.duration)
	}
	if m.splashCmd() == nil {
		t.Error("a timed splash armed no timer")
	}

	// Nothing saved means the built-in default, not no wait at all.
	m = reopen(t, dir, Options{})
	m.splash.duration = 0
	if d, ok := parseSplashDuration(0); !ok || d != splashDuration {
		t.Errorf("parseSplashDuration(0) = %v, %v, want the default", d, ok)
	}

	// The mode that waits for a key has nothing to time.
	if err := s.SaveSettings(store.Settings{SplashDismiss: splashKey.setting()}); err != nil {
		t.Fatal(err)
	}
	if m := reopen(t, dir, Options{}); m.splashCmd() != nil {
		t.Error("the key mode armed a timer")
	}

	// And no splash at all arms nothing either.
	if m := New(mustStore(t), nil, Options{Splash: splashOff}); m.splashCmd() != nil {
		t.Error("a switched-off splash armed a timer")
	}
}

// A hand-edited length outside the sane range is reported and falls back,
// rather than hanging the launch behind a splash nobody can dismiss.
func TestSplashDurationOutOfRange(t *testing.T) {
	for _, ms := range []int{-1, int(2 * time.Minute / time.Millisecond)} {
		s, dir := newStore(t)
		if err := s.SaveSettings(store.Settings{SplashMillis: ms}); err != nil {
			t.Fatal(err)
		}
		m := reopen(t, dir, Options{})
		if m.err == nil {
			t.Errorf("%dms was not reported", ms)
		}
		if m.splash.duration != splashDuration {
			t.Errorf("%dms gave duration %v, want the default", ms, m.splash.duration)
		}
	}
}

func TestSplashDurationLabels(t *testing.T) {
	for _, tc := range []struct {
		d    time.Duration
		want string
	}{
		{600 * time.Millisecond, "0.6s"},
		{splashDuration, "1.2s"},
		{2 * time.Second, "2s"},
	} {
		if got := splashDurationLabel(tc.d); got != tc.want {
			t.Errorf("label(%v) = %q, want %q", tc.d, got, tc.want)
		}
	}
}

// Stepping wraps, and a value from a hand-edited file starts from the nearest
// offered one rather than from the beginning of the list.
func TestStepSplashDuration(t *testing.T) {
	last := splashDurations[len(splashDurations)-1]
	if got := stepSplashDuration(last, 1); got != splashDurations[0] {
		t.Errorf("stepping past the end gave %v, want %v", got, splashDurations[0])
	}
	if got := stepSplashDuration(splashDurations[0], -1); got != last {
		t.Errorf("stepping back from the start gave %v, want %v", got, last)
	}
	// 1900ms is nearest 2s, so one step forward is the value after it.
	want := splashDurations[wrap(indexOfDuration(2*time.Second), 1, len(splashDurations))]
	if got := stepSplashDuration(1900*time.Millisecond, 1); got != want {
		t.Errorf("stepping from an unlisted value gave %v, want %v", got, want)
	}
}

func indexOfDuration(d time.Duration) int {
	for i, x := range splashDurations {
		if x == d {
			return i
		}
	}
	return 0
}

// The length row is dead while the dismissal has nothing to time.
func TestSplashTimeRowFollowsTheDismissMode(t *testing.T) {
	m := newModel(t)
	openSettings(t, m)

	m.settings.splashMode = splashSkip
	m.settings.cursor = rowSplashDismiss
	if !m.settings.enabled(rowSplashTime) {
		t.Fatal("the length row starts disabled under a timed dismissal")
	}

	// Step to the mode that waits for a key.
	for range len(splashModes) {
		if m.settings.splashMode == splashKey {
			break
		}
		send(t, m, "right")
	}
	if m.settings.splashMode != splashKey {
		t.Fatal("could not reach the key mode")
	}
	if m.settings.enabled(rowSplashTime) {
		t.Error("the length row is still live with nothing to time")
	}

	send(t, m, "down")
	if m.settings.cursor != rowSplashDismiss {
		t.Errorf("the cursor moved onto the disabled length row (%d)", m.settings.cursor)
	}
	draw(t, m)
	if _, ok := m.hits.find(action{kind: actSettingNext, index: rowSplashTime}); ok {
		t.Error("the disabled length row still offers a click target")
	}
}

// Art that no longer ships must not show as a name with nothing behind it.
func TestSettingsShowArtThatExists(t *testing.T) {
	var scr settingsScreen
	scr.reload(store.Settings{SplashArt: "deleted-art"})
	if scr.splashArt != banner.Default().Name {
		t.Errorf("splashArt = %q, want the default", scr.splashArt)
	}

	scr.reload(store.Settings{SplashArt: splashRandom})
	if scr.splashArt != splashRandom {
		t.Errorf("splashArt = %q, want random to survive", scr.splashArt)
	}
}
