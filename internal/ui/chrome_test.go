package ui

import (
	"fmt"
	"image/color"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/colorprofile"

	"github.com/nxck2005/surmise/internal/game"
)

// withColorProfile pretends the terminal has a given depth, and puts the real
// one back. colors is package state, like st.
func withColorProfile(t *testing.T, p colorprofile.Profile) {
	t.Helper()
	previous := colors
	setColorProfile(p)
	t.Cleanup(func() { setColorProfile(previous) })
}

func TestBlendRunsBetweenItsStops(t *testing.T) {
	withColorProfile(t, colorprofile.TrueColor)

	from := color.RGBA{R: 255, A: 255}
	to := color.RGBA{B: 255, A: 255}
	run := blend(16, from, to)

	if len(run) != 16 {
		t.Fatalf("blend gave %d colours, want 16", len(run))
	}
	// The ends are the stops themselves: a gradient that did not start and
	// finish on palette colours would be showing a colour no theme chose.
	if !sameColor(run[0], from) || !sameColor(run[len(run)-1], to) {
		t.Errorf("blend runs %v…%v, want %v…%v", run[0], run[len(run)-1], from, to)
	}
	if sameColor(run[len(run)/2], from) || sameColor(run[len(run)/2], to) {
		t.Error("the middle of the blend is one of its ends")
	}
}

// Below 256 colours a blend quantises to a few steps and reads as banding, so
// everything derived collapses to a flat colour instead.
func TestBlendIsFlatWithoutColourDepth(t *testing.T) {
	withColorProfile(t, colorprofile.ANSI)

	from := color.RGBA{R: 255, A: 255}
	to := color.RGBA{B: 255, A: 255}
	run := blend(8, from, to)

	if len(run) != 8 {
		t.Fatalf("blend gave %d colours, want 8", len(run))
	}
	for i, c := range run {
		if !sameColor(c, to) {
			t.Fatalf("colour %d is %v, want the flat %v", i, c, to)
		}
	}
	// lift and dim answer to the same rule, so nothing derived changes hue on a
	// terminal that cannot show the difference.
	if !sameColor(lift(from, 0.5), from) || !sameColor(dim(from, 0.5), from) {
		t.Error("lift or dim moved a colour on a low-depth terminal")
	}
}

func TestBlendSurvivesSillyInput(t *testing.T) {
	withColorProfile(t, colorprofile.TrueColor)

	if run := blend(0, color.Black); run != nil {
		t.Errorf("blend(0) = %v, want nothing", run)
	}
	// Fewer steps than stops: Blend1D returns the stops themselves, which would
	// leave a caller indexing past the end of a short run.
	if run := blend(1, color.Black, color.White); len(run) != 1 {
		t.Errorf("blend(1, two stops) gave %d colours, want 1", len(run))
	}
	if got := colorAt(nil, 3); got != nil {
		t.Errorf("colorAt(nil) = %v, want nil", got)
	}
	if got := colorAt([]color.Color{color.Black}, 9); !sameColor(got, color.Black) {
		t.Error("colorAt past the end did not clamp")
	}
}

// The rule is a gradient on a terminal that can show one, and exactly the frame
// this app drew before on one that cannot. Both draws are covered: the cached
// ordinary frame and the accent's uncached one, which must agree when they are
// given the same colour.
func TestPanelRuleGradesOnlyWhenItCan(t *testing.T) {
	// Wide enough that a gradient has somewhere to go.
	panel := func(p colorprofile.Profile) string {
		withColorProfile(t, p)
		return renderPanel("title", "", "×", strings.Repeat("x", 60))
	}

	rich, _, _ := strings.Cut(panel(colorprofile.TrueColor), "\n")
	poor, _, _ := strings.Cut(panel(colorprofile.ANSI), "\n")

	// The line carries the title as well as the rule, so the flat case is not
	// one colour but two: what matters is that the rule stops stepping.
	if got := countColors(rich); got < 5 {
		t.Errorf("the rule uses %d colours on a true-colour terminal, want a gradient", got)
	}
	if got := countColors(poor); got > 2 {
		t.Errorf("the rule uses %d colours on a 16-colour terminal, want it flat", got)
	}
	// Colour is all that changed: the rule is the same runes either way.
	if a, b := sgr.ReplaceAllString(rich, ""), sgr.ReplaceAllString(poor, ""); a != b {
		t.Errorf("the gradient moved the rule\n rich: %q\n poor: %q", a, b)
	}
}

// The ordinary frame draws from a cache; the win accent cannot, because its
// colour changes every frame. They are the same renderer wearing two hats, and
// this is what says so: handed the same border style, they must produce the same
// bytes at every colour depth and every width, or the cached frame is not the
// frame the app drew before it existed.
func TestPanelCacheMatchesTheUncachedRule(t *testing.T) {
	for _, p := range []colorprofile.Profile{colorprofile.TrueColor, colorprofile.ANSI256, colorprofile.ANSI} {
		withColorProfile(t, p)
		for _, width := range []int{0, 1, 12, 60, 200} {
			body := strings.Repeat("x", width)
			cached := renderPanel("title", "2/6", "×", body)
			lit := renderPanelLit("title", "2/6", "×", body, st.border)
			if cached != lit {
				t.Errorf("at %v and width %d the cached panel differs from the uncached one:\n%q\n---\n%q",
					p, width, cached, lit)
			}
		}
	}
}

// The cache belongs to the style set, so a theme change cannot leave the old
// palette's rule on the frame: setTheme builds a new set with a new cache.
func TestPanelRuleFollowsTheTheme(t *testing.T) {
	withColorProfile(t, colorprofile.TrueColor)

	before := renderPanel("title", "", "×", strings.Repeat("x", 60))
	withTheme(t, themed(t, `
accent = "#ff00ff"
muted = "#00ff00"
`))
	after := renderPanel("title", "", "×", strings.Repeat("x", 60))

	if before == after {
		t.Error("the rule kept the previous theme's colours")
	}
	// And it is the new palette that is on it, not just any change: the muted
	// border colour the gradient eases back to is the theme's.
	if !strings.Contains(after, "\x1b[38;2;0;255;0m") {
		t.Errorf("the rule does not use the new border colour:\n%q", after)
	}
}

// A colour-profile change is the other way the cached material goes stale: the
// gradient is exactly what a terminal without the depth for it does not want, so
// the material rendered under the old profile has to go.
func TestPanelRuleFollowsTheColourProfile(t *testing.T) {
	withColorProfile(t, colorprofile.TrueColor)
	rich := renderPanel("title", "", "×", strings.Repeat("x", 60))

	withColorProfile(t, colorprofile.ANSI)
	poor := renderPanel("title", "", "×", strings.Repeat("x", 60))

	if sgr.ReplaceAllString(rich, "") != sgr.ReplaceAllString(poor, "") {
		t.Error("the profile change moved the rule")
	}
	if got := countColors(poor); got > 2 {
		t.Errorf("the rule kept %d colours on a 16-colour terminal, want it flat", got)
	}
}

// The status is inlaid like the close box, so it has to be measured like one —
// and given up rather than allowed to eat the rule.
func TestPanelDropsAStatusItCannotAfford(t *testing.T) {
	withColorProfile(t, colorprofile.TrueColor)

	const status = "a status far wider than this panel"
	narrow := renderPanel("title", status, "×", "body")
	plain := renderPanel("title", "", "×", "body")

	if strings.Contains(sgr.ReplaceAllString(narrow, ""), status) {
		t.Errorf("a status wider than the rule was drawn anyway:\n%s", narrow)
	}
	if lipgloss.Width(narrow) != lipgloss.Width(plain) {
		t.Errorf("the dropped status still moved the panel: %d vs %d",
			lipgloss.Width(narrow), lipgloss.Width(plain))
	}

	// Wide enough, and it appears — without widening the panel, which is sized
	// by its content.
	wide := renderPanel("title", "2/6", "×", strings.Repeat("x", 60))
	if !strings.Contains(sgr.ReplaceAllString(wide, ""), "2/6") {
		t.Errorf("a status the rule could afford was dropped:\n%s", wide)
	}
	if want := lipgloss.Width(renderPanel("title", "", "×", strings.Repeat("x", 60))); lipgloss.Width(wide) != want {
		t.Errorf("the status widened the panel: %d, want %d", lipgloss.Width(wide), want)
	}
}

// The board's kind, mode and score live on the rule now, not in the header, so
// the two must not both carry them: the kind as the title, the rest as the
// status.
func TestBoardStateLivesInTheChrome(t *testing.T) {
	m := dailyModel(t, Options{})
	playDaily(t, m, 5)
	send(t, m, "a", "b", "o", "u", "t", "enter")
	frame := sgr.ReplaceAllString(draw(t, m), "")

	for _, want := range []string{"─ daily ", testDay, "5 letters", "1/6"} {
		if !strings.Contains(frame, want) {
			t.Errorf("the frame does not say %q:\n%s", want, frame)
		}
		if got := strings.Count(frame, want); got != 1 {
			t.Errorf("%q appears %d times, want once", want, got)
		}
	}
}

func TestChromeCountsWhatEachScreenIsAbout(t *testing.T) {
	m := dailyModel(t, Options{})
	playDaily(t, m, 5)
	m.game.g.Answer = "crane"
	send(t, m, "c", "r", "a", "n", "e", "enter")
	send(t, m, "esc")

	m.screen = screenMenu
	showDaily(t, m)
	if frame := sgr.ReplaceAllString(m.View().Content, ""); !strings.Contains(frame, "1/3 done") {
		t.Errorf("the daily rule does not carry the day's progress:\n%s", frame)
	}

	m.screen = screenMenu
	m.menu.point(menuIndex(t, m, choiceList, 0))
	frame := sgr.ReplaceAllString(send(t, m, "enter"), "")
	if want := fmt.Sprintf("%d saved", len(m.list.items)); !strings.Contains(frame, want) {
		t.Errorf("the puzzle list rule does not say %q:\n%s", want, frame)
	}
}

func sameColor(a, b color.Color) bool {
	if a == nil || b == nil {
		return a == b
	}
	ar, ag, ab, aa := a.RGBA()
	br, bg, bb, ba := b.RGBA()
	return ar == br && ag == bg && ab == bb && aa == ba
}

func countColors(s string) int {
	seen := make(map[string]bool)
	for _, m := range sgr.FindAllString(s, -1) {
		seen[m] = true
	}
	delete(seen, "\x1b[m")
	return len(seen)
}

// The board is titled by its kind, and the status inlay does not say it again.
func TestBoardIsTitledByItsKind(t *testing.T) {
	m := boardModel(t, 5)
	g := m.game.g
	for _, tc := range []struct {
		want string
		set  func()
	}{
		{"puzzle", func() {}},
		{"custom", func() { g.Custom = true }},
		{"challenge", func() { g.Custom = false; g.Challenge = &game.ChallengeInfo{} }},
	} {
		tc.set()
		if got := m.screenTitle(); got != tc.want {
			t.Errorf("title = %q, want %q", got, tc.want)
		}
		if status := m.screenStatus(); strings.Contains(status, tc.want) {
			t.Errorf("the status %q repeats the title %q", status, tc.want)
		}
	}
}

// The panel's rule names the screen, so no body repeats that name as a line of
// its own: it cost two rows on every screen, which a short terminal could not
// spare.
func TestNoScreenRepeatsItsTitle(t *testing.T) {
	check := func(t *testing.T, m *Model, where string) {
		t.Helper()
		title := m.screenTitle()
		lines := strings.Split(plain(draw(t, m)), "\n")
		for _, line := range lines {
			if strings.Contains(line, "╭") {
				continue // the rule itself
			}
			if strings.Trim(line, " │") == title {
				t.Errorf("%s: the body repeats the title %q", where, title)
			}
		}
	}

	menu := newModel(t)
	check(t, menu, "menu")
	for i, c := range menu.menu.choices {
		if c.kind == choiceQuit {
			continue
		}
		m := newModel(t)
		m.menu.cursor = i
		send(t, m, "enter")
		check(t, m, c.label)
	}
	for i, label := range socialLabels {
		m := newModel(t)
		m.menu.cursor = menuIndex(t, m, choiceSocial, 0)
		send(t, m, "enter")
		m.social.cursor = i
		send(t, m, "enter")
		check(t, m, label)
	}
}

// panelWidthOf is how wide the drawn panel is, from its top rule.
func panelWidthOf(t *testing.T, frame string) int {
	t.Helper()
	for _, line := range strings.Split(plain(frame), "\n") {
		if i := strings.Index(line, "╭"); i >= 0 {
			return lipgloss.Width(strings.TrimRight(line[i:], " "))
		}
	}
	t.Fatalf("no panel in the frame:\n%s", plain(frame))
	return 0
}

// Screens are drawn at a few panel widths rather than each at its own, so the
// frame does not grow and shrink as the player moves between them. The
// list-like screens share the narrow width; the board and the setups that sit
// beside it share the wide one.
func TestPanelsComeInAFewWidths(t *testing.T) {
	open := func(kind choiceKind, length int) *Model {
		m := newModel(t)
		if kind != choiceQuit {
			m.menu.cursor = menuIndex(t, m, kind, length)
			send(t, m, "enter")
		}
		return m
	}
	groups := map[string][]*Model{
		"narrow": {open(choiceQuit, 0), open(choiceDaily, 0), open(choiceSocial, 0),
			open(choiceSettings, 0), open(choiceHowTo, 0), open(choiceThemes, 0), open(choiceAbout, 0)},
		"wide": {open(choiceNewGame, 4), open(choiceNewGame, 6), open(choiceSprint, 0)},
	}
	widths := map[string]int{}
	for name, ms := range groups {
		for i, m := range ms {
			w := panelWidthOf(t, draw(t, m))
			if i == 0 {
				widths[name] = w
			} else if w != widths[name] {
				t.Errorf("%s screen %d is %d wide, want %d like the first", name, i, w, widths[name])
			}
		}
	}
	if widths["narrow"] >= widths["wide"] {
		t.Errorf("narrow panels are %d, wide %d", widths["narrow"], widths["wide"])
	}

	// A terminal narrower than a step gets a panel that fits it, as before.
	m := open(choiceSettings, 0)
	const narrow = 60
	m.Update(tea.WindowSizeMsg{Width: narrow, Height: testHeight})
	if w := panelWidthOf(t, m.View().Content); w > narrow {
		t.Errorf("the settings panel is %d wide on a %d-column terminal", w, narrow)
	}
}

func TestPanelWidthSteps(t *testing.T) {
	for _, tc := range []struct{ content, room, want int }{
		{30, 0, panelSteps[0]},
		{panelSteps[0] + 1, 0, panelSteps[1]},
		{panelSteps[1] + 5, 0, panelSteps[1] + 5}, // wider than every step keeps its own
		{30, 40, 40}, // the terminal is narrower than the step
		{45, 40, 45}, // never narrower than the content
	} {
		if got := panelWidth(tc.content, tc.room); got != tc.want {
			t.Errorf("panelWidth(%d, %d) = %d, want %d", tc.content, tc.room, got, tc.want)
		}
	}
}
