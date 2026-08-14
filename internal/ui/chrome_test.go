package ui

import (
	"fmt"
	"image/color"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/colorprofile"
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
// this app drew before on one that cannot.
func TestPanelRuleGradesOnlyWhenItCan(t *testing.T) {
	// Wide enough that a gradient has somewhere to go.
	panel := func(p colorprofile.Profile) string {
		withColorProfile(t, p)
		return renderPanel("title", "", "×", strings.Repeat("x", 60), st.border)
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

// The status is inlaid like the close box, so it has to be measured like one —
// and given up rather than allowed to eat the rule.
func TestPanelDropsAStatusItCannotAfford(t *testing.T) {
	withColorProfile(t, colorprofile.TrueColor)

	const status = "a status far wider than this panel"
	narrow := renderPanel("title", status, "×", "body", st.border)
	plain := renderPanel("title", "", "×", "body", st.border)

	if strings.Contains(sgr.ReplaceAllString(narrow, ""), status) {
		t.Errorf("a status wider than the rule was drawn anyway:\n%s", narrow)
	}
	if lipgloss.Width(narrow) != lipgloss.Width(plain) {
		t.Errorf("the dropped status still moved the panel: %d vs %d",
			lipgloss.Width(narrow), lipgloss.Width(plain))
	}

	// Wide enough, and it appears — without widening the panel, which is sized
	// by its content.
	wide := renderPanel("title", "2/6", "×", strings.Repeat("x", 60), st.border)
	if !strings.Contains(sgr.ReplaceAllString(wide, ""), "2/6") {
		t.Errorf("a status the rule could afford was dropped:\n%s", wide)
	}
	if want := lipgloss.Width(renderPanel("title", "", "×", strings.Repeat("x", 60), st.border)); lipgloss.Width(wide) != want {
		t.Errorf("the status widened the panel: %d, want %d", lipgloss.Width(wide), want)
	}
}

// The board's mode and score live on the rule now, not in the header, so the
// two must not both carry them.
func TestBoardStateLivesInTheChrome(t *testing.T) {
	m := dailyModel(t, Options{})
	playDaily(t, m, 5)
	send(t, m, "a", "b", "o", "u", "t", "enter")
	frame := sgr.ReplaceAllString(draw(t, m), "")

	for _, want := range []string{"daily " + testDay, "5 letters", "1/6"} {
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
