package ui

// Mouse hit-testing.
//
// Where an element lands on screen is only knowable after three layers of
// centring: each screen's JoinVertical(Center, …), then renderPanel's border and
// padding, then lipgloss.Place in the root View. Predicting that arithmetic is
// possible but silently fragile — lipgloss even rounds the odd cell to the left
// when joining and to the right when placing.
//
// So instead of predicting positions we measure them. Every clickable atom is
// prefixed at render time with a zero-width marker carrying an id (mark); the
// root View then scans the finished frame for those markers to learn their real
// cell coordinates and strips them before the string reaches Bubble Tea (scan).
// The marker is an APC escape: terminals ignore APC strings outright, and the
// ANSI state machine lipgloss measures with counts them as zero width, so a
// marker cannot shift a single cell of layout — TestMarkersDoNotAffectLayout
// holds us to that.
//
// A *hitMap is threaded explicitly through view()/help() rather than kept in a
// package-level manager, and every method is nil-safe, so rendering without
// collecting hit regions stays possible (which is what the layout test needs).

import (
	"strconv"
	"strings"

	"charm.land/lipgloss/v2"
)

const (
	markerStart = "\x1b_"
	markerEnd   = "\x1b\\"
)

// actionKind is what a click does. Every keybind that changes state has a kind
// here, which is what makes mouse-only play possible.
type actionKind int

const (
	actNone        actionKind = iota
	actMenuChoice             // index: menu row
	actListRow                // index: puzzle list row
	actDailyRow               // index: daily screen row, one per mode
	actThemeRow               // index: theme picker row
	actThemeReload            // re-read the themes directory now
	actSettingNext            // index: settings row, next value
	actSettingPrev            // index: settings row, previous value
	actLetter                 // letter: on-screen keyboard cap
	actSubmit                 // enter
	actBackspace              // backspace
	actTrim                   // index: erase the typed row back to this slot
	actNewPuzzle              // tab+enter
	actCancelNew              // dismiss the armed new-puzzle prompt
	actBack                   // esc
	actQuit
	actDeletePuzzle // index: puzzle list row to delete; arms, then confirms
	actCancelDelete // dismiss the armed delete prompt
	actJumpTop      // home: first row of a scrolling list
	actJumpBottom   // end: last row of a scrolling list
	actSplashDismiss
)

// action identifies one clickable thing. It is comparable so it doubles as the
// hover key: a screen asks hovered(a) while rendering the very atom it marks.
type action struct {
	kind   actionKind
	index  int
	letter byte

	// help marks the help bar's copy of an action. The bar's buttons repeat
	// what the screen already offers — "enter select" carries the very action
	// the selected row does — and because the action doubles as the hover key,
	// the two atoms could not be told apart: pointing at a row lit the button
	// up as well, and vice versa. The flag exists only to separate those two
	// hover identities. dispatch never reads it, so the button and the row
	// still do exactly the same thing, and find ignores it so a test can ask
	// for a target without knowing which copy it will get.
	help bool
}

// sameTarget reports whether two actions do the same thing, regardless of which
// copy of it they are.
func (a action) sameTarget(b action) bool {
	a.help, b.help = false, false
	return a == b
}

type rect struct{ x, y, w, h int }

func (r rect) contains(x, y int) bool {
	return x >= r.x && x < r.x+r.w && y >= r.y && y < r.y+r.h
}

type zone struct {
	act  action
	rect rect
	// scanned reports that scan found this zone's marker in the frame, which is
	// what fills in rect.x/y. Without it an unscanned zone would keep the zero
	// position mark gave it and answer for cell (0,0) — a phantom target in the
	// frame's top-left corner. Nothing dropped a marker before clip existed;
	// now that one can, the flag is what stops the corner going live.
	scanned bool
}

// hitMap collects the clickable regions of a single frame. It is rebuilt on
// every render; only hover survives from the previous one.
type hitMap struct {
	zones []zone
	hover action
}

// mark registers s as the click target for a and returns it prefixed with the
// marker scan looks for. The atom's size is taken here, where it is known;
// scan fills in the position.
func (h *hitMap) mark(a action, s string) string {
	if h == nil || a.kind == actNone {
		return s
	}
	id := len(h.zones)
	h.zones = append(h.zones, zone{
		act:  a,
		rect: rect{w: lipgloss.Width(s), h: lipgloss.Height(s)},
	})
	return markerStart + strconv.Itoa(id) + markerEnd + s
}

// hovered reports whether the pointer is currently over a.
func (h *hitMap) hovered(a action) bool {
	return h != nil && a.kind != actNone && h.hover == a
}

// scan records where every marked atom landed and returns the frame with the
// markers removed, ready to hand to the renderer.
func (h *hitMap) scan(frame string) string {
	if h == nil || !strings.Contains(frame, markerStart) {
		return frame
	}

	var b strings.Builder
	b.Grow(len(frame))

	for y, line := range strings.Split(frame, "\n") {
		if y > 0 {
			b.WriteByte('\n')
		}
		// Columns are counted in display cells, not bytes, so styled and wide
		// text place their markers correctly.
		col := 0
		for {
			i := strings.Index(line, markerStart)
			if i < 0 {
				break
			}
			j := strings.Index(line[i:], markerEnd)
			if j < 0 {
				break
			}

			b.WriteString(line[:i])
			col += lipgloss.Width(line[:i])

			if id, err := strconv.Atoi(line[i+len(markerStart) : i+j]); err == nil && id < len(h.zones) {
				h.zones[id].rect.x = col
				h.zones[id].rect.y = y
				h.zones[id].scanned = true
			}
			line = line[i+j+len(markerEnd):]
		}
		b.WriteString(line)
	}
	return b.String()
}

// clip moves the recorded regions into the coordinates the terminal actually
// shows, and drops the ones it does not show at all.
//
// scan measures the composed frame, but a frame taller than the terminal is not
// what the player sees: nothing here truncates it (lipgloss.PlaceVertical
// returns an over-tall block unchanged), so Bubble Tea's renderer takes the
// excess off the *top* — "if the frame height is greater than the screen
// height, we drop the lines from the top of the buffer". Clicks then arrive in
// terminal coordinates while every zone is still recorded in frame ones, and
// every target on the screen is wrong by exactly that overflow.
//
// So the overflow is subtracted here, once, where View already knows both
// heights. A zone that ends up above the first visible row is dropped rather
// than clamped: it is genuinely not on screen, and a target that answers for a
// cell the player cannot see is worse than no target.
func (h *hitMap) clip(dy, height int) {
	if h == nil || dy <= 0 {
		return
	}
	kept := h.zones[:0]
	for _, z := range h.zones {
		z.rect.y -= dy
		if z.rect.y+z.rect.h <= 0 || (height > 0 && z.rect.y >= height) {
			continue
		}
		kept = append(kept, z)
	}
	h.zones = kept
}

// at returns the action at a screen cell. Later marks win, so an atom drawn
// inside another is still reachable.
func (h *hitMap) at(x, y int) (action, bool) {
	if h == nil {
		return action{}, false
	}
	for i := len(h.zones) - 1; i >= 0; i-- {
		if z := h.zones[i]; z.scanned && z.rect.w > 0 && z.rect.contains(x, y) {
			return z.act, true
		}
	}
	return action{}, false
}

// find returns where an action was drawn. It is how tests click a target
// without hard-coding coordinates that every layout tweak would invalidate.
func (h *hitMap) find(a action) (rect, bool) {
	if h == nil {
		return rect{}, false
	}
	for _, z := range h.zones {
		if z.act.sameTarget(a) {
			return z.rect, true
		}
	}
	return rect{}, false
}
