package ui

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/nxck2005/surmise/internal/store"
	"github.com/nxck2005/surmise/internal/theme"
)

// withTheme applies a theme for the duration of a test. The active style set is
// package state — one program, one look — so a test that changes it has to put
// it back or it colours everything that runs after.
func withTheme(t *testing.T, th *theme.Theme) {
	t.Helper()
	previous := st
	setTheme(th)
	t.Cleanup(func() { st = previous })
}

func themed(t *testing.T, body string) *theme.Theme {
	t.Helper()
	th, warns := theme.Parse("test", []byte(body))
	if len(warns) > 0 {
		t.Fatalf("test theme has warnings: %v", warns)
	}
	return th
}

// The point of a theme is that it changes what you see.
func TestThemeChangesRenderedColours(t *testing.T) {
	m := newModel(t)
	m.Update(tea.WindowSizeMsg{Width: testWidth, Height: testHeight})
	send(t, m, "enter") // onto a board, where the tiles are

	before := m.View().Content

	withTheme(t, themed(t, `
bg = "#010203"
accent = "#0f0f0f"
correct = "#112233"
`))
	after := m.View().Content

	if before == after {
		t.Fatal("switching theme changed nothing on screen")
	}
	if sgr.ReplaceAllString(before, "") != sgr.ReplaceAllString(after, "") {
		t.Error("a colour-only theme moved the layout")
	}
}

// Terminal background and foreground are OSC-level, set on the tea.View rather
// than inline, so they need their own check that a theme reaches them.
func TestThemeReachesTheTerminalColours(t *testing.T) {
	m := newModel(t)
	withTheme(t, themed(t, `bg = "#010203"`))

	r, g, b, _ := m.View().BackgroundColor.RGBA()
	if [3]uint32{r >> 8, g >> 8, b >> 8} != [3]uint32{1, 2, 3} {
		t.Errorf("view background = %v, want the theme's", [3]uint32{r >> 8, g >> 8, b >> 8})
	}
}

// Every bundled theme must leave the layout alone. Hit regions are recorded by
// measuring the composed frame, so a theme that shifted a cell would move every
// click target with it — this is the guard that lets themes be data.
func TestBundledThemesDoNotMoveTheLayout(t *testing.T) {
	m := newModel(t)
	m.Update(tea.WindowSizeMsg{Width: testWidth, Height: testHeight})
	send(t, m, "enter")
	m.game.g.Answer = "crane"
	send(t, m, "a", "b", "o", "u", "t", "enter", "c", "r")
	// want is captured once and compared against a frame per theme; without
	// this the header's timer ticks partway through and every theme after the
	// tick "moves the layout".
	freezeClock(m)

	withTheme(t, theme.Default())
	want := sgr.ReplaceAllString(m.frame(nil), "")

	for _, e := range theme.Bundled().Entries() {
		// The themes that deliberately change glyphs or metrics are exempt:
		// they are opting into a different shape, which is allowed.
		if e.Theme.Glyphs != theme.Default().Glyphs || e.Theme.Metrics != theme.Default().Metrics {
			continue
		}
		t.Run(e.Name, func(t *testing.T) {
			withTheme(t, e.Theme)
			if got := sgr.ReplaceAllString(m.frame(nil), ""); got != want {
				t.Errorf("%s moved the layout", e.Name)
			}
		})
	}
}

// A theme may change the shape as well as the colours — and when it does, the
// click targets must follow, because they are measured rather than predicted.
func TestClickTargetsFollowThemeMetrics(t *testing.T) {
	m := gameModel(t)
	withTheme(t, themed(t, "[metrics]\ntile_width = 11\n"))

	frame := draw(t, m)
	r, ok := m.hits.find(action{kind: actLetter, letter: 'q'})
	if !ok {
		t.Fatal("no Q keycap on screen")
	}
	if got := strings.TrimSpace(at(t, frame, r)); got != "Q" {
		t.Errorf("the Q target covers %q", got)
	}

	// And clicking it still types, with the wider board in play.
	click(t, m, action{kind: actLetter, letter: 'q'})
	if m.game.typing != "q" {
		t.Errorf("typing = %q, want q", m.game.typing)
	}
}

// --- the picker ---

func TestThemePickerPersistsChoice(t *testing.T) {
	dir := t.TempDir()
	s, err := store.NewJSON(dir)
	if err != nil {
		t.Fatal(err)
	}
	withTheme(t, theme.Default())

	m := New(s, theme.Bundled(), Options{})
	m.screen = screenMenu
	m.Update(tea.WindowSizeMsg{Width: testWidth, Height: testHeight})

	openThemes(t, m)
	send(t, m, "down", "enter")

	if m.screen != screenThemes && m.themeName == theme.DefaultName {
		t.Fatal("choosing a theme did nothing")
	}
	if got := s.Settings().Theme; got != m.themeName {
		t.Errorf("saved theme = %q, want %q", got, m.themeName)
	}

	// A fresh model over the same directory comes back with that theme.
	reopened, err := store.NewJSON(dir)
	if err != nil {
		t.Fatal(err)
	}
	if m2 := New(reopened, theme.Bundled(), Options{}); m2.themeName != m.themeName {
		t.Errorf("reopened with %q, want %q", m2.themeName, m.themeName)
	}
}

// Backing out of the picker undoes the preview: looking at a theme is not
// choosing it.
func TestThemePickerRevertsOnEscape(t *testing.T) {
	m := newModel(t)
	withTheme(t, theme.Default())
	m.Update(tea.WindowSizeMsg{Width: testWidth, Height: testHeight})

	openThemes(t, m)
	before := m.themeName
	send(t, m, "down", "down")

	if st.theme.Name == before {
		t.Fatal("moving the cursor did not preview anything")
	}
	send(t, m, "esc")

	if m.themeName != before {
		t.Errorf("committed theme = %q, want %q", m.themeName, before)
	}
	if st.theme.Name != before {
		t.Errorf("escape left %q applied, want %q restored", st.theme.Name, before)
	}
}

// Mouse parity: hovering previews and clicking commits, through the same
// methods the keys use.
func TestThemePickerClickMatchesKeys(t *testing.T) {
	m := newModel(t)
	withTheme(t, theme.Default())
	m.Update(tea.WindowSizeMsg{Width: testWidth, Height: testHeight})
	openThemes(t, m)

	row := action{kind: actThemeRow, index: 2}
	want := m.themes.entries[2].Name

	point(t, m, row)
	if st.theme.Name != want {
		t.Errorf("hover previewed %q, want %q", st.theme.Name, want)
	}

	click(t, m, row)
	if m.themeName != want {
		t.Errorf("click committed %q, want %q", m.themeName, want)
	}
	if m.screen != screenMenu {
		t.Error("committing did not return to the menu")
	}
}

// A theme that failed to parse must not be selectable, and must say why.
func TestBrokenThemeIsShownButNotApplied(t *testing.T) {
	m := newModel(t)
	withTheme(t, theme.Default())
	m.themes.entries = []theme.Entry{{
		Name:     "broken",
		Source:   "/tmp/broken.toml",
		Warnings: []theme.Warning{{Line: 2, Msg: "bad hex colour"}},
	}}
	m.screen = screenThemes

	frame := draw(t, m)
	for _, want := range []string{"broken", "/tmp/broken.toml", "bad hex colour"} {
		if !strings.Contains(sgr.ReplaceAllString(frame, ""), want) {
			t.Errorf("the picker does not mention %q", want)
		}
	}

	before := st.theme.Name
	send(t, m, "enter")
	if st.theme.Name != before {
		t.Error("a theme that failed to load was applied anyway")
	}
}

// A theme named on the command line that does not exist is a typo worth
// showing, not a reason to refuse to start.
func TestUnknownThemeNameFallsBackVisibly(t *testing.T) {
	s, err := store.NewJSON(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	withTheme(t, theme.Default())

	m := New(s, theme.Bundled(), Options{Theme: "no such theme"})
	m.Update(tea.WindowSizeMsg{Width: testWidth, Height: testHeight})

	if m.themeName != theme.DefaultName {
		t.Errorf("themeName = %q, want the default", m.themeName)
	}
	if !strings.Contains(sgr.ReplaceAllString(m.View().Content, ""), "no theme named") {
		t.Error("the unknown theme was swallowed silently")
	}
}

// --- hot reload ---

// hotModel is a model over a real themes directory, which is what the reload
// path needs and what newModel deliberately does not have: the bundled library
// has no directory, so nothing there ever reloads.
func hotModel(t *testing.T, dir string) *Model {
	t.Helper()
	s, err := store.NewJSON(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	withTheme(t, theme.Default())

	m := New(s, theme.Open(dir), Options{})
	m.screen = screenMenu
	m.Update(tea.WindowSizeMsg{Width: testWidth, Height: testHeight})
	return m
}

// writeTheme puts a theme file in place and returns its name.
func writeTheme(t *testing.T, dir, file, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, file), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// reload drives the message the watch delivers. send drops the commands it gets
// back, so the watch cannot be exercised through it; this is the same message
// the timer would produce.
func reload(t *testing.T, m *Model) {
	t.Helper()
	m.Update(themesMsg{lib: m.themeLib.Reopen()})
}

// A theme file added above the highlighted one shifts every index below it. The
// cursor has to follow the name, or an edit made while browsing slides the
// selection out from under the player.
func TestThemeReloadKeepsTheCursorWhereItWas(t *testing.T) {
	dir := t.TempDir()
	writeTheme(t, dir, "zzz.toml", "name = \"zzz mine\"\naccent = \"#ff00ff\"\n")
	m := hotModel(t, dir)

	openThemes(t, m)
	send(t, m, "end") // "zzz mine" sorts last
	want := m.themes.entries[m.themes.cursor].Name
	if want != "zzz mine" {
		t.Fatalf("cursor started on %q, want the user theme", want)
	}

	// A name sorting first, so every index moves.
	writeTheme(t, dir, "aaa.toml", "name = \"aaa mine\"\naccent = \"#00ff00\"\n")
	reload(t, m)

	if got := m.themes.entries[m.themes.cursor].Name; got != want {
		t.Errorf("cursor moved to %q, want it left on %q", got, want)
	}
	if _, ok := m.themeLib.Get("aaa mine"); !ok {
		t.Error("the new theme was not picked up")
	}
	if st.theme.Name != want {
		t.Errorf("the preview became %q, want %q", st.theme.Name, want)
	}
}

// The point of reloading outside the picker: editing the theme you are playing
// under recolours the board where you are.
func TestEditedThemeAppliesOutsideThePicker(t *testing.T) {
	dir := t.TempDir()
	writeTheme(t, dir, "mine.toml", "name = \"mine\"\naccent = \"#ff00ff\"\n")
	m := hotModel(t, dir)

	openThemes(t, m)
	for i, e := range m.themes.entries {
		if e.Name == "mine" {
			m.themes.cursor = i
		}
	}
	m.commitTheme()
	if m.screen != screenMenu {
		t.Fatal("committing did not leave the picker")
	}
	before := st.accent.GetForeground()

	writeTheme(t, dir, "mine.toml", "name = \"mine\"\naccent = \"#00ff00\"\n")
	reload(t, m)

	if st.theme.Name != "mine" {
		t.Fatalf("active theme = %q, want it still mine", st.theme.Name)
	}
	if st.accent.GetForeground() == before {
		t.Error("the edit did not reach the styles")
	}
}

// A theme that is deleted, or that stops being readable, leaves nothing to
// apply. Falling back to the default is what keeps the screen legible, and what
// stops esc putting a theme back that no longer exists.
func TestVanishedActiveThemeFallsBackToDefault(t *testing.T) {
	dir := t.TempDir()
	writeTheme(t, dir, "mine.toml", "name = \"mine\"\naccent = \"#ff00ff\"\n")
	m := hotModel(t, dir)

	openThemes(t, m)
	for i, e := range m.themes.entries {
		if e.Name == "mine" {
			m.themes.cursor = i
		}
	}
	m.commitTheme()

	if err := os.Remove(filepath.Join(dir, "mine.toml")); err != nil {
		t.Fatal(err)
	}
	reload(t, m)

	if st.theme.Name != theme.DefaultName {
		t.Errorf("fell back to %q, want %q", st.theme.Name, theme.DefaultName)
	}
	// The name is kept: the settings file still says "mine", so putting the file
	// back brings the theme back.
	if m.themeName != "mine" {
		t.Errorf("themeName = %q, want it left alone", m.themeName)
	}

	// And backing out of the picker must not resurrect it either.
	openThemes(t, m)
	send(t, m, "esc")
	if st.theme.Name != theme.DefaultName {
		t.Errorf("escape restored %q, want the default", st.theme.Name)
	}
}

// The other way a theme stops being applicable: the file is still there but
// cannot be read, so the entry is listed with its error and a nil Theme. A
// warning is not this — a bad line still loads over the default, by design —
// so only the nil case falls back.
func TestUnreadableActiveThemeFallsBackToDefault(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("file modes do not deny reads on Windows")
	}
	if os.Geteuid() == 0 {
		t.Skip("root reads a mode-000 file anyway")
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "mine.toml")
	writeTheme(t, dir, "mine.toml", "name = \"mine\"\naccent = \"#ff00ff\"\n")
	m := hotModel(t, dir)

	openThemes(t, m)
	for i, e := range m.themes.entries {
		if e.Name == "mine" {
			m.themes.cursor = i
		}
	}
	m.commitTheme()

	if err := os.Chmod(path, 0o000); err != nil {
		t.Fatal(err)
	}
	reload(t, m)

	if st.theme.Name != theme.DefaultName {
		t.Errorf("fell back to %q, want %q", st.theme.Name, theme.DefaultName)
	}
}

// Mouse parity for the new keybind: the r key and the help bar's button are the
// same method.
func TestThemeReloadKeyMatchesItsButton(t *testing.T) {
	for _, byClick := range []bool{false, true} {
		name := "key"
		if byClick {
			name = "click"
		}
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			m := hotModel(t, dir)
			openThemes(t, m)

			writeTheme(t, dir, "mine.toml", "name = \"mine\"\naccent = \"#ff00ff\"\n")
			if _, ok := m.themeLib.Get("mine"); ok {
				t.Fatal("the theme arrived without a reload")
			}

			if byClick {
				click(t, m, action{kind: actThemeReload})
			} else {
				send(t, m, "r")
			}

			if _, ok := m.themeLib.Get("mine"); !ok {
				t.Error("the reload did not pick the new theme up")
			}
			if m.screen != screenThemes {
				t.Error("reloading left the picker")
			}
		})
	}
}

// openThemes walks the menu to the themes entry the way a player would, so the
// test breaks if the entry is removed rather than silently skipping it.
func openThemes(t *testing.T, m *Model) {
	t.Helper()
	for i, c := range m.menu.choices {
		if c.kind == choiceThemes {
			m.menu.cursor = i
			send(t, m, "enter")
			if m.screen != screenThemes {
				t.Fatalf("menu entry did not open the picker")
			}
			return
		}
	}
	t.Fatal("no themes entry on the menu")
}
