package ui

import (
	"strings"
	"testing"
)

// scan is the only place markers come out of a finished frame, so it is the
// place a marker-shaped sequence in data has to be survivable. No accepted
// value can carry an ESC today — game.Validate holds words to lowercase
// letters, theme.Parse refuses control characters, safeText repairs the prose
// sinks — so these are all reached by calling scan directly, which is the point:
// the guarantee is enforced in four other packages, and this is the parser that
// would have to survive their regression.
//
// What is asserted is that scan is total, not that it is clever. A panic here is
// a crash on the frame after any of those layers changes.

// A negative id parses. Atoi accepts it, and the old bound was one-sided, so
// this indexed zones[-1] and took the process down on the next frame.
func TestScanSurvivesAMarkerIdThatIsNotAZone(t *testing.T) {
	for _, id := range []string{"-1", "-999", "9999", "", "abc", "1.5", "0x1", " 1", "+1"} {
		t.Run("id="+id, func(t *testing.T) {
			h := &hitMap{}
			h.mark(action{kind: actQuit}, "quit")
			before := len(h.zones)

			frame := "left" + markerStart + id + markerEnd + "right"
			got := h.scan(frame)

			if len(h.zones) != before {
				t.Errorf("scan changed the zone count to %d, want %d", len(h.zones), before)
			}
			// Whatever the payload was, the introducer does not reach the
			// renderer: a terminal would read it as an APC string and swallow
			// the row to the next terminator.
			if strings.Contains(got, markerStart) || strings.Contains(got, markerEnd) {
				t.Errorf("a marker survived into the frame: %q", got)
			}
			if want := "leftright"; got != want {
				t.Errorf("scan = %q, want the text with the marker removed: %q", got, want)
			}
		})
	}
}

// An introducer with no terminator is not a marker of ours — mark always writes
// both halves — and it used to be written to the frame verbatim, where a
// terminal consumes the rest of the row looking for the end of the string.
func TestScanStripsAnUnterminatedIntroducer(t *testing.T) {
	h := &hitMap{}
	h.mark(action{kind: actQuit}, "quit")

	frame := "visible" + markerStart + " and the rest of the row"
	got := h.scan(frame)

	if strings.Contains(got, markerStart) {
		t.Errorf("an unterminated introducer reached the frame: %q", got)
	}
	if want := "visible and the rest of the row"; got != want {
		t.Errorf("scan = %q, want the row with the introducer removed: %q", got, want)
	}
}

// The row-level cases compose: several bad markers on one line, an unterminated
// one after a good one, and a real marker either side of them. A good zone must
// still land where it was drawn — a fix that dropped the whole row would lose
// a click target as surely as a panic loses the app.
func TestScanKeepsRealMarkersAroundBadOnes(t *testing.T) {
	h := &hitMap{}
	quit := h.mark(action{kind: actQuit}, "quit")
	back := h.mark(action{kind: actBack}, "back")

	// A good marker, a negative id, an unterminated introducer, then the other
	// good marker on the next line.
	frame := quit + " x " + markerStart + "-1" + markerEnd +
		"tail\n" + markerStart + "dangling" + "junk\n" + back
	got := h.scan(frame)

	if strings.Contains(got, markerStart) || strings.Contains(got, markerEnd) {
		t.Errorf("a marker survived into the frame: %q", got)
	}
	// The negative id must not have moved anything, and the terminator-stripped
	// row must still be there.
	if !strings.Contains(got, "quit x tail") {
		t.Errorf("the first row lost its text: %q", got)
	}
	if !strings.Contains(got, "danglingjunk") {
		t.Errorf("the second row lost its text: %q", got)
	}
	if !strings.Contains(got, "back") {
		t.Errorf("the second marker was lost: %q", got)
	}

	// The zone the negative id named is the one that exists, so it must be
	// positioned by its own marker and not by the forged one.
	if r, ok := h.find(action{kind: actQuit}); !ok {
		t.Error("the quit target went missing")
	} else if r.x != 0 {
		t.Errorf("the quit target is at column %d, want 0 — the negative id moved it", r.x)
	}
	if r, ok := h.find(action{kind: actBack}); !ok {
		t.Error("the back target went missing")
	} else if r.y != 2 {
		t.Errorf("the back target is on row %d, want 2", r.y)
	}
}

// The unterminated case must not consume the rest of the frame. The old loop
// broke out of the per-line walk, so this was true, and it is the property that
// makes stripping the introducer better than passing it through: the row
// survives either way, but only one of them leaves the terminal able to read it.
func TestScanLeavesLaterRowsAlone(t *testing.T) {
	h := &hitMap{}
	h.mark(action{kind: actQuit}, "quit")

	frame := "row one" + markerStart + "\nrow two\nrow three"
	got := h.scan(frame)

	if n := strings.Count(got, "\n"); n != 2 {
		t.Errorf("scan produced %d newlines, want 2: %q", n, got)
	}
	for _, want := range []string{"row one", "row two", "row three"} {
		if !strings.Contains(got, want) {
			t.Errorf("row %q was lost: %q", want, got)
		}
	}
}

// An empty frame, and one with no markers at all, are the two shapes a screen
// that draws nothing produces.
func TestScanOnAFrameWithNothingToStrip(t *testing.T) {
	h := &hitMap{}
	for _, frame := range []string{"", "plain text", "styled \x1b[1mbold\x1b[0m text"} {
		if got := h.scan(frame); got != frame {
			t.Errorf("scan(%q) = %q, want it unchanged", frame, got)
		}
	}
	// And a nil hitMap, which every render path may hold.
	var nilMap *hitMap
	if got := nilMap.scan(markerStart + "0" + markerEnd + "x"); !strings.Contains(got, "x") {
		t.Errorf("a nil hitMap mangled the frame: %q", got)
	}
}
