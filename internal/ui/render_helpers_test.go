package ui

// The frame pipeline's fast paths are byte-for-byte the calls they replace.
//
// These helpers exist because lipgloss measures a composed line every time it
// touches one — twice in JoinVertical, once more in Place, once per line for
// padding — and the frame is the widest thing the app draws. The tests here
// hold each fast path to the library call it stands in for, so a dependency
// bump that changes one of their rules fails here rather than as a frame that
// moved a column in a terminal nobody was looking at.

import (
	"errors"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"

	"github.com/nxck2005/surmise/internal/game"
	"github.com/nxck2005/surmise/internal/theme"
)

func TestJoinVerticalMatchesLipgloss(t *testing.T) {
	withTheme(t, theme.Default())
	// Two weights, a blank, a wide line, an empty part, and a multi-line block
	// with an odd width, all in the shapes the screens really pass.
	cases := [][]string{
		{},
		{"one"},
		{"", ""},
		{"heading", "", "list\nof\nrows"},
		{st.accent.Render("solved in 3"), st.muted.Render("a longer muted line"), ""},
		{"café", "日本語", "x"},
		{markerStart + "0" + markerEnd + "marked", "plain", "  indented"},
		{"a\nb\nc", "d"},
	}
	for _, parts := range cases {
		if got, want := joinVerticalCentre(parts...), lipgloss.JoinVertical(lipgloss.Center, parts...); got != want {
			t.Errorf("joinVerticalCentre(%q) = %q, want %q", parts, got, want)
		}
		if got, want := joinVerticalLeft(parts...), lipgloss.JoinVertical(lipgloss.Left, parts...); got != want {
			t.Errorf("joinVerticalLeft(%q) = %q, want %q", parts, got, want)
		}
	}
}

func TestCentreLineMatchesLipgloss(t *testing.T) {
	withTheme(t, theme.Default())
	lines := []string{
		"", "x", "ab", st.accent.Render("styled"), "日本語", "  padded  ",
	}
	for _, line := range lines {
		w := lipgloss.Width(line)
		for width := w; width <= w+7; width++ {
			want := lipgloss.NewStyle().Width(width).Align(lipgloss.Center).Render(line)
			if got := centreLine(line, width); got != want {
				t.Errorf("centreLine(%q, %d) = %q, want %q", line, width, got, want)
			}
		}
	}
}

// referenceBlock is block's old two-pass implementation, kept as the answer the
// one-pass version must reproduce.
func referenceBlock(s string) string {
	lines := strings.Split(s, "\n")
	width := 0
	for _, l := range lines {
		if w := lipgloss.Width(l); w > width {
			width = w
		}
	}
	for i, l := range lines {
		if pad := width - lipgloss.Width(l); pad > 0 {
			lines[i] = l + strings.Repeat(" ", pad)
		}
	}
	return strings.Join(lines, "\n")
}

func TestBlockMatchesReference(t *testing.T) {
	withTheme(t, theme.Default())
	for _, s := range []string{
		"",
		"one line",
		"short\nlonger line\nshort",
		"\n\nsame\n\n",
		st.accent.Render("styled") + "\n" + st.muted.Render("a longer styled line"),
		"café\n日本語",
		markerStart + "0" + markerEnd + "marked\nplain",
		"already\nsquare",
	} {
		if got, want := block(s), referenceBlock(s); got != want {
			t.Errorf("block(%q) = %q, want %q", s, got, want)
		}
	}
}

// TestPanelBodyMatchesLipgloss holds the hand-written padding to the style call
// it replaced: Padding(row, column).Render, which also squares short lines and
// makes the blank rows as wide as the block.
func TestPanelBodyMatchesLipgloss(t *testing.T) {
	withTheme(t, theme.Default())
	padY, padX := st.metric.PanelPadY, st.metric.PanelPadX
	contents := []string{
		"",
		"body",
		"short\nmuch longer line\nshort",
		st.accent.Render("solved") + "\n" + st.muted.Render("a longer muted line"),
		"line\n\n" + markerStart + "0" + markerEnd + "marked",
		"日本語\nab",
	}
	for _, content := range contents {
		want := lipgloss.NewStyle().
			Padding(padY, padX).
			Render(content)
		lines, width := squareLines(content)
		lines, width = panelBodyLines(lines, width)
		got := strings.Join(lines, "\n")
		if got != want {
			t.Errorf("panel body for %q =\n%q\nwant\n%q", content, got, want)
		}
		for i, line := range lines {
			if w := lipgloss.Width(line); w != width {
				t.Errorf("panel line %d is %d cells, want %d", i, w, width)
			}
		}
	}
}

// referenceFrameLines is the old widen: square the lines, centre the block in
// the step panelWidth picks, and let the panel split it again.
func referenceFrameLines(s string, room int) ([]string, int) {
	lines := strings.Split(s, "\n")
	w := 0
	for _, l := range lines {
		if lw := lipgloss.Width(l); lw > w {
			w = lw
		}
	}
	target := panelWidth(w, room)
	left := (target - w) / 2
	l, r := strings.Repeat(" ", left), strings.Repeat(" ", target-w-left)
	for i, line := range lines {
		lines[i] = l + line + strings.Repeat(" ", w-lipgloss.Width(line)) + r
	}
	return lines, target
}

func TestFrameContentMatchesJoinThenWiden(t *testing.T) {
	withTheme(t, theme.Default())
	bodies := []string{
		"",
		"body",
		"one\n\ntwo",
		"odd\nwidth",
		st.accent.Render("styled body") + "\n" + st.muted.Render("second line"),
	}
	helps := []string{"esc menu", "", "a longer help line"}
	for _, body := range bodies {
		for _, help := range helps {
			joined := lipgloss.JoinVertical(lipgloss.Center, body, help)
			want, wantWidth := referenceFrameLines(joined, bodyWidth(100))
			lines, width := frameContent(body, help, bodyWidth(100))
			if got := strings.Join(lines, "\n"); got != strings.Join(want, "\n") {
				t.Errorf("frameContent(%q, %q) =\n%q\nwant\n%q", body, help, got, strings.Join(want, "\n"))
			}
			if width != wantWidth {
				t.Errorf("frameContent(%q, %q) width = %d, want %d", body, help, width, wantWidth)
			}
		}
	}
}

func TestPlaceCentreKnownMatchesLipgloss(t *testing.T) {
	withTheme(t, theme.Default())
	blocks := []string{
		"a",
		"ab\ncd",
		"one\nlonger line\nx",
		"",
	}
	for _, raw := range blocks {
		square := block(raw)
		bw := lipgloss.Width(square)
		bh := lipgloss.Height(square)
		widths := []int{bw, bw + 1, bw + 2, bw + 9}
		if bw > 2 {
			widths = append(widths, bw-1, bw-2)
		}
		for _, w := range widths {
			for _, h := range []int{bh, bh + 1, bh + 2, bh + 9} {
				want := lipgloss.Place(w, h, lipgloss.Center, lipgloss.Center, square)
				if got := placeCentreKnown(w, h, square, bw, bh); got != want {
					t.Errorf("placeCentreKnown(%d,%d) for %q =\n%q\nwant\n%q", w, h, raw, got, want)
				}
			}
		}
	}
}

// TestCachedCellsMatchFreshRender draws every cached kind twice and compares
// the second with a direct Render. A cache that outlived its style set, or
// keyed a cell on too little, would show up as a tile that kept the previous
// theme's colours.
func TestCachedCellsMatchFreshRender(t *testing.T) {
	for _, th := range []*theme.Theme{
		theme.Default(),
		themed(t, "accent = \"#ff0000\"\n[style.tile.correct]\nbold = false\n"),
	} {
		withTheme(t, th)
		faces := []cellFace{
			faceCorrect, facePresent, faceAbsent, faceActive, faceActiveRefused,
			faceCaret, faceEmpty, faceEmptyBlank,
			faceKeyUnused, faceKeyCorrect, faceKeyPresent, faceKeyAbsent,
		}
		texts := []string{"A", "Z", "·"}
		for _, face := range faces {
			style, frame := st.faceStyle(face)
			for _, text := range texts {
				for _, tiles := range []int{flatTile, tallTile} {
					for _, outline := range []bool{false, true} {
						if !face.slot() && outline {
							continue
						}
						// Warm the cache, then ask again: the second answer is
						// what every later frame draws.
						_ = st.cell(face, text, tiles, outline)
						got := st.cell(face, text, tiles, outline)
						var want string
						if face.slot() {
							want = slotCell(style, frame, text, tiles, outline)
						} else {
							want = sized(style, tiles).Render(text)
						}
						if got != want {
							t.Errorf("cell face %d %q tiles=%d outline=%v = %q, want %q",
								face, text, tiles, outline, got, want)
						}
					}
				}
			}
		}
	}
}

// A scored row is what the cache is for, so it gets its own answer: the bytes
// it draws are the bytes each tile draws on its own, with the gutter between
// them.
func TestScoredRowIsTheTilesItCaches(t *testing.T) {
	withTheme(t, theme.Default())
	marks := []game.Mark{game.Correct, game.Present, game.Absent, game.Correct, game.Present}
	_ = renderScoredRow("crane", marks)
	got := renderScoredRow("crane", marks)
	cells := make([]string, len(marks))
	for i, m := range marks {
		cells[i] = st.cell(faceForMark(m), upperLetter("crane"[i]), flatTile, false)
	}
	if want := joinTiles(cells); got != want {
		t.Errorf("cached scored row = %q, want %q", got, want)
	}
}

// TestThemeNoteBoxFitsEveryNote pins the cached box size: the note drawn when
// the cursor moves must never outgrow the box measured when the list loaded,
// whatever theme is being previewed at the time. Styling only adds escapes, so
// a theme change cannot move a cell — but if a future note grows, this fails
// rather than pushing the picker's list up and down.
func TestThemeNoteBoxFitsEveryNote(t *testing.T) {
	withTheme(t, theme.Default())
	entries := []theme.Entry{
		{Name: "built", Source: "built-in", Theme: &theme.Theme{Author: "someone"}},
		{Name: "plain", Source: "built-in", Theme: &theme.Theme{}},
		{Name: "mine", Source: "/home/me/.config/surmise/themes/mine.toml", Theme: &theme.Theme{}},
		{Name: "warned", Source: "warned.toml", Theme: &theme.Theme{},
			Warnings: []theme.Warning{{Line: 3, Msg: "unknown key"}, {Line: 4, Msg: "bad colour"}}},
		{Name: "broken", Source: "broken.toml", Err: errors.New("unreadable")},
	}
	m := &themeScreen{entries: entries}
	m.measureNotes()

	// A preview changes the style set; the measured box must still bound every
	// note exactly as it did before.
	withTheme(t, theme.Default())
	for _, e := range entries {
		n := m.noteFor(e)
		if w := lipgloss.Width(n); w > m.noteW {
			t.Errorf("%s: note is %d cells, box is %d", e.Name, w, m.noteW)
		}
		if h := lipgloss.Height(n); h > m.noteH {
			t.Errorf("%s: note is %d lines, box is %d", e.Name, h, m.noteH)
		}
	}
}
