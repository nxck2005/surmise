package ui

import (
	"image/color"
	"regexp"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// fgCodes finds every foreground colour in a frame, bold or not.
var fgCodes = regexp.MustCompile(`\x1b\[[0-9;]*38;2;([0-9;]+)m`)

func colorsIn(frame string) map[string]bool {
	seen := make(map[string]bool)
	for _, m := range fgCodes.FindAllStringSubmatch(frame, -1) {
		seen[m[1]] = true
	}
	return seen
}

// raiseSplashAt puts the splash up with the clock held still, and returns the
// model and the instant the sweep started from.
func raiseSplashAt(t *testing.T, motion string, now *time.Time) *Model {
	t.Helper()
	m := newModelWithMotion(t, motion)
	m.Update(tea.WindowSizeMsg{Width: testWidth, Height: testHeight})
	withClock(t, now)
	m.raiseSplash()
	if m.screen != screenSplash {
		t.Fatal("the splash did not go up")
	}
	return m
}

// The whole claim of the sweep: it is light crossing the art and then gone. Once
// it is over the splash is exactly the flat drawing it always was, and while it
// runs it changes colours only.
func TestSplashShimmerSettlesToTheFlatArt(t *testing.T) {
	now := time.Now()
	m := raiseSplashAt(t, motionPronouncedName, &now)

	first := m.View().Content
	middle := ""
	for range 40 {
		advance(t, m, &now, frameInterval)
		if p, ok := m.anim.shimmer(now); ok && p > 0.4 && p < 0.6 {
			middle = m.View().Content
		}
	}
	if middle == "" {
		t.Fatal("the sweep never reached the middle of the art")
	}
	settle(t, m, &now)
	settled := m.View().Content

	still := newModelWithMotion(t, motionOffName)
	still.Update(tea.WindowSizeMsg{Width: testWidth, Height: testHeight})
	withClock(t, &now)
	still.raiseSplash()

	if settled != still.View().Content {
		t.Error("the settled splash is not the frame motion-off draws")
	}
	// Mid-sweep it is the same drawing, differently lit.
	if sgr.ReplaceAllString(middle, "") != sgr.ReplaceAllString(settled, "") {
		t.Error("the sweep moved the art")
	}
	if len(colorsIn(middle)) <= len(colorsIn(settled)) {
		t.Error("the sweep did not light anything")
	}
	if first == middle {
		t.Error("the sweep did not move between its start and its middle")
	}
}

func TestSplashDoesNotShimmerWithMotionOff(t *testing.T) {
	now := time.Now()
	m := raiseSplashAt(t, motionOffName, &now)

	if _, ok := m.anim.shimmer(now); ok {
		t.Error("motion off started a sweep")
	}
	if m.anim.busy(now) {
		t.Error("motion off armed the animation chain")
	}
}

// The frame answers a win by turning, not by blinking: the accent arrives, holds
// and leaves, and the border it lands on is the one it started from.
func TestWinAccentRisesAndFalls(t *testing.T) {
	now := time.Now()
	m := animModel(t)
	withClock(t, &now)
	startBoard(t, m, "crane")
	m.Update(tea.WindowSizeMsg{Width: testWidth, Height: testHeight})
	send(t, m, "c", "r", "a", "n", "e", "enter")

	border := st.border.GetForeground()
	var peak, sides []string
	for range 200 {
		if !m.anim.busy(now) {
			break
		}
		sides = append(sides, sideColor(t, m.frame(nil)))
		if p, ok := m.anim.winning(now); ok && p > 0.4 && p < 0.6 {
			peak = append(peak, sideColor(t, m.frame(nil)))
		}
		advance(t, m, &now, frameInterval)
	}

	if len(peak) == 0 {
		t.Fatal("the win never reached the middle of its accent")
	}
	if peak[0] != colorCode(t, st.accent.GetForeground()) {
		t.Errorf("the frame at the peak of a win is %s, want the accent %s",
			peak[0], colorCode(t, st.accent.GetForeground()))
	}
	if sides[0] != colorCode(t, border) {
		t.Errorf("the frame started at %s, want the border %s", sides[0], colorCode(t, border))
	}
	if got := sideColor(t, m.frame(nil)); got != colorCode(t, border) {
		t.Errorf("the settled frame is %s, want the border %s", got, colorCode(t, border))
	}
	// It really eased rather than switching: more than the two end colours were
	// used on the way.
	unique := make(map[string]bool)
	for _, c := range sides {
		unique[c] = true
	}
	if len(unique) < 4 {
		t.Errorf("the accent used %d colours, want it eased in", len(unique))
	}
}

// The light walks the solved word once, forwards, and is gone before the effect
// is — so a win ends on the settled row rather than on a light still moving.
func TestCelebrationWalksTheSolvedRowOnce(t *testing.T) {
	now := time.Now()
	m := animModel(t)
	withClock(t, &now)
	startBoard(t, m, "crane")
	send(t, m, "c", "r", "a", "n", "e", "enter")

	id := m.game.g.ID
	settledText := sgr.ReplaceAllString(m.frame(nil), "")

	last, seen, ended := -1, 0, false
	for range 200 {
		if !m.anim.busy(now) {
			break
		}
		if lit, ok := m.anim.celebrating(now, id, 0); ok {
			if lit < last {
				t.Fatalf("the light went backwards: %d after %d", lit, last)
			}
			if ended {
				t.Fatal("the light came back after it had finished")
			}
			last, seen = lit, seen+1
		} else if seen > 0 {
			ended = true
		}
		if got := sgr.ReplaceAllString(m.frame(nil), ""); got != settledText {
			t.Fatal("the celebration moved the board")
		}
		advance(t, m, &now, frameInterval)
	}

	if seen == 0 {
		t.Fatal("the solved row was never lit")
	}
	if last != m.game.g.Length-1 {
		t.Errorf("the light stopped at tile %d, want the last one (%d)", last, m.game.g.Length-1)
	}
	if !ended {
		t.Error("the light was still running when the effect ended")
	}
}

// sideColor is the colour of a panel's left border, which is drawn in the border
// style itself rather than along the rules' gradient.
func sideColor(t *testing.T, frame string) string {
	t.Helper()
	lines := strings.Split(frame, "\n")
	m := fgCodes.FindStringSubmatch(lines[len(lines)/2])
	if m == nil {
		t.Fatalf("no colour on the middle line of the frame:\n%s", lines[len(lines)/2])
	}
	return m[1]
}

// colorCode is how a colour is written in a frame, so a test can compare what
// it sees against a palette colour without knowing the escape format.
func colorCode(t *testing.T, c color.Color) string {
	t.Helper()
	m := fgCodes.FindStringSubmatch(lipgloss.NewStyle().Foreground(c).Render("x"))
	if m == nil {
		t.Fatalf("colour %v rendered without a foreground code", c)
	}
	return m[1]
}
