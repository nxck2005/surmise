package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/colorprofile"
)

// The menu reads in two weights: what gets you a board, and what gets you
// somewhere else. It costs no row, which matters on a terminal that has none to
// spare.
func TestMenuReadsInTwoWeights(t *testing.T) {
	m := newModel(t)

	play := map[choiceKind]bool{choiceNewGame: true, choiceDaily: true, choiceCustom: true}
	for _, c := range m.menu.choices {
		want := st.muted
		if play[c.kind] {
			want = st.text
		}
		if got := m.menu.weight(c); got.GetForeground() != want.GetForeground() {
			t.Errorf("%q is drawn in %v, want %v", c.label, got.GetForeground(), want.GetForeground())
		}
	}

	// And it reaches the frame: a mode and a navigation row are not the same
	// colour on screen.
	frame := draw(t, m)
	mode := rowColor(t, m, frame, choiceNewGame, 5)
	nav := rowColor(t, m, frame, choiceAbout, 0)
	if mode == nav {
		t.Errorf("modes and navigation are both drawn in %s", mode)
	}
}

// rowColor is the colour a menu row's label is drawn in, read from the frame.
func rowColor(t *testing.T, m *Model, frame string, kind choiceKind, length int) string {
	t.Helper()
	r, ok := m.hits.find(action{kind: actMenuChoice, index: menuIndex(t, m, kind, length)})
	if !ok {
		t.Fatalf("no target for menu kind %v", kind)
	}
	line := strings.Split(frame, "\n")[r.y]
	got := fgCodes.FindStringSubmatch(line[min(r.x, len(line)):])
	if got == nil {
		t.Fatalf("no colour on menu row %d", r.y)
	}
	return got[1]
}

// A bar ends part-way through a cell, so two counts a hair apart no longer draw
// the same length.
func TestHistogramMeasuresInEighths(t *testing.T) {
	m := newModel(t)
	// Counts that do not divide the width evenly, which is the whole point of
	// measuring in eighths.
	m.profile.summary.Distribution = map[int]int{3: 3, 4: 4, 5: 7}

	plain := sgr.ReplaceAllString(m.profile.renderDistribution(), "")
	rows := strings.Split(plain, "\n")[1:]
	if len(rows) != 3 {
		t.Fatalf("got %d bars, want 3:\n%s", len(rows), plain)
	}
	if rows[0] == rows[1] {
		t.Errorf("8 and 9 draw the same bar:\n%s", plain)
	}
	if !strings.ContainsAny(plain, strings.Join(eighths[:], "")) {
		t.Errorf("no partial cell anywhere in the histogram:\n%s", plain)
	}
	// The full bar is whole cells: the peak is exactly the width.
	if got := strings.Count(rows[2], fullBlock); got != distributionWidth {
		t.Errorf("the peak bar is %d cells, want %d", got, distributionWidth)
	}
}

// A theme that chose its own bar rune means it: half of somebody else's glyph is
// not a smaller version of it.
func TestHistogramKeepsWholeCellsForACustomBarGlyph(t *testing.T) {
	// After the model, which applies the saved theme as it starts.
	m := newModel(t)
	withTheme(t, themed(t, "[glyphs]\nbar = \"=\"\n"))
	m.profile.summary.Distribution = map[int]int{3: 3, 4: 4}

	plain := sgr.ReplaceAllString(m.profile.renderDistribution(), "")
	if strings.ContainsAny(plain, strings.Join(eighths[:], "")) {
		t.Errorf("a custom bar glyph still drew partial blocks:\n%s", plain)
	}
	if !strings.Contains(plain, "=") {
		t.Errorf("the custom bar glyph was not used:\n%s", plain)
	}
}

// The shading stays inside the theme's own bar colour, and disappears entirely
// on a terminal that cannot show it.
func TestHistogramShadesOnlyWhenItCan(t *testing.T) {
	m := newModel(t)
	m.profile.summary.Distribution = map[int]int{4: 10}

	withColorProfile(t, colorprofile.TrueColor)
	shaded := len(colorsIn(m.profile.renderDistribution()))
	if shaded < 5 {
		t.Errorf("the histogram uses %d colours on a true-colour terminal, want a ramp", shaded)
	}

	withColorProfile(t, colorprofile.ANSI)
	flat := m.profile.renderDistribution()
	// The row still carries its label and its count, so a flat bar is not one
	// colour — it is however many the ramp is not adding.
	if got := len(colorsIn(flat)); got >= shaded {
		t.Errorf("the histogram uses %d colours on a 16-colour terminal, want fewer than %d",
			got, shaded)
	}
	if want := colorCode(t, st.bar.GetForeground()); !colorsIn(flat)[want] {
		t.Errorf("the flat bar is not the theme's bar colour (%s):\n%q", want, flat)
	}
}
